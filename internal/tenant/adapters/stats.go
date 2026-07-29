// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/tenant/adapters/tenantsql"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// UnitStats is the unit dashboard aggregate (M57 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListUnits pages under the same filters.
//
// subjectPersonID empty means the INSTANCE-ADMIN arm (no visibility predicate). Otherwise the shadow
// gate is folded into the candidate CTE, which is the whole point of a separate query: on the list
// the gate trims the page after it is cut — right for a page, wrong for a count.
func (r *Repository) UnitStats(ctx context.Context, subjectPersonID string, f domain.UnitFilter, sel stats.Selection) ([]stats.Group, error) {
	w := unitStatsWants(sel)
	if subjectPersonID != "" {
		rows, err := r.q.UnitStatsForSubject(ctx, tenantsql.UnitStatsForSubjectParams{
			SubjectPersonID: subjectPersonID,
			OrgID:           f.OrgID,
			DomainID:        textPtr(f.DomainID),
			KindID:          textPtr(f.KindID),
			Level:           int2Ptr(f.Level),
			Visibility:      textPtr(f.Visibility),
			State:           textPtr(f.State),
			PdpScoped:       boolPtr(f.PDPScoped),
			WantOrg:         w.org,
			WantDomain:      w.domain,
			WantUnitKind:    w.unitKind,
			WantLevel:       w.level,
			WantVisibility:  w.visibility,
			WantState:       w.state,
			WantPdpScoped:   w.pdpScoped,
		})
		if err != nil {
			return nil, err
		}
		out := make([]stats.Group, 0, len(rows))
		for _, row := range rows {
			out = append(out, unitStatsGroup(row.Facet, row.Bucket, row.N))
		}
		return out, nil
	}
	rows, err := r.q.UnitStats(ctx, tenantsql.UnitStatsParams{
		OrgID:          f.OrgID,
		DomainID:       textPtr(f.DomainID),
		KindID:         textPtr(f.KindID),
		Level:          int2Ptr(f.Level),
		Visibility:     textPtr(f.Visibility),
		State:          textPtr(f.State),
		PdpScoped:      boolPtr(f.PDPScoped),
		WantOrg:        w.org,
		WantDomain:     w.domain,
		WantUnitKind:   w.unitKind,
		WantLevel:      w.level,
		WantVisibility: w.visibility,
		WantState:      w.state,
		WantPdpScoped:  w.pdpScoped,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, unitStatsGroup(row.Facet, row.Bucket, row.N))
	}
	return out, nil
}

// unitStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is a
// one-time false filter the planner skips, so it is never grouped.
type unitStatsWantFlags struct {
	org, domain, unitKind, level, visibility, state, pdpScoped bool
}

func unitStatsWants(sel stats.Selection) unitStatsWantFlags {
	return unitStatsWantFlags{
		org:        sel.Wants("org"),
		domain:     sel.Wants("domain"),
		unitKind:   sel.Wants("unitKind"),
		level:      sel.Wants("level"),
		visibility: sel.Wants("visibility"),
		state:      sel.Wants("state"),
		pdpScoped:  sel.Wants("pdpScoped"),
	}
}

// unitStatsGroup maps one raw aggregate row; a NULL bucket stays NULL (the (unknown) bucket). No
// ordinal: no unit facet has an inherent order beyond its declared band/CHECK-set order, which the
// kernel supplies from the catalog.
func unitStatsGroup(facetKey string, bucket pgtype.Text, n int64) stats.Group {
	g := stats.Group{Facet: facetKey, Count: n}
	if bucket.Valid {
		k := bucket.String
		g.Key = &k
	}
	return g
}
