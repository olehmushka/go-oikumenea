// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
)

// The GATED-FILTER gate (M59 / D-ObjectFacets rule 2, list side), pinned structurally for the reason
// the education module's holder-scope gate is: the gate lives PER HANDLER in the transport, so
// nothing outside this package can observe its absence. An application-layer test has no handler to
// lose it from and would stay green through a reopened leak.
//
// What it guards. `registrationCountry` reads vehicle_registrations, whose own endpoints
// (ListRegistrations, ListPersonVehicles) require vehicle.registration.read. Until M59 the facet
// carried no code at all, so a caller with plain vehicle.read could group the fleet by registration
// country and filter the list by it — recovering, one value at a time, exactly what those endpoints
// refuse to return. Both handlers now ask facet.FilterReadCodes what the request needs and let the
// PEP refuse it.
//
// The question asked here is STRUCTURAL — does the handler route through the one helper that owns
// the rule — because the rule itself is data (the catalog's ReadPermission) and will grow more
// facets. A handler that stops asking is the regression whatever the catalog currently says.

// gatedFilterHandlers are the two endpoints that accept the vehicle facet vocabulary. Any future
// endpoint taking these args belongs here in the same commit that adds it.
var gatedFilterHandlers = []string{
	"ListVehicles", // the fleet list
	"VehicleStats", // the dashboard aggregate over the same filter set
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
			t.Errorf("%s does not call requireFilterCodes — a caller without vehicle.registration.read "+
				"could filter by registrationCountry, which is the disclosure the code gates", name)
		}
	}
}

// TestVehicleHasAGatedFacet is the non-vacuity floor: the guard above describes behaviour that only
// matters while some vehicle facet carries a code. If the catalog is ever un-gated, this fails loudly
// instead of leaving a test that asserts nothing.
func TestVehicleHasAGatedFacet(t *testing.T) {
	o, ok := facet.Default.Get("vehicle")
	if !ok {
		t.Fatal("vehicle is not registered in the facet catalog")
	}
	for _, f := range o.Facets {
		if f.ReadPermission != "" {
			return
		}
	}
	t.Fatal("no vehicle facet carries a ReadPermission — requireFilterCodes is then a no-op and the " +
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
