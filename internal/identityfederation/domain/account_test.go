package domain

import (
	"errors"
	"testing"
)

func TestAccountValidate(t *testing.T) {
	if err := (Account{PersonID: "urn:oikumenea:person:local:person:1"}).Validate(); err != nil {
		t.Fatalf("valid account rejected: %v", err)
	}
	if err := (Account{}).Validate(); !errors.Is(err, ErrAccountInvalid) {
		t.Fatalf("empty personId should be ErrAccountInvalid, got %v", err)
	}
}

func TestExternalIdentityValidate(t *testing.T) {
	ok := ExternalIdentity{Issuer: "https://idp.example", Subject: "sub-1"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid identity rejected: %v", err)
	}
	cases := []ExternalIdentity{
		{Subject: "sub-1"},              // missing issuer
		{Issuer: "https://idp.example"}, // missing subject
		{Issuer: "  ", Subject: "  "},   // blank
	}
	for i, c := range cases {
		if err := c.Validate(); !errors.Is(err, ErrIdentityInvalid) {
			t.Errorf("case %d: expected ErrIdentityInvalid, got %v", i, err)
		}
	}
}
