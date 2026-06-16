package domain

import (
	"errors"
	"time"
)

// Location is the shared, standalone place entity (D-Location, M19): a precise WGS84 coordinate, the
// DB-derived MGRS + H3 spatial indexes, and a structured postal address over the country registry. A
// location carries no owner/visibility/purpose — a referencing module owns the meaning on its own link.
type Location struct {
	ID          string
	Latitude    float64
	Longitude   float64
	MGRS        *string // DB-derived (NULL for polar UPS coordinates)
	H3Res5      *string // DB-derived H3 cell, ~9km
	H3Res7      *string // ~1.2km
	H3Res9      *string // ~150m
	H3Res11     *string // ~20m
	CountryID   string
	AdminArea1  *string
	AdminArea2  *string
	Locality    *string
	Street      *string
	HouseNumber *string
	PostalCode  *string
	RawAddress  *string
	TypeID      *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// LocationType is an instance-admin catalog label classifying a place (building/address/online);
// descriptive only, never branched on. Name is the default-locale value (the transport overlays the
// i18n store into a locale->text map).
type LocationType struct {
	ID     string
	Code   string
	Name   string
	Status string
}

// LocationWrite is the create/replace input. The coordinate is the required spine; MGRS + H3 are never
// supplied (DB-derived). The transport maps a missing coordinate to Location:CoordinateRequired before
// constructing this, so by the time the application sees a LocationWrite the coordinate is present.
type LocationWrite struct {
	Latitude    float64
	Longitude   float64
	CountryID   string
	AdminArea1  *string
	AdminArea2  *string
	Locality    *string
	Street      *string
	HouseNumber *string
	PostalCode  *string
	RawAddress  *string
	TypeID      *string
}

// Validate guards the coordinate range + required country before any DB work.
func (w LocationWrite) Validate() error {
	if w.Latitude < -90 || w.Latitude > 90 || w.Longitude < -180 || w.Longitude > 180 {
		return ErrCoordinateOutOfRange
	}
	if w.CountryID == "" {
		return ErrInvalidLocation
	}
	return nil
}

// Location domain errors. The transport maps these to the Conjure Location:* SerializableErrors.
var (
	ErrLocationNotFound     = errors.New("location not found")
	ErrLocationInUse        = errors.New("location is referenced and cannot be deleted")
	ErrCoordinateOutOfRange = errors.New("coordinate out of range")
	ErrInvalidLocation      = errors.New("invalid location")
)
