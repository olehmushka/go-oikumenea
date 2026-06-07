//go:build integration

// Integration test for the RLS backstop (D-RLSDefenseInDepth, migration 0012). It connects as the
// NON-superuser application role `oikumenea` (the only way RLS is in force — a superuser bypasses it)
// and proves, against a real migrated Postgres, that:
//   - with no app.* GUCs a unit-scoped read returns nothing (a forgotten-filter read leaks nothing);
//   - app.readable_units filters reads to exactly the reachable units;
//   - the app.is_instance_admin GUC flag bypasses the predicate (the instance plane);
//   - a write to a unit outside app.writable_units is rejected by the policy's WITH CHECK.
//
// It also exercises db.AcquireScoped, which sets/resets those GUCs on a pinned connection. The test
// needs the restricted login role provisioned (see .env.example / migration 0012):
//
//	CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;
//
// Run (the *superuser* DSN seeds; the test derives the restricted DSN from it by swapping the user):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/postgres?sslmode=disable" \
//	  go test -tags integration ./internal/platform/db/...
package db_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// rowQuerier is the QueryRow surface shared by *pgxpool.Pool and *pgxpool.Conn.
type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"

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

func TestRLSBackstop(t *testing.T) {
	ctx := context.Background()

	// Superuser pool seeds two units (bypassing RLS) and cleans up at the end. NewPool sets the
	// app.environment GUC every connection needs for new_rid (D-ResourceIdentifiers).
	super, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer super.Close()

	// Distinct ids so the test is independent of any pre-existing rows; minted via new_rid so the
	// id RID-shape CHECK holds. Two are seeded now; two are reserved for the write-policy test.
	var idReadable, idHidden, idWriteOK, idWriteDenied string
	for _, p := range []*string{&idReadable, &idHidden, &idWriteOK, &idWriteDenied} {
		if err := super.QueryRow(ctx, "SELECT oikumenea.new_rid('tenant','unit')").Scan(p); err != nil {
			t.Fatalf("mint rid: %v", err)
		}
	}
	mkCode := func(s string) string { return "rls-test-" + s[len(s)-12:] }
	for _, id := range []string{idReadable, idHidden} {
		if _, err := super.Exec(ctx,
			"INSERT INTO oikumenea.tenant_units (id, code, name) VALUES ($1, $2, $3)",
			id, mkCode(id), "RLS test unit"); err != nil {
			t.Fatalf("seed unit: %v", err)
		}
	}
	defer func() {
		_, _ = super.Exec(context.Background(),
			"DELETE FROM oikumenea.tenant_units WHERE id = ANY($1)",
			[]string{idReadable, idHidden, idWriteOK, idWriteDenied})
	}()

	// Restricted pool: the non-superuser role, so the policies apply.
	app, err := pdb.NewPool(ctx, restrictedDSN(t), "local")
	if err != nil {
		t.Skipf("restricted role not provisioned (CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app): %v", err)
	}
	defer app.Close()

	// Confirm RLS is actually in force: a superuser MUST bypass it (sees seeded rows without GUCs);
	// the app role must NOT. A raw pooled connection (no app.* GUCs) must hide the seeded unit.
	if visible(ctx, t, app, idReadable) {
		t.Fatal("RLS not enforced: app role sees a unit with no app.readable_units GUC")
	}

	t.Run("readable_units filters reads", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{ReadableUnits: []string{idReadable}})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, idReadable) {
			t.Error("a unit in readable_units should be visible")
		}
		if visible(ctx, t, conn, idHidden) {
			t.Error("a unit NOT in readable_units must be hidden")
		}
	})

	t.Run("instance-admin GUC bypasses the predicate", func(t *testing.T) {
		conn, release, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{IsInstanceAdmin: true})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer release()
		if !visible(ctx, t, conn, idHidden) {
			t.Error("an instance admin should see every unit")
		}
	})

	t.Run("write outside writable_units is rejected", func(t *testing.T) {
		// In writable reach -> the WITH CHECK passes.
		okConn, releaseOK, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{WritableUnits: []string{idWriteOK}})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseOK()
		if _, err := okConn.Exec(ctx,
			"INSERT INTO oikumenea.tenant_units (id, code, name) VALUES ($1, $2, $3)",
			idWriteOK, mkCode(idWriteOK), "RLS write ok"); err != nil {
			t.Errorf("insert into a writable unit id should succeed, got: %v", err)
		}

		// Not in writable reach -> the WITH CHECK rejects it.
		denyConn, releaseDeny, err := pdb.AcquireScoped(ctx, app, pdb.RLSState{})
		if err != nil {
			t.Fatalf("acquire scoped: %v", err)
		}
		defer releaseDeny()
		_, err = denyConn.Exec(ctx,
			"INSERT INTO oikumenea.tenant_units (id, code, name) VALUES ($1, $2, $3)",
			idWriteDenied, mkCode(idWriteDenied), "RLS write denied")
		if err == nil {
			t.Error("insert with empty writable_units must be rejected by RLS WITH CHECK")
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
