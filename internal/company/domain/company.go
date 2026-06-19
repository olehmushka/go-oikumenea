// Package domain holds the company module's entities, ports, and invariants (docs/modules/company.md /
// D-Companies) — a generic legal-entity registry: companies, registrations, industry classification,
// locations, positions/appointments (mirrors membership), and the ownership/affiliation graph
// (foundings, shareholdings, beneficiaries, successions, branches). The domain owns its Repository
// interface and imports no framework.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// ISODate is the wire/storage format for the date-only fields (founded/dissolved, effective dates).
const ISODate = "2006-01-02"

// Holder kinds for the polymorphic ownership links (founder / shareholder is a person OR a company).
const (
	HolderPerson  = "person"
	HolderCompany = "company"
)

// ---- sentinel errors (mapped to Conjure Company:* in transport) ----
var (
	ErrCompanyNotFound       = errors.New("company: company not found")
	ErrPositionNotFound      = errors.New("company: position not found")
	ErrAppointmentNotFound   = errors.New("company: appointment not found")
	ErrLinkNotFound          = errors.New("company: link not found")
	ErrConflict              = errors.New("company: code or identifier already exists in scope")
	ErrInvalid               = errors.New("company: invalid request or unknown reference")
	ErrPositionAlreadyFilled = errors.New("company: position already filled")
	ErrInUse                 = errors.New("company: entity still referenced")
	ErrLifecycle             = errors.New("company: invalid lifecycle transition")
)

// ---- catalogs ----

type LegalForm struct {
	ID, Code, Name, Status string
	Abbreviation           string // "" = none
	CountryID              string // "" = generic
	SortOrder              *int
}

type RegistrationScheme struct {
	ID, Code, Name, Status string
	ValidatorPattern       string // "" = no validation
	IsGlobal               bool
	SortOrder              *int
}

type IndustryClass struct {
	ID, Code, Name, System, Status string
	SortOrder                      *int
}

// ---- objects ----

type Company struct {
	ID, Code, LegalName       string
	ShortName                 string // "" = none
	LegalFormID               string
	OwnershipCategory         string
	CountryID                 string // "" = none
	FoundedOn, DissolvedOn    string // ISO date; "" = none
	State                     string
	CreatedAt, UpdatedAt      time.Time
}

type Registration struct {
	ID, CompanyID, SchemeID, Identifier string
	Validated                           bool
	CreatedAt, UpdatedAt                time.Time
}

type IndustryAssignment struct {
	ID, CompanyID, IndustryClassID string
	IsPrimary                      bool
	CreatedAt, UpdatedAt           time.Time
}

type CompanyLocation struct {
	ID, CompanyID, LocationID, Role string
	CreatedAt, UpdatedAt            time.Time
}

type Position struct {
	ID, CompanyID, Code, Title, Status string
	SortOrder                          *int
	Holder                             *Appointment
	CreatedAt, UpdatedAt               time.Time
}

type Appointment struct {
	ID, PersonID, PositionID, Status string
	EffectiveFrom                    time.Time
	EffectiveTo                      *time.Time
	CreatedAt, UpdatedAt             time.Time
}

// PersonAppointment is an Appointment enriched with the position title + owning company, for the
// read-only person view.
type PersonAppointment struct {
	ID, PersonID, PositionID, Status string
	PositionTitle                    string
	CompanyID, CompanyName           string
	EffectiveFrom                    time.Time
	EffectiveTo                      *time.Time
	CreatedAt, UpdatedAt             time.Time
}

// ---- ownership / affiliation links ----

type Founding struct {
	ID, CompanyID            string
	CompanyLabel             string // best-effort
	HolderKind, HolderID     string
	HolderLabel              string // best-effort (company legal name)
	FoundedOn                string // ISO date; "" = none
	CreatedAt, UpdatedAt     time.Time
}

type Shareholding struct {
	ID, CompanyID            string
	CompanyLabel             string
	HolderKind, HolderID     string
	HolderLabel              string
	StakePct                 *float64
	EffectiveFrom, EffectiveTo string // ISO date; "" = none
	CreatedAt, UpdatedAt     time.Time
}

type Beneficiary struct {
	ID, CompanyID        string
	CompanyLabel         string
	PersonID             string
	UltimatePct          *float64
	Declared             bool
	CreatedAt, UpdatedAt time.Time
}

type Succession struct {
	ID, PredecessorID, SuccessorID string
	PredecessorLabel, SuccessorLabel string
	Kind                           string
	EffectiveOn                    string // ISO date; "" = none
	CreatedAt, UpdatedAt           time.Time
}

type Branch struct {
	ID, BranchID, ParentID     string
	BranchLabel, ParentLabel   string
	CreatedAt, UpdatedAt       time.Time
}

// OwnershipGraph is the one-hop ownership/affiliation neighbourhood of a company.
type OwnershipGraph struct {
	CompanyID     string
	Shareholders  []Shareholding // stakes held IN this company
	Holdings      []Shareholding // stakes this company holds in others (subsidiaries)
	Beneficiaries []Beneficiary
	Founders      []Founding
	Successions   []Succession
	Branches      []Branch
}

// PersonAffiliations is a person's company links (employment, founding, ownership, beneficiary-of).
type PersonAffiliations struct {
	Appointments  []PersonAppointment
	Foundings     []Founding
	Shareholdings []Shareholding
	BeneficiaryOf []Beneficiary
}

// ---- inputs ----

type CompanyInput struct {
	Code, LegalName, LegalFormID  string
	ShortName, OwnershipCategory  *string
	CountryID, FoundedOn          *string
}

type CompanyUpdate struct {
	LegalName, ShortName, LegalFormID, OwnershipCategory *string
	CountryID, FoundedOn, DissolvedOn, State             *string
}

type RegistrationInput struct {
	SchemeID, Identifier string
}

type IndustryInput struct {
	IndustryClassID string
	IsPrimary       bool
}

type CompanyLocationInput struct {
	LocationID string
	Role       *string
}

type PositionInput struct {
	Code, Title string
	SortOrder   *int
}

type PositionUpdate struct {
	Title     *string
	SortOrder *int
}

type FoundingInput struct {
	HolderKind, HolderID string
	FoundedOn            *string
}

type ShareholdingInput struct {
	HolderKind, HolderID       string
	StakePct                   *float64
	EffectiveFrom, EffectiveTo *string
}

type BeneficiaryInput struct {
	PersonID    string
	UltimatePct *float64
	Declared    *bool
}

type SuccessionInput struct {
	SuccessorID string
	Kind        *string
	EffectiveOn *string
}

// validCode is a stable, non-empty, whitespace-free identifier (D-Code).
func validCode(code string) bool {
	return code != "" && len(code) <= 128 && !strings.ContainsAny(code, " \t\n")
}

func validHolderKind(k string) bool { return k == HolderPerson || k == HolderCompany }

func (in CompanyInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.LegalName) == "" || in.LegalFormID == "" {
		return ErrInvalid
	}
	return nil
}

func (in RegistrationInput) Validate() error {
	if in.SchemeID == "" || strings.TrimSpace(in.Identifier) == "" {
		return ErrInvalid
	}
	return nil
}

func (in IndustryInput) Validate() error {
	if in.IndustryClassID == "" {
		return ErrInvalid
	}
	return nil
}

func (in CompanyLocationInput) Validate() error {
	if in.LocationID == "" {
		return ErrInvalid
	}
	return nil
}

func (in PositionInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.Title) == "" {
		return ErrInvalid
	}
	return nil
}

func (in FoundingInput) Validate() error {
	if !validHolderKind(in.HolderKind) || in.HolderID == "" {
		return ErrInvalid
	}
	return nil
}

func (in ShareholdingInput) Validate() error {
	if !validHolderKind(in.HolderKind) || in.HolderID == "" {
		return ErrInvalid
	}
	if in.StakePct != nil && (*in.StakePct < 0 || *in.StakePct > 100) {
		return ErrInvalid
	}
	return nil
}

func (in BeneficiaryInput) Validate() error {
	if in.PersonID == "" {
		return ErrInvalid
	}
	if in.UltimatePct != nil && (*in.UltimatePct < 0 || *in.UltimatePct > 100) {
		return ErrInvalid
	}
	return nil
}

func (in SuccessionInput) Validate() error {
	if in.SuccessorID == "" {
		return ErrInvalid
	}
	return nil
}

// ValidatesIdentifier reports whether an identifier matches the scheme's validator pattern (empty
// pattern accepts anything). Used to set Registration.validated.
func ValidatesIdentifier(pattern, identifier string) bool {
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(identifier)
}
