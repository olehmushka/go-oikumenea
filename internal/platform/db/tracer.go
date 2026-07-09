// Query-count tracing (review-2026-07 Phase 0 measurement harness). A pgx.QueryTracer is installed
// on every pool NewPool constructs; it is a no-op (one context lookup) unless a *QueryCounter has
// been attached to the context with WithQueryCounter. Integration tests use it to assert per-request
// statement budgets (e.g. "exactly one grants fetch per guarded request" — review R-01 acceptance).
package db

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
)

// QueryCounter accumulates the statements executed under a context subtree.
type QueryCounter struct {
	n       atomic.Int64
	capture atomic.Bool
	mu      sync.Mutex
	sqls    []string
}

// Count returns the number of statements traced so far.
func (c *QueryCounter) Count() int64 { return c.n.Load() }

// CaptureSQL enables recording of statement text (off by default — counting alone is lock-free).
func (c *QueryCounter) CaptureSQL() { c.capture.Store(true) }

// Statements returns a copy of the captured statement texts (empty unless CaptureSQL was called
// before the traced work ran).
func (c *QueryCounter) Statements() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.sqls))
	copy(out, c.sqls)
	return out
}

// CountContaining returns how many captured statements contain the given substring
// (case-insensitive). Requires CaptureSQL.
func (c *QueryCounter) CountContaining(substr string) int {
	needle := strings.ToLower(substr)
	n := 0
	for _, s := range c.Statements() {
		if strings.Contains(strings.ToLower(s), needle) {
			n++
		}
	}
	return n
}

func (c *QueryCounter) record(sql string) {
	c.n.Add(1)
	if !c.capture.Load() {
		return
	}
	c.mu.Lock()
	c.sqls = append(c.sqls, sql)
	c.mu.Unlock()
}

type queryCounterKey struct{}

// WithQueryCounter attaches a fresh QueryCounter to the context; every statement pgx executes under
// the returned context (on a pool built by NewPool) is counted.
func WithQueryCounter(ctx context.Context) (context.Context, *QueryCounter) {
	c := &QueryCounter{}
	return context.WithValue(ctx, queryCounterKey{}, c), c
}

// testingTB is the sliver of *testing.T AssertQueryCount needs — declared locally so this
// production package does not import "testing".
type testingTB interface {
	Helper()
	Errorf(format string, args ...any)
}

// AssertQueryCount fails the test unless exactly want captured statements contain sqlSubstring.
// The counter must have had CaptureSQL enabled before the traced work ran.
func AssertQueryCount(t testingTB, c *QueryCounter, sqlSubstring string, want int) {
	t.Helper()
	got := c.CountContaining(sqlSubstring)
	if got != want {
		t.Errorf("query count for %q: got %d, want %d\ntraced statements:\n  %s",
			sqlSubstring, got, want, strings.Join(c.Statements(), "\n  "))
	}
}

// queryTracer implements pgx.QueryTracer; it feeds any QueryCounter found on the query context.
type queryTracer struct{}

func (queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if c, ok := ctx.Value(queryCounterKey{}).(*QueryCounter); ok {
		c.record(d.SQL)
	}
	return ctx
}

func (queryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
