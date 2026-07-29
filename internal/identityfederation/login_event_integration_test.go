// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the login security log (M37 / D-LoginSecurityLog) against a real Postgres:
// the bounded-dedup record path, the newest-first read, the purge-erase fan-out, and the retention
// sweep. Run:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestLoginEvent ./internal/identityfederation/...
package identityfederation_test

import (
	"context"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/identityfederation/adapters"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/application"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/ipintel"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// newAccount makes an account (with an initial external identity) for personID and returns its ID.
func newAccount(t *testing.T, svc *application.Service, personID string) string {
	t.Helper()
	acct, err := svc.CreateAccount(context.Background(), domain.Account{PersonID: personID},
		&domain.ExternalIdentity{Issuer: "https://idp.example", Subject: uniq("le-sub")})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acct.ID
}

func TestLoginEventStore(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	// Enable the login-security log with a 1h dedup window, retain-forever.
	svc.SetLoginEvents(func(conn pdb.DBTX) domain.LoginEventRepository { return adapters.NewLoginEventRepo(conn) },
		ipintel.NoOp{}, 3600, 0)
	ctx := context.Background()

	personID := makePerson(t, pool, uniq("le-p"))
	accountID := newAccount(t, svc, personID)
	repo := adapters.NewLoginEventRepo(pool)

	// 1) Dedup within the window: two identical (account, login, ip) occurrences collapse to one row
	//    with occurrence_count = 2; the IP-intel overlay stays NULL (no-op resolver).
	svc.RecordLoginSeen(ctx, accountID, domain.LoginContextLogin, "203.0.113.5", "agent/1.0")
	svc.RecordLoginSeen(ctx, accountID, domain.LoginContextLogin, "203.0.113.5", "agent/1.0")
	rows, err := svc.ListLoginEvents(ctx, accountID, "", 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("dedup: want 1 row, got %d", len(rows))
	}
	if rows[0].OccurrenceCount != 2 {
		t.Errorf("dedup: occurrence_count = %d, want 2", rows[0].OccurrenceCount)
	}
	if rows[0].IP != "203.0.113.5" || rows[0].Context != domain.LoginContextLogin {
		t.Errorf("row = %+v, want ip 203.0.113.5 / login", rows[0])
	}
	if rows[0].ResolvedCountry != nil || rows[0].IsVPN != nil {
		t.Errorf("no-op intel should leave resolved fields NULL, got country=%v vpn=%v", rows[0].ResolvedCountry, rows[0].IsVPN)
	}

	// 2) The dedup key is (account, context, ip): a different context OR a different ip is a new row.
	svc.RecordLoginSeen(ctx, accountID, domain.LoginContextRegistration, "203.0.113.5", "agent/1.0") // diff context
	svc.RecordLoginSeen(ctx, accountID, domain.LoginContextLogin, "198.51.100.9", "agent/1.0")       // diff ip
	rows, _ = svc.ListLoginEvents(ctx, accountID, "", 50)
	if len(rows) != 3 {
		t.Fatalf("distinct (context,ip): want 3 rows, got %d", len(rows))
	}

	// 3) Past the window a repeat (account, context, ip) becomes a NEW row (not a bump). Age the
	//    login/203.0.113.5 row 2h into the past, then record again with a 1h window.
	if _, err := pool.Exec(ctx, `
		UPDATE oikumenea.account_login_events SET last_seen_at = now() - interval '2 hours'
		 WHERE account_id = $1 AND context = 'login' AND ip = '203.0.113.5'::inet`, accountID); err != nil {
		t.Fatalf("age row: %v", err)
	}
	if err := repo.RecordSeen(ctx, accountID, domain.LoginContextLogin, "203.0.113.5", nil, domain.IPIntel{}, 3600); err != nil {
		t.Fatalf("record past window: %v", err)
	}
	var loginIPCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM oikumenea.account_login_events
		 WHERE account_id = $1 AND context = 'login' AND ip = '203.0.113.5'::inet`, accountID).Scan(&loginIPCount); err != nil {
		t.Fatalf("count: %v", err)
	}
	if loginIPCount != 2 {
		t.Errorf("past-window: want 2 rows for login/203.0.113.5, got %d", loginIPCount)
	}

	// 4) Keyset paging (newest-first): page size 2 then the remainder.
	page1, _ := svc.ListLoginEvents(ctx, accountID, "", 2)
	if len(page1) != 2 {
		t.Fatalf("page1: want 2, got %d", len(page1))
	}
	page2, _ := svc.ListLoginEvents(ctx, accountID, page1[len(page1)-1].ID, 50)
	if len(page2) != 2 { // 4 rows total now
		t.Errorf("page2: want 2, got %d", len(page2))
	}

	// 5) Retention sweep: delete everything older than now+1h → all rows gone.
	n, err := repo.DeleteBefore(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n < 4 {
		t.Errorf("sweep deleted %d rows, want >= 4", n)
	}
	rows, _ = svc.ListLoginEvents(ctx, accountID, "", 50)
	if len(rows) != 0 {
		t.Errorf("after sweep: want 0 rows, got %d", len(rows))
	}
}

// TestLoginEventEraseByPerson proves the purge fan-out erases a person's login events.
func TestLoginEventEraseByPerson(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	svc.SetLoginEvents(func(conn pdb.DBTX) domain.LoginEventRepository { return adapters.NewLoginEventRepo(conn) },
		ipintel.NoOp{}, 3600, 0)
	ctx := context.Background()

	personID := makePerson(t, pool, uniq("le-erase"))
	accountID := newAccount(t, svc, personID)
	svc.RecordLoginSeen(ctx, accountID, domain.LoginContextLogin, "203.0.113.7", "a/1")

	// A different person's events must NOT be erased.
	otherPerson := makePerson(t, pool, uniq("le-other"))
	otherAccount := newAccount(t, svc, otherPerson)
	svc.RecordLoginSeen(ctx, otherAccount, domain.LoginContextLogin, "203.0.113.8", "a/1")

	repo := adapters.NewLoginEventRepo(pool)
	erased, err := repo.EraseByPerson(ctx, personID)
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	if erased != 1 {
		t.Errorf("erased %d, want 1", erased)
	}
	if rows, _ := svc.ListLoginEvents(ctx, accountID, "", 50); len(rows) != 0 {
		t.Errorf("purged person still has %d login events", len(rows))
	}
	if rows, _ := svc.ListLoginEvents(ctx, otherAccount, "", 50); len(rows) != 1 {
		t.Errorf("other person's events must survive, got %d", len(rows))
	}
}
