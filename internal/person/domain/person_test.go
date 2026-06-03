// Unit tests for the person domain: validation of the aggregate, name variants, citizenship and
// residence links, and the reversible deactivate -> purge lifecycle gates (D-PersonNamesCLDR /
// D-PersonBio / D-Geo / D-PersonReadScope). Pure logic — no database.
package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPersonValidate(t *testing.T) {
	base := func() Person { return Person{Name: Name{DisplayName: "Олег Мушка"}, Sex: "male"} }

	cases := []struct {
		name    string
		mutate  func(*Person)
		wantErr bool
	}{
		{"valid", func(*Person) {}, false},
		{"empty display name", func(p *Person) { p.DisplayName = "  " }, true},
		{"empty optional code ok", func(p *Person) { p.Code = "" }, false},
		{"code with whitespace", func(p *Person) { p.Code = "svc 123" }, true},
		{"unknown sex", func(p *Person) { p.Sex = "other" }, true},
		{"empty sex ok (defaulted upstream)", func(p *Person) { p.Sex = "" }, false},
		{"good birthdate", func(p *Person) { p.Birthdate = "1990-05-02" }, false},
		{"bad birthdate", func(p *Person) { p.Birthdate = "1990-13-40" }, true},
		{"non-date birthdate", func(p *Person) { p.Birthdate = "yesterday" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base()
			tc.mutate(&p)
			err := p.Validate()
			if tc.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPersonPatchValidate(t *testing.T) {
	empty := ""
	bad := "not-a-date"
	good := "2000-01-01"
	sexBad := "robot"

	if err := (PersonPatch{DisplayName: &empty}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("clearing displayName should be invalid, got %v", err)
	}
	if err := (PersonPatch{Birthdate: &bad}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad birthdate should be invalid, got %v", err)
	}
	if err := (PersonPatch{Sex: &sexBad}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad sex should be invalid, got %v", err)
	}
	if err := (PersonPatch{Birthdate: &good}).Validate(); err != nil {
		t.Fatalf("good patch should validate, got %v", err)
	}
	if err := (PersonPatch{}).Validate(); err != nil {
		t.Fatalf("empty patch should validate, got %v", err)
	}
}

func TestCitizenshipValidate(t *testing.T) {
	if err := (Citizenship{Country: "UA", Basis: "birth"}).Validate(); err != nil {
		t.Fatalf("valid citizenship: %v", err)
	}
	if err := (Citizenship{Country: "ukr", Basis: "birth"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("3-letter country should be invalid, got %v", err)
	}
	if err := (Citizenship{Country: "UA", Basis: "gift"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown basis should be invalid, got %v", err)
	}
	if err := (Citizenship{Country: "UA", Basis: "other", AcquiredOn: "bad"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad acquiredOn should be invalid, got %v", err)
	}
}

func TestResidenceValidate(t *testing.T) {
	if err := (Residence{Country: "PL", ValidFrom: "2021-09-01"}).Validate(); err != nil {
		t.Fatalf("valid residence: %v", err)
	}
	if err := (Residence{Country: "PL"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing validFrom should be invalid, got %v", err)
	}
	if err := (Residence{Country: "PL", ValidFrom: "2021-09-01", ValidTo: "nope"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad validTo should be invalid, got %v", err)
	}
}

func TestNameVariantValidate(t *testing.T) {
	if err := (NameVariant{Locale: "eng", Name: Name{DisplayName: "Oleh Mushka"}}).Validate(); err != nil {
		t.Fatalf("valid variant: %v", err)
	}
	if err := (NameVariant{Name: Name{DisplayName: "Oleh"}}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing locale should be invalid, got %v", err)
	}
	if err := (NameVariant{Locale: "eng"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing displayName should be invalid, got %v", err)
	}
}

func TestLifecycleGates(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	active := Person{Status: StatusActive}
	if active.CanReactivate() {
		t.Fatal("active person should not be reactivatable")
	}
	if active.CanPurge(now) {
		t.Fatal("active person (never deactivated) should not be purgeable")
	}

	deactivatedInGrace := Person{Status: StatusDeactivated, PurgeAfter: &future}
	if !deactivatedInGrace.CanReactivate() {
		t.Fatal("deactivated person should be reactivatable")
	}
	if deactivatedInGrace.CanPurge(now) {
		t.Fatal("purge before purge_after must be refused")
	}

	deactivatedPastGrace := Person{Status: StatusDeactivated, PurgeAfter: &past}
	if !deactivatedPastGrace.CanPurge(now) {
		t.Fatal("purge after purge_after must be allowed")
	}

	purged := Person{Status: StatusPurged}
	if purged.CanReactivate() || purged.CanPurge(now) {
		t.Fatal("purged tombstone is terminal")
	}
}
