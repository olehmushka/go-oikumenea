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
// its result through the ONE helper that owns its object's rule, not whether it produces a particular
// answer today. That choice paid immediately — organization reach changed from "empty by
// construction" to "derived from unit reach" one commit later, and every assertion here survived
// unchanged, where anything written against the old answers would have needed rewriting at precisely
// the moment a guard is least likely to be re-derived correctly.
//
// The helper differs by object because the QUESTION differs: a shadow unit is visible when the
// subject reaches that unit, a shadow organization when the subject reaches any of its live units.
var shadowGatedHandlers = map[string]string{
	// Units: the flat list and both closure walks. `GetUnit` is absent on purpose — it gates through
	// the SCOPED pep.Require(unit.read, unitID), which resolves reach for that one unit and is a
	// stronger check than a post-hoc trim. Organizations have no scoped form (the PDP scopes on
	// units), which is why theirs must go through a gate helper.
	"ListUnits":       "gateUnits",
	"UnitAncestors":   "gateUnits",
	"UnitDescendants": "gateUnits",
	// Organizations: the list, and the point read this guard was written for. They must call
	// gateOrgs, NOT gateUnits — the org list called gateUnits for the whole of M40..M58 ticket 4,
	// which type-checked and asked the reach probe whether an ORGANIZATION rid was among the
	// subject's readable UNITS, a question whose answer is always no.
	"ListOrganizations": "gateOrgs",
	"GetOrganization":   "gateOrgs",
}

func TestShadowGatedHandlersCallTheirGateHelper(t *testing.T) {
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
	for name, helper := range shadowGatedHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method on this package's Service — renamed or removed? A shadow-gated "+
				"handler that disappears from the source must fail here rather than silently leave the list", name)
			continue
		}
		if !strings.Contains(body, helper+" ") {
			t.Errorf("%s does not route its result through %s — a shadow-bearing object would be returned "+
				"to a caller who cannot reach it. This is the getOrganization leak (M58 ticket 4): the list "+
				"trimmed shadow rows and the point read did not, while the contract claimed both did.", name, helper)
		}
	}
	// An organization handler calling gateUnits is the ORIGINAL bug, not merely the wrong helper: it
	// type-checks and asks the unit reach probe about an organization RID, which is always no. Name it
	// separately so the failure says what actually went wrong.
	for name, helper := range shadowGatedHandlers {
		if helper != "gateOrgs" {
			continue
		}
		if strings.Contains(bodies[name], "gateUnits ") {
			t.Errorf("%s calls gateUnits on organizations — that probe asks whether an ORGANIZATION rid is "+
				"among the subject's readable UNITS, and the answer is always no. Organization reach is "+
				"DERIVED from unit reach; use gateOrgs.", name)
		}
	}
}

// TestGateHelpersOwnTheShadowRule keeps the guard above meaningful. It is a presence check on helper
// NAMES, so it is only worth anything while those helpers really are the single implementation of the
// rule — a second, inlined `Visibility == VisibilityShadow` comparison in a handler would satisfy
// nothing here and drift on its own.
//
// The gate helpers themselves are exempt (they ARE the rule), and so are the pure wire<->domain enum
// converters,
// which TRANSLATE the value rather than deciding anything with it — `toAPIVisibility` turning
// `VisibilityShadow` into its Conjure spelling is not a visibility decision and never could be.
func TestGateHelpersOwnTheShadowRule(t *testing.T) {
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
				if !ok || fn.Body == nil || fn.Name.Name == "gateUnits" || fn.Name.Name == "gateOrgs" || converters[fn.Name.Name] {
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
	// Outside the two helpers the only legitimate mention is a `shadow func(T) bool` closure handed to
	// gateUnits. Anything else naming VisibilityShadow has written its own copy of the rule.
	for _, fnName := range offenders {
		if fnName == "" {
			continue
		}
		if _, found := shadowGatedHandlers[fnName]; !found {
			t.Errorf("%s compares against VisibilityShadow but is not a registered shadow-gated handler — "+
				"either it inlined the rule (add it to gateUnits instead) or the list above is stale", fnName)
		}
	}
}
