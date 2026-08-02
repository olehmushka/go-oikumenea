// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The shadow-visibility gate (F-002 / D-VisibilityScope) lives in this package, applied per handler —
// there is no single choke point a reviewer can check, so every read handler that returns a
// shadow-bearing object has to remember it. `getOrganization` did not, for the whole of M40 through
// M58 ticket 3, while the CONTRACT said it did: `listOrganizations` trimmed shadow organizations for
// a non-admin and the point read on the same object handed them over to anyone holding
// `organization.read`.
//
// That is the shape that makes a visibility leak invisible: two surfaces on one object, each correct
// read on its own, and the docs line that should have caught it asserting the false claim. So the
// invariant is pinned HERE, at the layer the gate actually lives, by source inspection — an
// integration test over the application service cannot see a transport handler at all, and would
// have stayed green through exactly this regression.
//
// The check is deliberately structural rather than behavioural: it asks whether the handler routes
// its result through `gateUnits`, the ONE helper that owns the rule, and not whether it produces a
// particular answer today. That matters because organization reachability is currently empty by
// construction (docs/architecture/facets.md open seams) — a behavioural assertion written against
// today's answers would have to be rewritten the moment that is fixed, which is precisely when a
// guard is least likely to be re-derived correctly.
var shadowGatedHandlers = []string{
	// Units: the flat list and both closure walks. `GetUnit` is absent on purpose — it gates through
	// the SCOPED pep.Require(unit.read, unitID), which resolves reach for that one unit and is a
	// stronger check than the post-hoc trim. Organizations have no such scoped form (the PDP scopes
	// on units), which is why theirs must go through gateUnits.
	"ListUnits",
	"UnitAncestors",
	"UnitDescendants",
	// Organizations: the list, and the point read this guard exists for.
	"ListOrganizations",
	"GetOrganization",
}

func TestShadowGatedHandlersCallGateUnits(t *testing.T) {
	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	bodies := map[string]string{}
	for _, p := range pkg {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Body == nil {
					continue
				}
				var sb strings.Builder
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok {
						sb.WriteString(id.Name)
						sb.WriteByte(' ')
					}
					return true
				})
				bodies[fn.Name.Name] = sb.String()
			}
		}
	}
	if len(bodies) < 20 {
		t.Fatalf("parsed only %d methods — the parse is broken, so every check below is vacuous", len(bodies))
	}
	for _, name := range shadowGatedHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method on this package's Service — renamed or removed? A shadow-gated "+
				"handler that disappears from the source must fail here rather than silently leave the list", name)
			continue
		}
		if !strings.Contains(body, "gateUnits ") {
			t.Errorf("%s does not route its result through gateUnits — a shadow-bearing object would be "+
				"returned to a caller who cannot reach it. This is the getOrganization leak (M58 ticket 4): "+
				"the list trimmed shadow rows and the point read did not, while the contract claimed both did.", name)
		}
	}
}

// TestGateUnitsIsTheOnlyShadowRule keeps the guard above meaningful. It is a presence check on ONE
// helper name, so it is only worth anything while that helper really is the single implementation of
// the rule — a second, inlined `Visibility == VisibilityShadow` comparison in a handler would satisfy
// nothing here and drift on its own.
//
// gateUnits itself is exempt (it IS the rule), and so are the pure wire<->domain enum converters,
// which TRANSLATE the value rather than deciding anything with it — `toAPIVisibility` turning
// `VisibilityShadow` into its Conjure spelling is not a visibility decision and never could be.
func TestGateUnitsIsTheOnlyShadowRule(t *testing.T) {
	// Named rather than pattern-matched: a converter is exempt because someone checked what it does,
	// and a new one has to be checked too rather than inheriting the exemption from its name.
	converters := map[string]bool{"toAPIVisibility": true, "fromAPIVisibility": true}

	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	var offenders []string
	for _, p := range pkg {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name.Name == "gateUnits" || converters[fn.Name.Name] {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "VisibilityShadow" {
						return true
					}
					// A comparison INSIDE a call argument is the gateUnits shadow-extractor closure,
					// which is how the rule is meant to be expressed. Anything else is an inlined rule.
					offenders = append(offenders, fn.Name.Name)
					return true
				})
			}
		}
	}
	// Every legitimate mention is a `shadow func(T) bool` closure handed to gateUnits, so each
	// offender must also call gateUnits. Anything naming VisibilityShadow WITHOUT calling it has
	// written its own rule.
	for _, fnName := range offenders {
		if fnName == "" {
			continue
		}
		found := false
		for _, h := range shadowGatedHandlers {
			if h == fnName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s compares against VisibilityShadow but is not a registered shadow-gated handler — "+
				"either it inlined the rule (add it to gateUnits instead) or the list above is stale", fnName)
		}
	}
}
