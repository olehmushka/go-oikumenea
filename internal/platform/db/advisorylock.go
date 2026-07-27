// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Advisory-lock helper (R-13 / M49): cross-process mutual exclusion for the boot-time seeding paths
// (pinax autoseed + first-admin bootstrap). Those paths are idempotent but not race-free — two fresh
// replicas booting the same empty database would both run the full seed (wasted duplicate work, lock
// contention, constraint-violation noise) or both pass the "no admin yet" check. A session-level
// pg_advisory_lock on a well-known key serializes them: the second replica WAITS, then re-runs the
// now-no-op idempotent checks. Session-level (not xact) because the seed spans multiple transactions.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LockBootSeed serializes the oikumenea boot-time seeding section (pinax autoseed + first-admin
// bootstrap) across replicas. The value is arbitrary but must be stable and unique within this
// database ("oik-boot" as a number).
const LockBootSeed int64 = 0x01_6F_69_6B_B0_07_5E_ED

// WithAdvisoryLock runs fn while holding the session-level advisory lock `key`, taken on a dedicated
// pooled connection (session locks belong to a connection; the pool must not recycle it mid-fn). It
// BLOCKS until the lock is granted — for boot seeding that is the point: the loser waits for the
// winner's seed to finish, then finds everything already seeded. The lock is released (and the
// connection returned) when fn returns.
func WithAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, key int64, fn func(context.Context) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		return err
	}
	defer func() {
		// Best-effort unlock; if it fails the session is broken and the lock dies with the connection.
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", key)
	}()
	return fn(ctx)
}
