// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for GH-41: CreateChildOrg's tenant_unit_edges insert failed RLS (SQLSTATE 42501,
// a real row-security violation) for a service-principal caller holding an org-scoped
// religionorg.manage grant, even though the immediately preceding tenant_units insert (same
// transaction, same connection) succeeded, and a manual SQL reproduction of the insert (without a
// RETURNING clause) succeeded too.
//
// ROOT CAUSE, confirmed by live diagnosis against a real Postgres (not guessed): InsertEdge's sqlc
// query uses `RETURNING ...`. In this Postgres version, when a row's INSERT WITH CHECK passes but
// the row does NOT also satisfy the table's SELECT-applicable USING check, Postgres raises the SAME
// "new row violates row-level security policy" error for the RETURNING clause — it does not silently
// return zero rows. tenant_unit_edges_reach's USING clause needs a `.read`-suffixed
// authz_principal_org_in_reach match (`(permission_code LIKE '%.read') = NOT wr`); a `.manage`-only
// grant satisfies WITH CHECK (write) but not USING (read; RETURNING's implicit check). This is the
// SAME requirement TestRLSCreateChildOrg_NonAdminPersonWithSubtreeGrant already has to work around
// for a PERSON caller (its seedSubtreeOrgManageGrant grants BOTH religionorg.manage AND
// religion.read, with a comment explaining exactly this) — GH-41 is that same gotcha, previously
// undocumented for a MACHINE caller. This was empirically isolated (not assumed) by: running the
// exact same INSERT without RETURNING on the SAME transaction (it succeeded), then confirming that
// adding the missing religion.read grant made the real RETURNING-based InsertEdge succeed too.
//
// This is not an RLS policy bug or a GUC-propagation bug (GH-41's own suspicion) — the policy and
// connection plumbing both work correctly. It is a genuine, easy-to-hit operational gap: granting a
// service principal ONLY the write-shaped permission for a RETURNING-using write path is
// insufficient. The fix here is the regression test (proving the correct two-grant recipe works, and
// that a manage-only grant fails predictably) plus the doc-comment/decisions-doc note steering future
// operators away from the same mistake.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestRLSCreateChildOrg_ServicePrincipal ./internal/religion/...
package religion_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// seedPrincipal inserts a machine subject directly, since the registry belongs to
// identity-federation (mirrors internal/authorization/principalgrant_integration_test.go's helper of
// the same name).
func seedPrincipal(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.account_service_principals (code, name, issuer, subject)
		 VALUES ($1, $1, 'urn:test', $1) RETURNING id`, code).Scan(&id); err != nil {
		t.Fatalf("seed principal: %v", err)
	}
	return id
}

// seedOrgScopedPrincipalGrant gives principalID a real, org-CONFINED permissionCode grant over the
// SAME organization seedUnit's units belong to (test-religion-org) — the exact shape GH-41 documents
// (GrantPrincipalPermissionRequest.OrgId set; an instance-wide grant "confers NO operational reach"
// per migration 0011_infra.sql's authz_principal_org_in_reach comment).
func seedOrgScopedPrincipalGrant(t *testing.T, pool *pgxpool.Pool, principalID, permissionCode string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, ensureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.authz_principal_grants (principal_id, permission_code, org_id)
		SELECT $1, $2, o.id FROM oikumenea.tenant_organizations o WHERE o.code = 'test-religion-org'`,
		principalID, permissionCode); err != nil {
		t.Fatalf("seed org-scoped principal grant: %v", err)
	}
}

// TestRLSCreateChildOrg_ServicePrincipalWithOrgScopedGrant reproduces GH-41 and pins its fix: a
// service principal needs BOTH an org-scoped religionorg.manage grant (satisfies the INSERT's WITH
// CHECK) AND an org-scoped religion.read grant (satisfies the RETURNING clause's implicit
// SELECT-applicable USING check) to CreateChildOrg successfully — manage alone is not enough, exactly
// as it already isn't for a person caller (see TestRLSCreateChildOrg_NonAdminPersonWithSubtreeGrant's
// seedSubtreeOrgManageGrant).
func TestRLSCreateChildOrg_ServicePrincipalWithOrgScopedGrant(t *testing.T) {
	super := newPool(t)
	app := restrictedPool(t)
	ctx := context.Background()

	t.Run("manage-only: still denied (GH-41's exact repro — RETURNING needs read reach too)", func(t *testing.T) {
		parent := seedUnit(t, super)
		principalID := seedPrincipal(t, super, uniq("gh41-manage-only"))
		seedOrgScopedPrincipalGrant(t, super, principalID, "religionorg.manage")

		svc := newService(t, super)
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: principalID})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		rctx := pdb.WithConn(ctx, conn)

		if _, err := svc.CreateChildOrg(rctx, parent, uniq("gh41-child-denied"), "Should Not Exist", "", "", ""); err == nil {
			t.Fatal("expected CreateChildOrg to fail for a manage-only grant (no read reach for the RETURNING clause), it succeeded")
		}
	})

	t.Run("manage + read: CreateChildOrg succeeds end to end", func(t *testing.T) {
		parent := seedUnit(t, super)
		principalID := seedPrincipal(t, super, uniq("gh41-manage-and-read"))
		seedOrgScopedPrincipalGrant(t, super, principalID, "religionorg.manage")
		seedOrgScopedPrincipalGrant(t, super, principalID, "religion.read")

		svc := newService(t, super) // bound to the superuser pool, but every call below pins a scoped
		// restricted connection into ctx (db.RequestQuerier prefers a pinned conn over s.pool), so the
		// actual writes run under the principal's real RLS reach, not as superuser.

		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: principalID})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		rctx := pdb.WithConn(ctx, conn)

		prof, err := svc.CreateChildOrg(rctx, parent, uniq("gh41-child"), "GH-41 Child Parish", "", "", "")
		if err != nil {
			t.Fatalf("CreateChildOrg should succeed for a service principal with manage+read org-scoped grants, got: %v", err)
		}
		if prof.UnitID == "" {
			t.Fatal("expected a created child unit profile")
		}

		// Verify (as superuser) the closure + edge rows this success depends on actually landed — the
		// exact rows GH-41 reports as missing (the tenant_unit_edges insert failing RLS).
		var closureDepth int
		if err := super.QueryRow(ctx, `
			SELECT c.depth FROM oikumenea.tenant_unit_closure c
			JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
			WHERE g.code = 'canonical' AND g.org_id IS NULL AND c.ancestor_id = $1 AND c.descendant_id = $2`,
			parent, prof.UnitID).Scan(&closureDepth); err != nil {
			t.Fatalf("expected a closure row (parent -> child): %v", err)
		}
		if closureDepth != 1 {
			t.Fatalf("expected closure depth 1 for a direct parent->child edge, got %d", closureDepth)
		}
		var edges int
		if err := super.QueryRow(ctx, `
			SELECT count(*) FROM oikumenea.tenant_unit_edges e
			JOIN oikumenea.tenant_graphs g ON g.id = e.graph_id
			WHERE g.code = 'canonical' AND e.parent_id = $1 AND e.child_id = $2`,
			parent, prof.UnitID).Scan(&edges); err != nil {
			t.Fatalf("check edge: %v", err)
		}
		if edges != 1 {
			t.Fatalf("expected 1 canonical edge parent->child, got %d", edges)
		}
	})
}
