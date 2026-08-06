// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// ScanStatsGroups reads the four-column shape every facet aggregate returns — facet key, bucket key
// (NULL for the total row and for a NULL group), count, and an optional SQL-supplied ordinal — into
// the kernel's []stats.Group (M57 / D-ObjectFacets).
//
// It lives here rather than in pkg/stats because it is the pgx-shaped half: pkg/stats is a pure
// assembly kernel over []Group and knows nothing about a driver. The sqlc-backed modules get this
// scan generated for them; the RAW-PGX modules (religion, externalorg, and the vehicle/finance types
// still to come) write their aggregate by hand and would otherwise each carry a verbatim copy of the
// same twelve lines — including the one detail that is easy to get subtly wrong, which is that a NULL
// bucket must stay a nil Key so the kernel can decide whether it becomes an (unknown) bucket or a
// synthetic one, rather than being flattened to "" here.
func ScanStatsGroups(rows pgx.Rows) ([]stats.Group, error) {
	defer rows.Close()
	var out []stats.Group
	for rows.Next() {
		var facetKey string
		var bucket pgtype.Text
		var n int64
		var ord pgtype.Int8
		if err := rows.Scan(&facetKey, &bucket, &n, &ord); err != nil {
			return nil, err
		}
		g := stats.Group{Facet: facetKey, Count: n}
		if bucket.Valid {
			k := bucket.String
			g.Key = &k
		}
		if ord.Valid {
			o := ord.Int64
			g.Ord = &o
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
