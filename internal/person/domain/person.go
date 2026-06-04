// Package domain holds the person module's pure logic: the directory aggregate (Person), its
// per-person Name variants, temporal Citizenship/Residence links, the reversible deactivate -> purge
// lifecycle, and the Repository port it needs from the outside world (overview.md layering). No I/O,
// no framework imports — only the standard library.
//
// A person is instance-global (D-PersonGlobal), account-optional (L-AccountOptional), and holds at
// most one rank — a DIRECTORY attribute that grants no authority (D-Rank); this package never reads
// rank to make a decision. Names follow the Unicode CLDR fixed field set (D-PersonNamesCLDR):
// DisplayName is authoritative, the structured parts are advisory, and there is no patronymic field
// (the Slavic по-батькові lives in Given2). Calendar dates are carried as ISO-8601 "YYYY-MM-DD"
// strings (a day, not an instant); "" means absent.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ISODate is the layout person calendar-date fields (birthdate, citizenship/residence windows) are
// formatted with — a day, not an instant (D-PersonBio / D-Geo).
const ISODate = "2006-01-02"

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (partial-unique code/citizenship, RESTRICT/CASCADE FKs) enforce the same shapes as a backstop.
var (
	ErrNotFound            = errors.New("person not found")
	ErrCodeConflict        = errors.New("person code already exists")
	ErrCitizenshipConflict = errors.New("active citizenship for this country already exists")
	ErrInvalid             = errors.New("invalid person request")
	ErrUnknownRank         = errors.New("rank does not exist")
	ErrUnknownCountry      = errors.New("country does not exist")
	ErrUnknownLocale       = errors.New("locale does not exist")
	ErrNameVariantNotFound = errors.New("name variant not found")
	ErrCitizenshipNotFound = errors.New("citizenship not found")
	ErrResidenceNotFound   = errors.New("residence not found")
	ErrLifecycle           = errors.New("invalid lifecycle transition")
)

// Status is the person lifecycle state (D-PersonReadScope reversibility window).
type Status string

const (
	StatusActive      Status = "active"
	StatusDeactivated Status = "deactivated"
	StatusPurged      Status = "purged"
)

// Sex is the ISO/IEC 5218 biological-sex value, stored as readable text (D-PersonBio). It is NOT
// gender identity (which would be pii:special and is out of scope).
var validSex = map[string]bool{"not_known": true, "male": true, "female": true, "not_applicable": true}

// CitizenshipBasis records how a citizenship was acquired (D-Geo).
var validBasis = map[string]bool{"birth": true, "descent": true, "naturalization": true, "other": true}

// DefaultSex / DefaultBasis are the values substituted when the request omits them.
const (
	DefaultSex   = "not_known"
	DefaultBasis = "other"
)

// Name is the Unicode CLDR Person Names fixed field set shared by a Person and each of its name
// variants (D-PersonNamesCLDR). DisplayName is authoritative; every other part is advisory and ""
// when unset. There is intentionally no patronymic field — the по-батькові lives in Given2.
type Name struct {
	DisplayName   string
	Title         string
	Given         string
	Given2        string
	Surname       string
	SurnamePrefix string
	Surname2      string
	Generation    string
	Credentials   string
	Preferred     string
}

func (n Name) validate() error {
	if strings.TrimSpace(n.DisplayName) == "" {
		return wrapInvalid("displayName is required")
	}
	return nil
}

// Person is the directory aggregate root. Attributes is the long-tail JSONB directory grab-bag
// (pii:special ceiling); "" / nil means the default empty object. NameVariants/Citizenships/
// Residences are populated only when a single person is read, and are empty in list responses.
type Person struct {
	ID             string
	Code           string // "" when unset; unique among active persons
	Name                  // embedded CLDR parts (Person.DisplayName etc.)
	Birthdate      string // ISO-8601 date or ""
	Sex            string
	CountryOfBirth string // ISO-3166-1 alpha-2 or ""
	Attributes     []byte // raw JSON; nil/empty => "{}"
	RankID         string // "" when unranked
	Status         Status
	DeactivatedAt  *time.Time
	PurgeAfter     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time

	NameVariants []NameVariant
	Citizenships []Citizenship
	Residences   []Residence
}

// Validate enforces the create-time invariants: a valid optional code, a non-empty display name, a
// known sex, and a parseable optional birthdate. Unknown rank/country codes are caught by the DB FKs
// and surfaced as ErrUnknownRank / ErrUnknownCountry.
func (p Person) Validate() error {
	if p.Code != "" && !validCode(p.Code) {
		return wrapInvalid("code must be non-empty and contain no whitespace")
	}
	if err := p.Name.validate(); err != nil {
		return err
	}
	if p.Sex != "" && !validSex[p.Sex] {
		return wrapInvalid("sex must be one of not_known|male|female|not_applicable")
	}
	if !validDate(p.Birthdate) {
		return wrapInvalid("birthdate must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// CanReactivate reports whether the person may be reactivated (only from deactivated).
func (p Person) CanReactivate() bool { return p.Status == StatusDeactivated }

// CanPurge reports whether the person may be purged at time now: it must be deactivated and past the
// grace window (purge_after). A person never deactivated has no purge_after and cannot be purged.
func (p Person) CanPurge(now time.Time) bool {
	return p.Status == StatusDeactivated && p.PurgeAfter != nil && !now.Before(*p.PurgeAfter)
}

// PersonPatch is a partial update (nil = unchanged). An empty-string pointer clears an optional name
// part. Code is immutable by convention and rank is set via the dedicated SetRank path.
type PersonPatch struct {
	DisplayName    *string
	Title          *string
	Given          *string
	Given2         *string
	Surname        *string
	SurnamePrefix  *string
	Surname2       *string
	Generation     *string
	Credentials    *string
	Preferred      *string
	Birthdate      *string
	Sex            *string
	CountryOfBirth *string
	Attributes     []byte // nil = unchanged
}

// Validate enforces the patch invariants for the fields actually present.
func (p PersonPatch) Validate() error {
	if p.DisplayName != nil && strings.TrimSpace(*p.DisplayName) == "" {
		return wrapInvalid("displayName cannot be cleared")
	}
	if p.Sex != nil && !validSex[*p.Sex] {
		return wrapInvalid("sex must be one of not_known|male|female|not_applicable")
	}
	if p.Birthdate != nil && !validDate(*p.Birthdate) {
		return wrapInvalid("birthdate must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// NameVariant is a full transliterated name form for one locale (e.g. ukr native, eng Latin) —
// per-person data managed by the person's admins, NOT the instance localization store (D-i18n).
type NameVariant struct {
	ID        string
	PersonID  string
	Locale    string
	Name      // embedded CLDR parts
	IsPrimary bool
}

// Validate enforces a non-empty locale and display name.
func (v NameVariant) Validate() error {
	if strings.TrimSpace(v.Locale) == "" {
		return wrapInvalid("locale is required")
	}
	return v.Name.validate()
}

// Citizenship is a person's effective-dated nationality in a country (D-Geo). A person may hold
// several; at most one active per country, and IsPrimary marks at most one.
type Citizenship struct {
	ID         string
	PersonID   string
	Country    string
	Basis      string
	AcquiredOn string // ISO-8601 date or ""
	LostOn     string // ISO-8601 date or "" (still held)
	IsPrimary  bool
}

// Validate enforces a 2-letter country code, a known basis, and parseable optional dates.
func (c Citizenship) Validate() error {
	if !validCountry(c.Country) {
		return wrapInvalid("country must be a 2-letter ISO-3166-1 code")
	}
	if c.Basis != "" && !validBasis[c.Basis] {
		return wrapInvalid("basis must be one of birth|descent|naturalization|other")
	}
	if !validDate(c.AcquiredOn) || !validDate(c.LostOn) {
		return wrapInvalid("acquiredOn/lostOn must be ISO-8601 dates (YYYY-MM-DD)")
	}
	return nil
}

// Residence is a person's effective-dated residence in a country/region (D-Geo); locator data.
type Residence struct {
	ID        string
	PersonID  string
	Country   string
	Region    string
	ValidFrom string // ISO-8601 date (required)
	ValidTo   string // ISO-8601 date or "" (current)
}

// Validate enforces a 2-letter country code and a required, parseable valid_from (plus optional
// valid_to).
func (r Residence) Validate() error {
	if !validCountry(r.Country) {
		return wrapInvalid("country must be a 2-letter ISO-3166-1 code")
	}
	if r.ValidFrom == "" || !validDate(r.ValidFrom) {
		return wrapInvalid("validFrom is required and must be an ISO-8601 date (YYYY-MM-DD)")
	}
	if !validDate(r.ValidTo) {
		return wrapInvalid("validTo must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

func wrapInvalid(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code:
// operator-assigned, locale-agnostic, immutable by convention).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// validCountry checks the ISO-3166-1 alpha-2 shape (existence is enforced by the geo FK).
func validCountry(c string) bool {
	if len(c) != 2 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// validDate reports whether s is empty (absent) or a valid ISO-8601 calendar date.
func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse(ISODate, s)
	return err == nil
}

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted rows.
type Repository interface {
	// persons
	InsertPerson(ctx context.Context, p Person) (Person, error)
	GetPerson(ctx context.Context, id string) (Person, error)
	// GetActivePersonByCode resolves an active person by stable code (JIT/bootstrap); ErrNotFound
	// when none matches.
	GetActivePersonByCode(ctx context.Context, code string) (Person, error)
	UpdatePerson(ctx context.Context, id string, patch PersonPatch) (Person, error)
	ListPersons(ctx context.Context, after string, limit int) ([]Person, error)
	SetRank(ctx context.Context, id string, rankID *string) (Person, error)

	// lifecycle
	Deactivate(ctx context.Context, id string, purgeAfter time.Time) (Person, error)
	Reactivate(ctx context.Context, id string) (Person, error)
	Purge(ctx context.Context, id string) (Person, error) // NULLs PII, removes child rows, status=purged

	// name variants
	UpsertNameVariant(ctx context.Context, v NameVariant) (NameVariant, error)
	ClearPrimaryNameVariants(ctx context.Context, personID string) error
	DeleteNameVariant(ctx context.Context, personID, locale string) error
	ListNameVariants(ctx context.Context, personID string) ([]NameVariant, error)

	// citizenships
	UpsertCitizenship(ctx context.Context, c Citizenship) (Citizenship, error)
	ClearPrimaryCitizenships(ctx context.Context, personID string) error
	DeleteCitizenship(ctx context.Context, personID, country string) error
	ListCitizenships(ctx context.Context, personID string) ([]Citizenship, error)

	// residences (r.ID == "" => insert; otherwise replace that row)
	UpsertResidence(ctx context.Context, r Residence) (Residence, error)
	DeleteResidence(ctx context.Context, personID, residenceID string) error
	ListResidences(ctx context.Context, personID string) ([]Residence, error)
}
