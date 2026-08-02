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

### The fourth rule, and the one property it relaxes (M58 ticket 2)

4. **A bucket's count is the number of rows its own filter returns.** This is the property the whole
   vocabulary rests on — it is what makes a chart segment and a filter *the same act* — and it is
   asserted per bucket by every module's differential test.

Almost every facet also **partitions**: each counted row lands in exactly one bucket, so a
distribution sums to `totalCount`. Two shapes cannot, and both arrived with the taxonomy in M58
ticket 2 — the first *tree* to reach the vocabulary. `Facet.NonPartitioning` is the reason string
that marks them (the `Ledger` pattern: it carries its own justification, so a second one costs an
argument rather than a copy-paste):

- a **closure** facet (`taxon.subtree`) counts each row under every ancestor it has;
- an **M:N** facet (`taxon.classification`) counts each row under every tag it carries.

**What that exempts is the sum, and nothing else.** Rule 4 is untouched: the overlap is *between
buckets*, never between a bucket and its own filter. Two build-time guards keep it contained
(`pkg/facet/rawpgx_test.go`): the reason must be ≥ 40 characters, and the facet's `Table` must not be
the listed table — a row has one value in its own column, so a facet grouping one *cannot* overlap,
and an exemption there would be imitation rather than need.

The closure facet has one further requirement that is easy to miss and was found by the differential
test rather than by reading: **its buckets must be confined to the current candidate set.** Once a
caller has filtered to X's subtree, X's own ancestors are still ancestors of every remaining row, so
they would otherwise appear as buckets — and `parent` is single-valued, so clicking one *replaces*
the anchor and **widens** the result, landing on more rows than the bucket counted. Joining the
ancestor back to the candidate set confines the buckets to taxa strictly inside the current view,
where a bucket's subtree is contained in the candidate set and rule 4 holds at every depth. At the
top level every ancestor is a candidate anyway, so the rule is uniform rather than conditional.

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
| ~~**organization**~~ | — | **BUILT (M58 ticket 4)** — see below |
| **company** (`company_org_profiles`) | `legalForm`, `ownershipCategory`, `countryId`, `industryClass`, `foundedOn`(range), `state` | Legal-form bar · ownership donut · industry (NACE) top-15 bar · foundings-per-year histogram · country bar |
| ~~**vehicle**~~ | — | **BUILT (M58 ticket 3)** — see below |
| ~~**finance-account**~~ / ~~**finance-card**~~ | — | **BUILT (M58 ticket 3)** as `account` and `card` — see below |
| **institution** (`education_org_profiles`) | `kindId`, `countryId`, `foundedOn`(range), `state` | Kind mix bar · country bar · state tiles |
| **enrollment** (`person_education_enrollments`) | `institutionId`, `degreeLevelId`, `status`, `effectiveFrom`(range) | Enrollments per intake histogram · degree-level bar (ISCED-ordered, **not** count-ordered) · status tiles. ⚠️ **Defect (M58 ticket-2 survey):** `programId` and `startedOn` name columns that do not exist — the nearest are `field_of_study` (free TEXT, so a `code` facet at best) and `effective_from`. Corrected above. Also has **no top-level list**: only `GET /education/v1/persons/{personId}/enrollments` |


| **location** (`location_locations`) | `countryId`, `typeId` | Locations per country bar · type mix bar. ⚠️ **Defect (M58 ticket-2 survey):** `hasCoordinate` is DEGENERATE — `location_locations.geom` is `NOT NULL`, so the facet is constant-true and the geocoded-vs-not tile would always read 100%. Dropped above. Also: `listLocations` REQUIRES a spatial window (`Location:QueryWindowRequired`), so this type needs an unwindowed list mode before it can have a dashboard at all |
| ~~**languoid**~~ | — | **BUILT (M58 ticket 4)** — see below |
| **assignment** (`authz_role_assignments`) | `roleId`, `targetUnitId`, `scope`, `graphId`, ~~`active`~~, ~~`expiresAt`~~ | Grants per role bar · unit-vs-subtree donut. ⚠️ `listAssignments` returns only ACTIVE assignments and **keeps** that default (decided in ticket 3 — see [open seams](#open-seams)), so `active` and `expiresAt` are **struck** along with the expiring-soon tile and the active-vs-revoked tiles: a distribution whose every row is active is a chart with one bar. Also has **no unconditional list** — it requires exactly one of `subjectPersonId`/`targetUnitId` |

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

#### `external_organization` — [external-organizations](../modules/external-organizations.md) · `external_organizations` — **BUILT (M58 ticket 2)**

The first **vertical** on the seam, and the cheapest one: four of its six args already existed (M30).

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `kind` | ref → external_org_kind | `kind_id` | **ArgOverride `kind`**, and the arg was **WIDENED** to accept a code *or* a RID. It took a kind code; a ref bucket's key is the kind's RID, and a bucket key must remain a usable filter value. Widening beat adding a second arg meaning almost the same thing — the precedent religion's own `religion` arg had already set. `ClassSuperseded` could not be used: that class is restricted to RANGE kinds |
| `countryId` | ref → country | `country_id` | **ArgOverride `country`** (M30, and it already carried a RID). Nullable — an org may be supranational or unattributed |
| `status` | enum | `status` | `provisional` / `resolved`. A provisional row is an unresolved import stub awaiting a merge |
| `source` | enum | `source` | `self_declared` / `operator_verified` / `imported` — the D-OverlayFoundation attribution column-set. Chart order is ascending authority, **not** alphabetical |
| `confidence` | enum | `confidence` | `confirmed` / `probable` / `possible`. Descending certainty; crossed with `source` this is the view the dashboard exists for, and a frequency sort would scramble both axes |
| `asOf` | date-range | `as_of` | When the assertion was *observed*, not the row's lifetime (`created_at` is deliberately not faceted). Nullable ⇒ mandatory `(unknown)`, which reads as "asserted without an observation date". Conjure **datetime**, so the console declares `argType: "datetime"` |

**Components.** ① **Resolution tiles** (`status`). ② **Attribution confidence donut**, `possible` toned
red. ③ **Attribution source** bar. ④ **Kinds** bar. ⑤ **Top countries** bar. ⑥ **Observations per
month** histogram.

> **ONE aggregate arm — for the OPPOSITE reason to the ledger's.** Audit's single arm is a visibility
> decision made entirely by the connection the query runs on. This one is the *absence* of a
> visibility decision: `external_organizations` is flat instance-global reference data with no
> row-level security, no unit reach and no shadow flag, so `externalorg.read` held anywhere is the
> whole gate and there is nothing for a second arm to narrow.

#### `taxon` — [religion](../modules/religion.md) · `religion_taxa` — **BUILT (M58 ticket 2)**

The first **tree**, and therefore the origin of the non-partitioning property above.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `rankId` | ref → taxon_rank | `religion_taxa.rank_id` | **ArgOverride `rank`**, **WIDENED** to code-or-RID like `external_organization.kind`. Ordered by the rank's OWN ordinal via the SQL-supplied `Ord` (the rank-seniority path `topNBuckets` already honours): religion → branch → tradition is a ladder, and re-sorting it by frequency destroys the only ordering that means anything |
| `religionId` | ref → taxon | `religion_taxa.religion_id` | **ArgOverride `religion`**, which already accepted code-or-RID. The denormalized root. Unlike `subtree` this one **does** partition — every taxon has exactly one root, and a root's own row has none, which is the `(unknown)` bucket |
| `subtree` | ref → taxon | `religion_taxa_closure.ancestor_id` | **NON-PARTITIONING.** **ArgOverride `parent`**, which already meant proper descendants. Counted with `depth > 0` on **both** sides, so the reflexive `(t,t,0)` row is excluded from the bucket exactly as from the click-through — otherwise the two disagree by precisely one row. Buckets are confined to the candidate set (see rule 4 above). The one facet whose `Table` is not the listed table |
| `classification` | ref → classification | `religion_taxon_classifications.classification_id` | **NON-PARTITIONING.** **EFFECTIVE** tags, resolved to the nearest *declaring* ancestor through the closure — the same resolution `getEffectiveClassifications` performs, so the chart and the object view agree. Counting only directly-declared tags would partition and would also be useless: theism is declared on roots and inherited by everything below, so nearly every bucket would be `(unknown)` |

**Components.** ① **Taxa per rank** bar, in the ladder's own order. ② **Theism donut** (effective).
③ **Subtree size** bar — the recursive drill: click a bar to descend, and the chart re-draws over that
subtree's own branches, repeating all the way down. ④ **Per root religion** bar, which does partition.

> **Why a closure facet rather than an exact-parent one.** Grouping by `parent_id` and filtering to an
> exact parent partitions cleanly and then **dead-ends**: after one click every remaining row shares
> one parent, so the chart collapses to a single bucket and there is nowhere to go. The closure facet
> keeps working at every depth, at the cost of overlapping buckets — and the overlap is between
> buckets, never between a bucket and its own filter. That trade is the whole content of
> `Facet.NonPartitioning`.

> **ONE aggregate arm**, for the same reason as `external_organization`: the taxonomy is flat
> instance-global reference data. The row-level security in this module is on the unit-scoped
> `religion_org_*` tables, not here.

**Both are raw-pgx modules**, and that is the fifth kernel assumption M58 has named — see below.

#### `vehicle` — [vehicles](../modules/vehicle.md) · `vehicle_vehicles` — **BUILT (M58 ticket 3)**

The third raw-pgx module, and the first type this vocabulary reached whose facets are all ordinary.
It is the repetition ticket 2 was supposed to be — see the review entry for what it cost anyway.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `typeId` | ref → vehicle_type | `type_id` | The instance-extensible catalog. NOT NULL |
| `brandId` | ref → vehicle_brand | `vehicle_models.brand_id` | **TWO-HOP** — a vehicle has no brand column; the brand hangs off its model. Not a new join: `vehicleSelect` has always LEFT JOINed `vehicle_models` and projected the derived `brand_id`. `(unknown)` = the vehicles with no model, which therefore have no brand |
| `modelId` | ref → vehicle_model | `model_id` | Nullable — a vehicle of a known type may have an unknown model |
| `color` | ref → color | `color_id` | → `platform_colors` (domain='vehicle'), a **hard FK since M42/D-Color**. See the correction below. Nullable |
| `status` | enum | `status` | `active` / `scrapped` / `exported` |
| `manufactureDate` | date-range | `manufacture_date` | A calendar **DATE**, not a timestamptz — so the month bucket inverse sends bare `YYYY-MM-DD` bounds and needs **no** RFC-3339 widening, the opposite of `external_organization.asOf`. Nullable ⇒ mandatory `(unknown)` |
| `registrationCountry` | ref → country | `vehicle_registrations.country_id` | **ACTIVE registration only**, and that choice is what makes it partition — see below. `(unknown)` = never registered or deregistered |

**Components.** ① **Fleet status** tiles, `scrapped` toned red. ② **Type mix** bar. ③ **Top brands**
bar. ④ **Colours** bar, **painted the colours it names**. ⑤ **Fleet age** histogram by month of
manufacture. ⑥ **Registered in** bar.

> **The `color` defect this table used to record DID NOT EXIST.** The ticket-2 survey reported
> `vehicle_vehicles.color` as free TEXT with no FK, and proposed either adding one or dropping the
> component. Both were unnecessary: [0009_enrichment.sql:824-834](../../migrations/0009_enrichment.sql)
> replaced `color` with `color_id uuid REFERENCES oikumenea.platform_colors(id)`, backfilled it,
> **dropped the old column** and indexed it — under M42/D-Color, long before M58. The survey read the
> `CREATE TABLE` near the top of a 900-line consolidated migration and never reached the `ALTER` 600
> lines below it. The lesson is narrow and worth keeping: **since the migration consolidation
> (46 files → 15), a table's shape is no longer its `CREATE TABLE`** — a column may be added, altered
> or dropped later in the same file. Grep the column, not the table.

> **A facet that had to choose a SET, and chose the one that partitions.** `vehicle_registrations` is
> ownership HISTORY — one-to-many, so grouping it raw counts a re-registered vehicle under every
> country it has ever worn plates in, which would need `Facet.NonPartitioning` and would legitimately
> earn it (the table is not the listed table). It is instead confined to the **ACTIVE** registration,
> of which `CloseActiveRegistrationsForVehicle` guarantees at most one per vehicle. That is the
> `person.rankId` precedent — match the active row — and it is also the question the chart is read
> for: *where is this fleet registered now*. The exemption exists; it was not taken, because a set
> that partitions honestly is better than an exemption that is available.

> **ONE aggregate arm**, for `external_organization`'s reason and **not** the ledger's: no row-level
> security, no unit column, no reach — `vehicle.read` held anywhere is the whole gate.

#### `account` / `card` — [finance](../modules/finance.md) · `finance_accounts`, `finance_cards` — **BUILT (M58 ticket 3)**

The first module to bring **two object types at once**, and `card` is the first type whose
**collection-level list this vocabulary had to add**.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `account.institutionId` | ref → organization | `institution_id` | The holding **bank** is a `company`-domain `tenant_organizations` row (M21/M41, D-UnifiedOrgGraph), never a finance-owned entity — so the RefType is `organization` and the buckets label through the same resolver that names an org everywhere else. NOT NULL |
| `account.currency` | code | `currency` | ISO 4217. `KindCode`, not enum: the column carries **no CHECK**, so the value set is open — the `audit.action` case. The key is its own label. Nullable |
| `account.accountTypeId` | ref → account_type | `account_type_id` | Instance-extensible catalog. Nullable |
| `account.status` | enum | `status` | `active` / `closed` / `frozen`, `frozen` toned red |
| `card.networkId` | ref → card_network | `network_id` | Instance-extensible catalog. Nullable |
| `card.cardType` | enum | `card_type` | `debit` / `credit`. Keyed `cardType`, not `type`: beside `networkId` a bare `type` reads as the card's network |
| `card.status` | enum | `status` | `active` / `blocked` / `expired` |

**Components.** *account*: ① status tiles ② accounts-per-bank bar ③ currency donut ④ type-mix bar.
*card*: ① status tiles ② networks bar ③ debit/credit donut.

> **`card` had no collection to describe, so the ticket built one.** Cards were reachable only at
> `GET /accounts/{accountId}/cards` — unpaged, per account. `GET /cards` is new, and the naming moved
> with it: the per-account list is now **`listAccountCards`**, beside the `listAccountHolders` already
> there, and the plain `listCards` names the registry, as every other faceted type's list endpoint
> does. **HTTP paths are unchanged**, so no client URL broke.
>
> The new list is **metadata only**, and that is a compliance boundary rather than a convenience:
> retained PANs put `finance_cards` in **PCI-DSS CDE scope** (D-DataScope). It returns exactly the
> projection the per-account list already returned — `bin`, `lastFour`, network, type, status, expiry
> — under exactly the `finance.read` that already gated it. It **widens the scope of a read the code
> already permits and discloses no new field**; that is why it needed no new permission code. The PAN
> is decrypted by `getCard` alone, one card at a time.

> **What has no facet here, and cannot.** `iban_*` and `pan_*` are envelope-encrypted: there is no
> plaintext to GROUP BY, and D-DataScope's aggregation rule forbids the surface independently of
> that. The **blind index** is technically groupable and is still not a facet — it is a per-value
> HMAC, so its distribution is one row per distinct IBAN and its buckets would BE the identifiers.
> `bin`/`last_four` are clear and are still not facets: they identify one card rather than describing
> a population, and a top-N over four-digit suffixes ranks nothing.

> **ONE aggregate arm** on both, same reason as `vehicle`.

#### `organization` — [tenant](../modules/tenant.md) · `tenant_organizations` — **BUILT (M58 ticket 4)**

The tenant module's **second** object type, and the **last** M58 type with an app-layer visibility
predicate.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `domain` | ref → domain | `domain_id` | The org-kind catalog (D-TenantOrganizations, M40). The `domain` arg predates this vocabulary and already took a domain RID, so a bucket key was a usable filter value from the start — no widening, no `ArgOverride`. NOT NULL |
| `visibility` | enum | `visibility` | `public` / `shadow`. Narrows, never widens — but see below: for an org a non-admin's `shadow` bucket is **structurally** zero rather than reach-dependent |
| `state` | enum | `state` | `active` / `suspended` / `archived`; suspended amber, archived slate |

**Components.** ① orgs-per-domain bar ② lifecycle tiles ③ **visibility donut** — a third component
beyond the two this table originally catalogued, added because the facet is declared and the seed
change gives it two buckets. Recorded here rather than shipped silently.

> **The tranche row above was right about the shape and wrong about the predicate.** It said
> organization "needs the full M57 two-arm treatment (rule 3)". Two arms: yes. Unit's arm: no.
>
> `listOrganizations` is gated by **`gateUnits`** — the *unit* gate, applied to organization rows. On
> a unit that gate is real: `FilterVisibleUnits` probes the subject's reach with
> `ReadableUnitsForSubjectAmong`, and a shadow unit inside the reach passes. On an organization it is
> not. That probe's only match arms test `a.target_unit_id = cand.unit_id`, and
> `authz_role_assignments.target_unit_id` is `NOT NULL REFERENCES tenant_units` — **an organization
> RID can never appear in it.** The reach set for an org is empty by construction, and a shadow
> organization is visible to an instance admin and to nobody else.
>
> That gap is now **closed** (follow-up commit; D-VisibilityScope amendment): organization reach is
> **derived from unit reach** — an organization is visible when any of its live units is in the
> subject's reach. The scoped arm is `visibility = 'public' OR id IN (<orgs of reachable units>)`,
> and the list gates through `gateOrgs`, a sibling of `gateUnits` rather than a call into it.
>
> The part that survives unchanged is why the arm can never be unit's
> `id IN (authz_readable_units(...))`: a unit IS a grant target, an organization is not, so that
> predicate copied across matches nothing. It has to join through `tenant_units` to mean anything.
>
> `TestOrganizationShadowIsUnreachableForEveryNonAdmin` did what it was written to do — went red on
> the day this changed, carrying the message that said so — and is replaced by
> `TestOrganizationReachIsDerivedFromUnitReach`, which asserts both directions from one setup.

> **`domain` collapses its tail in SQL; `UnitStats`' ref branches do not.** The kernel's `topNBuckets`
> orders and appends `(unknown)`/`(other)` but never truncates, so a facet declaring `TopN 15` whose
> SQL emits every group renders more bars than it promised. Unit's branches get away with it because
> a unit's org/domain/kind cardinality is a handful of catalog rows; the domain catalog is
> instance-extensible, so this one keeps its own promise. The asymmetry is deliberate and it is
> organization's side that is right.

> **What is not faceted.** `tenant_organizations.search_text` is `pii:basic` and `listOrganizations`
> ships no `query` arg. Adding one is a search decision, not a facet one — out of scope here.

#### `languoid` — [language](../modules/language.md) · `language_languoids` — **BUILT (M58 ticket 4)**

27 177 rows of instance-global reference data. The **second** type with an R-21 search twin (after
person) and the **first** with a composite code facet.

| Facet | Kind | Source | Notes |
|---|---|---|---|
| `level` | enum | `level` | `family` / `language` / `dialect`, in **tree order** — not alphabetical, not by frequency. The arg predates the vocabulary and took the same values, so no `ArgOverride`; what changed is the SQL |
| `macroarea` | code | `macroarea` | **Set-valued, semicolon-joined.** `KindCode`, not enum: no CHECK, so the value set is open (the `audit.action` case) and the key is its own label. Nullable ⇒ `(unknown)` mandatory |
| `status` | enum | `status` | AES endangerment. The CHECK set is **already in severity order** in the DDL, and that order is the chart's |
| `family` | code | `family_code` | The denormalized root-family **glottocode**, derived via the closure. Code, not ref — a glottocode is not a RID and is its own label. `char(8)`, so keys are `rtrim()`ed. Nullable in the DDL ⇒ `(unknown)` mandatory |

**Components.** ① level-mix donut ② **endangerment bar, ordered by severity** ③ macroarea bar
④ largest-families top-15 bar.

> **The composite code facet.** `macroarea` is set-valued and stored semicolon-joined (183 of 27 177
> rows read `Africa;Eurasia`). It is grouped by the **literal string** rather than unnested, and that
> is not a shortcut: the filter is an exact match, so each bucket's count equals what its own filter
> returns — the property the whole vocabulary rests on. It therefore **partitions**, needs no
> `NonPartitioning`, and **could not take one anyway**: the kernel refuses that exemption when the
> facet's `Table` IS the listed table, because a row has one value in its own column and cannot
> overlap. Unnesting would double-count and would need an exemption that is not available. The
> composite keys read as what they are — a languoid spanning two macroareas — and
> `TestLanguoidCompositeMacroareaRoundTrips` pins that `Africa;Eurasia` is a working filter value,
> because if it ever stops being one the facet has to be redesigned rather than patched.

> **`family_code` is `char(8)`, and the padding risk turned out to be theoretical — measured, not
> assumed.** The column is `char(8)`, which pads on read, and the facet emits `rtrim(family_code)`.
> Checking against the live catalog after the fact: **every glottocode is exactly 8 characters**
> (`min(length)=max(length)=8`, zero padded rows), and the `::text` cast the branch already applies
> **strips trailing blanks on its own** (`length('ab'::char(8)::text) = 2`). So the `rtrim` fixes
> nothing today and would be redundant even if a short code appeared.
>
> It stays, and is documented as belt-and-braces rather than as a fix: it states the intent where the
> conversion rule would otherwise be load-bearing and invisible. `TestLanguoidFamilyKeysAreTrimmed`
> is likewise a regression guard for a change that removed the cast, not evidence of a bug that was
> found. Recorded this way deliberately — ticket 3 spent a round re-deciding a `vehicle.color`
> "defect" that had been fixed years earlier, and the cost there was a claim nobody had measured.

> **`limit` → `pageSize`: the first WIRE BREAK this vocabulary has demanded.** `ClassPaging` covers
> only `pageSize`/`pageToken`, and eleven other types hold that convention; `listLanguages` was the
> lone holdout. The guard describes a real convention and the endpoint was the outlier, so the arg
> was renamed rather than the guard widened. Every consumer is in-repo (the generated Go/TS SDKs, the
> console's `search:` string, three positional `LanguagePicker` calls); `hermenea` does not call this
> endpoint, and the one Go consumer uses the *domain* `Filter.Limit`, which deliberately keeps its
> name — the rename was a contract decision and propagating it inward would churn the connector and
> dataimport for nothing.

> **The four facet predicates moved off the legacy `sqlc.arg(x)::text = ''` sentinels onto nargs.**
> A sentinel forces one generic plan across every filter shape (R-21's whole argument) and is
> invisible to the parity guard, which reads a facet's narg out of every list *and* stats query. The
> two **traversal** args keep their sentinels: they are not facets and no aggregate counts them.

> **ONE aggregate arm**, for `vehicle`'s and `external_organization`'s reason — the **absence** of a
> visibility decision — and emphatically not `audit`'s, where the single arm *is* the decision, made
> by the connection the query runs on. `language_languoids` has no RLS, no unit column and no reach
> predicate; `language.read` held anywhere is the whole gate.

> **`parent`/`topLevel` are `ClassTraversal`, and that stretches the class.** `unit.parent` switches
> the endpoint to a *different* query (`ListChildUnits`); languoid's `parent` adds a predicate to the
> same one. What it shares with unit's — the part the class is for — is that it selects a tree-walk
> **mode** rather than describing the registry, so no aggregate counts it. Recorded rather than left
> as an unremarked stretch. Neither is a facet on purpose: an exact-parent dimension partitions
> honestly and then dead-ends after one click, which is `taxon.subtree`'s argument in reverse.

### The raw-pgx arm of the parity guards (M58 ticket 2)

`sqlparity_test.go` and `statsparity_test.go` prove a type's list and its dashboard see one world by
parsing `internal/<module>/adapters/queries/*.sql`: every facet's `sqlc.narg` in every query, and the
aggregate half byte-identical across arms. That is a proof about **static text**, and it works because
sqlc queries are static text.

Four modules are not. `religion` and `externalorg` (ticket 2), `vehicle` and `finance` (ticket 3)
build SQL at runtime, each by a documented choice in its package doc. They have no `queries`
directory at all, so the existing guards cannot see them — and a registered type nothing checks is
exactly the hole those files' coverage floors exist to refuse.

**All four are now covered, and ticket 3 was the first test of whether the guard generalizes** — it
was written alongside religion and externalorg, so until three types it had never been applied to one
it did not already describe. It held without amendment. The one thing it had not seen is **two object
types in one module**: `account` and `card` both live in `finance`, so the module's AST is parsed
twice and each group must find its own builder and its own aggregate const. That falls out of the
existing design (the groups key on the object type and look functions up by name) rather than needing
a change, but it is the reason `financeAggregate` does not exist: one shared const would satisfy
neither direction of the branch-coverage check, and the two types' facet sets are disjoint.

The **invariant is unchanged**; only the proof is, because the mechanism the proof was written in does
not exist here (`pkg/facet/rawpgx_test.go`):

| sqlc modules | raw-pgx modules |
|---|---|
| every facet's `sqlc.narg` appears in the list query and the stats query | the list path and the stats path both call ONE shared filter builder, checked by **parsing the adapter's AST** — a comment naming the builder must not satisfy it |
| the aggregate half is byte-identical across arms | the aggregate is a single named `const`, referenced by the stats path — with one arm there is nothing to compare to, so what is worth proving is that the text has one definition and cannot drift from a copy |
| every branch names a declared facet, and every facet has a branch | the same, read out of the const |

The two sqlc-shaped coverage floors **defer** to it rather than exempting the types: each still
requires that *something* checks the type, it just is not that file.

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

- **`assignment` keeps its implicit active-only filter, and DROPS two catalogued facets.** Decided
  ahead of its ticket (M58 ticket 3), so the ticket does not re-litigate it. `listAssignments` returns
  only ACTIVE assignments, which is a hidden default of exactly the shape M56 rejected for
  `link__member_of` — and the tranche table above catalogues `active` and `expiresAt` facets that
  cannot coexist with it, since a distribution over `active` whose every row is active is a chart with
  one bar. The endpoint's semantics stand and **those two facets are struck**: the assignments
  dashboard describes the ACTIVE grant population and says so. The alternative — dropping the default
  the way membership did — was rejected because these are not symmetric cases: an ended membership is
  ordinary directory history, while a revoked grant is a security artefact whose reachability is an
  authz-plane read-surface decision, not a facet-vocabulary one. `roleId`, `targetUnitId`, `scope` and
  `graphId` are unaffected.
- ~~**A shadow organization is reachable by nobody but an instance admin.**~~ **CLOSED (M58 ticket 4
  follow-up): organization reach is now DERIVED from unit reach.** The seam was that
  `authz_role_assignments.target_unit_id` is `NOT NULL REFERENCES tenant_units`, so an organization
  RID could never appear in a grant and the org reach set was **empty by construction** — making
  shadow-org visibility an admin-only bit by accident of the assignment table's shape rather than by
  anyone's decision. Of the three shapes on the table, **deriving reach from unit reach** was chosen:
  an organization is visible when any of its live units is in the subject's reach
  (`ReadableOrgsForSubjectAmong`, `application.FilterVisibleOrgs`, and the same predicate folded into
  `OrganizationStatsForSubject`).
  It **leaks nothing new**, which is what made it the cheap answer: `listUnits` takes the org RID as a
  REQUIRED argument and gates the *units*, not the organization, so a subject with reach inside an
  org could already enumerate its units and was already holding its RID — the organization simply did
  not say so. It is also **precise**: reaching one shadow org does not reveal another (verified live —
  a subject with reach into `privatbank` sees it and still 404s on `upc-parish`).
  The alternatives are recorded as rejected rather than dropped: a nullable `target_org_id` and an
  org-level `scope` both add a new grant primitive to the PDP for a visibility question that unit
  reach already answers.
  `TestOrganizationShadowIsUnreachableForEveryNonAdmin` did what it was written to do — it went red on
  the day this changed, carrying the message that said so — and is replaced by
  `TestOrganizationReachIsDerivedFromUnitReach`, which asserts **both** directions from one setup
  (either alone is satisfiable by a bug: "sees it with reach" passes if the gate stopped gating,
  "does not see it without" passes if reach went back to empty).
- **A table's shape is no longer its `CREATE TABLE`.** The migration consolidation (46 files → 15)
  means a column may be created near the top of a file and altered or dropped 600 lines below it, in
  that same file. The M58 ticket-2 survey read `vehicle_vehicles`' `CREATE TABLE`, recorded a defect
  that had been fixed by an `ALTER` in the same file since M42, and the correction cost ticket 3 a
  round of re-deciding. Nothing structural to fix — but **grep the column, not the table**, and prefer
  the live schema to the DDL when both are available.
- **Two M58 types have no RID token of their own.** `company` (`company_org_profiles`) and
  `institution` (`education_org_profiles`) are **sidecar tables on `tenant_organizations`**
  (M41 / D-UnifiedOrgGraph): their primary keys FK to `tenant_organizations(id)`, whose RID is
  `organization` (4/1/6). `facet.Register` refuses a `Type` that is not a registered `pkg/rid` token,
  and the `Ledger` escape cannot absorb them — it is capped at ONE by a guard and `audit` holds it,
  correctly, since these tables are not ledgers and their rows *do* have a type token; it just is not
  theirs alone. So both are **blocked** on a catalog decision, in the same register as the four
  assumptions ticket 1 surfaced. Three shapes are on the table, none yet argued: a declared
  "profile of an existing token" arm; a `domain`-discriminated `organization` type whose facets differ
  per domain; or new RID types for the profiles, which is a schema change and the most invasive.
  **Decide before starting either type's ticket, not during it.**
- **A per-module action catalog whose `targetType` is the module, not the object type.** The object
  view sources its inline actions by matching `ActionType.targetType` to the registry type, so
  `taxon`'s actions do not surface there: `religion.taxon.update` and friends declare
  `TargetType: "religion"`. That is why M58 ticket 2 kept the taxonomy editor on `/religion` rather
  than folding it into the object view the way `external_organization` could (its actions do declare
  `external_organization`). Retargeting them is a one-word change per row and a real data question —
  `target_type` is written into `audit_log`, so new rows would stop matching historical ones.
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
