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
