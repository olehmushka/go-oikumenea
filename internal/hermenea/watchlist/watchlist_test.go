// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Unit tests for the M34 live watchlist screening (D-Watchlists): the Interpol provider against a fake
// Red Notices server, and the Service's cache-first fan-out + aggregation. No DB — the cache is a fake.
package watchlist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// memCache is an in-memory Cache for the Service test.
type memCache struct {
	rows  map[string]domain.WatchlistResult
	puts  int
	gets  int
}

func (c *memCache) Get(_ context.Context, key string) (domain.WatchlistResult, bool, error) {
	c.gets++
	r, ok := c.rows[key]
	return r, ok, nil
}
func (c *memCache) Put(_ context.Context, key string, r domain.WatchlistResult) error {
	c.puts++
	c.rows[key] = r
	return nil
}

// TestInterpolProvider hits a fake Red Notices endpoint: a hit for a known surname, a clear for another.
func TestInterpolProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		w.Header().Set("Content-Type", "application/json")
		if name == "Escobar" {
			_, _ = w.Write([]byte(`{"total":1,"_embedded":{"notices":[{"forename":"Pablo","name":"Escobar"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"total":0,"_embedded":{"notices":[]}}`))
	}))
	defer srv.Close()

	p := NewInterpol(srv.URL, srv.Client())

	hit, err := p.Screen(context.Background(), domain.WatchlistQuery{FullName: "Pablo Escobar"})
	if err != nil {
		t.Fatalf("screen hit: %v", err)
	}
	if !hit.OnList || len(hit.Lists) != 1 || hit.Lists[0] != "INTERPOL_RED" {
		t.Fatalf("expected an INTERPOL_RED hit, got %+v", hit)
	}
	if hit.Score == nil || *hit.Score != 1.0 {
		t.Fatalf("expected an exact-match score of 1.0, got %+v", hit.Score)
	}

	clear, err := p.Screen(context.Background(), domain.WatchlistQuery{FullName: "Jane Ordinary"})
	if err != nil {
		t.Fatalf("screen clear: %v", err)
	}
	if clear.OnList {
		t.Fatalf("expected a clear, got a hit: %+v", clear)
	}
}

// TestServiceCacheFirst proves the Service aggregates provider hits, stores the result, and short-circuits
// a fresh cache row on the second check.
func TestServiceCacheFirst(t *testing.T) {
	hitProvider := stubProvider{hit: Hit{OnList: true, Lists: []string{"INTERPOL_RED"}, Program: "Red", Score: fptr(0.6)}}
	cache := &memCache{rows: map[string]domain.WatchlistResult{}}
	svc := NewService([]Provider{hitProvider, SanctionsStub{}}, cache, time.Hour)

	q := domain.WatchlistQuery{SubjectKey: "person-1", FullName: "Someone"}
	res, err := svc.Check(context.Background(), q)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !res.OnList || len(res.Lists) != 1 || res.NextCheckDue == nil {
		t.Fatalf("aggregate mismatch: %+v", res)
	}
	if cache.puts != 1 {
		t.Fatalf("expected one cache put, got %d", cache.puts)
	}

	// Second check within TTL is served from cache — the providers are not consulted again.
	hitProvider2 := &countingProvider{}
	svc2 := NewService([]Provider{hitProvider2}, cache, time.Hour)
	if _, err := svc2.Check(context.Background(), q); err != nil {
		t.Fatalf("cached check: %v", err)
	}
	if hitProvider2.calls != 0 {
		t.Fatalf("expected the provider to be skipped on a fresh cache hit, got %d calls", hitProvider2.calls)
	}
}

func fptr(v float64) *float64 { return &v }

type stubProvider struct{ hit Hit }

func (stubProvider) Name() string { return "stub" }
func (s stubProvider) Screen(context.Context, domain.WatchlistQuery) (Hit, error) { return s.hit, nil }

type countingProvider struct{ calls int }

func (c *countingProvider) Name() string { return "counting" }
func (c *countingProvider) Screen(context.Context, domain.WatchlistQuery) (Hit, error) {
	c.calls++
	return Hit{}, nil
}

// TestInterpolLive hits the REAL interpol.api.bund.dev Red Notices endpoint. Env-gated (the sandbox IP is
// often blocked by public APIs, as with M30 Wikidata / M43 Factbook): set OIKUMENEA_INTERPOL_E2E=1 to run.
func TestInterpolLive(t *testing.T) {
	if os.Getenv("OIKUMENEA_INTERPOL_E2E") == "" {
		t.Skip("set OIKUMENEA_INTERPOL_E2E=1 to hit the live INTERPOL Red Notices endpoint")
	}
	p := NewInterpol("", nil)
	// A common surname returns notices; we assert only that the call succeeds and is well-formed.
	if _, err := p.Screen(context.Background(), domain.WatchlistQuery{FullName: "John Smith"}); err != nil {
		t.Fatalf("live interpol screen: %v", err)
	}
}
