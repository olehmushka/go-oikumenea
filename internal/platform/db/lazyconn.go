// Lazy RLS connection pinning (review-2026-07 R-03). Before this, the authenticator pinned a
// pooled connection for EVERY authenticated request for its full duration — including requests
// that never touch an RLS-guarded table and across external egress — so in-flight requests were
// capped at the pool size. Now the authenticator installs a lazy holder (WithLazyConn); the
// connection is acquired, GUC-scoped (one set_config round trip) and pinned only when a handler
// first runs a statement through an RLS-consuming module's querier (RequestQuerier), and released
// on request end only if it was ever acquired.
package db

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"
)

// lazyConn defers AcquireScoped-style pinning until the first RLS-scoped statement of the request.
type lazyConn struct {
	pool  *pgxpool.Pool
	state RLSState

	mu   sync.Mutex
	conn *pgxpool.Conn
	err  error // sticky: an acquire failure fails all of the request's scoped DB work deterministically
	done bool  // released; further use is a bug surfaced as an error, not a stale-conn reuse
}

// get acquires + GUC-scopes the connection on first use. The sticky error means a request whose
// acquire failed gets the same error on every statement (→ one clean 500), never a half-scoped conn.
func (l *lazyConn) get(ctx context.Context) (*pgxpool.Conn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return nil, l.err
	}
	if l.done {
		return nil, werror.ErrorWithContextParams(ctx, "rls connection used after request release")
	}
	if l.conn != nil {
		return l.conn, nil
	}
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		l.err = werror.WrapWithContextParams(ctx, err, "acquire rls-scoped connection")
		return nil, l.err
	}
	if err := setRLSGUCs(ctx, conn, l.state); err != nil {
		conn.Release()
		l.err = err
		return nil, l.err
	}
	l.conn = conn
	return l.conn, nil
}

// release resets the GUCs (background context — must fire even on a cancelled request) and returns
// the connection to the pool, iff it was ever acquired.
func (l *lazyConn) release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.done = true
	if l.conn == nil {
		return
	}
	_ = resetRLSGUCs(context.Background(), l.conn)
	l.conn.Release()
	l.conn = nil
}

type lazyConnKey struct{}

// WithLazyConn installs a lazy RLS-scoped connection holder for the request. The authenticator
// calls it once per authenticated request and defers the returned release; a request that never
// touches an RLS-consuming module costs zero pool time and zero GUC round trips.
func WithLazyConn(ctx context.Context, pool *pgxpool.Pool, state RLSState) (context.Context, func()) {
	l := &lazyConn{pool: pool, state: state}
	return context.WithValue(ctx, lazyConnKey{}, l), l.release
}

// RequestQuerier is what an RLS-consuming module's querier(ctx)/reader(ctx) accessor returns:
// an explicitly pinned connection wins (WithConn — the RunAsSystem/system path), else the request's
// lazy holder (acquired on first statement), else the module's own fallback (the bare pool —
// out-of-request work: boot seeds, subscribers, tests).
func RequestQuerier(ctx context.Context, fallback Querier) Querier {
	if c, ok := ConnFromContext(ctx); ok {
		return c
	}
	if l, ok := ctx.Value(lazyConnKey{}).(*lazyConn); ok {
		return lazyQuerier{l}
	}
	return fallback
}

// RequestDBTX is RequestQuerier for read-only accessors whose fallback is a bare DBTX (audit's
// reader): same precedence, no transaction surface.
func RequestDBTX(ctx context.Context, fallback DBTX) DBTX {
	if c, ok := ConnFromContext(ctx); ok {
		return c
	}
	if l, ok := ctx.Value(lazyConnKey{}).(*lazyConn); ok {
		return lazyQuerier{l}
	}
	return fallback
}

// lazyQuerier adapts the lazy holder to the Querier surface: every call acquires-once and delegates.
type lazyQuerier struct{ l *lazyConn }

func (q lazyQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	conn, err := q.l.get(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return conn.Exec(ctx, sql, args...)
}

func (q lazyQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	conn, err := q.l.get(ctx)
	if err != nil {
		return nil, err
	}
	return conn.Query(ctx, sql, args...)
}

func (q lazyQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	conn, err := q.l.get(ctx)
	if err != nil {
		return errRow{err} // pgx.Row has no error return; surface it from Scan
	}
	return conn.QueryRow(ctx, sql, args...)
}

func (q lazyQuerier) Begin(ctx context.Context) (pgx.Tx, error) {
	conn, err := q.l.get(ctx)
	if err != nil {
		return nil, err
	}
	return conn.Begin(ctx)
}

// errRow is a pgx.Row that fails with the acquire error on Scan.
type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }
