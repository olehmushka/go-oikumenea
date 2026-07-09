// Package outbox is the after-commit half of the transactional outbox (review-2026-07 R-10 /
// D-EventOutbox): it drains oikumenea.platform_outbox (migration 0036) and delivers each `notify`-class
// event to its registered handlers OUT OF PROCESS, after the producing transaction has committed. The
// producer side is pkg/events.OutboxWriter (enqueue on the write tx); this side polls, claims rows FOR
// UPDATE SKIP LOCKED (so multiple replicas share the queue safely, like the hermenea worker, R-13), runs
// handlers, and marks each row dispatched / retried-with-backoff / dead-lettered.
//
// Delivery is AT LEAST ONCE: a crash after the handler runs but before the row is marked re-delivers the
// event, so handlers must be idempotent. Today every domain event is `atomic` (delivered inside the
// publisher's tx by pkg/events.Bus), so no notify handlers are registered yet — the dispatcher runs
// live over an empty queue as a proven seam.
package outbox

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// Handler processes one delivered notify event, after commit and out of band (no publisher transaction).
// It receives the raw JSON payload the producer marshaled and must be idempotent (at-least-once
// delivery). Returning an error reschedules the event with backoff; exceeding MaxAttempts dead-letters it.
type Handler func(ctx context.Context, payload []byte) error

// Config tunes the poll loop and retry policy (mirrors internal/hermenea/runtime.Config). Zero values
// get sensible defaults in New.
type Config struct {
	PollInterval time.Duration // queue poll cadence when idle
	BatchSize    int           // max rows drained per DispatchOnce pass
	BackoffBase  time.Duration // first-retry delay (doubled each attempt)
	BackoffMax   time.Duration // backoff cap
}

func (c Config) withDefaults() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = 2 * time.Second
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.BackoffBase <= 0 {
		c.BackoffBase = 5 * time.Second
	}
	if c.BackoffMax <= 0 {
		c.BackoffMax = 5 * time.Minute
	}
	return c
}

// Dispatcher owns the poll goroutine and the handler registry.
type Dispatcher struct {
	pool *pgxpool.Pool
	cfg  Config

	mu       sync.Mutex
	handlers map[string][]Handler
	sealed   bool
}

// New builds a dispatcher over pool. Register handlers before Start/Seal.
func New(pool *pgxpool.Pool, cfg Config) *Dispatcher {
	return &Dispatcher{pool: pool, cfg: cfg.withDefaults(), handlers: make(map[string][]Handler)}
}

// Register wires a handler for a notify event type. Registering after Seal panics (R-10): notify
// consumers, like atomic subscribers, are wired once at boot before serving.
func (d *Dispatcher) Register(eventType string, h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sealed {
		panic("outbox: Register(" + eventType + ") after the dispatcher was sealed — notify consumers must be wired at boot")
	}
	d.handlers[eventType] = append(d.handlers[eventType], h)
}

// Seal freezes the handler set. The composition root calls it once, after wiring, before serving.
func (d *Dispatcher) Seal() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sealed = true
}

func (d *Dispatcher) handlersFor(eventType string) []Handler {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.handlers[eventType]
}

// Start launches the poll goroutine bound to ctx and returns a Stop that cancels it and waits for the
// in-flight pass to finish. Wire Stop into the composition root's cleanup.
func (d *Dispatcher) Start(ctx context.Context) (stop func()) {
	loopCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.loop(loopCtx)
	}()
	return func() {
		cancel()
		wg.Wait()
	}
}

// loop drains the queue whenever there is work and otherwise waits a poll interval.
func (d *Dispatcher) loop(ctx context.Context) {
	logger := svc1log.FromContext(ctx)
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		n, err := d.DispatchOnce(ctx)
		if err != nil {
			logger.Warn("outbox dispatcher: pass failed", svc1log.Stacktrace(err))
		} else if n == d.cfg.BatchSize {
			continue // queue may still hold work — drain without waiting
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// DispatchOnce claims and processes up to BatchSize due rows and returns the number processed (delivered
// or failed-and-rescheduled). It is exported so tests (and an ops trigger) can drive delivery
// deterministically. Each row is handled in its own transaction so the claim lock is held only for that
// row's handlers, and a handler failure rolls back only its own status write.
func (d *Dispatcher) DispatchOnce(ctx context.Context) (int, error) {
	processed := 0
	for processed < d.cfg.BatchSize {
		ok, err := d.dispatchRow(ctx)
		if err != nil {
			return processed, err
		}
		if !ok {
			break // queue drained
		}
		processed++
	}
	return processed, nil
}

// dispatchRow claims one due pending row FOR UPDATE SKIP LOCKED, runs its handlers, and records the
// outcome — all in one transaction. Returns ok=false when no row was available.
func (d *Dispatcher) dispatchRow(ctx context.Context) (bool, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	var (
		id          string
		eventType   string
		payload     []byte
		attempts    int
		maxAttempts int
	)
	err = tx.QueryRow(ctx, `
		SELECT id, event_type, payload, attempts, max_attempts
		FROM oikumenea.platform_outbox
		WHERE status = 'pending' AND next_attempt_at <= now()
		ORDER BY next_attempt_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&id, &eventType, &payload, &attempts, &maxAttempts)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	deliverErr := d.deliver(ctx, eventType, payload)
	if deliverErr == nil {
		if _, err = tx.Exec(ctx, `
			UPDATE oikumenea.platform_outbox
			SET status = 'dispatched', attempts = attempts + 1, dispatched_at = now(), last_error = NULL
			WHERE id = $1`, id); err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}

	nextAttempts := attempts + 1
	if nextAttempts >= maxAttempts {
		if _, err = tx.Exec(ctx, `
			UPDATE oikumenea.platform_outbox
			SET status = 'dead', attempts = $2, last_error = $3
			WHERE id = $1`, id, nextAttempts, deliverErr.Error()); err != nil {
			return false, err
		}
		return true, tx.Commit(ctx)
	}

	nextAt := time.Now().Add(backoff(attempts, d.cfg.BackoffBase, d.cfg.BackoffMax))
	if _, err = tx.Exec(ctx, `
		UPDATE oikumenea.platform_outbox
		SET attempts = $2, next_attempt_at = $3, last_error = $4
		WHERE id = $1`, id, nextAttempts, nextAt, deliverErr.Error()); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// deliver runs every handler registered for eventType. The first handler error fails the delivery (the
// row is rescheduled); an event with no handlers is delivered successfully (drained, not stuck) — a
// notify event nobody consumes is a no-op, matching the atomic bus's no-subscriber semantics.
func (d *Dispatcher) deliver(ctx context.Context, eventType string, payload []byte) error {
	for _, h := range d.handlersFor(eventType) {
		if err := h(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

// backoff returns base*2^attempt capped at max (attempt is the count of prior attempts).
func backoff(attempt int, base, max time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	if d > max {
		return max
	}
	return d
}
