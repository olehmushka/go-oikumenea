// Package personalcode is the national-identifier validator registry (D-PersonalCodes): a compiled,
// reviewable set of scheme validators (UA RNOKPP checksum, US SSN format, PL PESEL checksum, …) keyed
// on the scheme code, plus the documented validation precedence the document module applies on
// personal-code create/update — code validator > the scheme's catalog regex fallback > accept-with-
// warning (docs/modules/platform.md). It is framework-free (standard library only) and holds no I/O.
//
// Validation runs on the PLAINTEXT value before encryption; the caller persists only the returned
// normalized value's ciphertext + blind index. Normalization is per scheme (e.g. strip separators on
// digit schemes) so that equal identifiers index identically.
package personalcode

import (
	"errors"
	"regexp"
	"strings"
)

// ErrInvalid is the sentinel a value fails its scheme's validator (or fallback regex) with; the
// document module maps it to Document:PersonalCodeInvalid. Wrapped with a reason via errors.Join.
var ErrInvalid = errors.New("personal code is invalid for its scheme")

// Outcome reports which layer accepted the value, so the caller can log the accept-with-warning case
// (an unknown scheme with no compiled validator and no catalog regex; D-PersonalCodes).
type Outcome int

const (
	// OutcomeValidated: a compiled scheme validator accepted (and normalized) the value.
	OutcomeValidated Outcome = iota
	// OutcomeRegex: no compiled validator; the scheme's catalog validation_regex accepted the value.
	OutcomeRegex
	// OutcomeAcceptedWarn: neither a validator nor a regex exists — accepted as-is with a warning.
	OutcomeAcceptedWarn
)

// Result is the validation outcome: the normalized value to persist + which layer accepted it.
type Result struct {
	Normalized string
	Outcome    Outcome
}

// validator validates and normalizes one scheme's value, returning ErrInvalid (joined with a reason)
// when the value does not satisfy the scheme.
type validator func(value string) (normalized string, err error)

// Registry holds the compiled scheme validators. The zero value is unusable; build one with New.
type Registry struct {
	validators map[string]validator
}

// New returns the default registry with the built-in scheme validators (D-PersonalCodes). Adding a
// scheme validator is a code change (reviewable, like the permission catalog).
func New() *Registry {
	return &Registry{validators: map[string]validator{
		"ua-rnokpp": validateUARNOKPP,
		"ua-ipn":    validateUARNOKPP, // alias: the РНОКПП was historically called ІПН
		"us-ssn":    validateUSSSN,
		"pl-pesel":  validatePLPESEL,
		// M12 RU/BY/LATAM schemes (D-PersonalCodes). Checksum validators where the algorithm is
		// well-known; structural (format) validators for the MX schemes whose homoclave check digit is
		// name-derived and not verifiable from the code alone; ar-dni / co-cedula / by-personal-number
		// have no compiled validator and fall back to the catalog regex.
		"ru-inn":   validateRUINN,
		"ru-snils": validateRUSNILS,
		"br-cpf":   validateBRCPF,
		"ar-cuil":  validateARCUIL,
		"cl-rut":   validateCLRUT,
		"mx-curp":  validateMXCURP,
		"mx-rfc":   validateMXRFC,
	}}
}

// HasValidator reports whether a compiled validator exists for the scheme.
func (r *Registry) HasValidator(scheme string) bool {
	_, ok := r.validators[scheme]
	return ok
}

// Validate applies the documented precedence to value for the given scheme: a compiled validator if
// one exists, else the scheme's catalog fallbackRegex if non-empty, else accept-with-warning. It
// returns ErrInvalid (joined with a reason) when a present validator/regex rejects the value.
func (r *Registry) Validate(scheme, value, fallbackRegex string) (Result, error) {
	if v, ok := r.validators[scheme]; ok {
		norm, err := v(value)
		if err != nil {
			return Result{}, err
		}
		return Result{Normalized: norm, Outcome: OutcomeValidated}, nil
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return Result{}, errors.Join(ErrInvalid, errors.New("value is empty"))
	}

	if fallbackRegex != "" {
		re, err := regexp.Compile(fallbackRegex)
		if err != nil {
			// A bad catalog regex is an operator error; fall through to accept-with-warning rather than
			// rejecting an otherwise valid value on a misconfigured scheme.
			return Result{Normalized: trimmed, Outcome: OutcomeAcceptedWarn}, nil
		}
		if !re.MatchString(trimmed) {
			return Result{}, errors.Join(ErrInvalid, errors.New("value does not match the scheme's validation pattern"))
		}
		return Result{Normalized: trimmed, Outcome: OutcomeRegex}, nil
	}

	return Result{Normalized: trimmed, Outcome: OutcomeAcceptedWarn}, nil
}

// ---------------------------------------------------------------- built-in validators

// digitsOnly strips every non-digit rune (separators, spaces) so digit-based identifiers normalize
// to a bare digit string.
func digitsOnly(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// validateUARNOKPP validates the Ukrainian РНОКПП / ІПН: 10 digits with a weighted check digit.
// weights = [-1,5,7,9,4,6,10,5,7] over the first 9 digits; check = (Σ wᵢdᵢ) mod 11 mod 10 == d₁₀.
func validateUARNOKPP(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 10 {
		return "", errors.Join(ErrInvalid, errors.New("РНОКПП must be 10 digits"))
	}
	weights := [9]int{-1, 5, 7, 9, 4, 6, 10, 5, 7}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(d[i]-'0') * weights[i]
	}
	check := (sum % 11) % 10
	if check != int(d[9]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("РНОКПП checksum does not match"))
	}
	return d, nil
}

// validateUSSSN validates a US Social Security Number's structure (no checksum exists): 9 digits where
// the area is not 000/666/900-999, the group is not 00, and the serial is not 0000.
func validateUSSSN(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 9 {
		return "", errors.Join(ErrInvalid, errors.New("SSN must be 9 digits"))
	}
	area := d[0:3]
	group := d[3:5]
	serial := d[5:9]
	switch {
	case area == "000" || area == "666" || area[0] == '9':
		return "", errors.Join(ErrInvalid, errors.New("SSN area number is invalid"))
	case group == "00":
		return "", errors.Join(ErrInvalid, errors.New("SSN group number is invalid"))
	case serial == "0000":
		return "", errors.Join(ErrInvalid, errors.New("SSN serial number is invalid"))
	}
	return d, nil
}

// validatePLPESEL validates the Polish PESEL: 11 digits with a weighted check digit.
// weights = [1,3,7,9,1,3,7,9,1,3]; check = (10 - (Σ wᵢdᵢ mod 10)) mod 10 == d₁₁.
func validatePLPESEL(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 11 {
		return "", errors.Join(ErrInvalid, errors.New("PESEL must be 11 digits"))
	}
	weights := [10]int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3}
	sum := 0
	for i := 0; i < 10; i++ {
		sum += int(d[i]-'0') * weights[i]
	}
	check := (10 - (sum % 10)) % 10
	if check != int(d[10]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("PESEL checksum does not match"))
	}
	return d, nil
}

// allSameDigit reports whether every rune of a digit string is identical (e.g. "00000000000"), a class
// of values that pass several national checksums but are never valid identifiers.
func allSameDigit(d string) bool {
	for i := 1; i < len(d); i++ {
		if d[i] != d[0] {
			return false
		}
	}
	return len(d) > 0
}

// validateRUINN validates the Russian individual ИНН: 12 digits with two weighted check digits.
// d₁₁ uses weights [7,2,4,10,3,5,9,4,6,8] over the first 10; d₁₂ uses [3,7,2,4,10,3,5,9,4,6,8] over the
// first 11; each check = (Σ wᵢdᵢ) mod 11 mod 10.
func validateRUINN(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 12 {
		return "", errors.Join(ErrInvalid, errors.New("ИНН must be 12 digits"))
	}
	w11 := [10]int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	s := 0
	for i := 0; i < 10; i++ {
		s += int(d[i]-'0') * w11[i]
	}
	if (s%11)%10 != int(d[10]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("ИНН checksum does not match"))
	}
	w12 := [11]int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	s = 0
	for i := 0; i < 11; i++ {
		s += int(d[i]-'0') * w12[i]
	}
	if (s%11)%10 != int(d[11]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("ИНН checksum does not match"))
	}
	return d, nil
}

// validateRUSNILS validates the Russian СНИЛС: 11 digits where the last two are a check number over the
// first nine. sum = Σ dᵢ·(9−i) for i=0..8; check = sum mod 101 (a result of 100 maps to 00).
func validateRUSNILS(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 11 {
		return "", errors.Join(ErrInvalid, errors.New("СНИЛС must be 11 digits"))
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(d[i]-'0') * (9 - i)
	}
	check := sum % 101
	if check == 100 {
		check = 0
	}
	want := int(d[9]-'0')*10 + int(d[10]-'0')
	if check != want {
		return "", errors.Join(ErrInvalid, errors.New("СНИЛС checksum does not match"))
	}
	return d, nil
}

// validateBRCPF validates the Brazilian CPF: 11 digits with two mod-11 check digits over the preceding
// digits (weights descending from 10, then 11); a result of 10 maps to 0. All-equal sequences are
// rejected (they satisfy the arithmetic but are not valid CPFs).
func validateBRCPF(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 11 {
		return "", errors.Join(ErrInvalid, errors.New("CPF must be 11 digits"))
	}
	if allSameDigit(d) {
		return "", errors.Join(ErrInvalid, errors.New("CPF is not a valid number"))
	}
	s := 0
	for i := 0; i < 9; i++ {
		s += int(d[i]-'0') * (10 - i)
	}
	r := (s * 10) % 11
	if r == 10 {
		r = 0
	}
	if r != int(d[9]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("CPF checksum does not match"))
	}
	s = 0
	for i := 0; i < 10; i++ {
		s += int(d[i]-'0') * (11 - i)
	}
	r = (s * 10) % 11
	if r == 10 {
		r = 0
	}
	if r != int(d[10]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("CPF checksum does not match"))
	}
	return d, nil
}

// validateARCUIL validates the Argentine CUIL/CUIT: 11 digits with a mod-11 check digit using weights
// [5,4,3,2,7,6,5,4,3,2]; check = 11 − (Σ wᵢdᵢ mod 11), where 11 maps to 0 and 10 is invalid.
func validateARCUIL(value string) (string, error) {
	d := digitsOnly(value)
	if len(d) != 11 {
		return "", errors.Join(ErrInvalid, errors.New("CUIL must be 11 digits"))
	}
	w := [10]int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	s := 0
	for i := 0; i < 10; i++ {
		s += int(d[i]-'0') * w[i]
	}
	check := 11 - (s % 11)
	switch check {
	case 11:
		check = 0
	case 10:
		return "", errors.Join(ErrInvalid, errors.New("CUIL checksum does not match"))
	}
	if check != int(d[10]-'0') {
		return "", errors.Join(ErrInvalid, errors.New("CUIL checksum does not match"))
	}
	return d, nil
}

// validateCLRUT validates the Chilean RUT/RUN: a 7–8 digit body plus a mod-11 verifier (0–9 or K),
// computed with cyclic weights 2..7 over the reversed body. Dots/dashes are stripped; the normalized
// form is the bare body+verifier (e.g. "12345678K").
func validateCLRUT(value string) (string, error) {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if (r >= '0' && r <= '9') || r == 'K' {
			b.WriteRune(r)
		}
	}
	norm := b.String()
	if len(norm) < 8 || len(norm) > 9 {
		return "", errors.Join(ErrInvalid, errors.New("RUT must be a 7-8 digit body plus a verifier"))
	}
	body, ver := norm[:len(norm)-1], norm[len(norm)-1]
	for i := 0; i < len(body); i++ {
		if body[i] < '0' || body[i] > '9' {
			return "", errors.Join(ErrInvalid, errors.New("RUT body must be digits"))
		}
	}
	sum, weight := 0, 2
	for i := len(body) - 1; i >= 0; i-- {
		sum += int(body[i]-'0') * weight
		if weight++; weight > 7 {
			weight = 2
		}
	}
	var expected byte
	switch r := 11 - (sum % 11); r {
	case 11:
		expected = '0'
	case 10:
		expected = 'K'
	default:
		expected = byte('0' + r)
	}
	if ver != expected {
		return "", errors.Join(ErrInvalid, errors.New("RUT verifier does not match"))
	}
	return norm, nil
}

// reMXCURP / reMXRFC are the structural patterns for the Mexican CURP (18) and RFC for personas físicas
// (13). The homoclave/check character is name-derived and not verifiable from the code alone, so these
// validators check format only (a legitimate compiled validator: uppercases + structurally validates).
var (
	reMXCURP = regexp.MustCompile(`^[A-Z]{4}[0-9]{6}[HM][A-Z]{2}[A-Z]{3}[0-9A-Z][0-9]$`)
	reMXRFC  = regexp.MustCompile(`^[A-ZÑ&]{4}[0-9]{6}[0-9A-Z]{3}$`)
)

// validateMXCURP structurally validates a Mexican CURP (18 chars) and returns its uppercased form.
func validateMXCURP(value string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(value))
	if !reMXCURP.MatchString(s) {
		return "", errors.Join(ErrInvalid, errors.New("CURP must be 18 characters in the CURP format"))
	}
	return s, nil
}

// validateMXRFC structurally validates a Mexican RFC for a persona física (13 chars) and returns its
// uppercased form.
func validateMXRFC(value string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(value))
	if !reMXRFC.MatchString(s) {
		return "", errors.Join(ErrInvalid, errors.New("RFC must be 13 characters in the RFC format"))
	}
	return s, nil
}
