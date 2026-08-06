// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/rid"
)

// ISODate is the calendar-date layout every facet date bound is carried in on the wire. Conjure's
// `datetime` is an INSTANT and is deliberately not used for a calendar bound (the person module
// fixes the same convention).
const ISODate = "2006-01-02"

// MembershipFilter is the membership facet vocabulary in Go (M56 / D-ObjectFacets), shared verbatim
// by both top-level list paths:
//
//   - the instance-admin path — ListMemberships;
//   - the reach-scoped path   — ListMembershipsForSubject.
//
// Two paths, ONE struct, so they cannot drift: the same filter set must select the same memberships,
// and the scoped path must be exactly the admin path intersected with the subject's readable reach.
// A no-DB narg-parity test and a DB differential test both hold that line.
//
// Every predicate this struct carries is folded INTO the SQL, before the LIMIT (review-2026-07
// R-06). A Go-side re-filter after the page is cut would produce a short page WITH a nextPageToken,
// so nothing here may ever become a post-filter.
//
// NOTE the absent default: unlike ListMembersByUnit / ListMembershipsByPerson, which are hard-wired
// to status='active', a zero MembershipFilter selects EVERY status. A hidden default would make
// M57's totalCount disagree with its own status distribution.
type MembershipFilter struct {
	// UnitID matches the unit EXACTLY — it does NOT expand to the subtree, the opposite of
	// person.unitId. A membership names the one unit the person belongs to; expanding would
	// double-count a person against every ancestor.
	UnitID *string
	// OrgID matches the ORGANIZATION of the membership's unit, resolved through the RLS-exempt
	// authz_unit_org projection (M55, migration 0011) rather than tenant_units — which is RLS-FORCED,
	// so a semi-join into it from this module's queries would be trimmed by a policy written for unit
	// reads. It is what lets an org-scoped dashboard (the unit dashboard) draw a membership chart
	// without mixing organizations.
	OrgID      *string
	PersonID   *string
	PositionID *string
	Status     *string

	// EffectiveFromAfter / Before are INCLUSIVE calendar-date bounds on effective_from, which is a
	// timestamptz: the upper bound is compared against the END of the given day, so passing the same
	// date to both selects exactly that day.
	EffectiveFromAfter  *time.Time
	EffectiveFromBefore *time.Time
}

// IsZero reports whether the filter constrains nothing — the plain-listing case.
func (f MembershipFilter) IsZero() bool {
	return f.UnitID == nil && f.OrgID == nil && f.PersonID == nil && f.PositionID == nil && f.Status == nil &&
		f.EffectiveFromAfter == nil && f.EffectiveFromBefore == nil
}

// Validate rejects a malformed filter with ErrMembershipInvalid, which the transport maps to
// Membership:MembershipInvalid. It runs ONCE, in the application layer, before the admin/scoped
// dispatch — so both paths reject identically, which is half of the no-drift contract.
//
// Enum values are checked against the facet catalog rather than a second copy of the CHECK set: the
// catalog is already proven against the DDL (pkg/facet plaintext_test.go), so there is exactly one
// place a value list can be wrong.
func (f MembershipFilter) Validate() error {
	if err := validateFacetEnum("status", f.Status); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"unitId", f.UnitID},
		{"org", f.OrgID},
		{"personId", f.PersonID},
		{"positionId", f.PositionID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrMembershipInvalid, r.arg)
		}
	}
	if f.EffectiveFromAfter != nil && f.EffectiveFromBefore != nil &&
		f.EffectiveFromBefore.Before(*f.EffectiveFromAfter) {
		return fmt.Errorf("%w: effectiveFromBefore precedes effectiveFromAfter", ErrMembershipInvalid)
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). An unknown facet key is a programming error in this file, not a caller error.
func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("link__member_of")
	if !ok {
		return fmt.Errorf("%w: membership facets are not registered", ErrMembershipInvalid)
	}
	for _, f := range o.Facets {
		if f.Key != facetKey {
			continue
		}
		for _, allowed := range f.Values {
			if *v == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: %s must be one of %v", ErrMembershipInvalid, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for membership", ErrMembershipInvalid, facetKey)
}
