// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/palantir/pkg/bearertoken"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	tenantapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/tenant"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// UnitStats implements GET /tenant/v1/stats/units — the dashboard half of the unit facet vocabulary
// (M57 / D-ObjectFacets).
//
// Same gate and same filter parsing as ListUnits, so the two views describe one world. What differs
// is WHERE the shadow gate runs: the list trims its page afterwards (gateUnits), which cannot work
// for a count, so the subject is passed down and the gate is folded into SQL. An instance admin
// passes no subject and is counted against everything.
func (s Service) UnitStats(ctx context.Context, token bearertoken.Token, org string, facets *string, domainID *string, unitKind *string, level *int, levelMin *int, levelMax *int, visibility *string, state *string, pdpScoped *bool) (tenantapi.UnitStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermUnitRead)); err != nil {
		return tenantapi.UnitStats{}, err
	}
	filter := domain.UnitFilter{
		OrgID:      org,
		DomainID:   domainID,
		KindID:     unitKind,
		Level:      level,
		LevelMin:   levelMin,
		LevelMax:   levelMax,
		Visibility: visibility,
		State:      state,
		PDPScoped:  pdpScoped,
	}
	sel, err := selectUnitFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return tenantapi.UnitStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return tenantapi.UnitStats{}, s.mapError(ctx, err, errCtx{orgID: org})
	}
	// Both the subject AND the admin flag go down: stats.Compute owns the arm convention, so the
	// transport does not get to encode "empty subject means admin" — a non-admin with no subject reads
	// nothing rather than everything.
	res, err := s.app.UnitStats(ctx, subject, isAdmin, filter, sel)
	if err != nil {
		return tenantapi.UnitStats{}, s.mapError(ctx, err, errCtx{orgID: org})
	}
	return tenantapi.UnitStats{TotalCount: int(res.TotalCount), Facets: toAPIUnitDistributions(res)}, nil
}

// selectUnitFacets resolves the `facets` CSV against the catalog: an undeclared key is a caller
// error, a facet whose read code the caller lacks is silently omitted (D-ObjectFacets rule 2).
func selectUnitFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("unit")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, tenantapi.NewUnitInvalid("unit facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, tenantapi.NewUnitInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIUnitDistributions maps the assembled result onto the wire type, carrying each bucket key
// verbatim: it is what the caller passes back as a filter value.
func toAPIUnitDistributions(res stats.Result) []tenantapi.FacetDistribution {
	out := make([]tenantapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]tenantapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := tenantapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, tenantapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
