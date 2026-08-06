// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/education/adapters/educationsql"
	"github.com/olehmushka/go-oikumenea/internal/education/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// The institution dashboard aggregate (M58 ticket 5 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListInstitutions pages under the same
// filters.
//
// FOUR arms, for the reason company's are four: an institution has BOTH a visibility gate (it IS a
// `university`-domain tenant organization — M41 / D-UnifiedOrgGraph) and an R-21 search twin, so the
// square is {plain, search} × {instance-admin, visibility-scoped}.

// textPtr maps an optional filter value onto a sqlc narg: nil is SQL NULL, which each predicate reads
// as "criterion disabled".
func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// institutionQuery is the ONE place the search/plain branch condition is written, so ListInstitutions
// and InstitutionStats cannot drift into picking different plan shapes for the same input.
func institutionQuery(f domain.InstitutionFilter) string { return strings.TrimSpace(f.Query) }

// InstitutionStats answers the whole dashboard in one statement.
//
// subjectPersonID empty means the INSTANCE-ADMIN arm, which carries no visibility predicate at all.
// Otherwise the organization shadow gate is folded into the candidate CTE — right for a count, where
// trimming the page after the fact (what gateInstitutions does on the list) would undercount silently.
func (r *Repository) InstitutionStats(ctx context.Context, subjectPersonID string, f domain.InstitutionFilter, sel stats.Selection) ([]stats.Group, error) {
	w := institutionStatsWants(sel)
	q := institutionQuery(f)
	switch {
	case subjectPersonID != "" && q != "":
		rows, err := r.q.InstitutionStatsForSubjectSearch(ctx, educationsql.InstitutionStatsForSubjectSearchParams{
			SubjectPersonID: subjectPersonID,
			Query:           pgtype.Text{String: q, Valid: true},
			KindID:          textPtr(f.KindID),
			CountryID:       textPtr(f.CountryID),
			FoundedOnFrom:   datePtr(f.FoundedOnFrom),
			FoundedOnTo:     datePtr(f.FoundedOnTo),
			State:           textPtr(f.State),
			TopN:            int32(sel.TopN()),
			WantKindID:      w.kindID,
			WantCountryID:   w.countryID,
			WantFoundedOn:   w.foundedOn,
			WantState:       w.state,
		})
		if err != nil {
			return nil, err
		}
		return institutionStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case subjectPersonID != "":
		rows, err := r.q.InstitutionStatsForSubject(ctx, educationsql.InstitutionStatsForSubjectParams{
			SubjectPersonID: subjectPersonID,
			KindID:          textPtr(f.KindID),
			CountryID:       textPtr(f.CountryID),
			FoundedOnFrom:   datePtr(f.FoundedOnFrom),
			FoundedOnTo:     datePtr(f.FoundedOnTo),
			State:           textPtr(f.State),
			TopN:            int32(sel.TopN()),
			WantKindID:      w.kindID,
			WantCountryID:   w.countryID,
			WantFoundedOn:   w.foundedOn,
			WantState:       w.state,
		})
		if err != nil {
			return nil, err
		}
		return institutionStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case q != "":
		rows, err := r.q.InstitutionStatsSearch(ctx, educationsql.InstitutionStatsSearchParams{
			Query:         pgtype.Text{String: q, Valid: true},
			KindID:        textPtr(f.KindID),
			CountryID:     textPtr(f.CountryID),
			FoundedOnFrom: datePtr(f.FoundedOnFrom),
			FoundedOnTo:   datePtr(f.FoundedOnTo),
			State:         textPtr(f.State),
			TopN:          int32(sel.TopN()),
			WantKindID:    w.kindID,
			WantCountryID: w.countryID,
			WantFoundedOn: w.foundedOn,
			WantState:     w.state,
		})
		if err != nil {
			return nil, err
		}
		return institutionStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	default:
		rows, err := r.q.InstitutionStats(ctx, educationsql.InstitutionStatsParams{
			KindID:        textPtr(f.KindID),
			CountryID:     textPtr(f.CountryID),
			FoundedOnFrom: datePtr(f.FoundedOnFrom),
			FoundedOnTo:   datePtr(f.FoundedOnTo),
			State:         textPtr(f.State),
			TopN:          int32(sel.TopN()),
			WantKindID:    w.kindID,
			WantCountryID: w.countryID,
			WantFoundedOn: w.foundedOn,
			WantState:     w.state,
		})
		if err != nil {
			return nil, err
		}
		return institutionStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	}
}

// institutionStatsWants projects a selection onto the per-branch flags. An unselected facet's branch
// is a one-time false filter the planner skips, so it is never grouped.
type institutionStatsWantFlags struct {
	kindID, countryID, foundedOn, state bool
}

func institutionStatsWants(sel stats.Selection) institutionStatsWantFlags {
	return institutionStatsWantFlags{
		kindID:    sel.Wants("kindId"),
		countryID: sel.Wants("countryId"),
		foundedOn: sel.Wants("foundedOn"),
		state:     sel.Wants("state"),
	}
}

// institutionStatsGroups maps the raw aggregate rows; a NULL bucket stays NULL (the (unknown)
// bucket). The four arms return four generated row types with identical fields, so the accessor is
// passed in rather than the slice — one mapping, four callers.
func institutionStatsGroups(n int, at func(int) (string, pgtype.Text, int64)) []stats.Group {
	out := make([]stats.Group, 0, n)
	for i := 0; i < n; i++ {
		facetKey, bucket, count := at(i)
		g := stats.Group{Facet: facetKey, Count: count}
		if bucket.Valid {
			k := bucket.String
			g.Key = &k
		}
		out = append(out, g)
	}
	return out
}
