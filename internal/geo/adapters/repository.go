// Package adapters implements the geo domain ports against infrastructure: the pgx/sqlc repository
// over oikumenea.geo_countries. It depends on the database, never the reverse (overview.md).
// Generated sqlc code lives in the geosql subpackage and is never hand-edited.
package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/geo/adapters/geosql"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX
// (the pool for reads). The raw command surface `c` backs hand-written queries (geo_places) that sit
// outside the sqlc set.
type Repository struct {
	q *geosql.Queries
	c db.DBTX
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: geosql.New(conn), c: conn}
}

// ListPlaces returns active geo_places of a placetype under a country, in name order (D-GeoPlaces).
// Hand-written (raw pgx) so it can join the WOF gazetteer without an sqlc query.
func (r *Repository) ListPlaces(ctx context.Context, countryID, placetype string) ([]domain.Place, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, placetype, name, country_id, status
		FROM oikumenea.geo_places
		WHERE country_id = $1 AND placetype = $2 AND status = 'active'
		ORDER BY name`, countryID, placetype)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Place, 0)
	for rows.Next() {
		var p domain.Place
		var country pgtype.Text
		if err := rows.Scan(&p.ID, &p.Placetype, &p.Name, &country, &p.Status); err != nil {
			return nil, err
		}
		if country.Valid {
			p.CountryID = country.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

func (r *Repository) ListCountries(ctx context.Context) ([]domain.Country, error) {
	rows, err := r.q.ListCountries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Country, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Country{
			ID:     row.ID,
			Code:   row.Code,
			Name:   row.Name,
			Status: row.Status,
		})
	}
	return out, nil
}
