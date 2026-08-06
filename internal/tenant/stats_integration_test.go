// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the unit DASHBOARD aggregate (M57 / D-ObjectFacets) against a real Postgres.
//
// The differential is the same one M57's exit criterion states — totalCount equals the rows an
// exhaustive paging of the same list returns under the same filters — but the unit case carries the
// harder half of the decision: the shadow-visibility gate must be folded INTO the count.
//
// On the list, `gateUnits` trims the page AFTER it is cut. That is right for a page (a short page,
// never a skipped row) and wrong for a count, because a trimmed row has already been counted. So the
// scoped arm is a separate query, and the assertion that matters is that a shadow unit outside the
// caller's reach moves neither the total nor any bucket.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/... -run Stats
package tenant_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/tenant/application"
	"github.com/olehmushka/go-oikumenea/internal/tenant/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

func allUnitFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("unit")
	if !ok {
		t.Fatal("unit is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func unitBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func countOf(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func sumOf(bs []stats.Bucket) int64 {
	var n int64
	for _, b := range bs {
		n += b.Count
	}
	return n
}

// TestUnitStatsLevelBandsAreClickThrough is the property M57 ticket 3's levelMin/levelMax exist for,
// and it is the one a chart cannot check on its own: every band bucket of the `level` distribution
// must contain exactly the units the filter its bar links to returns.
//
// Until the range args shipped, a level bar could only be drawn, not clicked — the contract's scalar
// `level` matches ONE level and a band is a range of two. A wrong inverse here would not fail
// loudly: the operator would land on a list that quietly disagrees with the bar they clicked.
func TestUnitStatsLevelBandsAreClickThrough_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	// One unit per declared band, plus one with no level at all — the `(unknown)` bucket, which is
	// deliberately NOT click-through (every bound excludes NULLs, so the nearest filter would return
	// the bucket's complement).
	for _, lvl := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 12} {
		u := mustCreate(t, svc, org, uniqueCode(t, "lvl"))
		if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET level=$2 WHERE id=$1`, u.ID, lvl); err != nil {
			t.Fatalf("seed level: %v", err)
		}
	}
	mustCreate(t, svc, org, uniqueCode(t, "nolvl"))

	res, err := svc.UnitStats(ctx, "", true, domain.UnitFilter{OrgID: org.ID}, allUnitFacets(t))
	if err != nil {
		t.Fatalf("UnitStats: %v", err)
	}
	buckets := unitBuckets(t, res, "level")

	// The bounds the console derives from each band key (lib/ontology/buckets.ts): "6-7" is
	// levelMin=6&levelMax=7, and the open-ended "8+" clears the upper bound.
	bounds := map[string][2]*int{
		"0-1": {intp(0), intp(1)},
		"2-3": {intp(2), intp(3)},
		"4-5": {intp(4), intp(5)},
		"6-7": {intp(6), intp(7)},
		"8+":  {intp(8), nil},
	}
	var covered int64
	for key, b := range bounds {
		want := countOf(buckets, key)
		if want < 0 {
			t.Fatalf("no %q band in the level distribution", key)
		}
		covered += want
		got := allUnitIDs(t, func(tok string) (application.UnitPage, error) {
			return svc.ListUnits(ctx, domain.UnitFilter{OrgID: org.ID, LevelMin: b[0], LevelMax: b[1]}, "", nil, false, 0, tok)
		})
		if int64(len(got)) != want {
			t.Errorf("band %s: bucket counts %d units, its own filter returns %d — the bar and the list it links to disagree",
				key, want, len(got))
		}
	}

	// Every levelled unit lands in exactly one band, so the bands plus `(unknown)` account for the
	// whole filtered set — the same "every counted row lands in exactly one bucket" invariant the
	// kernel promises, checked here against the click-through bounds rather than against SQL.
	if unknown := countOf(buckets, stats.BucketUnknown); covered+unknown != res.TotalCount {
		t.Errorf("bands (%d) + unknown (%d) = %d, want totalCount %d", covered, unknown, covered+unknown, res.TotalCount)
	}
}

// TestUnitStatsTotalEqualsExhaustivePaging: the admin arm, per filter. `org` is required, so the
// world is naturally bounded and no tagging trick is needed.
func TestUnitStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	sel := allUnitFacets(t)

	public := mustCreate(t, svc, org, uniqueCode(t, "spub"))
	shadow := mustCreate(t, svc, org, uniqueCode(t, "sshd"))
	suspended := mustCreate(t, svc, org, uniqueCode(t, "ssus"))
	reference := mustCreate(t, svc, org, uniqueCode(t, "sref"))
	deep := mustCreate(t, svc, org, uniqueCode(t, "sdeep"))
	for sql, id := range map[string]string{
		`UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`: shadow.ID,
		`UPDATE oikumenea.tenant_units SET state='suspended' WHERE id=$1`:   suspended.ID,
		`UPDATE oikumenea.tenant_units SET pdp_scoped=false WHERE id=$1`:    reference.ID,
		`UPDATE oikumenea.tenant_units SET level=7 WHERE id=$1`:             deep.ID,
	} {
		if _, err := pool.Exec(ctx, sql, id); err != nil {
			t.Fatalf("seed facet column: %v", err)
		}
	}
	_ = public

	filters := []domain.UnitFilter{
		{OrgID: org.ID},
		{OrgID: org.ID, Visibility: strp("public")},
		{OrgID: org.ID, Visibility: strp("shadow")},
		{OrgID: org.ID, State: strp("active")},
		{OrgID: org.ID, State: strp("suspended")},
		{OrgID: org.ID, PDPScoped: boolp(true)},
		{OrgID: org.ID, PDPScoped: boolp(false)},
		{OrgID: org.ID, State: strp("active"), Visibility: strp("public")},
	}
	for i, f := range filters {
		paged := allUnitIDs(t, func(tok string) (application.UnitPage, error) {
			return svc.ListUnits(ctx, f, "", nil, false, 0, tok)
		})
		res, err := svc.UnitStats(ctx, "", true, f, sel)
		if err != nil {
			t.Fatalf("filter %d: stats: %v", i, err)
		}
		if got, want := res.TotalCount, int64(len(paged)); got != want {
			t.Errorf("filter %d (%+v): totalCount = %d, but exhaustively paging listUnits with the same "+
				"filters returned %d units", i, f, got, want)
		}
		// Every unit facet is a column of tenant_units, so all of them partition the candidate set.
		for _, key := range []string{"org", "domain", "unitKind", "level", "visibility", "state", "pdpScoped"} {
			if got := sumOf(unitBuckets(t, res, key)); got != res.TotalCount {
				t.Errorf("filter %d: facet %q sums to %d, want totalCount %d", i, key, got, res.TotalCount)
			}
		}
	}
}

// TestUnitStatsDistributionsMatchTheSeededOrg pins the numbers, so a self-consistent but wrong query
// cannot hide behind the sum invariant.
func TestUnitStatsDistributionsMatchTheSeededOrg_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	sel := allUnitFacets(t)

	a := mustCreate(t, svc, org, uniqueCode(t, "da"))
	b := mustCreate(t, svc, org, uniqueCode(t, "db"))
	c := mustCreate(t, svc, org, uniqueCode(t, "dc"))
	for sql, id := range map[string]string{
		`UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`: b.ID,
		`UPDATE oikumenea.tenant_units SET state='archived' WHERE id=$1`:    c.ID,
		`UPDATE oikumenea.tenant_units SET level=3 WHERE id=$1`:             a.ID,
	} {
		if _, err := pool.Exec(ctx, sql, id); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	res, err := svc.UnitStats(ctx, "", true, domain.UnitFilter{OrgID: org.ID}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	vis := unitBuckets(t, res, "visibility")
	if got := countOf(vis, "shadow"); got != 1 {
		t.Errorf("visibility[shadow] = %d, want 1", got)
	}
	if countOf(vis, "public") < 2 {
		t.Errorf("visibility[public] = %d, want the other two units", countOf(vis, "public"))
	}
	state := unitBuckets(t, res, "state")
	for _, want := range []string{"active", "suspended", "archived"} {
		if countOf(state, want) < 0 {
			t.Errorf("state distribution is missing the declared bucket %q — zero-count buckets must be "+
				"emitted so a chart keeps its shape", want)
		}
	}
	if got := countOf(state, "archived"); got != 1 {
		t.Errorf("state[archived] = %d, want 1", got)
	}
	// level 3 falls in the declared 2-3 band; the others sit at their seeded default.
	if got := countOf(unitBuckets(t, res, "level"), "2-3"); got != 1 {
		t.Errorf("level band 2-3 = %d, want the one unit at level 3", got)
	}
	// The org facet is the required scope, so it is one bucket carrying everything.
	if got := countOf(unitBuckets(t, res, "org"), org.ID); got != res.TotalCount {
		t.Errorf("org[%s] = %d, want the whole scoped total %d", org.ID, got, res.TotalCount)
	}
}

// TestUnitStatsShadowGateIsInsideTheCount is the property the separate scoped query exists for: a
// shadow unit the subject cannot reach must not be counted at all. The admin arm counts it, so the
// asymmetry proves the predicate rather than an empty world.
func TestUnitStatsShadowGateIsInsideTheCount_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	sel := allUnitFacets(t)

	open := mustCreate(t, svc, org, uniqueCode(t, "gopen"))
	hidden := mustCreate(t, svc, org, uniqueCode(t, "ghid"))
	if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`, hidden.ID); err != nil {
		t.Fatalf("seed shadow: %v", err)
	}
	_ = open

	// A subject with NO grants anywhere: their reach is empty, so every shadow unit is invisible and
	// every public one is not.
	stranger := seedStatsPerson(t, pool)

	admin, err := svc.UnitStats(ctx, "", true, domain.UnitFilter{OrgID: org.ID}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	scoped, err := svc.UnitStats(ctx, stranger, false, domain.UnitFilter{OrgID: org.ID}, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}

	if got := countOf(unitBuckets(t, admin, "visibility"), "shadow"); got != 1 {
		t.Errorf("admin visibility[shadow] = %d, want the hidden unit — otherwise this test proves "+
			"nothing about the scoped arm", got)
	}
	if got := countOf(unitBuckets(t, scoped, "visibility"), "shadow"); got != 0 {
		t.Errorf("scoped visibility[shadow] = %d, want 0: an unreachable shadow unit must never be "+
			"counted, only trimmed-after-the-fact on a list", got)
	}
	if got, want := scoped.TotalCount, admin.TotalCount-1; got != want {
		t.Errorf("scoped totalCount = %d, want %d (the admin total minus the unreachable shadow unit)", got, want)
	}
	// …and the public unit is still counted, so the gate narrows rather than blanking the dashboard.
	if scoped.TotalCount < 1 {
		t.Errorf("scoped totalCount = %d, want the public units to remain visible", scoped.TotalCount)
	}
}

// seedStatsPerson inserts a bare person to act as a grantless subject.
func seedStatsPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('stats stranger') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}
	return id
}

// TestUnitStatsNonAdminWithNoSubjectReadsNothing: an empty subject means the ADMIN arm in the SQL, so a
// caller who is neither an admin nor an identified person (a machine principal — pep.SubjectAuthority
// returns ("", false) for one) must read NOTHING rather than every unit in the org. pkg/stats.Compute
// owns the rule; this proves tenant is wired through it, which it was NOT before M57 ticket 2 — the
// transport used to collapse the two cases into one argument.
func TestUnitStatsNonAdminWithNoSubjectReadsNothing_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)
	mustCreate(t, svc, org, uniqueCode(t, "armconv"))

	res, err := svc.UnitStats(ctx, "", false, domain.UnitFilter{OrgID: org.ID}, allUnitFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("a non-admin with no subject got totalCount %d and %d distributions — it must read "+
			"nothing, never the admin arm", res.TotalCount, len(res.Distributions))
	}
}
