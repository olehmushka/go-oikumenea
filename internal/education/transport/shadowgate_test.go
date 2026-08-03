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

// The shadow-visibility gate for INSTITUTIONS (M58 ticket 5), pinned the way tenant's is and for the
// same reason it had to be.
//
// An institution IS a `university`-domain tenant organization plus a sidecar (M41 / D-UnifiedOrgGraph),
// so it carries that organization's public/shadow bit. This module applied NO gate at all from M20
// until M58 ticket 5: `listInstitutions` joined tenant_organizations, never looked at `visibility`,
// and handed shadow organizations to any caller holding `education.read` — while `listOrganizations`
// trimmed the very same rows. That is worse than the `getOrganization` leak it repeats, because it is
// a whole list rather than one RID at a time, and it went unnoticed for exactly the reason that one
// did: the gate is applied PER HANDLER in the transport, so nothing outside this package can observe
// its absence. An application-layer test has no handler to lose it from.
//
// So the invariant is pinned here, by source inspection, and STRUCTURALLY: it asks whether a handler
// routes its result through the one helper that owns the rule, not whether it produces a particular
// answer today. Organization reach moved once already (empty-by-construction → derived from unit
// reach, M58 ticket 4) and tenant's equivalent guard survived that unchanged; a behavioural assertion
// would have needed rewriting at precisely the moment a guard is least likely to be re-derived
// correctly.
//
// Only the INSTITUTION is gated here, not its children. A building, unit, group or position is reached
// through its institution's RID, which this gate is what a caller obtains — and those rows carry no
// visibility bit of their own to trim by. If that ever changes, they belong in this map, not in a
// second rule.
var shadowGatedHandlers = map[string]string{
	"ListInstitutions": "gateInstitutions",
	"GetInstitution":   "gateInstitutions",
}

func TestShadowGatedHandlersCallTheirGateHelper(t *testing.T) {
	bodies := parseMethodBodies(t)
	if len(bodies) < 20 {
		t.Fatalf("parsed only %d methods — the parse is broken, so every check below is vacuous", len(bodies))
	}
	for name, helper := range shadowGatedHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method on this package's service — renamed or removed? A shadow-gated "+
				"handler that disappears from the source must fail here rather than silently leave the list", name)
			continue
		}
		if !strings.Contains(body, helper+" ") {
			t.Errorf("%s does not route its result through %s — a shadow institution would be returned to "+
				"a caller who cannot reach it, which is what this module did from M20 until M58 ticket 5 "+
				"while listOrganizations trimmed the same rows.", name, helper)
		}
	}
}

// TestGateHelperOwnsTheShadowRule keeps the guard above meaningful. It is a presence check on a helper
// NAME, so it is worth something only while that helper really is the single implementation of the
// rule — a second, inlined `Visibility == VisibilityShadow` comparison in some other handler would
// satisfy nothing above and drift on its own.
func TestGateHelperOwnsTheShadowRule(t *testing.T) {
	fset := token.NewFileSet()
	pkg, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	for _, p := range pkg {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name.Name == "gateInstitutions" {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, sok := n.(*ast.SelectorExpr)
					if !sok || sel.Sel.Name != "VisibilityShadow" {
						return true
					}
					t.Errorf("%s (%s) compares against VisibilityShadow outside gateInstitutions — it has "+
						"written its own copy of the rule; put it in the helper so both surfaces move "+
						"together when organization reach changes again", fn.Name.Name, name)
					return true
				})
			}
		}
	}
}

// parseMethodBodies flattens every non-test method body in this package to a space-separated identifier
// stream — enough to ask "does this handler mention that helper?" without matching a comment, which a
// grep would.
func parseMethodBodies(t *testing.T) map[string]string {
	t.Helper()
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
					if id, iok := n.(*ast.Ident); iok {
						sb.WriteString(id.Name)
						sb.WriteByte(' ')
					}
					return true
				})
				bodies[fn.Name.Name] = sb.String()
			}
		}
	}
	return bodies
}
