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
		{"type missing category", ErrInvalid, func() error { return Type{Code: "officers", Name: "Officers"}.Validate() }},
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
	for _, l := range []Level{LevelCategory, LevelType, LevelRank} {
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
