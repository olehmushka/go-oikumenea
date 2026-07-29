// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the geo module's pure logic: the Country registry entry and the Repository
// port it needs from the outside world (overview.md layering). No I/O, no framework imports — only
// the standard library. Geo owns the read side of the location service's country registry (D-Geo):
// countries are RID-keyed (F-014 / D-ResourceIdentifiers) and referenced by person/document/rank;
// this module lets clients resolve a country to its RID. The registry itself is written by the
// hermenea import pipeline (geo-countries / WOF), not here.
package domain

import (
	"context"
)

// Country is one entry in the ISO-3166-1 registry. ID is the RID (the reference key other modules
// store); Code is the stable ISO-3166-1 alpha-2 lookup code; Name is the default-locale name.
type Country struct {
	ID     string
	Code   string
	Name   string
	Status string
}

// Place is one administrative node in the WOF geo_places gazetteer (D-GeoPlaces): a country/region/
// county/locality. ID is the RID consumers reference (e.g. a vehicle plate region); CountryID is the
// RID of the containing country ("" for a country node). Name is the default-locale name.
type Place struct {
	ID        string
	Placetype string
	Name      string
	CountryID string
	Status    string
}

// NearestPlace is the gazetteer place closest to a coordinate (D-GeoPlaces): a locality
// (city/town/village) when one exists, else the nearest county/region. CountryID is the RID of the
// containing country ("" if unknown); DistanceMeters is the great-circle distance from the input point
// to the place's centroid.
type NearestPlace struct {
	ID             string
	Placetype      string
	Name           string
	CountryID      string
	DistanceMeters float64
}

// CoordinateResolution is the reverse-geocode of a coordinate: the containing country plus the nearest
// place. Either field is nil when the gazetteer has no coverage at the point.
type CoordinateResolution struct {
	Country *Country
	Place   *NearestPlace
}

// Repository is the geo module's port: a read-only view of the country registry plus the audited CRUD
// + spatial reads over the shared Location entity (D-Location, M19). The country side is written by the
// hermenea import pipeline (not here); the location side is owned by this module.
type Repository interface {
	ListCountries(ctx context.Context) ([]Country, error)
	// ListPlaces returns active geo_places of the given placetype under a country, in name order
	// (powers region pickers, e.g. a vehicle plate region — D-GeoPlaces).
	ListPlaces(ctx context.Context, countryID, placetype string) ([]Place, error)
	// ResolveCoordinate reverse-geocodes a WGS84 coordinate to the containing country plus the nearest
	// gazetteer place (locality, else county/region) — powers the locations-form prefill (D-GeoPlaces).
	ResolveCoordinate(ctx context.Context, lat, lng float64) (CoordinateResolution, error)

	// location (D-Location)
	InsertLocation(ctx context.Context, w LocationWrite) (Location, error)
	GetLocation(ctx context.Context, id string) (Location, error)
	UpdateLocation(ctx context.Context, id string, w LocationWrite) (Location, error)
	SoftDeleteLocation(ctx context.Context, id string) (int64, error)
	ListLocationsNear(ctx context.Context, lat, lng, radiusM, afterDist float64, afterID string, limit int) ([]Location, error)
	ListLocationsInBbox(ctx context.Context, minLat, minLng, maxLat, maxLng float64, after string, limit int) ([]Location, error)
	SearchLocationsByText(ctx context.Context, query, after string, limit int) ([]Location, error)
	ListLocationTypes(ctx context.Context) ([]LocationType, error)
}
