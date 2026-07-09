//go:build integration

// Integration tests for the transactional outbox (review-2026-07 R-10 / D-EventOutbox). They prove the
// three load-bearing properties: (1) a notify event enqueued on a transaction is durable across commit
// independent of dispatch (the "crash between commit and dispatch" case — the row is simply still
// there); (2) the dispatcher delivers it to the registered handler and marks it dispatched; (3) a failing
// handler leaves the row pending and it is REDELIVERED (at-least-once) until it succeeds, then
// dead-letters past max attempts. Requires OIKUMENEA_TEST_DSN (the migrated oikumenea_test DB).
package outbox_test

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/platform/outbox"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testEvent is a stand-in notify event (there are no real notify producers yet — every domain event is
// atomic today). It is JSON-marshaled by the OutboxWriter and unmarshaled by the handler.
type testEvent struct {
	PersonID string `json:"personId"`
	N        int    `json:"n"`
}

func (testEvent) Type() string { return "test.notify" }

func testDSN() string { return os.Getenv("OIKUMENEA_TEST_DSN") }

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testDSN()
	if dsn == "" {
		t.Skip("no test database (set OIKUMENEA_TEST_DSN)")
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	return pool
}

func truncateOutbox(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `TRUNCATE oikumenea.platform_outbox`); err != nil {
		t.Fatalf("truncate outbox: %v", err)
	}
}

// enqueue publishes evt inside its own transaction and commits — mirroring a producer that enqueues a
// notify event atomically with its write.
func enqueue(ctx context.Context, t *testing.T, pool *pgxpool.Pool, evt events.Event) {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := (events.OutboxWriter{}).PublishNotify(ctx, tx, evt); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("publish notify: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func rowState(ctx context.Context, t *testing.T, pool *pgxpool.Pool) (status string, attempts int, dispatched bool) {
	t.Helper()
	var dispatchedAt *time.Time
	err := pool.QueryRow(ctx, `SELECT status, attempts, dispatched_at FROM oikumenea.platform_outbox LIMIT 1`).
		Scan(&status, &attempts, &dispatchedAt)
	if err != nil {
		t.Fatalf("read row state: %v", err)
	}
	return status, attempts, dispatchedAt != nil
}

func TestOutboxDurableAcrossCommitThenDelivered(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	defer pool.Close()
	truncateOutbox(ctx, t, pool)

	enqueue(ctx, t, pool, testEvent{PersonID: "p-1", N: 7})

	// (1) Durability across the commit/dispatch gap: with NO dispatcher run yet, the row is committed and
	// pending — a process crash here loses nothing; the next dispatcher pass will pick it up.
	if status, attempts, dispatched := rowState(ctx, t, pool); status != "pending" || attempts != 0 || dispatched {
		t.Fatalf("after commit, before dispatch: status=%q attempts=%d dispatched=%v; want pending/0/false", status, attempts, dispatched)
	}

	// (2) The dispatcher delivers to the registered handler and marks the row dispatched.
	var mu sync.Mutex
	var got []testEvent
	d := outbox.New(pool, outbox.Config{})
	d.Register(testEvent{}.Type(), func(_ context.Context, payload []byte) error {
		var e testEvent
		if err := json.Unmarshal(payload, &e); err != nil {
			return err
		}
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
		return nil
	})

	n, err := d.DispatchOnce(ctx)
	if err != nil {
		t.Fatalf("DispatchOnce: %v", err)
	}
	if n != 1 {
		t.Fatalf("processed = %d, want 1", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].PersonID != "p-1" || got[0].N != 7 {
		t.Fatalf("handler saw %+v, want one {p-1 7}", got)
	}
	if status, attempts, dispatched := rowState(ctx, t, pool); status != "dispatched" || attempts != 1 || !dispatched {
		t.Fatalf("after delivery: status=%q attempts=%d dispatched=%v; want dispatched/1/true", status, attempts, dispatched)
	}

	// A second pass has nothing to do (no pending rows) — proving dispatched rows are not re-delivered.
	if n, err := d.DispatchOnce(ctx); err != nil || n != 0 {
		t.Fatalf("second pass processed=%d err=%v, want 0/nil", n, err)
	}
}

func TestOutboxRedeliversOnHandlerFailure(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	defer pool.Close()
	truncateOutbox(ctx, t, pool)

	enqueue(ctx, t, pool, testEvent{PersonID: "p-2", N: 1})

	var calls int
	// Large backoff so a failed row is NOT re-claimed within the same drain pass — we then simulate the
	// backoff elapsing by moving next_attempt_at to now(), making redelivery deterministic (no sleeping).
	d := outbox.New(pool, outbox.Config{BackoffBase: time.Hour, BackoffMax: time.Hour})
	d.Register(testEvent{}.Type(), func(_ context.Context, _ []byte) error {
		calls++
		if calls == 1 {
			return errFail // fail the first delivery
		}
		return nil // succeed the second
	})

	// First pass: handler fails -> row stays pending, attempts=1, next_attempt_at ~1h out (backoff).
	if n, err := d.DispatchOnce(ctx); err != nil || n != 1 {
		t.Fatalf("first pass processed=%d err=%v, want 1/nil", n, err)
	}
	if status, attempts, _ := rowState(ctx, t, pool); status != "pending" || attempts != 1 {
		t.Fatalf("after failed delivery: status=%q attempts=%d; want pending/1", status, attempts)
	}

	// Backoff holds it: an immediate pass finds nothing due (proves the retry is scheduled, not spun on).
	if n, err := d.DispatchOnce(ctx); err != nil || n != 0 {
		t.Fatalf("during backoff processed=%d err=%v, want 0/nil (retry not yet due)", n, err)
	}

	// Simulate the backoff window elapsing.
	if _, err := pool.Exec(ctx, `UPDATE oikumenea.platform_outbox SET next_attempt_at = now()`); err != nil {
		t.Fatalf("advance next_attempt_at: %v", err)
	}

	// Redelivery: handler now succeeds -> marked dispatched (at-least-once contract).
	if n, err := d.DispatchOnce(ctx); err != nil || n != 1 {
		t.Fatalf("redelivery pass processed=%d err=%v, want 1/nil", n, err)
	}
	if status, attempts, dispatched := rowState(ctx, t, pool); status != "dispatched" || attempts != 2 || !dispatched {
		t.Fatalf("after redelivery: status=%q attempts=%d dispatched=%v; want dispatched/2/true", status, attempts, dispatched)
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}
}

func TestOutboxNoHandlerDrains(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	defer pool.Close()
	truncateOutbox(ctx, t, pool)

	enqueue(ctx, t, pool, testEvent{PersonID: "p-3", N: 0})

	// A notify event nobody consumes is a no-op success (drained, not stuck) — matching the atomic bus's
	// no-subscriber semantics.
	d := outbox.New(pool, outbox.Config{})
	if n, err := d.DispatchOnce(ctx); err != nil || n != 1 {
		t.Fatalf("processed=%d err=%v, want 1/nil", n, err)
	}
	if status, _, dispatched := rowState(ctx, t, pool); status != "dispatched" || !dispatched {
		t.Fatalf("unconsumed event: status=%q dispatched=%v; want dispatched/true", status, dispatched)
	}
}

type failErr struct{}

func (failErr) Error() string { return "handler failed (test)" }

var errFail = failErr{}
