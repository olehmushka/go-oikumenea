//go:build integration

// Integration tests for the tenant module against a real Postgres (M3 exit criteria, D-Graphs /
// D-ClosureIntegrity / D-Audit):
//   - the graph registry is boot-seeded (command default+authority-bearing, operational);
//   - a multi-parent DAG builds and its closure answers ancestors/descendants in one lookup;
//   - a per-graph cycle attempt (and a self-loop) is rejected, while the reverse edge in a
//     different graph is allowed;
//   - a lifecycle transition is recorded + audited, and an illegal transition is rejected;
//   - closure verify reports zero drift after incremental maintenance; rebuild is a no-op then;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/...
package tenant_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

// seedGraphsSQL mirrors tenant.Register's boot seed (the integration test builds the application
// service directly, so it seeds the registry itself). Idempotent on the partial-unique code index.
const seedGraphsSQL = `
INSERT INTO oikumenea.tenant_graphs (code, name, is_default, is_authority_bearing) VALUES
  ('command',     'Command',     true,  true),
  ('operational', 'Operational', false, true)
ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, seedGraphsSQL); err != nil {
		t.Fatalf("seed graphs: %v", err)
	}

	auditSvc := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, auditSvc), pool
}

func newAuditSvc(pool *pgxpool.Pool) *auditapp.Service {
	return auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
}

// uniqueCode returns a fresh unit/graph code per test run so repeated runs against a persistent DB
// don't collide on the partial-unique code index.
func uniqueCode(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

func mustCreate(t *testing.T, svc *application.Service, code string) domain.Unit {
	t.Helper()
	u, err := svc.CreateUnit(context.Background(), domain.Unit{Code: code, Name: code})
	if err != nil {
		t.Fatalf("create unit %q: %v", code, err)
	}
	return u
}

// TestSeededGraphs asserts the boot seed produced command (default + authority-bearing) + operational.
func TestSeededGraphs(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	graphs, err := svc.ListGraphs(ctx)
	if err != nil {
		t.Fatalf("list graphs: %v", err)
	}
	byCode := make(map[string]domain.Graph)
	for _, g := range graphs {
		byCode[g.Code] = g
	}
	cmd, ok := byCode["command"]
	if !ok || !cmd.IsDefault || !cmd.IsAuthorityBearing {
		t.Fatalf("expected command default+authority-bearing, got %+v (present=%v)", cmd, ok)
	}
	op, ok := byCode["operational"]
	if !ok || op.IsDefault || !op.IsAuthorityBearing {
		t.Fatalf("expected operational non-default authority-bearing, got %+v (present=%v)", op, ok)
	}
}

// TestCreateUnitWritesAuditRow is the headline D-Audit guarantee: the unit and its audit row are
// both readable after the write, sharing one transaction.
func TestCreateUnitWritesAuditRow(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)

	u := mustCreate(t, svc, uniqueCode(t, "unit"))

	got, err := svc.GetUnit(ctx, u.ID)
	if err != nil {
		t.Fatalf("get unit: %v", err)
	}
	if got.Code != u.Code || got.State != domain.StateActive || got.Visibility != domain.VisibilityPublic {
		t.Fatalf("unexpected unit: %+v", got)
	}

	tt := "unit"
	page, err := newAuditSvc(pool).Query(ctx, auditapp.QueryParams{TargetType: &tt, TargetID: &u.ID})
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 audit row for the new unit, got %d", len(page.Entries))
	}
	e := page.Entries[0]
	if e.ActorType != auditdomain.ActorSystem || e.Subsystem != "tenant-admin" || e.Action != "unit.create" {
		t.Fatalf("unexpected audit entry: %+v", e)
	}
}

// TestMultiParentDAGAndClosure builds a diamond (d has two parents) and checks the closure answers.
func TestMultiParentDAGAndClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	a := mustCreate(t, svc, uniqueCode(t, "a"))
	b := mustCreate(t, svc, uniqueCode(t, "b"))
	c := mustCreate(t, svc, uniqueCode(t, "c"))
	d := mustCreate(t, svc, uniqueCode(t, "d"))

	// command edges: a->b, a->c, b->d, c->d (AddEdge(child, parent, graph)).
	for _, e := range [][2]string{{b.ID, a.ID}, {c.ID, a.ID}, {d.ID, b.ID}, {d.ID, c.ID}} {
		if _, err := svc.AddEdge(ctx, e[0], e[1], "command"); err != nil {
			t.Fatalf("add edge %v: %v", e, err)
		}
	}

	ancestors, err := svc.Ancestors(ctx, d.ID, "command")
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	gotAnc := idDepth(ancestors)
	if len(gotAnc) != 3 || gotAnc[a.ID] != 2 || gotAnc[b.ID] != 1 || gotAnc[c.ID] != 1 {
		t.Fatalf("unexpected ancestors of d: %+v", gotAnc)
	}

	desc, err := svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil {
		t.Fatalf("descendants: %v", err)
	}
	gotDesc := idDepth(desc.Refs)
	if len(gotDesc) != 3 || gotDesc[b.ID] != 1 || gotDesc[c.ID] != 1 || gotDesc[d.ID] != 2 {
		t.Fatalf("unexpected descendants of a: %+v", gotDesc)
	}
}

// TestCycleRejectedAndCrossGraphAllowed proves per-graph DAG enforcement and cross-graph freedom.
func TestCycleRejectedAndCrossGraphAllowed(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	a := mustCreate(t, svc, uniqueCode(t, "a"))
	b := mustCreate(t, svc, uniqueCode(t, "b"))

	// command: a->b.
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add a->b: %v", err)
	}
	// command: b->a would close a cycle.
	if _, err := svc.AddEdge(ctx, a.ID, b.ID, "command"); !errors.Is(err, domain.ErrUnitCycle) {
		t.Fatalf("expected ErrUnitCycle for b->a in command, got %v", err)
	}
	// self-loop is the degenerate cycle.
	if _, err := svc.AddEdge(ctx, a.ID, a.ID, "command"); !errors.Is(err, domain.ErrUnitCycle) {
		t.Fatalf("expected ErrUnitCycle for self-loop, got %v", err)
	}
	// operational: b->a is legal (a different graph; cross-graph cycles are allowed).
	if _, err := svc.AddEdge(ctx, a.ID, b.ID, "operational"); err != nil {
		t.Fatalf("expected b->a allowed in operational, got %v", err)
	}
}

// TestTransitionRecordedAndAudited transitions a unit and checks the audit row + illegal rejection.
func TestTransitionRecordedAndAudited(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)

	u := mustCreate(t, svc, uniqueCode(t, "unit"))

	suspended, err := svc.TransitionUnit(ctx, u.ID, domain.StateSuspended, "drill")
	if err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if suspended.State != domain.StateSuspended {
		t.Fatalf("expected suspended, got %s", suspended.State)
	}

	tt := "unit"
	action := "unit.transition"
	page, err := newAuditSvc(pool).Query(ctx, auditapp.QueryParams{TargetType: &tt, TargetID: &u.ID, Action: &action})
	if err != nil {
		t.Fatalf("audit query: %v", err)
	}
	if len(page.Entries) != 1 {
		t.Fatalf("expected 1 transition audit row, got %d", len(page.Entries))
	}

	// archived -> suspended is illegal; first archive, then attempt the illegal hop.
	if _, err := svc.TransitionUnit(ctx, u.ID, domain.StateArchived, ""); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := svc.TransitionUnit(ctx, u.ID, domain.StateSuspended, ""); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition archived->suspended, got %v", err)
	}
}

// TestVerifyAndRebuildClosure asserts incremental maintenance leaves no drift, and rebuild agrees.
func TestVerifyAndRebuildClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	a := mustCreate(t, svc, uniqueCode(t, "a"))
	b := mustCreate(t, svc, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}

	cmd := "command"
	reports, err := svc.VerifyClosure(ctx, &cmd)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(reports) != 1 || reports[0].InDrift || reports[0].MissingCount != 0 || reports[0].ExtraCount != 0 {
		t.Fatalf("expected zero drift after maintenance, got %+v", reports)
	}

	rebuilt, err := svc.RebuildClosure(ctx, &cmd)
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if len(rebuilt) != 1 {
		t.Fatalf("expected one rebuild report, got %d", len(rebuilt))
	}
	// closure is consistent again after rebuild.
	reports, err = svc.VerifyClosure(ctx, &cmd)
	if err != nil {
		t.Fatalf("verify after rebuild: %v", err)
	}
	if reports[0].InDrift {
		t.Fatalf("expected no drift after rebuild, got %+v", reports[0])
	}
}

// TestRemoveEdgeUpdatesClosure detaches an edge and confirms the descendant disappears.
func TestRemoveEdgeUpdatesClosure(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	a := mustCreate(t, svc, uniqueCode(t, "a"))
	b := mustCreate(t, svc, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	desc, err := svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil || len(desc.Refs) != 1 {
		t.Fatalf("expected 1 descendant before remove, got %d (err=%v)", len(desc.Refs), err)
	}
	if err := svc.RemoveEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("remove edge: %v", err)
	}
	desc, err = svc.Descendants(ctx, a.ID, "command", 0, "")
	if err != nil {
		t.Fatalf("descendants after remove: %v", err)
	}
	if len(desc.Refs) != 0 {
		t.Fatalf("expected 0 descendants after remove, got %d", len(desc.Refs))
	}
}

func idDepth(refs []domain.UnitRef) map[string]int {
	m := make(map[string]int, len(refs))
	for _, r := range refs {
		m[r.ID] = r.Depth
	}
	return m
}
