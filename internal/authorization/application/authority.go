// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Request-scoped authority context (review-2026-07 R-01.1). The subject's authority state —
// instance-admin flag + active grants — is fetched ONCE per request by the identity-federation
// authenticator (ContextWithAuthority) and stashed on the context; every PDP-consuming method on
// the Service (Decide, DecideBatch, HoldsPermissionAnywhere, EffectiveReach, FilterVisibleUnits)
// consumes the snapshot via authorityFor instead of re-running the grants join per call. Semantics
// are unchanged: the state was already a snapshot-at-call-time; now it is a snapshot-at-request-
// start (the same two queries, executed once).
//
// Out-of-request callers (CLI, tests, system paths) carry no snapshot and fall back to a fresh
// fetch, so nothing depends on the middleware having run.
package application

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// AuthoritySnapshot is the subject's authority state at request start.
type AuthoritySnapshot struct {
	PersonID        string
	IsInstanceAdmin bool
	Grants          []domain.ActiveGrant
}

type authorityCtxKey struct{}

func withAuthority(ctx context.Context, snap AuthoritySnapshot) context.Context {
	return context.WithValue(ctx, authorityCtxKey{}, snap)
}

func authorityFrom(ctx context.Context) (AuthoritySnapshot, bool) {
	snap, ok := ctx.Value(authorityCtxKey{}).(AuthoritySnapshot)
	return snap, ok
}

// authorityFor resolves (isAdmin, grants) for a subject: the request snapshot when it matches the
// subject, else the epoch-validated cross-request cache (grantcache.go), which itself falls back
// to the database. The PersonID guard means a decision about a person other than the request
// subject never reads the wrong snapshot.
func (s *Service) authorityFor(ctx context.Context, subjectPersonID string) (bool, []domain.ActiveGrant, error) {
	if snap, ok := authorityFrom(ctx); ok && snap.PersonID == subjectPersonID {
		return snap.IsInstanceAdmin, snap.Grants, nil
	}
	return s.cachedAuthority(ctx, subjectPersonID)
}

// fetchAuthority runs the two authority queries (instance-admin probe + grants join) on the pool.
func (s *Service) fetchAuthority(ctx context.Context, subjectPersonID string) (bool, []domain.ActiveGrant, error) {
	repo := s.newRepo(s.pool)
	isAdmin, err := repo.IsActiveInstanceAdmin(ctx, subjectPersonID)
	if err != nil {
		return false, nil, err
	}
	grants, err := repo.ActiveGrantsForSubject(ctx, subjectPersonID)
	if err != nil {
		return false, nil, err
	}
	return isAdmin, grants, nil
}

// IsInstanceAdminFor reports whether the subject is on the instance plane, resolved through the
// request snapshot / grant cache (zero queries on the request path). Transport read-scope callers
// use it (via pep.SubjectAuthority) for the instance-admin short-circuit that used to ride on the
// materialized Reach.
func (s *Service) IsInstanceAdminFor(ctx context.Context, subjectPersonID string) (bool, error) {
	isAdmin, _, err := s.authorityFor(ctx, subjectPersonID)
	return isAdmin, err
}

// ContextWithAuthority is the authenticator's per-request entry: resolve the subject's authority
// state once (through the epoch-validated cache — steady state is zero authority queries per
// request within the TTL window), attach the snapshot to the context, and return the RLS backstop
// GUC state derived from the same state. Everything the request subsequently asks the PDP reuses
// the snapshot.
func (s *Service) ContextWithAuthority(ctx context.Context, personID string) (context.Context, db.RLSState, error) {
	isAdmin, grants, err := s.cachedAuthority(ctx, personID)
	if err != nil {
		return ctx, db.RLSState{}, err
	}
	ctx = withAuthority(ctx, AuthoritySnapshot{PersonID: personID, IsInstanceAdmin: isAdmin, Grants: grants})
	// D-RLSLiveReach: the GUC state is just the subject identity — the RLS policies compute reach
	// live in SQL, so nothing expands the closure app-side on the request path anymore (R-02.2).
	return ctx, db.RLSState{PersonID: personID, IsInstanceAdmin: isAdmin}, nil
}
