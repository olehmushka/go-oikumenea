// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	personapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// PersonStats implements GET /person/v1/stats/persons — the dashboard half of the person facet
// vocabulary (M57 / D-ObjectFacets).
//
// It is the SAME request state as ListPersons: the identical gate, the identical filter parsing and
// the identical read-scope dispatch, differing only in aggregating rather than paging. That is not
// tidiness — a dashboard computed over a different candidate set than its list is a chart that
// describes rows the list will not return.
func (s Service) PersonStats(ctx context.Context, token bearertoken.Token, facets *string, query *string, sex *string, status *string, birthdateFrom *string, birthdateTo *string, countryOfBirth *string, rankID *string, unitID *string, graph *string, hasAccount *bool) (personapi.PersonStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.PersonStats{}, err
	}
	filter, err := personFilter(query, sex, status, birthdateFrom, birthdateTo, countryOfBirth, rankID, unitID, graph, hasAccount)
	if err != nil {
		return personapi.PersonStats{}, s.mapError(ctx, err, "")
	}
	sel, err := selectPersonFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return personapi.PersonStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return personapi.PersonStats{}, s.mapError(ctx, err, "")
	}
	res, err := s.app.PersonStats(ctx, subject, isAdmin, filter, sel)
	if err != nil {
		return personapi.PersonStats{}, s.mapError(ctx, err, "")
	}
	return personapi.PersonStats{TotalCount: int(res.TotalCount), Facets: toAPIDistributions(res)}, nil
}

// selectPersonFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2, the D-UnifiedSearch skip-the-provider behaviour). An undeclared facet key
// IS a caller error, because it is a typo rather than a permission.
func selectPersonFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("person")
	if !ok { // the boot-time MustBeBound makes this unreachable; failing loudly beats an empty dashboard
		return stats.Selection{}, personapi.NewPersonInvalid("person facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, personapi.NewPersonInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIDistributions maps the assembled kernel result onto the wire type. The bucket key is carried
// verbatim — it is what the caller passes back as a filter value, so it must survive the round trip
// untouched, synthetic `(unknown)`/`(other)` keys included (which the console renders unlinked).
func toAPIDistributions(res stats.Result) []personapi.FacetDistribution {
	out := make([]personapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]personapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := personapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, personapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
