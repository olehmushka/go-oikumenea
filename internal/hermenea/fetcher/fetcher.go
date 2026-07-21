// Package fetcher implements the hermenea Fetcher seam (M16 / D-Hermenea): pluggable strategies that
// pull a source's raw payload. Two ship now — HTTP(S) download and the degenerate `file` case
// (bundled presets / deterministic tests). New source types (DS-44) are new Fetcher impls.
//
// NAMING (M53 / D-ConnectorPlane): this seam was called `Connector` through M52. The connector plane
// gives "connector" a different, higher meaning in the core — a whole deployable agent, of which
// hermenea is one — so the fetch strategy is a `Fetcher` here to keep the two altitudes distinct.
// The names AT REST deliberately did not move: the column `hermenea.import_sources.connector_type`,
// the install-config key `connector-type:`, and the Conjure field `connectorType` are all unchanged,
// because renaming them would break every deployment's config and need a migration for zero benefit.
// So a mapping like `ConnectorType: s.FetcherType` in the adapters is correct, not a leftover.
package fetcher

import (
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// maxPayload bounds a fetched body (16 MiB) so a runaway source can't exhaust memory.
const maxPayload = 16 << 20

// ErrStreamingOnly is returned by a StreamingFetcher's Fetch: large sources go through Stage + the
// paged pipeline, never the in-memory Fetch path.
var ErrStreamingOnly = errors.New("fetcher is streaming-only; use the paged pipeline")

// Registry maps a connector-type (the at-rest discriminator — see the package doc) to its Fetcher.
type Registry map[string]domain.Fetcher

// Default builds the standard fetcher registry (http + file + wof-sqlite). The wof-sqlite client has
// no overall timeout — a planet-scale download is governed by the job timeout / context deadline, not a
// fixed client deadline (operators raise worker.job-timeout-ms for WOF sources).
func Default() Registry {
	return Registry{
		domain.FetcherHTTP:      HTTP{client: &http.Client{Timeout: 30 * time.Second}},
		domain.FetcherFile:      File{},
		domain.FetcherWOFSQLite: WOFSQLite{client: &http.Client{Timeout: 0}},
		// No fixed client deadline: the Glottolog CLDF values.csv is large and the transform runs per
		// run; bounded by the job-timeout context instead (mirrors WOFSQLite).
		domain.FetcherHTTPFiles: HTTPFiles{client: &http.Client{Timeout: 0}},
		// Factbook stages ~260 country files; no fixed client deadline (bounded by the job timeout).
		domain.FetcherFactbook: Factbook{client: &http.Client{Timeout: 0}},
	}
}

// HTTP fetches a source's payload over HTTP(S). source_version comes from a strong validator header
// (ETag / Last-Modified) when present, else a short content checksum.
type HTTP struct{ client *http.Client }

func (h HTTP) Fetch(ctx context.Context, src domain.Source) (domain.RawBatch, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.Locator, nil)
	if err != nil {
		return domain.RawBatch{}, err
	}
	// A descriptive User-Agent + a JSON Accept are required by some public APIs (e.g. the Wikidata SPARQL
	// endpoint 403s a request with the default Go UA) and harmless for plain file/JSON sources.
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return domain.RawBatch{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.RawBatch{}, fmt.Errorf("connector http: %s returned %d", src.Locator, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPayload))
	if err != nil {
		return domain.RawBatch{}, err
	}
	sum := checksum(body)
	version := firstNonEmpty(resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), sum[:12])
	return domain.RawBatch{Payload: body, SourceVersion: version, Checksum: sum}, nil
}

// File reads a source's payload from a bundled file (the degenerate connector; used for presets and
// deterministic tests). source_version is the file mod-time, falling back to a short checksum.
type File struct{}

func (File) Fetch(_ context.Context, src domain.Source) (domain.RawBatch, error) {
	body, err := os.ReadFile(src.Locator)
	if err != nil {
		return domain.RawBatch{}, err
	}
	sum := checksum(body)
	version := sum[:12]
	if fi, err := os.Stat(src.Locator); err == nil {
		version = fi.ModTime().UTC().Format(time.RFC3339)
	}
	return domain.RawBatch{Payload: body, SourceVersion: version, Checksum: sum}, nil
}

// WOFSQLite stages a Who's-On-First SQLite distribution (D-GeoPlaces): it streams the source's
// `.db.bz2` over HTTP, bzip2-decompresses it to a temp file, and returns the file reference — the
// payload (gigabytes of geometry) never lands in memory or the raw-batch column. The geo-places
// PagedMapper opens the staged DB. source_version is the response's ETag/Last-Modified (the dist
// edition), falling back to a checksum of the decompressed bytes.
type WOFSQLite struct{ client *http.Client }

// Fetch is never called for a StreamingFetcher (the pipeline routes it through Stage).
func (WOFSQLite) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, ErrStreamingOnly
}

// compile-time assertions: WOFSQLite is both a Connector (registry value) and a StreamingFetcher.
var (
	_ domain.Fetcher          = WOFSQLite{}
	_ domain.StreamingFetcher = WOFSQLite{}
)

func (w WOFSQLite) Stage(ctx context.Context, src domain.Source) (domain.StagedSource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.Locator, nil)
	if err != nil {
		return domain.StagedSource{}, err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return domain.StagedSource{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return domain.StagedSource{}, fmt.Errorf("connector wof-sqlite: %s returned %d", src.Locator, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "wof-*.db")
	if err != nil {
		return domain.StagedSource{}, err
	}
	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }

	// Decompress straight to disk, hashing the decompressed bytes as we go (no full in-memory copy).
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), bzip2.NewReader(resp.Body)); err != nil {
		_ = f.Close()
		cleanup()
		return domain.StagedSource{}, fmt.Errorf("connector wof-sqlite: decompress %s: %w", src.Locator, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return domain.StagedSource{}, err
	}

	sum := hex.EncodeToString(h.Sum(nil))
	version := firstNonEmpty(resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), sum[:12])
	return domain.StagedSource{Path: path, SourceVersion: version, Checksum: sum, Cleanup: cleanup}, nil
}

// HTTPFiles stages a whitespace-separated LIST of URLs to a temp directory (D-Languages, M18): each is
// streamed to dir/<url-basename> (no in-memory cap — raw Glottolog values.csv is large), and the paged
// language mappers read the files they need from the directory by name. source_version is a sha256 over
// the per-file content digests (sorted by name) so an unchanged upstream master skips idempotently.
type HTTPFiles struct{ client *http.Client }

// compile-time assertions: HTTPFiles is both a Connector (registry value) and a StreamingFetcher.
var (
	_ domain.Fetcher          = HTTPFiles{}
	_ domain.StreamingFetcher = HTTPFiles{}
)

// Fetch is never called for a StreamingFetcher (the pipeline routes it through Stage).
func (HTTPFiles) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, ErrStreamingOnly
}

func (h HTTPFiles) Stage(ctx context.Context, src domain.Source) (domain.StagedSource, error) {
	urls := strings.Fields(src.Locator)
	if len(urls) == 0 {
		return domain.StagedSource{}, fmt.Errorf("connector http-files: empty locator (expected a whitespace-separated URL list)")
	}
	dir, err := os.MkdirTemp("", "hermenea-files-*")
	if err != nil {
		return domain.StagedSource{}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	fileSums := make(map[string]string, len(urls))
	names := make([]string, 0, len(urls))
	for _, u := range urls {
		name := path.Base(u)
		if i := strings.IndexAny(name, "?#"); i >= 0 {
			name = name[:i]
		}
		if name == "" || name == "." || name == "/" {
			cleanup()
			return domain.StagedSource{}, fmt.Errorf("connector http-files: cannot derive a filename from %q", u)
		}
		if _, dup := fileSums[name]; dup {
			cleanup()
			return domain.StagedSource{}, fmt.Errorf("connector http-files: duplicate staged filename %q", name)
		}
		sum, err := h.download(ctx, u, filepath.Join(dir, name))
		if err != nil {
			cleanup()
			return domain.StagedSource{}, err
		}
		fileSums[name] = sum
		names = append(names, name)
	}

	sort.Strings(names)
	combined := sha256.New()
	for _, name := range names {
		_, _ = fmt.Fprintf(combined, "%s:%s\n", name, fileSums[name])
	}
	sum := hex.EncodeToString(combined.Sum(nil))
	return domain.StagedSource{Path: dir, SourceVersion: sum[:16], Checksum: sum, Cleanup: cleanup}, nil
}

// userAgentEnv lets an operator override the outbound User-Agent (e.g. to supply their OWN contact for
// upstreams that require one). Nothing personal is baked into the binary.
const userAgentEnv = "OIKUMENEA_HTTP_USER_AGENT"

// defaultUserAgent is a NEUTRAL default that identifies the importer without leaking any personal
// contact. Some hosts (e.g. SIL/iso639-3.sil.org) 403 the bare Go default "Go-http-client" UA, so a
// descriptive product token is still sent. Sources with a stricter policy — notably the Wikidata SPARQL
// endpoint (Wikimedia UA policy, D-ExternalOrgs / M30) — expect a CONTACT (URL + e-mail); operators that
// use those sources should set OIKUMENEA_HTTP_USER_AGENT to a string carrying their own contact.
const defaultUserAgent = "go-oikumenea/1.0"

// userAgent is the effective outbound User-Agent, resolved once from the env override or the neutral
// default.
var userAgent = resolveUserAgent()

// UserAgent returns the effective outbound User-Agent (env override or the neutral default). Exported so
// other hermenea HTTP callers (e.g. the watchlist connectors) share the same value.
func UserAgent() string { return userAgent }

func resolveUserAgent() string {
	if v := strings.TrimSpace(os.Getenv(userAgentEnv)); v != "" {
		return v
	}
	return defaultUserAgent
}

// download streams url to dest and returns the sha256 of its content (no full in-memory copy).
func (h HTTPFiles) download(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("connector http-files: %s returned %d", url, resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	hsh := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, hsh), resp.Body); err != nil {
		return "", fmt.Errorf("connector http-files: download %s: %w", url, err)
	}
	return hex.EncodeToString(hsh.Sum(nil)), nil
}

func checksum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
