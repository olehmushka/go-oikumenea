package domain

import (
	"errors"
	"strings"
	"time"
)

// Reference-layer sentinels (mapped to the educationref Conjure errors in transport). The shared
// ErrConflict / ErrInvalid / ErrInUse from education.go are reused.
var (
	ErrRefNotFound = errors.New("education: reference entity not found")
	ErrPrereqCycle = errors.New("education: prerequisite would create a cycle")
)

// ---- reference objects ----

type Program struct {
	ID, InstitutionID, OwningUnitID, DegreeLevelID, Code, Name, Mode, DurationYears, State string
	CreditHoursTotal                                                                       *int
	CreatedAt, UpdatedAt                                                                   time.Time
}

type Course struct {
	ID, InstitutionID, OwningUnitID, Code, Title, Description, DeliveryMode, Status string
	CreditHours, Level                                                              *int
	CreatedAt, UpdatedAt                                                            time.Time
}

type CurriculumVersion struct {
	ID, ProgramID, VersionCode, EffectiveFrom, EffectiveTo, Status string
	CreatedAt, UpdatedAt                                           time.Time
}

type CurriculumItem struct {
	ID, VersionID, CourseID                     string
	IsRequired                                  bool
	YearOfStudy, CreditAllocation, SemesterSlot *int
	CreatedAt, UpdatedAt                        time.Time
}

type CoursePrerequisite struct {
	ID, CourseID, RequiredCourseID, Kind, MinGrade string
	CreatedAt, UpdatedAt                           time.Time
}

type ResearchCentre struct {
	ID, InstitutionID, Code, Name, Kind, FundingSource, FoundedOn, DissolvedOn, Status string
	CreatedAt, UpdatedAt                                                               time.Time
}

type ResearchGroup struct {
	ID, InstitutionID, CentreID, UnitID, Code, Name, FocusArea, Status string
	CreatedAt, UpdatedAt                                               time.Time
}

type Grant struct {
	ID, InstitutionID, Code, Title, Funder, FunderRef, Amount, Currency, StartOn, EndOn, Status string
	CreatedAt, UpdatedAt                                                                        time.Time
}

type Publication struct {
	ID, InstitutionID, Code, Title, Kind, Doi, Venue, PublishedOn string
	OpenAccess                                                    bool
	CreatedAt, UpdatedAt                                          time.Time
}

type GovernanceBody struct {
	ID, InstitutionID, Code, Name, Kind, Mandate, Status string
	CreatedAt, UpdatedAt                                 time.Time
}

type Policy struct {
	ID, InstitutionID, GovernanceBodyID, SupersedesID, Code, Title, Kind, EffectiveOn, ExpiryOn, DocumentURL, Status string
	CreatedAt, UpdatedAt                                                                                             time.Time
}

type Qualification struct {
	ID, InstitutionID, ProgramID, DegreeLevelID, Code, Name, FrameworkCode, FrameworkLevel, AwardingBody, Status string
	CreatedAt, UpdatedAt                                                                                         time.Time
}

type Scholarship struct {
	ID, InstitutionID, Code, Name, Kind, Amount, Currency, Frequency, Conditions, Status string
	Renewable                                                                            bool
	CreatedAt, UpdatedAt                                                                 time.Time
}

type AccreditationEvent struct {
	ID, EntityKind, InstitutionID, ProgramID, Body, BodyCountryID, Outcome, ReviewOn, EffectiveFrom, EffectiveTo, Notes string
	CreatedAt, UpdatedAt                                                                                                time.Time
}

// ---- person-binding links ----

type PublicationAuthorship struct {
	ID, PersonID, PublicationID string
	AuthorOrder                 *int
	Corresponding               bool
	EffectiveFrom, EffectiveTo  string
	CreatedAt, UpdatedAt        time.Time
}

type ResearchMembership struct {
	ID, PersonID, GroupID, Role, Status string
	EffectiveFrom, EffectiveTo          string
	CreatedAt, UpdatedAt                time.Time
}

type GrantHolding struct {
	ID, PersonID, GrantID, Role, Status string
	EffectiveFrom, EffectiveTo          string
	CreatedAt, UpdatedAt                time.Time
}

type GovernanceMembership struct {
	ID, PersonID, BodyID, RoleInBody, Status string
	EffectiveFrom, EffectiveTo               string
	CreatedAt, UpdatedAt                     time.Time
}

type QualificationAward struct {
	ID, PersonID, QualificationID, EnrollmentID, AwardedOn, Gpa, Status string
	WithDistinction                                                     bool
	CreatedAt, UpdatedAt                                                time.Time
}

type ScholarshipAward struct {
	ID, PersonID, ScholarshipID, Status string
	EffectiveFrom, EffectiveTo          string
	CreatedAt, UpdatedAt                time.Time
}

// ---- inputs (one upsert input per entity; create uses required fields, update ignores code) ----

type ProgramInput struct {
	Code, Name                                              string
	OwningUnitID, DegreeLevelID, Mode, DurationYears, State *string
	CreditHoursTotal                                        *int
}

type CourseInput struct {
	Code, Title                                     string
	OwningUnitID, Description, DeliveryMode, Status *string
	CreditHours, Level                              *int
}

type CurriculumVersionInput struct {
	VersionCode                        string
	EffectiveFrom, EffectiveTo, Status *string
}

type CurriculumItemInput struct {
	CourseID                                    string
	IsRequired                                  *bool
	YearOfStudy, CreditAllocation, SemesterSlot *int
}

type CoursePrerequisiteInput struct {
	RequiredCourseID string
	Kind, MinGrade   *string
}

type ResearchCentreInput struct {
	Code, Name                                          string
	Kind, FundingSource, FoundedOn, DissolvedOn, Status *string
}

type ResearchGroupInput struct {
	Code, Name                          string
	CentreID, UnitID, FocusArea, Status *string
}

type GrantInput struct {
	Code, Title                                                 string
	Funder, FunderRef, Amount, Currency, StartOn, EndOn, Status *string
}

type PublicationInput struct {
	Code, Title                                  string
	InstitutionID, Kind, Doi, Venue, PublishedOn *string
	OpenAccess                                   *bool
}

type GovernanceBodyInput struct {
	Code, Name            string
	Kind, Mandate, Status *string
}

type PolicyInput struct {
	Code, Title                                                                      string
	GovernanceBodyID, SupersedesID, Kind, EffectiveOn, ExpiryOn, DocumentURL, Status *string
}

type QualificationInput struct {
	Code, Name                                                                    string
	ProgramID, DegreeLevelID, FrameworkCode, FrameworkLevel, AwardingBody, Status *string
}

type ScholarshipInput struct {
	Code, Name                                                           string
	InstitutionID, Kind, Amount, Currency, Frequency, Conditions, Status *string
	Renewable                                                            *bool
}

type AccreditationEventInput struct {
	EntityKind                                                                                          string
	InstitutionID, ProgramID, Body, BodyCountryID, Outcome, ReviewOn, EffectiveFrom, EffectiveTo, Notes *string
}

type PublicationAuthorshipInput struct {
	PublicationID              string
	AuthorOrder                *int
	Corresponding              *bool
	EffectiveFrom, EffectiveTo *string
}

type ResearchMembershipInput struct {
	GroupID                                  string
	Role, Status, EffectiveFrom, EffectiveTo *string
}

type GrantHoldingInput struct {
	GrantID                                  string
	Role, Status, EffectiveFrom, EffectiveTo *string
}

type GovernanceMembershipInput struct {
	BodyID                                         string
	RoleInBody, Status, EffectiveFrom, EffectiveTo *string
}

type QualificationAwardInput struct {
	QualificationID                      string
	EnrollmentID, AwardedOn, Gpa, Status *string
	WithDistinction                      *bool
}

type ScholarshipAwardInput struct {
	ScholarshipID                      string
	Status, EffectiveFrom, EffectiveTo *string
}

// ---- validation (required fields) ----

func (in ProgramInput) Validate() error               { return reqCodeName(in.Code, in.Name) }
func (in CourseInput) Validate() error                { return reqCodeName(in.Code, in.Title) }
func (in CurriculumVersionInput) Validate() error     { return reqStr(in.VersionCode) }
func (in CurriculumItemInput) Validate() error        { return reqStr(in.CourseID) }
func (in CoursePrerequisiteInput) Validate() error    { return reqStr(in.RequiredCourseID) }
func (in ResearchCentreInput) Validate() error        { return reqCodeName(in.Code, in.Name) }
func (in ResearchGroupInput) Validate() error         { return reqCodeName(in.Code, in.Name) }
func (in GrantInput) Validate() error                 { return reqCodeName(in.Code, in.Title) }
func (in PublicationInput) Validate() error           { return reqCodeName(in.Code, in.Title) }
func (in GovernanceBodyInput) Validate() error        { return reqCodeName(in.Code, in.Name) }
func (in PolicyInput) Validate() error                { return reqCodeName(in.Code, in.Title) }
func (in QualificationInput) Validate() error         { return reqCodeName(in.Code, in.Name) }
func (in ScholarshipInput) Validate() error           { return reqCodeName(in.Code, in.Name) }
func (in PublicationAuthorshipInput) Validate() error { return reqStr(in.PublicationID) }
func (in ResearchMembershipInput) Validate() error    { return reqStr(in.GroupID) }
func (in GrantHoldingInput) Validate() error          { return reqStr(in.GrantID) }
func (in GovernanceMembershipInput) Validate() error  { return reqStr(in.BodyID) }
func (in QualificationAwardInput) Validate() error    { return reqStr(in.QualificationID) }
func (in ScholarshipAwardInput) Validate() error      { return reqStr(in.ScholarshipID) }

func (in AccreditationEventInput) Validate() error {
	if in.EntityKind != "institution" && in.EntityKind != "program" {
		return ErrInvalid
	}
	if in.EntityKind == "institution" && (in.InstitutionID == nil || *in.InstitutionID == "") {
		return ErrInvalid
	}
	if in.EntityKind == "program" && (in.ProgramID == nil || *in.ProgramID == "") {
		return ErrInvalid
	}
	return nil
}

func reqCodeName(code, name string) error {
	if !validCode(code) || strings.TrimSpace(name) == "" {
		return ErrInvalid
	}
	return nil
}

func reqStr(s string) error {
	if strings.TrimSpace(s) == "" {
		return ErrInvalid
	}
	return nil
}
