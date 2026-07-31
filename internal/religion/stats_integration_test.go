// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the taxonomy dashboard aggregate (M58 ticket 2 / D-ObjectFacets).
//
// This is the type the NonPartitioning property was invented for, so these tests carry the whole
// argument for it. Two of the four facets count each taxon under several buckets — `subtree` under
// every ancestor, `classification` under every effective theism tag — and the point of the design is
// that the overlap is between BUCKETS and never between a bucket and its own filter:
//
//   - TestTaxonStatsPartitioningFacetsSumToTotal holds the ordinary invariant for the two facets that
//     do partition, so the exemption is scoped rather than blanket;
//   - TestTaxonStatsNonPartitioningFacetsOverlap proves the overlap is real (sum > total) rather than
//     an accident nobody would notice;
//   - TestTaxonStatsBucketsClickThrough holds the invariant that MATTERS for all four alike;
//   - TestTaxonSubtreeDrillsDownRecursively is the one that justifies the whole choice: click a
//     bucket, re-group inside the result, click again, and the counts keep meaning what they say.
//
// Every test runs against the migration-seeded taxonomy, which is a real multi-level tree
// (christianity → protestantism → … ) rather than a fixture — the closure arithmetic is only
// interesting at depth.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/religion/... -run TaxonStats
package religion_test

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/religion/application"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func rptr(v string) *string { return &v }

func bucketCountOf(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func allTaxonFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("taxon")
	if !ok {
		t.Fatal("taxon is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func taxonBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

// pageAllTaxa counts the rows an exhaustive paging of ListTaxa returns under the same filter. It
// pages rather than asking for one big page, because the failure this exists to catch is a dashboard
// that agrees with the first page and not with the set.
func pageAllTaxa(t *testing.T, svc *application.Service, f domain.TaxonFilter) int {
	t.Helper()
	ctx := context.Background()
	n, token := 0, ""
	for i := 0; i < 200; i++ {
		rows, err := svc.ListTaxa(ctx, "", f, token, 25)
		if err != nil {
			t.Fatalf("ListTaxa(%+v): %v", f, err)
		}
		if len(rows) > 25 {
			rows = rows[:25]
			n += len(rows)
			token = rows[len(rows)-1].Code // the keyset column, matching ListTaxa's ORDER BY
			continue
		}
		return n + len(rows)
	}
	t.Fatal("paging did not terminate in 200 pages")
	return 0
}

// TestTaxonStatsTotalMatchesExhaustivePaging is the M57 exit contract: the number the dashboard
// prints is the number of rows the list hands back.
func TestTaxonStatsTotalMatchesExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))

	for _, f := range []domain.TaxonFilter{
		{},
		{Rank: rptr("religion")},
		{Rank: rptr("denomination")},
		{Religion: rptr("christianity")},
		{Classification: rptr("monotheistic")},
		{Religion: rptr("christianity"), Rank: rptr("branch")},
	} {
		res, err := svc.TaxonStats(ctx, "", f, allTaxonFacets(t))
		if err != nil {
			t.Fatalf("TaxonStats(%+v): %v", f, err)
		}
		if want := pageAllTaxa(t, svc, f); int(res.TotalCount) != want {
			t.Errorf("filter %+v: totalCount %d, exhaustive paging %d", f, res.TotalCount, want)
		}
	}
}

// TestTaxonStatsPartitioningFacetsSumToTotal keeps the exemption SCOPED: the two facets that are not
// declared NonPartitioning must still obey the ordinary invariant, so the property cannot quietly
// become "this type's numbers do not have to add up".
func TestTaxonStatsPartitioningFacetsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	o, _ := facet.Default.Get("taxon")

	res, err := svc.TaxonStats(ctx, "", domain.TaxonFilter{}, allTaxonFacets(t))
	if err != nil {
		t.Fatalf("TaxonStats: %v", err)
	}
	checked := 0
	for _, f := range o.Facets {
		if f.NonPartitioning != "" {
			continue
		}
		checked++
		var sum int64
		for _, b := range taxonBuckets(t, res, f.Key) {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("facet %q is not declared NonPartitioning but sums to %d against a totalCount of %d",
				f.Key, sum, res.TotalCount)
		}
	}
	if checked == 0 {
		t.Fatal("no partitioning facet on taxon — this test would be vacuous, and it is the one that " +
			"keeps the exemption from covering the whole type")
	}
}

// TestTaxonStatsNonPartitioningFacetsOverlap proves the overlap is REAL. Without this, a declaration
// that turned out to partition after all would sit in the catalog claiming an exemption it does not
// need — and the next person would copy it.
func TestTaxonStatsNonPartitioningFacetsOverlap_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))
	o, _ := facet.Default.Get("taxon")

	res, err := svc.TaxonStats(ctx, "", domain.TaxonFilter{}, allTaxonFacets(t))
	if err != nil {
		t.Fatalf("TaxonStats: %v", err)
	}
	checked := 0
	for _, f := range o.Facets {
		if f.NonPartitioning == "" {
			continue
		}
		checked++
		var sum int64
		for _, b := range taxonBuckets(t, res, f.Key) {
			sum += b.Count
		}
		if sum < res.TotalCount {
			t.Errorf("facet %q sums to %d, BELOW the totalCount of %d — a non-partitioning facet may "+
				"double-count but must never lose a row", f.Key, sum, res.TotalCount)
		}
	}
	if checked != 2 {
		t.Fatalf("expected exactly 2 NonPartitioning taxon facets (subtree, classification), found %d", checked)
	}
	// And the overlap must be VISIBLE in the seeded tree, not merely permitted: a deep taxonomy has
	// more (taxon, ancestor) pairs than taxa. If this ever fails, the closure join is wrong — or the
	// facet did not need the exemption at all.
	var subtreeSum int64
	for _, b := range taxonBuckets(t, res, "subtree") {
		subtreeSum += b.Count
	}
	if subtreeSum <= res.TotalCount {
		t.Errorf("subtree sums to %d against a totalCount of %d — the closure join is not producing "+
			"the multi-ancestor rows the whole design depends on", subtreeSum, res.TotalCount)
	}
}

// TestTaxonStatsBucketsClickThrough is the invariant NonPartitioning does NOT relax, and the reason
// the overlap is acceptable: whatever a bucket counted, its own filter returns exactly that.
func TestTaxonStatsBucketsClickThrough_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))

	res, err := svc.TaxonStats(ctx, "", domain.TaxonFilter{}, allTaxonFacets(t))
	if err != nil {
		t.Fatalf("TaxonStats: %v", err)
	}
	for _, spec := range []struct {
		facet string
		apply func(key string) domain.TaxonFilter
	}{
		{"rankId", func(k string) domain.TaxonFilter { return domain.TaxonFilter{Rank: &k} }},
		{"religionId", func(k string) domain.TaxonFilter { return domain.TaxonFilter{Religion: &k} }},
		{"subtree", func(k string) domain.TaxonFilter { return domain.TaxonFilter{Parent: &k} }},
		{"classification", func(k string) domain.TaxonFilter { return domain.TaxonFilter{Classification: &k} }},
	} {
		checked := 0
		for _, b := range taxonBuckets(t, res, spec.facet) {
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
				continue
			}
			checked++
			if got := pageAllTaxa(t, svc, spec.apply(b.Key)); int64(got) != b.Count {
				t.Errorf("%s bucket %q counted %d but its click-through returns %d rows",
					spec.facet, b.Key, b.Count, got)
			}
		}
		if checked == 0 {
			t.Errorf("%s produced no real buckets — the assertion above ran zero times", spec.facet)
		}
	}
}

// TestTaxonSubtreeDrillsDownRecursively is the test this design was chosen for.
//
// The alternative — grouping by parent_id and filtering to an exact parent — partitions cleanly and
// then dead-ends: after one click every remaining row shares one parent, so the chart collapses to a
// single bucket and there is nowhere to go. The closure facet keeps working, and this walks it:
//
//	level 0: pick the largest subtree bucket
//	level 1: filter to it; its buckets are that subtree's own internal nodes, each still exact
//	level 2: filter to one of THOSE; still exact
//
// At every step the bucket count must equal what the list returns under that bucket's filter, and the
// candidate set must strictly shrink — a drill that stopped narrowing would be a loop, not a descent.
func TestTaxonSubtreeDrillsDownRecursively_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))

	filter := domain.TaxonFilter{}
	prevTotal := int64(-1)
	for level := 0; level < 3; level++ {
		res, err := svc.TaxonStats(ctx, "", filter, allTaxonFacets(t))
		if err != nil {
			t.Fatalf("level %d: TaxonStats: %v", level, err)
		}
		if prevTotal >= 0 && res.TotalCount >= prevTotal {
			t.Fatalf("level %d: totalCount %d did not shrink below the previous %d — the drill is not "+
				"descending", level, res.TotalCount, prevTotal)
		}
		prevTotal = res.TotalCount

		// EVERY bucket must be a real descent. Once filtered to X's subtree, neither X nor any of X's
		// own ancestors may appear: `parent` is single-valued, so clicking one of those would replace
		// the anchor and WIDEN the set — landing on more rows than the bucket counted. The aggregate's
		// join back to the candidate set is what excludes them, and this is the assertion that says so.
		if filter.Parent != nil {
			if got := bucketCountOf(taxonBuckets(t, res, "subtree"), *filter.Parent); got != -1 {
				t.Errorf("level %d: the anchor %q is still offered as a bucket (count %d) — clicking it "+
					"is a no-op at best and a widening at worst", level, *filter.Parent, got)
			}
		}

		// The largest bucket: the deepest subtree still available to descend into.
		var pick stats.Bucket
		for _, b := range taxonBuckets(t, res, "subtree") {
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
				continue
			}
			if b.Count > pick.Count {
				pick = b
			}
		}
		if pick.Key == "" {
			if level == 0 {
				t.Fatal("no subtree buckets at all — the seeded taxonomy is flat and this test is vacuous")
			}
			return // a leaf-ward subtree with no internal nodes left; the descent legitimately ends
		}

		// The click, exactly as the console builds it: parent = the bucket key.
		next := filter
		next.Parent = &pick.Key
		got := pageAllTaxa(t, svc, next)
		if int64(got) != pick.Count {
			t.Fatalf("level %d: subtree bucket %q counted %d but its click-through returns %d rows — "+
				"the recursion is only honest if this holds at EVERY level, not just the first",
				level, pick.Key, pick.Count, got)
		}
		filter = next
	}
}

// TestTaxonClassificationResolvesEffectiveTags proves the classification facet counts INHERITED tags,
// not just declared ones. The seed declares theism only on root religions, so a declared-only
// implementation would put nearly every taxon in (unknown) — passing every other test in this file
// while making the chart useless. This is the test that tells the two apart.
func TestTaxonClassificationResolvesEffectiveTags_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newService(t, newPool(t))

	res, err := svc.TaxonStats(ctx, "", domain.TaxonFilter{}, allTaxonFacets(t))
	if err != nil {
		t.Fatalf("TaxonStats: %v", err)
	}
	var unknown, tagged int64
	for _, b := range taxonBuckets(t, res, "classification") {
		if b.Key == stats.BucketUnknown {
			unknown = b.Count
			continue
		}
		tagged += b.Count
	}
	if tagged <= unknown {
		t.Errorf("classification: %d tagged vs %d untagged — the seed declares theism only on ROOTS, "+
			"so a majority of untagged taxa means the resolution is counting declared tags instead of "+
			"effective ones", tagged, unknown)
	}
	// And concretely: a deep Christian taxon inherits `monotheistic` from the root, so filtering by it
	// must return more than the handful of roots that declare it.
	monotheistic := pageAllTaxa(t, svc, domain.TaxonFilter{Classification: rptr("monotheistic")})
	roots := pageAllTaxa(t, svc, domain.TaxonFilter{Rank: rptr("religion")})
	if monotheistic <= roots {
		t.Errorf("classification=monotheistic returns %d taxa but there are %d roots in total — "+
			"inheritance down the closure is not happening", monotheistic, roots)
	}
}
