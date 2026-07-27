// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Watchlists & regulatory exposure (D-Watchlists, M34): the persisted residue of a live screening check
// (person_watchlist_matches — match METADATA only, never the lists) plus the durable regulatory-sanction
// overlay (person_regulatory_sanctions). Screening never stores the lists statically — the live check
// runs OUT to the hermenea companion via the WatchlistLookup seam; PEP is derived locally from the M33
// government positions. Both surfaces are pii:sensitive.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	// D-Watchlists (M34)
	ErrWatchlistUnavailable       = errors.New("watchlist screening unavailable")
	ErrRegulatorySanctionNotFound = errors.New("regulatory sanction not found")
)

var (
	validSanctionAction = map[string]bool{"fine": true, "ban": true, "license_revocation": true, "warning": true, "settlement": true, "debarment": true, "other": true}
	validSanctionStatus = map[string]bool{"active": true, "appealed": true, "overturned": true, "expired": true, "settled": true}
)

// WatchlistMatch is the persisted result of a live screening check (object watchlist_match). One active
// row per person; CheckWatchlists refreshes it. pii:sensitive.
type WatchlistMatch struct {
	ID           string
	PersonID     string
	OnList       bool
	Lists        []string
	Program      string
	MatchScore   *float64
	PEP          bool
	LastChecked  time.Time
	NextCheckDue *time.Time
	Source       string
	Confidence   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RegulatorySanction is a structured regulatory/enforcement action against a person (object
// regulatory_sanction). Durable overlay data; a hermenea import target (idempotent by external_id).
// pii:sensitive.
type RegulatorySanction struct {
	ID           string
	PersonID     string
	Regulator    string
	ActionType   string // fine|ban|license_revocation|warning|settlement|debarment|other
	Amount       *float64
	Currency     string
	Status       string // active|appealed|overturned|expired|settled
	SanctionDate string // ISO date or ""
	SourceURL    string
	ExternalID   string
	LegalBasis   string
	Source       string
	Confidence   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s RegulatorySanction) Validate() error {
	if s.PersonID == "" || s.Regulator == "" {
		return ErrInvalid
	}
	if s.ActionType != "" && !validSanctionAction[s.ActionType] {
		return ErrInvalid
	}
	if s.Status != "" && !validSanctionStatus[s.Status] {
		return ErrInvalid
	}
	if s.Amount != nil && s.Currency == "" {
		return ErrInvalid
	}
	if s.Source != "" && !validTieSource[s.Source] {
		return ErrInvalid
	}
	if s.Confidence != "" && !validTieConfide[s.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(s.SanctionDate) {
		return ErrInvalid
	}
	return nil
}

// WatchlistQuery is the person-identity payload sent to the screening seam.
type WatchlistQuery struct {
	SubjectKey  string // the person RID (cache key; not sent upstream)
	FullName    string
	Birthdate   string
	Nationality string
}

// WatchlistScreenResult is the match metadata the screening seam returns (never the lists).
type WatchlistScreenResult struct {
	OnList       bool
	Lists        []string
	Program      string
	MatchScore   *float64
	NextCheckDue *time.Time
}

// WatchlistLookup screens a person identity against external watchlists via the hermenea companion
// (D-Watchlists, M34). Late-bound in main.go (mirrors LocationLookup) so the person domain imports no
// hermenea client and the outbound call stays an injected dependency; the PDP core makes no egress call.
type WatchlistLookup interface {
	Screen(ctx context.Context, q WatchlistQuery) (WatchlistScreenResult, error)
}

// PEPStatusReader is the cross-concern seam personsensitive's watchlist screening uses to snapshot a
// person's politically-exposed flag from the personprofile government-position ties (D-InstitutionalTies,
// M33). After the R-09 split the government positions are owned by personprofile, so CheckWatchlists reads
// the PEP flag through this seam (late-bound to the personprofile service in main.go) rather than its own
// repository — keeping personsensitive off personprofile's tables.
type PEPStatusReader interface {
	IsPoliticallyExposed(ctx context.Context, personID string) (bool, error)
}
