// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for GH-36: CreateChildOrg was structurally unable to pass tenant_units RLS for
// ANY non-admin person-shaped caller, even with a real subtree-scoped religionorg.manage grant on
// the parent unit — the WITH CHECK on the new child's own INSERT required a tenant_unit_closure row
// that could not exist yet (it was populated later, in AddEdge's separate transaction). The fix
// (tenant.Service.CreateUnitWithEdge) seeds closure for the child BEFORE its own INSERT, in one
// transaction, so the check finds the subtree match.
//
// This connects as the NON-superuser application role `oikumenea` (the only way RLS is in force) and
// proves:
//   - a person with a genuine subtree grant on the parent unit can now CreateChildOrg successfully
//     (the exact GH-36 repro);
//   - the created child's tenant_unit_closure and tenant_unit_edges rows exist and are correct;
//   - a grant-less stranger still cannot (the fix did not widen reach beyond the real grant).
//
// Run:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestRLSCreateChildOrg ./internal/religion/...
package religion_test

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// restrictedPool rewrites the superuser test DSN's userinfo to the non-superuser app login role and
// connects with it, so RLS policies are actually enforced (a superuser bypasses RLS entirely).
func restrictedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := defaultTestDSN
	if v := os.Getenv("OIKUMENEA_TEST_DSN"); v != "" {
		dsn = v
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse test dsn: %v", err)
	}
	u.User = url.UserPassword("oikumenea", "dev")
	pool, err := pdb.NewPool(context.Background(), u.String(), "local")
	if err != nil {
		t.Skipf("restricted role not provisioned (CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app): %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedSubtreeOrgManageGrant gives readerID a real, non-bootstrap religionorg.manage grant over the
// canonical graph's subtree rooted at parentUnitID — the exact grant shape GH-36 reports as unable to
// ever pass tenant_units_reach's WITH CHECK on a freshly created child unit. The role also carries
// religion.read (any real operational role does — you can't usefully manage what you can't see):
// CreateChildOrg's edge insert uses RETURNING, and Postgres additionally enforces the SELECT-applicable
// USING policy on a RETURNING row, which needs READ reach specifically — orthogonal to GH-36's WRITE-
// reach bug, but a write-only permission would trip it and obscure the fix being tested here.
func seedSubtreeOrgManageGrant(t *testing.T, pool *pgxpool.Pool, readerID, parentUnitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'GH-36 subtree org-manage role') RETURNING id`,
		uniq("gh36-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) SELECT $1, unnest($2::text[])`,
		roleID, []string{"religionorg.manage", "religion.read"}); err != nil {
		t.Fatalf("seed role permissions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
		SELECT $1, $2, $3, 'subtree', g.id
		FROM oikumenea.tenant_graphs g
		WHERE g.code = 'canonical' AND g.org_id IS NULL AND g.deleted_at IS NULL`,
		readerID, roleID, parentUnitID); err != nil {
		t.Fatalf("seed subtree assignment: %v", err)
	}
}

// TestRLSCreateChildOrg_NonAdminPersonWithSubtreeGrant reproduces GH-36 end to end: before the fix,
// this failed on every attempt with "new row violates row-level security policy for table
// tenant_units", regardless of how genuine the caller's subtree grant was.
func TestRLSCreateChildOrg_NonAdminPersonWithSubtreeGrant(t *testing.T) {
	super := newPool(t)
	app := restrictedPool(t)
	ctx := context.Background()

	parent := seedUnit(t, super)
	reader := seedPerson(t, super, "GH-36 reader")
	stranger := seedPerson(t, super, "GH-36 stranger")
	seedSubtreeOrgManageGrant(t, super, reader, parent)

	svc := newService(t, super) // bound to the superuser pool, but every call below pins a scoped
	// restricted connection into ctx (db.RequestQuerier prefers a pinned conn over s.pool), so the
	// actual writes run under the reader's/stranger's real RLS reach, not as superuser.

	t.Run("a genuine subtree grant on the parent now lets CreateChildOrg succeed", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: reader})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		rctx := pdb.WithConn(ctx, conn)

		prof, err := svc.CreateChildOrg(rctx, parent, uniq("gh36-child"), "GH-36 Child Parish", "", "", "")
		if err != nil {
			t.Fatalf("CreateChildOrg should succeed for a non-admin person with a genuine subtree grant on the parent, got: %v", err)
		}
		if prof.UnitID == "" {
			t.Fatal("expected a created child unit profile")
		}

		// Verify (as superuser) the closure + edge rows this success depends on actually landed.
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

	t.Run("a grant-less stranger still cannot create a child under the same parent", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: stranger})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		rctx := pdb.WithConn(ctx, conn)

		if _, err := svc.CreateChildOrg(rctx, parent, uniq("gh36-stranger-child"), "Should Not Exist", "", "", ""); err == nil {
			t.Fatal("expected a grant-less stranger's CreateChildOrg to fail, it succeeded")
		}
	})
}
