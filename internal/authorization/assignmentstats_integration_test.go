// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the assignment LIST and DASHBOARD (M58 ticket 6 / D-ObjectFacets) against a
// real Postgres.
//
// Two things are being proved here that no other type's suite proves.
//
//  1. The reach trim is asked for `assignment.read` SPECIFICALLY. The unit tests can only check that
//     the SQL names the narrow function and the Go passes the narrow code; whether that actually
//     produces a narrower answer is a question about the database. So a subject is given generic read
//     reach over a unit WITHOUT assignment.read, and must see nothing there — which is precisely the
//     widening that borrowing authz_readable_units() would have caused, and which would otherwise
//     have looked like a working feature.
//
//  2. The ACTIVE-ONLY default is a default, not an empty table. A revoked grant exists and appears in
//     neither the list nor any bucket, so `totalCount counts ACTIVE grants` is demonstrated rather
//     than asserted in a doc comment.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/authorization/... -run AssignmentStats
package authorization_test

import (
	"context"
	"testing"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

func allAssignmentFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("link__has_role")
	if !ok {
		t.Fatal("link__has_role is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

// pageAllAssignments exhaustively pages the list under one filter and returns the row count. Pages of
// 2 on purpose: a keyset that loses a row at a page boundary only shows up once the sweep turns over.
func pageAllAssignments(t *testing.T, h harness, f authzdomain.AssignmentFilter, reader string, isAdmin bool) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 500; i++ {
		page, err := h.authz.ListAssignments(ctx, f, reader, isAdmin, 2, token)
		if err != nil {
			t.Fatalf("list assignments: %v", err)
		}
		for _, a := range page.Assignments {
			if seen[a.ID] {
				t.Fatalf("assignment %s returned twice — the keyset is broken", a.ID)
			}
			seen[a.ID] = true
		}
		if page.NextPageToken == "" {
			return len(seen)
		}
		token = page.NextPageToken
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// assignmentSpread grants a population on ONE fresh unit, so every assertion can be scoped to this
// test's rows: the test database is persistent, and an unfiltered assertion would race every other
// suite that grants a role.
type assignmentSpread struct {
	unit, otherUnit  string
	holderA, holderB string
	roleReader       string
	revoked          string
}

func seedAssignmentSpread(t *testing.T, h harness) assignmentSpread {
	t.Helper()
	sp := assignmentSpread{
		unit:       h.seedUnit(t),
		otherUnit:  h.seedUnit(t),
		holderA:    h.seedPerson(t),
		holderB:    h.seedPerson(t),
		roleReader: h.roleID(t, "unit-reader"),
	}
	admin := h.roleID(t, "unit-admin")
	// Two people, two roles, both scopes, on one unit — so role, subject and scope all have more than
	// one bucket. A single-bucket distribution makes a differential pass without testing anything.
	h.grant(t, sp.holderA, sp.roleReader, sp.unit, authzdomain.ScopeUnit, "")
	h.grant(t, sp.holderA, admin, sp.unit, authzdomain.ScopeSubtree, "command")
	h.grant(t, sp.holderB, sp.roleReader, sp.unit, authzdomain.ScopeSubtree, "command")
	// One on a DIFFERENT unit, so the targetUnitId facet is not constant either.
	h.grant(t, sp.holderB, admin, sp.otherUnit, authzdomain.ScopeUnit, "")
	// And one revoked, which must appear nowhere.
	rev := h.grant(t, sp.holderB, sp.roleReader, sp.otherUnit, authzdomain.ScopeUnit, "")
	if _, err := h.authz.RevokeAssignment(context.Background(), rev.ID, ""); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	sp.revoked = rev.ID
	return sp
}

// TestAssignmentStatsTotalEqualsExhaustivePaging_Integration is D-ObjectFacets' headline promise on
// the admin arm: the number the dashboard prints is the number of rows the list hands over.
func TestAssignmentStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	sp := seedAssignmentSpread(t, h)

	f := authzdomain.AssignmentFilter{TargetUnitID: &sp.unit}
	res, err := h.authz.AssignmentStats(ctx, f, "", true, allAssignmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), pageAllAssignments(t, h, f, "", true); got != want {
		t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
	}
	if res.TotalCount != 3 {
		t.Errorf("expected the 3 active grants on this unit, got %d", res.TotalCount)
	}
}

// TestAssignmentStatsEveryBucketEqualsItsOwnFilter_Integration: clicking a segment must land on
// exactly the rows that segment counted. A wrong inverse fails silently — the operator gets a list
// that quietly disagrees with the bar they clicked, and neither number looks wrong on its own.
func TestAssignmentStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	sp := seedAssignmentSpread(t, h)

	base := authzdomain.AssignmentFilter{TargetUnitID: &sp.unit}
	res, err := h.authz.AssignmentStats(ctx, base, "", true, allAssignmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	checked := 0
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == "(unknown)" || b.Key == "(other)" || b.Count == 0 {
				continue // synthetic keys are deliberately not filter values
			}
			f := base
			k := b.Key
			switch d.Facet {
			case "subjectPersonId":
				f.SubjectPersonID = &k
			case "roleId":
				f.RoleID = &k
			case "targetUnitId":
				f.TargetUnitID = &k
			case "scope":
				f.Scope = &k
			case "graphId":
				f.GraphID = &k
			default:
				t.Fatalf("facet %q has no filter inverse in this test — a new facet was declared and "+
					"its click-through is unchecked", d.Facet)
			}
			if got := pageAllAssignments(t, h, f, "", true); got != int(b.Count) {
				t.Errorf("%s bucket %q counted %d, its own filter returns %d", d.Facet, b.Key, b.Count, got)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no bucket was checked — the spread produced no distribution and this test is vacuous")
	}
}

// TestAssignmentStatsDistributionsSumToTotal_Integration: every assignment facet reads the LISTED
// table, so each row has exactly one value in each and the buckets partition. `graphId` is the one to
// watch — it is NULL exactly when scope is `unit`, so its (unknown) bucket must carry that count
// rather than vanish.
func TestAssignmentStatsDistributionsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	sp := seedAssignmentSpread(t, h)

	f := authzdomain.AssignmentFilter{TargetUnitID: &sp.unit}
	res, err := h.authz.AssignmentStats(ctx, f, "", true, allAssignmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, d := range res.Distributions {
		var sum int64
		for _, b := range d.Buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("%s sums to %d, totalCount is %d — a partitioning facet lost or doubled a row",
				d.Facet, sum, res.TotalCount)
		}
	}
}

// TestAssignmentRevokedGrantsAreInvisible_Integration proves the active-only default is a DEFAULT.
// The spread revokes one grant; it must appear in no page and in no bucket. Without a revoked row in
// the fixture, "the list returns active grants" is true of an empty set and proves nothing.
func TestAssignmentRevokedGrantsAreInvisible_Integration(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	sp := seedAssignmentSpread(t, h)

	f := authzdomain.AssignmentFilter{TargetUnitID: &sp.otherUnit}
	page, err := h.authz.ListAssignments(ctx, f, "", true, 50, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, a := range page.Assignments {
		if a.ID == sp.revoked {
			t.Fatalf("the revoked assignment %s is in the list — the active-only default is gone, and "+
				"totalCount no longer means what the contract says", sp.revoked)
		}
	}
	res, err := h.authz.AssignmentStats(ctx, f, "", true, allAssignmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), len(page.Assignments); got != want {
		t.Errorf("totalCount = %d but the list returned %d — the aggregate and the list disagree about "+
			"whether a revoked grant counts", got, want)
	}
	if res.TotalCount != 1 {
		t.Errorf("expected the 1 surviving active grant on the other unit, got %d", res.TotalCount)
	}
}

// TestAssignmentReachIsAssignmentReadOnly_Integration is the ticket's load-bearing claim, and the one
// that can only be proved against a database.
//
// A reader is given a role carrying `person.read` (a '%.read' code) over the spread's unit, and
// nothing else. Generic read-reach — what authz_readable_units() computes and what every other
// module's scoped list trims with — therefore COVERS that unit. assignment.read reach does not.
//
// So: if this endpoint borrowed the generic family, the reader would see the unit's grants. It must
// see none. The test is written to go red in exactly that case, which is the case that looks like a
// working feature.
func TestAssignmentReachIsAssignmentReadOnly_Integration(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t)
	sp := seedAssignmentSpread(t, h)

	// A role with a '.read' code that is NOT assignment.read. Created here rather than reused from the
	// base set, because the base roles bundle assignment.read with the other reads — which is exactly
	// how this distinction stays invisible until someone constructs the case.
	code := uniq("person-only-reader")
	if _, err := h.authz.CreateRole(ctx, authzdomain.Role{
		Code: code, Name: "Person-only reader",
		Permissions: []authzdomain.Permission{authzdomain.PermPersonRead},
	}); err != nil {
		t.Fatalf("create role: %v", err)
	}
	reader := h.seedPerson(t)
	h.grant(t, reader, h.roleID(t, code), sp.unit, authzdomain.ScopeUnit, "")

	f := authzdomain.AssignmentFilter{TargetUnitID: &sp.unit}
	if got := pageAllAssignments(t, h, f, reader, false); got != 0 {
		t.Errorf("a reader holding only person.read over the unit sees %d of its grants. Generic "+
			"'%%.read' reach covers this unit and assignment.read reach does not, so the trim is asking "+
			"the WIDE question — this endpoint got wider by acquiring a filter", got)
	}
	res, err := h.authz.AssignmentStats(ctx, f, reader, false, allAssignmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 {
		t.Errorf("the dashboard counts %d grants for a reader who may list none — the list and the "+
			"aggregate are trimming with different questions", res.TotalCount)
	}

	// The other direction, from the same setup: either assertion alone is satisfiable by a bug (a trim
	// that returns nothing at all also passes the first). Granting assignment.read must make the unit's
	// grants appear — ALL of them, which is what the admin arm sees.
	//
	// Compared against the admin count rather than a literal, deliberately: the reader's own two grants
	// are themselves rows on this unit, so a hardcoded number here would encode how many roles this
	// test happens to grant and would go red the next time someone adds one — a test that fails for
	// arithmetic rather than for behaviour teaches the next reader to distrust it.
	h.grant(t, reader, sp.roleReader, sp.unit, authzdomain.ScopeUnit, "")
	want := pageAllAssignments(t, h, f, "", true)
	if got := pageAllAssignments(t, h, f, reader, false); got != want {
		t.Errorf("with assignment.read over the unit the reader sees %d of the %d active grants an "+
			"admin sees — the trim is now refusing reach it should honour", got, want)
	}
	if want == 0 {
		t.Error("the admin arm sees nothing either — the fixture is empty and both directions of this " +
			"test are vacuous")
	}
}
