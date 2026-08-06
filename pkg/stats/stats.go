// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package stats is the module-dashboard kernel (M57 / D-ObjectFacets): the half of a
// GET /<module>/v1/<collection>/stats response that is NOT SQL.
//
// A stats endpoint does exactly two things this package owns:
//
//   - SELECTION — turn the optional `facets` CSV into the set of facets this request computes,
//     dropping any whose inherited read code the caller lacks (D-ObjectFacets rule 2: OMITTED, never
//     a zeroed bucket and never a 403 — the D-UnifiedSearch skip-the-provider behaviour). The
//     selection is what the module's query reads as its per-facet `want_*` flags, so an unselected
//     facet is not merely hidden: it is never grouped.
//   - ASSEMBLY — turn the raw (facet, key, count) rows the GROUP BY returns into the declared
//     buckets: enum values zero-filled in CHART order, numeric/date values assigned to the declared
//     bands, top-N ref buckets ordered (by a SQL-supplied ordinal where the scheme has one — rank
//     seniority — else by count), and NULLs surfaced as the mandatory `(unknown)` bucket.
//
// What it deliberately does NOT own: counting. Every count arrives pre-computed from SQL, INSIDE the
// visibility predicate (D-ObjectFacets rule 3) — nothing here may ever filter, drop or re-weight a
// row, because a Go-side trim after the aggregate is exactly the `gateUnits`-trims-the-page mistake
// that is right for a list and wrong for a count. The one invariant every assembly rule preserves:
// EVERY counted row lands in exactly one bucket (an unexpected enum value is appended, not dropped;
// an out-of-band number goes to `(other)`), so a facet over the listed table's own column always
// sums to totalCount.
//
// stdlib-only, plus pkg/facet (itself a leaf), so transports, application services and adapters can
// all import it.
package stats

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
)

// Labeler resolves a batch of ONE object type's RIDs to display names, each a locale→text map
// (D-i18n: all locales in every response). It is the D-LinkTraversal labeler seam applied to bucket
// keys, and is wired at the composition root from the same per-type resolvers the links service
// uses, so a unit is named identically in a graph row and in a chart segment.
type Labeler func(ctx context.Context, objectType string, ids []string) (map[string]map[string]string, error)

// Label fills in every ref bucket's display name. Best effort in the same sense as the traversal
// labelers: a nil Labeler, an unregistered type or an unresolved id leaves the RID key standing, and
// the client falls back to the RID tail — a dashboard is never withheld because a name is missing.
func Label(ctx context.Context, l Labeler, sel Selection, res *Result) error {
	if l == nil {
		return nil
	}
	ids := res.RefIDs(sel)
	if len(ids) == 0 {
		return nil
	}
	types := make([]string, 0, len(ids))
	for t := range ids {
		types = append(types, t)
	}
	sort.Strings(types) // deterministic call order, so a failure is reproducible
	out := make(map[string]map[string]map[string]string, len(types))
	for _, t := range types {
		m, err := l(ctx, t, ids[t])
		if err != nil {
			return err
		}
		out[t] = m
	}
	res.ApplyLabels(sel, out)
	return nil
}

// The three synthetic bucket keys. They are parenthesized so they cannot collide with a real enum
// value or RID, and they are constants here because both the SQL (which emits `(other)` for the
// collapsed top-N tail) and the console (which must not turn one into a filter link) name them.
const (
	// BucketUnknown holds the rows whose facet column is NULL. Mandatory for a nullable column —
	// facet.Buckets.IncludeUnknown is enforced against the DDL by pkg/facet's plaintext guard.
	BucketUnknown = "(unknown)"
	// BucketOther holds a top-N facet's collapsed tail, and any value that fits no declared band.
	BucketOther = "(other)"
	// TotalFacet is the pseudo-facet the total-count row arrives under, so one query returns the
	// whole dashboard as one row set.
	TotalFacet = "(total)"
)

// Group is one raw aggregate row: which facet it belongs to, the raw group key (nil = SQL NULL) and
// the count. Ord is an optional chart-order key the query supplies when the bucket set has an
// inherent order that is not "by count" — rank seniority is the reason it exists (an ordered scheme
// re-sorted by frequency destroys the only ordering that means anything).
type Group struct {
	Facet string
	Key   *string
	Count int64
	Ord   *int64
}

// Bucket is one assembled bucket. Label is the locale→text display name (D-i18n: all locales in
// every response), filled for ref buckets by the labeler seam; nil for enum/band/bool buckets, whose
// keys are stable codes the console translates itself.
type Bucket struct {
	Key   string
	Label map[string]string
	Count int64
}

// Distribution is one facet's buckets, in chart order.
type Distribution struct {
	Facet   string
	Buckets []Bucket
}

// Result is the assembled response body.
type Result struct {
	TotalCount    int64
	Distributions []Distribution
}

// ErrUnknownFacet is returned when the `facets` CSV names something the object type does not
// declare. It is a caller error (400), unlike a facet the caller may not READ, which is silently
// omitted.
var ErrUnknownFacet = errors.New("unknown facet")

// Selection is the set of facets one stats request computes, in catalog order.
type Selection struct {
	obj      facet.ObjectType
	selected []facet.Facet
	set      map[string]bool
}

// Select resolves the optional `facets` CSV against an object type's declared vocabulary.
//
// An ABSENT or empty CSV means "every facet I may read" — a dashboard wants the whole set, and
// making the common case explicit would put the vocabulary in the client. A named facet the object
// type does not declare is ErrUnknownFacet. A facet whose inherited read code `holds` denies is
// dropped from the selection WHETHER OR NOT it was named explicitly: rule 2 says omitted, never a
// 403, so asking for a facet you cannot read is not an error — the facet is simply absent, exactly
// as if you had not asked.
//
// holds may be nil, meaning "every read code is held" (the instance-admin case); it is called at
// most once per distinct read code.
func Select(o facet.ObjectType, csv string, holds func(readPermission string) (bool, error)) (Selection, error) {
	sel := Selection{obj: o, set: map[string]bool{}}
	want := map[string]bool{}
	named := strings.TrimSpace(csv) != ""
	if named {
		declared := map[string]bool{}
		for _, f := range o.Facets {
			declared[f.Key] = true
		}
		for _, raw := range strings.Split(csv, ",") {
			k := strings.TrimSpace(raw)
			if k == "" {
				continue
			}
			if !declared[k] {
				return Selection{}, fmt.Errorf("%w: %s declares no facet %q", ErrUnknownFacet, o.Type, k)
			}
			want[k] = true
		}
		if len(want) == 0 { // "facets=,," — a request for the total alone, not for everything
			return sel, nil
		}
	}
	decided := map[string]bool{}
	for _, f := range o.Facets {
		if named && !want[f.Key] {
			continue
		}
		if f.ReadPermission != "" && holds != nil {
			ok, seen := decided[f.ReadPermission]
			if !seen {
				var err error
				ok, err = holds(f.ReadPermission)
				if err != nil {
					return Selection{}, err
				}
				decided[f.ReadPermission] = ok
			}
			if !ok {
				continue
			}
		}
		sel.selected = append(sel.selected, f)
		sel.set[f.Key] = true
	}
	return sel, nil
}

// Wants reports whether a facet is in the selection — the flag a module's stats query reads to skip
// (not merely hide) an unselected facet's GROUP BY branch.
func (s Selection) Wants(key string) bool { return s.set[key] }

// Facets returns the selected facets in catalog order.
func (s Selection) Facets() []facet.Facet { return s.selected }

// TopN is the bucket cutoff every selected ref facet shares — the single `top_n` a module's stats
// query binds across its ref branches. Zero when no ref facet is selected, in which case the
// parameter is bound and unused. pkg/facet refuses a type whose ref facets disagree, so "shared" is
// an invariant here rather than an assumption.
func (s Selection) TopN() int {
	// Every TOP-N facet, not just the ref ones: M58's `code` kind ranks an open value set exactly as a
	// ref ranks RIDs, and a selection of code facets alone (audit's action + targetType, with no ref
	// among them) would otherwise bind top_n = 0 and collapse EVERY bucket into `(other)` — a chart
	// that is not empty, not an error, and entirely wrong. Register already holds one TopN per type,
	// so the first is the type's.
	for _, f := range s.selected {
		if f.Buckets.Strategy == facet.StrategyTopN {
			return f.Buckets.TopN
		}
	}
	return 0
}

// RefIDs returns, per ref facet's target object token, the RID bucket keys that need a label —
// synthetic keys excluded. The composition root's labelers are keyed by that same token.
func (r Result) RefIDs(sel Selection) map[string][]string {
	byKey := map[string]facet.Facet{}
	for _, f := range sel.selected {
		byKey[f.Key] = f
	}
	out := map[string][]string{}
	for _, d := range r.Distributions {
		f, ok := byKey[d.Facet]
		if !ok || f.Kind != facet.KindRef {
			continue
		}
		for _, b := range d.Buckets {
			if b.Key == BucketUnknown || b.Key == BucketOther {
				continue
			}
			out[f.RefType] = append(out[f.RefType], b.Key)
		}
	}
	return out
}

// ApplyLabels attaches the resolved locale→text names to every ref bucket, keyed by target token.
// Best effort, matching the D-LinkTraversal labeler seam: an id with no label keeps its RID key and
// the client falls back to the RID tail.
func (r *Result) ApplyLabels(sel Selection, labels map[string]map[string]map[string]string) {
	byKey := map[string]facet.Facet{}
	for _, f := range sel.selected {
		byKey[f.Key] = f
	}
	for i := range r.Distributions {
		f, ok := byKey[r.Distributions[i].Facet]
		if !ok || f.Kind != facet.KindRef {
			continue
		}
		byID := labels[f.RefType]
		if byID == nil {
			continue
		}
		for j := range r.Distributions[i].Buckets {
			if m, ok := byID[r.Distributions[i].Buckets[j].Key]; ok {
				r.Distributions[i].Buckets[j].Label = m
			}
		}
	}
}

// Compute is the whole non-SQL body of a module's stats endpoint: pick the arm, run the module's
// aggregate, assemble the buckets, attach the labels. Every stats endpoint goes through it so the ARM
// CONVENTION is written down once.
//
// The convention, and the reason it is not left to each transport: `fetch` receives the subject whose
// visibility predicate the SQL must apply, and the EMPTY STRING means "no predicate at all — the
// instance-admin arm". Those two facts make one mistake possible and expensive: a NON-admin whose
// subject is empty would be handed the whole instance's counts. That is reachable in principle —
// pep.SubjectAuthority returns ("", false) for a machine subject (M51: a principal has no person
// identity and no reach) — so the guard belongs here rather than in five transports, where four of
// them would be one edit away from the leak. A non-admin with no subject reads nothing.
func Compute(
	ctx context.Context,
	l Labeler,
	sel Selection,
	isAdmin bool,
	subject string,
	fetch func(subject string) ([]Group, error),
) (Result, error) {
	arm := subject
	switch {
	case isAdmin:
		arm = "" // the admin arm carries no visibility predicate
	case subject == "":
		return Result{}, nil // no identity, no reach, no counts — never the admin arm
	}
	groups, err := fetch(arm)
	if err != nil {
		return Result{}, err
	}
	res := Assemble(sel, groups)
	if err := Label(ctx, l, sel, &res); err != nil {
		return Result{}, err
	}
	return res, nil
}

// Assemble turns the raw aggregate rows into the declared buckets, in catalog order. Rows naming a
// facet outside the selection are ignored (a query branch that ran when it should not have is a bug
// the guard tests catch, not something to surface mid-response); the total arrives under TotalFacet.
func Assemble(sel Selection, groups []Group) Result {
	byFacet := map[string][]Group{}
	var res Result
	for _, g := range groups {
		if g.Facet == TotalFacet {
			res.TotalCount += g.Count
			continue
		}
		byFacet[g.Facet] = append(byFacet[g.Facet], g)
	}
	for _, f := range sel.selected {
		res.Distributions = append(res.Distributions, Distribution{
			Facet:   f.Key,
			Buckets: bucketsFor(f, byFacet[f.Key]),
		})
	}
	return res
}

func bucketsFor(f facet.Facet, groups []Group) []Bucket {
	switch f.Buckets.Strategy {
	case facet.StrategyIdentity:
		return identityBuckets(f, groups)
	case facet.StrategyBool:
		return boolBuckets(f, groups)
	case facet.StrategyBands:
		return bandBuckets(f, groups)
	case facet.StrategyDateTrunc:
		return chronologicalBuckets(f, groups)
	case facet.StrategyTopN:
		return topNBuckets(f, groups)
	case facet.StrategyCatalog:
		return catalogBuckets(f, groups)
	default:
		return []Bucket{}
	}
}

// catalogBuckets orders a ref facet's buckets by the CATALOG's own ordinal and emits no `(other)` —
// the two things that separate it from topN (M58 ticket 7).
//
// Where topNBuckets prefers an ordinal when every row happens to carry one, here the ordinal is the
// whole point, so a row arriving without one is a query that failed to join its catalog: it sorts
// last rather than silently re-sorting the scale by frequency, which is the exact failure the
// strategy exists to prevent.
//
// The ZERO-COUNT levels come from the SQL, not from here: this package cannot enumerate a catalog it
// has never read, so the module's aggregate LEFT JOINs CatalogTable and a guard asserts it does. That
// is why there is no zero-fill loop in the shape identityBuckets has — the declared value set for an
// enum lives in the catalog declaration, and for this strategy it lives in a table.
func catalogBuckets(f facet.Facet, groups []Group) []Bucket {
	type row struct {
		key   string
		ord   *int64
		count int64
	}
	var rows []row
	var unknown int64
	var sawUnknown bool
	for _, g := range groups {
		switch {
		case g.Key == nil, g.Key != nil && *g.Key == BucketUnknown:
			unknown += g.Count
			sawUnknown = true
		default:
			rows = append(rows, row{key: *g.Key, ord: g.Ord, count: g.Count})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		switch {
		case rows[i].ord == nil && rows[j].ord == nil:
			return rows[i].key < rows[j].key
		case rows[i].ord == nil:
			return false // an unordered row is a join that did not happen — pin it after the scale
		case rows[j].ord == nil:
			return true
		case *rows[i].ord != *rows[j].ord:
			return *rows[i].ord < *rows[j].ord
		default:
			return rows[i].key < rows[j].key
		}
	})
	out := make([]Bucket, 0, len(rows)+1)
	for _, r := range rows {
		out = append(out, Bucket{Key: r.key, Count: r.count})
	}
	if sawUnknown || f.Buckets.IncludeUnknown {
		out = append(out, Bucket{Key: BucketUnknown, Count: unknown})
	}
	return out
}

// identityBuckets emits one bucket per declared value, IN CHART ORDER and zero-filled, so a chart's
// shape is stable across filterings. A value the CHECK set does not declare is appended rather than
// dropped: the catalog being stale must show up as an odd bar, never as a total that disagrees with
// its own distribution.
func identityBuckets(f facet.Facet, groups []Group) []Bucket {
	counts, unknown, extra := indexGroups(groups)
	out := make([]Bucket, 0, len(f.Values)+len(extra)+1)
	for _, v := range f.Values {
		out = append(out, Bucket{Key: v, Count: counts[v]})
	}
	declared := map[string]bool{}
	for _, v := range f.Values {
		declared[v] = true
	}
	for _, k := range extra {
		if !declared[k] {
			out = append(out, Bucket{Key: k, Count: counts[k]})
		}
	}
	return appendUnknown(out, f, unknown)
}

func boolBuckets(f facet.Facet, groups []Group) []Bucket {
	counts, unknown, _ := indexGroups(groups)
	out := []Bucket{{Key: "true", Count: counts["true"]}, {Key: "false", Count: counts["false"]}}
	return appendUnknown(out, f, unknown)
}

// bandBuckets assigns each raw numeric key (an age in whole years, a unit level) to the first
// half-open band [Lo, Hi) that contains it, emitting the bands in declared order and zero-filled. A
// value outside every band lands in `(other)` rather than vanishing.
func bandBuckets(f facet.Facet, groups []Group) []Bucket {
	bands := f.Buckets.Bands
	counts := make([]int64, len(bands))
	var unknown, other int64
	var sawUnknown bool
	for _, g := range groups {
		if g.Key == nil {
			unknown += g.Count
			sawUnknown = true
			continue
		}
		n, err := strconv.ParseInt(strings.TrimSpace(*g.Key), 10, 64)
		if err != nil {
			other += g.Count
			continue
		}
		idx := -1
		for i, b := range bands {
			if (b.Lo == nil || n >= int64(*b.Lo)) && (b.Hi == nil || n < int64(*b.Hi)) {
				idx = i
				break
			}
		}
		if idx < 0 {
			other += g.Count
			continue
		}
		counts[idx] += g.Count
	}
	out := make([]Bucket, 0, len(bands)+2)
	for i, b := range bands {
		out = append(out, Bucket{Key: b.Key, Count: counts[i]})
	}
	if other > 0 {
		out = append(out, Bucket{Key: BucketOther, Count: other})
	}
	if sawUnknown || f.Buckets.IncludeUnknown {
		out = append(out, Bucket{Key: BucketUnknown, Count: unknown})
	}
	return out
}

// chronologicalBuckets emits date_trunc keys in ascending time order (ISO-8601 keys sort
// lexicographically), with no zero-fill: a histogram of what happened has no declared value set, and
// inventing empty months between the extremes is the chart's job, not the API's.
//
// The `(unknown)` bucket, however, IS declared, and follows the same rule as every other strategy: a
// facet over a nullable column always emits it. That matters concretely here — an order register with
// no drafts, or a document register where everything expires, would otherwise drop the bucket and
// change the chart's shape, when the honest answer is a zero. Whether the population is empty is data;
// whether the bucket exists is the catalog's decision.
func chronologicalBuckets(f facet.Facet, groups []Group) []Bucket {
	counts, unknown, keys := indexGroups(groups)
	sort.Strings(keys)
	out := make([]Bucket, 0, len(keys)+1)
	for _, k := range keys {
		out = append(out, Bucket{Key: k, Count: counts[k]})
	}
	return appendUnknown(out, f, unknown)
}

// topNBuckets orders a ref facet's buckets. The SQL has already collapsed the tail into `(other)`
// (the tail sum cannot be computed without grouping everything, so it is computed where the rows
// are); this decides the ORDER, and pins the two synthetic buckets last.
//
// When every real bucket carries an Ord the query supplied, that ordinal wins over the counts: a
// rank distribution is read as a seniority profile, and sorting it by frequency would destroy the
// only ordering that means anything (facets.md ④). Otherwise it is descending by count, with the key
// as a stable tiebreak so a page is reproducible.
func topNBuckets(f facet.Facet, groups []Group) []Bucket {
	type row struct {
		key   string
		ord   *int64
		count int64
	}
	var rows []row
	var other, unknown int64
	var sawUnknown bool
	for _, g := range groups {
		switch {
		case g.Key == nil:
			unknown += g.Count
			sawUnknown = true
		case *g.Key == BucketOther:
			other += g.Count
		case *g.Key == BucketUnknown:
			unknown += g.Count
			sawUnknown = true
		default:
			rows = append(rows, row{key: *g.Key, ord: g.Ord, count: g.Count})
		}
	}
	ordered := len(rows) > 0
	for _, r := range rows {
		if r.ord == nil {
			ordered = false
			break
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if ordered {
			if *rows[i].ord != *rows[j].ord {
				return *rows[i].ord < *rows[j].ord
			}
			return rows[i].key < rows[j].key
		}
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].key < rows[j].key
	})
	out := make([]Bucket, 0, len(rows)+2)
	for _, r := range rows {
		out = append(out, Bucket{Key: r.key, Count: r.count})
	}
	if other > 0 {
		out = append(out, Bucket{Key: BucketOther, Count: other})
	}
	if sawUnknown || f.Buckets.IncludeUnknown {
		out = append(out, Bucket{Key: BucketUnknown, Count: unknown})
	}
	return out
}

// indexGroups splits raw rows into (key -> summed count), the NULL count, and the observed keys in
// first-seen order.
func indexGroups(groups []Group) (map[string]int64, int64, []string) {
	counts := map[string]int64{}
	var unknown int64
	var keys []string
	for _, g := range groups {
		if g.Key == nil {
			unknown += g.Count
			continue
		}
		if _, seen := counts[*g.Key]; !seen {
			keys = append(keys, *g.Key)
		}
		counts[*g.Key] += g.Count
	}
	return counts, unknown, keys
}

// appendUnknown adds the `(unknown)` bucket when the facet declares one (a nullable column always
// does) or when NULLs were actually counted — the second arm keeps a stale declaration from hiding
// rows rather than merely mis-shaping the chart.
func appendUnknown(out []Bucket, f facet.Facet, unknown int64) []Bucket {
	if f.Buckets.IncludeUnknown || unknown > 0 {
		return append(out, Bucket{Key: BucketUnknown, Count: unknown})
	}
	return out
}
