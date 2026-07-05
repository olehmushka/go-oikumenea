// Package domain holds the finance module's entities, ports, and invariants (docs/modules/finance.md /
// D-Finance, M44): bank accounts (envelope-encrypted IBAN), payment cards (envelope-encrypted PAN, no
// CVV), a polymorphic temporal holder link (person|company), and two reference catalogs. A bank is not
// a new entity — it is a `company`-domain tenant_organizations row referenced by institution_id. The
// domain owns its Repository interface and imports no framework.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Holder kinds for the polymorphic account holder (an account is held by a person OR a company).
const (
	HolderPerson  = "person"
	HolderCompany = "company"
)

// Holder roles on an account.
const (
	RolePrimary          = "primary"
	RoleJoint            = "joint"
	RoleAuthorizedSigner = "authorized_signer"
)

// Card types.
const (
	CardDebit  = "debit"
	CardCredit = "credit"
)

// ---- sentinel errors (mapped to Conjure Finance:* in transport) ----
var (
	ErrAccountNotFound = errors.New("finance: account not found")
	ErrCardNotFound    = errors.New("finance: card not found")
	ErrHolderNotFound  = errors.New("finance: holder not found")
	ErrConflict        = errors.New("finance: identifier already exists in scope")
	ErrInvalid         = errors.New("finance: invalid request or unknown reference")
)

// ---- catalogs ----

type AccountType struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

type CardNetwork struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

// ---- objects ----

// Account is a bank account. IBAN is decrypted into IBAN only on the single-account read path; list
// projections leave it "". The encrypted columns live in StoredAccount (the repository shape).
type Account struct {
	ID, InstitutionID    string
	IBAN                 string // decrypted plaintext; "" unless a getAccount decrypted it
	Currency             string // ISO 4217; "" = none
	AccountTypeID        string // "" = none
	Status               string
	CreatedAt, UpdatedAt time.Time
}

// StoredAccount is the persistence shape carrying the envelope columns (never leaves the module).
type StoredAccount struct {
	ID, InstitutionID string
	IBANCiphertext    []byte
	IBANWrappedDEK    []byte
	KeyRef            string
	IBANBlindIndex    []byte
	Currency          string
	AccountTypeID     string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Card is a payment card contained by an account. PAN is decrypted only on the single-card read path.
type Card struct {
	ID, AccountID        string
	PAN                  string // decrypted plaintext; "" unless a getCard decrypted it
	BIN, LastFour        string
	NetworkID            string // "" = none
	CardType             string
	ExpiryMonth          *int
	ExpiryYear           *int
	CardholderPersonID   string // "" = none
	Status               string
	CreatedAt, UpdatedAt time.Time
}

// StoredCard is the persistence shape carrying the PAN envelope columns (never leaves the module).
type StoredCard struct {
	ID, AccountID      string
	PANCiphertext      []byte
	PANWrappedDEK      []byte
	KeyRef             string
	PANBlindIndex      []byte
	BIN, LastFour      string
	NetworkID          string
	CardType           string
	ExpiryMonth        *int
	ExpiryYear         *int
	CardholderPersonID string
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ---- reified link ----

// AccountHolder is the ownership edge (link__held_by): a polymorphic, temporal holder of an account.
type AccountHolder struct {
	ID, AccountID        string
	HolderKind, HolderID string
	Role                 string
	EffectiveFrom        time.Time
	EffectiveTo          *time.Time
	CreatedAt, UpdatedAt time.Time
}

// PersonAccount is the read-side view of an account a person holds, enriched with the person's role.
type PersonAccount struct {
	ID, InstitutionID string
	Currency          string
	AccountTypeID     string
	Role              string
	Status            string
	CreatedAt         time.Time
}

// ---- inputs ----

// AccountInput carries the encrypted IBAN + the account attributes for an insert.
type AccountInput struct {
	InstitutionID  string
	IBANCiphertext []byte
	IBANWrappedDEK []byte
	KeyRef         string
	IBANBlindIndex []byte
	Currency       string
	AccountTypeID  string
}

// AccountUpdate carries partial updates; a nil field is left unchanged. IBAN* are set together when
// the plaintext IBAN is re-keyed.
type AccountUpdate struct {
	IBANCiphertext []byte // set (with the others) only when re-keying the IBAN
	IBANWrappedDEK []byte
	KeyRef         string
	IBANBlindIndex []byte
	RekeyIBAN      bool
	Currency       *string
	AccountTypeID  *string
	Status         *string
}

// CardInput carries the encrypted PAN + card attributes for an insert.
type CardInput struct {
	PANCiphertext      []byte
	PANWrappedDEK      []byte
	KeyRef             string
	PANBlindIndex      []byte
	BIN, LastFour      string
	NetworkID          string
	CardType           string
	ExpiryMonth        *int
	ExpiryYear         *int
	CardholderPersonID string
}

// CardUpdate carries partial updates; a nil field is left unchanged.
type CardUpdate struct {
	NetworkID          *string
	CardType           *string
	ExpiryMonth        *int
	ExpiryYear         *int
	CardholderPersonID *string
	Status             *string
}

type HolderInput struct {
	HolderKind, HolderID string
	Role                 string
}

// ---- validators ----

func validHolderKind(k string) bool { return k == HolderPerson || k == HolderCompany }

func validRole(r string) bool {
	return r == RolePrimary || r == RoleJoint || r == RoleAuthorizedSigner
}

func validCardType(t string) bool { return t == CardDebit || t == CardCredit }

func (in HolderInput) Validate() error {
	if !validHolderKind(in.HolderKind) || strings.TrimSpace(in.HolderID) == "" {
		return ErrInvalid
	}
	if in.Role != "" && !validRole(in.Role) {
		return ErrInvalid
	}
	return nil
}

// ValidCardType is exported so the application service can guard the plaintext request before encrypting.
func ValidCardType(t string) bool { return validCardType(t) }
