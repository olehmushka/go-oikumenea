# Facets — the shared filter/aggregation vocabulary and the dashboard catalog

> **Status: the M57 tranche is COMPLETE and verified.** The kernel is built — `person` and `unit`
> (M56 ticket 2), `membership` / `order` / `document` (M56 ticket 3, alongside their new top-level
> list endpoints), and all five reach the console as URL-borne list filters (M56 ticket 4). All five
> have stats endpoints (M57 tickets 1–2) and all five have **dashboards** — which are the **default
> view** of their collection, with `?view=table` as the opt-out (M57 ticket 3) — verified live
> end-to-end (ticket 4). The remaining types are M58. Binding
> design lives in
> [`decisions.md` → D-ObjectFacets](decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope)
> and [D-ConsoleDashboards](decisions.md#d-consoledashboards--every-listable-type-gets-a-list-view-and-a-dashboard-view-over-one-url-borne-filter-set-amends-d-webui);
> this note is the readable overview **and the catalog** — the per-object-type list of every facet and
> every chart. It becomes binding-against-code as each milestone enters implementation.
>
> **This catalog is the artifact to argue with.** Each dashboard component below is a proposal;
> changing one is a doc edit here, not a code change.

## What a facet is

A **facet** is one declared, filterable, groupable dimension of an object type — declared **once**, by
the module that owns the table, and consumed **twice**:

| Consumer | Shape |
|---|---|
| **List filter** | a typed optional Conjure query arg on the owning module's list endpoint |
| **Stats bucket** | a bucket set on that module's `GET /<module>/v1/stats/<collection>` |

> The stats path is `/stats/<collection>`, not the `/<collection>/stats` this catalog and
> D-ObjectFacets originally wrote: httprouter refuses a literal path segment beside `{id}` at the same
> position and **panics at server startup**, so the specified shape was unroutable. A contract-wide
> guard (`internal/platform/transport/route_conflict_test.go`) now fails the build on that shape.

Both take the **same argument names and the same values**, which is the whole point: a chart segment
and a list filter are the same act, so toggling between the console's list and dashboard views
carries the filter set across, and clicking a bar narrows the list.

```
Facet {
  key             // the query-arg name AND the groupBy token, e.g. "sex", "unitKind"
  kind            // enum | ref | date-range | bool | numeric-range
  column          // the plaintext source column
  readPermission  // inherited gate; "" = the endpoint's own read code is the whole decision
  buckets         // how values become buckets (identity, ranges, date_trunc, top-N + other)
  refType         // ref facets only: the object token the bucket RIDs point at, for labels
}
```

Facet kinds and what each contributes:

| Kind | Filter arg | Buckets | Example |
|---|---|---|---|
| `enum` | one value from a `TEXT`+`CHECK` set | identity, one per allowed value (zero-count buckets included, so a chart's shape is stable) | `person.sex`, `unit.visibility` |
| `ref` | an RID | top-N by count + an `other` bucket; labels resolved as `locale → text` maps (D-i18n) | `person.rankId`, `document.typeId` |
| `date-range` | `<key>From` / `<key>To` (RFC 3339 / `date`) | `date_trunc` to a caller-chosen grain, or named bands | `order.issuedOn`, `person.birthdate` |
| `bool` | `true`/`false` | two buckets | `unit.pdpScoped` |
| `numeric-range` | `<key>Min` / `<key>Max` | fixed-width bands | `unit.level` |
| `code` | one plaintext code | top-N by count + `other`; **the key is its own label** (nothing to resolve) | `audit.action`, `audit.targetType` |

## The three rules that keep a facet legal

Restated from D-ObjectFacets because they are easy to violate by accident when adding a facet:

1. **Plaintext only.** A facet may name only a plaintext column. Every envelope-encrypted
   `pii:special` value is stored as ciphertext + blind index — there is nothing to `GROUP BY`, and
   [D-DataScope](decisions.md#d-datascope--what-a-deployment-may-hold-the-product-is-a-personnel-directory--registry-platform-owns-the-pci-dss-posture)'s
   aggregation rule forbids the surface regardless. Asserted at build time.
2. **A facet above `pii:basic` inherits its field's own read code**, and a caller lacking it gets that
   facet **omitted** from the stats response — not a zeroed bucket, not a 403 (the D-UnifiedSearch
   "skip the provider" behaviour).
3. **Counted inside the visibility predicate.** Person-scoped counts fold in the reach semi-join
   (`VisiblePersonIDsForSubjectSparse`/`Dense`); unit-scoped counts fold the shadow gate into SQL —
   `gateUnits` trims *after* the page is cut, which is right for a list and wrong for a count.

There is deliberately **no bucket-size suppression**: every counted row is a row the caller may
already read and page through the same list endpoint under the same filters, so a k-anonymity floor
would protect nothing it does not already protect.

---

## Catalog

Grouped by the milestone that lands it. "Components" are the dashboard tiles/charts, each rendered
from one facet distribution unless noted; every one is click-through — a segment is a link to the same
URL with one more filter applied.

### M57 tranche — the operational core

#### `person` — [person](../modules/person.md) · `person_persons`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `sex` | enum | `sex` | ISO/IEC 5218 (`not_known`/`male`/`female`/`not_applicable`) |
| `status` | enum | `status` | `active`/`deactivated`/`purged`/`provisional` |
| `birthdate` | date-range | `birthdate` | nullable — a `(unknown)` bucket is mandatory, not optional |
| `countryOfBirth` | ref | `country_of_birth_id` → `geo_countries` | |
| `rankId` | ref | `person_ranks.rank_id` (active row) | one rank per rank system — the facet is per-system |
| `unitId` | ref | `membership_memberships.unit_id` (active) | subtree-expanding: filtering by a unit includes its closure descendants over **every authority-bearing graph**. `listPersons` also takes a `graph` arg that narrows the expansion to one graph; it is a traversal arg, not a facet, and is rejected on its own. The expansion is an **uncorrelated** closure set — see [review-2026-07](review-2026-07.md#m56-ticket-2--person-facet-filters-2026-07-28) |
| `hasAccount` | bool | `account_accounts` semi-join | account-optional directory (L-AccountOptional) |

**Components.** ① **Age pyramid** — horizontal histogram of `birthdate` age bands (`0–17, 18–24,
25–34, 35–44, 45–54, 55–64, 65+, unknown`) split by `sex`, the two sexes mirrored about a centre axis;
the canonical personnel-structure view. **As built it is the one chart that costs more than one
request:** a distribution is per-facet and this is a cross-tab, so each wing is the same request state
plus `sex=<value>` and `facets=birthdate` — which is not a workaround but the design working, since a
wing is exactly the list its bar links to. When `sex` is already filtered the pyramid collapses to the
single-series band histogram rather than drawing one empty wing. Both wings share ONE x scale; scaling
each to its own maximum would make an 80/20 split look symmetric, which is the thing the chart exists
to show. ② **Sex donut** with an explicit `not_known` slice — the slice
is the data-quality signal, so it is never hidden. ③ **Status tiles** — four `StatTile`s
(active/deactivated/provisional/purged) with `totalCount` as the headline. ④ **Rank distribution** —
vertical bar ordered by **rank seniority, not by count** (rank is an ordered scheme; sorting by
frequency would destroy the only ordering that means anything). As built: the top-15 cutoff still
SELECTS by count — that is what `topN` means — and the resulting buckets are then ORDERED by the
scheme's own ordinal (category → type → rank sort_order), which the query supplies per bucket. So the
chart reads as a seniority profile over the fifteen most-held ranks. ⑤ **Top units** — top-15 bar +
`other`. ⑥ **Country of birth** — top-15 bar + `other`.

> No facet over ethnicity, party membership, political leaning, religious affiliation, health or
> legal records. See [What has no facet](#what-has-no-facet).

#### `unit` — [tenant](../modules/tenant.md) · `tenant_units`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `org` | ref | `org_id` | **required** today (`listUnits` rejects an unscoped listing) |
| `domain` | ref | `domain_id` | |
| `unitKind` | ref | `kind_id` | domain-scoped catalog |
| `level` | numeric-range | `level` | binds `levelMin`/`levelMax` (inclusive), which is what makes the band chart click-through — a band is a RANGE, and the **scalar exact-match `level`** the contract has always shipped cannot express one. That scalar is retained and still honoured (expand-only) and is classified `superseded`; all three predicates are ANDed. Landed M57 ticket 3, having been deferred at M56 "to when the bands are consumed" |
| `visibility` | enum | `visibility` | `public`/`shadow` |
| `state` | enum | `state` | `active`/`suspended`/`archived` |
| `pdpScoped` | bool | `pdp_scoped` | operational vs reference units (D-UnifiedOrgGraph) |

> `graph` is **not** a facet. It selects which DAG `parent`/`rootsOnly` walk and adds no predicate to
> `tenant_units` — there is no `tenant_units.graph_id` to filter or `GROUP BY`. M56 classifies it as a
> traversal arg, which is what the drift guard checks it against.

**Components.** ① **Units per level** — bar, level ascending; the org chart's width profile. Each bar
links to `levelMin`/`levelMax` bracketing its band; those args exist *because* this chart does (the
scalar `level` matches one level, a band is two). The equality is asserted, not assumed:
`TestUnitStatsLevelBandsAreClickThrough` holds every band's bucket against the row count its own bar's
filter returns. ②
**Kind mix** donut. ③ **Public/shadow split** — a two-segment bar, not a donut: the shadow count is a
governance number an operator reads exactly, so the label carries the count. ④ **State tiles**. ⑤
~~**Headcount by unit**~~ — **NOT BUILT (M57 ticket 3), and it is a contract gap rather than a
console omission.** The unit dashboard is org-scoped (`org` is a *required* filter) but
`membershipStats` ships no `org` arg, and `membership.unitId` is an exact-match facet — so no
membership query can be narrowed to an organization, and the chart would have shown units from other
orgs beneath an org-filtered dashboard. Recorded in [Open seams](#open-seams) with the two routes out.

#### `membership` (token `link__member_of`) — [membership](../modules/membership.md) · `membership_memberships`

> The **first faceted reified link**. A link is a first-class row with its own identity, attributes
> and history (D-Ontology), so it lists and filters exactly like an object; `pkg/facet` accepts
> object and link tokens, and the token carries the `link__` prefix because that is what the console's
> ontology registry is keyed by. Actions remain non-listable.
>
> The top-level `GET /memberships` carries **no implicit status filter**, unlike the per-unit roster
> and the per-person listing, which are hard-wired to `status='active'`. A hidden default would make
> M57's `totalCount` disagree with its own status distribution, and would leave ended memberships
> unreachable through any endpoint.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `unitId` | ref | `unit_id` | EXACT match, **not** subtree-expanding — the opposite of `person.unitId`. A membership names the one unit the person belongs to; expanding would double-count a person against every ancestor and make the M57 headcount-by-unit chart lie |
| `personId` | ref | `person_id` | |
| `positionId` | ref | `position_id` | nullable — membership without a billet is legal |
| `status` | enum | `status` | `active`/`ended` |
| `effectiveFrom` | date-range | `effective_from` | Args are `effectiveFromAfter`/`effectiveFromBefore`, not the derived `…From`/`…To` — the key already ends in the word a date-range appends. The column is a `timestamptz`; the bounds are calendar dates compared against the start/end of the given day, so passing one date to both selects that day |
| ~~`positionState`~~ (positions) | enum | `status` + fill state | **DEFERRED.** It is sourced from `membership_positions`, a different table behind a different endpoint (the per-unit `listPositions`), so it belongs to a `position` object type rather than to `link__member_of`. Declaring it here would have required a facet whose `Table` is unrelated to its list endpoint. Lands with `position` in M58 |

**Components.** ① **Active vs ended tiles**. ② **Joins per month** — `date_trunc('month',
effective_from)` histogram; the intake curve. ③ **Vacant vs filled positions** donut — the staffing
gap, the number this module exists to answer. ④ **Tenure histogram** — bands over
`now() - effective_from` for active rows.

#### `order` — [order](../modules/order.md) · `order_orders`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `issuingUnitId` | ref | `issuing_unit_id` | |
| `orderTypeId` | ref | `order_order_items.type_id` | An order's *effect* lives on its items, so the filter is an `EXISTS` semi-join — never a join, which would multiply the order across its items and corrupt the keyset page |
| `status` | enum | `status` | `draft`/`issued`/`revoked` |
| `issuedOn` | date-range | `issued_on` | Nullable — a **draft** order has no issue date, so the `(unknown)` bucket is the draft backlog and any bound excludes drafts |

**Components.** ① **Orders per month** histogram. ② **Type mix** bar — which administrative effects
this org actually issues. ③ **Draft/issued/revoked tiles**, revoked toned `red`. ④ **Revocation rate**
— a single `StatTile` (revoked ÷ issued), the audit-facing number.

#### `document` — [document](../modules/document.md) · `document_documents`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `typeId` | ref | `type_id` | |
| `status` | enum | `status` | `active`/`superseded`/`revoked` |
| `issuingCountryId` | ref | `issuing_country_id` | |
| `issuedOn` | date-range | `issued_on` | |
| `expiresOn` | date-range | `expires_on` | nullable — `(no expiry)` bucket |

**Components.** ① **Expiring soon** — a `StatTile` (expiring within 90 days) over a histogram of
`expires_on` by month, past-due bars toned `red`; the one component with an operational deadline
attached, so it leads. As built the tile is a **second bounded count** (`expiresOnFrom=today`,
`expiresOnTo=today+90d`, `facets=` — the total alone), not a sum of month buckets: the window the tile
claims does not fall on month boundaries, so deriving it from the histogram would be off by up to two
months. It carries a `Sparkline` of the next twelve months, so the number reads as a window on a
curve. ② **Type mix** bar. ③ **Status donut**. ④ **Issuing country** top-15 bar.

### M58 tranche — the verticals and reference plane

Facets and components in brief; each is expanded to the table form above when its session starts.

| Type | Facets | Components |
|---|---|---|
| **organization** (`tenant_organizations`) | `domain`, `visibility`, `state` | Orgs per domain bar · state tiles |
| **company** (`company_org_profiles`) | `legalForm`, `ownershipCategory`, `countryId`, `industryClass`, `foundedOn`(range), `state` | Legal-form bar · ownership donut · industry (NACE) top-15 bar · foundings-per-year histogram · country bar |
| **vehicle** (`vehicle_vehicles`) | `typeId`, `brandId`, `modelId`, `color`, `status`, `manufactureDate`(range), `registrationCountry` | Type mix bar · brand top-15 bar · fleet-age histogram · colour bar (**bars coloured by the `platform_colors` hex**, the one place the palette is the data) · status tiles |
| **finance-account** (`finance_accounts`) | `institutionId`, `currency`, `accountTypeId`, `status` | Accounts per bank bar · currency donut · type mix bar · status tiles |
| **finance-card** (`finance_cards`) | `networkId`, `cardType`, `status` | Network bar · debit/credit donut · status tiles |
| **institution** (`education_org_profiles`) | `kindId`, `countryId`, `foundedOn`(range), `state` | Kind mix bar · country bar · state tiles |
| **enrollment** (`person_education_enrollments`) | `institutionId`, `programId`, `degreeLevelId`, `status`, `startedOn`(range) | Enrollments per intake histogram · degree-level bar (ISCED-ordered, **not** count-ordered) · status tiles |
| **religion-taxon** (`religion_taxa`) | `rankId`, `parent`, `religionId`, `classification` | Taxa per rank bar · per-religion tree-size bar · theism classification donut |
| **external-org** (`external_organizations`) | `kind`, `country`, `status`, `source`, `confidence`, `asOf`(range) | Kind mix bar · country bar · provisional/resolved tiles · **confidence × source heat bar** (the OSINT attribution quality view, D-OverlayFoundation) |
| **location** (`location_locations`) | `countryId`, `typeId`, `hasCoordinate` | Locations per country bar · type mix bar · geocoded-vs-not tile |
| **languoid** (`language_languoids`) | `level`, `macroarea`, `status`, `family` | Level mix donut · macroarea bar · endangerment-status bar (**ordered by severity**) |
| **assignment** (`authz_role_assignments`) | `roleId`, `targetUnitId`, `scope`, `graphId`, `active`, `expiresAt`(range) | Grants per role bar · unit-vs-subtree donut · expiring-soon tile + histogram · active-vs-revoked tiles |

#### `audit` — [audit](../modules/audit.md) · `audit_log` — **BUILT (M58 ticket 1)**

The first **ledger**: an audit row records one Action, and that Action's RID belongs to the service
that produced it, so there is no `audit` token in `pkg/rid` for the catalog to validate against.
`ObjectType.Ledger` carries the reason and is the only escape from the token check; the kind rule
itself is untouched — an action *invocation* is still not listable, but the RECORD of one is a row in
a collection like any other.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `actorType` | enum | `actor_type` | `person` / `system`. Carried on the wire as a Conjure ENUM, which is legal because a generated enum upper-cases on unmarshal — so a bucket key (`person`, the DB's own spelling) is still a usable filter value |
| `actorPersonId` | ref | `actor_person_id` → person | Nullable; the `(unknown)` bucket is exactly the system half of `actorType` |
| `action` | code | `action` | The dotted action code. `pkg/action`'s registry (R-29) is its value set, but the column carries no CHECK — an enum whose `Values` zero-filled 288 buckets would be absurd |
| `targetType` | code | `target_type` | The acted-on entity kind; open-set text |
| `targetId` | code | `target_id` | A FILTER facet with no chart (the `link__member_of.personId` precedent) — polymorphic, so its buckets carry no labels and must not |
| `outcome` | enum | `outcome` | `success` / `denied` / `error` |
| `unitId` | ref | `unit_id` → unit | The column the RLS read policy probes. NULL = a system / instance-plane event, visible only to an instance admin — so that bucket is empty for everyone else BY THE POLICY, not by a Go-side trim |
| `createdAt` | date-range | `created_at` | **ArgOverride `since`/`until`** (they predate the vocabulary, the membership case). Conjure **datetime**, not calendar dates, so the console declares `argType: "datetime"` and widens a picked day to its RFC-3339 endpoints. `dateTrunc` at **DAY** grain — the first |

**Components.** ① **Outcome donut**, denied toned `red` — a denial rate is the number an auditor opens
this dashboard for. ② **Actions per day** histogram; day rather than month, because a monthly bar hides
the spike an audit trail is read for. ③ **Top actions** bar. ④ **Top actors** bar, whose unlabelled
bucket is the system actions.

> **Two things are structurally different here, and both are properties of the ledger.**
> **Visibility is RLS**: `audit_log` is `FORCE ROW LEVEL SECURITY` with a `unit_id` reach probe, and
> the transport gate is the coarse `audit.read`-anywhere — so the aggregate ships **ONE arm**, with no
> subject and no scoped twin. The whole of the visibility decision is which connection it runs on; on
> the bare pool it answers a confident zero, which is what the `db` source guard now covers for audit
> too. And the table is **month-partitioned and unbounded**: `since`/`until` are the only pruning
> lever, so the console's dashboard link carries a **30-day default window in the URL** — a visible,
> clearable chip rather than a server-side default that would make `totalCount` disagree with the
> caller's own filters.

`audit` was the cheapest and most convincing first M58 module for the reason the milestone gave (its
filter args already existed) and for one it did not: every way it differs from the M57 five is a
place where the kernel had quietly assumed something — an RID-typed collection, a closed value set,
an app-layer visibility predicate, a month-grain axis. All four assumptions are now named.

---

## What has no facet

Not an omission — an invariant (D-ObjectFacets rule 1). These columns are envelope-encrypted with a
blind index; there is no plaintext to group, and grouping them is precisely the join D-DataScope's
aggregation rule exists to prevent:

| Surface | Table | Tier |
|---|---|---|
| Declared ethnicity | `person_ethnicities` | `pii:special` (Art. 9) |
| Party membership | `person_party_memberships` | `pii:special` (Art. 9) |
| Inferred political leaning | `person_political_leaning` | `pii:special`, inferred |
| Religious affiliation | `religion_affiliations` | `pii:special` (Art. 9) |
| Health record detail | `person_health_records` | `pii:special` (Art. 9) |
| Legal record offence detail | `person_legal_records` | `pii:special` (Art. 10) |
| IBAN / PAN | `finance_accounts`, `finance_cards` | PCI-DSS CDE |

Plaintext discriminators that sit *beside* those encrypted values (`person_health_records.kind`,
`person_legal_records.kind`/`disposition`, `finance_cards.card_type`) **may** be faceted, but only
under rule 2 — each inherits its surface's own read code (`person.health.read`,
`person.legal-record.read`, `finance.read`), so the facet is simply absent for a caller without it.
`person_persons.attributes` is a free-form `pii:special` bag and is **never** faceted: the boundary
there is policy, not a code split (D-DataScope's residual).

## The two plan shapes a scoped list ships

Not a detail — it is why every scoped list endpoint carries **two** SQL queries rather than one, and
the ticket-3 measurement is what forced it (see
[review-2026-07](review-2026-07.md#m56-ticket-3--top-level-list-endpoints-2026-07-29)).

Folding visibility into SQL can be done two ways, and neither is safe alone:

| | **set form** — `unit_id IN (SELECT authz_readable_units(subject))` | **point probe** — `authz_unit_readable_by(unit, subject)` |
|---|---|---|
| leaf reach (1 unit) | 1.3 ms | 2 500 – 13 100 ms |
| mid reach (658) | 27 – 122 ms | 128 – 162 ms |
| root reach (100 000) | 640 – 6 400 ms | 3.6 – 6.3 ms |

The set form materializes the reach and semi-joins it — right when the reach is small. At root it
makes the planner drive from the *reach* side, build a ~10⁶-row hash and top-N sort, so the `LIMIT`
never terminates early. The point probe leaves the driving table in keyset order and asks the
question per candidate row — right when nearly every row qualifies, catastrophic when almost none do,
because it scans the table to find out.

So the adapters **dispatch on reach cardinality** (`authz_readable_unit_count`, capped at the
threshold), exactly as `VisiblePersonIDsForSubject*` has since R-02.1. All three reach forms are SQL
**functions** defined once in migration `0017`, not inlined per query: their parity with the Go PDP
oracle is the most safety-critical invariant in the codebase, and a differential test holds the
functions, the inline CTE and the oracle to one answer over randomized worlds.

## What the build-time guards actually check

Since M56 ticket 2 the three rules above are not prose. `pkg/facet/plaintext_test.go` parses
`migrations/*.sql` (no database) and fails the build when a facet names a `pii:special` column or an
envelope-encryption artefact, when a facet above `pii:basic` leaves `readPermission` empty, or when a
nullable column omits its `(unknown)` bucket — plus a contrapositive sweep asserting that **no**
`pii:special` column anywhere in the schema carries a facet, so the guard fires whether someone adds
the facet or downgrades the tier. `pkg/facet/args_test.go` holds the vocabulary and the Conjure
contract in agreement in both directions. Editing this catalog therefore changes behaviour; it is no
longer only a document.

Ticket 4 adds the **third consumer** to the same discipline: `pkg/facet/console_test.go` parses
`web/src/lib/ontology/registry.ts` (the same no-database technique, applied to TypeScript) and holds
each type's `FilterDef[]` against the catalog in **both** directions — a facet the API filters on may
not be one the console silently omits, and a `FilterDef` may not name an arg the contract does not
ship (checked against the IR mirror, not only against the catalog, so a hand-re-derived
`levelMin`/`levelMax` is caught at the console). Because it is a regex over TypeScript, a
non-vacuity floor asserts the parse actually found every registered type's block and every declared
facet's def: a broken parse goes red rather than turning the other assertions into vacuous passes.
Console-side omissions must be declared in `consoleExempt` with a reason — the `NonFacetArg.Why`
idiom — so the mechanism cannot decay into an allowlist. `filters:` blocks must therefore stay
literal `key`/`kind`/`params` literals; the constraint is stated in `registry.ts` beside the
interface, where the edit happens.

M58 ticket 1 adds three more checks, each earned by something the audit ledger does that nothing else
does: **`Ledger` must explain itself** (a reason of substance, a token that is NOT registered, and no
more than one such type in the catalog — a second ledger is a conversation, not a copy-paste); a
**`code` facet may not name a RefType** (its key is its own label, so a labeler would be a promise
never kept); and the console's **`argType`** must match the contract's (a `datetime` arg the console
does not know about produces a control and a chart link that both 400).

The **non-facet classes** gained a fourth in the same ticket: `superseded`, for an arg a facet's own
args have replaced and that the expand-only contract cannot remove (`unit.level`). Its check is
earned like the others — the named successor must exist, be a RANGE facet over the same column, take
the same Conjure type, and not itself bind the arg — so the class can excuse a genuine predecessor
and nothing else.

M57 ticket 3 adds the **fourth** consumer to the same discipline: `pkg/facet/dashboard_test.go` parses
the registry's `dashboard:` blocks and holds every `ChartDef` against the catalog. The failure it
guards is worse than a missing filter — the console asks for exactly the facets it draws
(`?facets=a,b,c`), so a key the type no longer declares is a **400 on the whole request**: one stale
chart blanks the entire dashboard rather than itself. It also checks the dashboard's `path` against
the **Conjure YAML** (`base-path` + the endpoint's `http:` line), because a hand-typed
`/documents/stats` is the exact shape httprouter refuses and no unit test would otherwise see it;
that a `tone:` key still names a value of its enum; that a pyramid's `splitBy` param is a real facet
arg; and that the console's `buckets:` declaration matches the catalog's bucket **strategy**, since
the click-through inverts a bucket key back into a filter and the inverse of an age band is not the
inverse of a calendar month. Same non-vacuity floor, same live-negative fixtures.

## Open seams

- **Cross-type dashboards.** Every dashboard is single-type by construction (per-module stats
  endpoints, D-ObjectFacets). "Persons by unit *and* by rank at once" is a two-facet cross-tab, not
  supported; a genuine cross-type roll-up would want the fan-in service D-ObjectFacets rejected, and
  should be re-argued on evidence rather than assumed. **What M57 ticket 3 showed is that the cheap
  case is already covered without one:** a cross-tab against an *enum* facet is N extra requests, each
  the same request state plus one filter value (the age pyramid is two). That works because a wing is
  a real, reachable list — it does not generalize to a high-cardinality ref, where N is the
  cardinality.
- **A dashboard chart that needs another module's stats.** `unit` ⑤ headcount-by-unit is the only
  catalogued component of the M57 tranche that was NOT built, because it cannot be drawn honestly: the
  unit dashboard is org-scoped and `membershipStats` has no `org` arg, so the chart would mix
  organizations. Two routes out, both additive: declare an `org` facet on `link__member_of` (the
  membership row has no `org_id`, so it would be a join through `tenant_units` — a facet whose Table
  is not the listed table, which the catalog does not do today), or draw it from `person`'s stats
  filtered by the org's root unit, since `person.unitId` IS subtree-expanding — which undercounts on a
  multi-root org and so needs the facet anyway. M58.
- ~~**A band is only click-through when the contract ships bounds.**~~ **CLOSED (M57 ticket 3
  follow-up).** `unit.level`'s bars were inert because the arg was a scalar exact match and the
  buckets are pairs of levels — the case `levelMin`/`levelMax` had been deferred against ("additive
  and deferred to when the bands are consumed"). They are consumed now, so they shipped: the facet
  binds the derived pair, the scalar is retained and classified `superseded`, and the bars link.
  What remains is the general rule, worth keeping in view when M58 declares a range facet: **a band
  is drawable from any bucketing, but click-through needs the contract to ship BOUNDS.** Age bands
  never had the problem — `birthdateFrom`/`birthdateTo` already existed, and the inverse of an age
  band is a birthdate range (`age >= lo ⟺ birthdate <= today − lo years`, `age <= hi ⟺ birthdate >=
  today − (hi+1) years + 1 day`).
- **A ledger's aggregate has ONE arm, and that is not a shortcut.** Every M57 type ships an
  admin/scoped pair because the visibility predicate is folded into SQL. `audit` ships one, because
  its visibility is the RLS policy — the query carries no subject at all. What replaces the parity
  guard there is the source guard: unpinned, the same statement answers a confident zero. If a future
  type's policy ever needs an app-layer predicate as WELL, the pair comes back.
- **D-ObjectFacets rule 2 has no live case yet.** Every facet in the catalog is `pii:none`/`pii:basic`
  with an empty `ReadPermission`, so no caller has ever seen a facet omitted. The console's
  absent-is-not-empty branch (an omitted facet draws NO card, never a zeroed one) is therefore
  exercised only by construction; the first gated facet — `person_health_records.kind` and its
  siblings, M58 — is what will exercise it for real.
- **Time series over history.** Every count is *as of now*. "Headcount over the last 12 months" needs
  the tier-(a) `valid_from`/`valid_to` link history (D-Temporal) folded into the aggregate, which is
  the same seam R-31's re-scope left open — not attempted here.
- **Sort.** The contract still has **no sort param anywhere**. Facets give filtering and grouping;
  ordering a list by a facet is a separate, additive change. The console's column sort is therefore
  still client-side `useState` over the rows already fetched — it reorders *a page*, not the list,
  and M56 ticket 4 deliberately left it out of the URL rather than encoding a server semantic that
  does not exist. The same applies to the quick-filter box on the four types whose list endpoint
  ships no search arg (unit, order, document, `link__member_of`): it stays page-local and now says
  so ("Filter rows on this page…", "N of M **on this page**"), where the old "N of M" beside a
  keyset-paginated table read as a total. Person and languoid, which do ship `query`, narrow
  server-side through the URL instead.
- **`totalCount` on list envelopes.** Deliberately not added — the stats endpoint carries the count,
  so list pagination stays a pure forward-only keyset with no counting cost per page.
- **The reach-cardinality dispatch tax.** Every scoped list pays one capped reach count before it
  picks a plan shape — ~180 ms at 100 000-unit reach, which is the floor under every root-reach number
  in the ticket-3 table. It is **not new**: the equivalent shipped probe `CountReadableUnitsCapped`,
  which the person directory has paid since R-02.1, measures 248 ms on the same subject. The cost is
  the `UNION`'s sort of the whole reach before the cap can apply; `UNION ALL` would stream, at the
  price of counting duplicate grants. Fixing it means touching person's measured-and-green path, so
  it is recorded here rather than attempted inside a list ticket.
- **Estimated totals — now measured, still open.** The fear was right: at 10⁶ persons a root-reach
  subject's dashboard costs **14.5 s** for all seven facets and **6.6 s** for `totalCount` alone (the
  reach semi-join, not the aggregation). A mid-reach subject — the ordinary case — draws the full
  dashboard in 873 ms. The count stays EXACT because D-ObjectFacets promises `totalCount` equals what
  exhaustively paging the same list returns, and the differential test asserts it; `pg_class.reltuples`
  is worth considering only for the unfiltered, whole-world case, and would be wrong rather than
  approximate for a filtered or scoped one. The console's real lever meanwhile is the `facets` CSV:
  ask for what you draw. Numbers in
  [review-2026-07](review-2026-07.md#m57-ticket-1--the-dashboard-aggregates-2026-07-29).
- **Two membership components have no facet behind them.** facets.md's ③ vacant-vs-filled donut and ④
  tenure histogram are NOT backed by declared facets, so M57 ticket 2 did not ship them: `positionState`
  is sourced from `membership_positions` behind a different endpoint (it belongs to a `position` object
  type — M58), and tenure would need a facet over `now() - effective_from`, which is a computed value
  rather than a column. What the shipped `positionId` distribution DOES give is the same signal in
  cruder form: its `(unknown)` bucket is the memberships with no billet. Deliberate: adding a facet is a
  catalog decision, not something a repetition ticket should smuggle in.
- **A top-N over a high-cardinality ref column is expensive.** `link__member_of.personId` costs 8.6 s
  alone at 10^6 distinct persons — the ranking window sorts every group to keep fifteen. No shipped
  chart asks for it (it is a filter facet), and the dashboard as drawn costs 1.3 s admin / 3.2 s root
  against 9.6 / 11.1 s for all facets, which is precisely what the `facets` CSV is for. The fix, if a
  genuine high-cardinality ref chart is ever wanted, is a bounded `ORDER BY … LIMIT k` heapsort plus a
  per-facet group-total row so `(other)` stays derivable; costed in
  [review-2026-07](review-2026-07.md#m57-ticket-2--the-membership--order--document-dashboards-2026-07-30).
- **One plan shape for a scoped aggregate.** A scoped LIST ships two (the table above); a scoped
  aggregate ships one, because the set form beat the point probe at every reach once the `LIMIT` was
  gone (8.3 / 79.8 / 7 144 ms against 12 926 / 17 066 / 24 869 ms). If a future stats endpoint ever
  paginates its buckets, that reasoning lapses and the dispatch comes back.
