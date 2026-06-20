// Package domain holds the religion module's pure values, inputs, and sentinel errors (D-Religion,
// M22). No I/O, no framework imports. The taxonomy is a recursive tree (Taxon + closure) with a
// catalog-driven level marker (TaxonRank); organization attributes attach to reused tenant Units.
package domain

import (
	"errors"
	"strings"
	"time"
)

// ---- catalogs ----

// TaxonRank is an ordered structural level (religion → branch → … → denomination).
type TaxonRank struct {
	ID        string
	Code      string
	Name      string // default-locale; translated via the i18n store
	Ordinal   int
	Status    string
	SortOrder *int
}

// Classification is a religion-type ("theism") tag (monotheistic/polytheistic/…).
type Classification struct {
	ID          string
	Code        string
	Name        string
	Description string // "" = none
	Status      string
	SortOrder   *int
}

// OrgKind is a descriptive organizational-level label for a religious-body unit.
type OrgKind struct {
	ID         string
	ReligionID string // "" = generic across faiths
	Code       string
	Name       string
	Ordinal    *int
	Status     string
	SortOrder  *int
}

// PolicyKind is the data-driven org-policy vocabulary (e.g. excludes_child_creation).
type PolicyKind struct {
	ID          string
	Code        string
	Name        string
	Description string // "" = none
	Status      string
	SortOrder   *int
}

// ---- taxonomy ----

// Taxon is one node in the recursive faith taxonomy.
type Taxon struct {
	ID          string
	ParentID    string // "" = root religion
	RankID      string
	RankCode    string
	ReligionID  string // denormalized root
	Code        string
	Name        string
	Description string
	WikidataID  string
	SortOrder   *int
	Depth       int // populated by list/search when a parent/root anchor is given
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TaxonInput creates a taxon.
type TaxonInput struct {
	Code        string
	Name        string
	RankID      string
	ParentID    string // "" = root religion
	Description string
	WikidataID  string
	SortOrder   *int
}

// Validate checks a create input.
func (in TaxonInput) Validate() error {
	if strings.TrimSpace(in.Code) == "" || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.RankID) == "" {
		return ErrInvalid
	}
	return nil
}

// TaxonUpdate patches a taxon (nil = unchanged). Parent changes go through reparent.
type TaxonUpdate struct {
	Name        *string
	RankID      *string
	Description *string
	WikidataID  *string
	SortOrder   *int
}

// ClosureReport is the result of a closure rebuild.
type ClosureReport struct {
	Rows    int
	InDrift bool
}

// ---- organization ----

// OrgClassification is a tradition tag on a unit (link__classified_as).
type OrgClassification struct {
	ID         string
	UnitID     string
	TaxonID    string
	TaxonCode  string
	TaxonName  string
	IsPrimary  bool
	Source     string
	Confidence string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OrgProfile is the 1:1 faith attributes of a religious-body unit.
type OrgProfile struct {
	UnitID          string
	OrgKindID       string // "" = unset
	ShortCode       string // "" = unset
	Classifications []OrgClassification
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// OrgPolicy is a data-driven eligibility/exclusion rule on a unit.
type OrgPolicy struct {
	ID                string
	UnitID            string
	PolicyKindID      string
	PolicyKindCode    string
	Reason            string
	DecidedByPersonID string
	DecidedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// EffectiveType is a unit's resolved religion-type, with the source that supplied it.
type EffectiveType struct {
	UnitID          string
	Classifications []Classification
	Source          string // "unit" | "taxon:<code>" | "none"
}

// PolicyExcludesChildCreation is the policy-kind code that blocks creating child organizations.
const PolicyExcludesChildCreation = "excludes_child_creation"

// ---- sentinels ----

var (
	ErrTaxonNotFound          = errors.New("religion: taxon not found")
	ErrClassificationNotFound = errors.New("religion: classification not found")
	ErrOrgKindNotFound        = errors.New("religion: org kind not found")
	ErrPolicyKindNotFound     = errors.New("religion: policy kind not found")
	ErrProfileNotFound        = errors.New("religion: org profile not found")
	ErrPolicyNotFound         = errors.New("religion: org policy not found")
	ErrConflict               = errors.New("religion: code already exists in scope")
	ErrInvalid                = errors.New("religion: invalid request or unknown reference")
	ErrTaxonCycle             = errors.New("religion: reparent would create a cycle")
	ErrInUse                  = errors.New("religion: entity still referenced")
	ErrChildCreationExcluded  = errors.New("religion: parent excludes child creation")
)
