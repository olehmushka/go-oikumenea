// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/olehmushka/go-oikumenea/internal/company/domain"
	companyapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/company"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// The company dashboard endpoint (M58 ticket 5 / D-ObjectFacets) — the second half of the facet
// vocabulary, over the same filter args `listCompanies` takes.

func (s CompanyService) CompanyStats(ctx context.Context, token bearertoken.Token, facets *string, query *string, legalForm *string, ownershipCategory *string, countryID *string, industryClass *string, foundedOnFrom *string, foundedOnTo *string, state *string) (companyapi.CompanyStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return companyapi.CompanyStats{}, err
	}
	sel, err := selectCompanyFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return companyapi.CompanyStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return companyapi.CompanyStats{}, s.mapError(ctx, err)
	}
	// Both the subject AND the admin flag go down: stats.Compute owns the arm convention, so a
	// non-admin with no subject (a machine principal) reads nothing rather than everything.
	res, err := s.app.CompanyStats(ctx, subject, isAdmin,
		companyFilterFrom(query, legalForm, ownershipCategory, countryID, industryClass, foundedOnFrom, foundedOnTo, state), sel)
	if err != nil {
		return companyapi.CompanyStats{}, s.mapError(ctx, err)
	}
	return companyapi.CompanyStats{TotalCount: int(res.TotalCount), Facets: toAPIFacetDistributions(res)}, nil
}

// companyFilterFrom builds the company facet filter from the raw query args. Shared by ListCompanies
// and CompanyStats so a list and its dashboard cannot read the same URL differently.
func companyFilterFrom(query, legalForm, ownershipCategory, countryID, industryClass, foundedOnFrom, foundedOnTo, state *string) domain.CompanyFilter {
	return domain.CompanyFilter{
		Query:             strOr(query),
		LegalFormID:       legalForm,
		OwnershipCategory: ownershipCategory,
		CountryID:         countryID,
		IndustryClassID:   industryClass,
		FoundedOnFrom:     foundedOnFrom,
		FoundedOnTo:       foundedOnTo,
		State:             state,
	}
}

// selectCompanyFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectCompanyFacets(ctx context.Context, s CompanyService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("company")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, companyapi.NewInvalid("company facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, companyapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIFacetDistributions maps the kernel result onto the generated wire types.
func toAPIFacetDistributions(res stats.Result) []companyapi.FacetDistribution {
	out := make([]companyapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]companyapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := companyapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, companyapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
