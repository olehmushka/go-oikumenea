// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
)

// The GATED-FILTER gate for the account vocabulary (M59 / D-ObjectFacets rule 2, list side). Same
// shape and same reasoning as the vehicle module's copy: the gate lives per handler, so only a test
// inside this package can see it go missing.
//
// `holderKind` reads finance_account_holders — the polymorphic person|company holder link, whose own
// endpoint (listAccountHolders) requires finance.holder.read from a separate base role. Both account
// endpoints therefore ask facet.FilterReadCodes what the request needs before applying the filter.
// Cards are untouched: none of their facets leaves finance_cards.
var gatedFilterHandlers = []string{
	"ListAccounts", // the account registry
	"AccountStats", // the dashboard aggregate over the same filter set
}

func TestGatedFilterHandlersRequireTheFacetCodes(t *testing.T) {
	bodies := parseTransportMethodBodies(t)
	if len(bodies) < 10 {
		t.Fatalf("parsed only %d methods — the parse is broken, so every check below is vacuous", len(bodies))
	}
	for _, name := range gatedFilterHandlers {
		body, ok := bodies[name]
		if !ok {
			t.Errorf("%s is not a method on this package's services — renamed or removed? An endpoint "+
				"taking a gated facet arg must fail here rather than silently leave the list", name)
			continue
		}
		if !strings.Contains(body, "requireFilterCodes") {
			t.Errorf("%s does not call requireFilterCodes — a caller without finance.holder.read could "+
				"filter accounts by holderKind, which is the disclosure the code gates", name)
		}
	}
}

// TestAccountHasAGatedFacet is the non-vacuity floor for the guard above.
func TestAccountHasAGatedFacet(t *testing.T) {
	o, ok := facet.Default.Get("account")
	if !ok {
		t.Fatal("account is not registered in the facet catalog")
	}
	for _, f := range o.Facets {
		if f.ReadPermission != "" {
			return
		}
	}
	t.Fatal("no account facet carries a ReadPermission — requireFilterCodes is then a no-op and the " +
		"guard above asserts nothing (M59)")
}

// parseTransportMethodBodies flattens each method in the non-test sources of this package into a
// stream of identifiers, so a structural question can be asked without depending on formatting.
func parseTransportMethodBodies(t *testing.T) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	//nolint:staticcheck // ParseDir ignores build tags, which is exactly right here: the question is
	// whether the SOURCE routes through the gate, and every file in this package is one answer.
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
