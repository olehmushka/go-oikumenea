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

// D-ObjectFacets rules 1 and 2, enforced against the migration DDL with no database — the technique
// cmd/oikumenea/rewrap_registry_test.go established for the rewrap registry (review R-22).
//
// Rule 1: a facet may name only a PLAINTEXT column. Every envelope-encrypted pii:special value is
// ciphertext + blind index — there is nothing to GROUP BY, and D-DataScope's aggregation rule ("one
// person read must never unlock the join of ethnicity + religion + politics") forbids the surface
// regardless.
//
// Rule 2: a facet above pii:basic inherits its field's OWN read code, so M57 can omit it for a caller
// without that code instead of zeroing a bucket or failing the request.
//
// The rules are enforced at the COLUMN level, not the table level: facets.md explicitly permits a
// plaintext discriminator sitting beside an encrypted value (person_health_records.kind), so a table
// carrying a wrapped DEK is not blanket-banned — only its ciphertext columns are.

var (
	createRe  = regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?(oikumenea\.[a-z_]+)`)
	columnRe  = regexp.MustCompile(`^\s{2,}([a-z_][a-z0-9_]*)\s+([a-z]}?[a-z0-9_ ()]*)`)
	// ` +IS`, not ` IS`: a migration may ALIGN a block of COMMENT ON COLUMN statements (0011's
	// authz_unit_org does), and a single-space pattern silently skips every aligned line. That is not
	// cosmetic — an unparsed comment reads as "no classification", so an aligned pii:special column
	// would be INVISIBLE to TestNoSpecialCategoryColumnIsFaceted's contrapositive sweep while the
	// sweep's non-vacuity floor still passed on the 500-odd columns it did see. 11 columns were being
	// skipped when this was found (M59); none was pii:special, so nothing had slipped through — the
	// sweep was correct by luck rather than by construction, which is the state a guard must not be in.
	commentRe = regexp.MustCompile(`COMMENT ON COLUMN (oikumenea\.[a-z_]+)\.([a-z_][a-z0-9_]*) +IS '([a-z:]+)'`)
	// The envelope-encryption artefact suffixes actually used in migrations/ (`wrapped_dek` appears
	// both bare and prefixed). A facet may never name one of these regardless of its declared tier.
	cipherRe = regexp.MustCompile(`(^|_)(ciphertext|blind_index|bidx|wrapped_dek)$`)
)

// schema is the parsed DDL: which columns exist, whether each is NOT NULL, and its pii tier.
type schema struct {
	notNull map[string]bool   // "oikumenea.t.c" -> declared NOT NULL
	exists  map[string]bool   // "oikumenea.t.c"
	tier    map[string]string // "oikumenea.t.c" -> "none" | "basic" | "contact" | "sensitive" | "special"
}

func parseSchema(t *testing.T) schema {
	t.Helper()
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	s := schema{notNull: map[string]bool{}, exists: map[string]bool{}, tier: map[string]string{}}
	files := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		files++
		var current string
		inTable := false
		for _, line := range strings.Split(string(body), "\n") {
			if m := createRe.FindStringSubmatch(line); m != nil {
				current, inTable = m[1], true
				continue
			}
			if inTable && strings.HasPrefix(line, ")") {
				inTable = false
				continue
			}
			if inTable && current != "" {
				trimmed := strings.TrimSpace(line)
				// Skip table-level constraints and comment-only lines; only column definitions matter.
				if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "CONSTRAINT") ||
					strings.HasPrefix(trimmed, "PRIMARY KEY") || strings.HasPrefix(trimmed, "UNIQUE") ||
					strings.HasPrefix(trimmed, "CHECK") || strings.HasPrefix(trimmed, "FOREIGN KEY") {
					continue
				}
				if m := columnRe.FindStringSubmatch(line); m != nil {
					key := current + "." + m[1]
					// A column may be re-declared by a later ALTER; first declaration wins for
					// existence, and NOT NULL is sticky (ALTER ... SET NOT NULL only tightens).
					s.exists[key] = true
					if strings.Contains(line, "NOT NULL") {
						s.notNull[key] = true
					}
				}
			}
			if m := commentRe.FindStringSubmatch(line); m != nil {
				s.exists[m[1]+"."+m[2]] = true
				s.tier[m[1]+"."+m[2]] = strings.TrimPrefix(m[3], "pii:")
			}
		}
		// ALTER TABLE ... ADD COLUMN lands outside a CREATE body; pick those up too.
		for _, line := range strings.Split(string(body), "\n") {
			if m := regexp.MustCompile(`ALTER TABLE (oikumenea\.[a-z_]+)\s+ADD COLUMN (?:IF NOT EXISTS )?([a-z_][a-z0-9_]*)`).FindStringSubmatch(line); m != nil {
				key := m[1] + "." + m[2]
				s.exists[key] = true
				if strings.Contains(line, "NOT NULL") {
					s.notNull[key] = true
				}
			}
		}
	}
	if files < 10 {
		t.Fatalf("parsed only %d migration files — the walk is broken, so every assertion below would pass vacuously", files)
	}
	if len(s.tier) < 500 {
		t.Fatalf("parsed only %d pii column classifications — the COMMENT parser is broken", len(s.tier))
	}
	return s
}

// TestFacetColumnsArePlaintext is rule 1 plus the existence check that makes it meaningful: a facet
// pointed at a dropped or misspelled column would otherwise pass a tier check vacuously.
func TestFacetColumnsArePlaintext(t *testing.T) {
	s := parseSchema(t)
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			key := f.Table + "." + f.Column
			if !s.exists[key] {
				t.Errorf("%s.%s: column %s does not exist in migrations/", o.Type, f.Key, key)
				continue
			}
			tier, ok := s.tier[key]
			if !ok {
				t.Errorf("%s.%s: column %s carries no `COMMENT ON COLUMN ... IS 'pii:<tier>'` classification", o.Type, f.Key, key)
				continue
			}
			if tier == "special" {
				t.Errorf("%s.%s: column %s is pii:special — D-ObjectFacets rule 1 forbids a facet over it", o.Type, f.Key, key)
			}
			if cipherRe.MatchString(f.Column) {
				t.Errorf("%s.%s: column %s is an envelope-encryption artefact (ciphertext/blind index/wrapped DEK)", o.Type, f.Key, key)
			}
		}
	}
}

// TestNoSpecialCategoryColumnIsFaceted is the contrapositive sweep: rather than checking only the
// columns the catalog names, walk EVERY pii:special column in the schema and assert none is faceted.
// This fails whether someone adds the facet or downgrades the tier.
func TestNoSpecialCategoryColumnIsFaceted(t *testing.T) {
	s := parseSchema(t)
	faceted := map[string]string{} // column key -> "type.facetKey"
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			faceted[f.Table+"."+f.Column] = o.Type + "." + f.Key
		}
	}
	var bad []string
	for key, tier := range s.tier {
		if tier != "special" {
			continue
		}
		if by, ok := faceted[key]; ok {
			bad = append(bad, key+" (faceted by "+by+")")
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Fatalf("pii:special columns carry a facet, violating D-ObjectFacets rule 1: %s", strings.Join(bad, ", "))
	}
	// Non-vacuity: the sweep must actually have special-category columns to check.
	special := 0
	for _, tier := range s.tier {
		if tier == "special" {
			special++
		}
	}
	if special < 20 {
		t.Fatalf("only %d pii:special columns found — the parser is not seeing the encrypted surfaces", special)
	}
}

// TestFacetReadPermissionInheritance is rule 2: a column above pii:basic must carry its field's own
// read code, so M57 omits the facet for a caller lacking it. A pii:none/basic column may leave it
// empty — the endpoint's own read code is then the whole decision.
func TestFacetReadPermissionInheritance(t *testing.T) {
	s := parseSchema(t)
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			tier := s.tier[f.Table+"."+f.Column]
			switch tier {
			case "none", "basic":
				// ReadPermission optional. Setting one is allowed (it only ever narrows).
			case "contact", "sensitive":
				if f.ReadPermission == "" {
					t.Errorf("%s.%s: column is pii:%s and must inherit its field's own read code (D-ObjectFacets rule 2)", o.Type, f.Key, tier)
				}
			}
		}
	}
}

// TestNullableFacetsIncludeUnknown makes facets.md's "a (unknown) bucket is mandatory, not optional"
// an invariant: a nullable source column may not omit it. The rule is one-directional — a NOT NULL
// column MAY still opt in, which is how a cross-table facet (person.unitId, where a person with no
// membership has no row at all) declares its own absence bucket.
func TestNullableFacetsIncludeUnknown(t *testing.T) {
	s := parseSchema(t)
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			key := f.Table + "." + f.Column
			if !s.exists[key] || s.notNull[key] {
				continue
			}
			if !f.Buckets.IncludeUnknown {
				t.Errorf("%s.%s: %s is nullable, so Buckets.IncludeUnknown must be true (facets.md: the (unknown) bucket is mandatory)", o.Type, f.Key, key)
			}
		}
	}
}

// TestPlaintextGuardsFireOnAViolation proves the guards above are live rather than trivially green on
// a clean catalog: a synthetic facet over the declared-ethnicity ciphertext must be caught by both
// the tier check and the ciphertext-name check.
func TestPlaintextGuardsFireOnAViolation(t *testing.T) {
	s := parseSchema(t)

	// The real pii:special surface D-DataScope's aggregation rule exists to protect.
	const ethnicity = "oikumenea.person_ethnicities.value_ciphertext"
	if !s.exists[ethnicity] {
		t.Fatalf("fixture column %s not found — the parser or the schema moved; update this guard", ethnicity)
	}
	if got := s.tier[ethnicity]; got != "special" {
		t.Fatalf("fixture column %s is classified pii:%s, expected special", ethnicity, got)
	}
	for _, col := range []string{"value_ciphertext", "value_blind_index", "wrapped_dek", "leaning_wrapped_dek"} {
		if !cipherRe.MatchString(col) {
			t.Errorf("the encryption-artefact name check would not fire on %q", col)
		}
	}
	// ...and must NOT fire on the plaintext discriminators facets.md explicitly permits beside them.
	for _, col := range []string{"kind", "disposition", "card_type", "status"} {
		if cipherRe.MatchString(col) {
			t.Errorf("the encryption-artefact name check wrongly fires on plaintext column %q", col)
		}
	}

	// And a nullable column with no unknown bucket must be catchable.
	if !s.exists["oikumenea.person_persons.birthdate"] || s.notNull["oikumenea.person_persons.birthdate"] {
		t.Error("fixture: person_persons.birthdate should parse as an existing NULLABLE column")
	}
	if !s.notNull["oikumenea.person_persons.sex"] {
		t.Error("fixture: person_persons.sex should parse as NOT NULL")
	}
}
