// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/audit/adapters/auditsql"
	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// Stats is the ledger's dashboard aggregate (M58 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set Query pages under the same filters.
//
// ONE query, where the five M57 types ship two. Audit visibility is the ROW-LEVEL SECURITY policy on
// audit_log — a unit_id reach probe, with NULL-unit (system) rows visible only to an instance admin —
// rather than an app-layer predicate a scoped query folds in. So there is no subject to pass and no
// admin/scoped pair to keep byte-identical; what there IS, and what makes the single arm safe, is the
// requirement that this runs on the request-pinned connection (application.Service.reader). On the
// bare pool the policy matches nothing and the same statement answers a confident ZERO.
func (r *Repository) Stats(ctx context.Context, f domain.Filter, sel stats.Selection) ([]stats.Group, error) {
	w := auditStatsWants(sel)
	rows, err := r.q.AuditStats(ctx, auditsql.AuditStatsParams{
		ActorPersonID:     textPtr(f.ActorPersonID),
		ActorType:         textStringer(f.ActorType),
		TargetType:        textPtr(f.TargetType),
		TargetID:          textPtr(f.TargetID),
		UnitID:            textPtr(f.UnitID),
		Action:            textPtr(f.Action),
		Outcome:           textStringer(f.Outcome),
		Since:             timestamptzPtr(f.Since),
		Until:             timestamptzPtr(f.Until),
		WantActorType:     w.actorType,
		WantActorPersonID: w.actorPersonID,
		WantAction:        w.action,
		WantTargetType:    w.targetType,
		WantTargetID:      w.targetID,
		WantOutcome:       w.outcome,
		WantUnitID:        w.unitID,
		WantCreatedAt:     w.createdAt,
		TopN:              w.topN,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditStatsGroup(row.Facet, row.Bucket, row.N, row.Ord))
	}
	return out, nil
}

// auditWants mirrors the query's want_* flags. One field per facet, read from the selection, so an
// unselected facet is never grouped rather than merely dropped from the response.
type auditWants struct {
	actorType     bool
	actorPersonID bool
	action        bool
	targetType    bool
	targetID      bool
	outcome       bool
	unitID        bool
	createdAt     bool
	topN          int32
}

func auditStatsWants(sel stats.Selection) auditWants {
	return auditWants{
		actorType:     sel.Wants("actorType"),
		actorPersonID: sel.Wants("actorPersonId"),
		action:        sel.Wants("action"),
		targetType:    sel.Wants("targetType"),
		targetID:      sel.Wants("targetId"),
		outcome:       sel.Wants("outcome"),
		unitID:        sel.Wants("unitId"),
		createdAt:     sel.Wants("createdAt"),
		topN:          int32(sel.TopN()),
	}
}

func auditStatsGroup(facetKey string, bucket pgtype.Text, n int64, ord pgtype.Int8) stats.Group {
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
