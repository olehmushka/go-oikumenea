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
	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
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
		res, err := svc.UnitStats(ctx, "", f, sel)
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

	res, err := svc.UnitStats(ctx, "", domain.UnitFilter{OrgID: org.ID}, sel)
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

	admin, err := svc.UnitStats(ctx, "", domain.UnitFilter{OrgID: org.ID}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	scoped, err := svc.UnitStats(ctx, stranger, domain.UnitFilter{OrgID: org.ID}, sel)
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
