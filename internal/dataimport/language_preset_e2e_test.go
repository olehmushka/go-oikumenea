//go:build integration

// Full-preset end-to-end re-verification of the M18 language pipeline (D-Languages): loads the REAL
// bundled Glottolog 5.3 + CLDR-scripts presets through the actual hermenea mappers and the oikumenea
// import handlers into the test DB — the same path a docker-compose cross-service run exercises, minus
// HTTP. It is gated behind OIKUMENEA_LANG_E2E=1 because it loads ~27k languoids (a few seconds); the
// fast suite covers the same logic on a small fixture.
//
//	OIKUMENEA_LANG_E2E=1 OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestLanguagePresetsFullLoad -timeout 300s ./internal/dataimport/...
package dataimport_test

import (
	"context"
	"os"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/cldrscripts"
	hdomain "github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/glottolog"
)

const (
	glottologPreset   = "../../deploy/language-presets/glottolog-5.3.json"
	cldrScriptsPreset = "../../deploy/language-presets/cldr-scripts.json"
)

func mapRecords(t *testing.T, m hdomain.Mapper, path string) []domain.Record {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preset %s: %v", path, err)
	}
	out, err := m.Map(hdomain.RawBatch{Payload: payload, SourceVersion: "preset"})
	if err != nil {
		t.Fatalf("map %s: %v", path, err)
	}
	recs := make([]domain.Record, len(out))
	for i, r := range out {
		recs[i] = domain.Record(r)
	}
	return recs
}

func TestLanguagePresetsFullLoad(t *testing.T) {
	if os.Getenv("OIKUMENEA_LANG_E2E") == "" {
		t.Skip("set OIKUMENEA_LANG_E2E=1 to run the full 27k-languoid preset load")
	}
	ctx := context.Background()
	pool := newPool(t)
	svc := newLanguageImportService(t, pool)

	// 1) language-scheme: the full Glottolog forest, parent-first via the real mapper.
	scheme := mapRecords(t, glottolog.Mapper{}, glottologPreset)
	t.Logf("glottolog preset: %d languoids", len(scheme))
	sum, err := svc.Import(ctx, domain.ObjectTypeLanguageScheme, application.Envelope{
		ObjectType: domain.ObjectTypeLanguageScheme, Source: "glottolog", SourceVersion: "5.3", Records: scheme,
	})
	if err != nil {
		t.Fatalf("import scheme: %v", err)
	}
	t.Logf("scheme import: %+v", sum)
	// The migration seeds ~50 top languages, so those import as Updated, the rest as Created.
	if sum.Created+sum.Updated != len(scheme) {
		t.Fatalf("created+updated = %d, want %d", sum.Created+sum.Updated, len(scheme))
	}
	if sum.Updated < 50 {
		t.Fatalf("expected ≥50 seeded rows updated, got %d", sum.Updated)
	}

	// closure + family_code derived for the whole forest
	var languoids, closure, withFamily int
	pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.language_languoids`).Scan(&languoids)
	pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.language_languoid_closure`).Scan(&closure)
	pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.language_languoids WHERE family_code IS NOT NULL`).Scan(&withFamily)
	t.Logf("languoids=%d closure=%d withFamily=%d", languoids, closure, withFamily)
	if languoids < 20000 || closure < languoids || withFamily != languoids {
		t.Fatalf("unexpected counts: languoids=%d closure=%d withFamily=%d", languoids, closure, withFamily)
	}

	// the seeded UI locales are reconciled to their canonical languoid (eng -> English, ukr -> Ukrainian)
	var localeLinks int
	pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.i18n_locale_languages`).Scan(&localeLinks)
	if localeLinks < 2 {
		t.Fatalf("i18n_locale_languages = %d, want >= 2 (eng, ukr)", localeLinks)
	}

	// 2) language-scripts: the CLDR language->writing-system links via the real mapper.
	scripts := mapRecords(t, cldrscripts.Mapper{}, cldrScriptsPreset)
	t.Logf("cldr-scripts preset: %d links", len(scripts))
	ssum, err := svc.Import(ctx, domain.ObjectTypeLanguageScripts, application.Envelope{
		ObjectType: domain.ObjectTypeLanguageScripts, Source: "cldr", SourceVersion: "45", Records: scripts,
	})
	if err != nil {
		t.Fatalf("import scripts: %v", err)
	}
	t.Logf("scripts import: %+v", ssum)
	if ssum.Created == 0 {
		t.Fatalf("expected some script links created, got %+v", ssum)
	}

	// 3) idempotent re-run: same edition skips the whole forest.
	resum, err := svc.Import(ctx, domain.ObjectTypeLanguageScheme, application.Envelope{
		ObjectType: domain.ObjectTypeLanguageScheme, Source: "glottolog", SourceVersion: "5.3", Records: scheme,
	})
	if err != nil {
		t.Fatalf("re-import scheme: %v", err)
	}
	if resum.Skipped != len(scheme) || resum.Created != 0 || resum.Updated != 0 {
		t.Fatalf("re-import = %+v, want all %d skipped", resum, len(scheme))
	}
}
