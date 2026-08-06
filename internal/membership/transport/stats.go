// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	membershipapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/membership"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
	"github.com/palantir/pkg/bearertoken"
)

// MembershipStats implements GET /membership/v1/stats/memberships — the dashboard half of the link__member_of facet
// vocabulary (M57 / D-ObjectFacets).
//
// It is the SAME request state as ListMemberships: the identical gate, the identical filter parsing and the
// identical arm dispatch, differing only in aggregating rather than paging. That is not tidiness — a
// dashboard computed over a different candidate set than its list is a chart that describes rows the
// list will not return.
func (s Service) MembershipStats(ctx context.Context, token bearertoken.Token, facets *string, orgID, unitID, personID, positionID, status, effectiveFromAfter, effectiveFromBefore *string) (membershipapi.MembershipStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermMembershipRead)); err != nil {
		return membershipapi.MembershipStats{}, err
	}
	filter, err := membershipFilter(orgID, unitID, personID, positionID, status, effectiveFromAfter, effectiveFromBefore)
	if err != nil {
		return membershipapi.MembershipStats{}, s.mapError(ctx, err, errCtx{})
	}
	sel, err := selectMembershipFacets(ctx, s, token, derefOr(facets, ""))
	if err != nil {
		return membershipapi.MembershipStats{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return membershipapi.MembershipStats{}, s.mapError(ctx, err, errCtx{})
	}
	res, err := s.app.MembershipStats(ctx, subject, isAdmin, filter, sel)
	if err != nil {
		return membershipapi.MembershipStats{}, s.mapError(ctx, err, errCtx{})
	}
	return membershipapi.MembershipStats{TotalCount: int(res.TotalCount), Facets: toAPIMembershipDistributions(res)}, nil
}

// selectMembershipFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
func selectMembershipFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("link__member_of")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, membershipapi.NewMembershipInvalid("link__member_of facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, membershipapi.NewMembershipInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIMembershipDistributions maps the assembled kernel result onto the wire type, carrying each bucket key
// VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)` keys
// included (which the console renders unlinked). Copied per module because Conjure generates the types
// per file — there is no shared type to map to.
func toAPIMembershipDistributions(res stats.Result) []membershipapi.FacetDistribution {
	out := make([]membershipapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]membershipapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := membershipapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, membershipapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
