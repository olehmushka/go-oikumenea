// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"sort"
	"strings"
	"testing"
)

// The stats-ARG drift guard (M57 / D-ObjectFacets), the third consumer of the same discipline
// args_test.go applies to the list endpoint.
//
// D-ObjectFacets' promise is that a stats endpoint takes "exactly the same filter args as the list
// endpoint plus an optional `facets` CSV". That is what makes a chart segment and a list filter the
// same act — the console reuses ONE URL query string for both views. If the two endpoints' arg sets
// drift, the failure is quiet and specific: toggling from list to dashboard silently drops a filter,
// and the chart describes a wider set than the list it came from.
//
// So this checks both directions against the IR mirror:
//
//   - every facet arg the LIST ships, the STATS endpoint must ship too, with the same type;
//   - every arg the STATS endpoint ships is a facet arg, `facets`, or a classified non-facet arg —
//     and paging args are specifically forbidden, since an aggregate has no page.

// statsPending are the registered types whose stats endpoint has not shipped yet, each with the
// ticket that lands it. It is a PENDING list, not an exemption list: a type that is neither bound nor
// listed here fails, and a type listed here that HAS a stats endpoint fails too, so the list cannot
// outlive its reason (the NonFacetArg.Why idiom).
var statsPending = map[string]string{
	// EMPTY as of M57 ticket 2: all five registered types ship a stats endpoint. An M58 type registered
	// without one belongs here with the ticket that lands it, and TestEveryTypeIsBoundOrPending fails
	// either way — an unbound type with no entry, or an entry for a type that has since been bound.
}

func TestEveryTypeIsBoundOrPending(t *testing.T) {
	for _, o := range Default.All() {
		_, mirrored := statsArgs[o.Type]
		why, pending := statsPending[o.Type]
		switch {
		case mirrored && pending:
			t.Errorf("%s HAS a stats endpoint but is still listed as pending (%q) — delete the entry", o.Type, why)
		case !mirrored && !pending:
			t.Errorf("%s has no StatsEndpoint and is not in statsPending — every registered type must "+
				"either ship a dashboard or say which ticket lands it", o.Type)
		case mirrored && o.StatsEndpoint == "":
			t.Errorf("%s is in the IR mirror but declares no StatsEndpoint — the mirror is stale", o.Type)
		}
	}
	for typ := range statsPending {
		if _, ok := Default.Get(typ); !ok {
			t.Errorf("statsPending names %q, which is not a registered object type (stale entry)", typ)
		}
	}
}

func TestStatsEndpointShipsEveryListFacetArg(t *testing.T) {
	for _, o := range Default.All() {
		stats, ok := statsArgs[o.Type]
		if !ok {
			continue // covered by TestEveryTypeIsBoundOrPending
		}
		byName := map[string]ArgSpec{}
		for _, a := range stats {
			byName[a.Name] = a
		}
		listByName := map[string]ArgSpec{}
		for _, a := range listArgs[o.Type] {
			listByName[a.Name] = a
		}

		for _, f := range o.Facets {
			for _, arg := range f.Args() {
				got, present := byName[arg]
				if !present {
					t.Errorf("%s: facet %q binds arg %q, which %s does NOT ship — the dashboard would "+
						"silently ignore a filter the list honours", o.Type, f.Key, arg, o.StatsEndpoint)
					continue
				}
				if want := listByName[arg]; want.Type != "" && got.Type != want.Type {
					t.Errorf("%s: arg %q is %s on the list but %s on the stats endpoint — the same URL "+
						"query string feeds both", o.Type, arg, want.Type, got.Type)
				}
			}
		}
		if _, ok := byName["facets"]; !ok {
			t.Errorf("%s: %s ships no `facets` arg — a dashboard cannot ask for a subset, and a caller "+
				"cannot ask for the count alone", o.Type, o.StatsEndpoint)
		}
	}
}

func TestEveryStatsArgIsAccountedFor(t *testing.T) {
	for _, o := range Default.All() {
		stats, ok := statsArgs[o.Type]
		if !ok {
			continue
		}
		known := map[string]string{"facets": "the M57 facet-selection CSV"}
		for _, f := range o.Facets {
			for _, arg := range f.Args() {
				known[arg] = "facet " + f.Key
			}
		}
		for _, n := range o.NonFacetArgs {
			if n.Class == ClassPaging {
				continue // deliberately NOT known: see below
			}
			known[n.Arg] = string(n.Class) + " arg"
		}
		var unknown []string
		for _, a := range stats {
			if _, ok := known[a.Name]; ok {
				continue
			}
			// A paging arg on a stats endpoint is not "unclassified", it is wrong: an aggregate has no
			// page, and shipping one would imply a partial count.
			for _, n := range o.NonFacetArgs {
				if n.Arg == a.Name && n.Class == ClassPaging {
					t.Errorf("%s: %s ships the paging arg %q — an aggregate has no page, and a paged "+
						"count would be a lie about the total", o.Type, o.StatsEndpoint, a.Name)
				}
			}
			unknown = append(unknown, a.Name)
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			t.Errorf("%s: %s ships arg(s) %s bound to no facet and no classified role — declare them, or "+
				"the vocabulary and the contract have quietly diverged",
				o.Type, o.StatsEndpoint, strings.Join(unknown, ", "))
		}
	}
}

// TestStatsArgGuardIsNonVacuous: the mirror must actually be populated, or every check above passes
// on empty maps.
func TestStatsArgGuardIsNonVacuous(t *testing.T) {
	if len(statsArgs) == 0 {
		t.Fatal("statsArgs is empty — the generator wrote no stats mirror, and every assertion in this " +
			"file is vacuous")
	}
	for typ, args := range statsArgs {
		if len(args) < 2 {
			t.Errorf("statsArgs[%q] has %d args — too few to be a real endpoint mirror", typ, len(args))
		}
	}
}
