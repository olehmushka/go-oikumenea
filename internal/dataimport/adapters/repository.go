// Package adapters implements the data-import domain ports against infrastructure: the pgx/sqlc
// upsert over oikumenea.geo_countries (M16 first catalog). It depends on the database, never the
// reverse. Generated sqlc code lives in the dataimportsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// GeoCountryRepo is the pgx/sqlc-backed implementation of domain.GeoCountryStore, bound to a single
// db.DBTX — the pool for reads, or a caller-supplied transaction so the upsert and its audit row
// commit together (D-Audit).
type GeoCountryRepo struct {
	q *dataimportsql.Queries
}

// NewGeoCountryRepo binds a store to the given command surface. A db.DBTX satisfies the interface sqlc
// generates, so the pool and a pgx.Tx are both accepted.
func NewGeoCountryRepo(conn db.DBTX) *GeoCountryRepo {
	return &GeoCountryRepo{q: dataimportsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.GeoCountryStore = (*GeoCountryRepo)(nil)

// GetName returns the country's current name and whether the row exists.
func (r *GeoCountryRepo) GetName(ctx context.Context, code string) (string, bool, error) {
	name, err := r.q.GetGeoCountryName(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

// Insert adds a country row with import provenance.
func (r *GeoCountryRepo) Insert(ctx context.Context, code, name string, prov domain.Provenance) error {
	return r.q.InsertGeoCountryImport(ctx, dataimportsql.InsertGeoCountryImportParams{
		Code:          code,
		Name:          name,
		Source:        text(prov.Source),
		SourceVersion: text(prov.SourceVersion),
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport updates a country's name + provenance (called only when the name changed).
func (r *GeoCountryRepo) UpdateImport(ctx context.Context, code, name string, prov domain.Provenance) error {
	return r.q.UpdateGeoCountryImport(ctx, dataimportsql.UpdateGeoCountryImportParams{
		Code:          code,
		Name:          name,
		Source:        text(prov.Source),
		SourceVersion: text(prov.SourceVersion),
		ImportedAt:    ts(prov.ImportedAt),
	})
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()} }

// GeoPlaceRepo is the pgx/sqlc-backed implementation of domain.GeoPlaceStore (D-GeoPlaces), bound to a
// single db.DBTX (the caller's transaction so the upsert + audit row commit together — D-Audit).
type GeoPlaceRepo struct {
	q *dataimportsql.Queries
}

// NewGeoPlaceRepo binds a geo-places store to the given command surface.
func NewGeoPlaceRepo(conn db.DBTX) *GeoPlaceRepo {
	return &GeoPlaceRepo{q: dataimportsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.GeoPlaceStore = (*GeoPlaceRepo)(nil)

// GetVersion returns the place's stored source_version (the idempotency key) and whether the row
// exists. A NULL stored version reads back as "" (always treated as stale, so it re-imports).
func (r *GeoPlaceRepo) GetVersion(ctx context.Context, wofID int64) (string, bool, error) {
	v, err := r.q.GetGeoPlaceVersion(ctx, wofID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, true, nil
}

// Insert adds a gazetteer row; geometry is materialized from GeoJSON, provenance stamped.
func (r *GeoPlaceRepo) Insert(ctx context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	return r.q.InsertGeoPlaceImport(ctx, dataimportsql.InsertGeoPlaceImportParams{
		WofID:         p.WofID,
		Placetype:     p.Placetype,
		ParentID:      deref(p.ParentID),
		CountryCode:   p.CountryCode,
		Name:          p.Name,
		Population:    deref(p.Population),
		Hierarchy:     p.Hierarchy,
		Concordances:  p.Concordances,
		Status:        p.Status,
		Geometry:      p.GeometryJSON,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport rewrites a gazetteer row (called when the source edition changed).
func (r *GeoPlaceRepo) UpdateImport(ctx context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	return r.q.UpdateGeoPlaceImport(ctx, dataimportsql.UpdateGeoPlaceImportParams{
		WofID:         p.WofID,
		Placetype:     p.Placetype,
		ParentID:      deref(p.ParentID),
		CountryCode:   p.CountryCode,
		Name:          p.Name,
		Population:    deref(p.Population),
		Hierarchy:     p.Hierarchy,
		Concordances:  p.Concordances,
		Status:        p.Status,
		Geometry:      p.GeometryJSON,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// EnrichCountry mirrors a country place's wof_id + geometry onto its geo_countries row (D-GeoPlaces).
func (r *GeoPlaceRepo) EnrichCountry(ctx context.Context, p domain.GeoPlace, _ domain.Provenance) error {
	return r.q.EnrichGeoCountryFromWOF(ctx, dataimportsql.EnrichGeoCountryFromWOFParams{
		Code:        p.CountryCode,
		WofID:       p.WofID,
		Geometry:    p.GeometryJSON,
		IsoA3:       p.ISOA3,
		NumericCode: p.NumericCode,
	})
}

// deref returns the pointed-to int64 or 0 (the absent sentinel the queries fold to NULL via NULLIF).
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
