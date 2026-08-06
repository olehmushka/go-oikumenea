// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the identity-federation module's generated Conjure
// IdentityFederationService interface: it translates the wire contract to/from the application
// service and maps domain errors to Conjure SerializableErrors (D-Conjure). Generated code in
// internal/conjure is never hand-edited.
//
// Account/identity management gates on the `person.*` permissions of the LINKED person (an account is
// an attachment to a person — identity-federation.md). None of these endpoints carry a unit in the
// request, so they use the coarse "holds the permission anywhere" PEP form pending the precise
// person-scoped tightening (a follow-up shared with person's own id-keyed reads). /whoami is
// authenticated-only: it returns the PDP subject the validation middleware attached to the request
// context (pkg/authn) — no permission check, since the middleware is the authentication boundary.
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	identityapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olehmushka/go-oikumenea/internal/identityfederation/application"
	"github.com/olehmushka/go-oikumenea/internal/identityfederation/domain"
	"github.com/olehmushka/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// Service adapts *application.Service to the generated identityapi.IdentityFederationService
// interface.
type Service struct {
	app *application.Service
	pep *pep.Enforcer
	// issuers is the static set of accepted IdP issuers (install config, public fields only — never
	// the HS256 secrets), surfaced by ListIssuers so binding UIs offer a choice instead of free text.
	issuers []identityapi.IssuerOption
}

// NewService builds the transport adapter over the identity-federation application service, the PEP
// enforcer, and the configured issuers (public fields only) offered to binding UIs via ListIssuers.
func NewService(app *application.Service, enforcer *pep.Enforcer, issuers []identityapi.IssuerOption) Service {
	return Service{app: app, pep: enforcer, issuers: issuers}
}

// compile-time assertion that the transport satisfies the generated server interface.
var _ identityapi.IdentityFederationService = Service{}

func (s Service) GetAccount(ctx context.Context, token bearertoken.Token, accountID string) (identityapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonRead)); err != nil {
		return identityapi.Account{}, err
	}
	a, err := s.app.GetAccount(ctx, accountID)
	if err != nil {
		return identityapi.Account{}, s.mapError(ctx, err, errCtx{accountID: accountID})
	}
	return toAPIAccount(a), nil
}

// ListIssuers returns the instance's accepted IdP issuers (public fields only) for binding UIs. It is
// a read of static config gated on person.read — the same surface the binding views already require.
func (s Service) ListIssuers(ctx context.Context, token bearertoken.Token) ([]identityapi.IssuerOption, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonRead)); err != nil {
		return nil, err
	}
	if s.issuers == nil {
		return []identityapi.IssuerOption{}, nil
	}
	return s.issuers, nil
}

func (s Service) GetAccountByPerson(ctx context.Context, token bearertoken.Token, personID string) (identityapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonRead)); err != nil {
		return identityapi.Account{}, err
	}
	a, err := s.app.GetAccountByPerson(ctx, personID)
	if err != nil {
		return identityapi.Account{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIAccount(a), nil
}

func (s Service) CreateAccount(ctx context.Context, token bearertoken.Token, req identityapi.CreateAccountRequest) (identityapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonUpdate)); err != nil {
		return identityapi.Account{}, err
	}
	account := domain.Account{PersonID: req.PersonId, Email: derefOr(req.Email, "")}
	var initial *domain.ExternalIdentity
	if req.Identity != nil {
		initial = &domain.ExternalIdentity{Issuer: req.Identity.Issuer, Subject: req.Identity.Subject}
	}
	created, err := s.app.CreateAccount(ctx, account, initial)
	if err != nil {
		return identityapi.Account{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIAccount(created), nil
}

func (s Service) DisableAccount(ctx context.Context, token bearertoken.Token, accountID string) (identityapi.Account, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonUpdate)); err != nil {
		return identityapi.Account{}, err
	}
	disabled, err := s.app.DisableAccount(ctx, accountID)
	if err != nil {
		return identityapi.Account{}, s.mapError(ctx, err, errCtx{accountID: accountID})
	}
	return toAPIAccount(disabled), nil
}

func (s Service) LinkIdentity(ctx context.Context, token bearertoken.Token, accountID string, req identityapi.LinkIdentityRequest) (identityapi.ExternalIdentity, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonUpdate)); err != nil {
		return identityapi.ExternalIdentity{}, err
	}
	linked, err := s.app.LinkIdentity(ctx, accountID, domain.ExternalIdentity{Issuer: req.Issuer, Subject: req.Subject})
	if err != nil {
		return identityapi.ExternalIdentity{}, s.mapError(ctx, err, errCtx{accountID: accountID})
	}
	return toAPIIdentity(linked), nil
}

func (s Service) UnlinkIdentity(ctx context.Context, token bearertoken.Token, accountID, identityID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermPersonUpdate)); err != nil {
		return err
	}
	if err := s.app.UnlinkIdentity(ctx, accountID, identityID); err != nil {
		return s.mapError(ctx, err, errCtx{accountID: accountID, identityID: identityID})
	}
	return nil
}

// Whoami returns the caller's resolved PDP context. It is authenticated-only: the subject is read
// from the request context the validation middleware attached (pkg/authn). Absence means the request
// reached here without a validated identity (the middleware would normally have rejected) -> a uniform
// Unauthorized-shaped error.
func (s Service) Whoami(ctx context.Context, _ bearertoken.Token) (identityapi.Whoami, error) {
	subject, ok := authn.FromContext(ctx)
	if !ok || subject.PersonID == "" {
		return identityapi.Whoami{}, werror.WrapWithContextParams(ctx, errUnauthenticated, "resolve caller identity")
	}
	return identityapi.Whoami{
		PersonId:  subject.PersonID,
		AccountId: strPtrOrNil(subject.AccountID),
		Email:     strPtrOrNil(subject.Email),
	}, nil
}

var errUnauthenticated = errors.New("no authenticated subject on the request")

// ---------------------------------------------------------------- response assembly

func toAPIAccount(a domain.Account) identityapi.Account {
	ids := make([]identityapi.ExternalIdentity, 0, len(a.Identities))
	for _, e := range a.Identities {
		ids = append(ids, toAPIIdentity(e))
	}
	return identityapi.Account{
		Id:         a.ID,
		PersonId:   a.PersonID,
		Email:      strPtrOrNil(a.Email),
		Status:     string(a.Status),
		Identities: ids,
		CreatedAt:  datetime.DateTime(a.CreatedAt),
		UpdatedAt:  datetime.DateTime(a.UpdatedAt),
	}
}

func toAPIIdentity(e domain.ExternalIdentity) identityapi.ExternalIdentity {
	return identityapi.ExternalIdentity{
		Id:        e.ID,
		AccountId: e.AccountID,
		Issuer:    e.Issuer,
		Subject:   e.Subject,
		CreatedAt: datetime.DateTime(e.CreatedAt),
	}
}

// ---------------------------------------------------------------- error mapping

type errCtx struct {
	accountID   string
	identityID  string
	principalID string
}

// mapError translates domain/application errors into the Conjure SerializableError contract.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrAccountNotFound):
		return identityapi.NewAccountNotFound(c.accountID)
	case errors.Is(err, domain.ErrAccountConflict):
		return identityapi.NewAccountConflict("the person already has an active account or the email is taken")
	case errors.Is(err, domain.ErrUnknownPerson):
		return identityapi.NewAccountInvalid("person does not exist")
	case errors.Is(err, domain.ErrAccountInvalid):
		return identityapi.NewAccountInvalid(err.Error())
	case errors.Is(err, domain.ErrIdentityNotFound):
		return identityapi.NewIdentityNotFound(c.identityID)
	case errors.Is(err, domain.ErrIdentityConflict):
		return identityapi.NewIdentityConflict("the (issuer, subject) is already linked to an account")
	case errors.Is(err, domain.ErrLinkingDisabled):
		return identityapi.NewIdentityConflict("additional identity linking is disabled for this account")
	case errors.Is(err, domain.ErrIdentityInvalid):
		return identityapi.NewIdentityInvalid(err.Error())
	case errors.Is(err, domain.ErrPrincipalNotFound):
		return identityapi.NewServicePrincipalNotFound(c.principalID)
	case errors.Is(err, domain.ErrPrincipalConflict):
		return identityapi.NewServicePrincipalConflict("the code or (issuer, subject) is already taken — an (issuer, subject) is a person identity or a service principal, never both")
	case errors.Is(err, domain.ErrPrincipalIdentityImmutable):
		return identityapi.NewServicePrincipalInvalid("a service principal's (issuer, subject) is immutable; register a new principal instead")
	case errors.Is(err, domain.ErrPrincipalInvalid):
		return identityapi.NewServicePrincipalInvalid(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "identity-federation request failed")
	}
}

// ---------------------------------------------------------------- value helpers

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
