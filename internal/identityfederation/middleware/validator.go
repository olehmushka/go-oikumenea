// Package middleware is the identity-federation inbound-token validation seam (identity-federation.md
// "Inbound token validation"): it verifies a bearer JWT against the configured issuer(s) — OIDC
// discovery + JWKS for production RS256, or a symmetric HS256 key for local-dev — maps the verified
// (issuer, subject) to a PDP subject (account -> person), and attaches it to the request context
// (pkg/authn). go-oikumenea does NOT authenticate (L-AuthzOnly); this validates an identity issued
// elsewhere. It never stores tokens.
package middleware

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
)

// errInvalidToken is the uniform validation failure — callers map every cause (bad signature, wrong
// issuer, expired, unknown identity) to the same Unauthorized, leaking no oracle about which check
// failed (identity-federation.md invariant).
var errInvalidToken = errors.New("invalid inbound token")

// IssuerType selects how an issuer's tokens are verified.
const (
	IssuerOIDC  = "oidc"  // production: OIDC discovery + JWKS (RS256/asymmetric)
	IssuerHS256 = "hs256" // local-dev: symmetric HMAC key from install config
)

// IssuerConfig describes one accepted issuer (install config — ECV).
type IssuerConfig struct {
	Issuer   string // the `iss` value; also the OIDC discovery base URL
	Audience string // expected `aud`; empty skips the audience check
	Type     string // IssuerOIDC (default) | IssuerHS256
	HMACKey  string // symmetric verification key for IssuerHS256 (secret)
}

// Config is the validator's configuration: the accepted issuers + the JIT mapping.
type Config struct {
	Issuers    []IssuerConfig
	ClockSkew  time.Duration
	JITEnabled bool
	JITClaim   string // token claim whose value maps to a person.code (D-JIT link-on-match)
}

// Claims is the minimal verified projection the middleware needs: the federation key (issuer,
// subject), the optional asserted email, and the optional JIT claim value.
type Claims struct {
	Issuer   string
	Subject  string
	Email    string
	JITValue string // the configured JIT claim's string value, "" when absent
}

// Validator verifies inbound tokens against the configured issuers. OIDC verifiers are built lazily
// on first use (so a fresh boot does not require the IdP to be reachable) and cached.
type Validator struct {
	cfg      Config
	byIssuer map[string]IssuerConfig

	mu            sync.Mutex
	oidcVerifiers map[string]*oidc.IDTokenVerifier
}

// NewValidator indexes the configured issuers by their `iss` value.
func NewValidator(cfg Config) *Validator {
	idx := make(map[string]IssuerConfig, len(cfg.Issuers))
	for _, ic := range cfg.Issuers {
		idx[ic.Issuer] = ic
	}
	return &Validator{cfg: cfg, byIssuer: idx, oidcVerifiers: map[string]*oidc.IDTokenVerifier{}}
}

// Validate verifies a raw bearer token and returns its claims, or errInvalidToken on any failure. It
// routes on the token's (unverified) `iss` to the matching issuer config, then fully verifies.
func (v *Validator) Validate(ctx context.Context, raw string) (Claims, error) {
	iss, err := unverifiedIssuer(raw)
	if err != nil {
		return Claims{}, errInvalidToken
	}
	ic, ok := v.byIssuer[iss]
	if !ok {
		return Claims{}, errInvalidToken
	}
	switch ic.Type {
	case IssuerHS256:
		return v.validateHS256(raw, ic)
	default:
		return v.validateOIDC(ctx, raw, ic)
	}
}

func (v *Validator) validateHS256(raw string, ic IssuerConfig) (Claims, error) {
	claims := jwt.MapClaims{}
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(ic.Issuer),
		jwt.WithLeeway(v.cfg.ClockSkew),
		jwt.WithExpirationRequired(),
	}
	if ic.Audience != "" {
		opts = append(opts, jwt.WithAudience(ic.Audience))
	}
	keyFunc := func(_ *jwt.Token) (interface{}, error) { return []byte(ic.HMACKey), nil }
	if _, err := jwt.ParseWithClaims(raw, claims, keyFunc, opts...); err != nil {
		return Claims{}, errInvalidToken
	}
	sub, _ := claims.GetSubject()
	return v.project(ic.Issuer, sub, claims), nil
}

func (v *Validator) validateOIDC(ctx context.Context, raw string, ic IssuerConfig) (Claims, error) {
	verifier, err := v.oidcVerifier(ctx, ic)
	if err != nil {
		return Claims{}, errInvalidToken
	}
	tok, err := verifier.Verify(ctx, raw)
	if err != nil {
		return Claims{}, errInvalidToken
	}
	var all map[string]any
	if err := tok.Claims(&all); err != nil {
		return Claims{}, errInvalidToken
	}
	return v.project(tok.Issuer, tok.Subject, all), nil
}

// oidcVerifier lazily builds and caches the OIDC verifier for an issuer (discovery + JWKS).
func (v *Validator) oidcVerifier(ctx context.Context, ic IssuerConfig) (*oidc.IDTokenVerifier, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if ver, ok := v.oidcVerifiers[ic.Issuer]; ok {
		return ver, nil
	}
	provider, err := oidc.NewProvider(ctx, ic.Issuer)
	if err != nil {
		return nil, err
	}
	cfg := &oidc.Config{}
	if ic.Audience != "" {
		cfg.ClientID = ic.Audience
	} else {
		cfg.SkipClientIDCheck = true
	}
	ver := provider.Verifier(cfg)
	v.oidcVerifiers[ic.Issuer] = ver
	return ver, nil
}

// project extracts the fields the middleware needs from a verified claim set. claims is either a
// jwt.MapClaims (HS256) or a decoded map[string]any (OIDC) — both are map[string]any.
func (v *Validator) project(issuer, subject string, claims map[string]any) Claims {
	out := Claims{Issuer: issuer, Subject: subject}
	if e, ok := claims["email"].(string); ok {
		out.Email = e
	}
	if v.cfg.JITClaim != "" {
		if val, ok := claims[v.cfg.JITClaim].(string); ok {
			out.JITValue = val
		}
	}
	return out
}

// unverifiedIssuer reads the `iss` claim WITHOUT verifying the signature — used only to route to the
// right issuer config, after which the token is fully verified. Safe because routing alone grants
// nothing.
func unverifiedIssuer(raw string) (string, error) {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		return "", err
	}
	iss, err := claims.GetIssuer()
	if err != nil {
		return "", err
	}
	return iss, nil
}
