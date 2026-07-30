// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the membership-roster DASHBOARD aggregate (M57 ticket 2 / D-ObjectFacets).
//
// The contract is M57's exit criterion, asserted on both arms:
//
//	for a fixed subject and a fixed filter set, stats.totalCount equals the number of rows obtained
//	by exhaustively paging the SAME list endpoint with the SAME filters.
//
// Two things are specific to this type. First, the top-level list carries NO implicit status filter,
// so the total and the status distribution must agree — the reason that default was refused in M56
// ticket 3 was precisely to keep this dashboard honest, and here it is checked rather than assumed.
// Second, every membership facet is a per-ROW attribute, so ALL of them partition the candidate set;
// there is no multiplying facet to exempt.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/membership/... -run Stats
package membership_test

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// statsSel is the selection a dashboard makes by default: every facet the caller may read.
func statsSel(t *testing.T, objectType string) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get(objectType)
	if !ok {
		t.Fatalf("%s is not registered in the facet catalog", objectType)
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func bucketsFor(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func countFor(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func sumFor(bs []stats.Bucket) int64 {
	var n int64
	for _, b := range bs {
		n += b.Count
	}
	return n
}

// TestMembershipStatsTotalEqualsExhaustivePaging is the exit criterion. The scoped arm needs no
// bounding (the reader's reach IS the world); the admin arm is bounded to the world's unit, for the
// reason given at the call site — the shared test database moves under an unbounded comparison.
func TestMembershipStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()
	sel := statsSel(t, "link__member_of")

	filters := []domain.MembershipFilter{
		{},
		{Status: msp("active")},
		{Status: msp("ended")},
		{UnitID: msp(w.unitIn)},
		{PersonID: msp(w.personA)},
		{PositionID: msp(w.positionA)},
		{EffectiveFromAfter: mdp("2023-01-01")},
		{EffectiveFromBefore: mdp("2021-01-01")},
		{EffectiveFromAfter: mdp("2024-03-04"), EffectiveFromBefore: mdp("2024-03-04")}, // one calendar day
		{UnitID: msp(w.unitIn), Status: msp("active")},
	}
	for i, f := range filters {
		scopedStats, err := w.svc.MembershipStats(ctx, w.reader, false, f, sel)
		if err != nil {
			t.Fatalf("filter %d: scoped stats: %v", i, err)
		}
		if got, want := scopedStats.TotalCount, int64(len(w.scoped(t, f))); got != want {
			t.Errorf("filter %d (%+v): scoped totalCount = %d, but exhaustively paging the scoped list "+
				"returned %d rows", i, f, got, want)
		}
		// The admin arm is bounded to this world's unit. Not cosmetics: the test database is SHARED, and
		// other suites in this package insert memberships while these two measurements run, so an
		// unbounded admin comparison fails on a concurrent insert rather than on drift between the
		// aggregate and the list. The differential holds over ANY predicate, so bounding it costs
		// nothing — and the predicate block being compared is the same block either way.
		af := f
		if af.UnitID == nil {
			af.UnitID = msp(w.unitIn)
		}
		adminStats, err := w.svc.MembershipStats(ctx, "", true, af, sel)
		if err != nil {
			t.Fatalf("filter %d: admin stats: %v", i, err)
		}
		if got, want := adminStats.TotalCount, int64(len(w.admin(t, af))); got != want {
			t.Errorf("filter %d (%+v): admin totalCount = %d, but exhaustively paging the admin list "+
				"returned %d rows", i, f, got, want)
		}
	}
}

// TestMembershipStatsBucketsSumToTotal: every membership facet is a per-row attribute, so each one
// partitions the candidate set exactly — including the nullable positionId, whose (unknown) bucket is
// the memberships with no billet.
func TestMembershipStatsBucketsSumToTotal_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()
	sel := statsSel(t, "link__member_of")

	res, err := w.svc.MembershipStats(ctx, w.reader, false, domain.MembershipFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, key := range []string{"unitId", "personId", "positionId", "status", "effectiveFrom"} {
		if got := sumFor(bucketsFor(t, res, key)); got != res.TotalCount {
			t.Errorf("facet %q sums to %d, want totalCount %d — a counted row fell out of every bucket "+
				"(or was counted twice)", key, got, res.TotalCount)
		}
	}
}

// TestMembershipStatsDistributionsMatchTheSeededWorld pins the numbers, so a query that is
// self-consistent but wrong cannot hide behind the sum invariant.
func TestMembershipStatsDistributionsMatchTheSeededWorld_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()
	sel := statsSel(t, "link__member_of")

	// The reader's reach is unitIn: mActive (active, 2024-03), mEnded (ended, 2020-01),
	// mWithPos (active, WITH positionA, 2024-03).
	res, err := w.svc.MembershipStats(ctx, w.reader, false, domain.MembershipFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 3 {
		t.Fatalf("totalCount = %d, want the three in-reach memberships", res.TotalCount)
	}

	// The status distribution agrees with the total — the property the "no implicit status default"
	// decision exists to protect. An active-only default would have made these two disagree.
	status := bucketsFor(t, res, "status")
	if got := countFor(status, "active"); got != 2 {
		t.Errorf("status[active] = %d, want 2", got)
	}
	if got := countFor(status, "ended"); got != 1 {
		t.Errorf("status[ended] = %d, want 1 — an ended row is reachable only through this surface", got)
	}

	// positionId: one billet-filling membership, two without — the vacancy signal.
	pos := bucketsFor(t, res, "positionId")
	if got := countFor(pos, w.positionA); got != 1 {
		t.Errorf("positionId[%s] = %d, want 1", w.positionA, got)
	}
	if got := countFor(pos, stats.BucketUnknown); got != 2 {
		t.Errorf("positionId[(unknown)] = %d, want the two memberships with no billet", got)
	}

	// Monthly intake histogram: two joins in 2024-03, one in 2020-01, ascending.
	eff := bucketsFor(t, res, "effectiveFrom")
	if got := countFor(eff, "2024-03"); got != 2 {
		t.Errorf("effectiveFrom[2024-03] = %d, want 2", got)
	}
	if got := countFor(eff, "2020-01"); got != 1 {
		t.Errorf("effectiveFrom[2020-01] = %d, want 1", got)
	}
	if len(eff) >= 2 && eff[0].Key > eff[1].Key {
		t.Errorf("month buckets are not ascending: %v", eff)
	}

	// unitId is EXACT here (not subtree-expanding, unlike person.unitId), so every row is in unitIn.
	if got := countFor(bucketsFor(t, res, "unitId"), w.unitIn); got != 3 {
		t.Errorf("unitId[%s] = %d, want all three", w.unitIn, got)
	}
}

// TestMembershipStatsCountsInsideTheReachPredicate: the reader must not be told how many memberships
// exist outside their reach. The admin arm counts the out-of-reach row; the scoped arm must not.
func TestMembershipStatsCountsInsideTheReachPredicate_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()
	sel := statsSel(t, "link__member_of")

	scoped, err := w.svc.MembershipStats(ctx, w.reader, false, domain.MembershipFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}
	if got := countFor(bucketsFor(t, scoped, "unitId"), w.unitOut); got > 0 {
		t.Errorf("the scoped dashboard counts %d memberships in an out-of-reach unit — the count is "+
			"being taken outside the visibility predicate", got)
	}
	if got := countFor(bucketsFor(t, scoped, "personId"), w.personOut); got > 0 {
		t.Errorf("the scoped dashboard counts an out-of-reach person (%d rows)", got)
	}

	admin, err := w.svc.MembershipStats(ctx, "", true, domain.MembershipFilter{UnitID: msp(w.unitOut)}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	if admin.TotalCount < 1 {
		t.Errorf("the admin arm counts %d memberships in unitOut — this test proves nothing about the "+
			"scoped arm unless the row really exists", admin.TotalCount)
	}
}

// TestMembershipStatsSelectionPushesDown: the `facets` CSV selects which branches run at all, and the
// count-only request is the cheap "how many".
func TestMembershipStatsSelectionPushesDown_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()
	o, _ := facet.Default.Get("link__member_of")

	sel, err := stats.Select(o, "status", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	res, err := w.svc.MembershipStats(ctx, w.reader, false, domain.MembershipFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(res.Distributions) != 1 || res.Distributions[0].Facet != "status" {
		t.Fatalf("distributions = %+v, want [status] alone", res.Distributions)
	}
	if res.TotalCount != 3 {
		t.Errorf("totalCount = %d with a narrowed selection, want the whole visible set", res.TotalCount)
	}

	none, err := stats.Select(o, ",", nil)
	if err != nil {
		t.Fatalf("Select(none): %v", err)
	}
	res, err = w.svc.MembershipStats(ctx, w.reader, false, domain.MembershipFilter{}, none)
	if err != nil {
		t.Fatalf("stats(none): %v", err)
	}
	if len(res.Distributions) != 0 || res.TotalCount != 3 {
		t.Errorf("count-only request returned %d distributions and total %d, want 0 and 3",
			len(res.Distributions), res.TotalCount)
	}
}

// TestMembershipStatsNonAdminWithNoSubjectReadsNothing is the arm-convention guard at the service
// layer: an empty subject means the ADMIN arm in the SQL, so a caller who is neither an admin nor an
// identified person (a machine principal — pep.SubjectAuthority returns ("", false) for one) must get
// nothing rather than the whole instance. pkg/stats owns the rule; this proves the module is wired
// through it.
func TestMembershipStatsNonAdminWithNoSubjectReadsNothing_Integration(t *testing.T) {
	w := seedMFacetWorld(t)
	res, err := w.svc.MembershipStats(context.Background(), "", false, domain.MembershipFilter{}, statsSel(t, "link__member_of"))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("a non-admin with no subject got totalCount %d and %d distributions — it must read "+
			"nothing, never the admin arm", res.TotalCount, len(res.Distributions))
	}
}

// compile-time assurance that the page type is still what the differential pages through.
var _ = func(p application.MembershipPage) int { return len(p.Memberships) }
