// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Sentinel errors for the service-principal registry (M51 / D-ServiceIdentities), mapped to Conjure
// SerializableErrors by the transport. The DB constraints (the active (issuer, subject) and code
// unique indexes, the symmetric external-identity collision triggers) enforce the same shapes as a
// backstop.
var (
	ErrPrincipalNotFound = errors.New("service principal not found")
	// ErrPrincipalConflict: the code is taken, or the (issuer, subject) already names a principal OR
	// a person external identity — an (issuer, subject) is one or the other, never both.
	ErrPrincipalConflict = errors.New("the service principal code or (issuer, subject) is already taken")
	ErrPrincipalInvalid  = errors.New("invalid service principal request")
	// ErrPrincipalIdentityImmutable: (issuer, subject) is the principal's identity key and cannot be
	// re-pointed after registration — register a new principal and revoke the old one instead.
	ErrPrincipalIdentityImmutable = errors.New("a service principal's (issuer, subject) is immutable")
)

// PrincipalStatus is the machine subject's lifecycle state. A disabled principal fails resolution, so
// its tokens stop working immediately without deleting the audit trail that references it.
type PrincipalStatus string

const (
	PrincipalActive   PrincipalStatus = "active"
	PrincipalDisabled PrincipalStatus = "disabled"
)

// ServicePrincipal is a MACHINE subject — a facade with standing of its own (M52) or a connector
// (M53) — authenticated by the external IdP's client-credentials grant on the same (issuer, subject)
// key shape as an ExternalIdentity, and resolved by the same middleware.
//
// A principal holds no role assignment and has NO UNIT REACH: its authority is the flat
// per-principal grant set owned by the authorization module (D-ServiceIdentities). ClientID is a
// display label projected from the token's azp/client_id claim so an operator can see which IdP
// client a principal is; it is NEVER an authorization input.
type ServicePrincipal struct {
	ID          string
	Code        string // stable, locale-agnostic machine name (D-Code)
	Name        string
	Description string
	Issuer      string
	Subject     string
	ClientID    string // "" when unset; display only
	Status      PrincipalStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate enforces the registration invariants: a code, a name, and the (issuer, subject) identity
// key the middleware resolves on.
func (p ServicePrincipal) Validate() error {
	if strings.TrimSpace(p.Code) == "" {
		return wrapInvalid(ErrPrincipalInvalid, "code is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return wrapInvalid(ErrPrincipalInvalid, "name is required")
	}
	if strings.TrimSpace(p.Issuer) == "" {
		return wrapInvalid(ErrPrincipalInvalid, "issuer is required")
	}
	if strings.TrimSpace(p.Subject) == "" {
		return wrapInvalid(ErrPrincipalInvalid, "subject is required")
	}
	return nil
}

// PrincipalResolution is the machine counterpart of Resolution: what a validated client-credentials
// token maps to. The middleware turns this into a service subject (no person, no account, no reach).
type PrincipalResolution struct {
	PrincipalID string
	Code        string
	ClientID    string
}

// PrincipalRepository is the persistence port for the registry. Reads exclude soft-deleted rows.
type PrincipalRepository interface {
	InsertPrincipal(ctx context.Context, p ServicePrincipal) (ServicePrincipal, error)
	GetPrincipal(ctx context.Context, id string) (ServicePrincipal, error)
	GetPrincipalByCode(ctx context.Context, code string) (ServicePrincipal, error)
	// UpdatePrincipal changes only the mutable fields (name, description, client_id); the identity
	// key is immutable by design.
	UpdatePrincipal(ctx context.Context, p ServicePrincipal) (ServicePrincipal, error)
	SetPrincipalStatus(ctx context.Context, id string, status PrincipalStatus) (ServicePrincipal, error)
	ListPrincipals(ctx context.Context, afterID string, limit int) ([]ServicePrincipal, error)

	// ResolvePrincipalBySubject maps a verified (issuer, subject) to an ACTIVE principal, or
	// ErrPrincipalNotFound when unregistered, disabled, or soft-deleted.
	ResolvePrincipalBySubject(ctx context.Context, issuer, subject string) (PrincipalResolution, error)
	// PrincipalIsActive backs the authorization module's PrincipalDirectory port, so a grant write can
	// validate its principal without reading another module's table.
	PrincipalIsActive(ctx context.Context, id string) (bool, error)
}
