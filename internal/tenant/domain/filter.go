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
	OrgID    string
	DomainID *string
	KindID   *string
	// Level is the EXACT-match arg listUnits has always shipped; LevelMin/LevelMax are the range the
	// facet's own args bind (M57 ticket 3). All three are ANDed, so a caller passing the superseded
	// scalar alongside a range gets the intersection rather than a silently ignored predicate.
	Level      *int
	LevelMin   *int
	LevelMax   *int
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
	for _, r := range []struct {
		arg string
		val *int
	}{{"level", f.Level}, {"levelMin", f.LevelMin}, {"levelMax", f.LevelMax}} {
		if r.val != nil && (*r.val < 0 || *r.val > 32767) {
			return fmt.Errorf("%w: %s is out of range", ErrInvalidUnit, r.arg)
		}
	}
	// An inverted range is a caller mistake, not an empty result: silently returning zero rows would
	// read as "there are none at that depth" rather than "that range is impossible".
	if f.LevelMin != nil && f.LevelMax != nil && *f.LevelMin > *f.LevelMax {
		return fmt.Errorf("%w: levelMin must not exceed levelMax", ErrInvalidUnit)
	}
	if err := validateUnitEnum("visibility", f.Visibility); err != nil {
		return err
	}
	return validateUnitEnum("state", f.State)
}

// OrgFilter is the organization facet vocabulary (M58 ticket 4 / D-ObjectFacets). Unlike UnitFilter
// there was no filter struct here at all before this ticket: listOrganizations took a bare *string
// domain, which is why the application-layer signature changes with it.
//
// Visibility NARROWS and never widens, exactly as on UnitFilter — but for a different reason. A
// shadow UNIT inside the subject's reach is genuinely visible; a shadow ORGANIZATION is visible to
// an instance admin and to nobody else, because a role assignment's target_unit_id FKs tenant_units
// and can never name an organization. Asking for visibility=shadow without instance-admin standing
// therefore yields an empty page rather than an error, and the dashboard's scoped arm is a flat
// `visibility = 'public'` rather than unit's reach predicate (see OrganizationStatsForSubject).
type OrgFilter struct {
	DomainID   *string
	Visibility *string
	State      *string
}

// Validate rejects a malformed filter with ErrInvalidOrg, which the transport maps to
// Tenant:OrganizationInvalid.
func (f OrgFilter) Validate() error {
	if f.DomainID != nil && !rid.IsRID(*f.DomainID) {
		return fmt.Errorf("%w: domain must be a RID", ErrInvalidOrg)
	}
	if err := validateFacetEnum("organization", "visibility", f.Visibility, ErrInvalidOrg); err != nil {
		return err
	}
	return validateFacetEnum("organization", "state", f.State, ErrInvalidOrg)
}

func validateUnitEnum(facetKey string, v *string) error {
	return validateFacetEnum("unit", facetKey, v, ErrInvalidUnit)
}

// validateFacetEnum checks one enum arg against the facet catalog's declared Values, so the CHECK set
// is written down in Go exactly once (in pkg/facet) rather than once per module filter. sentinel is
// the caller's own invalid-argument error, so each object type keeps its own Conjure error mapping.
func validateFacetEnum(objectType, facetKey string, v *string, sentinel error) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get(objectType)
	if !ok {
		return fmt.Errorf("%w: %s facets are not registered", sentinel, objectType)
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
		return fmt.Errorf("%w: %s must be one of %v", sentinel, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for %s", sentinel, facetKey, objectType)
}
