// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/membership/adapters/membershipsql"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// VisiblePersonStatsForSubject is the READ-SCOPE arm of the directory dashboard (M57 /
// D-ObjectFacets): the same aggregate person's PersonStats computes, over the candidate set narrowed
// by the subject's effective readable reach — folded into the SQL, so every count is taken INSIDE the
// visibility predicate rather than over a set that is trimmed afterwards.
//
// Unlike the LIST arms it makes no sparse/dense dispatch. That split exists because a LIMIT cannot
// terminate early once the planner drives from the reach side; an aggregate has no LIMIT to spoil, so
// the uncorrelated reach set — evaluated once and probed as a hash — is the right shape at every
// reach size. Measured in review-2026-07 (M57 ticket 1).
func (r *Repository) VisiblePersonStatsForSubject(ctx context.Context, subjectPersonID string, f persondomain.PersonFilter, sel stats.Selection) ([]stats.Group, error) {
	fa := personFacetArgs(f)
	w := personStatsWants(sel)
	if f.Query != "" {
		rows, err := r.q.VisiblePersonStatsForSubjectSearch(ctx, membershipsql.VisiblePersonStatsForSubjectSearchParams{
			SubjectPersonID:    subjectPersonID,
			Query:              pgtype.Text{String: f.Query, Valid: true},
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
			out = append(out, statsGroup(row.Facet, row.Bucket, row.N, row.Ord))
		}
		return out, nil
	}
	rows, err := r.q.VisiblePersonStatsForSubject(ctx, membershipsql.VisiblePersonStatsForSubjectParams{
		SubjectPersonID:    subjectPersonID,
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
		out = append(out, statsGroup(row.Facet, row.Bucket, row.N, row.Ord))
	}
	return out, nil
}

// statsWants / personStatsWants mirror person's mapping of a selection onto the per-branch flags.
// Duplicated rather than shared because the two modules bind different generated param structs; the
// facet KEYS they name are the catalog's, and pkg/facet is what keeps those from drifting.
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

// statsGroup maps one raw aggregate row to the kernel's Group; a NULL bucket stays NULL (it is the
// (unknown) bucket, not an empty string).
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
