// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package application holds the membership module's application service — the orchestrator the
// transport layer calls to read/mutate positions (unit-owned billets) and memberships (people
// belonging to / filling them), recording an audit row in the same transaction as each write
// (D-Audit). It depends on the domain port, the platform DB surface, and the audit service; it never
// imports the adapters package directly (the repository factory is injected by module.go).
//
// Existence of referenced persons/units/ranks is validated by the DB foreign keys (mapped to domain
// sentinels in the adapter), so there are no pre-check lookups; the one cross-row invariant the
// application enforces is "a filling's position must belong to the membership's unit" and the
// one-holder / abolish-in-use guards. Position is a directory attribute and never an authz input.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/membership/domain"
	orderevents "github.com/olehmushka/go-oikumenea/internal/order/events"
	persondomain "github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/events"
	"github.com/olehmushka/go-oikumenea/pkg/listing"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// pageSize is this module's page-size policy (M56 / pkg/listing): the shared clamp bound to the
// module's own Default/Max, replacing the per-module resolvePageSize copy.
var pageSizePolicy = listing.PageSize{Default: DefaultPageSize, Max: MaxPageSize}

// auditSubsystem labels the interim system actor for membership's admin writes. Until authorization
// (M7) + identity-federation (M8) resolve the acting person, these writes are recorded as a `system`
// action under this subsystem (the no-unaudited-mutation ground rule still holds). M7/M8 replace
// this with the resolved person actor.
const auditSubsystem = "membership-admin"

// eventSubsystem labels the system actor for writes made by an ORDER-EVENT SUBSCRIBER (D-OrderApply):
// when membership effects are auto-applied inside an order's issue transaction, they audit as
// `system` / `event-subscriber`, correlated to the human's order.issue row by the shared request_id.
const eventSubsystem = "event-subscriber"

// Audit target types (the audited entity kinds).
const (
	targetPosition   = "position"
	targetMembership = "membership"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a
// caller's transaction for an audited write (D-Audit). Injected by module.go so the application
// layer never imports adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the membership application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly.
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service // labeler resolves a dashboard's ref-bucket RIDs to locale->text names (M57 / D-ObjectFacets).
	// Optional: unset, a chart segment carries its RID and the client falls back to the RID tail.
	labeler stats.Labeler
}

// SetBucketLabeler binds the optional dashboard label resolver (M57 / D-ObjectFacets), wired at the
// composition root from the same per-type labelers the links service uses — so an object reads
// identically in a graph row and in a chart segment.
func (s *Service) SetBucketLabeler(l stats.Labeler) { s.labeler = l }

// NewService wires the service with the pool, the repository factory, and the audit service every
// write records into.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit}
}

// PositionPage / MembershipPage are keyset-paginated slices plus the opaque next-page token.
type PositionPage struct {
	Positions     []domain.Position
	NextPageToken string
}
type MembershipPage struct {
	Memberships   []domain.Membership
	NextPageToken string
}

// ---------------------------------------------------------------- positions

// CreatePosition validates and creates a billet in a unit (vacant), then records the action. The
// unit and (optional) required rank are validated by the DB FKs (ErrUnknownUnit / ErrUnknownRank);
// a duplicate code within the unit surfaces ErrPositionCodeConflict.
func (s *Service) CreatePosition(ctx context.Context, p domain.Position) (domain.Position, error) {
	if err := p.Validate(); err != nil {
		return domain.Position{}, err
	}
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertPosition(ctx, p)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, auditSubsystem, "position.create", targetPosition, created.ID, map[string]any{"id": created.ID, "unitId": created.UnitID, "code": created.Code})
	})
	return out, err
}

// GetPosition reads one position with its current holder (the single active filling) attached.
func (s *Service) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	repo := s.newRepo(s.querier(ctx))
	p, err := repo.GetPosition(ctx, id)
	if err != nil {
		return domain.Position{}, err
	}
	holder, err := repo.ActiveFillingByPosition(ctx, id)
	switch {
	case err == nil:
		p.Holder = &holder
	case errors.Is(err, domain.ErrMembershipNotFound):
		// vacant — no holder
	default:
		return domain.Position{}, err
	}
	return p, nil
}

// UpdatePosition applies a partial change to title/required-rank/sort-order and records the action.
func (s *Service) UpdatePosition(ctx context.Context, id string, patch domain.PositionPatch) (domain.Position, error) {
	if err := patch.Validate(); err != nil {
		return domain.Position{}, err
	}
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdatePosition(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, auditSubsystem, "position.update", targetPosition, id, map[string]any{"id": id})
	})
	return out, err
}

// AbolishPosition abolishes a billet (reversible status flip). Refused with ErrPositionInUse when the
// billet has an active filling (end the membership first). Idempotent: an already-abolished position
// is returned unchanged.
func (s *Service) AbolishPosition(ctx context.Context, id string) (domain.Position, error) {
	var out domain.Position
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		p, err := repo.GetPosition(ctx, id)
		if err != nil {
			return err
		}
		if p.Status == domain.PositionAbolished {
			out = p // idempotent no-op
			return nil
		}
		if _, err := repo.ActiveFillingByPosition(ctx, id); err == nil {
			return domain.ErrPositionInUse
		} else if !errors.Is(err, domain.ErrMembershipNotFound) {
			return err
		}
		abolished, err := repo.AbolishPosition(ctx, id)
		if err != nil {
			return err
		}
		out = abolished
		return s.record(ctx, tx, auditSubsystem, "position.abolish", targetPosition, id, map[string]any{"id": id, "status": string(abolished.Status)})
	})
	return out, err
}

// ListPositions returns a keyset-paginated page of a unit's positions, optionally filtered to
// vacant / filled (D-Position: a vacancy is an active, unfilled position).
func (s *Service) ListPositions(ctx context.Context, unitID string, filter domain.PositionFilter, pageSize int, pageToken string) (PositionPage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return PositionPage{}, err
	}
	repo := s.newRepo(s.querier(ctx))
	positions, err := repo.ListPositions(ctx, unitID, filter, after, size+1)
	if err != nil {
		return PositionPage{}, err
	}
	page := positions
	var next string
	if len(positions) > size {
		page = positions[:size]
		next = listing.EncodeCursor(positions[size-1].ID)
	}
	// Attach the current holder to each billet so callers (e.g. the unit page) can show and link the
	// person filling it, not just a vacant/filled flag. The page is unit-scoped and admin-sized, so a
	// per-position lookup is acceptable; a vacant billet simply has no active filling.
	for i := range page {
		holder, herr := repo.ActiveFillingByPosition(ctx, page[i].ID)
		switch {
		case herr == nil:
			page[i].Holder = &holder
		case errors.Is(herr, domain.ErrMembershipNotFound):
			// vacant — no holder
		default:
			return PositionPage{}, herr
		}
	}
	return PositionPage{Positions: page, NextPageToken: next}, nil
}

// ---------------------------------------------------------------- memberships

// CreateMembership adds a person's belonging to a unit, optionally filling a position, and records
// the action. When a position is referenced it must exist, be active, and belong to the same unit
// (else ErrUnknownPosition / ErrMembershipInvalid / ErrPositionUnitMismatch); the one-holder index
// surfaces ErrPositionAlreadyFilled and the belonging index ErrMembershipConflict.
func (s *Service) CreateMembership(ctx context.Context, m domain.Membership) (domain.Membership, error) {
	if err := m.Validate(); err != nil {
		return domain.Membership{}, err
	}
	var out domain.Membership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.createMembershipTx(ctx, tx, auditSubsystem, m)
		out = created
		return err
	})
	return out, err
}

// createMembershipTx is the shared create core: it runs on the caller's transaction so a write and its
// audit row commit together, recording under `subsystem` (membership-admin for API calls,
// event-subscriber for order-driven appointments). The membership is pre-validated by the caller.
func (s *Service) createMembershipTx(ctx context.Context, tx pgx.Tx, subsystem string, m domain.Membership) (domain.Membership, error) {
	repo := s.newRepo(tx)
	if m.PositionID != "" {
		if err := s.checkPositionForFill(ctx, repo, m.PositionID, m.UnitID); err != nil {
			return domain.Membership{}, err
		}
	}
	created, err := repo.InsertMembership(ctx, m)
	if err != nil {
		return domain.Membership{}, err
	}
	return created, s.recordMembershipWith(ctx, tx, subsystem, "membership.create", created)
}

// FillPosition fills a vacant position with a person (a membership whose unit is the position's).
// The position must exist (else ErrPositionNotFound) and be active; an already-filled billet
// surfaces ErrPositionAlreadyFilled.
func (s *Service) FillPosition(ctx context.Context, positionID, personID, orderItemID string, effectiveFrom time.Time) (domain.Membership, error) {
	var out domain.Membership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.fillPositionTx(ctx, tx, auditSubsystem, positionID, personID, orderItemID, effectiveFrom)
		out = created
		return err
	})
	return out, err
}

// fillPositionTx is the shared fill core, running on the caller's transaction and recording under
// `subsystem`. It is the membership-start effect path for an order that names a position (D-OrderApply).
func (s *Service) fillPositionTx(ctx context.Context, tx pgx.Tx, subsystem, positionID, personID, orderItemID string, effectiveFrom time.Time) (domain.Membership, error) {
	repo := s.newRepo(tx)
	pos, err := repo.GetPosition(ctx, positionID)
	if err != nil {
		return domain.Membership{}, err
	}
	if pos.Status != domain.PositionActive {
		return domain.Membership{}, errors.Join(domain.ErrMembershipInvalid, errors.New("position is not active"))
	}
	m := domain.Membership{
		PersonID:      personID,
		UnitID:        pos.UnitID,
		PositionID:    positionID,
		OrderItemID:   orderItemID,
		EffectiveFrom: effectiveFrom,
	}
	if err := m.Validate(); err != nil {
		return domain.Membership{}, err
	}
	created, err := repo.InsertMembership(ctx, m)
	if err != nil {
		return domain.Membership{}, err
	}
	return created, s.recordMembershipWith(ctx, tx, subsystem, "membership.fill", created)
}

// EndMembership ends a membership, vacating any filled billet (reversible flip + effective_to).
// Allowed only from active (else ErrMembershipLifecycle).
func (s *Service) EndMembership(ctx context.Context, id, orderItemID string, effectiveTo time.Time) (domain.Membership, error) {
	var out domain.Membership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		ended, err := s.endMembershipTx(ctx, tx, auditSubsystem, id, orderItemID, effectiveTo)
		out = ended
		return err
	})
	return out, err
}

// endMembershipTx is the shared end core, running on the caller's transaction and recording under
// `subsystem`. It is the membership-end effect path for an order (D-OrderApply); a zero effectiveTo
// defaults to now.
func (s *Service) endMembershipTx(ctx context.Context, tx pgx.Tx, subsystem, id, orderItemID string, effectiveTo time.Time) (domain.Membership, error) {
	if effectiveTo.IsZero() {
		effectiveTo = time.Now().UTC()
	}
	repo := s.newRepo(tx)
	m, err := repo.GetMembership(ctx, id)
	if err != nil {
		return domain.Membership{}, err
	}
	if !m.CanEnd() {
		return domain.Membership{}, domain.ErrMembershipLifecycle
	}
	ended, err := repo.EndMembership(ctx, id, effectiveTo, orderItemPtr(orderItemID))
	if err != nil {
		return domain.Membership{}, err
	}
	return ended, s.recordMembershipWith(ctx, tx, subsystem, "membership.end", ended)
}

// ---------------------------------------------------------------- order-event subscribers (D-OrderApply)

// SubscribeOrderEvents registers the membership effect handlers on the bus: AppointmentOrdered creates
// the membership (fills the named position, or a plain belonging) and RemovalOrdered ends the target
// membership — both run synchronously in the order's issue transaction, so a violated invariant rolls
// the whole issue back. Registered once at composition time (module.go), before serving.
func (s *Service) SubscribeOrderEvents(bus *events.Bus) {
	bus.Subscribe(orderevents.TypeAppointmentOrdered, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(orderevents.AppointmentOrdered)
		if !ok {
			return nil
		}
		return s.handleAppointmentOrdered(ctx, tx, e)
	})
	bus.Subscribe(orderevents.TypeRemovalOrdered, func(ctx context.Context, tx pgx.Tx, evt events.Event) error {
		e, ok := evt.(orderevents.RemovalOrdered)
		if !ok {
			return nil
		}
		return s.handleRemovalOrdered(ctx, tx, e)
	})
}

// handleAppointmentOrdered realizes a membership-start item: it fills the named billet, or — when the
// item names a unit but no position — creates a plain belonging, citing order_item_id as provenance.
func (s *Service) handleAppointmentOrdered(ctx context.Context, tx pgx.Tx, e orderevents.AppointmentOrdered) error {
	if e.PositionID != "" {
		_, err := s.fillPositionTx(ctx, tx, eventSubsystem, e.PositionID, e.PersonID, e.OrderItemID, e.EffectiveFrom)
		return err
	}
	m := domain.Membership{PersonID: e.PersonID, UnitID: e.UnitID, OrderItemID: e.OrderItemID, EffectiveFrom: e.EffectiveFrom}
	if err := m.Validate(); err != nil {
		return err
	}
	_, err := s.createMembershipTx(ctx, tx, eventSubsystem, m)
	return err
}

// handleRemovalOrdered realizes a membership-end item: it resolves the target membership (the filling
// of the named position, or the person's plain belonging in the named unit) and ends it. A missing
// target surfaces ErrMembershipNotFound, rolling the issue back.
func (s *Service) handleRemovalOrdered(ctx context.Context, tx pgx.Tx, e orderevents.RemovalOrdered) error {
	repo := s.newRepo(tx)
	var target domain.Membership
	var err error
	if e.PositionID != "" {
		target, err = repo.ActiveFillingByPosition(ctx, e.PositionID)
	} else {
		target, err = repo.ActivePlainMembership(ctx, e.PersonID, e.UnitID)
	}
	if err != nil {
		return err
	}
	_, err = s.endMembershipTx(ctx, tx, eventSubsystem, target.ID, e.OrderItemID, e.EffectiveTo)
	return err
}

// ListMembers returns a keyset-paginated page of a unit's active memberships (its roster).
func (s *Service) ListMembers(ctx context.Context, unitID string, pageSize int, pageToken string) (MembershipPage, error) {
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.querier(ctx)).ListMembersByUnit(ctx, unitID, after, limit)
	})
}

// ListPersonMemberships returns a keyset-paginated page of a person's active memberships.
func (s *Service) ListPersonMemberships(ctx context.Context, personID string, pageSize int, pageToken string) (MembershipPage, error) {
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.querier(ctx)).ListMembershipsByPerson(ctx, personID, after, limit)
	})
}

// ListMemberships is the INSTANCE-ADMIN arm of the top-level facet-filtered listing (M56 ticket 3 /
// D-ObjectFacets): every membership, every status, narrowed only by the caller's filter.
//
// Validate runs HERE rather than in either arm, so the admin and read-scope paths reject an invalid
// filter identically — half of the no-drift contract the two paths are held to.
func (s *Service) ListMemberships(ctx context.Context, f domain.MembershipFilter, pageSize int, pageToken string) (MembershipPage, error) {
	if err := f.Validate(); err != nil {
		return MembershipPage{}, err
	}
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.querier(ctx)).ListMemberships(ctx, f, after, limit)
	})
}

// ListVisibleMemberships is the READ-SCOPE arm: the same filter intersected with the subject's
// effective readable reach, folded into the SQL so the page is cut after the intersection (R-06 — a
// Go-side re-filter would return a short page with a nextPageToken).
func (s *Service) ListVisibleMemberships(ctx context.Context, subjectPersonID string, f domain.MembershipFilter, pageSize int, pageToken string) (MembershipPage, error) {
	if err := f.Validate(); err != nil {
		return MembershipPage{}, err
	}
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.querier(ctx)).ListMembershipsForSubject(ctx, subjectPersonID, f, after, limit)
	})
}

// ReadableUnitIDsForSubject exposes migration 0017's reach projection for the differential test that
// holds it equal to the inline CTE (see domain.Repository).
func (s *Service) ReadableUnitIDsForSubject(ctx context.Context, subjectPersonID string) ([]string, error) {
	return s.newRepo(s.querier(ctx)).ReadableUnitIDsForSubject(ctx, subjectPersonID)
}

// ActiveUnitIDsForPerson returns the distinct units a person currently belongs to via active
// memberships — the cross-module query the person/document read-scope projection intersects with the
// reader's effective readable units (D-PersonReadScope). Runs on the request-pinned connection.
func (s *Service) ActiveUnitIDsForPerson(ctx context.Context, personID string) ([]string, error) {
	return s.newRepo(s.querier(ctx)).ActiveUnitIDsByPerson(ctx, personID)
}

// VisiblePersonIDsForSubject returns the distinct persons the subject may read under
// D-PersonReadScope, computed as one SQL semi-join over memberships × the subject's authz reach
// (review-2026-07 R-02.1 — replaces the app-side reach flatten + PersonIDsWithActiveMembershipInUnits
// array union). Keyset-paginated by person RID; powers the directory list (GET /persons).
func (s *Service) VisiblePersonIDsForSubject(ctx context.Context, subjectPersonID, after string, f persondomain.PersonFilter, limit int) ([]string, error) {
	return s.newRepo(s.querier(ctx)).VisiblePersonIDsForSubject(ctx, subjectPersonID, after, f, limit)
}

// MembershipStats is the roster dashboard (M57 / D-ObjectFacets): every selected facet's distribution
// plus the total, over EXACTLY the set ListMemberships/ListVisibleMemberships would page under the same
// filter. One round-trip, one scan, counts taken inside the visibility predicate.
//
// The admin/scoped dispatch is the caller's, as it is for the list: an instance admin aggregates every
// membership, anyone else aggregates their reach. The filter is validated here, once, so both arms
// reject an ill-formed facet value identically.
func (s *Service) MembershipStats(ctx context.Context, subjectPersonID string, isAdmin bool, f domain.MembershipFilter, sel stats.Selection) (stats.Result, error) {
	if err := f.Validate(); err != nil {
		return stats.Result{}, err
	}
	return stats.Compute(ctx, s.labeler, sel, isAdmin, subjectPersonID, func(subject string) ([]stats.Group, error) {
		return s.newRepo(s.querier(ctx)).MembershipStats(ctx, subject, f, sel)
	})
}

// VisiblePersonStatsForSubject is the read-scope directory DASHBOARD (M57 / D-ObjectFacets): the
// same filter and the same reach predicate as VisiblePersonIDsForSubject, aggregated rather than
// paged, with every count taken inside the visibility predicate.
func (s *Service) VisiblePersonStatsForSubject(ctx context.Context, subjectPersonID string, f persondomain.PersonFilter, sel stats.Selection) ([]stats.Group, error) {
	return s.newRepo(s.querier(ctx)).VisiblePersonStatsForSubject(ctx, subjectPersonID, f, sel)
}

// SubjectCanReadPerson is the point probe of the same reach predicate: whether any of the person's
// active-membership units falls in the subject's readable reach (GET /persons/{id} and the document
// holder gate).
func (s *Service) SubjectCanReadPerson(ctx context.Context, subjectPersonID, personID string) (bool, error) {
	return s.newRepo(s.querier(ctx)).SubjectCanReadPerson(ctx, subjectPersonID, personID)
}

// SubjectReadablePersonsAmong is the batch form of the same probe: which of the candidate persons
// fall in the subject's readable reach (D-VisibilityScope person-scope adapter, review-2026-09 R-30).
// Unordered set semantics; callers preserve their own candidate order.
func (s *Service) SubjectReadablePersonsAmong(ctx context.Context, subjectPersonID string, personIDs []string) ([]string, error) {
	return s.newRepo(s.querier(ctx)).SubjectReadablePersonsAmong(ctx, subjectPersonID, personIDs)
}

// ---------------------------------------------------------------- helpers

// checkPositionForFill verifies a referenced position exists, is active, and belongs to the cited
// unit before a filling is inserted. A missing position becomes ErrUnknownPosition (the membership
// request named an invalid position id), distinct from a position-targeted ErrPositionNotFound.
func (s *Service) checkPositionForFill(ctx context.Context, repo domain.Repository, positionID, unitID string) error {
	pos, err := repo.GetPosition(ctx, positionID)
	if err != nil {
		if errors.Is(err, domain.ErrPositionNotFound) {
			return domain.ErrUnknownPosition
		}
		return err
	}
	if pos.Status != domain.PositionActive {
		return errors.Join(domain.ErrMembershipInvalid, errors.New("position is not active"))
	}
	if pos.UnitID != unitID {
		return domain.ErrPositionUnitMismatch
	}
	return nil
}

// listMemberships is the shared keyset-pagination wrapper for the two membership listings.
func (s *Service) listMemberships(ctx context.Context, pageSize int, pageToken string, fetch func(after string, limit int) ([]domain.Membership, error)) (MembershipPage, error) {
	size := pageSizePolicy.Resolve(pageSize)
	after, err := listing.DecodeCursor(pageToken)
	if err != nil {
		return MembershipPage{}, err
	}
	ms, err := fetch(after, size+1)
	if err != nil {
		return MembershipPage{}, err
	}
	if len(ms) > size {
		return MembershipPage{Memberships: ms[:size], NextPageToken: listing.EncodeCursor(ms[size-1].ID)}, nil
	}
	return MembershipPage{Memberships: ms}, nil
}

// querier returns the request-pinned RLS connection if one is in context (db.AcquireScoped/WithConn),
// else the bare pool. Reads/writes on the unit-scoped membership tables MUST go through it so the
// app.* RLS GUCs apply (D-RLSDefenseInDepth).
func (s *Service) querier(ctx context.Context) db.Querier {
	return db.RequestQuerier(ctx, s.pool)
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.querier(ctx).Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recordMembershipWith writes the audit row for a membership action under the given subsystem, carrying
// only non-PII identifiers (the membership/person/unit ids + status), never personal data — person is
// the PII store. The subsystem is membership-admin for API writes, event-subscriber for order-driven
// ones (D-OrderApply).
func (s *Service) recordMembershipWith(ctx context.Context, tx pgx.Tx, subsystem, action string, m domain.Membership) error {
	after := map[string]any{"id": m.ID, "personId": m.PersonID, "unitId": m.UnitID, "status": string(m.Status)}
	if m.PositionID != "" {
		after["positionId"] = m.PositionID
	}
	if m.OrderItemID != "" {
		after["orderItemId"] = m.OrderItemID
	}
	return s.record(ctx, tx, subsystem, action, targetMembership, m.ID, after)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the
// audit entry commits iff the change commits (D-Audit). The actor is the interim system actor under
// the given subsystem.
func (s *Service) record(ctx context.Context, tx pgx.Tx, subsystem, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  subsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID mints an Action RID (membership service=7, kind=action=3, generic action type=0).
// The specific action name is recorded separately in audit_log.action (D-Audit).
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	_ = action
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_id(7, 3, 0)").Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func orderItemPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// requestID is the correlation key shared with logs/metrics/traces: the request's trace id, with a
// generated fallback for out-of-request callers (e.g. integration tests).
func requestID(ctx context.Context) string {
	if id := wtracing.TraceIDFromContext(ctx); id != "" {
		return string(id)
	}
	return "req-" + uuid.NewString()
}

func toJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}
