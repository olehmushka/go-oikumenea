//go:build integration

// Scale-world measurement for the incremental closure maintenance (review-2026-07 Phase 2 / M48).
// Runs single edge attach/detach operations against the synthetic 100k-unit graph seeded by
// scripts/seed-scale and logs wall time, statement counts, and closure-row deltas — mid-tree and
// near-root — next to one timed full RebuildClosure (≡ the pre-M48 cost of EVERY edge edit).
// It asserts nothing about performance — it measures; the numbers land in
// docs/architecture/review-2026-07.md § Measurements. The world is restored (fresh probe units
// are detached again; the moved mid-tree edge is re-attached), so TestScaleMeasure re-runs are
// unaffected.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestScaleClosureMeasure -v ./internal/tenant/
package tenant_test

import (
	"context"
	"os"
	"testing"
	"time"

	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

const scaleGraphCode = "scale-command"

func TestScaleClosureMeasure(t *testing.T) {
	dsn := os.Getenv("OIKUMENEA_SCALE_DSN")
	if dsn == "" {
		t.Skip("set OIKUMENEA_SCALE_DSN to the seed-scale database (scripts/seed-scale) to run the measurement harness")
	}
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, dsn, "local")
	if err != nil {
		t.Fatalf("connect scale db: %v", err)
	}
	t.Cleanup(pool.Close)

	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)

	var graphID, orgID, domainID string
	if err := pool.QueryRow(ctx, `
		SELECT g.id, o.id, o.domain_id
		FROM oikumenea.tenant_graphs g JOIN oikumenea.tenant_organizations o ON o.id = g.org_id
		WHERE g.code = $1`, scaleGraphCode).Scan(&graphID, &orgID, &domainID); err != nil {
		t.Fatalf("scale graph not found (run scripts/seed-scale first): %v", err)
	}

	closureRows := func() int64 {
		var n int64
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM oikumenea.tenant_unit_closure WHERE graph_id = $1", graphID).Scan(&n); err != nil {
			t.Fatalf("count closure: %v", err)
		}
		return n
	}
	subtreeSize := func(unitID string) int64 {
		var n int64
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM oikumenea.tenant_unit_closure WHERE graph_id = $1 AND ancestor_id = $2",
			graphID, unitID).Scan(&n); err != nil {
			t.Fatalf("subtree size: %v", err)
		}
		return n
	}

	// Anchors: a mid-tree unit (subtree 200..20k, the seed-scale mid-probe shape) and the largest
	// non-root subtree (near-root).
	var midUnit, nearRootUnit string
	if err := pool.QueryRow(ctx, `
		SELECT ancestor_id FROM oikumenea.tenant_unit_closure WHERE graph_id = $1
		GROUP BY ancestor_id HAVING count(*) BETWEEN 200 AND 20000 ORDER BY ancestor_id LIMIT 1`,
		graphID).Scan(&midUnit); err != nil {
		t.Fatalf("pick mid unit: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT c.ancestor_id FROM oikumenea.tenant_unit_closure c
		WHERE c.graph_id = $1 AND EXISTS (
		  SELECT 1 FROM oikumenea.tenant_unit_edges e WHERE e.graph_id = $1 AND e.child_id = c.ancestor_id)
		GROUP BY c.ancestor_id ORDER BY count(*) DESC LIMIT 1`,
		graphID).Scan(&nearRootUnit); err != nil {
		t.Fatalf("pick near-root unit: %v", err)
	}
	t.Logf("anchors: mid subtree=%d units, near-root subtree=%d units, closure=%d rows",
		subtreeSize(midUnit), subtreeSize(nearRootUnit), closureRows())

	// measureEdge attaches a FRESH leaf under the anchor and detaches it again, logging both ops.
	measureEdge := func(label, anchor string) {
		leaf, err := svc.CreateUnit(ctx, tenantdomain.Unit{
			OrgID: orgID, DomainID: domainID, Name: "m48-probe-" + label,
		})
		if err != nil {
			t.Fatalf("create probe leaf: %v", err)
		}
		before := closureRows()
		cctx, counter := pdb.WithQueryCounter(ctx)
		start := time.Now()
		if _, err := svc.AddEdge(cctx, leaf.ID, anchor, scaleGraphCode); err != nil {
			t.Fatalf("attach %s: %v", label, err)
		}
		t.Logf("%s attach (fresh leaf): %v  statements=%d  closure-row delta=%+d",
			label, time.Since(start).Round(time.Millisecond), counter.Count(), closureRows()-before)

		before = closureRows()
		cctx, counter = pdb.WithQueryCounter(ctx)
		start = time.Now()
		if err := svc.RemoveEdge(cctx, leaf.ID, anchor, scaleGraphCode); err != nil {
			t.Fatalf("detach %s: %v", label, err)
		}
		t.Logf("%s detach (fresh leaf): %v  statements=%d  closure-row delta=%+d",
			label, time.Since(start).Round(time.Millisecond), counter.Count(), closureRows()-before)
	}
	measureEdge("mid-tree", midUnit)
	measureEdge("near-root", nearRootUnit)

	// The hard direction at real size: detach an EXISTING edge above the mid unit (slice ≈
	// ancestors × its whole subtree), then re-attach to restore the world.
	var midParent string
	if err := pool.QueryRow(ctx,
		"SELECT parent_id FROM oikumenea.tenant_unit_edges WHERE graph_id = $1 AND child_id = $2 LIMIT 1",
		graphID, midUnit).Scan(&midParent); err != nil {
		t.Fatalf("mid unit has no parent edge: %v", err)
	}
	before := closureRows()
	cctx, counter := pdb.WithQueryCounter(ctx)
	start := time.Now()
	if err := svc.RemoveEdge(cctx, midUnit, midParent, scaleGraphCode); err != nil {
		t.Fatalf("detach mid subtree: %v", err)
	}
	t.Logf("mid-subtree detach (existing edge, subtree=%d): %v  statements=%d  closure-row delta=%+d",
		subtreeSize(midUnit), time.Since(start).Round(time.Millisecond), counter.Count(), closureRows()-before)

	before = closureRows()
	cctx, counter = pdb.WithQueryCounter(ctx)
	start = time.Now()
	if _, err := svc.AddEdge(cctx, midUnit, midParent, scaleGraphCode); err != nil {
		t.Fatalf("re-attach mid subtree: %v", err)
	}
	t.Logf("mid-subtree re-attach: %v  statements=%d  closure-row delta=%+d",
		time.Since(start).Round(time.Millisecond), counter.Count(), closureRows()-before)

	// Baseline: one full rebuild of the 100k-unit graph — what EVERY edge edit paid before M48.
	code := scaleGraphCode
	start = time.Now()
	if _, err := svc.RebuildClosure(ctx, &code); err != nil {
		t.Fatalf("rebuild closure: %v", err)
	}
	t.Logf("full RebuildClosure (pre-M48 per-edit cost): %v  closure=%d rows",
		time.Since(start).Round(time.Millisecond), closureRows())
}
