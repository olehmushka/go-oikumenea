// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/olegamysk/go-oikumenea/internal/externalorg/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// This file holds the ONE predicate the list and the dashboard share, and the aggregate built on it.
//
// The M57 five express that sharing in sqlc: the stats query's candidate CTE carries the list query's
// filter block verbatim, and a build-time guard proves every facet's sqlc.narg appears in both. This
// module has no query files — it writes SQL at runtime, by the choice recorded in the package doc
// (cross-module label lookups and a code-filtered listing) — so the same agreement is expressed
// differently and checked differently: both paths call buildOrgFilter, and an AST guard
// (pkg/facet/rawpgx_test.go) proves they do. Same invariant, different proof, because the mechanism
// the invariant was first written in does not exist here.

// argBuf accumulates positional pgx placeholders. Raw pgx has no named-parameter facility, and the
// two callers bind different numbers of arguments before and after the shared block, so the
// placeholder numbers cannot be written literally in a shared string.
type argBuf struct{ args []any }

func (a *argBuf) add(v any) string {
	a.args = append(a.args, v)
	return "$" + strconv.Itoa(len(a.args))
}

// buildOrgFilter is THE external-organization predicate (M58 / D-ObjectFacets). Every filtered read
// of this table goes through it, which is what makes `totalCount` describe exactly the set the list
// pages: a chart segment and a filter are the same act only if they are the same WHERE.
//
// It expects the candidate relation to be aliased `o` and joined to external_org_kinds as `k`.
// The keyset cursor is deliberately NOT here — a page boundary is not a filter, and folding it in
// would make the dashboard count one page instead of the whole set.
func buildOrgFilter(a *argBuf, query string, f domain.OrgFilter) string {
	w := "o.deleted_at IS NULL"
	if query != "" {
		p := a.add(query)
		w += " AND (o.name ILIKE '%'||" + p + "||'%' OR o.code ILIKE '%'||" + p + "||'%')"
	}
	if f.Kind != nil {
		// Code OR RID. Compared in TEXT contexts on both sides so the one placeholder is inferred as
		// text — a uuid column "= $n" against a text param fails with 42883.
		p := a.add(*f.Kind)
		w += " AND (k.code = " + p + " OR o.kind_id::text = " + p + ")"
	}
	if f.CountryID != nil {
		w += " AND o.country_id = " + a.add(*f.CountryID) + "::uuid"
	}
	if f.Status != nil {
		w += " AND o.status = " + a.add(*f.Status)
	}
	if f.Source != nil {
		w += " AND o.source = " + a.add(*f.Source)
	}
	if f.Confidence != nil {
		w += " AND o.confidence = " + a.add(*f.Confidence)
	}
	// Inclusive bounds. A NULL as_of fails both comparisons, so an unobserved row drops out whenever
	// either bound is set — the (unknown) bucket is a distribution, never a filterable value.
	if f.AsOfFrom != nil {
		w += " AND o.as_of >= " + a.add(*f.AsOfFrom)
	}
	if f.AsOfTo != nil {
		w += " AND o.as_of <= " + a.add(*f.AsOfTo)
	}
	return w
}

// orgAggregate is the aggregate half: every selected facet's distribution plus the total, in ONE
// round-trip and ONE scan of the candidate set. A branch whose want_* flag is false is skipped by the
// planner, not merely dropped from the response, so asking for two facets costs two facets.
//
// ONE ARM, where the five M57 types ship an admin/scoped pair — and for the OPPOSITE reason to the
// audit ledger's single arm. Audit's is a visibility decision made entirely by the connection the
// query runs on. This one is the ABSENCE of a visibility decision: external_organizations is flat
// instance-global reference data with no row-level security and no unit reach, so `externalorg.read`
// held anywhere is the whole gate and there is nothing for a second arm to narrow.
//
// The %[n]s verbs are placeholders for the want_* flags and the top-N cutoff, in this order:
// 1 status, 2 source, 3 confidence, 4 kind, 5 countryId, 6 topN, 7 asOf. Nothing else is
// interpolated — the filter block is concatenated ahead of it and every caller value is bound.
const orgAggregate = `
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[1]s::boolean GROUP BY 2
UNION ALL
SELECT 'source'::text, c.source::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[2]s::boolean GROUP BY 2
UNION ALL
SELECT 'confidence'::text, c.confidence::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[3]s::boolean GROUP BY 2
UNION ALL
SELECT 'kind'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.kind_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[4]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'countryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[5]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'asOf'::text, to_char(date_trunc('month', c.as_of), 'YYYY-MM'), count(*)::bigint, NULL::bigint
FROM cand c WHERE %[7]s::boolean GROUP BY 2`

// OrgStats aggregates the same candidate set ListOrgs pages, under the same filters.
func (r *Repository) OrgStats(ctx context.Context, query string, f domain.OrgFilter, sel stats.Selection) ([]stats.Group, error) {
	a := &argBuf{}
	where := buildOrgFilter(a, query, f)
	sql := `WITH cand AS MATERIALIZED (
  SELECT o.kind_id, o.country_id, o.status, o.source, o.confidence, o.as_of
  FROM oikumenea.external_organizations o
  JOIN oikumenea.external_org_kinds k ON k.id = o.kind_id
  WHERE ` + where + `
)` + fmt.Sprintf(orgAggregate,
		a.add(sel.Wants("status")),
		a.add(sel.Wants("source")),
		a.add(sel.Wants("confidence")),
		a.add(sel.Wants("kind")),
		a.add(sel.Wants("countryId")),
		a.add(sel.TopN()),
		a.add(sel.Wants("asOf")),
	)
	rows, err := r.c.Query(ctx, sql, a.args...)
	if err != nil {
		return nil, err
	}
	return db.ScanStatsGroups(rows)
}
