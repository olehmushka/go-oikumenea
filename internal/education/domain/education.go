// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the education module's entities, ports, and invariants (docs/modules/education.md
// / D-Education) — external reference institutions, their recursive structure tree, buildings, groups,
// positions/appointments (mirrors membership), and the person bindings (enrollments, dorm stays). The
// domain owns its Repository interface and imports no framework.
package domain

import (
	"errors"
	"strings"
	"time"
)

// ISODate is the wire/storage format for the date-only fields (effective_from/to, founded/closed).
const ISODate = "2006-01-02"

// ---- sentinel errors (mapped to Conjure Education:* in transport) ----
var (
	ErrInstitutionNotFound   = errors.New("education: institution not found")
	ErrUnitNotFound          = errors.New("education: unit not found")
	ErrBuildingNotFound      = errors.New("education: building not found")
	ErrGroupNotFound         = errors.New("education: group not found")
	ErrPositionNotFound      = errors.New("education: position not found")
	ErrAppointmentNotFound   = errors.New("education: appointment not found")
	ErrEnrollmentNotFound    = errors.New("education: enrollment not found")
	ErrDormNotFound          = errors.New("education: dormitory stay not found")
	ErrConflict              = errors.New("education: code already exists in scope")
	ErrInvalid               = errors.New("education: invalid request or unknown reference")
	ErrUnitCycle             = errors.New("education: reparent would create a cycle")
	ErrPositionAlreadyFilled = errors.New("education: position already filled")
	ErrInUse                 = errors.New("education: entity still referenced")
	ErrLifecycle             = errors.New("education: invalid lifecycle transition")
)

// ---- catalogs ----

type InstitutionKind struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

type UnitKind struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

type DegreeLevel struct {
	ID, Code, Name string
	IscedLevel     int
	Status         string
	SortOrder      *int
}

// ---- objects ----

// VisibilityShadow is the owning organization's `shadow` value (D-VisibilityScope). Declared here
// rather than imported from the tenant domain because a module's domain imports no other module's —
// the value is the tenant_organizations CHECK set's own spelling, and the queries project it verbatim.
const VisibilityShadow = "shadow"

type Institution struct {
	ID, Code, Name, KindID string
	CountryID              string // "" = none
	FoundedOn, ClosedOn    string // ISO date; "" = none
	State                  string
	// Visibility is the OWNING tenant organization's public/shadow bit (M41 / D-UnifiedOrgGraph — an
	// institution IS a `university`-domain organization). Carried for the transport's shadow gate and
	// NOT on the wire: `shadow` hides existence, so the value only ever decides whether the caller sees
	// the row at all. "public" | "shadow" (D-VisibilityScope).
	Visibility           string
	CreatedAt, UpdatedAt time.Time
}

type Unit struct {
	ID, InstitutionID, ParentID, KindID, Code, Name, Status string
	SortOrder                                               *int
	Depth                                                   int
	CreatedAt, UpdatedAt                                    time.Time
}

type Building struct {
	ID, InstitutionID, UnitID, LocationID, Code, Name, Kind string
	CreatedAt, UpdatedAt                                    time.Time
}

type Group struct {
	ID, UnitID, Code, Name string
	AdmissionYear          *int
	Status                 string
	CreatedAt, UpdatedAt   time.Time
}

type Position struct {
	ID, InstitutionID, UnitID, Code, Title, Status string
	SortOrder                                      *int
	Holder                                         *Appointment
	CreatedAt, UpdatedAt                           time.Time
}

type Appointment struct {
	ID, PersonID, PositionID, Status string
	EffectiveFrom                    time.Time
	EffectiveTo                      *time.Time
	CreatedAt, UpdatedAt             time.Time
}

// PersonAppointment is an Appointment enriched with the position's title and owning institution, for
// the read-only person view (GET /persons/{id}/appointments).
type PersonAppointment struct {
	ID, PersonID, PositionID, Status string
	PositionTitle                    string
	InstitutionID, InstitutionName   string
	EffectiveFrom                    time.Time
	EffectiveTo                      *time.Time
	CreatedAt, UpdatedAt             time.Time
}

// ---- person bindings ----

type Enrollment struct {
	ID, PersonID, InstitutionID                                                                   string
	UnitID, GroupID, ProgramID, DegreeLevelID, FieldOfStudy, StudentNumber, Status, Qualification string
	EffectiveFrom, EffectiveTo                                                                    string // ISO date; "" = none
	CreatedAt, UpdatedAt                                                                          time.Time
}

type DormitoryStay struct {
	ID, PersonID, BuildingID, Room, Status string
	EffectiveFrom, EffectiveTo             string // ISO date; "" = none
	CreatedAt, UpdatedAt                   time.Time
}

// ClosureReport is the result of a unit-tree closure verify/rebuild for an institution.
type ClosureReport struct {
	InstitutionID  string
	Missing, Extra int
	InDrift        bool
}

// ---- inputs ----

type InstitutionInput struct {
	Code, Name, KindID             string
	CountryID, FoundedOn, ClosedOn *string
}

type InstitutionUpdate struct {
	Name, KindID, CountryID, FoundedOn, ClosedOn, State *string
}

type UnitInput struct {
	Code, Name, KindID string
	ParentID           *string
	SortOrder          *int
}

type UnitUpdate struct {
	Name, KindID, Status *string
	SortOrder            *int
}

type BuildingInput struct {
	Code, Name, Kind   string
	UnitID, LocationID *string
}

type BuildingUpdate struct {
	Name, Kind, UnitID, LocationID *string
}

type GroupInput struct {
	Code, Name    string
	AdmissionYear *int
}

type GroupUpdate struct {
	Name          *string
	Status        *string
	AdmissionYear *int
}

type PositionInput struct {
	Code, Title string
	UnitID      *string
	SortOrder   *int
}

type PositionUpdate struct {
	Title     *string
	SortOrder *int
}

type EnrollmentInput struct {
	InstitutionID                                           string
	UnitID, GroupID, ProgramID, DegreeLevelID, FieldOfStudy *string
	StudentNumber                                           *string
	Status, Qualification, EffectiveFrom, EffectiveTo       *string
}

type DormInput struct {
	BuildingID                               string
	Room, Status, EffectiveFrom, EffectiveTo *string
}

// validCode is a stable, non-empty, whitespace-free identifier (D-Code).
func validCode(code string) bool {
	return code != "" && len(code) <= 128 && !strings.ContainsAny(code, " \t\n")
}

// Validate checks an institution create input.
func (in InstitutionInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.Name) == "" || in.KindID == "" {
		return ErrInvalid
	}
	return nil
}

func (in UnitInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.Name) == "" || in.KindID == "" {
		return ErrInvalid
	}
	return nil
}

func (in BuildingInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.Name) == "" || in.Kind == "" {
		return ErrInvalid
	}
	return nil
}

func (in GroupInput) Validate() error {
	if !validCode(in.Code) || strings.TrimSpace(in.Name) == "" {
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

func (in EnrollmentInput) Validate() error {
	if in.InstitutionID == "" {
		return ErrInvalid
	}
	return nil
}

func (in DormInput) Validate() error {
	if in.BuildingID == "" {
		return ErrInvalid
	}
	return nil
}

// InstitutionFilter is the facet filter block `listInstitutions` and `institutionStats` BOTH take
// (M58 ticket 5 / D-ObjectFacets). One struct, built by one transport helper, so a list and its
// dashboard cannot read the same URL differently — the sqlc parity guard proves the same for the SQL
// half.
//
// Every field is a pointer: nil means "criterion disabled", which the predicates read as a SQL NULL
// narg rather than an empty-string sentinel (a sentinel forces one generic plan across every filter
// shape and is invisible to the parity guard).
type InstitutionFilter struct {
	// Query is the free-text arm, routed to SearchInstitutions rather than ANDed as a predicate
	// (R-21: a `(@query = '' OR …)` guard would defeat the trigram GIN). Empty = the plain arm.
	Query         string
	KindID        *string
	CountryID     *string
	FoundedOnFrom *string // ISO date, INCLUSIVE
	FoundedOnTo   *string // ISO date, INCLUSIVE
	State         *string
}
