// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the membership `org` facet (M59 / D-ObjectFacets) — the arg that made M57's
// headcount-by-unit chart drawable, and the first facet in the vocabulary whose value lives on a
// table the listed module does not own and cannot read directly.
//
// Three things are specific to it, and each is a way it could be wrong while looking right:
//
//  1. It must SELECT, not merely parse. A cross-table predicate that matched everything would leave
//     the org-scoped dashboard showing units from every organization — the exact dishonesty M57
//     refused to ship — so the world here has two organizations, both INSIDE the reader's reach, and
//     every assertion is that the other org's rows are excluded.
//
//  2. It must behave IDENTICALLY on the admin and reach-scoped arms, like every other facet
//     (scoped(f) == admin(f) ∩ reach).
//
//  3. It must not be silently emptied by RLS. The org lives on tenant_units, which is RLS-FORCED;
//     the predicate therefore reads the RLS-EXEMPT authz_unit_org projection instead. On a bare pool
//     a tenant_units semi-join answers a confident ZERO to an entitled caller (the trap M58 ticket 7
//     recorded), and these tests run on exactly such a pool — so a regression to tenant_units turns
//     every assertion below red rather than passing quietly.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/membership/... -run OrgFacet
package membership_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
)

// mOrgWorld is two organizations, each with one unit holding one membership, and a reader whose
// reach covers BOTH units. Reach is deliberately not the discriminator here: if the org filter did
// nothing, every assertion would still see both rows, which is what makes them meaningful.
type mOrgWorld struct {
	svc  *application.Service
	pool *pgxpool.Pool

	orgA, orgB       string
	unitA, unitB     string
	memberA, memberB string
	reader           string
}

func seedMOrgWorld(t *testing.T) *mOrgWorld {
	t.Helper()
	svc, pool := newService(t)
	w := &mOrgWorld{svc: svc, pool: pool}

	w.orgA, w.unitA = seedOrgWithUnit(t, pool, "m59-org-a")
	w.orgB, w.unitB = seedOrgWithUnit(t, pool, "m59-org-b")

	w.memberA = seedFacetMembership(t, pool, seedPerson(t, pool), w.unitA, "", "active", "2024-03-04")
	w.memberB = seedFacetMembership(t, pool, seedPerson(t, pool), w.unitB, "", "active", "2024-03-04")

	w.reader = seedPerson(t, pool)
	seedMembershipReadGrant(t, pool, w.reader, w.unitA)
	seedMembershipReadGrant(t, pool, w.reader, w.unitB)
	return w
}

// seedOrgWithUnit creates its OWN organization (the shared test-org is not enough — this suite needs
// two) plus one unit in it, and returns both RIDs. The authz_unit_org projection is written by the
// BEFORE INSERT trigger on tenant_units, so nothing here maintains it by hand: if that trigger ever
// stopped firing, these tests would go red, which is the right place to find out.
func seedOrgWithUnit(t *testing.T, pool *pgxpool.Pool, codePrefix string) (orgID, unitID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, ensureOrgSQL); err != nil {
		t.Fatalf("ensure domain: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
		 SELECT $1, 'M59 Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='test-domain'
		 RETURNING id`, code(t, codePrefix)).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT $1, o.domain_id, $2, 'M59 Unit' FROM oikumenea.tenant_organizations o WHERE o.id = $1
		 RETURNING id`, orgID, code(t, codePrefix+"-unit")).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return orgID, unitID
}

// TestOrgFacetSelectsOnBothListArms is the facet's core claim on the list: filtering by an
// organization returns that org's memberships and NOT the other's, identically for an admin and for
// a subject whose reach covers both.
func TestOrgFacetSelectsOnBothListArms_Integration(t *testing.T) {
	w := seedMOrgWorld(t)
	ctx := context.Background()

	for _, arm := range []struct {
		name string
		list func(f domain.MembershipFilter, token string) (application.MembershipPage, error)
	}{
		{"admin", func(f domain.MembershipFilter, token string) (application.MembershipPage, error) {
			return w.svc.ListMemberships(ctx, f, 50, token)
		}},
		{"scoped", func(f domain.MembershipFilter, token string) (application.MembershipPage, error) {
			return w.svc.ListVisibleMemberships(ctx, w.reader, f, 50, token)
		}},
	} {
		t.Run(arm.name, func(t *testing.T) {
			got := mAllIDs(t, func(token string) (application.MembershipPage, error) {
				return arm.list(domain.MembershipFilter{OrgID: msp(w.orgA)}, token)
			})
			if !got[w.memberA] {
				t.Errorf("org=A does not return A's membership — the predicate excludes what it should keep")
			}
			if got[w.memberB] {
				t.Errorf("org=A returns B's membership — the predicate does not discriminate, so an " +
					"org-scoped dashboard would mix organizations")
			}

			// The other direction, from the same world: a filter that returned nothing at all would
			// also have passed the assertions above.
			gotB := mAllIDs(t, func(token string) (application.MembershipPage, error) {
				return arm.list(domain.MembershipFilter{OrgID: msp(w.orgB)}, token)
			})
			if !gotB[w.memberB] || gotB[w.memberA] {
				t.Errorf("org=B returned A=%v B=%v, want A=false B=true", gotB[w.memberA], gotB[w.memberB])
			}

			// Unfiltered, the same arm sees both — so the exclusions above are the FILTER's doing and
			// not reach, visibility or an empty world.
			all := mAllIDs(t, func(token string) (application.MembershipPage, error) {
				return arm.list(domain.MembershipFilter{}, token)
			})
			if !all[w.memberA] || !all[w.memberB] {
				t.Fatalf("unfiltered arm %s sees A=%v B=%v — the world is not what the assertions above assume",
					arm.name, all[w.memberA], all[w.memberB])
			}
		})
	}
}

// TestOrgFacetStatsTotalEqualsExhaustivePaging is M57's exit criterion applied to the new facet, on
// both arms: the number the chart is drawn from must equal the number of rows the list returns under
// the same filter. This is the assertion the headcount chart's honesty rests on.
func TestOrgFacetStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	w := seedMOrgWorld(t)
	ctx := context.Background()
	f := domain.MembershipFilter{OrgID: msp(w.orgA)}
	sel := statsSel(t, "link__member_of")

	for _, arm := range []struct {
		name    string
		subject string
		list    func(token string) (application.MembershipPage, error)
	}{
		{"admin", "", func(token string) (application.MembershipPage, error) {
			return w.svc.ListMemberships(ctx, f, 50, token)
		}},
		{"scoped", w.reader, func(token string) (application.MembershipPage, error) {
			return w.svc.ListVisibleMemberships(ctx, w.reader, f, 50, token)
		}},
	} {
		t.Run(arm.name, func(t *testing.T) {
			res, err := w.svc.MembershipStats(ctx, arm.subject, arm.subject == "", f, sel)
			if err != nil {
				t.Fatalf("MembershipStats: %v", err)
			}
			paged := int64(len(mAllIDs(t, arm.list)))
			if res.TotalCount != paged {
				t.Errorf("totalCount=%d, exhaustive paging=%d under org=A", res.TotalCount, paged)
			}
			if paged == 0 {
				t.Fatal("both are zero — the world seeded nothing, so the equality proves nothing")
			}
		})
	}
}

// TestOrgFacetDistributionPartitions checks the DISTRIBUTION rather than the filter: every counted
// membership lands in exactly one org bucket, and the org this world seeded is one of them with its
// own count. The buckets summing to totalCount is what makes the bar chart's segments add up to the
// number printed beside it.
func TestOrgFacetDistributionPartitions_Integration(t *testing.T) {
	w := seedMOrgWorld(t)
	ctx := context.Background()
	sel := statsSel(t, "link__member_of")

	res, err := w.svc.MembershipStats(ctx, "", true, domain.MembershipFilter{}, sel)
	if err != nil {
		t.Fatalf("MembershipStats: %v", err)
	}
	buckets := bucketsFor(t, res, "org")
	var sum int64
	for _, b := range buckets {
		sum += b.Count
	}
	if sum != res.TotalCount {
		t.Errorf("org buckets sum to %d, totalCount=%d — a membership is counted twice or not at all", sum, res.TotalCount)
	}

	// The seeded org must appear with at least its own membership — unless it fell into the top-N
	// tail, which a shared test database makes possible. Filtering to it is the check that survives
	// that: one org, one bucket, and its count is the filtered total.
	single, err := w.svc.MembershipStats(ctx, "", true, domain.MembershipFilter{OrgID: msp(w.orgA)}, sel)
	if err != nil {
		t.Fatalf("MembershipStats(org=A): %v", err)
	}
	if got := countFor(bucketsFor(t, single, "org"), w.orgA); got != single.TotalCount || got == 0 {
		t.Errorf("filtered to org A: bucket=%d totalCount=%d, want equal and non-zero", got, single.TotalCount)
	}
}

// TestOrgFacetReadsTheRLSExemptProjection is the RLS trap made explicit. tenant_units is RLS-FORCED
// under tenant_units_reach; authz_unit_org is exempt. On the bare pool these tests use, the two
// answer DIFFERENTLY for a pdp_scoped unit, and only one of them answers correctly — so this asserts
// the projection is populated for the seeded units and that the filter agrees with it.
//
// Written as a direct SQL comparison rather than through the service because the failure it guards
// is invisible at the service layer: a tenant_units join returns no error, just zero rows.
func TestOrgFacetReadsTheRLSExemptProjection_Integration(t *testing.T) {
	w := seedMOrgWorld(t)
	ctx := context.Background()

	var projected string
	if err := w.pool.QueryRow(ctx,
		`SELECT org_id::text FROM oikumenea.authz_unit_org WHERE unit_id = $1`, w.unitA).Scan(&projected); err != nil {
		t.Fatalf("authz_unit_org has no row for the seeded unit: %v — the trigger that maintains the projection did not fire", err)
	}
	if projected != w.orgA {
		t.Errorf("projection says org=%s, unit was seeded into %s", projected, w.orgA)
	}

	// What the RLS-FORCED table answers on this same connection, for contrast: if this returns a row
	// too, the trap is dormant HERE, but the projection is still the correct table to read (the app's
	// own pool pins per request and this one does not).
	var viaUnits int
	if err := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.tenant_units WHERE id = $1 AND org_id = $2`, w.unitA, w.orgA).Scan(&viaUnits); err != nil {
		t.Fatalf("count via tenant_units: %v", err)
	}
	t.Logf("tenant_units answered %d row(s) for the same question on an unpinned connection; "+
		"authz_unit_org answered 1 — the predicate reads the latter", viaUnits)

	// And the filter itself agrees with the projection.
	got := mAllIDs(t, func(token string) (application.MembershipPage, error) {
		return w.svc.ListMemberships(ctx, domain.MembershipFilter{OrgID: msp(w.orgA)}, 50, token)
	})
	if !got[w.memberA] {
		t.Error("the org filter returns nothing for a unit the projection places in that org")
	}
}
