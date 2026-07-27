// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the boot-seed advisory lock (R-13 / M49): WithAdvisoryLock must give strict
// cross-connection mutual exclusion — N concurrent critical sections (each on its own pooled
// connection, as N replicas would be) never overlap, and every contender eventually runs (the loser
// WAITS rather than skipping — the boot-seed semantics).
package db_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

func TestWithAdvisoryLockMutualExclusion(t *testing.T) {
	ctx := context.Background()
	pool, err := pdb.NewPool(ctx, superuserDSN(), "local")
	if err != nil {
		t.Skipf("no test database (set OIKUMENEA_TEST_DSN): %v", err)
	}
	defer pool.Close()

	const contenders = 8
	const testKey int64 = 0x7E57_AD_71_50_12 // test-only key, distinct from LockBootSeed

	var (
		inside   atomic.Int32
		overlaps atomic.Int32
		ran      atomic.Int32
		start    = make(chan struct{})
		wg       sync.WaitGroup
	)
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := pdb.WithAdvisoryLock(ctx, pool, testKey, func(context.Context) error {
				if inside.Add(1) > 1 {
					overlaps.Add(1)
				}
				time.Sleep(20 * time.Millisecond) // hold the "seed" long enough for overlap to show
				inside.Add(-1)
				ran.Add(1)
				return nil
			})
			if err != nil {
				t.Errorf("WithAdvisoryLock: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := overlaps.Load(); got != 0 {
		t.Fatalf("critical sections overlapped %d times, want 0", got)
	}
	if got := ran.Load(); got != contenders {
		t.Fatalf("only %d/%d contenders ran — the lock must make losers wait, not skip", got, contenders)
	}
}
