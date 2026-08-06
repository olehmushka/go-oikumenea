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
// `datetime` is an INSTANT and is deliberately not used for a calendar bound.
const ISODate = "2006-01-02"

// OrderFilter is the order facet vocabulary in Go (M56 ticket 3 / D-ObjectFacets), shared verbatim
// by both top-level list paths:
//
//   - the instance-admin path — ListOrders;
//   - the reach-scoped path   — ListOrdersForSubject.
//
// Two paths, ONE struct, so they cannot drift: the same filter set must select the same orders, and
// the scoped path must be exactly the admin path intersected with the subject's readable reach on
// the ISSUING unit. A no-DB narg-parity test and a DB differential test both hold that line.
//
// Every predicate is folded INTO the SQL, before the LIMIT (review-2026-07 R-06); nothing here may
// ever become a post-filter.
type OrderFilter struct {
	// IssuingUnitID matches the issuing unit EXACTLY — it does not expand to the subtree.
	IssuingUnitID *string
	// OrderTypeID matches an order with AT LEAST ONE item of this type: an order's effect lives on
	// its items (D-Orders), so this is an EXISTS semi-join, never a join.
	OrderTypeID *string
	Status      *string

	// IssuedOnFrom / To are INCLUSIVE calendar-date bounds. A DRAFT order has no issue date, so
	// setting either bound excludes drafts (SQL three-valued logic) — which is correct: a date-bounded
	// question is about issued orders.
	IssuedOnFrom *time.Time
	IssuedOnTo   *time.Time
}

// IsZero reports whether the filter constrains nothing.
func (f OrderFilter) IsZero() bool {
	return f.IssuingUnitID == nil && f.OrderTypeID == nil && f.Status == nil &&
		f.IssuedOnFrom == nil && f.IssuedOnTo == nil
}

// Validate rejects a malformed filter with ErrOrderInvalid, which the transport maps to
// Order:OrderInvalid. It runs ONCE, in the application layer, before the admin/scoped dispatch — so
// both paths reject identically, which is half of the no-drift contract.
//
// Enum values are checked against the facet catalog rather than a second copy of the CHECK set: the
// catalog is already proven against the DDL (pkg/facet plaintext_test.go).
func (f OrderFilter) Validate() error {
	if err := validateFacetEnum("status", f.Status); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"issuingUnitId", f.IssuingUnitID},
		{"orderTypeId", f.OrderTypeID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrOrderInvalid, r.arg)
		}
	}
	if f.IssuedOnFrom != nil && f.IssuedOnTo != nil && f.IssuedOnTo.Before(*f.IssuedOnFrom) {
		return fmt.Errorf("%w: issuedOnTo precedes issuedOnFrom", ErrOrderInvalid)
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). An unknown facet key is a programming error in this file, not a caller error.
func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("order")
	if !ok {
		return fmt.Errorf("%w: order facets are not registered", ErrOrderInvalid)
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
		return fmt.Errorf("%w: %s must be one of %v", ErrOrderInvalid, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for order", ErrOrderInvalid, facetKey)
}
