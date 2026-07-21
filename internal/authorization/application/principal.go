package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// The machine-subject grant plane (M51 / D-ServiceIdentities). Granting a permission to a service
// principal is an instance-admin act gated by `service-principal.manage` at the transport, audited
// like every other authority mutation.

const targetPrincipalGrant = "principal_grant"

// PrincipalRepositoryFactory binds a domain.PrincipalRepository to a command surface, mirroring
// RepositoryFactory so the application layer never imports adapters.
type PrincipalRepositoryFactory func(conn db.DBTX) domain.PrincipalRepository

// GrantPrincipalPermission grants one permission code to a machine subject, optionally confined to
// one organization (OrgID "" = instance-wide).
//
// The principal is validated through the cross-module PrincipalDirectory port rather than a join
// into identity-federation's tables (CLAUDE.md: cross-module queries are interface calls). Unknown
// permission codes are rejected against the closed Go catalog, exactly as role permissions are.
func (s *Service) GrantPrincipalPermission(ctx context.Context, in domain.PrincipalGrantInput) (domain.PrincipalGrant, error) {
	if err := in.Validate(); err != nil {
		return domain.PrincipalGrant{}, err
	}
	if err := s.rejectDisabledCode(in.Permission); err != nil {
		return domain.PrincipalGrant{}, err
	}
	if s.principals == nil {
		// A boot-wiring bug, not a caller error: fail loudly rather than granting unvalidated.
		return domain.PrincipalGrant{}, domain.ErrUnknownPrincipal
	}
	active, err := s.principals.PrincipalIsActive(ctx, in.PrincipalID)
	if err != nil {
		return domain.PrincipalGrant{}, err
	}
	if !active {
		return domain.PrincipalGrant{}, domain.ErrUnknownPrincipal
	}

	var out domain.PrincipalGrant
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newPrincipalRepo(tx)
		granted, err := repo.InsertPrincipalGrant(ctx, in)
		if err != nil {
			return err
		}
		// Uniform invariant: every authority-mutating transaction bumps the epoch, so any future
		// caching of principal grants inherits the same revocation contract the person path has
		// (D-AuthzGrantCache). Principal reads are uncached today, so this is belt-and-braces.
		if err := s.newRepo(tx).BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		if err := s.record(ctx, tx, "service-principal.grant", targetPrincipalGrant, granted.ID, principalGrantAudit(granted)); err != nil {
			return err
		}
		out = granted
		return nil
	})
	if err != nil {
		return domain.PrincipalGrant{}, err
	}
	s.grants.reset(ctx)
	return out, nil
}

// RevokePrincipalPermission revoke-flips a grant (never deletes: the history stays readable).
// Because principal grants are read per request with no cache, a revoke takes effect immediately.
func (s *Service) RevokePrincipalPermission(ctx context.Context, grantID, revokedBy string) (domain.PrincipalGrant, error) {
	var out domain.PrincipalGrant
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		revoked, err := s.newPrincipalRepo(tx).RevokePrincipalGrant(ctx, grantID, revokedBy)
		if err != nil {
			return err
		}
		if err := s.newRepo(tx).BumpAuthzEpoch(ctx); err != nil {
			return err
		}
		if err := s.record(ctx, tx, "service-principal.revoke", targetPrincipalGrant, revoked.ID, principalGrantAudit(revoked)); err != nil {
			return err
		}
		out = revoked
		return nil
	})
	if err != nil {
		return domain.PrincipalGrant{}, err
	}
	s.grants.reset(ctx)
	return out, nil
}

// ListPrincipalGrants returns a principal's active grants.
func (s *Service) ListPrincipalGrants(ctx context.Context, principalID string) ([]domain.PrincipalGrant, error) {
	return s.newPrincipalRepo(s.pool).ListPrincipalGrants(ctx, principalID)
}

func principalGrantAudit(g domain.PrincipalGrant) map[string]any {
	out := map[string]any{
		"id":          g.ID,
		"principalId": g.PrincipalID,
		"permission":  string(g.Permission),
		"scope":       "instance",
	}
	if g.OrgID != "" {
		out["scope"] = "organization"
		out["orgId"] = g.OrgID
	}
	return out
}
