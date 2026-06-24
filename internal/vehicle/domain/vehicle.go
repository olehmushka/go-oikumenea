// Package domain holds the vehicle module's entities, ports, and invariants (docs/modules/vehicle.md /
// D-Vehicles) — a generic vehicle registry: a brand/model/type taxonomy, the physical vehicle (VIN),
// the brand→manufacturer link (to a M21 company), and the ownership+plate registration record. The
// domain owns its Repository interface and imports no framework.
package domain

import (
	"errors"
	"strings"
	"time"
)

// ISODate is the wire/storage format for the date-only fields (manufacture dates, link windows).
const ISODate = "2006-01-02"

// Owner kinds for the polymorphic registration owner (a vehicle is registered to a person OR a company).
const (
	OwnerPerson  = "person"
	OwnerCompany = "company"
)

// ---- sentinel errors (mapped to Conjure Vehicle:* in transport) ----
var (
	ErrVehicleNotFound = errors.New("vehicle: vehicle not found")
	ErrBrandNotFound   = errors.New("vehicle: brand not found")
	ErrLinkNotFound    = errors.New("vehicle: link not found")
	ErrConflict        = errors.New("vehicle: code or identifier already exists in scope")
	ErrInvalid         = errors.New("vehicle: invalid request or unknown reference")
	ErrRegionInvalid   = errors.New("vehicle: plate region is not a geo_places region")
)

// ---- catalogs ----

type VehicleType struct {
	ID, Code, Name, Status string
	ParentID, RootID       string // "" = root / unset
	SortOrder              *int
}

type Brand struct {
	ID, Code, Name, Status string
	CountryID              string // "" = none
	SortOrder              *int
}

type Model struct {
	ID, BrandID, Code, Name, Status  string
	Generation                       string // "" = none
	ManufactureStart, ManufactureEnd string // ISO date; "" = none
	SortOrder                        *int
}

type NumberType struct {
	ID, Code, Name, Status string
	SortOrder              *int
}

// ---- object ----

// Vehicle is a physical vehicle. BrandID is derived from the model (a containment FK chain) for label
// resolution; it is "" when no model is set.
type Vehicle struct {
	ID, TypeID           string
	ModelID              string // "" = unknown model
	BrandID              string // derived from model; "" when no model
	VIN, Color           string // "" = none
	ManufactureDate      string // ISO date; "" = none
	Attributes           string // JSON object string; "{}" when unset
	Status               string
	CreatedAt, UpdatedAt time.Time
}

// ---- reified links ----

type Manufacturer struct {
	ID, BrandID, CompanyID     string
	CompanyLabel               string // best-effort, resolved in transport
	EffectiveFrom, EffectiveTo string // ISO date; "" = none
	CreatedAt, UpdatedAt       time.Time
}

type Registration struct {
	ID, VehicleID, OwnerKind, OwnerID string
	OwnerLabel                        string // best-effort, resolved in transport
	CountryID                         string
	SubdivisionID                     string // "" = none
	SubdivisionLabel                  string // best-effort, resolved in transport
	RegistrationNumber                string
	NumberTypeID                      string // "" = none
	Status                            string
	EffectiveFrom                     time.Time
	EffectiveTo                       *time.Time
	CreatedAt, UpdatedAt              time.Time
}

// PersonRegistration is the read-side view of a person-owned registration, enriched with the vehicle's
// identity. Labels are resolved in transport.
type PersonRegistration struct {
	ID, VehicleID      string
	VIN                string
	TypeID, ModelID    string
	BrandID            string
	RegistrationNumber string
	CountryID          string
	SubdivisionID      string
	Status             string
	EffectiveFrom      time.Time
	EffectiveTo        *time.Time
}

// ---- inputs ----

type VehicleInput struct {
	TypeID          string
	ModelID         string // "" = none
	VIN             string // normalized; "" = none
	Color           string
	ManufactureDate string // ISO date; "" = none
	Attributes      string // JSON object string; "" → "{}"
}

// VehicleUpdate carries partial updates; a nil field is left unchanged.
type VehicleUpdate struct {
	TypeID          *string
	ModelID         *string
	VIN             *string
	Color           *string
	ManufactureDate *string
	Attributes      *string
	Status          *string
}

type RegistrationInput struct {
	OwnerKind, OwnerID string
	CountryID          string
	SubdivisionID      string // "" = none
	RegistrationNumber string
	NumberTypeID       string // "" = none
	EffectiveFrom      *time.Time
}

type ManufacturerInput struct {
	CompanyID                  string
	EffectiveFrom, EffectiveTo string // ISO date; "" = none
}

// ---- validators ----

// NormalizeVIN trims and upper-cases a VIN (registry convention); empty stays empty.
func NormalizeVIN(vin string) string {
	return strings.ToUpper(strings.TrimSpace(vin))
}

func (in VehicleInput) Validate() error {
	if strings.TrimSpace(in.TypeID) == "" {
		return ErrInvalid
	}
	return nil
}

func validOwnerKind(k string) bool { return k == OwnerPerson || k == OwnerCompany }

func (in RegistrationInput) Validate() error {
	if !validOwnerKind(in.OwnerKind) || strings.TrimSpace(in.OwnerID) == "" {
		return ErrInvalid
	}
	if strings.TrimSpace(in.CountryID) == "" || strings.TrimSpace(in.RegistrationNumber) == "" {
		return ErrInvalid
	}
	return nil
}
