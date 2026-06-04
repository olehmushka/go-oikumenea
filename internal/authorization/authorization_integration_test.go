//go:build integration

// Integration tests for the authorization module against a real Postgres (M7 exit criteria — the PDP
// centerpiece): decisions over the tenant unit-graph closure with per-assignment scope × graph.
//   - `subtree` cascades to descendants over the (authority-bearing) graph; `unit` leaks nothing down;
//   - the union is taken across graphs (command + operational both contribute);
//   - a directory-only graph rejects subtree grants (D-DirectoryGraphs);
//   - expired assignments are inactive at decision time (D-TimeBoundGrants);
//   - the instance-admin plane is allowed everything; instance-scope actions are denied to others;
//   - no self-escalation: granting requires assignment.grant reaching the target;
//   - the effective read/write reach expands a subtree over the closure (D-RLSDefenseInDepth);
//   - base roles are immutable; a role with an instance-scope permission is rejected;
//   - a grant write + its audit row share one transaction.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/postgres?sslmode=disable" \
//	  go test -tags integration ./internal/authorization/...
package authorization_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzadapters "github.com/olegamysk/go-oikumenea/internal/authorization/adapters"
	authzapp "github.com/olegamysk/go-oikumenea/internal/authorization/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"

// seedGraphsSQL mirrors tenant.Register's boot seed (the test bypasses Register).
const seedGraphsSQL = `
INSERT INTO oikumenea.tenant_graphs (code, name, is_default, is_authority_bearing) VALUES
  ('command',     'Command',     true,  true),
  ('operational', 'Operational', false, true)
ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

type harness struct {
	authz  *authzapp.Service
	tenant *tenantapp.Service
	pool   *pgxpool.Pool
}

func newHarness(t *testing.T) harness {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), seedGraphsSQL); err != nil {
		t.Fatalf("seed graphs: %v", err)
	}

	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })

	tenantSvc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)

	pdp := authzdomain.NewPDP(tenantSvc)
	authzSvc := authzapp.NewService(pool, func(conn pdb.DBTX) authzdomain.Repository {
		return authzadapters.NewRepository(conn)
	}, audit, pdp, tenantSvc)

	if err := authzSvc.SeedBaseRoles(context.Background()); err != nil {
		t.Fatalf("seed base roles: %v", err)
	}
	return harness{authz: authzSvc, tenant: tenantSvc, pool: pool}
}

func uniq(prefix string) string { return prefix + "-" + uuid.NewString()[:8] }

func (h harness) seedUnit(t *testing.T) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (code, name) VALUES ($1, 'Unit') RETURNING id`, uniq("unit")).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

func (h harness) seedPerson(t *testing.T) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('P') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

func (h harness) roleID(t *testing.T, code string) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.authz_roles WHERE code = $1 AND deleted_at IS NULL`, code).Scan(&id); err != nil {
		t.Fatalf("role %q: %v", code, err)
	}
	return id
}

func (h harness) grant(t *testing.T, subject, roleID, unit string, scope authzdomain.Scope, graph string) authzdomain.Assignment {
	t.Helper()
	a, err := h.authz.GrantAssignment(context.Background(), authzdomain.GrantInput{
		SubjectPersonID: subject, RoleID: roleID, TargetUnitID: unit, Scope: scope, GraphCode: graph,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	return a
}

func (h harness) allow(t *testing.T, subject, action, unit string) bool {
	t.Helper()
	d, err := h.authz.Decide(context.Background(), subject, action, unit, false)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	return d.Allow
}

// TestSubtreeAndUnitScope: subtree cascades over the command closure; unit scope leaks nothing down.
func TestSubtreeAndUnitScope(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	root, child, grand := h.seedUnit(t), h.seedUnit(t), h.seedUnit(t)
	if _, err := h.tenant.AddEdge(ctx, child, root, "command"); err != nil {
		t.Fatalf("edge root->child: %v", err)
	}
	if _, err := h.tenant.AddEdge(ctx, grand, child, "command"); err != nil {
		t.Fatalf("edge child->grand: %v", err)
	}

	reader, manager := h.seedPerson(t), h.seedPerson(t)
	h.grant(t, reader, h.roleID(t, authzdomain.BaseRoleUnitReader), root, authzdomain.ScopeSubtree, "command")
	h.grant(t, manager, h.roleID(t, authzdomain.BaseRoleUnitManager), child, authzdomain.ScopeUnit, "")

	// subtree reader reaches root, child, grandchild for person.read.
	for _, u := range []string{root, child, grand} {
		if !h.allow(t, reader, "person.read", u) {
			t.Fatalf("reader should read at %s", u)
		}
	}
	// reader has no write permission.
	if h.allow(t, reader, "person.update", child) {
		t.Fatal("reader must not have person.update")
	}
	// unit-scope manager: writes at child only, nothing at grandchild.
	if !h.allow(t, manager, "person.update", child) {
		t.Fatal("unit manager should write at its own unit")
	}
	if h.allow(t, manager, "person.update", grand) {
		t.Fatal("unit scope must not cascade to a descendant")
	}
}

// TestCrossGraphUnion: command + operational grants both contribute at the same unit.
func TestCrossGraphUnion(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	cmdRoot, opRoot, target := h.seedUnit(t), h.seedUnit(t), h.seedUnit(t)
	if _, err := h.tenant.AddEdge(ctx, target, cmdRoot, "command"); err != nil {
		t.Fatalf("command edge: %v", err)
	}
	if _, err := h.tenant.AddEdge(ctx, target, opRoot, "operational"); err != nil {
		t.Fatalf("operational edge: %v", err)
	}
	subj := h.seedPerson(t)
	h.grant(t, subj, h.roleID(t, authzdomain.BaseRoleUnitReader), cmdRoot, authzdomain.ScopeSubtree, "command")
	h.grant(t, subj, h.roleID(t, authzdomain.BaseRoleUnitManager), opRoot, authzdomain.ScopeSubtree, "operational")

	// person.read comes from either; person.update only from the operational manager grant.
	if !h.allow(t, subj, "person.read", target) {
		t.Fatal("read should be satisfied via the command subtree")
	}
	if !h.allow(t, subj, "person.update", target) {
		t.Fatal("update should be satisfied via the operational subtree (union across graphs)")
	}

	// decision-explain names both reaching grants for person.read.
	d, err := h.authz.Decide(ctx, subj, "person.read", target, true)
	if err != nil || !d.Allow {
		t.Fatalf("explain decide: allow=%v err=%v", d.Allow, err)
	}
	if len(d.Via) < 2 {
		t.Fatalf("explain should name >=2 contributing grants, got %d", len(d.Via))
	}
}

// TestDirectoryGraphRejectsSubtree: a subtree grant on a directory-only graph is rejected.
func TestDirectoryGraphRejectsSubtree(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	dirCode := uniq("dir")
	if _, err := h.pool.Exec(ctx,
		`INSERT INTO oikumenea.tenant_graphs (code, name, is_default, is_authority_bearing) VALUES ($1, 'Dir', false, false)`,
		dirCode); err != nil {
		t.Fatalf("seed directory graph: %v", err)
	}
	unit := h.seedUnit(t)
	subj := h.seedPerson(t)
	_, err := h.authz.GrantAssignment(ctx, authzdomain.GrantInput{
		SubjectPersonID: subj, RoleID: h.roleID(t, authzdomain.BaseRoleUnitReader),
		TargetUnitID: unit, Scope: authzdomain.ScopeSubtree, GraphCode: dirCode,
	})
	if !errors.Is(err, authzdomain.ErrNonAuthorityBearingGraph) {
		t.Fatalf("subtree on directory-only graph: want ErrNonAuthorityBearingGraph, got %v", err)
	}
}

// TestExpiryAndInstanceAdmin: expired grants are inactive; the instance plane is allowed everything.
func TestExpiryAndInstanceAdmin(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	unit := h.seedUnit(t)
	subj := h.seedPerson(t)
	past := time.Now().Add(-time.Hour)
	if _, err := h.authz.GrantAssignment(ctx, authzdomain.GrantInput{
		SubjectPersonID: subj, RoleID: h.roleID(t, authzdomain.BaseRoleUnitReader),
		TargetUnitID: unit, Scope: authzdomain.ScopeUnit, ExpiresAt: &past,
	}); err != nil {
		t.Fatalf("grant expired: %v", err)
	}
	if h.allow(t, subj, "person.read", unit) {
		t.Fatal("an expired assignment must not authorize")
	}

	admin := h.seedPerson(t)
	if _, err := h.authz.GrantInstanceAdmin(ctx, admin, ""); err != nil {
		t.Fatalf("grant instance admin: %v", err)
	}
	if !h.allow(t, admin, "graph.manage", "") {
		t.Fatal("instance admin should hold instance-scope actions")
	}
	if !h.allow(t, admin, "person.update", unit) {
		t.Fatal("instance admin should be allowed unit-scoped actions everywhere")
	}
	// a non-admin is denied an instance-scope action even with a unit grant.
	if h.allow(t, subj, "graph.manage", unit) {
		t.Fatal("instance-scope action must be denied to a non-admin")
	}
}

// TestNoSelfEscalation: a grantor lacking assignment.grant at the target cannot grant.
func TestNoSelfEscalation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	unit := h.seedUnit(t)
	grantor, subject := h.seedPerson(t), h.seedPerson(t)
	_, err := h.authz.GrantAssignment(ctx, authzdomain.GrantInput{
		SubjectPersonID: subject, RoleID: h.roleID(t, authzdomain.BaseRoleUnitReader),
		TargetUnitID: unit, Scope: authzdomain.ScopeUnit, GrantedBy: grantor,
	})
	if !errors.Is(err, authzdomain.ErrSelfEscalation) {
		t.Fatalf("grant without authority: want ErrSelfEscalation, got %v", err)
	}

	// a unit-admin (holds assignment.grant) at the unit CAN grant there.
	admin := h.seedPerson(t)
	h.grant(t, admin, h.roleID(t, authzdomain.BaseRoleUnitAdmin), unit, authzdomain.ScopeUnit, "")
	if _, err := h.authz.GrantAssignment(ctx, authzdomain.GrantInput{
		SubjectPersonID: subject, RoleID: h.roleID(t, authzdomain.BaseRoleUnitReader),
		TargetUnitID: unit, Scope: authzdomain.ScopeUnit, GrantedBy: admin,
	}); err != nil {
		t.Fatalf("authorized grant: %v", err)
	}
}

// TestEffectiveReach expands a subtree grant over the closure into the readable set.
func TestEffectiveReach(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	root, child := h.seedUnit(t), h.seedUnit(t)
	if _, err := h.tenant.AddEdge(ctx, child, root, "command"); err != nil {
		t.Fatalf("edge: %v", err)
	}
	subj := h.seedPerson(t)
	h.grant(t, subj, h.roleID(t, authzdomain.BaseRoleUnitReader), root, authzdomain.ScopeSubtree, "command")

	reach, err := h.authz.EffectiveReach(ctx, subj)
	if err != nil {
		t.Fatalf("reach: %v", err)
	}
	for _, u := range []string{root, child} {
		if _, ok := reach.Readable[u]; !ok {
			t.Fatalf("readable reach should include %s", u)
		}
	}
	if len(reach.Writable) != 0 {
		t.Fatalf("a reader's writable reach should be empty, got %d", len(reach.Writable))
	}
}

// TestRoleInvariants: base roles immutable; instance-scope perm rejected; grant audited in one tx.
func TestRoleInvariants(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	// base role immutable.
	base := h.roleID(t, authzdomain.BaseRoleUnitReader)
	name := "hacked"
	if _, err := h.authz.UpdateRole(ctx, base, authzdomain.RolePatch{Name: &name}); !errors.Is(err, authzdomain.ErrRoleIsBase) {
		t.Fatalf("update base role: want ErrRoleIsBase, got %v", err)
	}

	// a custom role with an instance-scope permission is rejected.
	if _, err := h.authz.CreateRole(ctx, authzdomain.Role{
		Code: uniq("role"), Name: "Bad", Permissions: []authzdomain.Permission{authzdomain.PermGraphManage},
	}); !errors.Is(err, authzdomain.ErrRoleInvalid) {
		t.Fatalf("instance-scope perm in role: want ErrRoleInvalid, got %v", err)
	}

	// a valid custom role persists its permission set.
	cr, err := h.authz.CreateRole(ctx, authzdomain.Role{
		Code: uniq("role"), Name: "Readers", Permissions: []authzdomain.Permission{authzdomain.PermPersonRead},
	})
	if err != nil {
		t.Fatalf("create custom role: %v", err)
	}
	got, err := h.authz.GetRole(ctx, cr.ID)
	if err != nil || len(got.Permissions) != 1 || got.Permissions[0] != authzdomain.PermPersonRead {
		t.Fatalf("custom role perms = %+v err=%v", got.Permissions, err)
	}

	// a grant records exactly one audit row keyed to the assignment.
	unit := h.seedUnit(t)
	subj := h.seedPerson(t)
	a := h.grant(t, subj, base, unit, authzdomain.ScopeUnit, "")
	var n int
	if err := h.pool.QueryRow(ctx,
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'assignment.grant' AND actor_type = 'system' AND subsystem = 'authz-admin'",
		a.ID).Scan(&n); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows for %s = %d, want 1", a.ID, n)
	}
}
