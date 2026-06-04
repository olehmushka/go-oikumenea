package personalcode

import (
	"errors"
	"testing"
)

func TestUARNOKPP(t *testing.T) {
	r := New()
	// 1234567899 has a valid weighted check digit (9); see the РНОКПП algorithm.
	res, err := r.Validate("ua-rnokpp", "1234567899", "")
	if err != nil {
		t.Fatalf("valid РНОКПП rejected: %v", err)
	}
	if res.Outcome != OutcomeValidated || res.Normalized != "1234567899" {
		t.Fatalf("unexpected result: %+v", res)
	}
	// Normalization strips separators.
	if res, err := r.Validate("ua-rnokpp", "1234 567899", ""); err != nil || res.Normalized != "1234567899" {
		t.Fatalf("separator normalization failed: %+v err %v", res, err)
	}
	// Wrong check digit.
	if _, err := r.Validate("ua-rnokpp", "1234567890", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad checksum should be invalid, got %v", err)
	}
	// Wrong length.
	if _, err := r.Validate("ua-rnokpp", "12345", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short value should be invalid, got %v", err)
	}
}

func TestUSSSN(t *testing.T) {
	r := New()
	if res, err := r.Validate("us-ssn", "123-45-6789", ""); err != nil || res.Normalized != "123456789" {
		t.Fatalf("valid SSN rejected: %+v err %v", res, err)
	}
	for _, bad := range []string{"000-45-6789", "666-45-6789", "900-45-6789", "123-00-6789", "123-45-0000", "12345678"} {
		if _, err := r.Validate("us-ssn", bad, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("SSN %q should be invalid, got %v", bad, err)
		}
	}
}

func TestPLPESEL(t *testing.T) {
	r := New()
	if res, err := r.Validate("pl-pesel", "44051401458", ""); err != nil || res.Outcome != OutcomeValidated {
		t.Fatalf("valid PESEL rejected: %+v err %v", res, err)
	}
	if _, err := r.Validate("pl-pesel", "44051401459", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad PESEL checksum should be invalid, got %v", err)
	}
}

func TestRegexFallback(t *testing.T) {
	r := New()
	// No compiled validator for this scheme; the catalog regex governs.
	res, err := r.Validate("xx-custom", "AB1234", `^[A-Z]{2}\d{4}$`)
	if err != nil || res.Outcome != OutcomeRegex {
		t.Fatalf("regex fallback should accept: %+v err %v", res, err)
	}
	if _, err := r.Validate("xx-custom", "bad", `^[A-Z]{2}\d{4}$`); !errors.Is(err, ErrInvalid) {
		t.Fatalf("regex mismatch should be invalid, got %v", err)
	}
}

func TestAcceptWithWarning(t *testing.T) {
	r := New()
	res, err := r.Validate("xx-unknown", "anything-goes", "")
	if err != nil {
		t.Fatalf("accept-with-warning should not error: %v", err)
	}
	if res.Outcome != OutcomeAcceptedWarn || res.Normalized != "anything-goes" {
		t.Fatalf("unexpected accept-with-warning result: %+v", res)
	}
	// Empty value is rejected even with no validator.
	if _, err := r.Validate("xx-unknown", "   ", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty value should be invalid, got %v", err)
	}
}

func TestBadCatalogRegexDegradesToWarn(t *testing.T) {
	r := New()
	res, err := r.Validate("xx-custom", "value", "([") // invalid regex
	if err != nil || res.Outcome != OutcomeAcceptedWarn {
		t.Fatalf("a bad catalog regex should degrade to accept-with-warning: %+v err %v", res, err)
	}
}
