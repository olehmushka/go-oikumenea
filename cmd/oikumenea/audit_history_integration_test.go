// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for AuditService.getObjectHistory (review-2026-09 R-31 / D-Temporal tier b)
// against a real Postgres. Acceptance from the finding: the endpoint returns the audited change list
// for a person whose rank changed twice, in order, permission-gated — plus the Phase-17 redaction
// posture the user chose (before/after withheld unless the caller holds the sensitive-reader
// capability). Reuses the links-test world harness (same package) for pool + authz + seed helpers.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	go test -tags integration ./cmd/oikumenea/ -run TestGetObjectHistory
package main

import (
	"context"
	"testing"
	"time"

	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	audittransport "github.com/olegamysk/go-oikumenea/internal/audit/transport"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/pkg/bearertoken"
)

// auditHistoryWorld pairs the shared link-test world (pool + authz + seed helpers) with an audit
// transport service over the same pool, so the tests exercise the real gate + redaction wiring.
func newAuditHistoryWorld(t *testing.T) (linkWorld, audittransport.Service) {
	t.Helper()
	lw := newLinkWorld(t)
	enforcer := pep.New(lw.authz)
	audit := auditapp.NewService(lw.pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return lw, audittransport.NewService(audit, enforcer)
}

// seedRankChange raw-inserts one audit row keyed to targetRID (an Action RID with kind=action). Uses
// SQL directly (not audit.Record) so the test controls created_at and payloads precisely.
func (w linkWorld) seedRankChange(t *testing.T, targetRID, actorRID string, at time.Time, before, after string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.audit_log
		   (id, created_at, actor_type, actor_person_id, action, target_type, target_id, request_id, before, after, outcome)
		 VALUES (oikumenea.new_id(6,3,0), $1, 'person', $2, 'person.rank.assign', 'person', $3, $4, $5::jsonb, $6::jsonb, 'success')`,
		at, actorRID, targetRID, "req-"+at.Format("150405"), before, after); err != nil {
		t.Fatalf("seed audit row: %v", err)
	}
}

// TestGetObjectHistoryOrderedForAdmin: a person whose rank changed twice yields both changes, newest
// first, with before/after visible to an instance admin (who clears the sensitive-reader bar).
func TestGetObjectHistoryOrderedForAdmin(t *testing.T) {
	w, svc := newAuditHistoryWorld(t)
	admin := w.seedPerson(t, "Hist Admin", "")
	w.makeAdmin(t, admin)
	target := w.seedPerson(t, "Rank Changer", "")
	actor := w.seedPerson(t, "Rank Actor", "")

	t1 := time.Now().Add(-2 * time.Hour).UTC()
	t2 := time.Now().Add(-1 * time.Hour).UTC()
	w.seedRankChange(t, target, actor, t1, `{"rank":"private"}`, `{"rank":"corporal"}`)
	w.seedRankChange(t, target, actor, t2, `{"rank":"corporal"}`, `{"rank":"sergeant"}`)

	res, err := svc.GetObjectHistory(linksSubjectCtx(admin), bearertoken.Token(""), target, nil, nil)
	if err != nil {
		t.Fatalf("GetObjectHistory: %v", err)
	}
	if len(res.Events) != 2 {
		t.Fatalf("want 2 history events, got %d", len(res.Events))
	}
	if res.Redacted {
		t.Errorf("admin holds sensitive-reader (short-circuit) — response must not be redacted")
	}
	// Reverse-chronological: newest (t2) first.
	if !time.Time(res.Events[0].At).After(time.Time(res.Events[1].At)) {
		t.Errorf("events not reverse-chronological: %v then %v", res.Events[0].At, res.Events[1].At)
	}
	for _, e := range res.Events {
		if e.Action != "person.rank.assign" {
			t.Errorf("unexpected action %q", e.Action)
		}
		if e.Before == nil || e.After == nil {
			t.Errorf("admin must see before/after payloads; got before=%v after=%v", e.Before, e.After)
		}
	}
}

// TestGetObjectHistoryRedactedForAuditReader: a subject holding audit.read but NOT the sensitive-reader
// capability sees the timeline (action/actor/when) but the before/after payloads are withheld.
func TestGetObjectHistoryRedactedForAuditReader(t *testing.T) {
	w, svc := newAuditHistoryWorld(t)
	unit := w.seedUnit(t)
	viewer := w.seedPerson(t, "Audit Reader", "")
	w.seedGrant(t, viewer, unit, string(authzdomain.PermAuditRead)) // audit.read only, no Art.9 reads
	target := w.seedPerson(t, "Redacted Target", "")
	actor := w.seedPerson(t, "Redacted Actor", "")

	w.seedRankChange(t, target, actor, time.Now().Add(-30*time.Minute).UTC(), `{"rank":"a"}`, `{"rank":"b"}`)

	res, err := svc.GetObjectHistory(linksSubjectCtx(viewer), bearertoken.Token(""), target, nil, nil)
	if err != nil {
		t.Fatalf("GetObjectHistory: %v", err)
	}
	if len(res.Events) != 1 {
		t.Fatalf("want 1 event, got %d", len(res.Events))
	}
	if !res.Redacted {
		t.Errorf("caller lacks sensitive-reader — response must be redacted")
	}
	e := res.Events[0]
	if e.Action != "person.rank.assign" {
		t.Errorf("timeline must remain visible; action=%q", e.Action)
	}
	if e.Before != nil || e.After != nil {
		t.Errorf("before/after must be withheld without sensitive-reader; got before=%v after=%v", e.Before, e.After)
	}
}

// TestGetObjectHistoryDeniedWithoutAuditRead: a subject with no audit.read anywhere is denied outright.
func TestGetObjectHistoryDeniedWithoutAuditRead(t *testing.T) {
	w, svc := newAuditHistoryWorld(t)
	nobody := w.seedPerson(t, "No Perms", "")
	target := w.seedPerson(t, "Unseen Target", "")
	actor := w.seedPerson(t, "Unseen Actor", "")
	w.seedRankChange(t, target, actor, time.Now().Add(-10*time.Minute).UTC(), `{"x":1}`, `{"x":2}`)

	if _, err := svc.GetObjectHistory(linksSubjectCtx(nobody), bearertoken.Token(""), target, nil, nil); err == nil {
		t.Fatalf("expected permission-denied without audit.read, got nil error")
	}
}
