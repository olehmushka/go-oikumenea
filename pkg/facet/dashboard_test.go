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
	consoleChartsRe   = regexp.MustCompile(`(?m)^\s*charts: \[`)
	consoleDashPathRe = regexp.MustCompile(`\bpath: "([^"]*)"`)
	chartFormRe       = regexp.MustCompile(`\bform: "([^"]*)"`)
	chartFacetRe      = regexp.MustCompile(`\bfacet: "([^"]*)"`)
	chartToneRe       = regexp.MustCompile(`\btone: \{([^}]*)\}`)
	chartToneKeyRe    = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*|"[^"]*")\s*:\s*"[^"]*"`)
	chartSplitParamRe = regexp.MustCompile(`splitBy: \{ param: "([^"]*)"`)
	// `source: { path: "…", type: "…", carry: ["a", "b"] }` — the cross-source chart (M59).
	chartSourceRe      = regexp.MustCompile(`source: \{ path: "([^"]*)", type: "([^"]*)", carry: \[([^\]]*)\] \}`)
	chartCarryMemberRe = regexp.MustCompile(`"([^"]*)"`)
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
	// The cross-source declaration (M59), empty for an ordinary chart.
	sourcePath  string
	sourceType  string
	sourceCarry []string
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
		if m := chartSourceRe.FindStringSubmatch(obj); m != nil {
			c.sourcePath, c.sourceType = m[1], m[2]
			for _, q := range chartCarryMemberRe.FindAllStringSubmatch(m[3], -1) {
				c.sourceCarry = append(c.sourceCarry, q[1])
			}
		} else if strings.Contains(obj, "source:") {
			// The literal moved out of the one shape the regex knows. Loud, because a silently
			// unparsed source turns every check below into the ORDINARY-chart checks, which would
			// then demand the facet exist on the HOST type and mislead rather than fail honestly.
			t.Errorf("%s: %s chart %q declares a `source:` the guard cannot parse — keep it on one line as `source: { path: …, type: …, carry: […] }`",
				consoleRegistryPath, typeToken, c.key)
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

			// A CROSS-SOURCE chart (M59) draws another type's aggregate, so every check below must
			// be made against THAT type: its facets label the buckets, its stats endpoint answers the
			// request, and its list is where the segments link. Checking it against the host would
			// pass a chart that 400s on every load — the host declares no such facet.
			owner := o
			ownerKeys := byKey
			if c.sourceType != "" {
				src, found := catByType(cat, c.sourceType)
				if !found {
					problems = append(problems, fmt.Sprintf(
						"%s.%s: source type %q is not registered — the chart would fetch an endpoint no catalog describes",
						o.Type, c.key, c.sourceType))
					continue
				}
				owner = src
				ownerKeys = map[string]Facet{}
				for _, sf := range src.Facets {
					ownerKeys[sf.Key] = sf
				}
				if want, known := paths[src.Type]; known && c.sourcePath != want {
					problems = append(problems, fmt.Sprintf(
						"%s.%s: source path %q, but the contract serves %s's stats at %q",
						o.Type, c.key, c.sourcePath, src.Type, want))
				}
				// Every carried param must be a real filter arg of BOTH endpoints: of the source,
				// or the request silently ignores it and the chart counts a wider set than the
				// dashboard around it claims; and of the host, or the param can never be set on
				// this page and the carry is dead code that reads as scoping.
				for _, p := range c.sourceCarry {
					if !isFacetArg(src, p) {
						problems = append(problems, fmt.Sprintf(
							"%s.%s: carries %q, which is not a facet arg of the source type %s — the source would ignore it and the chart would count a WIDER set than this dashboard",
							o.Type, c.key, p, src.Type))
					}
					if !isFacetArg(o, p) {
						problems = append(problems, fmt.Sprintf(
							"%s.%s: carries %q, which is not a facet arg of %s — the host dashboard can never set it",
							o.Type, c.key, p, o.Type))
					}
				}
			}

			f, ok := ownerKeys[c.facet]
			if !ok {
				problems = append(problems, fmt.Sprintf(
					"%s: chart %q draws facet %q, which %s does not declare — an undeclared key is a 400 on the WHOLE stats request, so this blanks the dashboard",
					o.Type, c.key, c.facet, owner.Type))
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
			if c.splitByOn != "" && !isFacetArg(owner, c.splitByOn) {
				problems = append(problems, fmt.Sprintf(
					"%s.%s: splitBy param %q is not a facet arg of %s — each wing would be the whole set",
					o.Type, c.key, c.splitByOn, owner.Type))
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

func catByType(cat []ObjectType, typeToken string) (ObjectType, bool) {
	for _, o := range cat {
		if o.Type == typeToken {
			return o, true
		}
	}
	return ObjectType{}, false
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
		// ── the cross-source arm (M59) ──────────────────────────────────────
		{
			name: "a sourced chart pointed at a type nobody registers",
			dash: map[string]consoleDashboard{"person": {
				path: "/person/v1/stats/persons",
				charts: []consoleChart{{
					key: "x", form: "bar", facet: "unitId",
					sourcePath: "/membership/v1/stats/memberships", sourceType: "link__member_of",
				}},
			}},
			want: "is not registered",
		},
		{
			name: "a sourced chart drawing a facet the SOURCE does not declare",
			dash: map[string]consoleDashboard{"person": {
				path: "/person/v1/stats/persons",
				charts: []consoleChart{{
					key: "x", form: "bar", facet: "nosuch",
					sourcePath: "/person/v1/stats/persons", sourceType: "person",
				}},
			}},
			want: "does not declare",
		},
		{
			name: "a sourced chart fetching a path the contract does not serve",
			dash: map[string]consoleDashboard{"person": {
				path: "/person/v1/stats/persons",
				charts: []consoleChart{{
					key: "x", form: "bar", facet: "sex",
					sourcePath: "/person/v1/persons/stats", sourceType: "person",
				}},
			}},
			want: "but the contract serves",
		},
		{
			name: "a carried param the source would silently ignore",
			dash: map[string]consoleDashboard{"person": {
				path: "/person/v1/stats/persons",
				charts: []consoleChart{{
					key: "x", form: "bar", facet: "sex",
					sourcePath: "/person/v1/stats/persons", sourceType: "person",
					sourceCarry: []string{"org"},
				}},
			}},
			want: "not a facet arg of the source type",
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

// TestDatetimeFacetsHaveAWidenedBucketInverse closes the gap M58 ticket 2 found LIVE and by nothing
// else: a facet whose args the contract types as DATETIME must have its bucket→filter inverse emit
// RFC-3339 endpoints, because a datetime arg rejects a bare `YYYY-MM-DD` outright with a 400. A
// segment that navigates to an error is not a broken chart — the chart looks perfect — it is a
// broken LINK, and only a real request finds it.
//
// `dayPatch` had the branch from ticket 1 (audit buckets by day and its since/until are datetimes).
// `monthPatch` did not, and had never needed to: every earlier month-grain facet
// (document.issuedOn/expiresOn, order.issuedOn) takes calendar dates. `external_organization.asOf` is
// the first month-grain DATETIME facet, so the two halves had never met.
//
// The guard is structural rather than behavioural — there is no JS test runner in web/, and every
// other console guard in this package parses the TypeScript the same way — but it is anchored on the
// CATALOG: it only demands the branch for a grain some registered datetime facet actually uses, so it
// cannot become a blanket style rule.
func TestDatetimeFacetsHaveAWidenedBucketInverse(t *testing.T) {
	parsed := parseConsoleRegistry(t)
	body, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "ontology", "buckets.ts"))
	if err != nil {
		t.Fatalf("read buckets.ts: %v", err)
	}
	src := string(body)

	// grain -> the patch function that inverts it.
	fn := map[string]string{"day": "dayPatch", "month": "monthPatch", "year": "yearPatch"}
	need := map[string]string{} // patch fn -> the facet that demands it, for the message
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.Kind != KindDateRange || f.Buckets.Strategy != StrategyDateTrunc {
				continue
			}
			d, ok := findConsoleFilter(parsed[o.Type], f.Key)
			if !ok || d.argType != "datetime" {
				continue
			}
			name, ok := fn[f.Buckets.Grain]
			if !ok {
				t.Errorf("%s.%s: datetime facet buckets by grain %q, which buckets.ts has no inverse for",
					o.Type, f.Key, f.Buckets.Grain)
				continue
			}
			need[name] = o.Type + "." + f.Key
		}
	}
	if len(need) == 0 {
		t.Fatal("no datetime dateTrunc facet in the catalog — this guard is vacuous, and it exists " +
			"because the failure it catches is invisible without a live request")
	}
	for name, facetName := range need {
		fnBody, ok := tsFuncBody(src, name)
		if !ok {
			t.Errorf("buckets.ts declares no %s, but %s needs it", name, facetName)
			continue
		}
		if !strings.Contains(fnBody, `argType === "datetime"`) || !strings.Contains(fnBody, "dayBound") {
			t.Errorf("buckets.ts %s does not widen to RFC-3339 for a datetime arg (needs the "+
				"`argType === \"datetime\"` branch and dayBound), but %s is a datetime facet at that "+
				"grain — every one of its segments would link to a 400", name, facetName)
		}
	}
}

func findConsoleFilter(fs []consoleFilter, key string) (consoleFilter, bool) {
	for _, f := range fs {
		if f.key == key {
			return f, true
		}
	}
	return consoleFilter{}, false
}

// tsFuncBody extracts one top-level `function name(...) {...}` body by brace matching — enough for a
// hand-written module, and it fails closed (not found) rather than matching something else.
func tsFuncBody(src, name string) (string, bool) {
	i := strings.Index(src, "function "+name+"(")
	if i < 0 {
		return "", false
	}
	open := strings.Index(src[i:], "{")
	if open < 0 {
		return "", false
	}
	depth, start := 0, i+open
	for j := start; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : j+1], true
			}
		}
	}
	return "", false
}
