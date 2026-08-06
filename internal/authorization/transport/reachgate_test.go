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

// The assignment read surfaces are reach-trimmed, and the trim is applied HERE — the handler resolves
// the caller with pep.SubjectAuthority and hands both the subject and the admin flag down. That makes
// this a transport property, and a transport property needs a transport-shaped guard.
//
// M58 ticket 4 paid for that lesson: the obvious behavioural test for a gate of this kind — "the
// point read and the list agree" — stayed GREEN when the leak was reintroduced, because the gate is
// applied per handler and an application-layer test has no handler to lose it from. So this checks
// the handlers themselves, by AST, exactly as transport/shadowgate_test.go does in the modules that
// carry a visibility gate.
//
// What it refuses, concretely: a handler that reads assignments, resolves no subject, and calls the
// application with a hardcoded admin flag or an empty reader — which is how a "small refactor" turns
// a trimmed list into an untrimmed one without touching a single line of SQL.

// reachTrimmedHandlers are the assignment read handlers. Each must resolve the caller's authority and
// pass BOTH results into the application call; the application then owns the arm convention.
var reachTrimmedHandlers = map[string]string{
	"ListAssignments": "ListAssignments",
	"AssignmentStats": "AssignmentStats",
}

func TestAssignmentReadHandlersResolveSubjectAuthority(t *testing.T) {
	for handler, appCall := range reachTrimmedHandlers {
		fn := findMethod(t, handler)
		if fn == nil {
			t.Errorf("no handler %q in package transport — it was renamed or removed; this guard must "+
				"follow it rather than be deleted", handler)
			continue
		}
		if !callsSelector(fn, "pep", "SubjectAuthority") && !callsAnyNamed(fn, "SubjectAuthority") {
			t.Errorf("%s does not call pep.SubjectAuthority — an assignment read that never asks who "+
				"the caller is cannot be reach-trimmed, and will serve every grant in the instance",
				handler)
		}
		if !callsAnyNamed(fn, appCall) {
			t.Errorf("%s does not call s.app.%s — the reach trim lives behind that call; a handler "+
				"reaching the repository another way would bypass it", handler, appCall)
		}
		// The two results of SubjectAuthority must BOTH reach the application call. Passing a literal
		// instead is the whole failure mode: `true` is "everyone is an admin", `""` is "no reader".
		if passesBoolLiteral(fn, appCall) {
			t.Errorf("%s passes a bool LITERAL to s.app.%s — the admin arm must be what the PEP said, "+
				"never what the handler assumed", handler, appCall)
		}
	}
}

// TestNoAssignmentHandlerGatesPerUnit pins the collapse M58 ticket 6 made. `listAssignments` used to
// gate its targetUnitId arm with pep.Require(…, unit) — a per-unit check that asked "is this unit in
// my assignment.read reach" one unit at a time. That question is now asked for every row by the SQL
// trim, and re-adding the per-unit gate would be a second, divergent implementation of it: the two
// would answer differently the day either changes.
func TestNoAssignmentHandlerGatesPerUnit(t *testing.T) {
	for handler := range reachTrimmedHandlers {
		fn := findMethod(t, handler)
		if fn == nil {
			continue
		}
		if callsSelector(fn, "pep", "Require") {
			t.Errorf("%s calls pep.Require (the UNIT-scoped gate). The reach question is answered in "+
				"SQL for every row now; asking it again here would be two implementations of one rule",
				handler)
		}
		if !callsSelector(fn, "pep", "RequireAnywhere") {
			t.Errorf("%s does not call pep.RequireAnywhere — the endpoint must still demand "+
				"assignment.read somewhere; the reach trim narrows, it does not authorize", handler)
		}
	}
}

// ---------------------------------------------------------------- AST helpers

func findMethod(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	for _, file := range []string{"service.go", "stats.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Recv != nil && fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

// callsSelector reports whether fn calls x.<field>.<method>(…) for the given field/method pair — e.g.
// s.pep.RequireAnywhere matches ("pep", "RequireAnywhere").
func callsSelector(fn *ast.FuncDecl, field, method string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			return true
		}
		inner, ok := sel.X.(*ast.SelectorExpr)
		if ok && inner.Sel.Name == field {
			found = true
		}
		return true
	})
	return found
}

func callsAnyNamed(fn *ast.FuncDecl, method string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == method {
			found = true
		}
		return true
	})
	return found
}

// passesBoolLiteral reports whether any call to the named method takes a bare `true`/`false`.
func passesBoolLiteral(fn *ast.FuncDecl, method string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			return true
		}
		for _, a := range call.Args {
			if id, ok := a.(*ast.Ident); ok && (id.Name == "true" || id.Name == "false") {
				found = true
			}
		}
		return true
	})
	return found
}

// TestReachTrimmedHandlersCoverEveryAssignmentRead is the non-vacuity floor: a new assignment READ
// handler with no entry above would go unchecked while both tests stayed green.
func TestReachTrimmedHandlersCoverEveryAssignmentRead(t *testing.T) {
	fset := token.NewFileSet()
	for _, file := range []string{"service.go", "stats.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			name := fn.Name.Name
			if !strings.Contains(name, "Assignment") {
				continue
			}
			// Writes and point reads are out of scope: a grant/revoke authorizes on the target unit,
			// and there is no getAssignment endpoint.
			if strings.HasPrefix(name, "Grant") || strings.HasPrefix(name, "Revoke") {
				continue
			}
			if _, listed := reachTrimmedHandlers[name]; !listed {
				t.Errorf("handler %q reads assignments but is not listed in reachTrimmedHandlers — add "+
					"it, so its trim is checked rather than assumed", name)
			}
		}
	}
}
