// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/person/adapters/personsql"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// PersonStats is the INSTANCE-ADMIN dashboard aggregate (M57 / D-ObjectFacets): one round-trip
// returning every selected facet's distribution plus the total, over the same candidate set
// ListPersons pages.
//
// It dispatches on the text query exactly as ListPersons does, and for the same R-21 reason — a
// nullable trigram predicate is not indexable, so the two plan shapes stay two queries. A scoped
// caller never reaches here; membership's VisiblePersonStatsForSubject is the read-scope arm.
func (r *Repository) PersonStats(ctx context.Context, f domain.PersonFilter, sel stats.Selection) ([]stats.Group, error) {
	fa := personFacetArgs(f)
	w := personStatsWants(sel)
	if q := strings.TrimSpace(f.Query); q != "" {
		rows, err := r.q.PersonStatsSearch(ctx, personsql.PersonStatsSearchParams{
			Query:              pgtype.Text{String: q, Valid: true},
			Sex:                fa.sex,
			Status:             fa.status,
			BirthdateFrom:      fa.birthdateFrom,
			BirthdateTo:        fa.birthdateTo,
			CountryOfBirthID:   fa.countryOfBirth,
			RankID:             fa.rankID,
			HasAccount:         fa.hasAccount,
			FilterUnitID:       fa.unitID,
			FilterGraph:        fa.graph,
			WantSex:            w.sex,
			WantStatus:         w.status,
			WantBirthdate:      w.birthdate,
			WantCountryOfBirth: w.countryOfBirth,
			WantRankID:         w.rankID,
			WantUnitID:         w.unitID,
			WantHasAccount:     w.hasAccount,
			TopN:               w.topN,
		})
		if err != nil {
			return nil, err
		}
		out := make([]stats.Group, 0, len(rows))
		for _, row := range rows {
			out = append(out, personStatsGroup(row.Facet, row.Bucket, row.N, row.Ord))
		}
		return out, nil
	}
	rows, err := r.q.PersonStats(ctx, personsql.PersonStatsParams{
		Sex:                fa.sex,
		Status:             fa.status,
		BirthdateFrom:      fa.birthdateFrom,
		BirthdateTo:        fa.birthdateTo,
		CountryOfBirthID:   fa.countryOfBirth,
		RankID:             fa.rankID,
		HasAccount:         fa.hasAccount,
		FilterUnitID:       fa.unitID,
		FilterGraph:        fa.graph,
		WantSex:            w.sex,
		WantStatus:         w.status,
		WantBirthdate:      w.birthdate,
		WantCountryOfBirth: w.countryOfBirth,
		WantRankID:         w.rankID,
		WantUnitID:         w.unitID,
		WantHasAccount:     w.hasAccount,
		TopN:               w.topN,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, personStatsGroup(row.Facet, row.Bucket, row.N, row.Ord))
	}
	return out, nil
}

// statsWants is one selection projected onto the per-branch flags the stats query binds. An
// unselected facet's branch is skipped by the planner (a one-time false filter), so a facet the
// caller may not read is not merely hidden — it is never grouped.
type statsWants struct {
	sex, status, birthdate, countryOfBirth, rankID, unitID, hasAccount bool
	topN                                                               int32
}

func personStatsWants(sel stats.Selection) statsWants {
	return statsWants{
		sex:            sel.Wants("sex"),
		status:         sel.Wants("status"),
		birthdate:      sel.Wants("birthdate"),
		countryOfBirth: sel.Wants("countryOfBirth"),
		rankID:         sel.Wants("rankId"),
		unitID:         sel.Wants("unitId"),
		hasAccount:     sel.Wants("hasAccount"),
		topN:           int32(sel.TopN()),
	}
}

// personStatsGroup maps one raw aggregate row to the kernel's Group. A NULL bucket stays NULL: it is
// the (unknown) bucket, and collapsing it to "" here would merge it with a real empty-string value.
func personStatsGroup(facetKey string, bucket pgtype.Text, n int64, ord pgtype.Int8) stats.Group {
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
