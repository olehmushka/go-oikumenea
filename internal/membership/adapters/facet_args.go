// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/membership/domain"
	persondomain "github.com/olehmushka/go-oikumenea/internal/person/domain"
)

// facetArgs binds the PERSON facet block (M56 / D-ObjectFacets) to pgx wire types for membership's
// three visibility queries. Person owns the vocabulary; membership consumes it, the same direction
// review R-06 already established when it folded the @query predicate into this module's SQL.
//
// Why the filter belongs here at all: the predicates must run INSIDE the visibility query, before
// the LIMIT. Applying them in Go after the page is cut would return a page shorter than pageSize
// while still handing back a nextPageToken — the failure R-06 exists to prevent.
//
// The block is byte-identical to person's own admin queries. The SQL narg-parity test proves that
// mechanically across all five queries; this mapper is its Go-side twin.
type facetArgs struct {
	sex            pgtype.Text
	status         pgtype.Text
	birthdateFrom  pgtype.Date
	birthdateTo    pgtype.Date
	countryOfBirth pgtype.Text
	rankID         pgtype.Text
	hasAccount     pgtype.Bool
	unitID         pgtype.Text
	graph          pgtype.Text
}

// personFacetArgs projects a validated PersonFilter onto its wire types. Query is excluded: it picks
// WHICH visibility query runs (the R-21 List/Search split), it is not part of the facet block.
func personFacetArgs(f persondomain.PersonFilter) facetArgs {
	return facetArgs{
		sex:            facetText(f.Sex),
		status:         facetText(f.Status),
		birthdateFrom:  facetDate(f.BirthdateFrom),
		birthdateTo:    facetDate(f.BirthdateTo),
		countryOfBirth: facetText(f.CountryOfBirth),
		rankID:         facetText(f.RankID),
		hasAccount:     facetBool(f.HasAccount),
		unitID:         facetText(f.UnitID),
		graph:          facetTextEmptyNull(f.Graph),
	}
}

func facetText(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func facetDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// facetBool keeps FALSE a real filter value — hasAccount=false selects the account-less half of the
// directory — which is why the parameter is nullable rather than defaulted.
func facetBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func facetTextEmptyNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// ── membership's own facets (M56 ticket 3) ──────────────────────────────────
//
// Distinct from the person block above: that one is person's vocabulary consumed by membership's
// visibility queries; this one is membership's OWN vocabulary for the top-level GET /memberships.
// Both live here because both bind through the same helpers, and a second copy of facetDate is
// exactly the drift the shared file avoids.
type membershipFacetArgsT struct {
	unitID              pgtype.Text
	orgID               pgtype.Text
	personID            pgtype.Text
	positionID          pgtype.Text
	status              pgtype.Text
	effectiveFromAfter  pgtype.Date
	effectiveFromBefore pgtype.Date
}

// membershipFacetArgs projects a validated MembershipFilter onto its wire types — ONE mapping used
// by BOTH list arms, so the admin and reach-scoped paths cannot bind a facet differently.
func membershipFacetArgs(f domain.MembershipFilter) membershipFacetArgsT {
	return membershipFacetArgsT{
		unitID:              facetText(f.UnitID),
		orgID:               facetText(f.OrgID),
		personID:            facetText(f.PersonID),
		positionID:          facetText(f.PositionID),
		status:              facetText(f.Status),
		effectiveFromAfter:  facetDate(f.EffectiveFromAfter),
		effectiveFromBefore: facetDate(f.EffectiveFromBefore),
	}
}
