// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// DocumentFilter is the document facet vocabulary in Go (M56 ticket 3 / D-ObjectFacets), shared
// verbatim by both top-level list paths:
//
//   - the instance-admin path  — ListDocuments;
//   - the holder-scoped path   — ListDocumentsForSubject.
//
// Two paths, ONE struct, so they cannot drift: the same filter set must select the same documents,
// and the scoped path must be exactly the admin path intersected with the holders the subject may
// read (D-PersonReadScope). A no-DB narg-parity test and a DB differential test both hold that line.
//
// Nothing here names a pii:basic or pii:special column: `number`, `issuer` and the `attributes` bag
// are readable but NOT filterable (D-ObjectFacets rule 1 — asserted against the DDL in
// pkg/facet/plaintext_test.go).
type DocumentFilter struct {
	TypeID           *string
	Status           *string
	IssuingCountryID *string

	// The date bounds are INCLUSIVE. A document with an unset issued_on / no expiry is excluded
	// whenever the corresponding bound is set (SQL three-valued logic); the no-expiry set is a
	// distinct (unknown) bucket on M57's stats endpoint rather than a filterable value.
	IssuedOnFrom  *time.Time
	IssuedOnTo    *time.Time
	ExpiresOnFrom *time.Time
	ExpiresOnTo   *time.Time
}

// IsZero reports whether the filter constrains nothing.
func (f DocumentFilter) IsZero() bool {
	return f.TypeID == nil && f.Status == nil && f.IssuingCountryID == nil &&
		f.IssuedOnFrom == nil && f.IssuedOnTo == nil &&
		f.ExpiresOnFrom == nil && f.ExpiresOnTo == nil
}

// Validate rejects a malformed filter with ErrDocumentInvalid, which the transport maps to
// Document:DocumentInvalid. It runs ONCE, in the application layer, before the admin/scoped
// dispatch — so both paths reject identically, which is half of the no-drift contract.
//
// Enum values are checked against the facet catalog rather than a second copy of the CHECK set: the
// catalog is already proven against the DDL (pkg/facet plaintext_test.go).
func (f DocumentFilter) Validate() error {
	if err := validateFacetEnum("status", f.Status); err != nil {
		return err
	}
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"typeId", f.TypeID},
		{"issuingCountryId", f.IssuingCountryID},
	} {
		if r.val != nil && !rid.IsRID(*r.val) {
			return fmt.Errorf("%w: %s must be a RID", ErrDocumentInvalid, r.arg)
		}
	}
	for _, r := range []struct {
		lo, hi   *time.Time
		loN, hiN string
	}{
		{f.IssuedOnFrom, f.IssuedOnTo, "issuedOnFrom", "issuedOnTo"},
		{f.ExpiresOnFrom, f.ExpiresOnTo, "expiresOnFrom", "expiresOnTo"},
	} {
		if r.lo != nil && r.hi != nil && r.hi.Before(*r.lo) {
			return fmt.Errorf("%w: %s precedes %s", ErrDocumentInvalid, r.hiN, r.loN)
		}
	}
	return nil
}

// validateFacetEnum checks a value against the shipped facet's declared Values (the CHECK set in
// chart order). An unknown facet key is a programming error in this file, not a caller error.
func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("document")
	if !ok {
		return fmt.Errorf("%w: document facets are not registered", ErrDocumentInvalid)
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
		return fmt.Errorf("%w: %s must be one of %v", ErrDocumentInvalid, facetKey, f.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for document", ErrDocumentInvalid, facetKey)
}
