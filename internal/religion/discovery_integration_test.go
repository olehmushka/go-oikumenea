//go:build integration

// Integration tests for the Religion discovery surface against a real Postgres (M25 exit criteria,
// D-Religion discovery surface / D-Location / D-Audit). They exercise attaching a primary site (over the
// shared location_locations) + a weekly main-service schedule, the closure-aware PostGIS discovery
// search (radius + service language/day filters) with app-side public_precision coarsening, a
// transliteration alias matching a fuzzy query, the one-primary-per-unit invariant, and the
// online-requires-meeting-url guard.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/religion/...
package religion_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
)

func seedLocation(t *testing.T, pool *pgxpool.Pool, lat, lng float64) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO oikumenea.location_locations (geom, country_id)
		VALUES (ST_SetSRID(ST_MakePoint($1,$2),4326)::geography,
		        (SELECT id FROM oikumenea.geo_countries WHERE code='UA'))
		RETURNING id`, lng, lat).Scan(&id); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	return id
}

func discoverTypeID(t *testing.T, pool *pgxpool.Pool, table, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea."+table+" WHERE code=$1 AND deleted_at IS NULL ORDER BY id LIMIT 1", code).Scan(&id); err != nil {
		t.Fatalf("resolve %s %s: %v", table, code, err)
	}
	return id
}

func fptr(f float64) *float64 { return &f }
func iptr(i int) *int         { return &i }

// TestDiscoverySearchAndCoarsening proves the M25 exit slice: attach a primary site + a Sunday main
// service in Ukrainian, then a radius+language+day search returns it; the coordinate is exact at
// `exact` precision, rounded at `city`, and omitted at `hidden`; a transliteration alias matches.
func TestDiscoverySearchAndCoarsening(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	unit := seedUnit(t, pool)
	// classify the community under Christianity so the religion-taxon filter (closure) matches.
	christianity := taxonID(t, pool, "christianity")
	if _, err := svc.AddOrgClassification(ctx, unit, christianity, true, nil, nil); err != nil {
		t.Fatalf("classify unit: %v", err)
	}

	// a place in central Kyiv.
	lat, lng := 50.4501, 30.5234
	loc := seedLocation(t, pool, lat, lng)
	church := discoverTypeID(t, pool, "religion_site_types", "church")
	mainService := discoverTypeID(t, pool, "religion_service_types", "main")

	site, err := svc.AddSite(ctx, domain.SiteInput{OrgUnitID: unit, LocationID: loc, SiteTypeID: church, IsPrimary: true})
	if err != nil {
		t.Fatalf("add site: %v", err)
	}
	if !site.IsPrimary || site.PublicPrecision != "exact" || site.SiteTypeCode != "church" {
		t.Fatalf("unexpected site %+v", site)
	}

	// a Sunday (0) main service in Ukrainian.
	if _, err := svc.AddSchedule(ctx, domain.ScheduleInput{
		SiteID: site.ID, ServiceTypeID: mainService, DayOfWeek: iptr(0),
		StartTime: "10:00", Timezone: "Europe/Kyiv", Language: "ukr",
	}); err != nil {
		t.Fatalf("add schedule: %v", err)
	}

	// an online service WITHOUT a meeting URL is rejected.
	if _, err := svc.AddSchedule(ctx, domain.ScheduleInput{
		SiteID: site.ID, ServiceTypeID: mainService, DayOfWeek: iptr(3), Timezone: "Europe/Kyiv", Mode: "online",
	}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected ErrInvalid for online without meeting URL, got %v", err)
	}

	// a transliteration alias.
	if _, err := svc.AddAlias(ctx, domain.AliasInput{UnitID: unit, AliasText: "Svyato-Uspenska", AliasType: "transliteration"}); err != nil {
		t.Fatalf("add alias: %v", err)
	}

	// search: within 5 km of Kyiv, Christianity, a Ukrainian service on Sunday → returns the site, exact coords.
	q := domain.DiscoveryQuery{
		Lat: fptr(50.45), Lng: fptr(30.52), RadiusM: fptr(5000),
		Religion: christianity, Language: "ukr", DayOfWeek: iptr(0),
	}
	hits, err := svc.SearchSites(ctx, q)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !containsSite(hits, site.ID) {
		t.Fatalf("expected the site in radius+language+day search, got %d hits", len(hits))
	}
	hit := findSite(hits, site.ID)
	if hit.Latitude == nil || math.Abs(*hit.Latitude-lat) > 1e-9 {
		t.Fatalf("exact precision should return the full latitude, got %v", hit.Latitude)
	}

	// a different day → no match (the language+day filter hits the SAME schedule row).
	q2 := q
	q2.DayOfWeek = iptr(2)
	if hits2, err := svc.SearchSites(ctx, q2); err != nil || containsSite(hits2, site.ID) {
		t.Fatalf("expected no Tuesday match (err=%v, n=%d)", err, len(hits2))
	}

	// coarsen to `city` (2 dp) → coordinate rounded.
	city := "city"
	if _, err := svc.UpdateSite(ctx, site.ID, domain.SiteUpdate{PublicPrecision: &city}); err != nil {
		t.Fatalf("set city precision: %v", err)
	}
	hits, err = svc.SearchSites(ctx, q)
	if err != nil {
		t.Fatalf("search after city: %v", err)
	}
	hit = findSite(hits, site.ID)
	if hit.Latitude == nil || math.Abs(*hit.Latitude-50.45) > 1e-9 || math.Abs(*hit.Longitude-30.52) > 1e-9 {
		t.Fatalf("city precision should round to 2dp, got lat=%v lng=%v", hit.Latitude, hit.Longitude)
	}

	// coarsen to `hidden` → coordinate omitted.
	hidden := "hidden"
	if _, err := svc.UpdateSite(ctx, site.ID, domain.SiteUpdate{PublicPrecision: &hidden}); err != nil {
		t.Fatalf("set hidden precision: %v", err)
	}
	hits, err = svc.SearchSites(ctx, q)
	if err != nil {
		t.Fatalf("search after hidden: %v", err)
	}
	hit = findSite(hits, site.ID)
	if hit.Latitude != nil || hit.Longitude != nil {
		t.Fatalf("hidden precision should omit the coordinate, got lat=%v lng=%v", hit.Latitude, hit.Longitude)
	}

	// a transliteration alias matches a fuzzy query.
	byAlias, err := svc.SearchSites(ctx, domain.DiscoveryQuery{Query: "uspensk"})
	if err != nil {
		t.Fatalf("alias search: %v", err)
	}
	if !containsSite(byAlias, site.ID) {
		t.Fatalf("expected the site via its transliteration alias")
	}
}

// TestOnePrimarySite proves promoting a second site to primary clears the first (the invariant the
// partial-unique index backstops).
func TestOnePrimarySite(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	unit := seedUnit(t, pool)
	church := discoverTypeID(t, pool, "religion_site_types", "church")
	loc1 := seedLocation(t, pool, 50.45, 30.52)
	loc2 := seedLocation(t, pool, 49.84, 24.03)

	first, err := svc.AddSite(ctx, domain.SiteInput{OrgUnitID: unit, LocationID: loc1, SiteTypeID: church, IsPrimary: true})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	if _, err := svc.AddSite(ctx, domain.SiteInput{OrgUnitID: unit, LocationID: loc2, SiteTypeID: church, IsPrimary: true}); err != nil {
		t.Fatalf("add second primary: %v", err)
	}

	sites, err := svc.ListUnitSites(ctx, unit)
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	primaries := 0
	for _, s := range sites {
		if s.IsPrimary {
			primaries++
		}
		if s.ID == first.ID && s.IsPrimary {
			t.Fatalf("the first site should no longer be primary")
		}
	}
	if primaries != 1 {
		t.Fatalf("expected exactly one primary site, got %d", primaries)
	}
}

func containsSite(hits []domain.DiscoverySite, id string) bool {
	for _, h := range hits {
		if h.ID == id {
			return true
		}
	}
	return false
}

func findSite(hits []domain.DiscoverySite, id string) domain.DiscoverySite {
	for _, h := range hits {
		if h.ID == id {
			return h
		}
	}
	return domain.DiscoverySite{}
}
