// Package domain holds the external-organizations module's entities, ports, and invariants
// (docs/modules/external-organizations.md / D-ExternalOrgs, M30) — a registry of external organizations
// a person is tied to but which the deploying org neither owns nor commands (parties, government bodies,
// foreign military, NGOs, lobbying registrants). The domain owns its Repository interface and imports no
// framework. External orgs carry the D-OverlayFoundation (M29) provisional/resolved status + the
// source/confidence/as_of attribution column-set.
package domain

import (
	"errors"
	"strings"
	"time"
)

// Lifecycle status values (D-OverlayFoundation provisional/resolved).
const (
	StatusProvisional = "provisional"
	StatusResolved    = "resolved"
)

// Attribution source values (docs/architecture/conventions.md §Attribution).
const (
	SourceSelfDeclared     = "self_declared"
	SourceOperatorVerified = "operator_verified"
	SourceImported         = "imported"
)

// Attribution confidence values.
const (
	ConfidenceConfirmed = "confirmed"
	ConfidenceProbable  = "probable"
	ConfidencePossible  = "possible"
)

// ---- sentinel errors (mapped to Conjure ExternalOrg:* in transport) ----
var (
	ErrNotFound = errors.New("externalorg: organization not found")
	ErrConflict = errors.New("externalorg: code or wikidata id already exists in scope")
	ErrInvalid  = errors.New("externalorg: invalid request or unknown reference")
)

// ---- catalog ----

type Kind struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

// ---- object ----

// Organization is an external organization at registry grade. KindLabel/CountryLabel are best-effort
// default-locale labels resolved in transport ("" when absent).
type Organization struct {
	ID, KindID, Name, Status string
	Code                     string // "" = none
	CountryID                string // "" = none
	WikidataID               string // "" = none
	Source, Confidence       string
	AsOf                     *time.Time
	CreatedAt, UpdatedAt     time.Time
}

// ---- inputs ----

// OrgInput creates an organization. Empty Status/Source/Confidence fall back to resolved/
// operator_verified/possible.
type OrgInput struct {
	KindID     string
	Name       string
	Code       string // "" = none
	CountryID  string // "" = none
	WikidataID string // "" = none
	Status     string // "" → resolved
	Source     string // "" → operator_verified
	Confidence string // "" → possible
	AsOf       *time.Time
}

// OrgUpdate carries partial updates; a nil field is left unchanged.
type OrgUpdate struct {
	KindID     *string
	Name       *string
	Code       *string
	CountryID  *string
	WikidataID *string
	Status     *string
	Source     *string
	Confidence *string
	AsOf       *time.Time
}

// ---- validators ----

func (in OrgInput) Validate() error {
	if strings.TrimSpace(in.KindID) == "" || strings.TrimSpace(in.Name) == "" {
		return ErrInvalid
	}
	if in.Status != "" && !validStatus(in.Status) {
		return ErrInvalid
	}
	if in.Source != "" && !validSource(in.Source) {
		return ErrInvalid
	}
	if in.Confidence != "" && !validConfidence(in.Confidence) {
		return ErrInvalid
	}
	return nil
}

func validStatus(s string) bool { return s == StatusProvisional || s == StatusResolved }

func validSource(s string) bool {
	return s == SourceSelfDeclared || s == SourceOperatorVerified || s == SourceImported
}

func validConfidence(s string) bool {
	return s == ConfidenceConfirmed || s == ConfidenceProbable || s == ConfidencePossible
}

// ValidStatus / ValidConfidence are exported for the partial-update path validation.
func ValidStatus(s string) bool     { return validStatus(s) }
func ValidSource(s string) bool     { return validSource(s) }
func ValidConfidence(s string) bool { return validConfidence(s) }
