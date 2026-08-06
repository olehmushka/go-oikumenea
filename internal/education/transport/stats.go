// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	educationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/education"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// The institution dashboard endpoint (M58 ticket 5 / D-ObjectFacets) — the second half of the facet
// vocabulary, over the same filter args `listInstitutions` takes.

func (s EducationService) InstitutionStats(ctx context.Context, token bearertoken.Token, facets *string, query *string, kindID *string, countryID *string, foundedOnFrom *string, foundedOnTo *string, state *string) (educationapi.InstitutionStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.InstitutionStats{}, err
	}
	sel, err := selectInstitutionFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return educationapi.InstitutionStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return educationapi.InstitutionStats{}, s.mapError(ctx, err)
	}
	// Both the subject AND the admin flag go down: stats.Compute owns the arm convention, so a
	// non-admin with no subject (a machine principal) reads nothing rather than everything.
	res, err := s.app.InstitutionStats(ctx, subject, isAdmin,
		institutionFilterFrom(query, kindID, countryID, foundedOnFrom, foundedOnTo, state), sel)
	if err != nil {
		return educationapi.InstitutionStats{}, s.mapError(ctx, err)
	}
	return educationapi.InstitutionStats{TotalCount: int(res.TotalCount), Facets: toAPIFacetDistributions(res)}, nil
}

// institutionFilterFrom builds the institution facet filter from the raw query args. Shared by
// ListInstitutions and InstitutionStats so a list and its dashboard cannot read the same URL
// differently.
func institutionFilterFrom(query, kindID, countryID, foundedOnFrom, foundedOnTo, state *string) domain.InstitutionFilter {
	return domain.InstitutionFilter{
		Query:         strOr(query),
		KindID:        kindID,
		CountryID:     countryID,
		FoundedOnFrom: foundedOnFrom,
		FoundedOnTo:   foundedOnTo,
		State:         state,
	}
}

// selectInstitutionFacets resolves the `facets` CSV against the catalog: an undeclared key is a
// caller error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectInstitutionFacets(ctx context.Context, s EducationService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("institution")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, educationapi.NewInvalid("institution facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, educationapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// ============================ enrollment dashboard (M58 ticket 7) ============================

func (s EducationService) EnrollmentStats(ctx context.Context, token bearertoken.Token, facets *string, institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFromFrom, effectiveFromTo *string) (educationapi.EnrollmentStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EnrollmentStats{}, err
	}
	sel, err := selectEnrollmentFacets(ctx, s, token, strOr(facets))
	if err != nil {
		return educationapi.EnrollmentStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return educationapi.EnrollmentStats{}, s.mapError(ctx, err)
	}
	// Both the subject AND the admin flag go down: stats.Compute owns the arm convention, so a
	// non-admin with no subject (a machine principal) reads nothing rather than everything.
	res, err := s.app.EnrollmentStats(ctx, subject, isAdmin,
		enrollmentFilterFrom(institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFromFrom, effectiveFromTo), sel)
	if err != nil {
		return educationapi.EnrollmentStats{}, s.mapError(ctx, err)
	}
	return educationapi.EnrollmentStats{TotalCount: int(res.TotalCount), Facets: toAPIFacetDistributions(res)}, nil
}

// enrollmentFilterFrom builds the enrollment facet filter from the raw query args. Shared by
// ListEnrollments and EnrollmentStats so a list and its dashboard cannot read the same URL
// differently — the property the sqlc parity guard proves for the SQL half.
func enrollmentFilterFrom(institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFromFrom, effectiveFromTo *string) domain.EnrollmentFilter {
	return domain.EnrollmentFilter{
		InstitutionID:     institutionID,
		ProgramID:         programID,
		UnitID:            unitID,
		GroupID:           groupID,
		DegreeLevelID:     degreeLevelID,
		Status:            status,
		EffectiveFromFrom: effectiveFromFrom,
		EffectiveFromTo:   effectiveFromTo,
	}
}

// selectEnrollmentFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectEnrollmentFacets(ctx context.Context, s EducationService, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("link__studied_at")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, educationapi.NewInvalid("enrollment facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, educationapi.NewInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIFacetDistributions maps the kernel result onto the generated wire types.
func toAPIFacetDistributions(res stats.Result) []educationapi.FacetDistribution {
	out := make([]educationapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]educationapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := educationapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, educationapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
