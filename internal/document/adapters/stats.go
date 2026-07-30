// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/document/adapters/documentsql"
	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// DocumentStats is the document-register dashboard aggregate (M57 / D-ObjectFacets): every selected
// facet's distribution plus the total, over the same candidate set ListDocuments pages under the same
// filters.
//
// An empty subjectPersonID is the INSTANCE-ADMIN arm. Otherwise reach goes THROUGH THE HOLDER — a
// document carries no unit — using the same active-membership semi-join the list arm uses, folded into
// the candidate CTE so an unreadable holder's documents are absent from the count rather than counted
// and then trimmed.
//
// One scoped shape, no sparse/dense dispatch: this is the table whose LIST could not use the
// materialized reach set at root reach, and re-measuring the aggregate both ways showed the set form
// still wins (see the query's comment for the numbers).
func (r *Repository) DocumentStats(ctx context.Context, subjectPersonID string, f domain.DocumentFilter, sel stats.Selection) ([]stats.Group, error) {
	fa := documentFacetArgs(f)
	w := documentStatsWants(sel)
	if subjectPersonID != "" {
		rows, err := r.q.DocumentStatsForSubject(ctx, documentsql.DocumentStatsForSubjectParams{
			SubjectPersonID:      subjectPersonID,
			TypeID:               fa.typeID,
			Status:               fa.status,
			IssuingCountryID:     fa.issuingCountryID,
			IssuedOnFrom:         fa.issuedOnFrom,
			IssuedOnTo:           fa.issuedOnTo,
			ExpiresOnFrom:        fa.expiresOnFrom,
			ExpiresOnTo:          fa.expiresOnTo,
			WantTypeID:           w.typeID,
			WantStatus:           w.status,
			WantIssuingCountryID: w.issuingCountryID,
			WantIssuedOn:         w.issuedOn,
			WantExpiresOn:        w.expiresOn,
			TopN:                 w.topN,
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
	rows, err := r.q.DocumentStats(ctx, documentsql.DocumentStatsParams{
		TypeID:               fa.typeID,
		Status:               fa.status,
		IssuingCountryID:     fa.issuingCountryID,
		IssuedOnFrom:         fa.issuedOnFrom,
		IssuedOnTo:           fa.issuedOnTo,
		ExpiresOnFrom:        fa.expiresOnFrom,
		ExpiresOnTo:          fa.expiresOnTo,
		WantTypeID:           w.typeID,
		WantStatus:           w.status,
		WantIssuingCountryID: w.issuingCountryID,
		WantIssuedOn:         w.issuedOn,
		WantExpiresOn:        w.expiresOn,
		TopN:                 w.topN,
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

// documentStatsWantFlags is one selection projected onto the per-branch flags the query binds; an
// unselected facet's branch is skipped by the planner rather than merely hidden from the response.
type documentStatsWantFlags struct {
	typeID, status, issuingCountryID, issuedOn, expiresOn bool
	topN                                                  int32
}

func documentStatsWants(sel stats.Selection) documentStatsWantFlags {
	return documentStatsWantFlags{
		typeID:           sel.Wants("typeId"),
		status:           sel.Wants("status"),
		issuingCountryID: sel.Wants("issuingCountryId"),
		issuedOn:         sel.Wants("issuedOn"),
		expiresOn:        sel.Wants("expiresOn"),
		topN:             int32(sel.TopN()),
	}
}

// statsGroup maps one raw aggregate row to the kernel's Group. A NULL bucket stays NULL: for expiresOn
// it is the no-expiry population, which is a real set rather than missing data.
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
