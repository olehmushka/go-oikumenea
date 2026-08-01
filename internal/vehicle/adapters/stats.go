// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// This file holds the ONE predicate the list and the dashboard share, and the aggregate built on it.
//
// The M57 five express that sharing in sqlc: the stats query's candidate CTE carries the list query's
// filter block verbatim, and a build-time guard proves every facet's sqlc.narg appears in both. This
// module has no query files — it writes SQL at runtime — so the same agreement is expressed
// differently and checked differently: both paths call buildVehicleFilter, and an AST guard
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

// buildVehicleFilter is THE vehicle predicate (M58 ticket 3 / D-ObjectFacets). Every filtered read of
// this table goes through it, which is what makes `totalCount` describe exactly the set the list
// pages: a chart segment and a filter are the same act only if they are the same WHERE.
//
// It expects the candidate relation to be aliased `v` and LEFT JOINed to vehicle_models as `m` — the
// shape vehicleSelect has always had, because the list has always projected the derived brand_id.
// The keyset cursor is deliberately NOT here — a page boundary is not a filter, and folding it in
// would make the dashboard count one page instead of the whole set.
func buildVehicleFilter(a *argBuf, query string, f domain.VehicleFilter) string {
	w := "v.deleted_at IS NULL"
	if query != "" {
		w += " AND v.vin ILIKE '%'||" + a.add(query) + "||'%'"
	}
	if f.TypeID != nil {
		w += " AND v.type_id = " + a.add(*f.TypeID) + "::uuid"
	}
	if f.ModelID != nil {
		w += " AND v.model_id = " + a.add(*f.ModelID) + "::uuid"
	}
	if f.ColorID != nil {
		w += " AND v.color_id = " + a.add(*f.ColorID) + "::uuid"
	}
	// Two-hop: the brand hangs off the model, and the LEFT JOIN is already in the projection. A
	// vehicle with no model has a NULL m.brand_id and so fails this comparison, which is correct —
	// it is in the (unknown) bucket, and (unknown) is not a filterable value.
	if f.BrandID != nil {
		w += " AND m.brand_id = " + a.add(*f.BrandID) + "::uuid"
	}
	if f.Status != nil {
		w += " AND v.status = " + a.add(*f.Status)
	}
	// Inclusive bounds. A NULL manufacture_date fails both comparisons, so an undated vehicle drops
	// out whenever either bound is set — the (unknown) bucket is a distribution, never a filter value.
	if f.ManufactureDateFrom != nil {
		w += " AND v.manufacture_date >= " + a.add(*f.ManufactureDateFrom) + "::date"
	}
	if f.ManufactureDateTo != nil {
		w += " AND v.manufacture_date <= " + a.add(*f.ManufactureDateTo) + "::date"
	}
	// EXISTS, never a join: vehicle_registrations is ownership history and is one-to-many. A join
	// would multiply the vehicle across every country it has ever been registered in, making the list
	// return duplicate rows and the count exceed the number of vehicles. Confined to the ACTIVE
	// registration, of which there is at most one per vehicle.
	if f.RegistrationCountry != nil {
		w += ` AND EXISTS (SELECT 1 FROM oikumenea.vehicle_registrations r
		         WHERE r.vehicle_id = v.id AND r.status = 'active' AND r.deleted_at IS NULL
		           AND r.country_id = ` + a.add(*f.RegistrationCountry) + `::uuid)`
	}
	return w
}

// vehicleAggregate is the aggregate half: every selected facet's distribution plus the total, in ONE
// round-trip and ONE scan of the candidate set. A branch whose want_* flag is false is skipped by the
// planner, not merely dropped from the response, so asking for two facets costs two facets.
//
// ONE ARM, where the five M57 types ship an admin/scoped pair — and for external_organization's
// reason, NOT the audit ledger's. Audit's single arm IS a visibility decision, made entirely by the
// connection the query runs on. This one is the ABSENCE of a visibility decision: vehicle_vehicles has
// no row-level security and no unit reach, so `vehicle.read` held anywhere is the whole gate and
// there is nothing for a second arm to narrow.
//
// registrationCountry is grouped from the candidate CTE's LEFT-JOINed active registration, so a
// vehicle contributes exactly one row to it — the same set the EXISTS filter selects. Grouping the
// registration table directly would overlap and would need Facet.NonPartitioning; confining it to the
// active row means it partitions honestly and no exemption is taken.
//
// The %[n]s verbs are placeholders for the want_* flags and the top-N cutoff, in this order:
// 1 status, 2 typeId, 3 brandId, 4 modelId, 5 color, 6 topN, 7 manufactureDate, 8 registrationCountry.
// Nothing else is interpolated — the filter block is concatenated ahead of it and every caller value
// is bound.
const vehicleAggregate = `
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE %[1]s::boolean GROUP BY 2
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[2]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'brandId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.brand_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[3]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'modelId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.model_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[4]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'color'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.color_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[5]s::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'manufactureDate'::text, to_char(date_trunc('month', c.manufacture_date), 'YYYY-MM'),
       count(*)::bigint, NULL::bigint
FROM cand c WHERE %[7]s::boolean GROUP BY 2
UNION ALL
SELECT 'registrationCountry'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= %[6]s::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.reg_country_id::text AS k, count(*) AS n
            FROM cand c
            WHERE %[8]s::boolean
            GROUP BY 1) g) t
GROUP BY 2`

// VehicleStats aggregates the same candidate set ListVehicles pages, under the same filters.
func (r *Repository) VehicleStats(ctx context.Context, query string, f domain.VehicleFilter, sel stats.Selection) ([]stats.Group, error) {
	a := &argBuf{}
	where := buildVehicleFilter(a, query, f)
	// The candidate CTE carries the ACTIVE registration's country as a projected column, so the
	// registrationCountry branch groups a per-vehicle value rather than joining the history table a
	// second time. LEFT JOIN, so an unregistered vehicle keeps its row and lands in (unknown).
	sql := `WITH cand AS MATERIALIZED (
  SELECT v.type_id, m.brand_id, v.model_id, v.color_id, v.status, v.manufacture_date,
         (SELECT r.country_id FROM oikumenea.vehicle_registrations r
           WHERE r.vehicle_id = v.id AND r.status = 'active' AND r.deleted_at IS NULL
           LIMIT 1) AS reg_country_id
  FROM oikumenea.vehicle_vehicles v
  LEFT JOIN oikumenea.vehicle_models m ON m.id = v.model_id
  WHERE ` + where + `
)` + fmt.Sprintf(vehicleAggregate,
		a.add(sel.Wants("status")),
		a.add(sel.Wants("typeId")),
		a.add(sel.Wants("brandId")),
		a.add(sel.Wants("modelId")),
		a.add(sel.Wants("color")),
		a.add(sel.TopN()),
		a.add(sel.Wants("manufactureDate")),
		a.add(sel.Wants("registrationCountry")),
	)
	rows, err := r.c.Query(ctx, sql, a.args...)
	if err != nil {
		return nil, err
	}
	return db.ScanStatsGroups(rows)
}
