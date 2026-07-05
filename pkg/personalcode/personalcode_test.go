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

// ---------------------------------------------------------------- M12 RU/BY/LATAM schemes

func TestRUINN(t *testing.T) {
	r := New()
	if res, err := r.Validate("ru-inn", "500100732259", ""); err != nil || res.Outcome != OutcomeValidated {
		t.Fatalf("valid ИНН rejected: %+v err %v", res, err)
	}
	for _, bad := range []string{"500100732258", "5001007322", "abc"} {
		if _, err := r.Validate("ru-inn", bad, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("ИНН %q should be invalid, got %v", bad, err)
		}
	}
}

func TestRUSNILS(t *testing.T) {
	r := New()
	if res, err := r.Validate("ru-snils", "112-233-445 95", ""); err != nil || res.Normalized != "11223344595" {
		t.Fatalf("valid СНИЛС rejected/not normalized: %+v err %v", res, err)
	}
	if _, err := r.Validate("ru-snils", "11223344594", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad СНИЛС checksum should be invalid, got %v", err)
	}
}

func TestBRCPF(t *testing.T) {
	r := New()
	if res, err := r.Validate("br-cpf", "111.444.777-35", ""); err != nil || res.Normalized != "11144477735" {
		t.Fatalf("valid CPF rejected/not normalized: %+v err %v", res, err)
	}
	for _, bad := range []string{"11144477734", "11111111111", "1234567890"} {
		if _, err := r.Validate("br-cpf", bad, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("CPF %q should be invalid, got %v", bad, err)
		}
	}
}

func TestARCUIL(t *testing.T) {
	r := New()
	if res, err := r.Validate("ar-cuil", "20-12345678-6", ""); err != nil || res.Normalized != "20123456786" {
		t.Fatalf("valid CUIL rejected/not normalized: %+v err %v", res, err)
	}
	if _, err := r.Validate("ar-cuil", "20123456787", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad CUIL checksum should be invalid, got %v", err)
	}
}

func TestCLRUT(t *testing.T) {
	r := New()
	if res, err := r.Validate("cl-rut", "12.345.678-5", ""); err != nil || res.Normalized != "123456785" {
		t.Fatalf("valid RUT rejected/not normalized: %+v err %v", res, err)
	}
	// K verifier case (body 00000006 → verifier K).
	if res, err := r.Validate("cl-rut", "00000006-K", ""); err != nil || res.Normalized != "00000006K" {
		t.Fatalf("valid K-verifier RUT rejected: %+v err %v", res, err)
	}
	if _, err := r.Validate("cl-rut", "12.345.678-6", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad RUT verifier should be invalid, got %v", err)
	}
}

func TestMXCURP(t *testing.T) {
	r := New()
	if res, err := r.Validate("mx-curp", "badd110313hcmlns09", ""); err != nil || res.Normalized != "BADD110313HCMLNS09" {
		t.Fatalf("valid CURP rejected/not uppercased: %+v err %v", res, err)
	}
	if _, err := r.Validate("mx-curp", "BADD110313XCMLNS09", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed CURP (bad sex char) should be invalid, got %v", err)
	}
}

func TestMXRFC(t *testing.T) {
	r := New()
	if res, err := r.Validate("mx-rfc", "gode561231gr8", ""); err != nil || res.Normalized != "GODE561231GR8" {
		t.Fatalf("valid RFC rejected/not uppercased: %+v err %v", res, err)
	}
	if _, err := r.Validate("mx-rfc", "GODE561231", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short RFC should be invalid, got %v", err)
	}
}

// ---------------------------------------------------------------- M44 finance schemes

func TestIBAN(t *testing.T) {
	r := New()
	// Well-known valid IBANs (ISO 13616 reference examples); spaces + lower case normalize away.
	for _, ok := range []struct{ in, norm string }{
		{"GB82 WEST 1234 5698 7654 32", "GB82WEST12345698765432"},
		{"de89 3704 0044 0532 0130 00", "DE89370400440532013000"},
		{"UA903052992990004149123456789", "UA903052992990004149123456789"},
	} {
		res, err := r.Validate("iban", ok.in, "")
		if err != nil || res.Outcome != OutcomeValidated || res.Normalized != ok.norm {
			t.Fatalf("valid IBAN %q rejected/not normalized: %+v err %v", ok.in, res, err)
		}
	}
	for _, bad := range []string{
		"GB82WEST12345698765433", // corrupted check
		"GB00WEST12345698765432", // wrong check digits
		"XX",                     // too short
		"1234567890123456",       // no country letters
	} {
		if _, err := r.Validate("iban", bad, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("IBAN %q should be invalid, got %v", bad, err)
		}
	}
}

func TestPAN(t *testing.T) {
	r := New()
	// Visa/Mastercard/Amex test numbers (all Luhn-valid); separators normalize away.
	for _, ok := range []struct{ in, norm, bin, last4 string }{
		{"4111 1111 1111 1111", "4111111111111111", "411111", "1111"},
		{"5555-5555-5555-4444", "5555555555554444", "555555", "4444"},
		{"3782 822463 10005", "378282246310005", "378282", "0005"},
	} {
		res, err := r.Validate("pan", ok.in, "")
		if err != nil || res.Outcome != OutcomeValidated || res.Normalized != ok.norm {
			t.Fatalf("valid PAN %q rejected/not normalized: %+v err %v", ok.in, res, err)
		}
		if bin, last4 := SplitPAN(res.Normalized); bin != ok.bin || last4 != ok.last4 {
			t.Fatalf("SplitPAN(%q) = %q/%q, want %q/%q", ok.norm, bin, last4, ok.bin, ok.last4)
		}
	}
	for _, bad := range []string{
		"4111111111111112", // Luhn fails
		"411111",           // too short
		"41111111111111111111", // too long (20)
	} {
		if _, err := r.Validate("pan", bad, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("PAN %q should be invalid, got %v", bad, err)
		}
	}
}

// TestNewSchemesRegistered guards that every M12 scheme expected to carry a compiled validator does.
func TestNewSchemesRegistered(t *testing.T) {
	r := New()
	for _, s := range []string{"ru-inn", "ru-snils", "br-cpf", "ar-cuil", "cl-rut", "mx-curp", "mx-rfc"} {
		if !r.HasValidator(s) {
			t.Fatalf("scheme %q should have a compiled validator", s)
		}
	}
	// ar-dni / co-cedula / by-personal-number intentionally rely on the catalog regex fallback.
	for _, s := range []string{"ar-dni", "co-cedula", "by-personal-number"} {
		if r.HasValidator(s) {
			t.Fatalf("scheme %q should NOT have a compiled validator (regex fallback)", s)
		}
	}
}
