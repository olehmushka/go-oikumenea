// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	locationapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/location"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// The location dashboard endpoint (M58 ticket 6 / D-ObjectFacets) — the second half of the facet
// vocabulary, over the same arguments `listLocations` takes.
//
// It takes the WINDOW as well as the structural filters, which is what separates this type from
// languoid: a radius or a box is a predicate over the listed table, so it narrows the aggregate
// exactly as it narrows the list. A tree-walk arg selects a listing mode and is counted by nothing;
// a window selects a subset of the same population and must be.

func (s LocationService) LocationStats(ctx context.Context, token bearertoken.Token, facets *string, lat, lng, radiusM, minLat, minLng, maxLat, maxLng *float64, query, countryID, typeID *string) (locationapi.LocationStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, locReadPerm); err != nil {
		return locationapi.LocationStats{}, err
	}
	sel, err := selectLocationFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return locationapi.LocationStats{}, err
	}
	// No subject and no admin flag: a location has no visibility bit, so there is no arm to pick. The
	// application says so in the one place the arm convention is read (stats.Compute).
	res, err := s.app.LocationStats(ctx,
		locationFilterFrom(lat, lng, radiusM, minLat, minLng, maxLat, maxLng, query, countryID, typeID), sel)
	if err != nil {
		return locationapi.LocationStats{}, s.mapError(ctx, err, "")
	}
	return locationapi.LocationStats{TotalCount: int(res.TotalCount), Facets: toAPIFacetDistributions(res)}, nil
}

// selectLocationFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectLocationFacets(ctx context.Context, s LocationService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("location")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, locationapi.NewInvalid("location facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, locationapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIFacetDistributions maps the kernel result onto the generated wire types.
func toAPIFacetDistributions(res stats.Result) []locationapi.FacetDistribution {
	out := make([]locationapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]locationapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := locationapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, locationapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}

// strOr dereferences an optional query arg to its value or the empty string.
func strOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
