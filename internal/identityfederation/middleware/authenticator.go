package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/authn"
)

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

// RLSResolver computes the per-request RLS backstop GUC state for a resolved subject
// (D-RLSDefenseInDepth). The authorization application service satisfies it (RLSStateFor).
type RLSResolver interface {
	RLSStateFor(ctx context.Context, personID string) (db.RLSState, error)
}

// Authenticator is the inbound-token validation middleware (installed via server.WithMiddleware). It
// supports LATE BINDING: the composition root registers it on the server before Start, then Binds the
// validator + resolver once the DB pool and services exist inside the boot InitFunc — all before any
// request is served (mirrors the PEP enforcer's bootstrap-ordering pattern).
type Authenticator struct {
	mu    sync.RWMutex
	bound *bound
}

type bound struct {
	validator  *Validator
	resolver   Resolver
	persons    PersonDirectory
	jitEnabled bool
	rls        RLSResolver
	pool       *pgxpool.Pool
}

// NewUnbound builds an Authenticator whose validator/resolver are wired later via Bind.
func NewUnbound() *Authenticator { return &Authenticator{} }

// Bind wires the validator, the (issuer, subject) resolver, the person directory (for JIT), the
// JIT-enabled flag, the RLS-state resolver, and the pool used to pin a per-request RLS-scoped
// connection (D-RLSDefenseInDepth). Called once at boot.
func (a *Authenticator) Bind(validator *Validator, resolver Resolver, persons PersonDirectory, jitEnabled bool, rls RLSResolver, pool *pgxpool.Pool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.bound = &bound{validator: validator, resolver: resolver, persons: persons, jitEnabled: jitEnabled, rls: rls, pool: pool}
}

func (a *Authenticator) snapshot() *bound {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.bound
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
	claims, err := b.validator.Validate(r.Context(), raw)
	if err != nil {
		unauthorized(rw)
		return
	}
	res, err := b.resolve(r.Context(), claims)
	if err != nil {
		unauthorized(rw)
		return
	}
	ctx := authn.NewContext(r.Context(), authn.Subject{PersonID: res.PersonID, AccountID: res.AccountID, Email: res.Email})

	// RLS backstop (D-RLSDefenseInDepth): compute the subject's read/write unit reach and pin a
	// connection with the app.* GUCs set, so unit-scoped reads/writes are filtered at the DB even if a
	// handler forgets the PDP/shadow-gate filter. Reach is computed on the bare pool (its reads hit
	// only non-RLS tables + the exempt closure), then the request runs on the pinned connection.
	if b.rls != nil && b.pool != nil {
		state, err := b.rls.RLSStateFor(ctx, res.PersonID)
		if err != nil {
			serverError(rw)
			return
		}
		conn, release, err := db.AcquireScoped(ctx, b.pool, state)
		if err != nil {
			serverError(rw)
			return
		}
		defer release()
		ctx = db.WithConn(ctx, conn)
	}

	next.ServeHTTP(rw, r.WithContext(ctx))
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
