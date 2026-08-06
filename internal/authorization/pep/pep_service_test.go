// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package pep

import (
	"context"
	"testing"

	"github.com/olehmushka/go-oikumenea/pkg/authn"
	"github.com/palantir/pkg/bearertoken"
)

// The machine-subject guarantees of the PEP (M51 / D-ServiceIdentities).
//
// The load-bearing property is that a service principal is denied by every PERSON-shaped path. That
// holds structurally — a principal sets no PersonID, and each of those methods guards on an empty
// subject — but "it happens to work" is not a guarantee, so these tests pin it. A regression here
// would hand a connector a person's authority surface, which is exactly the escalation the org-scoped
// grant model exists to prevent.

func serviceCtx() context.Context {
	return authn.NewContext(context.Background(), authn.Subject{
		Service:     "test-connector",
		PrincipalID: "01890000-0000-8000-8000-000000000001",
	})
}

func personCtx() context.Context {
	return authn.NewContext(context.Background(), authn.Subject{PersonID: "01890000-0000-8000-8000-0000000000ff"})
}

// TestServiceSubjectDeniedOnPersonPaths proves a machine cannot reach any person-shaped enforcement
// path. The enforcer is deliberately UNBOUND (svc == nil): if any of these reached the authorization
// service instead of denying first, the call would nil-panic — so the test also proves they never
// consult the PDP for a principal.
func TestServiceSubjectDeniedOnPersonPaths(t *testing.T) {
	e := NewUnbound()
	ctx := serviceCtx()
	var tok bearertoken.Token

	t.Run("Require", func(t *testing.T) {
		if err := e.Require(ctx, tok, "person.read", "unit-1"); err == nil {
			t.Fatal("Require allowed a service principal; want denied (a machine has no unit reach)")
		}
	})
	t.Run("RequireAny", func(t *testing.T) {
		if err := e.RequireAny(ctx, tok, "unit-1", "unit.edges.manage"); err == nil {
			t.Fatal("RequireAny allowed a service principal; want denied")
		}
	})
	t.Run("RequireAnywhere", func(t *testing.T) {
		if err := e.RequireAnywhere(ctx, tok, "person.read"); err == nil {
			t.Fatal("RequireAnywhere allowed a service principal; want denied")
		}
	})
	t.Run("AllowedAnywhere", func(t *testing.T) {
		ok, err := e.AllowedAnywhere(ctx, tok, "person.read")
		if err != nil || ok {
			t.Fatalf("AllowedAnywhere = (%v, %v); want (false, nil) for a service principal", ok, err)
		}
	})
	t.Run("SubjectAuthority", func(t *testing.T) {
		subj, isAdmin, err := e.SubjectAuthority(ctx)
		if err != nil || subj != "" || isAdmin {
			t.Fatalf("SubjectAuthority = (%q, %v, %v); want empty/false/nil — a machine is never instance admin", subj, isAdmin, err)
		}
	})
	t.Run("FilterVisibleUnits", func(t *testing.T) {
		got, err := e.FilterVisibleUnits(ctx, []string{"unit-1", "unit-2"}, map[string]bool{"unit-1": true})
		if err != nil || len(got) != 0 {
			t.Fatalf("FilterVisibleUnits = (%v, %v); want (empty, nil) for a service principal", got, err)
		}
	})
}

// TestRequireServiceDeniesNonMachineSubjects: RequireService is the machine-only door. A person (or an
// unauthenticated request) must not walk through it — a human's authority comes from the PDP, and
// letting one satisfy a grant check would bypass instance-scope rules.
func TestRequireServiceDeniesNonMachineSubjects(t *testing.T) {
	e := NewUnbound() // nil svc: a denial must happen before any service call
	var tok bearertoken.Token

	if err := e.RequireService(personCtx(), tok, "import.manage", ""); err == nil {
		t.Error("RequireService allowed a PERSON subject; want denied (machine-only path)")
	}
	if err := e.RequireService(context.Background(), tok, "import.manage", ""); err == nil {
		t.Error("RequireService allowed an UNAUTHENTICATED request; want denied")
	}
}

// TestUnauthenticatedDeniedEverywhere keeps the pre-existing baseline honest alongside the new arm.
func TestUnauthenticatedDeniedEverywhere(t *testing.T) {
	e := NewUnbound()
	ctx := context.Background()
	var tok bearertoken.Token

	if err := e.Require(ctx, tok, "person.read", "unit-1"); err == nil {
		t.Error("Require allowed an unauthenticated request")
	}
	if err := e.RequireAnywhere(ctx, tok, "person.read"); err == nil {
		t.Error("RequireAnywhere allowed an unauthenticated request")
	}
}
