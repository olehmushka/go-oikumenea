// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/order/adapters/ordersql"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// OrderStats is the order-register dashboard aggregate (M57 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListOrders pages under the same filters.
//
// An empty subjectPersonID is the INSTANCE-ADMIN arm (no visibility predicate at all); otherwise reach
// on the issuing unit is folded into the candidate CTE, so every count is taken inside the visibility
// predicate rather than over a set trimmed afterwards.
//
// Unlike the LIST, there is no sparse/dense dispatch: an aggregate has no LIMIT for the materialized
// reach set to spoil, and the measurement agrees at every reach size (see the query's comment).
func (r *Repository) OrderStats(ctx context.Context, subjectPersonID string, f domain.OrderFilter, sel stats.Selection) ([]stats.Group, error) {
	fa := orderFacetArgs(f)
	w := orderStatsWants(sel)
	if subjectPersonID != "" {
		rows, err := r.q.OrderStatsForSubject(ctx, ordersql.OrderStatsForSubjectParams{
			SubjectPersonID:   subjectPersonID,
			IssuingUnitID:     fa.issuingUnitID,
			OrderTypeID:       fa.orderTypeID,
			Status:            fa.status,
			IssuedOnFrom:      fa.issuedOnFrom,
			IssuedOnTo:        fa.issuedOnTo,
			WantIssuingUnitID: w.issuingUnitID,
			WantOrderTypeID:   w.orderTypeID,
			WantStatus:        w.status,
			WantIssuedOn:      w.issuedOn,
			TopN:              w.topN,
		})
		if err != nil {
			return nil, err
		}
		out := make([]stats.Group, 0, len(rows))
		for _, row := range rows {
			out = append(out, statsGroup(row.Facet, row.Bucket, row.N, row.Ord))
		}
		return out, nil
	}
	rows, err := r.q.OrderStats(ctx, ordersql.OrderStatsParams{
		IssuingUnitID:     fa.issuingUnitID,
		OrderTypeID:       fa.orderTypeID,
		Status:            fa.status,
		IssuedOnFrom:      fa.issuedOnFrom,
		IssuedOnTo:        fa.issuedOnTo,
		WantIssuingUnitID: w.issuingUnitID,
		WantOrderTypeID:   w.orderTypeID,
		WantStatus:        w.status,
		WantIssuedOn:      w.issuedOn,
		TopN:              w.topN,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, statsGroup(row.Facet, row.Bucket, row.N, row.Ord))
	}
	return out, nil
}

// orderStatsWantFlags is one selection projected onto the per-branch flags the query binds. An
// unselected facet's branch becomes a one-time false filter the planner skips, so a facet the caller
// may not read (or did not ask for) is never grouped — not merely dropped from the response.
type orderStatsWantFlags struct {
	issuingUnitID, orderTypeID, status, issuedOn bool
	topN                                         int32
}

func orderStatsWants(sel stats.Selection) orderStatsWantFlags {
	return orderStatsWantFlags{
		issuingUnitID: sel.Wants("issuingUnitId"),
		orderTypeID:   sel.Wants("orderTypeId"),
		status:        sel.Wants("status"),
		issuedOn:      sel.Wants("issuedOn"),
		topN:          int32(sel.TopN()),
	}
}

// statsGroup maps one raw aggregate row to the kernel's Group. A NULL bucket stays NULL: it is the
// (unknown) bucket, and collapsing it to "" here would merge it with a real empty-string value.
func statsGroup(facetKey string, bucket pgtype.Text, n int64, ord pgtype.Int8) stats.Group {
	g := stats.Group{Facet: facetKey, Count: n}
	if bucket.Valid {
		k := bucket.String
		g.Key = &k
	}
	if ord.Valid {
		o := ord.Int64
		g.Ord = &o
	}
	return g
}
