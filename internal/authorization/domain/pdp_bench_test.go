package domain

import (
	"context"
	"strconv"
	"testing"
)

// PDP performance pass (M11). The PDP is the per-request hot path (every gated endpoint authorizes,
// and the RLS middleware computes reach per request). These benchmarks are pure — over an in-memory
// ClosurePort — so they measure the engine's union/closure-walk logic, not the DB; a real closure
// lookup is a single indexed PK read on tenant_unit_closure (exercised by the integration tests).
//
//	go test -tags '' -run '^$' -bench BenchmarkPDP ./internal/authorization/domain/...

// benchClosure builds a single authority-bearing graph "g" with `width` units that all sit under a
// common ancestor "root" (root reaches each, reflexively too), plus the per-grant subtree expansion.
func benchClosure(width int) (fakeClosure, []string) {
	paths := map[[3]string]bool{}
	desc := map[[2]string][]string{}
	units := make([]string, width)
	kids := make([]string, width)
	for i := 0; i < width; i++ {
		u := "u" + strconv.Itoa(i)
		units[i] = u
		kids[i] = u
		paths[[3]string{"g", "root", u}] = true
		paths[[3]string{"g", u, u}] = true
	}
	paths[[3]string{"g", "root", "root"}] = true
	desc[[2]string{"g", "root"}] = kids
	return fakeClosure{paths: paths, bearing: map[string]bool{"g": true}, desc: desc}, units
}

// BenchmarkPDPDecide measures one authorize decision against a subject holding many subtree grants,
// querying a deep unit (worst case: the matching grant is last, so the union walks them all).
func BenchmarkPDPDecide(b *testing.B) {
	const nGrants = 64
	fc, units := benchClosure(256)
	p := NewPDP(fc)

	grants := make([]ActiveGrant, nGrants)
	for i := range grants {
		// Each grant targets "root" in the bearing graph, carrying person.read; the union must walk
		// all of them and consult the closure to reach the queried descendant.
		grants[i] = grant("root", ScopeSubtree, "g", PermPersonRead)
	}
	target := units[len(units)-1]
	in := DecisionInput{Grants: grants, Action: "person.read", UnitID: target}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d, err := p.Decide(context.Background(), in)
		if err != nil || !d.Allow {
			b.Fatalf("decide: allow=%v err=%v", d.Allow, err)
		}
	}
}

// BenchmarkPDPReachSet measures expanding a subject's grants into the read/write unit reach (the RLS
// backstop + shadow gate input), the per-request computation the authenticator does on every call.
func BenchmarkPDPReachSet(b *testing.B) {
	const nGrants = 32
	fc, _ := benchClosure(256)
	p := NewPDP(fc)

	grants := make([]ActiveGrant, nGrants)
	for i := range grants {
		grants[i] = grant("root", ScopeSubtree, "g", PermPersonRead)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, err := p.ReachSet(context.Background(), grants, false)
		if err != nil || len(r.Readable) == 0 {
			b.Fatalf("reachset: readable=%d err=%v", len(r.Readable), err)
		}
	}
}
