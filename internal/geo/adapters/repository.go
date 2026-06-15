// Package adapters implements the geo domain ports against infrastructure: the pgx/sqlc repository
// over oikumenea.geo_countries. It depends on the database, never the reverse (overview.md).
// Generated sqlc code lives in the geosql subpackage and is never hand-edited.
package adapters

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/geo/adapters/geosql"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX
// (the pool for reads).
type Repository struct {
	q *geosql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: geosql.New(conn)}
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
