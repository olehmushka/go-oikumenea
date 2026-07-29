// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package db_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleListPathsUseTheRequestPinnedConnection guards a failure mode the integration suite
// structurally CANNOT see (M56 ticket 3 / D-ObjectFacets, D-RLSDefenseInDepth).
//
// The rule: a read whose SQL can touch an RLS-protected table must obtain its repository from
// RequestQuerier(ctx, pool) — the request-pinned connection — and never from the bare pool. On an
// unpinned connection the app.* GUCs are unset, so authz_unit_in_reach sees neither the
// instance-admin bypass nor any grant, every reach predicate matches NOTHING, and the endpoint
// answers 200 with an EMPTY page to a caller entitled to every row. A filter that silently returns
// nothing is the worst shape of wrong: it reads as "no results", not as a fault.
//
// M56 ticket 2 shipped exactly this bug on the person directory and only the LIVE run caught it —
// the integration suite connects as a superuser and bypasses RLS entirely. The guard therefore lives
// in source, at the layer where the mistake is made.
//
// It lives HERE, in the package that owns RequestQuerier, rather than as a copy per module: the rule
// is one rule, and the ticket-3 endpoints made it apply to three more modules at once. Adding a
// module to the table below is the whole cost of covering it.
//
// person's own guard (internal/person/application/rls_querier_test.go) predates this one and stays —
// it carries the narrative of the original bug — so person is deliberately not repeated here.
func TestModuleListPathsUseTheRequestPinnedConnection(t *testing.T) {
	cases := []struct {
		module string
		fns    []string
	}{
		// membership_memberships and order_orders are RLS-protected on their unit column (0005).
		{"membership", []string{"ListMemberships", "ListVisibleMemberships"}},
		{"order", []string{"ListOrders", "ListVisibleOrders"}},
		// document_documents has NO policy, but the read-scope arm's holder semi-join probes
		// membership_memberships, which does — so the same rule binds.
		{"document", []string{"ListDocuments", "ListVisibleDocuments"}},
	}

	for _, c := range cases {
		t.Run(c.module, func(t *testing.T) {
			path := filepath.Join("..", "..", c.module, "application", "service.go")
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}

			bodies := map[string]string{}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv == nil || fn.Body == nil {
					continue
				}
				var b strings.Builder
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if sel, ok := n.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok {
							b.WriteString(id.Name + "." + sel.Sel.Name + " ")
						}
					}
					return true
				})
				bodies[fn.Name.Name] = b.String()
			}

			// The helper every module routes through must itself be the pinned one. Checking it
			// separately is what lets the per-function check accept `s.newRepo(s.querier(ctx))`
			// without that becoming an unverified indirection.
			querier, ok := bodies["querier"]
			if !ok {
				t.Fatalf("%s/application has no querier(ctx) helper", c.module)
			}
			if !strings.Contains(querier, "db.RequestQuerier") {
				t.Errorf("%s/application querier(ctx) does not call db.RequestQuerier — every read routed "+
					"through it silently loses the app.* RLS GUCs", c.module)
			}

			for _, name := range c.fns {
				src, ok := bodies[name]
				if !ok {
					t.Errorf("%s.%s not found — renamed or removed? (this guard must be updated with it)", c.module, name)
					continue
				}
				if !strings.Contains(src, "s.newRepo") {
					continue // delegates without building a repo itself
				}
				if strings.Contains(src, "s.querier") || strings.Contains(src, "db.RequestQuerier") {
					continue
				}
				t.Errorf("%s.%s builds its repository from the bare pool: use s.querier(ctx) (or "+
					"db.RequestQuerier(ctx, s.pool)), or any predicate over an RLS-protected table "+
					"silently matches nothing", c.module, name)
			}
		})
	}
}
