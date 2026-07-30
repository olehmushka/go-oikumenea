// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Scale measurement for the three M57 ticket-2 dashboards — memberships, orders, documents — against
// the synthetic national-scale world (10^6 memberships / 2x10^5 orders / 6x10^5 documents, seeded by
// scripts/seed-scale -enrich). It reuses the ticket-3 three-service harness in
// facet_scale_integration_test.go, which already builds all three services over one pool.
//
// The question here is NOT the same as the lists'. A list pays for a page, so its cost is dominated by
// whether the LIMIT can terminate early — which is why every scoped list ships two reach plan shapes
// and dispatches between them. An aggregate has no LIMIT, so the materialized reach set should win at
// every reach size and one scoped query per module should be enough. Ticket 1 established that on
// person; this ticket re-measured it per table before relying on it, including for DOCUMENTS, whose
// list was the case where the set form collapsed (6 419 ms at root reach, because reach runs through
// the holder). Numbers in docs/architecture/review-2026-07.md § Measurements.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestTicket2Stats -v ./internal/membership/
package membership_test

import (
	"context"
	"testing"
	"time"

	membershipdomain "github.com/olegamysk/go-oikumenea/internal/membership/domain"
	orderdomain "github.com/olegamysk/go-oikumenea/internal/order/domain"

	documentdomain "github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// ticket2StatsBudget is the per-request budget for a whole-register dashboard at this scale. A
// dashboard aggregates every row the filter admits, so it is legitimately slower than a first page;
// this budget separates "an aggregate over a million rows" from "a plan regression".
const ticket2StatsBudget = 25 * time.Second

func scaleSel(t *testing.T, objectType, csv string) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get(objectType)
	if !ok {
		t.Fatalf("%s is not registered", objectType)
	}
	sel, err := stats.Select(o, csv, nil)
	if err != nil {
		t.Fatalf("Select(%q): %v", csv, err)
	}
	return sel
}

// TestTicket2StatsScale measures each of the three dashboards at admin reach and at all three probe
// subjects, all facets and count-only.
func TestTicket2StatsScale(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	ctx := context.Background()
	unit := ticket3Unit(t, pool)

	memberships := ticket3Membership(pool)
	orders := ticket3Order(pool)
	documents := ticket3Document(t, pool)

	subjects := []string{"scale-leaf-subject", "scale-mid-subject", "scale-root-subject"}

	type probe struct {
		name string
		run  func(subject string, isAdmin bool, all bool) (stats.Result, error)
	}
	probes := []probe{
		{"memberships", func(subject string, isAdmin, all bool) (stats.Result, error) {
			csv := ""
			if !all {
				csv = ","
			}
			return memberships.MembershipStats(ctx, subject, isAdmin,
				membershipdomain.MembershipFilter{}, scaleSel(t, "link__member_of", csv))
		}},
		{"orders", func(subject string, isAdmin, all bool) (stats.Result, error) {
			csv := ""
			if !all {
				csv = ","
			}
			return orders.OrderStats(ctx, subject, isAdmin,
				orderdomain.OrderFilter{}, scaleSel(t, "order", csv))
		}},
		{"documents", func(subject string, isAdmin, all bool) (stats.Result, error) {
			csv := ""
			if !all {
				csv = ","
			}
			return documents.DocumentStats(ctx, subject, isAdmin,
				documentdomain.DocumentFilter{}, scaleSel(t, "document", csv))
		}},
	}

	for _, p := range probes {
		for _, all := range []bool{true, false} {
			label := "all facets"
			if !all {
				label = "count only"
			}
			t.Run(p.name+"/admin/"+label, func(t *testing.T) {
				start := time.Now()
				res, err := p.run("", true, all)
				took := time.Since(start)
				if err != nil {
					t.Fatalf("%s admin stats: %v", p.name, err)
				}
				t.Logf("%-12s admin      %-11s %8.1f ms  (total %d)", p.name, label,
					float64(took.Microseconds())/1000, res.TotalCount)
				if took > ticket2StatsBudget {
					t.Errorf("dashboard took %s, budget %s", took, ticket2StatsBudget)
				}
			})
			for _, code := range subjects {
				id := ticket3Subject(t, pool, code)
				t.Run(p.name+"/"+code+"/"+label, func(t *testing.T) {
					start := time.Now()
					res, err := p.run(id, false, all)
					took := time.Since(start)
					if err != nil {
						t.Fatalf("%s scoped stats: %v", p.name, err)
					}
					t.Logf("%-12s %-18s %-11s %8.1f ms  (total %d)", p.name, code, label,
						float64(took.Microseconds())/1000, res.TotalCount)
					if took > ticket2StatsBudget {
						t.Errorf("dashboard took %s, budget %s", took, ticket2StatsBudget)
					}
				})
			}
		}
	}
	_ = unit
}

// TestTicket2StatsFilteredScale measures the filtered shapes an operator actually clicks into from a
// chart segment — the click-to-filter round trip ticket 3 will wire up.
func TestTicket2StatsFilteredScale(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	ctx := context.Background()
	unit := ticket3Unit(t, pool)
	mid := ticket3Subject(t, pool, "scale-mid-subject")

	memberships := ticket3Membership(pool)
	orders := ticket3Order(pool)
	documents := ticket3Document(t, pool)

	cases := []struct {
		name string
		run  func() (stats.Result, error)
	}{
		{"memberships status=ended (admin)", func() (stats.Result, error) {
			s := "ended"
			return memberships.MembershipStats(ctx, "", true,
				membershipdomain.MembershipFilter{Status: &s}, scaleSel(t, "link__member_of", ""))
		}},
		{"memberships unitId (mid reach)", func() (stats.Result, error) {
			return memberships.MembershipStats(ctx, mid, false,
				membershipdomain.MembershipFilter{UnitID: &unit}, scaleSel(t, "link__member_of", ""))
		}},
		{"orders status=draft (admin)", func() (stats.Result, error) {
			s := "draft"
			return orders.OrderStats(ctx, "", true,
				orderdomain.OrderFilter{Status: &s}, scaleSel(t, "order", ""))
		}},
		{"documents status=active (mid reach)", func() (stats.Result, error) {
			s := "active"
			return documents.DocumentStats(ctx, mid, false,
				documentdomain.DocumentFilter{Status: &s}, scaleSel(t, "document", ""))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start := time.Now()
			res, err := c.run()
			took := time.Since(start)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			t.Logf("%-38s %8.1f ms  (total %d)", c.name, float64(took.Microseconds())/1000, res.TotalCount)
			if took > ticket2StatsBudget {
				t.Errorf("took %s, budget %s", took, ticket2StatsBudget)
			}
		})
	}
}

// TestTicket2StatsDashboardAsDrawn is the number that actually matters, and it is deliberately separate
// from the all-facets sweep above.
//
// "All facets" is a worst case nobody requests: `link__member_of` declares personId as a FILTER, and a
// top-N over 10^6 distinct persons costs 8.6 s on its own — the window function must rank a million
// groups to keep fifteen. facets.md's membership dashboard draws status tiles, the monthly intake curve
// and the billet-fill split; it never draws "top persons". So this measures the facet set the console
// will ask for, which is what the `facets` CSV exists to let it do.
func TestTicket2StatsDashboardAsDrawn(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	svc := ticket3Membership(pool)
	ctx := context.Background()
	sel := scaleSel(t, "link__member_of", "status,effectiveFrom,positionId")

	for _, c := range []struct {
		name    string
		subject string
		admin   bool
	}{
		{"admin", "", true},
		{"root reach", ticket3Subject(t, pool, "scale-root-subject"), false},
		{"mid reach", ticket3Subject(t, pool, "scale-mid-subject"), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			start := time.Now()
			res, err := svc.MembershipStats(ctx, c.subject, c.admin, membershipdomain.MembershipFilter{}, sel)
			took := time.Since(start)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			t.Logf("dashboard as drawn %-12s %8.1f ms  (total %d)", c.name,
				float64(took.Microseconds())/1000, res.TotalCount)
			if took > ticket2StatsBudget {
				t.Errorf("took %s, budget %s", took, ticket2StatsBudget)
			}
		})
	}
}
