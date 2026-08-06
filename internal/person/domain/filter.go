// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/rid"
)

// PersonFilter is the ONE person facet vocabulary in Go (M56 / D-ObjectFacets), shared verbatim by
// the two directory list paths:
//
//   - the instance-admin path — person's own ListPersons / SearchPersons;
//   - the read-scope path — membership's VisiblePersonIDsForSubject* (D-PersonReadScope).
//
// Two paths, ONE struct, so they cannot drift: the same filter set must select the same people, and
// the scoped path must be exactly the admin path intersected with the subject's reach. A no-DB
// narg-parity test and a DB differential test both hold that line.
//
// Every predicate this struct carries is folded INTO the SQL, before the LIMIT (review-2026-07 R-06).
// A Go-side re-filter after the page is cut would produce a short page WITH a nextPageToken — the
// exact bug R-06's @query folding avoided — so nothing here may ever become a post-filter.
type PersonFilter struct {
	// Query is NOT a facet (pkg/facet classifies it ClassSearch): a free-text name/code substring
	// routed to a different plan shape (the trigram GIN bitmap scan, R-21's List/Search split). It
	// rides in this struct because it selects which visibility query runs, alongside the facets.
	Query string

	Sex            *string
	Status         *string
	BirthdateFrom  *time.Time
	BirthdateTo    *time.Time
	CountryOfBirth *string
	RankID         *string

	// UnitID is SUBTREE-EXPANDING: it matches an active membership in that unit or in any of its
	// closure descendants.
	UnitID *string
	// Graph narrows the UnitID expansion to one graph code. Empty = every authority-bearing graph,
	// the same closure set the read-scope predicate itself uses, so the filter can never widen beyond
	// what the subject may already read. Meaningless (and ignored) without UnitID.
	Graph string

	HasAccount *bool
}

// IsZero reports whether the filter constrains nothing — the plain-directory case.
func (f PersonFilter) IsZero() bool {
	return f.Query == "" && f.Sex == nil && f.Status == nil && f.BirthdateFrom == nil &&
		f.BirthdateTo == nil && f.CountryOfBirth == nil && f.RankID == nil && f.UnitID == nil &&
		f.Graph == "" && f.HasAccount == nil
}

// Validate rejects a malformed filter with ErrInvalid, which the transport maps to
// Person:PersonInvalid. It runs ONCE, in the application layer, before the admin/scoped dispatch —
// so both paths reject identically, which is half of the no-drift contract.
//
// Enum values are checked against the facet catalog rather than a second copy of the CHECK set: the
// catalog is already proven against the DDL (pkg/facet plaintext_test.go), so there is exactly one
// place a value list can be wrong.
func (f PersonFilter) Validate() error {
	if err := validateEnum("sex", f.Sex); err != nil {
		return err
	}
	if err := validateEnum("status", f.Status); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"countryOfBirth", f.CountryOfBirth},
		{"rankId", f.RankID},
		{"unitId", f.UnitID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrInvalid, r.arg)
		}
	}
	if f.BirthdateFrom != nil && f.BirthdateTo != nil && f.BirthdateTo.Before(*f.BirthdateFrom) {
		return fmt.Errorf("%w: birthdateTo precedes birthdateFrom", ErrInvalid)
	}
	if f.Graph != "" && f.UnitID == nil {
		return fmt.Errorf("%w: graph narrows the unitId subtree expansion and is meaningless without unitId", ErrInvalid)
	}
	return nil
}

// validateEnum checks a value against the shipped facet's declared Values (the CHECK set in chart
// order). An unknown facet key is a programming error in this file, not a caller error.
func validateEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("person")
	if !ok {
		return fmt.Errorf("%w: person facets are not registered", ErrInvalid)
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
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalid, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for person", ErrInvalid, facetKey)
}
