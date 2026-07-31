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

// The CHART half of the console drift guard (M57 ticket 3, D-ObjectFacets / D-ConsoleDashboards).
//
// console_test.go holds the console's `filters:` blocks against the catalog. This holds its
// `dashboard:` blocks, and the failure it exists to prevent is worse than a missing filter: the
// console asks for exactly the facets it draws (`?facets=a,b,c`), and a key the type does not declare
// is a 400 on the WHOLE request — one stale chart blanks the entire dashboard rather than itself.
// The same class of mistake at the list layer only loses one control.
//
// Four things are checked, each a mistake with a plausible way of happening:
//
//  1. every ChartDef names a facet the catalog declares for that type (a renamed or removed facet);
//  2. every registered type has a dashboard, or an entry in dashboardExempt WITH A REASON — the
//     NonFacetArg.Why idiom, so the map cannot decay into an allowlist;
//  3. the dashboard's `path` is the one the CONTRACT actually serves, read out of the Conjure YAML
//     (base-path + the stats endpoint's http line) — a hand-typed `/persons/stats` is the exact shape
//     httprouter refuses, and no unit test would otherwise see it;
//  4. the console's `buckets:` declaration matches the catalog's bucket STRATEGY, because M57's
//     click-through inverts a bucket key back into a filter and the inverse of an age band is not the
//     inverse of a calendar month.
//
// Plus the non-vacuity floor console_test.go established: a regex over TypeScript that silently
// matches nothing must go RED, not turn every assertion above into a vacuous pass.

var (
	// The `dashboard: {` opener inside an object-type entry.
	consoleDashboardRe = regexp.MustCompile(`(?m)^\s*dashboard: \{`)
	// `charts: [` inside a dashboard block.
	consoleChartsRe = regexp.MustCompile(`(?m)^\s*charts: \[`)
	consoleDashPathRe = regexp.MustCompile(`\bpath: "([^"]*)"`)
	chartFormRe       = regexp.MustCompile(`\bform: "([^"]*)"`)
	chartFacetRe      = regexp.MustCompile(`\bfacet: "([^"]*)"`)
	chartToneRe       = regexp.MustCompile(`\btone: \{([^}]*)\}`)
	chartToneKeyRe    = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*|"[^"]*")\s*:\s*"[^"]*"`)
	chartSplitParamRe = regexp.MustCompile(`splitBy: \{ param: "([^"]*)"`)
)

// dashboardExempt lists registered object types that deliberately ship no dashboard, with the reason.
// Empty today: M57 gives all five operational-core types one. An M58 type registered without charts
// belongs here with the ticket that lands it.
var dashboardExempt = map[string]string{}

// consoleChart is one parsed ChartDef — only the fields the catalog and the contract can be held to.
// Titles, notes and orientation are presentation and are deliberately not checked.
type consoleChart struct {
	key       string
	form      string
	facet     string
	toneKeys  []string
	splitByOn string
}

type consoleDashboard struct {
	path   string
	charts []consoleChart
}

// parseConsoleDashboards reads registry.ts and returns the dashboard block per object type. As in
// console_test.go, failure to read or recognise the file is fatal: a guard that cannot see its
// subject must not pass.
func parseConsoleDashboards(t *testing.T) map[string]consoleDashboard {
	t.Helper()
	body, err := os.ReadFile(filepath.Clean(consoleRegistryPath))
	if err != nil {
		t.Fatalf("read %s: %v — the console registry moved; this guard must follow it", consoleRegistryPath, err)
	}
	src := string(body)
	if !strings.Contains(src, "export const OBJECT_TYPES") {
		t.Fatalf("%s does not contain `export const OBJECT_TYPES` — the parser is looking at the wrong file", consoleRegistryPath)
	}

	out := map[string]consoleDashboard{}
	entries := consoleEntryRe.FindAllStringSubmatchIndex(src, -1)
	for i, e := range entries {
		typeToken := src[e[2]:e[3]]
		end := len(src)
		if i+1 < len(entries) {
			end = entries[i+1][0]
		}
		block := src[e[1]:end]
		loc := consoleDashboardRe.FindStringIndex(block)
		if loc == nil {
			continue
		}
		obj, ok := sliceBracketed(block[loc[1]-1:], '{', '}')
		if !ok {
			t.Fatalf("%s: %s has an unterminated `dashboard: {` block", consoleRegistryPath, typeToken)
		}
		d := consoleDashboard{}
		if m := consoleDashPathRe.FindStringSubmatch(obj); m != nil {
			d.path = m[1]
		}
		cl := consoleChartsRe.FindStringIndex(obj)
		if cl == nil {
			t.Fatalf("%s: %s has a dashboard with no `charts: [` array", consoleRegistryPath, typeToken)
		}
		arr, ok := sliceBracketed(obj[cl[1]-1:], '[', ']')
		if !ok {
			t.Fatalf("%s: %s has an unterminated `charts: [` array", consoleRegistryPath, typeToken)
		}
		d.charts = parseConsoleCharts(t, typeToken, arr)
		out[typeToken] = d
	}
	return out
}

func parseConsoleCharts(t *testing.T, typeToken, arr string) []consoleChart {
	t.Helper()
	var out []consoleChart
	for i := 0; i < len(arr); i++ {
		if arr[i] != '{' {
			continue
		}
		obj, ok := sliceBracketed(arr[i:], '{', '}')
		if !ok {
			t.Fatalf("%s: %s has an unterminated ChartDef object literal", consoleRegistryPath, typeToken)
		}
		i += len(obj) - 1

		var c consoleChart
		if m := consoleKeyRe.FindStringSubmatch(obj); m != nil {
			c.key = m[1]
		}
		if m := chartFormRe.FindStringSubmatch(obj); m != nil {
			c.form = m[1]
		}
		if m := chartFacetRe.FindStringSubmatch(obj); m != nil {
			c.facet = m[1]
		}
		if m := chartToneRe.FindStringSubmatch(obj); m != nil {
			for _, kv := range chartToneKeyRe.FindAllStringSubmatch(m[1], -1) {
				c.toneKeys = append(c.toneKeys, strings.Trim(kv[1], `"`))
			}
		}
		if m := chartSplitParamRe.FindStringSubmatch(obj); m != nil {
			c.splitByOn = m[1]
		}
		if c.key == "" {
			t.Errorf("%s: %s has a ChartDef with no `key:` — the guard cannot check it", consoleRegistryPath, typeToken)
			continue
		}
		out = append(out, c)
	}
	return out
}

// compareDashboards is the comparison as a pure function, so the live-negative test can drive it with
// a synthetic registry rather than editing the real one.
func compareDashboards(cat []ObjectType, parsed map[string]consoleDashboard, paths map[string]string) []string {
	var problems []string

	for _, o := range cat {
		d, ok := parsed[o.Type]
		if !ok {
			if _, exempt := dashboardExempt[o.Type]; exempt {
				continue
			}
			problems = append(problems, fmt.Sprintf(
				"%s: no `dashboard:` block in the console registry — the type ships a stats endpoint no console surface draws",
				o.Type))
			continue
		}

		byKey := map[string]Facet{}
		for _, f := range o.Facets {
			byKey[f.Key] = f
		}

		if want, known := paths[o.Type]; known && d.path != want {
			problems = append(problems, fmt.Sprintf(
				"%s: console dashboard path %q, contract serves %q (the console would 404 every dashboard)",
				o.Type, d.path, want))
		}

		if len(d.charts) == 0 {
			problems = append(problems, fmt.Sprintf("%s: dashboard declares no charts", o.Type))
		}

		seen := map[string]bool{}
		for _, c := range d.charts {
			if seen[c.key] {
				problems = append(problems, fmt.Sprintf("%s: two charts share the key %q", o.Type, c.key))
			}
			seen[c.key] = true

			f, ok := byKey[c.facet]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"%s: chart %q draws facet %q, which the catalog does not declare — an undeclared key is a 400 on the WHOLE stats request, so this blanks the dashboard",
					o.Type, c.key, c.facet))
				continue
			}

			// A tone map keys buckets by their VALUE, so an enum whose CHECK set changed leaves a
			// tone pointing at nothing — the segment silently reverts to the default fill.
			for _, k := range c.toneKeys {
				if f.Kind != KindEnum {
					problems = append(problems, fmt.Sprintf(
						"%s.%s: tone key %q on a %s facet — only an enum has stable value keys to tone",
						o.Type, c.key, k, f.Kind))
					continue
				}
				if !containsString(f.Values, k) {
					problems = append(problems, fmt.Sprintf(
						"%s.%s: tone key %q is not a value of facet %q (%v)",
						o.Type, c.key, k, c.facet, f.Values))
				}
			}

			// A pyramid fetches one extra request per split value, narrowed by that param. The param
			// must be a real facet arg or the extra requests silently return the unsplit set.
			if c.splitByOn != "" && !isFacetArg(o, c.splitByOn) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: splitBy param %q is not a facet arg of %s — each wing would be the whole set",
					o.Type, c.key, c.splitByOn, o.Type))
			}
		}
	}

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
				"console registry declares a dashboard for %q, which pkg/facet does not register", typeToken))
		}
	}

	sort.Strings(problems)
	return problems
}

func isFacetArg(o ObjectType, arg string) bool {
	for _, f := range o.Facets {
		for _, a := range f.Args() {
			if a == arg {
				return true
			}
		}
	}
	return false
}

func containsString(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

// ── the guards ──────────────────────────────────────────────────────────────

func TestConsoleDashboardsMatchTheCatalog(t *testing.T) {
	parsed := parseConsoleDashboards(t)
	for _, p := range compareDashboards(Default.All(), parsed, statsPathsFromContract(t)) {
		t.Error(p)
	}
}

func TestDashboardExemptionsAreEarned(t *testing.T) {
	for typeToken, why := range dashboardExempt {
		if strings.TrimSpace(why) == "" {
			t.Errorf("dashboardExempt[%q] has no reason", typeToken)
		}
		if _, ok := Default.Get(typeToken); !ok {
			t.Errorf("dashboardExempt[%q] is not a registered object type — a stale exemption hides a real gap", typeToken)
		}
	}
}

// TestConsoleBucketStrategiesMatchTheCatalog holds the console's `buckets:` declaration against the
// catalog's strategy. The console needs it because M57 turns a bucket key back into a FILTER, and the
// inverse differs by strategy: `2026-03` bounds a calendar month, `25-34` is an age band whose inverse
// is a BIRTHDATE range running the other way. Declaring the wrong one produces a link that navigates
// happily to the wrong row set — the one failure a click-through does not reveal by itself.
//
// It is required exactly where `kind` does not imply the strategy (date-range, numeric-range) and
// forbidden elsewhere, so it cannot spread into decoration.
func TestConsoleBucketStrategiesMatchTheCatalog(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	for _, o := range Default.All() {
		byKey := map[string]consoleFilter{}
		for _, d := range parsed[o.Type] {
			byKey[d.key] = d
		}
		for _, f := range o.Facets {
			d, ok := byKey[f.Key]
			if !ok {
				continue // console_test.go already reports the missing FilterDef
			}
			want := ""
			switch f.Kind {
			case KindDateRange, KindNumericRange:
				switch f.Buckets.Strategy {
				case StrategyBands:
					want = "bands"
				case StrategyDateTrunc:
					want = "dateTrunc"
				default:
					t.Errorf("%s.%s: %s facet buckets by %q, which the console has no declaration for",
						o.Type, f.Key, f.Kind, f.Buckets.Strategy)
					continue
				}
			}
			if d.buckets != want {
				t.Errorf("%s.%s: console buckets=%q, catalog strategy %q wants %q",
					o.Type, f.Key, d.buckets, f.Buckets.Strategy, want)
			}
		}
	}
}

// TestConsoleArgTypesMatchTheContract holds the console's `argType` against the IR mirror. It exists
// because of one concrete failure: audit's since/until are Conjure DATETIMEs, so a control that sends
// a bare `2026-07-30` gets a 400, and a histogram bar that links to one sends the operator to an error
// page rather than to its own rows. The console must therefore know a range arg is a timestamp — and
// knowing it is exactly the kind of fact that rots when it lives only in someone's memory.
//
// Both directions: a datetime arg the console does not declare, and a declaration on an arg the
// contract types as a plain date.
func TestConsoleArgTypesMatchTheContract(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	for _, o := range Default.All() {
		byKey := map[string]consoleFilter{}
		for _, d := range parsed[o.Type] {
			byKey[d.key] = d
		}
		shipped := map[string]ArgSpec{}
		for _, a := range listArgs[o.Type] {
			shipped[a.Name] = a
		}
		for _, f := range o.Facets {
			d, ok := byKey[f.Key]
			if !ok {
				continue // console_test.go already reports the missing FilterDef
			}
			want := ""
			for _, arg := range f.Args() {
				if shipped[arg].Type == "datetime" {
					want = "datetime"
				}
			}
			if d.argType != want {
				t.Errorf("%s.%s: console argType=%q, contract wants %q — a datetime arg needs day bounds, "+
					"a date arg must not get them", o.Type, f.Key, d.argType, want)
			}
		}
	}
}

// TestConsoleDashboardsAreNonVacuous is what makes the regex parse safe, exactly as
// TestConsoleFilterDefsAreNonVacuous does for the filter blocks.
func TestConsoleDashboardsAreNonVacuous(t *testing.T) {
	parsed := parseConsoleDashboards(t)
	want := len(Default.All()) - len(dashboardExempt)
	if len(parsed) < want {
		t.Fatalf("parsed %d dashboard blocks for %d registered types — the parse is broken or a type lost its charts",
			len(parsed), want)
	}
	charts := 0
	for typeToken, d := range parsed {
		if d.path == "" {
			t.Errorf("%s: dashboard parsed with no `path:` — the field regex is not seeing the literal", typeToken)
		}
		for _, c := range d.charts {
			if c.form == "" || c.facet == "" {
				t.Errorf("%s.%s parsed with form=%q facet=%q — the field regexes are not seeing the literal",
					typeToken, c.key, c.form, c.facet)
			}
		}
		charts += len(d.charts)
	}
	// Five dashboards, at least three charts each: a parse that found the blocks but not their
	// contents would otherwise satisfy the count above.
	if charts < 3*want {
		t.Fatalf("parsed %d charts across %d dashboards — the parse is broken", charts, want)
	}
}

// TestDashboardGuardFiresOnAViolation proves compareDashboards is live rather than trivially green,
// the TestConsoleGuardFiresOnAViolation discipline. Each fixture is a mistake with a plausible way of
// actually happening.
func TestDashboardGuardFiresOnAViolation(t *testing.T) {
	cat := []ObjectType{{
		Type:         "person",
		Module:       "person",
		ListEndpoint: "PersonService.listPersons",
		Facets: []Facet{
			{Key: "sex", Kind: KindEnum, Values: []string{"male", "female"}},
			{Key: "birthdate", Kind: KindDateRange, Buckets: Buckets{Strategy: StrategyBands}},
		},
	}}
	paths := map[string]string{"person": "/person/v1/stats/persons"}

	cases := []struct {
		name string
		dash map[string]consoleDashboard
		want string
	}{
		{
			name: "facet renamed out from under a chart",
			dash: map[string]consoleDashboard{"person": {
				path:   "/person/v1/stats/persons",
				charts: []consoleChart{{key: "sex", form: "donut", facet: "gender"}},
			}},
			want: "does not declare",
		},
		{
			name: "the specified-but-unroutable path shape",
			dash: map[string]consoleDashboard{"person": {
				path:   "/person/v1/persons/stats",
				charts: []consoleChart{{key: "sex", form: "donut", facet: "sex"}},
			}},
			want: "contract serves",
		},
		{
			name: "a tone keyed on a value the CHECK set no longer has",
			dash: map[string]consoleDashboard{"person": {
				path:   "/person/v1/stats/persons",
				charts: []consoleChart{{key: "sex", form: "donut", facet: "sex", toneKeys: []string{"unknown"}}},
			}},
			want: "is not a value of facet",
		},
		{
			name: "a pyramid split on something that is not a filter arg",
			dash: map[string]consoleDashboard{"person": {
				path:   "/person/v1/stats/persons",
				charts: []consoleChart{{key: "p", form: "pyramid", facet: "birthdate", splitByOn: "gender"}},
			}},
			want: "is not a facet arg",
		},
		{
			name: "the whole block dropped",
			dash: map[string]consoleDashboard{},
			want: "no `dashboard:` block",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := compareDashboards(cat, tc.dash, paths)
			if len(problems) == 0 {
				t.Fatalf("guard stayed green on: %s", tc.name)
			}
			if !strings.Contains(strings.Join(problems, "\n"), tc.want) {
				t.Errorf("problems %v do not mention %q", problems, tc.want)
			}
		})
	}
}

// ── the contract's own stats paths ──────────────────────────────────────────

var (
	conjureServiceRe  = regexp.MustCompile(`(?m)^  ([A-Za-z]+Service):$`)
	conjureBasePathRe = regexp.MustCompile(`(?m)^\s*base-path: (\S+)`)
)

// statsPathsFromContract reads each registered type's stats path out of the Conjure YAML — the source
// of truth for what the server actually routes — as `base-path` + the endpoint's `http:` line. Parsed
// rather than hardcoded so the guard follows a contract edit instead of having to be remembered.
func statsPathsFromContract(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("../../api/*.conjure.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob api/*.conjure.yml: %v (%d files) — the guard cannot see the contract", err, len(files))
	}
	out := map[string]string{}
	for _, o := range Default.All() {
		service, endpoint, ok := strings.Cut(o.StatsEndpoint, ".")
		if !ok {
			continue // no stats endpoint declared; statsargs_test.go owns that case
		}
		for _, file := range files {
			raw, err := os.ReadFile(filepath.Clean(file))
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			src := string(raw)
			loc := conjureServiceRe.FindAllStringSubmatchIndex(src, -1)
			for _, s := range loc {
				if src[s[2]:s[3]] != service {
					continue
				}
				body := src[s[1]:]
				base := ""
				if m := conjureBasePathRe.FindStringSubmatch(body); m != nil {
					base = m[1]
				}
				httpRe := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(endpoint) + `:\n\s*http: [A-Z]+ (\S+)`)
				if m := httpRe.FindStringSubmatch(body); m != nil {
					out[o.Type] = base + m[1]
				}
			}
		}
		if out[o.Type] == "" {
			t.Fatalf("could not read a stats path for %s (%s) out of the Conjure YAML — the parse is broken",
				o.Type, o.StatsEndpoint)
		}
	}
	return out
}
