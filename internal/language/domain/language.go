// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the language module's pure logic: the Languoid + WritingSystem registry entries
// and the Repository port it needs (overview.md layering). No I/O, no framework imports — only the
// standard library. Language owns the READ side of the Glottolog languoid forest + ISO-15924 writing
// systems (D-Languages, M18); the registry itself is written by the hermenea import pipeline
// (language-scheme / language-scripts), not here.
package domain

import (
	"context"
	"errors"
	"fmt"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

// ErrInvalidLanguoid is the languoid filter/facet sentinel (M58 ticket 4); the transport maps it to
// Language:LanguoidInvalid.
var ErrInvalidLanguoid = errors.New("invalid languoid filter")

// Languoid is one node in the Glottolog forest. ID is the RID (the reference key person/unit/locale
// links store); Code is the stable glottocode; optional fields fold to "" when absent.
type Languoid struct {
	ID          string
	Code        string
	Level       string
	Name        string
	ParentID    string
	HasChildren bool
	FamilyCode  string
	ISO639_3    string
	Macroarea   string
	Status      string
}

// WritingSystem is one ISO-15924 script. ID is the RID; Code is the ISO-15924 lookup code.
type WritingSystem struct {
	ID         string
	Code       string
	Name       string
	ScriptType string
}

// Filter narrows a languoid listing. Parent restricts to the immediate children of a languoid RID
// (one tree level); TopLevel restricts to the forest roots (no parent).
//
// The four FACET criteria (Level, Family, Macroarea, Status — M58 ticket 4 / D-ObjectFacets) are
// pointers: nil disables, and they are the only fields the dashboard aggregate shares with the list.
// The traversal, search and paging fields keep their empty-value sentinels — they are not facets, and
// no aggregate counts them.
type Filter struct {
	Level     *string
	Family    *string
	Macroarea *string
	Status    *string
	Parent    string
	TopLevel  bool
	Query     string
	// Limit is the page size, clamped upstream. It keeps its name although the WIRE arg was renamed
	// `limit` -> `pageSize` in M58 ticket 4: the rename was a contract decision, and propagating it
	// into the domain would churn the connector and dataimport callers for nothing.
	Limit int
	// After is a keyset cursor: when non-empty, only languoids whose code sorts strictly after it are
	// returned (the list is ordered by code). Empty disables the criterion (first page).
	After string
}

// Validate rejects a malformed filter with ErrInvalidLanguoid. Only the two CLOSED value sets are
// checked: `macroarea` and `family` name open code sets (no CHECK constraint, and a glottocode is
// whatever the imported catalog says), so an unknown value there is an empty result, not an error.
func (f Filter) Validate() error {
	if err := validateFacetEnum("level", f.Level); err != nil {
		return err
	}
	return validateFacetEnum("status", f.Status)
}

func validateFacetEnum(facetKey string, v *string) error {
	if v == nil {
		return nil
	}
	o, ok := facet.Default.Get("languoid")
	if !ok {
		return fmt.Errorf("%w: languoid facets are not registered", ErrInvalidLanguoid)
	}
	for _, fc := range o.Facets {
		if fc.Key != facetKey {
			continue
		}
		for _, allowed := range fc.Values {
			if *v == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: %s must be one of %v", ErrInvalidLanguoid, facetKey, fc.Values)
	}
	return fmt.Errorf("%w: no %q facet is declared for languoid", ErrInvalidLanguoid, facetKey)
}

// Repository is the language module's port: a read-only view of the languoid + writing-system registry.
type Repository interface {
	ListLanguoids(ctx context.Context, f Filter) ([]Languoid, error)
	// LanguoidStats is the dashboard aggregate over the same candidate set ListLanguoids pages
	// (M58 ticket 4 / D-ObjectFacets). One arm: the registry is instance-global reference data with no
	// visibility predicate to fold in, so no subject is passed.
	LanguoidStats(ctx context.Context, f Filter, sel stats.Selection) ([]stats.Group, error)
	GetLanguoid(ctx context.Context, id string) (Languoid, bool, error)
	ListWritingSystems(ctx context.Context) ([]WritingSystem, error)
}
