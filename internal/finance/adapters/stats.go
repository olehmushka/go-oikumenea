// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// This file holds the ONE predicate each list shares with its dashboard, and the aggregates built on
// them.
//
// The M57 five express that sharing in sqlc: the stats query's candidate CTE carries the list query's
// filter block verbatim, and a build-time guard proves every facet's sqlc.narg appears in both. This
// module has no query files — it writes SQL at runtime — so the same agreement is expressed
// differently and checked differently: each path pair calls one shared builder, and an AST guard
// (pkg/facet/rawpgx_test.go) proves it does.
//
// TWO object types live in this one module, which is new — both ticket-2 raw-pgx groups were one type
// per module. Each type gets its OWN builder and its OWN aggregate const: the guard keys on the
// object type and looks functions up by name, so a single shared financeAggregate would satisfy
// neither branch-coverage direction, and more importantly accounts and cards have disjoint facet
// sets that would then have to be merged into one text and split at read time.

// argBuf accumulates positional pgx placeholders. Raw pgx has no named-parameter facility, and the
// callers bind different numbers of arguments before and after the shared block, so the placeholder
// numbers cannot be written literally in a shared string.
type argBuf struct{ args []any }

func (a *argBuf) add(v any) string {
	a.args = append(a.args, v)
	return "$" + strconv.Itoa(len(a.args))
}

// ============================ accounts ============================

// buildAccountFilter is THE account predicate (M58 ticket 3 / D-ObjectFacets). Every filtered read of
// this table goes through it, which is what makes `totalCount` describe exactly the set the list
// pages.
//
// It expects the candidate relation to be aliased `a`. The keyset cursor is deliberately NOT here —
// a page boundary is not a filter, and folding it in would make the dashboard count one page instead
// of the whole set.
func buildAccountFilter(b *argBuf, f domain.AccountFilter) string {
	w := "a.deleted_at IS NULL"
	if f.InstitutionID != nil {
		w += " AND a.institution_id = " + b.add(*f.InstitutionID) + "::uuid"
	}
	if f.Currency != nil {
		w += " AND a.currency = " + b.add(*f.Currency)
	}
	if f.AccountTypeID != nil {
		w += " AND a.account_type_id = " + b.add(*f.AccountTypeID) + "::uuid"
	}
	if f.Status != nil {
		w += " AND a.status = " + b.add(*f.Status)
	}
	// holderKind is the one predicate here that leaves finance_accounts. It is an EXISTS semi-join
	// confined to the ACTIVE PRIMARY holder — the row finance_account_holders_primary_active admits at
	// most one of per account — so filtering cannot multiply a jointly held account, and the filtered
	// set is exactly the set the matching aggregate branch counts. Reaching this arg at all requires
	// finance.holder.read (facet.FilterReadCodes, gated in the transport).
	if f.HolderKind != nil {
		w += ` AND EXISTS (SELECT 1 FROM oikumenea.finance_account_holders h
              WHERE h.account_id = a.id AND h.deleted_at IS NULL
                AND h.role = 'primary' AND h.effective_to IS NULL
                AND h.holder_kind = ` + b.add(*f.HolderKind) + `)`
	}
	return w
}

// accountAggregate is the aggregate half: every selected facet's distribution plus the total, in ONE
// round-trip and ONE scan of the candidate set. A branch whose want_* flag is false is skipped by the
// planner, not merely dropped from the response.
//
// ONE ARM, for external_organization's reason: finance_accounts has no row-level security and no unit
// reach, so `finance.read` held anywhere is the whole gate and there is nothing for a second arm to
// narrow.
//
// The %[n]s verbs are placeholders for the want_* flags and the top-N cutoff, in this order:
// 1 status, 2 institutionId, 3 accountTypeId, 4 currency, 5 topN.
const accountAggregate = `
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[1]s::boolean GROUP BY 2
UNION ALL
SELECT 'institutionId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.institution_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[2]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'accountTypeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.account_type_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[3]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'currency'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[5]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.currency::text AS k, count(*) AS n
            FROM cand c
            WHERE %[4]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
-- holderKind: the FIRST GATED distribution to ship (M59 / D-ObjectFacets rule 2). The candidate
-- carries the kind of its ACTIVE PRIMARY holder, resolved once in the CTE by a LATERAL that the
-- primary-active partial UNIQUE makes single-valued — so this partitions, and a jointly held account
-- is counted once, under the holder the account is actually in the name of. NULL (no active primary
-- holder) lands in (unknown), which keeps the sum equal to totalCount.
--
-- The want flag carries the ENTITLEMENT as well as the selection: stats.Selection has already dropped
-- this facet for a caller without finance.holder.read, so the branch is skipped by the planner and the
-- kind is never grouped. The CTE's LATERAL still runs, which is harmless — it projects a value that
-- nothing reads — and keeping the projection unconditional is what lets the filter and the aggregate
-- share one candidate set.
SELECT 'holderKind'::text, c.holder_kind::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[6]s::boolean GROUP BY 2`

// AccountStats aggregates the same candidate set ListAccounts pages, under the same filters.
func (r *Repository) AccountStats(ctx context.Context, f domain.AccountFilter, sel stats.Selection) ([]stats.Group, error) {
	b := &argBuf{}
	where := buildAccountFilter(b, f)
	sql := `WITH cand AS MATERIALIZED (
  SELECT a.institution_id, a.currency, a.account_type_id, a.status, h.holder_kind
  FROM oikumenea.finance_accounts a
  LEFT JOIN LATERAL (
    SELECT hh.holder_kind FROM oikumenea.finance_account_holders hh
    WHERE hh.account_id = a.id AND hh.deleted_at IS NULL
      AND hh.role = 'primary' AND hh.effective_to IS NULL
    LIMIT 1
  ) h ON true
  WHERE ` + where + `
)` + fmt.Sprintf(accountAggregate,
		b.add(sel.Wants("status")),
		b.add(sel.Wants("institutionId")),
		b.add(sel.Wants("accountTypeId")),
		b.add(sel.Wants("currency")),
		b.add(sel.TopN()),
		b.add(sel.Wants("holderKind")),
	)
	rows, err := r.c.Query(ctx, sql, b.args...)
	if err != nil {
		return nil, err
	}
	return db.ScanStatsGroups(rows)
}

// ============================ cards ============================

// buildCardFilter is THE card predicate for the instance-wide registry (M58 ticket 3). It expects the
// candidate relation to be aliased `c`.
//
// Note what is NOT here and cannot be: there is no PAN predicate. The column is envelope-encrypted,
// so there is nothing to compare; the blind index supports equality on a KNOWN PAN, which is a lookup
// (getCard's job), not a filter over a registry.
func buildCardFilter(b *argBuf, f domain.CardFilter) string {
	w := "c.deleted_at IS NULL"
	if f.NetworkID != nil {
		w += " AND c.network_id = " + b.add(*f.NetworkID) + "::uuid"
	}
	if f.CardType != nil {
		w += " AND c.card_type = " + b.add(*f.CardType)
	}
	if f.Status != nil {
		w += " AND c.status = " + b.add(*f.Status)
	}
	return w
}

// cardAggregate is the card registry's aggregate half. ONE ARM, same reason as accountAggregate.
//
// Three facets, all of them describing the INSTRUMENT. There is deliberately no distribution over
// `bin` or `last_four` even though both are clear columns: they identify one card rather than
// describing a population, and a top-N over four-digit suffixes ranks nothing meaningful.
//
// The %[n]s verbs are: 1 cardType, 2 status, 3 networkId, 4 topN.
const cardAggregate = `
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'cardType'::text, c.card_type::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[1]s::boolean GROUP BY 2
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[2]s::boolean GROUP BY 2
UNION ALL
SELECT 'networkId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[4]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.network_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[3]s::boolean
            GROUP BY 1) g) t
GROUP BY 2`

// CardStats aggregates the same candidate set ListCards pages, under the same filters.
func (r *Repository) CardStats(ctx context.Context, f domain.CardFilter, sel stats.Selection) ([]stats.Group, error) {
	b := &argBuf{}
	where := buildCardFilter(b, f)
	sql := `WITH cand AS MATERIALIZED (
  SELECT c.network_id, c.card_type, c.status
  FROM oikumenea.finance_cards c
  WHERE ` + where + `
)` + fmt.Sprintf(cardAggregate,
		b.add(sel.Wants("cardType")),
		b.add(sel.Wants("status")),
		b.add(sel.Wants("networkId")),
		b.add(sel.TopN()),
	)
	rows, err := r.c.Query(ctx, sql, b.args...)
	if err != nil {
		return nil, err
	}
	return db.ScanStatsGroups(rows)
}
