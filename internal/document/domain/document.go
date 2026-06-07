// Package domain holds the document module's pure logic: person-held papers (Document) typed by a
// catalog (DocumentType), and government personal codes (PersonalCode) typed by a country-namespaced
// catalog (PersonalCodeScheme), plus the Repository port it needs from the outside world (overview.md
// layering). No I/O, no framework imports — only the standard library.
//
// A personal code's VALUE is pii:sensitive: this package models the plaintext view (PersonalCode) and
// the encrypted persistence view (StoredPersonalCode) separately, and the Repository deals only in the
// latter's opaque bytes — encryption lives in the application layer over pkg/crypto (D-CryptoProvider).
// A document/code carries NO authority (directory data, like rank/position); this package never reads
// it to make a decision. Visibility derives from the holder, gated at read time by the PDP (M7).
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ISODate is the wire/calendar date format (a day, not an instant) for issued/expires dates.
const ISODate = "2006-01-02"

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (unique indexes, RESTRICT FKs, CHECKs) enforce the same shapes as a backstop.
var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrDocumentConflict = errors.New("the person already holds this (type, number)")
	ErrDocumentInvalid  = errors.New("invalid document request")

	ErrDocumentTypeNotFound = errors.New("document type not found")
	ErrDocumentTypeConflict = errors.New("document type code already exists")

	ErrPersonalCodeNotFound  = errors.New("personal code not found")
	ErrPersonalCodeInvalid   = errors.New("invalid personal code request")
	ErrPersonalCodeDuplicate = errors.New("this (scheme, value) is already held")

	ErrSchemeNotFound = errors.New("personal-code scheme not found")
	ErrSchemeConflict = errors.New("personal-code scheme code already exists")

	// FK-validation sentinels: unknown references caught by the DB foreign keys and surfaced as
	// INVALID_ARGUMENT by the transport.
	ErrUnknownPerson  = errors.New("person does not exist")
	ErrUnknownType    = errors.New("document type does not exist")
	ErrUnknownCountry = errors.New("country does not exist")
	ErrUnknownScheme  = errors.New("personal-code scheme does not exist")
)

// DocumentStatus / CatalogStatus are the lifecycle CHECK vocabularies.
type DocumentStatus string

const (
	DocumentActive     DocumentStatus = "active"
	DocumentSuperseded DocumentStatus = "superseded"
	DocumentRevoked    DocumentStatus = "revoked"
)

func validDocumentStatus(s string) bool {
	switch DocumentStatus(s) {
	case DocumentActive, DocumentSuperseded, DocumentRevoked:
		return true
	}
	return false
}

// CatalogStatus is the active|retired status shared by the type and scheme catalogs.
type CatalogStatus string

const (
	CatalogActive  CatalogStatus = "active"
	CatalogRetired CatalogStatus = "retired"
)

func validCatalogStatus(s string) bool {
	return CatalogStatus(s) == CatalogActive || CatalogStatus(s) == CatalogRetired
}

// genericCategories is the closed semantic-grouping vocabulary for personal-code schemes (the
// cross-scheme join key; D-PersonalCodes), matching the DB CHECK.
var genericCategories = map[string]struct{}{
	"tax-id": {}, "national-id": {}, "social-insurance": {},
	"health-insurance": {}, "residence-permit": {}, "other": {},
}

// ---------------------------------------------------------------- document type (paper catalog)

// DocumentType is an instance-admin catalog entry for a PAPER kind. Name is the default-locale
// fallback; the transport assembles the locale->text map from the i18n store.
type DocumentType struct {
	ID         string
	Code       string
	Name       string
	AttrSchema json.RawMessage // optional per-type attribute schema (D-DocumentAttrSchema); nil = none
	Status     CatalogStatus
	SortOrder  *int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Validate enforces a valid code, a non-empty name, and (when present) a well-formed attr_schema.
func (t DocumentType) Validate() error {
	if !validCode(t.Code) {
		return wrapInvalid(ErrDocumentInvalid, "code must be non-empty, <=128 chars, and contain no whitespace")
	}
	if strings.TrimSpace(t.Name) == "" {
		return wrapInvalid(ErrDocumentInvalid, "name is required")
	}
	return ValidateAttrSchema(t.AttrSchema)
}

// DocumentTypePatch is a partial update (nil = unchanged). Code is immutable by convention.
type DocumentTypePatch struct {
	Name       *string
	AttrSchema *json.RawMessage // nil = unchanged; non-nil replaces the schema (D-DocumentAttrSchema)
	Status     *string
	SortOrder  *int
}

// Validate checks the present fields.
func (p DocumentTypePatch) Validate() error {
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return wrapInvalid(ErrDocumentInvalid, "name cannot be cleared")
	}
	if p.Status != nil && !validCatalogStatus(*p.Status) {
		return wrapInvalid(ErrDocumentInvalid, "status must be active or retired")
	}
	if p.AttrSchema != nil {
		return ValidateAttrSchema(*p.AttrSchema)
	}
	return nil
}

// ---------------------------------------------------------------- document (paper)

// Document is a person-held paper of some type (aggregate root). IssuedOn/ExpiresOn are ISO-8601 date
// strings ("" = absent); Attributes is raw JSON (the pii:special ceiling).
type Document struct {
	ID             string
	PersonID       string
	TypeID         string
	Number         string
	Issuer         string
	IssuingCountry string
	IssuedOn       string
	ExpiresOn      string
	Attributes     json.RawMessage
	Status         DocumentStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate enforces the create-time invariants: a type reference and well-formed dates. Unknown
// person/type/country are caught by the DB FKs and surfaced as the matching ErrUnknown* sentinel.
func (d Document) Validate() error {
	if strings.TrimSpace(d.TypeID) == "" {
		return wrapInvalid(ErrDocumentInvalid, "typeId is required")
	}
	if err := validDate(d.IssuedOn); err != nil {
		return wrapInvalid(ErrDocumentInvalid, "issuedOn must be an ISO-8601 date (YYYY-MM-DD)")
	}
	if err := validDate(d.ExpiresOn); err != nil {
		return wrapInvalid(ErrDocumentInvalid, "expiresOn must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// DocumentPatch is a partial update (nil = unchanged). A NULL narg leaves the stored value unchanged
// (COALESCE); clearing number/issuer/issuing-country to NULL is an open seam.
type DocumentPatch struct {
	Number         *string
	Issuer         *string
	IssuingCountry *string
	IssuedOn       *string
	ExpiresOn      *string
	Attributes     *json.RawMessage
	Status         *string
}

// Validate checks the present fields.
func (p DocumentPatch) Validate() error {
	if p.IssuedOn != nil {
		if err := validDate(*p.IssuedOn); err != nil {
			return wrapInvalid(ErrDocumentInvalid, "issuedOn must be an ISO-8601 date (YYYY-MM-DD)")
		}
	}
	if p.ExpiresOn != nil {
		if err := validDate(*p.ExpiresOn); err != nil {
			return wrapInvalid(ErrDocumentInvalid, "expiresOn must be an ISO-8601 date (YYYY-MM-DD)")
		}
	}
	if p.Status != nil && !validDocumentStatus(*p.Status) {
		return wrapInvalid(ErrDocumentInvalid, "status must be active, superseded, or revoked")
	}
	return nil
}

// ---------------------------------------------------------------- personal-code scheme (catalog)

// PersonalCodeScheme is the country-namespaced national-identifier scheme catalog entry. Name is the
// default-locale fallback. CountryISO/ValidationRegex are "" when unset.
type PersonalCodeScheme struct {
	Code            string
	CountryISO      string
	GenericCategory string
	Name            string
	ValidationRegex string
	Status          CatalogStatus
	SortOrder       *int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Validate enforces a valid code, a known generic category, and a non-empty name on create.
func (s PersonalCodeScheme) Validate() error {
	if !validCode(s.Code) {
		return wrapInvalid(ErrPersonalCodeInvalid, "code must be non-empty, <=128 chars, and contain no whitespace")
	}
	if _, ok := genericCategories[s.GenericCategory]; !ok {
		return wrapInvalid(ErrPersonalCodeInvalid, "genericCategory must be one of tax-id|national-id|social-insurance|health-insurance|residence-permit|other")
	}
	if strings.TrimSpace(s.Name) == "" {
		return wrapInvalid(ErrPersonalCodeInvalid, "name is required")
	}
	return nil
}

// PersonalCodeSchemePatch is a partial update (nil = unchanged). Code is immutable by convention.
type PersonalCodeSchemePatch struct {
	CountryISO      *string
	GenericCategory *string
	Name            *string
	ValidationRegex *string
	Status          *string
	SortOrder       *int
}

// Validate checks the present fields.
func (p PersonalCodeSchemePatch) Validate() error {
	if p.GenericCategory != nil {
		if _, ok := genericCategories[*p.GenericCategory]; !ok {
			return wrapInvalid(ErrPersonalCodeInvalid, "genericCategory is invalid")
		}
	}
	if p.Name != nil && strings.TrimSpace(*p.Name) == "" {
		return wrapInvalid(ErrPersonalCodeInvalid, "name cannot be cleared")
	}
	if p.Status != nil && !validCatalogStatus(*p.Status) {
		return wrapInvalid(ErrPersonalCodeInvalid, "status must be active or retired")
	}
	return nil
}

// ---------------------------------------------------------------- personal code (value)

// PersonalCode is the plaintext, API-facing view of a person-held national identifier: Value is the
// decrypted identifier (pii:sensitive). It is produced by the application after decryption and is
// never persisted in this shape.
type PersonalCode struct {
	ID         string
	PersonID   string
	SchemeCode string
	Value      string
	Status     DocumentStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StoredPersonalCode is the encrypted persistence view the Repository reads/writes: the value lives as
// opaque ciphertext + wrapped DEK + key ref + blind index (D-CryptoProvider). The domain treats these
// as bytes; the application owns the crypto. After crypto-erase ValueCiphertext/WrappedDEK are nil.
type StoredPersonalCode struct {
	ID              string
	PersonID        string
	SchemeCode      string
	ValueCiphertext []byte
	WrappedDEK      []byte
	KeyRef          string
	ValueBlindIndex []byte
	Status          DocumentStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// StoredPersonalCodeUpdate is the full mutable set for an update: the application supplies either the
// re-encrypted value fields (when the value changes) or the existing ones (unchanged), plus the status.
type StoredPersonalCodeUpdate struct {
	ValueCiphertext []byte
	WrappedDEK      []byte
	KeyRef          string
	ValueBlindIndex []byte
	Status          DocumentStatus
}

// ---------------------------------------------------------------- shared helpers

func wrapInvalid(base error, msg string) error { return errors.Join(base, errors.New(msg)) }

// validDate accepts "" (absent) or a strict ISO-8601 calendar date.
func validDate(s string) error {
	if s == "" {
		return nil
	}
	_, err := time.Parse(ISODate, s)
	return err
}

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted rows.
type Repository interface {
	// document types
	InsertDocumentType(ctx context.Context, t DocumentType) (DocumentType, error)
	GetDocumentType(ctx context.Context, id string) (DocumentType, error)
	UpdateDocumentType(ctx context.Context, id string, patch DocumentTypePatch) (DocumentType, error)
	ListDocumentTypes(ctx context.Context) ([]DocumentType, error)

	// documents (papers)
	InsertDocument(ctx context.Context, d Document) (Document, error)
	GetDocument(ctx context.Context, id string) (Document, error)
	UpdateDocument(ctx context.Context, id string, patch DocumentPatch) (Document, error)
	SoftDeleteDocument(ctx context.Context, id string) error
	ListDocumentsByPerson(ctx context.Context, personID, after string, limit int) ([]Document, error)
	// ErasePersonDocuments NULLs number/issuer/attributes for a person's documents (purge; D-PIITiers).
	ErasePersonDocuments(ctx context.Context, personID string) (int64, error)

	// personal-code schemes
	InsertScheme(ctx context.Context, s PersonalCodeScheme) (PersonalCodeScheme, error)
	GetScheme(ctx context.Context, code string) (PersonalCodeScheme, error)
	UpdateScheme(ctx context.Context, code string, patch PersonalCodeSchemePatch) (PersonalCodeScheme, error)
	ListSchemes(ctx context.Context, country, category string) ([]PersonalCodeScheme, error)

	// personal codes (encrypted values)
	InsertPersonalCode(ctx context.Context, c StoredPersonalCode) (StoredPersonalCode, error)
	GetPersonalCode(ctx context.Context, id string) (StoredPersonalCode, error)
	UpdatePersonalCode(ctx context.Context, id string, upd StoredPersonalCodeUpdate) (StoredPersonalCode, error)
	SoftDeletePersonalCode(ctx context.Context, id string) error
	ListPersonalCodesByPerson(ctx context.Context, personID string) ([]StoredPersonalCode, error)
	// CryptoErasePersonCodes destroys the wrapped DEK + ciphertext of a person's codes (purge).
	CryptoErasePersonCodes(ctx context.Context, personID string) (int64, error)
}
