package hermenea

import (
	"context"
	"os"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/cldrscripts"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/fetcher"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/glottolog"
)

// TestLiveLanguageImport exercises the LIVE language path end-to-end up to the load boundary: the
// http-files connector stages the real Glottolog CLDF + CLDR upstream master, and the Go mappers
// transform them. Network-gated (HERMENEA_LIVE=1) so it stays out of the default suite, like the
// dataimport OIKUMENEA_LANG_E2E gate. It does not hit oikumenea — it proves fetch + transform.
func TestLiveLanguageImport(t *testing.T) {
	if os.Getenv("HERMENEA_LIVE") == "" {
		t.Skip("set HERMENEA_LIVE=1 to run the live upstream fetch+transform")
	}
	ctx := context.Background()
	sc, ok := fetcher.Default()[domain.FetcherHTTPFiles].(domain.StreamingFetcher)
	if !ok {
		t.Fatal("http-files connector is not a StreamingFetcher")
	}

	// Glottolog CLDF → language-scheme records (the whole forest, parent-first).
	staged, err := sc.Stage(ctx, domain.Source{Locator: "" +
		"https://raw.githubusercontent.com/glottolog/glottolog-cldf/master/cldf/languages.csv " +
		"https://raw.githubusercontent.com/glottolog/glottolog-cldf/master/cldf/values.csv"})
	if err != nil {
		t.Fatalf("stage glottolog: %v", err)
	}
	t.Cleanup(staged.Cleanup)
	var langs []map[string]any
	if err := (glottolog.CLDFMapper{}).MapPaged(ctx, staged, func(r []map[string]any) error { langs = r; return nil }); err != nil {
		t.Fatalf("map glottolog: %v", err)
	}
	t.Logf("glottolog live: %d languoids (version %s)", len(langs), staged.SourceVersion)
	if len(langs) < 20000 {
		t.Fatalf("expected ≫20k languoids from live master, got %d", len(langs))
	}
	if _, hasParent := langs[0]["parent"]; hasParent {
		t.Fatalf("first record should be a root (no parent): %+v", langs[0])
	}

	// CLDR supplemental + ISO-639 → language-scripts records.
	staged2, err := sc.Stage(ctx, domain.Source{Locator: "" +
		"https://raw.githubusercontent.com/unicode-org/cldr/main/common/supplemental/supplementalData.xml " +
		"https://iso639-3.sil.org/sites/iso639-3/files/downloads/iso-639-3.tab"})
	if err != nil {
		t.Fatalf("stage cldr: %v", err)
	}
	t.Cleanup(staged2.Cleanup)
	var links []map[string]any
	if err := (cldrscripts.SupplementalMapper{}).MapPaged(ctx, staged2, func(r []map[string]any) error { links = r; return nil }); err != nil {
		t.Fatalf("map cldr: %v", err)
	}
	t.Logf("cldr live: %d language→script links", len(links))
	if len(links) == 0 {
		t.Fatal("expected some CLDR language→script links")
	}
}
