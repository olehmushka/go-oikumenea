// Package adapters implements the order domain ports against infrastructure: the pgx/sqlc repository
// over the oikumenea.order_* tables. It depends on the database, never the reverse (overview.md).
// Generated sqlc code lives in the ordersql subpackage and is never hand-edited.
//
// Calendar DATE columns (issued_on, effective_from/to) map to ISO-8601 "YYYY-MM-DD" strings ("" =
// absent); existence of referenced type/person/unit/position/rank is validated by the FKs and mapped
// to domain sentinels here (no pre-check lookups).
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/order/adapters/ordersql"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// isoDate is the calendar-date wire format (a day, not an instant).
const isoDate = "2006-01-02"

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *ordersql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: ordersql.New(conn)}
}

var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- order types

func (r *Repository) InsertOrderType(ctx context.Context, t domain.OrderType) (domain.OrderType, error) {
	var sortOrder interface{}
	if t.SortOrder != nil {
		sortOrder = int32(*t.SortOrder)
	}
	row, err := r.q.InsertOrderType(ctx, ordersql.InsertOrderTypeParams{
		Code:      t.Code,
		Name:      t.Name,
		Category:  string(t.Category),
		Effect:    string(t.Effect),
		SortOrder: sortOrder,
	})
	if err != nil {
		return domain.OrderType{}, mapWriteErr(err)
	}
	return toOrderType(row), nil
}

func (r *Repository) UpdateOrderType(ctx context.Context, id string, patch domain.OrderTypePatch) (domain.OrderType, error) {
	row, err := r.q.UpdateOrderType(ctx, ordersql.UpdateOrderTypeParams{
		Name:      textPtr(patch.Name),
		Status:    textPtr(patch.Status),
		SortOrder: int4Ptr(patch.SortOrder),
		ID:        id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrderType{}, domain.ErrOrderTypeNotFound
		}
		return domain.OrderType{}, mapWriteErr(err)
	}
	return toOrderType(row), nil
}

func (r *Repository) GetOrderType(ctx context.Context, id string) (domain.OrderType, error) {
	row, err := r.q.GetOrderType(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrderType{}, domain.ErrOrderTypeNotFound
		}
		return domain.OrderType{}, err
	}
	return toOrderType(row), nil
}

func (r *Repository) ListOrderTypes(ctx context.Context) ([]domain.OrderType, error) {
	rows, err := r.q.ListOrderTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.OrderType, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOrderType(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- orders (header)

func (r *Repository) InsertOrder(ctx context.Context, o domain.Order) (domain.Order, error) {
	row, err := r.q.InsertOrder(ctx, ordersql.InsertOrderParams{
		Number:        text(o.Number),
		IssuedOn:      dateArg(o.IssuedOn),
		IssuingUnitID: o.IssuingUnitID,
	})
	if err != nil {
		return domain.Order{}, mapWriteErr(err)
	}
	return toOrder(row), nil
}

func (r *Repository) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	row, err := r.q.GetOrder(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, err
	}
	return toOrder(row), nil
}

func (r *Repository) UpdateOrderHeader(ctx context.Context, id string, number, issuedOn *string) (domain.Order, error) {
	row, err := r.q.UpdateOrderHeader(ctx, ordersql.UpdateOrderHeaderParams{
		Number:   textPtr(number),
		IssuedOn: datePtr(issuedOn),
		ID:       id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, mapWriteErr(err)
	}
	return toOrder(row), nil
}

func (r *Repository) MarkIssued(ctx context.Context, id string) (domain.Order, error) {
	row, err := r.q.MarkIssued(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrAlreadyIssued
		}
		return domain.Order{}, err
	}
	return toOrder(row), nil
}

func (r *Repository) MarkRevoked(ctx context.Context, id string, revokingOrderID *string) (domain.Order, error) {
	row, err := r.q.MarkRevoked(ctx, ordersql.MarkRevokedParams{
		RevokingOrderID: textPtr(revokingOrderID),
		ID:              id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Order{}, domain.ErrNotIssued
		}
		return domain.Order{}, mapWriteErr(err)
	}
	return toOrder(row), nil
}

func (r *Repository) ListOrdersByUnit(ctx context.Context, unitID, after string, limit int) ([]domain.Order, error) {
	rows, err := r.q.ListOrdersByUnit(ctx, ordersql.ListOrdersByUnitParams{IssuingUnitID: unitID, After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	return ordersFrom(rows), nil
}

func (r *Repository) ListOrdersByPerson(ctx context.Context, personID, after string, limit int) ([]domain.Order, error) {
	rows, err := r.q.ListOrdersByPerson(ctx, ordersql.ListOrdersByPersonParams{PersonID: personID, After: after, Lim: int32(limit)})
	if err != nil {
		return nil, err
	}
	return ordersFrom(rows), nil
}

// ---------------------------------------------------------------- order items

func (r *Repository) InsertOrderItem(ctx context.Context, it domain.OrderItem) (domain.OrderItem, error) {
	row, err := r.q.InsertOrderItem(ctx, ordersql.InsertOrderItemParams{
		OrderID:       it.OrderID,
		TypeID:        it.TypeID,
		PersonID:      it.PersonID,
		UnitID:        text(it.UnitID),
		PositionID:    text(it.PositionID),
		RankID:        text(it.RankID),
		EffectiveFrom: dateArg(it.EffectiveFrom),
		EffectiveTo:   dateArg(it.EffectiveTo),
		Note:          text(it.Note),
	})
	if err != nil {
		return domain.OrderItem{}, mapWriteErr(err)
	}
	return toItemModel(row), nil
}

func (r *Repository) GetOrderItems(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	rows, err := r.q.GetOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.OrderItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, toItemRow(row))
	}
	return out, nil
}

func (r *Repository) DeleteOrderItems(ctx context.Context, orderID string) error {
	return r.q.DeleteOrderItems(ctx, orderID)
}

// ---------------------------------------------------------------- mapping helpers

func toOrderType(r ordersql.OikumeneaOrderOrderType) domain.OrderType {
	return domain.OrderType{
		ID:        r.ID,
		Code:      r.Code,
		Name:      r.Name,
		Category:  domain.OrderCategory(r.Category),
		Effect:    domain.OrderEffect(r.Effect),
		Status:    domain.TypeStatus(r.Status),
		SortOrder: int4Val(r.SortOrder),
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

func toOrder(r ordersql.OikumeneaOrderOrder) domain.Order {
	return domain.Order{
		ID:               r.ID,
		Number:           r.Number.String,
		IssuedOn:         dateVal(r.IssuedOn),
		IssuingUnitID:    r.IssuingUnitID,
		Status:           domain.OrderStatus(r.Status),
		RevokedByOrderID: r.RevokedByOrderID.String,
		RevokedAt:        tsPtr(r.RevokedAt),
		CreatedAt:        r.CreatedAt.Time,
		UpdatedAt:        r.UpdatedAt.Time,
	}
}

func ordersFrom(rows []ordersql.OikumeneaOrderOrder) []domain.Order {
	out := make([]domain.Order, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOrder(row))
	}
	return out
}

func toItemModel(r ordersql.OikumeneaOrderOrderItem) domain.OrderItem {
	return domain.OrderItem{
		ID:            r.ID,
		OrderID:       r.OrderID,
		TypeID:        r.TypeID,
		PersonID:      r.PersonID,
		UnitID:        r.UnitID.String,
		PositionID:    r.PositionID.String,
		RankID:        r.RankID.String,
		EffectiveFrom: dateVal(r.EffectiveFrom),
		EffectiveTo:   dateVal(r.EffectiveTo),
		Note:          r.Note.String,
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

func toItemRow(r ordersql.GetOrderItemsRow) domain.OrderItem {
	return domain.OrderItem{
		ID:            r.ID,
		OrderID:       r.OrderID,
		TypeID:        r.TypeID,
		PersonID:      r.PersonID,
		UnitID:        r.UnitID.String,
		PositionID:    r.PositionID.String,
		RankID:        r.RankID.String,
		EffectiveFrom: dateVal(r.EffectiveFrom),
		EffectiveTo:   dateVal(r.EffectiveTo),
		Note:          r.Note.String,
		Effect:        domain.OrderEffect(r.TypeEffect),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels: the
// partial-unique (issuing_unit_id, number) index and the order-type code unique key are the two
// 23505 cases; the item/header FKs name the offending reference (type / person / position / rank /
// unit) for 23503.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "unit_number"):
			return domain.ErrOrderConflict
		case strings.Contains(name, "code"):
			return domain.ErrOrderTypeConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "type"):
			return domain.ErrUnknownType
		case strings.Contains(name, "person"):
			return domain.ErrUnknownPerson
		case strings.Contains(name, "position"):
			return domain.ErrUnknownPosition
		case strings.Contains(name, "rank"):
			return domain.ErrUnknownRank
		case strings.Contains(name, "unit"):
			return domain.ErrUnknownUnit
		}
	}
	return err
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// textPtr maps a patch pointer: nil leaves the column unchanged (NULL narg → COALESCE keeps it).
func textPtr(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func int4Ptr(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int4Val(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

// dateArg parses an ISO-8601 "YYYY-MM-DD" into a pgtype.Date; "" or an unparseable value yields NULL.
func dateArg(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// datePtr maps a patch pointer to a date arg: nil leaves the column unchanged (NULL narg → COALESCE).
func datePtr(p *string) pgtype.Date {
	if p == nil {
		return pgtype.Date{}
	}
	return dateArg(*p)
}

// dateVal renders a stored date back to ISO-8601 ("" when NULL).
func dateVal(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(isoDate)
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}
