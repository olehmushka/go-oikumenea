// Package application holds the geo module's application service — the read-only orchestrator the
// transport layer calls to list the country registry, and that other modules could call in-process
// to resolve a country. It depends on the domain port and the platform DB surface; it never imports
// the adapters package directly (the repository factory is injected by module.go).
package application

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	"github.com/olegamysk/go-oikumenea/internal/geo/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a caller's
// transaction for an audited Location write (D-Audit). Injected by module.go so the application layer
// never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the geo/location application service. Country reads run on the pool directly; the Location
// CRUD owns its writes, recording an audit row in the same transaction as each change (D-Audit).
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
}

// NewService wires the service with the pool, the repository factory, and the audit service every
// Location write records into.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// ListCountries returns the active countries in display order.
func (s *Service) ListCountries(ctx context.Context) ([]domain.Country, error) {
	return s.newRepo(s.pool).ListCountries(ctx)
}

// ListPlaces returns active geo_places of a placetype (default region) under a country, in name order
// (D-GeoPlaces) — powers region pickers such as a vehicle plate region.
func (s *Service) ListPlaces(ctx context.Context, countryID, placetype string) ([]domain.Place, error) {
	if placetype == "" {
		placetype = "region"
	}
	return s.newRepo(s.pool).ListPlaces(ctx, countryID, placetype)
}

// ResolveCoordinate reverse-geocodes a WGS84 coordinate to the containing country plus the nearest
// gazetteer place (locality, else county/region) — powers the locations-form prefill (D-GeoPlaces).
func (s *Service) ResolveCoordinate(ctx context.Context, lat, lng float64) (domain.CoordinateResolution, error) {
	return s.newRepo(s.pool).ResolveCoordinate(ctx, lat, lng)
}
