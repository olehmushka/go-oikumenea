// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the MACHINE-SUBJECT authority plane (M51 / D-ServiceIdentities) against a
// real Postgres. These prove the authorization half of the M51 exit criteria:
//
//   - a grant is validated against the closed permission catalog AND against the cross-module
//     principal directory (an unknown or disabled machine cannot be granted anything);
//
//   - org scoping bounds the blast radius: an instance-wide grant answers everything, an org-confined
//     grant answers only its own organization and NOT an unqualified request;
//
//   - double-granting is rejected for BOTH the instance-wide and org-scoped shapes (the two partial
//     unique indexes), and a revoke takes effect immediately;
//
//   - every grant/revoke bumps authz_epoch, keeping the "authority mutation bumps the epoch"
//     invariant uniform with the person path (D-AuthzGrantCache).
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/authorization/...
package authorization_test

import (
	"context"
	"errors"
	"testing"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
)

// stubPrincipals satisfies the cross-module PrincipalDirectory port without dragging the
// identity-federation service into this module's tests (the real binding is asserted at boot).
type stubPrincipals struct{ active map[string]bool }

func (s stubPrincipals) PrincipalIsActive(_ context.Context, id string) (bool, error) {
	return s.active[id], nil
}

// seedPrincipal inserts a machine subject directly, since the registry belongs to identity-federation.
func seedPrincipal(t *testing.T, h harness, code string) string {
	t.Helper()
	var id string
	err := h.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.account_service_principals (code, name, issuer, subject)
		 VALUES ($1, $1, 'urn:test', $1) RETURNING id`, code).Scan(&id)
	if err != nil {
		t.Fatalf("seed principal: %v", err)
	}
	return id
}

// testOrgID returns the RID of the organization newHarness seeds (seedOrgSQL), so the org-scoped
// grants below reference a real tenant_organizations row (the grant's org_id is a hard FK).
func testOrgID(t *testing.T, h harness) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.tenant_organizations WHERE code = 'test-org' AND deleted_at IS NULL`).Scan(&id); err != nil {
		t.Fatalf("read seeded test org: %v", err)
	}
	return id
}

func readEpoch(t *testing.T, h harness) int64 {
	t.Helper()
	var epoch int64
	if err := h.pool.QueryRow(context.Background(), `SELECT epoch FROM oikumenea.authz_epoch`).Scan(&epoch); err != nil {
		t.Fatalf("read authz_epoch: %v", err)
	}
	return epoch
}

func TestPrincipalGrantValidation(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	principalID := seedPrincipal(t, h, uniq("connector"))
	h.authz.BindPrincipalDirectory(stubPrincipals{active: map[string]bool{principalID: true}})

	t.Run("unknown permission code rejected", func(t *testing.T) {
		_, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principalID, Permission: "not.a.real.code",
		})
		if !errors.Is(err, authzdomain.ErrUnknownPermission) {
			t.Errorf("granting an unknown code = %v; want ErrUnknownPermission (the catalog is closed)", err)
		}
	})

	t.Run("unknown or disabled principal rejected", func(t *testing.T) {
		_, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: seedPrincipal(t, h, uniq("unregistered")), // seeded but NOT marked active in the stub
			Permission:  authzdomain.PermImportManage,
		})
		if !errors.Is(err, authzdomain.ErrUnknownPrincipal) {
			t.Errorf("granting to an inactive principal = %v; want ErrUnknownPrincipal", err)
		}
	})
}

// The core authorization semantics: what a machine may do, and where.
func TestPrincipalGrantOrgScoping(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	instanceWide := seedPrincipal(t, h, uniq("global-connector"))
	confined := seedPrincipal(t, h, uniq("org-connector"))
	h.authz.BindPrincipalDirectory(stubPrincipals{active: map[string]bool{instanceWide: true, confined: true}})

	orgID := testOrgID(t, h)

	if _, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
		PrincipalID: instanceWide, Permission: authzdomain.PermImportManage,
	}); err != nil {
		t.Fatalf("grant instance-wide: %v", err)
	}
	if _, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
		PrincipalID: confined, Permission: authzdomain.PermImportManage, OrgID: orgID,
	}); err != nil {
		t.Fatalf("grant org-confined: %v", err)
	}

	cases := []struct {
		name        string
		principalID string
		orgID       string
		want        bool
		why         string
	}{
		{"instance-wide, unqualified request", instanceWide, "", true, "reference-catalog imports"},
		{"instance-wide, in an org", instanceWide, orgID, true, "instance-wide covers every org"},
		{"confined, its own org", confined, orgID, true, "the connector's organization"},
		{"confined, unqualified request", confined, "", false,
			"an unqualified endpoint could reach outside the confined org — this is what keeps a church scraper off /import"},
		{"confined, a different org", confined, "00000000-0000-8000-8000-000000000000", false,
			"a scraper must not touch another organization's data"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.authz.HoldsPrincipalPermission(ctx, tc.principalID, string(authzdomain.PermImportManage), tc.orgID)
			if err != nil {
				t.Fatalf("HoldsPrincipalPermission: %v", err)
			}
			if got != tc.want {
				t.Errorf("holds = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}

func TestPrincipalGrantConflictAndRevoke(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	principalID := seedPrincipal(t, h, uniq("connector"))
	h.authz.BindPrincipalDirectory(stubPrincipals{active: map[string]bool{principalID: true}})
	orgID := testOrgID(t, h)

	granted, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
		PrincipalID: principalID, Permission: authzdomain.PermImportManage,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}

	// Both partial unique indexes must reject a duplicate — the instance-wide one matters because a
	// NULL org_id does not de-duplicate under a plain UNIQUE.
	t.Run("duplicate instance-wide rejected", func(t *testing.T) {
		_, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principalID, Permission: authzdomain.PermImportManage,
		})
		if !errors.Is(err, authzdomain.ErrPrincipalGrantConflict) {
			t.Errorf("duplicate instance-wide grant = %v; want ErrPrincipalGrantConflict", err)
		}
	})

	t.Run("duplicate org-scoped rejected", func(t *testing.T) {
		if _, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principalID, Permission: authzdomain.PermPersonRead, OrgID: orgID,
		}); err != nil {
			t.Fatalf("first org-scoped grant: %v", err)
		}
		_, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principalID, Permission: authzdomain.PermPersonRead, OrgID: orgID,
		})
		if !errors.Is(err, authzdomain.ErrPrincipalGrantConflict) {
			t.Errorf("duplicate org-scoped grant = %v; want ErrPrincipalGrantConflict", err)
		}
	})

	t.Run("revoke is immediate", func(t *testing.T) {
		if _, err := h.authz.RevokePrincipalPermission(ctx, granted.ID, ""); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		holds, err := h.authz.HoldsPrincipalPermission(ctx, principalID, string(authzdomain.PermImportManage), "")
		if err != nil {
			t.Fatalf("holds after revoke: %v", err)
		}
		if holds {
			t.Error("principal still holds a revoked permission; machine revocation must be immediate (grants are uncached)")
		}
		// Re-granting after a revoke is allowed: the partial uniques exclude revoked rows.
		if _, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
			PrincipalID: principalID, Permission: authzdomain.PermImportManage,
		}); err != nil {
			t.Errorf("re-grant after revoke rejected: %v", err)
		}
	})
}

// Keeps the "every authority-mutating transaction bumps the epoch" invariant uniform, so any future
// caching of principal grants inherits the D-AuthzGrantCache revocation contract.
func TestPrincipalGrantBumpsAuthzEpoch(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	principalID := seedPrincipal(t, h, uniq("connector"))
	h.authz.BindPrincipalDirectory(stubPrincipals{active: map[string]bool{principalID: true}})

	before := readEpoch(t, h)
	granted, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
		PrincipalID: principalID, Permission: authzdomain.PermImportManage,
	})
	if err != nil {
		t.Fatalf("grant: %v", err)
	}
	afterGrant := readEpoch(t, h)
	if afterGrant <= before {
		t.Errorf("epoch %d -> %d on grant; want a bump", before, afterGrant)
	}

	if _, err := h.authz.RevokePrincipalPermission(ctx, granted.ID, ""); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if afterRevoke := readEpoch(t, h); afterRevoke <= afterGrant {
		t.Errorf("epoch %d -> %d on revoke; want a bump", afterGrant, afterRevoke)
	}
}
