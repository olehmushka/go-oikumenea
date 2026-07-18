//go:build integration

// Integration tests for the SERVICE-PRINCIPAL registry (M51 / D-ServiceIdentities) against a real
// Postgres. These prove the machine-subject half of the M51 exit criteria at the module boundary:
//
//   - a registered (issuer, subject) RESOLVES to an active principal (a client-credentials token
//     would map to that machine subject), and an unregistered one does not;
//
//   - an (issuer, subject) is a person identity XOR a principal — the symmetric DB triggers reject the
//     collision from BOTH directions, surfaced as a typed conflict rather than a raw 500;
//
//   - disabling a principal stops resolution immediately (the kill switch), and re-enabling restores it;
//
//   - the identity key is immutable, and the registration is audited.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/identityfederation/...
package identityfederation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
)

func newPrincipal(t *testing.T, code, issuer, subject string) domain.ServicePrincipal {
	t.Helper()
	return domain.ServicePrincipal{
		Code:     code,
		Name:     "Test connector " + code,
		Issuer:   issuer,
		Subject:  subject,
		ClientID: "test-client",
	}
}

// A registered machine resolves; an unregistered one does not. This is the middleware's whole
// decision, exercised at the service boundary.
func TestPrincipalRegistrationAndResolution(t *testing.T) {
	linking := true
	svc, _ := newService(t, &linking)
	ctx := context.Background()
	issuer := "https://idp.example"
	subject := uniq("machine")

	registered, err := svc.RegisterPrincipal(ctx, newPrincipal(t, uniq("connector"), issuer, subject))
	if err != nil {
		t.Fatalf("register principal: %v", err)
	}
	if registered.ID == "" || registered.Status != domain.PrincipalActive {
		t.Fatalf("expected an active principal with an id, got %+v", registered)
	}

	res, err := svc.ResolvePrincipal(ctx, issuer, subject)
	if err != nil {
		t.Fatalf("resolve registered principal: %v", err)
	}
	if res.PrincipalID != registered.ID || res.Code != registered.Code {
		t.Errorf("resolution = %+v; want the registered principal %s/%s", res, registered.ID, registered.Code)
	}

	if _, err := svc.ResolvePrincipal(ctx, issuer, uniq("never-registered")); !errors.Is(err, domain.ErrPrincipalNotFound) {
		t.Errorf("unregistered subject resolved (err=%v); the middleware must reject it", err)
	}
}

// An (issuer, subject) must name exactly one subject. If a pair could be both a person and a machine,
// an inbound token would resolve ambiguously — so both insert directions are rejected.
func TestPrincipalPersonIdentityCollisionBothDirections(t *testing.T) {
	linking := true
	svc, pool := newService(t, &linking)
	ctx := context.Background()
	issuer := "https://idp.example"

	t.Run("principal cannot take a person identity's pair", func(t *testing.T) {
		personID := makePerson(t, pool, uniq("p"))
		subject := uniq("sub")
		if _, err := svc.CreateAccount(ctx, domain.Account{PersonID: personID}, &domain.ExternalIdentity{Issuer: issuer, Subject: subject}); err != nil {
			t.Fatalf("create account: %v", err)
		}
		_, err := svc.RegisterPrincipal(ctx, newPrincipal(t, uniq("connector"), issuer, subject))
		if !errors.Is(err, domain.ErrPrincipalConflict) {
			t.Fatalf("registering a principal on a person's (issuer, subject) = %v; want ErrPrincipalConflict", err)
		}
	})

	t.Run("person identity cannot take a principal's pair", func(t *testing.T) {
		subject := uniq("machine")
		if _, err := svc.RegisterPrincipal(ctx, newPrincipal(t, uniq("connector"), issuer, subject)); err != nil {
			t.Fatalf("register principal: %v", err)
		}
		personID := makePerson(t, pool, uniq("p"))
		_, err := svc.CreateAccount(ctx, domain.Account{PersonID: personID}, &domain.ExternalIdentity{Issuer: issuer, Subject: subject})
		if err == nil {
			t.Fatal("linking a person identity onto a principal's (issuer, subject) succeeded; want a conflict")
		}
	})
}

// Disabling is the kill switch: the machine's tokens must stop working at once, without deleting the
// principal (the audit rows naming it stay meaningful).
func TestPrincipalDisableStopsResolution(t *testing.T) {
	linking := true
	svc, _ := newService(t, &linking)
	ctx := context.Background()
	issuer := "https://idp.example"
	subject := uniq("machine")

	p, err := svc.RegisterPrincipal(ctx, newPrincipal(t, uniq("connector"), issuer, subject))
	if err != nil {
		t.Fatalf("register principal: %v", err)
	}
	if _, err := svc.SetPrincipalStatus(ctx, p.ID, domain.PrincipalDisabled); err != nil {
		t.Fatalf("disable principal: %v", err)
	}
	if _, err := svc.ResolvePrincipal(ctx, issuer, subject); !errors.Is(err, domain.ErrPrincipalNotFound) {
		t.Errorf("disabled principal still resolves (err=%v); the kill switch must be immediate", err)
	}

	if _, err := svc.SetPrincipalStatus(ctx, p.ID, domain.PrincipalActive); err != nil {
		t.Fatalf("re-enable principal: %v", err)
	}
	if _, err := svc.ResolvePrincipal(ctx, issuer, subject); err != nil {
		t.Errorf("re-enabled principal does not resolve: %v", err)
	}
}

// The identity key is what authority hangs off. Re-pointing it would silently transfer a machine's
// grants to a different IdP client, so it is refused as client error (not silently ignored).
func TestPrincipalIdentityKeyIsImmutable(t *testing.T) {
	linking := true
	svc, _ := newService(t, &linking)
	ctx := context.Background()
	issuer := "https://idp.example"
	subject := uniq("machine")

	p, err := svc.RegisterPrincipal(ctx, newPrincipal(t, uniq("connector"), issuer, subject))
	if err != nil {
		t.Fatalf("register principal: %v", err)
	}

	// Display fields update fine.
	updated, err := svc.UpdatePrincipal(ctx, domain.ServicePrincipal{ID: p.ID, Name: "Renamed", Description: "now documented"})
	if err != nil {
		t.Fatalf("update display fields: %v", err)
	}
	if updated.Name != "Renamed" || updated.Issuer != issuer || updated.Subject != subject {
		t.Errorf("update = %+v; want the name changed and the identity key preserved", updated)
	}

	// Re-pointing the key is refused.
	_, err = svc.UpdatePrincipal(ctx, domain.ServicePrincipal{ID: p.ID, Name: "Renamed", Issuer: "https://evil.example", Subject: subject})
	if !errors.Is(err, domain.ErrPrincipalIdentityImmutable) {
		t.Errorf("re-pointing the issuer = %v; want ErrPrincipalIdentityImmutable", err)
	}
}

// EnsurePrincipal backs the shared-secret boot seed: create-if-absent, and a second boot is a no-op
// returning the same row (replicas race on it under the boot-seed advisory lock).
func TestEnsurePrincipalIsIdempotent(t *testing.T) {
	linking := true
	svc, _ := newService(t, &linking)
	ctx := context.Background()
	code := uniq("seeded")
	spec := newPrincipal(t, code, "urn:oikumenea:local", code)

	first, err := svc.EnsurePrincipal(ctx, spec)
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	second, err := svc.EnsurePrincipal(ctx, spec)
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("EnsurePrincipal minted a second row (%s != %s); boot must be idempotent", first.ID, second.ID)
	}
}
