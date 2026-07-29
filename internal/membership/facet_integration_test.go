// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the top-level membership listing and its facet filters (M56 ticket 3 /
// D-ObjectFacets) against a real Postgres.
//
// What matters is not that a filter "works" but that it works IDENTICALLY on the two list paths —
// the instance-admin ListMemberships and the reach-scoped ListMembershipsForSubject — because they
// are two separate SQL blocks carrying the same vocabulary. So each facet is asserted on both, and
// then the paths are compared directly:
//
//	scoped(f) == admin(f) ∩ reach     for every filter f
//
// which is the property that makes the duplication safe. Paging is asserted at pageSize=1 across a
// filtered set, because the whole reason the predicates live inside the SQL is that a Go-side
// re-filter would hand back a short page with a nextPageToken (review-2026-07 R-06).
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/membership/... -run Facet
package membership_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
)

func msp(s string) *string    { return &s }
func mdp(s string) *time.Time { t, _ := time.Parse(domain.ISODate, s); return &t }

// mFacetWorld is a two-unit world: the reader may read unitIn, not unitOut. Every facet has at least
// one matching and one non-matching membership INSIDE the reader's reach — otherwise a filter could
// "pass" by returning nothing on both paths — and at least one matching row OUTSIDE it, so the
// intersection assertion is not trivially the whole set.
type mFacetWorld struct {
	svc  *application.Service
	pool *pgxpool.Pool

	unitIn, unitOut string
	reader          string

	// inside the reader's reach
	mActive   string // active, no position, effective 2024-03-04
	mEnded    string // ended,  no position, effective 2020-01-02
	mWithPos  string // active, WITH position, effective 2024-03-04
	positionA string
	personA   string

	// outside the reader's reach — same shapes, so every filter has an out-of-reach match
	mOutActive string
	personOut  string
}

func seedMFacetWorld(t *testing.T) *mFacetWorld {
	t.Helper()
	svc, pool := newService(t)
	w := &mFacetWorld{svc: svc, pool: pool}

	w.unitIn = seedUnit(t, pool)
	w.unitOut = seedUnit(t, pool)

	w.personA = seedPerson(t, pool)
	personB := seedPerson(t, pool)
	personC := seedPerson(t, pool)
	w.personOut = seedPerson(t, pool)

	w.mActive = seedFacetMembership(t, pool, w.personA, w.unitIn, "", "active", "2024-03-04")
	w.mEnded = seedFacetMembership(t, pool, personB, w.unitIn, "", "ended", "2020-01-02")
	w.positionA = seedFacetPosition(t, pool, w.unitIn)
	w.mWithPos = seedFacetMembership(t, pool, personC, w.unitIn, w.positionA, "active", "2024-03-04")

	w.mOutActive = seedFacetMembership(t, pool, w.personOut, w.unitOut, "", "active", "2024-03-04")

	w.reader = seedPerson(t, pool)
	seedMembershipReadGrant(t, pool, w.reader, w.unitIn)
	return w
}

// seedFacetMembership inserts one membership with an explicit status and effective_from. It writes
// SQL directly rather than going through the service because the service only ever creates ACTIVE
// rows, and the whole point of the top-level list is that it shows every status.
func seedFacetMembership(t *testing.T, pool *pgxpool.Pool, personID, unitID, positionID, status, effectiveFrom string) string {
	t.Helper()
	var pos any
	if positionID != "" {
		pos = positionID
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.membership_memberships (person_id, unit_id, position_id, status, effective_from)
		 VALUES ($1, $2, $3, $4, $5::date) RETURNING id`,
		personID, unitID, pos, status, effectiveFrom).Scan(&id); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return id
}

func seedFacetPosition(t *testing.T, pool *pgxpool.Pool, unitID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.membership_positions (unit_id, code, title) VALUES ($1, $2, 'Facet billet') RETURNING id`,
		unitID, code(t, "facet-pos")).Scan(&id); err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return id
}

func seedMembershipReadGrant(t *testing.T, pool *pgxpool.Pool, readerID, unitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Membership facet test role') RETURNING id`,
		code(t, "mfacet-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'membership.read')`,
		roleID); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1, $2, $3, 'unit')`, readerID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
}

// mAllIDs pages a listing to exhaustion. The test database is shared, so the admin view holds rows
// from every other suite; comparing a single page against a seeded expectation would fail on
// pagination rather than on filtering. Exhaustive paging is also the exact shape M56's exit
// criterion names.
func mAllIDs(t *testing.T, list func(pageToken string) (application.MembershipPage, error)) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	token := ""
	for i := 0; ; i++ {
		if i > 2000 {
			t.Fatal("paging did not terminate after 2000 pages")
		}
		page, err := list(token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		for _, m := range page.Memberships {
			out[m.ID] = true
		}
		if page.NextPageToken == "" {
			return out
		}
		token = page.NextPageToken
	}
}

func (w *mFacetWorld) admin(t *testing.T, f domain.MembershipFilter) map[string]bool {
	t.Helper()
	return mAllIDs(t, func(tok string) (application.MembershipPage, error) {
		return w.svc.ListMemberships(context.Background(), f, 50, tok)
	})
}

func (w *mFacetWorld) scoped(t *testing.T, f domain.MembershipFilter) map[string]bool {
	t.Helper()
	return mAllIDs(t, func(tok string) (application.MembershipPage, error) {
		return w.svc.ListVisibleMemberships(context.Background(), w.reader, f, 50, tok)
	})
}

// TestMembershipFacetsRoundTrip asserts every declared facet selects what it should on BOTH paths.
func TestMembershipFacetsRoundTrip(t *testing.T) {
	w := seedMFacetWorld(t)

	cases := []struct {
		name              string
		filter            domain.MembershipFilter
		wantIn, wantOutOf []string // must be present / absent in the ADMIN result
	}{
		{
			"unitId", domain.MembershipFilter{UnitID: msp(w.unitIn)},
			[]string{w.mActive, w.mEnded, w.mWithPos}, []string{w.mOutActive},
		},
		{
			"personId", domain.MembershipFilter{PersonID: msp(w.personA)},
			[]string{w.mActive}, []string{w.mEnded, w.mWithPos, w.mOutActive},
		},
		{
			"positionId", domain.MembershipFilter{PositionID: msp(w.positionA)},
			[]string{w.mWithPos}, []string{w.mActive, w.mEnded},
		},
		{
			// The facet that only exists because the top-level list has no implicit active-only
			// default: an ended membership is unreachable through any other endpoint.
			"status=ended", domain.MembershipFilter{Status: msp("ended")},
			[]string{w.mEnded}, []string{w.mActive, w.mWithPos, w.mOutActive},
		},
		{
			"status=active", domain.MembershipFilter{Status: msp("active")},
			[]string{w.mActive, w.mWithPos, w.mOutActive}, []string{w.mEnded},
		},
		{
			"effectiveFrom range", domain.MembershipFilter{
				EffectiveFromAfter:  mdp("2024-01-01"),
				EffectiveFromBefore: mdp("2024-12-31"),
			},
			[]string{w.mActive, w.mWithPos}, []string{w.mEnded},
		},
		{
			// Same date on both bounds must select that whole DAY — the reason the upper bound
			// compares against the end of the day and not midnight.
			"effectiveFrom single day", domain.MembershipFilter{
				EffectiveFromAfter:  mdp("2024-03-04"),
				EffectiveFromBefore: mdp("2024-03-04"),
			},
			[]string{w.mActive, w.mWithPos}, []string{w.mEnded},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.admin(t, c.filter)
			for _, id := range c.wantIn {
				if !got[id] {
					t.Errorf("admin: %s missing from the filtered set", id)
				}
			}
			for _, id := range c.wantOutOf {
				if got[id] {
					t.Errorf("admin: %s should not match this filter", id)
				}
			}
			// The scoped path must agree wherever it can see: never more than admin, and never less
			// than admin ∩ reach.
			assertScopedIsAdminIntersectReach(t, w, c.filter)
		})
	}
}

// TestMembershipUnfilteredHasNoImplicitStatusDefault pins the decision the endpoint is built on: the
// top-level list returns EVERY status. A hidden active-only default would make M57's totalCount
// disagree with its own status distribution, and would make ended rows unreachable entirely.
func TestMembershipUnfilteredHasNoImplicitStatusDefault(t *testing.T) {
	w := seedMFacetWorld(t)
	got := w.admin(t, domain.MembershipFilter{})
	for _, id := range []string{w.mActive, w.mEnded, w.mWithPos} {
		if !got[id] {
			t.Errorf("unfiltered listing omits %s — the top-level list must carry no implicit status filter", id)
		}
	}
}

// TestMembershipScopedEqualsAdminIntersectReach is the property that makes two SQL blocks safe.
func TestMembershipScopedEqualsAdminIntersectReach(t *testing.T) {
	w := seedMFacetWorld(t)
	for _, f := range []domain.MembershipFilter{
		{},
		{Status: msp("active")},
		{Status: msp("ended")},
		{UnitID: msp(w.unitIn)},
		{UnitID: msp(w.unitOut)},
		{PersonID: msp(w.personA)},
		{PositionID: msp(w.positionA)},
		{EffectiveFromAfter: mdp("2024-01-01")},
	} {
		assertScopedIsAdminIntersectReach(t, w, f)
	}
}

func assertScopedIsAdminIntersectReach(t *testing.T, w *mFacetWorld, f domain.MembershipFilter) {
	t.Helper()
	admin := w.admin(t, f)
	scoped := w.scoped(t, f)

	// The reader's reach is exactly unitIn, so the expected scoped set is the admin set restricted to
	// memberships in that unit. Computing it from the DB rather than from the seeded ids keeps the
	// assertion honest against the other suites' rows sharing this database.
	inUnit := map[string]bool{}
	rows, err := w.pool.Query(context.Background(),
		`SELECT id FROM oikumenea.membership_memberships WHERE unit_id = $1 AND deleted_at IS NULL`, w.unitIn)
	if err != nil {
		t.Fatalf("read unit memberships: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		inUnit[id] = true
	}

	for id := range scoped {
		if !admin[id] {
			t.Errorf("scoped returned %s, which the admin path with the same filter does not — the "+
				"scoped path must never be WIDER than admin", id)
		}
		if !inUnit[id] {
			t.Errorf("scoped returned %s, which is outside the reader's reach", id)
		}
	}
	for id := range admin {
		if inUnit[id] && !scoped[id] {
			t.Errorf("scoped omitted %s, which is both admin-visible and in reach — the two filter "+
				"blocks have drifted", id)
		}
	}
}

// TestMembershipFilteredPagingDrainsExactly is the R-06 assertion: a filtered listing paged one row
// at a time must yield exactly the same set as a single large page, with no short page carrying a
// next-page token. This is the failure a Go-side post-filter would produce.
func TestMembershipFilteredPagingDrainsExactly(t *testing.T) {
	w := seedMFacetWorld(t)
	f := domain.MembershipFilter{UnitID: msp(w.unitIn)}

	bulk := w.admin(t, f)
	oneAtATime := mAllIDs(t, func(tok string) (application.MembershipPage, error) {
		return w.svc.ListMemberships(context.Background(), f, 1, tok)
	})

	if len(bulk) != len(oneAtATime) {
		t.Fatalf("pageSize=1 drain yielded %d rows, bulk yielded %d", len(oneAtATime), len(bulk))
	}
	for id := range bulk {
		if !oneAtATime[id] {
			t.Errorf("%s appears in the bulk page but not in the one-at-a-time drain", id)
		}
	}
	// And the same for the scoped path, where the reach predicate is also inside the query.
	scopedBulk := w.scoped(t, f)
	scopedDrain := mAllIDs(t, func(tok string) (application.MembershipPage, error) {
		return w.svc.ListVisibleMemberships(context.Background(), w.reader, f, 1, tok)
	})
	if len(scopedBulk) != len(scopedDrain) {
		t.Fatalf("scoped pageSize=1 drain yielded %d rows, bulk yielded %d", len(scopedDrain), len(scopedBulk))
	}
}

// TestMembershipFilterValidation: a bad value is a 400-shaped domain error on BOTH paths, never a
// 500 and never a silently-ignored filter (which would return MORE rows than asked for).
func TestMembershipFilterValidation(t *testing.T) {
	w := seedMFacetWorld(t)
	ctx := context.Background()

	for _, c := range []struct {
		name string
		f    domain.MembershipFilter
	}{
		{"unknown status", domain.MembershipFilter{Status: msp("retired")}},
		{"non-RID unitId", domain.MembershipFilter{UnitID: msp("not-a-rid")}},
		{"inverted range", domain.MembershipFilter{
			EffectiveFromAfter: mdp("2025-01-01"), EffectiveFromBefore: mdp("2024-01-01"),
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := w.svc.ListMemberships(ctx, c.f, 50, ""); err == nil {
				t.Error("admin path accepted an invalid filter")
			}
			if _, err := w.svc.ListVisibleMemberships(ctx, w.reader, c.f, 50, ""); err == nil {
				t.Error("scoped path accepted an invalid filter")
			}
		})
	}
}
