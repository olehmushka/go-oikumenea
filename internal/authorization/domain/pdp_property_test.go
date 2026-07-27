// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

// Randomized differential/property tests for the PDP. Each iteration generates a random multi-graph
// unit DAG plus random grants, then checks Decide against an independent oracle written straight
// from the documented algorithm (docs/modules/authorization.md), and asserts the load-bearing
// invariants by name: unit scope never cascades, directory-only graphs contribute nothing, union
// monotonicity, reach/decision agreement, and explain soundness. Every failure names the seed that
// reproduces it.

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
)

// permPool is the unit-scoped catalog sample grants draw from; readPool is its `*.read` subset
// (what ReachSet classifies as read-bearing).
var permPool = []Permission{
	PermUnitRead, PermPersonRead, PermMembershipRead, PermOrderRead,
	PermUnitCreate, PermUnitUpdate, PermPersonCreate,
}

// world is one generated scenario: units, per-graph strict reachability (ancestor -> descendants),
// authority-bearing flags, and the subject's active grants.
type world struct {
	units   []string
	graphs  []string
	bearing map[string]bool
	reach   map[string]map[string]map[string]bool // graph -> ancestor -> descendant -> true (strict)
	grants  []ActiveGrant
}

// genWorld builds a random world. Acyclicity holds by construction: edges only go from a
// lower-indexed unit to a higher-indexed one, which still permits multi-parent nodes (a DAG).
func genWorld(r *rand.Rand) world {
	n := 2 + r.Intn(14)
	w := world{
		bearing: map[string]bool{},
		reach:   map[string]map[string]map[string]bool{},
	}
	for i := range n {
		w.units = append(w.units, fmt.Sprintf("u%02d", i))
	}
	nGraphs := 1 + r.Intn(3)
	for gi := range nGraphs {
		g := fmt.Sprintf("g%d", gi)
		w.graphs = append(w.graphs, g)
		w.bearing[g] = r.Intn(4) != 0 // ~1 in 4 graphs is directory-only

		children := make([][]int, n)
		for i := range n {
			for j := i + 1; j < n; j++ {
				if r.Intn(n) < 2 { // sparse; multiple i for one j => multi-parent
					children[i] = append(children[i], j)
				}
			}
		}
		// Strict transitive reach, computed bottom-up (edges only point to higher indexes).
		reach := make([]map[int]bool, n)
		for i := n - 1; i >= 0; i-- {
			reach[i] = map[int]bool{}
			for _, c := range children[i] {
				reach[i][c] = true
				for d := range reach[c] {
					reach[i][d] = true
				}
			}
		}
		w.reach[g] = map[string]map[string]bool{}
		for i := range n {
			m := map[string]bool{}
			for d := range reach[i] {
				m[w.units[d]] = true
			}
			w.reach[g][w.units[i]] = m
		}
	}
	for k := r.Intn(8); k > 0; k-- {
		w.grants = append(w.grants, genGrant(r, w, len(w.grants)))
	}
	return w
}

func genGrant(r *rand.Rand, w world, i int) ActiveGrant {
	perms := map[Permission]struct{}{}
	for len(perms) == 0 {
		for _, p := range permPool {
			if r.Intn(3) == 0 {
				perms[p] = struct{}{}
			}
		}
	}
	g := ActiveGrant{
		AssignmentID: fmt.Sprintf("as%02d", i),
		RoleID:       "r", RoleCode: "r",
		TargetUnitID: w.units[r.Intn(len(w.units))],
		Scope:        ScopeUnit,
		Perms:        perms,
	}
	if r.Intn(2) == 0 {
		g.Scope = ScopeSubtree
		g.GraphID = w.graphs[r.Intn(len(w.graphs))]
		g.GraphCode = g.GraphID
	}
	return g
}

// closureOf adapts a world into the ClosurePort the PDP consumes (via the fakeClosure from
// pdp_test.go), including strict-descendant lists for ReachSet.
func closureOf(w world) fakeClosure {
	fc := fakeClosure{paths: map[[3]string]bool{}, bearing: w.bearing, desc: map[[2]string][]string{}}
	for g, byAnc := range w.reach {
		for anc, descs := range byAnc {
			var list []string
			for d := range descs {
				fc.paths[[3]string{g, anc, d}] = true
				list = append(list, d)
			}
			fc.desc[[2]string{g, anc}] = list
		}
	}
	return fc
}

// oracleAllow is the independent reference: a literal transcription of the documented algorithm
// over the world's raw reachability sets, sharing no code with PDP.Decide.
func oracleAllow(w world, grants []ActiveGrant, isAdmin bool, action, unit string) bool {
	if isAdmin {
		return true
	}
	if IsInstanceScope(action) {
		return false
	}
	for _, g := range grants {
		if !g.Has(Permission(action)) {
			continue
		}
		switch g.Scope {
		case ScopeUnit:
			if g.TargetUnitID == unit {
				return true
			}
		case ScopeSubtree:
			if w.bearing[g.GraphID] && (g.TargetUnitID == unit || w.reach[g.GraphID][g.TargetUnitID][unit]) {
				return true
			}
		}
	}
	return false
}

const propertyIterations = 300

// TestPDP_PropertyDifferential checks Decide against the oracle for every (unit, action) pair of
// each random world — including an instance-scope action, which must deny for non-admins no matter
// the grants — plus union monotonicity (removing a grant never turns a DENY into an ALLOW) and
// explain soundness (every named contribution independently justifies the decision).
func TestPDP_PropertyDifferential(t *testing.T) {
	ctx := context.Background()
	actions := make([]string, 0, len(permPool)+1)
	for _, p := range permPool {
		actions = append(actions, string(p))
	}
	actions = append(actions, string(PermGraphManage)) // instance-scope: deny unless admin

	for seed := range propertyIterations {
		r := rand.New(rand.NewSource(int64(seed)))
		w := genWorld(r)
		p := NewPDP(closureOf(w))

		subset := w.grants
		if len(subset) > 0 {
			subset = w.grants[:r.Intn(len(w.grants))]
		}

		for _, unit := range w.units {
			for _, action := range actions {
				in := DecisionInput{Grants: w.grants, Action: action, UnitID: unit}
				d, err := p.Decide(ctx, in)
				if err != nil {
					t.Fatalf("seed %d: Decide(%s@%s): %v", seed, action, unit, err)
				}
				if want := oracleAllow(w, w.grants, false, action, unit); d.Allow != want {
					t.Fatalf("seed %d: Decide(%s@%s)=%v, oracle=%v (grants=%+v)", seed, action, unit, d.Allow, want, w.grants)
				}

				// Monotonicity: an ALLOW under a grant subset must survive adding the rest.
				if oracleAllow(w, subset, false, action, unit) && !d.Allow {
					t.Fatalf("seed %d: adding grants turned ALLOW into DENY for %s@%s", seed, action, unit)
				}

				// Explain soundness: each contribution must justify the decision on its own.
				if d.Allow {
					ex, err := p.Decide(ctx, DecisionInput{Grants: w.grants, Action: action, UnitID: unit, Explain: true})
					if err != nil || !ex.Allow || len(ex.Via) == 0 {
						t.Fatalf("seed %d: explain for allowed %s@%s: allow=%v via=%d err=%v", seed, action, unit, ex.Allow, len(ex.Via), err)
					}
					for _, c := range ex.Via {
						g, ok := grantByID(w.grants, c.AssignmentID)
						if !ok {
							t.Fatalf("seed %d: explain names unknown assignment %q", seed, c.AssignmentID)
						}
						if !oracleAllow(w, []ActiveGrant{g}, false, action, unit) {
							t.Fatalf("seed %d: explain names %q, which does not justify %s@%s alone", seed, c.AssignmentID, action, unit)
						}
					}
				}

				// The instance plane allows everything, unit-scoped or instance-scope.
				if d, err := p.Decide(ctx, DecisionInput{Grants: w.grants, IsInstanceAdmin: true, Action: action, UnitID: unit}); err != nil || !d.Allow {
					t.Fatalf("seed %d: instance admin denied %s@%s (err=%v)", seed, action, unit, err)
				}
			}
		}
	}
}

func grantByID(grants []ActiveGrant, id string) (ActiveGrant, bool) {
	for _, g := range grants {
		if g.AssignmentID == id {
			return g, true
		}
	}
	return ActiveGrant{}, false
}

// TestPDP_PropertyUnitScopeNeverCascades pins the D-Inherit invariant by name: in a world holding
// ONLY unit-scope grants, an ALLOW can occur only at a grant's exact target — never at a child,
// however the DAG is shaped.
func TestPDP_PropertyUnitScopeNeverCascades(t *testing.T) {
	ctx := context.Background()
	for seed := range propertyIterations {
		r := rand.New(rand.NewSource(int64(seed)))
		w := genWorld(r)
		var grants []ActiveGrant
		targets := map[string]bool{}
		for _, g := range w.grants {
			g.Scope, g.GraphID, g.GraphCode = ScopeUnit, "", ""
			grants = append(grants, g)
			targets[g.TargetUnitID] = true
		}
		p := NewPDP(closureOf(w))
		for _, unit := range w.units {
			for _, perm := range permPool {
				d, err := p.Decide(ctx, DecisionInput{Grants: grants, Action: string(perm), UnitID: unit})
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if d.Allow && !targets[unit] {
					t.Fatalf("seed %d: unit-scope grant cascaded to non-target %s", seed, unit)
				}
			}
		}
	}
}

// TestPDP_PropertyDirectoryGraphContributesNothing pins D-DirectoryGraphs: with every graph
// directory-only, subtree grants confer nothing anywhere (not even at their own target), so
// decisions equal those of the unit-scope grants alone.
func TestPDP_PropertyDirectoryGraphContributesNothing(t *testing.T) {
	ctx := context.Background()
	for seed := range propertyIterations {
		r := rand.New(rand.NewSource(int64(seed)))
		w := genWorld(r)
		for g := range w.bearing {
			w.bearing[g] = false
		}
		var unitOnly []ActiveGrant
		for _, g := range w.grants {
			if g.Scope == ScopeUnit {
				unitOnly = append(unitOnly, g)
			}
		}
		p := NewPDP(closureOf(w))
		for _, unit := range w.units {
			for _, perm := range permPool {
				full, err := p.Decide(ctx, DecisionInput{Grants: w.grants, Action: string(perm), UnitID: unit})
				if err != nil {
					t.Fatalf("seed %d: %v", seed, err)
				}
				if full.Allow != oracleAllow(w, unitOnly, false, string(perm), unit) {
					t.Fatalf("seed %d: a directory-only subtree grant changed the decision at %s for %s", seed, unit, perm)
				}
			}
		}
	}
}

// TestPDP_PropertyReachSetMatchesDecisions ties the two PDP surfaces together: a unit is in
// Readable/Writable exactly when SOME read/write permission decides ALLOW there (the property the
// shadow gate and the RLS GUCs rely on — D-RLSDefenseInDepth), and the shadow gate hides exactly
// the shadow units outside the readable reach.
func TestPDP_PropertyReachSetMatchesDecisions(t *testing.T) {
	ctx := context.Background()
	for seed := range propertyIterations {
		r := rand.New(rand.NewSource(int64(seed)))
		w := genWorld(r)
		p := NewPDP(closureOf(w))
		reach, err := p.ReachSet(ctx, w.grants, false)
		if err != nil {
			t.Fatalf("seed %d: ReachSet: %v", seed, err)
		}
		shadow := map[string]bool{}
		for _, unit := range w.units {
			shadow[unit] = r.Intn(2) == 0
			var wantRead, wantWrite bool
			for _, perm := range permPool {
				if !oracleAllow(w, w.grants, false, string(perm), unit) {
					continue
				}
				if isReadPermission(perm) {
					wantRead = true
				} else {
					wantWrite = true
				}
			}
			if _, got := reach.Readable[unit]; got != wantRead {
				t.Fatalf("seed %d: Readable[%s]=%v, decisions say %v", seed, unit, got, wantRead)
			}
			if _, got := reach.Writable[unit]; got != wantWrite {
				t.Fatalf("seed %d: Writable[%s]=%v, decisions say %v", seed, unit, got, wantWrite)
			}
		}
		visible := ShadowGate(reach, w.units, shadow)
		for _, unit := range w.units {
			_, got := visible[unit]
			want := !shadow[unit] || reach.Reachable(unit)
			if got != want {
				t.Fatalf("seed %d: ShadowGate(%s)=%v, want %v (shadow=%v)", seed, unit, got, want, shadow[unit])
			}
		}
	}
}
