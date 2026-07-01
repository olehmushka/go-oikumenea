package factbookethnicities

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMapPagedParsesAndDedups drives the PagedMapper over a small staged directory of fake Factbook
// country files (no network): it must parse the free-text, derive ISO from the ccTLD (incl. .uk->GB),
// dedup group names across countries, and drop catch-alls + fragments.
func TestMapPagedParsesAndDedups(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Ukraine (.ua -> UA): Ukrainian + Russian + catch-all "other".
	write("europe__up.json", `{"People and Society":{"Ethnic groups":{"text":"Ukrainian 77.8%, Russian 17.3%, Crimean Tatar 0.5%, other 4.9% (2001 est.)"}},"Communications":{"Internet country code":{"text":".ua"}}}`)
	// United Kingdom (.uk -> GB exception): White + Russian (dedup with Ukraine) + a note clause.
	write("europe__uk.json", `{"People and Society":{"Ethnic groups":{"text":"White 87.2%, Russian 0.5% (2011 est.) note: data represents Great Britain"}},"Communications":{"Internet country code":{"text":".uk"}}}`)
	// A non-country file with no "Ethnic groups" — must be skipped, not fail the run.
	write("meta__categories.json", `{"Government":{"Capital":{"text":"n/a"}}}`)

	var got []map[string]any
	err := PagedMapper{}.MapPaged(context.Background(), domain.StagedSource{Path: dir}, func(recs []map[string]any) error {
		got = append(got, recs...)
		return nil
	})
	if err != nil {
		t.Fatalf("MapPaged: %v", err)
	}

	by := map[string]map[string]any{}
	for _, r := range got {
		by[r["code"].(string)] = r
	}
	strs := func(code string) []string {
		out := []string{}
		if v, ok := by[code]["countries"].([]any); ok {
			for _, c := range v {
				out = append(out, c.(string))
			}
		}
		return out
	}

	if _, ok := by["other"]; ok {
		t.Fatal("catch-all 'other' must be dropped")
	}
	if by["ukrainian"] == nil || by["ukrainian"]["name"] != "Ukrainian" {
		t.Fatalf("ukrainian missing/misnamed: %+v", by["ukrainian"])
	}
	if got := strs("ukrainian"); len(got) != 1 || got[0] != "UA" {
		t.Fatalf("ukrainian countries = %v, want [UA]", got)
	}
	// White comes only from the UK file; .uk maps to GB (the ccTLD exception).
	if got := strs("white"); len(got) != 1 || got[0] != "GB" {
		t.Fatalf("white countries = %v, want [GB]", got)
	}
	// Russian appears in both files -> deduped to one record spanning both countries (sorted).
	if got := strs("russian"); len(got) != 2 || got[0] != "GB" || got[1] != "UA" {
		t.Fatalf("russian countries = %v, want [GB UA]", got)
	}
	// Multi-word name kept verbatim; the "note:" clause after UK's percent did not leak a bogus group.
	if by["crimean-tatar"] == nil || by["crimean-tatar"]["name"] != "Crimean Tatar" {
		t.Fatalf("crimean-tatar missing/misnamed: %+v", by["crimean-tatar"])
	}
}
