// Criminal / arrest / court records (D-LegalRecords, M38) — the last in-scope person-intelligence
// vertical. A LegalRecord is a category-level (NO full charge sheet) record whose `Detail` is
// envelope-encrypted (sealed on write, decrypted on read, crypto-erased on purge — exactly like the
// M36 health detail) with a required legal_basis (Art. 10) and a need-to-know read gate. Two things
// distinguish it from health: `Disposition` is mandatory (arrest ≠ guilt), and sealed/expunged
// records are SUPPRESSED — retained but withheld from the normal read gate. Never inferred.
package domain

import (
	"errors"
	"time"
)

var (
	// D-LegalRecords (M38)
	ErrLegalRecordNotFound = errors.New("legal record not found")
)

var (
	validLegalKind        = map[string]bool{"criminal_conviction": true, "arrest": true, "court_judgment": true}
	validLegalDisposition = map[string]bool{
		"convicted": true, "acquitted": true, "dismissed": true, "pending": true,
		"sealed": true, "expunged": true, "no_charges": true,
	}
	validSuppressedReason = map[string]bool{"sealed": true, "expunged": true}
)

// LegalRecord is the decrypted view of a person's category-level criminal/arrest/court record (object
// legal_record). Detail is the decrypted category-level offence ("" for a crypto-erased tombstone).
// pii:special.
type LegalRecord struct {
	ID               string
	PersonID         string
	Kind             string // criminal_conviction|arrest|court_judgment
	Disposition      string // mandatory outcome (arrest ≠ guilt)
	Detail           string // decrypted category-level offence (NO full charge sheet); "" when crypto-erased
	Jurisdiction     string // ISO-3166-1 country code or ""
	OccurredAt       string // ISO date or ""
	DispositionDate  string // ISO date or ""
	IsSuppressed     bool   // sealed/expunged → withheld from the normal read gate
	SuppressedReason string // sealed|expunged (present iff IsSuppressed)
	LegalBasis       string
	Source           string
	Confidence       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (r LegalRecord) Validate() error {
	if r.PersonID == "" || r.LegalBasis == "" {
		return ErrInvalid
	}
	if !validLegalKind[r.Kind] || !validLegalDisposition[r.Disposition] {
		return ErrInvalid
	}
	// suppressed_reason is present iff the record is suppressed (mirrors the DB CHECK).
	if r.IsSuppressed {
		if !validSuppressedReason[r.SuppressedReason] {
			return ErrInvalid
		}
	} else if r.SuppressedReason != "" {
		return ErrInvalid
	}
	if r.Source != "" && !validTieSource[r.Source] {
		return ErrInvalid
	}
	if r.Confidence != "" && !validTieConfide[r.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(r.OccurredAt) || !isoOrEmpty(r.DispositionDate) {
		return ErrInvalid
	}
	return nil
}

// StoredLegalRecord is the at-rest row: the category-level detail is envelope-encrypted (all four
// envelope columns NULL after a crypto-erase tombstone). JurisdictionCountryID is the geo_countries
// RID (write side); Jurisdiction is the resolved ISO code (read side, from the list join).
type StoredLegalRecord struct {
	ID                    string
	PersonID              string
	Kind                  string
	Disposition           string
	DetailCiphertext      []byte
	DetailWrappedDEK      []byte
	DetailKeyRef          string
	DetailBlindIndex      []byte
	JurisdictionCountryID string
	Jurisdiction          string
	OccurredAt            string
	DispositionDate       string
	IsSuppressed          bool
	SuppressedReason      string
	LegalBasis            string
	Source                string
	Confidence            string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
