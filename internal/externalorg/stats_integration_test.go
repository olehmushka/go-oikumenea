// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the external-organization dashboard aggregate (M58 ticket 2 /
// D-ObjectFacets).
//
// The differential is M57's exit criterion: `totalCount` equals the rows an exhaustive paging of the
// same list returns under the same filters, and every bucket's count equals the rows its own
// click-through returns. What is specific to this type is that it is the first VERTICAL on the seam,
// and that its `kind` filter accepts a code OR a RID — a bucket key is a RID, so if the widening were
// missing every kind segment would land on an empty list while looking perfectly fine on the chart.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/externalorg/... -run Stats
package externalorg_test

import (
	"context"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/externalorg/application"
	"github.com/olegamysk/go-oikumenea/internal/externalorg/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func allOrgFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("external_organization")
	if !ok {
		t.Fatal("external_organization is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func orgBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

// pageAllOrgs counts the rows an exhaustive paging of ListOrgs returns under the same filter — the
// right side of every differential below. It pages rather than asking for one big page, because the
// bug this is built to catch is exactly a dashboard that agrees with the first page and not with the
// set.
func pageAllOrgs(t *testing.T, svc *application.Service, f domain.OrgFilter) int {
	t.Helper()
	ctx := context.Background()
	n, token := 0, ""
	for i := 0; i < 200; i++ {
		rows, err := svc.ListOrgs(ctx, "", f, token, 25)
		if err != nil {
			t.Fatalf("ListOrgs: %v", err)
		}
		if len(rows) > 25 {
			rows = rows[:25]
			n += len(rows)
			token = rows[len(rows)-1].ID
			continue
		}
		return n + len(rows)
	}
	t.Fatal("paging did not terminate in 200 pages")
	return 0
}

// seedOrgSpread registers a spread of organizations across every facet, so no distribution is a
// single bucket and the top-N / (unknown) arms have something to do.
func seedOrgSpread(t *testing.T, svc *application.Service) {
	t.Helper()
	ctx := context.Background()
	party := kindID(t, newPool(t), "party")
	gov := kindID(t, newPool(t), "government_body")
	ua := uaCountryID(t, newPool(t))
	obs := time.Date(2025, 3, 14, 12, 0, 0, 0, time.UTC)
	older := time.Date(2024, 11, 2, 9, 0, 0, 0, time.UTC)

	rows := []domain.OrgInput{
		{KindID: party, Name: uniq("Party A"), Code: uniq("pa"), CountryID: ua, Status: "resolved", Source: "operator_verified", Confidence: "confirmed", AsOf: &obs},
		{KindID: party, Name: uniq("Party B"), Code: uniq("pb"), CountryID: ua, Status: "provisional", Source: "imported", Confidence: "possible", AsOf: &older},
		{KindID: gov, Name: uniq("Ministry"), Code: uniq("mi"), CountryID: ua, Status: "resolved", Source: "self_declared", Confidence: "probable", AsOf: &obs},
		// No country and no as_of: this row is the (unknown) bucket in two distributions at once.
		{KindID: gov, Name: uniq("Unattributed"), Code: uniq("un"), Status: "resolved", Source: "imported", Confidence: "possible"},
	}
	for _, in := range rows {
		if _, err := svc.CreateOrg(ctx, in); err != nil {
			t.Fatalf("CreateOrg %s: %v", in.Code, err)
		}
	}
}

// TestOrgStatsTotalMatchesExhaustivePaging is the M57 exit contract for this type: the number the
// dashboard prints is the number of rows the list would hand back, not an estimate and not a page.
func TestOrgStatsTotalMatchesExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	seedOrgSpread(t, svc)

	for _, f := range []domain.OrgFilter{
		{},
		{Status: ptr("resolved")},
		{Source: ptr("imported")},
		{Confidence: ptr("possible")},
		{Status: ptr("resolved"), Confidence: ptr("confirmed")},
	} {
		res, err := svc.OrgStats(ctx, "", f, allOrgFacets(t))
		if err != nil {
			t.Fatalf("OrgStats(%+v): %v", f, err)
		}
		if want := pageAllOrgs(t, svc, f); int(res.TotalCount) != want {
			t.Errorf("filter %+v: totalCount %d, exhaustive paging %d", f, res.TotalCount, want)
		}
	}
}

// TestOrgStatsFacetsSumToTotal: every counted row lands in exactly one bucket of every distribution.
// EVERY external-organization facet partitions — none is declared NonPartitioning — so this holds
// without exception here, which is what makes the taxon test's deliberate exception meaningful.
func TestOrgStatsFacetsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	seedOrgSpread(t, svc)

	o, _ := facet.Default.Get("external_organization")
	res, err := svc.OrgStats(ctx, "", domain.OrgFilter{}, allOrgFacets(t))
	if err != nil {
		t.Fatalf("OrgStats: %v", err)
	}
	for _, f := range o.Facets {
		if f.NonPartitioning != "" {
			t.Errorf("%s is declared NonPartitioning — no external-organization facet should be; "+
				"if that changed deliberately, this test must change with it", f.Key)
			continue
		}
		var sum int64
		for _, b := range orgBuckets(t, res, f.Key) {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("facet %q sums to %d, totalCount is %d", f.Key, sum, res.TotalCount)
		}
	}
}

// TestOrgStatsBucketsClickThrough is the property the whole vocabulary rests on: a chart segment and
// a filter are the same act. Every bucket of every facet is re-fetched as a list filter, and must
// return exactly the rows it counted.
//
// The `kind` case is the one that would fail without the code-or-RID widening: its bucket keys are
// kind RIDs and the arg was code-only until this ticket.
func TestOrgStatsBucketsClickThrough_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	seedOrgSpread(t, svc)

	res, err := svc.OrgStats(ctx, "", domain.OrgFilter{}, allOrgFacets(t))
	if err != nil {
		t.Fatalf("OrgStats: %v", err)
	}
	for _, spec := range []struct {
		facet string
		apply func(key string) domain.OrgFilter
	}{
		{"kind", func(k string) domain.OrgFilter { return domain.OrgFilter{Kind: &k} }},
		{"countryId", func(k string) domain.OrgFilter { return domain.OrgFilter{CountryID: &k} }},
		{"status", func(k string) domain.OrgFilter { return domain.OrgFilter{Status: &k} }},
		{"source", func(k string) domain.OrgFilter { return domain.OrgFilter{Source: &k} }},
		{"confidence", func(k string) domain.OrgFilter { return domain.OrgFilter{Confidence: &k} }},
	} {
		for _, b := range orgBuckets(t, res, spec.facet) {
			// (unknown) and (other) name no value, so no filter expresses them — bucketPatch returns
			// null for both in the console, and there is nothing to assert here either.
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
				continue
			}
			if got := pageAllOrgs(t, svc, spec.apply(b.Key)); int64(got) != b.Count {
				t.Errorf("%s bucket %q counted %d but its click-through returns %d rows",
					spec.facet, b.Key, b.Count, got)
			}
		}
	}
}

// TestOrgStatsMonthBucketClickThrough covers the one facet whose click-through is a RANGE rather than
// an equality: a `YYYY-MM` bucket becomes the month's two inclusive endpoints, and an off-by-one at
// either end is invisible on the chart and wrong in the list.
func TestOrgStatsMonthBucketClickThrough_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	seedOrgSpread(t, svc)

	res, err := svc.OrgStats(ctx, "", domain.OrgFilter{}, allOrgFacets(t))
	if err != nil {
		t.Fatalf("OrgStats: %v", err)
	}
	for _, b := range orgBuckets(t, res, "asOf") {
		if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
			continue
		}
		from, err := time.Parse("2006-01", b.Key)
		if err != nil {
			t.Fatalf("asOf bucket key %q is not a YYYY-MM month", b.Key)
		}
		// The console's monthPatch: first instant of the month to the last instant of it. The upper
		// bound is INCLUSIVE, deliberately not the next month's midnight.
		to := from.AddDate(0, 1, 0).Add(-time.Millisecond)
		got := pageAllOrgs(t, svc, domain.OrgFilter{AsOfFrom: &from, AsOfTo: &to})
		if int64(got) != b.Count {
			t.Errorf("asOf bucket %q counted %d but %s..%s returns %d rows", b.Key, b.Count, from, to, got)
		}
	}
}

// TestOrgStatsFacetsCsvSelectsExactly guards the M58 ticket-1 finding that an EMPTY facets list is
// not "no facets": absent/blank means every readable facet, and an explicit list of nothing means the
// total alone. Getting this backwards made the document dashboard fetch itself twice.
func TestOrgStatsFacetsCsvSelectsExactly_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	seedOrgSpread(t, svc)
	o, _ := facet.Default.Get("external_organization")

	countOnly, err := stats.Select(o, ",", nil)
	if err != nil {
		t.Fatalf("Select(count-only): %v", err)
	}
	res, err := svc.OrgStats(ctx, "", domain.OrgFilter{}, countOnly)
	if err != nil {
		t.Fatalf("OrgStats(count-only): %v", err)
	}
	if len(res.Distributions) != 0 {
		t.Errorf("`facets=,` returned %d distributions, want 0 (the total alone)", len(res.Distributions))
	}
	if res.TotalCount == 0 {
		t.Error("`facets=,` returned no total — the count is the whole point of the count-only form")
	}

	two, err := stats.Select(o, "status,source", nil)
	if err != nil {
		t.Fatalf("Select(two): %v", err)
	}
	res, err = svc.OrgStats(ctx, "", domain.OrgFilter{}, two)
	if err != nil {
		t.Fatalf("OrgStats(two): %v", err)
	}
	if len(res.Distributions) != 2 {
		t.Errorf("`facets=status,source` returned %d distributions, want 2", len(res.Distributions))
	}
}
