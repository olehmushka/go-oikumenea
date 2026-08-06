// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"errors"
	"testing"

	"github.com/olehmushka/go-oikumenea/pkg/facet"
)

func k(s string) *string { return &s }
func n(i int64) *int64   { return &i }

func person(t *testing.T) facet.ObjectType {
	t.Helper()
	o, ok := facet.Default.Get("person")
	if !ok {
		t.Fatal("person is not registered in the facet catalog")
	}
	return o
}

// ---------------------------------------------------------------- selection

func TestSelectAbsentCSVTakesEveryFacet(t *testing.T) {
	o := person(t)
	sel, err := Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got, want := len(sel.Facets()), len(o.Facets); got != want {
		t.Fatalf("selected %d facets, want all %d", got, want)
	}
	for _, f := range o.Facets {
		if !sel.Wants(f.Key) {
			t.Errorf("facet %q not selected by the default (absent CSV)", f.Key)
		}
	}
}

func TestSelectNamedSubsetKeepsCatalogOrder(t *testing.T) {
	o := person(t)
	// Deliberately reversed relative to the catalog: the response's chart order is the catalog's, not
	// the order the client happened to type.
	sel, err := Select(o, "status , sex", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	got := []string{}
	for _, f := range sel.Facets() {
		got = append(got, f.Key)
	}
	if len(got) != 2 || got[0] != "sex" || got[1] != "status" {
		t.Fatalf("selection %v, want [sex status] in catalog order", got)
	}
	if sel.Wants("birthdate") {
		t.Error("an unnamed facet is selected")
	}
}

func TestSelectUnknownFacetIsACallerError(t *testing.T) {
	_, err := Select(person(t), "sex,eyeColour", nil)
	if !errors.Is(err, ErrUnknownFacet) {
		t.Fatalf("err = %v, want ErrUnknownFacet", err)
	}
}

func TestSelectEmptyListIsTotalOnly(t *testing.T) {
	sel, err := Select(person(t), " , ", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(sel.Facets()) != 0 {
		t.Fatalf("selected %d facets, want none", len(sel.Facets()))
	}
}

// A facet whose inherited read code the caller lacks is OMITTED — never a zeroed bucket, never a
// 403 (D-ObjectFacets rule 2), and that holds even when the caller NAMED it explicitly.
func TestSelectOmitsFacetsTheCallerMayNotRead(t *testing.T) {
	o := facet.ObjectType{
		Type: "person", Module: "person", ListEndpoint: "PersonService.listPersons",
		Facets: []facet.Facet{
			{Key: "sex", Kind: facet.KindEnum, Table: "oikumenea.person_persons", Column: "sex",
				Values: []string{"male", "female"}, Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}},
			{Key: "kind", Kind: facet.KindEnum, Table: "oikumenea.person_health_records", Column: "kind",
				ReadPermission: "person.health.read", Values: []string{"a", "b"},
				Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}},
		},
	}
	holds := func(code string) (bool, error) { return code != "person.health.read", nil }

	for _, csv := range []string{"", "sex,kind", "kind"} {
		sel, err := Select(o, csv, holds)
		if err != nil {
			t.Fatalf("Select(%q): %v", csv, err)
		}
		if sel.Wants("kind") {
			t.Errorf("Select(%q): gated facet selected without its read code", csv)
		}
	}
	// ...and it IS selected once the code is held, so the omission is the permission and not a typo.
	sel, err := Select(o, "kind", func(string) (bool, error) { return true, nil })
	if err != nil || !sel.Wants("kind") {
		t.Fatalf("gated facet not selected for a holder (sel=%v err=%v)", sel.Wants("kind"), err)
	}
}

func TestSelectPropagatesPermissionErrors(t *testing.T) {
	o := facet.ObjectType{
		Type: "person", Module: "person", ListEndpoint: "PersonService.listPersons",
		Facets: []facet.Facet{{Key: "kind", Kind: facet.KindEnum, Table: "oikumenea.person_health_records",
			Column: "kind", ReadPermission: "person.health.read", Values: []string{"a"},
			Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}}},
	}
	boom := errors.New("pdp unavailable")
	if _, err := Select(o, "", func(string) (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the PDP error propagated (a permission check that FAILED is not a denial)", err)
	}
}

// ---------------------------------------------------------------- assembly

func selectOne(t *testing.T, f facet.Facet) Selection {
	t.Helper()
	return Selection{selected: []facet.Facet{f}, set: map[string]bool{f.Key: true}}
}

func dist(t *testing.T, r Result) []Bucket {
	t.Helper()
	if len(r.Distributions) != 1 {
		t.Fatalf("got %d distributions, want 1", len(r.Distributions))
	}
	return r.Distributions[0].Buckets
}

func assertBuckets(t *testing.T, got []Bucket, want []Bucket) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d buckets %v, want %d %v", len(got), keysOf(got), len(want), keysOf(want))
	}
	for i := range want {
		if got[i].Key != want[i].Key || got[i].Count != want[i].Count {
			t.Errorf("bucket %d = {%s %d}, want {%s %d}", i, got[i].Key, got[i].Count, want[i].Key, want[i].Count)
		}
	}
}

func keysOf(bs []Bucket) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.Key)
	}
	return out
}

func TestIdentityBucketsAreZeroFilledInChartOrder(t *testing.T) {
	f := facet.Facet{Key: "sex", Kind: facet.KindEnum, Values: []string{"not_known", "male", "female", "not_applicable"},
		Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "sex", Key: k("female"), Count: 7},
		{Facet: "sex", Key: k("male"), Count: 3},
	}))
	assertBuckets(t, got, []Bucket{{Key: "not_known"}, {Key: "male", Count: 3}, {Key: "female", Count: 7}, {Key: "not_applicable"}})
}

// A value the catalog does not declare must be APPENDED, never dropped: a stale CHECK set has to
// show up as an odd bar, not as a total that disagrees with its own distribution.
func TestIdentityBucketsKeepUndeclaredValues(t *testing.T) {
	f := facet.Facet{Key: "status", Kind: facet.KindEnum, Values: []string{"active"},
		Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "status", Key: k("active"), Count: 2},
		{Facet: "status", Key: k("hibernating"), Count: 5},
	}))
	assertBuckets(t, got, []Bucket{{Key: "active", Count: 2}, {Key: "hibernating", Count: 5}})
}

func TestBoolBucketsAlwaysCarryBothSides(t *testing.T) {
	f := facet.Facet{Key: "hasAccount", Kind: facet.KindBool, Buckets: facet.Buckets{Strategy: facet.StrategyBool}}
	got := dist(t, Assemble(selectOne(t, f), []Group{{Facet: "hasAccount", Key: k("true"), Count: 9}}))
	assertBuckets(t, got, []Bucket{{Key: "true", Count: 9}, {Key: "false"}})
}

func TestBandBucketsAssignAndZeroFill(t *testing.T) {
	f := facet.Facet{Key: "birthdate", Kind: facet.KindDateRange,
		Buckets: facet.Buckets{Strategy: facet.StrategyBands, Bands: ageBands(), IncludeUnknown: true}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "birthdate", Key: k("17"), Count: 1},  // 0-17
		{Facet: "birthdate", Key: k("18"), Count: 2},  // 18-24 (half-open: the boundary belongs above)
		{Facet: "birthdate", Key: k("101"), Count: 3}, // 65+ (open-ended tail)
		{Facet: "birthdate", Key: nil, Count: 4},      // NULL birthdate
	}))
	assertBuckets(t, got, []Bucket{
		{Key: "0-17", Count: 1}, {Key: "18-24", Count: 2}, {Key: "25-34"}, {Key: "35-44"},
		{Key: "45-54"}, {Key: "55-64"}, {Key: "65+", Count: 3}, {Key: BucketUnknown, Count: 4},
	})
}

// The sum invariant: a value outside every band goes to (other) rather than vanishing.
func TestBandBucketsKeepOutOfBandValues(t *testing.T) {
	lo, hi := 0, 10
	f := facet.Facet{Key: "level", Kind: facet.KindNumericRange,
		Buckets: facet.Buckets{Strategy: facet.StrategyBands, Bands: []facet.Band{{Key: "0-9", Lo: &lo, Hi: &hi}}}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "level", Key: k("3"), Count: 1},
		{Facet: "level", Key: k("-2"), Count: 5},
	}))
	assertBuckets(t, got, []Bucket{{Key: "0-9", Count: 1}, {Key: BucketOther, Count: 5}})
}

func TestChronologicalBucketsSortAscending(t *testing.T) {
	f := facet.Facet{Key: "issuedOn", Kind: facet.KindDateRange,
		Buckets: facet.Buckets{Strategy: facet.StrategyDateTrunc, Grain: "month"}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "issuedOn", Key: k("2026-03"), Count: 2},
		{Facet: "issuedOn", Key: k("2025-11"), Count: 1},
		{Facet: "issuedOn", Key: nil, Count: 4},
	}))
	assertBuckets(t, got, []Bucket{{Key: "2025-11", Count: 1}, {Key: "2026-03", Count: 2}, {Key: BucketUnknown, Count: 4}})
}

// A declared (unknown) bucket is emitted even when nothing is in it — the same rule every other
// strategy follows. Concretely: an order register with no drafts must still show a zero draft backlog,
// or the chart changes shape as the data does (M57 ticket 2).
func TestChronologicalBucketsKeepAnEmptyDeclaredUnknown(t *testing.T) {
	f := facet.Facet{Key: "issuedOn", Kind: facet.KindDateRange,
		Buckets: facet.Buckets{Strategy: facet.StrategyDateTrunc, Grain: "month", IncludeUnknown: true}}
	got := dist(t, Assemble(selectOne(t, f), []Group{{Facet: "issuedOn", Key: k("2026-03"), Count: 2}}))
	assertBuckets(t, got, []Bucket{{Key: "2026-03", Count: 2}, {Key: BucketUnknown, Count: 0}})

	// ...and a facet that declares no unknown bucket still surfaces NULLs it actually counted, rather
	// than losing the rows.
	g := facet.Facet{Key: "issuedOn", Kind: facet.KindDateRange,
		Buckets: facet.Buckets{Strategy: facet.StrategyDateTrunc, Grain: "month"}}
	got = dist(t, Assemble(selectOne(t, g), []Group{{Facet: "issuedOn", Key: k("2026-03"), Count: 2}}))
	assertBuckets(t, got, []Bucket{{Key: "2026-03", Count: 2}})
}

// ---------------------------------------------------------------- the arm convention

// The property Compute exists to guarantee: an empty subject means the ADMIN arm, so a non-admin
// without a subject (a machine principal — pep.SubjectAuthority returns ("", false) for one) must read
// NOTHING rather than everything. This is the leak the helper centralizes away from five transports.
func TestComputeNeverGivesANonAdminTheAdminArm(t *testing.T) {
	sel := selectOne(t, facet.Facet{Key: "sex", Kind: facet.KindEnum, Values: []string{"male"},
		Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}})

	var armSeen string
	fetched := false
	fetch := func(subject string) ([]Group, error) {
		armSeen, fetched = subject, true
		return []Group{{Facet: TotalFacet, Count: 999}}, nil
	}

	res, err := Compute(context.Background(), nil, sel, false, "", fetch)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if fetched {
		t.Errorf("a non-admin with no subject reached the query at all (arm %q) — it must read nothing", armSeen)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("got %+v, want an empty result", res)
	}

	// An instance admin gets the unscoped arm...
	if _, err = Compute(context.Background(), nil, sel, true, "someone", fetch); err != nil {
		t.Fatalf("Compute(admin): %v", err)
	}
	if armSeen != "" {
		t.Errorf("admin arm passed subject %q — the admin arm carries no visibility predicate", armSeen)
	}

	// ...and an ordinary subject is scoped to itself.
	if _, err = Compute(context.Background(), nil, sel, false, "subject-1", fetch); err != nil {
		t.Fatalf("Compute(scoped): %v", err)
	}
	if armSeen != "subject-1" {
		t.Errorf("scoped arm passed subject %q, want subject-1", armSeen)
	}
}

func TestComputeAssemblesAndPropagatesErrors(t *testing.T) {
	sel := selectOne(t, facet.Facet{Key: "sex", Kind: facet.KindEnum, Values: []string{"male", "female"},
		Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}})
	res, err := Compute(context.Background(), nil, sel, true, "", func(string) ([]Group, error) {
		return []Group{{Facet: TotalFacet, Count: 3}, {Facet: "sex", Key: k("male"), Count: 3}}, nil
	})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.TotalCount != 3 || len(res.Distributions) != 1 || res.Distributions[0].Buckets[0].Count != 3 {
		t.Fatalf("got %+v, want the assembled sex distribution", res)
	}

	boom := errors.New("db down")
	if _, err := Compute(context.Background(), nil, sel, true, "", func(string) ([]Group, error) {
		return nil, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the fetch error propagated", err)
	}
}

func TestTopNBucketsOrderByCountWithSyntheticsLast(t *testing.T) {
	f := facet.Facet{Key: "unitId", Kind: facet.KindRef, RefType: "unit",
		Buckets: facet.Buckets{Strategy: facet.StrategyTopN, TopN: 15, IncludeUnknown: true}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "unitId", Key: k("u-b"), Count: 3},
		{Facet: "unitId", Key: k(BucketOther), Count: 100},
		{Facet: "unitId", Key: k("u-a"), Count: 9},
		{Facet: "unitId", Key: nil, Count: 1},
	}))
	assertBuckets(t, got, []Bucket{
		{Key: "u-a", Count: 9}, {Key: "u-b", Count: 3}, {Key: BucketOther, Count: 100}, {Key: BucketUnknown, Count: 1},
	})
}

// An ordered scheme (rank seniority) must NOT be re-sorted by frequency — facets.md ④. The ordinal
// comes from SQL because only SQL knows the scheme's order.
func TestTopNBucketsHonourTheSuppliedOrdinal(t *testing.T) {
	f := facet.Facet{Key: "rankId", Kind: facet.KindRef, RefType: "rank",
		Buckets: facet.Buckets{Strategy: facet.StrategyTopN, TopN: 15}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "rankId", Key: k("general"), Count: 1, Ord: n(900)},
		{Facet: "rankId", Key: k("private"), Count: 500, Ord: n(100)},
		{Facet: "rankId", Key: k("sergeant"), Count: 50, Ord: n(400)},
	}))
	assertBuckets(t, got, []Bucket{{Key: "private", Count: 500}, {Key: "sergeant", Count: 50}, {Key: "general", Count: 1}})
}

// A partially-supplied ordinal must not silently order half the chart: fall back to counts.
func TestTopNBucketsFallBackWhenAnOrdinalIsMissing(t *testing.T) {
	f := facet.Facet{Key: "rankId", Kind: facet.KindRef, RefType: "rank",
		Buckets: facet.Buckets{Strategy: facet.StrategyTopN, TopN: 15}}
	got := dist(t, Assemble(selectOne(t, f), []Group{
		{Facet: "rankId", Key: k("a"), Count: 1, Ord: n(1)},
		{Facet: "rankId", Key: k("b"), Count: 5},
	}))
	assertBuckets(t, got, []Bucket{{Key: "b", Count: 5}, {Key: "a", Count: 1}})
}

// ---- StrategyCatalog (M58 ticket 7) ----------------------------------------------------------

// catalogFacet is the shape enrollment.degreeLevelId declares: a ref facet over a closed, ordered
// catalog. The CatalogTable/CatalogOrder fields are read by the module's SQL, not by this package —
// what this package owns is the ORDER and the absence of a tail bucket.
func catalogFacet() facet.Facet {
	return facet.Facet{
		Key: "degreeLevelId", Kind: facet.KindRef, RefType: "degree_level",
		Buckets: facet.Buckets{
			Strategy:       facet.StrategyCatalog,
			CatalogTable:   "oikumenea.education_degree_levels",
			CatalogOrder:   "isced_level",
			IncludeUnknown: true,
		},
	}
}

// The defining property: the CATALOG's ordinal wins over the counts, always — not "when every row
// happens to carry one", which is topN's rule. A scale sorted by frequency is a ranking, and the
// whole reason this strategy exists is that "Bachelor, Doctoral, Master" is not an ISCED scale.
func TestCatalogBucketsOrderByTheCatalogOrdinalNotTheCounts(t *testing.T) {
	got := dist(t, Assemble(selectOne(t, catalogFacet()), []Group{
		// The counts and the scale disagree DELIBERATELY — 7 is the largest, 6 the smallest — because a
		// fixture whose counts happen to descend with the ordinal passes under either rule and proves
		// nothing. (Found by hand-breaking this guard: the first fixture written here did exactly that.)
		{Facet: "degreeLevelId", Key: k("isced-8"), Count: 11, Ord: n(8)},
		{Facet: "degreeLevelId", Key: k("isced-6"), Count: 2, Ord: n(6)},
		{Facet: "degreeLevelId", Key: k("isced-7"), Count: 40, Ord: n(7)},
	}))
	assertBuckets(t, got, []Bucket{
		{Key: "isced-6", Count: 2}, {Key: "isced-7", Count: 40}, {Key: "isced-8", Count: 11},
		{Key: BucketUnknown, Count: 0},
	})
}

// Zero-count levels arrive from the SQL's LEFT JOIN and must SURVIVE assembly — on a scale an empty
// level is information ("no doctoral candidates at all"), where on a ranking an absent value is
// merely outside the top N. Dropping them here would look like a shorter chart and read as a
// different fact.
func TestCatalogBucketsKeepEmptyLevels(t *testing.T) {
	got := dist(t, Assemble(selectOne(t, catalogFacet()), []Group{
		{Facet: "degreeLevelId", Key: k("isced-6"), Count: 0, Ord: n(6)},
		{Facet: "degreeLevelId", Key: k("isced-7"), Count: 0, Ord: n(7)},
		{Facet: "degreeLevelId", Key: k("isced-8"), Count: 40, Ord: n(8)},
	}))
	assertBuckets(t, got, []Bucket{
		{Key: "isced-6", Count: 0}, {Key: "isced-7", Count: 0}, {Key: "isced-8", Count: 40},
		{Key: BucketUnknown, Count: 0},
	})
}

// A row arriving with NO ordinal is a query that failed to join its catalog. It sorts LAST rather
// than being dropped (a bucket that vanishes is worse than one out of place) and, critically, rather
// than making the whole distribution fall back to count order the way topN's does — falling back
// here would silently restore the exact behaviour the strategy exists to prevent.
func TestCatalogBucketsPinAnUnorderedRowLastWithoutRePricingTheScale(t *testing.T) {
	got := dist(t, Assemble(selectOne(t, catalogFacet()), []Group{
		{Facet: "degreeLevelId", Key: k("isced-8"), Count: 40, Ord: n(8)},
		{Facet: "degreeLevelId", Key: k("orphan"), Count: 999},
		{Facet: "degreeLevelId", Key: k("isced-6"), Count: 2, Ord: n(6)},
	}))
	assertBuckets(t, got, []Bucket{
		{Key: "isced-6", Count: 2}, {Key: "isced-8", Count: 40}, {Key: "orphan", Count: 999},
		{Key: BucketUnknown, Count: 0},
	})
}

// No `(other)`: the catalog is closed and every row of it is named, so there is no tail to collapse.
// A key that says otherwise is data, and folding it into a synthetic bucket would hide it.
func TestCatalogBucketsHaveNoOtherTail(t *testing.T) {
	got := dist(t, Assemble(selectOne(t, catalogFacet()), []Group{
		{Facet: "degreeLevelId", Key: k("isced-6"), Count: 40, Ord: n(6)},
		{Facet: "degreeLevelId", Key: nil, Count: 3},
	}))
	for _, b := range got {
		if b.Key == BucketOther {
			t.Fatalf("catalog buckets emitted an (other) tail: %+v", got)
		}
	}
	assertBuckets(t, got, []Bucket{{Key: "isced-6", Count: 40}, {Key: BucketUnknown, Count: 3}})
}

// A catalog facet must NOT contribute a top_n. It has no tail to cut, and Selection.TopN() binding
// its (absent) cutoff would be the ticket-1 bug in reverse — there, a selection of code facets bound
// top_n = 0 and collapsed every bucket into `(other)`.
func TestCatalogFacetsBindNoTopN(t *testing.T) {
	if got := selectOne(t, catalogFacet()).TopN(); got != 0 {
		t.Errorf("TopN() = %d for a catalog-only selection, want 0", got)
	}
	// …and it must not SUPPRESS one either: a type mixing both (enrollment does — four topN refs
	// beside the scale) still needs the topN branches' cutoff bound.
	mixed := facet.Facet{Key: "unitId", Kind: facet.KindRef, RefType: "unit",
		Buckets: facet.Buckets{Strategy: facet.StrategyTopN, TopN: 15}}
	sel := Selection{
		selected: []facet.Facet{catalogFacet(), mixed},
		set:      map[string]bool{"degreeLevelId": true, "unitId": true},
	}
	if got := sel.TopN(); got != 15 {
		t.Errorf("TopN() = %d for a mixed selection, want 15", got)
	}
}

func TestAssembleReadsTheTotalAndKeepsFacetOrder(t *testing.T) {
	o := person(t)
	sel, err := Select(o, "sex,status", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	res := Assemble(sel, []Group{
		{Facet: TotalFacet, Count: 42},
		{Facet: "status", Key: k("active"), Count: 42},
		{Facet: "sex", Key: k("male"), Count: 42},
	})
	if res.TotalCount != 42 {
		t.Fatalf("totalCount = %d, want 42", res.TotalCount)
	}
	if len(res.Distributions) != 2 || res.Distributions[0].Facet != "sex" || res.Distributions[1].Facet != "status" {
		t.Fatalf("distributions %v, want [sex status]", res.Distributions)
	}
}

// The invariant the whole package exists to keep: a facet over the listed table's own column sums to
// totalCount. Assembly may reshape buckets; it may never lose a counted row.
func TestAssemblyPreservesTheRowCount(t *testing.T) {
	o := person(t)
	sel, err := Select(o, "sex,status,birthdate", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	groups := []Group{
		{Facet: TotalFacet, Count: 10},
		{Facet: "sex", Key: k("male"), Count: 6}, {Facet: "sex", Key: k("female"), Count: 3},
		{Facet: "sex", Key: k("martian"), Count: 1}, // undeclared value
		{Facet: "status", Key: k("active"), Count: 10},
		{Facet: "birthdate", Key: k("30"), Count: 7}, {Facet: "birthdate", Key: k("-1"), Count: 1},
		{Facet: "birthdate", Key: nil, Count: 2},
	}
	res := Assemble(sel, groups)
	for _, d := range res.Distributions {
		var sum int64
		for _, b := range d.Buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("facet %q buckets sum to %d, want totalCount %d", d.Facet, sum, res.TotalCount)
		}
	}
}

// ---------------------------------------------------------------- labels

func TestLabelsAttachToRefBucketsOnly(t *testing.T) {
	o := person(t)
	sel, err := Select(o, "sex,countryOfBirth", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	res := Assemble(sel, []Group{
		{Facet: TotalFacet, Count: 3},
		{Facet: "sex", Key: k("male"), Count: 3},
		{Facet: "countryOfBirth", Key: k("rid-ua"), Count: 2},
		{Facet: "countryOfBirth", Key: nil, Count: 1},
	})
	ids := res.RefIDs(sel)
	if got := ids["country"]; len(got) != 1 || got[0] != "rid-ua" {
		t.Fatalf("RefIDs[country] = %v, want [rid-ua] (synthetic keys are never labelled)", got)
	}
	if _, ok := ids["person"]; ok {
		t.Error("RefIDs returned ids for an unselected/non-ref facet")
	}
	res.ApplyLabels(sel, map[string]map[string]map[string]string{
		"country": {"rid-ua": {"eng": "Ukraine", "ukr": "Україна"}},
	})
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			switch {
			case d.Facet == "countryOfBirth" && b.Key == "rid-ua":
				if b.Label["eng"] != "Ukraine" {
					t.Errorf("ref bucket label = %v, want the locale map", b.Label)
				}
			case b.Label != nil:
				t.Errorf("facet %q bucket %q carries a label it should not", d.Facet, b.Key)
			}
		}
	}
}

func ageBands() []facet.Band {
	o, _ := facet.Default.Get("person")
	for _, f := range o.Facets {
		if f.Key == "birthdate" {
			return f.Buckets.Bands
		}
	}
	return nil
}

// TestSelectionTopNCoversEveryTopNKind is a regression test for a bug the LIVE run caught and no unit
// test would have: TopN() used to look only for a ref facet, so a selection made entirely of `code`
// facets (M58's audit dashboard asking for `action` + `targetType` and nothing else) bound top_n = 0.
// Every bucket then failed the `rk <= top_n` test and collapsed into `(other)` — a chart that is not
// empty, not an error, and completely wrong.
func TestSelectionTopNCoversEveryTopNKind(t *testing.T) {
	o := facet.ObjectType{
		Type: "audit", Module: "audit", ListEndpoint: "AuditService.query",
		Ledger: "a test ledger whose rows carry no type token of their own, as the real one does",
		Facets: []facet.Facet{
			{Key: "outcome", Kind: facet.KindEnum, Table: "oikumenea.audit_log", Column: "outcome",
				Values: []string{"success", "denied"}, Buckets: facet.Buckets{Strategy: facet.StrategyIdentity}},
			{Key: "action", Kind: facet.KindCode, Table: "oikumenea.audit_log", Column: "action",
				Buckets: facet.Buckets{Strategy: facet.StrategyTopN, TopN: 15}},
		},
	}
	sel, err := Select(o, "outcome,action", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if got := sel.TopN(); got != 15 {
		t.Errorf("TopN() = %d for a selection with a code facet and no ref facet; want 15 — a zero here "+
			"collapses every ranked bucket into (other)", got)
	}
}
