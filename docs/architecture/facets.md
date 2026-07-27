# Facets — the shared filter/aggregation vocabulary and the dashboard catalog

> **Status: designed (M56–M58), not yet built.** Binding design lives in
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
| **Stats bucket** | a `groupBy` key on that module's `GET /<module>/v1/<collection>/stats` |

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
| `unitId` | ref | `membership_memberships.unit_id` (active) | subtree-expanding: filtering by a unit includes its closure descendants |
| `hasAccount` | bool | `identity_accounts` semi-join | account-optional directory (L-AccountOptional) |

**Components.** ① **Age pyramid** — horizontal histogram of `birthdate` age bands (`0–17, 18–24,
25–34, 35–44, 45–54, 55–64, 65+, unknown`) split by `sex`, the two sexes mirrored about a centre axis;
the canonical personnel-structure view. ② **Sex donut** with an explicit `not_known` slice — the slice
is the data-quality signal, so it is never hidden. ③ **Status tiles** — four `StatTile`s
(active/deactivated/provisional/purged) with `totalCount` as the headline. ④ **Rank distribution** —
vertical bar ordered by **rank seniority, not by count** (rank is an ordered scheme; sorting by
frequency would destroy the only ordering that means anything). ⑤ **Top units** — top-15 bar +
`other`. ⑥ **Country of birth** — top-15 bar + `other`.

> No facet over ethnicity, party membership, political leaning, religious affiliation, health or
> legal records. See [What has no facet](#what-has-no-facet).

#### `unit` — [tenant](../modules/tenant.md) · `tenant_units`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `org` | ref | `org_id` | **required** today (`listUnits` rejects an unscoped listing) |
| `domain` | ref | `domain_id` | |
| `unitKind` | ref | `kind_id` | domain-scoped catalog |
| `level` | numeric-range | `level` | |
| `visibility` | enum | `visibility` | `public`/`shadow` |
| `state` | enum | `state` | `active`/`suspended`/`archived` |
| `graph` | ref | `tenant_graphs` | selects which DAG `parent`/`rootsOnly` traverse |
| `pdpScoped` | bool | `pdp_scoped` | operational vs reference units (D-UnifiedOrgGraph) |

**Components.** ① **Units per level** — bar, level ascending; the org chart's width profile. ②
**Kind mix** donut. ③ **Public/shadow split** — a two-segment bar, not a donut: the shadow count is a
governance number an operator reads exactly, so the label carries the count. ④ **State tiles**. ⑤
**Headcount by unit** — top-15 bar of active memberships, the one component sourced from
`membership`'s stats rather than `unit`'s (a cross-module read, gated on `membership.read`, omitted
without it).

#### `membership` — [membership](../modules/membership.md) · `membership_memberships`, `membership_positions`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `unitId` | ref | `unit_id` | |
| `personId` | ref | `person_id` | |
| `positionId` | ref | `position_id` | nullable — membership without a billet is legal |
| `status` | enum | `status` | `active`/`ended` |
| `effectiveFrom` | date-range | `effective_from` | |
| `positionState` (positions) | enum | `status` + fill state | `active`/`abolished` × `vacant`/`filled` |

**Components.** ① **Active vs ended tiles**. ② **Joins per month** — `date_trunc('month',
effective_from)` histogram; the intake curve. ③ **Vacant vs filled positions** donut — the staffing
gap, the number this module exists to answer. ④ **Tenure histogram** — bands over
`now() - effective_from` for active rows.

#### `order` — [order](../modules/order.md) · `order_orders`

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `issuingUnitId` | ref | `issuing_unit_id` | |
| `orderTypeId` | ref | `order_items.type_id` | an order's *effect* lives on its items |
| `status` | enum | `status` | `draft`/`issued`/`revoked` |
| `issuedOn` | date-range | `issued_on` | |

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

## Open seams

- **Cross-type dashboards.** Every dashboard is single-type by construction (per-module stats
  endpoints, D-ObjectFacets). "Persons by unit *and* by rank at once" is a two-facet cross-tab, not
  supported; a genuine cross-type roll-up would want the fan-in service D-ObjectFacets rejected, and
  should be re-argued on evidence rather than assumed.
- **Time series over history.** Every count is *as of now*. "Headcount over the last 12 months" needs
  the tier-(a) `valid_from`/`valid_to` link history (D-Temporal) folded into the aggregate, which is
  the same seam R-31's re-scope left open — not attempted here.
- **Sort.** The contract still has **no sort param anywhere**. Facets give filtering and grouping;
  ordering a list by a facet is a separate, additive change.
- **`totalCount` on list envelopes.** Deliberately not added — the stats endpoint carries the count,
  so list pagination stays a pure forward-only keyset with no counting cost per page.
- **Estimated totals.** At registry scale an exact `count(*)` over an unfiltered, visibility-scoped
  10⁶-row set may exceed its latency budget; `pg_class.reltuples` estimation is the fallback and is
  measured, not assumed, during M57.
