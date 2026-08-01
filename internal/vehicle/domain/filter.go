// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// VehicleFilter is the vehicle facet vocabulary in Go (M58 ticket 3 / D-ObjectFacets), shared
// verbatim by the list path and the stats path.
//
// Like external_organization and taxon, and unlike the M57 five, there is no admin/scoped pair to
// keep in step: `vehicle_vehicles` carries no row-level security, no unit column and no reach
// predicate, so `vehicle.read` held anywhere is the whole visibility decision and the aggregate ships
// ONE arm. The struct exists for the OTHER half of the no-drift contract — the list and the dashboard
// must select the same rows from the same arguments, or a chart segment and a filter stop being the
// same act.
type VehicleFilter struct {
	// TypeID / ModelID / ColorID are RIDs of this module's or the platform's own catalogs. ColorID
	// points at platform_colors (domain='vehicle') — a HARD FK since M42/D-Color, which is what lets
	// the console tint the colour chart from platform_colors.hex rather than guessing at free text.
	TypeID  *string
	ModelID *string
	ColorID *string

	// BrandID is TWO-HOP: a vehicle has no brand column, so this matches through the vehicle's model
	// (vehicle_models.brand_id). The list projection has always carried the derived brand_id, so the
	// join is not new — only the predicate is.
	BrandID *string

	Status *string

	// The bounds are INCLUSIVE calendar dates (manufacture_date is a DATE, not a timestamptz — so no
	// RFC-3339 widening, the opposite of external_organization.asOf). A vehicle with no manufacture
	// date is excluded whenever either bound is set (SQL three-valued logic); that set is a distinct
	// (unknown) bucket on the stats endpoint rather than a filterable value.
	ManufactureDateFrom *string
	ManufactureDateTo   *string

	// RegistrationCountry matches the country of the vehicle's ACTIVE registration, as an EXISTS
	// semi-join. Registration is ownership HISTORY and is one-to-many; a join here would count a
	// re-registered vehicle once per country it has ever worn plates in. Confining it to the active
	// row — of which there is at most one per vehicle — is what makes both this filter and its
	// distribution partition.
	RegistrationCountry *string
}

// Validate rejects a caller value the SQL would otherwise accept and silently return nothing for —
// an enum outside the declared CHECK set, a malformed RID, or an inverted date range. Every check
// reads the SHIPPED facet catalog rather than a local copy of the value set, so a CHECK constraint
// and its filter cannot drift apart.
func (f VehicleFilter) Validate() error {
	if err := validateFacetEnum("status", f.Status); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"typeId", f.TypeID},
		{"modelId", f.ModelID},
		{"color", f.ColorID},
		{"brandId", f.BrandID},
		{"registrationCountry", f.RegistrationCountry},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrInvalid, r.arg)
		}
	}
	// Compared as plain ISO-8601 calendar dates: manufacture_date is a DATE, so lexical order is
	// chronological order and there is no timezone to normalize first.
	if f.ManufactureDateFrom != nil && f.ManufactureDateTo != nil &&
		*f.ManufactureDateTo < *f.ManufactureDateFrom {
		return fmt.Errorf("%w: manufactureDateTo precedes manufactureDateFrom", ErrInvalid)
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). An unknown facet key is a programming error in this file, not a caller error.
func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("vehicle")
	if !ok {
		return fmt.Errorf("vehicle is not a registered facet type")
	}
	for _, ft := range o.Facets {
		if ft.Key != facetKey {
			continue
		}
		for _, allowed := range ft.Values {
			if *v == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalid, facetKey, ft.Values)
	}
	return fmt.Errorf("%s is not a declared vehicle facet", facetKey)
}
