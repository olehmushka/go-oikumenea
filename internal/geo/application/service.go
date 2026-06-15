// Package application holds the geo module's application service — the read-only orchestrator the
// transport layer calls to list the country registry, and that other modules could call in-process
// to resolve a country. It depends on the domain port and the platform DB surface; it never imports
// the adapters package directly (the repository factory is injected by module.go).
package application

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// RepositoryFactory binds a domain.Repository to a command surface (the pool for reads). Injected by
// module.go so the application layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the geo application service. Reads run on the pool directly; it owns no writes.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
}

// NewService wires the service with the pool and the repository factory.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory) *Service {
	return &Service{pool: pool, newRepo: newRepo}
}

// ListCountries returns the active countries in display order.
func (s *Service) ListCountries(ctx context.Context) ([]domain.Country, error) {
	return s.newRepo(s.pool).ListCountries(ctx)
}
