// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the person DASHBOARD aggregate (M57 / D-ObjectFacets) against a real
// Postgres.
//
// The contract this file exists to hold is one sentence, and it is M57's exit criterion:
//
//	for a fixed subject and a fixed filter set, stats.totalCount equals the number of rows
//	obtained by exhaustively paging the SAME list endpoint with the SAME filters.
//
// It is asserted on BOTH arms — the instance-admin one and the read-scope one — because they are
// separate SQL, and the failure it guards against is silent: a dashboard and a list that disagree
// look plausible individually. It is the same differential shape R-30 used for scope.Visibility.
//
// The second property is the sum invariant: a facet over person's OWN columns partitions the
// candidate set, so its buckets sum to totalCount. The cross-table facets (rankId, unitId) do not,
// and are asserted to behave the way the catalog says they do instead.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/person/... -run Stats
package person_test

import (
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// allPersonFacets is the selection a dashboard makes by default: everything the caller may read.
func allPersonFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("person")
	if !ok {
		t.Fatal("person is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func bucketsOf(t *testing.T, res stats.Result, facetKey string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == facetKey {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response (facets present: %v)", facetKey, facetKeys(res))
	return nil
}

func facetKeys(res stats.Result) []string {
	out := make([]string, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		out = append(out, d.Facet)
	}
	return out
}

func bucketCount(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func sumBuckets(bs []stats.Bucket) int64 {
	var n int64
	for _, b := range bs {
		n += b.Count
	}
	return n
}

// TestStatsTotalEqualsExhaustivePaging is the exit criterion, asserted per filter on both arms.
//
// The admin arm is bounded by the world's tag (carried in every seeded display name) because the
// test database is shared: without it the admin total would legitimately count every other suite's
// rows and the comparison would be about the harness rather than about the aggregate. Carrying the
// tag as `Query` also exercises the SEARCH arm of both queries, which is the copy most likely to
// drift, since it is the one with an extra predicate.
func TestStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	sel := allPersonFacets(t)

	filters := []domain.PersonFilter{
		{},
		{Sex: sp("female")},
		{Sex: sp("male")},
		{Status: sp("active")},
		{Status: sp("provisional")},
		{HasAccount: bp(true)},
		{HasAccount: bp(false)},
		{RankID: sp(w.rank)},
		{CountryOfBirth: sp(w.country)},
		{UnitID: sp(w.unit)},
		{BirthdateFrom: dp("1980-01-01")},
		{BirthdateTo: dp("1980-01-01")},
		{BirthdateFrom: dp("1985-01-01"), BirthdateTo: dp("1995-01-01")},
		{Sex: sp("female"), Status: sp("active"), UnitID: sp(w.unit)},
	}

	for i, f := range filters {
		// ---- read-scope arm: the reader's whole visible world, no tag needed ----
		scopedPaged := allIDs(t, func(tok string) (application.Page, error) {
			return w.svc.ListVisiblePersons(ctx, w.reader, f, 0, tok)
		})
		scopedStats, err := w.svc.PersonStats(ctx, w.reader, false, f, sel)
		if err != nil {
			t.Fatalf("filter %d: scoped stats: %v", i, err)
		}
		if got, want := scopedStats.TotalCount, int64(len(scopedPaged)); got != want {
			t.Errorf("filter %d (%+v): scoped totalCount = %d, but exhaustively paging the scoped list "+
				"returned %d rows", i, f, got, want)
		}

		// ---- instance-admin arm, bounded to this world by the search tag ----
		tagged := f
		tagged.Query = w.tag
		adminPaged := allIDs(t, func(tok string) (application.Page, error) {
			return w.svc.ListPersons(ctx, tagged, 0, tok)
		})
		adminStats, err := w.svc.PersonStats(ctx, "", true, tagged, sel)
		if err != nil {
			t.Fatalf("filter %d: admin stats: %v", i, err)
		}
		if got, want := adminStats.TotalCount, int64(len(adminPaged)); got != want {
			t.Errorf("filter %d (%+v): admin totalCount = %d, but exhaustively paging the admin list "+
				"returned %d rows", i, f, got, want)
		}
	}
}

// TestStatsBucketsSumToTotal: a facet over person's own columns partitions the candidate set. Every
// counted row must land in exactly one bucket — including the NULL rows, which is what the mandatory
// (unknown) bucket is for.
func TestStatsBucketsSumToTotal_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	sel := allPersonFacets(t)

	res, err := w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	// sex / status / birthdate are columns of person_persons; hasAccount is a per-person EXISTS, so
	// it partitions too.
	for _, key := range []string{"sex", "status", "birthdate", "hasAccount"} {
		if got := sumBuckets(bucketsOf(t, res, key)); got != res.TotalCount {
			t.Errorf("facet %q buckets sum to %d, want totalCount %d — a counted row fell out of every "+
				"bucket (or was counted twice)", key, got, res.TotalCount)
		}
	}
}

// TestStatsDistributionsMatchTheSeededWorld pins the actual numbers, so a query that is
// self-consistent but wrong (every row in one bucket, say) cannot pass the sum invariant above.
func TestStatsDistributionsMatchTheSeededWorld_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	sel := allPersonFacets(t)

	res, err := w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	// The world: alice (female, active, 1990, account, rank), bogdan (male, active, 1975),
	// clara (female, provisional, NULL birthdate), reader (seeded default). The reader's own sex and
	// status come from the seed helper, so only the three characterised people are asserted.
	sex := bucketsOf(t, res, "sex")
	if got := bucketCount(sex, "female"); got < 2 {
		t.Errorf("sex[female] = %d, want at least alice + clara", got)
	}
	if got := bucketCount(sex, "male"); got < 1 {
		t.Errorf("sex[male] = %d, want at least bogdan", got)
	}
	// Zero-count buckets are PRESENT, so a chart's shape is stable across filterings.
	for _, want := range []string{"not_known", "male", "female", "not_applicable"} {
		if bucketCount(sex, want) < 0 {
			t.Errorf("sex distribution is missing the declared bucket %q — zero-count buckets must be "+
				"emitted, or a chart changes shape as it is filtered", want)
		}
	}
	if got, want := len(sex), 4; got != want && bucketCount(sex, stats.BucketUnknown) < 0 {
		t.Errorf("sex has %d buckets, want the %d declared values", got, want)
	}

	// clara's NULL birthdate is the (unknown) age band — the data-quality signal the catalog makes
	// mandatory for a nullable column.
	if got := bucketCount(bucketsOf(t, res, "birthdate"), stats.BucketUnknown); got < 1 {
		t.Errorf("birthdate[(unknown)] = %d, want at least clara — a nullable column's NULLs must be "+
			"visible, not dropped", got)
	}
	// alice is 30-something: the band is computed in SQL as whole years and assigned by the catalog.
	if sum := sumBuckets(bucketsOf(t, res, "birthdate")); sum != res.TotalCount {
		t.Errorf("birthdate bands sum to %d, want %d", sum, res.TotalCount)
	}

	// rankId LEFT-joins, so the rankless people are the (unknown) bucket rather than missing rows.
	rank := bucketsOf(t, res, "rankId")
	if got := bucketCount(rank, w.rank); got != 1 {
		t.Errorf("rankId[%s] = %d, want alice alone", w.rank, got)
	}
	if got := bucketCount(rank, stats.BucketUnknown); got < 3 {
		t.Errorf("rankId[(unknown)] = %d, want the three rankless people", got)
	}

	// unitId counts ACTIVE memberships: everyone in this world belongs to w.unit.
	if got := bucketCount(bucketsOf(t, res, "unitId"), w.unit); got != 4 {
		t.Errorf("unitId[%s] = %d, want all four seeded people", w.unit, got)
	}

	// hasAccount is a real two-sided distribution (the directory is account-optional).
	acct := bucketsOf(t, res, "hasAccount")
	if bucketCount(acct, "true") < 1 || bucketCount(acct, "false") < 1 {
		t.Errorf("hasAccount = %v, want both sides populated", acct)
	}
}

// TestStatsSelectionSkipsUnselectedFacets: the `facets` CSV is a pushdown, not a response filter —
// an unselected facet is never grouped, and it is absent from the response rather than zeroed.
func TestStatsSelectionSkipsUnselectedFacets_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	o, _ := facet.Default.Get("person")

	sel, err := stats.Select(o, "sex,status", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	res, err := w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got := facetKeys(res); len(got) != 2 || got[0] != "sex" || got[1] != "status" {
		t.Fatalf("distributions = %v, want [sex status]", got)
	}
	// The total is still computed: it is what "how many" means, and it costs one branch.
	if res.TotalCount < 4 {
		t.Errorf("totalCount = %d with a narrowed selection, want the whole visible set", res.TotalCount)
	}

	// An empty selection is the cheap count-only request.
	none, err := stats.Select(o, ",", nil)
	if err != nil {
		t.Fatalf("Select(none): %v", err)
	}
	res, err = w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, none)
	if err != nil {
		t.Fatalf("stats(none): %v", err)
	}
	if len(res.Distributions) != 0 {
		t.Errorf("distributions = %v, want none", facetKeys(res))
	}
	if res.TotalCount < 4 {
		t.Errorf("count-only totalCount = %d, want the whole visible set", res.TotalCount)
	}
}

// TestStatsCountsInsideTheVisibilityPredicate: the reader must not be told how many people they
// cannot see. A person outside the reach is absent from the scoped total AND from every bucket,
// while the admin arm counts them — the asymmetry is the proof the predicate is folded in.
func TestStatsCountsInsideTheVisibilityPredicate_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	sel := allPersonFacets(t)

	// Snapshots BEFORE the outsider exists, so the assertion is "adding an unreadable person moved
	// the admin total and not the scoped one" rather than a comparison of two differently-bounded
	// worlds (the admin arm is bounded by the tag, which the reader does not carry).
	beforeScoped, err := w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats (before): %v", err)
	}
	beforeAdmin, err := w.svc.PersonStats(ctx, "", true, domain.PersonFilter{Query: w.tag}, sel)
	if err != nil {
		t.Fatalf("admin stats (before): %v", err)
	}

	// A person in a unit the reader has no grant on, tagged into this world so the admin arm sees it.
	otherUnit := seedUnit(t, w.pool)
	outsider := seedPerson(t, w.svc)
	seedMembership(t, w.pool, outsider, otherUnit)
	exec(t, w.pool, `UPDATE oikumenea.person_persons SET sex='male', display_name = $2 || ' ' || display_name WHERE id=$1`,
		outsider, w.tag)

	scoped, err := w.svc.PersonStats(ctx, w.reader, false, domain.PersonFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}
	scopedUnits := bucketsOf(t, scoped, "unitId")
	if got := bucketCount(scopedUnits, otherUnit); got > 0 {
		t.Errorf("the scoped dashboard counts %d people in an out-of-reach unit — the count is being "+
			"taken outside the visibility predicate", got)
	}

	admin, err := w.svc.PersonStats(ctx, "", true, domain.PersonFilter{Query: w.tag}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	if got := bucketCount(bucketsOf(t, admin, "unitId"), otherUnit); got != 1 {
		t.Errorf("admin unitId[%s] = %d, want the outsider — otherwise this test proves nothing about "+
			"the scoped arm", otherUnit, got)
	}
	if got, want := admin.TotalCount, beforeAdmin.TotalCount+1; got != want {
		t.Errorf("admin totalCount = %d after adding one tagged person, want %d — this test proves "+
			"nothing about the scoped arm unless the admin arm actually counts the outsider", got, want)
	}
	if got, want := scoped.TotalCount, beforeScoped.TotalCount; got != want {
		t.Errorf("scoped totalCount moved from %d to %d when a person the reader CANNOT read was "+
			"added — the count is being taken outside the visibility predicate", want, got)
	}
}
