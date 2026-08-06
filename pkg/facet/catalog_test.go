// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/olehmushka/go-oikumenea/pkg/rid"
)

// TestDefaultRegistryBuilds proves the shipped catalog is structurally valid and bound. mustBuild
// panics on a malformed literal, so this also pins that Default is usable at import time.
func TestDefaultRegistryBuilds(t *testing.T) {
	if err := Default.MustBeBound(); err != nil {
		t.Fatalf("catalog is not bound: %v", err)
	}
	// M56 registered the first five; M58 ticket 1 added `audit`, the first LEDGER type (no RID token),
	// and ticket 2 added the first two VERTICALS — `external_organization` and `taxon`, the latter the
	// first TREE and so the first type with non-partitioning facets. Ticket 3 added `vehicle`,
	// `account` and `card`: the remaining raw-pgx modules, and `card` the first type whose
	// COLLECTION-LEVEL LIST this vocabulary had to add (cards were per-account only). Ticket 4 added
	// `organization` — the LAST type with an app-layer visibility predicate, and the tenant module's
	// second — and `languoid`, the second type with an R-21 search twin and the first with a COMPOSITE
	// (set-valued, semicolon-joined) code facet.
	// Ticket 5 added `company` and `institution`, the first two PROFILE types: sidecar rows keyed by a
	// tenant organization's RID (M41 / D-UnifiedOrgGraph), so their token is `organization` and neither
	// has one of its own.
	// Ticket 6 added the last two, both of which had NO LIST MODE to declare facets over until it made
	// one: `location` (three windowed branches and a 400 when given none, hence the first ClassWindow
	// args) and `link__has_role` (exactly one of subjectPersonId/targetUnitId required, hence a reach
	// trim asked for ONE permission code rather than the '%.read' family).
	// Ticket 7 added the last of the tranche, `link__studied_at` — the third type with no list mode,
	// and the first whose visibility comes from neither a unit column nor a shadow bit but from its
	// HOLDER (D-PersonReadScope), hence the document plan shapes rather than any M58 type's.
	want := []string{
		"person", "unit", "organization", "link__member_of", "order", "document", "audit",
		"external_organization", "taxon", "languoid", "vehicle", "account", "card",
		"company", "institution", "location", "link__has_role", "link__studied_at",
	}
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
		t.Errorf("the catalog registers exactly %v; got %d types", want, got)
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
			f.Kind, f.Values, f.RefType, f.Buckets = KindRef, nil, "country", Buckets{Strategy: StrategyTopN}
		}), "positive TopN"},
		{"ref without RefType", mut(func(f *Facet) {
			f.Kind, f.Values, f.Buckets = KindRef, nil, Buckets{Strategy: StrategyTopN, TopN: 15}
		}), "must declare RefType"},
		{"ref at an unregistered target", mut(func(f *Facet) {
			f.Kind, f.Values, f.RefType, f.Buckets = KindRef, nil, "planet", Buckets{Strategy: StrategyTopN, TopN: 15}
		}), "not a registered object type token"},
		{"ref at a LINK target", mut(func(f *Facet) {
			// A ref bucket counts BY an object; a reified link is itself listable and is never the
			// target of another type's column, so the check is kind=object, not "anything in pkg/rid".
			f.Kind, f.Values, f.RefType, f.Buckets = KindRef, nil, "link__member_of", Buckets{Strategy: StrategyTopN, TopN: 15}
		}), "not a registered object type token"},
		{"RefType on a non-ref", mut(func(f *Facet) { f.RefType = "country" }), "meaningful only for a ref facet"},
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

// TestLedgerExemptionsAreEarned holds the ONE escape from the type-token check to its reason (M58).
// `Ledger` exists for a collection whose rows carry no type of their own — the audit log, whose every
// entry is keyed by an Action RID minted by the service that produced it. The danger is obvious: a
// free-text field that turns "this token is not in pkg/rid" from an error into a shrug. So it must
// carry a reason, must NOT name a token that IS registered (Register enforces that), and must stay
// rare enough to notice — a second ledger is a conversation, not a copy-paste.
func TestLedgerExemptionsAreEarned(t *testing.T) {
	var ledgers []string
	for _, o := range Default.All() {
		if o.Ledger == "" {
			continue
		}
		ledgers = append(ledgers, o.Type)
		if len(strings.TrimSpace(o.Ledger)) < 40 {
			t.Errorf("%s: Ledger must explain WHY the type has no RID token, in a sentence", o.Type)
		}
		if listableTypeTokens()[o.Type] {
			t.Errorf("%s claims Ledger but IS a registered type token — the escape is for collections "+
				"with no token, not a way around the check", o.Type)
		}
	}
	// The floor, and the point of the test: exactly one type may be extraordinary without anyone
	// noticing. This number changing is a review moment.
	if len(ledgers) > 1 {
		t.Errorf("more than one Ledger type registered (%v) — each is an exception to D-Ontology's "+
			"kind rule and needs its own argument in decisions.md, not a precedent", ledgers)
	}
}

// TestProfileTypesAreKeyedToTheirProfiledToken is the STRUCTURAL half of the second escape (M58
// ticket 5), and the reason `Profile` needs no prose where `Ledger` needs a sentence.
//
// A ledger's claim ("these rows have no token of their own") cannot be checked against anything — the
// absence of a token is not written down anywhere — so it is held by a reason and a cap. A profile's
// claim IS checkable: it says the rows are keyed by ANOTHER table's RID, and that is a fact the DDL
// records. So this reads the migrations and asserts the profile's own table is PRIMARY-KEY-FK'd to
// the profiled token's table. A declaration that profiles the wrong parent, or a table that stops
// being a sidecar and grows its own RID, goes red here rather than being believed.
//
// The parent's table is not hard-coded: it is the profiled TYPE's own listed table, taken from the
// registry. So `organization`'s table moving moves this check with it.
func TestProfileTypesAreKeyedToTheirProfiledToken(t *testing.T) {
	pks := parsePrimaryKeyRefs(t)
	seen := 0
	for _, o := range Default.All() {
		if o.Profile == "" {
			continue
		}
		seen++
		parent, ok := Default.Get(o.Profile)
		if !ok {
			t.Errorf("%s profiles %q, which is not itself a registered object type — the guard has no "+
				"table to check the key against, so the claim would go unverified", o.Type, o.Profile)
			continue
		}
		parentTable, ownTable := listedTable(parent), listedTable(o)
		if ownTable == "" || parentTable == "" {
			t.Errorf("%s: cannot determine the listed table (own=%q parent=%q)", o.Type, ownTable, parentTable)
			continue
		}
		ref, found := pks[ownTable]
		switch {
		case !found:
			t.Errorf("%s: %s declares no single-column PRIMARY KEY in migrations/ — a profile is a "+
				"SIDECAR keyed by its parent's RID, and this table is not shaped like one", o.Type, ownTable)
		case ref == "":
			t.Errorf("%s: %s's primary key REFERENCES nothing — it mints its own identifiers, so it is "+
				"not a profile of %s and should carry its own RID type token instead",
				o.Type, ownTable, o.Profile)
		case ref != parentTable:
			t.Errorf("%s: %s's primary key REFERENCES %s, but the type declares Profile %q whose table "+
				"is %s — the declaration and the schema disagree about what these rows ARE",
				o.Type, ownTable, ref, o.Profile, parentTable)
		}
	}
	// Non-vacuity: this guard reads files outside the package and parses SQL by hand, so a parse that
	// silently matches nothing would turn every assertion above into a pass. If the catalog has profile
	// types, the parser must have found primary keys.
	if seen > 0 && len(pks) < 10 {
		t.Fatalf("parsed only %d primary keys from migrations/ — the parser is broken, and every check "+
			"above is vacuous", len(pks))
	}
}

// TestProfileTypesShareOneParentToday. `Profile` is deliberately UNCAPPED where `Ledger` is capped at
// one: the sidecar-on-organization shape has two members already (company and education org
// profiles), so a second profile type is the pattern working, not eroding. What is worth a review
// moment is a profile of a DIFFERENT parent — and that is not hypothetical: religion_org_profiles is
// the same sidecar shape keyed to tenant_UNITS, so the day a religion profile becomes listable this
// guard goes red and asks for the modelling argument rather than accepting the copy.
func TestProfileTypesShareOneParentToday(t *testing.T) {
	parents := map[string][]string{}
	for _, o := range Default.All() {
		if o.Profile != "" {
			parents[o.Profile] = append(parents[o.Profile], o.Type)
		}
	}
	if len(parents) > 1 {
		t.Errorf("profile types now hang off more than one parent (%v) — the sidecar-on-organization "+
			"shape is D-UnifiedOrgGraph's, and a second parent is a modelling decision that belongs in "+
			"decisions.md before it belongs here", parents)
	}
	if got := parents["organization"]; len(got) > 0 && len(got) < 2 {
		t.Errorf("expected both sidecar profile types on `organization`, got %v", got)
	}
}

// TestRegisterProfileRules pins the arms Register grew for the second escape, including the two that
// keep it from becoming a way AROUND the token check rather than an admission of one.
func TestRegisterProfileRules(t *testing.T) {
	base := func() ObjectType {
		return ObjectType{Type: "company", Module: "company", ListEndpoint: "S.e", Facets: []Facet{validFacet()}}
	}
	cases := []struct {
		name string
		edit func(*ObjectType)
		want string
	}{
		{"no escape at all", func(o *ObjectType) {}, "not a registered object or link type token"},
		{"both escapes", func(o *ObjectType) { o.Profile, o.Ledger = "organization", "because" }, "different admissions"},
		{"profiles itself", func(o *ObjectType) { o.Profile = "company" }, "profiles itself"},
		{"profiles a non-token", func(o *ObjectType) { o.Profile = "not_a_token" }, "not a registered object type token"},
		{"profiles a link", func(o *ObjectType) { o.Profile = "link__member_of" }, "not a registered object type token"},
		{"a real token claiming Profile", func(o *ObjectType) { o.Type, o.Profile = "person", "organization" }, "must not claim Profile"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := base()
			c.edit(&o)
			err := New().Register(o)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error mentioning %q", err, c.want)
			}
		})
	}
	ok := base()
	ok.Profile = "organization"
	if err := New().Register(ok); err != nil {
		t.Fatalf("a well-formed profile declaration must be registrable: %v", err)
	}
}

// parsePrimaryKeyRefs maps each table in migrations/ to the table its single-column PRIMARY KEY
// REFERENCES, or "" when the key references nothing. Tables with no single-column PK are absent.
//
// Both the inline form (`company_id uuid PRIMARY KEY REFERENCES oikumenea.tenant_organizations(id)`)
// and a later `ALTER TABLE … ADD … FOREIGN KEY (pk) REFERENCES …` are recognised: since the 46→15
// migration consolidation a table's shape is NOT its CREATE TABLE, and a guard that reads only the
// CREATE block is reading a snapshot (the vehicle.color defect ticket 3 had to un-decide).
func parsePrimaryKeyRefs(t *testing.T) map[string]string {
	t.Helper()
	var (
		pkInline = regexp.MustCompile(`^\s{2,}([a-z_][a-z0-9_]*)\s+[a-z]+\s+PRIMARY KEY(.*)$`)
		refRe    = regexp.MustCompile(`REFERENCES\s+(oikumenea\.[a-z_]+)`)
		alterFK  = regexp.MustCompile(`ALTER TABLE (?:ONLY )?(oikumenea\.[a-z_]+)[\s\S]{0,200}?FOREIGN KEY \(([a-z_]+)\)\s+REFERENCES\s+(oikumenea\.[a-z_]+)`)
	)
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	refs := map[string]string{} // table -> referenced table ("" = none)
	pkCol := map[string]string{}
	var bodies []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		bodies = append(bodies, string(body))
		var current string
		for _, line := range strings.Split(string(body), "\n") {
			if m := createRe.FindStringSubmatch(line); m != nil {
				current = m[1]
				continue
			}
			if current == "" {
				continue
			}
			if strings.HasPrefix(line, ")") {
				current = ""
				continue
			}
			if m := pkInline.FindStringSubmatch(line); m != nil {
				if _, dup := refs[current]; !dup {
					pkCol[current] = m[1]
					if r := refRe.FindStringSubmatch(m[2]); r != nil {
						refs[current] = r[1]
					} else {
						refs[current] = ""
					}
				}
			}
		}
	}
	// A PK whose FK arrived later, by ALTER.
	for _, body := range bodies {
		for _, m := range alterFK.FindAllStringSubmatch(body, -1) {
			if pkCol[m[1]] == m[2] && refs[m[1]] == "" {
				refs[m[1]] = m[3]
			}
		}
	}
	return refs
}

// TestCodeFacetsCarryNoLabelPromise: a code facet's KEY IS ITS LABEL, which is the whole reason the
// kind exists beside ref. Declaring a RefType on one would promise a labeler that will never be
// called — Register already rejects it, and this says why in the place a reader looks for intent.
func TestCodeFacetsCarryNoLabelPromise(t *testing.T) {
	for _, o := range Default.All() {
		for _, f := range o.Facets {
			if f.Kind != KindCode {
				continue
			}
			if f.RefType != "" {
				t.Errorf("%s.%s is a code facet with RefType %q — a code bucket is its own label", o.Type, f.Key, f.RefType)
			}
			if f.Buckets.Strategy != StrategyTopN {
				t.Errorf("%s.%s is a code facet bucketed %q — an open value set can only be ranked", o.Type, f.Key, f.Buckets.Strategy)
			}
		}
	}
}
