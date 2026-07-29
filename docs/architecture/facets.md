# Facets — the shared filter/aggregation vocabulary and the dashboard catalog

> **Status: the kernel is BUILT — `person` and `unit` (M56 ticket 2), `membership` / `order` /
> `document` (M56 ticket 3, alongside their new top-level list endpoints), and all five reach the
> console as URL-borne list filters (M56 ticket 4). The DASHBOARD half is under way: `person` and
> `unit` now have stats endpoints (M57 ticket 1); the remaining three follow in ticket 2, the charts
> in ticket 3, and the remaining types are M58.** Binding design lives in
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
the canonical personnel-structure view. ② **Sex donut** with an explicit `not_known` slice — the slice
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
| `level` | numeric-range | `level` | the contract ships a **scalar exact-match `level`** arg that predates this vocabulary and (expand-only) keeps it; the facet pins that name. M57 bands the same column; `levelMin`/`levelMax` are additive and deferred to when the bands are consumed |
| `visibility` | enum | `visibility` | `public`/`shadow` |
| `state` | enum | `state` | `active`/`suspended`/`archived` |
| `pdpScoped` | bool | `pdp_scoped` | operational vs reference units (D-UnifiedOrgGraph) |

> `graph` is **not** a facet. It selects which DAG `parent`/`rootsOnly` walk and adds no predicate to
> `tenant_units` — there is no `tenant_units.graph_id` to filter or `GROUP BY`. M56 classifies it as a
> traversal arg, which is what the drift guard checks it against.

**Components.** ① **Units per level** — bar, level ascending; the org chart's width profile. ②
**Kind mix** donut. ③ **Public/shadow split** — a two-segment bar, not a donut: the shadow count is a
governance number an operator reads exactly, so the label carries the count. ④ **State tiles**. ⑤
**Headcount by unit** — top-15 bar of active memberships, the one component sourced from
`membership`'s stats rather than `unit`'s (a cross-module read, gated on `membership.read`, omitted
without it).

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
attached, so it leads. ② **Type mix** bar. ③ **Status donut**. ④ **Issuing country** top-15 bar.

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
| **audit** (`audit_log`) | *already has 9 filter args* — formalize as facets: `actorType`, `action`, `targetType`, `outcome`, `unitId`, `since`/`until` | Actions per day histogram · outcome donut (denied toned `red`) · top actions bar · top actors bar |

`audit` is the cheapest and most convincing first M58 module: its filter args already exist, so it
becomes a stats endpoint and a dashboard with no contract churn — the "audit analytics" use the R-29
action-type catalog was partly built for.

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

## Open seams

- **Cross-type dashboards.** Every dashboard is single-type by construction (per-module stats
  endpoints, D-ObjectFacets). "Persons by unit *and* by rank at once" is a two-facet cross-tab, not
  supported; a genuine cross-type roll-up would want the fan-in service D-ObjectFacets rejected, and
  should be re-argued on evidence rather than assumed.
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
- **One plan shape for a scoped aggregate.** A scoped LIST ships two (the table above); a scoped
  aggregate ships one, because the set form beat the point probe at every reach once the `LIMIT` was
  gone (8.3 / 79.8 / 7 144 ms against 12 926 / 17 066 / 24 869 ms). If a future stats endpoint ever
  paginates its buckets, that reasoning lapses and the dispatch comes back.
