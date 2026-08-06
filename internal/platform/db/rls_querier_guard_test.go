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
		// helper names the module's pinned-connection accessor and the platform function it must
		// call. Four modules route through `querier(ctx)` → db.RequestQuerier; audit's is `reader(ctx)`
		// → db.RequestDBTX, because its repository factory takes a DBTX so an audited write can pass
		// the caller's transaction. The rule is the same; only the surface differs, so the guard is
		// parameterized rather than duplicated — and a module that names NEITHER still fails.
		helper string
		pinned string
	}{
		// membership_memberships and order_orders are RLS-protected on their unit column (0005).
		{"membership", []string{"ListMemberships", "ListVisibleMemberships",
			// M57: the read-scope dashboard aggregate, same predicate without a LIMIT.
			"VisiblePersonStatsForSubject",
			// M57 ticket 2: the roster's own dashboard, same predicate without a LIMIT.
			"MembershipStats"}, "", ""},
		{"order", []string{"ListOrders", "ListVisibleOrders", "OrderStats"}, "", ""},
		// document_documents has NO policy, but the read-scope arm's holder semi-join probes
		// membership_memberships, which does — so the same rule binds.
		{"document", []string{"ListDocuments", "ListVisibleDocuments", "DocumentStats"}, "", ""},
		// tenant_units is RLS-protected (0005/0042); the unit dashboard folds the shadow gate into
		// SQL, so it reads through the same pinned connection the list does.
		{"tenant", []string{"ListUnits", "UnitStats"}, "", ""},
		// audit_log carries a SELECT policy keyed on unit_id (0005), and it is the ONE type whose
		// visibility is ENTIRELY that policy — the aggregate folds in no predicate of its own, so the
		// pinned connection is not defence in depth here, it IS the defence. Unpinned, the ledger
		// dashboard answers a confident zero to an instance admin.
		{"audit", []string{"Query", "Stats"}, "reader", "db.RequestDBTX"},
		// M58 ticket 7. person_education_enrollments has NO policy either, and its read-scope arm probes
		// membership_memberships exactly as document's does — the same rule, the same table, one module
		// later. It is in this table because the ticket shipped it on the bare pool and the LIVE run is
		// what caught it: the guard was right and simply had never been pointed at this module, which is
		// the same shape as a drift guard reading a stale mirror.
		{"education", []string{"ListEnrollmentRegister", "EnrollmentStats"}, "", ""},
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
			helper, pinned := c.helper, c.pinned
			if helper == "" {
				helper, pinned = "querier", "db.RequestQuerier"
			}
			accessor, ok := bodies[helper]
			if !ok {
				t.Fatalf("%s/application has no %s(ctx) helper", c.module, helper)
			}
			if !strings.Contains(accessor, pinned) {
				t.Errorf("%s/application %s(ctx) does not call %s — every read routed through it "+
					"silently loses the app.* RLS GUCs", c.module, helper, pinned)
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
				if strings.Contains(src, "s."+helper) || strings.Contains(src, pinned) {
					continue
				}
				t.Errorf("%s.%s builds its repository from the bare pool: use s.%s(ctx) (or "+
					"%s(ctx, s.pool)), or any predicate over an RLS-protected table "+
					"silently matches nothing", c.module, name, helper, pinned)
			}
		})
	}
}
