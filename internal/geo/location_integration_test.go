//go:build integration

// Integration tests for the Location vertical against a real PostGIS Postgres (M19 exit criteria,
// D-Location / D-Audit). They exercise the geo module's audited Location CRUD + spatial queries with
// app-derived MGRS and the multi-format coordinate input:
//
//   - create from a coordinate (any format) -> the app resolves WGS84 + derives the MGRS on write;
//   - the original input is preserved verbatim in source_coordinate;
//   - update the coordinate -> MGRS recomputes;
//   - out-of-range coordinate is rejected (ErrCoordinateOutOfRange), unparseable -> ErrCoordinateInvalid;
//   - ListLocationsNear returns rows within radius and excludes those outside (ST_DWithin);
//   - soft-delete removes the row from reads (ErrLocationNotFound afterwards);
//   - each write emits exactly one `system`-actor audited Action in the same transaction.
//
// Run against a throwaway DB that has the migrations applied (PostGIS required — the stock postgis image):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/geo/...
package geo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

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

func ptr(s string) *string   { return &s }
func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

// latLonInput builds a WGS84 lat/lon LocationInput for a country.
func latLonInput(lat, lng float64, country string) domain.LocationInput {
	return domain.LocationInput{
		Coordinate: domain.CoordinateInput{Format: domain.FormatLatLon, Latitude: f64(lat), Longitude: f64(lng)},
		CountryID:  country,
	}
}

const (
	kyivLat, kyivLng     = 50.4501, 30.5234 // -> 36U...
	sydneyLat, sydneyLng = -33.8688, 151.2093
)

func TestLocationCreateDerivesMGRS(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	in := latLonInput(kyivLat, kyivLng, ua)
	in.Locality = ptr("Kyiv")
	in.RawAddress = ptr("Maidan Nezalezhnosti")
	loc, err := svc.CreateLocation(ctx, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if loc.MGRS == nil || !strings.HasPrefix(*loc.MGRS, "36U") {
		t.Fatalf("expected MGRS to start 36U, got %v", loc.MGRS)
	}
	if len(*loc.MGRS) != 15 { // 2-digit zone + band + 2 square letters + 5+5 digits
		t.Fatalf("expected 15-char MGRS, got %q", *loc.MGRS)
	}
	// the original input is preserved verbatim.
	if got := sourceFormat(t, loc.SourceCoordinate); got != domain.FormatLatLon {
		t.Fatalf("expected source format %q, got %q", domain.FormatLatLon, got)
	}
	assertOneAction(t, pool, loc.ID, "location.create")
}

func TestLocationUpdateRecomputesMGRS(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	loc, err := svc.CreateLocation(ctx, latLonInput(kyivLat, kyivLng, ua))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	moved, err := svc.UpdateLocation(ctx, loc.ID, latLonInput(sydneyLat, sydneyLng, ua))
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

// TestLocationCreateFromFormats: a coordinate supplied as MGRS or UTM resolves to the same canonical
// WGS84 point (within MGRS precision) and preserves its source format.
func TestLocationCreateFromFormats(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	mgrs := domain.DeriveMGRS(kyivLat, kyivLng)
	if mgrs == nil {
		t.Fatal("nil MGRS for Kyiv")
	}
	cases := []struct {
		name       string
		coord      domain.CoordinateInput
		wantFormat string
	}{
		{"mgrs", domain.CoordinateInput{Format: domain.FormatMGRS, MGRS: mgrs}, domain.FormatMGRS},
		{"utm", domain.CoordinateInput{Format: domain.FormatUTM, Zone: iptr(36), Hemisphere: ptr("N"), Easting: f64(324182), Northing: f64(5591607)}, domain.FormatUTM},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc, err := svc.CreateLocation(ctx, domain.LocationInput{Coordinate: c.coord, CountryID: ua})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if math.Abs(loc.Latitude-kyivLat) > 0.01 || math.Abs(loc.Longitude-kyivLng) > 0.01 {
				t.Fatalf("expected ~Kyiv, got (%v,%v)", loc.Latitude, loc.Longitude)
			}
			if loc.MGRS == nil || !strings.HasPrefix(*loc.MGRS, "36U") {
				t.Fatalf("expected MGRS 36U, got %v", loc.MGRS)
			}
			if got := sourceFormat(t, loc.SourceCoordinate); got != c.wantFormat {
				t.Fatalf("expected source format %q, got %q", c.wantFormat, got)
			}
		})
	}
}

func TestLocationCoordinateOutOfRangeRejected(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ua := countryID(t, pool, "UA")
	if _, err := svc.CreateLocation(context.Background(), latLonInput(200, 0, ua)); err != domain.ErrCoordinateOutOfRange {
		t.Fatalf("expected ErrCoordinateOutOfRange, got %v", err)
	}
}

func TestLocationCoordinateInvalidRejected(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ua := countryID(t, pool, "UA")
	bad := domain.LocationInput{Coordinate: domain.CoordinateInput{Format: domain.FormatMGRS, MGRS: ptr("not-an-mgrs")}, CountryID: ua}
	if _, err := svc.CreateLocation(context.Background(), bad); err != domain.ErrCoordinateInvalid {
		t.Fatalf("expected ErrCoordinateInvalid, got %v", err)
	}
}

func TestLocationRadiusQuery(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	near, err := svc.CreateLocation(ctx, latLonInput(kyivLat+0.001, kyivLng+0.001, ua))
	if err != nil {
		t.Fatalf("create near: %v", err)
	}
	far, err := svc.CreateLocation(ctx, latLonInput(kyivLat+1.0, kyivLng+1.0, ua))
	if err != nil {
		t.Fatalf("create far: %v", err)
	}

	rows, _, err := svc.ListLocationsNear(ctx, kyivLat, kyivLng, 5000, 0, "", 100)
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

// TestLocationNearKeysetPagination walks the nearest-first radius search one row per page via the
// (distance, id) keyset cursor (review R-21: OFFSET is gone). It asserts every location inside the
// radius is returned exactly once, in non-decreasing distance order, and pages fill correctly.
func TestLocationNearKeysetPagination(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	const n = 5
	created := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		// Distinct, increasing offsets so each sits at a different distance from the query point, all
		// comfortably inside 50km.
		loc, err := svc.CreateLocation(ctx, latLonInput(kyivLat+float64(i+1)*0.01, kyivLng, ua))
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		created[loc.ID] = true
	}

	// The shared test DB holds locations from other tests (many at tied distances), so page all the way
	// to exhaustion — a safety cap guards against a keyset that fails to advance (which would loop).
	seen := make(map[string]bool, n)
	var afterDist float64
	var afterID string
	lastDist := -1.0
	for pages := 0; pages < 10000; pages++ {
		page, more, err := svc.ListLocationsNear(ctx, kyivLat, kyivLng, 50000, afterDist, afterID, 1)
		if err != nil {
			t.Fatalf("near page: %v", err)
		}
		for _, r := range page {
			if seen[r.ID] {
				t.Fatalf("location %s returned twice across keyset pages", r.ID)
			}
			seen[r.ID] = true
			if r.DistanceM < lastDist {
				t.Fatalf("distance order regressed: %f after %f", r.DistanceM, lastDist)
			}
			lastDist = r.DistanceM
			afterDist, afterID = r.DistanceM, r.ID
		}
		if !more {
			break
		}
	}
	for id := range created {
		if !seen[id] {
			t.Fatalf("keyset paging skipped location %s", id)
		}
	}
}

// TestLocationSearchKeysetPagination proves the trigram text search (review R-21: search_text GIN,
// keyset on id, no OFFSET) is findable and pages fill correctly without duplicates.
func TestLocationSearchKeysetPagination(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	// A per-run unique marker isolates this test's rows from prior runs' (the DB persists across the
	// package run), so the search result set is exactly the n created below.
	marker := fmt.Sprintf("zzqxmarker%d", time.Now().UnixNano())
	const n = 4
	created := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		in := latLonInput(kyivLat+float64(i+1)*0.01, kyivLng, ua)
		locality := marker
		in.Locality = &locality
		loc, err := svc.CreateLocation(ctx, in)
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		created[loc.ID] = true
	}

	seen := make(map[string]bool, n)
	after := ""
	for pages := 0; pages < 10000; pages++ {
		page, more, err := svc.SearchLocations(ctx, marker, after, 2)
		if err != nil {
			t.Fatalf("search page: %v", err)
		}
		for _, r := range page {
			if seen[r.ID] {
				t.Fatalf("location %s returned twice across search pages", r.ID)
			}
			seen[r.ID] = true
			after = r.ID
		}
		if !more {
			break
		}
	}
	for id := range created {
		if !seen[id] {
			t.Fatalf("search keyset paging skipped location %s", id)
		}
	}
}

func TestLocationSoftDelete(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()
	ua := countryID(t, pool, "UA")

	loc, err := svc.CreateLocation(ctx, latLonInput(kyivLat, kyivLng, ua))
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

func sourceFormat(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var c domain.CoordinateInput
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("unmarshal source_coordinate: %v", err)
	}
	return c.Format
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
