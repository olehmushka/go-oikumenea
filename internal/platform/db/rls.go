// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"

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

// RLSState is the per-request authorization context the Postgres RLS backstop reads
// (D-RLSDefenseInDepth as reshaped by D-RLSLiveReach): just the subject identity — the policies
// compute reach LIVE via oikumenea.authz_unit_in_reach(unit, wr), so no unit list ever crosses the
// wire (the old ReadableUnits/WritableUnits GUC payload was O(org) — multi-MB for a staff-level
// subject). Applied to a pinned connection as three app.* session GUCs in ONE round trip. The PDP +
// shadow gate remain authoritative; RLS only guards the forgotten-filter bug class.
//
// The subject is EITHER a person (PersonID, optionally IsInstanceAdmin) OR a machine principal
// (PrincipalID) — never both. A person request leaves PrincipalID "", a service request leaves
// PersonID/IsInstanceAdmin zero; the reach predicate's person and principal arms key on the
// respective GUC, so the unset one is an empty probe (M55 / D-ServiceIdentities RLS arm).
type RLSState struct {
	PersonID        string
	IsInstanceAdmin bool
	PrincipalID     string
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

// AcquireScoped pins a pooled connection and sets the three app.* RLS GUCs on it from state (session
// scope, is_local=false; one set_config round trip). Statements on the returned connection are
// filtered by the RLS policies (migration 0011). The returned release func resets the GUCs and
// returns the connection to the pool; callers MUST defer it. Reset runs on a background context so
// it still fires when the request context has been cancelled.
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

// setRLSGUCs sets the three app.* GUCs in ONE round trip (review-2026-07 R-03: the old four-GUC loop
// cost 4 round trips per acquire; the machine reach arm added app.principal_id in M55). The policies
// read them with current_setting(name, true), so an unset GUC reads as NULL (no rows) rather than
// erroring; nullif(”) maps the reset value to NULL.
func setRLSGUCs(ctx context.Context, conn *pgxpool.Conn, state RLSState) error {
	if _, err := conn.Exec(ctx,
		"SELECT set_config('app.person_id', $1, false), set_config('app.is_instance_admin', $2, false), set_config('app.principal_id', $3, false)",
		state.PersonID, boolGUC(state.IsInstanceAdmin), state.PrincipalID); err != nil {
		return werror.WrapWithContextParams(ctx, err, "set rls gucs")
	}
	return nil
}

// resetRLSGUCs clears the three GUCs (one round trip) before the connection returns to the pool, so
// no later borrower inherits a prior subject's identity.
func resetRLSGUCs(ctx context.Context, conn *pgxpool.Conn) error {
	if _, err := conn.Exec(ctx,
		"SELECT set_config('app.person_id', '', false), set_config('app.is_instance_admin', '', false), set_config('app.principal_id', '', false)"); err != nil {
		return werror.WrapWithContextParams(ctx, err, "reset rls gucs")
	}
	return nil
}

func boolGUC(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
