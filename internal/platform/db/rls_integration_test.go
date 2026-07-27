// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the live-reach RLS backstop (D-RLSDefenseInDepth as reshaped by
// D-RLSLiveReach, migration 0011). It connects as the NON-superuser application role `oikumenea`
// (the only way RLS is in force — a superuser bypasses it) and proves, against a real migrated
// Postgres, that with the two O(1) GUCs (app.person_id + app.is_instance_admin):
//   - with no GUCs a unit-scoped read returns nothing (a forgotten-filter read leaks nothing);
//   - reads are filtered to the subject's LIVE reach computed by oikumenea.authz_unit_in_reach
//     from their real role assignments (no unit-list GUC exists anymore);
//   - public units stay selectable regardless of reach (F-002 public-read policy);
//   - the app.is_instance_admin GUC flag bypasses the predicate (the instance plane);
//   - a write against a unit outside the subject's WRITE reach (read-only role) is rejected by the
//     policy's WITH CHECK, while a read+write grant passes;
//   - REVOCATION IS LIVE: revoking the assignment hides the rows on the very same pinned
//     connection — stronger than the old snapshot-at-request-start GUCs (D-RLSLiveReach).
//
// It also exercises db.AcquireScoped, which sets/resets those GUCs on a pinned connection in one
// round trip each way. The test needs the restricted login role provisioned (see .env.example /
// migration 0011):
//
//	CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;
//
// Run (the *superuser* DSN seeds; the test derives the restricted DSN from it by swapping the user):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/platform/db/...
package db_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// rowQuerier is the QueryRow surface shared by *pgxpool.Pool and *pgxpool.Conn.
type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func superuserDSN() string {
	if dsn := os.Getenv("OIKUMENEA_TEST_DSN"); dsn != "" {
		return dsn
	}
	return defaultTestDSN
}

// restrictedDSN rewrites the superuser DSN's userinfo to the non-superuser app login role.
func restrictedDSN(t *testing.T) string {
	u, err := url.Parse(superuserDSN())
	if err != nil {
		t.Fatalf("parse test dsn: %v", err)
	}
	u.User = url.UserPassword("oikumenea", "dev")
	return u.String()
}

// rlsFixture is the seeded world: three shadow units, one public unit, and three subjects with real
// authority rows (the live policies read authz_role_assignments directly).
type rlsFixture struct {
	uReadable, uHidden, uPublic, uWritable string
	reader, writer, stranger               string // person RIDs
	readerAssignment                       string // assignment RID (revoked by the live-revocation case)
}

func seedRLSFixture(t *testing.T, super *pgxpool.Pool) rlsFixture {
	t.Helper()
	ctx := context.Background()
	if _, err := super.Exec(ctx, `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('rls-test-domain','RLS Test Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'rls-test-org','RLS Test Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='rls-test-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed rls org: %v", err)
	}

	var f rlsFixture
	unit := func(name, visibility string) string {
		var id string
		if err := super.QueryRow(ctx, `
			INSERT INTO oikumenea.tenant_units (name, visibility, org_id, domain_id)
			SELECT $1, $2, o.id, o.domain_id FROM oikumenea.tenant_organizations o WHERE o.code='rls-test-org'
			RETURNING id`, name, visibility).Scan(&id); err != nil {
			t.Fatalf("seed unit %s: %v", name, err)
		}
		return id
	}
	// Shadow units are governed solely by the reach predicate (not the public-read exception).
	f.uReadable = unit("RLS readable", "shadow")
	f.uHidden = unit("RLS hidden", "shadow")
	f.uWritable = unit("RLS writable", "shadow")
	f.uPublic = unit("RLS public", "public")

	person := func(name string) string {
		var id string
		if err := super.QueryRow(ctx,
			`INSERT INTO oikumenea.person_persons (display_name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
			t.Fatalf("seed person %s: %v", name, err)
		}
		return id
	}
	f.reader = person("RLS reader")
	f.writer = person("RLS writer")
	f.stranger = person("RLS stranger")

	role := func(code string, perms ...string) string {
		var id string
		if err := super.QueryRow(ctx,
			`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'RLS test role') RETURNING id`, code).Scan(&id); err != nil {
			t.Fatalf("seed role %s: %v", code, err)
		}
		for _, p := range perms {
			if _, err := super.Exec(ctx,
				`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, $2)`, id, p); err != nil {
				t.Fatalf("seed perm %s: %v", p, err)
			}
		}
		return id
	}
	// Unique-per-run role codes: tenant_units RIDs make good suffixes.
	readRole := role("rls-read-"+f.uReadable[24:], "unit.read")
	rwRole := role("rls-rw-"+f.uReadable[24:], "unit.read", "unit.update")

	grant := func(subject, roleID, target string) string {
		var id string
		if err := super.QueryRow(ctx, `
			INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
			VALUES ($1, $2, $3, 'unit') RETURNING id`, subject, roleID, target).Scan(&id); err != nil {
			t.Fatalf("seed assignment: %v", err)
		}
		return id
	}
	f.readerAssignment = grant(f.reader, readRole, f.uReadable)
	grant(f.writer, rwRole, f.uWritable)
	return f
}

func TestRLSBackstop(t *testing.T) {
	ctx := context.Background()

	// Superuser pool seeds the fixture (bypassing RLS). NewPool sets the app.environment GUC every
	// connection carries (vestigial, D-ResourceIdentifiers).
	super, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer super.Close()
	f := seedRLSFixture(t, super)

	// Restricted pool: the non-superuser role, so the policies apply.
	app, err := pdb.NewPool(ctx, restrictedDSN(t), "local")
	if err != nil {
		t.Skipf("restricted role not provisioned (CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app): %v", err)
	}
	defer app.Close()

	// Confirm RLS is actually in force: a raw pooled connection (no app.* GUCs) must hide the
	// seeded shadow unit even though the reader has a grant — no subject GUC, no reach.
	if visible(ctx, t, app, f.uReadable) {
		t.Fatal("RLS not enforced: app role sees a shadow unit with no app.person_id GUC")
	}

	t.Run("live reach filters reads by the subject's real assignments", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: f.reader})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uReadable) {
			t.Error("a unit in the subject's read reach should be visible")
		}
		if visible(ctx, t, conn, f.uHidden) {
			t.Error("a unit outside the subject's reach must be hidden")
		}
	})

	t.Run("a grant-less subject sees no shadow units", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: f.stranger})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if visible(ctx, t, conn, f.uReadable) {
			t.Error("a grant-less subject must not see a shadow unit")
		}
	})

	t.Run("public units are selectable regardless of reach (F-002)", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uPublic) {
			t.Error("a public unit must be selectable even with no subject")
		}
		if visible(ctx, t, conn, f.uHidden) {
			t.Error("a shadow unit out of reach must stay hidden")
		}
	})

	t.Run("instance-admin GUC bypasses the predicate", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{IsInstanceAdmin: true})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uHidden) {
			t.Error("an instance admin should see every unit")
		}
	})

	t.Run("write outside write reach is rejected by WITH CHECK", func(t *testing.T) {
		// The writer's role carries unit.update (write-bearing) on uWritable -> UPDATE passes.
		okConn, releaseOK, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: f.writer})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseOK()
		if _, err := okConn.Exec(ctx,
			`UPDATE oikumenea.tenant_units SET name = 'RLS writable (updated)' WHERE id = $1`, f.uWritable); err != nil {
			t.Errorf("update within write reach should succeed, got: %v", err)
		}

		// The reader's role is read-only: uReadable is VISIBLE (USING passes) but the WITH CHECK
		// requires write reach -> the update must be rejected.
		roConn, releaseRO, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: f.reader})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseRO()
		if _, err := roConn.Exec(ctx,
			`UPDATE oikumenea.tenant_units SET name = 'RLS readable (updated)' WHERE id = $1`, f.uReadable); err == nil {
			t.Error("update with a read-only grant must be rejected by RLS WITH CHECK")
		}
	})

	t.Run("revocation is live on an already-pinned connection", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{PersonID: f.reader})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, f.uReadable) {
			t.Fatal("precondition: reader must see uReadable before the revoke")
		}
		if _, err := super.Exec(ctx,
			`UPDATE oikumenea.authz_role_assignments SET revoked_at = now() WHERE id = $1`, f.readerAssignment); err != nil {
			t.Fatalf("revoke assignment: %v", err)
		}
		if visible(ctx, t, conn, f.uReadable) {
			t.Error("D-RLSLiveReach: a revoked assignment must hide rows on the SAME pinned connection")
		}
	})
}

// visible reports whether the given unit id is selectable on the querier (i.e. passes RLS).
func visible(ctx context.Context, t *testing.T, q rowQuerier, id string) bool {
	t.Helper()
	var got string
	err := q.QueryRow(ctx, "SELECT id FROM oikumenea.tenant_units WHERE id = $1", id).Scan(&got)
	if err == nil {
		return true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	t.Fatalf("query unit %s: %v", id, err)
	return false
}
