// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"encoding/json"
	"errors"
	"time"
)

// Location is the shared, standalone place entity (D-Location, M19): a precise WGS84 coordinate, the
// app-derived MGRS index, the original input coordinate, and a structured postal address over the
// country registry. A location carries no owner/visibility/purpose — a referencing module owns the
// meaning on its own link.
type Location struct {
	ID               string
	Latitude         float64
	Longitude        float64
	MGRS             *string         // app-derived (nil for polar UPS coordinates)
	SourceCoordinate json.RawMessage // the coordinate input as originally supplied (format + raw values)
	CountryID        string
	AdminArea1       *string
	AdminArea2       *string
	Locality         *string
	Street           *string
	HouseNumber      *string
	PostalCode       *string
	RawAddress       *string
	TypeID           *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// DistanceM is the metres-from-query distance, populated only by the nearest-first radius search
	// (ListLocationsNear) so the transport can build its (distance, id) keyset page token (review R-21).
	// Zero for every other read; never surfaced in the API projection.
	DistanceM float64
}

// LocationMode names which of `listLocations`' four listing modes a request selected (M58 ticket 6).
// It is resolved ONCE, in the transport, and carried on the filter — so the list and the dashboard
// are handed the same answer rather than each re-deriving it from the raw arguments.
type LocationMode string

const (
	// LocationModeBrowse is the whole registry in RID order — the mode this type had no way to ask for
	// before M58 ticket 6, which is why it had no filters and no dashboard.
	LocationModeBrowse LocationMode = "browse"
	// LocationModeText matches the address haystack (search_text) — the R-21 trigram twin.
	LocationModeText LocationMode = "text"
	// LocationModeRadius is ST_DWithin around (Lat,Lng), nearest first.
	LocationModeRadius LocationMode = "radius"
	// LocationModeBbox is ST_Intersects against the envelope.
	LocationModeBbox LocationMode = "bbox"
)

// LocationFilter is one listing request: the resolved mode, its window, and the structural facet
// filters (D-ObjectFacets). CountryID/TypeID apply in EVERY mode — they narrow the window rather than
// replacing it — which is what lets the dashboard describe a windowed list rather than the registry.
type LocationFilter struct {
	Mode  LocationMode
	Query string

	// Radius window (Mode == LocationModeRadius).
	Lat, Lng, RadiusM float64
	// Bounding-box window (Mode == LocationModeBbox).
	MinLat, MinLng, MaxLat, MaxLng float64

	CountryID *string
	TypeID    *string
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

// LocationInput is the create/replace request as it arrives from the transport: a coordinate in any
// supported format plus the structured address. ToWrite converts the coordinate to canonical WGS84,
// derives the MGRS, and captures the original input — producing the LocationWrite the repository stores.
type LocationInput struct {
	Coordinate  CoordinateInput
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

// ToWrite resolves the coordinate (ErrCoordinateInvalid/ErrCoordinateOutOfRange on failure) and requires
// a country (ErrInvalidLocation), then builds the resolved LocationWrite (canonical lat/lon + derived
// MGRS + the source coordinate preserved verbatim).
func (in LocationInput) ToWrite() (LocationWrite, error) {
	lat, lon, err := in.Coordinate.ToWGS84()
	if err != nil {
		return LocationWrite{}, err
	}
	if in.CountryID == "" {
		return LocationWrite{}, ErrInvalidLocation
	}
	return LocationWrite{
		Latitude:         lat,
		Longitude:        lon,
		MGRS:             DeriveMGRS(lat, lon),
		SourceCoordinate: in.Coordinate.Raw(),
		CountryID:        in.CountryID,
		AdminArea1:       in.AdminArea1,
		AdminArea2:       in.AdminArea2,
		Locality:         in.Locality,
		Street:           in.Street,
		HouseNumber:      in.HouseNumber,
		PostalCode:       in.PostalCode,
		RawAddress:       in.RawAddress,
		TypeID:           in.TypeID,
	}, nil
}

// LocationWrite is the create/replace input the repository persists: the canonical WGS84 coordinate the
// application has already resolved from the supplied CoordinateInput, the MGRS it derived, and the
// original input (SourceCoordinate). MGRS is never client-supplied.
type LocationWrite struct {
	Latitude         float64
	Longitude        float64
	MGRS             *string
	SourceCoordinate json.RawMessage
	CountryID        string
	AdminArea1       *string
	AdminArea2       *string
	Locality         *string
	Street           *string
	HouseNumber      *string
	PostalCode       *string
	RawAddress       *string
	TypeID           *string
}

// Location domain errors. The transport maps these to the Conjure Location:* SerializableErrors.
var (
	ErrLocationNotFound     = errors.New("location not found")
	ErrLocationInUse        = errors.New("location is referenced and cannot be deleted")
	ErrCoordinateOutOfRange = errors.New("coordinate out of range")
	ErrCoordinateInvalid    = errors.New("coordinate could not be parsed or converted")
	ErrInvalidLocation      = errors.New("invalid location")
)
