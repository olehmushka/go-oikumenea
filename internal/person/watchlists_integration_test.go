// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the M34 watchlists & regulatory-exposure slice (D-Watchlists) against a real
// Postgres. Proves the exit criteria:
//
//   - CheckWatchlists persists ONLY match metadata (on_list/lists/program/score), one row per person;
//     a re-check refreshes it in place (no second row) and re-snapshots the PEP flag;
//
//   - PEP is derived locally from the M33 government positions (the fake screening seam never returns it);
//
//   - a regulatory sanction round-trips (incl. amount/currency) and lists/deletes;
//
//   - MergePerson re-homes the durable regulatory sanction onto the canonical person and drops the stub's
//     transient watchlist match;
//
//   - purge hard-deletes both the watchlist match and the regulatory sanctions;
//
//   - CheckWatchlists without the seam wired returns ErrWatchlistUnavailable.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/internal/watchlistclient"
)

// fakeWatchlist is a deterministic WatchlistLookup stand-in for the hermenea seam. It records the last
// query and returns a configurable screening result — the live INTERPOL call is exercised separately in
// the hermenea package (env-gated).
type fakeWatchlist struct {
	result domain.WatchlistScreenResult
	err    error
	lastQ  domain.WatchlistQuery
	calls  int
}

func (f *fakeWatchlist) Screen(_ context.Context, q domain.WatchlistQuery) (domain.WatchlistScreenResult, error) {
	f.calls++
	f.lastQ = q
	return f.result, f.err
}

func fptr(v float64) *float64 { return &v }

func TestWatchlists(t *testing.T) {
	ctx := context.Background()
	svc, prof, sens, pool := newServices(t, 720)

	// ---- CheckWatchlists with the disabled no-op seam => ErrWatchlistUnavailable (R-11) ----
	// The seam is always wired at boot; the "no companion" case is watchlistclient.Disabled{}.
	sens.SetWatchlistLookup(watchlistclient.Disabled{})
	p := newPerson(t, svc, "Ivan Screened")
	if _, err := sens.CheckWatchlists(ctx, p.ID); !errors.Is(err, domain.ErrWatchlistUnavailable) {
		t.Fatalf("expected ErrWatchlistUnavailable with the disabled seam, got %v", err)
	}

	// ---- CheckWatchlists persists only match metadata (a hit) ----
	fake := &fakeWatchlist{result: domain.WatchlistScreenResult{
		OnList: true, Lists: []string{"INTERPOL_RED"}, Program: "INTERPOL Red Notice", MatchScore: fptr(1.0),
	}}
	sens.SetWatchlistLookup(fake)

	m, err := sens.CheckWatchlists(ctx, p.ID)
	if err != nil {
		t.Fatalf("check watchlists: %v", err)
	}
	if !m.OnList || len(m.Lists) != 1 || m.Lists[0] != "INTERPOL_RED" || m.Program != "INTERPOL Red Notice" {
		t.Fatalf("match metadata mismatch: %+v", m)
	}
	if m.PEP {
		t.Fatal("pep should be false before any government position")
	}
	if fake.lastQ.SubjectKey != p.ID || fake.lastQ.FullName != "Ivan Screened" {
		t.Fatalf("screening query mismatch: %+v", fake.lastQ)
	}

	// exactly one active row.
	assertMatchRows(t, pool, p.ID, 1)

	got, ok, err := sens.GetWatchlistMatch(ctx, p.ID)
	if err != nil || !ok || got.ID != m.ID {
		t.Fatalf("get watchlist match: ok=%v err=%v got=%+v", ok, err, got)
	}

	// ---- PEP derives from an M33 government position; re-check refreshes in place ----
	if _, err := prof.UpsertGovernmentPosition(ctx, domain.GovernmentPosition{
		PersonID: p.ID, Title: "Senator", Body: "Parliament",
	}); err != nil {
		t.Fatalf("add government position: %v", err)
	}
	m2, err := sens.CheckWatchlists(ctx, p.ID)
	if err != nil {
		t.Fatalf("re-check: %v", err)
	}
	if !m2.PEP {
		t.Fatal("pep should be true after a pep_trigger government position")
	}
	if m2.ID != m.ID {
		t.Fatalf("re-check should refresh the same row, got %s want %s", m2.ID, m.ID)
	}
	assertMatchRows(t, pool, p.ID, 1)

	// ---- regulatory sanction round-trip (amount/currency) ----
	x, err := sens.UpsertRegulatorySanction(ctx, domain.RegulatorySanction{
		PersonID: p.ID, Regulator: "SEC", ActionType: "fine", Amount: fptr(50000), Currency: "USD",
		Status: "active", SanctionDate: "2021-06-01", ExternalID: "SEC-2021-42",
	})
	if err != nil {
		t.Fatalf("add regulatory sanction: %v", err)
	}
	if x.Amount == nil || *x.Amount != 50000 || x.Currency != "USD" {
		t.Fatalf("sanction amount round-trip mismatch: %+v", x)
	}
	// amount without currency is rejected.
	if _, err := sens.UpsertRegulatorySanction(ctx, domain.RegulatorySanction{
		PersonID: p.ID, Regulator: "FCA", Amount: fptr(10), // no currency
	}); err == nil {
		t.Fatal("expected amount-without-currency to be rejected")
	}
	xs, err := sens.ListRegulatorySanctions(ctx, p.ID)
	if err != nil || len(xs) != 1 {
		t.Fatalf("list regulatory sanctions: n=%d err=%v", len(xs), err)
	}
	if err := sens.DeleteRegulatorySanction(ctx, p.ID, x.ID); err != nil {
		t.Fatalf("delete regulatory sanction: %v", err)
	}
	xs, _ = sens.ListRegulatorySanctions(ctx, p.ID)
	if len(xs) != 0 {
		t.Fatalf("regulatory sanction should be deleted, have %d", len(xs))
	}

	// ---- purge hard-deletes the watchlist match + regulatory sanctions ----
	// re-add a sanction so purge has something to erase.
	if _, err := sens.UpsertRegulatorySanction(ctx, domain.RegulatorySanction{
		PersonID: p.ID, Regulator: "NBU", ExternalID: "NBU-1",
	}); err != nil {
		t.Fatalf("re-add sanction: %v", err)
	}
	svcNow, _, _, _ := newServices(t, 0) // zero-grace purge; auto-wires the PersonPurged bus (R-09)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "test"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	assertMatchRows(t, pool, p.ID, 0)
	var sanctionRows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.person_regulatory_sanctions WHERE person_id=$1`, p.ID).Scan(&sanctionRows); err != nil {
		t.Fatalf("post-purge sanction count: %v", err)
	}
	if sanctionRows != 0 {
		t.Fatalf("regulatory sanctions should be hard-deleted on purge, have %d", sanctionRows)
	}
}

// TestWatchlistMerge proves MergePerson re-homes the durable regulatory sanction onto the canonical
// person and drops the stub's transient watchlist match.
func TestWatchlistMerge(t *testing.T) {
	ctx := context.Background()
	svc, _, sens, pool := newServices(t, 720)
	sens.SetWatchlistLookup(&fakeWatchlist{result: domain.WatchlistScreenResult{OnList: false, Lists: []string{}}})

	stub, err := svc.CreateProvisionalPerson(ctx, domain.Person{Name: domain.Name{DisplayName: "Watchlist Stub"}})
	if err != nil {
		t.Fatalf("create provisional: %v", err)
	}
	canonical := newPerson(t, svc, "Watchlist Canonical")

	// The stub carries a durable regulatory sanction + a transient watchlist match.
	if _, err := sens.UpsertRegulatorySanction(ctx, domain.RegulatorySanction{
		PersonID: stub.ID, Regulator: "OFAC", ExternalID: "OFAC-STUB-1",
	}); err != nil {
		t.Fatalf("add stub sanction: %v", err)
	}
	if _, err := sens.CheckWatchlists(ctx, stub.ID); err != nil {
		t.Fatalf("screen stub: %v", err)
	}
	assertMatchRows(t, pool, stub.ID, 1)

	if _, err := svc.MergePerson(ctx, stub.ID, canonical.ID, "confirmed"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// The regulatory sanction is re-homed onto the canonical person.
	xs, err := sens.ListRegulatorySanctions(ctx, canonical.ID)
	if err != nil || len(xs) != 1 || xs[0].Regulator != "OFAC" {
		t.Fatalf("sanction not re-homed onto canonical: %+v err=%v", xs, err)
	}
	// The stub's transient watchlist match is gone (dropped by the merge's purge step).
	assertMatchRows(t, pool, stub.ID, 0)
}

func assertMatchRows(t *testing.T, pool *pgxpool.Pool, personID string, want int) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM oikumenea.person_watchlist_matches WHERE person_id=$1 AND deleted_at IS NULL`, personID).Scan(&n); err != nil {
		t.Fatalf("count watchlist matches: %v", err)
	}
	if n != want {
		t.Fatalf("watchlist match rows = %d, want %d", n, want)
	}
}
