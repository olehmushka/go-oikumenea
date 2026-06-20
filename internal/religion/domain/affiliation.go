// Lay-affiliation domain (D-ReligiousAffiliation / D-SpecialPII, M24): the per-tradition affiliation-type
// catalog + the reified, GDPR Art. 9 pii:special affiliation Link (Person → a faith/tradition/community +
// type). The optional belief value is envelope-encrypted at rest; the application layer seals/opens it
// and the repository sees only ciphertext (StoredAffiliation), mirroring document personal codes.
package domain

import (
	"errors"
	"strings"
	"time"
)

// AffiliationType is a per-tradition lay-affiliation / milestone type (adherent/baptized/shahada/…).
type AffiliationType struct {
	ID               string
	TraditionTaxonID string // "" = generic
	Code             string
	Name             string
	Status           string
	SortOrder        *int
}

// Affiliation is a lay affiliation with the belief Value DECRYPTED (the application-facing shape).
type Affiliation struct {
	ID                  string
	PersonID            string
	ReligionID          string // "" = unset
	TraditionUnitID     string // "" = unset
	CommunityUnitID     string // "" = unset
	AffiliationTypeID   string
	AffiliationTypeCode string
	AffiliationTypeName string // default-locale; translated via the i18n store
	Value               string // decrypted belief detail ("" = unset or crypto-erased)
	Status              string // active | lapsed | renounced
	EffectiveFrom       time.Time
	EffectiveTo         *time.Time
	Source              string
	Confidence          string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StoredAffiliation is the persistence-facing shape: the belief value stays ENCRYPTED (envelope columns).
type StoredAffiliation struct {
	ID                  string
	PersonID            string
	ReligionID          string
	TraditionUnitID     string
	CommunityUnitID     string
	AffiliationTypeID   string
	AffiliationTypeCode string
	AffiliationTypeName string
	ValueCiphertext     []byte
	WrappedDEK          []byte
	KeyRef              string
	ValueBlindIndex     []byte
	Status              string
	EffectiveFrom       time.Time
	EffectiveTo         *time.Time
	Source              string
	Confidence          string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// AffiliationInput is the create payload (structural anchors + the already-sealed envelope, if any).
type AffiliationInput struct {
	PersonID          string
	AffiliationTypeID string
	ReligionID        string
	TraditionUnitID   string
	CommunityUnitID   string
	Source            string
	Confidence        string
	// Envelope set by the application after Seal (empty when no value supplied).
	ValueCiphertext []byte
	WrappedDEK      []byte
	KeyRef          string
	ValueBlindIndex []byte
}

// Validate checks a create input.
func (in AffiliationInput) Validate() error {
	if strings.TrimSpace(in.PersonID) == "" || strings.TrimSpace(in.AffiliationTypeID) == "" {
		return ErrInvalid
	}
	return nil
}

// AffiliationUpdate patches an affiliation: status and/or a re-sealed value.
type AffiliationUpdate struct {
	Status *string
	// ValueProvided replaces the envelope; the application seals the new value first.
	ValueProvided   bool
	ValueCiphertext []byte
	WrappedDEK      []byte
	KeyRef          string
	ValueBlindIndex []byte
}

var affiliationStatuses = map[string]struct{}{"active": {}, "lapsed": {}, "renounced": {}}

// ValidAffiliationStatus reports whether s is a known affiliation status.
func ValidAffiliationStatus(s string) bool { _, ok := affiliationStatuses[s]; return ok }

var (
	ErrAffiliationTypeNotFound = errors.New("religion: affiliation type not found")
	ErrAffiliationNotFound     = errors.New("religion: affiliation not found")
)
