// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"strings"
	"testing"
)

// The stats-aggregate parity guard (M57 / D-ObjectFacets).
//
// A module's dashboard query has two halves: a CANDIDATE CTE (the filter block plus, per arm, a
// trigram id-set and/or the visibility predicate) and an AGGREGATE half (one UNION ALL branch per
// facet). The filter half is already policed by sqlparity_test.go; this guard covers the aggregate
// half, whose copies differ only by which arm they sit in.
//
// Why copies exist at all: the arms are separate queries because a nullable trigram predicate is not
// indexable (R-21) and because the admin arm must carry no visibility predicate whatsoever. That is
// a plan-shape decision, not a style one — but it means the same 60 lines of GROUP BY appear in
// every arm, and a facet fixed in one arm and forgotten in another would give an admin and a scoped
// caller different distributions of the same world.
//
// So: the aggregate half must be BYTE-IDENTICAL across a type's arms. The four person arms are
// generated from one template for exactly this reason, and this test is what keeps them that way.

// statsAggregateGroups are the arm sets whose aggregate halves must match, one entry per object type.
var statsAggregateGroups = []struct {
	objectType string
	queries    []struct{ module, query string }
}{
	{"person", []struct{ module, query string }{
		{"person", "PersonStats"},
		{"person", "PersonStatsSearch"},
		{"membership", "VisiblePersonStatsForSubject"},
		{"membership", "VisiblePersonStatsForSubjectSearch"},
	}},
	{"unit", []struct{ module, query string }{
		{"tenant", "UnitStats"},
		{"tenant", "UnitStatsForSubject"},
	}},
}

func TestStatsAggregateHalvesAreIdentical(t *testing.T) {
	for _, g := range statsAggregateGroups {
		want := ""
		for _, q := range g.queries {
			got := aggregateHalf(t, q.module, q.query)
			if len(got) < 200 {
				t.Fatalf("%s.%s: extracted a %d-char aggregate half — the splitter is broken and every "+
					"comparison here would be vacuous", q.module, q.query, len(got))
			}
			if want == "" {
				want = got
				continue
			}
			if got != want {
				t.Errorf("%s: %s.%s's aggregate half differs from %s.%s's. The arms must group IDENTICALLY; "+
					"they may differ only in their candidate CTE (the trigram set / the visibility predicate).\n"+
					"first difference at offset %d:\n  want …%s…\n  got  …%s…",
					g.objectType, q.module, q.query, g.queries[0].module, g.queries[0].query,
					firstDiff(want, got), excerpt(want, firstDiff(want, got)), excerpt(got, firstDiff(want, got)))
			}
		}
	}
}

// TestEveryStatsBranchNamesADeclaredFacet closes the other direction: an aggregate branch must group
// by a facet the catalog declares, and every declared facet must have a branch. Without it, a facet
// could be selected, flagged and then silently never counted — a zeroed chart rather than an error.
func TestEveryStatsBranchNamesADeclaredFacet(t *testing.T) {
	for _, g := range statsAggregateGroups {
		o, ok := Default.Get(g.objectType)
		if !ok {
			t.Fatalf("%s is not registered", g.objectType)
		}
		body := aggregateHalf(t, g.queries[0].module, g.queries[0].query)
		for _, f := range o.Facets {
			// Each branch labels its rows with the facet key: SELECT 'sex'::text, …
			if !strings.Contains(body, "'"+f.Key+"'::text") {
				t.Errorf("%s.%s has no aggregate branch in %s.%s — the facet would be selectable but never "+
					"counted, which reads as an all-zero chart rather than an error",
					g.objectType, f.Key, g.queries[0].module, g.queries[0].query)
			}
			// …and reads its want flag, so an unselected facet is skipped rather than merely hidden.
			if !strings.Contains(body, "want_"+snake(f.Key)) {
				t.Errorf("%s.%s's branch does not read sqlc.arg('want_%s') — an unselected or unreadable "+
					"facet would still be grouped", g.objectType, f.Key, snake(f.Key))
			}
		}
		if !strings.Contains(body, "'(total)'::text") {
			t.Errorf("%s: no total branch — totalCount is the one number every dashboard shows", g.objectType)
		}
	}
}

// aggregateHalf returns everything from the first UNION-ALL-joined aggregate onward: the query body
// after its candidate CTE closes. The split point is the `)` line that ends `WITH cand AS
// MATERIALIZED (`, which every stats arm shares by construction.
func aggregateHalf(t *testing.T, module, query string) string {
	t.Helper()
	body := queryBody(t, module, query)
	marker := "\n)\nSELECT '(total)'::text"
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("%s.%s does not have the expected `WITH cand AS MATERIALIZED (…)` + total-branch shape", module, query)
	}
	return body[i+len("\n)\n"):]
}

// snake maps a lowerCamel facet key to the snake_case want_* parameter name the SQL binds.
func snake(key string) string {
	var b strings.Builder
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c >= 'A' && c <= 'Z' {
			b.WriteByte('_')
			b.WriteByte(c + ('a' - 'A'))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func firstDiff(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func excerpt(s string, at int) string {
	lo, hi := at-40, at+40
	if lo < 0 {
		lo = 0
	}
	if hi > len(s) {
		hi = len(s)
	}
	return strings.ReplaceAll(s[lo:hi], "\n", "⏎")
}
