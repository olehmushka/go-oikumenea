// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"

	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// Repository is the vehicle module's persistence port (implemented by adapters over raw pgx). It is
// bound to a single command surface — the pool for reads, or a caller's transaction for an audited
// write (D-Audit). The application layer owns transaction boundaries; the repository never opens its own.
type Repository interface {
	// catalogs
	ListVehicleTypes(ctx context.Context) ([]VehicleType, error)
	GetVehicleType(ctx context.Context, id string) (VehicleType, error)
	UpsertVehicleType(ctx context.Context, code, name string, parentID *string, sortOrder *int) (VehicleType, error)
	ListBrands(ctx context.Context, query string) ([]Brand, error)
	GetBrand(ctx context.Context, id string) (Brand, error)
	UpsertBrand(ctx context.Context, code, name string, countryID *string, sortOrder *int) (Brand, error)
	ListModelsByBrand(ctx context.Context, brandID string) ([]Model, error)
	UpsertModel(ctx context.Context, brandID, code, name string, generation, manufactureStart, manufactureEnd *string, sortOrder *int) (Model, error)
	ListNumberTypes(ctx context.Context) ([]NumberType, error)
	UpsertNumberType(ctx context.Context, code, name string, sortOrder *int) (NumberType, error)

	// vehicles
	InsertVehicle(ctx context.Context, in VehicleInput) (Vehicle, error)
	GetVehicle(ctx context.Context, id string) (Vehicle, error)
	UpdateVehicle(ctx context.Context, id string, up VehicleUpdate) (Vehicle, error)
	// ListVehicles pages the same set VehicleStats aggregates, under the same VehicleFilter — one
	// shared predicate, so `totalCount` describes exactly what paging returns (M58 / D-ObjectFacets).
	ListVehicles(ctx context.Context, query, after string, f VehicleFilter, lim int) ([]Vehicle, error)
	// VehicleStats is the dashboard half: one round-trip, one scan, every selected facet's
	// distribution plus the total. ONE arm — vehicle_vehicles has no row-level security and no unit
	// reach, so there is no visibility predicate for a second arm to narrow.
	VehicleStats(ctx context.Context, query string, f VehicleFilter, sel stats.Selection) ([]stats.Group, error)
	SoftDeleteVehicle(ctx context.Context, id string) (int64, error)

	// registrations (ownership history)
	InsertRegistration(ctx context.Context, vehicleID string, in RegistrationInput) (Registration, error)
	GetRegistration(ctx context.Context, id string) (Registration, error)
	CloseActiveRegistrationsForVehicle(ctx context.Context, vehicleID string) error
	CloseRegistration(ctx context.Context, id string) (Registration, error)
	ListRegistrationsByVehicle(ctx context.Context, vehicleID string) ([]Registration, error)
	ListRegistrationsByPersonOwner(ctx context.Context, personID string) ([]PersonRegistration, error)
	// ErasePersonRegistrations soft-deletes a person's owned registrations on purge (D-PIITiers).
	ErasePersonRegistrations(ctx context.Context, personID string) (int64, error)

	// brand manufacturers
	InsertManufacturer(ctx context.Context, brandID string, in ManufacturerInput) (Manufacturer, error)
	GetManufacturer(ctx context.Context, id string) (Manufacturer, error)
	SoftDeleteManufacturer(ctx context.Context, id string) (int64, error)
	ListManufacturersByBrand(ctx context.Context, brandID string) ([]Manufacturer, error)

	// label helpers (best-effort default-locale display names for the API label fields)
	TypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	BrandNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	ModelNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)

	// cross-reference helpers (label resolution + region validation)
	IsRegion(ctx context.Context, placeID string) (bool, error)
	// CompanyNamesByIDs returns default-locale legal names for company ids (owner/manufacturer labels).
	CompanyNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
	// PlaceNamesByIDs returns default-locale geo_places names for ids (subdivision labels).
	PlaceNamesByIDs(ctx context.Context, ids []string) (map[string]string, error)
}
