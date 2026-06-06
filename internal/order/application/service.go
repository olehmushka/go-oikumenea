// Package application holds the order module's application service — the orchestrator the transport
// layer calls to manage administrative orders (наказ) and their type catalog, recording an audit row
// in the same transaction as each write (D-Audit). It owns the headline ISSUE flow (D-OrderApply):
// issuing a draft locks it and publishes, on the in-process event bus and WITHIN THE ISSUE
// TRANSACTION, an effect-typed intent event per structural item that membership/person subscribers
// apply — all-or-nothing, so a violated target invariant rolls the whole issue back and the order
// stays draft.
//
// Existence of referenced types/persons/units/positions/ranks is validated by the DB foreign keys
// (mapped to domain sentinels in the adapter); which target columns an item must carry is checked
// against its type's effect. An order is a directory/record entity and never an authz input.
package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
	orderevents "github.com/olegamysk/go-oikumenea/internal/order/events"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/palantir/witchcraft-go-tracing/wtracing"
)

// Page-size policy (API conventions: token pagination, bounded pages).
const (
	DefaultPageSize = 50
	MaxPageSize     = 500
)

// isoDate is the calendar-date wire format used for the effective-date legal metadata on items.
const isoDate = "2006-01-02"

// auditSubsystem labels the interim system actor for order writes. Until the acting person is threaded
// through (the shared M7/M8 follow-up) these are recorded as a `system` action under this subsystem.
const auditSubsystem = "order-admin"

// Audit target types (the audited entity kinds).
const (
	targetOrder     = "order"
	targetOrderType = "order_type"
)

// RepositoryFactory binds a domain.Repository to a command surface — the pool for reads, or a caller's
// transaction for an audited write (D-Audit). Injected by module.go so the application never imports
// adapters.
type RepositoryFactory func(conn db.DBTX) domain.Repository

// Service is the order application service. It owns its writes, so it holds the pool to open
// transactions; reads run on the pool directly. The bus dispatches issue-time effect events to
// membership/person subscribers within the issue transaction (D-OrderApply).
type Service struct {
	pool    *pgxpool.Pool
	newRepo RepositoryFactory
	audit   *auditapp.Service
	bus     *events.Bus
}

// NewService wires the service with the pool, the repository factory, the audit service, and the
// event bus.
func NewService(pool *pgxpool.Pool, newRepo RepositoryFactory, audit *auditapp.Service, bus *events.Bus) *Service {
	return &Service{pool: pool, newRepo: newRepo, audit: audit, bus: bus}
}

// OrderPage is a keyset-paginated slice of orders (headers only) plus the opaque next-page token.
type OrderPage struct {
	Orders        []domain.Order
	NextPageToken string
}

// CreateOrderInput / UpdateOrderInput carry a create/edit request from the transport.
type CreateOrderInput struct {
	Number   string
	IssuedOn string
	Items    []domain.OrderItem
}

// UpdateOrderInput edits a draft order. Nil scalar pointers leave the field unchanged; a nil Items
// pointer leaves items unchanged, a non-nil (possibly empty — rejected) pointer REPLACES them.
type UpdateOrderInput struct {
	Number   *string
	IssuedOn *string
	Items    *[]domain.OrderItem
}

// ---------------------------------------------------------------- order types

func (s *Service) CreateOrderType(ctx context.Context, t domain.OrderType) (domain.OrderType, error) {
	if err := t.Validate(); err != nil {
		return domain.OrderType{}, err
	}
	var out domain.OrderType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		created, err := s.newRepo(tx).InsertOrderType(ctx, t)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "order.type.create", targetOrderType, created.ID,
			map[string]any{"id": created.ID, "code": created.Code, "category": string(created.Category), "effect": string(created.Effect)})
	})
	return out, err
}

func (s *Service) UpdateOrderType(ctx context.Context, id string, patch domain.OrderTypePatch) (domain.OrderType, error) {
	if err := patch.Validate(); err != nil {
		return domain.OrderType{}, err
	}
	var out domain.OrderType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		updated, err := s.newRepo(tx).UpdateOrderType(ctx, id, patch)
		if err != nil {
			return err
		}
		out = updated
		return s.record(ctx, tx, "order.type.update", targetOrderType, id, map[string]any{"id": id, "status": string(updated.Status)})
	})
	return out, err
}

func (s *Service) ListOrderTypes(ctx context.Context) ([]domain.OrderType, error) {
	return s.newRepo(s.querier(ctx)).ListOrderTypes(ctx)
}

// ---------------------------------------------------------------- orders

// CreateOrder creates a draft order with its items (≥1) for an issuing unit, validating each item's
// required targets against its type's effect, and records the action.
func (s *Service) CreateOrder(ctx context.Context, issuingUnitID string, in CreateOrderInput) (domain.Order, error) {
	if len(in.Items) == 0 {
		return domain.Order{}, errors.Join(domain.ErrOrderInvalid, errors.New("an order needs at least one item"))
	}
	var out domain.Order
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		order, err := repo.InsertOrder(ctx, domain.Order{Number: in.Number, IssuedOn: in.IssuedOn, IssuingUnitID: issuingUnitID})
		if err != nil {
			return err
		}
		items, err := s.insertItems(ctx, repo, order.ID, in.Items)
		if err != nil {
			return err
		}
		order.Items = items
		out = order
		return s.record(ctx, tx, "order.create", targetOrder, order.ID,
			map[string]any{"id": order.ID, "unitId": issuingUnitID, "items": len(items)})
	})
	return out, err
}

// UpdateOrder edits a draft order's header and (optionally) replaces its items. Rejected once issued
// (ErrAlreadyIssued).
func (s *Service) UpdateOrder(ctx context.Context, id string, in UpdateOrderInput) (domain.Order, error) {
	if in.Items != nil && len(*in.Items) == 0 {
		return domain.Order{}, errors.Join(domain.ErrOrderInvalid, errors.New("an order needs at least one item"))
	}
	var out domain.Order
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		cur, err := repo.GetOrder(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status != domain.OrderDraft {
			return domain.ErrAlreadyIssued
		}
		updated, err := repo.UpdateOrderHeader(ctx, id, in.Number, in.IssuedOn)
		if err != nil {
			return err
		}
		var items []domain.OrderItem
		if in.Items != nil {
			if err := repo.DeleteOrderItems(ctx, id); err != nil {
				return err
			}
			if items, err = s.insertItems(ctx, repo, id, *in.Items); err != nil {
				return err
			}
		} else if items, err = repo.GetOrderItems(ctx, id); err != nil {
			return err
		}
		updated.Items = items
		out = updated
		return s.record(ctx, tx, "order.update", targetOrder, id, map[string]any{"id": id, "items": len(items)})
	})
	return out, err
}

// IssueOrder is the headline legal-basis act (D-OrderApply): in ONE transaction it locks the draft,
// audits the issue, then publishes the umbrella OrderIssued plus one effect-typed intent event per
// structural item on the bus — each handled synchronously by the owning module's subscriber in this
// same transaction. A subscriber error (a violated target invariant) aborts the whole issue: it
// surfaces wrapped in ErrEffectFailed and the order stays draft.
func (s *Service) IssueOrder(ctx context.Context, id string) (domain.Order, error) {
	var out domain.Order
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		cur, err := repo.GetOrder(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status != domain.OrderDraft {
			return domain.ErrAlreadyIssued
		}
		items, err := repo.GetOrderItems(ctx, id)
		if err != nil {
			return err
		}
		issued, err := repo.MarkIssued(ctx, id)
		if err != nil {
			return err
		}
		if err := s.record(ctx, tx, "order.issue", targetOrder, id,
			map[string]any{"id": id, "unitId": issued.IssuingUnitID, "items": len(items)}); err != nil {
			return err
		}
		if err := s.bus.Publish(ctx, tx, orderevents.OrderIssued{OrderID: id, IssuingUnitID: issued.IssuingUnitID}); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrEffectFailed, err)
		}
		for _, it := range items {
			evt := effectEvent(it)
			if evt == nil {
				continue // record-only: authoritative as the item itself, no downstream write
			}
			if err := s.bus.Publish(ctx, tx, evt); err != nil {
				return fmt.Errorf("%w: %s", domain.ErrEffectFailed, err)
			}
		}
		issued.Items = items
		out = issued
		return nil
	})
	return out, err
}

// RevokeOrder flips an issued order to revoked (a legal-status change; effects are NOT auto-reversed —
// undo is the revoking order's own items). Emits OrderRevoked (no subscriber today).
func (s *Service) RevokeOrder(ctx context.Context, id string, revokingOrderID *string) (domain.Order, error) {
	var out domain.Order
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		cur, err := repo.GetOrder(ctx, id)
		if err != nil {
			return err
		}
		if cur.Status != domain.OrderIssued {
			return domain.ErrNotIssued
		}
		revoked, err := repo.MarkRevoked(ctx, id, revokingOrderID)
		if err != nil {
			return err
		}
		items, err := repo.GetOrderItems(ctx, id)
		if err != nil {
			return err
		}
		revoked.Items = items
		out = revoked
		if err := s.record(ctx, tx, "order.revoke", targetOrder, id, map[string]any{"id": id, "revokedBy": derefStr(revokingOrderID)}); err != nil {
			return err
		}
		return s.bus.Publish(ctx, tx, orderevents.OrderRevoked{OrderID: id})
	})
	return out, err
}

// GetOrder returns an order header with its items (each carrying the type's effect).
func (s *Service) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	repo := s.newRepo(s.querier(ctx))
	order, err := repo.GetOrder(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	items, err := repo.GetOrderItems(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	order.Items = items
	return order, nil
}

func (s *Service) ListOrdersByUnit(ctx context.Context, unitID string, pageSize int, pageToken string) (OrderPage, error) {
	return s.listOrders(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Order, error) {
		return s.newRepo(s.querier(ctx)).ListOrdersByUnit(ctx, unitID, after, limit)
	})
}

func (s *Service) ListOrdersByPerson(ctx context.Context, personID string, pageSize int, pageToken string) (OrderPage, error) {
	return s.listOrders(ctx, pageSize, pageToken, func(after string, limit int) ([]domain.Order, error) {
		return s.newRepo(s.querier(ctx)).ListOrdersByPerson(ctx, personID, after, limit)
	})
}

// ---------------------------------------------------------------- helpers

// insertItems validates each item against its type's effect (the type is loaded for the effect; an
// unknown type id becomes ErrUnknownType) and inserts it under the order, returning the inserted items
// with their effect resolved.
func (s *Service) insertItems(ctx context.Context, repo domain.Repository, orderID string, ins []domain.OrderItem) ([]domain.OrderItem, error) {
	out := make([]domain.OrderItem, 0, len(ins))
	for _, it := range ins {
		ot, err := repo.GetOrderType(ctx, it.TypeID)
		if err != nil {
			if errors.Is(err, domain.ErrOrderTypeNotFound) {
				return nil, domain.ErrUnknownType
			}
			return nil, err
		}
		if !domain.RequiredTargetsPresent(ot.Effect, it) {
			return nil, errors.Join(domain.ErrOrderInvalid, fmt.Errorf("an item with effect %q is missing its required target", ot.Effect))
		}
		it.OrderID = orderID
		created, err := repo.InsertOrderItem(ctx, it)
		if err != nil {
			return nil, err
		}
		created.Effect = ot.Effect
		out = append(out, created)
	}
	return out, nil
}

// effectEvent maps a structural item to its intent event; record-only items map to nil (no event).
func effectEvent(it domain.OrderItem) events.Event {
	switch it.Effect {
	case domain.EffectMembershipStart:
		return orderevents.AppointmentOrdered{
			OrderItemID:   it.ID,
			PersonID:      it.PersonID,
			UnitID:        it.UnitID,
			PositionID:    it.PositionID,
			EffectiveFrom: parseDate(it.EffectiveFrom),
		}
	case domain.EffectMembershipEnd:
		return orderevents.RemovalOrdered{
			OrderItemID: it.ID,
			PersonID:    it.PersonID,
			UnitID:      it.UnitID,
			PositionID:  it.PositionID,
			EffectiveTo: parseDate(it.EffectiveTo),
		}
	case domain.EffectRankChange:
		return orderevents.RankChangeOrdered{
			OrderItemID: it.ID,
			PersonID:    it.PersonID,
			RankID:      it.RankID,
		}
	default:
		return nil
	}
}

// parseDate turns an ISO-8601 date into an instant (UTC midnight); "" or an unparseable value yields
// the zero time, which membership/person subscribers treat as "now".
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func (s *Service) listOrders(ctx context.Context, pageSize int, pageToken string, fetch func(after string, limit int) ([]domain.Order, error)) (OrderPage, error) {
	size := resolvePageSize(pageSize)
	after, err := decodeCursor(pageToken)
	if err != nil {
		return OrderPage{}, err
	}
	orders, err := fetch(after, size+1)
	if err != nil {
		return OrderPage{}, err
	}
	if len(orders) > size {
		return OrderPage{Orders: orders[:size], NextPageToken: encodeCursor(orders[size-1].ID)}, nil
	}
	return OrderPage{Orders: orders}, nil
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---------------------------------------------------------------- tx + audit plumbing

// querier returns the request-pinned RLS connection if one is in context (db.AcquireScoped/WithConn),
// else the bare pool. Reads/writes on order_orders (unit-scoped on issuing_unit_id) MUST go through it
// so the app.* RLS GUCs apply (D-RLSDefenseInDepth).
func (s *Service) querier(ctx context.Context) db.Querier {
	if c, ok := db.ConnFromContext(ctx); ok {
		return c
	}
	return s.pool
}

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

// record mints an Action RID in the caller's transaction and writes the audit row on it, so the audit
// entry commits iff the change commits (D-Audit). PII discipline: `after` carries only ids/keys —
// never an order item's free-text note.
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

// mintActionRID composes the Action RID via the same SQL generator every module uses (e.g.
// "order.issue" → entity_type "action__order_issue"), satisfying the audit log's action__<type> shape.
func mintActionRID(ctx context.Context, tx pgx.Tx, action string) (string, error) {
	entityType := "action__" + sanitizeAction(action)
	var rid string
	if err := tx.QueryRow(ctx, "SELECT oikumenea.new_rid('order', $1)", entityType).Scan(&rid); err != nil {
		return "", err
	}
	return rid, nil
}

func sanitizeAction(action string) string {
	b := make([]byte, len(action))
	for i := 0; i < len(action); i++ {
		switch action[i] {
		case '.', '-':
			b[i] = '_'
		default:
			b[i] = action[i]
		}
	}
	return string(b)
}

// requestID is the correlation key shared with logs/metrics/traces and the issue's subscriber audit
// rows: the request's trace id, with a generated fallback for out-of-request callers (tests).
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
