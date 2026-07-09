//go:build integration

// Integration tests for the M47 authority-path acceptance criteria (review-2026-07 R-01):
//
//   - a guarded request issues EXACTLY ONE grants fetch no matter how many Require* gates run
//     (request-scoped snapshot, D-AuthzRequestContext), and a second request within the cache TTL
//     issues ZERO (epoch-validated grant cache, D-AuthzGrantCache) — asserted with the Phase-0
//     query-counter tracer;
//   - a revoke performed by ANOTHER process (simulated by a second Service over the same DB)
//     becomes effective within the ≤2 s TTL bound recorded in D-AuthzGrantCache; the local process
//     sees its own mutations immediately (post-commit cache reset).
package authorization_test

import (
	"context"
	"testing"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

func TestAuthorityFetchedOncePerGuardedRequest(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	unit := h.seedUnit(t)
	subj := h.seedPerson(t)
	h.grant(t, subj, h.roleID(t, authzdomain.BaseRoleUnitReader), unit, authzdomain.ScopeUnit, "")

	// Request 1: cold cache — the authenticator's fetch is the ONLY grants join; 3 gates reuse it.
	cctx, counter := pdb.WithQueryCounter(ctx)
	counter.CaptureSQL()
	actx, _, err := h.authz.ContextWithAuthority(cctx, subj)
	if err != nil {
		t.Fatalf("ContextWithAuthority: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := h.authz.Enforce(actx, subj, "unit.read", unit); err != nil {
			t.Fatalf("Enforce #%d: %v", i, err)
		}
	}
	pdb.AssertQueryCount(t, counter, "FROM oikumenea.authz_role_assignments", 1)

	// Request 2 (same subject, within the 2 s TTL): zero grants joins — served from the cache.
	cctx2, counter2 := pdb.WithQueryCounter(ctx)
	counter2.CaptureSQL()
	actx2, _, err := h.authz.ContextWithAuthority(cctx2, subj)
	if err != nil {
		t.Fatalf("ContextWithAuthority #2: %v", err)
	}
	if err := h.authz.Enforce(actx2, subj, "unit.read", unit); err != nil {
		t.Fatalf("Enforce on request 2: %v", err)
	}
	pdb.AssertQueryCount(t, counter2, "FROM oikumenea.authz_role_assignments", 0)
}

func TestGrantCacheCrossReplicaConvergence(t *testing.T) {
	// Two Service instances over the same DB = two replicas with independent in-process caches.
	replicaA := newHarness(t)
	replicaB := newHarness(t)
	ctx := context.Background()
	unit := replicaA.seedUnit(t)
	subj := replicaA.seedPerson(t)

	granted, err := replicaA.authz.GrantAssignment(ctx, authzdomain.GrantInput{
		SubjectPersonID: subj, RoleID: replicaA.roleID(t, authzdomain.BaseRoleUnitReader),
		TargetUnitID: unit, Scope: authzdomain.ScopeUnit,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	// Warm replica B's cache with the ALLOW state.
	if err := replicaB.authz.Enforce(ctx, subj, "unit.read", unit); err != nil {
		t.Fatalf("replica B should allow after the grant: %v", err)
	}

	// Replica A revokes. A sees it immediately (its cache resets post-commit)…
	if _, err := replicaA.authz.RevokeAssignment(ctx, granted.ID, ""); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := replicaA.authz.Enforce(ctx, subj, "unit.read", unit); err == nil {
		t.Fatal("replica A must deny immediately after its own revoke")
	}

	// …replica B converges within the TTL bound (2 s + validation slack): the stale entry expires,
	// the epoch read sees the bump, and the refetch returns no grants.
	deadline := time.Now().Add(4 * time.Second)
	for {
		if err := replicaB.authz.Enforce(ctx, subj, "unit.read", unit); err != nil {
			break // converged to deny
		}
		if time.Now().After(deadline) {
			t.Fatal("replica B still allows >4s after the revoke — the epoch-validated TTL did not converge")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
