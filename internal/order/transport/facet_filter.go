// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"strings"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/order/domain"
)

// orderFilter assembles the listOrders query args into the one OrderFilter both list paths take
// (M56 ticket 3 / D-ObjectFacets). It performs only the parsing the wire types force — calendar
// bounds arrive as `YYYY-MM-DD` strings. Every VALUE check lives in OrderFilter.Validate, run once
// in the application layer, so the admin and read-scope paths reject identically.
func orderFilter(issuingUnitID, orderTypeID, status, issuedOnFrom, issuedOnTo *string) (domain.OrderFilter, error) {
	from, err := parseFacetDate("issuedOnFrom", issuedOnFrom)
	if err != nil {
		return domain.OrderFilter{}, err
	}
	to, err := parseFacetDate("issuedOnTo", issuedOnTo)
	if err != nil {
		return domain.OrderFilter{}, err
	}
	return domain.OrderFilter{
		IssuingUnitID: trimmedPtr(issuingUnitID),
		OrderTypeID:   trimmedPtr(orderTypeID),
		Status:        trimmedPtr(status),
		IssuedOnFrom:  from,
		IssuedOnTo:    to,
	}, nil
}

// parseFacetDate reads an ISO-8601 calendar date bound. A malformed value is ErrOrderInvalid (mapped
// to Order:OrderInvalid) rather than a 500 or a silently-ignored filter — silently dropping an
// unparseable bound would return MORE rows than the caller asked for, which is the dangerous
// direction for a filter to fail in.
func parseFacetDate(arg string, v *string) (*time.Time, error) {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil, nil
	}
	t, err := time.Parse(domain.ISODate, strings.TrimSpace(*v))
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be an ISO-8601 calendar date (YYYY-MM-DD)", domain.ErrOrderInvalid, arg)
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
