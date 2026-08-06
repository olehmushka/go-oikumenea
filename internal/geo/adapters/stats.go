// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/geo/adapters/geosql"
	"github.com/olehmushka/go-oikumenea/internal/geo/domain"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// The location dashboard aggregate (M58 ticket 6 / D-ObjectFacets): every selected facet's
// distribution plus the total, over the same candidate set ListLocations pages under the same filter.
//
// FOUR arms, and the axis is the listing MODE rather than visibility. A location carries no owner and
// no public/shadow bit, so there is no second visibility arm for a decision to be made in — that is
// languoid's shape, the absence of a decision. What is four-way is the window, because each mode is a
// different plan and a nullable spatial predicate is not index-served; the switch below mirrors
// ListLocations' exactly, over the mode the TRANSPORT resolved, so neither surface re-derives it.

// LocationStats answers the whole dashboard in one statement.
func (r *Repository) LocationStats(ctx context.Context, f domain.LocationFilter, sel stats.Selection) ([]stats.Group, error) {
	w := locationStatsWants(sel)
	switch f.Mode {
	case domain.LocationModeText:
		rows, err := r.q.LocationStatsSearch(ctx, geosql.LocationStatsSearchParams{
			Query:         f.Query,
			CountryID:     textPtr(f.CountryID),
			TypeID:        textPtr(f.TypeID),
			TopN:          int32(sel.TopN()),
			WantCountryID: w.countryID,
			WantTypeID:    w.typeID,
		})
		if err != nil {
			return nil, err
		}
		return locationStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case domain.LocationModeRadius:
		rows, err := r.q.LocationStatsNear(ctx, geosql.LocationStatsNearParams{
			Lng:           f.Lng,
			Lat:           f.Lat,
			RadiusM:       f.RadiusM,
			CountryID:     textPtr(f.CountryID),
			TypeID:        textPtr(f.TypeID),
			TopN:          int32(sel.TopN()),
			WantCountryID: w.countryID,
			WantTypeID:    w.typeID,
		})
		if err != nil {
			return nil, err
		}
		return locationStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	case domain.LocationModeBbox:
		rows, err := r.q.LocationStatsInBbox(ctx, geosql.LocationStatsInBboxParams{
			MinLng:        f.MinLng,
			MinLat:        f.MinLat,
			MaxLng:        f.MaxLng,
			MaxLat:        f.MaxLat,
			CountryID:     textPtr(f.CountryID),
			TypeID:        textPtr(f.TypeID),
			TopN:          int32(sel.TopN()),
			WantCountryID: w.countryID,
			WantTypeID:    w.typeID,
		})
		if err != nil {
			return nil, err
		}
		return locationStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	default:
		rows, err := r.q.LocationStats(ctx, geosql.LocationStatsParams{
			CountryID:     textPtr(f.CountryID),
			TypeID:        textPtr(f.TypeID),
			TopN:          int32(sel.TopN()),
			WantCountryID: w.countryID,
			WantTypeID:    w.typeID,
		})
		if err != nil {
			return nil, err
		}
		return locationStatsGroups(len(rows), func(i int) (string, pgtype.Text, int64) {
			return rows[i].Facet, rows[i].Bucket, rows[i].N
		}), nil
	}
}

// locationStatsWants projects a selection onto the per-branch flags. An unselected facet's branch is a
// one-time false filter the planner skips, so it is never grouped.
type locationStatsWantFlags struct {
	countryID, typeID bool
}

func locationStatsWants(sel stats.Selection) locationStatsWantFlags {
	return locationStatsWantFlags{
		countryID: sel.Wants("countryId"),
		typeID:    sel.Wants("typeId"),
	}
}

// locationStatsGroups maps the raw aggregate rows; a NULL bucket stays NULL (the (unknown) bucket).
// The four arms return four generated row types with identical fields, so the accessor is passed in
// rather than the slice — one mapping, four callers, the same reason the aggregate SQL is one block.
func locationStatsGroups(n int, at func(int) (string, pgtype.Text, int64)) []stats.Group {
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
