package transport

import (
	"context"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	identityapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// The service-principal registry endpoints (M51 / D-ServiceIdentities). Minting a machine identity is
// an instance-plane act: every write gates on `service-principal.manage`, reads on
// `service-principal.read` — both instance-scope codes, so only an instance admin passes the PDP.
//
// These are PERSON-gated on purpose: a machine must not be able to register or re-grant machines.
// RequireAnywhere denies a service principal structurally (it carries no person id), so a connector
// calling here is rejected without a special case.

// defaultPrincipalPageSize matches the repo-wide keyset default.
const defaultPrincipalPageSize = 50

func (s Service) RegisterServicePrincipal(ctx context.Context, token bearertoken.Token, request identityapi.RegisterServicePrincipalRequest) (identityapi.ServicePrincipal, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermServicePrincipalManage)); err != nil {
		return identityapi.ServicePrincipal{}, err
	}
	p, err := s.app.RegisterPrincipal(ctx, domain.ServicePrincipal{
		Code:        request.Code,
		Name:        request.Name,
		Description: derefStr(request.Description),
		Issuer:      request.Issuer,
		Subject:     request.Subject,
		ClientID:    derefStr(request.ClientId),
	})
	if err != nil {
		return identityapi.ServicePrincipal{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIPrincipal(p), nil
}

func (s Service) GetServicePrincipal(ctx context.Context, token bearertoken.Token, principalID string) (identityapi.ServicePrincipal, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermServicePrincipalRead)); err != nil {
		return identityapi.ServicePrincipal{}, err
	}
	p, err := s.app.GetPrincipal(ctx, principalID)
	if err != nil {
		return identityapi.ServicePrincipal{}, s.mapError(ctx, err, errCtx{principalID: principalID})
	}
	return toAPIPrincipal(p), nil
}

func (s Service) ListServicePrincipals(ctx context.Context, token bearertoken.Token, pageSize *int, pageToken *string) (identityapi.ServicePrincipalPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermServicePrincipalRead)); err != nil {
		return identityapi.ServicePrincipalPage{}, err
	}
	limit := defaultPrincipalPageSize
	if pageSize != nil && *pageSize > 0 {
		limit = *pageSize
	}
	// Fetch one extra to decide whether a next page exists without a second count query.
	rows, err := s.app.ListPrincipals(ctx, derefStr(pageToken), limit+1)
	if err != nil {
		return identityapi.ServicePrincipalPage{}, s.mapError(ctx, err, errCtx{})
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1].ID
		next = &last
	}
	out := make([]identityapi.ServicePrincipal, 0, len(rows))
	for _, p := range rows {
		out = append(out, toAPIPrincipal(p))
	}
	return identityapi.ServicePrincipalPage{Principals: out, NextPageToken: next}, nil
}

func (s Service) UpdateServicePrincipal(ctx context.Context, token bearertoken.Token, principalID string, request identityapi.UpdateServicePrincipalRequest) (identityapi.ServicePrincipal, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermServicePrincipalManage)); err != nil {
		return identityapi.ServicePrincipal{}, err
	}
	p, err := s.app.UpdatePrincipal(ctx, domain.ServicePrincipal{
		ID:          principalID,
		Name:        request.Name,
		Description: derefStr(request.Description),
		ClientID:    derefStr(request.ClientId),
	})
	if err != nil {
		return identityapi.ServicePrincipal{}, s.mapError(ctx, err, errCtx{principalID: principalID})
	}
	return toAPIPrincipal(p), nil
}

func (s Service) DisableServicePrincipal(ctx context.Context, token bearertoken.Token, principalID string) (identityapi.ServicePrincipal, error) {
	return s.setPrincipalStatus(ctx, token, principalID, domain.PrincipalDisabled)
}

func (s Service) EnableServicePrincipal(ctx context.Context, token bearertoken.Token, principalID string) (identityapi.ServicePrincipal, error) {
	return s.setPrincipalStatus(ctx, token, principalID, domain.PrincipalActive)
}

func (s Service) setPrincipalStatus(ctx context.Context, token bearertoken.Token, principalID string, status domain.PrincipalStatus) (identityapi.ServicePrincipal, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermServicePrincipalManage)); err != nil {
		return identityapi.ServicePrincipal{}, err
	}
	p, err := s.app.SetPrincipalStatus(ctx, principalID, status)
	if err != nil {
		return identityapi.ServicePrincipal{}, s.mapError(ctx, err, errCtx{principalID: principalID})
	}
	return toAPIPrincipal(p), nil
}

func toAPIPrincipal(p domain.ServicePrincipal) identityapi.ServicePrincipal {
	return identityapi.ServicePrincipal{
		Id:          p.ID,
		Code:        p.Code,
		Name:        p.Name,
		Description: optStr(p.Description),
		Issuer:      p.Issuer,
		Subject:     p.Subject,
		ClientId:    optStr(p.ClientID),
		Status:      string(p.Status),
		CreatedAt:   datetime.DateTime(p.CreatedAt),
		UpdatedAt:   datetime.DateTime(p.UpdatedAt),
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
