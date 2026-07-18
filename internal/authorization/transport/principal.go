package transport

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	authzapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/authorization"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// The machine-subject grant endpoints (M51 / D-ServiceIdentities). Granting authority to a machine is
// an instance-plane act gated on `service-principal.manage` — an instance-scope code, so only an
// instance admin passes.
//
// Person-gated on purpose: RequireAnywhere denies a service principal structurally (it carries no
// person id), so a connector cannot escalate itself or another machine.

func (s Service) GrantPrincipalPermission(ctx context.Context, token bearertoken.Token, request authzapi.GrantPrincipalPermissionRequest) (authzapi.PrincipalGrant, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermServicePrincipalManage)); err != nil {
		return authzapi.PrincipalGrant{}, err
	}
	granted, err := s.app.GrantPrincipalPermission(ctx, domain.PrincipalGrantInput{
		PrincipalID: request.PrincipalId,
		Permission:  domain.Permission(request.Permission),
		OrgID:       derefOr(request.OrgId, ""),
		GrantedBy:   pep.Subject(ctx),
	})
	if err != nil {
		return authzapi.PrincipalGrant{}, s.mapError(ctx, err)
	}
	return toAPIPrincipalGrant(granted), nil
}

func (s Service) RevokePrincipalPermission(ctx context.Context, token bearertoken.Token, grantID string) (authzapi.PrincipalGrant, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermServicePrincipalManage)); err != nil {
		return authzapi.PrincipalGrant{}, err
	}
	revoked, err := s.app.RevokePrincipalPermission(ctx, grantID, pep.Subject(ctx))
	if err != nil {
		return authzapi.PrincipalGrant{}, s.mapError(ctx, err)
	}
	return toAPIPrincipalGrant(revoked), nil
}

func (s Service) ListPrincipalGrants(ctx context.Context, token bearertoken.Token, principalID string) (authzapi.PrincipalGrantPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermServicePrincipalRead)); err != nil {
		return authzapi.PrincipalGrantPage{}, err
	}
	rows, err := s.app.ListPrincipalGrants(ctx, principalID)
	if err != nil {
		return authzapi.PrincipalGrantPage{}, s.mapError(ctx, err)
	}
	out := make([]authzapi.PrincipalGrant, 0, len(rows))
	for _, g := range rows {
		out = append(out, toAPIPrincipalGrant(g))
	}
	return authzapi.PrincipalGrantPage{Grants: out}, nil
}

func toAPIPrincipalGrant(g domain.PrincipalGrant) authzapi.PrincipalGrant {
	out := authzapi.PrincipalGrant{
		Id:          g.ID,
		PrincipalId: g.PrincipalID,
		Permission:  string(g.Permission),
		GrantedAt:   datetime.DateTime(g.GrantedAt),
	}
	if g.OrgID != "" {
		orgID := g.OrgID
		out.OrgId = &orgID
	}
	if g.GrantedBy != "" {
		by := g.GrantedBy
		out.GrantedBy = &by
	}
	if g.RevokedAt != nil {
		at := datetime.DateTime(*g.RevokedAt)
		out.RevokedAt = &at
	}
	return out
}
