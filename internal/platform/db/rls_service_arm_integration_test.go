// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the MACHINE reach arm of the RLS backstop (M55 — the deferred RLS arm of
// D-ServiceIdentities / D-ConnectorPlane, migration 0042). Companion to rls_integration_test.go: it
// connects as the NON-superuser application role and proves, against a real migrated Postgres, that a
// service principal pinned via app.principal_id (pdb.RLSState{PrincipalID}) reaches EXACTLY its
// org-confined grant's organization:
//   - an org-O read grant makes O's shadow units visible and another org's invisible;
//   - a read-only grant passes USING but is rejected by WITH CHECK on write; a write grant passes;
//   - a principal can CREATE a brand-new unit in its org (the WITH CHECK passes via the trigger-
//     populated authz_unit_org projection — the edgeless/not-yet-in-closure case), and a cross-org
//     create is rejected at the DB;
//   - an INSTANCE-WIDE grant (org_id NULL) confers NO operational reach (blast-radius boundary);
//   - revocation is LIVE on an already-pinned connection (grants are read live, no snapshot);
//   - the authz_unit_org projection is trigger-maintained and readable by the plain app role (exempt).
//
// Run (same DSN contract as rls_integration_test.go):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestRLSServiceArm ./internal/platform/db/...
package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// svcArmFixture: two organizations, each with one shadow (reach-scoped) unit, and one machine
// principal whose grants each subtest sets. orgDomain is O's domain_id, needed for a principal-driven
// unit INSERT (org_id + domain_id are NOT NULL).
type svcArmFixture struct {
	orgO, orgP string // organization RIDs
	orgODomain string // O's domain_id (for principal-created units)
	uO, uP     string // shadow unit RIDs, one per org
	principal  string // service-principal RID
}

func seedServiceArmFixture(t *testing.T, super *pgxpool.Pool) svcArmFixture {
	t.Helper()
	ctx := context.Background()
	if _, err := super.Exec(ctx, `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('rls-svc-domain','RLS Svc Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'rls-svc-org-o','RLS Svc Org O', d.id FROM oikumenea.tenant_domains d WHERE d.code='rls-svc-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'rls-svc-org-p','RLS Svc Org P', d.id FROM oikumenea.tenant_domains d WHERE d.code='rls-svc-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed svc-arm orgs: %v", err)
	}

	var f svcArmFixture
	if err := super.QueryRow(ctx,
		`SELECT id, domain_id FROM oikumenea.tenant_organizations WHERE code='rls-svc-org-o'`).Scan(&f.orgO, &f.orgODomain); err != nil {
		t.Fatalf("read org O: %v", err)
	}
	if err := super.QueryRow(ctx,
		`SELECT id FROM oikumenea.tenant_organizations WHERE code='rls-svc-org-p'`).Scan(&f.orgP); err != nil {
		t.Fatalf("read org P: %v", err)
	}

	unit := func(name, org string) string {
		var id string
		if err := super.QueryRow(ctx, `
			INSERT INTO oikumenea.tenant_units (name, visibility, org_id, domain_id)
			SELECT $1, 'shadow', o.id, o.domain_id FROM oikumenea.tenant_organizations o WHERE o.id=$2
			RETURNING id`, name, org).Scan(&id); err != nil {
			t.Fatalf("seed unit %s: %v", name, err)
		}
		return id
	}
	f.uO = unit("RLS svc O unit", f.orgO)
	f.uP = unit("RLS svc P unit", f.orgP)

	if err := super.QueryRow(ctx, `
		INSERT INTO oikumenea.account_service_principals (code, name, issuer, subject, status)
		VALUES ('rls-svc-connector','RLS Svc Connector','urn:test:rls-svc-arm','rls-svc-arm-subject','active')
		ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&f.principal); err != nil {
		t.Fatalf("seed principal: %v", err)
	}
	return f
}

// setGrants replaces the principal's grants with the given (permission_code, org_id) pairs; an empty
// org string means an instance-wide (org_id NULL) grant.
func setGrants(t *testing.T, super *pgxpool.Pool, principal string, pairs ...[2]string) {
	t.Helper()
	ctx := context.Background()
	if _, err := super.Exec(ctx, `DELETE FROM oikumenea.authz_principal_grants WHERE principal_id=$1`, principal); err != nil {
		t.Fatalf("reset grants: %v", err)
	}
	for _, p := range pairs {
		var org any
		if p[1] != "" {
			org = p[1]
		}
		if _, err := super.Exec(ctx,
			`INSERT INTO oikumenea.authz_principal_grants (principal_id, permission_code, org_id) VALUES ($1,$2,$3)`,
			principal, p[0], org); err != nil {
			t.Fatalf("insert grant %s/%v: %v", p[0], p[1], err)
		}
	}
}

func TestRLSServiceArm(t *testing.T) {
	ctx := context.Background()
	super, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer super.Close()
	f := seedServiceArmFixture(t, super)

	app, err := pdb.NewPool(ctx, restrictedDSN(t), "local")
	if err != nil {
		t.Skipf("restricted role not provisioned: %v", err)
	}
	defer app.Close()

	// The trigger populates the exempt projection on the superuser-seeded units, and the plain app role
	// can read it (no RLS on authz_unit_org).
	t.Run("authz_unit_org projection is trigger-maintained and exempt-readable", func(t *testing.T) {
		var org string
		if err := app.QueryRow(ctx, `SELECT org_id FROM oikumenea.authz_unit_org WHERE unit_id=$1`, f.uO).Scan(&org); err != nil {
			t.Fatalf("projection row for uO missing (trigger did not fire / not exempt-readable): %v", err)
		}
		if org != f.orgO {
			t.Errorf("projection org_id = %s, want %s", org, f.orgO)
		}
	})

	t.Run("org-confined read grant sees only its org's units", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"unit.read", f.orgO})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uO) {
			t.Error("a unit in the principal's org must be visible")
		}
		if visible(ctx, t, conn, f.uP) {
			t.Error("a unit in another org must be hidden from the principal")
		}
	})

	t.Run("instance-wide (org NULL) grant confers no operational reach", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"unit.read", ""})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if visible(ctx, t, conn, f.uO) {
			t.Error("an instance-wide grant must not reach operational org-owned rows")
		}
	})

	// A grant carries a SINGLE read/write class (mirroring the person reach split): a `.read` code
	// confers read reach, any other code confers write reach. A connector therefore holds BOTH a read
	// and a write grant to see AND modify its org's rows — a write-only grant can pass a WITH CHECK but
	// cannot read the row back (e.g. an UPDATE's USING or an INSERT ... RETURNING).
	t.Run("read-only grant is rejected by WITH CHECK on write; read+write grant passes", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"unit.read", f.orgO})
		roConn, releaseRO, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseRO()
		// USING (read) passes so the row is found, but WITH CHECK (write) rejects the new image.
		if _, err := roConn.Exec(ctx,
			`UPDATE oikumenea.tenant_units SET name='svc O (blocked)' WHERE id=$1`, f.uO); err == nil {
			t.Error("update with a read-only principal grant must be rejected by WITH CHECK")
		}

		setGrants(t, super, f.principal, [2]string{"unit.read", f.orgO}, [2]string{"unit.update", f.orgO})
		rwConn, releaseRW, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseRW()
		tag, err := rwConn.Exec(ctx,
			`UPDATE oikumenea.tenant_units SET name='svc O (updated)' WHERE id=$1`, f.uO)
		if err != nil {
			t.Errorf("update within a read+write org grant should succeed, got: %v", err)
		} else if tag.RowsAffected() != 1 {
			t.Errorf("update should touch exactly the org's unit, affected %d rows", tag.RowsAffected())
		}
	})

	t.Run("principal creates a brand-new unit in-org; cross-org create is rejected at the DB", func(t *testing.T) {
		// A connector holds read + write on its org: read so it can RETURN the created row, write so the
		// WITH CHECK passes for the new (edgeless, not-yet-in-closure) unit via its org_id column.
		setGrants(t, super, f.principal, [2]string{"unit.read", f.orgO}, [2]string{"unit.create", f.orgO})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()

		// In-org create: the unit is not in any closure yet (edgeless), so this can ONLY pass via the
		// trigger-populated authz_unit_org projection — the whole point of the M55 design.
		var newUnit string
		if err := conn.QueryRow(ctx, `
			INSERT INTO oikumenea.tenant_units (name, visibility, org_id, domain_id)
			VALUES ('svc created O', 'shadow', $1, $2) RETURNING id`, f.orgO, f.orgODomain).Scan(&newUnit); err != nil {
			t.Fatalf("principal must be able to create a brand-new unit in its org, got: %v", err)
		}

		// Cross-org create: the projection resolves the new unit to org P, which the O-scoped grant does
		// not satisfy -> WITH CHECK rejects it.
		if _, err := conn.Exec(ctx, `
			INSERT INTO oikumenea.tenant_units (name, visibility, org_id, domain_id)
			SELECT 'svc created P', 'shadow', o.id, o.domain_id FROM oikumenea.tenant_organizations o WHERE o.id=$1`,
			f.orgP); err == nil {
			t.Error("a principal must not create a unit in an org it is not granted")
		}
	})

	t.Run("revocation is live on an already-pinned connection", func(t *testing.T) {
		setGrants(t, super, f.principal, [2]string{"unit.read", f.orgO})
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PrincipalID: f.principal})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uO) {
			t.Fatal("precondition: principal must see uO before revoke")
		}
		if _, err := super.Exec(ctx,
			`UPDATE oikumenea.authz_principal_grants SET revoked_at=now() WHERE principal_id=$1`, f.principal); err != nil {
			t.Fatalf("revoke grant: %v", err)
		}
		if visible(ctx, t, conn, f.uO) {
			t.Error("a revoked principal grant must hide rows on the SAME pinned connection (live reach)")
		}
	})
}
