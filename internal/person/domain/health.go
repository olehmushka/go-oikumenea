// Health & vulnerability records (D-HealthVulnerability, M36) — the strictest gate. HealthRecord is a
// category-level (NO diagnosis) record whose `Detail` is envelope-encrypted (sealed on write, decrypted on
// read, crypto-erased on purge — exactly like the M31 ethnicity / M33 party / M35 political leaning) with a
// required legal_basis (Art. 9) and a need-to-know read gate. Insurance is a plaintext pii:sensitive
// coverage record, gated on person.read and hard-erased on purge. Health is NEVER inferred.
package domain

import (
	"errors"
	"time"
)

var (
	// D-HealthVulnerability (M36)
	ErrHealthRecordNotFound = errors.New("health record not found")
	ErrInsuranceNotFound    = errors.New("insurance record not found")
)

var (
	validHealthKind    = map[string]bool{"hospitalization": true, "mental_health": true, "disability": true}
	validInsuranceType = map[string]bool{"health": true, "life": true, "disability": true, "ltc": true}
)

// HealthRecord is the decrypted view of a person's category-level health/vulnerability record (object
// health_record). Detail is the decrypted category-level note ("" for a crypto-erased tombstone). pii:special.
type HealthRecord struct {
	ID             string
	PersonID       string
	Kind           string // hospitalization|mental_health|disability
	Detail         string // decrypted category-level note (NO diagnosis); "" when crypto-erased
	IsPublicRecord bool
	AssessedAt     string // ISO date or ""
	LegalBasis     string
	Source         string
	Confidence     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (h HealthRecord) Validate() error {
	if h.PersonID == "" || h.LegalBasis == "" {
		return ErrInvalid
	}
	if !validHealthKind[h.Kind] {
		return ErrInvalid
	}
	if h.Source != "" && !validTieSource[h.Source] {
		return ErrInvalid
	}
	if h.Confidence != "" && !validTieConfide[h.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(h.AssessedAt) {
		return ErrInvalid
	}
	return nil
}

// StoredHealthRecord is the at-rest row: the category-level detail is envelope-encrypted (all four
// envelope columns NULL after a crypto-erase tombstone).
type StoredHealthRecord struct {
	ID                string
	PersonID          string
	Kind              string
	DetailCiphertext  []byte
	DetailWrappedDEK  []byte
	DetailKeyRef      string
	DetailBlindIndex  []byte
	IsPublicRecord    bool
	AssessedAt        string
	LegalBasis        string
	Source            string
	Confidence        string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Insurance is a person's insurance coverage (object insurance). pii:sensitive; gated on person.read.
type Insurance struct {
	ID                string
	PersonID          string
	Type              string // health|life|disability|ltc
	Provider          string
	PolicyReference   string
	EmployerSponsored bool
	ValidFrom         string // ISO date or ""
	ValidTo           string // ISO date or ""
	Source            string
	Confidence        string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (i Insurance) Validate() error {
	if i.PersonID == "" {
		return ErrInvalid
	}
	if !validInsuranceType[i.Type] {
		return ErrInvalid
	}
	if i.Source != "" && !validTieSource[i.Source] {
		return ErrInvalid
	}
	if i.Confidence != "" && !validTieConfide[i.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(i.ValidFrom) || !isoOrEmpty(i.ValidTo) {
		return ErrInvalid
	}
	return nil
}
