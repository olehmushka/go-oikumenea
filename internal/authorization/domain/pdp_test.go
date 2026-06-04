package domain

import (
	"context"
	"testing"
	"time"
)

// fakeClosure is an in-memory ClosurePort for the PDP tests: paths is the set of (graph,ancestor,
// descendant) reflexive+transitive edges present in the closure, bearing flags directory-only graphs.
type fakeClosure struct {
	paths   map[[3]string]bool // [graphID, ancestor, descendant] -> present
	bearing map[string]bool    // graphID -> is_authority_bearing (absent => true)
	desc    map[[2]string][]string
}

func (f fakeClosure) IsAncestorOrSelf(_ context.Context, g, a, d string) (bool, error) {
	return f.paths[[3]string{g, a, d}], nil
}
func (f fakeClosure) IsAuthorityBearing(_ context.Context, g string) (bool, error) {
	b, ok := f.bearing[g]
	if !ok {
		return true, nil
	}
	return b, nil
}
func (f fakeClosure) DescendantUnitIDs(_ context.Context, g, u string) ([]string, error) {
	return f.desc[[2]string{g, u}], nil
}

func grant(target string, scope Scope, graphID string, perms ...Permission) ActiveGrant {
	m := make(map[Permission]struct{}, len(perms))
	for _, p := range perms {
		m[p] = struct{}{}
	}
	return ActiveGrant{AssignmentID: "a-" + target, RoleCode: "r", TargetUnitID: target, Scope: scope, GraphID: graphID, GraphCode: "command", Perms: m}
}

func decide(t *testing.T, p PDP, in DecisionInput) bool {
	t.Helper()
	d, err := p.Decide(context.Background(), in)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	return d.Allow
}

func TestPDP_UnitScopeLeaksNothingDownward(t *testing.T) {
	// command graph: T -> C (T reaches C). A `unit` grant at T must NOT reach C.
	fc := fakeClosure{paths: map[[3]string]bool{{"g", "T", "C"}: true, {"g", "T", "T"}: true}}
	p := NewPDP(fc)
	g := grant("T", ScopeUnit, "", PermUnitRead)

	if !decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "unit.read", UnitID: "T"}) {
		t.Fatal("unit grant should allow at its own target")
	}
	if decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "unit.read", UnitID: "C"}) {
		t.Fatal("unit grant must not leak to a child")
	}
}

func TestPDP_SubtreeCascadesAndSelf(t *testing.T) {
	fc := fakeClosure{paths: map[[3]string]bool{{"g", "T", "C"}: true}} // note: no reflexive T,T row
	p := NewPDP(fc)
	g := grant("T", ScopeSubtree, "g", PermPersonRead)

	if !decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "person.read", UnitID: "C"}) {
		t.Fatal("subtree grant should reach a descendant")
	}
	// Self must be allowed WITHOUT a reflexive closure row (a subtree includes its root).
	if !decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "person.read", UnitID: "T"}) {
		t.Fatal("subtree grant should authorize its own target unit (self)")
	}
	// An unrelated unit is not reached.
	if decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "person.read", UnitID: "X"}) {
		t.Fatal("subtree grant must not reach an unrelated unit")
	}
}

func TestPDP_DirectoryGraphCascadesNothing(t *testing.T) {
	fc := fakeClosure{
		paths:   map[[3]string]bool{{"dir", "T", "C"}: true},
		bearing: map[string]bool{"dir": false},
	}
	p := NewPDP(fc)
	g := grant("T", ScopeSubtree, "dir", PermUnitRead)
	if decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "unit.read", UnitID: "C"}) {
		t.Fatal("a subtree grant on a directory-only graph must cascade nothing")
	}
	if decide(t, p, DecisionInput{Grants: []ActiveGrant{g}, Action: "unit.read", UnitID: "T"}) {
		t.Fatal("a subtree grant on a directory-only graph must not even authorize its target")
	}
}

func TestPDP_UnionAcrossGraphs(t *testing.T) {
	// command: A reaches U with person.read; operational: B reaches U with order.read. Both contribute.
	fc := fakeClosure{paths: map[[3]string]bool{{"cmd", "A", "U"}: true, {"op", "B", "U"}: true}}
	p := NewPDP(fc)
	grants := []ActiveGrant{
		grant("A", ScopeSubtree, "cmd", PermPersonRead),
		grant("B", ScopeSubtree, "op", PermOrderRead),
	}
	if !decide(t, p, DecisionInput{Grants: grants, Action: "person.read", UnitID: "U"}) {
		t.Fatal("command-graph grant should satisfy person.read at U")
	}
	if !decide(t, p, DecisionInput{Grants: grants, Action: "order.read", UnitID: "U"}) {
		t.Fatal("operational-graph grant should satisfy order.read at U (union across graphs)")
	}
	if decide(t, p, DecisionInput{Grants: grants, Action: "unit.update", UnitID: "U"}) {
		t.Fatal("no grant carries unit.update — should deny")
	}
}

func TestPDP_ExpiryAndInstanceScope(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	a := Assignment{RevokedAt: nil, ExpiresAt: &past}
	if a.Active(time.Now()) {
		t.Fatal("an expired assignment must be inactive")
	}

	fc := fakeClosure{}
	p := NewPDP(fc)
	// Non-admin asking an instance-scope action is denied even with a matching unit grant attempt.
	if decide(t, p, DecisionInput{Grants: []ActiveGrant{grant("T", ScopeUnit, "")}, Action: "graph.manage", UnitID: "T"}) {
		t.Fatal("instance-scope action must be denied to a non-instance-admin")
	}
	// Instance admin is allowed everything.
	if !decide(t, p, DecisionInput{IsInstanceAdmin: true, Action: "graph.manage", UnitID: ""}) {
		t.Fatal("instance admin should be allowed an instance-scope action")
	}
	if !decide(t, p, DecisionInput{IsInstanceAdmin: true, Action: "unit.update", UnitID: "anything"}) {
		t.Fatal("instance admin should be allowed unit-scoped actions everywhere")
	}
}

func TestPDP_ExplainNamesContributors(t *testing.T) {
	fc := fakeClosure{paths: map[[3]string]bool{{"cmd", "A", "U"}: true, {"op", "B", "U"}: true}}
	p := NewPDP(fc)
	grants := []ActiveGrant{
		grant("A", ScopeSubtree, "cmd", PermPersonRead),
		grant("B", ScopeSubtree, "op", PermPersonRead),
	}
	d, err := p.Decide(context.Background(), DecisionInput{Grants: grants, Action: "person.read", UnitID: "U", Explain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allow || len(d.Via) != 2 {
		t.Fatalf("explain should name both contributing graphs, got allow=%v via=%d", d.Allow, len(d.Via))
	}
}

func TestPDP_ReachSet(t *testing.T) {
	fc := fakeClosure{
		desc: map[[2]string][]string{{"g", "T"}: {"C1", "C2"}},
	}
	p := NewPDP(fc)
	grants := []ActiveGrant{
		grant("T", ScopeSubtree, "g", PermPersonRead), // read-bearing subtree
		grant("U", ScopeUnit, "", PermUnitUpdate),     // write-bearing unit
	}
	r, err := p.ReachSet(context.Background(), grants, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"T", "C1", "C2"} {
		if _, ok := r.Readable[want]; !ok {
			t.Fatalf("readable should include %s", want)
		}
	}
	if _, ok := r.Writable["U"]; !ok {
		t.Fatal("writable should include the unit-scope write target U")
	}
	if _, ok := r.Writable["C1"]; ok {
		t.Fatal("a read-only subtree must not contribute to the writable set")
	}
}

func TestValidatePermissionSet(t *testing.T) {
	if err := ValidatePermissionSet([]Permission{PermUnitRead, PermPersonCreate}); err != nil {
		t.Fatalf("valid unit-scoped perms should pass: %v", err)
	}
	if err := ValidatePermissionSet([]Permission{"nope.invalid"}); err == nil {
		t.Fatal("unknown permission must be rejected")
	}
	if err := ValidatePermissionSet([]Permission{PermGraphManage}); err == nil {
		t.Fatal("instance-scope permission must be rejected in a role set")
	}
}
