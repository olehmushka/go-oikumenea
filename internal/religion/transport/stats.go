// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	religionapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/religion"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// TaxonStats implements GET /religion/v1/stats/taxa — the dashboard half of the taxonomy facet
// vocabulary (M58 ticket 2 / D-ObjectFacets).
//
// It is the SAME request state as ListTaxa: the identical gate and the identical filter parsing,
// differing only in aggregating rather than paging. That is not tidiness — a dashboard computed over
// a different candidate set than its list is a chart describing rows the list will not return.
//
// No subject is resolved and no arm is picked, because there is no arm to pick: the taxonomy is flat
// instance-global reference data, and `religion.read` held anywhere — the same gate the list applies
// — is the whole visibility decision.
func (s ReligionService) TaxonStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	rank *string,
	parent *string,
	religion *string,
	classification *string,
	query *string,
) (religionapi.TaxonStats, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.TaxonStats{}, err
	}
	sel, err := selectTaxonFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return religionapi.TaxonStats{}, err
	}
	res, err := s.app.TaxonStats(ctx, strOr(query), taxonFilter(rank, parent, religion, classification), sel)
	if err != nil {
		return religionapi.TaxonStats{}, s.mapError(ctx, err)
	}
	return religionapi.TaxonStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPITaxonDistributions(res),
	}, nil
}

// taxonFilter is the ONE place a request's facet args become the domain filter, shared by the list
// and the stats endpoint. Sharing it is half of the no-drift contract (buildTaxonFilter in the
// adapter is the other half): the two endpoints must read the same arguments the same way, or the
// same URL means two different things depending on which surface renders it.
func taxonFilter(rank, parent, religion, classification *string) domain.TaxonFilter {
	return domain.TaxonFilter{
		Rank:           rank,
		Parent:         parent,
		Religion:       religion,
		Classification: classification,
	}
}

// selectTaxonFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
// (Every taxonomy facet is pii:none — the taxonomy is public reference data, and the pii:special
// surface in this module is person_affiliations, which has no facet at all.)
func selectTaxonFacets(ctx context.Context, s ReligionService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("taxon")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, religionapi.NewInvalid("taxon facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, religionapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPITaxonDistributions maps the assembled kernel result onto the wire type, carrying each bucket
// key VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)`
// keys included (which the console renders unlinked). Copied per module because Conjure generates the
// types per file — there is no shared type to map to.
func toAPITaxonDistributions(res stats.Result) []religionapi.FacetDistribution {
	out := make([]religionapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]religionapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := religionapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, religionapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
