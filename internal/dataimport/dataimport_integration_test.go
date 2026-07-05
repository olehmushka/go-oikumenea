//go:build integration

// Integration tests for the data-import module against a real PostGIS Postgres (M16 exit criteria,
// D-Hermenea / D-GeoPlaces / D-Audit). They exercise the oikumenea SIDE of the ingestion pipeline —
// the generic POST /import/{objectType} upsert the hermenea companion calls:
//
//   - geo-countries: import creates, a re-import of unchanged data is a no-op (Skipped), a changed
//     name updates (Updated); per-row provenance (source/source_version/imported_at) is stamped; every
//     import records exactly one `system`-actor audited Action.
//   - geo-places: a parent-first country -> region -> locality set inserts; a placetype=country record
//     enriches the matching geo_countries row (wof_id + geometry) in the SAME transaction; re-import of
//     the same source_version skips, a newer edition updates; a forward parent reference (RESTRICT)
//     fails the whole transaction (nothing committed).
//
// Run against a throwaway DB that has the migrations applied (PostGIS required):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/dataimport/...
package dataimport_test

import (
	"context"
	"os"
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

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc := application.NewService(pool, audit)
	svc.Register(domain.ObjectTypeGeoCountries, application.GeoCountriesHandler(
		func(conn pdb.DBTX) domain.GeoCountryStore { return adapters.NewGeoCountryRepo(conn) },
	))
	svc.Register(domain.ObjectTypeGeoPlaces, application.GeoPlacesHandler(
		func(conn pdb.DBTX) domain.GeoPlaceStore { return adapters.NewGeoPlaceRepo(conn) },
	))
	return svc
}

// testCountry is a non-ISO code never present in the seeded geo_countries registry, so the test owns
// its lifecycle (create -> update -> skip) without touching real seed data.
const testCountry = "ZZ"

func cleanupGeo(t *testing.T, pool *pgxpool.Pool, wofIDs ...int64) {
	t.Helper()
	ctx := context.Background()
	// Children first (parent_id is RESTRICT), then the synthetic country row.
	for i := len(wofIDs) - 1; i >= 0; i-- {
		if _, err := pool.Exec(ctx, "DELETE FROM oikumenea.geo_places WHERE wof_id = $1", wofIDs[i]); err != nil {
			t.Fatalf("cleanup geo_places %d: %v", wofIDs[i], err)
		}
	}
	if _, err := pool.Exec(ctx, "DELETE FROM oikumenea.geo_countries WHERE code = $1", testCountry); err != nil {
		t.Fatalf("cleanup geo_countries: %v", err)
	}
}

func TestGeoCountriesImportIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	cleanupGeo(t, pool)
	t.Cleanup(func() { cleanupGeo(t, pool) })

	auditBefore := countAudit(t, pool, "import.geo-countries")

	env := func(name, ver string) application.Envelope {
		return application.Envelope{
			ObjectType:    domain.ObjectTypeGeoCountries,
			Source:        "iso-3166",
			SourceVersion: ver,
			Records:       []domain.Record{{"code": testCountry, "name": name}},
		}
	}

	// create
	sum, err := svc.Import(ctx, domain.ObjectTypeGeoCountries, env("Testland", "v1"))
	if err != nil {
		t.Fatalf("import create: %v", err)
	}
	if sum.Created != 1 || sum.Updated != 0 || sum.Skipped != 0 {
		t.Fatalf("create summary = %+v, want Created=1", sum)
	}
	assertCountryProvenance(t, pool, "iso-3166", "v1")

	// re-import unchanged -> skip (idempotent no-op)
	sum, err = svc.Import(ctx, domain.ObjectTypeGeoCountries, env("Testland", "v1"))
	if err != nil {
		t.Fatalf("import skip: %v", err)
	}
	if sum.Skipped != 1 || sum.Created != 0 || sum.Updated != 0 {
		t.Fatalf("skip summary = %+v, want Skipped=1", sum)
	}

	// changed name -> update + refreshed provenance
	sum, err = svc.Import(ctx, domain.ObjectTypeGeoCountries, env("Testlandia", "v2"))
	if err != nil {
		t.Fatalf("import update: %v", err)
	}
	if sum.Updated != 1 {
		t.Fatalf("update summary = %+v, want Updated=1", sum)
	}
	assertCountryProvenance(t, pool, "iso-3166", "v2")

	// one system-actor audit Action per import (3 imports above)
	if got := countAudit(t, pool, "import.geo-countries") - auditBefore; got != 3 {
		t.Fatalf("audit rows added = %d, want 3", got)
	}
}

func TestGeoPlacesImportParentFirstAndEnrich(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)

	const (
		countryWOF  = 900001
		regionWOF   = 900002
		localityWOF = 900003
		danglingWOF = 900004
	)
	cleanupGeo(t, pool, countryWOF, regionWOF, localityWOF, danglingWOF)
	t.Cleanup(func() { cleanupGeo(t, pool, countryWOF, regionWOF, localityWOF, danglingWOF) })

	// A synthetic geo_countries row for the country place to enrich (EnrichCountry is an UPDATE by code).
	if _, err := pool.Exec(ctx, "INSERT INTO oikumenea.geo_countries (code, name) VALUES ($1, 'Testland')", testCountry); err != nil {
		t.Fatalf("seed country: %v", err)
	}

	point := func(lon, lat float64) map[string]any {
		return map[string]any{"type": "Point", "coordinates": []any{lon, lat}}
	}
	records := []domain.Record{
		{"wofId": float64(countryWOF), "placetype": "country", "name": "Testland", "countryCode": testCountry, "isoA3": "ZZZ", "numericCode": "999", "geometry": point(0, 0)},
		{"wofId": float64(regionWOF), "placetype": "region", "name": "Test Region", "countryCode": testCountry, "parentId": float64(countryWOF), "geometry": point(1, 1)},
		{"wofId": float64(localityWOF), "placetype": "locality", "name": "Test City", "countryCode": testCountry, "parentId": float64(regionWOF), "geometry": point(2, 2)},
	}

	sum, err := svc.Import(ctx, domain.ObjectTypeGeoPlaces, application.Envelope{
		ObjectType: domain.ObjectTypeGeoPlaces, Source: "wof", SourceVersion: "v1", Records: records,
	})
	if err != nil {
		t.Fatalf("import places: %v", err)
	}
	if sum.Created != 3 {
		t.Fatalf("places summary = %+v, want Created=3", sum)
	}

	// country place enriched the geo_countries row (wof_id + geometry materialized) in the same tx
	var wofID int64
	var hasGeom bool
	if err := pool.QueryRow(ctx, "SELECT wof_id, geom IS NOT NULL FROM oikumenea.geo_countries WHERE code=$1", testCountry).Scan(&wofID, &hasGeom); err != nil {
		t.Fatalf("read enriched country: %v", err)
	}
	if wofID != countryWOF || !hasGeom {
		t.Fatalf("country not enriched: wof_id=%d hasGeom=%v", wofID, hasGeom)
	}

	// per-row provenance stamped, geometry materialized
	var src, ver string
	if err := pool.QueryRow(ctx, "SELECT source, source_version FROM oikumenea.geo_places WHERE wof_id=$1", localityWOF).Scan(&src, &ver); err != nil {
		t.Fatalf("read place provenance: %v", err)
	}
	if src != "wof" || ver != "v1" {
		t.Fatalf("place provenance = %s/%s, want wof/v1", src, ver)
	}

	// re-import same edition -> all skipped (idempotent)
	sum, err = svc.Import(ctx, domain.ObjectTypeGeoPlaces, application.Envelope{
		ObjectType: domain.ObjectTypeGeoPlaces, Source: "wof", SourceVersion: "v1", Records: records,
	})
	if err != nil {
		t.Fatalf("re-import places: %v", err)
	}
	if sum.Skipped != 3 {
		t.Fatalf("re-import summary = %+v, want Skipped=3", sum)
	}

	// newer edition -> all updated
	sum, err = svc.Import(ctx, domain.ObjectTypeGeoPlaces, application.Envelope{
		ObjectType: domain.ObjectTypeGeoPlaces, Source: "wof", SourceVersion: "v2", Records: records,
	})
	if err != nil {
		t.Fatalf("update places: %v", err)
	}
	if sum.Updated != 3 {
		t.Fatalf("update summary = %+v, want Updated=3", sum)
	}

	// forward parent reference (RESTRICT) fails the whole transaction -> nothing committed
	_, err = svc.Import(ctx, domain.ObjectTypeGeoPlaces, application.Envelope{
		ObjectType: domain.ObjectTypeGeoPlaces, Source: "wof", SourceVersion: "v3",
		Records: []domain.Record{
			{"wofId": float64(danglingWOF), "placetype": "locality", "name": "Orphan", "countryCode": testCountry, "parentId": float64(999999), "geometry": point(3, 3)},
		},
	})
	if err == nil {
		t.Fatal("expected forward-reference import to fail")
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM oikumenea.geo_places WHERE wof_id=$1", danglingWOF).Scan(&n); err != nil {
		t.Fatalf("count dangling: %v", err)
	}
	if n != 0 {
		t.Fatalf("dangling row committed despite RESTRICT failure (n=%d)", n)
	}
}

func assertCountryProvenance(t *testing.T, pool *pgxpool.Pool, source, version string) {
	t.Helper()
	var src, ver string
	var importedAtSet bool
	err := pool.QueryRow(context.Background(),
		"SELECT source, source_version, imported_at IS NOT NULL FROM oikumenea.geo_countries WHERE code=$1",
		testCountry).Scan(&src, &ver, &importedAtSet)
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	if src != source || ver != version || !importedAtSet {
		t.Fatalf("provenance = %s/%s/imported=%v, want %s/%s/true", src, ver, importedAtSet, source, version)
	}
}

func countAudit(t *testing.T, pool *pgxpool.Pool, action string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM oikumenea.audit_log WHERE action=$1 AND actor_type='system'", action).Scan(&n)
	if err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return n
}

