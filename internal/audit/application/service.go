// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/olegamysk/go-oikumenea/pkg/action"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
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
	// labeler resolves a dashboard's ref-bucket RIDs to locale→text names (M58 / D-ObjectFacets).
	// Optional: unset, a chart segment carries its RID and the client falls back to the RID tail.
	labeler stats.Labeler
}

// SetBucketLabeler binds the optional dashboard label resolver, wired at the composition root from
// the same per-type labelers the links service uses.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// NewService wires the service with the read pool, the repository factory, and a refreshable
// default page size.
func NewService(pool db.DBTX, newRepo RepositoryFactory, defaultSize func() int) *Service {
	return &Service{pool: pool, newRepo: newRepo, defaultSize: defaultSize}
}

// reader returns the request's RLS-scoped command surface: an explicitly pinned connection, else
// the request's lazily-pinned connection (db.WithLazyConn — acquired on first use, R-03), else the
// bare pool. audit_log carries a SELECT-only RLS policy keyed on unit_id (the read backstop —
// D-RLSDefenseInDepth), so reads must run on the GUC-bearing connection; inserts are unrestricted
// (Record uses the caller's transaction, which for request-driven writes is already the pinned conn).
func (s *Service) reader(ctx context.Context) db.DBTX {
	return db.RequestDBTX(ctx, s.pool)
}

// Record persists one audit entry on the caller-supplied command surface — typically the open
// transaction of the mutation being audited, so the audit row commits iff the change commits
// (D-Audit). It is the in-process entry point every write-bearing module calls; there is no HTTP
// write endpoint.
func (s *Service) Record(ctx context.Context, conn db.DBTX, e domain.Entry) error {
	if err := e.Validate(); err != nil {
		return err
	}
	// Write-time action-type gate (D-ActionTypes, review-2026-09 R-29): the action must be in the
	// pkg/action catalog. Kept out of domain.Validate so the domain stays stdlib-only.
	if err := action.Validate(e.Action); err != nil {
		return err
	}
	return s.newRepo(conn).Insert(ctx, e)
}

// Get reads one entry by its Action RID, returning domain.ErrNotFound when absent.
// Stats answers the ledger dashboard (M58 / D-ObjectFacets): the selected facets' distributions plus
// the total, over exactly the set Query would page under the same filter. It shares QueryParams with
// the list on purpose — the two views must describe one world, and a second params struct is a second
// thing that can drift — and simply ignores the paging fields, since a page boundary is not a filter.
//
// It is the ONE stats path that does not route through stats.Compute, and the reason is that Compute
// exists to own the arm convention ("an empty subject is the admin arm, and a non-admin with no
// subject reads nothing" — M57 ticket 2). There is no arm here: the aggregate carries no subject
// parameter at all, because audit visibility is the RLS policy on audit_log. The whole of the
// visibility decision is which CONNECTION this runs on, and reader() is that connection — on the bare
// pool the policy matches nothing and this answers a confident zero.
func (s *Service) Stats(ctx context.Context, p QueryParams, sel stats.Selection) (stats.Result, error) {
	groups, err := s.newRepo(s.reader(ctx)).Stats(ctx, domain.Filter{
		ActorPersonID: p.ActorPersonID,
		ActorType:     p.ActorType,
		TargetType:    p.TargetType,
		TargetID:      p.TargetID,
		UnitID:        p.UnitID,
		Action:        p.Action,
		Outcome:       p.Outcome,
		Since:         p.Since,
		Until:         p.Until,
	}, sel)
	if err != nil {
		return stats.Result{}, err
	}
	res := stats.Assemble(sel, groups)
	// Label the two ref facets (actorPersonId → person, unitId → unit) through the same resolvers the
	// links service uses, so an object reads identically in a graph row and in a chart segment. The
	// `action`/`targetType` code facets carry no labels by construction: the key IS the label.
	if err := stats.Label(ctx, s.labeler, sel, &res); err != nil {
		return stats.Result{}, err
	}
	return res, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Entry, error) {
	return s.newRepo(s.reader(ctx)).Get(ctx, id)
}

// EnsureCurrentPartitions rolls the monthly range-partition window forward (review-2026-07 R-07):
// idempotently create the current + next month's partition. Called at boot (advisory-locked in the
// composition root) so live inserts always land in a real monthly partition. Runs on the pool.
func (s *Service) EnsureCurrentPartitions(ctx context.Context) error {
	return s.newRepo(s.pool).EnsureCurrentPartitions(ctx)
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

// resolvePageSize clamps through the shared policy (M56 / pkg/listing). Audit is the one module whose
// Default comes from RUNTIME config (`audit.default-page-size`) rather than a constant, so the policy
// is built per call from the refreshable value; the fallback guards a configured non-positive value.
func (s *Service) resolvePageSize(requested int) int {
	def := s.defaultSize()
	if def <= 0 {
		def = 50
	}
	return listing.PageSize{Default: def, Max: MaxPageSize}.Resolve(requested)
}
