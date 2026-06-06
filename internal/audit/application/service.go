// Package application holds the audit module's application service — the flat orchestrator other
// modules call to record writes (in their transaction) and the transport layer calls to read the
// log (overview.md). It depends on the domain port and the platform DB surface, never on the
// adapters package directly: the repository factory is injected at wiring time (module.go).
package application

import (
	"context"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// MaxPageSize caps a client-requested page size (API conventions: token pagination, bounded pages).
const MaxPageSize = 500

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an in-transaction Record (D-Audit). Injected by module.go so the
// application layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the audit application service.
type Service struct {
	pool        db.DBTX
	newRepo     RepositoryFactory
	defaultSize func() int
}

// NewService wires the service with the read pool, the repository factory, and a refreshable
// default page size.
func NewService(pool db.DBTX, newRepo RepositoryFactory, defaultSize func() int) *Service {
	return &Service{pool: pool, newRepo: newRepo, defaultSize: defaultSize}
}

// reader returns the request-pinned RLS connection if one is in context (db.AcquireScoped/WithConn),
// else the bare pool. audit_log carries a SELECT-only RLS policy keyed on unit_id (the read backstop —
// D-RLSDefenseInDepth), so reads must run on the GUC-bearing connection; inserts are unrestricted
// (Record uses the caller's transaction, which for request-driven writes is already the pinned conn).
func (s *Service) reader(ctx context.Context) db.DBTX {
	if c, ok := db.ConnFromContext(ctx); ok {
		return c
	}
	return s.pool
}

// Record persists one audit entry on the caller-supplied command surface — typically the open
// transaction of the mutation being audited, so the audit row commits iff the change commits
// (D-Audit). It is the in-process entry point every write-bearing module calls; there is no HTTP
// write endpoint.
func (s *Service) Record(ctx context.Context, conn db.DBTX, e domain.Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}
	return s.newRepo(conn).Insert(ctx, e)
}

// Get reads one entry by its Action RID, returning domain.ErrNotFound when absent.
func (s *Service) Get(ctx context.Context, id string) (domain.Entry, error) {
	return s.newRepo(s.reader(ctx)).Get(ctx, id)
}

// QueryParams are the read filters plus pagination request. Pointer fields are optional (nil
// matches everything). PageSize <= 0 means the configured default; PageToken is opaque.
type QueryParams struct {
	ActorPersonID *string
	ActorType     *domain.ActorType
	TargetType    *string
	TargetID      *string
	UnitID        *string
	Action        *string
	Outcome       *domain.Outcome
	Since         *time.Time
	Until         *time.Time
	PageSize      int
	PageToken     string
}

// Page is a page of entries plus the opaque token for the next page (empty when exhausted).
type Page struct {
	Entries       []domain.Entry
	NextPageToken string
}

// Query runs a filtered, token-paginated read. It fetches one extra row to decide whether a
// further page exists, then mints the next-page cursor from the last returned entry.
func (s *Service) Query(ctx context.Context, p QueryParams) (Page, error) {
	size := s.resolvePageSize(p.PageSize)

	cursor, err := decodeToken(p.PageToken)
	if err != nil {
		return Page{}, err
	}

	entries, err := s.newRepo(s.reader(ctx)).Query(ctx, domain.Filter{
		ActorPersonID: p.ActorPersonID,
		ActorType:     p.ActorType,
		TargetType:    p.TargetType,
		TargetID:      p.TargetID,
		UnitID:        p.UnitID,
		Action:        p.Action,
		Outcome:       p.Outcome,
		Since:         p.Since,
		Until:         p.Until,
		Cursor:        cursor,
		Limit:         size + 1, // +1 sentinel row to detect a further page
	})
	if err != nil {
		return Page{}, err
	}

	if len(entries) > size {
		last := entries[size-1]
		return Page{
			Entries:       entries[:size],
			NextPageToken: encodeToken(domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}),
		}, nil
	}
	return Page{Entries: entries}, nil
}

func (s *Service) resolvePageSize(requested int) int {
	if requested <= 0 {
		requested = s.defaultSize()
	}
	if requested <= 0 {
		requested = 50
	}
	if requested > MaxPageSize {
		requested = MaxPageSize
	}
	return requested
}
