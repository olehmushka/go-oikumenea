// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// This file holds the ONE predicate the taxonomy list and its dashboard share, and the aggregate
// built on it.
//
// The M57 five express that sharing in sqlc: the stats query's candidate CTE carries the list query's
// filter block verbatim, and a build-time guard proves every facet's sqlc.narg appears in both. This
// module has no query files — it writes SQL at runtime, by the choice recorded in the package doc
// (the taxonomy is closure- and resolution-heavy) — so the same agreement is expressed differently
// and checked differently: both paths call buildTaxonFilter, and an AST guard
// (pkg/facet/rawpgx_test.go) proves they do. Same invariant, different proof.

// argBuf accumulates positional pgx placeholders. Raw pgx has no named-parameter facility, and the
// two callers bind different numbers of arguments before and after the shared block, so the
// placeholder numbers cannot be written literally in a shared string.
type argBuf struct{ args []any }

func (a *argBuf) add(v any) string {
	a.args = append(a.args, v)
	return "$" + strconv.Itoa(len(a.args))
}

// effectiveTags resolves each taxon's EFFECTIVE theism classification: the tags declared by its
// NEAREST DECLARING ancestor, which is what getEffectiveClassifications returns and what the object
// view shows. Single-sourced here because it is used twice — once as the `classification` filter's
// predicate and once as that facet's GROUP BY — and those two must agree exactly or a bucket would
// count rows its own click-through does not return.
//
// rank() rather than row_number(): a taxon may carry SEVERAL tags at the nearest declaring depth
// (dualistic AND monotheistic), and all of them are effective. That multiplicity is exactly why the
// facet is declared NonPartitioning.
//
// The closure's reflexive (t,t,0) row is what makes a taxon's OWN declared tags win over an
// ancestor's — depth 0 sorts first — so self-declaration needs no special case.
const effectiveTags = `
  SELECT x.taxon_id, x.classification_id
  FROM (
    SELECT c.descendant_id AS taxon_id, tc.classification_id,
           rank() OVER (PARTITION BY c.descendant_id ORDER BY c.depth) AS rnk
    FROM oikumenea.religion_taxa_closure c
    JOIN oikumenea.religion_taxon_classifications tc ON tc.taxon_id = c.ancestor_id
  ) x
  WHERE x.rnk = 1`

// buildTaxonFilter is THE taxonomy predicate (M58 / D-ObjectFacets). Every filtered read of
// religion_taxa goes through it, which is what makes `totalCount` describe exactly the set the list
// pages.
//
// It expects the candidate relation aliased `t` and joined to religion_taxon_ranks as `rk`. The
// keyset cursor is deliberately NOT here — a page boundary is not a filter, and folding it in would
// make the dashboard count one page instead of the whole set.
func buildTaxonFilter(a *argBuf, query string, f domain.TaxonFilter) string {
	w := "t.deleted_at IS NULL"
	if f.Rank != nil {
		// Code OR RID. Compared in TEXT contexts on both sides so the one placeholder is inferred as
		// text — a uuid column "= $n" against a text param fails with 42883.
		p := a.add(*f.Rank)
		w += " AND (rk.code = " + p + " OR t.rank_id::text = " + p + ")"
	}
	if f.Parent != nil {
		// PROPER descendants: depth > 0 excludes the reflexive row, so the ancestor itself is not in
		// its own subtree. The `subtree` facet counts with the same depth > 0, which is the whole
		// reason a bucket's count equals the number of rows this returns for it.
		w += " AND t.id IN (SELECT descendant_id FROM oikumenea.religion_taxa_closure" +
			" WHERE ancestor_id = " + a.add(*f.Parent) + "::uuid AND depth > 0)"
	}
	if f.Religion != nil {
		p := a.add(*f.Religion)
		w += " AND t.religion_id IN (SELECT id FROM oikumenea.religion_taxa" +
			" WHERE code = " + p + " OR id::text = " + p + ")"
	}
	if f.Classification != nil {
		p := a.add(*f.Classification)
		w += " AND t.id IN (SELECT e.taxon_id FROM (" + effectiveTags + ") e" +
			" JOIN oikumenea.religion_classifications rc ON rc.id = e.classification_id" +
			" WHERE rc.code = " + p + " OR rc.id::text = " + p + ")"
	}
	if query != "" {
		p := a.add("%" + query + "%")
		w += " AND (t.code ILIKE " + p + " OR t.name ILIKE " + p + ")"
	}
	return w
}

// taxonAggregate is the aggregate half: every selected facet's distribution plus the total, in ONE
// round-trip over the candidate set. A branch whose want_* flag is false is skipped by the planner,
// not merely dropped from the response.
//
// ONE ARM, for the same reason external_organizations has one: the taxonomy is flat instance-global
// reference data with no row-level security and no unit reach, so there is no visibility decision for
// a second arm to make.
//
// TWO of the four branches do not partition the candidate set, and both are properties of a tree:
//
//   - `subtree` joins the closure and groups by ANCESTOR, so a taxon three levels deep is counted in
//     three buckets. That is what makes the chart drillable — the bucket's count is a whole subtree's
//     size and clicking it (parent=<ancestor>) returns precisely those rows, whereupon re-grouping
//     within them yields that subtree's own internal nodes, recursively.
//
//     The second join back to `cand` is what keeps that promise at depth, and it is not an
//     optimisation. Without it, once the caller has filtered to X's subtree the distribution still
//     offers X's OWN ancestors as buckets — and `parent` is single-valued, so clicking one REPLACES
//     the anchor and WIDENS the set instead of narrowing it, landing on more rows than the bucket
//     counted. Requiring the ancestor to be a candidate itself confines the buckets to taxa strictly
//     inside the current view, where subtree(Y) is contained in the candidate set and the count and
//     the click-through cannot disagree. At the top level every ancestor is a candidate anyway, so
//     the rule is uniform rather than conditional on the filter.
//
//   - `classification` counts EFFECTIVE tags, and a taxon may resolve to several.
//
// `rankId` supplies its own ORD — the rank's ordinal — because a rank ladder (religion, branch,
// tradition, denomination) re-sorted by frequency loses the only ordering that means anything. The
// kernel's topNBuckets honours a SQL-supplied Ord when every row carries one.
//
// The %[n]s verbs are placeholders for the want_* flags and the top-N cutoff, in this order:
// 1 rankId, 2 religionId, 3 subtree, 4 classification, 5 topN.
const taxonAggregate = `
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'rankId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, min(t.ordinal)::bigint
FROM (SELECT g.k, g.n, g.ordinal, row_number() OVER (ORDER BY (g.k IS NULL), g.ordinal, g.k) AS rk
      FROM (SELECT c.rank_id::text AS k, min(c.rank_ordinal) AS ordinal, count(*) AS n
            FROM cand c
            WHERE %[1]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'religionId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.religion_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[2]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'subtree'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT cl.ancestor_id::text AS k, count(*) AS n
            FROM cand c
            JOIN oikumenea.religion_taxa_closure cl ON cl.descendant_id = c.id AND cl.depth > 0
            JOIN cand ca ON ca.id = cl.ancestor_id
            WHERE %[3]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'classification'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT e.classification_id::text AS k, count(*) AS n
            FROM cand c
            LEFT JOIN eff e ON e.taxon_id = c.id
            WHERE %[4]s::boolean
            GROUP BY 1) g) t
GROUP BY 2`

// TaxonStats aggregates the same candidate set ListTaxa pages, under the same filters.
func (r *Repository) TaxonStats(ctx context.Context, query string, f domain.TaxonFilter, sel stats.Selection) ([]stats.Group, error) {
	a := &argBuf{}
	where := buildTaxonFilter(a, query, f)
	sql := `WITH eff AS (` + effectiveTags + `), cand AS MATERIALIZED (
  SELECT t.id, t.rank_id, rk.ordinal AS rank_ordinal, t.religion_id
  FROM oikumenea.religion_taxa t
  JOIN oikumenea.religion_taxon_ranks rk ON rk.id = t.rank_id
  WHERE ` + where + `
)` + fmt.Sprintf(taxonAggregate,
		a.add(sel.Wants("rankId")),
		a.add(sel.Wants("religionId")),
		a.add(sel.Wants("subtree")),
		a.add(sel.Wants("classification")),
		a.add(sel.TopN()),
	)
	rows, err := r.c.Query(ctx, sql, a.args...)
	if err != nil {
		return nil, err
	}
	return db.ScanStatsGroups(rows)
}
