// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The RAW-PGX arm of the list/stats parity guards (M58 ticket 2 / D-ObjectFacets).
//
// sqlparity_test.go and statsparity_test.go prove that a type's list and its dashboard see one world,
// and both do it by parsing `internal/<module>/adapters/queries/*.sql`: every facet's sqlc.narg must
// appear in every one of the type's queries, and the aggregate half must be byte-identical across
// arms. That is a proof about STATIC TEXT, and it works because sqlc queries are static text.
//
// Four modules are not: religion and externalorg here, vehicle and finance in the M58 tranche still
// to come. They build SQL at runtime, each by a documented choice recorded in its package doc
// (closure- and resolution-heavy taxonomy walks; cross-module label lookups over a code-filtered
// listing). They have no queries directory at all, so the existing guards cannot see them — and a
// registered type nothing checks is exactly the hole the coverage floors in those two files exist to
// refuse.
//
// The INVARIANT is unchanged: the list and the aggregate must apply the same predicate, and the
// aggregate must be one text rather than a per-arm copy. Only the proof changes, because the
// mechanism the proof was first written in does not exist here:
//
//   - sqlc's narg parity becomes: both paths call ONE shared filter builder. Checked by parsing the
//     adapter's AST for the call, not by grepping for a string — a comment mentioning the builder
//     must not satisfy it.
//   - byte-identical arms becomes: the aggregate is a single named const, referenced by the stats
//     path. There is one arm to be identical to, so what is worth proving is that the text has ONE
//     definition and cannot drift from a copy.
//
// Adding a raw-pgx type to the catalog is therefore a two-place change, and this is the place.
var rawPgxGroups = []struct {
	objectType string
	module     string
	// builder is the shared filter function both paths must call.
	builder string
	// callers are the adapter methods that must call it: the list path and the stats path.
	callers []string
	// aggregate is the const holding the UNION ALL branches, referenced by the stats path.
	aggregate string
}{
	{
		objectType: "external_organization",
		module:     "externalorg",
		builder:    "buildOrgFilter",
		callers:    []string{"ListOrgs", "OrgStats"},
		aggregate:  "orgAggregate",
	},
	{
		objectType: "taxon",
		module:     "religion",
		builder:    "buildTaxonFilter",
		callers:    []string{"ListTaxa", "TaxonStats"},
		aggregate:  "taxonAggregate",
	},
	// M58 ticket 3 — the two raw-pgx modules the header above predicted, and the first types this
	// guard was NOT written alongside. Everything it asserts was derived from religion and externalorg;
	// these three are where it either generalizes or turns out to have described two implementations
	// rather than an invariant.
	{
		objectType: "vehicle",
		module:     "vehicle",
		builder:    "buildVehicleFilter",
		callers:    []string{"ListVehicles", "VehicleStats"},
		aggregate:  "vehicleAggregate",
	},
	{
		objectType: "account",
		module:     "finance",
		builder:    "buildAccountFilter",
		callers:    []string{"ListAccounts", "AccountStats"},
		aggregate:  "accountAggregate",
	},
	// Two types in ONE module, which neither ticket-2 group was: the guard keys on the object type and
	// looks the module's functions up by name, so `finance` is parsed twice and each group must find
	// its own builder and its own aggregate const. A single shared financeAggregate would satisfy
	// neither branch-coverage direction.
	{
		objectType: "card",
		module:     "finance",
		builder:    "buildCardFilter",
		callers:    []string{"ListCards", "CardStats"},
		aggregate:  "cardAggregate",
	},
}

// rawPgxTypes is the set the sqlc-shaped coverage floors defer to, so that neither file has to know
// what a raw-pgx module is — it only has to know that SOMETHING checks the type.
func rawPgxTypes() map[string]bool {
	out := map[string]bool{}
	for _, g := range rawPgxGroups {
		out[g.objectType] = true
	}
	return out
}

// TestRawPgxTypesShareOneFilterBuilder is the narg-parity analogue: the list path and the stats path
// must build their WHERE from the same function, or `totalCount` describes a set the list does not
// page and a chart segment stops being a filter.
func TestRawPgxTypesShareOneFilterBuilder(t *testing.T) {
	for _, g := range rawPgxGroups {
		if _, ok := Default.Get(g.objectType); !ok {
			t.Errorf("rawPgxGroups names %q, which is not a registered object type (stale entry)", g.objectType)
			continue
		}
		calls := funcCalls(t, g.module)
		for _, caller := range g.callers {
			callees, ok := calls[caller]
			if !ok {
				t.Errorf("%s: no func %q in internal/%s/adapters — the guard names a method that does "+
					"not exist, so it checks nothing", g.objectType, caller, g.module)
				continue
			}
			if !callees[g.builder] {
				t.Errorf("%s: %s does not call %s. Both the list and the stats path must build their "+
					"WHERE from the one shared builder; a second hand-written predicate is exactly the "+
					"drift sqlc.narg parity catches for the other modules.\n%s calls: %v",
					g.objectType, caller, g.builder, caller, sortedKeys(callees))
			}
		}
	}
}

// TestRawPgxAggregatesAreSingleConsts is the byte-identical analogue. With one arm there is nothing
// to compare against, so what is worth proving is that the aggregate text has exactly ONE definition:
// a const, referenced by the stats path. A second copy pasted into a future scoped arm would then be
// a visible change to this guard rather than an invisible divergence.
func TestRawPgxAggregatesAreSingleConsts(t *testing.T) {
	for _, g := range rawPgxGroups {
		consts, calls := stringConsts(t, g.module), funcCalls(t, g.module)
		body, ok := consts[g.aggregate]
		if !ok {
			t.Errorf("%s: internal/%s/adapters declares no const %q — the aggregate must be one named "+
				"text, not an expression assembled per call site", g.objectType, g.module, g.aggregate)
			continue
		}
		// A stats aggregate is 40+ lines of UNION ALL; anything short means the const was gutted or
		// the parse is wrong, and every assertion here would be vacuous.
		if len(body) < 400 {
			t.Errorf("%s: const %s is %d chars — too short to be an aggregate half; the guard would be "+
				"vacuous", g.objectType, g.aggregate, len(body))
		}
		if !strings.Contains(body, "UNION ALL") || !strings.Contains(body, "'(total)'") {
			t.Errorf("%s: const %s carries no UNION ALL / no '(total)' row — it is not the aggregate",
				g.objectType, g.aggregate)
		}
		statsPath := g.callers[len(g.callers)-1]
		if !calls[statsPath][g.aggregate] {
			t.Errorf("%s: %s does not reference %s — the stats path must run the single shared "+
				"aggregate, not a local copy", g.objectType, statsPath, g.aggregate)
		}
	}
}

// TestRawPgxAggregatesCoverEveryFacet is the other half of statsparity's per-branch check: every
// declared facet must have a branch in the aggregate, and every branch must name a declared facet.
// The branch label is the facet key as a quoted literal — the same string the kernel assembles by.
func TestRawPgxAggregatesCoverEveryFacet(t *testing.T) {
	for _, g := range rawPgxGroups {
		o, ok := Default.Get(g.objectType)
		if !ok {
			continue
		}
		body := stringConsts(t, g.module)[g.aggregate]
		for _, f := range o.Facets {
			if !strings.Contains(body, "'"+f.Key+"'::text") {
				t.Errorf("%s: facet %q has no branch in %s — a declared facet the aggregate never groups "+
					"is a chart that silently never renders", g.objectType, f.Key, g.aggregate)
			}
		}
		// And the reverse: a branch naming nothing declared.
		declared := map[string]bool{}
		for _, f := range o.Facets {
			declared[f.Key] = true
		}
		for _, label := range branchLabels(body) {
			if label == "(total)" {
				continue
			}
			if !declared[label] {
				t.Errorf("%s: %s groups a branch labelled %q, which is not a declared facet — the "+
					"response would carry a distribution the catalog does not know",
					g.objectType, g.aggregate, label)
			}
		}
	}
}

// TestNonPartitioningFacetsAreEarned is not a coverage test but a containment one. The
// property exempts a facet from the buckets-sum-to-total assertion, which is the strongest thing the
// differential tests say — so it must not spread by imitation. Every use has to be a facet whose
// SOURCE TABLE IS NOT THE LISTED TABLE: that is what an overlap actually requires (a closure join or
// an M:N join), and a facet grouping a column of its own row cannot need it.
func TestNonPartitioningFacetsAreEarned(t *testing.T) {
	seen := 0
	for _, o := range Default.All() {
		listed := listedTable(o)
		for _, f := range o.Facets {
			if f.NonPartitioning == "" {
				continue
			}
			seen++
			if len(f.NonPartitioning) < 40 {
				t.Errorf("%s.%s: NonPartitioning reason is %d chars — state WHY the buckets overlap, "+
					"since this exempts the strongest assertion the differential tests make",
					o.Type, f.Key, len(f.NonPartitioning))
			}
			if listed != "" && f.Table == listed {
				t.Errorf("%s.%s: NonPartitioning names the LISTED table %s. A row has one value in its "+
					"own column, so its buckets cannot overlap — an overlap needs a closure or an M:N "+
					"join, i.e. another table. This is the exemption being copied rather than earned",
					o.Type, f.Key, f.Table)
			}
			if f.Buckets.Strategy != StrategyTopN {
				t.Errorf("%s.%s: NonPartitioning on a %s facet — only a topN ref/code facet can overlap",
					o.Type, f.Key, f.Buckets.Strategy)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no NonPartitioning facet in the catalog — this guard is vacuous, and it is the one " +
			"that keeps the exemption from spreading")
	}
}

// listedTable is the table most of a type's facets name — the type's own row. Used to tell "a column
// of the listed row" (which cannot overlap) from a join (which can).
func listedTable(o ObjectType) string {
	count := map[string]int{}
	for _, f := range o.Facets {
		count[f.Table]++
	}
	best, n := "", 0
	for tbl, c := range count {
		if c > n {
			best, n = tbl, c
		}
	}
	return best
}

// ---- AST helpers ----------------------------------------------------------
//
// Parsing rather than grepping, for the reason the RLS pinned-connection guard gives: a string match
// is satisfied by a comment or a variable name, and a guard that a comment can satisfy is not a
// guard.

// funcCalls maps each func/method name in a module's adapters package to the set of identifiers it
// calls or references.
func funcCalls(t *testing.T, module string) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	for _, file := range parseAdapters(t, module) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			names := map[string]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					names[id.Name] = true
				}
				return true
			})
			out[fn.Name.Name] = names
		}
	}
	return out
}

// stringConsts maps each string const in a module's adapters package to its (unquoted) value.
func stringConsts(t *testing.T, module string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, file := range parseAdapters(t, module) {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					out[name.Name] = strings.Trim(lit.Value, "`\"")
				}
			}
		}
	}
	return out
}

// parseAdapters parses the module's adapters package. ParseFile per entry rather than ParseDir: the
// latter is deprecated for not honouring build tags, and this is the same shape the RLS
// pinned-connection guard already uses.
func parseAdapters(t *testing.T, module string) []*ast.File {
	t.Helper()
	dir := filepath.Join("..", "..", "internal", module, "adapters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatalf("parsed no Go files under %s — every assertion over this module would be vacuous", dir)
	}
	return files
}

// branchLabels pulls the `'<facet>'::text` branch labels out of an aggregate body. Anchored on the
// quoted literal, so a `c.kind_id::text` column cast (no quotes) is not mistaken for one.
var branchLabelRe = regexp.MustCompile(`'([A-Za-z0-9_()]+)'::text`)

func branchLabels(body string) []string {
	var out []string
	for _, m := range branchLabelRe.FindAllStringSubmatch(body, -1) {
		out = append(out, m[1])
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
