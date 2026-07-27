// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// The machine-subject authority plane (M51 / D-ServiceIdentities).
//
// A service principal — a facade with standing of its own (M52), a connector (M53) — holds FLAT
// per-principal grants, never role assignments, and has NO UNIT REACH. Two properties of the built
// system make the "give the machine a role" reading unimplementable:
//
//   - authz_role_assignments.subject_person_id is a hard person_persons FK; and
//   - the PDP satisfies INSTANCE-SCOPE permissions — which import.manage and every M53 wiring code
//     are — only for instance admins, never through a role (see pdp.go / D-InstanceAdmin). A
//     principal granted a role could therefore not import at all.
//
// So a principal decision is a grant MATCH, not a DAG traversal: the PDP is not involved. That also
// keeps machines out of the reach/RLS hot path built by M47 (D-RLSLiveReach, D-AuthzGrantCache).

var (
	ErrPrincipalGrantNotFound = errors.New("principal grant not found")
	ErrPrincipalGrantConflict = errors.New("an identical active principal grant already exists")
	ErrPrincipalGrantInvalid  = errors.New("invalid principal grant request")
	// ErrUnknownPrincipal: the principal does not exist or is not active, per the PrincipalDirectory
	// port (the registry itself is owned by identity-federation).
	ErrUnknownPrincipal = errors.New("service principal does not exist or is not active")
	// ErrUnknownOrganization: the named organization does not exist (caught by the DB FK).
	ErrUnknownOrganization = errors.New("organization does not exist")
)

// PrincipalGrant is the reified PRINCIPAL_GRANT link: one permission code held by one machine
// subject, optionally confined to one organization.
//
// OrgID "" means INSTANCE-WIDE (reference-catalog imports, the M53 wiring codes). A named
// organization confines the principal to that org's data — the blast-radius boundary for a connector
// is the organization, not the unit, because a scraper feeds one organization
// (D-TenantOrganizations) and has no place in the unit DAG.
type PrincipalGrant struct {
	ID          string
	PrincipalID string
	Permission  Permission
	OrgID       string // "" = instance-wide
	GrantedBy   string
	GrantedAt   time.Time
	RevokedAt   *time.Time
	RevokedBy   string
}

// Active reports whether the grant is still in force. Principal grants carry no expiry (unlike
// D-TimeBoundGrants assignments): a machine's authority is revoked explicitly.
func (g PrincipalGrant) Active() bool { return g.RevokedAt == nil }

// PrincipalGrantInput is a validated request to grant a permission to a machine subject.
type PrincipalGrantInput struct {
	PrincipalID string
	Permission  Permission
	OrgID       string // "" = instance-wide
	GrantedBy   string
}

// Validate enforces a principal, a code from the closed catalog, and nothing else — there is no
// scope/graph/unit to validate, by design.
func (g PrincipalGrantInput) Validate() error {
	if strings.TrimSpace(g.PrincipalID) == "" {
		return wrapInvalid(ErrPrincipalGrantInvalid, "principalId is required")
	}
	if !IsKnownPermission(string(g.Permission)) {
		// Append only the offending code: wrapInvalid joins onto the sentinel, whose own message is
		// already "unknown permission code", so restating it here doubled the rendered `reason`.
		// Same convention as ValidatePermissionSet on the role path.
		return wrapInvalid(ErrUnknownPermission, string(g.Permission))
	}
	return nil
}

// Satisfies reports whether this grant answers `action` for a request touching `orgID`.
//
// An INSTANCE-WIDE grant (OrgID "") satisfies any org context; an org-confined grant satisfies only
// its own organization. A request with no org context (orgID "") demands an instance-wide grant —
// an org-confined connector must not pass an endpoint that is not org-qualified, because the
// endpoint could then reach data outside its organization.
func (g PrincipalGrant) Satisfies(action Permission, orgID string) bool {
	if g.Permission != action {
		return false
	}
	if g.OrgID == "" {
		return true
	}
	return orgID != "" && g.OrgID == orgID
}

// PrincipalDirectory is the cross-module port into identity-federation's registry (CLAUDE.md:
// cross-module queries are direct interface calls, never a join across another module's tables).
// The grant write uses it to reject an unknown or disabled principal before inserting.
type PrincipalDirectory interface {
	PrincipalIsActive(ctx context.Context, principalID string) (bool, error)
}

// PrincipalRepository is the persistence port for the grant plane.
type PrincipalRepository interface {
	InsertPrincipalGrant(ctx context.Context, in PrincipalGrantInput) (PrincipalGrant, error)
	GetPrincipalGrant(ctx context.Context, id string) (PrincipalGrant, error)
	RevokePrincipalGrant(ctx context.Context, id, revokedBy string) (PrincipalGrant, error)
	ListPrincipalGrants(ctx context.Context, principalID string) ([]PrincipalGrant, error)
	// ActiveGrantsForPrincipal is the per-request authority fetch — the service-side counterpart of
	// ActiveGrantsForSubject.
	ActiveGrantsForPrincipal(ctx context.Context, principalID string) ([]PrincipalGrant, error)
}
