// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package db builds hermenea's own pgx connection pool (M16 / D-Hermenea). Hermenea's database is
// separate from oikumenea's and has none of oikumenea's RID/RLS machinery, so this is a plain pool
// (no per-connection GUC wiring).
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	werror "github.com/palantir/witchcraft-go-error"
)

// NewPool opens hermenea's connection pool from a DSN.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "parse hermenea postgres dsn")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, werror.WrapWithContextParams(ctx, err, "open hermenea postgres pool")
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, werror.WrapWithContextParams(ctx, err, "ping hermenea postgres")
	}
	return pool, nil
}
