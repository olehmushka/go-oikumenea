// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the company DASHBOARD aggregate (M58 ticket 5 / D-ObjectFacets) against a
// real Postgres.
//
// The differential is the vocabulary's usual one — totalCount equals the rows an exhaustive paging of
// the same list returns under the same filters, and every bucket's count equals what its own filter
// returns. What company adds is a facet whose table is NOT the listed one: `industryClass` reads
// company_industry_assignments, an M:N table, confined to the PRIMARY assignment so the distribution
// partitions. That confinement is the ticket's one real design decision here, and
// TestCompanyIndustryFacetCountsOnlyThePrimary pins it rather than leaving it as prose — a company
// with a primary and two secondaries must be counted ONCE, under its primary.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/company/... -run CompanyStats
package company_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/company/application"
	"github.com/olehmushka/go-oikumenea/internal/company/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

func allCompanyFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("company")
	if !ok {
		t.Fatal("company is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

// pageAllCompanies exhaustively pages listCompanies under one filter and returns the row count. Small
// pages on purpose: a bug that loses rows at a page boundary (the shape ticket 2 found in listTaxa,
// which had silently dropped 16 of 100 taxa since M22) only shows up when the sweep turns over. It
// also asserts no id repeats, which is the other half of a broken keyset.
func pageAllCompanies(t *testing.T, svc *application.Service, f domain.CompanyFilter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 500; i++ {
		rows, err := svc.ListCompanies(ctx, f, token, 3)
		if err != nil {
			t.Fatalf("list companies: %v", err)
		}
		page := rows
		next := ""
		if len(rows) > 3 {
			page = rows[:3]
			next = page[len(page)-1].ID
		}
		for _, c := range page {
			if seen[c.ID] {
				t.Fatalf("company %s returned twice — the keyset is broken", c.ID)
			}
			seen[c.ID] = true
		}
		if next == "" {
			return len(seen)
		}
		token = next
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// seedCompanySpread creates a small population sharing one legal form, so every assertion below can
// be scoped to THIS test's rows: the test database is persistent, and an unfiltered assertion would
// race every other test that creates a company. The spread covers two ownership categories, two
// states and two founding years, because a single-bucket distribution makes a differential pass
// without testing anything.
func seedCompanySpread(t *testing.T, pool *pgxpool.Pool, svc *application.Service) (legalForm string) {
	t.Helper()
	ctx := context.Background()
	legalForm = catalogID(t, pool, "company_legal_forms", "gmbh") // unused by the other suites
	if _, err := pool.Exec(ctx,
		`DELETE FROM oikumenea.company_org_profiles WHERE legal_form_id = $1`, legalForm); err != nil {
		t.Fatalf("clear prior spread: %v", err)
	}
	rows := []struct {
		code, ownership, founded string
	}{
		{uniq("spread-a"), "private", "1998-03-01"},
		{uniq("spread-b"), "private", "1998-07-15"},
		{uniq("spread-c"), "state_owned", "2014-01-20"},
	}
	for _, r := range rows {
		c, err := svc.CreateCompany(ctx, domain.CompanyInput{
			Code: r.code, LegalName: "Spread " + r.code, LegalFormID: legalForm,
			OwnershipCategory: ptr(r.ownership), FoundedOn: ptr(r.founded),
		})
		if err != nil {
			t.Fatalf("seed company %s: %v", r.code, err)
		}
		_ = c
	}
	return legalForm
}

// TestCompanyStatsTotalEqualsExhaustivePaging is D-ObjectFacets' headline promise, on the ADMIN arm:
// the number the dashboard prints is the number of rows the list would hand over, not an estimate and
// not a differently-filtered count.
func TestCompanyStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	legalForm := seedCompanySpread(t, pool, svc)

	f := domain.CompanyFilter{LegalFormID: &legalForm}
	res, err := svc.CompanyStats(ctx, "", true, f, allCompanyFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), pageAllCompanies(t, svc, f); got != want {
		t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
	}
}

// TestCompanyStatsEveryBucketEqualsItsOwnFilter is the property the whole vocabulary rests on, and
// the one a chart cannot check for itself: clicking a segment must land on exactly the rows that
// segment counted. A wrong inverse fails silently — the operator gets a list that quietly disagrees
// with the bar they clicked, and neither number looks wrong on its own.
func TestCompanyStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	legalForm := seedCompanySpread(t, pool, svc)

	base := domain.CompanyFilter{LegalFormID: &legalForm}
	res, err := svc.CompanyStats(ctx, "", true, base, allCompanyFacets(t))
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
			case "legalForm":
				k := b.Key
				f.LegalFormID = &k
			case "ownershipCategory":
				k := b.Key
				f.OwnershipCategory = &k
			case "countryId":
				k := b.Key
				f.CountryID = &k
			case "industryClass":
				k := b.Key
				f.IndustryClassID = &k
			case "state":
				k := b.Key
				f.State = &k
			case "foundedOn":
				// A YEAR bucket, the first grain of its kind in the vocabulary. Its inverse is the
				// whole calendar year — the same arithmetic the console's yearPatch does, asserted
				// here against the database rather than trusted.
				from, to := b.Key+"-01-01", b.Key+"-12-31"
				f.FoundedOnFrom, f.FoundedOnTo = &from, &to
			default:
				t.Fatalf("unhandled facet %q — a new facet must be given its filter inverse here", d.Facet)
			}
			if got, want := pageAllCompanies(t, svc, f), int(b.Count); got != want {
				t.Errorf("%s bucket %q counted %d but its own filter returns %d rows", d.Facet, b.Key, want, got)
			}
			checked++
		}
	}
	if checked < 6 {
		t.Fatalf("only %d buckets checked — the fixture is too thin to be a differential", checked)
	}
}

// TestCompanyIndustryFacetCountsOnlyThePrimary pins the ticket's one real design decision here.
//
// company_industry_assignments is M:N — one primary plus secondaries — so grouping it raw would count
// a diversified company once per NACE code it carries, and the distribution would need the
// NonPartitioning exemption. Confining it to the primary (of which the schema's partial unique index
// guarantees at most one) makes it partition honestly AND answers the question the chart is read for.
//
// The assertion is that a company with a primary and two secondaries appears ONCE, under its primary,
// and NOT under either secondary — which is what a raw join would get wrong, and what would otherwise
// only show up as a distribution that mysteriously sums to more than totalCount.
func TestCompanyIndustryFacetCountsOnlyThePrimary_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	legalForm := catalogID(t, pool, "company_legal_forms", "plc") // this test's own corner
	if _, err := pool.Exec(ctx, `DELETE FROM oikumenea.company_org_profiles WHERE legal_form_id = $1`, legalForm); err != nil {
		t.Fatalf("clear prior: %v", err)
	}
	primary := catalogID(t, pool, "company_industry_classes", "nace-j")
	secondA := catalogID(t, pool, "company_industry_classes", "nace-g")
	secondB := catalogID(t, pool, "company_industry_classes", "nace-m")

	c, err := svc.CreateCompany(ctx, domain.CompanyInput{
		Code: uniq("multi"), LegalName: "Diversified PLC", LegalFormID: legalForm,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, in := range []domain.IndustryInput{
		{IndustryClassID: primary, IsPrimary: true},
		{IndustryClassID: secondA},
		{IndustryClassID: secondB},
	} {
		if _, err := svc.AssignIndustry(ctx, c.ID, in); err != nil {
			t.Fatalf("assign industry: %v", err)
		}
	}

	f := domain.CompanyFilter{LegalFormID: &legalForm}
	res, err := svc.CompanyStats(ctx, "", true, f, allCompanyFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 1 {
		t.Fatalf("totalCount = %d, want the 1 company this test created", res.TotalCount)
	}
	var total int64
	for _, d := range res.Distributions {
		if d.Facet != "industryClass" {
			continue
		}
		for _, b := range d.Buckets {
			total += b.Count
			switch b.Key {
			case primary:
				if b.Count != 1 {
					t.Errorf("primary industry bucket counted %d, want 1", b.Count)
				}
			case secondA, secondB:
				t.Errorf("a SECONDARY industry (%s) has a bucket of %d — the facet is joining the whole "+
					"M:N table, so a diversified company is counted once per code it carries and the "+
					"distribution no longer partitions", b.Key, b.Count)
			}
		}
	}
	// The summation property in its smallest form: one company, one bucket, one row.
	if total != res.TotalCount {
		t.Errorf("industryClass buckets sum to %d but totalCount is %d — the distribution does not "+
			"partition, which is exactly what confining it to the primary assignment prevents",
			total, res.TotalCount)
	}
}
