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
	return []ObjectType{personType(), unitType(), membershipType(), orderType(), documentType()}
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
				Key:         "level",
				Kind:        KindNumericRange,
				Table:       "oikumenea.tenant_units",
				Column:      "level",
				ArgOverride: []string{"level"},
				Buckets: Buckets{
					Strategy:       StrategyBands,
					Bands:          levelBands(),
					IncludeUnknown: true,
				},
				Note: "The contract already ships a SCALAR exact-match `level` arg that predates this " +
					"vocabulary, and the contract is expand-only, so the facet pins it rather than " +
					"minting levelMin/levelMax. M57 bands the same column; adding the range args is " +
					"additive and deferred to when the bands are actually consumed.",
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
