// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	orderapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/order"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// OrderStats implements GET /order/v1/stats/orders — the dashboard half of the order facet
// vocabulary (M57 / D-ObjectFacets).
//
// It is the SAME request state as ListOrders: the identical gate, the identical filter parsing and the
// identical arm dispatch, differing only in aggregating rather than paging. That is not tidiness — a
// dashboard computed over a different candidate set than its list is a chart that describes rows the
// list will not return.
func (s Service) OrderStats(ctx context.Context, token bearertoken.Token, facets *string, issuingUnitID, orderTypeID, status, issuedOnFrom, issuedOnTo *string) (orderapi.OrderStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderRead)); err != nil {
		return orderapi.OrderStats{}, err
	}
	filter, err := orderFilter(issuingUnitID, orderTypeID, status, issuedOnFrom, issuedOnTo)
	if err != nil {
		return orderapi.OrderStats{}, s.mapError(ctx, err, errCtx{})
	}
	sel, err := selectOrderFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return orderapi.OrderStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return orderapi.OrderStats{}, s.mapError(ctx, err, errCtx{})
	}
	res, err := s.app.OrderStats(ctx, subject, isAdmin, filter, sel)
	if err != nil {
		return orderapi.OrderStats{}, s.mapError(ctx, err, errCtx{})
	}
	return orderapi.OrderStats{TotalCount: int(res.TotalCount), Facets: toAPIOrderDistributions(res)}, nil
}

// selectOrderFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
func selectOrderFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("order")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, orderapi.NewOrderInvalid("order facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, orderapi.NewOrderInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIOrderDistributions maps the assembled kernel result onto the wire type, carrying each bucket key
// VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)` keys
// included (which the console renders unlinked). Copied per module because Conjure generates the types
// per file — there is no shared type to map to.
func toAPIOrderDistributions(res stats.Result) []orderapi.FacetDistribution {
	out := make([]orderapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]orderapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := orderapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, orderapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
