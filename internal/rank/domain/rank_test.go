package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNodeValidate(t *testing.T) {
	tests := []struct {
		name  string
		err   error // nil = valid
		check func() error
	}{
		{"category ok", nil, func() error { return Category{Code: "army", Name: "Army"}.Validate() }},
		{"category blank code", ErrInvalid, func() error { return Category{Code: "", Name: "Army"}.Validate() }},
		{"category code with space", ErrInvalid, func() error { return Category{Code: "land forces", Name: "X"}.Validate() }},
		{"category blank name", ErrInvalid, func() error { return Category{Code: "army", Name: "  "}.Validate() }},
		{"type ok", nil, func() error { return Type{Code: "officers", Name: "Officers", CategoryID: "c1"}.Validate() }},
		{"type nested ok", nil, func() error { return Type{Code: "junior", Name: "Junior", ParentTypeID: "t1"}.Validate() }},
		{"type missing parent", ErrInvalid, func() error { return Type{Code: "officers", Name: "Officers"}.Validate() }},
		{"rank ok", nil, func() error { return Rank{Code: "sgt", Name: "Sergeant", TypeID: "t1"}.Validate() }},
		{"rank missing type", ErrInvalid, func() error { return Rank{Code: "sgt", Name: "Sergeant"}.Validate() }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.check()
			if tc.err == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("want %v, got %v", tc.err, err)
			}
		})
	}
}

func TestValidCodeLength(t *testing.T) {
	if (Category{Code: strings.Repeat("x", 129), Name: "n"}).Validate() == nil {
		t.Fatal("code over 128 chars should be invalid")
	}
	if err := (Category{Code: strings.Repeat("x", 128), Name: "n"}).Validate(); err != nil {
		t.Fatalf("128-char code should be valid: %v", err)
	}
}

func TestValidLevel(t *testing.T) {
	for _, l := range []Level{LevelSystem, LevelCategory, LevelType, LevelRank} {
		if !ValidLevel(l) {
			t.Errorf("%q should be a valid level", l)
		}
	}
	for _, l := range []Level{"", "categories", "grade", "RANK"} {
		if ValidLevel(l) {
			t.Errorf("%q should not be a valid level", l)
		}
	}
}

func TestIsSenior(t *testing.T) {
	of5a := Grade{Code: "OF-5", Tier: TierOfficer, Ordinal: 5}
	of5b := Grade{Code: "OF-5", Tier: TierOfficer, Ordinal: 5} // same grade, another system
	of4 := Grade{Code: "OF-4", Tier: TierOfficer, Ordinal: 4}
	or9 := Grade{Code: "OR-9", Tier: TierEnlisted, Ordinal: 9}
	wo1 := Grade{Code: "WO-1", Tier: TierWarrant, Ordinal: 1}

	tests := []struct {
		name         string
		a, b         Grade
		want, known  bool
	}{
		{"officer over officer by ordinal", of5a, of4, true, true},
		{"junior officer not senior", of4, of5a, false, true},
		{"equivalent grades not strictly senior", of5a, of5b, false, true},
		{"officer over enlisted across tiers", of5a, or9, true, true},
		{"warrant over enlisted across tiers", wo1, or9, true, true},
		{"enlisted under officer", or9, of5a, false, true},
		{"absent grade on a is unknown", Grade{}, of5a, false, false},
		{"absent grade on b is unknown", of5a, Grade{}, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, known := IsSenior(tc.a, tc.b)
			if got != tc.want || known != tc.known {
				t.Fatalf("IsSenior(%v,%v) = (%v,%v), want (%v,%v)", tc.a, tc.b, got, known, tc.want, tc.known)
			}
		})
	}
}

func TestEquivalent(t *testing.T) {
	of5 := Grade{Code: "OF-5", Tier: TierOfficer, Ordinal: 5}
	if !Equivalent(of5, Grade{Code: "OF-5", Tier: TierOfficer, Ordinal: 5}) {
		t.Fatal("same grade code should be equivalent across systems")
	}
	if Equivalent(of5, Grade{Code: "OF-4", Tier: TierOfficer, Ordinal: 4}) {
		t.Fatal("different grade codes should not be equivalent")
	}
	if Equivalent(Grade{}, Grade{}) {
		t.Fatal("two ungraded ranks must not be equivalent")
	}
}
