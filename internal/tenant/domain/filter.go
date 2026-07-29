// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// UnitFilter is the unit facet vocabulary (M56 / D-ObjectFacets): the structural predicates the FLAT
// unit listing accepts. Four of them (OrgID, DomainID, KindID, Level) retro-declare args listUnits
// already shipped; Visibility, State and PDPScoped are new.
//
// These apply to the flat listing only. The traversal modes (parent / rootsOnly) select a different
// query shape and ignore the filters, as the endpoint's contract has always said.
//
// Visibility NARROWS, it never widens: the shadow-visibility gate still trims the page after it is
// cut (gateUnits, correct for a list — a short page, never a skipped row). Asking for
// visibility=shadow without shadow reach therefore yields an empty page, not an error and not a leak.
type UnitFilter struct {
	// OrgID is REQUIRED — a fully-unscoped listing is rejected (D-TenantOrganizations, M40).
	OrgID      string
	DomainID   *string
	KindID     *string
	Level      *int
	Visibility *string
	State      *string
	PDPScoped  *bool
}

// Validate rejects a malformed filter with ErrInvalidUnit, which the transport maps to
// Tenant:UnitInvalid. Enum values are checked against the facet catalog, so there is exactly one
// place the CHECK set is written down in Go.
func (f UnitFilter) Validate() error {
	if f.OrgID == "" {
		return fmt.Errorf("%w: org is required (a fully-unscoped listing is rejected)", ErrInvalidUnit)
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"domain", f.DomainID},
		{"unitKind", f.KindID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrInvalidUnit, r.arg)
		}
	}
	if f.Level != nil && (*f.Level < 0 || *f.Level > 32767) {
		return fmt.Errorf("%w: level is out of range", ErrInvalidUnit)
	}
	if err := validateUnitEnum("visibility", f.Visibility); err != nil {
		return err
	}
	return validateUnitEnum("state", f.State)
}

func validateUnitEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("unit")
	if !ok {
		return fmt.Errorf("%w: unit facets are not registered", ErrInvalidUnit)
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
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalidUnit, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for unit", ErrInvalidUnit, facetKey)
}
