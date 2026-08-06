// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package cldrscripts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// TestSupplementalMapper transforms a small raw CLDR fixture (supplementalData.xml + iso-639-3.tab) and
// asserts: the 639-1→3 resolution (en→eng, ru→rus, de→deu), primary ordering (first script is primary),
// and alt="secondary" links emitted as non-primary.
func TestSupplementalMapper(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("iso-639-3.tab", "Id\tPart1\tRef_Name\neng\ten\tEnglish\nrus\tru\tRussian\ndeu\tde\tGerman\n")
	write("supplementalData.xml", `<supplementalData><languageData>
  <language type="en" scripts="Latn Brai"/>
  <language type="ru" scripts="Cyrl"/>
  <language type="de" scripts="Runr" alt="secondary"/>
  <language type="xx"/>
</languageData></supplementalData>`)

	var got []map[string]any
	err := SupplementalMapper{}.MapPaged(context.Background(), domain.StagedSource{Path: dir}, func(recs []map[string]any) error {
		got = recs
		return nil
	})
	if err != nil {
		t.Fatalf("MapPaged: %v", err)
	}
	// expected (sorted by iso, ws): deu/Runr(false), eng/Brai(false), eng/Latn(true), rus/Cyrl(true)
	want := []struct {
		iso, ws   string
		isPrimary bool
	}{
		{"deu", "Runr", false},
		{"eng", "Brai", false},
		{"eng", "Latn", true},
		{"rus", "Cyrl", true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d links, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i]["iso639_3"] != w.iso || got[i]["writingSystem"] != w.ws || got[i]["isPrimary"] != w.isPrimary {
			t.Fatalf("link %d = %+v, want %+v", i, got[i], w)
		}
	}
}
