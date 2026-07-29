// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"strings"
	"testing"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

// TestDefaultRegistryBuilds proves the shipped catalog is structurally valid and bound. mustBuild
// panics on a malformed literal, so this also pins that Default is usable at import time.
func TestDefaultRegistryBuilds(t *testing.T) {
	if err := Default.MustBeBound(); err != nil {
		t.Fatalf("catalog is not bound: %v", err)
	}
	want := []string{"person", "unit", "link__member_of", "order", "document"}
	for _, w := range want {
		o, ok := Default.Get(w)
		if !ok {
			t.Fatalf("object type %q is not registered", w)
		}
		if o.Module == "" || o.ListEndpoint == "" {
			t.Errorf("%s: incomplete declaration %+v", w, o)
		}
	}
	if got := len(Default.All()); got != len(want) {
		t.Errorf("M56 registers exactly %v; got %d types", want, got)
	}
}

// TestPersonHasNoSpecialCategoryFacet is the readable statement of D-ObjectFacets rule 1 for the type
// it matters most for. plaintext_test.go proves it mechanically against the DDL; this one names the
// surfaces, so a future session sees the intent and not just a passing parser.
func TestPersonHasNoSpecialCategoryFacet(t *testing.T) {
	o, _ := Default.Get("person")
	banned := []string{
		"oikumenea.person_ethnicities",
		"oikumenea.person_party_memberships",
		"oikumenea.person_political_leaning",
		"oikumenea.religion_affiliations",
		"oikumenea.person_health_records",
		"oikumenea.person_legal_records",
	}
	for _, f := range o.Facets {
		for _, b := range banned {
			if f.Table == b {
				t.Errorf("facet %q names %s — a pii:special surface D-DataScope's aggregation rule forbids", f.Key, b)
			}
		}
	}
}

// TestArgsDerivation pins the derivation the drift guard's first direction depends on.
func TestArgsDerivation(t *testing.T) {
	cases := []struct {
		name string
		f    Facet
		want []string
	}{
		{"enum", Facet{Key: "sex", Kind: KindEnum}, []string{"sex"}},
		{"ref", Facet{Key: "rankId", Kind: KindRef}, []string{"rankId"}},
		{"bool", Facet{Key: "hasAccount", Kind: KindBool}, []string{"hasAccount"}},
		{"date-range", Facet{Key: "birthdate", Kind: KindDateRange}, []string{"birthdateFrom", "birthdateTo"}},
		{"numeric-range", Facet{Key: "level", Kind: KindNumericRange}, []string{"levelMin", "levelMax"}},
		{"override wins", Facet{Key: "level", Kind: KindNumericRange, ArgOverride: []string{"level"}, Note: "legacy"}, []string{"level"}},
	}
	for _, c := range cases {
		got := c.f.Args()
		if len(got) != len(c.want) {
			t.Errorf("%s: Args()=%v want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: Args()=%v want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// TestArgTypes pins the Conjure primitive each kind must be carried as — the guard compares against
// this, so a date facet declared as a datetime in the contract fails.
func TestArgTypes(t *testing.T) {
	for kind, want := range map[Kind]string{
		KindEnum:         "string",
		KindRef:          "string",
		KindDateRange:    "string",
		KindBool:         "boolean",
		KindNumericRange: "integer",
	} {
		if got := (Facet{Kind: kind}).ArgType(); got != want {
			t.Errorf("%s: ArgType()=%q want %q", kind, got, want)
		}
	}
}

// validFacet is a minimal well-formed facet the rejection cases mutate.
func validFacet() Facet {
	return Facet{
		Key: "status", Kind: KindEnum,
		Table: "oikumenea.person_persons", Column: "status",
		Values: []string{"active"}, Buckets: Buckets{Strategy: StrategyIdentity},
	}
}

func withFacet(f Facet) ObjectType {
	return ObjectType{Type: "person", Module: "person", ListEndpoint: "PersonService.listPersons", Facets: []Facet{f}}
}

// TestRegisterRejects exercises every structural rule, so the validator is proven to fire rather than
// assumed to. Each case names the mutation and the substring the error must carry.
func TestRegisterRejects(t *testing.T) {
	mut := func(fn func(*Facet)) ObjectType {
		f := validFacet()
		fn(&f)
		return withFacet(f)
	}
	cases := []struct {
		name string
		o    ObjectType
		want string
	}{
		{"unknown object type", ObjectType{Type: "not_a_type", Module: "m", ListEndpoint: "S.e", Facets: []Facet{validFacet()}}, "not a registered object or link type token"},
		// A link's BARE name is not its token — only the link__ form is accepted, which is what keeps
		// pkg/facet and the console registry naming the same type identically.
		{"bare link name", ObjectType{Type: "member_of", Module: "membership", ListEndpoint: "S.e", Facets: []Facet{validFacet()}}, "not a registered object or link type token"},
		{"no module", ObjectType{Type: "person", ListEndpoint: "S.e", Facets: []Facet{validFacet()}}, "no Module"},
		{"no list endpoint", ObjectType{Type: "person", Module: "person", Facets: []Facet{validFacet()}}, "no ListEndpoint"},
		{"no facets", ObjectType{Type: "person", Module: "person", ListEndpoint: "S.e"}, "declares no facets"},
		{"no key", mut(func(f *Facet) { f.Key = "" }), "no Key"},
		{"snake_case key", mut(func(f *Facet) { f.Key = "country_of_birth" }), "lowerCamelCase"},
		{"no table", mut(func(f *Facet) { f.Table = "" }), "no Table"},
		{"unqualified table", mut(func(f *Facet) { f.Table = "person_persons" }), "schema-qualified"},
		{"no column", mut(func(f *Facet) { f.Column = "" }), "no Column"},
		{"unknown kind", mut(func(f *Facet) { f.Kind = "histogram" }), "unknown Kind"},
		{"enum without values", mut(func(f *Facet) { f.Values = nil }), "must declare Values"},
		{"values on a ref", mut(func(f *Facet) {
			f.Kind, f.Buckets = KindRef, Buckets{Strategy: StrategyTopN, TopN: 15}
		}), "meaningful only for an enum"},
		{"no bucket strategy", mut(func(f *Facet) { f.Buckets = Buckets{} }), "no bucket Strategy"},
		{"unknown strategy", mut(func(f *Facet) { f.Buckets = Buckets{Strategy: "pie"} }), "unknown bucket Strategy"},
		{"identity on a non-enum", mut(func(f *Facet) {
			f.Kind, f.Values = KindBool, nil
		}), "identity buckets require an enum"},
		{"topN without N", mut(func(f *Facet) {
			f.Kind, f.Values, f.Buckets = KindRef, nil, Buckets{Strategy: StrategyTopN}
		}), "positive TopN"},
		{"dateTrunc without grain", mut(func(f *Facet) {
			f.Kind, f.Values, f.Buckets = KindDateRange, nil, Buckets{Strategy: StrategyDateTrunc}
		}), "Grain day|month|year"},
		{"bands without bands", mut(func(f *Facet) {
			f.Kind, f.Values, f.Buckets = KindNumericRange, nil, Buckets{Strategy: StrategyBands}
		}), "declares no Bands"},
		{"empty band", mut(func(f *Facet) {
			f.Kind, f.Values = KindNumericRange, nil
			f.Buckets = Buckets{Strategy: StrategyBands, Bands: []Band{{Key: "x", Lo: iptr(5), Hi: iptr(5)}}}
		}), "is empty (Lo >= Hi)"},
		{"grain on the wrong strategy", mut(func(f *Facet) { f.Buckets.Grain = "month" }), "Grain is meaningful only"},
		{"override without note", mut(func(f *Facet) { f.ArgOverride = []string{"legacy"} }), "requires a Note"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := New().Register(c.o)
			if err == nil {
				t.Fatalf("Register accepted a malformed declaration")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not mention %q", err, c.want)
			}
		})
	}
}

// TestRegisterAcceptsLinkTypes: a reified link is listable and filterable exactly like an object
// (D-Ontology), so its token is a legal ObjectType.Type. Pinned because the registry admitted only
// kind=object until M56 ticket 3, and membership — the first faceted link — would silently have no
// home if this regressed.
func TestRegisterAcceptsLinkTypes(t *testing.T) {
	o := ObjectType{
		Type: "link__member_of", Module: "membership",
		ListEndpoint: "MembershipService.listMemberships",
		Facets:       []Facet{validFacet()},
	}
	if err := New().Register(o); err != nil {
		t.Fatalf("a link token must be registrable: %v", err)
	}
}

// TestListableTokensExcludeActions pins the kind filter itself: whatever pkg/rid grows, the facet
// registry admits objects and links and nothing else.
func TestListableTokensExcludeActions(t *testing.T) {
	tokens := listableTypeTokens()
	for token, info := range rid.Tokens() {
		switch rid.Kind(info.Kind) {
		case rid.KindObject, rid.KindLink:
			if !tokens[token] {
				t.Errorf("token %q (kind %s) should be listable", token, rid.Kind(info.Kind))
			}
		default:
			if tokens[token] {
				t.Errorf("token %q (kind %s) must not be listable", token, rid.Kind(info.Kind))
			}
		}
	}
	if !tokens["person"] || !tokens["link__member_of"] {
		t.Error("expected both an object and a link token to be listable")
	}
}

// TestRegisterRejectsArgCollisions: two facets, or a facet and a classified arg, may never claim the
// same query-arg name — that would make the drift guard ambiguous.
func TestRegisterRejectsArgCollisions(t *testing.T) {
	dup := ObjectType{Type: "person", Module: "person", ListEndpoint: "S.e", Facets: []Facet{validFacet(), validFacet()}}
	if err := New().Register(dup); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("duplicate facet key accepted or wrong error: %v", err)
	}

	clash := withFacet(validFacet())
	clash.NonFacetArgs = []NonFacetArg{{Arg: "status", Class: ClassPaging, Why: "nope"}}
	if err := New().Register(clash); err == nil {
		t.Error("an arg claimed by both a facet and a non-facet entry was accepted")
	}

	twice := New()
	if err := twice.Register(withFacet(validFacet())); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := twice.Register(withFacet(validFacet())); err == nil || !strings.Contains(err.Error(), "duplicate declaration") {
		t.Errorf("re-registering an object type accepted or wrong error: %v", err)
	}
}

// TestNonFacetArgRules: the classification is checked, not merely recorded — a paging class may only
// cover the two paging args, and search/traversal must say what they drive.
func TestNonFacetArgRules(t *testing.T) {
	cases := []struct {
		name string
		n    NonFacetArg
		want string
	}{
		{"no class", NonFacetArg{Arg: "q", Why: "w"}, "no Class"},
		{"unknown class", NonFacetArg{Arg: "q", Class: "misc", Why: "w"}, "unknown Class"},
		{"no why", NonFacetArg{Arg: "pageSize", Class: ClassPaging}, "needs a Why"},
		{"paging misused", NonFacetArg{Arg: "sortBy", Class: ClassPaging, Why: "w"}, "only pageSize/pageToken"},
		{"search without drives", NonFacetArg{Arg: "query", Class: ClassSearch, Why: "w"}, "requires Drives"},
		{"traversal without drives", NonFacetArg{Arg: "parent", Class: ClassTraversal, Why: "w"}, "requires Drives"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := withFacet(validFacet())
			o.NonFacetArgs = []NonFacetArg{c.n}
			err := New().Register(o)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error mentioning %q", err, c.want)
			}
		})
	}
}

// TestMustBeBoundFailsEmpty: an unwired registry is a boot error, not a silently empty vocabulary.
func TestMustBeBoundFailsEmpty(t *testing.T) {
	if err := New().MustBeBound(); err == nil {
		t.Error("an empty registry must not be bound")
	}
}

// TestExemptIsMutuallyExclusive: a type cannot be both covered and deliberately omitted.
func TestExemptIsMutuallyExclusive(t *testing.T) {
	r := New()
	r.Exempt("person", "test")
	if err := r.Register(withFacet(validFacet())); err == nil {
		t.Error("registering an exempt type was accepted")
	}
	if got := r.Exemptions(); len(got) != 1 || got[0] != "person" {
		t.Errorf("Exemptions()=%v", got)
	}
}
