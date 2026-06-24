// Package adapters implements the geo domain ports against infrastructure: the pgx/sqlc repository
// over oikumenea.geo_countries. It depends on the database, never the reverse (overview.md).
// Generated sqlc code lives in the geosql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
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

// ResolveCoordinate reverse-geocodes a WGS84 coordinate (D-GeoPlaces). It finds the nearest gazetteer
// place — preferring a locality (city/town/village), falling back to county then region when no
// locality row exists — and the containing country (smallest polygon by area), falling back to the
// nearest place's country when no country shape covers the point. Either result may be nil if the
// gazetteer has no coverage. Hand-written (raw pgx) so it can run the PostGIS spatial queries directly.
func (r *Repository) ResolveCoordinate(ctx context.Context, lat, lng float64) (domain.CoordinateResolution, error) {
	var res domain.CoordinateResolution

	// Nearest place: try locality, then county, then region; the GiST index on centroid backs the KNN.
	for _, pt := range []string{"locality", "county", "region"} {
		var p domain.NearestPlace
		var country pgtype.Text
		err := r.c.QueryRow(ctx, `
			SELECT id, placetype, name, country_id,
			       ST_Distance(centroid::geography,
			                   ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography)::double precision AS distance_m
			FROM oikumenea.geo_places
			WHERE status = 'active' AND placetype = $3 AND centroid IS NOT NULL
			ORDER BY centroid <-> ST_SetSRID(ST_MakePoint($2, $1), 4326)
			LIMIT 1`, lat, lng, pt).Scan(&p.ID, &p.Placetype, &p.Name, &country, &p.DistanceMeters)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return res, err
		}
		if country.Valid {
			p.CountryID = country.String
		}
		res.Place = &p
		break
	}

	// Containing country: smallest polygon that covers the point.
	var c domain.Country
	err := r.c.QueryRow(ctx, `
		SELECT id, code, name, status
		FROM oikumenea.geo_countries
		WHERE geom IS NOT NULL AND ST_Contains(geom, ST_SetSRID(ST_MakePoint($2, $1), 4326))
		ORDER BY ST_Area(geom) ASC
		LIMIT 1`, lat, lng).Scan(&c.ID, &c.Code, &c.Name, &c.Status)
	switch {
	case err == nil:
		res.Country = &c
	case errors.Is(err, pgx.ErrNoRows):
		// No country shape covers the point; fall back to the nearest place's country, if any.
		if res.Place != nil && res.Place.CountryID != "" {
			var fc domain.Country
			ferr := r.c.QueryRow(ctx, `
				SELECT id, code, name, status FROM oikumenea.geo_countries WHERE id = $1`,
				res.Place.CountryID).Scan(&fc.ID, &fc.Code, &fc.Name, &fc.Status)
			if ferr == nil {
				res.Country = &fc
			} else if !errors.Is(ferr, pgx.ErrNoRows) {
				return res, ferr
			}
		}
	default:
		return res, err
	}

	return res, nil
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
