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
		Issuers:   []IssuerConfig{{Issuer: testIssuer, Audience: testAud, Type: IssuerHS256, HMACKey: testKey}},
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
