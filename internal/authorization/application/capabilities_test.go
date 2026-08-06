// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Unit tests for EffectivePermissions — the self-serve introspection primitive behind
// GET /me/capabilities (D-SelfCapabilities): a caller learns their OWN effective permission codes.
package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// adminFakeService reuses the authority test harness but with the instance-admin flag set.
func adminFakeService(fetches *int) *Service {
	repo := fakeRepo{fetches: fetches, isAdmin: true}
	return NewService(nil, func(_ db.DBTX) domain.Repository { return repo }, nil, domain.NewPDP(fakeClosure{}), nil, nil)
}

func TestEffectivePermissionsUnionsGrantCodesSorted(t *testing.T) {
	fetches := 0
	// Two grants; overlapping + unordered codes must union, dedupe, and sort.
	svc := newAuthorityTestService(&fetches, []domain.ActiveGrant{
		grantOn("u1", "unit.read", "person.read"),
		grantOn("u2", "person.read", "order.read"),
	})

	perms, isAdmin, err := svc.EffectivePermissions(context.Background(), "p1")
	if err != nil {
		t.Fatalf("EffectivePermissions: %v", err)
	}
	if isAdmin {
		t.Errorf("isAdmin = true, want false for a plain grant holder")
	}
	want := []string{"order.read", "person.read", "unit.read"}
	if !reflect.DeepEqual(perms, want) {
		t.Errorf("perms = %v, want %v (sorted, deduped union)", perms, want)
	}
}

func TestEffectivePermissionsInstanceAdminReturnsFlagNotCatalog(t *testing.T) {
	fetches := 0
	svc := adminFakeService(&fetches)

	perms, isAdmin, err := svc.EffectivePermissions(context.Background(), "admin")
	if err != nil {
		t.Fatalf("EffectivePermissions: %v", err)
	}
	if !isAdmin {
		t.Fatalf("isAdmin = false, want true")
	}
	if len(perms) != 0 {
		t.Errorf("perms = %v, want empty (admin flag means show-all; catalog is not enumerated)", perms)
	}
}

func TestEffectivePermissionsNoGrantsIsEmpty(t *testing.T) {
	fetches := 0
	svc := newAuthorityTestService(&fetches, nil)

	perms, isAdmin, err := svc.EffectivePermissions(context.Background(), "p1")
	if err != nil {
		t.Fatalf("EffectivePermissions: %v", err)
	}
	if isAdmin {
		t.Errorf("isAdmin = true, want false")
	}
	if len(perms) != 0 {
		t.Errorf("perms = %v, want empty for a subject with no grants", perms)
	}
}
