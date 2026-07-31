// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// OrgFilter is the external-organization facet vocabulary in Go (M58 ticket 2 / D-ObjectFacets),
// shared verbatim by the list path and the stats path.
//
// Unlike the M57 five, there is no admin/scoped pair to keep in step: `external_organizations` is a
// flat instance-global reference table with no row-level security and no unit reach, so
// `externalorg.read` held anywhere is the whole visibility decision and the aggregate ships ONE arm.
// The struct still exists for the OTHER half of the no-drift contract — the list and the dashboard
// must select the same rows from the same arguments, or a chart segment and a filter stop being the
// same act.
type OrgFilter struct {
	// Kind accepts EITHER a kind code or a kind RID. The arg predates the facet vocabulary and took a
	// code; a `kind` bucket's key is the kind's RID, and a bucket key must remain a usable filter
	// value, so the filter was widened rather than duplicated.
	Kind      *string
	CountryID *string
	Status    *string

	// The D-OverlayFoundation attribution set: who asserted this organization exists, and how sure
	// they were. Both are closed CHECK sets, validated against the facet catalog below.
	Source     *string
	Confidence *string

	// The bounds are INCLUSIVE. An organization with no as_of is excluded whenever either bound is
	// set (SQL three-valued logic); that set is a distinct (unknown) bucket on the stats endpoint
	// rather than a filterable value.
	AsOfFrom *time.Time
	AsOfTo   *time.Time
}

// IsZero reports whether the filter constrains nothing.
func (f OrgFilter) IsZero() bool {
	return f.Kind == nil && f.CountryID == nil && f.Status == nil &&
		f.Source == nil && f.Confidence == nil && f.AsOfFrom == nil && f.AsOfTo == nil
}

// Validate rejects a malformed filter with ErrInvalid, which the transport maps to
// ExternalOrg:Invalid. It runs ONCE, in the application layer, so the list and the stats path reject
// identically — the other half of the no-drift contract.
//
// Enum values are checked against the facet catalog rather than a second copy of the CHECK set: the
// catalog is already proven against the DDL (pkg/facet plaintext_test.go).
func (f OrgFilter) Validate() error {
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"status", f.Status},
		{"source", f.Source},
		{"confidence", f.Confidence},
	} {
		if err := validateFacetEnum(r.arg, r.val); err != nil {
			return err
		}
	}
	// `kind` is deliberately NOT RID-checked: it accepts a code too. `country` is RID-only and always
	// was, so a wrong shape there is a caller error rather than the other spelling.
	if f.CountryID != nil && !rid.IsRID(*f.CountryID) {
		return fmt.Errorf("%w: country must be a RID", ErrInvalid)
	}
	if f.AsOfFrom != nil && f.AsOfTo != nil && f.AsOfTo.Before(*f.AsOfFrom) {
		return fmt.Errorf("%w: asOfTo precedes asOfFrom", ErrInvalid)
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). An unknown facet key is a programming error in this file, not a caller error.
func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("external_organization")
	if !ok {
		return fmt.Errorf("external_organization is not a registered facet type")
	}
	for _, ft := range o.Facets {
		if ft.Key != facetKey {
			continue
		}
		for _, allowed := range ft.Values {
			if *v == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalid, facetKey, ft.Values)
	}
	return fmt.Errorf("%s is not a declared external_organization facet", facetKey)
}
