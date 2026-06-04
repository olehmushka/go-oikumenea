package domain

import (
	"errors"
	"testing"
)

func TestDocumentTypeValidate(t *testing.T) {
	if err := (DocumentType{Code: "passport", Name: "Passport"}).Validate(); err != nil {
		t.Fatalf("valid type rejected: %v", err)
	}
	if err := (DocumentType{Code: "", Name: "x"}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("empty code should be invalid")
	}
	if err := (DocumentType{Code: "bad code", Name: "x"}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("code with whitespace should be invalid")
	}
	if err := (DocumentType{Code: "ok", Name: " "}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("blank name should be invalid")
	}
}

func TestDocumentValidate(t *testing.T) {
	good := Document{TypeID: "urn:type", IssuedOn: "2020-01-02", ExpiresOn: "2030-01-02"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
	if err := (Document{TypeID: ""}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("missing typeId should be invalid")
	}
	if err := (Document{TypeID: "t", IssuedOn: "not-a-date"}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("bad issuedOn should be invalid")
	}
	if err := (Document{TypeID: "t", ExpiresOn: "2030-13-40"}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("bad expiresOn should be invalid")
	}
}

func TestDocumentPatchValidate(t *testing.T) {
	bad := "2020-99-99"
	if err := (DocumentPatch{IssuedOn: &bad}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("bad issuedOn patch should be invalid")
	}
	badStatus := "frozen"
	if err := (DocumentPatch{Status: &badStatus}).Validate(); !errors.Is(err, ErrDocumentInvalid) {
		t.Fatal("unknown status should be invalid")
	}
	ok := "superseded"
	if err := (DocumentPatch{Status: &ok}).Validate(); err != nil {
		t.Fatalf("valid status patch rejected: %v", err)
	}
}

func TestSchemeValidate(t *testing.T) {
	if err := (PersonalCodeScheme{Code: "ua-rnokpp", GenericCategory: "tax-id", Name: "РНОКПП"}).Validate(); err != nil {
		t.Fatalf("valid scheme rejected: %v", err)
	}
	if err := (PersonalCodeScheme{Code: "x", GenericCategory: "nope", Name: "n"}).Validate(); !errors.Is(err, ErrPersonalCodeInvalid) {
		t.Fatal("unknown generic category should be invalid")
	}
	if err := (PersonalCodeScheme{Code: "bad code", GenericCategory: "tax-id", Name: "n"}).Validate(); !errors.Is(err, ErrPersonalCodeInvalid) {
		t.Fatal("code with whitespace should be invalid")
	}
}

func TestSchemePatchValidate(t *testing.T) {
	bad := "nope"
	if err := (PersonalCodeSchemePatch{GenericCategory: &bad}).Validate(); !errors.Is(err, ErrPersonalCodeInvalid) {
		t.Fatal("unknown category patch should be invalid")
	}
	ok := "national-id"
	if err := (PersonalCodeSchemePatch{GenericCategory: &ok}).Validate(); err != nil {
		t.Fatalf("valid category patch rejected: %v", err)
	}
}
