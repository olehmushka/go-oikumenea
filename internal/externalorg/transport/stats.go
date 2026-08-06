// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"
	"time"

	externalorgapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/externalorg"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// ExternalOrgStats implements GET /external-orgs/v1/stats/external-orgs — the dashboard half of the
// external-organization facet vocabulary (M58 ticket 2 / D-ObjectFacets).
//
// It is the SAME request state as ListExternalOrgs: the identical gate and the identical filter
// parsing, differing only in aggregating rather than paging. That is not tidiness — a dashboard
// computed over a different candidate set than its list is a chart describing rows the list will not
// return.
//
// No subject is resolved and no arm is picked, because there is no arm to pick: this table is flat
// instance-global reference data, and `externalorg.read` held anywhere — the same gate the list
// applies — is the whole visibility decision.
func (s ExternalOrganizationService) ExternalOrgStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	query *string,
	kind *string,
	country *string,
	status *string,
	source *string,
	confidence *string,
	asOfFrom *datetime.DateTime,
	asOfTo *datetime.DateTime,
) (externalorgapi.ExternalOrgStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return externalorgapi.ExternalOrgStats{}, err
	}
	sel, err := selectOrgFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return externalorgapi.ExternalOrgStats{}, err
	}
	res, err := s.app.OrgStats(ctx, strOr(query), orgFilter(kind, country, status, source, confidence, asOfFrom, asOfTo), sel)
	if err != nil {
		return externalorgapi.ExternalOrgStats{}, s.mapError(ctx, err)
	}
	return externalorgapi.ExternalOrgStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPIOrgDistributions(res),
	}, nil
}

// orgFilter is the ONE place a request's facet args become the domain filter, shared by the list and
// the stats endpoint. Sharing it is half of the no-drift contract (buildOrgFilter in the adapter is
// the other half): the two endpoints must read the same arguments the same way, or the same URL
// means two different things depending on which surface renders it.
func orgFilter(kind, country, status, source, confidence *string, asOfFrom, asOfTo *datetime.DateTime) domain.OrgFilter {
	return domain.OrgFilter{
		Kind:       kind,
		CountryID:  country,
		Status:     status,
		Source:     source,
		Confidence: confidence,
		AsOfFrom:   fromAPITime(asOfFrom),
		AsOfTo:     fromAPITime(asOfTo),
	}
}

func fromAPITime(t *datetime.DateTime) *time.Time {
	if t == nil {
		return nil
	}
	v := time.Time(*t)
	return &v
}

// selectOrgFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
// (Every external-org facet is pii:none, so the omission arm has no live case here.)
func selectOrgFacets(ctx context.Context, s ExternalOrganizationService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("external_organization")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, externalorgapi.NewInvalid("external-organization facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, externalorgapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIOrgDistributions maps the assembled kernel result onto the wire type, carrying each bucket key
// VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)` keys
// included (which the console renders unlinked). Copied per module because Conjure generates the
// types per file — there is no shared type to map to.
func toAPIOrgDistributions(res stats.Result) []externalorgapi.FacetDistribution {
	out := make([]externalorgapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]externalorgapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := externalorgapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, externalorgapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
