// Package domain holds the identity-federation module's pure logic: the optional login Account (an
// attachment to a person), the verified (issuer, subject) ExternalIdentity that federates to it, and
// the Repository port it needs from the outside world (overview.md layering). No I/O, no framework
// imports — only the standard library.
//
// go-oikumenea does NOT authenticate (L-AuthzOnly): it stores no credentials and issues no tokens.
// This package owns the directory side of federation (who may log in, mapped to which person); the
// actual token validation (OIDC/JWKS) is the authn middleware in the sibling authn package. An
// account is OPTIONAL (a person may be roster-only) and there is at most one active account per
// person; an account may federate one or several login points, the cap on ADDITIONAL ones being an
// operator config knob enforced in the application, not here.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (the per-person active-account index, the global (issuer, subject) unique index, the person FK)
// enforce the same shapes as a backstop.
var (
	ErrAccountNotFound = errors.New("account not found")
	ErrAccountConflict = errors.New("the person already has an active account, or the email is taken")
	ErrAccountInvalid  = errors.New("invalid account request")
	ErrUnknownPerson   = errors.New("person does not exist")

	ErrIdentityNotFound = errors.New("external identity not found")
	ErrIdentityConflict = errors.New("the (issuer, subject) is already linked to an account")
	ErrIdentityInvalid  = errors.New("invalid identity request")
	// ErrLinkingDisabled: identity linking is disabled (account.identity_linking.enabled = false)
	// and the account already has an active identity; surfaced to the wire as an Identity:Conflict.
	ErrLinkingDisabled = errors.New("additional identity linking is disabled for this account")
)

// AccountStatus is the login-attachment lifecycle state.
type AccountStatus string

const (
	AccountActive   AccountStatus = "active"
	AccountDisabled AccountStatus = "disabled"
)

// Account is the optional login attachment to exactly one person (aggregate root). Email is "" when
// unset. Identities is populated on a read; nil otherwise.
type Account struct {
	ID        string
	PersonID  string
	Email     string // "" when unset
	Status    AccountStatus
	CreatedAt time.Time
	UpdatedAt time.Time

	Identities []ExternalIdentity // populated on a read; nil otherwise
}

// Validate enforces the create-time invariants: a person reference. Unknown person is caught by the
// DB FK and surfaced as ErrUnknownPerson.
func (a Account) Validate() error {
	if strings.TrimSpace(a.PersonID) == "" {
		return wrapInvalid(ErrAccountInvalid, "personId is required")
	}
	return nil
}

// ExternalIdentity is a verified (issuer, subject) login point federating to one account. Immutable
// once created; removed by unlink.
type ExternalIdentity struct {
	ID        string
	AccountID string
	Issuer    string
	Subject   string
	CreatedAt time.Time
}

// Validate enforces a non-empty issuer and subject (the verified IdP claims).
func (e ExternalIdentity) Validate() error {
	if strings.TrimSpace(e.Issuer) == "" {
		return wrapInvalid(ErrIdentityInvalid, "issuer is required")
	}
	if strings.TrimSpace(e.Subject) == "" {
		return wrapInvalid(ErrIdentityInvalid, "subject is required")
	}
	return nil
}

// Resolution is the directory side of a verified inbound token: the active account + person a
// validated (issuer, subject) maps to. The authn middleware turns this into the PDP context.
type Resolution struct {
	PersonID  string
	AccountID string
	Email     string // "" when the account asserts no email
}

func wrapInvalid(base error, msg string) error { return errors.Join(base, errors.New(msg)) }

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted accounts.
type Repository interface {
	// accounts
	InsertAccount(ctx context.Context, a Account) (Account, error)
	GetAccount(ctx context.Context, id string) (Account, error)
	// GetActiveAccountByPerson returns the person's single active account, or ErrAccountNotFound.
	GetActiveAccountByPerson(ctx context.Context, personID string) (Account, error)
	DisableAccount(ctx context.Context, id string) (Account, error)

	// external identities
	InsertIdentity(ctx context.Context, e ExternalIdentity) (ExternalIdentity, error)
	GetIdentity(ctx context.Context, id string) (ExternalIdentity, error)
	DeleteIdentity(ctx context.Context, accountID, id string) error
	ListIdentitiesByAccount(ctx context.Context, accountID string) ([]ExternalIdentity, error)
	CountActiveIdentities(ctx context.Context, accountID string) (int, error)

	// resolution (the inbound-token directory lookup): maps a verified (issuer, subject) to the
	// active account + person, or ErrIdentityNotFound when there is no link or the account is not
	// active.
	ResolveBySubject(ctx context.Context, issuer, subject string) (Resolution, error)
}
