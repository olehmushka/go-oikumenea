// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration test for GH-39: religion.ReligionService.CreateChildOrg (and any future unit-scoped
// write of the same shape) gated on pep.Require alone was structurally unreachable by a service
// principal — Require resolves the acting subject via the person-shaped Subject(ctx), which is always
// empty for a machine (M51 / D-ServiceIdentities), so it denied every machine caller regardless of
// grants. The fix adds pep.Enforcer.RequireServiceOrTarget, a machine door alongside the UNCHANGED
// person-shaped Require — deliberately NOT the existing RequireServiceOrPerson, whose person arm is
// the broader RequireAnywhere ("holds the permission somewhere") rather than a target-scoped check;
// reusing it here would have let any person holding religionorg.manage on an unrelated unit create a
// child org anywhere, a real widening the read-only GH-33/36/37 fixes never had to worry about.
//
// This proves, against a real Postgres and the real PDP/grant paths (not the unbound structural pins
// in internal/authorization/pep/pep_service_test.go):
//   - a machine subject with no grant is denied;
//   - a machine subject with an INSTANCE-WIDE grant is allowed at ANY unit (principal grants carry no
//     unit/subtree scope today — the GH-39 "open question" this issue deliberately leaves open);
//   - a person is allowed at a unit their subtree grant actually reaches;
//   - that SAME person is still denied at an unrelated unit outside their grant — the regression guard
//     that would fail if RequireServiceOrTarget's person arm had been RequireAnywhere instead of
//     Require.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration -run TestRequireServiceOrTarget ./internal/authorization/...
package authorization_test

import (
	"context"
	"testing"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	"github.com/olehmushka/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
)

// seedUnitScopedManageRole creates a custom role carrying `code` — the base roles seeded by
// SeedBaseRoles don't carry a domain-specific permission like religionorg.manage, so tests that need
// one build it directly, mirroring religion's own GH-36 fixture (seedSubtreeOrgManageGrant).
func (h harness) seedUnitScopedManageRole(t *testing.T, code string) string {
	t.Helper()
	var id string
	if err := h.pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'GH-39 role') RETURNING id`,
		uniq("gh39-role")).Scan(&id); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := h.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, $2)`,
		id, code); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	return id
}

func TestRequireServiceOrTarget_MachineGrantIsInstanceWide(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	e := pep.New(h.authz)
	var tok bearertoken.Token
	const action = string(authzdomain.PermReligionOrgManage)

	principalID := seedPrincipal(t, h, uniq("gh39-connector"))
	h.authz.BindPrincipalDirectory(stubPrincipals{active: map[string]bool{principalID: true}})
	someUnit := h.seedUnit(t)
	svcCtx := authn.NewContext(ctx, authn.Subject{Service: "gh39-test", PrincipalID: principalID})

	t.Run("no grant: denied", func(t *testing.T) {
		if err := e.RequireServiceOrTarget(svcCtx, tok, action, someUnit); err == nil {
			t.Fatal("expected denial for a grant-less service principal")
		}
	})

	if _, err := h.authz.GrantPrincipalPermission(ctx, authzdomain.PrincipalGrantInput{
		PrincipalID: principalID, Permission: authzdomain.PermReligionOrgManage,
	}); err != nil {
		t.Fatalf("grant principal permission: %v", err)
	}

	t.Run("instance-wide grant: allowed regardless of which unit is targeted", func(t *testing.T) {
		otherUnit := h.seedUnit(t)
		for _, u := range []string{someUnit, otherUnit} {
			if err := e.RequireServiceOrTarget(svcCtx, tok, action, u); err != nil {
				t.Fatalf("expected the instance-wide grant to satisfy unit %s, got: %v", u, err)
			}
		}
	})
}

func TestRequireServiceOrTarget_PersonStaysTargetScoped(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	e := pep.New(h.authz)
	var tok bearertoken.Token
	const action = string(authzdomain.PermReligionOrgManage)

	roleID := h.seedUnitScopedManageRole(t, action)
	granted, elsewhere := h.seedUnit(t), h.seedUnit(t)
	subject := h.seedPerson(t)
	h.grant(t, subject, roleID, granted, authzdomain.ScopeSubtree, "command")

	personCtx := authn.NewContext(ctx, authn.Subject{PersonID: subject})

	t.Run("within the granted subtree: allowed", func(t *testing.T) {
		if err := e.RequireServiceOrTarget(personCtx, tok, action, granted); err != nil {
			t.Fatalf("expected allow at the granted unit, got: %v", err)
		}
	})

	t.Run("outside the granted subtree: still denied", func(t *testing.T) {
		if err := e.RequireServiceOrTarget(personCtx, tok, action, elsewhere); err == nil {
			t.Fatal("RequireServiceOrTarget must stay target-scoped for a person — this subject holds " +
				"religionorg.manage on a DIFFERENT unit only, so RequireAnywhere would wrongly allow this " +
				"but the target-scoped Require underneath RequireServiceOrTarget must not")
		}
	})
}
