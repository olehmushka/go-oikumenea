// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Outbox multi-replica self-test seam (architecture review R-13, Phase 8). The transactional outbox
// (D-EventOutbox) is a live-but-empty seam — every domain event is `atomic`, so there are no notify
// producers/handlers to exercise it across replicas. This file adds a tiny, ENV-GATED notify producer
// + counting handler so the scripts/scale-e2e.sh two-replica compose scenario can prove the outbox's
// cross-process properties: exactly-once effect under at-least-once delivery, both replicas draining a
// shared queue (FOR UPDATE SKIP LOCKED), and kill -9 redelivery on the survivor.
//
// Everything here is inert unless OIKUMENEA_OUTBOX_SELFTEST=1 (precedent: OIKUMENEA_LANG_E2E). Off by
// default, a normal deployment sees no extra table, handler, or subcommand — there is no schema
// migration and no ExpectedSchemaRevision bump. The deliveries table is created lazily under the flag.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/platform/outbox"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/palantir/witchcraft-go-logging/wlog"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

const (
	outboxSelftestEnv       = "OIKUMENEA_OUTBOX_SELFTEST"
	outboxSelftestEventType = "platform.selftest"
)

// outboxSelftestEnabled reports whether the env-gated self-test seam is active.
func outboxSelftestEnabled() bool { return os.Getenv(outboxSelftestEnv) == "1" }

// selftestEvent is the notify event the enqueue subcommand publishes and the handler consumes. ID is a
// unique key so the handler's ON CONFLICT dedup turns at-least-once delivery into an observable
// exactly-once effect (a redelivered row inserts nothing new).
type selftestEvent struct {
	ID string `json:"id"`
}

func (selftestEvent) Type() string { return outboxSelftestEventType }

// registerOutboxSelftest wires the counting handler onto the dispatcher (before Seal) when the seam is
// enabled. On each delivered event it records WHICH replica (os.Hostname = the container id) drained it
// into oikumenea.platform_outbox_selftest_deliveries — so the scenario can assert both replicas
// participated (distinct replica count == 2) and the exactly-once effect (row count == enqueued count).
// A no-op when the flag is unset, so the composition root can call it unconditionally.
//
// The deliveries table is provisioned out of band by the verification harness as the migration
// owner/superuser (scripts/scale-e2e.sh), NOT here: the server connects as the non-superuser app role
// (D-RLSDefenseInDepth), which has no CREATE privilege. The handler only INSERTs (granted to the app
// role), and only at dispatch time, so registration needs no table — keeping the prod schema untouched
// (no migration, no ExpectedSchemaRevision bump).
func registerOutboxSelftest(ctx context.Context, pool *pgxpool.Pool, dispatcher *outbox.Dispatcher) {
	if !outboxSelftestEnabled() {
		return
	}
	replica, _ := os.Hostname()
	dispatcher.Register(outboxSelftestEventType, func(ctx context.Context, payload []byte) error {
		var evt selftestEvent
		if err := json.Unmarshal(payload, &evt); err != nil {
			return err
		}
		// Idempotent: a redelivery (crash after handler, before the row was marked dispatched) inserts
		// nothing, keeping the deliveries count at exactly the number of enqueued events.
		_, err := pool.Exec(ctx,
			`INSERT INTO oikumenea.platform_outbox_selftest_deliveries (event_id, replica)
			 VALUES ($1, $2) ON CONFLICT (event_id) DO NOTHING`,
			evt.ID, replica)
		return err
	})
	svc1log.FromContext(ctx).Info("outbox self-test seam enabled (R-13 scale verification)",
		svc1log.SafeParam("replica", replica), svc1log.SafeParam("eventType", outboxSelftestEventType))
}

// runOutboxSelftestEnqueue is the `outbox-selftest-enqueue` subcommand (sibling of `seed`/`recover-admin`).
// It enqueues --n notify events onto the shared outbox via the standard OutboxWriter (each on its own
// transaction), so the two replicas' dispatchers drain them concurrently. Refuses unless the seam is
// enabled — it is a verification tool, not an operator command.
func runOutboxSelftestEnqueue(args []string) int {
	const cmd = "outbox-selftest-enqueue"
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var configPath string
	var n int
	fs.StringVar(&configPath, "config", "var/conf/install.yml", "path to the install config")
	fs.IntVar(&n, "n", 100, "number of self-test notify events to enqueue")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !outboxSelftestEnabled() {
		fmt.Fprintf(os.Stderr, "%s: refusing — set %s=1 to use the self-test seam\n", cmd, outboxSelftestEnv)
		return 1
	}
	if n <= 0 {
		fmt.Fprintf(os.Stderr, "%s: --n must be positive\n", cmd)
		return 2
	}

	install, err := loadInstall(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: load install config: %v\n", cmd, err)
		return 1
	}
	ctx := svc1log.WithLogger(context.Background(), svc1log.New(os.Stderr, wlog.InfoLevel))
	pool, err := db.NewPool(ctx, install.Postgres.DSN, install.Environment)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: connect database: %v\n", cmd, err)
		return 1
	}
	defer pool.Close()

	var writer events.OutboxWriter
	for i := 0; i < n; i++ {
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			return writer.PublishNotify(ctx, tx, selftestEvent{ID: uuid.NewString()})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "%s: enqueue #%d: %v\n", cmd, i, err)
			return 1
		}
	}
	fmt.Fprintf(os.Stdout, "%s: enqueued %d %q events\n", cmd, n, outboxSelftestEventType)
	return 0
}
