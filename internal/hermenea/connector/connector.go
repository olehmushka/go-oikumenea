// Package connector implements the hermenea Connector seam (M16 / D-Hermenea): pluggable fetchers
// that pull a source's raw payload. Two ship now — HTTP(S) download and the degenerate `file` case
// (bundled presets / deterministic tests). New source types (DS-44) are new Connector impls.
package connector

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
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// maxPayload bounds a fetched body (16 MiB) so a runaway source can't exhaust memory.
const maxPayload = 16 << 20

// ErrStreamingOnly is returned by a StreamingConnector's Fetch: large sources go through Stage + the
// paged pipeline, never the in-memory Fetch path.
var ErrStreamingOnly = errors.New("connector is streaming-only; use the paged pipeline")

// Registry maps a connector-type to its Connector.
type Registry map[string]domain.Connector

// Default builds the standard connector registry (http + file + wof-sqlite). The wof-sqlite client has
// no overall timeout — a planet-scale download is governed by the job timeout / context deadline, not a
// fixed client deadline (operators raise worker.job-timeout-ms for WOF sources).
func Default() Registry {
	return Registry{
		domain.ConnectorHTTP:      HTTP{client: &http.Client{Timeout: 30 * time.Second}},
		domain.ConnectorFile:      File{},
		domain.ConnectorWOFSQLite: WOFSQLite{client: &http.Client{Timeout: 0}},
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

// Fetch is never called for a StreamingConnector (the pipeline routes it through Stage).
func (WOFSQLite) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, ErrStreamingOnly
}

// compile-time assertions: WOFSQLite is both a Connector (registry value) and a StreamingConnector.
var (
	_ domain.Connector          = WOFSQLite{}
	_ domain.StreamingConnector = WOFSQLite{}
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
