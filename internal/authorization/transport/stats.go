// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	authzapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/authorization"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// The assignment dashboard endpoint (M58 ticket 6 / D-ObjectFacets) — the second half of the facet
// vocabulary, over the same filter args `listAssignments` takes.

func (s Service) AssignmentStats(ctx context.Context, token bearertoken.Token, facets *string, subjectPersonID *string, targetUnitID *string, roleID *string, scope *string, graphID *string) (authzapi.AssignmentStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(domain.PermAssignmentRead)); err != nil {
		return authzapi.AssignmentStats{}, err
	}
	sel, err := selectAssignmentFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return authzapi.AssignmentStats{}, err
	}
	reader, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return authzapi.AssignmentStats{}, s.mapError(ctx, err)
	}
	// Both the reader AND the admin flag go down: stats.Compute owns the arm convention, so a
	// non-admin with no subject (a machine principal) reads nothing rather than everything.
	res, err := s.app.AssignmentStats(ctx,
		assignmentFilterFrom(subjectPersonID, targetUnitID, roleID, scope, graphID), reader, isAdmin, sel)
	if err != nil {
		return authzapi.AssignmentStats{}, s.mapError(ctx, err)
	}
	return authzapi.AssignmentStats{TotalCount: int(res.TotalCount), Facets: toAPIFacetDistributions(res)}, nil
}

// selectAssignmentFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectAssignmentFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("link__has_role")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, authzapi.NewAssignmentInvalid("assignment facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, authzapi.NewAssignmentInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIFacetDistributions maps the kernel result onto the generated wire types.
func toAPIFacetDistributions(res stats.Result) []authzapi.FacetDistribution {
	out := make([]authzapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]authzapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := authzapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, authzapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
