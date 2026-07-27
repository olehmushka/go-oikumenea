// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Institutional & political ties (D-InstitutionalTies, M33): per-type person↔organization affiliation
// edges — party memberships (pii:special, envelope-encrypted party), government positions (pii:basic,
// PEP-triggering), lobbying relationships (pii:basic), and external references (pii:basic objects). Each
// edge carries the D-OverlayFoundation attribution pair (source/confidence). Emergency contacts reuse the
// M14 person_relation_types catalog (no type here). Inferred political leaning is a SEPARATE M35 overlay,
// never modelled here.
package domain

import (
	"errors"
	"time"
)

var (
	// D-InstitutionalTies (M33)
	ErrPartyMembershipNotFound    = errors.New("party membership not found")
	ErrGovernmentPositionNotFound = errors.New("government position not found")
	ErrLobbyingNotFound           = errors.New("lobbying relationship not found")
	ErrExternalReferenceNotFound  = errors.New("external reference not found")
)

var (
	validPartyRole  = map[string]bool{"member": true, "official": true, "candidate": true, "donor": true, "supporter": true, "other": true}
	validGovLevel   = map[string]bool{"international": true, "national": true, "regional": true, "local": true}
	validRefKind    = map[string]bool{"wikipedia": true, "news": true, "registry": true, "social": true, "court": true, "academic": true, "other": true}
	validTieSource  = map[string]bool{"self_declared": true, "operator_verified": true, "imported": true}
	validTieConfide = map[string]bool{"confirmed": true, "probable": true, "possible": true}
)

// isoOrEmpty reports whether s is empty or a valid ISO-8601 calendar date.
func isoOrEmpty(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse(ISODate, s)
	return err == nil
}

// PartyMembership is the decrypted view of a person's party affiliation (link__party_membership). Party
// holds the decrypted party identity ("" for a crypto-erased tombstone). pii:special.
type PartyMembership struct {
	ID         string
	PersonID   string
	Party      string // decrypted party name / external-org RID
	Role       string // member|official|candidate|donor|supporter|other
	ValidFrom  string // ISO date or ""
	ValidTo    string // ISO date or "" (current)
	LegalBasis string
	Status     string // active|retired
	Source     string
	Confidence string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (p PartyMembership) Validate() error {
	if p.PersonID == "" || p.LegalBasis == "" {
		return ErrInvalid
	}
	if p.Role == "" {
		p.Role = "member"
	}
	if !validPartyRole[p.Role] {
		return ErrInvalid
	}
	if p.Status != "" && p.Status != "active" && p.Status != "retired" {
		return ErrInvalid
	}
	if p.Source != "" && !validTieSource[p.Source] {
		return ErrInvalid
	}
	if p.Confidence != "" && !validTieConfide[p.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(p.ValidFrom) || !isoOrEmpty(p.ValidTo) {
		return ErrInvalid
	}
	return nil
}

// StoredPartyMembership is the at-rest row: the party identity is envelope-encrypted (all four envelope
// columns NULL after a crypto-erase tombstone).
type StoredPartyMembership struct {
	ID              string
	PersonID        string
	PartyCiphertext []byte
	PartyWrappedDEK []byte
	PartyKeyRef     string
	PartyBlindIndex []byte
	Role            string
	ValidFrom       string
	ValidTo         string
	LegalBasis      string
	Status          string
	Source          string
	Confidence      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GovernmentPosition is a public office a person holds/held (link__government_position). pii:basic;
// PEPTrigger persists after the position ends and feeds the M34 watchlist PEP check.
type GovernmentPosition struct {
	ID         string
	PersonID   string
	Title      string
	Body       string
	OrgID      string // optional resolved body RID (polymorphic; "" when unresolved)
	CountryID  string // optional geo_countries RID
	Level      string // international|national|regional|local
	RoleType   string
	ValidFrom  string
	ValidTo    string
	PEPTrigger bool
	Source     string
	Confidence string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (g GovernmentPosition) Validate() error {
	if g.PersonID == "" || g.Title == "" || g.Body == "" {
		return ErrInvalid
	}
	if g.Level != "" && !validGovLevel[g.Level] {
		return ErrInvalid
	}
	if g.Source != "" && !validTieSource[g.Source] {
		return ErrInvalid
	}
	if g.Confidence != "" && !validTieConfide[g.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(g.ValidFrom) || !isoOrEmpty(g.ValidTo) {
		return ErrInvalid
	}
	return nil
}

// LobbyingRelationship is a public lobbying filing (link__lobbying_rel). pii:basic.
type LobbyingRelationship struct {
	ID              string
	PersonID        string
	Registrant      string
	Client          string
	LegislativeBody string
	Issues          []string
	FilingID        string
	SourceURL       string
	ValidFrom       string
	ValidTo         string
	Source          string
	Confidence      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (l LobbyingRelationship) Validate() error {
	if l.PersonID == "" || l.Registrant == "" {
		return ErrInvalid
	}
	if l.Source != "" && !validTieSource[l.Source] {
		return ErrInvalid
	}
	if l.Confidence != "" && !validTieConfide[l.Confidence] {
		return ErrInvalid
	}
	if !isoOrEmpty(l.ValidFrom) || !isoOrEmpty(l.ValidTo) {
		return ErrInvalid
	}
	return nil
}

// ExternalReference is an off-platform pointer about a person (object external_reference). pii:basic; a
// hermenea import target (idempotent by URL).
type ExternalReference struct {
	ID          string
	PersonID    string
	Kind        string // wikipedia|news|registry|social|court|academic|other
	URL         string
	ExternalID  string
	Categories  []string
	LastChecked *time.Time
	Disputed    bool
	Source      string
	Confidence  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r ExternalReference) Validate() error {
	if r.PersonID == "" || r.URL == "" {
		return ErrInvalid
	}
	if r.Kind != "" && !validRefKind[r.Kind] {
		return ErrInvalid
	}
	if r.Source != "" && !validTieSource[r.Source] {
		return ErrInvalid
	}
	if r.Confidence != "" && !validTieConfide[r.Confidence] {
		return ErrInvalid
	}
	return nil
}
