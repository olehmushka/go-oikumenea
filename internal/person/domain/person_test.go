// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

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
	base := func() Person { return Person{Name: Name{DisplayName: "Тарас Шевченко"}, Sex: "male"} }

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
		{"good date_of_death", func(p *Person) { p.DateOfDeath = "2024-01-15" }, false},
		{"bad date_of_death", func(p *Person) { p.DateOfDeath = "2024-13-40" }, true},
		{"non-date date_of_death", func(p *Person) { p.DateOfDeath = "tomorrow" }, true},
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

func TestNormalizeSex(t *testing.T) {
	cases := map[string]string{
		"0": "not_known", "1": "male", "2": "female", "9": "not_applicable",
		" 1 ":   "male",  // surrounding whitespace tolerated
		"male":  "male",  // already canonical, passed through
		"":      "",      // empty stays empty (defaulted upstream)
		"7":     "7",     // unknown numeric passes through for Validate to reject
		"robot": "robot", // garbage passes through
	}
	for in, want := range cases {
		if got := NormalizeSex(in); got != want {
			t.Errorf("NormalizeSex(%q) = %q, want %q", in, got, want)
		}
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
	if err := (PersonPatch{DateOfDeath: &bad}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad date_of_death should be invalid, got %v", err)
	}
	if err := (PersonPatch{DateOfDeath: &good}).Validate(); err != nil {
		t.Fatalf("good date_of_death patch should validate, got %v", err)
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

// validCountryRID is a hand-built, well-formed country RID (location service=12, object, type=country,
// UUIDv8 version 8, app 1) — countries are RID-keyed (F-014), so Country fields now carry a RID.
const validCountryRID = "01020304-0506-8101-8c01-000000000000"

func TestCitizenshipValidate(t *testing.T) {
	if err := (Citizenship{Country: validCountryRID, Basis: "birth"}).Validate(); err != nil {
		t.Fatalf("valid citizenship: %v", err)
	}
	if err := (Citizenship{Country: "UA", Basis: "birth"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("non-RID country should be invalid, got %v", err)
	}
	if err := (Citizenship{Country: validCountryRID, Basis: "gift"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown basis should be invalid, got %v", err)
	}
	if err := (Citizenship{Country: validCountryRID, Basis: "other", AcquiredOn: "bad"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad acquiredOn should be invalid, got %v", err)
	}
}

func TestResidenceValidate(t *testing.T) {
	if err := (Residence{Country: validCountryRID, ValidFrom: "2021-09-01"}).Validate(); err != nil {
		t.Fatalf("valid residence: %v", err)
	}
	if err := (Residence{Country: validCountryRID}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing validFrom should be invalid, got %v", err)
	}
	if err := (Residence{Country: validCountryRID, ValidFrom: "2021-09-01", ValidTo: "nope"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad validTo should be invalid, got %v", err)
	}
}

func TestPersonLanguageValidate(t *testing.T) {
	valid := "00000000-0000-0000-0000-000000000001"
	if err := (PersonLanguage{LanguageID: valid, CEFRLevel: "B2", IsNative: true}).Validate(); err != nil {
		t.Fatalf("valid person language: %v", err)
	}
	if err := (PersonLanguage{LanguageID: valid}).Validate(); err != nil {
		t.Fatalf("empty cefr is allowed: %v", err)
	}
	if err := (PersonLanguage{}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing languageId should be invalid, got %v", err)
	}
	if err := (PersonLanguage{LanguageID: valid, CEFRLevel: "Z9"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad cefr should be invalid, got %v", err)
	}
}

func TestNameVariantValidate(t *testing.T) {
	if err := (NameVariant{Locale: "eng", Name: Name{DisplayName: "John Doe"}}).Validate(); err != nil {
		t.Fatalf("valid variant: %v", err)
	}
	if err := (NameVariant{Name: Name{DisplayName: "John"}}).Validate(); !errors.Is(err, ErrInvalid) {
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

// Relationship invariants (D-PersonRelationships, M14).

func TestRelationshipValidate(t *testing.T) {
	if err := (Partnership{Status: "married"}).Validate(); err != nil {
		t.Fatalf("valid partnership status: %v", err)
	}
	if err := (Partnership{Status: "dating"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad partnership status should be invalid")
	}
	if err := (Partnership{Status: "married", EffectiveFrom: "nope"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad effective date should be invalid")
	}
	if !(Partnership{Status: "engaged"}).IsActivePartnership() {
		t.Fatal("engaged counts as active")
	}
	if (Partnership{Status: "divorced"}).IsActivePartnership() {
		t.Fatal("divorced is not active")
	}
	if err := (Kinship{Status: "active"}).Validate(); err != nil {
		t.Fatalf("valid kinship: %v", err)
	}
	if err := (Kinship{Status: "cousin"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad kinship status should be invalid")
	}
	if err := (Sponsorship{Status: "active"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("sponsorship requires a relation code")
	}
	if err := (Sponsorship{RelationCode: "godparent", Status: "active"}).Validate(); err != nil {
		t.Fatalf("valid sponsorship: %v", err)
	}
	if err := (Association{Kind: "coi", Status: "active"}).Validate(); err != nil {
		t.Fatalf("valid association: %v", err)
	}
	if err := (Association{Kind: "enemy", Status: "active"}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("bad association kind should be invalid")
	}
}

func TestRelationLinkType(t *testing.T) {
	// Native UUIDv8 RIDs: byte6 low nibble = kind (2=link), byte8 low 6 bits = service (6=person),
	// byte9 = type-code low 8 bits. partnered_with=2, kin_parent_of=3, associated_with=7.
	cases := map[string]string{
		"00000000-0000-8201-8602-000000000000": LinkPartnership, // person/link/type 2
		"00000000-0000-8201-8603-000000000000": LinkKinship,     // type 3
		"00000000-0000-8201-8607-000000000000": LinkAssociation, // type 7
	}
	for rid, want := range cases {
		if got := RelationLinkType(rid); got != want {
			t.Fatalf("RelationLinkType(%q) = %q, want %q", rid, got, want)
		}
	}
	// Non-person-link RIDs must not be treated as relationships.
	for _, bad := range []string{
		"",
		"not-a-rid",
		"00000000-0000-8101-8401-000000000000", // tenant/object/unit (service 4, kind 1)
		"00000000-0000-8101-8601-000000000000", // person/object (kind 1, not link)
	} {
		if got := RelationLinkType(bad); got != "" {
			t.Fatalf("RelationLinkType(%q) = %q, want \"\"", bad, got)
		}
	}
}
