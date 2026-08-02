// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the organization DASHBOARD aggregate (M58 ticket 4 / D-ObjectFacets) against
// a real Postgres.
//
// The differential is the vocabulary's usual one — totalCount equals the rows an exhaustive paging of
// the same list returns under the same filters, and every bucket's count equals what its own filter
// returns — but organization carries the ticket's one real design argument, and two tests here exist
// to pin it rather than leave it as prose.
//
// listOrganizations is gated by gateUnits, the SAME app-layer shadow gate the unit list uses. On a
// unit that gate is real. On an organization it is not: authz_role_assignments.target_unit_id is NOT
// NULL and REFERENCES tenant_units, so an organization RID matches neither arm of
// ReadableUnitsForSubjectAmong and the reach set is empty by construction. A shadow organization is
// therefore visible to an instance admin and to nobody else, and the scoped aggregate arm is a flat
// `visibility = 'public'` rather than unit's reach predicate.
//
// That is a load-bearing claim about ANOTHER module's schema, so it is asserted, not assumed:
// TestOrganizationShadowIsUnreachableForEveryNonAdmin grants the subject assignments that would reach
// the org's units and shows the org count does not move. It is meant to go RED on the day org
// reachability is fixed — which is exactly the day this arm must change.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/... -run OrganizationStats
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

func allOrgFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("organization")
	if !ok {
		t.Fatal("organization is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func orgBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

// pageAllOrgs exhaustively pages listOrganizations under one filter and returns the row count. Small
// pages on purpose: a bug that loses rows at a page boundary (the shape ticket 2 found in listTaxa,
// which had silently dropped 16 of 100 taxa since M22) only shows up when the sweep actually turns
// over. It also asserts no id repeats, which is the other half of a broken keyset.
func pageAllOrgs(t *testing.T, svc *application.Service, f domain.OrgFilter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 500; i++ {
		page, err := svc.ListOrganizations(ctx, f, 3, token)
		if err != nil {
			t.Fatalf("list organizations: %v", err)
		}
		for _, o := range page.Orgs {
			if seen[o.ID] {
				t.Fatalf("organization %s returned twice — the keyset is broken", o.ID)
			}
			seen[o.ID] = true
		}
		if page.NextPageToken == "" {
			return len(seen)
		}
		token = page.NextPageToken
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// TestOrganizationStatsTotalEqualsExhaustivePaging is D-ObjectFacets' headline promise, on the ADMIN
// arm: the number the dashboard prints is the number of rows the list would hand over, not an
// estimate and not a differently-filtered count.
func TestOrganizationStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	// Scope the comparison to this test's own domain: the test database is persistent, so an
	// unfiltered assertion would race every other test that creates an organization.
	f := domain.OrgFilter{DomainID: &org.DomainID}
	res, err := svc.OrganizationStats(ctx, "", true, f, allOrgFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), pageAllOrgs(t, svc, f); got != want {
		t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
	}
}

// TestOrganizationStatsEveryBucketEqualsItsOwnFilter is the property the whole vocabulary rests on,
// and the one a chart cannot check for itself: clicking a segment must land on exactly the rows that
// segment counted. A wrong inverse fails silently — the operator gets a list that quietly disagrees
// with the bar they clicked, and neither number looks wrong on its own.
func TestOrganizationStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)
	// A second org in the same domain, shadowed and suspended, so visibility and state each have two
	// non-empty buckets — a single-bucket distribution would make this test pass without testing.
	other := seedOrg(t, svc)
	repointDomain(t, svc, other.ID, org.DomainID)
	shadowOrg(t, svc, other.ID)
	suspendOrg(t, svc, other.ID)

	base := domain.OrgFilter{DomainID: &org.DomainID}
	res, err := svc.OrganizationStats(ctx, "", true, base, allOrgFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
				continue // synthetic keys are deliberately not filter values, and not clickable
			}
			f := base
			switch d.Facet {
			case "domain":
				k := b.Key
				f.DomainID = &k
			case "visibility":
				k := b.Key
				f.Visibility = &k
			case "state":
				k := b.Key
				f.State = &k
			default:
				t.Fatalf("undeclared distribution %q in the response", d.Facet)
			}
			if got, want := int(b.Count), pageAllOrgs(t, svc, f); got != want {
				t.Errorf("%s[%s] counted %d, but its own filter returns %d rows", d.Facet, b.Key, got, want)
			}
		}
	}
}

// TestOrganizationStatsDistributionsSumToTotal: organization declares no NonPartitioning facet, so
// every distribution must account for every counted row exactly once. This is the assertion that
// exemption suspends, and it holds here because all three facets are single-valued NOT NULL columns
// on the listed table.
func TestOrganizationStatsDistributionsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	f := domain.OrgFilter{DomainID: &org.DomainID}
	res, err := svc.OrganizationStats(ctx, "", true, f, allOrgFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(res.Distributions) != 3 {
		t.Fatalf("got %d distributions, want 3 (domain, visibility, state)", len(res.Distributions))
	}
	for _, d := range res.Distributions {
		if got := sumOf(d.Buckets); got != res.TotalCount {
			t.Errorf("%s sums to %d, totalCount is %d — a partitioning facet must account for every row once",
				d.Facet, got, res.TotalCount)
		}
	}
}

// TestOrganizationStatsShadowGateIsInsideTheCount is the ticket's headline assertion. It checks BOTH
// halves: that the scoped arm drops the shadow org, and that what it leaves is exactly what the LIST
// leaves after gateUnits trims it. Asserting only the first would prove the SQL says `public`; it is
// the second that proves the two surfaces agree, which is what the differential contract is.
func TestOrganizationStatsShadowGateIsInsideTheCount_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	hidden := seedOrg(t, svc)
	repointDomain(t, svc, hidden.ID, org.DomainID)
	shadowOrg(t, svc, hidden.ID)
	subject := seedStatsPerson(t, pool)

	f := domain.OrgFilter{DomainID: &org.DomainID}
	sel := allOrgFacets(t)
	admin, err := svc.OrganizationStats(ctx, "", true, f, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	scoped, err := svc.OrganizationStats(ctx, subject, false, f, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}

	if got := countOf(orgBuckets(t, admin, "visibility"), "shadow"); got != 1 {
		t.Errorf("admin visibility[shadow] = %d, want the hidden org — otherwise this test proves "+
			"nothing about the scoped arm", got)
	}
	if got := countOf(orgBuckets(t, scoped, "visibility"), "shadow"); got != 0 {
		t.Errorf("scoped visibility[shadow] = %d, want 0: a shadow organization must never be counted "+
			"for a non-admin, only trimmed-after-the-fact on a list", got)
	}
	if got, want := scoped.TotalCount, admin.TotalCount-1; got != want {
		t.Errorf("scoped totalCount = %d, want %d (the admin total minus the shadow org)", got, want)
	}
	if scoped.TotalCount < 1 {
		t.Errorf("scoped totalCount = %d, want the public org to remain visible — the gate narrows, "+
			"it does not blank the dashboard", scoped.TotalCount)
	}
}

// TestOrganizationReachIsDerivedFromUnitReach pins the rule organizations actually run on: an
// organization is visible when ANY of its live units is in the subject's reach.
//
// It replaces `TestOrganizationShadowIsUnreachableForEveryNonAdmin`, which pinned the OPPOSITE and
// was written to go red on the day this changed — and did, with the message it carried for the
// purpose. That is the guard working, not a guard being wrong: what it protected against was the
// semantics drifting silently, and the semantics changed loudly instead.
//
// Both directions are asserted from ONE setup, because either alone is satisfiable by a bug. "Sees
// it with reach" alone passes if the gate stopped gating; "does not see it without" alone passes if
// reach stayed empty by construction, which is exactly the state this replaced.
func TestOrganizationReachIsDerivedFromUnitReach_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	hidden := seedOrg(t, svc)
	repointDomain(t, svc, hidden.ID, org.DomainID)
	shadowOrg(t, svc, hidden.ID)
	unit := mustCreate(t, svc, hidden, uniqueCode(t, "reachprobe"))
	graph := commandGraph(t, svc, hidden.ID)

	insider := seedStatsPerson(t, pool)
	grantSubtreeRead(t, pool, insider, unit.ID, graph)
	outsider := seedStatsPerson(t, pool) // no grants at all

	// The grant has to be real, or "the insider sees it" would be trivially true of everyone.
	if n := readableUnitCount(t, pool, insider); n == 0 {
		t.Fatal("the insider reaches no units — the grant did not take, so this test proves nothing")
	}
	if n := readableUnitCount(t, pool, outsider); n != 0 {
		t.Fatalf("the outsider reaches %d units — it must reach none, or the negative half is vacuous", n)
	}

	f := domain.OrgFilter{DomainID: &org.DomainID}
	sel := allOrgFacets(t)

	inside, err := svc.OrganizationStats(ctx, insider, false, f, sel)
	if err != nil {
		t.Fatalf("insider stats: %v", err)
	}
	if got := countOf(orgBuckets(t, inside, "visibility"), "shadow"); got != 1 {
		t.Errorf("insider counted %d shadow organizations, want 1: reach into a shadow org's units is "+
			"what makes that org visible (D-VisibilityScope as amended after M58 ticket 4)", got)
	}

	outside, err := svc.OrganizationStats(ctx, outsider, false, f, sel)
	if err != nil {
		t.Fatalf("outsider stats: %v", err)
	}
	if got := countOf(orgBuckets(t, outside, "visibility"), "shadow"); got != 0 {
		t.Errorf("outsider counted %d shadow organizations, want 0: deriving reach must not make every "+
			"shadow organization public", got)
	}
	if inside.TotalCount <= outside.TotalCount {
		t.Errorf("insider total %d is not greater than outsider total %d — the derivation is not "+
			"narrowing anything", inside.TotalCount, outside.TotalCount)
	}

	// The list-vs-chart agreement ACROSS the visibility boundary is deliberately not asserted here.
	// The gate that trims the list is applied in the transport (gateOrgs), and this suite constructs
	// the application service — it has no handler to run the gate. Faking it with a test-only
	// accessor would produce a test that agrees with itself; that is the same layer confusion that
	// made the first getOrganization guard useless. It is verified live over HTTP instead.
}

// TestOrganizationPointReadAgreesWithTheList pins the APPLICATION layer's half of the contract: the
// point read and the list resolve the same organizations from the same store, so a divergence here
// would be a repository or filter bug rather than a gating one.
//
// It is deliberately NOT the guard for the M58 ticket 4 leak, and saying so matters. That leak —
// `getOrganization` documented "(shadow-gated)" while applying no gate, so a caller holding
// `organization.read` could read by RID exactly the shadow organizations the list hid — lives
// entirely in the TRANSPORT, where the gate is applied per handler. This test was written for it
// first and hand-breaking proved it useless for that: reintroducing the leak leaves it green,
// because the application service it calls has no gate to lose. The real guard is
// `transport/shadowgate_test.go`, which inspects the handlers themselves.
//
// The assertion is relative — both surfaces agree, for every organization — rather than "a shadow
// org 404s", so it keeps holding once organization reachability is fixed (facets.md open seam) and
// the set of visible shadow orgs stops being empty.
func TestOrganizationPointReadAgreesWithTheList_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)
	hidden := seedOrg(t, svc)
	repointDomain(t, svc, hidden.ID, org.DomainID)
	shadowOrg(t, svc, hidden.ID)

	f := domain.OrgFilter{DomainID: &org.DomainID}
	page, err := svc.ListOrganizations(ctx, f, 100, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listed := map[string]bool{}
	for _, o := range page.Orgs {
		listed[o.ID] = true
	}
	// Both orgs are in the domain, and the LIST is unfiltered by visibility, so the admin arm must
	// carry both — otherwise the comparison below is vacuous.
	if !listed[org.ID] || !listed[hidden.ID] {
		t.Fatalf("the admin list is missing one of the seeded orgs — this test would prove nothing")
	}
	for _, id := range []string{org.ID, hidden.ID} {
		if _, err := svc.GetOrganization(ctx, id); err != nil {
			t.Errorf("the admin list returned %s but the point read refused it: %v", id, err)
		}
	}
}

// TestOrganizationStatsNonAdminWithNoSubjectReadsNothing: an empty subject selects the ADMIN arm in
// SQL, so a caller who is neither an admin nor an identified person (a machine principal —
// pep.SubjectAuthority returns ("", false) for one) must read NOTHING rather than every organization
// on the instance. pkg/stats.Compute owns the rule; this proves the org path is wired through it.
func TestOrganizationStatsNonAdminWithNoSubjectReadsNothing_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	res, err := svc.OrganizationStats(ctx, "", false, domain.OrgFilter{DomainID: &org.DomainID}, allOrgFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("a non-admin with no subject got totalCount %d and %d distributions — it must read "+
			"nothing, never the admin arm", res.TotalCount, len(res.Distributions))
	}
}

// TestOrgFilterRejectsMalformedValues: an out-of-set enum and a non-RID domain are caller mistakes and
// must be 400s, not 500s and not silently-empty pages. Both arms validate through one Validate(), so
// an admin and a scoped caller reject identically.
func TestOrgFilterRejectsMalformedValues_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	bad := "not-a-state"
	notRID := "12345"
	for name, f := range map[string]domain.OrgFilter{
		"state":      {State: &bad},
		"visibility": {Visibility: &bad},
		"domain":     {DomainID: &notRID},
	} {
		if _, err := svc.OrganizationStats(ctx, "", true, f, allOrgFacets(t)); err == nil {
			t.Errorf("%s: a malformed filter was accepted by the stats path", name)
		}
		if _, err := svc.ListOrganizations(ctx, f, 10, ""); err == nil {
			t.Errorf("%s: a malformed filter was accepted by the list path", name)
		}
	}
}

// ---------------------------------------------------------------------------- helpers

// repointDomain moves an org into another domain so several test orgs share one domain filter, which
// is what keeps these assertions isolated on a persistent test database.
func repointDomain(t *testing.T, svc *application.Service, orgID, domainID string) {
	t.Helper()
	if _, err := svc.UpdateOrganization(context.Background(), orgID, domain.OrgPatch{DomainID: &domainID}); err != nil {
		t.Fatalf("repoint domain: %v", err)
	}
}

func shadowOrg(t *testing.T, svc *application.Service, orgID string) {
	t.Helper()
	v := domain.VisibilityShadow
	if _, err := svc.UpdateOrganization(context.Background(), orgID, domain.OrgPatch{Visibility: &v}); err != nil {
		t.Fatalf("shadow org: %v", err)
	}
}

func suspendOrg(t *testing.T, svc *application.Service, orgID string) {
	t.Helper()
	if _, err := svc.TransitionOrganization(context.Background(), orgID, domain.StateSuspended, "facet test"); err != nil {
		t.Fatalf("suspend org: %v", err)
	}
}

// grantSubtreeRead gives the subject a role carrying a '*.read' permission, assigned subtree-scoped
// over one unit — written directly rather than through the authz service, because this file is a
// tenant test and what it needs is the ROW shape the reach probe reads.
// commandGraph returns the org's default authority-bearing graph, which a subtree grant must name
// (authz_role_assignments_graph_scope: graph_id is NOT NULL exactly when scope='subtree').
func commandGraph(t *testing.T, svc *application.Service, orgID string) string {
	t.Helper()
	graphs, err := svc.ListGraphs(context.Background(), &orgID)
	if err != nil {
		t.Fatalf("list graphs: %v", err)
	}
	for _, g := range graphs {
		if g.Code == "command" {
			return g.ID
		}
	}
	t.Fatal("no command graph — createOrganization is supposed to seed one")
	return ""
}

func grantSubtreeRead(t *testing.T, pool *pgxpool.Pool, subjectPersonID, unitID, graphID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Org reach probe') RETURNING id`,
		uniqueCode(t, "reachrole")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'unit.read')`,
		roleID); err != nil {
		t.Fatalf("grant permission: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		VALUES ($1, $2, $3, 'subtree', $4)`, subjectPersonID, roleID, unitID, graphID); err != nil {
		t.Fatalf("assign role: %v", err)
	}
}

func readableUnitCount(t *testing.T, pool *pgxpool.Pool, subjectPersonID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM oikumenea.authz_readable_units($1::uuid)`, subjectPersonID).Scan(&n); err != nil {
		t.Fatalf("reach count: %v", err)
	}
	return n
}
