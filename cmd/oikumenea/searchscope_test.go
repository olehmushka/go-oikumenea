// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The unified-search visibility guard (M58 ticket 5).
//
// Every provider in search_providers.go is registered with a `vis scope.Visibility`, and
// `scope.NewCatalogScope()` is the IDENTITY trim — correct for genuinely instance-global reference
// data (a languoid, a country, a location), and a silent leak for anything that carries a visibility
// bit. `institution` and `company` were registered under it from M20/M21: they ARE tenant
// organizations (M41 / D-UnifiedOrgGraph), so their rows carry the organization's public/shadow bit
// and `POST /search` handed shadow organizations to any caller holding the module's read code — the
// same disclosure the list and the point read made, through a longer path.
//
// This guard exists because breaking the fix here was the ONE break in the ticket that no existing
// test caught. The per-module `transport/shadowgate_test.go` files cannot see it: search bypasses the
// transport entirely and calls the application service, and the scope that replaces the gate is a
// field in a composition-root literal that nothing outside this package can observe. That is exactly
// the shape ticket 4 named — a rule applied per call site, with no choke point a reviewer can check.
//
// It is STRUCTURAL for the same reason the transport guards are: it asks which scope a provider is
// registered with, not what a particular subject sees today. Organization reach has already moved
// once (M58 ticket 4), and an assertion written against today's answers would need rewriting at
// precisely the moment a guard is least likely to be re-derived correctly.
var searchProviderScopes = map[string]string{
	// The two sidecar PROFILE types. `orgs` is scope.NewOrgScope over FilterVisibleOrgs — the same
	// derived reach the list and the dashboard apply.
	"institution": "orgs",
	"company":     "orgs",
	// The genuinely catalog-scoped types, listed rather than ignored: a type that MOVES between these
	// groups must edit this map, which is the review moment the guard exists to force.
	"languoid":    "catalog",
	"location":    "catalog",
	"publication": "catalog",
	"scholarship": "catalog",
	// Person has its own read-scope policy (D-PersonReadScope) and folds visibility into SQL.
	"person": "personScope",
}

func TestSearchProvidersUseTheRightVisibilityScope(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "search_providers.go", nil, 0)
	if err != nil {
		t.Fatalf("parse search_providers.go: %v", err)
	}

	got := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		var objectType, vis string
		for _, el := range lit.Elts {
			kv, kok := el.(*ast.KeyValueExpr)
			if !kok {
				continue
			}
			key, kok := kv.Key.(*ast.Ident)
			if !kok {
				continue
			}
			switch key.Name {
			case "p":
				objectType = objectTypeOf(kv.Value)
			case "vis":
				if id, iok := kv.Value.(*ast.Ident); iok {
					vis = id.Name
				}
			}
		}
		if objectType != "" && vis != "" {
			got[objectType] = vis
		}
		return true
	})

	// Non-vacuity floor: this parses Go by shape, so a refactor that moves the registration into a
	// helper would silently find nothing and turn every assertion below into a pass.
	if len(got) < 5 {
		t.Fatalf("parsed only %d search providers (%v) — the parse is broken, so every check below is "+
			"vacuous", len(got), got)
	}

	for objectType, want := range searchProviderScopes {
		switch have, ok := got[objectType]; {
		case !ok:
			t.Errorf("search provider %q is no longer registered — removed, renamed, or moved out of the "+
				"literal this guard reads. A type that leaves the search fan-in must leave this map too", objectType)
		case have != want:
			t.Errorf("search provider %q is registered with scope %q, want %q. `catalog` is the IDENTITY "+
				"trim: registering a type whose rows carry a visibility bit under it means /search returns "+
				"rows its own list endpoint refuses to name (the M58 ticket-5 leak).", objectType, have, want)
		}
	}
	for objectType, have := range got {
		if _, known := searchProviderScopes[objectType]; !known {
			t.Errorf("search provider %q (scope %q) is not in searchProviderScopes — a NEW provider must "+
				"state whether its rows carry a visibility bit, rather than inheriting `catalog` by "+
				"whichever line it was copied from", objectType, have)
		}
	}
}

// objectTypeOf reads the ObjectType string out of a searchdomain.Provider composite literal.
func objectTypeOf(v ast.Expr) string {
	lit, ok := v.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	for _, el := range lit.Elts {
		kv, kok := el.(*ast.KeyValueExpr)
		if !kok {
			continue
		}
		key, kok := kv.Key.(*ast.Ident)
		if !kok || key.Name != "ObjectType" {
			continue
		}
		if bl, bok := kv.Value.(*ast.BasicLit); bok && bl.Kind == token.STRING {
			return bl.Value[1 : len(bl.Value)-1] // strip the quotes
		}
	}
	return ""
}
