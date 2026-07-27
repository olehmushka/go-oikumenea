// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the LANGUAGE pipeline (D-Languages, M18) against a real Postgres — the
// re-verification the i18n consistency review (milestones.md M18 Verdict) requires before M18 re-earns
// its `verified` gate. They exercise the oikumenea side end-to-end:
//
//   - language-scheme import: parent-first upsert; the transitive closure + denormalized family_code
//     are rebuilt in SQL; the i18n_locale_languages link is reconciled (locale ISO-639-3 -> languoid
//     iso639_3); a re-import of the same source_version is a pure no-op (idempotent).
//   - i18n name path (Verdict Gap 1): the languoid display name assembles as a locale->text map via
//     LocalizationService.NamesByID, and an i18n_translations override for entity_type='languoid'
//     surfaces in the map (the shape every other entity uses; D-i18n).
//   - writing systems: the migration-seeded ISO-15924 catalog reads back.
//
// Run against the dedicated test DB (migrations applied):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/dataimport/...
package dataimport_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/application"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	langadapters "github.com/olegamysk/go-oikumenea/internal/language/adapters"
	langapp "github.com/olegamysk/go-oikumenea/internal/language/application"
	langdomain "github.com/olegamysk/go-oikumenea/internal/language/domain"
	locadapters "github.com/olegamysk/go-oikumenea/internal/localization/adapters"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	locdomain "github.com/olegamysk/go-oikumenea/internal/localization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// newLanguageImportService builds an import service with the two language object-types registered.
func newLanguageImportService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc := application.NewService(pool, audit)
	svc.Register(domain.ObjectTypeLanguageScheme, application.LanguageSchemeHandler(
		func(conn pdb.DBTX) domain.LanguoidStore { return adapters.NewLanguoidRepo(conn) },
	))
	svc.Register(domain.ObjectTypeLanguageScripts, application.LanguageScriptsHandler(
		func(conn pdb.DBTX) domain.LanguageScriptStore { return adapters.NewLanguageScriptRepo(conn) },
	))
	return svc
}

// testGlottocodes are the synthetic codes these tests own. Cleanup is SCOPED to them (not a blanket
// truncate) so the language tests don't clobber the bare languoids other packages seed concurrently —
// `go test ./...` runs package binaries in parallel against the shared test DB.
// Synthetic 8-char codes that can never collide with a real glottocode (or the migration's 50-language
// seed) — the language tests own these and clean them up.
var testGlottocodes = []string{"zzfm0001", "zzln0001", "zzdl0001", "zzln0002"}

func cleanupLang(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	codes := testGlottocodes
	ids := `(SELECT id FROM oikumenea.language_languoids WHERE code = ANY($1))`
	// Dependent rows first (links/closure/ties/translations), then the languoids children-first.
	deps := []string{
		`DELETE FROM oikumenea.i18n_locale_languages WHERE language_id IN ` + ids,
		`DELETE FROM oikumenea.language_languoid_closure WHERE ancestor_id IN ` + ids + ` OR descendant_id IN ` + ids,
		`DELETE FROM oikumenea.language_languoid_countries WHERE languoid_id IN ` + ids,
		`DELETE FROM oikumenea.language_writing_systems WHERE languoid_id IN ` + ids,
		`DELETE FROM oikumenea.person_languages WHERE language_id IN ` + ids,
		// i18n_translations.entity_id is polymorphic TEXT, so compare against id::text.
		`DELETE FROM oikumenea.i18n_translations WHERE entity_type = 'languoid' AND entity_id IN (SELECT id::text FROM oikumenea.language_languoids WHERE code = ANY($1))`,
	}
	for _, s := range deps {
		if _, err := pool.Exec(ctx, s, codes); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
	// The synthetic 'qtz' UI locale used by the reconcile test (its locale_language link is removed
	// above by language_id; this drops the locale row itself). ISO-639-3 'qtz' is reserved for local
	// use, so it never collides with a real seeded/imported languoid's iso639_3.
	if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.i18n_locales WHERE code = 'qtz'`); err != nil {
		t.Fatalf("cleanup qtz locale: %v", err)
	}
	// dialect -> language -> family (parent_id RESTRICT): delete deepest level first.
	for _, lvl := range []string{"dialect", "language", "family"} {
		if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.language_languoids WHERE code = ANY($1) AND level = $2`, codes, lvl); err != nil {
			t.Fatalf("cleanup languoids %s: %v", lvl, err)
		}
	}
}

func TestLanguageSchemeImportClosureAndReconcile(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newLanguageImportService(t, pool)
	cleanupLang(t, pool)
	t.Cleanup(func() { cleanupLang(t, pool) })

	// A synthetic UI locale whose ISO-639-3 ('qtz', reserved for local use) matches the imported test
	// languoid below — so reconcile binds it to the imported row without colliding with the migration's
	// pre-seeded eng/ukr locale links. Disabled + non-default so the default-enforcement trigger is unaffected.
	if _, err := pool.Exec(ctx, `INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order)
		VALUES ('qtz', 'Zz Test Locale', false, false, 999) ON CONFLICT (code) DO NOTHING`); err != nil {
		t.Fatalf("seed qtz locale: %v", err)
	}

	// Parent-first: family -> language (iso qtz, matching the seeded 'qtz' locale) -> dialect. Synthetic
	// codes so they never collide with the migration's 50-language seed.
	records := []domain.Record{
		{"code": "zzfm0001", "level": "family", "name": "Zz Test Family"},
		{"code": "zzln0001", "level": "language", "name": "Zz Test Language", "parent": "zzfm0001", "iso639_3": "qtz", "status": "not_endangered"},
		{"code": "zzdl0001", "level": "dialect", "name": "Zz Test Dialect", "parent": "zzln0001"},
	}
	env := func(ver string) application.Envelope {
		return application.Envelope{ObjectType: domain.ObjectTypeLanguageScheme, Source: "glottolog", SourceVersion: ver, Records: records}
	}

	sum, err := svc.Import(ctx, domain.ObjectTypeLanguageScheme, env("5.3"))
	if err != nil {
		t.Fatalf("import scheme: %v", err)
	}
	if sum.Created != 3 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("create summary = %+v, want Created=3", sum)
	}

	// Closure: Indo-European reaches the dialect at depth 2.
	var depth int
	if err := pool.QueryRow(ctx, `
		SELECT c.depth FROM oikumenea.language_languoid_closure c
		JOIN oikumenea.language_languoids a ON a.id = c.ancestor_id
		JOIN oikumenea.language_languoids d ON d.id = c.descendant_id
		WHERE a.code = 'zzfm0001' AND d.code = 'zzdl0001'`).Scan(&depth); err != nil {
		t.Fatalf("closure depth: %v", err)
	}
	if depth != 2 {
		t.Fatalf("closure depth = %d, want 2", depth)
	}

	// family_code derived for every node (the dialect's root family is Indo-European).
	var familyCode string
	if err := pool.QueryRow(ctx, `SELECT family_code FROM oikumenea.language_languoids WHERE code = 'zzdl0001'`).Scan(&familyCode); err != nil {
		t.Fatalf("family_code: %v", err)
	}
	if familyCode != "zzfm0001" {
		t.Fatalf("family_code = %q, want zzfm0001", familyCode)
	}

	// Verdict Gap 2: the 'qtz' locale is reconciled to the imported languoid by iso639_3.
	var qtzLangCode string
	if err := pool.QueryRow(ctx, `
		SELECT l.code FROM oikumenea.i18n_locale_languages ll
		JOIN oikumenea.language_languoids l ON l.id = ll.language_id
		WHERE ll.locale = 'qtz'`).Scan(&qtzLangCode); err != nil {
		t.Fatalf("locale-language reconcile: %v", err)
	}
	if qtzLangCode != "zzln0001" {
		t.Fatalf("locale qtz -> %q, want zzln0001", qtzLangCode)
	}

	// Idempotent re-run: same edition -> all skipped.
	sum, err = svc.Import(ctx, domain.ObjectTypeLanguageScheme, env("5.3"))
	if err != nil {
		t.Fatalf("re-import scheme: %v", err)
	}
	if sum.Skipped != 3 || sum.Created != 0 || sum.Updated != 0 {
		t.Fatalf("re-import summary = %+v, want Skipped=3", sum)
	}
}

func TestLanguageNameLocaleMapAndOverride(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	imp := newLanguageImportService(t, pool)
	cleanupLang(t, pool)
	t.Cleanup(func() { cleanupLang(t, pool) })

	if _, err := imp.Import(ctx, domain.ObjectTypeLanguageScheme, application.Envelope{
		ObjectType: domain.ObjectTypeLanguageScheme, Source: "glottolog", SourceVersion: "5.3",
		Records: []domain.Record{
			{"code": "zzfm0001", "level": "family", "name": "Zz Test Family"},
			{"code": "zzln0002", "level": "language", "name": "Zztestish", "parent": "zzfm0001", "status": "not_endangered"},
		},
	}); err != nil {
		t.Fatalf("import: %v", err)
	}

	langSvc := langapp.NewService(pool, func(conn pdb.DBTX) langdomain.Repository { return langadapters.NewRepository(conn) })
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository { return auditadapters.NewRepository(conn) }, func() int { return 50 })
	locSvc := locapp.NewService(pool, func(conn pdb.DBTX) locdomain.Repository { return locadapters.NewRepository(conn) }, audit)

	// Resolve the test languoid (unique synthetic name, so the 50-language seed can't shadow it).
	l, found, err := func() (langdomain.Languoid, bool, error) {
		ls, err := langSvc.ListLanguoids(ctx, langdomain.Filter{Query: "Zztestish"})
		if err != nil || len(ls) == 0 {
			return langdomain.Languoid{}, false, err
		}
		return ls[0], true, nil
	}()
	if err != nil || !found {
		t.Fatalf("resolve languoid: found=%v err=%v", found, err)
	}

	// Verdict Gap 1: the name assembles as a locale->text map seeded from the default-locale column.
	names, err := locSvc.NamesByID(ctx, "languoid", map[string]string{l.ID: l.Name})
	if err != nil {
		t.Fatalf("NamesByID: %v", err)
	}
	def, err := locSvc.DefaultLocale(ctx)
	if err != nil {
		t.Fatalf("DefaultLocale: %v", err)
	}
	if names[l.ID][def] != "Zztestish" {
		t.Fatalf("name map[%s] = %q, want Zztestish; full map=%v", def, names[l.ID][def], names[l.ID])
	}

	// An i18n_translations override for entity_type='languoid' surfaces in the map (D-i18n shape).
	if _, err := locSvc.UpsertTranslations(ctx, "languoid", l.ID, []locdomain.Translation{
		{EntityType: "languoid", EntityID: l.ID, Field: "name", Locale: "ukr", Text: "Англійська"},
	}); err != nil {
		t.Fatalf("upsert override: %v", err)
	}
	names, err = locSvc.NamesByID(ctx, "languoid", map[string]string{l.ID: l.Name})
	if err != nil {
		t.Fatalf("NamesByID after override: %v", err)
	}
	if names[l.ID]["ukr"] != "Англійська" {
		t.Fatalf("override not surfaced: map=%v", names[l.ID])
	}

	// The migration-seeded ISO-15924 writing-system catalog reads back.
	wss, err := langSvc.ListWritingSystems(ctx)
	if err != nil {
		t.Fatalf("list writing systems: %v", err)
	}
	if len(wss) == 0 {
		t.Fatalf("expected migration-seeded writing systems, got 0")
	}
}
