// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Unit tests for the request-scoped authority context (review-2026-07 R-01.1): once the
// authenticator attaches the snapshot via ContextWithAuthority, PDP-consuming calls must not
// re-fetch authority state — and a call about a DIFFERENT subject must not reuse it.
package application

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// fakeRepo implements only the two authority reads; everything else panics via the embedded nil.
type fakeRepo struct {
	domain.Repository
	fetches *int
	isAdmin bool
	grants  []domain.ActiveGrant
}

func (f fakeRepo) IsActiveInstanceAdmin(ctx context.Context, personID string) (bool, error) {
	*f.fetches++
	return f.isAdmin, nil
}

func (f fakeRepo) ActiveGrantsForSubject(ctx context.Context, subjectPersonID string) ([]domain.ActiveGrant, error) {
	return f.grants, nil
}

func (f fakeRepo) ReadAuthzEpoch(ctx context.Context) (int64, error) { return 0, nil }

// fakeClosure: single authority-bearing graph, no hierarchy.
type fakeClosure struct{}

func (fakeClosure) IsAncestorOrSelf(ctx context.Context, graphID, anc, desc string) (bool, error) {
	return false, nil
}
func (fakeClosure) IsAuthorityBearing(ctx context.Context, graphID string) (bool, error) {
	return true, nil
}
func (fakeClosure) DescendantUnitIDs(ctx context.Context, graphID, unitID string) ([]string, error) {
	return nil, nil
}

func newAuthorityTestService(fetches *int, grants []domain.ActiveGrant) *Service {
	repo := fakeRepo{fetches: fetches, grants: grants}
	return NewService(nil, func(conn db.DBTX) domain.Repository { return repo }, nil, domain.NewPDP(fakeClosure{}), nil, nil)
}

func grantOn(unit string, perms ...domain.Permission) domain.ActiveGrant {
	set := make(map[domain.Permission]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	return domain.ActiveGrant{AssignmentID: "a1", RoleID: "r1", TargetUnitID: unit, Scope: domain.ScopeUnit, Perms: set}
}

func TestAuthoritySnapshotFetchedOncePerRequest(t *testing.T) {
	fetches := 0
	svc := newAuthorityTestService(&fetches, []domain.ActiveGrant{grantOn("u1", "person.read")})

	ctx, _, err := svc.ContextWithAuthority(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ContextWithAuthority: %v", err)
	}
	if fetches != 1 {
		t.Fatalf("authority fetches after ContextWithAuthority = %d, want 1", fetches)
	}

	// A multi-gate handler: N Require-style calls + reach + a batch — zero further fetches.
	for i := 0; i < 3; i++ {
		if err := svc.Enforce(ctx, "p1", "person.read", "u1"); err != nil {
			t.Fatalf("Enforce: %v", err)
		}
	}
	if _, err := svc.IsInstanceAdminFor(ctx, "p1"); err != nil {
		t.Fatalf("IsInstanceAdminFor: %v", err)
	}
	if _, err := svc.HoldsPermissionAnywhere(ctx, "p1", "person.read"); err != nil {
		t.Fatalf("HoldsPermissionAnywhere: %v", err)
	}
	if _, err := svc.DecideBatch(ctx, "p1", []BatchQuery{{Action: "person.read", UnitID: "u1"}}, false); err != nil {
		t.Fatalf("DecideBatch: %v", err)
	}
	if fetches != 1 {
		t.Errorf("authority fetches after 6 PDP calls = %d, want 1 (snapshot must be reused)", fetches)
	}
}

func TestAuthoritySnapshotNotReusedForOtherSubject(t *testing.T) {
	fetches := 0
	svc := newAuthorityTestService(&fetches, []domain.ActiveGrant{grantOn("u1", "person.read")})

	ctx, _, err := svc.ContextWithAuthority(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ContextWithAuthority: %v", err)
	}
	if _, err := svc.Decide(ctx, "p2", "person.read", "u1", false); err != nil {
		t.Fatalf("Decide for other subject: %v", err)
	}
	if fetches != 2 {
		t.Errorf("authority fetches = %d, want 2 (p2 must not reuse p1's snapshot)", fetches)
	}
}

// BenchmarkEnforceOnSnapshot: decision cost must be independent of how many Require* gates a
// handler runs once the request snapshot is attached (review R-01 acceptance, application level —
// pair with the domain-level benches in internal/authorization/domain/pdp_bench_test.go).
func BenchmarkEnforceOnSnapshot(b *testing.B) {
	fetches := 0
	svc := newAuthorityTestService(&fetches, []domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx, _, err := svc.ContextWithAuthority(context.Background(), "p1")
	if err != nil {
		b.Fatalf("ContextWithAuthority: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := svc.Enforce(ctx, "p1", "person.read", "u1"); err != nil {
			b.Fatal(err)
		}
	}
	if fetches != 1 {
		b.Fatalf("authority fetches = %d, want 1 regardless of b.N", fetches)
	}
}

func TestAuthorityFallbackWithoutSnapshot(t *testing.T) {
	fetches := 0
	svc := newAuthorityTestService(&fetches, []domain.ActiveGrant{grantOn("u1", "person.read")})

	// No middleware ran (CLI / system / test path): every call fetches fresh.
	if err := svc.Enforce(context.Background(), "p1", "person.read", "u1"); err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if fetches != 1 {
		t.Errorf("authority fetches = %d, want 1", fetches)
	}
}
