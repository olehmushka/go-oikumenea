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
package pep

import (
	"context"
	"errors"

	"github.com/olegamysk/go-oikumenea/internal/authorization/application"
	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	authzapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/authorization"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
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

// EffectiveReach returns the request subject's effective read/write unit reach (D-PersonReadScope /
// D-RLSDefenseInDepth): the units the acting person may read/write plus the instance-admin flag. Read
// surfaces that project an instance-global resource through the unit graph (person/document
// read-scope) call it to intersect a candidate's active-membership units with the subject's readable
// set. An absent subject yields an empty reach (reads nothing), never an error — the permission
// precondition is enforced separately by RequireAnywhere.
func (e *Enforcer) EffectiveReach(ctx context.Context) (domain.Reach, error) {
	subject := Subject(ctx)
	if subject == "" {
		return domain.Reach{}, nil
	}
	return e.svc.EffectiveReach(ctx, subject)
}

// FilterVisibleUnits applies the shadow-visibility gate (owned by authorization, patterns.md): from
// `candidates`, drop `shadow` units the request subject's *.read does not reach; `public` units and
// reachable units pass, preserving input order. `shadow` reports per unit id whether it is shadow.
// Tenant's list/ancestors/descendants reads call it as the authoritative second pass after the
// permission decision (the tenant_units public-read RLS policy is its DB-level mirror). Call sites
// gate on RequireAnywhere/Require first, so the subject is non-empty here.
func (e *Enforcer) FilterVisibleUnits(ctx context.Context, candidates []string, shadow map[string]bool) ([]string, error) {
	return e.svc.FilterVisibleUnits(ctx, Subject(ctx), candidates, shadow)
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
