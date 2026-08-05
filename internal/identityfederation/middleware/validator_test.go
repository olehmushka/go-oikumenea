// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer = "https://local-dev.oikumenea.test"
	testAud    = "oikumenea"
	testKey    = "local-dev-insecure-signing-key-change-me"
)

func testValidator(jitClaim string) *Validator {
	return NewValidator(Config{
		Issuers:   []IssuerConfig{{Issuer: testIssuer, Audiences: []string{testAud}, Type: IssuerHS256, HMACKey: testKey}},
		ClockSkew: 60 * time.Second,
		JITClaim:  jitClaim,
	})
}

func mintHS256(t *testing.T, key string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := tok.SignedString([]byte(key))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func baseClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": testIssuer,
		"sub": "local-admin",
		"aud": testAud,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
}

func TestValidatorHS256Accepts(t *testing.T) {
	v := testValidator("person_code")
	claims := baseClaims()
	claims["email"] = "admin@example.test"
	claims["person_code"] = "admin"
	raw := mintHS256(t, testKey, claims)

	got, err := v.Validate(context.Background(), raw)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if got.Issuer != testIssuer || got.Subject != "local-admin" {
		t.Fatalf("unexpected claims: %+v", got)
	}
	if got.Email != "admin@example.test" || got.JITValue != "admin" {
		t.Fatalf("email/JIT claim not projected: %+v", got)
	}
}

func TestValidatorRejects(t *testing.T) {
	v := testValidator("")

	t.Run("wrong key", func(t *testing.T) {
		raw := mintHS256(t, "the-wrong-key", baseClaims())
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected rejection of token signed with the wrong key")
		}
	})

	t.Run("unknown issuer", func(t *testing.T) {
		c := baseClaims()
		c["iss"] = "https://evil.example"
		raw := mintHS256(t, testKey, c)
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected rejection of an unconfigured issuer")
		}
	})

	t.Run("wrong audience", func(t *testing.T) {
		c := baseClaims()
		c["aud"] = "some-other-service"
		raw := mintHS256(t, testKey, c)
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected rejection of a wrong audience")
		}
	})

	t.Run("expired", func(t *testing.T) {
		c := baseClaims()
		c["exp"] = time.Now().Add(-2 * time.Hour).Unix()
		raw := mintHS256(t, testKey, c)
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected rejection of an expired token")
		}
	})

	t.Run("alg confusion (none)", func(t *testing.T) {
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims())
		raw, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("sign none: %v", err)
		}
		if _, err := v.Validate(context.Background(), raw); err == nil {
			t.Fatal("expected rejection of an unsigned (alg=none) token")
		}
	})
}

// TestValidatorMultiAudience pins the SET semantics of an issuer's configured audiences: one issuer
// may serve several clients of the same deployment (console + CLI), and a token matching ANY of them
// validates while an unlisted one does not. The alternative reading — requiring a token to carry all
// configured audiences — would make a second client unreachable, so this is a behavioural lock, not a
// restatement of the code.
func TestValidatorMultiAudience(t *testing.T) {
	const consoleAud, cliAud = "oikumenea-console", "oikumenea-cli"
	v := NewValidator(Config{
		Issuers:   []IssuerConfig{{Issuer: testIssuer, Audiences: []string{consoleAud, cliAud}, Type: IssuerHS256, HMACKey: testKey}},
		ClockSkew: 60 * time.Second,
	})

	for _, aud := range []string{consoleAud, cliAud} {
		c := baseClaims()
		c["aud"] = aud
		if _, err := v.Validate(context.Background(), mintHS256(t, testKey, c)); err != nil {
			t.Fatalf("audience %q is configured but was rejected: %v", aud, err)
		}
	}

	t.Run("unlisted audience rejected", func(t *testing.T) {
		c := baseClaims()
		c["aud"] = "some-third-party-app"
		if _, err := v.Validate(context.Background(), mintHS256(t, testKey, c)); err == nil {
			t.Fatal("expected rejection of an audience outside the configured set")
		}
	})

	t.Run("multi-valued aud claim matches on intersection", func(t *testing.T) {
		c := baseClaims()
		c["aud"] = []string{"unrelated", cliAud}
		if _, err := v.Validate(context.Background(), mintHS256(t, testKey, c)); err != nil {
			t.Fatalf("token whose aud list contains a configured audience was rejected: %v", err)
		}
	})
}

// TestGuardIssuerAudience locks the fail-closed boot rule behind the multi-IdP examples: a PUBLIC
// issuer's `iss` is shared by every application registered with it, so an oidc issuer with no pinned
// audience would accept an ID token minted for an unrelated third-party app and resolve it to the
// linked person. hs256 is exempt (local/dev only, deployment-private key).
func TestGuardIssuerAudience(t *testing.T) {
	cases := []struct {
		name    string
		issuers []IssuerConfig
		wantErr bool
	}{
		{
			"oidc without audience rejected",
			[]IssuerConfig{{Issuer: "https://accounts.google.com", Type: IssuerOIDC}},
			true,
		},
		{
			"oidc with empty-string audience rejected",
			[]IssuerConfig{{Issuer: "https://accounts.google.com", Type: IssuerOIDC, Audiences: []string{}}},
			true,
		},
		{
			"defaulted (empty) type counts as oidc",
			[]IssuerConfig{{Issuer: "https://accounts.google.com"}},
			true,
		},
		{
			"oidc with audience allowed",
			[]IssuerConfig{{Issuer: "https://accounts.google.com", Type: IssuerOIDC, Audiences: []string{"client-id.apps.googleusercontent.com"}}},
			false,
		},
		{
			"hs256 without audience allowed",
			[]IssuerConfig{{Issuer: testIssuer, Type: IssuerHS256, HMACKey: testKey}},
			false,
		},
		{
			"one bad issuer among good ones is rejected",
			[]IssuerConfig{
				{Issuer: testIssuer, Type: IssuerHS256, HMACKey: testKey},
				{Issuer: "https://ok.example", Type: IssuerOIDC, Audiences: []string{"a"}},
				{Issuer: "https://accounts.google.com", Type: IssuerOIDC},
			},
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := GuardIssuerAudience(tc.issuers)
			if tc.wantErr && err == nil {
				t.Fatal("expected the guard to refuse this issuer set")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("guard unexpectedly refused: %v", err)
			}
		})
	}
}

func TestGuardSymmetricIssuers(t *testing.T) {
	hs256 := []IssuerConfig{{Issuer: testIssuer, Type: IssuerHS256, HMACKey: testKey}}
	oidc := []IssuerConfig{{Issuer: testIssuer, Type: IssuerOIDC}}

	cases := []struct {
		name    string
		issuers []IssuerConfig
		env     string
		wantErr bool
	}{
		{"hs256 in prod rejected", hs256, "prod", true},
		{"hs256 in staging rejected", hs256, "staging", true},
		{"hs256 in empty env rejected", hs256, "", true},
		{"hs256 in unknown env rejected", hs256, "production", true},
		{"hs256 in local allowed", hs256, "local", false},
		{"hs256 in dev allowed", hs256, "dev", false},
		{"oidc in prod allowed", oidc, "prod", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := GuardSymmetricIssuers(tc.issuers, tc.env)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for env %q with %s", tc.env, tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for env %q: %v", tc.env, err)
			}
		})
	}
}
