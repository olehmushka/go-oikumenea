// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the institution DASHBOARD aggregate (M58 ticket 5 / D-ObjectFacets) against a
// real Postgres.
//
// The differential is the vocabulary's usual one — totalCount equals the rows an exhaustive paging of
// the same list returns under the same filters, and every bucket's count equals what its own filter
// returns. Institution is the thinner of the two profile types (four facets against company's six),
// so what it carries alone is the YEAR-grain inverse over a genuinely wide span: an institution
// founded in 1661 and one founded in 2016 are one chart, which is the case the grain exists for.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/education/... -run InstitutionStats
package education_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/education/application"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func allInstitutionFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("institution")
	if !ok {
		t.Fatal("institution is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

// pageAllInstitutions exhaustively pages listInstitutions under one filter and returns the row count.
// Small pages on purpose: a bug that loses rows at a page boundary (the shape ticket 2 found in
// listTaxa, which had silently dropped 16 of 100 taxa since M22) only shows up when the sweep turns
// over. It also asserts no id repeats, which is the other half of a broken keyset.
func pageAllInstitutions(t *testing.T, svc *application.Service, f domain.InstitutionFilter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 500; i++ {
		rows, err := svc.ListInstitutions(ctx, f, token, 3)
		if err != nil {
			t.Fatalf("list institutions: %v", err)
		}
		page := rows
		next := ""
		if len(rows) > 3 {
			page = rows[:3]
			next = page[len(page)-1].ID
		}
		for _, in := range page {
			if seen[in.ID] {
				t.Fatalf("institution %s returned twice — the keyset is broken", in.ID)
			}
			seen[in.ID] = true
		}
		if next == "" {
			return len(seen)
		}
		token = next
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// seedInstitutionSpread creates a population sharing one KIND, so every assertion below can be scoped
// to this test's rows: the test database is persistent, and an unfiltered assertion would race every
// other test that creates an institution. The founding years span three centuries deliberately —
// that is the case the year grain exists for, and a spread inside one decade would let a month-grain
// regression pass.
func seedInstitutionSpread(t *testing.T, pool *pgxpool.Pool, svc *application.Service) (kind string) {
	t.Helper()
	ctx := context.Background()
	kind = catalogID(t, pool, "education_institution_kinds", "gymnasium") // this suite's own corner
	if _, err := pool.Exec(ctx,
		`DELETE FROM oikumenea.education_org_profiles WHERE kind_id = $1`, kind); err != nil {
		t.Fatalf("clear prior spread: %v", err)
	}
	rows := []struct{ code, founded string }{
		{uniq("spread-old"), "1661-01-20"},
		{uniq("spread-mid"), "1898-08-31"},
		{uniq("spread-new"), "2016-01-11"},
	}
	for _, r := range rows {
		if _, err := svc.CreateInstitution(ctx, domain.InstitutionInput{
			Code: r.code, Name: "Spread " + r.code, KindID: kind, FoundedOn: ptr(r.founded),
		}); err != nil {
			t.Fatalf("seed institution %s: %v", r.code, err)
		}
	}
	return kind
}

// TestInstitutionStatsTotalEqualsExhaustivePaging is D-ObjectFacets' headline promise, on the ADMIN
// arm: the number the dashboard prints is the number of rows the list would hand over.
func TestInstitutionStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	kind := seedInstitutionSpread(t, pool, svc)

	f := domain.InstitutionFilter{KindID: &kind}
	res, err := svc.InstitutionStats(ctx, "", true, f, allInstitutionFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), pageAllInstitutions(t, svc, f); got != want {
		t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
	}
}

// TestInstitutionStatsEveryBucketEqualsItsOwnFilter is the property the whole vocabulary rests on: a
// chart segment and a list filter must be the same act. The `foundedOn` case carries the year-grain
// inverse, which is where a new grain is most likely to be quietly wrong — an off-by-one on the year
// boundary is invisible on the chart and wrong in the list.
func TestInstitutionStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	kind := seedInstitutionSpread(t, pool, svc)

	base := domain.InstitutionFilter{KindID: &kind}
	res, err := svc.InstitutionStats(ctx, "", true, base, allInstitutionFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	years := 0
	checked := 0
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == "(unknown)" || b.Key == "(other)" || b.Count == 0 {
				continue
			}
			f := base
			switch d.Facet {
			case "kindId":
				k := b.Key
				f.KindID = &k
			case "countryId":
				k := b.Key
				f.CountryID = &k
			case "state":
				k := b.Key
				f.State = &k
			case "foundedOn":
				from, to := b.Key+"-01-01", b.Key+"-12-31"
				f.FoundedOnFrom, f.FoundedOnTo = &from, &to
				years++
			default:
				t.Fatalf("unhandled facet %q — a new facet must be given its filter inverse here", d.Facet)
			}
			if got, want := pageAllInstitutions(t, svc, f), int(b.Count); got != want {
				t.Errorf("%s bucket %q counted %d but its own filter returns %d rows", d.Facet, b.Key, want, got)
			}
			checked++
		}
	}
	if years < 3 {
		t.Errorf("only %d year buckets checked — the fixture must span several years or the year-grain "+
			"inverse is not being exercised", years)
	}
	if checked < 5 {
		t.Fatalf("only %d buckets checked — the fixture is too thin to be a differential", checked)
	}
}
