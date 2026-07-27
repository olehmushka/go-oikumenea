// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package action

import (
	"errors"
	"testing"
)

func TestValidateGate(t *testing.T) {
	// A known catalogued action passes; a plausible-looking typo fails with ErrUnregistered.
	if err := Validate("assignment.grant"); err != nil {
		t.Fatalf("assignment.grant should be registered: %v", err)
	}
	if err := Validate("assignment.granted"); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("assignment.granted must be rejected with ErrUnregistered, got %v", err)
	}
	if err := Validate(""); !errors.Is(err, ErrUnregistered) {
		t.Fatalf("empty action must be rejected")
	}
}

func TestCatalogWellFormed(t *testing.T) {
	all := All()
	if len(all) != Count() || Count() < 200 {
		t.Fatalf("catalog size looks wrong: All()=%d Count()=%d", len(all), Count())
	}
	seen := map[string]bool{}
	for _, a := range all {
		if a.Code == "" || a.Service == 0 || a.TargetType == "" || a.Permission == "" {
			t.Errorf("incomplete action type: %+v", a)
		}
		if seen[a.Code] {
			t.Errorf("duplicate action code %q", a.Code)
		}
		seen[a.Code] = true
	}
	// All() is sorted by (service, code).
	for i := 1; i < len(all); i++ {
		if all[i-1].Service > all[i].Service ||
			(all[i-1].Service == all[i].Service && all[i-1].Code >= all[i].Code) {
			t.Fatalf("All() not sorted at %d: %q then %q", i, all[i-1].Code, all[i].Code)
		}
	}
}
