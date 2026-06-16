//go:build integration

// Integration tests for the Location vertical against a real PostGIS + h3-pg Postgres (M19 exit
// criteria, D-Location / D-Audit). They exercise the geo module's audited Location CRUD + spatial
// queries + the DB-derived MGRS/H3 columns:
//
//   - create from a coordinate -> MGRS + all four H3 cells are derived (non-null) on write;
//   - update the coordinate -> derived columns recompute;
//   - out-of-range coordinate is rejected (ErrCoordinateOutOfRange);
//   - ListLocationsNear returns rows within radius and excludes those outside (ST_DWithin);
//   - soft-delete removes the row from reads (ErrLocationNotFound afterwards);
//   - each write emits exactly one `system`-actor audited Action in the same transaction;
//   - the location_mgrs() function matches known MGRS fixtures (incl. a southern-hemisphere point).
//
// Run against a throwaway DB that has the migrations applied (PostGIS + h3-pg required — the custom
// Dockerfile.postgres image):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/geo/...
package geo_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/geo/adapters"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
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
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit)
}

// countryID resolves a seeded country's RID (the registry carries the ISO-3166 set) for the FK.
func countryID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.geo_countries WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("resolve country %s: %v", code, err)
	}
	return id
}

func ptr(s string) *string { return &s }

// kyiv / sydney coordinates with their expected MGRS zone+band prefixes.
const (
	kyivLat, kyivLng     = 50.4501, 30.5234 // -> 36U...
	sydneyLat, sydneyLng = -33.8688, 151.2093
)

func TestLocationCreateDerivesMGRSAndH3(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	loc, err := svc.CreateLocation(ctx, domain.LocationWrite{
		Latitude: kyivLat, Longitude: kyivLng, CountryID: ua,
		Locality: ptr("Kyiv"), RawAddress: ptr("Maidan Nezalezhnosti"),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if loc.MGRS == nil || !strings.HasPrefix(*loc.MGRS, "36U") {
		t.Fatalf("expected MGRS to start 36U, got %v", loc.MGRS)
	}
	if len(*loc.MGRS) != 15 { // 2-digit zone + band + 2 square letters + 5+5 digits
		t.Fatalf("expected 15-char MGRS, got %q", *loc.MGRS)
	}
	for name, cell := range map[string]*string{"r5": loc.H3Res5, "r7": loc.H3Res7, "r9": loc.H3Res9, "r11": loc.H3Res11} {
		if cell == nil || *cell == "" {
			t.Fatalf("expected H3 %s cell derived, got %v", name, cell)
		}
	}

	// exactly one system-actor Action recorded for the create.
	assertOneAction(t, pool, loc.ID, "location.create")
}

func TestLocationUpdateRecomputesDerived(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	loc, err := svc.CreateLocation(ctx, domain.LocationWrite{Latitude: kyivLat, Longitude: kyivLng, CountryID: ua})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	moved, err := svc.UpdateLocation(ctx, loc.ID, domain.LocationWrite{Latitude: sydneyLat, Longitude: sydneyLng, CountryID: ua})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if moved.MGRS == nil || !strings.HasPrefix(*moved.MGRS, "56H") {
		t.Fatalf("expected moved MGRS to start 56H (Sydney), got %v", moved.MGRS)
	}
	if loc.MGRS != nil && moved.MGRS != nil && *loc.MGRS == *moved.MGRS {
		t.Fatalf("MGRS should change after moving the coordinate")
	}
	assertOneAction(t, pool, loc.ID, "location.update")
}

func TestLocationCoordinateOutOfRangeRejected(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ua := countryID(t, pool, "UA")
	if _, err := svc.CreateLocation(context.Background(), domain.LocationWrite{Latitude: 200, Longitude: 0, CountryID: ua}); err != domain.ErrCoordinateOutOfRange {
		t.Fatalf("expected ErrCoordinateOutOfRange, got %v", err)
	}
}

func TestLocationRadiusQuery(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	near, err := svc.CreateLocation(ctx, domain.LocationWrite{Latitude: kyivLat + 0.001, Longitude: kyivLng + 0.001, CountryID: ua})
	if err != nil {
		t.Fatalf("create near: %v", err)
	}
	far, err := svc.CreateLocation(ctx, domain.LocationWrite{Latitude: kyivLat + 1.0, Longitude: kyivLng + 1.0, CountryID: ua})
	if err != nil {
		t.Fatalf("create far: %v", err)
	}

	rows, _, err := svc.ListLocationsNear(ctx, kyivLat, kyivLng, 5000, 100, 0)
	if err != nil {
		t.Fatalf("near: %v", err)
	}
	if !containsID(rows, near.ID) {
		t.Fatalf("expected the ~150m location within 5km radius")
	}
	if containsID(rows, far.ID) {
		t.Fatalf("did not expect the ~150km location within 5km radius")
	}
}

func TestLocationSoftDelete(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	loc, err := svc.CreateLocation(ctx, domain.LocationWrite{Latitude: kyivLat, Longitude: kyivLng, CountryID: ua})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.DeleteLocation(ctx, loc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetLocation(ctx, loc.ID); err != domain.ErrLocationNotFound {
		t.Fatalf("expected ErrLocationNotFound after delete, got %v", err)
	}
	if err := svc.DeleteLocation(ctx, loc.ID); err != domain.ErrLocationNotFound {
		t.Fatalf("expected ErrLocationNotFound deleting twice, got %v", err)
	}
}

func TestLocationTypesSeeded(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	types, err := svc.ListLocationTypes(context.Background())
	if err != nil {
		t.Fatalf("list types: %v", err)
	}
	want := map[string]bool{"building": false, "address": false, "online": false}
	for _, ty := range types {
		if _, ok := want[ty.Code]; ok {
			want[ty.Code] = true
		}
	}
	for code, seen := range want {
		if !seen {
			t.Fatalf("expected seeded location type %q", code)
		}
	}
}

// TestLocationMGRSFixtures validates the DB MGRS function directly against known references, incl. a
// southern-hemisphere point and a UTM-zone boundary.
func TestLocationMGRSFixtures(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	cases := []struct {
		name         string
		lat, lng     float64
		wantZoneBand string
	}{
		{"kyiv", kyivLat, kyivLng, "36U"},
		{"sydney", sydneyLat, sydneyLng, "56H"},
		{"london", 51.5074, -0.1278, "30U"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var mgrs string
			if err := pool.QueryRow(ctx,
				"SELECT oikumenea.location_mgrs(ST_SetSRID(ST_MakePoint($1,$2),4326)::geography)",
				c.lng, c.lat).Scan(&mgrs); err != nil {
				t.Fatalf("mgrs: %v", err)
			}
			if !strings.HasPrefix(mgrs, c.wantZoneBand) {
				t.Fatalf("%s: expected MGRS prefix %s, got %s", c.name, c.wantZoneBand, mgrs)
			}
			if len(mgrs) != 15 {
				t.Fatalf("%s: expected 15-char MGRS, got %q", c.name, mgrs)
			}
		})
	}
}

func containsID(rows []domain.Location, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// assertOneAction asserts exactly one system-actor audit row exists for the target with the given
// action (the write + its audit row committed together — D-Audit).
func assertOneAction(t *testing.T, pool *pgxpool.Pool, targetID, action string) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = $2 AND actor_type = 'system'",
		targetID, action).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 %s system action for %s, got %d", action, targetID, n)
	}
}
