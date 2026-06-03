// Package domain holds the localization module's pure logic: the Locale registry entry and the
// polymorphic Translation it stores, their invariants, and the Repository port it needs from the
// outside world (overview.md layering). No I/O, no framework imports — only the standard library.
// Localization owns i18n (D-i18n / docs/modules/localization.md): the supported-locale registry and
// the translatable name/title/description of other modules' entities, returned everywhere as a
// locale->text map.
package domain

import (
	"context"
	"errors"
	"strings"
)

// ErrLocaleNotFound is returned by the repository when no locale has the given code.
var ErrLocaleNotFound = errors.New("locale not found")

// ErrLocaleConflict is returned when adding a locale whose code already exists.
var ErrLocaleConflict = errors.New("locale code already exists")

// ErrInvalidLocale is the sentinel wrapped by Locale.Validate failures (mapped to INVALID_ARGUMENT
// by the transport layer).
var ErrInvalidLocale = errors.New("invalid locale")

// ErrLocaleConstraint is returned when a change would leave zero enabled locales or
// not-exactly-one default — the registry invariant the DB constraint trigger also enforces.
var ErrLocaleConstraint = errors.New("locale registry constraint violated")

// ErrUnknownLocale is returned (wrapped by UnknownLocaleError) when a translation cites a locale
// code that is not a known, enabled locale (mapped to INVALID_ARGUMENT).
var ErrUnknownLocale = errors.New("unknown locale")

// UnknownLocaleError carries the offending locale code so the transport can name it in the Conjure
// error. It unwraps to ErrUnknownLocale, so errors.Is(err, ErrUnknownLocale) holds.
type UnknownLocaleError struct{ Code string }

func (e UnknownLocaleError) Error() string { return "unknown locale: " + e.Code }
func (e UnknownLocaleError) Unwrap() error { return ErrUnknownLocale }

// Locale is one supported language for the deployment (D-i18n). Code is the stable, locale-agnostic
// ISO 639-3 identifier (D-Code) and the natural key; Name is the endonym. Exactly one enabled
// locale is the default.
type Locale struct {
	Code      string
	Name      string
	Enabled   bool
	IsDefault bool
	SortOrder int
}

// LocalePatch is a partial update of a locale: a nil field leaves the stored value unchanged.
type LocalePatch struct {
	Name      *string
	Enabled   *bool
	IsDefault *bool
	SortOrder *int
}

// Validate enforces the locale invariants before insert: a 3-letter ISO 639-3 code and a non-empty
// name. The DB UNIQUE/CHECK constraints enforce the same shape as a backstop.
func (l Locale) Validate() error {
	if !isISO6393(l.Code) {
		return wrapInvalid("code must be a 3-letter ISO 639-3 identifier")
	}
	if strings.TrimSpace(l.Name) == "" {
		return wrapInvalid("name is required")
	}
	return nil
}

// ValidateCode checks an ISO 639-3 code in isolation (used on the {localeCode} path param).
func ValidateCode(code string) error {
	if !isISO6393(code) {
		return wrapInvalid("code must be a 3-letter ISO 639-3 identifier")
	}
	return nil
}

func wrapInvalid(msg string) error { return errors.Join(ErrInvalidLocale, errors.New(msg)) }

// isISO6393 is the cheap shape guard: exactly three lowercase ASCII letters (ISO 639-3 alpha-3).
func isISO6393(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, r := range code {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// Translation is one (entity_type, entity_id, field, locale) -> text row: the localized value of a
// named translatable field of some other module's entity. The entity_id is polymorphic and carries
// no FK (it spans many tables; docs/modules/localization.md).
type Translation struct {
	EntityType string
	EntityID   string
	Field      string
	Locale     string
	Text       string
}

// TranslationKey identifies a translatable field of an entity (without the locale): the key of the
// locale->text map TranslationsFor assembles for the owning module's responses.
type TranslationKey struct {
	EntityID string
	Field    string
}

// Repository is the persistence port the localization application service depends on; the pgx/sqlc
// adapter implements it. Each method runs on whatever DBTX the adapter was constructed with, so a
// write + its audit row can share one transaction (D-Audit).
type Repository interface {
	ListLocales(ctx context.Context) ([]Locale, error)
	GetLocaleByCode(ctx context.Context, code string) (Locale, error)
	InsertLocale(ctx context.Context, l Locale) (Locale, error)
	UpdateLocale(ctx context.Context, code string, patch LocalePatch) (Locale, error)
	// ClearDefault unsets is_default on every enabled, non-deleted locale (used before promoting a
	// new default, within the caller's transaction).
	ClearDefault(ctx context.Context) error
	// ExistingLocaleCodes returns the subset of codes that are known, enabled, non-deleted locales.
	ExistingLocaleCodes(ctx context.Context, codes []string) ([]string, error)
	GetTranslations(ctx context.Context, entityType, entityID string) ([]Translation, error)
	UpsertTranslations(ctx context.Context, ts []Translation) error
	TranslationsForBatch(ctx context.Context, entityType string, entityIDs, fields []string) ([]Translation, error)
}
