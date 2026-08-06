// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
)

// membershipFilter assembles the listMemberships query args into the one MembershipFilter both list
// paths take (M56 ticket 3 / D-ObjectFacets). It performs only the parsing the wire types force —
// calendar bounds arrive as `YYYY-MM-DD` strings, per the contract's date convention (Conjure
// `datetime` is an instant and is deliberately not used for a calendar bound). Every VALUE check
// lives in MembershipFilter.Validate, run once in the application layer, so the admin and read-scope
// paths reject identically.
func membershipFilter(orgID, unitID, personID, positionID, status, effectiveFromAfter, effectiveFromBefore *string) (domain.MembershipFilter, error) {
	after, err := parseFacetDate("effectiveFromAfter", effectiveFromAfter)
	if err != nil {
		return domain.MembershipFilter{}, err
	}
	before, err := parseFacetDate("effectiveFromBefore", effectiveFromBefore)
	if err != nil {
		return domain.MembershipFilter{}, err
	}
	return domain.MembershipFilter{
		UnitID:              trimmedPtr(unitID),
		OrgID:               trimmedPtr(orgID),
		PersonID:            trimmedPtr(personID),
		PositionID:          trimmedPtr(positionID),
		Status:              trimmedPtr(status),
		EffectiveFromAfter:  after,
		EffectiveFromBefore: before,
	}, nil
}

// parseFacetDate reads an ISO-8601 calendar date bound. A malformed value is ErrMembershipInvalid
// (mapped to Membership:MembershipInvalid) rather than a 500 or a silently-ignored filter — silently
// dropping an unparseable bound would return MORE rows than the caller asked for, which is the
// dangerous direction for a filter to fail in.
func parseFacetDate(arg string, v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	t, err := time.Parse(domain.ISODate, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be an ISO-8601 calendar date (YYYY-MM-DD)", domain.ErrMembershipInvalid, arg)
	}
	return &t, nil
}

// trimmedPtr normalizes an optional string arg: absent, or blank after trimming, both mean "no
// filter". A blank arg must NOT become a filter for the empty string, which would match nothing and
// silently return an empty page.
func trimmedPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	if t == "" {
		return nil
	}
	return &t
}
