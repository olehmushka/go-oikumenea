// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
)

// TaxonFilter is the taxonomy facet vocabulary in Go (M58 ticket 2 / D-ObjectFacets), shared verbatim
// by the list path and the stats path.
//
// There is no admin/scoped pair to keep in step: the taxonomy is flat instance-global reference data
// (the row-level security in this module is on the unit-scoped religion_org_* tables, not here), so
// `religion.read` held anywhere is the whole visibility decision. The struct exists for the OTHER
// half of the no-drift contract — the list and the dashboard must select the same taxa from the same
// arguments, or a chart segment and a filter stop being the same act.
//
// Three of the four fields accept EITHER a code or a RID. That is not laxity: a bucket key must
// remain a usable filter value, the bucket keys here are RIDs, and these args predate the vocabulary
// carrying codes. Widening was the alternative to a second arg meaning almost the same thing.
type TaxonFilter struct {
	// Rank matches religion_taxa.rank_id, by the rank's code or its RID.
	Rank *string
	// Parent restricts to PROPER descendants of the given taxon through the closure (depth > 0 — the
	// taxon itself is excluded). RID only; it always was. This is the `subtree` facet's click-through,
	// and the depth > 0 on both sides is what makes a bucket's count equal the rows it returns.
	Parent *string
	// Religion restricts to one root religion taxon, by code or RID.
	Religion *string
	// Classification restricts to taxa whose EFFECTIVE theism tag set includes this one — declared on
	// the taxon or inherited from its nearest declaring ancestor. By code or RID.
	Classification *string
}

// IsZero reports whether the filter constrains nothing.
func (f TaxonFilter) IsZero() bool {
	return f.Rank == nil && f.Parent == nil && f.Religion == nil && f.Classification == nil
}

// Validate rejects a malformed filter with ErrInvalid, which the transport maps to Religion:Invalid.
// It runs ONCE, in the application layer, so the list and the stats path reject identically.
//
// There is deliberately little to check: every field is a free-form code-or-RID resolved by a join,
// so an unknown value selects nothing rather than erroring — the same behaviour listTaxa has had
// since M22. The one real rule is the empty string: without it, a present-but-empty arg would mean
// "match a taxon whose code is empty" rather than "no filter".
func (f TaxonFilter) Validate() error {
	for _, r := range []struct {
		arg string
		val *string
	}{
		{"rank", f.Rank},
		{"parent", f.Parent},
		{"religion", f.Religion},
		{"classification", f.Classification},
	} {
		if r.val != nil && *r.val == "" {
			return fmt.Errorf("%w: %s is present but empty", ErrInvalid, r.arg)
		}
	}
	return nil
}
