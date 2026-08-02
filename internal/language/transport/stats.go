// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	languageapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/language"
	"github.com/olegamysk/go-oikumenea/internal/language/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
	werror "github.com/palantir/witchcraft-go-error"
)

// LanguoidStats implements GET /language/v1/stats/languages — the dashboard half of the languoid
// facet vocabulary (M58 ticket 4 / D-ObjectFacets).
//
// Same gate and the same filter builder as ListLanguages, so the two views describe one world. There
// is no subject and no arm choice: the languoid registry is instance-global reference data, and
// `language.read` held anywhere is the whole visibility decision.
func (s Service) LanguoidStats(ctx context.Context, token bearertoken.Token, facets *string, level, family, macroarea, status, query *string) (languageapi.LanguoidStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermLanguageRead)); err != nil {
		return languageapi.LanguoidStats{}, err
	}
	sel, err := selectLanguoidFacets(ctx, s, token, deref(facets))
	if err != nil {
		return languageapi.LanguoidStats{}, err
	}
	res, err := s.app.LanguoidStats(ctx, languoidFilter(level, family, macroarea, status, query), sel)
	if err != nil {
		return languageapi.LanguoidStats{}, mapLanguoidError(ctx, err, "languoid stats failed")
	}
	return languageapi.LanguoidStats{TotalCount: int(res.TotalCount), Facets: toAPILanguoidDistributions(res)}, nil
}

// languoidFilter builds the languoid facet filter from the raw query args. Shared by ListLanguages and
// LanguoidStats so a list and its dashboard cannot read the same URL differently. The traversal and
// paging fields are the caller's to add: the aggregate has no use for either.
func languoidFilter(level, family, macroarea, status, query *string) domain.Filter {
	return domain.Filter{
		Level:     level,
		Family:    family,
		Macroarea: macroarea,
		Status:    status,
		Query:     deref(query),
	}
}

// mapLanguoidError turns a domain sentinel into its Conjure error. Only one sentinel exists: a
// malformed facet value is the caller's mistake and must be a 400, not the 500 an unmapped error
// becomes (the mapError-drift lesson from the person/rank transports).
func mapLanguoidError(ctx context.Context, err error, msg string) error {
	if errors.Is(err, domain.ErrInvalidLanguoid) {
		return languageapi.NewLanguoidInvalid(err.Error())
	}
	return werror.WrapWithContextParams(ctx, err, msg)
}

// selectLanguoidFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectLanguoidFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("languoid")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, languageapi.NewLanguoidInvalid("languoid facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, languageapi.NewLanguoidInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPILanguoidDistributions maps the assembled result onto the wire type, carrying each bucket key
// verbatim: it is what the caller passes back as a filter value. No bucket carries a label — the type
// declares no ref facet.
func toAPILanguoidDistributions(res stats.Result) []languageapi.FacetDistribution {
	out := make([]languageapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]languageapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := languageapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, languageapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
