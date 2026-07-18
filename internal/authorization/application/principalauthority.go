// Request-scoped authority for MACHINE subjects (M51 / D-ServiceIdentities) — the service-side
// counterpart of authority.go.
//
// A service principal's authority is a flat set of (permission_code, org_id) grants: no
// instance-admin flag, no reach, no PDP. The middleware fetches it once per request
// (ContextWithPrincipalAuthority) and stashes it under its OWN context key; PEP service checks read
// the snapshot instead of re-querying.
//
// Deliberately SEPARATE from AuthoritySnapshot rather than reusing it with a synthetic PersonID:
// authorityFor validates a snapshot by `snap.PersonID == subjectPersonID`, and letting a principal
// RID flow into that guard is exactly the aliasing review-2026-07 R-01 designed out. A principal and
// a person can never be confused for one another here.
//
// NO CROSS-REQUEST CACHE. Unlike the person path (D-AuthzGrantCache, 2 s TTL) a principal's grants
// are read per request — one indexed lookup on authz_principal_grants(principal_id). Machine traffic
// is low-volume by nature (a connector sends chunk-sized requests, a facade forwards user tokens and
// calls on its own behalf only rarely), so the cache would buy little; and the absence of one makes
// revocation of a machine's authority IMMEDIATE everywhere rather than eventually-consistent within
// the TTL — the stronger property for a credential that may need to be cut off fast. Grant writes
// still bump authz_epoch (see principal.go) so the "every authority-mutating transaction bumps the
// epoch" invariant holds uniformly for any future caching.
package application

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
)

// PrincipalAuthoritySnapshot is a machine subject's authority state at request start.
type PrincipalAuthoritySnapshot struct {
	PrincipalID string
	Grants      []domain.PrincipalGrant
}

type principalAuthorityCtxKey struct{}

func withPrincipalAuthority(ctx context.Context, snap PrincipalAuthoritySnapshot) context.Context {
	return context.WithValue(ctx, principalAuthorityCtxKey{}, snap)
}

func principalAuthorityFrom(ctx context.Context) (PrincipalAuthoritySnapshot, bool) {
	snap, ok := ctx.Value(principalAuthorityCtxKey{}).(PrincipalAuthoritySnapshot)
	return snap, ok
}

// ContextWithPrincipalAuthority is the authenticator's per-request entry for a machine subject:
// resolve the principal's active grants once and attach them to the context.
//
// It returns no db.RLSState: a service request pins no connection and sets no app.person_id GUC,
// because the RLS policies compute reach from a PERSON (D-RLSLiveReach) and there is no principal
// arm until M53 / D-ConnectorPlane. The PEP therefore denies a principal every person-shaped path
// (see pep.go) — the machine can only reach surfaces that explicitly ask for a service grant.
func (s *Service) ContextWithPrincipalAuthority(ctx context.Context, principalID string) (context.Context, error) {
	grants, err := s.newPrincipalRepo(s.pool).ActiveGrantsForPrincipal(ctx, principalID)
	if err != nil {
		return ctx, err
	}
	return withPrincipalAuthority(ctx, PrincipalAuthoritySnapshot{PrincipalID: principalID, Grants: grants}), nil
}

// HoldsPrincipalPermission answers the machine authorization question: does this principal hold
// `action` for a request touching `orgID`?
//
// orgID "" means the request is NOT organization-qualified, and only an instance-wide grant
// (org_id NULL) satisfies it — an org-confined connector must not pass an endpoint that could reach
// data outside its organization. A named orgID is satisfied by an instance-wide grant OR a grant
// naming that same organization (domain.PrincipalGrant.Satisfies).
func (s *Service) HoldsPrincipalPermission(ctx context.Context, principalID, action, orgID string) (bool, error) {
	grants, err := s.principalGrantsFor(ctx, principalID)
	if err != nil {
		return false, err
	}
	for _, g := range grants {
		if g.Satisfies(domain.Permission(action), orgID) {
			return true, nil
		}
	}
	return false, nil
}

// principalGrantsFor prefers the request snapshot and falls back to a fresh read, so out-of-request
// callers (CLI, tests, the boot seeder) work without the middleware having run.
func (s *Service) principalGrantsFor(ctx context.Context, principalID string) ([]domain.PrincipalGrant, error) {
	if snap, ok := principalAuthorityFrom(ctx); ok && snap.PrincipalID == principalID {
		return snap.Grants, nil
	}
	return s.newPrincipalRepo(s.pool).ActiveGrantsForPrincipal(ctx, principalID)
}
