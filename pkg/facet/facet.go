// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package facet is the object-facet registry (M56 / D-ObjectFacets): the machine catalog of every
// declared, filterable, groupable dimension of a listable object type.
//
// A facet is declared ONCE, by the module that owns the table, and consumed TWICE:
//
//   - as a typed Conjure query arg on that module's list endpoint  (M56 — this milestone)
//   - as a groupBy token + bucket strategy on GET /<module>/v1/<collection>/stats  (M57)
//
// Because both consumers take the same argument names and the same values, a chart segment and a
// list filter are the same act: the console keeps the whole filter set in the URL, so toggling
// list<->dashboard preserves it and clicking a bar is ordinary navigation.
//
// The catalog (catalog.go) is the SOURCE OF TRUTH for the vocabulary; args_gen.go is the generated
// mirror of what the Conjure contract actually ships. args_test.go fails the build in BOTH
// directions — a facet without its query arg, or a query arg bound to neither a facet nor a
// classified non-facet role. plaintext_test.go enforces D-ObjectFacets' rules 1 and 2 against the
// migration DDL: a facet may name only a plaintext column, and a facet above pii:basic must carry
// its field's own read code.
//
// This is a stdlib-only leaf (plus pkg/rid, itself a leaf), so transports, application services and
// the M57 stats queries can all import it.
package facet

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/pkg/rid"
)

//go:generate sh -c "cd ../.. && scripts/gen-action-params.sh"

// Kind classifies what a facet contributes to each consumer: how it appears as a filter arg, and how
// its values become buckets (D-ObjectFacets; the catalog table in docs/architecture/facets.md).
type Kind string

const (
	// KindEnum is one value from a TEXT+CHECK set. Buckets are identity — one per allowed value,
	// zero-count buckets included, so a chart's shape is stable across filterings.
	KindEnum Kind = "enum"
	// KindRef is an RID pointing at another object. Buckets are top-N by count + an "other" bucket;
	// labels resolve as locale -> text maps (D-i18n).
	KindRef Kind = "ref"
	// KindCode is a plaintext code column whose value set is OPEN — no CHECK constraint to enumerate
	// (so not an enum, which zero-fills its declared values) and no RID to resolve (so not a ref,
	// which needs a labeler). Buckets are top-N + "other", and the KEY IS ITS OWN LABEL: a dotted
	// action code reads as itself, so nothing has to be looked up to draw the chart.
	//
	// M58's audit ledger is the first: `action` has a registry (pkg/action, 288 codes) but not a
	// CHECK set, and enumerating it as enum Values to zero-fill 288 buckets would be absurd.
	KindCode Kind = "code"
	// KindDateRange filters with <key>From / <key>To (ISO-8601 calendar dates) and buckets by
	// date_trunc to a grain or by named bands.
	KindDateRange Kind = "date-range"
	// KindBool filters true/false and buckets into two.
	KindBool Kind = "bool"
	// KindNumericRange filters with <key>Min / <key>Max and buckets into fixed-width bands.
	KindNumericRange Kind = "numeric-range"
)

// Strategy is how a facet's values become buckets. Declared in M56 (the strategies are already fixed
// prose in docs/architecture/facets.md, so declaring them now avoids migrating the same literals
// twice); validated here, and read by M57's stats queries.
type Strategy string

const (
	// StrategyIdentity emits one bucket per allowed enum value, including zero-count ones.
	StrategyIdentity Strategy = "identity"
	// StrategyTopN emits the TopN most frequent values plus an "other" bucket.
	StrategyTopN Strategy = "topN"
	// StrategyDateTrunc buckets a date/timestamp column by Grain.
	StrategyDateTrunc Strategy = "dateTrunc"
	// StrategyBands buckets a date or numeric column into the declared Bands.
	StrategyBands Strategy = "bands"
	// StrategyBool emits exactly two buckets.
	StrategyBool Strategy = "bool"
)

// Band is one half-open bucket [Lo, Hi) of a StrategyBands facet. A nil bound is unbounded, so the
// age pyramid's "65+" is Band{Key: "65+", Lo: ptr(65)}.
type Band struct {
	Key    string
	Lo, Hi *int
}

// Buckets is the M57 aggregation strategy for a facet. Nothing in M56 reads it except Register's
// validator — which is the point: a malformed declaration fails now, not when M57 first groups by it.
type Buckets struct {
	Strategy Strategy
	// TopN is the bucket count before "other"; StrategyTopN only. facets.md fixes this at 15.
	TopN int
	// Grain is the date_trunc unit ("day" | "month" | "year"); StrategyDateTrunc only.
	Grain string
	// Bands are the half-open buckets; StrategyBands only.
	Bands []Band
	// IncludeUnknown emits a distinct "(unknown)" bucket for NULL. MANDATORY for a nullable column —
	// plaintext_test.go reads the DDL and fails a nullable column that omits it, so the mandatory
	// (unknown) bucket facets.md promises is an invariant rather than a habit.
	IncludeUnknown bool
}

// Facet is one declared dimension of an object type (D-ObjectFacets). Owned by the module that owns
// Table; the Descriptor shape internal/links established — a module describes its own table once, and
// a build-time guard proves the description matches reality.
type Facet struct {
	// Key is the query-arg name AND the M57 groupBy token, e.g. "sex", "unitKind". camelCase.
	Key string
	// Kind drives both the arg shape (see Args) and the bucket strategy.
	Kind Kind
	// Table is the schema-qualified owning table, e.g. "oikumenea.person_persons". It is NOT always
	// the list endpoint's own table: a facet may probe another module's table (person.unitId probes
	// membership_memberships), in which case Note must say so.
	Table string
	// Column is the PLAINTEXT source column. D-ObjectFacets rule 1: never an envelope-encrypted
	// pii:special value — there is nothing to GROUP BY, and the aggregation rule forbids the surface
	// regardless. Asserted against the migration DDL in plaintext_test.go.
	Column string
	// ReadPermission is the inherited gate (D-ObjectFacets rule 2). "" means the endpoint's own read
	// code is the whole decision — legal only for a pii:none / pii:basic column. A facet above
	// pii:basic MUST name its field's read code, and M57 OMITS that facet for a caller lacking it
	// (never a zeroed bucket, never a 403 — the D-UnifiedSearch skip-the-provider behaviour).
	ReadPermission string
	// Buckets is the M57 strategy.
	Buckets Buckets
	// RefType is the rid registry token of the object a KindRef facet's column POINTS AT
	// ("country", "unit", "rank"), and is required for — and legal only on — a ref facet. M57 reads
	// it to label a bucket: a ref bucket's key is a RID, and a chart segment must carry the object's
	// locale→text display name (D-i18n), resolved by the labeler the composition root registers per
	// token. Declared here rather than derived per module so the label wiring is checkable: a ref
	// facet whose target type has no registered labeler is a boot failure, not a silently unlabelled
	// axis.
	RefType string
	// Values is the CHECK set IN CHART ORDER; KindEnum only. Chart order, not alphabetical: an
	// ordered scheme (rank seniority, ISCED level, endangerment severity) must not be re-sorted by
	// frequency, which would destroy the only ordering that means anything.
	Values []string
	// Required marks a facet whose query arg is NON-optional in the contract (unit.org — listUnits
	// rejects a fully-unscoped listing, D-TenantOrganizations M40).
	Required bool
	// ArgOverride pins the contract arg name(s) when they cannot be derived from Key — the escape
	// hatch for an arg that predates the vocabulary. Register REJECTS a non-empty ArgOverride with an
	// empty Note, so the hatch cannot be used silently.
	ArgOverride []string
	// Note records anything a reader would otherwise have to reverse-engineer: a cross-module Table,
	// non-obvious "active row" semantics, or the reason for an ArgOverride.
	Note string
	// NonPartitioning is the REASON this facet's buckets OVERLAP, and empty for every ordinary facet
	// — the Ledger pattern: it carries its own justification, so a second one costs an argument
	// rather than a copy-paste.
	//
	// Almost every facet partitions its result set: each counted row lands in exactly one bucket, so
	// the buckets sum to totalCount, and the M57 differential test asserts precisely that. Two shapes
	// genuinely cannot, and both are M58's religion taxonomy (the first tree to reach the vocabulary):
	//
	//   - a CLOSURE facet (taxon.subtree) counts each row under EVERY ancestor it has. That overlap is
	//     not a flaw to be corrected — it is what makes the chart drillable. A bucket's count is a
	//     whole subtree's size; clicking it returns exactly those rows; re-grouping within them yields
	//     that subtree's own internal nodes, recursively, all the way down. An exact-parent facet
	//     would partition honestly and then dead-end after one click, because every remaining row
	//     would share one parent.
	//   - an M:N facet (taxon.classification) counts a row once per tag it carries.
	//
	// WHAT THIS EXEMPTS IS EXACTLY ONE ASSERTION AND NO OTHER. The sum-to-totalCount check becomes
	// sum >= totalCount. The property the whole vocabulary rests on — a bucket's count equals the
	// number of rows the list returns under that bucket's own filter — is NOT relaxed, and is still
	// verified per bucket by the module's differential test. The overlap is between buckets, never
	// between a bucket and its own filter.
	//
	// Legal only on a topN ref/code facet: an enum's identity buckets come from a CHECK set, one value
	// per row by construction, and a date or band bucket is a single row's single value.
	NonPartitioning string
}

// Args returns the contract query-arg name(s) this facet binds. DERIVED from Key and Kind — never
// hand-written, which is what makes the drift guard's first direction meaningful: if the derivation
// and the contract disagree, the build fails rather than the mismatch being encoded in both places.
func (f Facet) Args() []string {
	if len(f.ArgOverride) > 0 {
		return f.ArgOverride
	}
	switch f.Kind {
	case KindDateRange:
		return []string{f.Key + "From", f.Key + "To"}
	case KindNumericRange:
		return []string{f.Key + "Min", f.Key + "Max"}
	default:
		return []string{f.Key}
	}
}

// ArgType is the Conjure primitive a facet's query arg must have, so the guard checks the contract's
// type and not merely the arg's presence.
func (f Facet) ArgType() string {
	switch f.Kind {
	case KindBool:
		return "boolean"
	case KindNumericRange:
		return "integer"
	default: // enum, ref, code, date-range — all carried as strings (RIDs and YYYY-MM-DD dates included)
		return "string"
	}
}

// ArgTypes is every contract type this facet's arg may legally have — ArgType is the canonical one,
// and these are the alternatives that preserve the property the vocabulary actually depends on:
// A BUCKET KEY MUST REMAIN A USABLE FILTER VALUE.
//
//   - An ENUM facet may be carried as a CONJURE ENUM rather than a bare string (audit's actorType and
//     outcome are). The bucket keys come from the database in the CHECK set's own lower-case spelling
//     while the enum's members are upper-case — which would be a broken click-through, except that a
//     generated enum's UnmarshalText upper-cases before matching, so `outcome=denied` is accepted.
//     That is the whole reason this is allowed, so it is written here rather than assumed: if the
//     generator ever stops normalizing case, this list is wrong and the click-through breaks.
//   - A DATE-RANGE facet may be carried as DATETIME rather than a calendar date (audit's since/until
//     are, and they are timestamps because the ledger is written by the millisecond). The console must
//     then declare `argType: "datetime"` so it widens a picked day to that day's RFC-3339 endpoints;
//     the console guard checks exactly that, against this mirror.
func (f Facet) ArgTypes() []string {
	switch f.Kind {
	case KindEnum:
		return []string{"string", "enum"}
	case KindDateRange:
		return []string{"string", "datetime"}
	default:
		return []string{f.ArgType()}
	}
}

// Class is the role of a query arg that is NOT a facet. Every such arg must be classified — and the
// classification is CHECKED, not merely present (see args_test.go), so this cannot degenerate into an
// allowlist that hides real drift.
type Class string

const (
	// ClassPaging is the keyset pagination pair; accepted only for pageSize/pageToken.
	ClassPaging Class = "paging"
	// ClassSearch is a free-text query routed to a search query, not a structural predicate
	// (D-PersonSearch / R-21 keep List and Search separate plan shapes).
	ClassSearch Class = "search"
	// ClassTraversal selects a traversal MODE rather than adding a predicate to the listed table.
	ClassTraversal Class = "traversal"
	// ClassSuperseded is a filter arg a FACET'S OWN args have replaced, retained only because the
	// contract is expand-only (L-UpgradeSafe): removing a query arg breaks every stored link and
	// every client that still sends it. `Drives` names the facet that supersedes it, and the guard
	// checks that facet exists, covers the SAME column, and does NOT itself bind this arg — so the
	// class can only ever excuse a genuine predecessor, never an unclassified filter.
	//
	// The arg keeps working: a superseded predicate is ANDed with its successor's, so a caller who
	// sends both gets the intersection rather than one silently winning.
	ClassSuperseded Class = "superseded"
)

// NonFacetArg classifies one query arg that carries no facet.
type NonFacetArg struct {
	Arg   string
	Class Class
	// Why is the human reason; mandatory for every class.
	Why string
	// Drives names the sqlc query (or, for a mode selector, the facet) this arg feeds. Mandatory for
	// ClassSearch and ClassTraversal, and resolved against the module's queries/*.sql by the guard.
	Drives string
}

// ObjectType is one listable type's whole declaration: its facets and its classified non-facet args.
type ObjectType struct {
	// Type is the pkg/rid registry token ("person", "unit", "link__member_of") — the same key the
	// console's ontology registry uses, so the two surfaces name types identically. Objects and
	// reified links both qualify; actions do not (see listableTypeTokens).
	Type string
	// Module is the owning module directory ("person", "tenant"); it locates queries/*.sql for the
	// guard and records ownership even when a facet probes another module's table.
	Module string
	// ListEndpoint is the Conjure service+endpoint the facets bind to, "PersonService.listPersons".
	// tools/genfacetargs resolves it against the IR and hard-errors if it does not exist.
	ListEndpoint string
	// StatsEndpoint is the M57 dashboard endpoint, "PersonService.personStats". Empty until the type's
	// stats endpoint ships; args_test.go keeps that honest with an explicit pending list rather than
	// letting an unbound type go unchecked. When set, the IR mirror proves it carries EVERY facet arg
	// the list carries — the two consumers must take the same argument names and the same values, or a
	// chart segment and a filter stop being the same act.
	StatsEndpoint string
	Facets        []Facet
	NonFacetArgs  []NonFacetArg
	// Ledger is the REASON a type's token is not an RID type token, and empty for every ordinary
	// collection. It is the single, narrow escape from the token check below, and it exists because
	// of one real case: the audit log (M58).
	//
	// The kind rule is unchanged — an action INVOCATION is still not listable. What a ledger says is
	// that the RECORD of those invocations is a collection: its rows have identity, attributes and
	// history, and they list and filter exactly like an object. There is no RID type token for such
	// a row because an audit entry's RID belongs to the service that PRODUCED the action (tenant,
	// person, …), never to `audit` — so registering one would make rid.TokenOf describe identifiers
	// that never exist, which is worse than admitting the exception.
	//
	// Register still refuses a Ledger token that IS an RID token, so the field cannot smuggle a real
	// type past the check it exempts.
	Ledger string
	// Profile is the RID type token this collection's rows are KEYED BY, and empty for every ordinary
	// collection whose Type is its own token. It is the second escape from the token check, and the
	// sibling of Ledger rather than a variant of it: a ledger's rows have no token at all, a profile's
	// rows have one that is not THEIRS ALONE (M58 ticket 5).
	//
	// The real case is the sidecar-on-organization shape (M41 / D-UnifiedOrgGraph). A company IS a
	// `company`-domain tenant organization plus a company_org_profiles row keyed by that organization's
	// RID; an institution is the same arrangement in the `university` domain. The row's identity, its
	// code and its translatable name all live on tenant_organizations, so the RID a caller holds for a
	// company decodes to `organization` — and registering a `company` token would make rid.TokenOf
	// describe identifiers that are never minted.
	//
	// Why the type is still declared separately rather than folded into `organization`: the facets bind
	// to a LIST ENDPOINT, and listCompanies / listInstitutions are the endpoints that actually serve
	// these rows with their sidecar columns. Folding them into the organization block would bind the
	// company facets to listOrganizations, which cannot filter on a legal form it does not select.
	//
	// Unlike Ledger, Profile is deliberately UNCAPPED: the shape already has three members in the
	// schema — company_org_profiles and education_org_profiles keyed to tenant_organizations, and
	// religion_org_profiles keyed to tenant_UNITS — so repetition here is the pattern rather than
	// erosion of it. What holds it honest instead is a STRUCTURAL guard —
	// a profile type's own table must be primary-key-FK'd to the profiled token's table, checked
	// against the migrations in catalog_test.go, so the claim is verified and not merely asserted.
	//
	// Register refuses a Profile on a type that HAS a token (that would be a way around the check
	// rather than an admission), refuses a Profile that is not itself a registered object token, and
	// refuses a declaration claiming both escapes at once.
	Profile string
}

// Registry holds the registered object types plus the deliberate exemptions. The zero value is not
// usable; use New.
type Registry struct {
	types  map[string]ObjectType
	order  []string
	exempt map[string]string
}

// New returns an empty registry.
func New() *Registry {
	return &Registry{types: map[string]ObjectType{}, exempt: map[string]string{}}
}

// listableTypeTokens is the set of registry TOKENS a facet declaration may name — the authority an
// ObjectType.Type is validated against, taken from the drift-proof pkg/rid registry (R-28).
//
// Objects AND links are listable: a reified link is a first-class row with its own identity,
// attributes and history (D-Ontology), so it lists and filters exactly like an object —
// link__member_of is the membership roster, link__has_role the assignment list. Actions are not: an
// action is an audited invocation, not a collection. pkg/rid registers no kind=action types today
// (the action catalog is pkg/action), so that arm is defensive rather than load-bearing — but it is
// the arm that keeps this a KIND check and not "anything in the registry".
//
// The token (rather than the bare rid name) is what the declaration carries, because it is what the
// console's ontology registry is keyed by — pkg/facet and web/src/lib/ontology/registry.ts must name
// the same type identically, and rid.TokenOf is the one definition of that name.
func listableTypeTokens() map[string]bool {
	out := map[string]bool{}
	for t, token := range rid.TokenOf() {
		if t.Kind == int(rid.KindObject) || t.Kind == int(rid.KindLink) {
			out[token] = true
		}
	}
	return out
}

// objectTypeTokens is the set of tokens a ref facet's RefType may name: kind=object types ONLY. A
// ref bucket's key is the RID of the thing being counted BY (a country, a rank, a unit), which is
// always an object — a reified link is itself listable, never the target of another type's column.
func objectTypeTokens() map[string]bool {
	out := map[string]bool{}
	for t, token := range rid.TokenOf() {
		if t.Kind == int(rid.KindObject) {
			out[token] = true
		}
	}
	return out
}

// Register adds one object type's declaration. Composition-time only; a declaration that names an
// unregistered object type, is structurally incomplete, or claims an arg name twice is an error.
func (r *Registry) Register(o ObjectType) error {
	switch {
	case o.Type == "":
		return errors.New("facet: object type has no Type")
	case o.Ledger != "" && o.Profile != "":
		return fmt.Errorf("facet: %q claims both Ledger and Profile — they are different admissions "+
			"(no token at all vs. a token that is not its own) and a type is at most one of them", o.Type)
	case o.Ledger == "" && o.Profile == "" && !listableTypeTokens()[o.Type]:
		return fmt.Errorf("facet: %q is not a registered object or link type token (pkg/rid) — "+
			"a collection whose rows carry no type of their own must say so via Ledger, and one whose "+
			"rows are keyed by ANOTHER type's token via Profile", o.Type)
	case o.Ledger != "" && listableTypeTokens()[o.Type]:
		return fmt.Errorf("facet: %q is a registered type token, so it must not claim Ledger — "+
			"the escape is for collections that have NO token, not a way around the check", o.Type)
	case o.Profile != "" && listableTypeTokens()[o.Type]:
		return fmt.Errorf("facet: %q is a registered type token, so it must not claim Profile — "+
			"the escape is for collections keyed by ANOTHER type's token, not a way around the check", o.Type)
	case o.Profile != "" && o.Profile == o.Type:
		return fmt.Errorf("facet: %q profiles itself — Profile names the OTHER token its rows are "+
			"keyed by", o.Type)
	case o.Profile != "" && !objectTypeTokens()[o.Profile]:
		return fmt.Errorf("facet: %s profiles %q, which is not a registered object type token "+
			"(pkg/rid) — a profile's rows are keyed by a real object's RID or they are keyed by "+
			"nothing", o.Type, o.Profile)
	case o.Module == "":
		return fmt.Errorf("facet: object type %q has no Module", o.Type)
	case o.ListEndpoint == "":
		return fmt.Errorf("facet: object type %q has no ListEndpoint", o.Type)
	case len(o.Facets) == 0:
		return fmt.Errorf("facet: object type %q declares no facets", o.Type)
	}
	if _, dup := r.types[o.Type]; dup {
		return fmt.Errorf("facet: duplicate declaration for object type %q", o.Type)
	}
	if _, ex := r.exempt[o.Type]; ex {
		return fmt.Errorf("facet: object type %q is both registered and exempt", o.Type)
	}

	claimed := map[string]string{} // arg -> what claimed it
	keys := map[string]bool{}
	for _, f := range o.Facets {
		if err := validateFacet(o.Type, f); err != nil {
			return err
		}
		if keys[f.Key] {
			return fmt.Errorf("facet: %s declares facet key %q twice", o.Type, f.Key)
		}
		keys[f.Key] = true
		for _, a := range f.Args() {
			if by, ok := claimed[a]; ok {
				return fmt.Errorf("facet: %s arg %q claimed by both %s and facet %q", o.Type, a, by, f.Key)
			}
			claimed[a] = "facet " + f.Key
		}
	}
	// One object type's ref facets must agree on TopN. The M57 stats query answers a whole dashboard
	// in one statement and binds ONE top_n across its ref branches; a facet declaring a different
	// cutoff would silently get the other one's. Cross-facet, so it cannot live in validateFacet.
	topN := 0
	for _, f := range o.Facets {
		if f.Buckets.Strategy != StrategyTopN {
			continue
		}
		if topN == 0 {
			topN = f.Buckets.TopN
			continue
		}
		if f.Buckets.TopN != topN {
			return fmt.Errorf("facet: %s top-N facet %q declares TopN %d but the type already uses %d — "+
				"one stats query binds a single top_n for every top-N branch", o.Type, f.Key, f.Buckets.TopN, topN)
		}
	}
	for _, n := range o.NonFacetArgs {
		if err := validateNonFacetArg(o.Type, n); err != nil {
			return err
		}
		if by, ok := claimed[n.Arg]; ok {
			return fmt.Errorf("facet: %s arg %q claimed by both %s and non-facet arg", o.Type, n.Arg, by)
		}
		claimed[n.Arg] = "non-facet arg"
	}

	r.types[o.Type] = o
	r.order = append(r.order, o.Type)
	return nil
}

func validateFacet(objectType string, f Facet) error {
	where := fmt.Sprintf("facet: %s.%s", objectType, f.Key)
	switch {
	case f.Key == "":
		return fmt.Errorf("facet: %s declares a facet with no Key", objectType)
	case !isLowerCamel(f.Key):
		return fmt.Errorf("%s: Key must be lowerCamelCase (it is the query-arg name)", where)
	case f.Table == "":
		return fmt.Errorf("%s: no Table", where)
	case !strings.HasPrefix(f.Table, "oikumenea."):
		return fmt.Errorf("%s: Table %q must be schema-qualified (oikumenea.<module>_*)", where, f.Table)
	case f.Column == "":
		return fmt.Errorf("%s: no Column", where)
	case len(f.ArgOverride) > 0 && f.Note == "":
		return fmt.Errorf("%s: ArgOverride requires a Note explaining why the derived name is not used", where)
	}
	switch f.Kind {
	case KindEnum:
		if len(f.Values) == 0 {
			return fmt.Errorf("%s: enum facet must declare Values (the CHECK set, in chart order)", where)
		}
	case KindRef, KindCode, KindDateRange, KindBool, KindNumericRange:
		if len(f.Values) > 0 {
			return fmt.Errorf("%s: Values is meaningful only for an enum facet", where)
		}
	default:
		return fmt.Errorf("%s: unknown Kind %q", where, f.Kind)
	}
	if f.Kind == KindRef {
		if f.RefType == "" {
			return fmt.Errorf("%s: a ref facet must declare RefType (the token its RID buckets point at, for M57 labels)", where)
		}
		if !objectTypeTokens()[f.RefType] {
			return fmt.Errorf("%s: RefType %q is not a registered object type token (pkg/rid)", where, f.RefType)
		}
	} else if f.RefType != "" {
		return fmt.Errorf("%s: RefType is meaningful only for a ref facet", where)
	}
	if f.NonPartitioning != "" && f.Kind != KindRef && f.Kind != KindCode {
		return fmt.Errorf("%s: NonPartitioning is legal only on a ref or code facet — an enum's "+
			"identity buckets come from a CHECK set (one value per row) and a date or band bucket is "+
			"a single row's single value, so neither CAN overlap", where)
	}
	return validateBuckets(where, f)
}

func validateBuckets(where string, f Facet) error {
	b := f.Buckets
	switch b.Strategy {
	case StrategyIdentity:
		if f.Kind != KindEnum {
			return fmt.Errorf("%s: identity buckets require an enum facet", where)
		}
	case StrategyTopN:
		// ref and code both rank an open value set: the difference is whether the key is an RID that
		// needs a labeler or a code that reads as itself.
		if f.Kind != KindRef && f.Kind != KindCode {
			return fmt.Errorf("%s: topN buckets require a ref or code facet", where)
		}
		if b.TopN <= 0 {
			return fmt.Errorf("%s: topN buckets require a positive TopN", where)
		}
	case StrategyDateTrunc:
		if f.Kind != KindDateRange {
			return fmt.Errorf("%s: dateTrunc buckets require a date-range facet", where)
		}
		switch b.Grain {
		case "day", "month", "year":
		default:
			return fmt.Errorf("%s: dateTrunc buckets need Grain day|month|year, got %q", where, b.Grain)
		}
	case StrategyBands:
		if f.Kind != KindDateRange && f.Kind != KindNumericRange {
			return fmt.Errorf("%s: bands require a date-range or numeric-range facet", where)
		}
		if len(b.Bands) == 0 {
			return fmt.Errorf("%s: bands strategy declares no Bands", where)
		}
		seen := map[string]bool{}
		for _, band := range b.Bands {
			if band.Key == "" {
				return fmt.Errorf("%s: a band has no Key", where)
			}
			if seen[band.Key] {
				return fmt.Errorf("%s: duplicate band key %q", where, band.Key)
			}
			seen[band.Key] = true
			if band.Lo != nil && band.Hi != nil && *band.Lo >= *band.Hi {
				return fmt.Errorf("%s: band %q is empty (Lo >= Hi)", where, band.Key)
			}
		}
	case StrategyBool:
		if f.Kind != KindBool {
			return fmt.Errorf("%s: bool buckets require a bool facet", where)
		}
	case "":
		return fmt.Errorf("%s: no bucket Strategy (M57 groups by this; declare it now)", where)
	default:
		return fmt.Errorf("%s: unknown bucket Strategy %q", where, b.Strategy)
	}
	if b.TopN > 0 && b.Strategy != StrategyTopN {
		return fmt.Errorf("%s: TopN is meaningful only for the topN strategy", where)
	}
	if b.Grain != "" && b.Strategy != StrategyDateTrunc {
		return fmt.Errorf("%s: Grain is meaningful only for the dateTrunc strategy", where)
	}
	if len(b.Bands) > 0 && b.Strategy != StrategyBands {
		return fmt.Errorf("%s: Bands are meaningful only for the bands strategy", where)
	}
	return nil
}

func validateNonFacetArg(objectType string, n NonFacetArg) error {
	where := fmt.Sprintf("facet: %s non-facet arg %q", objectType, n.Arg)
	if n.Arg == "" {
		return fmt.Errorf("facet: %s declares a non-facet arg with no Arg", objectType)
	}
	if n.Why == "" {
		return fmt.Errorf("%s: every classified arg needs a Why", where)
	}
	switch n.Class {
	case ClassPaging:
		if n.Arg != "pageSize" && n.Arg != "pageToken" {
			return fmt.Errorf("%s: the paging class covers only pageSize/pageToken", where)
		}
	case ClassSearch, ClassTraversal, ClassSuperseded:
		if n.Drives == "" {
			return fmt.Errorf("%s: the %s class requires Drives (the query or facet it feeds)", where, n.Class)
		}
	case "":
		return fmt.Errorf("%s: no Class", where)
	default:
		return fmt.Errorf("%s: unknown Class %q", where, n.Class)
	}
	return nil
}

// isLowerCamel reports whether s is a plausible lowerCamelCase query-arg name: leading lowercase
// letter, then letters and digits only.
func isLowerCamel(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}

// Exempt marks a listable object type as deliberately carrying no facets, with a rationale, so the
// M58 completeness sweep stays honest rather than silently incomplete.
func (r *Registry) Exempt(objectType, why string) { r.exempt[objectType] = why }

// Get returns one registered object type.
func (r *Registry) Get(objectType string) (ObjectType, bool) {
	o, ok := r.types[objectType]
	return o, ok
}

// All returns every registered object type, in registration order.
func (r *Registry) All() []ObjectType {
	out := make([]ObjectType, 0, len(r.order))
	for _, k := range r.order {
		out = append(out, r.types[k])
	}
	return out
}

// Exemptions returns the deliberate omissions, sorted.
func (r *Registry) Exemptions() []string {
	out := make([]string, 0, len(r.exempt))
	for k := range r.exempt {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MustBeBound is the boot-time coverage assertion (the D-ObjectFacets counterpart of the links
// service's R-27 guard), joined into cmd/oikumenea's seam loop: the registry is non-empty and every
// declaration is internally consistent.
//
// TODO(M58): widen the completeness set to "every object type with a list endpoint in the ontology
// registry is registered or Exempt". That sweep is only meaningful once the M58 rollout has given
// every listable type a facet set; asserting it today would either fail on ~20 not-yet-covered types
// or need an exemption list so long it would assert nothing.
func (r *Registry) MustBeBound() error {
	if len(r.types) == 0 {
		return errors.New("facet registry: no object types registered (pkg/facet catalog wiring)")
	}
	for _, o := range r.All() {
		if len(o.Facets) == 0 {
			return fmt.Errorf("facet registry: object type %q has no facets", o.Type)
		}
	}
	return nil
}
