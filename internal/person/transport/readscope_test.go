// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The person read-scope gate (M60 ticket 0), pinned STRUCTURALLY, for the reason education's holder
// gate is (holderscope_test.go there).
//
// From M0 until M60 ticket 0 only GetPerson applied D-PersonReadScope. The ~30 sub-resource reads
// beside it — contacts, addresses, physical identity, health, legal records, overlays, watchlist, the
// six relationship lists — gated their permission code ANYWHERE and returned the rows. No person-
// family table carries RLS, so one grant anywhere read every person's sub-resources instance-wide.
// The gate lives per handler in this package; nothing outside it can observe its absence, so the
// question asked here is "does every handler route through the one helper that owns the rule".

// TestEveryPersonReadProbesTheReadScope holds every `GET /persons/{personId}…` operation in the
// contract — the source of truth for what those endpoints ARE — to requireReadablePerson. A new
// endpoint is covered the moment it lands in the YAML; there is no hand-kept list to forget.
func TestEveryPersonReadProbesTheReadScope(t *testing.T) {
	bodies := parseMethodBodies(t)
	ops := personReadOps(t)
	if len(ops) < 30 {
		t.Fatalf("parsed only %d GET /persons/{personId}… operations from the contract (31 at M60 t0) — "+
			"the parse is broken and this check is vacuous", len(ops))
	}
	for _, op := range ops {
		method := strings.ToUpper(op[:1]) + op[1:]
		body, ok := bodies[method]
		if !ok {
			t.Errorf("%s serves GET /persons/{personId}… but is not a method in this package", method)
			continue
		}
		if !strings.Contains(body, "requireReadablePerson ") {
			t.Errorf("%s does not call requireReadablePerson — it returns one named person's data to any "+
				"caller holding its code ANYWHERE, which every sub-resource read did until M60 ticket 0 "+
				"(D-PersonReadScope)", method)
		}
	}
}

// relationshipLists name the person↔person link lists: each row carries a SECOND person, who must be
// readable too (the D-VisibilityScope counterpart trim).
var relationshipLists = []string{
	"ListPartnerships", "ListKinships", "ListGuardianships",
	"ListSponsorships", "ListNextOfKin", "ListAssociations",
}

func TestRelationshipListsTrimCounterparts(t *testing.T) {
	bodies := parseMethodBodies(t)
	for _, name := range relationshipLists {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method in this package — renamed or removed?", name)
			continue
		}
		if !strings.Contains(body, "trimByCounterpart ") {
			t.Errorf("%s does not trim counterparts — it discloses an unreadable relative's id behind a "+
				"readable person (D-VisibilityScope)", name)
		}
	}
}

// TestReadScopeHelpersOwnTheProjection keeps the two guards above meaningful: they check a helper
// NAME, which is worth something only while the helper is the sole caller of the probe. A handler
// calling ReadablePerson directly would skip the instance-admin arm and the NotFound mapping.
func TestReadScopeHelpersOwnTheProjection(t *testing.T) {
	owners := map[string]bool{"requireReadablePerson": true, "trimByCounterpart": true}
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	for _, p := range pkgs {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || owners[fn.Name.Name] {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					sel, sok := n.(*ast.SelectorExpr)
					if sok && (sel.Sel.Name == "ReadablePerson" || sel.Sel.Name == "ReadablePersonsAmong") {
						t.Errorf("%s (%s) calls %s outside the read-scope helpers — a private copy of "+
							"the projection", fn.Name.Name, name, sel.Sel.Name)
					}
					return true
				})
			}
		}
	}
}

// parseMethodBodies maps each method name in this package to the identifiers of its body.
func parseMethodBodies(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse transport package: %v", err)
	}
	bodies := map[string]string{}
	for _, p := range pkgs {
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

// personReadOps returns the operation name of every `GET /persons/{personId}` and
// `GET /persons/{personId}/…` endpoint in the person contract (block form: the op line, then http:).
func personReadOps(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../../api/person.conjure.yml")
	if err != nil {
		t.Fatalf("read person contract: %v", err)
	}
	opLine := regexp.MustCompile(`^\s{6}([a-z][A-Za-z0-9]*):\s*$`)
	httpLine := regexp.MustCompile(`^\s*http:\s*"?GET /persons/\{personId\}(/|"?\s*$)`)
	var out []string
	var pending string
	for _, line := range strings.Split(string(raw), "\n") {
		if m := opLine.FindStringSubmatch(line); m != nil {
			pending = m[1]
			continue
		}
		if pending != "" && httpLine.MatchString(line) {
			out = append(out, pending)
			pending = ""
		}
	}
	return out
}
