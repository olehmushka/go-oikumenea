// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The StrategyCatalog guards (M58 ticket 7).
//
// The strategy is an escape from the rule every other ref facet follows — rank by count, cut at
// top-N — and like the Ledger and Profile escapes before it, what keeps it from being copied is not
// its comment but a check. There are two halves, and they answer different questions:
//
//   - Register refuses a MALFORMED declaration (not a ref, no catalog table, no ordinal). Pure, and
//     it fires the moment the catalog is built.
//   - The DDL guard refuses a DISHONEST one: the strategy's claim is "the column points at a closed,
//     ordered catalog", and that claim is a fact the migrations record. A facet could name a catalog
//     table its column does not reference, or an ordinal column that does not exist, and Register
//     would be none the wiser — the buckets would come back from a LEFT JOIN against an unrelated
//     table, all zero, and the chart would render.
//
// This is the shape ticket 5 established for Profile: an escape whose claim can be checked against
// the schema must be, and only the ones that cannot (a ledger's "these rows have no token") are
// argued in prose.

// TestRegisterCatalogStrategyRules pins the arms Register grew for the third escape.
func TestRegisterCatalogStrategyRules(t *testing.T) {
	catalogFacet := func() Facet {
		return Facet{
			Key: "degreeLevelId", Kind: KindRef,
			Table: "oikumenea.person_education_enrollments", Column: "degree_level_id",
			RefType: "degree_level",
			Buckets: Buckets{
				Strategy:     StrategyCatalog,
				CatalogTable: "oikumenea.education_degree_levels",
				CatalogOrder: "isced_level",
			},
		}
	}
	cases := []struct {
		name string
		edit func(*Facet)
		want string
	}{
		{
			"not a ref facet",
			func(f *Facet) { f.Kind, f.RefType, f.Values = KindEnum, "", []string{"a"} },
			"catalog buckets require a ref facet",
		},
		{
			"no catalog table",
			func(f *Facet) { f.Buckets.CatalogTable = "" },
			"require CatalogTable",
		},
		{
			"catalog table not schema-qualified",
			func(f *Facet) { f.Buckets.CatalogTable = "education_degree_levels" },
			"must be schema-qualified",
		},
		{
			"no ordinal — a catalog facet that has stopped ordering is a topN facet",
			func(f *Facet) { f.Buckets.CatalogOrder = "" },
			"require CatalogOrder",
		},
		{
			"TopN alongside it",
			func(f *Facet) { f.Buckets.TopN = 15 },
			"TopN is meaningful only for the topN strategy",
		},
		{
			"claiming NonPartitioning",
			func(f *Facet) { f.NonPartitioning = "because I said so" },
			"not available to a catalog facet",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := catalogFacet()
			c.edit(&f)
			err := New().Register(withFacet(f))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error mentioning %q", err, c.want)
			}
		})
	}
	if err := New().Register(withFacet(catalogFacet())); err != nil {
		t.Fatalf("a well-formed catalog declaration must be registrable: %v", err)
	}
	// The mirror image: the two catalog fields must be refused on any OTHER strategy, or they become
	// decoration that reads as a promise.
	f := validFacet()
	f.Buckets.CatalogTable = "oikumenea.education_degree_levels"
	if err := New().Register(withFacet(f)); err == nil ||
		!strings.Contains(err.Error(), "CatalogTable is meaningful only") {
		t.Errorf("CatalogTable on an identity facet: got %v", err)
	}
	f = validFacet()
	f.Buckets.CatalogOrder = "isced_level"
	if err := New().Register(withFacet(f)); err == nil ||
		!strings.Contains(err.Error(), "CatalogOrder is meaningful only") {
		t.Errorf("CatalogOrder on an identity facet: got %v", err)
	}
}

// TestCatalogFacetsNameARealOrderedCatalog is the DDL half: for every catalog facet in the registry,
// the migrations must show that
//
//   - the facet's own column REFERENCES the declared CatalogTable (so the LEFT JOIN the aggregate
//     writes joins the rows the buckets claim to be), and
//   - CatalogOrder is a real column of that table (so `ORDER BY` orders by something).
//
// Both are read out of migrations/ rather than trusted, and both read the WHOLE file set rather than
// a CREATE TABLE block — ticket 3's rule, since the 46→15 consolidation a table's shape is not its
// CREATE TABLE (person_education_enrollments.program_id, the column this very ticket found the
// catalogue had declared non-existent, arrives by ALTER ~750 lines later).
func TestCatalogFacetsNameARealOrderedCatalog(t *testing.T) {
	cols := parseTableColumns(t)
	fkRefs := parseColumnRefs(t)
	seen := 0
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.Buckets.Strategy != StrategyCatalog {
				continue
			}
			seen++
			where := o.Type + "." + f.Key
			// 1. the column really points at the catalog.
			ref, found := fkRefs[f.Table+"."+f.Column]
			switch {
			case !found:
				t.Errorf("%s: %s.%s declares no FOREIGN KEY in migrations/ — a catalog facet's buckets ARE "+
					"another table's rows, so a column that references nothing has no catalog to enumerate "+
					"and belongs on StrategyTopN", where, f.Table, f.Column)
			case ref != f.Buckets.CatalogTable:
				t.Errorf("%s: %s.%s REFERENCES %s but the facet declares CatalogTable %s — the aggregate "+
					"would LEFT JOIN a table the column does not point at, and every bucket would come "+
					"back zero while the chart rendered", where, f.Table, f.Column, ref, f.Buckets.CatalogTable)
			}
			// 2. the ordinal is a real column of it.
			if c := cols[f.Buckets.CatalogTable]; c != nil && !c[f.Buckets.CatalogOrder] {
				t.Errorf("%s: CatalogOrder %q is not a column of %s — the ORDER BY that makes this a scale "+
					"rather than a ranking would not compile", where, f.Buckets.CatalogOrder, f.Buckets.CatalogTable)
			} else if c == nil {
				t.Errorf("%s: CatalogTable %s does not appear in migrations/ at all", where, f.Buckets.CatalogTable)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no catalog facet in the registry — this guard is vacuous, and the strategy it protects " +
			"has either been removed (delete this file) or never declared")
	}
	if len(cols) < 50 || len(fkRefs) < 50 {
		t.Fatalf("parsed %d tables and %d foreign keys from migrations/ — the parser is broken, and every "+
			"check above is vacuous", len(cols), len(fkRefs))
	}
}

var (
	catCreateRe = regexp.MustCompile(`^CREATE TABLE (?:IF NOT EXISTS )?(oikumenea\.[a-z_]+)`)
	catColRe    = regexp.MustCompile(`^\s{2,}([a-z_][a-z0-9_]*)\s+[a-z]`)
	catAlterAdd = regexp.MustCompile(`^\s*(?:ALTER TABLE (?:ONLY )?(oikumenea\.[a-z_]+)\s*)?ADD COLUMN\s+([a-z_][a-z0-9_]*)\s`)
	catRefRe    = regexp.MustCompile(`REFERENCES\s+(oikumenea\.[a-z_]+)`)
)

// parseTableColumns maps each table in migrations/ to the set of its column names, counting both the
// CREATE TABLE body and later ADD COLUMNs.
func parseTableColumns(t *testing.T) map[string]map[string]bool {
	t.Helper()
	out := map[string]map[string]bool{}
	forEachMigration(t, func(body string) {
		var current, altering string
		for _, line := range strings.Split(body, "\n") {
			if m := catCreateRe.FindStringSubmatch(line); m != nil {
				current = m[1]
				if out[current] == nil {
					out[current] = map[string]bool{}
				}
				continue
			}
			if strings.HasPrefix(line, "ALTER TABLE") {
				if m := regexp.MustCompile(`^ALTER TABLE (?:ONLY )?(oikumenea\.[a-z_]+)`).FindStringSubmatch(line); m != nil {
					altering = m[1]
				}
				current = ""
			}
			if m := catAlterAdd.FindStringSubmatch(line); m != nil {
				tbl := m[1]
				if tbl == "" {
					tbl = altering
				}
				if tbl != "" {
					if out[tbl] == nil {
						out[tbl] = map[string]bool{}
					}
					out[tbl][m[2]] = true
				}
				continue
			}
			if line == ");" || strings.HasPrefix(line, ")") {
				current = ""
				continue
			}
			if current == "" {
				continue
			}
			if m := catColRe.FindStringSubmatch(line); m != nil {
				switch m[1] {
				case "constraint", "primary", "unique", "foreign", "check":
				default:
					out[current][m[1]] = true
				}
			}
		}
	})
	return out
}

// parseColumnRefs maps "table.column" to the table its FOREIGN KEY references, over both the inline
// column form and later ALTER TABLE … ADD COLUMN … REFERENCES.
func parseColumnRefs(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	forEachMigration(t, func(body string) {
		var current, altering string
		for _, line := range strings.Split(body, "\n") {
			if m := catCreateRe.FindStringSubmatch(line); m != nil {
				current = m[1]
				continue
			}
			if strings.HasPrefix(line, "ALTER TABLE") {
				if m := regexp.MustCompile(`^ALTER TABLE (?:ONLY )?(oikumenea\.[a-z_]+)`).FindStringSubmatch(line); m != nil {
					altering = m[1]
				}
				current = ""
			}
			ref := catRefRe.FindStringSubmatch(line)
			if ref == nil {
				continue
			}
			if m := catAlterAdd.FindStringSubmatch(line); m != nil {
				tbl := m[1]
				if tbl == "" {
					tbl = altering
				}
				if tbl != "" {
					out[tbl+"."+m[2]] = ref[1]
				}
				continue
			}
			if current == "" {
				continue
			}
			if m := catColRe.FindStringSubmatch(line); m != nil {
				out[current+"."+m[1]] = ref[1]
			}
		}
	})
	return out
}

func forEachMigration(t *testing.T, fn func(body string)) {
	t.Helper()
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		fn(string(body))
	}
}
