// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// NotifyPublisher enqueues a `notify`-class event into the transactional outbox on the caller's
// transaction (docs/architecture/patterns.md). The row commits atomically with the originating write,
// but is delivered AFTER COMMIT — at least once, out of process — by the dispatcher
// (internal/platform/outbox). This is the seam for effects that must NOT widen the write transaction
// (webhooks, projections, cache invalidation, the R-01 grant-cache epoch bump).
//
// Contrast the atomic Bus in this package, whose subscribers run INSIDE the publisher's transaction
// (all-or-nothing). Adding an atomic subscriber widens every publisher's transaction and is a
// decision-level change; a notify effect goes here instead. Today every domain event is `atomic`, so
// there are no notify producers yet — the outbox is a live-but-empty seam.
type NotifyPublisher interface {
	// PublishNotify writes evt to the outbox on tx. It does not deliver — the dispatcher does, after the
	// tx commits. Returns an error only if marshaling or the INSERT fails (which aborts the caller's tx).
	PublishNotify(ctx context.Context, tx pgx.Tx, evt Event) error
}

// OutboxWriter is the default NotifyPublisher: it marshals the concrete event to JSON and inserts one
// `pending` row into oikumenea.platform_outbox on the supplied transaction (migration 0036). The
// dispatcher unmarshals the payload back into the handler's expected type.
type OutboxWriter struct{}

// PublishNotify implements NotifyPublisher.
func (OutboxWriter) PublishNotify(ctx context.Context, tx pgx.Tx, evt Event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO oikumenea.platform_outbox (event_type, payload) VALUES ($1, $2)`,
		evt.Type(), payload)
	return err
}
