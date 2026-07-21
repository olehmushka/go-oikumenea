package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// Factbook stages the CIA World Factbook country files (D-PhysicalIdentity amendment, M43) for the paged
// ethnicity pipeline. It enumerates every `<region>/<cc>.json` in the `factbook/factbook.json` GitHub
// mirror with ONE git-tree API call, then streams each raw file to a temp directory. A StreamingFetcher:
// ~260 whole Factbook docs exceed the 16 MiB in-memory cap, so the set is staged to disk and parsed by the
// factbookethnicities PagedMapper. The Factbook is US-government PUBLIC DOMAIN. Locator is `owner/repo@ref`
// (default `factbook/factbook.json@master`).
type Factbook struct{ client *http.Client }

// Fetch is never called for a StreamingFetcher (the pipeline routes it through Stage).
func (Factbook) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, ErrStreamingOnly
}

var (
	_ domain.Fetcher          = Factbook{}
	_ domain.StreamingFetcher = Factbook{}
)

// regionsSkip are top-level dirs in the mirror that are not per-country ethnicity sources.
var regionsSkip = map[string]bool{"meta": true, "world": true, "oceans": true, "antarctica": true}

type ghTree struct {
	SHA       string `json:"sha"`
	Truncated bool   `json:"truncated"`
	Tree      []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

// Stage enumerates the country files via the git-tree API and downloads each to a temp dir. The
// SourceVersion is the tree SHA — an unchanged upstream tree makes the import an idempotent no-op.
func (f Factbook) Stage(ctx context.Context, src domain.Source) (domain.StagedSource, error) {
	repo, ref := "factbook/factbook.json", "master"
	if loc := strings.TrimSpace(src.Locator); loc != "" {
		if at := strings.LastIndex(loc, "@"); at >= 0 {
			repo, ref = loc[:at], loc[at+1:]
		} else {
			repo = loc
		}
	}

	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/%s?recursive=1", repo, ref)
	body, err := f.get(ctx, treeURL, "application/json")
	if err != nil {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: list tree: %w", err)
	}
	var tree ghTree
	if err := json.Unmarshal(body, &tree); err != nil {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: decode tree: %w", err)
	}
	if tree.Truncated {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: git tree for %s@%s is truncated", repo, ref)
	}

	// Keep <region>/<cc>.json blobs from real country regions (the mapper additionally skips any file
	// without an "Ethnic groups" field, so this is only a download-count optimization).
	paths := make([]string, 0, 300)
	for _, e := range tree.Tree {
		if e.Type != "blob" || !strings.HasSuffix(e.Path, ".json") || strings.Count(e.Path, "/") != 1 {
			continue
		}
		if regionsSkip[e.Path[:strings.IndexByte(e.Path, '/')]] {
			continue
		}
		paths = append(paths, e.Path)
	}
	if len(paths) == 0 {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: no country files in %s@%s", repo, ref)
	}
	sort.Strings(paths)

	dir, err := os.MkdirTemp("", "hermenea-factbook-*")
	if err != nil {
		return domain.StagedSource{}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	combined := sha256.New()
	for _, p := range paths {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, ref, p)
		// Flatten <region>/<cc>.json -> <region>__<cc>.json so the staged dir is flat.
		dest := filepath.Join(dir, strings.ReplaceAll(p, "/", "__"))
		sum, err := f.download(ctx, rawURL, dest)
		if err != nil {
			cleanup()
			return domain.StagedSource{}, err
		}
		_, _ = fmt.Fprintf(combined, "%s:%s\n", p, sum)
	}

	checksum := hex.EncodeToString(combined.Sum(nil))
	version := tree.SHA
	if version == "" {
		version = checksum[:16]
	}
	return domain.StagedSource{Path: dir, SourceVersion: version, Checksum: checksum, Cleanup: cleanup}, nil
}

// get fetches a URL fully into memory (bounded) with the descriptive UA — used for the small tree JSON.
func (f Factbook) get(ctx context.Context, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxPayload))
}

// download streams url to dest and returns the sha256 of its content (no full in-memory copy).
func (f Factbook) download(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("connector factbook: %s returned %d", url, resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), resp.Body); err != nil {
		return "", fmt.Errorf("connector factbook: download %s: %w", url, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
