// Package domain holds the order module's core model and ports (docs/modules/order.md, D-Orders):
// administrative orders (наказ) — the formal acts that are the LEGAL BASIS for a change in a person's
// status. It imports no framework; the application orchestrates it and the adapters implement its
// Repository port.
//
// An order has an issuing unit, a number/date, a draft→issued→revoked lifecycle (mutable while draft;
// locked on issue), and ≥1 items, each targeting one person (+ optional unit/position/rank per the
// type's effect). The order TYPE is an instance-admin catalog carrying a `category` (the five UA-army
// families) and an `effect` that drives which target columns an item must carry and which intent event
// issue emits. An order carries NO authority — it is the legal record of an act, never an authz input.
package domain

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors. The adapter maps Postgres constraint violations to these; the transport maps them
// to Conjure SerializableErrors.
var (
	ErrOrderNotFound = errors.New("order not found")
	ErrOrderInvalid  = errors.New("invalid order request")
	ErrOrderConflict = errors.New("an order with this number already exists in the issuing unit")
	ErrAlreadyIssued = errors.New("order is already issued")
	ErrNotIssued     = errors.New("order is not issued")

	ErrOrderTypeNotFound = errors.New("order type not found")
	ErrOrderTypeConflict = errors.New("an order type with this code already exists")

	// FK-mapped "unknown reference" sentinels (a request named an id that does not exist).
	ErrUnknownType     = errors.New("order type does not exist")
	ErrUnknownPerson   = errors.New("person does not exist")
	ErrUnknownUnit     = errors.New("unit does not exist")
	ErrUnknownPosition = errors.New("position does not exist")
	ErrUnknownRank     = errors.New("rank does not exist")

	// ErrEffectFailed wraps a target module's domain error raised by an auto-applied effect on issue;
	// the whole issue rolled back. The transport surfaces it as Order:OrderEffectFailed, carrying the
	// (safe) underlying message.
	ErrEffectFailed = errors.New("an order effect failed")
)

// OrderCategory is the five-family UA-army "стройова частина" taxonomy (directory grouping only).
type OrderCategory string

const (
	CategoryPersonnelList       OrderCategory = "personnel-list"
	CategoryAppointment         OrderCategory = "appointment"
	CategoryLeaveTravel         OrderCategory = "leave-travel"
	CategoryDisciplineIncentive OrderCategory = "discipline-incentive"
	CategoryDutyRoster          OrderCategory = "duty-roster"
)

// OrderEffect is the downstream consequence of items of a type: it determines the required target
// columns (validated in the application) and the intent event issue emits (D-OrderApply).
type OrderEffect string

const (
	EffectMembershipStart OrderEffect = "membership-start"
	EffectMembershipEnd   OrderEffect = "membership-end"
	EffectRankChange      OrderEffect = "rank-change"
	EffectRecordOnly      OrderEffect = "record-only"
)

// TypeStatus / OrderStatus enumerate the lifecycle states.
type TypeStatus string

const (
	TypeActive  TypeStatus = "active"
	TypeRetired TypeStatus = "retired"
)

type OrderStatus string

const (
	OrderDraft   OrderStatus = "draft"
	OrderIssued  OrderStatus = "issued"
	OrderRevoked OrderStatus = "revoked"
)

// OrderType is an instance-admin catalog entry naming an order kind.
type OrderType struct {
	ID        string
	Code      string
	Name      string // default-locale label; translatable via the i18n store
	Category  OrderCategory
	Effect    OrderEffect
	Status    TypeStatus
	SortOrder *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OrderTypePatch is a partial update of an order type. `code`, `category`, and `effect` are immutable
// by convention (not patchable).
type OrderTypePatch struct {
	Name      *string
	Status    *string
	SortOrder *int
}

// Order is the order header (наказ) plus, when loaded, its items.
type Order struct {
	ID               string
	Number           string // "" when absent
	IssuedOn         string // ISO-8601 date "YYYY-MM-DD"; "" when absent
	IssuingUnitID    string
	Status           OrderStatus
	RevokedByOrderID string // "" when absent
	RevokedAt        *time.Time
	Items            []OrderItem
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// OrderItem is one affected person/act within an order. Effect is populated from the item's type on
// read (it drives the issue-time event dispatch); it is "" on create input.
type OrderItem struct {
	ID            string
	OrderID       string
	TypeID        string
	PersonID      string
	UnitID        string // "" when absent
	PositionID    string // "" when absent
	RankID        string // "" when absent
	EffectiveFrom string // ISO-8601 date; "" when absent
	EffectiveTo   string // ISO-8601 date; "" when absent
	Note          string // "" when absent; pii:basic
	Effect        OrderEffect
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ValidCategory / ValidEffect report whether a string is a known enum value.
func ValidCategory(c string) bool {
	switch OrderCategory(c) {
	case CategoryPersonnelList, CategoryAppointment, CategoryLeaveTravel, CategoryDisciplineIncentive, CategoryDutyRoster:
		return true
	}
	return false
}

func ValidEffect(e string) bool {
	switch OrderEffect(e) {
	case EffectMembershipStart, EffectMembershipEnd, EffectRankChange, EffectRecordOnly:
		return true
	}
	return false
}

func validTypeStatus(s string) bool {
	switch TypeStatus(s) {
	case TypeActive, TypeRetired:
		return true
	}
	return false
}

// Validate checks an order-type create request: a non-empty code/name and a known category + effect.
func (t OrderType) Validate() error {
	if t.Code == "" || t.Name == "" {
		return errors.Join(ErrOrderInvalid, errors.New("code and name are required"))
	}
	if !ValidCategory(string(t.Category)) {
		return errors.Join(ErrOrderInvalid, errors.New("unknown category"))
	}
	if !ValidEffect(string(t.Effect)) {
		return errors.Join(ErrOrderInvalid, errors.New("unknown effect"))
	}
	return nil
}

// Validate checks an order-type patch: a present status must be a known value.
func (p OrderTypePatch) Validate() error {
	if p.Status != nil && !validTypeStatus(*p.Status) {
		return errors.Join(ErrOrderInvalid, errors.New("unknown status"))
	}
	return nil
}

// RequiredTargetsPresent reports whether an item carries the target columns its type's effect requires
// (checked in the application — order.md): membership-start/-end need a unit and/or position;
// rank-change needs a rank; record-only needs none.
func RequiredTargetsPresent(effect OrderEffect, it OrderItem) bool {
	switch effect {
	case EffectMembershipStart, EffectMembershipEnd:
		return it.UnitID != "" || it.PositionID != ""
	case EffectRankChange:
		return it.RankID != ""
	default:
		return true
	}
}

// Repository is the persistence port the application depends on; the pgx/sqlc adapter implements it.
// Each method runs on whatever DBTX the adapter was constructed with, so a write and its audit row
// share one transaction (D-Audit). Reads exclude soft-deleted orders; items are parent-scoped.
type Repository interface {
	// order types
	InsertOrderType(ctx context.Context, t OrderType) (OrderType, error)
	UpdateOrderType(ctx context.Context, id string, patch OrderTypePatch) (OrderType, error)
	GetOrderType(ctx context.Context, id string) (OrderType, error)
	ListOrderTypes(ctx context.Context) ([]OrderType, error)

	// orders (header)
	InsertOrder(ctx context.Context, o Order) (Order, error)
	GetOrder(ctx context.Context, id string) (Order, error)
	UpdateOrderHeader(ctx context.Context, id string, number, issuedOn *string) (Order, error)
	MarkIssued(ctx context.Context, id string) (Order, error)
	MarkRevoked(ctx context.Context, id string, revokingOrderID *string) (Order, error)
	ListOrdersByUnit(ctx context.Context, unitID, after string, limit int) ([]Order, error)
	ListOrdersByPerson(ctx context.Context, personID, after string, limit int) ([]Order, error)

	// order items (parent-scoped)
	InsertOrderItem(ctx context.Context, it OrderItem) (OrderItem, error)
	GetOrderItems(ctx context.Context, orderID string) ([]OrderItem, error)
	DeleteOrderItems(ctx context.Context, orderID string) error
}
