// Package application holds the language module's application service — the read-only orchestrator the
// transport layer calls to browse the Glottolog languoid forest + ISO-15924 writing systems, and that
// other modules could call in-process to resolve a language. It depends on the domain port and the
// platform DB surface; it never imports the adapters package directly (the repository factory is
// injected by module.go).
package application

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/language/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// defaultLimit / maxLimit bound a languoid listing (the catalog is ~26k rows; callers narrow via the
// level/family/query filters).
const (
	defaultLimit = 100
	maxLimit     = 1000
)

// RepositoryFactory binds a domain.Repository to a command surface (the pool for reads). Injected by
// module.go so the application layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the language application service. Reads run on the pool directly; it owns no writes.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
}

// NewService wires the service with the pool and the repository factory.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory) *Service {
	return &Service{pool: pool, newRepo: newRepo}
}

// ListLanguoids returns matching languoids in code order, clamping the limit to [1, maxLimit].
func (s *Service) ListLanguoids(ctx context.Context, f domain.Filter) ([]domain.Languoid, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	return s.newRepo(s.pool).ListLanguoids(ctx, f)
}

// GetLanguoid returns one languoid by RID (found=false when absent).
func (s *Service) GetLanguoid(ctx context.Context, id string) (domain.Languoid, bool, error) {
	return s.newRepo(s.pool).GetLanguoid(ctx, id)
}

// ListWritingSystems returns the ISO-15924 writing systems in code order.
func (s *Service) ListWritingSystems(ctx context.Context) ([]domain.WritingSystem, error) {
	return s.newRepo(s.pool).ListWritingSystems(ctx)
}
