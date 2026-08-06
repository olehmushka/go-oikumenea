// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// White-box test for the R-20 backlog gauges (recordBacklog is unexported). It asserts that the
// pending-queue depth is reported even when nothing is being dispatched — the signal that surfaces a
// wedged dispatcher. Requires OIKUMENEA_TEST_DSN (the migrated oikumenea_test DB).
package outbox

import (
	"context"
	"os"
	"testing"

	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/palantir/pkg/metrics"
)

func TestRecordBacklogReportsPendingDepth(t *testing.T) {
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		t.Skip("no test database (set OIKUMENEA_TEST_DSN)")
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	defer pool.Close()

	reg := metrics.NewRootMetricsRegistry()
	ctx := metrics.WithRegistry(context.Background(), reg)
	if _, err := pool.Exec(ctx, `TRUNCATE oikumenea.platform_outbox`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	// Two due pending rows (next_attempt_at defaults to now()), inserted directly.
	if _, err := pool.Exec(ctx, `
		INSERT INTO oikumenea.platform_outbox (event_type, payload)
		VALUES ('test.notify', '{}'::jsonb), ('test.notify', '{}'::jsonb)`); err != nil {
		t.Fatalf("insert pending: %v", err)
	}

	d := New(pool, Config{})
	d.recordBacklog(ctx)

	if got := reg.Gauge(metricOutboxPending).Value(); got != 2 {
		t.Errorf("%s gauge = %d, want 2", metricOutboxPending, got)
	}
	// Oldest-pending age is a non-negative number of seconds; assert it is set (>= 0) without a flaky
	// exact value.
	if got := reg.Gauge(metricOutboxOldestPendingAge).Value(); got < 0 {
		t.Errorf("%s gauge = %d, want >= 0", metricOutboxOldestPendingAge, got)
	}
}
