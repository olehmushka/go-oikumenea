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
