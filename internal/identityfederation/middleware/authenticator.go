package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
	"github.com/palantir/witchcraft-go-logging/wlog/svclog/svc1log"
)

// ReservedLocalIssuer is the synthetic issuer the shared-secret fallback principal is registered
// under (M51 / D-ServiceIdentities). No real token can present it: the validator routes on `iss`
// against the configured issuer set only, and a boot guard refuses startup if an operator configures
// this value as a real issuer.
const ReservedLocalIssuer = "urn:oikumenea:local"

// Resolver maps a verified (issuer, subject) to a PDP subject, and performs the just-in-time
// link-on-match. The identity-federation application service satisfies it.
type Resolver interface {
	Resolve(ctx context.Context, issuer, subject string) (domain.Resolution, error)
	LinkOnMatch(ctx context.Context, personID, issuer, subject, email string) (domain.Resolution, error)
}

// PersonDirectory resolves a token claim value to an existing person (D-JIT: claim -> person.code).
// The person application service satisfies it.
type PersonDirectory interface {
	PersonIDByCode(ctx context.Context, code string) (string, bool, error)
}

// PrincipalResolver maps a verified (issuer, subject) to a MACHINE subject — a registered service
// principal (M51 / D-ServiceIdentities). The identity-federation application service satisfies it.
type PrincipalResolver interface {
	ResolvePrincipal(ctx context.Context, issuer, subject string) (domain.PrincipalResolution, error)
}

// PrincipalAuthorityResolver fetches a machine subject's flat grant set once per request, returning a
// derived context carrying the authorization module's principal snapshot (opaque here) plus the RLS
// backstop GUC state (D-RLSDefenseInDepth) computed from the same identity. Since M55 (the RLS
// service arm) it yields a db.RLSState carrying the PrincipalID, so a machine subject pins a lazy
// RLS-scoped connection like a person does. The authorization application service satisfies it
// (ContextWithPrincipalAuthority).
type PrincipalAuthorityResolver interface {
	ContextWithPrincipalAuthority(ctx context.Context, principalID string) (context.Context, db.RLSState, error)
}

// AuthorityResolver fetches the resolved subject's authority state ONCE per request (review-2026-07
// R-01.1): it returns a derived context carrying the authorization module's request-scoped authority
// snapshot (opaque to this module) plus the RLS backstop GUC state (D-RLSDefenseInDepth) computed
// from the same fetch. The authorization application service satisfies it (ContextWithAuthority).
type AuthorityResolver interface {
	ContextWithAuthority(ctx context.Context, personID string) (context.Context, db.RLSState, error)
}

// Authenticator is the inbound-token validation middleware (installed via server.WithMiddleware). It
// supports LATE BINDING: the composition root registers it on the server before Start, then Binds the
// validator + resolver once the DB pool and services exist inside the boot InitFunc — all before any
// request is served (mirrors the PEP enforcer's bootstrap-ordering pattern).
type Authenticator struct {
	mu sync.RWMutex
	// bound carries the JWT validator + resolver wired at boot.
	bound *bound
	// importToken is the runtime shared secret (HERMENEA_OIKUMENEA_TOKEN) that authenticates the
	// hermenea import SERVICE PRINCIPAL (D-Hermenea; L-AuthzOnly amendment). Empty disables the path.
	// It is validated by constant-time comparison — go-oikumenea stores no credential; the operator
	// supplies it at deploy time (mirrors the bootstrap-admin pattern).
	importToken string
}

type bound struct {
	validator  *Validator
	resolver   Resolver
	persons    PersonDirectory
	jitEnabled bool
	authority  AuthorityResolver
	pool       *pgxpool.Pool
	// The machine-subject arm (M51 / D-ServiceIdentities).
	principals         PrincipalResolver
	principalAuthority PrincipalAuthorityResolver
}

// NewUnbound builds an Authenticator whose validator/resolver are wired later via Bind.
func NewUnbound() *Authenticator { return &Authenticator{} }

// SetImportServiceToken sets the hermenea import service-principal shared secret (D-Hermenea). Called
// once at boot from runtime config/env; an empty value leaves the service-principal path disabled.
func (a *Authenticator) SetImportServiceToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.importToken = token
}

// importServiceToken returns the configured import service secret under the lock.
func (a *Authenticator) importServiceToken() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.importToken
}

// Bind wires the validator, the (issuer, subject) resolver, the person directory (for JIT), the
// JIT-enabled flag, the authority resolver (request-scoped authority snapshot + RLS GUC state), and
// the pool used to pin a per-request RLS-scoped connection (D-RLSDefenseInDepth). Called once at boot.
func (a *Authenticator) Bind(validator *Validator, resolver Resolver, persons PersonDirectory, jitEnabled bool, authority AuthorityResolver, pool *pgxpool.Pool, principals PrincipalResolver, principalAuthority PrincipalAuthorityResolver) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bound = &bound{
		validator: validator, resolver: resolver, persons: persons, jitEnabled: jitEnabled,
		authority: authority, pool: pool,
		principals: principals, principalAuthority: principalAuthority,
	}
}

func (a *Authenticator) snapshot() *bound {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bound
}

// MustBeBound reports whether Bind has wired the validator/resolver. The composition root calls it at
// boot (review-2026-07 R-11) so a forgotten Bind fails startup instead of 401-ing every request.
func (a *Authenticator) MustBeBound() error {
	b := a.snapshot()
	if b == nil {
		return errors.New("identity-federation authenticator not bound: call Bind before serving")
	}
	// The machine arm must be wired too, or client-credentials callers would silently 401 and the
	// shared-secret fallback would lose its grants (M51 / D-ServiceIdentities).
	if b.principals == nil || b.principalAuthority == nil {
		return errors.New("identity-federation authenticator: service-principal resolver not bound")
	}
	return nil
}

// Handle is the wrouter.RequestHandlerMiddleware. It validates the bearer token, resolves the PDP
// subject, attaches it to the request context, and calls next. Management/diagnostic paths
// (/status, /debug) are passed through unauthenticated — the same middleware list also wraps the
// management router (witchcraft multiRootRouter), and health/readiness probes must stay open.
func (a *Authenticator) Handle(rw http.ResponseWriter, r *http.Request, next http.Handler) {
	if isBypassPath(r.URL.Path) {
		next.ServeHTTP(rw, r)
		return
	}
	b := a.snapshot()
	if b == nil {
		unauthorized(rw)
		return
	}
	raw := bearerToken(r)
	if raw == "" {
		unauthorized(rw)
		return
	}

	// Shared-secret fallback (D-Hermenea, retained by D-ServiceIdentities for minimal installs): if
	// the bearer equals the configured import secret, the caller is the hermenea importer. Since M51
	// it resolves through the SAME registry lookup as a client-credentials caller — the boot seeder
	// registers it on the reserved local issuer — so it carries a real principal RID, real grants and
	// real audit attribution instead of being a hard-coded exemption. Constant-time comparison; an
	// empty configured token disables the path.
	if tok := a.importServiceToken(); tok != "" && subtle.ConstantTimeCompare([]byte(raw), []byte(tok)) == 1 {
		ctx, release, ok := b.serviceContext(r.Context(), ReservedLocalIssuer, authn.ServiceHermeneaImporter)
		if !ok {
			// Configured secret but no seeded principal: fail closed rather than granting a bare,
			// grant-less service subject.
			unauthorized(rw)
			return
		}
		defer release()
		next.ServeHTTP(rw, r.WithContext(ctx))
		return
	}

	claims, err := b.validator.Validate(r.Context(), raw)
	if err != nil {
		unauthorized(rw)
		return
	}

	res, err := b.resolve(r.Context(), claims)
	if err != nil {
		// Not a known person identity. Try the machine arm before rejecting: a client-credentials
		// token from a registered service principal resolves here (M51 / D-ServiceIdentities). The
		// person path runs FIRST so the human hot path costs no extra query.
		if errors.Is(err, errInvalidToken) || errors.Is(err, domain.ErrIdentityNotFound) {
			if ctx, release, ok := b.serviceContext(r.Context(), claims.Issuer, claims.Subject); ok {
				defer release()
				next.ServeHTTP(rw, r.WithContext(ctx))
				return
			}
			// Neither a person nor a principal: log the identifying claims (the subject is a machine
			// or IdP id, not a person's name) so an operator can copy the pair straight into a
			// registration call instead of guessing what the IdP sent.
			svc1log.FromContext(r.Context()).Info("rejected unknown token identity",
				svc1log.SafeParam("issuer", claims.Issuer),
				svc1log.SafeParam("authorizedParty", claims.AuthorizedParty),
				svc1log.UnsafeParam("subject", claims.Subject))
		}
		unauthorized(rw)
		return
	}
	ctx := authn.NewContext(r.Context(), authn.Subject{PersonID: res.PersonID, AccountID: res.AccountID, Email: res.Email})

	// Authority snapshot + RLS backstop (R-01.1 / R-03 / D-RLSDefenseInDepth): resolve the subject's
	// authority state once — the returned context carries the snapshot every later PDP call reuses —
	// and install a LAZY RLS-scoped connection holder: the connection is pinned (one GUC round trip)
	// only when a handler first touches an RLS-consuming module, and released after the response iff
	// it was acquired. Unit-scoped reads/writes stay DB-filtered even if a handler forgets the
	// PDP/shadow-gate filter; requests that never touch guarded tables no longer occupy the pool.
	// An acquire failure surfaces as the first scoped statement's error (fails closed at that query).
	if b.authority != nil && b.pool != nil {
		actx, state, err := b.authority.ContextWithAuthority(ctx, res.PersonID)
		if err != nil {
			serverError(rw)
			return
		}
		lctx, release := db.WithLazyConn(actx, b.pool, state)
		defer release()
		ctx = lctx
	}

	next.ServeHTTP(rw, r.WithContext(ctx))
}

// serviceContext resolves (issuer, subject) as a MACHINE subject and returns a context carrying the
// principal + its grant snapshot, or ok=false when the pair names no active principal. It also
// returns a release func the caller MUST defer (a no-op unless a lazy RLS connection was installed).
//
// Since M55 (the RLS service arm) a service request installs a LAZY RLS-scoped connection just like a
// person request: the reach predicate's principal arm (migration 0042) authorizes an org-confined
// grant against that org's RLS-guarded rows, so a principal reaches exactly its organization's data —
// enforced at the DB, not merely by the PEP. The connection is pinned only when a handler first
// touches a guarded table; instance-scope surfaces (wiring reads) never pin one.
func (b *bound) serviceContext(ctx context.Context, issuer, subject string) (context.Context, func(), bool) {
	noop := func() {}
	if b.principals == nil {
		return ctx, noop, false
	}
	res, err := b.principals.ResolvePrincipal(ctx, issuer, subject)
	if err != nil {
		return ctx, noop, false
	}
	sctx := authn.NewContext(ctx, authn.Subject{Service: res.Code, PrincipalID: res.PrincipalID})
	if b.principalAuthority != nil {
		actx, state, err := b.principalAuthority.ContextWithPrincipalAuthority(sctx, res.PrincipalID)
		if err != nil {
			return ctx, noop, false
		}
		sctx = actx
		if b.pool != nil {
			lctx, release := db.WithLazyConn(sctx, b.pool, state)
			return lctx, release, true
		}
	}
	return sctx, noop, true
}

// resolve turns verified claims into a PDP subject: first a direct (issuer, subject) lookup; on an
// unknown identity, just-in-time link-on-match (D-JIT) when enabled — match the configured claim to
// an EXISTING person.code and link; otherwise reject. JIT never creates a person.
func (b *bound) resolve(ctx context.Context, claims Claims) (domain.Resolution, error) {
	res, err := b.resolver.Resolve(ctx, claims.Issuer, claims.Subject)
	if err == nil {
		return res, nil
	}
	if !errors.Is(err, domain.ErrIdentityNotFound) {
		return domain.Resolution{}, err
	}
	if !b.jitEnabled || claims.JITValue == "" {
		return domain.Resolution{}, errInvalidToken
	}
	personID, ok, err := b.persons.PersonIDByCode(ctx, claims.JITValue)
	if err != nil {
		return domain.Resolution{}, err
	}
	if !ok {
		return domain.Resolution{}, errInvalidToken // no match -> reject
	}
	return b.resolver.LinkOnMatch(ctx, personID, claims.Issuer, claims.Subject, claims.Email)
}

// bearerToken extracts the token from the Authorization header (case-insensitive "Bearer " scheme).
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

// isBypassPath reports whether a path belongs to the management/diagnostic surface that must remain
// reachable without authentication (readiness/liveness/health, debug diagnostics).
func isBypassPath(path string) bool {
	return strings.HasPrefix(path, "/status") || strings.HasPrefix(path, "/debug")
}

// unauthorized writes a uniform 401 (no detail about which check failed —
// identity-federation.md invariant).
func unauthorized(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusUnauthorized)
	_, _ = rw.Write([]byte(`{"errorCode":"CUSTOM_CLIENT","errorName":"IdentityFederation:Unauthorized","parameters":{}}`))
}

// serverError writes a generic 500 when the request is authenticated but a server-side step (computing
// reach / pinning the RLS connection) fails — distinct from a 401 so a DB outage is not reported as an
// auth failure. Fails closed: the handler never runs without its RLS-scoped connection.
func serverError(rw http.ResponseWriter) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusInternalServerError)
	_, _ = rw.Write([]byte(`{"errorCode":"INTERNAL","errorName":"Default:Internal","parameters":{}}`))
}
