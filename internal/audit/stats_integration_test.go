// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the LEDGER dashboard aggregate (M58 ticket 1 / D-ObjectFacets).
//
// The differential is M57's exit criterion, restated for a type whose visibility comes from row-level
// security rather than a folded-in reach predicate: `totalCount` equals the rows an exhaustive paging
// of the same list returns under the same filters. What is specific to audit — and what these tests
// are really for — is the DAY GRAIN and its click-through: `since`/`until` are DATETIMES, so a day
// bucket's filter is that day's two RFC-3339 endpoints, and an off-by-one-millisecond inverse is
// invisible on a chart and wrong in the list.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/audit/... -run Stats
package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func allAuditFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("audit")
	if !ok {
		t.Fatal("audit is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func auditBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func bucketCount(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

// pageAll counts the rows an exhaustive paging of Query returns under the same filter — the right
// side of every differential below.
func pageAll(t *testing.T, svc *application.Service, p application.QueryParams) int {
	t.Helper()
	ctx := context.Background()
	n, token := 0, ""
	for i := 0; i < 100; i++ {
		p.PageSize, p.PageToken = 50, token
		page, err := svc.Query(ctx, p)
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		n += len(page.Entries)
		if page.NextPageToken == "" {
			return n
		}
		token = page.NextPageToken
	}
	t.Fatal("paging did not terminate")
	return 0
}

// seedLedger writes a small, self-identifying world: one actor, three outcomes, two actions, spread
// over three days. Every assertion filters on this actor, so the shared test database's accumulated
// rows cannot move a count (the accumulation flake that bites shared-DB tests).
func seedLedger(t *testing.T, svc *application.Service) (actorID string, days []string) {
	t.Helper()
	ctx := context.Background()
	_, pool := newService(t)
	actorID = uuid.NewString()

	now := time.Now().UTC()
	spread := []struct {
		daysAgo int
		action  string
		outcome domain.Outcome
		n       int
	}{
		{2, "assignment.grant", domain.OutcomeSuccess, 3},
		{2, "assignment.revoke", domain.OutcomeDenied, 1},
		{1, "assignment.grant", domain.OutcomeSuccess, 2},
		{0, "assignment.grant", domain.OutcomeError, 1},
	}
	seen := map[string]bool{}
	for _, s := range spread {
		day := now.AddDate(0, 0, -s.daysAgo)
		for i := 0; i < s.n; i++ {
			// Inserted directly, with created_at SET rather than defaulted: the ledger is append-only
			// (reject_mutation() guards UPDATE), so a row cannot be backdated afterwards — and the
			// point of these tests is a MULTI-DAY axis, which a world seeded entirely at now() would
			// make vacuous. Everything else about the row is what Record would have written.
			if _, err := pool.Exec(ctx, `
				INSERT INTO oikumenea.audit_log
					(id, created_at, actor_type, actor_person_id, action, target_type, request_id, outcome)
				VALUES (oikumenea.new_id(3, 3, 0), $1, 'person', $2, $3, 'role_assignment', $4, $5)`,
				day, actorID, s.action, "req-"+uuid.NewString(), string(s.outcome)); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
		if !seen[day.Format("2006-01-02")] {
			seen[day.Format("2006-01-02")] = true
			days = append(days, day.Format("2006-01-02"))
		}
	}
	return actorID, days
}

// TestAuditStatsTotalEqualsExhaustivePaging is the contract M57 states and M58 inherits: the
// dashboard and the list describe one world.
func TestAuditStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	actor, _ := seedLedger(t, svc)

	grant, denied := "assignment.grant", domain.OutcomeDenied
	for _, tc := range []struct {
		name string
		p    application.QueryParams
	}{
		{"actor only", application.QueryParams{ActorPersonID: &actor}},
		{"actor + action", application.QueryParams{ActorPersonID: &actor, Action: &grant}},
		{"actor + outcome", application.QueryParams{ActorPersonID: &actor, Outcome: &denied}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Stats(ctx, tc.p, allAuditFacets(t))
			if err != nil {
				t.Fatalf("Stats: %v", err)
			}
			if got := pageAll(t, svc, tc.p); int64(got) != res.TotalCount {
				t.Errorf("totalCount %d, exhaustive paging returns %d", res.TotalCount, got)
			}
		})
	}
}

// TestAuditStatsFacetsSumToTotal: every counted row lands in exactly one bucket. `targetId` is the
// only facet that could legitimately differ (a NULL target is its own bucket, which IS counted), so
// all of them are checked.
func TestAuditStatsFacetsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	actor, _ := seedLedger(t, svc)

	res, err := svc.Stats(ctx, application.QueryParams{ActorPersonID: &actor}, allAuditFacets(t))
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	for _, d := range res.Distributions {
		var sum int64
		for _, b := range d.Buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("facet %q sums to %d, totalCount is %d", d.Facet, sum, res.TotalCount)
		}
	}
}

// TestAuditStatsDayBucketsAreClickThrough is the property the day grain exists for, and the one a
// chart cannot check on its own: each day bucket must contain exactly the rows the filter its bar
// links to returns. The bounds are the console's own (lib/ontology/buckets: start-of-day →
// end-of-day, INCLUSIVE), so an off-by-one at either edge shows up here rather than in production as
// a bar that navigates to the wrong day.
func TestAuditStatsDayBucketsAreClickThrough_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	actor, days := seedLedger(t, svc)

	res, err := svc.Stats(ctx, application.QueryParams{ActorPersonID: &actor}, allAuditFacets(t))
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	buckets := auditBuckets(t, res, "createdAt")
	if len(days) < 2 {
		t.Fatalf("seed produced %d distinct days — the multi-day axis is what this test is about", len(days))
	}

	var covered int64
	for _, day := range days {
		want := bucketCount(buckets, day)
		if want <= 0 {
			t.Errorf("day %s has no bucket in the distribution (%v)", day, buckets)
			continue
		}
		covered += want
		since, err := time.Parse(time.RFC3339, day+"T00:00:00.000Z")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		until, err := time.Parse(time.RFC3339, day+"T23:59:59.999Z")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		got := pageAll(t, svc, application.QueryParams{ActorPersonID: &actor, Since: &since, Until: &until})
		if int64(got) != want {
			t.Errorf("day %s: bucket counts %d, its own filter returns %d — the bar and the list it "+
				"links to disagree", day, want, got)
		}
	}
	if covered != res.TotalCount {
		t.Errorf("the seeded days cover %d of %d rows — a bucket fell outside its own day", covered, res.TotalCount)
	}
}

// TestAuditStatsSelectionIsHonoured: `facets=` returns the count alone, an undeclared key is an
// error, and an unselected facet is never grouped (the want_* flags, not a post-filter).
func TestAuditStatsSelection_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	actor, _ := seedLedger(t, svc)
	o, _ := facet.Default.Get("audit")

	countOnly, err := stats.Select(o, ",", nil)
	if err != nil {
		t.Fatalf("Select(count only): %v", err)
	}
	res, err := svc.Stats(ctx, application.QueryParams{ActorPersonID: &actor}, countOnly)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(res.Distributions) != 0 {
		t.Errorf("facets=, returned %d distributions, want the total alone", len(res.Distributions))
	}
	if res.TotalCount == 0 {
		t.Error("facets=, returned no total — the count is the one thing it must carry")
	}

	one, err := stats.Select(o, "outcome", nil)
	if err != nil {
		t.Fatalf("Select(outcome): %v", err)
	}
	res, err = svc.Stats(ctx, application.QueryParams{ActorPersonID: &actor}, one)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(res.Distributions) != 1 || res.Distributions[0].Facet != "outcome" {
		t.Errorf("facets=outcome returned %+v", res.Distributions)
	}
	if _, err := stats.Select(o, "nonesuch", nil); err == nil {
		t.Error("an undeclared facet key must be an error, not a silent empty chart")
	}
}
