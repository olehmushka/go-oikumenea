package db

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"
)

// Querier is the command surface (DBTX) plus a transaction starter, satisfied by both *pgxpool.Pool
// and *pgxpool.Conn. A module's application service runs reads/writes on a Querier so the same code
// works against either the bare pool or the request-pinned, RLS-GUC-bearing connection (AcquireScoped).
type Querier interface {
	DBTX
	Begin(context.Context) (pgx.Tx, error)
}

// RLSState is the per-request authorization context the Postgres RLS backstop mirrors
// (D-RLSDefenseInDepth): the subject plus the PDP-computed read/write unit reach. It is applied to a
// pinned connection as the app.* session GUCs the migration-0012 policies read. The PDP + shadow gate
// remain authoritative; RLS only guards the forgotten-filter bug class.
type RLSState struct {
	PersonID        string
	IsInstanceAdmin bool
	ReadableUnits   []string // unit RIDs in the subject's read reach
	WritableUnits   []string // unit RIDs in the subject's write reach
}

type connKey struct{}

// WithConn returns a context carrying the request-pinned connection. The 4-table-touching modules'
// querier(ctx)/reader(ctx) accessors prefer it over the bare pool so their statements run under the
// RLS GUCs set on that connection.
func WithConn(ctx context.Context, conn *pgxpool.Conn) context.Context {
	return context.WithValue(ctx, connKey{}, conn)
}

// ConnFromContext returns the request-pinned connection, if any.
func ConnFromContext(ctx context.Context) (*pgxpool.Conn, bool) {
	c, ok := ctx.Value(connKey{}).(*pgxpool.Conn)
	return c, ok && c != nil
}

// AcquireScoped pins a pooled connection and sets the four app.* RLS GUCs on it from state (session
// scope, is_local=false). Statements on the returned connection are filtered by the RLS policies
// (migration 0012). The returned release func resets the GUCs and returns the connection to the pool;
// callers MUST defer it. Reset runs on a background context so it still fires when the request context
// has been cancelled.
func AcquireScoped(ctx context.Context, pool *pgxpool.Pool, state RLSState) (*pgxpool.Conn, func(), error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, nil, werror.WrapWithContextParams(ctx, err, "acquire rls-scoped connection")
	}
	if err := setRLSGUCs(ctx, conn, state); err != nil {
		conn.Release()
		return nil, nil, err
	}
	release := func() {
		_ = resetRLSGUCs(context.Background(), conn)
		conn.Release()
	}
	return conn, release, nil
}

// RunAsSystem pins a connection with the instance-admin GUC flag set (and empty unit reach) and runs
// fn with that connection in context. Trusted internal operations with no request subject — first-admin
// bootstrap (D-Bootstrap), the recover-admin CLI, and the person-purge crypto-erase subscriber — use
// this so their writes pass the RLS policies. It is the GUC equivalent of "this is a system action",
// never a DB superuser (the app role lacks BYPASSRLS — D-RLSDefenseInDepth).
func RunAsSystem(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	conn, release, err := AcquireScoped(ctx, pool, RLSState{IsInstanceAdmin: true})
	if err != nil {
		return err
	}
	defer release()
	return fn(WithConn(ctx, conn))
}

// rlsGUCNames is the canonical order/list of the four backstop GUCs.
var rlsGUCNames = [...]string{"app.person_id", "app.is_instance_admin", "app.readable_units", "app.writable_units"}

// setRLSGUCs sets the four app.* GUCs. Unit sets are comma-joined RID lists (RIDs contain no commas);
// the policies read them with string_to_array(current_setting(name, true), ','), so an unset GUC reads
// as NULL (no rows) rather than erroring.
func setRLSGUCs(ctx context.Context, conn *pgxpool.Conn, state RLSState) error {
	vals := [...]string{
		state.PersonID,
		boolGUC(state.IsInstanceAdmin),
		strings.Join(state.ReadableUnits, ","),
		strings.Join(state.WritableUnits, ","),
	}
	for i, name := range rlsGUCNames {
		if _, err := conn.Exec(ctx, "SELECT set_config($1, $2, false)", name, vals[i]); err != nil {
			return werror.WrapWithContextParams(ctx, err, "set rls guc", werror.SafeParam("guc", name))
		}
	}
	return nil
}

// resetRLSGUCs clears the four GUCs before the connection returns to the pool, so no later borrower
// inherits a prior subject's reach.
func resetRLSGUCs(ctx context.Context, conn *pgxpool.Conn) error {
	for _, name := range rlsGUCNames {
		if _, err := conn.Exec(ctx, "SELECT set_config($1, '', false)", name); err != nil {
			return werror.WrapWithContextParams(ctx, err, "reset rls guc", werror.SafeParam("guc", name))
		}
	}
	return nil
}

func boolGUC(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
