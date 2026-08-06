// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the location DASHBOARD aggregate (M58 ticket 6 / D-ObjectFacets) against a
// real Postgres.
//
// The differential is the vocabulary's usual one — totalCount equals the rows an exhaustive paging of
// the same list returns under the same arguments, and every bucket's count equals what its own filter
// returns. What location adds is the WINDOW: this is the first type whose aggregate carries a
// continuous predicate, so the differential has to be run in all four modes rather than one. A radius
// that narrowed the list and not the chart would leave both numbers looking reasonable on their own,
// which is exactly the failure the ticket set out to make impossible.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/geo/... -run LocationStats
package geo_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/geo/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// The spread's anchor point and text tag. The coordinates are arbitrary but FIXED, because the tests
// compare a tight radius around them against a wide one; the tag is what scopes every assertion to
// this test's own rows (the test database is persistent, and an unfiltered assertion would race every
// other suite that creates a location).
const (
	spreadLat = 41.5
	spreadLng = 12.5
	spreadTag = "m58t6spread"
)

// seedLocationSpread creates a population under ONE country — three points on top of the anchor and
// three ~150 km away, over two place types and one unclassified — so that both facets have more than
// one bucket AND a tight radius genuinely excludes rows. A single-bucket distribution makes a
// differential pass without testing anything.
func seedLocationSpread(t *testing.T, pool *pgxpool.Pool, svc *application.Service) (countryID2, typeID string) {
	t.Helper()
	ctx := context.Background()
	countryID2 = countryID(t, pool, "IT") // unused by the other geo suites
	if _, err := pool.Exec(ctx,
		`DELETE FROM oikumenea.location_locations WHERE country_id = $1`, countryID2); err != nil {
		t.Fatalf("clear prior spread: %v", err)
	}
	var types []string
	rows, err := pool.Query(ctx,
		`SELECT id FROM oikumenea.location_location_types WHERE deleted_at IS NULL ORDER BY code LIMIT 2`)
	if err != nil {
		t.Fatalf("read place types: %v", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan place type: %v", err)
		}
		types = append(types, id)
	}
	rows.Close()
	if len(types) < 2 {
		t.Fatal("fewer than two place types in the catalog — the type facet cannot be exercised")
	}
	typeID = types[0]

	// near/far: 1.5° of latitude is ~165 km, comfortably outside the 1 km probe and inside the 200 km
	// one being false for the far rows.
	spread := []struct {
		lat, lng float64
		typ      *string
	}{
		{spreadLat, spreadLng, &types[0]},
		{spreadLat + 0.001, spreadLng, &types[0]},
		{spreadLat, spreadLng + 0.001, &types[1]},
		{spreadLat + 1.5, spreadLng, &types[1]},
		{spreadLat + 1.5, spreadLng + 0.01, nil},
		{spreadLat + 1.5, spreadLng + 0.02, &types[0]},
	}
	for i, p := range spread {
		in := latLonInput(p.lat, p.lng, countryID2)
		in.Locality = ptr(spreadTag)
		in.RawAddress = ptr(spreadTag + "-" + string(rune('a'+i)))
		in.TypeID = p.typ
		if _, err := svc.CreateLocation(ctx, in); err != nil {
			t.Fatalf("seed location %d: %v", i, err)
		}
	}
	return countryID2, typeID
}

func allLocationFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("location")
	if !ok {
		t.Fatal("location is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

// pageAllLocations exhaustively pages one listing mode and returns the row count. Small pages on
// purpose: a bug that loses rows at a page boundary (the shape ticket 2 found in listTaxa, which had
// silently dropped 16 of 100 rows since M22) only shows up once the sweep turns over. It also asserts
// no id repeats, which is the other half of a broken keyset.
func pageAllLocations(t *testing.T, svc *application.Service, f domain.LocationFilter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	afterDist, afterID := 0.0, ""
	for i := 0; i < 500; i++ {
		rows, more, err := svc.ListLocations(ctx, f, afterDist, afterID, 3)
		if err != nil {
			t.Fatalf("list locations (%s): %v", f.Mode, err)
		}
		for _, l := range rows {
			if seen[l.ID] {
				t.Fatalf("location %s returned twice — the keyset is broken", l.ID)
			}
			seen[l.ID] = true
		}
		if !more || len(rows) == 0 {
			return len(seen)
		}
		last := rows[len(rows)-1]
		afterID = last.ID
		afterDist = last.DistanceM
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// TestLocationStatsTotalEqualsExhaustivePaging_Integration is D-ObjectFacets' headline promise, run
// once per MODE. Three of the four are the point: an unwindowed total that matched while a windowed
// one did not would mean the chart above a filtered list is describing the registry.
func TestLocationStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	countryID, typeID := seedLocationSpread(t, pool, svc)

	for _, tc := range []struct {
		name string
		f    domain.LocationFilter
	}{
		{"browse", domain.LocationFilter{Mode: domain.LocationModeBrowse, CountryID: &countryID}},
		{"typed browse", domain.LocationFilter{Mode: domain.LocationModeBrowse, CountryID: &countryID, TypeID: &typeID}},
		{"radius", domain.LocationFilter{
			Mode: domain.LocationModeRadius, CountryID: &countryID,
			Lat: spreadLat, Lng: spreadLng, RadiusM: 200_000,
		}},
		{"tight radius", domain.LocationFilter{
			Mode: domain.LocationModeRadius, CountryID: &countryID,
			Lat: spreadLat, Lng: spreadLng, RadiusM: 1_000,
		}},
		{"bbox", domain.LocationFilter{
			Mode: domain.LocationModeBbox, CountryID: &countryID,
			MinLat: spreadLat - 1, MinLng: spreadLng - 1, MaxLat: spreadLat + 1, MaxLng: spreadLng + 1,
		}},
		{"text", domain.LocationFilter{Mode: domain.LocationModeText, CountryID: &countryID, Query: spreadTag}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.LocationStats(ctx, tc.f, allLocationFacets(t))
			if err != nil {
				t.Fatalf("stats: %v", err)
			}
			if got, want := int(res.TotalCount), pageAllLocations(t, svc, tc.f); got != want {
				t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
			}
		})
	}
}

// TestLocationStatsWindowActuallyNarrows_Integration is the non-vacuity half of the test above. Every
// mode agreeing with its own paging is satisfiable by a window that does nothing at all — both sides
// would simply return the registry. This pins that the tight radius sees FEWER rows than the wide
// one, so the agreement above is agreement about a real predicate.
func TestLocationStatsWindowActuallyNarrows_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	countryID, _ := seedLocationSpread(t, pool, svc)

	wide := domain.LocationFilter{
		Mode: domain.LocationModeRadius, CountryID: &countryID,
		Lat: spreadLat, Lng: spreadLng, RadiusM: 200_000,
	}
	tight := wide
	tight.RadiusM = 1_000

	wideRes, err := svc.LocationStats(ctx, wide, allLocationFacets(t))
	if err != nil {
		t.Fatalf("wide stats: %v", err)
	}
	tightRes, err := svc.LocationStats(ctx, tight, allLocationFacets(t))
	if err != nil {
		t.Fatalf("tight stats: %v", err)
	}
	if tightRes.TotalCount >= wideRes.TotalCount {
		t.Errorf("a 1 km radius counted %d and a 200 km radius %d — the window is not narrowing the "+
			"aggregate, so every mode-vs-paging agreement above is agreement about nothing",
			tightRes.TotalCount, wideRes.TotalCount)
	}
	if tightRes.TotalCount == 0 {
		t.Error("the tight radius counted nothing — the spread's coordinates moved and this test is " +
			"now comparing two empty sets")
	}
}

// TestLocationStatsEveryBucketEqualsItsOwnFilter_Integration is the property the whole vocabulary
// rests on, and the one a chart cannot check for itself: clicking a segment must land on exactly the
// rows that segment counted. A wrong inverse fails silently — the operator gets a list that quietly
// disagrees with the bar they clicked, and neither number looks wrong alone.
//
// Run under a WINDOW rather than unwindowed, because that is the combination this ticket introduced:
// the bucket inverse has to compose with the window rather than replace it.
func TestLocationStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	countryID, _ := seedLocationSpread(t, pool, svc)

	base := domain.LocationFilter{
		Mode: domain.LocationModeRadius, CountryID: &countryID,
		Lat: spreadLat, Lng: spreadLng, RadiusM: 200_000,
	}
	res, err := svc.LocationStats(ctx, base, allLocationFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	checked := 0
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == "(unknown)" || b.Key == "(other)" || b.Count == 0 {
				continue // synthetic keys are deliberately not filter values
			}
			f := base
			switch d.Facet {
			case "countryId":
				k := b.Key
				f.CountryID = &k
			case "typeId":
				k := b.Key
				f.TypeID = &k
			default:
				t.Fatalf("facet %q has no filter inverse in this test — a new facet was declared and "+
					"its click-through is unchecked", d.Facet)
			}
			if got := pageAllLocations(t, svc, f); got != int(b.Count) {
				t.Errorf("%s bucket %q counted %d, its own filter returns %d",
					d.Facet, b.Key, b.Count, got)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no bucket was checked — the spread produced no distribution and this test is vacuous")
	}
}

// TestLocationStatsDistributionsSumToTotal_Integration: both location facets read the LISTED table,
// so each row has exactly one value in each and the buckets partition. Neither may take
// NonPartitioning — the kernel refuses that exemption when the facet's table IS the listed table —
// so a sum that missed would mean a lost or double-counted row rather than an overlap.
func TestLocationStatsDistributionsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	countryID, _ := seedLocationSpread(t, pool, svc)

	f := domain.LocationFilter{Mode: domain.LocationModeBrowse, CountryID: &countryID}
	res, err := svc.LocationStats(ctx, f, allLocationFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, d := range res.Distributions {
		var sum int64
		for _, b := range d.Buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("%s sums to %d, totalCount is %d — a partitioning facet lost or doubled a row",
				d.Facet, sum, res.TotalCount)
		}
	}
}
