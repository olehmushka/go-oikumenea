// Package domain holds the geo module's pure logic: the Country registry entry and the Repository
// port it needs from the outside world (overview.md layering). No I/O, no framework imports — only
// the standard library. Geo owns the read side of the location service's country registry (D-Geo):
// countries are RID-keyed (F-014 / D-ResourceIdentifiers) and referenced by person/document/rank;
// this module lets clients resolve a country to its RID. The registry itself is written by the
// hermenea import pipeline (geo-countries / WOF), not here.
package domain

import "context"

// Country is one entry in the ISO-3166-1 registry. ID is the RID (the reference key other modules
// store); Code is the stable ISO-3166-1 alpha-2 lookup code; Name is the default-locale name.
type Country struct {
	ID     string
	Code   string
	Name   string
	Status string
}

// Repository is the geo module's port: a read-only view of the country registry plus the audited CRUD
// + spatial reads over the shared Location entity (D-Location, M19). The country side is written by the
// hermenea import pipeline (not here); the location side is owned by this module.
type Repository interface {
	ListCountries(ctx context.Context) ([]Country, error)

	// location (D-Location)
	InsertLocation(ctx context.Context, w LocationWrite) (Location, error)
	GetLocation(ctx context.Context, id string) (Location, error)
	UpdateLocation(ctx context.Context, id string, w LocationWrite) (Location, error)
	SoftDeleteLocation(ctx context.Context, id string) (int64, error)
	ListLocationsNear(ctx context.Context, lat, lng, radiusM float64, limit, offset int) ([]Location, error)
	ListLocationsInBbox(ctx context.Context, minLat, minLng, maxLat, maxLng float64, limit, offset int) ([]Location, error)
	SearchLocationsByText(ctx context.Context, query string, limit, offset int) ([]Location, error)
	ListLocationTypes(ctx context.Context) ([]LocationType, error)
}
