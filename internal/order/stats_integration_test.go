// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the order-register DASHBOARD aggregate (M57 ticket 2 / D-ObjectFacets).
//
// The exit criterion is the same differential — `stats.totalCount` equals the rows an exhaustive
// paging of the same list returns under the same filters, on both arms.
//
// What is specific to `order` is the one MULTIPLYING facet in the whole M57 tranche: an order's effect
// lives on its ITEMS, so `orderTypeId` counts item types and an order with two effects lands in two
// buckets. That facet therefore does NOT sum to totalCount, and the test asserts the documented
// behaviour rather than exempting it silently. The `issuedOn` histogram's `(unknown)` bucket is the
// draft backlog, which must be present even when it is empty.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/order/... -run Stats
package order_test

import (
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/order/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

func orderSel(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("order")
	if !ok {
		t.Fatal("order is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func oBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func oCount(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func oSum(bs []stats.Bucket) int64 {
	var n int64
	for _, b := range bs {
		n += b.Count
	}
	return n
}

func TestOrderStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	w := seedOFacetWorld(t)
	ctx := context.Background()
	sel := orderSel(t)

	filters := []domain.OrderFilter{
		{},
		{Status: osp("draft")},
		{Status: osp("issued")},
		{Status: osp("revoked")},
		{IssuingUnitID: osp(w.unitIn)},
		{OrderTypeID: osp(w.typeA)},
		{IssuedOnFrom: odp("2023-01-01")},
		{IssuedOnTo: odp("2021-01-01")},
		{IssuedOnFrom: odp("2024-05-06"), IssuedOnTo: odp("2024-05-06")},
		{IssuingUnitID: osp(w.unitIn), Status: osp("issued")},
	}
	for i, f := range filters {
		scoped, err := w.svc.OrderStats(ctx, w.reader, false, f, sel)
		if err != nil {
			t.Fatalf("filter %d: scoped stats: %v", i, err)
		}
		if got, want := scoped.TotalCount, int64(len(w.scoped(t, f))); got != want {
			t.Errorf("filter %d (%+v): scoped totalCount = %d, but exhaustively paging the scoped list "+
				"returned %d rows", i, f, got, want)
		}
		// Bounded to this world's issuing unit: the test database is shared, so an unbounded admin
		// comparison would race a concurrent insert from another suite rather than measure drift between
		// the aggregate and the list. The differential holds over any predicate.
		af := f
		if af.IssuingUnitID == nil {
			af.IssuingUnitID = osp(w.unitIn)
		}
		admin, err := w.svc.OrderStats(ctx, "", true, af, sel)
		if err != nil {
			t.Fatalf("filter %d: admin stats: %v", i, err)
		}
		if got, want := admin.TotalCount, int64(len(w.admin(t, af))); got != want {
			t.Errorf("filter %d (%+v): admin totalCount = %d, but exhaustively paging the admin list "+
				"returned %d rows", i, f, got, want)
		}
	}
}

// TestOrderStatsSumInvariantAndTheMultiplyingFacet: the header facets partition the candidate set;
// orderTypeId deliberately does not, because it counts a different thing (items).
func TestOrderStatsSumInvariantAndTheMultiplyingFacet_Integration(t *testing.T) {
	w := seedOFacetWorld(t)
	ctx := context.Background()
	sel := orderSel(t)

	res, err := w.svc.OrderStats(ctx, w.reader, false, domain.OrderFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, key := range []string{"issuingUnitId", "status", "issuedOn"} {
		if got := oSum(oBuckets(t, res, key)); got != res.TotalCount {
			t.Errorf("facet %q sums to %d, want totalCount %d", key, got, res.TotalCount)
		}
	}
	// orderTypeId: one order carries an item of typeA, the other two carry none, so the (unknown)
	// bucket holds the item-less orders and the total still adds up here — with several items per order
	// it would exceed the total, which is the documented (and intended) behaviour.
	types := oBuckets(t, res, "orderTypeId")
	if got := oCount(types, w.typeA); got != 1 {
		t.Errorf("orderTypeId[%s] = %d, want the one order carrying that effect", w.typeA, got)
	}
	if got := oCount(types, stats.BucketUnknown); got != 2 {
		t.Errorf("orderTypeId[(unknown)] = %d, want the two orders with no items — a LEFT join, so an "+
			"item-less order is a bucket rather than a vanished row", got)
	}
}

func TestOrderStatsDistributionsMatchTheSeededWorld_Integration(t *testing.T) {
	w := seedOFacetWorld(t)
	ctx := context.Background()
	sel := orderSel(t)

	// The reader's reach is unitIn: oDraft (no issue date), oIssued (2024-05), oRevoked (2020-02).
	res, err := w.svc.OrderStats(ctx, w.reader, false, domain.OrderFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 3 {
		t.Fatalf("totalCount = %d, want the three in-reach orders", res.TotalCount)
	}
	status := oBuckets(t, res, "status")
	for _, c := range []struct {
		key  string
		want int64
	}{{"draft", 1}, {"issued", 1}, {"revoked", 1}} {
		if got := oCount(status, c.key); got != c.want {
			t.Errorf("status[%s] = %d, want %d", c.key, got, c.want)
		}
	}

	// The monthly histogram, and the draft backlog as the (unknown) bucket: a draft has no issue date,
	// so this bucket is the number an operator reads as "waiting to be issued".
	issued := oBuckets(t, res, "issuedOn")
	if got := oCount(issued, "2024-05"); got != 1 {
		t.Errorf("issuedOn[2024-05] = %d, want 1", got)
	}
	if got := oCount(issued, "2020-02"); got != 1 {
		t.Errorf("issuedOn[2020-02] = %d, want 1", got)
	}
	if got := oCount(issued, stats.BucketUnknown); got != 1 {
		t.Errorf("issuedOn[(unknown)] = %d, want the one draft", got)
	}

	// A filter that excludes drafts must zero the bucket rather than remove it — the chart keeps its
	// shape as the data is narrowed (the M57 ticket-2 kernel fix).
	filtered, err := w.svc.OrderStats(ctx, w.reader, false, domain.OrderFilter{IssuedOnFrom: odp("1900-01-01")}, sel)
	if err != nil {
		t.Fatalf("filtered stats: %v", err)
	}
	if got := oCount(oBuckets(t, filtered, "issuedOn"), stats.BucketUnknown); got != 0 {
		t.Errorf("with a date bound set, issuedOn[(unknown)] = %d, want a PRESENT bucket holding zero", got)
	}
}

func TestOrderStatsCountsInsideTheReachPredicate_Integration(t *testing.T) {
	w := seedOFacetWorld(t)
	ctx := context.Background()
	sel := orderSel(t)

	scoped, err := w.svc.OrderStats(ctx, w.reader, false, domain.OrderFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}
	if got := oCount(oBuckets(t, scoped, "issuingUnitId"), w.unitOut); got > 0 {
		t.Errorf("the scoped dashboard counts %d orders issued by an out-of-reach unit", got)
	}
	admin, err := w.svc.OrderStats(ctx, "", true, domain.OrderFilter{IssuingUnitID: osp(w.unitOut)}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	if admin.TotalCount < 1 {
		t.Errorf("the admin arm counts %d orders in unitOut — the assertion above proves nothing unless "+
			"the row exists", admin.TotalCount)
	}
}

func TestOrderStatsNonAdminWithNoSubjectReadsNothing_Integration(t *testing.T) {
	w := seedOFacetWorld(t)
	res, err := w.svc.OrderStats(context.Background(), "", false, domain.OrderFilter{}, orderSel(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("a non-admin with no subject got totalCount %d — it must read nothing, never the admin arm",
			res.TotalCount)
	}
}
