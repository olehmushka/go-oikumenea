// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestListPathsUseTheRequestPinnedConnection guards a failure mode the integration suite structurally
// cannot see (M56 / D-ObjectFacets, D-RLSDefenseInDepth).
//
// person's own tables carry no row security, so the directory list read from the bare pool for years
// and nothing noticed. The M56 `unitId` facet changed that: its predicate probes
// `membership_memberships`, which IS RLS-protected. On a connection that is not the request's pinned
// one, the `app.*` GUCs are unset, so `authz_unit_in_reach` finds neither the instance-admin bypass
// nor any grant — the EXISTS matches nothing and the endpoint returns an EMPTY page to a caller who
// may read every one of those people. A filter that silently returns nothing is the worst shape of
// wrong: it looks like "no results", not like a fault.
//
// The integration suite connects as a SUPERUSER, which bypasses RLS entirely, so it reported green
// throughout; only the live end-to-end run against the restricted application role caught it. That is
// exactly why this guard is a source check rather than another integration test — it holds at the
// layer where the mistake is made.
func TestListPathsUseTheRequestPinnedConnection(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "service.go", nil, 0)
	if err != nil {
		t.Fatalf("parse service.go: %v", err)
	}

	// Functions whose queries can touch an RLS-protected table and so must go through the pinned
	// connection rather than s.pool.
	want := map[string]bool{
		"ListPersons":        false,
		"ListVisiblePersons": false,
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		if _, tracked := want[fn.Name.Name]; !tracked {
			continue
		}
		var body strings.Builder
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					body.WriteString(id.Name + "." + sel.Sel.Name + " ")
				}
			}
			return true
		})
		src := body.String()
		if strings.Contains(src, "db.RequestQuerier") {
			want[fn.Name.Name] = true
			continue
		}
		// A function that never calls newRepo itself (it delegates) is fine.
		if !strings.Contains(src, "s.newRepo") {
			want[fn.Name.Name] = true
		}
	}

	for name, ok := range want {
		if !ok {
			t.Errorf("%s reads through the bare pool: its repository must come from "+
				"db.RequestQuerier(ctx, s.pool), or the app.* RLS GUCs are unset and any predicate over "+
				"an RLS-protected table (the unitId facet's membership probe) silently matches nothing",
				name)
		}
	}
}
