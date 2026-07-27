// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Clergy domain (D-ClergyCredential, M23): per-tradition grade catalogs + the reified, public
// clergy-credential Link (Person → ClergyGrade within an organization unit). A credential is never an
// authorization input (parallel to D-Rank); it is indelible where sacramental (status flip, not delete).
package domain

import (
	"errors"
	"strings"
	"time"
)

// GradeCategory groups clergy grades within a tradition (e.g. major/minor orders).
type GradeCategory struct {
	ID               string
	TraditionTaxonID string // "" = generic
	Code             string
	Name             string
	Ordinal          *int
	Status           string
	SortOrder        *int
}

// ClergyGrade is an ordered, per-tradition grade. Ordinal orders only within a tradition.
type ClergyGrade struct {
	ID               string
	TraditionTaxonID string // "" = generic
	GradeCategoryID  string
	Code             string
	Name             string
	Ordinal          int
	Status           string
	SortOrder        *int
}

// OfficeType is a clergy office-type label (offices are filled as membership Positions).
type OfficeType struct {
	ID               string
	TraditionTaxonID string // "" = generic
	Code             string
	Name             string
	Status           string
	SortOrder        *int
}

// ClergyCredential is the reified person↔religion ordination/standing link (link__clergy_credential).
type ClergyCredential struct {
	ID                  string
	PersonID            string
	ClergyGradeID       string
	GradeCode           string
	GradeName           string // default-locale; translated via the i18n store
	OrgUnitID           string
	GrantedOn           *time.Time // a day, not an instant
	ConferredByPersonID string     // "" = unset
	Status              string     // active | suspended | revoked
	EffectiveFrom       time.Time
	EffectiveTo         *time.Time
	Source              string
	Confidence          string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ClergyCredentialInput creates a credential.
type ClergyCredentialInput struct {
	PersonID            string
	ClergyGradeID       string
	OrgUnitID           string
	GrantedOn           *time.Time
	ConferredByPersonID string
	Source              string
	Confidence          string
}

// Validate checks a create input.
func (in ClergyCredentialInput) Validate() error {
	if strings.TrimSpace(in.PersonID) == "" || strings.TrimSpace(in.ClergyGradeID) == "" || strings.TrimSpace(in.OrgUnitID) == "" {
		return ErrInvalid
	}
	return nil
}

// ClergyCredentialUpdate patches a credential (status flip / effective-dating). Never a hard delete.
type ClergyCredentialUpdate struct {
	Status      *string
	EffectiveTo *time.Time
}

// CredentialStatuses are the fixed lifecycle statuses of a clergy credential.
var credentialStatuses = map[string]struct{}{"active": {}, "suspended": {}, "revoked": {}}

// ValidCredentialStatus reports whether s is a known credential status.
func ValidCredentialStatus(s string) bool { _, ok := credentialStatuses[s]; return ok }

var (
	ErrGradeNotFound      = errors.New("religion: clergy grade not found")
	ErrCredentialNotFound = errors.New("religion: clergy credential not found")
)
