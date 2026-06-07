// Package events is the in-process domain-event bus (platform.md). It dispatches domain events to
// registered subscribers SYNCHRONOUSLY, within the publisher's transaction: a handler receives the
// active pgx.Tx and does its work on it, so the publisher's write and every effect share one fate
// (all-or-nothing). The first handler error stops dispatch and is returned to the publisher, which
// rolls the transaction back.
//
// This is the mechanism behind the locked "cross-module mutations flow through domain events" rule
// (overview.md) WITHOUT a broker or a background worker (DS-25/DS-26 stay parked): same-transaction
// dispatch is just an in-process call chain inside one transaction. The outbox/at-least-once seam
// (publish-after-commit for out-of-band consumers) is a later addition; today every subscriber is
// transactional. D-OrderApply is the first user: order.issue publishes per-item effect events that
// membership/person subscribers apply in the issue transaction.
//
// The bus carries no domain types of its own — concrete events live with their producing module (e.g.
// internal/order/events). Subscribe wiring happens at composition time (main.go), before serving.
package events

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

// Event is a domain event. Its Type() is the dispatch key subscribers register against; the concrete
// struct carries the payload the handler type-asserts back to.
type Event interface {
	// Type returns the stable event-type key (e.g. "order.appointment-ordered").
	Type() string
}

// Handler reacts to an event within the publisher's transaction. It MUST use the supplied tx for any
// database work so the effect commits iff the originating action commits; returning an error aborts
// the whole publish (and, by the publisher's contract, the transaction).
type Handler func(ctx context.Context, tx pgx.Tx, evt Event) error

// Bus is an in-process synchronous event bus. It is safe for concurrent Publish once subscriptions are
// in place; subscriptions are expected to be registered at boot (single-threaded) before serving.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewBus returns an empty bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

// Subscribe registers a handler for an event type. Multiple handlers for one type run in registration
// order; a handler should be registered exactly once per (type, module) at composition time.
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish dispatches evt to every handler registered for evt.Type(), synchronously and in order,
// passing the active transaction. It returns the first handler error (stopping dispatch) so the
// publisher can roll back; an event with no subscribers is a no-op success.
func (b *Bus) Publish(ctx context.Context, tx pgx.Tx, evt Event) error {
	b.mu.RLock()
	hs := b.handlers[evt.Type()]
	b.mu.RUnlock()
	for _, h := range hs {
		if err := h(ctx, tx, evt); err != nil {
			return err
		}
	}
	return nil
}
