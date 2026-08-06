// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for the watchlist screening deadline (review-2026-07 R-12): a hung hermenea /
// sanctions upstream must fail CheckWatchlists at the client's HTTP timeout — through the REAL
// conjure client + watchlistclient adapter stack main.go wires (only the timeout is shortened) —
// and, with lazy pinning (R-03), the stalled request must hold NO pooled DB connection, so
// concurrent unrelated requests keep working.
package person_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hermeneaapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olehmushka/go-oikumenea/internal/watchlistclient"
	"github.com/palantir/conjure-go-runtime/v2/conjure-go-client/httpclient"
)

func TestWatchlistStallFailsAtDeadline(t *testing.T) {
	ctx := context.Background()
	svc, _, sens, pool := newServices(t, 720)
	p := newPerson(t, svc, "Stall Test Subject")

	// A hermenea that never answers within the deadline.
	stallRelease := make(chan struct{})
	stalling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-stallRelease
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer stalling.Close()
	defer close(stallRelease)

	// The exact client stack main.go builds, with the deadline shortened for the test (prod uses
	// watchlistclient.HTTPTimeout = 10s).
	const testTimeout = 300 * time.Millisecond
	wlHTTP, err := httpclient.NewClient(
		httpclient.WithBaseURLs([]string{stalling.URL}),
		httpclient.WithMaxRetries(0),
		httpclient.WithHTTPTimeout(testTimeout),
	)
	if err != nil {
		t.Fatalf("build stalling watchlist client: %v", err)
	}
	sens.SetWatchlistLookup(watchlistclient.New(hermeneaapi.NewHermeneaServiceClient(wlHTTP), "test-token"))

	type result struct {
		err     error
		elapsed time.Duration
	}
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		_, err := sens.CheckWatchlists(ctx, p.ID)
		done <- result{err: err, elapsed: time.Since(start)}
	}()

	// While the screening call stalls, the request must hold no pooled connection (R-03: the person
	// module reads on the bare pool and the egress happens before the write tx) and unrelated DB
	// work must proceed.
	time.Sleep(100 * time.Millisecond) // inside the stall window
	if got := pool.Stat().AcquiredConns(); got != 0 {
		t.Errorf("pooled conns held during the watchlist stall = %d, want 0", got)
	}
	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		t.Errorf("unrelated query during the stall failed: %v", err)
	}

	select {
	case r := <-done:
		if r.err == nil {
			t.Fatal("CheckWatchlists against a stalling hermenea must fail")
		}
		if r.elapsed > 5*time.Second {
			t.Fatalf("CheckWatchlists took %v — the HTTP deadline did not fire", r.elapsed)
		}
		t.Logf("stalled screening failed after %v: %v", r.elapsed.Round(time.Millisecond), r.err)
	case <-time.After(10 * time.Second):
		t.Fatal("CheckWatchlists hung past 10s — no deadline in the watchlist client stack")
	}
}
