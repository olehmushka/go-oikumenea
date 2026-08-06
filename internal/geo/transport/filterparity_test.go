// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// `listLocations` has four modes and a PRECEDENCE between them — query beats radius beats bbox, and
// no window at all is the browse mode. `locationStats` honours the same window, so the two surfaces
// must resolve that precedence identically or a chart would describe a different set than the list
// beside it, with nothing to show that anything was wrong.
//
// The structural answer is that the precedence is written ONCE, in locationFilterFrom, and both
// handlers call it. That is stronger than a test comparing two copies, because there is only one
// copy to be wrong — but it holds only as long as nobody re-derives the mode inline. This guard is
// what makes "only one copy" checkable.

// filterBuilderCallers are the handlers that must build their filter through the shared resolver.
var filterBuilderCallers = []string{"ListLocations", "LocationStats"}

func TestLocationSurfacesShareOneFilterBuilder(t *testing.T) {
	for _, name := range filterBuilderCallers {
		fn := findMethod(t, name)
		if fn == nil {
			t.Errorf("no handler %q in package transport — it was renamed or removed; this guard must "+
				"follow it rather than be deleted", name)
			continue
		}
		if !callsFunc(fn, "locationFilterFrom") {
			t.Errorf("%s does not call locationFilterFrom — the four-mode precedence (query beats "+
				"radius beats bbox, else browse) must be resolved in ONE place, or the list and its "+
				"dashboard will eventually read the same URL differently", name)
		}
		// Re-deriving the mode inline is the failure this guards against, and it looks exactly like
		// the switch that used to live in ListLocations. A handler that both calls the builder AND
		// names a mode constant is halfway back to two copies.
		if namesModeConstant(fn) {
			t.Errorf("%s names a domain.LocationMode constant directly. The mode is the builder's "+
				"answer; a handler that re-decides it is the second copy this guard exists to prevent "+
				"(reading f.Mode to pick a page-token shape is fine — assigning one is not)", name)
		}
	}
}

// TestFilterBuilderResolvesEveryMode pins that the resolver actually distinguishes all four. A
// builder that had quietly lost its radius arm would still be "one place", and every assertion above
// would pass while every radius request silently browsed the whole registry.
func TestFilterBuilderResolvesEveryMode(t *testing.T) {
	fn := findFunc(t, "locationFilterFrom")
	if fn == nil {
		t.Fatal("no locationFilterFrom in package transport")
	}
	want := []string{"LocationModeBrowse", "LocationModeText", "LocationModeRadius", "LocationModeBbox"}
	assigned := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			assigned[sel.Sel.Name] = true
		}
		return true
	})
	for _, m := range want {
		if !assigned[m] {
			t.Errorf("locationFilterFrom never produces domain.%s — a mode the contract documents that "+
				"the resolver cannot reach is a branch of the API that silently does something else", m)
		}
	}
}

// ---------------------------------------------------------------- AST helpers

func findMethod(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	return findDecl(t, name, true)
}

func findFunc(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()
	return findDecl(t, name, false)
}

func findDecl(t *testing.T, name string, method bool) *ast.FuncDecl {
	t.Helper()
	fset := token.NewFileSet()
	for _, file := range []string{"location.go", "stats.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			if (fn.Recv != nil) == method {
				return fn
			}
		}
	}
	return nil
}

func callsFunc(fn *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return true
	})
	return found
}

// namesModeConstant reports whether fn ASSIGNS a domain.LocationMode* constant. Reading one (to pick
// a page-token shape, which ListLocations legitimately does) is a comparison, not an assignment.
func namesModeConstant(fn *ast.FuncDecl) bool {
	found := false
	mode := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && pkg.Name == "domain" && len(sel.Sel.Name) > 12 && sel.Sel.Name[:12] == "LocationMode"
	}
	ast.Inspect(fn, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for _, r := range v.Rhs {
				if mode(r) {
					found = true
				}
			}
		case *ast.KeyValueExpr:
			if mode(v.Value) {
				found = true
			}
		}
		return true
	})
	return found
}
