// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Pure unit tests for the tenant domain logic (no DB): lifecycle transition legality and the
// unit/graph validation guards. The DB constraints enforce the same shapes as a backstop.
package domain

import (
	"errors"
	"testing"
)

func TestStateCanTransitionTo(t *testing.T) {
	cases := []struct {
		from, to State
		want     bool
	}{
		{StateActive, StateSuspended, true},
		{StateActive, StateArchived, true},
		{StateActive, StateActive, false}, // no-op is not a transition
		{StateSuspended, StateActive, true},
		{StateSuspended, StateArchived, true},
		{StateSuspended, StateSuspended, false},
		{StateArchived, StateActive, true}, // restore
		{StateArchived, StateSuspended, false},
		{StateArchived, StateArchived, false},
		{State("bogus"), StateActive, false},
	}
	for _, c := range cases {
		if got := c.from.CanTransitionTo(c.to); got != c.want {
			t.Errorf("%s -> %s: got %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestUnitValidate(t *testing.T) {
	cases := []struct {
		name string
		unit Unit
		ok   bool
	}{
		{"valid", Unit{OrgID: "o", DomainID: "d", Code: ptr("1-bn"), Name: "1st Battalion", Visibility: VisibilityPublic}, true},
		{"valid shadow", Unit{OrgID: "o", DomainID: "d", Code: ptr("ghq"), Name: "GHQ", Visibility: VisibilityShadow}, true},
		{"codeless sub-unit", Unit{OrgID: "o", DomainID: "d", Code: nil, Name: "3rd Platoon", Visibility: VisibilityPublic}, true}, // D-UnitCodeLifecycle
		{"missing org", Unit{DomainID: "d", Code: ptr("a"), Name: "x", Visibility: VisibilityPublic}, false},                       // D-TenantOrganizations (M40)
		{"empty code", Unit{OrgID: "o", DomainID: "d", Code: ptr(""), Name: "x", Visibility: VisibilityPublic}, false},
		{"whitespace code", Unit{OrgID: "o", DomainID: "d", Code: ptr("a b"), Name: "x", Visibility: VisibilityPublic}, false},
		{"empty name", Unit{OrgID: "o", DomainID: "d", Code: ptr("a"), Name: "  ", Visibility: VisibilityPublic}, false},
		{"bad visibility", Unit{OrgID: "o", DomainID: "d", Code: ptr("a"), Name: "x", Visibility: Visibility("hidden")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.unit.Validate()
			if c.ok && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !c.ok {
				if err == nil {
					t.Fatalf("expected invalid, got nil")
				}
				if !errors.Is(err, ErrInvalidUnit) {
					t.Fatalf("expected ErrInvalidUnit, got %v", err)
				}
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestValidateCode(t *testing.T) {
	if err := ValidateCode(nil); err != nil { // clearing the code is always valid
		t.Fatalf("nil code should be valid, got %v", err)
	}
	if err := ValidateCode(ptr("1-bn")); err != nil {
		t.Fatalf("well-shaped code should be valid, got %v", err)
	}
	if err := ValidateCode(ptr("a b")); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("whitespace code should be invalid, got %v", err)
	}
	if err := ValidateCode(ptr("")); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("empty code should be invalid, got %v", err)
	}
}

func TestGraphValidate(t *testing.T) {
	if err := (Graph{Code: "operational", Name: "Operational"}).Validate(); err != nil {
		t.Fatalf("expected valid graph, got %v", err)
	}
	if err := (Graph{Code: "", Name: "x"}).Validate(); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit for empty code, got %v", err)
	}
	if err := (Graph{Code: "g", Name: ""}).Validate(); !errors.Is(err, ErrInvalidUnit) {
		t.Fatalf("expected ErrInvalidUnit for empty name, got %v", err)
	}
}

func TestValidState(t *testing.T) {
	for _, s := range []State{StateActive, StateSuspended, StateArchived} {
		if !ValidState(s) {
			t.Errorf("expected %s valid", s)
		}
	}
	if ValidState(State("merged")) {
		t.Errorf("expected unknown state invalid")
	}
}
