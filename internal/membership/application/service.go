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
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// auditSubsystem labels the interim system actor for membership's admin writes. Until authorization
// (M7) + identity-federation (M8) resolve the acting person, these writes are recorded as a `system`
// action under this subsystem (the no-unaudited-mutation ground rule still holds). M7/M8 replace
// this with the resolved person actor.
const auditSubsystem = "membership-admin"

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
	audit   *auditapp.Service
}

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
		return s.record(ctx, tx, "position.create", targetPosition, created.ID, map[string]any{"id": created.ID, "unitId": created.UnitID, "code": created.Code})
	})
	return out, err
}

// GetPosition reads one position with its current holder (the single active filling) attached.
func (s *Service) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	repo := s.newRepo(s.pool)
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
		return s.record(ctx, tx, "position.update", targetPosition, id, map[string]any{"id": id})
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
		return s.record(ctx, tx, "position.abolish", targetPosition, id, map[string]any{"id": id, "status": string(abolished.Status)})
	})
	return out, err
}

// ListPositions returns a keyset-paginated page of a unit's positions, optionally filtered to
// vacant / filled (D-Position: a vacancy is an active, unfilled position).
func (s *Service) ListPositions(ctx context.Context, unitID string, filter domain.PositionFilter, pageSize int, pageToken string) (PositionPage, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return PositionPage{}, err
	}
	positions, err := s.newRepo(s.pool).ListPositions(ctx, unitID, filter, after, size+1)
	if err != nil {
		return PositionPage{}, err
	}
	if len(positions) > size {
		return PositionPage{Positions: positions[:size], NextPageToken: encodeCursor(positions[size-1].ID)}, nil
	}
	return PositionPage{Positions: positions}, nil
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
		repo := s.newRepo(tx)
		if m.PositionID != "" {
			if err := s.checkPositionForFill(ctx, repo, m.PositionID, m.UnitID); err != nil {
				return err
			}
		}
		created, err := repo.InsertMembership(ctx, m)
		if err != nil {
			return err
		}
		out = created
		return s.recordMembership(ctx, tx, "membership.create", created)
	})
	return out, err
}

// FillPosition fills a vacant position with a person (a membership whose unit is the position's).
// The position must exist (else ErrPositionNotFound) and be active; an already-filled billet
// surfaces ErrPositionAlreadyFilled.
func (s *Service) FillPosition(ctx context.Context, positionID, personID, orderItemID string, effectiveFrom time.Time) (domain.Membership, error) {
	var out domain.Membership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		pos, err := repo.GetPosition(ctx, positionID)
		if err != nil {
			return err
		}
		if pos.Status != domain.PositionActive {
			return errors.Join(domain.ErrMembershipInvalid, errors.New("position is not active"))
		}
		m := domain.Membership{
			PersonID:      personID,
			UnitID:        pos.UnitID,
			PositionID:    positionID,
			OrderItemID:   orderItemID,
			EffectiveFrom: effectiveFrom,
		}
		if err := m.Validate(); err != nil {
			return err
		}
		created, err := repo.InsertMembership(ctx, m)
		if err != nil {
			return err
		}
		out = created
		return s.recordMembership(ctx, tx, "membership.fill", created)
	})
	return out, err
}

// EndMembership ends a membership, vacating any filled billet (reversible flip + effective_to).
// Allowed only from active (else ErrMembershipLifecycle).
func (s *Service) EndMembership(ctx context.Context, id, orderItemID string, effectiveTo time.Time) (domain.Membership, error) {
	if effectiveTo.IsZero() {
		effectiveTo = time.Now().UTC()
	}
	var out domain.Membership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		m, err := repo.GetMembership(ctx, id)
		if err != nil {
			return err
		}
		if !m.CanEnd() {
			return domain.ErrMembershipLifecycle
		}
		ended, err := repo.EndMembership(ctx, id, effectiveTo, orderItemPtr(orderItemID))
		if err != nil {
			return err
		}
		out = ended
		return s.recordMembership(ctx, tx, "membership.end", ended)
	})
	return out, err
}

// ListMembers returns a keyset-paginated page of a unit's active memberships (its roster).
func (s *Service) ListMembers(ctx context.Context, unitID string, pageSize int, pageToken string) (MembershipPage, error) {
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.pool).ListMembersByUnit(ctx, unitID, after, limit)
	})
}

// ListPersonMemberships returns a keyset-paginated page of a person's active memberships.
func (s *Service) ListPersonMemberships(ctx context.Context, personID string, pageSize int, pageToken string) (MembershipPage, error) {
	return s.listMemberships(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Membership, error) {
		return s.newRepo(s.pool).ListMembershipsByPerson(ctx, personID, after, limit)
	})
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
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return MembershipPage{}, err
	}
	ms, err := fetch(after, size+1)
	if err != nil {
		return MembershipPage{}, err
	}
	if len(ms) > size {
		return MembershipPage{Memberships: ms[:size], NextPageToken: encodeCursor(ms[size-1].ID)}, nil
	}
	return MembershipPage{Memberships: ms}, nil
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *Service) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recordMembership writes the audit row for a membership action, carrying only non-PII identifiers
// (the membership/person/unit ids + status), never personal data — person is the PII store.
func (s *Service) recordMembership(ctx context.Context, tx pgx.Tx, action string, m domain.Membership) error {
	after := map[string]any{"id": m.ID, "personId": m.PersonID, "unitId": m.UnitID, "status": string(m.Status)}
	if m.PositionID != "" {
		after["positionId"] = m.PositionID
	}
	return s.record(ctx, tx, action, targetMembership, m.ID, after)
}

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the
// audit entry commits iff the change commits (D-Audit). The actor is the interim system actor.
func (s *Service) record(ctx context.Context, tx pgx.Tx, action, targetType, targetID string, after any) error {
	rid, err := mintActionRID(ctx, tx, action)
	if err != nil {
		return err
	}
	return s.audit.Record(ctx, tx, auditdomain.Entry{
		ID:         rid,
		ActorType:  auditdomain.ActorSystem,
		Subsystem:  auditSubsystem,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		RequestID:  requestID(ctx),
		After:      toJSON(after),
		Outcome:    auditdomain.OutcomeSuccess,
	})
}

// mintActionRID composes the Action RID via the same SQL generator every module uses, so the audit
// log's action__<type> RID-shape CHECK is satisfied. The action code (e.g. "membership.fill")
// becomes the entity_type slot "action__membership_fill".
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('membership', $1)", entityType).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			b[i] = '_'
		} else {
			b[i] = action[i]
		}
	}
	return string(b)
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

func resolvePageSize(requested int) int {
	if requested <= 0 {
		return DefaultPageSize
	}
	if requested > MaxPageSize {
		return MaxPageSize
	}
	return requested
}

// encodeCursor/decodeCursor make the keyset position (the last row's RID) into an opaque, URL-safe
// page token (API conventions: token pagination, no offsets).
func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
