// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package facet

// The catalog — the hand-written facet vocabulary, one block per listable object type. This file is
// the SOURCE OF TRUTH for the vocabulary; the Conjure contract is proven to match it by args_test.go
// (both directions) and the columns are proven plaintext + correctly gated by plaintext_test.go.
// The readable catalog with the M57 chart proposals is docs/architecture/facets.md.
//
// M56 ticket 2 registers `person` and `unit`; ticket 3 adds `link__member_of`, `order` and
// `document` alongside their new top-level list endpoints. M57-M58 add the rest; a new type is a
// block here plus its query args, never new infrastructure.

// catalog registers every declared object type. Kept in one place (rather than per-module Facets()
// functions wired at the composition root, the internal/links arrangement) because the IR-derived
// generated mirror and the drift guard must live in the same package as the vocabulary they check —
// that is exactly why pkg/action's catalog+generator+test triad works. Module ownership is carried by
// the mandatory Module field and enforced by the plaintext/table guards.
func catalog() []ObjectType {
	return []ObjectType{
		personType(), unitType(), membershipType(), orderType(), documentType(), auditType(),
		externalOrgType(), taxonType(), vehicleType(), accountType(), cardType(),
	}
}

// Default is the process-wide registry, built from the catalog. It panics on a malformed catalog:
// this is a compile-time-constant literal validated by pure functions, so a failure is a programming
// error caught by any test in the package, never a runtime condition.
var Default = mustBuild()

func mustBuild() *Registry {
	r := New()
	for _, o := range catalog() {
		if err := r.Register(o); err != nil {
			panic(err)
		}
	}
	return r
}

// ── person ──────────────────────────────────────────────────────────────────
//
// Every source column is pii:none or pii:basic, so every facet leaves ReadPermission empty: the
// endpoint's own person.read is the whole decision (D-ObjectFacets rule 2). There is deliberately NO
// facet over ethnicity, party membership, political leaning, religious affiliation, health or legal
// records — those are envelope-encrypted pii:special values with no plaintext to group, and
// D-DataScope's aggregation rule forbids the surface regardless (rule 1, asserted in
// plaintext_test.go). See docs/architecture/facets.md § What has no facet.
func personType() ObjectType {
	return ObjectType{
		Type:          "person",
		Module:        "person",
		ListEndpoint:  "PersonService.listPersons",
		StatsEndpoint: "PersonService.personStats",
		Facets: []Facet{
			{
				Key:     "sex",
				Kind:    KindEnum,
				Table:   "oikumenea.person_persons",
				Column:  "sex",
				Values:  []string{"not_known", "male", "female", "not_applicable"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "ISO/IEC 5218. The not_known slice is a data-quality signal and is never hidden.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.person_persons",
				Column:  "status",
				Values:  []string{"active", "deactivated", "provisional", "purged"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
			{
				Key:    "birthdate",
				Kind:   KindDateRange,
				Table:  "oikumenea.person_persons",
				Column: "birthdate",
				Buckets: Buckets{
					Strategy:       StrategyBands,
					Bands:          ageBands(),
					IncludeUnknown: true,
				},
				Note: "Nullable, so the (unknown) bucket is mandatory. The FILTER counterpart: setting " +
					"either bound EXCLUDES unknown birthdates (SQL three-valued logic).",
			},
			{
				Key:     "countryOfBirth",
				Kind:    KindRef,
				Table:   "oikumenea.person_persons",
				Column:  "country_of_birth_id",
				RefType: "country",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "-> oikumenea.geo_countries (D-Geo). Nullable.",
			},
			{
				Key:     "rankId",
				Kind:    KindRef,
				Table:   "oikumenea.person_ranks",
				Column:  "rank_id",
				RefType: "rank",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "Another table (still person's): matches the person's ACTIVE rank row " +
					"(deleted_at IS NULL). A person holds one rank PER RANK SYSTEM (D-Rank), so the " +
					"distribution is per-system. M57 orders its buckets by rank SENIORITY, not by count.",
			},
			{
				Key:     "unitId",
				Kind:    KindRef,
				Table:   "oikumenea.membership_memberships",
				Column:  "unit_id",
				RefType: "unit",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "Cross-module (membership owns the table): matches an ACTIVE membership. " +
					"SUBTREE-EXPANDING — the unit itself or any closure descendant. The `graph` arg " +
					"narrows the expansion to one graph; by default it spans every authority-bearing " +
					"graph, the same closure set the read-scope predicate uses, so the filter can " +
					"never widen beyond what the subject may already read.",
			},
			{
				Key:     "hasAccount",
				Kind:    KindBool,
				Table:   "oikumenea.account_accounts",
				Column:  "person_id",
				Buckets: Buckets{Strategy: StrategyBool},
				Note: "Cross-module (identity-federation owns the table): an EXISTS semi-join over " +
					"active accounts. The directory is account-optional (L-AccountOptional), so this " +
					"is a real distribution and not a near-constant.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
			{
				Arg:    "query",
				Class:  ClassSearch,
				Why:    "free-text name/code substring; a separate plan shape from the structural filters (R-21 List/Search split)",
				Drives: "SearchPersons",
			},
			{
				Arg:   "graph",
				Class: ClassTraversal,
				Why: "selects WHICH DAG the unitId facet's closure expansion walks; adds no predicate " +
					"of its own and is ignored without unitId. Default: every authority-bearing graph.",
				Drives: "ListPersons",
			},
		},
	}
}

// ageBands is the canonical personnel age structure (docs/architecture/facets.md ① age pyramid).
// Half-open [Lo, Hi) in years; the open-ended 65+ has no Hi.
func ageBands() []Band {
	return []Band{
		{Key: "0-17", Lo: iptr(0), Hi: iptr(18)},
		{Key: "18-24", Lo: iptr(18), Hi: iptr(25)},
		{Key: "25-34", Lo: iptr(25), Hi: iptr(35)},
		{Key: "35-44", Lo: iptr(35), Hi: iptr(45)},
		{Key: "45-54", Lo: iptr(45), Hi: iptr(55)},
		{Key: "55-64", Lo: iptr(55), Hi: iptr(65)},
		{Key: "65+", Lo: iptr(65)},
	}
}

func iptr(n int) *int { return &n }

// ── unit ────────────────────────────────────────────────────────────────────
//
// Four facets retro-declare args listUnits already ships; visibility/state/pdpScoped are new in M56.
// Note that `graph`, which docs/architecture/facets.md listed as a ref facet, is NOT one: it selects
// which DAG parent/rootsOnly walk and adds no predicate to tenant_units — there is no
// tenant_units.graph_id to filter or GROUP BY. It is classified as a traversal arg below.
func unitType() ObjectType {
	return ObjectType{
		Type:          "unit",
		Module:        "tenant",
		ListEndpoint:  "TenantService.listUnits",
		StatsEndpoint: "TenantService.unitStats",
		Facets: []Facet{
			{
				Key:      "org",
				Kind:     KindRef,
				Table:    "oikumenea.tenant_units",
				Column:   "org_id",
				RefType:  "organization",
				Required: true,
				Buckets:  Buckets{Strategy: StrategyTopN, TopN: 15},
				Note:     "REQUIRED — a fully-unscoped listing is rejected (D-TenantOrganizations, M40).",
			},
			{
				Key:     "domain",
				Kind:    KindRef,
				Table:   "oikumenea.tenant_units",
				Column:  "domain_id",
				RefType: "domain",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
			},
			{
				Key:     "unitKind",
				Kind:    KindRef,
				Table:   "oikumenea.tenant_units",
				Column:  "kind_id",
				RefType: "unit_kind",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "Domain-scoped catalog (tenant_unit_kinds). Nullable.",
			},
			{
				Key:    "level",
				Kind:   KindNumericRange,
				Table:  "oikumenea.tenant_units",
				Column: "level",
				Buckets: Buckets{
					Strategy:       StrategyBands,
					Bands:          levelBands(),
					IncludeUnknown: true,
				},
				Note: "Binds the DERIVED levelMin/levelMax pair. Until M57 ticket 3 it pinned the " +
					"pre-existing scalar `level` via ArgOverride, and the deferral was explicit: the " +
					"range args were 'additive and deferred to when the bands are actually consumed'. " +
					"Consuming them is exactly what a dashboard does — a band is a RANGE, and no " +
					"single value expresses one, so the bars were readable and inert. The scalar is " +
					"still shipped and still honoured (the contract is expand-only); it is classified " +
					"below as superseded.",
			},
			{
				Key:     "visibility",
				Kind:    KindEnum,
				Table:   "oikumenea.tenant_units",
				Column:  "visibility",
				Values:  []string{"public", "shadow"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note: "The shadow-visibility gate still trims the page AFTER it is cut; this filter " +
					"NARROWS, it never widens (D-VisibilityScope).",
			},
			{
				Key:     "state",
				Kind:    KindEnum,
				Table:   "oikumenea.tenant_units",
				Column:  "state",
				Values:  []string{"active", "suspended", "archived"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
			{
				Key:     "pdpScoped",
				Kind:    KindBool,
				Table:   "oikumenea.tenant_units",
				Column:  "pdp_scoped",
				Buckets: Buckets{Strategy: StrategyBool},
				Note:    "false = a reference unit (university/company), exempt from the reach predicate (D-UnifiedOrgGraph, M41).",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
			{
				Arg:    "graph",
				Class:  ClassTraversal,
				Why:    "selects which DAG parent/rootsOnly walk; tenant_units has no graph column to filter or group by",
				Drives: "ListChildUnits",
			},
			{
				Arg:    "parent",
				Class:  ClassTraversal,
				Why:    "switches the listing to DIRECT children of one unit within `graph`",
				Drives: "ListChildUnits",
			},
			{
				Arg:    "rootsOnly",
				Class:  ClassTraversal,
				Why:    "switches the listing to the org's root units within `graph`",
				Drives: "ListRootUnits",
			},
			{
				Arg:   "level",
				Class: ClassSuperseded,
				Why: "the exact-match scalar this endpoint has always shipped; levelMin/levelMax bind " +
					"the same column as a range (an exact match is levelMin=n&levelMax=n). Retained " +
					"and still honoured because the contract is expand-only — the three predicates " +
					"are ANDed rather than one silently winning",
				Drives: "level",
			},
		},
	}
}

// levelBands buckets the org chart's depth profile. Levels are small ordinals, so the bands are
// narrow; the open-ended tail keeps a pathologically deep tree from minting unbounded buckets.
func levelBands() []Band {
	return []Band{
		{Key: "0-1", Lo: iptr(0), Hi: iptr(2)},
		{Key: "2-3", Lo: iptr(2), Hi: iptr(4)},
		{Key: "4-5", Lo: iptr(4), Hi: iptr(6)},
		{Key: "6-7", Lo: iptr(6), Hi: iptr(8)},
		{Key: "8+", Lo: iptr(8)},
	}
}

// ── membership (link__member_of) ─────────────────────────────────────────────
//
// The FIRST faceted reified link (D-Ontology): a membership is a relationship with its own identity,
// attributes and history, so it lists and filters exactly like an object. The token carries the
// link__ prefix because that is what the console's ontology registry is keyed by (rid.TokenOf).
//
// Every source column is pii:none or pii:basic, so ReadPermission stays empty throughout —
// membership.read is the whole decision. Unlike the per-unit roster and the per-person listing,
// which are hard-wired to status='active', the top-level list carries NO implicit status filter: a
// hidden default would make M57's totalCount disagree with its own status distribution.
func membershipType() ObjectType {
	return ObjectType{
		Type:          "link__member_of",
		Module:        "membership",
		ListEndpoint:  "MembershipService.listMemberships",
		StatsEndpoint: "MembershipService.membershipStats",
		Facets: []Facet{
			{
				Key:     "unitId",
				Kind:    KindRef,
				Table:   "oikumenea.membership_memberships",
				Column:  "unit_id",
				RefType: "unit",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "EXACT match, NOT subtree-expanding — the opposite of person.unitId. A membership " +
					"names the one unit the person belongs to; expanding here would double-count a " +
					"person against every ancestor and make the M57 headcount-by-unit chart lie.",
			},
			{
				Key:     "personId",
				Kind:    KindRef,
				Table:   "oikumenea.membership_memberships",
				Column:  "person_id",
				RefType: "person",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
			},
			{
				Key:     "positionId",
				Kind:    KindRef,
				Table:   "oikumenea.membership_memberships",
				Column:  "position_id",
				RefType: "position",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "Nullable — a membership without a billet is a plain belonging (D-Position).",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.membership_memberships",
				Column:  "status",
				Values:  []string{"active", "ended"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
			{
				Key:         "effectiveFrom",
				Kind:        KindDateRange,
				Table:       "oikumenea.membership_memberships",
				Column:      "effective_from",
				ArgOverride: []string{"effectiveFromAfter", "effectiveFromBefore"},
				Buckets:     Buckets{Strategy: StrategyDateTrunc, Grain: "month"},
				Note: "ArgOverride because the derived names would be `effectiveFromFrom`/" +
					"`effectiveFromTo` — the key already ends in the word a date-range appends. The " +
					"column is a timestamptz, not a date; the bounds are calendar dates compared " +
					"against the start/end of the given day (UTC), so a single day is selectable by " +
					"passing the same date twice. M57 buckets it by month — the intake curve.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── order ───────────────────────────────────────────────────────────────────
//
// Orders are unit-scoped on issuing_unit_id; every source column is pii:none, so ReadPermission
// stays empty and order.read is the whole decision.
func orderType() ObjectType {
	return ObjectType{
		Type:          "order",
		Module:        "order",
		ListEndpoint:  "OrderService.listOrders",
		StatsEndpoint: "OrderService.orderStats",
		Facets: []Facet{
			{
				Key:     "issuingUnitId",
				Kind:    KindRef,
				Table:   "oikumenea.order_orders",
				Column:  "issuing_unit_id",
				RefType: "unit",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note:    "EXACT match, not subtree-expanding. Every order is unit-issued (D-Orders), so never null.",
			},
			{
				Key:     "orderTypeId",
				Kind:    KindRef,
				Table:   "oikumenea.order_order_items",
				Column:  "type_id",
				RefType: "order_type",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "ANOTHER TABLE (still order's): an order's EFFECT lives on its items, so the " +
					"filter matches an order with at least one item of this type — an EXISTS " +
					"semi-join, never a join that would multiply the order across its items.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.order_orders",
				Column:  "status",
				Values:  []string{"draft", "issued", "revoked"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "M57 tones the revoked segment red and derives the revocation rate from this distribution.",
			},
			{
				Key:     "issuedOn",
				Kind:    KindDateRange,
				Table:   "oikumenea.order_orders",
				Column:  "issued_on",
				Buckets: Buckets{Strategy: StrategyDateTrunc, Grain: "month", IncludeUnknown: true},
				Note: "Nullable — a DRAFT order has no issue date, so the (unknown) bucket is the draft " +
					"backlog and is mandatory. Setting either bound EXCLUDES drafts (SQL three-valued logic).",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── document ────────────────────────────────────────────────────────────────
//
// Documents have no unit column: they are scoped THROUGH THE HOLDER (D-PersonReadScope), which is
// why the list endpoint folds a holder semi-join rather than a unit predicate.
//
// The faceted columns are all pii:none — the pii:basic `number`/`issuer` and the pii:special
// `attributes` bag are deliberately unfaceted. `attributes` is the pii:special CEILING for the
// long-tail per-type fields; it is a free-form bag, so the boundary there is policy rather than a
// code split, exactly as with person_persons.attributes (D-DataScope's residual).
func documentType() ObjectType {
	return ObjectType{
		Type:          "document",
		Module:        "document",
		ListEndpoint:  "DocumentService.listDocuments",
		StatsEndpoint: "DocumentService.documentStats",
		Facets: []Facet{
			{
				Key:     "typeId",
				Kind:    KindRef,
				Table:   "oikumenea.document_documents",
				Column:  "type_id",
				RefType: "document_type",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note:    "-> oikumenea.document_document_types, the instance-admin-managed catalog.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.document_documents",
				Column:  "status",
				Values:  []string{"active", "superseded", "revoked"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
			{
				Key:     "issuingCountryId",
				Kind:    KindRef,
				Table:   "oikumenea.document_documents",
				Column:  "issuing_country_id",
				RefType: "country",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "-> oikumenea.geo_countries (D-Geo). Nullable.",
			},
			{
				Key:     "issuedOn",
				Kind:    KindDateRange,
				Table:   "oikumenea.document_documents",
				Column:  "issued_on",
				Buckets: Buckets{Strategy: StrategyDateTrunc, Grain: "month", IncludeUnknown: true},
				Note:    "Nullable.",
			},
			{
				Key:     "expiresOn",
				Kind:    KindDateRange,
				Table:   "oikumenea.document_documents",
				Column:  "expires_on",
				Buckets: Buckets{Strategy: StrategyDateTrunc, Grain: "month", IncludeUnknown: true},
				Note: "Nullable, and the (unknown) bucket is the meaningful `(no expiry)` set — a " +
					"permanent document, not missing data. M57's `expiring soon` tile reads the " +
					"near-future buckets and tones past-due ones red.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── audit ───────────────────────────────────────────────────────────────────
//
// The first LEDGER type (M58), and the first that is not RID-typed: an audit row records one Action,
// and that Action's RID belongs to the service that produced it (tenant, person, …), so there is no
// `audit` token in pkg/rid to validate against — see ObjectType.Ledger for why inventing one would be
// worse than the exception.
//
// Three things differ from the five M57 types, and each is a property of the ledger rather than a
// special case:
//
//   - VISIBILITY IS RLS. `audit_log` is FORCE ROW LEVEL SECURITY with a unit_id-probing read policy,
//     and the transport gate is the coarse RequireAnywhere(audit.read). So the aggregate is ONE query
//     with no subject predicate — safe only because the read goes through the request-pinned
//     connection, where the app.* GUCs the policy reads are set. On the bare pool the same query
//     returns a confident ZERO (the M56/M57 bug shape the db source guard exists for).
//   - `since`/`until` PREDATE the vocabulary, exactly as membership's effectiveFromAfter/Before do,
//     so the createdAt facet pins them with an ArgOverride. They are Conjure `datetime`, not the
//     calendar `date` every other range facet takes — the console declares that per FilterDef and the
//     arg guard checks it, because sending a bare YYYY-MM-DD to them is a 400.
//   - THE TABLE IS MONTH-PARTITIONED and never stops growing, so an unfiltered aggregate scans every
//     partition. `since`/`until` are the only pruning lever there is, which is why the console's
//     dashboard link carries a default window as a REAL, visible, clearable filter rather than the
//     endpoint defaulting one behind the caller's back.
//
// Every column is pii:none — the pii:special ceiling is the before/after payload (D-PIITiers), which
// is not, and can never be, a facet.
func auditType() ObjectType {
	return ObjectType{
		// Type / Module / ListEndpoint / StatsEndpoint stay ADJACENT and in this order: the IR-mirror
		// generator matches them as one strict pattern, and a field wedged between them drops the whole
		// type from the mirror (now a hard error rather than a thinner mirror — genfacetargs).
		Type:          "audit",
		Module:        "audit",
		ListEndpoint:  "AuditService.query",
		StatsEndpoint: "AuditService.auditStats",
		Ledger: "the append-only ledger of Actions (D-Audit): its rows are the RECORDS of actions, " +
			"which list and filter like any collection, but each row's RID is minted by the producing " +
			"service (kind=action, generic type 0), so no `audit` type exists in pkg/rid to name it",
		Facets: []Facet{
			{
				Key:     "actorType",
				Kind:    KindEnum,
				Table:   "oikumenea.audit_log",
				Column:  "actor_type",
				Values:  []string{"person", "system"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "The two actor kinds (D-Audit). There is no super_admin kind — an instance admin is a person.",
			},
			{
				Key:     "actorPersonId",
				Kind:    KindRef,
				Table:   "oikumenea.audit_log",
				Column:  "actor_person_id",
				RefType: "person",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "NULL for system actions, whose actor is the `subsystem` string — the (unknown) bucket is exactly the system half of actorType.",
			},
			{
				Key:     "action",
				Kind:    KindCode,
				Table:   "oikumenea.audit_log",
				Column:  "action",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "The dotted action code. pkg/action's registry (R-29) is its value set, but the " +
					"column carries no CHECK — hence KindCode rather than an enum whose Values would " +
					"zero-fill 288 buckets.",
			},
			{
				Key:     "targetType",
				Kind:    KindCode,
				Table:   "oikumenea.audit_log",
				Column:  "target_type",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note:    "The acted-on entity kind (unit, person, role_assignment, …); open-set text, like `action`.",
			},
			{
				Key:     "targetId",
				Kind:    KindCode,
				Table:   "oikumenea.audit_log",
				Column:  "target_id",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "A FILTER facet with no chart, like link__member_of.personId: it is the drill-down " +
					"the object-history view uses. POLYMORPHIC (a RID uuid text OR a natural code — the " +
					"D-ResourceIdentifiers carve-out), so its buckets carry no labels and must not.",
			},
			{
				Key:     "outcome",
				Kind:    KindEnum,
				Table:   "oikumenea.audit_log",
				Column:  "outcome",
				Values:  []string{"success", "denied", "error"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "M58 tones the denied segment red — a denial rate is the number an auditor opens this dashboard for.",
			},
			{
				Key:     "unitId",
				Kind:    KindRef,
				Table:   "oikumenea.audit_log",
				Column:  "unit_id",
				RefType: "unit",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "The unit context the RLS read policy probes. NULL = a system / instance-plane " +
					"event, visible only to an instance admin — so the (unknown) bucket is empty for " +
					"everyone else BY THE POLICY, not by a Go-side trim.",
			},
			{
				Key:         "createdAt",
				Kind:        KindDateRange,
				Table:       "oikumenea.audit_log",
				Column:      "created_at",
				ArgOverride: []string{"since", "until"},
				Buckets:     Buckets{Strategy: StrategyDateTrunc, Grain: "day"},
				Note: "ArgOverride: `since`/`until` predate this vocabulary (the membership " +
					"effectiveFromAfter/Before case). They are Conjure DATETIME, not calendar dates, and " +
					"they are the partition-pruning lever for a month-partitioned ledger. DAY grain — " +
					"the first: an audit trail is read day by day, and a month bar would hide the spike " +
					"an auditor is looking for. NOT NULL, so no (unknown) bucket.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── external organization ───────────────────────────────────────────────────
//
// M58 ticket 2, and the first VERTICAL to reach the vocabulary. A flat instance-global reference
// table: no row-level security, no unit reach, no shadow flag — `externalorg.read` held anywhere is
// the whole visibility decision. So its aggregate ships ONE arm, like the audit ledger's, for the
// exact OPPOSITE reason: audit's single arm is a visibility decision made entirely by the connection
// the query runs on, this one is the absence of any visibility decision to make.
//
// Four of its six args already existed (M30, D-ExternalOrgs); `source`, `confidence` and the `asOf`
// range are new, and all three are the D-OverlayFoundation attribution column-set, which is why the
// confidence x source view is the chart this type exists to draw.
func externalOrgType() ObjectType {
	return ObjectType{
		Type:          "external_organization",
		Module:        "externalorg",
		ListEndpoint:  "ExternalOrganizationService.listExternalOrgs",
		StatsEndpoint: "ExternalOrganizationService.externalOrgStats",
		Facets: []Facet{
			{
				Key:     "kind",
				Kind:    KindRef,
				Table:   "oikumenea.external_organizations",
				Column:  "kind_id",
				RefType: "external_org_kind",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "The arg predates this vocabulary and took a kind CODE, while a ref bucket's key " +
					"is the kind's RID — so the filter was WIDENED to accept either spelling rather than " +
					"gaining a second arg. A bucket key must remain a usable filter value; that is the " +
					"whole reason, and it is the same widening religion's `religion` arg already had.",
			},
			{
				Key:     "countryId",
				Kind:    KindRef,
				Table:   "oikumenea.external_organizations",
				Column:  "country_id",
				RefType: "country",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				// The arg is `country` and already carried a RID, so only the NAME needed pinning.
				ArgOverride: []string{"country"},
				Note: "ArgOverride: the arg is `country` (M30), not the derived `countryId`. It already " +
					"took a country RID, so the bucket key was a usable filter value from the start. " +
					"Nullable — an external org may be supranational or simply unattributed.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.external_organizations",
				Column:  "status",
				Values:  []string{"provisional", "resolved"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "A provisional row is an unresolved import stub awaiting a merge into a canonical org.",
			},
			{
				Key:     "source",
				Kind:    KindEnum,
				Table:   "oikumenea.external_organizations",
				Column:  "source",
				Values:  []string{"self_declared", "operator_verified", "imported"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note: "The D-OverlayFoundation attribution column-set (conventions.md, Attribution). " +
					"Chart order is ascending authority, not alphabetical.",
			},
			{
				Key:     "confidence",
				Kind:    KindEnum,
				Table:   "oikumenea.external_organizations",
				Column:  "confidence",
				Values:  []string{"confirmed", "probable", "possible"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note: "The other half of the attribution column-set. Chart order is descending " +
					"certainty: crossed with `source` it is the OSINT attribution-quality view this " +
					"dashboard exists for, and a frequency sort would scramble both axes.",
			},
			{
				Key:     "asOf",
				Kind:    KindDateRange,
				Table:   "oikumenea.external_organizations",
				Column:  "as_of",
				Buckets: Buckets{Strategy: StrategyDateTrunc, Grain: "month", IncludeUnknown: true},
				Note: "When the asserted value was observed or held true — attribution, not row " +
					"lifetime (created_at is not faceted). Nullable, so an (unknown) bucket is mandatory " +
					"and reads as `asserted without an observation date`. Conjure DATETIME, like audit's " +
					"since/until, so the console declares argType datetime.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "query", Class: ClassSearch, Why: "case-insensitive name/code substring match", Drives: "ListOrgs"},
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── religion taxon ──────────────────────────────────────────────────────────
//
// M58 ticket 2, and the first TREE to reach the vocabulary — which is where the non-partitioning
// property comes from. `religion_taxa` is flat instance-global reference data (the row-level security
// in this module is on the unit-scoped religion_org_* tables, not here), so like external_organization
// it ships ONE aggregate arm because there is no visibility decision to make.
//
// Two of its four facets overlap by construction, and each carries its own reason. See
// Facet.NonPartitioning: what that exempts is the buckets-sum-to-totalCount assertion and nothing
// else — every bucket here still returns exactly the rows it counted when clicked.
func taxonType() ObjectType {
	return ObjectType{
		Type:          "taxon",
		Module:        "religion",
		ListEndpoint:  "ReligionService.listTaxa",
		StatsEndpoint: "ReligionService.taxonStats",
		Facets: []Facet{
			{
				Key:         "rankId",
				Kind:        KindRef,
				Table:       "oikumenea.religion_taxa",
				Column:      "rank_id",
				RefType:     "taxon_rank",
				ArgOverride: []string{"rank"},
				Buckets:     Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "ArgOverride: the arg is `rank` (M22) and took a rank CODE, so it was WIDENED to " +
					"accept the RID a bucket key carries rather than gaining a second arg. Ordered by " +
					"the rank's OWN ordinal via the SQL-supplied Ord (the rank-seniority path topNBuckets " +
					"already honours): religion -> branch -> tradition is a ladder, and re-sorting it by " +
					"frequency would destroy the only ordering that means anything.",
			},
			{
				Key:         "religionId",
				Kind:        KindRef,
				Table:       "oikumenea.religion_taxa",
				Column:      "religion_id",
				RefType:     "taxon",
				ArgOverride: []string{"religion"},
				Buckets:     Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "ArgOverride: the arg is `religion` (M22), which already accepted a root taxon RID " +
					"or its code. The denormalized root, derived via the closure — so unlike `subtree` " +
					"this one DOES partition: every taxon has exactly one root, and a root's own row has " +
					"none, which is the (unknown) bucket.",
			},
			{
				Key:         "subtree",
				Kind:        KindRef,
				Table:       "oikumenea.religion_taxa_closure",
				Column:      "ancestor_id",
				RefType:     "taxon",
				ArgOverride: []string{"parent"},
				Buckets:     Buckets{Strategy: StrategyTopN, TopN: 15},
				NonPartitioning: "a closure facet counts each taxon under EVERY ancestor it has, which " +
					"is what makes the chart drillable rather than a defect: the bucket's count is its " +
					"whole subtree size, clicking it returns exactly those rows, and re-grouping within " +
					"them yields that subtree's own internal nodes, recursively. The honest alternative " +
					"— grouping by parent_id and filtering to an exact parent — partitions cleanly and " +
					"then dead-ends after one click, because every remaining row shares one parent.",
				Note: "ArgOverride: the arg is `parent` (M22), which already meant PROPER descendants " +
					"via the closure. Counted with depth > 0 on both sides, so the reflexive (t,t,0) row " +
					"is excluded from the bucket exactly as it is from the click-through — otherwise the " +
					"two would disagree by precisely one row. The one facet whose Table is not the listed " +
					"table.",
			},
			{
				Key:         "classification",
				Kind:        KindRef,
				Table:       "oikumenea.religion_taxon_classifications",
				Column:      "classification_id",
				RefType:     "classification",
				ArgOverride: []string{"classification"},
				Buckets:     Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				NonPartitioning: "theism tags are M:N (a taxon may be both dualistic and monotheistic) " +
					"AND inherited, so a taxon is counted once per EFFECTIVE tag it resolves to. Counting " +
					"only directly-declared tags would partition, and would also be useless: tags are " +
					"declared on roots and inherited by everything below, so nearly every bucket would " +
					"be (unknown).",
				Note: "ArgOverride is the derived name, pinned because the arg and Key coincide only by " +
					"luck and the guard must check the pinning rather than assume it. EFFECTIVE tags, " +
					"resolved to the nearest DECLARING ancestor through religion_taxa_closure — the same " +
					"resolution getEffectiveClassifications performs, so the chart and the object view " +
					"agree. (unknown) = no declaring ancestor anywhere up the tree.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "query", Class: ClassSearch, Why: "case-insensitive code/name substring match", Drives: "ListTaxa"},
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── vehicle ─────────────────────────────────────────────────────────────────
//
// M58 ticket 3, and the third RAW-PGX module to reach the vocabulary (religion and externalorg were
// ticket 2's). It has no adapters/queries/*.sql for the sqlc-shaped parity guards to parse, so the
// list/stats agreement is proved the other way — one shared buildVehicleFilter and one named
// vehicleAggregate const, checked by AST in rawpgx_test.go. Adding a raw-pgx type is a TWO-place
// change: this block and rawPgxGroups.
//
// ONE aggregate arm, for external_organization's reason and NOT the audit ledger's. The two single-arm
// cases are not interchangeable and the distinction is load-bearing: audit's single arm IS a
// visibility decision, made entirely by the connection the query runs on (unpinned, it answers a
// confident zero). This one is the ABSENCE of a visibility decision — vehicle_vehicles has no
// row-level security, no unit column and no reach predicate, so `vehicle.read` held anywhere is the
// whole gate and there is nothing for a second arm to narrow.
//
// Every faceted column is pii:none. `vin` is pii:basic and deliberately unfaceted — it is an identity
// for one vehicle, not a distribution, and it is already the `query` search arg.
func vehicleType() ObjectType {
	return ObjectType{
		// Type / Module / ListEndpoint / StatsEndpoint stay ADJACENT and in this order — genfacetargs
		// matches them as one strict pattern and a field wedged between them is a hard error.
		Type:          "vehicle",
		Module:        "vehicle",
		ListEndpoint:  "VehicleService.listVehicles",
		StatsEndpoint: "VehicleService.vehicleStats",
		Facets: []Facet{
			{
				Key:     "typeId",
				Kind:    KindRef,
				Table:   "oikumenea.vehicle_vehicles",
				Column:  "type_id",
				RefType: "vehicle_type",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note:    "-> oikumenea.vehicle_types, the instance-extensible catalog. NOT NULL.",
			},
			{
				Key:     "brandId",
				Kind:    KindRef,
				Table:   "oikumenea.vehicle_models",
				Column:  "brand_id",
				RefType: "vehicle_brand",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "ANOTHER TABLE (still vehicle's), and TWO-HOP: a vehicle has no brand column — the " +
					"brand hangs off its model. vehicleSelect already LEFT JOINs vehicle_models and " +
					"projects m.brand_id, so this is the projection the list path has always returned, not " +
					"a new join. The (unknown) bucket is the vehicles with no model, which therefore have " +
					"no brand either.",
			},
			{
				Key:     "modelId",
				Kind:    KindRef,
				Table:   "oikumenea.vehicle_vehicles",
				Column:  "model_id",
				RefType: "vehicle_model",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "Nullable — a vehicle of a known type may have an unknown model.",
			},
			{
				Key:     "color",
				Kind:    KindRef,
				Table:   "oikumenea.vehicle_vehicles",
				Column:  "color_id",
				RefType: "color",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "-> oikumenea.platform_colors (domain='vehicle'), a HARD FK since M42/D-Color. It is " +
					"worth stating plainly because facets.md recorded the opposite for a while: the " +
					"ticket-2 survey read the CREATE TABLE and missed the ALTER 600 lines later in the same " +
					"consolidated migration, and reported free text where a real catalog FK had stood since " +
					"M42. The console tints this facet's bars from platform_colors.hex, which is only " +
					"honest because the key is a catalog RID. Nullable.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.vehicle_vehicles",
				Column:  "status",
				Values:  []string{"active", "scrapped", "exported"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
			{
				Key:    "manufactureDate",
				Kind:   KindDateRange,
				Table:  "oikumenea.vehicle_vehicles",
				Column: "manufacture_date",
				Buckets: Buckets{
					Strategy:       StrategyDateTrunc,
					Grain:          "month",
					IncludeUnknown: true,
				},
				Note: "A calendar DATE column, not a timestamptz — so the bounds are plain YYYY-MM-DD and " +
					"the console's month bucket inverse needs NO RFC-3339 widening, the opposite of " +
					"external_organization.asOf. Nullable, so the (unknown) bucket is mandatory and reads " +
					"as `manufacture date not recorded`; setting either bound EXCLUDES those rows (SQL " +
					"three-valued logic). M58 buckets it by month — the fleet-age curve.",
			},
			{
				Key:     "registrationCountry",
				Kind:    KindRef,
				Table:   "oikumenea.vehicle_registrations",
				Column:  "country_id",
				RefType: "country",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "ANOTHER TABLE (still vehicle's), and the one facet here that had to choose a SET. " +
					"vehicle_registrations is ownership HISTORY — one-to-many, so grouping it raw would " +
					"count a re-registered vehicle under every country it has ever worn plates in, and " +
					"would need NonPartitioning. It is instead confined to the ACTIVE registration, of " +
					"which CloseActiveRegistrationsForVehicle guarantees at most one per vehicle, so the " +
					"distribution PARTITIONS honestly and NonPartitioning is neither taken nor needed. " +
					"That is the person.rankId precedent (match the active row), and it is also the " +
					"question the chart is read for: where is this fleet registered NOW. Matched as an " +
					"EXISTS semi-join, never a join. The (unknown) bucket is never-registered or " +
					"deregistered vehicles.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "query", Class: ClassSearch, Why: "case-insensitive VIN substring match", Drives: "ListVehicles"},
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── finance account ─────────────────────────────────────────────────────────
//
// M58 ticket 3. Raw-pgx, like vehicle; ONE aggregate arm, for the same absence-of-a-decision reason.
//
// The pii:sensitive columns — iban_ciphertext, iban_wrapped_dek, key_ref, iban_blind_index — are
// deliberately unfaceted and CANNOT be faceted: there is no plaintext to GROUP BY, and D-DataScope's
// aggregation rule forbids the surface independently of that (rule 1, asserted in plaintext_test.go).
// The blind index is technically groupable and is still not a facet: it is a per-value HMAC, so its
// distribution is a row count per distinct IBAN and its buckets would BE the identifiers.
func accountType() ObjectType {
	return ObjectType{
		Type:          "account",
		Module:        "finance",
		ListEndpoint:  "FinanceService.listAccounts",
		StatsEndpoint: "FinanceService.accountStats",
		Facets: []Facet{
			{
				Key:     "institutionId",
				Kind:    KindRef,
				Table:   "oikumenea.finance_accounts",
				Column:  "institution_id",
				RefType: "organization",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15},
				Note: "-> oikumenea.tenant_organizations. The holding BANK is a `company`-domain tenant " +
					"organization (M21/M41, D-UnifiedOrgGraph), never a finance-owned entity — which is why " +
					"the RefType is `organization` and the buckets label through the tenant labeler. NOT NULL.",
			},
			{
				Key:     "currency",
				Kind:    KindCode,
				Table:   "oikumenea.finance_accounts",
				Column:  "currency",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note: "ISO 4217. KindCode rather than enum: the column carries NO CHECK, so the value set is " +
					"open — the audit.action case. The key is its own label (`UAH` reads as itself), so " +
					"nothing is resolved to draw the chart. Nullable.",
			},
			{
				Key:     "accountTypeId",
				Kind:    KindRef,
				Table:   "oikumenea.finance_accounts",
				Column:  "account_type_id",
				RefType: "account_type",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "-> oikumenea.finance_account_types, the instance-extensible catalog. Nullable.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.finance_accounts",
				Column:  "status",
				Values:  []string{"active", "closed", "frozen"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note:    "M58 tones the frozen segment red — a frozen account is the one an analyst opens this dashboard for.",
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}

// ── finance card ────────────────────────────────────────────────────────────
//
// M58 ticket 3, and the first type whose COLLECTION-LEVEL LIST this vocabulary had to add: cards were
// reachable only per-account (GET /accounts/{accountId}/cards), so there was no collection for a
// dashboard to describe. The per-account list is now listAccountCards, beside listAccountHolders, and
// the plain listCards is the registry — the name every other faceted type's list endpoint carries.
//
// The new list is METADATA ONLY and that is a compliance boundary, not a convenience: retained PANs
// put this table in PCI-DSS CDE scope (D-DataScope). pan_ciphertext / pan_wrapped_dek / key_ref /
// pan_blind_index are unfaceted and unlisted; the PAN is decrypted only by getCard, one card at a
// time, for an authorized caller. `bin` and `last_four` are CLEAR columns and are still not facets:
// they identify one card rather than describing a population, and a top-N over last_four is a
// meaningless ranking of four-digit suffixes.
func cardType() ObjectType {
	return ObjectType{
		Type:          "card",
		Module:        "finance",
		ListEndpoint:  "FinanceService.listCards",
		StatsEndpoint: "FinanceService.cardStats",
		Facets: []Facet{
			{
				Key:     "networkId",
				Kind:    KindRef,
				Table:   "oikumenea.finance_cards",
				Column:  "network_id",
				RefType: "card_network",
				Buckets: Buckets{Strategy: StrategyTopN, TopN: 15, IncludeUnknown: true},
				Note:    "-> oikumenea.finance_card_networks, the instance-extensible catalog. Nullable.",
			},
			{
				Key:     "cardType",
				Kind:    KindEnum,
				Table:   "oikumenea.finance_cards",
				Column:  "card_type",
				Values:  []string{"debit", "credit"},
				Buckets: Buckets{Strategy: StrategyIdentity},
				Note: "The Key is `cardType`, not `type`: the arg sits beside `status` and `networkId` on a " +
					"card endpoint, where a bare `type` would read as the card's network.",
			},
			{
				Key:     "status",
				Kind:    KindEnum,
				Table:   "oikumenea.finance_cards",
				Column:  "status",
				Values:  []string{"active", "blocked", "expired"},
				Buckets: Buckets{Strategy: StrategyIdentity},
			},
		},
		NonFacetArgs: []NonFacetArg{
			{Arg: "pageSize", Class: ClassPaging, Why: "keyset page size (pkg/listing clamp)"},
			{Arg: "pageToken", Class: ClassPaging, Why: "keyset cursor (pkg/listing codec)"},
		},
	}
}
