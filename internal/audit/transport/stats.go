// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"

	"github.com/olegamysk/go-oikumenea/internal/audit/application"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	auditapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/audit"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// AuditStats implements GET /audit/v1/stats/audit — the dashboard half of the audit facet vocabulary
// (M58 / D-ObjectFacets).
//
// It is the SAME request state as Query: the identical gate and the identical filter parsing,
// differing only in aggregating rather than paging. That is not tidiness — a dashboard computed over
// a different candidate set than its list is a chart describing rows the list will not return.
//
// Where the five M57 types resolve a subject and pick an arm, this resolves nothing: audit visibility
// is the row-level security policy on `audit_log`, which the request-pinned connection carries. The
// gate here is therefore exactly Query's — the coarse `audit.read` anywhere — and the row-level
// decision happens in the database, for the aggregate exactly as for the list.
func (s Service) AuditStats(
	ctx context.Context,
	token bearertoken.Token,
	facets *string,
	actorPersonID *string,
	actorType *auditapi.AuditActorType,
	targetType *string,
	targetID *string,
	unitID *string,
	action *string,
	outcome *auditapi.AuditOutcome,
	since *datetime.DateTime,
	until *datetime.DateTime,
) (auditapi.AuditStats, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermAuditRead)); err != nil {
		return auditapi.AuditStats{}, err
	}
	sel, err := selectAuditFacets(ctx, s, token, deref(facets))
	if err != nil {
		return auditapi.AuditStats{}, err
	}
	res, err := s.app.Stats(ctx, application.QueryParams{
		ActorPersonID: actorPersonID,
		ActorType:     fromAPIActorType(actorType),
		TargetType:    targetType,
		TargetID:      targetID,
		UnitID:        unitID,
		Action:        action,
		Outcome:       fromAPIOutcome(outcome),
		Since:         fromAPITime(since),
		Until:         fromAPITime(until),
	}, sel)
	if err != nil {
		return auditapi.AuditStats{}, mapError(ctx, err, "")
	}
	return auditapi.AuditStats{
		TotalCount: int(res.TotalCount),
		Facets:     toAPIAuditDistributions(res),
	}, nil
}

// selectAuditFacets resolves the `facets` CSV against the catalog. A facet gated on a read code the
// caller lacks is dropped here — omitted from the response, never a zeroed bucket and never a 403
// (D-ObjectFacets rule 2). An undeclared facet key IS a caller error: it is a typo, not a permission.
// (Every audit facet is pii:none today, so the omission arm has no live case here.)
func selectAuditFacets(ctx context.Context, s Service, token bearertoken.Token, csv string) (stats.Selection, error) {
	o, ok := facet.Default.Get("audit")
	if !ok { // unreachable past the boot-time MustBeBound; loud beats an empty dashboard
		return stats.Selection{}, auditapi.NewAuditQueryInvalid("audit facets are not registered")
	}
	sel, err := stats.Select(o, csv, func(code string) (bool, error) {
		return s.pep.AllowedAnywhere(ctx, token, code)
	})
	if err != nil {
		if errors.Is(err, stats.ErrUnknownFacet) {
			return stats.Selection{}, auditapi.NewAuditQueryInvalid(fmt.Sprintf("%v", err))
		}
		return stats.Selection{}, err
	}
	return sel, nil
}

// toAPIAuditDistributions maps the assembled kernel result onto the wire type, carrying each bucket
// key VERBATIM: it is what the caller passes back as a filter value, synthetic `(unknown)`/`(other)`
// keys included (which the console renders unlinked). Copied per module because Conjure generates the
// types per file — there is no shared type to map to.
func toAPIAuditDistributions(res stats.Result) []auditapi.FacetDistribution {
	out := make([]auditapi.FacetDistribution, 0, len(res.Distributions))
	for _, d := range res.Distributions {
		buckets := make([]auditapi.FacetBucket, 0, len(d.Buckets))
		for _, b := range d.Buckets {
			bucket := auditapi.FacetBucket{Key: b.Key, Count: int(b.Count)}
			if len(b.Label) > 0 {
				label := b.Label
				bucket.Label = &label
			}
			buckets = append(buckets, bucket)
		}
		out = append(out, auditapi.FacetDistribution{Facet: d.Facet, Buckets: buckets})
	}
	return out
}
