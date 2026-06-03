// Package db owns the pgx connection pool and the boot-time schema-version check
// (docs/modules/platform.md). Domain modules receive the pool / a pgx.Tx and control their own
// transaction boundaries; platform provides the plumbing, not the queries.
package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"
)

// NewPool constructs the pgx pool against the operator DSN. Every pooled connection has the
// app.environment GUC set so oikumenea.new_rid() can compose RIDs (D-ResourceIdentifiers).
//
// The per-transaction RLS GUC seam (app.person_id / app.is_instance_admin / app.readable_units /
// app.writable_units — D-RLSDefenseInDepth) is deferred to the M0 follow-up; it layers onto this
// pool without changing its signature.
func NewPool(ctx context.Context, dsn, environment string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "parse postgres dsn")
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// SET cannot bind parameters; set_config does. Session-scoped (is_local = false).
		if _, err := conn.Exec(ctx, "SELECT set_config('app.environment', $1, false)", environment); err != nil {
			return werror.WrapWithContextParams(ctx, err, "set app.environment GUC")
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "create postgres pool")
	}
	return pool, nil
}
