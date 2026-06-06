package domain

import (
	"errors"
	"strings"
	"testing"
)

// TestPositionValidate covers the code + title invariants.
func TestPositionValidate(t *testing.T) {
	cases := []struct {
		name string
		pos  Position
		want error
	}{
		{"ok", Position{Code: "S1", Title: "Operations Officer"}, nil},
		{"empty code", Position{Code: "", Title: "x"}, ErrPositionInvalid},
		{"whitespace code", Position{Code: "a b", Title: "x"}, ErrPositionInvalid},
		{"empty title", Position{Code: "S1", Title: "  "}, ErrPositionInvalid},
		{"long code", Position{Code: strings.Repeat("x", 129), Title: "x"}, ErrPositionInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.pos.Validate()
			if c.want == nil && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("Validate() = %v, want %v", err, c.want)
			}
		})
	}
}

// TestPositionPatchValidate rejects clearing the title to blank.
func TestPositionPatchValidate(t *testing.T) {
	blank := "   "
	if err := (PositionPatch{Title: &blank}).Validate(); !errors.Is(err, ErrPositionInvalid) {
		t.Fatalf("blank title patch: want ErrPositionInvalid, got %v", err)
	}
	ok := "New Title"
	if err := (PositionPatch{Title: &ok}).Validate(); err != nil {
		t.Fatalf("valid title patch: %v", err)
	}
	// an all-nil patch is valid (no-op).
	if err := (PositionPatch{}).Validate(); err != nil {
		t.Fatalf("empty patch: %v", err)
	}
}

// TestMembershipValidate requires a person and a unit.
func TestMembershipValidate(t *testing.T) {
	if err := (Membership{PersonID: "p", UnitID: "u"}).Validate(); err != nil {
		t.Fatalf("valid membership: %v", err)
	}
	if err := (Membership{UnitID: "u"}).Validate(); !errors.Is(err, ErrMembershipInvalid) {
		t.Fatalf("missing person: want ErrMembershipInvalid, got %v", err)
	}
	if err := (Membership{PersonID: "p"}).Validate(); !errors.Is(err, ErrMembershipInvalid) {
		t.Fatalf("missing unit: want ErrMembershipInvalid, got %v", err)
	}
}

// TestCanEnd allows ending only an active membership.
func TestCanEnd(t *testing.T) {
	if !(Membership{Status: MembershipActive}).CanEnd() {
		t.Fatal("active membership should be endable")
	}
	if (Membership{Status: MembershipEnded}).CanEnd() {
		t.Fatal("ended membership should not be endable")
	}
}
