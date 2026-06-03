package domain

import (
	"errors"
	"testing"
)

func TestLocaleValidate(t *testing.T) {
	cases := []struct {
		name    string
		locale  Locale
		wantErr bool
	}{
		{"valid", Locale{Code: "ukr", Name: "Українська"}, false},
		{"valid eng", Locale{Code: "eng", Name: "English"}, false},
		{"code too short", Locale{Code: "uk", Name: "x"}, true},
		{"code too long", Locale{Code: "ukrr", Name: "x"}, true},
		{"code uppercase", Locale{Code: "UKR", Name: "x"}, true},
		{"code with digit", Locale{Code: "uk1", Name: "x"}, true},
		{"empty name", Locale{Code: "ukr", Name: "  "}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.locale.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr && !errors.Is(err, ErrInvalidLocale) {
				t.Fatalf("error should wrap ErrInvalidLocale, got %v", err)
			}
		})
	}
}

func TestUnknownLocaleErrorUnwraps(t *testing.T) {
	err := error(UnknownLocaleError{Code: "rus"})
	if !errors.Is(err, ErrUnknownLocale) {
		t.Fatalf("UnknownLocaleError should unwrap to ErrUnknownLocale")
	}
	var ule UnknownLocaleError
	if !errors.As(err, &ule) || ule.Code != "rus" {
		t.Fatalf("errors.As should recover the code, got %+v", ule)
	}
}
