// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for lazy RLS connection pinning (review-2026-07 R-03). The R-03 acceptance in
// miniature: a request that never touches an RLS-consuming module holds NO pooled connection (so
// in-flight requests can exceed the pool size), the first scoped statement pins + GUC-scopes the
// connection in one round trip, and release returns it GUC-clean.
package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

func TestLazyConnPinning(t *testing.T) {
	ctx := context.Background()
	super, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer super.Close()

	t.Run("no DB use pins nothing", func(t *testing.T) {
		lctx, release := pdb.WithLazyConn(ctx, super, pdb.RLSState{PersonID: ""})
		_ = lctx
		if got := super.Stat().AcquiredConns(); got != 0 {
			t.Errorf("acquired conns before any statement = %d, want 0", got)
		}
		release() // releasing an unacquired holder must be a no-op
		if got := super.Stat().AcquiredConns(); got != 0 {
			t.Errorf("acquired conns after no-op release = %d, want 0", got)
		}
	})

	t.Run("first statement pins and scopes; release unpins", func(t *testing.T) {
		lctx, release := pdb.WithLazyConn(ctx, super, pdb.RLSState{PersonID: "", IsInstanceAdmin: true})
		q := pdb.RequestQuerier(lctx, super)
		var guc string
		if err := q.QueryRow(lctx, "SELECT current_setting('app.is_instance_admin', true)").Scan(&guc); err != nil {
			t.Fatalf("scoped statement: %v", err)
		}
		if guc != "true" {
			t.Errorf("app.is_instance_admin on lazily pinned conn = %q, want \"true\"", guc)
		}
		if got := super.Stat().AcquiredConns(); got != 1 {
			t.Errorf("acquired conns after first statement = %d, want 1", got)
		}
		release()
		if got := super.Stat().AcquiredConns(); got != 0 {
			t.Errorf("acquired conns after release = %d, want 0", got)
		}
	})

	t.Run("in-flight requests exceed the pool size when idle on the DB", func(t *testing.T) {
		// A 2-conn pool with 10 concurrent "requests" that install the lazy holder but never touch
		// the DB: under the old eager pinning this deadlocks/queues at 2; lazily it must not block.
		small, err := pdb.NewPool(ctx, superuserDSN()+"&pool_max_conns=2", "local")
		if err != nil {
			t.Fatalf("small pool: %v", err)
		}
		defer small.Close()

		var wg sync.WaitGroup
		hold := make(chan struct{})
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, release := pdb.WithLazyConn(ctx, small, pdb.RLSState{})
				defer release()
				<-hold // all 10 "requests" are in flight simultaneously
			}()
		}
		done := make(chan struct{})
		go func() { close(hold); wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("10 in-flight lazy requests over a 2-conn pool did not complete (pinning is not lazy?)")
		}
	})
}
