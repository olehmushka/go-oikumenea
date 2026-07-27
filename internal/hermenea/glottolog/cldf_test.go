// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package glottolog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestCLDFMapper transforms a small raw Glottolog CLDF fixture (languages.csv + values.csv) and asserts
// the canonical records: parent-first order, classification→parent (last path segment), aes→status,
// ISO/macroarea/lat-lon, and countries split+upper.
func TestCLDFMapper(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("languages.csv", `ID,Name,Level,ISO639P3code,Macroarea,Latitude,Longitude,Countries
indo1319,Indo-European,family,,,,,
stan1293,English,language,eng,Eurasia,52.0,0.0,gb;us
some1234,Some Dialect,dialect,,,,,
`)
	write("values.csv", `Language_ID,Parameter_ID,Value,Code_ID
stan1293,classification,indo1319,
some1234,classification,indo1319/stan1293,
stan1293,aes,,aes-not_endangered
some1234,aes,,aes-threatened
`)

	var got []map[string]any
	err := CLDFMapper{}.MapPaged(context.Background(), domain.StagedSource{Path: dir}, func(recs []map[string]any) error {
		got = recs
		return nil
	})
	if err != nil {
		t.Fatalf("MapPaged: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d records, want 3", len(got))
	}
	// parent-first: family, then language, then dialect.
	order := []string{"indo1319", "stan1293", "some1234"}
	for i, code := range order {
		if got[i]["code"] != code {
			t.Fatalf("record %d code = %v, want %s (full order %v)", i, got[i]["code"], code, order)
		}
	}
	eng := got[1]
	if eng["parent"] != "indo1319" || eng["iso639_3"] != "eng" || eng["status"] != "not_endangered" || eng["macroarea"] != "Eurasia" {
		t.Fatalf("english record = %+v", eng)
	}
	cs, ok := eng["countries"].([]any)
	if !ok || len(cs) != 2 || cs[0] != "GB" || cs[1] != "US" {
		t.Fatalf("english countries = %v, want [GB US]", eng["countries"])
	}
	if got[2]["parent"] != "stan1293" || got[2]["status"] != "threatened" {
		t.Fatalf("dialect record = %+v", got[2])
	}
}
