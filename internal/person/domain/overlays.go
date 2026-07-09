// Financial / behavioural / psychological overlays (D-PersonOverlays, M35): three provenance-tagged
// person overlays. CryptoWallet + Personality are plaintext pii:sensitive (hard-erased on purge);
// PoliticalLeaning is an INFERRED pii:special overlay whose spectrum is envelope-encrypted (sealed on
// write, decrypted on read, crypto-erased on purge — exactly like the M31 ethnicity / M33 party) and is
// NEVER merged with the declared M33 party membership. Personality is declared/assessment-only (no
// text-inference) and political leaning is inference-only — the declared-vs-inferred split is enforced by
// keeping them in distinct tables.
package domain

import (
	"errors"
	"time"
)

var (
	// D-PersonOverlays (M35)
	ErrCryptoWalletNotFound     = errors.New("crypto wallet not found")
	ErrPersonalityNotFound      = errors.New("personality profile not found")
	ErrPoliticalLeaningNotFound = errors.New("political leaning not found")
)

var (
	validChain          = map[string]bool{"bitcoin": true, "ethereum": true, "solana": true, "tron": true, "bnb": true, "polygon": true, "monero": true, "other": true}
	validAttribMethod   = map[string]bool{"exchange_kyc": true, "blockchain_analysis": true, "self_declared": true, "leak": true, "public_post": true, "other": true}
	validFramework      = map[string]bool{"mbti": true, "big_five": true, "disc": true, "enneagram": true, "other": true}
	validPersonalityWay = map[string]bool{"self_declared_survey": true, "hr_assessment": true}
)

// CryptoWallet is a crypto-wallet attribution for a person (object crypto_wallet). The address is public
// on-chain data; attributing it to a person is pii:sensitive.
type CryptoWallet struct {
	ID                string
	PersonID          string
	Address           string
	Chain             string // bitcoin|ethereum|solana|tron|bnb|polygon|monero|other
	AttributionMethod string // exchange_kyc|blockchain_analysis|self_declared|leak|public_post|other
	BalanceUSDApprox  *float64
	FirstSeen         string // ISO date or ""
	LastSeen          string // ISO date or ""
	Source            string
	Confidence        string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (w CryptoWallet) Validate() error {
	if w.PersonID == "" || w.Address == "" {
		return ErrInvalid
	}
	if w.Chain != "" && !validChain[w.Chain] {
		return ErrInvalid
	}
	if w.AttributionMethod != "" && !validAttribMethod[w.AttributionMethod] {
		return ErrInvalid
	}
	if w.Source != "" && !validTieSource[w.Source] {
		return ErrInvalid
	}
	if w.Confidence != "" && !validTieConfide[w.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(w.FirstSeen) || !isoOrEmpty(w.LastSeen) {
		return ErrInvalid
	}
	return nil
}

// Personality is a declared or formally-assessed personality profile (object personality). Method is
// declared-survey or HR-assessment only — never inferred from text. pii:sensitive.
type Personality struct {
	ID         string
	PersonID   string
	Framework  string // mbti|big_five|disc|enneagram|other
	Result     string
	Instrument string
	Method     string // self_declared_survey|hr_assessment
	AssessedAt string // ISO date or ""
	Source     string
	Confidence string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (p Personality) Validate() error {
	if p.PersonID == "" || p.Result == "" {
		return ErrInvalid
	}
	if p.Framework != "" && !validFramework[p.Framework] {
		return ErrInvalid
	}
	if p.Method != "" && !validPersonalityWay[p.Method] {
		return ErrInvalid
	}
	if p.Source != "" && !validTieSource[p.Source] {
		return ErrInvalid
	}
	if p.Confidence != "" && !validTieConfide[p.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(p.AssessedAt) {
		return ErrInvalid
	}
	return nil
}

// PoliticalLeaning is the decrypted view of a person's INFERRED political leaning (object
// political_leaning). Spectrum is the decrypted [-1,1] position (0 for a crypto-erased tombstone).
// pii:special; never merged with the declared M33 party membership.
type PoliticalLeaning struct {
	ID               string
	PersonID         string
	Spectrum         float64 // inferred left/right position, [-1,1]
	InferenceSources []string
	AssessedAt       string // ISO date or ""
	LegalBasis       string
	Confidence       string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (l PoliticalLeaning) Validate() error {
	if l.PersonID == "" || l.LegalBasis == "" {
		return ErrInvalid
	}
	if l.Spectrum < -1 || l.Spectrum > 1 {
		return ErrInvalid
	}
	if l.Confidence != "" && !validTieConfide[l.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(l.AssessedAt) {
		return ErrInvalid
	}
	return nil
}

// StoredPoliticalLeaning is the at-rest row: the spectrum is envelope-encrypted (all four envelope
// columns NULL after a crypto-erase tombstone).
type StoredPoliticalLeaning struct {
	ID                string
	PersonID          string
	LeaningCiphertext []byte
	LeaningWrappedDEK []byte
	LeaningKeyRef     string
	LeaningBlindIndex []byte
	InferenceSources  []string
	AssessedAt        string
	LegalBasis        string
	Confidence        string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
