// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The console half of the D-ObjectFacets drift guard (M56 ticket 4). A facet is declared ONCE and
// consumed TWICE — as a typed list-filter query arg and (M57) as a groupBy key. This file adds the
// third consumer the decision names: the console's ontology registry, whose `FilterDef[]` is what
// actually puts a facet in front of an operator.
//
// Without this, the two halves drift silently in the direction that is invisible: the API keeps
// filtering on a facet the console never offers, so the feature is "shipped" and unreachable. The
// reverse direction is louder but worse — a FilterDef naming an arg the contract does not ship sends
// a param the server ignores, and the operator reads an unfiltered list as a filtered one.
//
// TECHNIQUE. The registry is TypeScript, so this parses it with regexes — the plaintext_test.go
// approach applied to a different non-Go source (rewrap_registry_test.go established it). That is a
// real limitation: a reformat that moves `key:` off its own line breaks the parse. The non-vacuity
// floor (TestConsoleFilterDefsAreNonVacuous) is what makes the limitation safe — a broken parse goes
// RED rather than turning every assertion below into a vacuous pass.
//
// The format contract this depends on is stated in registry.ts beside the FilterDef interface, so
// the constraint is visible where the edit happens rather than only here.

const consoleRegistryPath = "../../web/src/lib/ontology/registry.ts"

var (
	// `  person: {` / `  link__member_of: {` — an entry opening at exactly one indent level.
	consoleEntryRe = regexp.MustCompile(`(?m)^  ([a-z_][a-z0-9_]*): \{$`)
	// The `filters: [` array opener inside an entry.
	consoleFiltersRe = regexp.MustCompile(`(?m)^\s*filters: \[`)
	consoleKeyRe     = regexp.MustCompile(`\bkey: "([^"]*)"`)
	consoleKindRe    = regexp.MustCompile(`\bkind: "([^"]*)"`)
	consoleParamsRe  = regexp.MustCompile(`\bparams: \[([^\]]*)\]`)
	consoleReqRe     = regexp.MustCompile(`\brequired: true\b`)
	consoleReqsRe    = regexp.MustCompile(`\brequires: "([^"]*)"`)
	consoleBucketsRe = regexp.MustCompile(`\bbuckets: "([^"]*)"`)
	consoleArgTypeRe = regexp.MustCompile(`\bargType: "([^"]*)"`)
	consoleStringRe  = regexp.MustCompile(`"([^"]*)"`)
)

// consoleFilter is one parsed FilterDef — only the fields the catalog can be held to. Labels, hints
// and controls are console-side presentation and are deliberately NOT checked here.
type consoleFilter struct {
	key      string
	kind     string
	params   []string
	required bool
	requires string
	// M57: the console's mirror of Buckets.Strategy, declared only where `kind` does not imply it.
	// Parsed here (one parser for the file) and asserted in dashboard_test.go, where the reason it
	// exists lives.
	buckets string
	// M58: the contract type of a range facet's args, declared only where it is not a calendar date.
	argType string
}

// consoleExempt lists registered object types that deliberately carry no console filter block, with
// the reason. Empty today, and an entry must be EARNED — the NonFacetArg.Why idiom, so "the console
// does not offer this yet" cannot hide behind an unexplained omission.
var consoleExempt = map[string]string{}

// parseConsoleRegistry reads registry.ts and returns the FilterDef blocks per object type. Failures
// to read or find the file are fatal: a guard that cannot see its subject must not pass.
func parseConsoleRegistry(t *testing.T) map[string][]consoleFilter {
	t.Helper()
	body, err := os.ReadFile(filepath.Clean(consoleRegistryPath))
	if err != nil {
		t.Fatalf("read %s: %v — the console registry moved; this guard must follow it", consoleRegistryPath, err)
	}
	src := string(body)
	if !strings.Contains(src, "export const OBJECT_TYPES") {
		t.Fatalf("%s does not contain `export const OBJECT_TYPES` — the parser is looking at the wrong file", consoleRegistryPath)
	}

	out := map[string][]consoleFilter{}
	entries := consoleEntryRe.FindAllStringSubmatchIndex(src, -1)
	for i, e := range entries {
		typeToken := src[e[2]:e[3]]
		end := len(src)
		if i+1 < len(entries) {
			end = entries[i+1][0]
		}
		block := src[e[1]:end]
		loc := consoleFiltersRe.FindStringIndex(block)
		if loc == nil {
			continue
		}
		arr, ok := sliceBracketed(block[loc[1]-1:], '[', ']')
		if !ok {
			t.Fatalf("%s: %s has an unterminated `filters: [` array", consoleRegistryPath, typeToken)
		}
		out[typeToken] = parseConsoleFilters(t, typeToken, arr)
	}
	return out
}

// sliceBracketed returns s[0:n] spanning the balanced open/close pair starting at s[0], ignoring
// brackets inside double-quoted strings so a label like "a [b]" cannot unbalance the scan.
func sliceBracketed(s string, open, close byte) (string, bool) {
	depth, inStr, esc := 0, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case c == '\\' && inStr:
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
			// brackets inside a string literal do not count
		case c == open:
			depth++
		case c == close:
			depth--
			if depth == 0 {
				return s[:i+1], true
			}
		}
	}
	return "", false
}

// parseConsoleFilters splits a `filters: [...]` array into its `{ ... }` object literals.
func parseConsoleFilters(t *testing.T, typeToken, arr string) []consoleFilter {
	t.Helper()
	var out []consoleFilter
	for i := 0; i < len(arr); i++ {
		if arr[i] != '{' {
			continue
		}
		obj, ok := sliceBracketed(arr[i:], '{', '}')
		if !ok {
			t.Fatalf("%s: %s has an unterminated FilterDef object literal", consoleRegistryPath, typeToken)
		}
		i += len(obj) - 1

		var f consoleFilter
		if m := consoleKeyRe.FindStringSubmatch(obj); m != nil {
			f.key = m[1]
		}
		if m := consoleKindRe.FindStringSubmatch(obj); m != nil {
			f.kind = m[1]
		}
		if m := consoleParamsRe.FindStringSubmatch(obj); m != nil {
			for _, p := range consoleStringRe.FindAllStringSubmatch(m[1], -1) {
				f.params = append(f.params, p[1])
			}
		}
		f.required = consoleReqRe.MatchString(obj)
		if m := consoleReqsRe.FindStringSubmatch(obj); m != nil {
			f.requires = m[1]
		}
		if m := consoleBucketsRe.FindStringSubmatch(obj); m != nil {
			f.buckets = m[1]
		}
		if m := consoleArgTypeRe.FindStringSubmatch(obj); m != nil {
			f.argType = m[1]
		}
		if f.key == "" {
			t.Errorf("%s: %s has a FilterDef with no `key:` — the guard cannot check it", consoleRegistryPath, typeToken)
			continue
		}
		out = append(out, f)
	}
	return out
}

// compareConsole is the whole comparison, factored out as a pure function so the live-negative test
// can drive it with a synthetic registry instead of editing the real one. Returns one problem string
// per mismatch, empty when the two surfaces agree.
func compareConsole(cat []ObjectType, parsed map[string][]consoleFilter, args map[string][]ArgSpec) []string {
	var problems []string

	for _, o := range cat {
		defs, ok := parsed[o.Type]
		if !ok {
			if why, exempt := consoleExempt[o.Type]; exempt {
				_ = why
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: no `filters:` block in the console registry — the API filters on %d facets the console cannot offer",
				o.Type, len(o.Facets)))
			continue
		}
		byKey := map[string]consoleFilter{}
		for _, d := range defs {
			byKey[d.key] = d
		}

		// Direction 1: every facet reaches the console.
		for _, f := range o.Facets {
			d, ok := byKey[f.Key]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"%s: facet %q has no FilterDef in the console registry — the API filters on it, the console cannot offer it",
					o.Type, f.Key))
				continue
			}
			if d.kind != string(f.Kind) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: console kind %q, catalog kind %q", o.Type, f.Key, d.kind, f.Kind))
			}
			if want := f.Args(); !equalStrings(d.params, want) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: console params %v, contract args %v (Facet.Args() is the authority — ArgOverride included)",
					o.Type, f.Key, d.params, want))
			}
			if d.required != f.Required {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: console required=%v, catalog Required=%v", o.Type, f.Key, d.required, f.Required))
			}
			if d.requires != f.ReadPermission {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: console requires=%q, catalog ReadPermission=%q (D-ObjectFacets rule 2: the console must hide a facet the caller may not read)",
					o.Type, f.Key, d.requires, f.ReadPermission))
			}
		}

		// Direction 2: every FilterDef names a real facet.
		facetKeys := map[string]bool{}
		for _, f := range o.Facets {
			facetKeys[f.Key] = true
		}
		for _, d := range defs {
			if !facetKeys[d.key] {
				problems = append(problems, fmt.Sprintf(
					"%s: console FilterDef %q is not a registered facet", o.Type, d.key))
			}
		}

		// Direction 3: every param the console will send is one the CONTRACT ships. Catches a
		// hand-typed levelMin/levelMax at the console, without waiting for it to be transitively
		// caught through the catalog.
		shipped := map[string]bool{}
		for _, a := range args[o.Type] {
			shipped[a.Name] = true
		}
		for _, d := range defs {
			for _, p := range d.params {
				if !shipped[p] {
					problems = append(problems, fmt.Sprintf(
						"%s.%s: console sends param %q, which %s's list endpoint does not ship",
						o.Type, d.key, p, o.Type))
				}
			}
		}
	}

	// A console block for a type the catalog does not register at all.
	for typeToken := range parsed {
		found := false
		for _, o := range cat {
			if o.Type == typeToken {
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf(
				"console registry declares filters for %q, which pkg/facet does not register", typeToken))
		}
	}

	sort.Strings(problems)
	return problems
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestConsoleFilterDefsMatchTheCatalog is the guard proper: both directions plus the contract check.
func TestConsoleFilterDefsMatchTheCatalog(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	for _, p := range compareConsole(Default.All(), parsed, listArgs) {
		t.Error(p)
	}
}

// TestConsoleExemptionsAreEarned keeps the exemption map from becoming the allowlist the guard
// exists to prevent: an exemption must name a registered type and carry a reason.
func TestConsoleExemptionsAreEarned(t *testing.T) {
	for typeToken, why := range consoleExempt {
		if strings.TrimSpace(why) == "" {
			t.Errorf("consoleExempt[%q] has no reason", typeToken)
		}
		if _, ok := Default.Get(typeToken); !ok {
			t.Errorf("consoleExempt[%q] is not a registered object type — a stale exemption hides a real gap", typeToken)
		}
	}
}

// TestConsoleFilterDefsAreNonVacuous is what makes the regex parse safe. A parser that silently
// matched nothing would make every assertion above pass on an empty set — the failure mode a
// non-Go-source guard must foreclose (the TestGeneratedArgsAreNonVacuous lesson).
func TestConsoleFilterDefsAreNonVacuous(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	if len(parsed) < len(Default.All()) {
		t.Fatalf("parsed %d console filter blocks for %d registered types — the parse is broken or a type is unfiltered",
			len(parsed), len(Default.All()))
	}
	total := 0
	for _, defs := range parsed {
		total += len(defs)
	}
	want := 0
	for _, o := range Default.All() {
		want += len(o.Facets)
	}
	if total < want {
		t.Fatalf("parsed %d FilterDefs for %d registered facets — the parse is broken", total, want)
	}
	// Every parsed def must have carried a kind and at least one param, or the field regexes are
	// matching the object literal but not its contents.
	for typeToken, defs := range parsed {
		for _, d := range defs {
			if d.kind == "" || len(d.params) == 0 {
				t.Errorf("%s.%s parsed with kind=%q params=%v — the field regexes are not seeing the literal",
					typeToken, d.key, d.kind, d.params)
			}
		}
	}
}

// TestConsoleGuardFiresOnAViolation proves compareConsole is live rather than trivially green on a
// clean registry — the TestPlaintextGuardsFireOnAViolation discipline. Each fixture is a mistake that
// has a plausible way of actually happening.
func TestConsoleGuardFiresOnAViolation(t *testing.T) {
	cat := []ObjectType{{
		Type:         "unit",
		Module:       "tenant",
		ListEndpoint: "TenantService.listUnits",
		Facets: []Facet{
			{Key: "org", Kind: KindRef, Required: true},
			{Key: "level", Kind: KindNumericRange, ArgOverride: []string{"level"}, Note: "pins the pre-existing scalar arg"},
			{Key: "state", Kind: KindEnum},
		},
	}}
	args := map[string][]ArgSpec{"unit": {
		{Name: "org", Type: "string"},
		{Name: "level", Type: "integer", Optional: true},
		{Name: "state", Type: "string", Optional: true},
	}}
	clean := map[string][]consoleFilter{"unit": {
		{key: "org", kind: "ref", params: []string{"org"}, required: true},
		{key: "level", kind: "numeric-range", params: []string{"level"}},
		{key: "state", kind: "enum", params: []string{"state"}},
	}}
	if got := compareConsole(cat, clean, args); len(got) != 0 {
		t.Fatalf("the clean fixture must produce no problems, got %v", got)
	}

	cases := []struct {
		name  string
		mut   func(map[string][]consoleFilter)
		match string
	}{
		{
			name:  "a facet the console dropped",
			mut:   func(m map[string][]consoleFilter) { m["unit"] = m["unit"][:2] },
			match: "no FilterDef",
		},
		{
			name: "the derivation re-implemented instead of the override honoured",
			mut: func(m map[string][]consoleFilter) {
				m["unit"][1].params = []string{"levelMin", "levelMax"}
			},
			match: "does not ship",
		},
		{
			name:  "a kind that no longer matches",
			mut:   func(m map[string][]consoleFilter) { m["unit"][2].kind = "ref" },
			match: "catalog kind",
		},
		{
			name:  "a required filter turned optional",
			mut:   func(m map[string][]consoleFilter) { m["unit"][0].required = false },
			match: "required=",
		},
		{
			name: "a FilterDef for a facet that does not exist",
			mut: func(m map[string][]consoleFilter) {
				m["unit"] = append(m["unit"], consoleFilter{key: "ghost", kind: "enum", params: []string{"state"}})
			},
			match: "not a registered facet",
		},
		{
			name:  "the whole block missing",
			mut:   func(m map[string][]consoleFilter) { delete(m, "unit") },
			match: "no `filters:` block",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string][]consoleFilter{"unit": append([]consoleFilter(nil), clean["unit"]...)}
			tc.mut(m)
			problems := compareConsole(cat, m, args)
			if len(problems) == 0 {
				t.Fatalf("compareConsole reported nothing for %q", tc.name)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tc.match) {
				t.Fatalf("expected a problem containing %q, got %v", tc.match, problems)
			}
		})
	}
}
