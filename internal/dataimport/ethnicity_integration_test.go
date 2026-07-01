//go:build integration

// Integration test for the ETHNICITY pipeline (D-PhysicalIdentity amendment, M43) against a real
// Postgres. Exercises the oikumenea side of POST /import/ethnicity-scheme end-to-end:
//
//   - parent-first hierarchical upsert into person_ethnicity_types (parent_id resolved by code);
//   - the transitive closure (person_ethnicity_type_closure) is rebuilt in SQL;
//   - group-level language ties resolve by glottocode (unresolved keys are silently dropped — resilient),
//     and country ties resolve by ISO-3166 alpha-2;
//   - a re-import of the same source_version is an idempotent no-op; a new version updates.
//
// The catalog is plaintext reference data — this import never touches person_ethnicities (a person's
// encrypted, declared ethnicity).
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
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// synthetic codes these tests own (scoped cleanup — the shared test DB runs package binaries in parallel).
var (
	testEthCodes  = []string{"zzeth-sl", "zzeth-es", "zzeth-uk", "zzeth-ru"}
	testEthLangs  = []string{"zzukl001", "zzrul001"}
)

func newEthnicityImportService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc := application.NewService(pool, audit)
	svc.Register(domain.ObjectTypeLanguageScheme, application.LanguageSchemeHandler(
		func(conn pdb.DBTX) domain.LanguoidStore { return adapters.NewLanguoidRepo(conn) },
	))
	svc.Register(domain.ObjectTypeEthnicityScheme, application.EthnicitySchemeHandler(
		func(conn pdb.DBTX) domain.EthnicityStore { return adapters.NewEthnicityRepo(conn) },
	))
	return svc
}

func cleanupEth(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	ethIDs := `(SELECT id FROM oikumenea.person_ethnicity_types WHERE code = ANY($1))`
	for _, s := range []string{
		`DELETE FROM oikumenea.person_ethnicity_type_languages WHERE ethnicity_type_id IN ` + ethIDs,
		`DELETE FROM oikumenea.person_ethnicity_type_countries WHERE ethnicity_type_id IN ` + ethIDs,
		`DELETE FROM oikumenea.person_ethnicity_type_closure WHERE ancestor_id IN ` + ethIDs + ` OR descendant_id IN ` + ethIDs,
	} {
		if _, err := pool.Exec(ctx, s, testEthCodes); err != nil {
			t.Fatalf("cleanup %q: %v", s, err)
		}
	}
	// person_ethnicity_types.parent_id is RESTRICT — delete children before parents (leaf codes first).
	for _, code := range []string{"zzeth-uk", "zzeth-ru", "zzeth-es", "zzeth-sl"} {
		if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.person_ethnicity_types WHERE code = $1`, code); err != nil {
			t.Fatalf("cleanup ethnicity %s: %v", code, err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.language_languoid_closure
		WHERE ancestor_id IN (SELECT id FROM oikumenea.language_languoids WHERE code = ANY($1))
		   OR descendant_id IN (SELECT id FROM oikumenea.language_languoids WHERE code = ANY($1))`, testEthLangs); err != nil {
		t.Fatalf("cleanup lang closure: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.language_languoids WHERE code = ANY($1)`, testEthLangs); err != nil {
		t.Fatalf("cleanup langs: %v", err)
	}
}

func TestEthnicitySchemeImportHierarchyAndLinks(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newEthnicityImportService(t, pool)
	cleanupEth(t, pool)
	t.Cleanup(func() { cleanupEth(t, pool) })

	// Seed two synthetic language-level languoids so the group→language ties resolve by glottocode.
	if _, err := svc.Import(ctx, domain.ObjectTypeLanguageScheme, application.Envelope{
		ObjectType: domain.ObjectTypeLanguageScheme, Source: "glottolog", SourceVersion: "eth-test",
		Records: []domain.Record{
			{"code": "zzukl001", "level": "language", "name": "Zz Ukr-ish"},
			{"code": "zzrul001", "level": "language", "name": "Zz Rus-ish"},
		},
	}); err != nil {
		t.Fatalf("seed languoids: %v", err)
	}

	// Parent-first ethnicity records: Slavs → East Slavs → Ukrainians / Russians. Ukrainians links a
	// resolvable language (zzukl001) + country UA; Russians links a resolvable language + a BOGUS one
	// (dropped) + country RU.
	records := []domain.Record{
		{"code": "zzeth-sl", "name": "Zz Slavs"},
		{"code": "zzeth-es", "name": "Zz East Slavs", "parent": "zzeth-sl"},
		{"code": "zzeth-uk", "name": "Zz Ukrainians", "parent": "zzeth-es", "wikidataId": "Q37884",
			"languages": []any{"zzukl001"}, "countries": []any{"UA"}},
		{"code": "zzeth-ru", "name": "Zz Russians", "parent": "zzeth-es",
			"languages": []any{"zzrul001", "nonexist-lang"}, "countries": []any{"RU"}},
	}
	env := func(ver string) application.Envelope {
		return application.Envelope{ObjectType: domain.ObjectTypeEthnicityScheme, Source: "wikidata", SourceVersion: ver, Records: records}
	}

	sum, err := svc.Import(ctx, domain.ObjectTypeEthnicityScheme, env("v1"))
	if err != nil {
		t.Fatalf("import ethnicity scheme: %v", err)
	}
	if sum.Created != 4 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("create summary = %+v, want Created=4", sum)
	}

	// Hierarchy: parent_id resolved (Ukrainians' parent is East Slavs).
	var parentCode string
	if err := pool.QueryRow(ctx, `SELECT p.code FROM oikumenea.person_ethnicity_types e
		JOIN oikumenea.person_ethnicity_types p ON p.id = e.parent_id WHERE e.code = 'zzeth-uk'`).Scan(&parentCode); err != nil {
		t.Fatalf("parent lookup: %v", err)
	}
	if parentCode != "zzeth-es" {
		t.Fatalf("Ukrainians parent = %q, want zzeth-es", parentCode)
	}

	// Closure: Slavs reaches Ukrainians at depth 2.
	var depth int
	if err := pool.QueryRow(ctx, `SELECT c.depth FROM oikumenea.person_ethnicity_type_closure c
		JOIN oikumenea.person_ethnicity_types a ON a.id = c.ancestor_id
		JOIN oikumenea.person_ethnicity_types d ON d.id = c.descendant_id
		WHERE a.code = 'zzeth-sl' AND d.code = 'zzeth-uk'`).Scan(&depth); err != nil {
		t.Fatalf("closure depth: %v", err)
	}
	if depth != 2 {
		t.Fatalf("closure depth Slavs→Ukrainians = %d, want 2", depth)
	}

	// Language ties: Ukrainians → 1 (zzukl001); Russians → 1 (bogus dropped, resilient).
	assertCount := func(sql string, want int, what string) {
		var n int
		if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatalf("%s count: %v", what, err)
		}
		if n != want {
			t.Fatalf("%s = %d, want %d", what, n, want)
		}
	}
	assertCount(`SELECT count(*) FROM oikumenea.person_ethnicity_type_languages pel
		JOIN oikumenea.person_ethnicity_types e ON e.id = pel.ethnicity_type_id WHERE e.code = 'zzeth-uk'`, 1, "Ukrainians languages")
	assertCount(`SELECT count(*) FROM oikumenea.person_ethnicity_type_languages pel
		JOIN oikumenea.person_ethnicity_types e ON e.id = pel.ethnicity_type_id WHERE e.code = 'zzeth-ru'`, 1, "Russians languages (bogus dropped)")
	// Country tie: Ukrainians → UA.
	assertCount(`SELECT count(*) FROM oikumenea.person_ethnicity_type_countries pec
		JOIN oikumenea.person_ethnicity_types e ON e.id = pec.ethnicity_type_id
		JOIN oikumenea.geo_countries c ON c.id = pec.country_id
		WHERE e.code = 'zzeth-uk' AND c.code = 'UA'`, 1, "Ukrainians→UA")

	// Idempotent re-run: same version → all skipped, ties untouched.
	sum, err = svc.Import(ctx, domain.ObjectTypeEthnicityScheme, env("v1"))
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if sum.Skipped != 4 || sum.Created != 0 || sum.Updated != 0 {
		t.Fatalf("re-import summary = %+v, want Skipped=4", sum)
	}

	// New edition → all updated.
	sum, err = svc.Import(ctx, domain.ObjectTypeEthnicityScheme, env("v2"))
	if err != nil {
		t.Fatalf("re-import v2: %v", err)
	}
	if sum.Updated != 4 || sum.Created != 0 {
		t.Fatalf("v2 summary = %+v, want Updated=4", sum)
	}
}
