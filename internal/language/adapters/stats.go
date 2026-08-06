// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/language/adapters/languagesql"
	"github.com/olehmushka/go-oikumenea/internal/language/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// textPtr maps an optional filter value onto a sqlc narg: nil is SQL NULL, which each predicate reads
// as "criterion disabled". The facet filters are nargs rather than the empty-string sentinels this
// module used before M58 ticket 4 — a sentinel forces one generic plan across every filter shape, and
// it is invisible to the parity guard that proves the list and the dashboard share a filter block.
func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// strPtrOrNil folds the keyset cursor's empty-string "first page" into a nil so textPtr can turn it
// into the SQL NULL the predicate expects.
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// trimmedQuery is the ONE place the search/list branch condition is written, so ListLanguoids and
// LanguoidStats cannot drift into picking different plan shapes for the same input.
func trimmedQuery(f domain.Filter) string { return strings.TrimSpace(f.Query) }

// LanguoidStats is the languoid dashboard aggregate (M58 ticket 4 / D-ObjectFacets): every selected
// facet's distribution plus the total, over the same candidate set ListLanguoids pages under the same
// structural filters.
//
// It branches to the trigram twin on EXACTLY the condition ListLanguoids branches on, so a searched
// list and its dashboard cannot end up describing different sets. There is no subject and no second
// arm: the languoid registry is instance-global reference data with no visibility predicate to fold
// into the count.
func (r *Repository) LanguoidStats(ctx context.Context, f domain.Filter, sel stats.Selection) ([]stats.Group, error) {
	w := languoidStatsWants(sel)
	if q := trimmedQuery(f); q != "" {
		rows, err := r.q.LanguoidStatsSearch(ctx, languagesql.LanguoidStatsSearchParams{
			Level:         textPtr(f.Level),
			Family:        textPtr(f.Family),
			Macroarea:     textPtr(f.Macroarea),
			Status:        textPtr(f.Status),
			Q:             q,
			TopN:          int32(sel.TopN()),
			WantLevel:     w.level,
			WantStatus:    w.status,
			WantMacroarea: w.macroarea,
			WantFamily:    w.family,
		})
		if err != nil {
			return nil, err
		}
		out := make([]stats.Group, 0, len(rows))
		for _, row := range rows {
			out = append(out, languoidStatsGroup(row.Facet, row.Bucket, row.N))
		}
		return out, nil
	}
	rows, err := r.q.LanguoidStats(ctx, languagesql.LanguoidStatsParams{
		Level:         textPtr(f.Level),
		Family:        textPtr(f.Family),
		Macroarea:     textPtr(f.Macroarea),
		Status:        textPtr(f.Status),
		TopN:          int32(sel.TopN()),
		WantLevel:     w.level,
		WantStatus:    w.status,
		WantMacroarea: w.macroarea,
		WantFamily:    w.family,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, languoidStatsGroup(row.Facet, row.Bucket, row.N))
	}
	return out, nil
}

// languoidStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is a
// one-time false filter the planner skips, so it is never grouped.
type languoidStatsWantFlags struct {
	level, macroarea, status, family bool
}

func languoidStatsWants(sel stats.Selection) languoidStatsWantFlags {
	return languoidStatsWantFlags{
		level:     sel.Wants("level"),
		macroarea: sel.Wants("macroarea"),
		status:    sel.Wants("status"),
		family:    sel.Wants("family"),
	}
}

// languoidStatsGroup maps one raw aggregate row; a NULL bucket stays NULL (the (unknown) bucket). No
// ordinal: status's severity order and level's tree order come from the catalog's declared Values,
// which the kernel applies — SQL emits neither.
func languoidStatsGroup(facetKey string, bucket pgtype.Text, n int64) stats.Group {
	g := stats.Group{Facet: facetKey, Count: n}
	if bucket.Valid {
		k := bucket.String
		g.Key = &k
	}
	return g
}
