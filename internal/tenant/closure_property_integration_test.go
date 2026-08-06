// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Property/differential tests for the incremental closure maintenance (M48, review R-04):
// random attach/detach sequences on a random DAG, driven through the application service, are
// checked after EVERY step against an independent Go-side BFS oracle (edge list kept in the
// test) — deliberately not against RebuildClosureForGraph alone, since verify/rebuild share the
// same recursive-CTE shape and a shared bug would hide. VerifyClosureForGraph (depth-inclusive
// since M48) is asserted as well, as the drift-report cross-check. Every failure names the
// reproducing seed. Plus targeted regressions for the algebra's sharp edges: alternative paths
// surviving at greater depth, LEAST-depth shortening and its reversal, reflexive-row pruning on
// the last detach, and the phantom-detach no-op gate.
//
// Run (see tenant_integration_test.go for the DSN convention):
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run Closure ./internal/tenant/...
package tenant_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/tenant/adapters"
	"github.com/olehmushka/go-oikumenea/internal/tenant/application"
	"github.com/olehmushka/go-oikumenea/internal/tenant/domain"
)

// closurePropertySeeds/-Ops control the differential run; override the seed count with
// OIKUMENEA_CLOSURE_PROP_SEEDS for a longer soak before release.
const (
	closurePropertySeeds = 10
	closurePropertyOps   = 40
)

// oracleEdge is a directed parent->child edge in the test's independent model.
type oracleEdge struct{ parent, child string }

// oracleClosure computes the expected closure rows — reflexive rows for every unit appearing in
// an edge, plus BFS shortest-path depths — mirroring RebuildClosureForGraph's contract without
// sharing any of its SQL.
func oracleClosure(edges map[oracleEdge]bool) map[[2]string]int {
	children := map[string][]string{}
	nodes := map[string]bool{}
	for e := range edges {
		children[e.parent] = append(children[e.parent], e.child)
		nodes[e.parent], nodes[e.child] = true, true
	}
	out := map[[2]string]int{}
	for n := range nodes {
		// BFS from n over the edge lists.
		depth := map[string]int{n: 0}
		queue := []string{n}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, ch := range children[cur] {
				if _, seen := depth[ch]; !seen {
					depth[ch] = depth[cur] + 1
					queue = append(queue, ch)
				}
			}
		}
		for d, dep := range depth {
			out[[2]string{n, d}] = dep
		}
	}
	return out
}

// oracleHasPath reports whether ancestor reaches descendant in the oracle edge set (or is it).
func oracleHasPath(edges map[oracleEdge]bool, ancestor, descendant string) bool {
	if ancestor == descendant {
		return true
	}
	stack := []string{ancestor}
	seen := map[string]bool{ancestor: true}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for e := range edges {
			if e.parent == cur && !seen[e.child] {
				if e.child == descendant {
					return true
				}
				seen[e.child] = true
				stack = append(stack, e.child)
			}
		}
	}
	return false
}

// storedClosure fetches one graph's closure rows straight from the table.
func storedClosure(t *testing.T, pool *pgxpool.Pool, graphID string) map[[2]string]int {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT ancestor_id, descendant_id, depth FROM oikumenea.tenant_unit_closure WHERE graph_id = $1", graphID)
	if err != nil {
		t.Fatalf("query closure: %v", err)
	}
	defer rows.Close()
	out := map[[2]string]int{}
	for rows.Next() {
		var anc, desc string
		var depth int32
		if err := rows.Scan(&anc, &desc, &depth); err != nil {
			t.Fatalf("scan closure row: %v", err)
		}
		out[[2]string{anc, desc}] = int(depth)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("closure rows: %v", err)
	}
	return out
}

// commandGraphID resolves the org's command graph RID.
func commandGraphID(t *testing.T, svc *application.Service, org domain.Organization) string {
	t.Helper()
	graphs, err := svc.ListGraphs(context.Background(), &org.ID)
	if err != nil {
		t.Fatalf("list graphs: %v", err)
	}
	for _, g := range graphs {
		if g.Code == "command" {
			return g.ID
		}
	}
	t.Fatalf("org %s has no command graph", org.ID)
	return ""
}

func diffClosures(got, want map[[2]string]int) []string {
	var msgs []string
	for k, d := range want {
		if gd, ok := got[k]; !ok {
			msgs = append(msgs, fmt.Sprintf("missing (%s -> %s) depth %d", k[0], k[1], d))
		} else if gd != d {
			msgs = append(msgs, fmt.Sprintf("depth (%s -> %s) = %d, want %d", k[0], k[1], gd, d))
		}
	}
	for k, d := range got {
		if _, ok := want[k]; !ok {
			msgs = append(msgs, fmt.Sprintf("extra (%s -> %s) depth %d", k[0], k[1], d))
		}
	}
	sort.Strings(msgs)
	return msgs
}

// TestClosureIncrementalPropertyDifferential is the M48 acceptance property: after every random
// attach/detach (including rejected cycles, duplicate edges, and phantom detaches) the stored
// closure — maintained only incrementally — equals the Go BFS oracle, rows and depths both, and
// the SQL verify reports zero drift.
func TestClosureIncrementalPropertyDifferential(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	repo := adapters.NewRepository(pool)

	seeds := closurePropertySeeds
	if v := os.Getenv("OIKUMENEA_CLOSURE_PROP_SEEDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("bad OIKUMENEA_CLOSURE_PROP_SEEDS: %v", err)
		}
		seeds = n
	}

	for seed := 0; seed < seeds; seed++ {
		r := rand.New(rand.NewSource(int64(seed)))
		org := seedOrg(t, svc)
		gid := commandGraphID(t, svc, org)

		nUnits := 8 + r.Intn(5)
		units := make([]string, nUnits)
		for i := range units {
			units[i] = mustCreate(t, svc, org, uniqueCode(t, fmt.Sprintf("s%d-u%d", seed, i))).ID
		}

		edges := map[oracleEdge]bool{} // the oracle's model
		var edgeList []oracleEdge      // deterministic pick order for detaches
		check := func(op int, what string) {
			t.Helper()
			got := storedClosure(t, pool, gid)
			want := oracleClosure(edges)
			if msgs := diffClosures(got, want); len(msgs) != 0 {
				t.Fatalf("seed %d op %d (%s): stored closure diverges from oracle:\n%s",
					seed, op, what, msgs)
			}
			missing, extra, sample, err := repo.VerifyClosure(ctx, gid)
			if err != nil {
				t.Fatalf("seed %d op %d (%s): verify: %v", seed, op, what, err)
			}
			if missing != 0 || extra != 0 {
				t.Fatalf("seed %d op %d (%s): verify drift missing=%d extra=%d sample=%s",
					seed, op, what, missing, extra, sample)
			}
		}

		for op := 0; op < closurePropertyOps; op++ {
			switch roll := r.Float64(); {
			case roll < 0.55 || len(edgeList) == 0: // attach a random pair (may be dup/cycle/self-loop)
				parent := units[r.Intn(nUnits)]
				child := units[r.Intn(nUnits)]
				e := oracleEdge{parent: parent, child: child}
				_, err := svc.AddEdge(ctx, child, parent, "command")
				switch {
				case edges[e] || parent == child || oracleHasPath(edges, child, parent):
					// duplicate edge, self-loop, or would close a cycle: must fail, closure untouched.
					if err == nil {
						t.Fatalf("seed %d op %d: attach %s->%s should have failed (dup=%v cycle=%v)",
							seed, op, parent, child, edges[e], oracleHasPath(edges, child, parent))
					}
					check(op, "rejected attach")
				case err != nil:
					t.Fatalf("seed %d op %d: attach %s->%s: %v", seed, op, parent, child, err)
				default:
					edges[e] = true
					edgeList = append(edgeList, e)
					check(op, "attach")
				}
			case roll < 0.85: // detach a random existing edge
				i := r.Intn(len(edgeList))
				e := edgeList[i]
				if err := svc.RemoveEdge(ctx, e.child, e.parent, "command"); err != nil {
					t.Fatalf("seed %d op %d: detach %s->%s: %v", seed, op, e.parent, e.child, err)
				}
				delete(edges, e)
				edgeList = append(edgeList[:i], edgeList[i+1:]...)
				check(op, "detach")
			default: // phantom detach: a pair that is not an edge — no-op, closure untouched
				parent := units[r.Intn(nUnits)]
				child := units[r.Intn(nUnits)]
				if edges[oracleEdge{parent: parent, child: child}] || parent == child {
					continue
				}
				if err := svc.RemoveEdge(ctx, child, parent, "command"); err != nil {
					t.Fatalf("seed %d op %d: phantom detach %s->%s: %v", seed, op, parent, child, err)
				}
				check(op, "phantom detach")
			}
		}
	}
}

// TestDetachKeepsAlternativePathAtGreaterDepth is the hard detach direction: a->b at depth 1 is
// removed, but a->x->b keeps (a,b) alive at depth 2 — the slice re-derivation must find it.
func TestDetachKeepsAlternativePathAtGreaterDepth(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	x := mustCreate(t, svc, org, uniqueCode(t, "x"))
	for _, e := range [][2]string{{b.ID, a.ID}, {x.ID, a.ID}, {b.ID, x.ID}} { // a->b, a->x, x->b
		if _, err := svc.AddEdge(ctx, e[0], e[1], "command"); err != nil {
			t.Fatalf("add edge %v: %v", e, err)
		}
	}
	if anc := ancestorDepths(t, svc, b.ID); anc[a.ID] != 1 {
		t.Fatalf("expected depth(a,b)=1 before detach, got %+v", anc)
	}

	if err := svc.RemoveEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("remove a->b: %v", err)
	}
	anc := ancestorDepths(t, svc, b.ID)
	if len(anc) != 2 || anc[a.ID] != 2 || anc[x.ID] != 1 {
		t.Fatalf("expected ancestors of b = {a:2, x:1} after detach, got %+v", anc)
	}
}

// TestAttachShortensDepthThenDetachRestoresIt exercises the LEAST-depth merge (attach of a
// shortcut) and its exact reversal by the slice re-derivation (detach of the shortcut).
func TestAttachShortensDepthThenDetachRestoresIt(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	c := mustCreate(t, svc, org, uniqueCode(t, "c"))
	for _, e := range [][2]string{{b.ID, a.ID}, {c.ID, b.ID}} { // a->b->c
		if _, err := svc.AddEdge(ctx, e[0], e[1], "command"); err != nil {
			t.Fatalf("add edge %v: %v", e, err)
		}
	}
	if anc := ancestorDepths(t, svc, c.ID); anc[a.ID] != 2 {
		t.Fatalf("expected depth(a,c)=2 on the chain, got %+v", anc)
	}

	if _, err := svc.AddEdge(ctx, c.ID, a.ID, "command"); err != nil { // shortcut a->c
		t.Fatalf("add shortcut a->c: %v", err)
	}
	if anc := ancestorDepths(t, svc, c.ID); anc[a.ID] != 1 {
		t.Fatalf("expected depth(a,c)=1 with the shortcut, got %+v", anc)
	}

	if err := svc.RemoveEdge(ctx, c.ID, a.ID, "command"); err != nil {
		t.Fatalf("remove shortcut: %v", err)
	}
	if anc := ancestorDepths(t, svc, c.ID); anc[a.ID] != 2 {
		t.Fatalf("expected depth(a,c)=2 restored after removing the shortcut, got %+v", anc)
	}
}

// TestDetachLastEdgeClearsClosure: detaching a graph's only edge must leave zero closure rows —
// the reflexive rows of both endpoints are pruned, matching what a rebuild over zero edges emits.
func TestDetachLastEdgeClearsClosure(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	gid := commandGraphID(t, svc, org)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if rows := storedClosure(t, pool, gid); len(rows) != 3 { // (a,a) (b,b) (a,b)
		t.Fatalf("expected 3 closure rows after the single attach, got %d: %v", len(rows), rows)
	}
	if err := svc.RemoveEdge(ctx, b.ID, a.ID, "command"); err != nil {
		t.Fatalf("remove edge: %v", err)
	}
	if rows := storedClosure(t, pool, gid); len(rows) != 0 {
		t.Fatalf("expected empty closure after detaching the last edge, got %v", rows)
	}
	if missing, extra, sample, err := adapters.NewRepository(pool).VerifyClosure(ctx, gid); err != nil || missing != 0 || extra != 0 {
		t.Fatalf("verify after last detach: missing=%d extra=%d err=%v sample=%s", missing, extra, err, sample)
	}
}

// TestRemoveMissingEdgeLeavesClosureUntouched guards the phantom-detach gate: without the edge,
// acyclicity does not rule out a child->parent path, so the shrink must not run at all.
func TestRemoveMissingEdgeLeavesClosureUntouched(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)
	gid := commandGraphID(t, svc, org)

	a := mustCreate(t, svc, org, uniqueCode(t, "a"))
	b := mustCreate(t, svc, org, uniqueCode(t, "b"))
	if _, err := svc.AddEdge(ctx, b.ID, a.ID, "command"); err != nil { // a->b
		t.Fatalf("add edge: %v", err)
	}
	before := storedClosure(t, pool, gid)

	// Detach the REVERSE pair (b->a), which is not an edge — with a->b present, b does reach a's
	// descendant set, the exact configuration a non-gated shrink would corrupt.
	if err := svc.RemoveEdge(ctx, a.ID, b.ID, "command"); err != nil {
		t.Fatalf("phantom detach should be a no-op, got %v", err)
	}
	after := storedClosure(t, pool, gid)
	if msgs := diffClosures(after, before); len(msgs) != 0 {
		t.Fatalf("phantom detach changed the closure:\n%s", msgs)
	}
}

// ancestorDepths returns unitID's ancestors as id->depth via the public API.
func ancestorDepths(t *testing.T, svc *application.Service, unitID string) map[string]int {
	t.Helper()
	refs, err := svc.Ancestors(context.Background(), unitID, "command")
	if err != nil {
		t.Fatalf("ancestors of %s: %v", unitID, err)
	}
	return idDepth(refs)
}
