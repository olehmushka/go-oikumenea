// Package orderevents holds the order module's intent events (order.md / D-OrderApply): the
// effect-typed events order.issue publishes, one per order item, that membership/person subscribers
// apply IN THE ISSUE TRANSACTION. They are named *Ordered (intent) to stay distinct from each
// module's own fact events (MembershipCreated, PersonRankChanged) — no collision, no loop.
//
// This package is a leaf: it imports only pkg/events (for the Event interface) and the standard
// library, so the producer (order/application) and the consumers (membership/person application) can
// all depend on it without an import cycle — order never imports membership/person, and these event
// structs reference person/unit/position/rank by their string RIDs only.
package orderevents

import (
	"time"

	"github.com/olegamysk/go-oikumenea/pkg/events"
)

// Event-type dispatch keys (events.Event.Type()).
const (
	TypeOrderIssued        = "order.issued"
	TypeOrderRevoked       = "order.revoked"
	TypeAppointmentOrdered = "order.appointment-ordered"
	TypeRemovalOrdered     = "order.removal-ordered"
	TypeRankChangeOrdered  = "order.rank-change-ordered"
)

// OrderIssued is the umbrella event emitted once when an order is issued. No module subscribes today
// (the granular per-item events carry the effects); it exists for symmetry and future consumers.
type OrderIssued struct {
	OrderID       string
	IssuingUnitID string
}

// Type implements events.Event.
func (OrderIssued) Type() string { return TypeOrderIssued }

// OrderRevoked is emitted once when an issued order is revoked (a legal-status flip; effects are not
// auto-reversed — undo is the revoking order's own items).
type OrderRevoked struct {
	OrderID string
}

// Type implements events.Event.
func (OrderRevoked) Type() string { return TypeOrderRevoked }

// AppointmentOrdered (effect=membership-start) → membership creates the membership (fills the position
// / plain belonging) citing OrderItemID as provenance, then emits its own MembershipCreated.
type AppointmentOrdered struct {
	OrderItemID   string
	PersonID      string
	UnitID        string // target unit; for a plain belonging when no position is named
	PositionID    string // target billet; empty for a plain belonging
	EffectiveFrom time.Time
}

// Type implements events.Event.
func (AppointmentOrdered) Type() string { return TypeAppointmentOrdered }

// RemovalOrdered (effect=membership-end) → membership ends the person's active membership in the
// target unit (or the one filling the target position), citing the order item.
type RemovalOrdered struct {
	OrderItemID string
	PersonID    string
	UnitID      string // narrows to the membership in this unit
	PositionID  string // when set, ends the filling of this billet
	EffectiveTo time.Time
}

// Type implements events.Event.
func (RemovalOrdered) Type() string { return TypeRemovalOrdered }

// RankChangeOrdered (effect=rank-change) → person sets rank_id, then emits PersonRankChanged. Rank is
// a person COLUMN (no provenance FK), so OrderItemID is carried into the audit payload only.
type RankChangeOrdered struct {
	OrderItemID string
	PersonID    string
	RankID      string
}

// Type implements events.Event.
func (RankChangeOrdered) Type() string { return TypeRankChangeOrdered }

// compile-time assertions that each event satisfies the bus contract.
var (
	_ events.Event = OrderIssued{}
	_ events.Event = OrderRevoked{}
	_ events.Event = AppointmentOrdered{}
	_ events.Event = RemovalOrdered{}
	_ events.Event = RankChangeOrdered{}
)
