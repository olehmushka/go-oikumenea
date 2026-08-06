// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Scale measurement for the person DASHBOARD aggregate (M57 / D-ObjectFacets), against the synthetic
// national-scale world seeded by scripts/seed-scale (10^6 persons / 10^5 units).
//
// What is being measured, and why it is a different question from the list's:
//
// A list pays for a PAGE. Its cost is dominated by whether the LIMIT can terminate early, which is
// why M56 ticket 3 had to ship two reach plan shapes per scoped endpoint and dispatch between them.
// An aggregate pays for the WHOLE candidate set — there is no LIMIT to terminate early, and so no
// reach cardinality at which the set form's "materialize the reach and hash-probe it" turns into the
// wrong plan. That is the reasoning behind shipping ONE scoped stats query rather than a
// sparse/dense pair, and it is a claim about plans, so it is measured here rather than asserted.
//
// The numbers land in docs/architecture/review-2026-07.md § Measurements. The budget is loose on
// purpose: this harness exists to catch a plan regression (an aggregate that starts scanning
// something it should not), not to police shared developer hardware.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestStatsScale -v ./internal/person/
package person_test

import (
	"context"
	"testing"
	"time"

	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// statsBudget is the per-request budget for a whole-directory dashboard at 10^6 persons. A dashboard
// aggregates every row the filter admits, so it is legitimately slower than a first page; the budget
// is what separates "an aggregate over a million rows" from "a plan regression".
const statsBudget = 20 * time.Second

// TestStatsScaleAdminArm measures the instance-admin dashboard — the widest possible aggregate,
// every person in the world, every facet.
func TestStatsScaleAdminArm(t *testing.T) {
	pool := scalePool(t)
	requireEnriched(t, pool)
	svc := newServiceOn(t, pool)
	ctx := context.Background()
	unit := scaleFilterUnit(t, pool)
	o, _ := facet.Default.Get("person")
	all, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	cases := []struct {
		name string
		f    domain.PersonFilter
		sel  stats.Selection
	}{
		{"all facets, unfiltered", domain.PersonFilter{}, all},
		{"all facets, sex=female", domain.PersonFilter{Sex: sp("female")}, all},
		{"all facets, unitId subtree", domain.PersonFilter{UnitID: sp(unit)}, all},
		{"count only", domain.PersonFilter{}, mustSelect(t, o, ",")},
		{"one enum facet", domain.PersonFilter{}, mustSelect(t, o, "sex")},
		{"one ref facet (top-N over 10^5 units)", domain.PersonFilter{}, mustSelect(t, o, "unitId")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			res, err := svc.PersonStats(ctx, "", true, tc.f, tc.sel)
			took := time.Since(start)
			if err != nil {
				t.Fatalf("PersonStats: %v", err)
			}
			t.Logf("admin %-40s %8.1f ms  (total %d, %d facets)", tc.name,
				float64(took.Microseconds())/1000, res.TotalCount, len(res.Distributions))
			if took > statsBudget {
				t.Errorf("dashboard took %s, budget %s", took, statsBudget)
			}
		})
	}
}

// TestStatsScaleScopedArm measures the read-scope dashboard at all three reach sizes. This is the
// measurement that decides whether ONE scoped query is enough: if the set form degraded at root
// reach the way the LIST's did, the root-subject row here would be the one that shows it.
func TestStatsScaleScopedArm(t *testing.T) {
	pool := scalePool(t)
	requireEnriched(t, pool)
	svc := newServiceOn(t, pool)
	bindMembershipOn(t, svc, pool)
	ctx := context.Background()
	o, _ := facet.Default.Get("person")
	all, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	for _, subject := range []string{"scale-leaf-subject", "scale-mid-subject", "scale-root-subject"} {
		id := scaleProbeSubject(t, pool, subject)
		for _, tc := range []struct {
			name string
			f    domain.PersonFilter
			sel  stats.Selection
		}{
			{"all facets", domain.PersonFilter{}, all},
			{"all facets, sex=female", domain.PersonFilter{Sex: sp("female")}, all},
			{"count only", domain.PersonFilter{}, mustSelect(t, o, ",")},
		} {
			t.Run(subject+"/"+tc.name, func(t *testing.T) {
				start := time.Now()
				res, err := svc.PersonStats(ctx, id, false, tc.f, tc.sel)
				took := time.Since(start)
				if err != nil {
					t.Fatalf("PersonStats: %v", err)
				}
				t.Logf("%-20s %-26s %8.1f ms  (total %d)", subject, tc.name,
					float64(took.Microseconds())/1000, res.TotalCount)
				if took > statsBudget {
					t.Errorf("dashboard took %s, budget %s", took, statsBudget)
				}
			})
		}
	}
}

func mustSelect(t *testing.T, o facet.ObjectType, csv string) stats.Selection {
	t.Helper()
	sel, err := stats.Select(o, csv, nil)
	if err != nil {
		t.Fatalf("Select(%q): %v", csv, err)
	}
	return sel
}
