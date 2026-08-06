// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The GATED-FACET guards (M59): D-ObjectFacets rule 2 finally has live cases, and rule 2 now has a
// list side as well as a stats side.
//
// The stats side was already built and already tested — pkg/stats drops a facet the caller may not
// read. What had never shipped was a facet that actually CARRIES a code, so the omission branch had
// only ever run against synthetic fixtures. Two do now (vehicle.registrationCountry,
// account.holderKind), and these guards hold the three things that must stay true of them.

// TestFilterReadCodes is the list-side kernel: which codes a request needs, given which gated args it
// supplied. Table-driven over a synthetic type so it tests the RULE, not today's catalog.
func TestFilterReadCodes(t *testing.T) {
	o := ObjectType{
		Type: "t",
		Facets: []Facet{
			{Key: "plain", Kind: KindEnum},
			{Key: "gated", Kind: KindEnum, ReadPermission: "a.read"},
			{Key: "alsoGated", Kind: KindEnum, ReadPermission: "a.read"},
			{Key: "otherGated", Kind: KindEnum, ReadPermission: "b.read"},
		},
	}
	cases := []struct {
		name     string
		supplied map[string]bool
		want     []string
	}{
		{"nothing supplied gates nothing", map[string]bool{}, nil},
		{"a plain arg gates nothing", map[string]bool{"plain": true}, nil},
		{"a gated arg requires its code", map[string]bool{"gated": true}, []string{"a.read"}},
		{
			"an arg PRESENT-but-false gates nothing — absence is what matters",
			map[string]bool{"gated": false}, nil,
		},
		{
			"two facets inheriting one code require it once",
			map[string]bool{"gated": true, "alsoGated": true}, []string{"a.read"},
		},
		{
			"distinct codes are all required, sorted",
			map[string]bool{"otherGated": true, "gated": true}, []string{"a.read", "b.read"},
		},
		{
			"an unknown key gates nothing — a typo must not become an authorization event",
			map[string]bool{"nosuch": true}, nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := o.FilterReadCodes(tc.supplied)
			if !equalStrings(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
				t.Errorf("FilterReadCodes(%v) = %v, want %v", tc.supplied, got, tc.want)
			}
		})
	}
}

// TestGatedFacetsExistInTheCatalog is the non-vacuity floor. Every guard here and the whole omission
// branch in pkg/stats describe behaviour that only happens when SOME facet carries a code; if the
// catalog ever goes back to carrying none, these tests would keep passing while asserting nothing —
// the state rule 2 was in from M56 until M59, and the reason this test exists rather than being
// implied.
func TestGatedFacetsExistInTheCatalog(t *testing.T) {
	var gated []string
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.ReadPermission != "" {
				gated = append(gated, o.Type+"."+f.Key)
			}
		}
	}
	if len(gated) == 0 {
		t.Fatal("no facet carries a ReadPermission — D-ObjectFacets rule 2 has no live case, so its " +
			"omission and refusal paths are tested only by construction (see M59)")
	}
	sort.Strings(gated)
	t.Logf("gated facets: %s", strings.Join(gated, ", "))
}

// TestGatedFacetCodesAreRealPermissions holds every declared ReadPermission against the permission
// registry. A typo'd code is the worst possible failure here: RequireAnywhere would refuse it for
// EVERY caller (no role grants a code that does not exist), so the facet would silently disappear
// from every dashboard and its filter would 403 for everyone — a lockout that looks like an empty
// chart. Parsed out of the authorization module's Go source rather than imported, because pkg/facet
// is a stdlib-only leaf by design.
func TestGatedFacetCodesAreRealPermissions(t *testing.T) {
	body, err := os.ReadFile(filepath.Clean("../../internal/authorization/domain/permissions.go"))
	if err != nil {
		t.Fatalf("read permissions.go: %v — the guard cannot see its subject", err)
	}
	codeRe := regexp.MustCompile(`Permission = "([a-z0-9.\-_]+)"`)
	known := map[string]bool{}
	for _, m := range codeRe.FindAllStringSubmatch(string(body), -1) {
		known[m[1]] = true
	}
	if len(known) < 100 {
		t.Fatalf("parsed only %d permission codes — the parser is broken, so every check below would pass vacuously", len(known))
	}
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.ReadPermission != "" && !known[f.ReadPermission] {
				t.Errorf("%s.%s: ReadPermission %q is not a code in permissions.go — no role can hold it, so the facet would be invisible to EVERY caller",
					o.Type, f.Key, f.ReadPermission)
			}
		}
	}
}

// TestGatedFacetsAreGatedOnTheirListEndpoint is the guard the M58 defect this ticket fixes would have
// caught: vehicle.registrationCountry shipped ungated over a table whose own endpoints require
// vehicle.registration.read, so a vehicle.read caller could filter and group by it.
//
// The rule it enforces is narrow and checkable: if a facet's Table is a table some transport gates on
// a code, the facet must inherit A code. It does not try to decide WHICH — that is a judgement the
// declaration's Note carries — only that a facet reading a separately-gated surface may not be silent
// about it.
func TestGatedFacetsAreGatedOnTheirListEndpoint(t *testing.T) {
	// The surfaces that carry their own read code, and the code each one carries. Extend this when a
	// module adds a sub-resource code; the guard is a statement about the DOMAIN, not a parse.
	gatedTables := map[string]string{
		"oikumenea.vehicle_registrations":    "vehicle.registration.read",
		"oikumenea.finance_account_holders":  "finance.holder.read",
		"oikumenea.account_login_events":     "account.security-log.read",
		"oikumenea.person_addresses":         "person.address.read",
		"oikumenea.person_party_memberships": "person.party_membership.read",
		"oikumenea.person_health_records":    "person.health.read",
		"oikumenea.person_legal_records":     "person.legal-record.read",
	}
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			want, gated := gatedTables[f.Table]
			if !gated {
				continue
			}
			if f.ReadPermission == "" {
				t.Errorf("%s.%s reads %s, which its module gates on %q, but declares no ReadPermission — "+
					"the filter and the distribution would disclose one value at a time what the "+
					"sub-resource endpoints refuse to return (D-ObjectFacets rule 2)",
					o.Type, f.Key, f.Table, want)
			}
		}
	}
}

// TestGatedFacetsAreHiddenInTheConsole closes the loop at the UI: a gated facet must carry the same
// code in the console's FilterDef. console_test.go already compares the two whenever both are
// present; this states the direction that matters — a gated facet whose console filter forgot the
// code offers a control that now 403s on use rather than one that merely returns nothing.
func TestGatedFacetsAreHiddenInTheConsole(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.ReadPermission == "" {
				continue
			}
			var found bool
			for _, d := range parsed[o.Type] {
				if d.key == f.Key {
					found = true
					if d.requires != f.ReadPermission {
						t.Errorf("%s.%s: console requires=%q, catalog=%q", o.Type, f.Key, d.requires, f.ReadPermission)
					}
				}
			}
			if !found {
				t.Errorf("%s.%s is gated but has no console FilterDef to hide", o.Type, f.Key)
			}
		}
	}
}
