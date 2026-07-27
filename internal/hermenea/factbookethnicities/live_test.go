// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package factbookethnicities_test

// Live end-to-end test of the runtime Factbook ethnicity pipeline (D-PhysicalIdentity amendment, M43):
// the `factbook` StreamingFetcher enumerates + downloads the real CIA World Factbook country files from
// GitHub, and the PagedMapper parses them into ethnicity-scheme records — NO committed preset, all at
// runtime, in Go. Network-dependent, so env-gated (mirrors the WOF / Wikidata live tests):
//
//	OIKUMENEA_FACTBOOK_E2E=1 go test -run TestFactbookLive ./internal/hermenea/factbookethnicities/

import (
	"context"
	"os"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/fetcher"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/factbookethnicities"
)

func TestFactbookLive(t *testing.T) {
	if os.Getenv("OIKUMENEA_FACTBOOK_E2E") == "" {
		t.Skip("set OIKUMENEA_FACTBOOK_E2E=1 to run the live Factbook fetch")
	}
	ctx := context.Background()

	sc, ok := fetcher.Default()[domain.FetcherFactbook].(domain.StreamingFetcher)
	if !ok {
		t.Fatal("factbook connector is not a StreamingFetcher")
	}
	staged, err := sc.Stage(ctx, domain.Source{Locator: "factbook/factbook.json@master"})
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	defer staged.Cleanup()
	if staged.SourceVersion == "" {
		t.Fatal("expected a non-empty source version (git tree sha)")
	}

	var recs []map[string]any
	if err := (factbookethnicities.PagedMapper{}).MapPaged(ctx, staged, func(p []map[string]any) error {
		recs = append(recs, p...)
		return nil
	}); err != nil {
		t.Fatalf("MapPaged: %v", err)
	}

	if len(recs) < 400 {
		t.Fatalf("expected a few hundred ethnic groups, got %d", len(recs))
	}
	by := map[string]map[string]any{}
	for _, r := range recs {
		by[r["code"].(string)] = r
	}
	countries := func(code string) map[string]bool {
		m := map[string]bool{}
		if v, ok := by[code]["countries"].([]any); ok {
			for _, c := range v {
				m[c.(string)] = true
			}
		}
		return m
	}
	if by["ukrainian"] == nil || !countries("ukrainian")["UA"] {
		t.Fatalf("ukrainian missing or not linked to UA: %+v", by["ukrainian"])
	}
	ru := countries("russian")
	if !ru["RU"] || !ru["UA"] {
		t.Fatalf("russian not linked to RU+UA: %v", ru)
	}
	if !countries("white")["GB"] {
		t.Fatalf("white not linked to GB (.uk->GB): %v", countries("white"))
	}
	t.Logf("live Factbook: %d ethnic groups (tree %s)", len(recs), staged.SourceVersion)
}
