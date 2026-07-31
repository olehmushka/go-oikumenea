// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	documentapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/document"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// DocumentStats implements GET /document/v1/stats/documents — the dashboard half of the document facet
// vocabulary (M57 / D-ObjectFacets).
//
// It is the SAME request state as ListDocuments: the identical gate, the identical filter parsing and the
// identical arm dispatch, differing only in aggregating rather than paging. That is not tidiness — a
// dashboard computed over a different candidate set than its list is a chart that describes rows the
// list will not return.
func (s Service) DocumentStats(ctx context.Context, token bearertoken.Token, facets *string, typeID, status, issuingCountryID, issuedOnFrom, issuedOnTo, expiresOnFrom, expiresOnTo *string) (documentapi.DocumentStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermDocumentRead)); err != nil {
		return documentapi.DocumentStats{}, err
	}
	filter, err := documentFilter(typeID, status, issuingCountryID, issuedOnFrom, issuedOnTo, expiresOnFrom, expiresOnTo)
	if err != nil {
		return documentapi.DocumentStats{}, s.mapError(ctx, err, errCtx{})
	}
	sel, err := selectDocumentFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return documentapi.DocumentStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return documentapi.DocumentStats{}, s.mapError(ctx, err, errCtx{})
	}
	res, err := s.app.DocumentStats(ctx, subject, isAdmin, filter, sel)
	if err != nil {
		return documentapi.DocumentStats{}, s.mapError(ctx, err, errCtx{})
	}
	return documentapi.DocumentStats{TotalCount: int(res.TotalCount), Facets: toAPIDocumentDistributions(res)}, nil
}

// selectDocumentFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
func selectDocumentFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("document")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, documentapi.NewDocumentInvalid("document facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, documentapi.NewDocumentInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIDocumentDistributions maps the assembled kernel result onto the wire type, carrying each bucket key
// VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)` keys
// included (which the console renders unlinked). Copied per module because Conjure generates the types
// per file — there is no shared type to map to.
func toAPIDocumentDistributions(res stats.Result) []documentapi.FacetDistribution {
	out := make([]documentapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]documentapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := documentapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, documentapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
