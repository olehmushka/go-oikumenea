// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	factbookclient "github.com/olehmushka/go-factbook-client"
	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// Factbook stages the CIA World Factbook country files (D-PhysicalIdentity amendment, M43) for the paged
// ethnicity pipeline. It enumerates every `<region>/<cc>.json` in the `factbook/factbook.json` GitHub
// mirror with ONE git-tree API call, then streams each raw file to a temp directory. A StreamingFetcher:
// ~260 whole Factbook docs exceed the 16 MiB in-memory cap, so the set is staged to disk and parsed by the
// factbookethnicities PagedMapper. The Factbook is US-government PUBLIC DOMAIN. Locator is `owner/repo@ref`
// (default `factbook/factbook.json@master`). The git-tree walk and per-file download are
// go-factbook-client, extracted from this file; the staging (temp dir, combined checksum,
// StagedSource/Cleanup) below stays here — hermenea's own batch-import concern, not the client's.
type Factbook struct{ client *factbookclient.Client }

// Fetch is never called for a StreamingFetcher (the pipeline routes it through Stage).
func (Factbook) Fetch(context.Context, domain.Source) (domain.RawBatch, error) {
	return domain.RawBatch{}, ErrStreamingOnly
}

var (
	_ domain.Fetcher          = Factbook{}
	_ domain.StreamingFetcher = Factbook{}
)

// Stage enumerates the country files via the git-tree API and downloads each to a temp dir. The
// SourceVersion is the tree SHA — an unchanged upstream tree makes the import an idempotent no-op.
func (f Factbook) Stage(ctx context.Context, src domain.Source) (domain.StagedSource, error) {
	repo, ref := factbookclient.DefaultRepo, factbookclient.DefaultRef
	if loc := strings.TrimSpace(src.Locator); loc != "" {
		if at := strings.LastIndex(loc, "@"); at >= 0 {
			repo, ref = loc[:at], loc[at+1:]
		} else {
			repo = loc
		}
	}

	files, treeSHA, err := f.client.ListCountryFiles(ctx, repo, ref)
	if err != nil {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: list tree: %w", err)
	}
	if len(files) == 0 {
		return domain.StagedSource{}, fmt.Errorf("connector factbook: no country files in %s@%s", repo, ref)
	}

	dir, err := os.MkdirTemp("", "hermenea-factbook-*")
	if err != nil {
		return domain.StagedSource{}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	combined := sha256.New()
	for _, file := range files {
		// Flatten <region>/<cc>.json -> <region>__<cc>.json so the staged dir is flat — mirrors
		// go-factbook-client's own FetchAll flattening, done here so the per-file checksum loop below
		// stays under hermenea's control for the combined SourceVersion/Checksum computation.
		dest := filepath.Join(dir, strings.ReplaceAll(file.Path, "/", "__"))
		sum, err := f.client.DownloadCountryFile(ctx, repo, ref, file, dest)
		if err != nil {
			cleanup()
			return domain.StagedSource{}, err
		}
		_, _ = fmt.Fprintf(combined, "%s:%s\n", file.Path, sum)
	}

	checksum := hex.EncodeToString(combined.Sum(nil))
	version := treeSHA
	if version == "" {
		version = checksum[:16]
	}
	return domain.StagedSource{Path: dir, SourceVersion: version, Checksum: checksum, Cleanup: cleanup}, nil
}
