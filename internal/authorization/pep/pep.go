// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package pep is the Policy Enforcement Point seam every module's transport calls before a guarded
// operation. It resolves the acting subject from the request CONTEXT, asks the authorization PDP
// (in-process), and returns the shared Conjure Authorization:PermissionDenied on denial. Putting the
// subject resolution + denial mapping here keeps it in ONE place.
//
// SUBJECT RESOLUTION (M8): the acting person RID is read from the request context via pkg/authn. The
// identity-federation validation middleware (OIDC/JWKS) verifies the inbound token, maps
// (issuer, subject) → account → person, and attaches the resolved subject there
// (identity-federation.md step 4). There is no implicit "authenticated ⇒ may act" exemption — an
// absent subject is denied (read is an explicit grant; D-BaseRoles). The `token` parameter is retained
// on the Require* methods purely for call-site stability (the M7 transports already thread it
// through); the subject now comes from the context, so the parameter is unused.
//
// MACHINE SUBJECTS (M51 / D-ServiceIdentities): a request may instead act as a SERVICE PRINCIPAL — a
// facade with standing of its own, or a connector. A principal sets no PersonID, so every
// person-shaped method here (Require, RequireAny, RequireAnywhere, AllowedAnywhere, SubjectAuthority,
// FilterVisibleUnits) denies it structurally, and it pins no RLS connection. Machine access goes
// exclusively through RequireService / RequireServiceOrPerson, which consult the principal's flat
// grant set. Until the RLS service arm lands (M55) that is the whole story: a principal can reach the
// import endpoint and nothing that is unit-scoped or RLS-guarded.
package pep

import (
	"context"
	"errors"

	"github.com/olehmushka/go-oikumenea/internal/authorization/application"
	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	authzapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/authorization"
	"github.com/olehmushka/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
)

// Enforcer wraps the authorization application service for use as a PEP from any module's transport.
//
// It supports late binding: tenant/person/etc. register their routes (taking the shared enforcer)
// BEFORE the authorization service can be built (the PDP needs tenant's closure), so the composition
// root creates one unbound Enforcer, threads it everywhere, and Binds the service once authz is
// constructed — all within the boot InitFunc, before any request is served.
type Enforcer struct {
	svc *application.Service
}

// New builds an already-bound Enforcer over the authorization application service (used in tests).
func New(svc *application.Service) *Enforcer { return &Enforcer{svc: svc} }

// NewUnbound builds an Enforcer whose service is wired later via Bind (composition-root ordering).
func NewUnbound() *Enforcer { return &Enforcer{} }

// Bind wires the authorization service into a previously-unbound Enforcer. Called once at boot.
func (e *Enforcer) Bind(svc *application.Service) { e.svc = svc }

// MustBeBound reports whether the enforcer was wired via Bind. The composition root calls it at boot
// (review-2026-07 R-11) so a forgotten Bind fails startup instead of surfacing as a request-time nil.
func (e *Enforcer) MustBeBound() error {
	if e.svc == nil {
		return errors.New("authorization enforcer (pep) not bound: authorization.Register must Bind it before serving")
	}
	return nil
}

// Subject resolves the acting person RID from the request context (the subject the
// identity-federation middleware attached via pkg/authn). Returns "" when the request carries no
// authenticated subject. The authorization transport reads this for grant/revoke provenance
// (granted_by / revoked_by).
func Subject(ctx context.Context) string { return authn.PersonID(ctx) }

// Require enforces `action` at `unitID` for the request's subject. unitID is "" for instance-scope
// actions. Returns Authorization:PermissionDenied when the subject is absent or the PDP denies.
func (e *Enforcer) Require(ctx context.Context, token bearertoken.Token, action, unitID string) error {
	subject := Subject(ctx)
	if subject == "" {
		return authzapi.NewPermissionDenied(action)
	}
	if err := e.svc.Enforce(ctx, subject, action, unitID); err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			return authzapi.NewPermissionDenied(action)
		}
		return err
	}
	return nil
}

// RequireAny enforces that the token's subject satisfies AT LEAST ONE of `actions` at `unitID` —
// used for the per-graph-OR-broad edge permission (D-EdgePerms): unit.edges.<graph>.manage OR the
// broad unit.edges.manage. Returns Authorization:PermissionDenied (naming the first action) when none
// pass.
func (e *Enforcer) RequireAny(ctx context.Context, token bearertoken.Token, unitID string, actions ...string) error {
	subject := Subject(ctx)
	if subject == "" || len(actions) == 0 {
		return authzapi.NewPermissionDenied(firstOr(actions))
	}
	for _, action := range actions {
		err := e.svc.Enforce(ctx, subject, action, unitID)
		if err == nil {
			return nil
		}
		if !errors.Is(err, domain.ErrPermissionDenied) {
			return err
		}
	}
	return authzapi.NewPermissionDenied(actions[0])
}

func firstOr(actions []string) string {
	if len(actions) == 0 {
		return ""
	}
	return actions[0]
}

// SubjectAuthority returns the request subject's person RID and instance-admin flag, resolved
// through the request authority snapshot / grant cache — ZERO queries on the request path
// (review-2026-07 R-02.1; replaces the deleted EffectiveReach: read surfaces no longer materialize
// a reach set, they pass the subject to SQL semi-join queries and short-circuit on the admin flag).
// An absent subject yields ("", false), never an error — the permission precondition is enforced
// separately by RequireAnywhere.
func (e *Enforcer) SubjectAuthority(ctx context.Context) (string, bool, error) {
	// A machine subject has no person identity and no reach (M51 / D-ServiceIdentities), so it can
	// never be instance admin. Stated explicitly rather than relying on PersonID happening to be
	// empty, so a future change to Subject cannot silently promote a principal.
	if authn.IsService(ctx) {
		return "", false, nil
	}
	subject := Subject(ctx)
	if subject == "" {
		return "", false, nil
	}
	isAdmin, err := e.svc.IsInstanceAdminFor(ctx, subject)
	return subject, isAdmin, err
}

// FilterVisibleUnits applies the shadow-visibility gate (owned by authorization, patterns.md): from
// `candidates`, drop `shadow` units the request subject's *.read does not reach; `public` units and
// reachable units pass, preserving input order. `shadow` reports per unit id whether it is shadow.
// Tenant's list/ancestors/descendants reads call it as the authoritative second pass after the
// permission decision (the tenant_units public-read RLS policy is its DB-level mirror). Call sites
// gate on RequireAnywhere/Require first, so the subject is non-empty here.
func (e *Enforcer) FilterVisibleUnits(ctx context.Context, candidates []string, shadow map[string]bool) ([]string, error) {
	// Unlike the Require* methods this has no empty-subject guard of its own (call sites gate first),
	// so deny a machine subject explicitly: a principal has no reach, and passing its empty person id
	// through would ask the shadow gate a question about "the person with no RID" (M51).
	if authn.IsService(ctx) {
		return nil, nil
	}
	return e.svc.FilterVisibleUnits(ctx, Subject(ctx), candidates, shadow)
}

// FilterVisibleOrgs is the ORGANIZATION shadow gate — the same second pass FilterVisibleUnits makes,
// over a reach that is DERIVED rather than assigned: an organization is reachable when any of its
// live units is. Organizations cannot be granted directly (an assignment's target_unit_id FKs
// tenant_units), so before M58 ticket 4 their reach set was empty by construction and every shadow
// organization was instance-admin-only — an accident of the assignment table's shape, not a decision.
//
// Kept a separate method rather than folded into FilterVisibleUnits with a flag: the two answer
// different questions ("is this unit in reach" vs "is any unit of this org in reach"), and a shared
// entry point taking a mode would make the caller's choice invisible at the call site, which is
// exactly how the gate came to be applied to organization RIDs in the first place.
func (e *Enforcer) FilterVisibleOrgs(ctx context.Context, candidates []string, shadow map[string]bool) ([]string, error) {
	// Same machine-subject denial as FilterVisibleUnits, for the same reason: a principal has no unit
	// reach, so it has no derived organization reach either (M51).
	if authn.IsService(ctx) {
		return nil, nil
	}
	return e.svc.FilterVisibleOrgs(ctx, Subject(ctx), candidates, shadow)
}

// ============================ machine subjects (M51 / D-ServiceIdentities) ============================

// RequireService enforces `action` for a MACHINE subject (a facade with standing of its own, or a
// connector). A principal's authority is a flat per-principal grant set — no roles, no unit reach —
// so this consults the grants directly and never the PDP (a principal decision is a grant match, not
// a DAG traversal).
//
// orgID "" means the request is NOT organization-qualified and only an INSTANCE-WIDE grant satisfies
// it; an org-confined connector is denied there, because such an endpoint could otherwise reach data
// outside its organization. A non-empty orgID is satisfied by an instance-wide grant OR one naming
// that organization. A non-machine (or absent) subject is denied.
func (e *Enforcer) RequireService(ctx context.Context, token bearertoken.Token, action, orgID string) error {
	principalID := authn.PrincipalID(ctx)
	if principalID == "" {
		return authzapi.NewPermissionDenied(action)
	}
	ok, err := e.svc.HoldsPrincipalPermission(ctx, principalID, action, orgID)
	if err != nil {
		return err
	}
	if !ok {
		return authzapi.NewPermissionDenied(action)
	}
	return nil
}

// RequireServiceOrPerson gates a surface reachable by BOTH a machine and a human instance admin,
// dispatching on the actor kind: a principal is checked against its grants, a person against the PDP
// on the instance plane.
func (e *Enforcer) RequireServiceOrPerson(ctx context.Context, token bearertoken.Token, action, orgID string) error {
	if authn.IsService(ctx) {
		return e.RequireService(ctx, token, action, orgID)
	}
	return e.RequireAnywhere(ctx, token, action)
}

// RequireServiceOrTarget gates a UNIT-SCOPED WRITE reachable by both a machine and a person, WITHOUT
// widening the person's check the way RequireServiceOrPerson would: a person subject goes through the
// exact same target-scoped Require(action, unitID) as an unguarded call, so their authority stays
// bound to units their assignment actually reaches. A machine subject has no unit reach at all (M51 /
// D-ServiceIdentities), so it is checked against its flat grant set via RequireService with orgID ""
// — only an INSTANCE-WIDE grant satisfies it, unitID is not consulted for a machine caller.
//
// This is deliberate, not an oversight (GH-39): principal grants carry no unit/subtree scope today,
// so there is no narrower shape to check a machine against. A grant made to satisfy one intended
// subtree is instance-wide in practice — the calling application, not the permission model, is what
// keeps it pointed at the right units. Do not "fix" this by passing unitID's owning org as orgID
// without first reading GH-39: that would silently invent an org-scoping story RequireService's
// contract does not promise for unit-shaped actions (orgID there means a tenant ORGANIZATION, not a
// unit), and would not close the actual scoping gap the issue leaves open.
func (e *Enforcer) RequireServiceOrTarget(ctx context.Context, token bearertoken.Token, action, unitID string) error {
	if authn.IsService(ctx) {
		return e.RequireService(ctx, token, action, "")
	}
	return e.Require(ctx, token, action, unitID)
}

// RequireImport gates the generic data-import endpoint (M16 / D-Hermenea). Since M51 the importer is
// a REGISTERED service principal holding an `import.manage` grant like any other — the hard-coded
// `hermenea-importer` exemption is gone, so a machine's import rights are grantable and revocable. A
// human instance admin holding `import.manage` may still call it.
//
// orgID is the import target's owning organization, or "" for an instance-wide reference catalog
// (every object type accepted before M55). An empty orgID demands an INSTANCE-WIDE grant, so an
// org-confined connector cannot smuggle reference data; a non-empty orgID (M55 / the RLS service arm)
// is satisfied by an instance-wide grant OR one naming that same org, so an org-confined connector
// imports its own organization's data and a foreign-org grant is rejected. The rows the handler then
// writes are additionally gated at the DB by the reach predicate's principal arm (migration 0042).
func (e *Enforcer) RequireImport(ctx context.Context, token bearertoken.Token, orgID string) error {
	return e.RequireServiceOrPerson(ctx, token, string(domain.PermImportManage), orgID)
}

// AllowedAnywhere is the non-erroring probe form of RequireAnywhere: whether the request subject
// holds `action` at some unit (or on the instance plane). Cross-type surfaces (unified search,
// D-UnifiedSearch) use it to SKIP a per-type provider the subject cannot read rather than fail the
// whole request. An absent subject is simply not allowed (false, nil).
func (e *Enforcer) AllowedAnywhere(ctx context.Context, token bearertoken.Token, action string) (bool, error) {
	subject := Subject(ctx)
	if subject == "" {
		return false, nil
	}
	return e.svc.HoldsPermissionAnywhere(ctx, subject, action)
}

// RequireAnywhere enforces that the token's subject can satisfy `action` at some unit (or on the
// instance plane) — the gate for instance-global reads whose resource is not unit-keyed.
func (e *Enforcer) RequireAnywhere(ctx context.Context, token bearertoken.Token, action string) error {
	subject := Subject(ctx)
	if subject == "" {
		return authzapi.NewPermissionDenied(action)
	}
	ok, err := e.svc.HoldsPermissionAnywhere(ctx, subject, action)
	if err != nil {
		return err
	}
	if !ok {
		return authzapi.NewPermissionDenied(action)
	}
	return nil
}
