// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// The composition-root half of the facet guards (M56 / D-ObjectFacets). pkg/facet is a leaf — it
// cannot import the authorization permission catalog without a dependency cycle, so the two
// cross-registry checks live here, next to the other boot-time assertions (the links MustBeBound
// integration guard is this file's sibling).

// TestFacetReadPermissionsAreRealCodes: a facet that inherits a read code (D-ObjectFacets rule 2)
// must name a code in the closed permission catalog. A typo'd code would silently gate NOTHING —
// the M57 stats endpoint would look for a permission nobody can hold, and the facet would be
// omitted for every caller including one who legitimately has the real code.
func TestFacetReadPermissionsAreRealCodes(t *testing.T) {
	for _, o := range facet.Default.All() {
		for _, f := range o.Facets {
			if f.ReadPermission == "" {
				continue // the endpoint's own read code is the whole decision
			}
			if !authzdomain.IsKnownPermission(f.ReadPermission) {
				t.Errorf("%s.%s inherits read permission %q, which is not in the permission catalog",
					o.Type, f.Key, f.ReadPermission)
			}
		}
	}
}

// TestFacetObjectTypesAreRegisteredRIDTypes: a declared type must resolve as a registry TOKEN in the
// drift-proof pkg/rid registry (R-28), so the facet vocabulary, the ontology registry and the console
// all name types identically. Objects and reified links both qualify (a link lists and filters like
// an object — D-Ontology); actions do not. pkg/facet's own Register enforces this too; asserting it
// here keeps the check alive if that validation is ever relaxed.
func TestFacetObjectTypesAreRegisteredRIDTypes(t *testing.T) {
	tokens := rid.Tokens()
	seen := 0
	for _, o := range facet.Default.All() {
		info, ok := tokens[o.Type]
		switch {
		// A LEDGER is the declared exception (M58): its rows are the records of Actions minted by
		// other services, so it has no token of its own — and must not have one, or it would be an
		// ordinary type claiming an escape it does not need. Both halves are asserted, here as in
		// Register, because this copy exists precisely to survive that validation being relaxed.
		case o.Ledger != "":
			if ok {
				t.Errorf("facet type %q claims Ledger but IS a registered token (kind=%s)", o.Type, rid.Kind(info.Kind))
			}
		case !ok:
			t.Errorf("facet type %q is not a registry token in pkg/rid", o.Type)
		case rid.Kind(info.Kind) != rid.KindObject && rid.Kind(info.Kind) != rid.KindLink:
			t.Errorf("facet type %q is kind=%s; only objects and links are listable", o.Type, rid.Kind(info.Kind))
		}
		seen++
	}
	if seen == 0 {
		t.Fatal("no facet object types registered — the catalog is unwired")
	}
}

// TestFacetRegistryIsBoundAtBoot mirrors the assertion serve() makes, so a broken catalog fails in
// the unit sweep rather than only when a server actually starts.
func TestFacetRegistryIsBoundAtBoot(t *testing.T) {
	if err := facet.Default.MustBeBound(); err != nil {
		t.Fatalf("facet registry not bound: %v", err)
	}
}
