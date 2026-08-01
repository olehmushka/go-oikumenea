// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	vehicleapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/vehicle"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// VehicleStats is the dashboard half of the facet vocabulary (M58 ticket 3 / D-ObjectFacets). It
// takes exactly the filter args ListVehicles takes (minus paging) plus the `facets` CSV, and the
// filter is built by the SAME vehicleFilter helper the list uses — half of the no-drift contract
// (buildVehicleFilter in the adapter is the other half).
//
// The gate is the same coarse `vehicle.read` the list endpoint requires. There is no scoped arm:
// vehicle_vehicles has no row-level security and no unit reach, so a caller who passes this gate may
// read every row the aggregate counts.
func (s VehicleService) VehicleStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	query *string,
	typeID *string,
	brandID *string,
	modelID *string,
	color *string,
	status *string,
	manufactureDateFrom *string,
	manufactureDateTo *string,
	registrationCountry *string,
) (vehicleapi.VehicleStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.VehicleStats{}, err
	}
	sel, err := selectVehicleFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return vehicleapi.VehicleStats{}, err
	}
	f := vehicleFilter(typeID, brandID, modelID, color, status, manufactureDateFrom, manufactureDateTo, registrationCountry)
	res, err := s.app.VehicleStats(ctx, strOr(query), f, sel)
	if err != nil {
		return vehicleapi.VehicleStats{}, s.mapError(ctx, err)
	}
	return vehicleapi.VehicleStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPIVehicleDistributions(res),
	}, nil
}

// vehicleFilter is the ONE place a request's facet args become the domain filter, shared by the list
// and the stats endpoint. Sharing it is half of the no-drift contract: the two endpoints must read
// the same arguments the same way, or the same URL means two different things depending on which
// surface renders it.
func vehicleFilter(typeID, brandID, modelID, color, status, manufactureDateFrom, manufactureDateTo, registrationCountry *string) domain.VehicleFilter {
	return domain.VehicleFilter{
		TypeID:              typeID,
		BrandID:             brandID,
		ModelID:             modelID,
		ColorID:             color,
		Status:              status,
		ManufactureDateFrom: manufactureDateFrom,
		ManufactureDateTo:   manufactureDateTo,
		RegistrationCountry: registrationCountry,
	}
}

// selectVehicleFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
// (Every vehicle facet is pii:none, so the omission arm has no live case here.)
func selectVehicleFacets(ctx context.Context, s VehicleService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("vehicle")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, vehicleapi.NewInvalid("vehicle facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, vehicleapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIVehicleDistributions maps the assembled kernel result onto the wire type, carrying each bucket
// key VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)`
// keys included (which the console renders unlinked). Copied per module because Conjure generates the
// types per file — there is no shared type to map to.
func toAPIVehicleDistributions(res stats.Result) []vehicleapi.FacetDistribution {
	out := make([]vehicleapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]vehicleapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := vehicleapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, vehicleapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
