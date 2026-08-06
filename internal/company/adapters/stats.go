// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/company/adapters/companysql"
	"github.com/olehmushka/go-oikumenea/internal/company/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// The company dashboard aggregate (M58 ticket 5 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListCompanies pages under the same
// filters.
//
// FOUR arms, which is the person shape rather than the organization one. A company has BOTH a
// visibility gate (it IS a `company`-domain tenant organization — M41 / D-UnifiedOrgGraph) and an
// R-21 search twin, so the square is {plain, search} × {instance-admin, visibility-scoped}. The two
// axes are chosen independently and by exactly the conditions the list uses, so a searched, scoped
// dashboard and the list beside it describe one set.

// textPtr maps an optional filter value onto a sqlc narg: nil is SQL NULL, which each predicate reads
// as "criterion disabled".
func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// companyQuery is the ONE place the search/plain branch condition is written, so ListCompanies and
// CompanyStats cannot drift into picking different plan shapes for the same input.
func companyQuery(f domain.CompanyFilter) string { return strings.TrimSpace(f.Query) }

// CompanyStats answers the whole dashboard in one statement.
//
// subjectPersonID empty means the INSTANCE-ADMIN arm, which carries no visibility predicate at all.
// Otherwise the organization shadow gate is folded into the candidate CTE — right for a count, where
// trimming the page after the fact (what gateCompanies does on the list) would undercount silently.
func (r *Repository) CompanyStats(ctx context.Context, subjectPersonID string, f domain.CompanyFilter, sel stats.Selection) ([]stats.Group, error) {
	w := companyStatsWants(sel)
	q := companyQuery(f)
	switch {
	case subjectPersonID != "" && q != "":
		rows, err := r.q.CompanyStatsForSubjectSearch(ctx, companysql.CompanyStatsForSubjectSearchParams{
			SubjectPersonID:       subjectPersonID,
			Query:                 pgtype.Text{String: q, Valid: true},
			LegalFormID:           textPtr(f.LegalFormID),
			OwnershipCategory:     textPtr(f.OwnershipCategory),
			CountryID:             textPtr(f.CountryID),
			IndustryClassID:       textPtr(f.IndustryClassID),
			FoundedOnFrom:         datePtr(f.FoundedOnFrom),
			FoundedOnTo:           datePtr(f.FoundedOnTo),
			State:                 textPtr(f.State),
			TopN:                  int32(sel.TopN()),
			WantLegalForm:         w.legalForm,
			WantOwnershipCategory: w.ownershipCategory,
			WantCountryID:         w.countryID,
			WantIndustryClass:     w.industryClass,
			WantFoundedOn:         w.foundedOn,
			WantState:             w.state,
		})
		if err != nil {
			return nil, err
		}
		return companyStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case subjectPersonID != "":
		rows, err := r.q.CompanyStatsForSubject(ctx, companysql.CompanyStatsForSubjectParams{
			SubjectPersonID:       subjectPersonID,
			LegalFormID:           textPtr(f.LegalFormID),
			OwnershipCategory:     textPtr(f.OwnershipCategory),
			CountryID:             textPtr(f.CountryID),
			IndustryClassID:       textPtr(f.IndustryClassID),
			FoundedOnFrom:         datePtr(f.FoundedOnFrom),
			FoundedOnTo:           datePtr(f.FoundedOnTo),
			State:                 textPtr(f.State),
			TopN:                  int32(sel.TopN()),
			WantLegalForm:         w.legalForm,
			WantOwnershipCategory: w.ownershipCategory,
			WantCountryID:         w.countryID,
			WantIndustryClass:     w.industryClass,
			WantFoundedOn:         w.foundedOn,
			WantState:             w.state,
		})
		if err != nil {
			return nil, err
		}
		return companyStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case q != "":
		rows, err := r.q.CompanyStatsSearch(ctx, companysql.CompanyStatsSearchParams{
			Query:                 pgtype.Text{String: q, Valid: true},
			LegalFormID:           textPtr(f.LegalFormID),
			OwnershipCategory:     textPtr(f.OwnershipCategory),
			CountryID:             textPtr(f.CountryID),
			IndustryClassID:       textPtr(f.IndustryClassID),
			FoundedOnFrom:         datePtr(f.FoundedOnFrom),
			FoundedOnTo:           datePtr(f.FoundedOnTo),
			State:                 textPtr(f.State),
			TopN:                  int32(sel.TopN()),
			WantLegalForm:         w.legalForm,
			WantOwnershipCategory: w.ownershipCategory,
			WantCountryID:         w.countryID,
			WantIndustryClass:     w.industryClass,
			WantFoundedOn:         w.foundedOn,
			WantState:             w.state,
		})
		if err != nil {
			return nil, err
		}
		return companyStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	default:
		rows, err := r.q.CompanyStats(ctx, companysql.CompanyStatsParams{
			LegalFormID:           textPtr(f.LegalFormID),
			OwnershipCategory:     textPtr(f.OwnershipCategory),
			CountryID:             textPtr(f.CountryID),
			IndustryClassID:       textPtr(f.IndustryClassID),
			FoundedOnFrom:         datePtr(f.FoundedOnFrom),
			FoundedOnTo:           datePtr(f.FoundedOnTo),
			State:                 textPtr(f.State),
			TopN:                  int32(sel.TopN()),
			WantLegalForm:         w.legalForm,
			WantOwnershipCategory: w.ownershipCategory,
			WantCountryID:         w.countryID,
			WantIndustryClass:     w.industryClass,
			WantFoundedOn:         w.foundedOn,
			WantState:             w.state,
		})
		if err != nil {
			return nil, err
		}
		return companyStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	}
}

// companyStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is a
// one-time false filter the planner skips, so it is never grouped.
type companyStatsWantFlags struct {
	legalForm, ownershipCategory, countryID, industryClass, foundedOn, state bool
}

func companyStatsWants(sel stats.Selection) companyStatsWantFlags {
	return companyStatsWantFlags{
		legalForm:         sel.Wants("legalForm"),
		ownershipCategory: sel.Wants("ownershipCategory"),
		countryID:         sel.Wants("countryId"),
		industryClass:     sel.Wants("industryClass"),
		foundedOn:         sel.Wants("foundedOn"),
		state:             sel.Wants("state"),
	}
}

// companyStatsGroups maps the raw aggregate rows; a NULL bucket stays NULL (the (unknown) bucket). The
// four arms return four generated row types with identical fields, so the accessor is passed in rather
// than the slice — one mapping, four callers, which is the same reason the aggregate SQL is one block.
func companyStatsGroups(n int, at func(int) (string, pgtype.Text, int64)) []stats.Group {
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
