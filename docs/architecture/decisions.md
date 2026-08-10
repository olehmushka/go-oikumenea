# Decisions

The binding architectural decisions for the **built / in-progress surface (M0–M15)** — what the
code is actually held to. **If code and a decision here disagree, the code is wrong.** Each entry:
the decision, why, and the consequence. Two groups: decisions **resolved this session**, and
**carried-over locks** (settled earlier, restated here so this file is self-contained).

The **planned-tier (M16–M26)** decisions — decided/designed but not yet built — live in their own
[`roadmap-decisions.md`](roadmap-decisions.md) (split out per the F-008 review finding so this
binding file reflects the built surface); they become binding-against-code as each milestone enters
implementation.

Format note: these are intentionally lightweight ADR entries, not the full `drafts/`
ADR ceremony. They will be the seed for a `docs/decisions/` ADR set if the project later
wants per-file ADRs.

---

## Decision index

Load this table, fetch the block you need — the file is large. IDs link to the block; the
planned-tier (M16–M45) decisions live in [`roadmap-decisions.md`](roadmap-decisions.md) (its own index).

| ID | Decision |
| --- | --- |
| [D-Graph](#d-graph--the-unit-hierarchy-is-a-dag) | The unit hierarchy is a DAG |
| [D-Inherit](#d-inherit--inheritance-is-per-assignment-scope) | Inheritance is per-assignment scope |
| [D-Graphs](#d-graphs--multiple-named-hierarchies-typed-edges-per-graph-closure) | Multiple named hierarchies (typed edges, per-graph closure) |
| [D-Rank](#d-rank--rank-on-person-rank--permission) | Rank on person; rank ≠ permission |
| [D-Position](#d-position--position-is-a-unit-owned-billet-that-can-be-vacant) | Position is a unit-owned billet that can be vacant |
| [D-InstanceAdmin](#d-instanceadmin--a-separate-instance-admin-scope) | A separate instance-admin scope |
| [D-PersonGlobal](#d-personglobal--person-is-instance-global) | Person is instance-global |
| [D-NoRLS](#d-norls--app-layer-pdp-no-rls-for-unit-isolation) | App-layer PDP, no RLS for unit isolation |
| [D-Migrations](#d-migrations--atlas-versioned-migrations-one-location-lint-gate) | Atlas versioned migrations, one location, lint gate |
| [D-Conjure](#d-conjure--conjure-first-api-serializableerror-token-pagination) | Conjure-first API, SerializableError, token pagination |
| [D-Stack](#d-stack--the-palantir-oss-stack-reverses-drafts-fx) | The Palantir OSS stack (reverses drafts' fx) |
| [D-i18n](#d-i18n--i18n-is-required-all-translations-in-every-response) | i18n is required; all translations in every response |
| [D-Code](#d-code--stable-locale-agnostic-codes-separate-from-translatable-names) | Stable, locale-agnostic codes separate from translatable names |
| [D-Audit](#d-audit--every-write-is-audited-audit-reads-are-permission-scoped) | Every write is audited; audit reads are permission-scoped |
| [D-Bootstrap](#d-bootstrap--install-time-bootstrap-of-the-first-instance-admin) | Install-time bootstrap of the first instance admin |
| [D-BaseRoles](#d-baseroles--seeded-base-roles-reads-are-explicit-grants) | Seeded base roles; reads are explicit grants |
| [D-SelfCapabilities](#d-selfcapabilities--self-only-capability-introspection-get-mecapabilities-resolves-oq-5) | Self-only capability introspection (`GET /me/capabilities`) |
| [D-JIT](#d-jit--just-in-time-provisioning-is-link-on-match-only) | Just-in-time provisioning is link-on-match only |
| [D-DirectoryGraphs](#d-directorygraphs--graphs-may-be-directory-only-pdp-enforced-flag) | Graphs may be directory-only (PDP-enforced flag) |
| [D-EdgePerms](#d-edgeperms--edge-management-is-per-graph-code-defined-permissions--broad-fallback) | Edge management is per-graph (code-defined permissions + broad fallback) |
| [D-ClosureIntegrity](#d-closureintegrity--on-demand-per-graph-closure-verify--rebuild-decoupled-from-the-worker-runtime) | On-demand per-graph closure verify & rebuild |
| [D-PIITiers](#d-piitiers--5-tier-pii-classification-via-comment-on-column) | 5-tier PII classification via COMMENT ON COLUMN |
| [D-TimeBoundGrants](#d-timeboundgrants--role-assignments-may-be-time-bound-expiresat-active) | Role assignments may be time-bound (expires_at active) |
| [D-RLSDefenseInDepth](#d-rlsdefenseindepth--pdp-mirror-rls-backstop-defense-in-depth) | PDP-mirror RLS backstop (defense-in-depth) |
| [D-PersonReadScope](#d-personreadscope--a-persons-read-scope-projects-through-its-memberships) | A person's read scope projects through its memberships |
| [D-PersonBio](#d-personbio--person-bio-fields-structured-names-birthdate-iso-5218-sex) | Person bio fields: structured names, birthdate, ISO 5218 sex |
| [D-Documents](#d-documents--a-document-module-for-person-held-papers--personal-codes) | A document module for person-held papers & personal codes |
| [D-Orders](#d-orders--an-order-module-administrative-acts-as-the-legal-basis-for-status-changes) | An order module: administrative acts as the legal basis for status changes |
| [D-OrderApply](#d-orderapply--orders-auto-apply-their-effects-on-issue-synchronous-in-process-same-transaction) | Orders auto-apply their effects on issue (same transaction) |
| [D-ClosureDriftHealth](#d-closuredrifthealth--closure-drift-is-surfaced-via-a-diagnostic-health-reporter-no-scheduled-rebuild) | Closure drift surfaced via a diagnostic health reporter |
| [D-PersonNamesCLDR](#d-personnamescldr--names-follow-the-unicode-cldr-person-names-fixed-field-set-amends-d-personbio) | Names follow the Unicode CLDR Person Names fixed field set |
| [D-Geo](#d-geo--seeded-iso-3166-country-registry-citizenship-birth-and-residence-as-first-class-person-data) | Seeded ISO-3166 country registry; citizenship/birth/residence |
| [D-PersonalCodes](#d-personalcodes--national-identifiers-as-a-per-scheme-catalog-with-validation-extends-d-documents) | National identifiers as a per-scheme catalog with validation |
| [D-CryptoProvider](#d-cryptoprovider--pluggable-envelope-encryption-for-sensitive-pii-reshapes-ds-29) | Pluggable envelope encryption for sensitive PII |
| [D-ResourceIdentifiers](#d-resourceidentifiers--packed-uuidv8-rids-as-primary-keys-objects-links-actions) | Packed UUIDv8 RIDs as primary keys (Objects, Links, Actions) |
| [D-RIDSeeding](#d-ridseeding--rid-keyed-seed-rows-may-seed-in-migrations-boot-seeding-retained-by-choice) | RID-keyed seed rows MAY seed in migrations |
| [D-Ontology](#d-ontology--object--link--action-is-the-binding-domain-model) | Object / Link / Action is the binding domain model |
| [D-PersonContactChannels](#d-personcontactchannels--emails-phones-and-call-signs-as-effective-dated-person-child-tables-extends-d-geo) | Emails, phones, call signs as effective-dated person child tables |
| [D-DocumentAttrSchema](#d-documentattrschema--per-document-type-attribute-schema-with-write-time-validation-extends-d-documents) | Per-document-type attribute schema with write-time validation |
| [D-WebUI](#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop) | An optional standalone Next.js admin UI |
| [D-PersonSocialChannels](#d-personsocialchannels--social-network--messenger-presence-as-catalog-typed-person-channels-with-analytics-grade-attribution-extends-d-personcontactchannels) | Social-network & messenger presence as catalog-typed channels |
| [D-PersonRelationships](#d-personrelationships--personperson-ties-as-per-type-reified-self-links-extends-d-ontology-mirrors-memberships-temporal-link) | Person↔person ties as per-type reified self-links |
| [D-RankSystems](#d-ranksystems--multinational-rank-systems-standardized-grade-comparability-and-scheme-presets-extends-d-rank-refines-l-onerankscheme) | Multinational rank systems, standardized-grade comparability, presets |
| [D-RLSLiveReach](#d-rlslivereach--rls-policies-compute-reach-live-in-sql-the-guc-contract-is-o1) | RLS policies compute reach live in SQL; O(1) GUC contract |
| [D-AuthzRequestContext](#d-authzrequestcontext--authority-state-is-fetched-once-per-request-and-snapshotted-on-the-context) | Authority state fetched once per request, snapshotted on context |
| [D-AuthzGrantCache](#d-authzgrantcache--epoch-validated-per-process-grant-cache-2-s-revocation-bound) | Epoch-validated per-process grant cache (2 s revocation bound) |
| [D-PersonSearch](#d-personsearch--trigram-indexed-directory-search-over-names--variants-filtered-in-sql) | Trigram-indexed directory search over names + variants |
| [D-AuditRetention](#d-auditretention--monthly-partitioned-audit-ledger-retention-is-an-operator-act) | Monthly-partitioned audit ledger; retention is an operator act |
| [D-PersonModuleSplit](#d-personmodulesplit--the-person-god-module-splits-into-core--profile--sensitive-behind-one-personservice) | The person god module splits into core / profile / sensitive |
| [D-EventOutbox](#d-eventoutbox--transactional-outbox-for-the-notify-event-class-extends-the-pkgevents-bus) | Transactional outbox for the notify event class |
| [D-DataScope](#d-datascope--what-a-deployment-may-hold-the-product-is-a-personnel-directory--registry-platform-owns-the-pci-dss-posture) | What a deployment may hold; product is a registry platform; PCI-DSS posture |
| [D-VisibilityScope](#d-visibilityscope--one-read-visibility-interface-three-canonical-scope-shapes-registered-per-object-type) | One read-visibility interface, three canonical scope shapes, registered per object type |
| [D-UnifiedSearch](#d-unifiedsearch--one-cross-type-searchservice-as-a-fan-in-over-the-per-module-trigram-queries) | One cross-type SearchService as a fan-in over the per-module trigram queries |
| [D-LinkTraversal](#d-linktraversal--one-generic-getobjectlinks-endpoint-as-a-fan-in-over-a-pkgrid-derived-link-descriptor-registry) | One generic getObjectLinks endpoint as a fan-in over a pkg/rid-derived link-descriptor registry |
| [D-ActionTypes](#d-actiontypes--a-checked-action-type-catalog-behind-the-free-text-audit_logaction) | A checked action-type catalog behind the free-text audit_log.action |
| [D-ActionInvocation](#d-actioninvocation--an-ir-derived-endpoint-binding-per-action-driving-a-generic-console-action-runner) | An IR-derived endpoint binding per action, driving a generic console action runner |
| [D-LinkPermissions](#d-linkpermissions--per-relationship-read-codes-gating-the-module-endpoint-and-the-traversal-arm-alike) | Per-relationship read codes gating the module endpoint and the traversal arm alike |
| [D-Temporal](#d-temporal--a-three-tier-link-history-classification-native-validity-by-default-plus-getobjecthistory-over-the-audit-ledger) | A three-tier link-history classification (native validity by default) plus getObjectHistory over the audit ledger |
| [D-EnvConfig](#d-envconfig--environment-variables-override-the-yaml-config-and-the-yaml-file-is-optional) | Environment variables override the YAML config, and the YAML file is optional |
| [D-ObjectFacets](#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) | One per-object-type facet vocabulary driving both list filters and per-module stats endpoints |
| [D-ConsoleDashboards](#d-consoledashboards--every-listable-type-gets-a-list-view-and-a-dashboard-view-over-one-url-borne-filter-set-amends-d-webui) | Every listable type gets a list view and a dashboard view over one URL-borne filter set |
| [D-MultiIdPExamples](#d-multiidpexamples--public-idps-are-supported-in-two-documented-topologies-an-oidc-issuer-must-pin-an-audience-extends-d-jit-amends-d-webui) | Public IdPs in two topologies (brokered / direct); an `oidc` issuer must pin an audience |
| L-\* locks | [Carried-over locks](#carried-over-locks-settled-earlier-restated-for-self-containment): L-AuthzOnly, L-AccountOptional, L-SingleDomain, L-UnitIsTenant, L-OneRankScheme, L-Visibility, L-OperatorDB, L-UpgradeSafe, L-Conventions |

---

## Resolved this session

### D-Graph — The unit hierarchy is a DAG

**Decision.** Units relate by parent→child edges and may have **multiple parents**; there
may be **multiple roots** (units with no parent). Cycles are forbidden. Storage: an explicit
**edge table** (`tenant_unit_edges`) plus a maintained **transitive-closure table**
(`tenant_unit_closure`, `ancestor → descendant + depth`). Cycle prevention is enforced when
an edge is inserted (the new edge must not make an ancestor of the parent a descendant of the
child).

**Why.** Real organizations are not strict trees — a unit can report into more than one
parent (operational + administrative chains; a department under two faculties). The closure
table makes the PDP's ancestor/descendant test a single indexed lookup instead of a recursive
walk on every decision.

**Consequence.** The PDP resolves inheritance over the closure. Tenant owns closure
maintenance + cycle prevention as an invariant. See [tenant](../modules/tenant.md).
**Amended by D-Graphs** — edges are *typed* (each belongs to one named graph) and the closure
is maintained **per graph**; cycle prevention is per graph. **Further amended by
[D-TenantOrganizations](roadmap-decisions.md#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph)
(M40)** — graphs are scoped **per organization** (`tenant_graphs.org_id`, **nullable**: NULL = an
instance-global/cross-org graph, e.g. religion's taxonomy graphs); a unit carries `org_id` +
`domain_id`, and the closure/PDP isolate per org.

### D-Inherit — Inheritance is per-assignment scope

**Decision.** A role assignment is `(subject_person, role, target_unit, scope)` where
`scope ∈ {unit, subtree}`:

- `subtree` → the role's permissions apply at `target_unit` **and all descendants** (cascades
  across the DAG). For a queried unit U, effective permissions = the **union over every
  ancestor of U** that carries a `subtree` grant, plus `unit` grants at U.
- `unit` → permissions apply **only at `target_unit`**; descendants receive **nothing — not
  even read**.

`target_unit` is **independent of where the subject is placed** in the org — a low-placed (or
low-ranked) person can hold a `subtree` role on a high-level unit. There is **no
per-permission filtering** within an assignment.

**Why.** The user's requirement: some people must hold authority over a high-level unit and
everything beneath it; others must be confined to exactly one unit with no leakage downward.
Making scope an explicit property of the assignment expresses both cleanly and keeps the
decision rule simple.

**Consequence.** This is the PDP's core algorithm. Safety against over-broad grants comes from
reversibility + audit, not from filtering. See [authorization](../modules/authorization.md).
**Amended by D-Graphs** — a `subtree` assignment additionally names the **graph** whose closure
it cascades over; `unit` scope stays graph-independent; effective permissions **union across
graphs**.

### D-Graphs — Multiple named hierarchies (typed edges, per-graph closure)

**Decision.** The unit graph is not one DAG but a **set of named, instance-admin-managed
hierarchies** ("graphs") over the same units — seeded with **`command`** (the structural /
administrative authority chain; the default, undeletable) and **`operational`** (mission /
task-organization, OPCON-like). The set is **registry data** in a new `tenant_graphs` table
(stable `code` + translatable `name`, per D-Code / D-i18n), managed exactly like the locale
registry: an instance permission `graph.manage` (write) and `graph.read` (a reference read in
`unit-reader`), with the guard that **`command` cannot be deleted and ≥1 graph always exists**.
Each graph is independently a DAG. This yields:

- **Typed edges.** `tenant_unit_edges.graph_id` (NOT NULL); `UNIQUE (graph_id, parent_id,
  child_id)`; the same parent→child pair may exist in more than one graph. Cycle prevention is
  **per graph** (a cross-graph cycle — A commands B while B is operationally over A — is legal).
- **Per-graph closure.** `tenant_unit_closure` is keyed by `graph_id`; an edge change in graph K
  **incrementally adjusts only the affected closure rows of K** in the same transaction (M48,
  review R‑04 — amends the original full per-graph recompute): attach merges
  `anc*(parent) × desc*(child)` via a closure∘closure join with a `LEAST`-depth update (depth
  stays the shortest-path length — the ancestor/descendant listings order by it); detach deletes
  that slice and re-derives it from surviving edges plus closure rows outside the slice, then
  prunes the endpoints' reflexive rows if they left the edge set. Closure maintenance on one
  graph is serialized by a per-graph row lock (`tenant_graphs … FOR NO KEY UPDATE`), which also
  closes the guard-then-insert cycle race; the full recompute survives solely as the
  D-ClosureIntegrity repair path.
- **Graph on the assignment.** A `subtree` assignment names the graph whose closure it cascades
  over (`authz_role_assignments.graph_id`, **NULL iff `scope='unit'`**). A `unit` grant is
  graph-independent.
- **The PDP unions across graphs.** A subject is authorized over U if **any** of their
  assignments reaches U *in that assignment's own graph*. A unit's administrative chain
  (`command`) and its operational commander (`operational`) both confer authority — exactly the
  NATO **ADCON / OPCON** overlap.

**Why.** Real hierarchical organizations place a unit in several overlapping chains that do
**not** confer the same authority: military ADCON (who mans / equips / administers) vs.
OPCON / TACON (who commands it for a mission); matrix reporting in universities or companies
(a department *and* a research centre). The single-graph model as originally resolved
(D-Graph / D-Inherit) could express multi-parent but **not** "associated with B, yet governed
through a different chain" — every parent edge was authority-bearing. Naming the graph on the
edge and on the `subtree` assignment lets distinct chains cascade authority independently and
union at decision time — the actual operational reality — while leaving `unit` scope and the
instance plane untouched.

**Consequence.** **Amends D-Graph** (typed edges, per-graph closure) and **D-Inherit**
(scope × graph, union-across-graphs). The `/authorize(person, action, unit)` **question is
unchanged** — the graph lives in the *assignment*, not the query — so the Conjure contract is
stable; the decision-explain mode (now shipped — see
[authorization](../modules/authorization.md)) reports *which graph* produced ALLOW.
New `tenant_graphs` table + `graph.read` / `graph.manage` permissions; the costs are closure
storage **×(active graphs)** and the operator concept "which hierarchy does this `subtree` grant
cascade over?". A per-graph **`is_authority_bearing`** flag (a graph recorded for directory /
association only, never traversed by the PDP) is **promoted to PDP-enforced state** by
**D-DirectoryGraphs** below. See [tenant](../modules/tenant.md) and
[authorization](../modules/authorization.md). **Amended by
[D-TenantOrganizations](roadmap-decisions.md#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph)
(M40)** — `tenant_graphs` gains a **nullable** `org_id`; the seeded `command` (default, undeletable,
locked authority-bearing) + `operational` graphs and the "≥1 graph always exists" guard become **per
organization** (created in the same transaction as the org), not instance-global. `org_id NULL` =
an **instance-global/cross-org** graph (the religion taxonomy graphs); the code-active index splits
into per-org and global partial-unique indexes, and the single-default index keys on
`COALESCE(org_id, sentinel)`.

### D-Rank — Rank on person; rank ≠ permission

**Decision.** A `person` holds **one rank per rank system** (D-RankSystems), drawn from the single
system-wide scheme — so a person on a single-system deployment still holds at most one rank, while a
multi-track deployment may carry **concurrent** standings (one per system). **Rank is a directory
attribute and grants no authorization** — authority comes only from role assignments. (Position is
covered by D-Position.)

**Why.** Military/academic reality: rank (Sergeant, Professor) is a person's standing across the
whole organization. Coupling it to permissions would make authorization implicit and
unauditable. **Why one-per-system (not one global rank):** the army frame ("a person's single
standing across the whole organization") does not hold for the **university** and **church**
verticals, where concurrent tracks are routine — an academic who is also a **Dean** (the seeded
`academic` / `administrative` categories are parallel branches), or clergy whose standing is a
separate ladder. A single global `rank_id` forces those into a `membership` position (conflating
*seniority* with *billet*, the very split D-Rank/D-Position keep clean). Scoping "one rank" to "one
rank **per rank system**" — which the D-RankSystems machinery already half-implied — lets the second
track be a genuine rank in its own system without touching authority semantics.

**Consequence.** The HOLDS_RANK link is **reified** as `person_ranks (person_id, system_id, rank_id)`
with `UNIQUE (person_id, system_id)` among active rows; `system_id` is derived from the rank
(`rank → type → category → system`) and denormalized for the uniqueness check. The legacy single
`person_persons.rank_id` column is removed. The PDP never reads rank. See [rank](../modules/rank.md),
[person](../modules/person.md).

**Scheme shape.** The scheme is a **rank category** at the top, a **tree of rank types** within each
category (`rank_types.parent_type_id` self-FK; `NULL` = a root type of the category), and **ranks on
the leaf types**. A type's `category_id` is the root category, denormalized onto every type so
grouping, sibling code-uniqueness, and seniority need no recursive walk; codes are unique among
siblings (same category + parent). Ranks attach to **leaf types only** (a type with child types holds
no ranks, and vice-versa); `parent_type_id` is immutable after creation (no cycles; reparenting is an
open seam). Earlier the type level was a flat list under the category — the tree generalizes it so
real nested bands (e.g. *officers* → *junior/senior/general*) are first-class. **Extended by
[D-RankSystems](#d-ranksystems--multinational-rank-systems-standardized-grade-comparability-and-scheme-presets-extends-d-rank-refines-l-onerankscheme):**
a top **`rank_system`** level (above category) lets one registry hold several national systems (US, UA),
ranks carry an optional standardized **`grade_code`** (NATO STANAG 2116) for cross-system comparability,
and a `rank_system` subtree can be populated from a preset via `POST /rank-scheme/import`.

### D-Position — Position is a unit-owned billet that can be vacant

**Decision.** A **position is a billet belonging to a unit** (`unit_id`), with a stable `code`,
a translatable title, and an optional `required_rank_id`. It **exists vacant by default**; a
person **fills** it through a **membership** that references it. Memberships without a position
("belongs to unit") are also allowed. A **vacancy** is an active position with no active
filling. A position is a single billet → **at most one active filling** (multi-incumbent is a
seam). Managing a unit's positions is **unit-scoped**. Position grants **no authorization**.

**Why.** The user's requirement: positions are like *vacancies* — establishment slots that exist
whether or not someone fills them (the org's table of authorized billets). This is the standard
TO&E/establishment model and reconciles "position lives in membership" (a filled membership
references its billet) with "positions can be vacant" (unfilled billets have no membership).

**Consequence.** `membership_positions(unit_id, code, title, required_rank_id?, …)` and
`membership_memberships.position_id` (nullable) are owned by
[membership](../modules/membership.md). Supersedes the Increment-1 "instance-managed position
catalog" phrasing. Person *names* get per-record transliteration variants (person-managed, not
the translation store).

### D-InstanceAdmin — A separate instance-admin scope

**Decision.** The "top-permission role" is an **instance-level authority scope**, distinct
from unit role assignments. It holds instance-wide permissions: manage the rank scheme, define
roles, manage supported locales & translations, edit global config. Bootstrapped at install
(**how**: D-Bootstrap).

**Why.** Instance-wide configuration is not "about a unit," so binding it to a unit assignment
would be a category error. A separate plane (see [instance-scope vs unit-scope
pattern](patterns.md)) lets unit admins be powerful within their subtree yet unable to touch
deployment config.

**Consequence.** `authz_instance_admins` is a distinct table; the PDP unions instance-admin
permissions unconditionally. See [authorization](../modules/authorization.md).

### D-PersonGlobal — Person is instance-global

**Decision.** A person is **one record for the whole deployment**, not per-unit. The same
individual is a single `person` regardless of how many units they belong to.

**Why.** It is a *personnel directory* for one organization, and person-centric by design.
Per-unit person records (the drafts model, built for cross-tenant SaaS) would fragment the
directory and defeat "one person, many memberships."

**Consequence.** `person_persons` has no unit FK; the person↔unit relationship lives entirely
in [membership](../modules/membership.md). Departs from `drafts/` ADR-0011 §5.

### D-NoRLS — App-layer PDP, no RLS for unit isolation

**Decision.** Tenant/unit isolation is **not** enforced with Postgres Row-Level Security.
Authorization is the app-layer **PDP** + the **shadow-visibility gate** on reads.

**Why.** A deployment serves one organization; units are not mutually-distrusting tenants, so
the drafts' RLS-per-tenant model does not apply. A single PDP is the product's value and the
single point to reason about access.

**Consequence.** No per-table RLS policies *as the isolation/authorization model*. Departs from
`drafts/` ADR-0018 §3.6. See [conventions.md](conventions.md).

**Amended by D-RLSDefenseInDepth** — this decision rejected RLS as the *isolation/authz model*
(the PDP is and remains authoritative), **not** RLS as a backstop. RLS is now **enabled as a
DB-level defense-in-depth layer** that mirrors the PDP-computed read/write reach via per-transaction
`app.*` session GUCs. The "no GUC dance" line above is superseded by that contract.

### D-Migrations — Atlas versioned migrations, one location, lint gate

**Decision.** Schema evolves through **Atlas versioned migrations** in a **single repo-root
`migrations/` directory**. `atlas migrate lint` runs in CI with a **destructive-change gate**;
any drop/narrowing fails the build unless explicitly signed off and documented. Releases follow
**expand/contract**. CI runs **upgrade tests** (apply from each prior released version to HEAD,
assert invariants + row counts). The service performs a **boot-time schema-version check**.

**Why.** The locked non-destructive-upgrade guarantee needs deterministic, reviewable,
forward-only migrations with a hard gate on data-loss, not declarative auto-diffing that could
silently drop. (This resolves the layout question deferred by the high-level plan.)

**Consequence.** See [upgrade-safety.md](upgrade-safety.md) and
[platform](../modules/platform.md) (boot check). sqlc reads the same schema.

### D-Conjure — Conjure-first API, `SerializableError`, token pagination

**Decision.** The API is **Conjure-first**: `*.conjure.yml` is the source of truth; Go server
interfaces, clients, and the OpenAPI reference site are generated. The error envelope is
Conjure `SerializableError`; pagination is token-based.

**Why.** Contract-first gives generated clients + docs for free and compiler-enforced
transport. Aligns the service with the Palantir stack it showcases.

**Consequence.** See [conventions.md](conventions.md) API + Conjure sections.

### D-Stack — The Palantir OSS stack (reverses drafts' fx)

**Decision.** witchcraft-go-server, conjure, gödel, conjure-go-runtime,
witchcraft-go-logging/tracing/metrics/health, werror, ECV + `pkg/refreshable`; pgx + sqlc;
Atlas. This **reverses** the `drafts/` choice of `uber/fx` + generic OpenAPI.

**Why.** The service is explicitly built to be a reference implementation of the Palantir OSS
stack (and attractive to Palantir). The stack also delivers the observability/audit posture
the product wants.

**Consequence.** See [overview.md](overview.md) stack table.

### D-i18n — i18n is required; all translations in every response

**Decision.** Localization is a required feature. **Supported locales are instance-admin-managed
data**, seeded with **`ukr` + `eng`** (ISO 639-3), and the admin can add/disable more (never
below one enabled). The translatable `name`/title/description of units, graphs, ranks (category/type/
rank), positions, and roles lives in a shared **translation store** owned by the
[localization](../modules/localization.md) module. **Every response returns all locales as a
`locale → text` map — there is no Accept-Language negotiation** (user's choice). **Person name
transliteration** is the exception: per-person, person-managed name variants, *not* the admin
translation store. drafts' "no Russian locale" rule is **dropped** (domain-agnostic).

**Why.** The deployments are real multilingual organizations (UA context: Ukrainian + English at
minimum). Returning all translations keeps clients and the server simple and makes admin editing
trivial. This **reverses** the Increment-1 "locale-agnostic, no UI-locale machinery" line in
conventions.md.

**Consequence.** New module [localization](../modules/localization.md) (`oikumenea.i18n_*`); a
translatable field is a locale-map assembled from the entity's default-locale `name` + the
store; see [conventions.md](conventions.md) (i18n) and [patterns.md](patterns.md) (Translatable
label). **Extended by D-Documents / D-Orders:** the catalog type names `document_type` and
`order_type` are translatable on the same footing as the entities listed above, so the
`i18n_translations.entity_type` set and the translatable-entity enumerations include them.

**Amended (review R-19, 2026-07-11): an optional `locales=` projection bounds response size without
reversing the decision.** All-locales-in-every-response stays the **default**; a client MAY pass a
repeated `locales=ukr&locales=eng` query param to trim every translatable label map to that subset
(intersection with what is stored). This is a payload-size **projection, explicitly NOT
Accept-Language content negotiation** — the server does not choose a locale, it returns exactly the
subset asked for, and absent/empty means all locales. Demonstrated end-to-end on `GET
/platform/v1/colors` (`listColors`, [platform.conjure.yml](../../api/platform.conjure.yml)); the
transport-layer projection helper generalizes to any label-map endpoint. Response size on list
endpoints scales ×|locales|, so this is the cap for an instance that enables many locales (no hard
locale-count limit is imposed — the admin owns the locale set). Same pass: the fallback default-locale
lookup (`DefaultLocale`), consulted on every `LabelsByID`/`NamesByID` call, is now cached with a short
TTL + write-invalidation (it was a per-invocation `ListLocales` round-trip).

### D-Code — Stable, locale-agnostic codes separate from translatable names

**Decision.** Every structural/catalog entity carries a stable, unique, **locale-agnostic
`code`** — the machine identifier external systems reference in their own code — **separate from
its translatable `name`**. Applies to units, roles, positions, ranks (already coded), locales;
permissions are the degenerate case (the permission string *is* the code); persons get an
**optional** `code` (e.g. personnel/service number). Codes are **immutable by convention**.

**Why.** The user's requirement: external systems must refer to tenants/roles/permissions/etc.
by a stable handle that does not change when a display name is edited or translated. Splitting
the stable `code` from the translatable `name` makes both jobs clean.

**Consequence.** `code TEXT NOT NULL UNIQUE` on structural entities; the prior unit `slug`
becomes `code` (an API-only service has no subdomains). See [conventions.md](conventions.md)
(Code vs. name) and [patterns.md](patterns.md) (Stable code vs translatable name).

**Amended by D-UnitCodeLifecycle (M28)** — for **units only**, `code` becomes **optional**
(a codeless unit is a non-separate sub-unit) and **mutable** (a correctable human-readable ID set
via an audited recode op). The stable machine handle external systems reference is the **RID**, not
the unit `code`. Other structural/catalog entities keep `code TEXT NOT NULL UNIQUE`, immutable by
convention. See [roadmap-decisions.md](roadmap-decisions.md) (D-UnitCodeLifecycle).

**Extended by [D-TenantOrganizations](roadmap-decisions.md#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph)
(M40)** — the new tenant catalog/realm entities `tenant_domains`, `tenant_unit_kinds`, and
`tenant_organizations` carry `code TEXT NOT NULL UNIQUE`, immutable-by-convention, with translatable
`name` on the i18n footing (the unit `code` carve-out is unchanged). Free-text `tenant_units.unit_kind`
is replaced by a `kind_id` FK into the domain-scoped `tenant_unit_kinds` catalog.

### D-Audit — Every write is audited; audit reads are permission-scoped

**Amended by D-AuditRetention** (physical layout only): the `audit_log` table is monthly
range-partitioned with a trimmed index set and an operator retention policy. The semantics below —
same-transaction, append-only, one row per Action — are unchanged.

**Decision.** Every **write** (state mutation) in every module — create / update / state
transition / soft-delete / purge / grant / revoke / link / unlink — records an audit entry in the
**same DB transaction** as the change (the audit row commits iff the change commits). Denied
attempts on write actions are recorded with `outcome='denied'`. **Reads are not audited.** The
action list in [audit](../modules/audit.md) is **representative, not exhaustive** — completeness is
the rule; the list only illustrates it. Each entry names its **actor**: a `person` (all delegated
administration, exercised through permissions — unit admins, tenant creators, grantors)
or `system` (automated/internal action, including the install **bootstrap** path — D-Bootstrap),
named in a `subsystem` column. There is **no** `super_admin` actor entity (OQ-1): an instance admin
is a `person`, marked instance-scoped by the action's permission. Reading the
log is gated by `audit.read`, **unit-scoped exactly like `person.read`** (PDP over the closure +
shadow gate), and the audit query is filterable by **every audited entity type**, so read coverage
mirrors write coverage.

**Why.** Governance posture (D-Stack, Palantir-grade auditability) plus the D-Inherit consequence
already on file — *safety against over-broad grants comes from reversibility + audit, not from
filtering* — only hold if write coverage is **complete**. An enumerated allow-list silently drops
new write paths (person create/update, i18n edits, transliteration) as the service grows; making
"every write" the invariant closes that gap. Symmetrically, an audit trail is only useful if it can
be *read* by the right people at the right scope — so audit reads reuse the unit-scoped PDP model
rather than an all-or-nothing flag.

**Consequence.** [audit](../modules/audit.md)'s list becomes examples; every write-bearing module
calls the audit recorder in-transaction (see the *Audit-on-write* pattern in
[patterns.md](patterns.md)). `target_type` is a closed audited-entity vocabulary that every filter
keys on. The two seams once deferred here are now **resolved**: `super_admin` is **not** a distinct
entity (OQ-1, D-Bootstrap — an instance admin is a `person`); the subsystem behind a `system` action
is named in the `audit_log.subsystem` column (OQ-2). See [audit](../modules/audit.md) for the
`actor_type` / `subsystem` columns and their CHECK pairing.

### D-Bootstrap — Install-time bootstrap of the first instance admin

**Decision.** The first instance admin is seeded at **first boot** from a `bootstrap_admin` block in
the operator-owned, ECV-encrypted `install.yml` (`{ issuer, subject | email, display_name,
person_code? }`). If **no** instance admin exists yet, the service seeds — in one transaction — a
`person` → an `account` + `external_identity (issuer, subject)` → an `authz_instance_admins` grant;
the operation is **idempotent** (skips entirely once any instance admin exists). All seed writes are
audited with `actor_type='system', subsystem='bootstrap'` (D-Audit). The **unit graph starts empty**
— no placeholder unit is seeded; the seeded admin creates the first **root** unit through the normal
(instance-scoped `unit.create`) API. Bootstrap-origin grants set provenance columns (`granted_by`,
edge `created_by`) to **NULL**; origin lives in the bootstrap audit row.

**Why.** Authentication is delegated (L-AuthzOnly), so bootstrap binds an **IdP identity**, not a
credential — safe to keep in encrypted config and natural for self-hosted/containerized deploys. The
no-self-escalation invariant means the first admin cannot be granted from inside the API, so it must
be seeded out-of-band; config-seed needs no shell. Recovery/break-glass is operator-owned DB access
(L-OperatorDB; the operator owns Postgres), **not** a runtime super-tier — which resolves OQ-1:
there is no entity above the instance admin (cf. AWS root / k8s `system:masters` exist only because
those operators don't control the substrate).

**Consequence.** See [platform](../modules/platform.md) (`bootstrap_admin` config + first-boot seed),
[identity-federation](../modules/identity-federation.md) (first account/identity), and
[authorization](../modules/authorization.md) (`granted_by` NULL on the bootstrap grant).

**Recovery CLI (resolved — was a parked seam).** Recovery from a **lost sole instance admin** is now
the idempotent **`recover-admin` / `bootstrap-admin` CLI** on `cmd/oikumenea` (it reuses this same
seed transaction), **not** raw DB surgery. It is gated on *no active instance admin exists* OR an
explicit `--force`, respects the boot-time schema-version check, and is **operator-host-gated** —
possession of operator DB/host access is the authorization (the same trust level as the raw-DB path
it replaces; still **not** a runtime super-tier, so OQ-1 stands). Its writes audit as
`actor_type='system', subsystem='recover-admin'`. See [platform](../modules/platform.md).

### D-BaseRoles — Seeded base roles; reads are explicit grants

**Decision.** Four `is_base = TRUE`, **unit-scoped** base roles ship seeded (assignable with `unit`
or `subtree` scope), defined in code alongside the permission catalog:

- **`unit-reader`** — in-scope reads: `unit.read`, `person.read`, `membership.read`, `position.read`,
  `role.read`, `assignment.read`, plus the reference reads `rank.scheme.read`, `graph.read`,
  `locale.read`, `translation.read`.
- **`unit-manager`** — `unit-reader` + `unit.create/update`, `person.create/update`,
  `person.rank.assign`, `membership.create/update`, `position.create/update`.
- **`unit-admin`** — `unit-manager` + `unit.edges.manage` (broad form, covers all graphs incl.
  custom — **amended by D-EdgePerms**), `unit.lifecycle`, `person.lifecycle`,
  `person.purge`, `assignment.grant`, `assignment.revoke`.
- **`auditor`** — `audit.read` only (separation of duties; assigned alongside `unit-reader` when the
  auditor must resolve referenced entities).

Instance-only permissions (`role.create/update/delete`, `rank.scheme.manage`, `graph.manage`,
`locale.manage`, `translation.manage`, `instance.config`, `instance.admin.manage`) are held on the
**instance-admin plane** (D-InstanceAdmin), never via a base role. `rank.scheme.read` is **added to the catalog**.
**Read access is an explicit grant** — there is no implicit "any authenticated caller may read"
exemption; broad read is achieved by assigning `unit-reader` at a root with `subtree` scope. For the
same reason, `POST /authorize` and `/authorize/batch` require **`assignment.read`** with **no "self"
exemption** for arbitrary `(subject, action, unit)` questions (the narrow self-only carve-out is
**D-SelfCapabilities**, below).

**Why.** Mirrors the graduated, namespace-scoped Kubernetes defaults (`view`/`edit`/`admin`, with
`cluster-admin` ≡ the instance plane) — a natural fit for `unit`/`subtree` scoping. Explicit reads +
a dedicated `auditor` keep the model uniform (everything is a granted permission) and serve
least-privilege / separation-of-duties. Inline localized labels are server-assembled, so gating the
reference reads breaks no normal rendering.

**Consequence.** See [authorization](../modules/authorization.md) (base-role enumeration, the
`rank.scheme.read` catalog addition, the `/authorize` permission). Base roles are immutable by
instance admins (`is_base`).

### D-SelfCapabilities — Self-only capability introspection (`GET /me/capabilities`, resolves OQ-5)

**Decision.** Add one narrow, authenticated-but-**ungated** endpoint, `GET
/authorization/v1/me/capabilities`, returning `MyCapabilities { permissions: list<string>,
isInstanceAdmin: boolean }` — the flat, deduped union of the **caller's own** active-grant permission
codes plus whether they are an instance admin. The subject is taken from the request context (never a
parameter); machine/service-principal subjects receive an empty set. This is distinct from `POST
/authorize`, which stays gated on `assignment.read`: `/me/capabilities` answers only "what do *I*
hold?", never a `(subject, action, unit)` question, and exposes no assignment structure or other
subject.

**Why.** The console must hide modules a user cannot read. The prior path — one `POST /authorize`
probe per module code — fails closed for every non-admin (the probe itself needs `assignment.read`),
so it could only ever hide the two admin tools. A user learning their *own* effective permission
codes is standard, low-sensitivity self-introspection; the general-question sensitivity that motivated
"no self-exemption" (D-BaseRoles) does not apply. Resolves the OQ-5 deferral.

**Consequence.** UI gating built on this is **cosmetic only** — every endpoint still re-decides
through the PDP (D-NoRLS, D-RLSDefenseInDepth); the capability list permits nothing. The frontend
maps each object-type/tool to its `<module>.read` code and filters nav/palette/explorer/ontology by
the returned set (see [authorization](../modules/authorization.md); [D-WebUI](#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)).

### D-JIT — Just-in-time provisioning is link-on-match only

**Decision.** The default for an unknown verified inbound identity is **reject** (unchanged). When
JIT is enabled, the only behavior is **link-on-match**: the verified token is matched against an
**existing** `person` via an operator-configured mapping (a token claim → `person.code` or a
designated attribute); on a match the service creates the `account` + `external_identity` and links
them; on no match it **rejects**. JIT **never creates a person**.

**Why.** go-oikumenea is a personnel directory first, account-optional (L-AccountOptional): people
are placed on the roster (with rank/membership) before they ever log in, so first login is a *link*,
not a *create*. Auto-creating persons from external assertions harms directory hygiene and yields
empty, unauthorized records. This is the same link-to-existing model bootstrap uses (D-Bootstrap).

**Consequence.** See [identity-federation](../modules/identity-federation.md) (inbound validation
step 3 + the configurable claim→person-key mapping). Full auto-enrolment remains out of scope.

**As built.** Both mappings this decision allows now exist, selected by `idp.jit.match`: the
`person.code` arm (default, shipped with M8) and the **designated-attribute** arm `account-email`,
which matches the claim against `account_accounts.email` — unique among active accounts by index, so
"the single account" is true by construction. That arm exists because an operator enrolling people
usually knows an ADDRESS, not the IdP's opaque subject; they create a login-less shell account
carrying the email and the person's first sign-in attaches its `(issuer, subject)`. It requires
`email_verified` to be present and true, and it makes every configured issuer trusted to verify email
honestly — which is why it is opt-in and documented in
[`deploy/oauth/README.md`](../../deploy/oauth/README.md). Linking on this path is subject to
`account.identity_linking.enabled`, closing a gap where the login path could exceed a cap the admin
endpoint enforced.

### D-DirectoryGraphs — Graphs may be directory-only (PDP-enforced flag)

**Decision.** Each row in `tenant_graphs` carries
`is_authority_bearing BOOLEAN NOT NULL DEFAULT TRUE`. A graph with the flag TRUE cascades
`subtree` grants in the PDP exactly as in D-Graphs. A graph with the flag FALSE is **directory-only**:
its edges and closure are maintained for display / association, but the PDP **never cascades
through it**, and the assignment-write path **rejects** any `(scope='subtree', graph=G)` where
`G.is_authority_bearing = FALSE` with `Authorization:NonAuthorityBearingGraph`. The seeded
`command` graph is **locked to TRUE** forever — the structural chain cannot be made directory-only.
Other graphs: the flag is set at graph creation and mutable post-creation under one guard —
**TRUE→FALSE is allowed only when the graph has no active `subtree` assignments** (same shape as
the existing graph-deletion guard); **FALSE→TRUE is always safe**. The PDP also filters on
`is_authority_bearing = TRUE` in step 3 as defense-in-depth.

**Why.** Real hierarchical organizations express *associative-but-not-commanding* relationships
distinct from authority chains: NATO **DIRLAUTH** (direct liaison authorized — explicitly not a
command relationship) and **coordinating authority**; university matrix research-centre
affiliations alongside the department reporting line; deaneries in some ecclesiologies.
Without the flag, the registry can represent these graphs technically but the system cannot
enforce "no authority cascades here" — an operator can mis-grant a `subtree` on a graph they
treat as display-only and authority silently leaks. Promoting from a reserved seam to
PDP-enforced state makes the registry **self-policing** instead of relying on operator
convention.

**Consequence.** **Amends D-Graphs** (the reserved-seam mention is now resolved). New column
`tenant_graphs.is_authority_bearing`; the `command` row is CHECK-bound to TRUE. New Conjure
error `Authorization:NonAuthorityBearingGraph` on the grant path; `POST /graphs` body grows an
optional `isAuthorityBearing` (default TRUE); `PUT /graphs/{id}` may flip the flag subject to
the guard above. The PDP's step 3 filters `subtree` cascade on `graphs.is_authority_bearing =
TRUE`. See [tenant](../modules/tenant.md) (schema + guards) and
[authorization](../modules/authorization.md) (PDP step + write guard). Removed from
[open-questions](../open-questions.md) (was DS-32).

### D-EdgePerms — Edge management is per-graph (code-defined permissions + broad fallback)

**Decision.** Edge mutations on `tenant_unit_edges` are gated by **per-graph code-defined
permissions** `unit.edges.<graph_code>.manage`, seeded for the two seeded graphs:
`unit.edges.command.manage` and `unit.edges.operational.manage`. A **broad fallback**
`unit.edges.manage` is retained in the catalog as a separate code. `POST /units/{id}/edges`
and `DELETE /units/{id}/edges?graph={g}…` require the caller's effective set at the path unit
to contain **either** `unit.edges.<g>.manage` **or** the broad `unit.edges.manage` (unit-scoped
check; scope semantics unchanged). The base role **`unit-admin` keeps the broad
`unit.edges.manage`** — preserving current behaviour and ensuring it works for **custom graphs**
(instance-admin-added graphs that have no specific per-graph code yet; permissions are
compile-time, graphs are runtime data, per D-Code). Operators wanting the NATO ADCON-vs-OPCON
split craft a custom role holding only `unit.edges.command.manage` (or only the operational
form).

**Why.** Real hierarchical organizations vest **ADCON** (re-parenting administratively in
`command`) and **OPCON** (re-tasking operationally in `operational`) in **different
commanders**. A single `unit.edges.manage` conflates the two and forces operators to choose
between one omnipotent edge admin or no delegation at all. Per-graph permissions express the
doctrine; the broad fallback keeps the model usable for small deployments and for custom graphs
where a per-graph permission code does not yet exist.

**Consequence.** **Amends D-BaseRoles** (the `unit-admin` row's edge permission stays the
broad form on purpose; this is a deliberate, documented choice — not the only valid pick).
Permission catalog grows from `{unit.edges.manage}` to `{unit.edges.manage,
unit.edges.command.manage, unit.edges.operational.manage}`. Edge-mutation PEP becomes
`effective ⊇ {unit.edges.<graph>.manage} OR effective ⊇ {unit.edges.manage}`. New
instance-admin-added graphs **fall through to the broad permission** until a release ships
their specific per-graph code (consistent with D-Code's "permissions exist only in code"
invariant). See [authorization](../modules/authorization.md) (catalog, base-role row,
invariants) and [tenant](../modules/tenant.md) (edge-endpoint Perm cells). Removed from
[open-questions](../open-questions.md) (was DS-33).

### D-ClosureIntegrity — On-demand per-graph closure verify & rebuild (decoupled from the worker runtime)

**Decision.** The derived `tenant_unit_closure` table gains a **runtime integrity path** beside
its incremental maintenance: two **synchronous, admin-triggered, per-graph** operations on
`TenantService` —

- **verify** — recomputes the transitive closure of a graph's edges and diffs it
  against the stored closure, returning a drift report (missing / extra row counts + a sample).
  Since M48 the diff is **depth-inclusive** (a stored row with a wrong shortest-path depth counts
  as drift, one row in each direction), so incremental-maintenance depth bugs are reportable too.
  A read → **not audited** (D-Audit); it additionally **upserts a per-graph diagnostic status
  overlay** (`tenant_closure_status`) that the `closure-drift` health reporter consumes
  (D-ClosureDriftHealth) — derived health metadata, not an audited domain mutation.
- **rebuild** — truncate-and-recompute the affected graph(s)' closure rows, **one transaction
  per graph** (the same transactional discipline as the incremental edge path). A write →
  **audited** (D-Audit; `actor=person`, target = the graph).

Both omit-the-`graph`-param ⇒ all graphs. Both are gated by a **new instance-scope permission
`closure.rebuild`** (admin-plane diagnostics/recovery; never in a base role). This **needs no
scheduler** — it is on-demand, so it does **not** depend on the worker runtime (DS-25). It is
also the natural payload for the `recover-admin` CLI (D-Bootstrap), but the endpoint is the
primary surface.

**Why.** The closure is a derived table maintained by application code; under `L-OperatorDB`,
operators also have **raw DB access for recovery**, so silent drift (a maintenance bug, a manual
edit, a partial failure) is a real failure mode whose only current remedy is more raw DB surgery.
Materialized-transitive-closure authz systems handle exactly this with a **rebuild-from-source-of-
truth** path kept separate from the write path — Google Zanzibar's **Leopard** index (rebuilt
from the changelog), Active Directory's **KCC** consistency checker, and the classic
closure-table **reconciliation** pattern. The `tenant.md` invariant "each graph's closure equals
the transitive closure of its edges" was test-time only; this gives it a **runtime** counterpart
and an operator-facing repair tool. Splitting this off **narrows DS-2** to its other, weaker
half — a *scheduled, churn-driven* full rebuild — which stays parked behind DS-25 (the small,
rarely-re-orged org graph makes "edge churn dominates" unlikely to ever fire).

**Consequence.** New instance-scope permission `closure.rebuild`; two new `TenantService`
endpoints (`POST /closure/verify`, `POST /closure/rebuild`); the rebuild is an audited write.
**Narrowed DS-2** to the scheduled background job only, now **resolved by D-ClosureDriftHealth**
(detection is surfaced via a diagnostic health reporter; the scheduled auto-rebuild is ruled out).
See [tenant](../modules/tenant.md) (endpoints + invariant) and
[authorization](../modules/authorization.md) (catalog + base-role exclusion).

### D-PIITiers — 5-tier PII classification via `COMMENT ON COLUMN`

**Decision.** Every PII-bearing column carries a machine-parseable classification comment
`COMMENT ON COLUMN <col> IS 'pii:<tier>'` with a fixed **5-tier** vocabulary
(`pii:sensitive` added by **D-CryptoProvider**):

- `pii:none` — not personal data (codes, FKs, enums, timestamps).
- `pii:basic` — identifying personal data (`display_name`, CLDR name parts (`given`, `surname`, …),
  personnel `code`, IdP `subject`).
- `pii:contact` — contact / locator data (`email`, phone, address, residence).
- `pii:sensitive` — **national-identifier-class** government codes (tax number, national ID,
  social-/health-insurance number). Highly identifying, fraud-relevant, and legally controlled, but
  **not** GDPR Art. 9. This tier is the **machine-parseable "envelope-encrypt at rest" marker**
  (D-CryptoProvider) and drives stricter log redaction + tighter read scope.
- `pii:special` — **GDPR Art. 9** special-category (religion, health, biometrics, ethnicity).

`pii:sensitive` sits **between `pii:contact` and `pii:special`** in handling strictness; it is kept
distinct from `pii:special` because national IDs and Art. 9 data carry different legal regimes and
the envelope-at-rest obligation attaches to `pii:sensitive` specifically (Art. 9 data remains
**blocked** pending its own envelope seam — see below).

JSONB grab-bags (`person.attributes`, `document.attributes`, `audit.before`/`after`) are tagged at
their **ceiling** (`pii:special`) with a governance note: special-category data must **not** land
there without the envelope-encryption seam (**DS-29** for audit; a person-side equivalent).
**Clarified (F-013)** — this ceiling is a **convention-only control**: the `pii:special` comment
classifies the column but does **not** gate writes. There is no write-time reject for Art. 9 keys
(the `document` `attr_schema` validator checks *shape*, not special-category *content*;
`person.attributes` is unvalidated), so the "no special-category PII without the envelope" rule is an
**accepted residual risk**, not an enforced guarantee. Adding such a guard is out of scope until the
`pii:special` envelope seam (the DS-29 family) ships. **Secrets**
(dormant `account.password_hash`) are marked `secret` — a separate axis, not a `pii:` tier.
Applied **instance-wide** (person + name variants, identity-federation accounts, audit payloads,
the `document` module's personal codes).

**Why.** The target domains make the top tier unavoidable — a **church** deployment implies
*religious affiliation* and an **army** one can touch health/biometrics, both GDPR Art. 9. A
machine-parseable comment (not just prose) lets tooling — and an `atlas migrate lint`-style
check — assert that new PII columns are classified. The tiering is the **static-classification**
companion to the two existing runtime controls: `werror.UnsafeParam` log redaction and the
`person` purge (erasure).

**Consequence.** Column annotations across [person](../modules/person.md),
[identity-federation](../modules/identity-federation.md), [audit](../modules/audit.md), and
[document](../modules/document.md); the JSONB ceiling rule; the `secret` marker; **DS-29** named as
the escalation that must ship before special-category PII may enter audit payloads. Resolves **DS-8**.
**Amended by D-CryptoProvider** — adds the `pii:sensitive` tier (national-identifier-class) and the
"`pii:sensitive` ⇒ envelope-encrypt at rest" rule. See
[conventions.md](conventions.md) (PII-classification subsection) and
[glossary.md](../glossary.md) (PII tier).

### D-TimeBoundGrants — Role assignments may be time-bound (`expires_at` active)

**Decision.** `authz_role_assignments.expires_at` is an **active, optional** field (no longer a
dormant seam). `POST /assignments` accepts an optional `expiresAt` (RFC 3339); the PDP treats an
assignment as inactive once `expires_at <= now()` (PDP step 2, already written:
`revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`). Expiry **lapse is silent** —
evaluated at decision time, with no event and no scheduled job (a future sweep ties to DS-25).
The active-uniqueness index stays keyed on `revoked_at IS NULL` (an expired-not-revoked row still
occupies its tuple); **renewal is an update** of `expires_at` on the existing row, and re-granting
an identical expired tuple requires revoking the stale row first.

**Why.** Acting / temporary authority (acting CO during a deployment, TDY details,
delegation-during-absence) is bread-and-butter in the target domains, and time-bound grants are
the core of modern PAM/PIM (e.g. Azure PIM). The data model, index, and PDP step were already
designed for it — activation is surfacing it, not re-architecting.

**Consequence.** `expiresAt` on the grant payload; PDP expiry evaluation is live; lapse is
decision-time/silent; renewal-by-update semantics. Resolves **DS-15**. See
[authorization](../modules/authorization.md).

**Clarified (F-007) — acting authority is a grant, not a position fill.** Acting command,
dual-hatting, and secondment are modeled as a **time-bound role assignment** on the relevant unit,
**not** as a position fill: the substantive holder's membership/position is untouched (the
one-holder billet index never fights the acting case), authority comes only from the assignment
(D-Rank / D-Position), and it reverts silently on lapse. First-class leave/absence as a status and
showing both substantive + acting incumbents *on a billet* remain seams (DS-35 / multi-incumbent).
Worked example + pattern: [patterns.md](../architecture/patterns.md) (*Acting authority via
time-bound role assignment*), [authorization](../modules/authorization.md),
[membership](../modules/membership.md).

### D-RLSDefenseInDepth — PDP-mirror RLS backstop (defense-in-depth)

**Decision.** Postgres RLS is **enabled as a DB-level defense-in-depth backstop** that mirrors the
PDP-computed read/write reach — it does **not** replace the PDP. The app PDP + shadow gate remain
**authoritative**; RLS guards the *forgotten-filter* bug class (a `SELECT` that skips the PDP/gate),
not PDP-logic errors (RLS trusts the app-supplied set). Mechanism:

- Per-**transaction** session GUCs via `SET LOCAL` (auto-reset at txn end): `app.person_id`,
  `app.is_instance_admin` (bool), `app.readable_units` (text[] of unit RIDs — PDP read reach), `app.writable_units`
  (text[] — write reach).
- The app DB role **must not** hold `BYPASSRLS`; instance-admin is expressed via the GUC flag, never
  a DB superuser. Schema migrations run as the owner/migration role (which may bypass).
- The PDP exposes an **effective read/write unit-set** computation (expand each subtree
  read/write-bearing assignment over the graph closure + unit-scope targets) — the same reach the
  shadow gate uses, so RLS mirrors the gate.
- RLS policies on unit-scoped tables (`tenant_units`, `tenant_unit_edges`, `membership_positions`,
  `membership_memberships`, `order_orders` [keyed on `issuing_unit_id`; D-Orders], …):
  `USING (current_setting('app.is_instance_admin')::bool OR
  id|unit_id = ANY(current_setting('app.readable_units')::text[]))`; writes use `app.writable_units`.
- **Tables with no unit column are exempt from the direct predicate** — `person_persons`
  (instance-global; D-PersonGlobal), `document_documents` (scoped via the holder; D-PersonReadScope),
  and `order_order_items` (parent-scoped on its order; reads go through `order_orders`, which *is*
  covered). For these the **app-layer PDP is the authoritative scope**, and reads reach them only
  through a unit-scoped parent/holder that the backstop already guards. A person→unit / parent
  reach-join RLS policy is a **noted hardening seam**, not shipped — consistent with "RLS trusts the
  app-supplied set; it is a backstop against forgotten filters, not against PDP-logic errors."

**Why.** A multi-unit directory's most dangerous, most common bug is a read path that forgets to go
through the PDP/shadow gate and leaks rows. A PDP-mirror backstop makes that class of leak
impossible even when the app forgets the filter. This is the defense-in-depth use **D-NoRLS
explicitly left open** — distinct from RLS-as-the-isolation-model, which is still rejected.

**Consequence.** **Amends D-NoRLS.** New `app.*` session-GUC contract + a per-transaction
GUC-setting seam in [platform](../modules/platform.md); the PDP gains the read/write unit-set
export ([authorization](../modules/authorization.md)); RLS policies + enablement land via
expand/contract (permissive-first, then tighten — see
[upgrade-safety.md](upgrade-safety.md)); the app DB role must lack `BYPASSRLS`. Resolves **DS-17**.

**Amended by D-RLSLiveReach (M47)** — the GUC contract slims to `app.person_id` +
`app.is_instance_admin` (the unit-list GUCs are gone); policies compute reach LIVE via
`oikumenea.authz_unit_in_reach(unit, wr)`, making the backstop exact under revocation. The
backstop/authoritative split and the BYPASSRLS rule above are unchanged.
See [conventions.md](conventions.md).

**Realized mechanism (M11, revision `0011_rls`).** The seam is implemented as a **per-request pinned
connection** rather than per-statement `SET LOCAL`, because unit-scoped reads do not all open a
transaction: the identity-federation authenticator middleware, once it resolves the subject, calls
`authorization.RLSStateFor` (→ `EffectiveReach`) and **pins one pooled connection** for the request
with the four GUCs set via `set_config(name, value, false)` (`internal/platform/db` `AcquireScoped`),
resetting them on release. The four RLS-touching modules (`tenant`, `membership`, `order`, `audit`)
run their reads/writes on that pinned connection (`querier(ctx)`/`reader(ctx)`); a write begins its
transaction on it, so the GUCs cover the `WITH CHECK` too. Reach is computed on the bare pool first
(its reads hit only non-RLS tables + the exempt closure), so there is no chicken-and-egg. Unit sets
are comma-joined RID lists read by the policies as
`<col> = ANY (string_to_array(current_setting('app.readable_units', true), ','))` (the `, true`
missing-ok form means an unset GUC reads as no-reach, never an error). Trusted no-subject paths use
`db.RunAsSystem` (the `app.is_instance_admin` flag), never a DB superuser.

`audit_log` carries a **read-only** backstop: a `FOR SELECT` policy keyed on `unit_id` (NULL ⇒
instance-admin-only) plus a permissive `FOR INSERT` policy, because audit rows are appended from both
request transactions and system paths (first-admin bootstrap, boot seeds) that have no unit reach —
the app, not RLS, governs what is written (append-only is already enforced by `reject_mutation()`).

**Enablement timing (this never-released service).** `upgrade-safety.md` stages RLS as
permissive→tighten so a policy tightening on a **live, already-released** deployment cannot outrun the
GUC plumbing. go-oikumenea has **never been released**: revision `0011_rls` ships the GUC wiring and
the tightened policies **atomically in one revision**, since on a fresh install there is no window in
which the policy outruns the plumbing. The staged (permissive-first) rollout re-applies for any
**post-v1** RLS change.

**A `visibility = 'public'` row is a second, orthogonal, reach-independent SELECT affordance —
`tenant_units_public_read` (migration `0006_person_ext`) and, since migration `0025`,
`religion_sites_public_read` / `religion_service_schedules_public_read` / `religion_aliases_public_read`
(GH-34).** Each is a second permissive `FOR SELECT`-only policy on an already reach-gated table —
Postgres OR-combines permissive policies per command, so a public row is visible **regardless of
caller or grant shape**, while writes and non-public rows stay governed by the table's main reach
policy. This does **not** reopen `D-ServiceIdentities`' M55 rule that an instance-wide
(`org_id IS NULL`) principal grant "confers no operational reach" (`authz_principal_org_in_reach`,
migration `0011_infra`, unchanged) — that rule is about *operational* (org-scoped, non-public) reach.
`visibility = 'public'` rows were always meant to be broadly discoverable by design (the shadow gate's
"public units are discoverable" rule, and religion's `SearchSites` already hard-coding
`visibility = 'public'` in its own query); the RLS backstop had simply fallen out of sync with that
app-layer decision for the religion discovery tables. A person subject and a service principal see
public rows identically — the PEP/permission-code check (e.g. `religion.read`) still decides who may
call the endpoint at all; RLS only mirrors what the app layer already decided is public.

### D-PersonReadScope — A person's read scope projects through its memberships

**Extended by D-PersonSearch**: a text `query` on the directory list folds a pg_trgm predicate
(person names + variants) into this semi-join in SQL (`VisiblePersonIDsForSubjectSearch`), never a
Go-side post-filter — so scoped search stays O(page) and never returns an empty page while `hasMore`.

**Decision.** Read access to a **person** (and, by inheritance, to that person's
[documents](../modules/document.md)) is resolved through the person's **active memberships**, since
the PDP question is unit-keyed (`authorize(person, action, unitID)`) while a person is
**instance-global** with no unit FK (D-PersonGlobal). A subject may read person **P** iff **either**:

1. the subject is on the **instance plane** — an active instance admin, or holds `person.read` as an
   **instance-scope** grant; **or**
2. the subject's **effective readable unit-set** (D-RLSDefenseInDepth: each `subtree` read-bearing
   assignment expanded over its graph's closure ∪ the `unit`-scope `*.read` targets) **intersects**
   the set of units **P** currently belongs to via **active** `membership_memberships` — with the
   **shadow-visibility gate** applied to that join (a membership in a `shadow` unit counts only if
   the subject's `*.read` actually reaches that shadow unit).

A **membership-less** person therefore belongs to no unit, the intersection in (2) is empty, and the
person is readable **only on the instance plane**. `GET /persons/{id}` checks the single-person
intersection; `GET /persons` (directory search) returns the **union** of people reachable this way.
A document's reader must be able to read its **holder** by this rule (and hold `document.read`
reaching one of the holder's units / the instance plane). There is **no per-person "self" read
exemption** (consistent with [authorization](../modules/authorization.md)'s no-self-exemption
posture and "read is an explicit grant").

**Why.** A unit-keyed PDP must project person reads through memberships, and the membership-less
case was previously undefined. Instance-plane-only follows the "read is an explicit grant, no
implicit *authenticated ⇒ may read*" rule: a person not yet attached to the unit graph is not
reachable **through** the graph, so only the instance directory authority sees them. This avoids
both failure modes — silently leaking the entire unattached roster to every unit reader, and hiding
freshly-created people from the admins who own the directory (the create response still returns the
record; routine visibility begins once a membership attaches them).

**Consequence.** The canonical rule lives in [person](../modules/person.md) *Authorization
touchpoints*; [document](../modules/document.md) **references** it rather than restating scope.
Because `person_persons` / `document_documents` have **no unit column**, they are **not** directly
under the `app.readable_units` RLS predicate — read scoping is this app-layer projection.
**D-RLSDefenseInDepth resolves their backstop treatment: a documented exemption** (the app-layer PDP
is authoritative; a person→unit reach-join is a noted hardening seam). Resolves the person/document
read-scope seam.

**The rule binds EVERY person-binding read, not only person and document (M58 ticket 7).** Any table
whose rows describe one named person — education enrollments, dorm stays, education appointments, the
education reference-layer bindings — is under this projection, and a reader of those rows must be
able to read the holder by the rule above. That was not a new decision in ticket 7; it was already
what this block says, and it had simply never been implemented outside `document`. Nine education
endpoints gated their module read code ANYWHERE and returned the rows, so one grant anywhere
enumerated any person's education history instance-wide, from M20 until the sweep. Two shapes satisfy
it and the choice is the endpoint's: a per-person read PROBES the holder and answers an unreadable
one with an **empty list, never a 403** (a permission error confirms the person exists), while a
top-level list FOLDS the projection into its SQL, because a Go-side trim after a keyset page is cut
returns a short page still carrying a next-page token (R-06).

### D-PersonBio — Person bio fields: structured names, birthdate, ISO 5218 sex

**Decision.** `person` gains **bio/identity** fields beyond the original three name columns. The
`display_name` stays the **canonical, authoritative** full name; a curated, world-spanning set of
**optional** structured name parts is added to **both** `person_persons` **and**
`person_name_variants` (a variant is a full transliterated name form): `given_name`, `family_name`,
`patronymic` (Slavic по-батькові / Icelandic), `middle_name`, `second_family_name` (Hispanic /
Lusophone), `name_prefix` (particle: `van`/`von`/`de`/`bin`), `name_suffix` (`Jr.`/`III`),
`preferred_name`, `honorific`. Two bio columns live on `person_persons` only: `birthdate DATE`
(nullable; a calendar date, **not** a `TIMESTAMPTZ`) and `sex TEXT NOT NULL DEFAULT 'not_known' CHECK
(sex IN ('not_known','male','female','not_applicable'))` — **biological sex per ISO/IEC 5218**, stored
as readable `TEXT` (not the numeric `0/1/2/9`). All new name parts, `birthdate`, and `sex` are
**`pii:basic`**. **Gender identity is out of scope** — it is `pii:special` (GDPR Art. 9) and must not
be stored until the envelope-encryption seam ships.

**Why.** The target domains (army with the seeded `ukr` locale, church, university) carry names from
many naming cultures: a personnel directory that cannot hold a **patronymic** is unusable in the
Ukrainian context, and double surnames / particles / generational suffixes are common worldwide.
`birthdate` and `sex` are baseline personnel-record attributes. Keeping `display_name` authoritative
(structured parts advisory) follows the W3C "personal names around the world" guidance — over-
structuring fails real names. ISO 5218 is the international standard for recording sex in records and,
unlike gender identity, is **not** Art. 9 data, so it needs no special envelope.

**Consequence.** New columns on [person](../modules/person.md) (`person_persons` +
`person_name_variants`), each `COMMENT ON COLUMN`-tiered `pii:basic` (D-PIITiers) and added to the
person **purge** erasure list. No new endpoint — `PUT /persons/{id}` and the name-variant upsert carry
the new fields. Parks **DS-38** (partial/approximate birthdate) and the gender-identity seam (tied to
DS-29). See [person](../modules/person.md) and [glossary.md](../glossary.md).

**Amended by D-PersonNamesCLDR** — the bespoke structured-name part set above
(`given_name`/`family_name`/`patronymic`/`middle_name`/`second_family_name`/`name_prefix`/
`name_suffix`/`preferred_name`/`honorific`) is **replaced by the Unicode CLDR Person Names fixed
field set**; in particular there is **no dedicated `patronymic` column** — the patronymic moves into
CLDR `given2`. `birthdate` + `sex` (ISO 5218) are unchanged.

**Amended (M12) — `date_of_death`.** `person_persons` also carries a nullable `date_of_death DATE`
(a calendar date like `birthdate`, **`pii:basic`**, on the purge erasure list). Death is a **bio
attribute, not a lifecycle state** — it does **not** transition `status` to `deactivated`/`purged` (a
deceased person remains an active directory record). Partial/approximate death dates share the **DS-38**
seam with `birthdate`. Lands additively in [M12](../milestones.md) (item F); no new endpoint
(`PUT /persons/{id}` carries it).

### D-Documents — A `document` module for person-held papers & personal codes

**Decision.** A new **`document`** module (`oikumenea.document_*`) owns the documents a person
**holds** — identity papers (passport, national ID, driver's licence, military ID) and government
personal codes (tax number, social-insurance number). A document is attached to **exactly one
person** and stores **metadata only** (number, issuer, validity dates) — never document binaries. The
document **kind** is an **instance-admin-managed catalog** (`document_document_types`: stable `code` +
translatable `name`, D-Code / D-i18n), seeded with a representative set and extensible by the admin.
The document `number`/`issuer` are **`pii:basic`** and the JSONB `attributes` is the `pii:special`
ceiling. Documents participate in the person **purge** by subscribing to the **`PersonPurged`** event
and erasing the person's document PII (row kept as a tombstone). A document confers **no** authority.

**Why.** Personnel records routinely attach identity documents and statutory personal numbers; the
user requires passports + identification/social-insurance codes. Making the *kind* a catalog (not a
code-defined enum) matches the rank/locale pattern and lets each deployment/country add its own
document kinds without a release. Highly identifying numbers (passport, РНОКПП) must be erasable, so
the module hooks the existing person-purge erasure path rather than inventing a second one.

**Consequence.** New module doc [document](../modules/document.md); new permissions `document.create`,
`document.read`, `document.update`, `document.delete` (document plane, scoped through the holder per
D-PersonReadScope + shadow gate) and instance-plane `document.type.manage` / `document.type.read`. Takes the
service from 9 to **11 modules** (with D-Orders). New auditable write paths (D-Audit). See
[document](../modules/document.md), [person](../modules/person.md) (purge consumer), and
[README](../README.md) / [glossary.md](../glossary.md) / [conventions.md](conventions.md).

### D-Orders — An `order` module: administrative acts as the legal basis for status changes

**Decision.** A new **`order`** module (`oikumenea.order_*`) owns **administrative orders** (наказ) —
the formal acts that are the **legal basis** for changes in a person's status (arrival, appointment,
leave, transfer, discipline, duty roster). An **order** has an issuing unit, number, date, a
`draft → issued → revoked` lifecycle (mutable while draft; **locked on issue** — corrections are
amending orders, undo is a revoking order; reversibility pattern, not the append-only guard), and
**≥1 order items**, each targeting one person (+ optional unit/position/rank per the type). The order
**type** is an **instance-admin-managed catalog** (`order_order_types`: stable `code` + translatable
`name`) carrying a **`category`** (the five Ukrainian-army "стройова частина" families:
`personnel-list`, `appointment`, `leave-travel`, `discipline-incentive`, `duty-roster`) and an
**`effect`** (`membership-start` | `membership-end` | `rank-change` | `record-only`). An order takes
effect on other modules **only via domain events + provenance links** (the locked
cross-module-mutation rule), never a synchronous cross-module write: structural items are realized as
[membership](../modules/membership.md)/[rank](../modules/rank.md) changes that cite the order item as
provenance (`membership_memberships.order_item_id`); `record-only` items (leave, trip, discipline,
duty) are authoritative as themselves. Structural changes are **auto-applied on issue** by
synchronous in-process subscribers in the issue transaction (**D-OrderApply** below). An order
confers **no** authority.

**Why.** In the target domain an order is *the* legal instrument — "the basis for any change in a
serviceman's status" — so the system must model orders as first-class records that personnel changes
reference, not as a side effect of editing a membership. The five-family category set is exactly the
Ukrainian-army order taxonomy the user specified. Many order families (leave, business trip,
discipline, duty) have **no** existing module to land in, so `record-only` items give them an
authoritative home now without prematurely building leave/discipline subsystems. Routing effects
through events + provenance (rather than synchronous writes) preserves the extraction-ready
event-driven mutation rule and keeps each module's invariants in its own write path.

**Consequence.** New module doc [order](../modules/order.md); new permissions `order.create`,
`order.read`, `order.issue`, `order.revoke` (unit-scoped on the issuing unit + shadow gate) and
instance-plane `order.type.manage` / `order.type.read`; new nullable provenance FK
`membership_memberships.order_item_id` ([membership](../modules/membership.md)). Order create/issue/
revoke and type edits are audited (D-Audit); issue/revoke are the headline legal-basis events.
**Resolved by D-OrderApply** (auto-apply on issue) — was **DS-34**. Parks **DS-35** (first-class
leave/absence), **DS-36** (discipline/ incentive records), **DS-37** (duty roster). With D-Documents
this brings the service to **11 modules**. See [order](../modules/order.md),
[membership](../modules/membership.md), [audit](../modules/audit.md), [README](../README.md),
[glossary.md](../glossary.md).

### D-OrderApply — Orders auto-apply their effects on issue (synchronous, in-process, same transaction)

**Decision.** Issuing an order (`POST /orders/{id}/issue`) **automatically performs** its structural
effects, **resolving DS-34** and replacing the prior D-Orders v1 default ("an admin applies the
change citing the order"). The mechanism, settled this session:

- **Trigger & atomicity.** On issue, in **one transaction**: write `order.status='issued'` + the
  `order.issue` audit row, then for **each order item** emit a **granular, effect-typed** domain
  event that the owning module's subscriber handles **synchronously, in that same transaction**. The
  order row and every effect share one fate.
- **Granular per-effect events** (order-emitted *intent*, `*Ordered`-suffixed to stay distinct from
  each module's existing *fact* events — no collision, no loop):
  - `membership-start` → **`AppointmentOrdered`** → [membership](../modules/membership.md) creates
    the membership (fills the position / plain belonging) citing `order_item_id`, then emits its own
    `MembershipCreated`.
  - `membership-end` → **`RemovalOrdered`** → [membership](../modules/membership.md) ends the target
    membership, then emits `MembershipEnded`.
  - `rank-change` → **`RankChangeOrdered`** → [person](../modules/person.md) sets `rank_id`, then
    emits `PersonRankChanged` (provenance in the audit payload — rank is a column, no FK).
  - `record-only` → **no event**; the order item is authoritative as itself.
- **Effects land at issue.** `effective_from`/`effective_to` on an item are **legal metadata only**,
  not a scheduler trigger; future-dated/scheduled application is a parked seam (needs DS-25).
- **All-or-nothing.** If any effect violates a target module's invariant (e.g. the one-holder index),
  the **whole issue rolls back**, the order stays `draft`, and the target module's domain error
  surfaces (e.g. `Membership:PositionAlreadyFilled`). Each module keeps enforcing its own invariants
  in its own write path.
- **Revoke does not cascade.** Revoking an issued order is a legal-status flip only; it does **not**
  auto-reverse applied effects. Undo is expressed by the **revoking order's own items** (the
  "corrections are amending orders" stance).
- **Audit attribution** reuses the established **event-subscriber** rule (D-Audit): a subscriber's
  cross-module write records as `actor_type='system', subsystem='event-subscriber'`, correlated to
  the human's `order.issue` row by the shared `request_id`. No new audit shape.

**Why.** The seam was parked on the assumption that auto-apply needs a background worker/broker
(DS-25/DS-26). Synchronous, in-process, **same-transaction** dispatch is just an in-process call
chain inside one transaction — it needs **neither** DS-25 **nor** DS-26, and it yields immediate
consistency (reads right after issue see the effects) while preserving the locked cross-module
mutation-via-events rule and each module's write-path invariants. All-or-nothing matches the
single-transaction model; no-cascade-on-revoke avoids clobbering changes that later orders made on
top of the same person/position.

**Consequence.** **Amends D-Orders** (auto-apply on issue is the shipped behavior, not "admin applies
citing the order"). **Resolves DS-34** (removed from open-questions). The in-process `pkg/events` bus
([platform](../modules/platform.md)) dispatches subscribers within the originating transaction; the
outbox/broker seam (DS-26) and worker runtime (DS-25) stay parked — only **future-dated scheduling**
of effects would need DS-25. [membership](../modules/membership.md) and [person](../modules/person.md)
gain order-event subscribers; subscriber writes audit as `event-subscriber` (D-Audit). See
[order](../modules/order.md), [membership](../modules/membership.md), [person](../modules/person.md),
[audit](../modules/audit.md).

### D-ClosureDriftHealth — Closure drift is surfaced via a diagnostic health reporter (no scheduled rebuild)

**Decision.** The last remaining parked half of closure integrity — a **periodic, background, full
closure rebuild** (DS-2) — is **ruled out of scope**, and closure-drift **detection** is instead
surfaced through a new **`closure-drift` health reporter**. Settled this session:

- **No in-app scheduler / worker for closure** (DS-25 stays parked). Incremental per-graph
  maintenance remains authoritative; **repair stays on-demand** (`POST /closure/rebuild`). A
  scheduled auto-rebuild is judged unnecessary for a small, rarely-re-orged org graph (the
  D-ClosureIntegrity rationale).
- **`POST /closure/verify` persists its result.** Besides returning the drift report, verify
  **upserts a per-graph diagnostic status row** (`tenant_closure_status`: `last_checked_at`,
  `missing_count`, `extra_count`, `in_drift`, optional `sample`). This is derived health metadata —
  **not** an audited domain mutation (consistent with "reads aren't audited"). `?graph=g` updates one
  graph; no param updates all.
- **A `closure-drift` witchcraft-go-health reporter** reads `tenant_closure_status` and reports, per
  graph: **ERROR** when `in_drift = true`; **WARNING** when a graph was never verified or is stale
  beyond a freshness window; **HEALTHY** otherwise. Aggregate state = worst per-graph state.
- **Operator-refresh only.** The reporter does **not** recompute on health scrapes — it reflects the
  last verify. Automatic *detection* therefore still relies on an **operator-side cron** calling
  `/closure/verify`; the reporter's value is **unified surfacing** (drift appears in `/status/health`,
  which operators already scrape — no bespoke alert wiring) plus a **staleness nudge** if the cron
  stops.
- **Diagnostic-only.** The reporter is wired into `GET /status/health` but **excluded from
  `/status/readiness` and `/status/liveness`** — a drifted closure must **not** pull the pod from
  rotation (the PDP keeps serving off the stored closure; all replicas share the DB, so
  readiness-gating would flap the whole service on a non-fatal integrity warning).
- The freshness window is a `pkg/refreshable` runtime tunable `closure_drift.max_age` (default ~26h
  for a daily cron; `0` disables the staleness check). The status lives in the **shared DB**, so a
  verify landing on any replica updates the status that **all** replicas' reporters read.

**Why.** Drift (a maintenance bug, a manual DB edit under L-OperatorDB, a partial failure) is a real
failure mode whose only prior surfacing was an operator parsing the `/closure/verify` HTTP response
and wiring their own alert. Routing the verify result into the standard health surface makes drift
visible through tooling operators already run, at zero new runtime cost — no scheduler, no worker
(DS-25), no recompute on the hot health path. Keeping it diagnostic-only respects that drift is an
**integrity** concern, not an **availability** one: the service still answers authorization decisions
from the stored closure, and pulling pods would convert a quiet warning into an outage.

**Consequence.** **Resolves DS-2** (removed from open-questions); **amends D-ClosureIntegrity**
(verify now also upserts the diagnostic status overlay). New derived table
`tenant_closure_status` ([tenant](../modules/tenant.md)); new `closure-drift` health reporter +
`closure_drift.max_age` runtime tunable ([platform](../modules/platform.md)); health reporters now
split into **readiness-gating** (DB reachability, schema-version) vs **diagnostic-only** (this one).
Needs **neither** DS-25 **nor** a worker runtime. See [tenant](../modules/tenant.md),
[platform](../modules/platform.md), [conventions.md](conventions.md), [glossary.md](../glossary.md).

### D-PersonNamesCLDR — Names follow the Unicode CLDR Person Names fixed field set (amends D-PersonBio)

**Decision.** The bespoke structured-name part set introduced by **D-PersonBio** is **replaced by the
Unicode CLDR Person Names fixed field set**, on **both** `person_persons` and `person_name_variants`.
The fixed fields (all optional, all `pii:basic`): `title`, `given`, `given2`, `surname`,
`surname_prefix`, `surname2`, `generation`, `credentials`, `preferred`. `display_name` remains the
**canonical, authoritative** full form (the structured parts stay advisory). `birthdate DATE` + `sex`
(ISO 5218), on `person_persons` only, are **unchanged**.

**Pure CLDR (no dedicated patronymic).** There is **no `patronymic` column** — the Slavic
по-батькові / отчество (and the Icelandic patronymic) lives in **`given2`** by locale convention.
Formal Slavic address ("Тарас Григорович") is therefore **assembled by locale-aware formatting**
from `given` + `given2`, not read from a typed patronymic field. The old→new field map (for the
expand/contract migration): `given_name`→`given`, `family_name`→`surname`, `patronymic`→`given2`,
`middle_name`→`given2` (a person has at most one of patronymic/middle in practice; if both are
present the migration concatenates per locale convention and preserves the authoritative
`display_name`), `second_family_name`→`surname2`, `name_prefix`→`surname_prefix`,
`name_suffix`→`generation`, `honorific`→`title`, `preferred_name`→`preferred`. The world's long tail
(Arabic nasab chains, four-plus surnames, clan/tribal names) is **not** modeled as typed fields — it
is carried by the authoritative `display_name` (and, where a Latin form is wanted, a per-locale
`person_name_variants` row).

**Why.** CLDR Person Names is the cross-industry standard (the data the operating systems and
browsers format names with), curated by the same body behind the locale data this service already
uses for ISO 639-3 (D-i18n). Aligning to it makes the field set principled and interoperable rather
than a hand-rolled superset, and gives a documented formatting model per locale instead of ad-hoc
rendering. The user chose **pure CLDR** deliberately: the standard deals with the patronymic via
`given2` rather than a dedicated slot, and matching the standard exactly is worth more here than
preserving a typed patronymic field — the formal-address case is recovered by locale-aware
formatting. `display_name`-stays-authoritative continues to follow the W3C "personal names around the
world" guidance (over-structuring fails real names).

**Consequence.** **Amends D-PersonBio** (replaces its name-part columns; `birthdate`/`sex`
untouched). The CLDR columns are tiered `pii:basic` (D-PIITiers) and are on the person **purge**
erasure list. Migration is **expand/contract** (add CLDR columns, backfill per the map above, then
contract the old columns in a later announced release — L-UpgradeSafe + the `atlas migrate lint`
destructive gate). No new endpoint — `PUT /persons/{id}` and the name-variant upsert carry the CLDR
fields. The `given2`-holds-patronymic convention is documented in
[person](../modules/person.md), [conventions.md](conventions.md), and [glossary.md](../glossary.md).

### D-Geo — Seeded ISO-3166 country registry; citizenship, birth, and residence as first-class person data

**Decision.** Geography becomes first-class. A new **seeded country registry** `geo_countries`
(a shared reference table, owned/seeded by [platform](../modules/platform.md) like `uuid_v7()` and
the other shared objects — **not** a standalone domain module): `code CHAR(2)` PK (ISO-3166-1
alpha-2), translatable `name` (default-locale fallback + the i18n store, new `entity_type='country'`),
`status` (`active`/`retired`), `sort_order`, timestamps. Instance-admin-extensible (historical or
partially-recognized entities). All columns `pii:none`. It is referenced (FK) everywhere a country
appears.

On [person](../modules/person.md):
- `person_persons.country_of_birth CHAR(2) REFERENCES geo_countries(code)` — nullable, `pii:basic`.
- New **`person_citizenships`** (effective-dated): `person_id`, `country` → `geo_countries`,
  `acquired_on DATE?`, `lost_on DATE?`, `basis TEXT` (`birth`/`descent`/`naturalization`/`other`,
  `TEXT`+`CHECK`), `is_primary BOOLEAN`. A person may hold **several** citizenships; uniqueness is one
  **active** row per `(person, country)`; `pii:basic`. This is the model's answer to multiple
  citizenship.
- New **`person_residences`** (effective-dated): `person_id`, `country` → `geo_countries`,
  `region TEXT?`, `from DATE`, `to DATE?`; `pii:contact` (locator data).

**Why.** A universal personnel directory must represent people who were born in one country, hold
several citizenships, and reside in another — the army/church/university target domains all carry
multinational personnel. Modelling country as a **seeded registry with translatable names** (rather
than free-text or a compiled CHECK list) matches the locale/graph-registry pattern, lets the i18n
store localize country names, and lets an operator add edge-case entities without a code change.
Effective-dating citizenships and residences (rather than a single column) captures naturalization,
renunciation, and relocation as history — the same temporal discipline membership uses.

**Consequence.** New shared table `geo_countries` (platform-seeded); new tables `person_citizenships`,
`person_residences`, and column `person_persons.country_of_birth`
([person](../modules/person.md)). Country names join the [localization](../modules/localization.md)
translation store (`entity_type='country'`). New person sub-resource endpoints
(`/persons/{id}/citizenships`, `/persons/{id}/residences`) and a country read
(`GET /countries`) + instance-scope `country.manage`. Citizenship/residence writes are audited
(D-Audit) and erased on person **purge**. Module count is **unchanged** (geo is platform-owned
reference data, not a module). See [person](../modules/person.md),
[platform](../modules/platform.md), [localization](../modules/localization.md).

### D-PersonalCodes — National identifiers as a per-scheme catalog with validation (extends D-Documents)

**Decision.** Government **personal codes / national identifiers** get a dedicated model in the
[document](../modules/document.md) module, **split** from the generic document-type catalog (which
keeps modelling papers — passport, driver-license, military-id — unchanged):

- **Scheme catalog** `document_personal_code_schemes` (instance-admin-managed, D-Code/D-i18n) —
  **country-namespaced per scheme**, enriched with semantic metadata:
  - `code TEXT` PK — the scheme id, e.g. `ua-rnokpp`, `us-ssn`, `de-steuer-id`, `it-codice-fiscale`
  - `country_iso CHAR(2) REFERENCES geo_countries(code)` — the scheme's country (NOT NULL for
    national schemes)
  - `generic_category TEXT NOT NULL` — semantic grouping (`tax-id`, `national-id`,
    `social-insurance`, `health-insurance`, `residence-permit`, …): the **join key** for
    cross-scheme queries ("list everyone's tax IDs")
  - `validation_regex TEXT?` — optional data-side fallback regex (see validation below)
  - translatable `name`, `status` (`active`/`retired`), `sort_order`, timestamps
- **Data rows** `document_personal_codes` (lean): `person_id`, `scheme_id`, the identifier `value`
  (**`pii:sensitive`**, envelope-encrypted at rest — D-CryptoProvider), lifecycle timestamps.
  **Country derives from the scheme** (`scheme.country_iso`) — no per-row country on a personal code.
  Cross-person uniqueness is enforced on a **blind index** of the normalized value
  (`UNIQUE (scheme_id, value_blind_index) WHERE deleted_at IS NULL`), since the value itself is
  ciphertext.
- **Validation** is two-layer, **code-authoritative**: a compiled `pkg/personalcode` validator
  registry keyed on the scheme (e.g. UA-RNOKPP checksum, IT codice fiscale, US-SSN format) is the
  authority; the catalog's optional `validation_regex` is a **fallback** for schemes without a
  compiled validator; an unknown scheme with neither **accepts with a warning**. Precedence:
  **code validator > catalog regex > accept-and-warn**.

**Why.** National identifiers differ from ordinary papers: they are highly identifying, frequently
have **checksums/format rules**, are issued **per country** (so multi-citizenship means several), and
operators routinely query them by *kind* ("all tax IDs"). A country-namespaced scheme catalog
enriched with a `generic_category` gives both **per-scheme precision** (look up `ua-rnokpp` →
its exact checksum + country) and **cross-scheme queryability** (filter `generic_category='tax-id'`),
while keeping the data rows lean. Deriving the country from the scheme avoids a redundant,
drift-prone per-row country. Code-authoritative validation matches the existing "policy-as-data,
enforcement-as-code" stance (real checksums can't be expressed as operator regex), while the catalog
regex keeps unknown national schemes usable before a validator is compiled. **Extends D-Documents**
rather than replacing it: papers stay in the generic type catalog.

**Consequence.** New tables `document_personal_code_schemes` + `document_personal_codes`
([document](../modules/document.md)); the value is `pii:sensitive` and **envelope-encrypted**
(D-CryptoProvider) with a blind index for uniqueness/lookup; new `pkg/personalcode` validator
registry ([platform](../modules/platform.md)); new permissions `personal-code.create/read/update/
delete` (scoped through the holder per D-PersonReadScope) + instance-plane
`personal-code-scheme.manage`/`.read`; scheme names join the i18n store. Personal codes are erased on
person **purge** by **crypto-erase** (drop the wrapped DEK). All writes audited (D-Audit). See
[document](../modules/document.md), [person](../modules/person.md),
[platform](../modules/platform.md).

**Scheme set (expanded).** Beyond the originally seeded schemes (`ua-rnokpp`, `ua-unzr`, `us-ssn`,
`de-steuer-id`, `it-codice-fiscale`, `pl-pesel`), the catalog seeds RU/BY/LATAM identifiers:
`ru-inn`, `ru-snils`, `by-personal-number`, `br-cpf`, `ar-dni`, `ar-cuil`, `mx-curp`, `mx-rfc`,
`cl-rut`, `co-cedula`. Compiled `pkg/personalcode` **checksum** validators ship for the schemes with a
well-known algorithm (`ru-inn`, `ru-snils`, `br-cpf`, `ar-cuil`, `cl-rut`); **structural/format**
validators ship for `mx-curp` / `mx-rfc` (their homoclave check character is name-derived and not
verifiable from the code alone); `ar-dni`, `co-cedula`, `by-personal-number` rely on the catalog regex
/ accept-and-warn fallback. All
`country_iso` values are already in the seeded `geo_countries` registry. Purely additive — new seed
rows + new compiled validators, no schema or decision change.

### D-CryptoProvider — Pluggable envelope encryption for sensitive PII (reshapes DS-29)

**Decision.** Introduce **envelope encryption** behind a **pluggable key-provider seam**, used now to
protect `pii:sensitive` national-identifier values at rest:

- **Envelope model.** The protected value is stored as **ciphertext in Postgres**. A per-record
  **data-encryption key (DEK)** encrypts the value; the DEK is **wrapped by a key-encryption key
  (KEK) that never leaves an external KMS**. Each protected row carries `value_ciphertext`,
  `wrapped_dek`, `key_ref` (KEK id + version), and a keyed-HMAC **`value_blind_index`** for
  equality lookup / uniqueness without decryption. The KMS is on the **unwrap** path only and
  unwrapped DEKs are cacheable.
- **Pluggable `KeyProvider`.** A platform seam `KeyProvider { Wrap(dek) / Unwrap(wrapped) /
  KeyRef() }` with swappable backends — **`aws-kms`, `gcp-kms`, `vault-transit`, `azure-kv`,
  `local-dev`** — selected by **install config** (`var/conf/install.yml`). The model (ciphertext in
  DB) is fixed; the vendor is configuration. No vendor lock-in; self-hostable (Vault / local-dev).
- **Crypto-erase.** Erasure (person purge) destroys the wrapped DEK (and may nullify ciphertext), so
  the value is unrecoverable without re-keying — the erasure mechanism for `pii:sensitive`.
- **Scope (now): `pii:sensitive` only.** Only national-identifier values
  (`document_personal_codes.value`) are envelope-encrypted today. `pii:basic` data (names, birthdate,
  ordinary document numbers/issuer) stays plaintext under the existing "minimized, redacted logs"
  discipline. Extending envelope crypto to `pii:special` (Art. 9) person fields and to audit
  `before`/`after` payloads remains **parked under DS-29** (and is what gates the gender-identity
  seam, DS-38).

**Why.** National identifiers warrant encryption at rest, but a directory must still **find and
dedupe** them — envelope encryption with a blind index gives both (encrypted values, indexed
lookup), the standard pattern for queryable sensitive data. The user's requirement was explicitly a
**generic** secrets/KMS integration (AWS KMS, HashiCorp Vault, GCP KMS, others), so the key backend
is abstracted behind one interface and chosen per deployment — portable and self-hostable, in
keeping with L-OperatorDB. Scoping to `pii:sensitive` first keeps the blast radius small and ships a
working national-ID feature without waiting on the broader Art. 9 envelope.

**Consequence.** **Adds the `pii:sensitive` tier** to D-PIITiers and the "`pii:sensitive` ⇒
envelope-at-rest" rule. New `KeyProvider` seam + `pkg/crypto` (wrap/unwrap, blind-index HMAC, DEK
cache) and KMS backend install config ([platform](../modules/platform.md)). **Reshapes DS-29**: the
personal-code envelope mechanism ships now; DS-29 narrows to extending envelope crypto to audit
payloads + `pii:special` person fields. The app DB never holds the KEK; the operator owns the KMS.
See [document](../modules/document.md), [platform](../modules/platform.md),
[conventions.md](conventions.md), [open-questions](../open-questions.md) (DS-29).

**Amended (review R-22, 2026-07-11): key rotation is a first-class, executable operation.** The
`key_ref` persisted with every wrapped DEK was plumbing with no actuator — no way to rotate a KEK, so
a suspected KEK leak had no remediation and PCI-DSS Req 3.6 (mandatory for the retained PANs under
[D-DataScope](#d-datascope--what-a-deployment-may-hold-the-product-is-a-personnel-directory--registry-platform-owns-the-pci-dss-posture))
was unmet. Closed with two pieces, no schema change:
- **Rotation-capable provider.** A `KeyProvider` may hold an **active** KEK (used for `Wrap` +
  `KeyRef`) plus zero or more **previous** KEKs used for `Unwrap` only; `Unwrap` tries active-then-
  previous and relies on AES-GCM's auth tag to reject the wrong key (so the wrapped DEK's `key_ref`
  need not be consulted to pick the KEK, and the `KeyProvider` interface is unchanged). The `local-dev`
  backend reads previous KEKs from `crypto.local-dev.previous-keks`. A real KMS backend (aws-kms/vault/…)
  slots behind the same seam later; config is already file-based, so the multi-KEK `local-dev` provider
  is sufficient to rotate today.
- **`oikumenea rewrap` maintenance CLI** (operator-host-gated, sibling of `seed`/`recover-admin`).
  Walks a **code-defined registry of every envelope table** (guarded complete by
  `TestRewrapTablesMatchSchema`) and re-wraps each DEK under the active KEK — **payload ciphertext
  untouched**, only `wrapped_dek` + `key_ref` change. Batched per transaction and **resumable/idempotent**
  by construction (each pass only selects rows whose `key_ref` isn't yet the active one, so a re-run or
  a `kill -9` resumes). `--dry-run` prints the per-`key_ref` census; **`--reindex-blind-index`** runs a
  second pass that decrypts each value and recomputes its `*_blind_index` under the active blind-index
  key (the heavier blind-index rotation — safe because every module seals and blind-indexes the *same*
  normalized bytes, so reindex is `Open → BlindIndex`). **Rotation runbook:** promote a new active
  `kek`, move the old into `previous-keks`, deploy, run `oikumenea rewrap` (add `--reindex-blind-index`
  when rotating the blind-index key), then drop the old KEK from config. See
  [upgrade-safety.md](upgrade-safety.md).

---

### D-ResourceIdentifiers — Packed UUIDv8 RIDs as primary keys (Objects, Links, Actions)

**Amended (F-014).** The composed-URN RID (`urn:oikumenea:<service>:<environment>:<entity_type>:<uuid>`
TEXT PK) was replaced by a **packed native UUIDv8** that carries the same self-describing payload —
*app, service, kind, type* — in 16 bytes instead of ~70. The URN form widened every index/FK join to
encode `<service>`/`<environment>` segments that are **constant per database** (L-SingleDomain), and
forced the GUC workaround D-RIDSeeding existed only to paper over. The decomposable string survives as
a **boundary representation rendered from the bytes** (`pkg/rid`), never stored. (The historical URN
rationale is preserved at the end of this block.)

**Decision.** Every Object, Link, and Action is identified by a **native `uuid` primary key** that is a
**UUIDv8 (RFC 9562 §5.8)** packing a decomposable, self-describing key. The byte layout (0-indexed,
big-endian) — emitted by `oikumenea.new_id()` and read by the `rid_*` decoders:

| Bytes | Field | Meaning |
|-------|-------|---------|
| 0–5   | `timestamp` (48b) | unix-ms — preserves b-tree insert locality (like uuid_v7) |
| 6     | `version`=8 (hi nibble) · `kind` (lo nibble) | kind: 1=object, 2=link, 3=action |
| 7     | `app` (8b) | `oikumenea` = 1 |
| 8     | `variant`=0b10 (hi 2b) · `service` (lo 6b) | service code 0–63 |
| 9–10  | `type` (12b: byte 9 + hi nibble of byte 10) | **per-service** type code |
| 10–15 | `random` (~46b) | unguessable crypto component |

- `service` + `type` are **numeric codes** held in the seeded `platform_rid_services` /
  `platform_rid_types` registries (migration 0000), mirrored in **`pkg/rid`** with a boot-time equality
  assertion (`rid.AssertMatches`). Type codes are **per-module** (each service owns `0..4095`
  independently — a new module claims a service code and numbers its own types). The authoritative
  *list* of types is [ontology-mapping.md](../ontology-mapping.md); the codes are assigned there + the
  migration + `pkg/rid` together. Actions use a generic type code `0` (the specific action name lives
  in `audit_log.action`, so the RID only encodes kind=action).
- PKs are `id uuid PRIMARY KEY DEFAULT oikumenea.new_id(<service>,<kind>,<type>)`; FKs follow as `uuid`.
  A per-table shape `CHECK` (`rid_service(id)=<svc> AND rid_kind(id)=<kind> AND rid_type(id)=<code>`) is
  the structural guard.
- **Boundary representation.** The Go layer carries RIDs as their **canonical uuid text** (pgx
  scans/encodes `uuid`↔`string` and `uuid`↔`pgtype.Text` natively — the sqlc `uuid`→`string` override
  keeps every repo type identical to the prior text era). `pkg/rid` renders the human form
  `oikumenea:<service>:<kind>:<type>:<uuid>` and decodes any RID's fields; the API and web console
  consume the uuid (the web decodes the bytes for type routing). Nothing stores the rendered string.
- **Temporal Links** never encode validity in the RID (immutable); validity lives in
  `effective_from`/`effective_to`, `granted_at`/`revoked_at`(+`expires_at`).
- **Action RID = the natural key of the `audit_log` row** that records it (D-Audit).
- **Polymorphic id columns stay `TEXT`.** `audit_log.target_id` and `i18n_translations.entity_id`
  reference *either* a RID uuid *or* a natural-key code (locale, scheme), so they are `text`
  (a RID's canonical uuid text or a bare code), not `uuid`. `actor_person_id`/`unit_id` (always RIDs)
  are `uuid`.
- **Scope.** Tables keyed by `id` adopt the uuid RID. Pre-existing **natural-key catalog tables**
  (`document_personal_code_schemes.code`, `i18n_locales.code`, `person_*_types.code`,
  `person_platforms.code`, `person_relation_types.code`, `rank_grades.code`) and **composite-key**
  join/closure tables keep their keys (D-PersonalCodes / D-Code unchanged). *(Amended M16: the geo
  registry left this carve-out — `geo_countries` and `geo_places` are now RID-keyed under the new
  `location` service (code 12), with ISO `code` / `wof_id` retained as `UNIQUE` lookup/concordance
  keys and every country FK repointed to `geo_countries(id)`; see roadmap-decisions D-Geo / D-GeoPlaces.)*

**Why.** A self-hosted instance is one app, one environment, one database (L-SingleDomain), so the
URN's `<service>`/`<environment>` segments encoded values that never vary across the rows of a DB —
~70-byte TEXT keys paying width on every index page, join, and reference seed to say what the table
already implies. The UUIDv8 keeps the *decomposability* (app/service/kind/type all recoverable from the
bytes, by SQL and by `pkg/rid`) and the time-ordered b-tree locality, at native 16-byte width and with
no GUC dependency — so reference rows seed in migrations again (D-RIDSeeding relaxed). It is **not** a
Palantir-style cross-service URN because nothing here resolves a reference across a service boundary
without a shared database.

**Consequence. Restores L-Conventions' `uuid` PK** (now UUIDv8, time-ordered) — `uuid_v7()` is retained
as a helper but PKs default via `new_id()`. PKs/FKs are 16 B again. The **D-RLSDefenseInDepth** GUC
arrays return to `uuid[]` (`app.readable_units`/`writable_units`; policy casts
`string_to_array(nullif(current_setting(…),''),',')::uuid[]`). The `app.environment` GUC is no longer
read by any SQL (vestigial). Coexists with **D-Code**: the RID is the *machine resource handle*; `code`
stays the stable, locale-agnostic *business* key. See [conventions.md](conventions.md) (Resource
identifiers) and [ontology-mapping.md](../ontology-mapping.md).

**Historical note (superseded URN form).** The original scheme keyed every entity by a composed URN
`urn:oikumenea:<service>:<environment>:<entity_type>:<uuid>` TEXT PK via `new_rid()`, with a
`LIKE 'urn:oikumenea:…'` shape CHECK, modeled on Palantir resource identifiers. It is retained here as
provenance only; the packed UUIDv8 above is binding.

---

### D-RIDSeeding — RID-keyed seed rows MAY seed in migrations (boot-seeding retained by choice)

**Relaxed (F-014).** The hard constraint that forced this decision is **gone**: `new_id()`
(D-ResourceIdentifiers) reads **no GUC** — `app`/`service`/`type` are caller-supplied codes, not a
per-connection `app.environment`. So a migration that inserts a defaulted-RID row no longer fails, and
reference rows whose PK is a RID **may be seeded directly in the Atlas migration** that creates their
table (table-create and seed re-colocate). The original blocker — `new_rid()` needing the
`app.environment` GUC that Atlas's connection does not set — no longer exists.

**Decision (now).** RID-keyed reference rows may seed in migrations. **Boot-time idempotent seeding is
retained where the seed is code-derived**, not because of a GUC: the M7 **base roles** (D-BaseRoles)
map to the code-defined permission catalog (`domain/permissions.go`), and the M8 **first-admin**
bootstrap (D-Bootstrap) depends on install-config identity — both legitimately run in module
`Register(...)` / the bootstrap path on the app pool, via `INSERT … ON CONFLICT (<natural-code>) … DO
NOTHING`, idempotent on every boot. Migrations and boot seeds are interchangeable for RID rows now; the
choice is driven by whether the seed data lives in SQL (migration) or in Go (boot).

**Why.** The packed UUIDv8 carries no environment segment, so there is nothing only the application can
know at mint time — Atlas can mint correct RIDs during a migration. The remaining boot seeds stay at
boot because their *content* (permission sets, install identity) originates in code/config, which a
static migration cannot reproduce, not because of any RID limitation.

**Consequence.** No `unrecognized configuration parameter "app.environment"` failure mode; the GUC is
vestigial (still set by `db.NewPool` `AfterConnect`, read by nothing). The remaining boot seeds (base
roles, document/order type catalogs, first-admin) continue to work unchanged — they rely on the column
`DEFAULT new_id(...)`, which needs no GUC. The **tenant domain + unit-kind reference catalogs** moved
from boot (`tenant.Register`) **into migration `0003_tenant`** (M41) — their content is static SQL, so
the migration is the natural home; this also lets the migration ship the full universal `military`
echelon ladder. Per-org `command`/`operational` graphs are not seeded at all — they are created with each
organization (`CreateOrganization`). See [conventions.md](conventions.md) (Resource identifiers),
[tenant](../modules/tenant.md), and [platform](../modules/platform.md).

---

### D-Ontology — Object / Link / Action is the binding domain model

**Decision.** The domain is modeled as a Palantir-style **ontology**, and that modeling is
**binding**, not an after-the-fact lens. Every persisted entity is exactly one of:

- an **Object** — a thing with identity over time (`Unit`, `Person`, `Position`, `Order`, `Role`, …);
- a **Link** — a relationship that is **reified** (its own row + RID) when it carries identity,
  attributes, or history (`HAS_ROLE`/`Assignment`, `MEMBER_OF`, `PARENT_OF`, `HOLDS_RANK`, …); a
  relationship carrying none of those stays a plain FK column;
- an **Action** — a named, audited mutation (`IssueOrder`, `GrantAssignment`, `CreateUnit`, …),
  recorded in the [audit](../modules/audit.md) ledger.

Every module doc **must** classify each of its entities by kind and key it with the matching RID slot
(`<object>` / `link__<type>` / `action__<type>`; D-ResourceIdentifiers).

**Source-of-truth split (avoids a dual master).**
- [ontology-mapping.md](../ontology-mapping.md) is the binding **catalog** — the authoritative list of
  Object/Link/Action **types** and their kind, one row each, citing the owning module.
- The `modules/*.md` own the **detail** — columns, RID shape, lifecycle, invariants, endpoints.
- On any genuine conflict **this file wins** (unchanged precedence); record the conflict in
  [open-questions.md](../open-questions.md).

**Ratified divergences from the textbook ontology** (intentional, decision-backed — see
[ontology-mapping.md §4](../ontology-mapping.md#4-ratified-divergences-from-the-ontology-ideal)):
soft-delete + provenance instead of full bitemporal Link validity (4.1); provenance carried
non-uniformly (order refs + audit) rather than a uniform `source` column (4.4); lifecycle `status`
columns as the cleaner terminal state over `deleted_at` (4.5); public/shadow **visibility** as a
read-time gate, not a stored relationship (4.6); **permissions are code, not Objects** (4.7).

**Why.** The Palantir stack the service showcases is ontology-shaped; making Object/Link/Action the
binding vocabulary (not just a lens) keeps the module docs uniform, makes Links and Actions
first-class addressable resources, and lets the audit log be the action ledger. Pairs with
**D-ResourceIdentifiers** (the RID encodes the kind), **D-Audit** (action RID = audit key), and
**D-Code** (RID = machine handle, `code` = business key).

**Consequence.** [ontology-mapping.md](../ontology-mapping.md) is **promoted from analysis to binding
registry**; the module docs gain explicit Object/Link/Action labeling and RID-shaped data models (a
doc-only pass — no schema change beyond what D-ResourceIdentifiers already set). New entities must
declare their kind at design time.

### D-PersonContactChannels — Emails, phones, and call signs as effective-dated person child tables (extends D-Geo)

**Decision.** A person gains three additional **multi-valued contact/identity channels**, each modeled
as a [person](../modules/person.md) child table that follows the existing `person_citizenships` /
`person_residences` pattern (RID PK, `person_id` FK `ON DELETE CASCADE`, soft-delete, `is_primary`,
`set_updated_at`, all writes audited, erased on purge):

- **`person_emails`** — `address` (`citext`, `pii:contact`); a derived **`provider`** column
  (`pii:contact`) populated on write from a static domain→provider map (e.g. `gmail.com → google`);
  `type_code` FK to a new **`person_email_types`** catalog. Uniqueness: one **active** row per
  `(person, lower(address))`. The contact email is **distinct from the login email**
  (`account_accounts.email`, [identity-federation](../modules/identity-federation.md)) — no FK between
  them; they are independent concerns.
- **`person_phones`** — `number` stored **E.164-normalized** (`pii:contact`) and a derived
  **`country`** FK to `geo_countries` (`pii:contact`), both computed via a libphonenumber-class parser
  (`github.com/nyaruka/phonenumbers`); `type_code` FK to a new **`person_phone_types`** catalog.
  Uniqueness: one **active** row per `(person, number)`. **Carrier/provider lookup is out of scope**
  (not statically derivable — number portability needs an external HLR service) → parked as **DS-40**.
- **`person_call_signs`** — `call_sign` (`TEXT`, **NOT NULL**, `pii:basic`); **unique per person**
  among active rows (`UNIQUE (person_id, call_sign) WHERE deleted_at IS NULL`); `is_primary` marks at
  most one active.

The two **type catalogs** (`person_email_types`, `person_phone_types`) follow D-Code/D-i18n: natural
`code` PK, translatable `name` (default-locale fallback + the i18n store, new
`entity_type='email_type'`/`'phone_type'`), `status`, `sort_order`. Seeded: email
`personal`/`work`/`other`; phone `mobile`/`home`/`work`/`other`.

**Why.** A universal personnel directory must carry reachable contact data (people hold several emails
and phones) and, for the military target domain, **call signs (позивний)**. Modelling each as an
effective child table reuses the proven citizenship/residence slice rather than inventing a new shape;
catalog-typed kinds keep the vocabulary operator-managed and localizable instead of a compiled CHECK
list. Storing phones in E.164 with a derived country gives stable equality/dedup and lets the country
join `geo_countries`; deriving the email provider on write supports "who uses provider X" queries
without a separate lookup. Keeping the contact email distinct from the login email avoids conflating
directory data with the federated-identity seam.

**Consequence.** New tables `person_email_types`, `person_phone_types`, `person_emails`,
`person_phones`, `person_call_signs` ([person](../modules/person.md)); email/phone type names join the
[localization](../modules/localization.md) store (`entity_type='email_type'`/`'phone_type'`). New
person sub-resource endpoints (`/persons/{id}/emails`, `/phones`, `/call-signs`) + catalog reads
(`GET /person/email-types`, `/phone-types`), gated by `person.read`/`person.update` (scoped through the
holder per D-PersonReadScope). New dependency `github.com/nyaruka/phonenumbers`. All writes audited
(D-Audit); all three channels **erased on person purge** (the purge erasure list extends to their
`pii:contact`/`pii:basic` columns + `DeleteAll*` of the child rows). Carrier lookup parked as DS-40.
Module count unchanged. See [person](../modules/person.md), [localization](../modules/localization.md).

### D-DocumentAttrSchema — Per-document-type attribute schema with write-time validation (extends D-Documents)

**Decision.** The generic `document_documents.attributes` JSONB grab-bag gains **optional per-type
structure**. `document_document_types` gains a nullable **`attr_schema` JSONB** column declaring the
attribute fields a document of that type may/must carry:

```
{ "fields": { "<name>": { "type": "string|number|boolean|date", "required": <bool>,
                          "enum": [ ... ]? } } }
```

When a type's `attr_schema` is non-null, a document's `attributes` is **validated against it on every
create/update** (unknown keys rejected, required keys enforced, declared types/enums checked) by a
minimal field-spec validator in the [document](../modules/document.md) domain (standard-library only,
in the spirit of `pkg/personalcode`). When `attr_schema` is null, `attributes` is free-form as today.
The seeded `military-id` type ships a schema (e.g. VOS/specialty code, fitness category, mobilization
category, issuing commissariat).

**Why.** Military cards and similar papers carry well-known structured fields that operators want
validated, but promoting them to typed columns would fork the schema per country/type. A per-type
attribute schema keeps one generic `document_documents` table while giving typed, validated fields
where a type declares them — the investigate-then-decide military-doc item resolves to **generic
typed-attributes**, not country-specific columns. Reusing the existing `attributes` JSONB means no new
data table and no migration churn as schemas evolve.

**Why not typed columns / external JSON-Schema.** Typed columns fork the table per type and are hard
to expand; a full JSON-Schema engine is heavier than needed and pulls a dependency. A minimal,
code-owned field-spec validator matches the project's "enforcement-as-code" stance.

**Consequence.** New nullable column `document_document_types.attr_schema` (expand-only); a field-spec
validator + `ErrDocumentInvalid` mapping in [document](../modules/document.md); the `military-id`
seed gains a schema. `attributes` stays at the `pii:special` **ceiling** (D-PIITiers) — no
special-category data lands there without the envelope seam (DS-29); a typed field that is genuinely
`pii:special` still waits on DS-29. No new endpoint (document create/update already carry
`attributes`). See [document](../modules/document.md).

---

### D-WebUI — An optional standalone Next.js admin UI (reverses the "API-only, no UI" drop)

> **Amended by [D-ConsoleDashboards](#d-consoledashboards--every-listable-type-gets-a-list-view-and-a-dashboard-view-over-one-url-borne-filter-set-amends-d-webui) (M56–M57).**
> The "no broader component-library lock" stance below stands, but the count of small, focused
> libraries goes from **two** (`cmdk`, `@xyflow/react`) to **three**: `@visx/*` joins them for chart
> scales and shapes, with the chart markup still hand-rolled on Tailwind. Every listable type also
> gains a second view (`?view=dashboard`) over one URL-borne filter set. Nothing else here changes —
> the console remains an optional, unprivileged BFF consumer of the public API that makes no
> authorization decision of its own.

**Decision.** Ship an **optional, separately-run web admin console** as a standalone **Next.js**
application living in `web/`, served on **port 8445**. It is a **consumer** of the existing public
HTTP API, not a backend module: it adds **no Go code, no Conjure contract, no schema change, and no
new port to the `oikumenea` binary**. It is built on the **Backend-for-Frontend (BFF)** pattern —
the Next.js server holds an httpOnly session and proxies API calls with the bearer token attached;
the browser never sees a token and never calls the Go API directly. Authentication is the standard
**Keycloak OIDC Authorization-Code flow** (a confidential `oikumenea-web` client whose access token
carries `aud: oikumenea`), so [L-AuthzOnly](#carried-over-locks-settled-earlier-restated-for-self-containment)
holds unchanged — the UI authenticates *at the IdP* and the service still validates inbound tokens
and decides authorization. The UI is **opt-in to run** (a `ui`-profiled docker-compose service / a
`web/` dev server); a default deployment is unaffected. This **supersedes the earlier "the Next.js UI
(API-only)" drop**.

**Why.** Operators of a hierarchical org (army/church/university) need a human-usable surface for
directory and authorization administration; curl + the OpenAPI reference is not it. A standalone BFF
keeps the service **API-first** (the UI cannot do anything the API doesn't expose) while giving a
secure, idiomatic experience: the access/refresh token stays server-side, there is **no CORS surface
on the Go app**, and the UI stays an independently-versioned, independently-deployed, **removable**
artifact — extraction-friendly, exactly like the modular monolith's stance on the backend. Because
TypeScript types are **generated from `docs/api/openapi/openapi.json`**, the UI cannot drift from the
contract.

**Why not** (a) embed a static export served by the Go binary on a second listener: loses server-side
sessions/SSR and forces a browser-held token (weaker posture), and couples UI release to the Go
binary; (b) keep API-only: leaves operators without a usable console. The original drop was about
*not coupling* a UI into the core — a standalone, optional BFF honours that intent while still
delivering the UI.

**Consequence (binding rules the UI must honour).**
- **No client-side authorization.** The UI never decides access; it asks the PDP
  (`POST /authorization/v1/authorize`, or `/authorize/batch` with `explain` where the caller holds
  `assignment.read`) and renders accordingly. It never branches on rank/position
  ([D-Rank](#d-rank--rank-on-person-rank--permission)).
- **Shadow visibility is server-enforced.** The UI renders exactly what the API returns and never
  does its own visibility filtering ([L-Visibility](#carried-over-locks-settled-earlier-restated-for-self-containment),
  [D-PersonReadScope](#d-personreadscope--a-persons-read-scope-projects-through-its-memberships)).
- **All translations in every response** ([D-i18n](#d-i18n--i18n-is-required-all-translations-in-every-response)).
  Translatable labels arrive as `locale → text` maps; the UI picks per a UI-locale switch with
  fallback and writes the full map back. Person names use the per-person transliteration variants,
  not the admin translation store.
- New top-level `web/` (Next.js + Auth.js + generated typed client); a confidential `oikumenea-web`
  Keycloak client (dev realm); an **optional** `ui`-profiled `web` service in `docker-compose.yml`
  (port 8445). The generated `web/src/lib/api/schema.d.ts` is never hand-edited. See
  [web-ui.md](../web-ui.md).

---

### D-PersonSocialChannels — Social-network & messenger presence as catalog-typed person channels with analytics-grade attribution (extends D-PersonContactChannels)

**Decision.** A person gains a **social-network / messenger presence**, modelled in two additive
layers over the existing contact channels, plus an instance-admin platform catalog. All follow the
[person](../modules/person.md) child-table pattern (RID PK, `person_id`/parent FK `ON DELETE CASCADE`,
soft-delete, `set_updated_at`, all writes audited, reads scoped through the holder per
D-PersonReadScope, erased on purge):

- **`person_platforms`** — instance-admin catalog (D-Code/D-i18n: natural `code` PK, translatable
  `name` via the [localization](../modules/localization.md) store `entity_type='platform'`, `status`,
  `sort_order`) with **`category TEXT CHECK (category IN ('messenger','social'))`**. Seeded messengers
  `telegram`/`whatsapp`/`signal`/`viber`; socials `instagram`/`linkedin`/`x`/`facebook`.
- **`person_messenger_links`** *(layer a — reachability over existing channels)* — annotates an
  existing phone **or** email with a messenger platform: an **XOR FK** (`phone_id` →`person_phones`
  *or* `email_id` →`person_emails`, exactly one non-null via CHECK), `platform_code` →`person_platforms`
  (write-time restricted to `category='messenger'`), `is_primary`, optional `verified_at`. `pii:contact`.
  Ontology Link `link__reachable_on`.
- **`person_social_accounts`** *(layer b — standalone handle, independent of any phone/email)* —
  `person_id` →`person_persons`, `platform_code` →`person_platforms`, `is_primary`. Object
  `PersonSocialAccount` + Link `link__holds_account`. Enriched with **four analytics-grade practices**:
  - **Identity stability** — an immutable `platform_user_id TEXT` (the platform's internal id, the
    durable key) alongside the **mutable** current `handle TEXT`, both `pii:contact`; one active row per
    `(person, platform_code, platform_user_id)` when the id is known, else per
    `(person, platform_code, lower(handle))`.
  - **Verification** — `platform_verified BOOLEAN` (platform "blue-check") **distinct from**
    `verified_by_operator_at TIMESTAMPTZ` (operator confirmation); both **non-PII** metadata.
  - **Profile (stored now)** — `display_name`, derived `profile_url`, `language`, all `pii:contact`.
  - **Attribution provenance** *(the core practice)* — `source TEXT CHECK (source IN
    ('self_declared','operator_verified','imported'))` + `confidence TEXT CHECK (confidence IN
    ('confirmed','probable','possible'))` on the `HOLDS_ACCOUNT` link, so a claimed account is a
    **weighted, sourced assertion**, not a bare fact. Non-PII.
- **`person_social_account_handles`** — handle-rename **history** (`account_id` FK, `handle`,
  `valid_from`/`valid_to`, soft-delete; `pii:contact`) so a rename never breaks the link.

**Explicitly out of scope.** **No time-series social-graph metrics** (follower/following/activity
counts) — surveillance-adjacent and outside a personnel directory's purpose; not built, not parked.
**Free-text `bio` + `self_declared_location`** are `pii:sensitive` and are **NOT stored** until the
envelope-encryption seam is extended (**DS-29**) — the same gating stance as gender identity under
D-PersonBio; documented as a future column, not created now.

**Why.** A universal directory increasingly needs to record reachability on messengers and presence on
social networks. Splitting into a phone/email-derived **reachability** layer and a **standalone account**
layer mirrors how the data actually arrives (a number *is* a Telegram; a LinkedIn handle stands alone).
Adopting the proven Palantir-style ontology practices — **stable id vs mutable handle + rename history**,
**provenance + confidence on the attribution link**, and **platform-vs-operator verification** — is what
turns "a username" into analytics-grade, queryable, weightable data. Catalog-typing the platforms keeps
the vocabulary operator-managed and localizable. Excluding behavioural metrics and gating free-text
profile prose behind DS-29 keeps the feature inside the project's PII discipline (D-PIITiers).

**Why not** (a) one polymorphic `person_channels` table: loses typed FKs and the XOR reachability shape;
(b) store provenance/confidence on the account Object rather than the link: the *claim* is the
relationship, not the account; (c) collect follower/activity metrics: surveillance creep with no
authorization purpose; (d) store `bio`/location now at `pii:contact`: understates the tier and bypasses
the envelope rule.

**Consequence.** New tables `person_platforms`, `person_messenger_links`, `person_social_accounts`,
`person_social_account_handles` ([person](../modules/person.md)); platform names join the
[localization](../modules/localization.md) store (`entity_type='platform'`). New person sub-resource
endpoints (`/persons/{id}/messenger-links`, `/social-accounts`, the account's handle history) + a catalog
read (`GET /person/platforms`), gated `person.read`/`person.update` (scoped through the holder). The
`HOLDS_ACCOUNT` `source`/`confidence` columns are a registered analytics exception to the "provenance is
mostly absent as a column" divergence ([ontology-mapping](../ontology-mapping.md) §4.4). All four tables
**erased on person purge** (the purge erasure list + `DeleteAll*` extend to their `pii:contact` columns).
No new module, no new third-party dependency. Promotes open-question **DS-41**. See
[person](../modules/person.md), [localization](../modules/localization.md).

### D-PersonRelationships — Person↔person ties as per-type reified self-links (extends D-Ontology, mirrors membership's temporal link)

**Decision.** A person gains **relationships to other persons**, each modelled as a **per-type reified
self-link** (`Person → Person`, D-Ontology `link__<type>`) with **both endpoints in-directory**
(`person_persons` rows). Per-type tables (never one generic table, never a bare FK), each mirroring the
`membership_memberships` temporal-link shape (RID PK, soft-delete, timestamps, `effective_from`/
`effective_to` where a lifecycle applies, `status TEXT`+`CHECK`). All are **instance-global** (like
Person), reads project through D-PersonReadScope, all writes audited, and when **either** endpoint person
purges the link is erased (the `PersonPurged` subscriber extends to both endpoints):

- **`person_partnerships`** — marriage **and** engagement folded into one lifecycle: a symmetric pair
  (`CHECK (person_id_a < person_id_b)`, no self-pair), `status ∈ engaged|married|divorced|widowed|
  annulled|dissolved`, `effective_from`/`effective_to` (NULL = ongoing); **at most one active
  `engaged`-or-`married` row per person**. Link `link__partnered_with`.
- **`person_kinships`** — directional `parent_of` (`parent_id → child_id`, no self-edge),
  `status ∈ active|disestablished` + soft-delete (adoption / legal disestablishment). Siblings are
  **derived, not stored**. Link `link__kin_parent_of` (a distinct RID from tenant's unit
  `link__parent_of`).
- **`person_guardianships`** — `guardian_id → ward_id`, relation label, effective interval, `status` —
  legal guardian/dependent, **distinct from blood `parent_of`**. Link `link__guardian_of`.
- **`person_sponsorships`** — `sponsor_id → sponsored_id`, catalog-typed relation kind (godparent /
  academic advisor / military mentor), effective interval. Link `link__sponsor_of`.
- **`person_next_of_kin`** — `subject_id → contact_id` (**both in-directory**), relation label +
  priority ordering — a **nomination**, not a blood fact. External free-text next-of-kin is **out of
  scope** (both ends must be directory persons). Link `link__next_of_kin`.
- **`person_associations`** — symmetric `subject ↔ associate`, relation label, `kind ∈
  associate|coi|no_contact`, provenance — conflict-of-interest declarations + prohibited-contact
  (discipline). Link `link__associated_with`.
- **`person_social_links`** — friend/follower, `status ∈ active|archived`; Link `link__social_tie`.
  **Deferred — not built (revised 2026-06-09).** On review it was cut from the M14 delivery: unlike the
  other six it has **no consumer** (authority never derives from a relationship; no PDP rule, order
  effect, or report reads it), **no authoritative source** (a "friendship" is not recorded from a
  document/order/legal status, and D-PersonSocialChannels excludes social-graph integration), the
  intended "proof of friendship" gate (each endpoint merely *has* some social account) proves nothing
  about an actual tie, and it is **redundant with `person_associations`** for the only actionable
  adjacency (conflict-of-interest / no-contact). For an authorization+directory service it is scope
  creep. It may return later **only** with a real account-level model (linking the specific
  `person_social_accounts` and/or a shared platform) plus a trustworthy source — at which point it gets
  its own decision. The `SOCIAL_TIE` link type stays registered in
  [ontology-mapping](../ontology-mapping.md) §2 as *deferred*.

Open-ended relation vocabularies (sponsorship kind, association kind, next-of-kin relation label) are
**catalog-typed** via a new **`person_relation_types`** catalog (`code` + translatable `name` +
`category`), consistent with this project favouring operator-managed catalogs over compiled CHECK lists;
fixed lifecycle statuses (partnership, kinship) stay `TEXT`+`CHECK`.

**Why.** A personnel directory across army/church/university needs family and social structure: marriage
and kinship for next-of-kin and benefits; godparents (church) / advisors / mentors as sponsorship;
guardianship for dependents; and — Palantir-style — a generic **association** link for COI and
prohibited-contact tracking. Reifying each tie as its own per-type Link (with identity, attributes, and
history) rather than a bare FK is the binding D-Ontology stance and lets each relationship carry status
and an effective interval. Per-type tables keep each relationship's invariants explicit (canonical pair
ordering, one active marriage, directional kinship) instead of overloading one polymorphic table.

**Why not** (a) one generic `person_relationships` table with a `type` column: erases per-type
invariants and FKs, contradicts "never reify a bare FK / never one generic table"; (b) store next-of-kin
as free text: loses referential integrity and the directory's resolve-or-redact purge guarantee; (c)
keep marriage and engagement as separate tables: they share the symmetric-pair + effective-interval
lifecycle, so one `person_partnerships` table with a richer status set is the cleaner reified link.

**Consequence.** New tables `person_partnerships`, `person_kinships`, `person_guardianships`,
`person_sponsorships`, `person_next_of_kin`, `person_associations`, and the `person_relation_types`
catalog ([person](../modules/person.md)) — `person_social_links` is **deferred** (above) and **not
built**. New per-type sub-resource endpoints under `/persons/{id}/` plus a polymorphic
`DELETE /persons/{id}/relationships/{id}`. New Link types registered in
[ontology-mapping](../ontology-mapping.md) §2 (`SOCIAL_TIE` marked deferred). The `PersonPurged`
erasure path removes links on **either** endpoint's purge. No new module. Promotes open-question
**DS-42** (expanded). See [person](../modules/person.md).

### D-RankSystems — Multinational rank systems, standardized-grade comparability, and scheme presets (extends D-Rank, refines L-OneRankScheme)

**Decision.** The single rank scheme gains a new **top level — the `rank_system`** — so one deployment
can hold **several national/organizational rank systems at once** (a coalition directory carrying US and
Ukrainian ranks together). The scheme shape becomes **`rank_system → rank_category → rank_type` (tree)
`→ rank`**. Each rank carries an optional **standardized grade code** that makes ranks comparable
**across** systems, and a full system subtree can be populated from a **preset** rather than entered by
hand. Three parts:

- **`rank_systems`** (new) — a national/organizational rank ladder (`ua-armed-forces`, `us-armed-forces`,
  `nato`). RID PK, stable `code` (D-Code) + translatable `name`, `sort_order`, soft-delete; optional
  `country` → `geo_countries` (D-Geo; `NULL` for supranational systems like NATO/UN). `rank_categories`
  gains `system_id` (a branch — army/navy/air — *within* a system), and `system_id` is **denormalized
  down** onto `rank_types` and `rank_ranks` exactly as `category_id` already is, so grouping, sibling
  code-uniqueness, and seniority need no recursive walk. Sibling `code` uniqueness is scoped within the
  system.
- **Standardized grade (`rank_grades` + `rank_ranks.grade_code`)** — a seeded reference catalog
  `rank_grades` is the cross-system comparability scale: **NATO STANAG 2116** (officers `OF-1..OF-10`
  plus `OF(D)`; warrant; other ranks `OR-1..OR-9`), each row a `code` (natural-key PK), a `tier ∈
  officer|warrant|enlisted`, and an `ordinal`. Migration-seeded (the D-Geo reference-registry carve-out,
  natural key → no D-RIDSeeding GUC issue). A rank's optional `grade_code` FK references it. The name is
  deliberately generic (*standardized grade*, not "NATO code"): a **non-military** deployment leaves it
  `NULL` and simply has no cross-system comparator, honoring **L-SingleDomain** ("no org-type
  discriminator").
  - *Comparison semantics.* Intra-system seniority is unchanged — the structural order
    `(system, category.sort_order, type path, rank.sort_order)`. Cross-system **equivalence** = same
    `grade_code` (US `OF-5` ≈ UA `OF-5`). Cross-system **seniority** = `grade.tier` then `grade.ordinal`.
    If either rank lacks a `grade_code`, the pair is **incomparable across systems** — the pure
    `isSenior(a,b)` helper returns *unknown*, never a wrong answer.
- **Presets (bundled templates + import).** A *preset* is a curated document for one `rank_system`
  subtree (system → categories → types → ranks, each with `code`/`name`/`sort_order`/`grade_code`),
  shipped in-repo as opt-in reference data (e.g. `deploy/rank-presets/{ua-armed-forces,us-armed-forces,
  nato-generic}.json`) — **never auto-seeded** (rank stays deployment-specific). A new endpoint
  **`POST /rank-scheme/import`** (`rank.scheme.manage`, instance-scope) applies one preset as a
  **code-keyed, idempotent upsert in one transaction** (RIDs minted at import on the GUC-bearing pool per
  **D-RIDSeeding**; re-import updates `name`/`sort_order`, never duplicates). It is **additive/upsert
  only — it never deletes an in-use node**. Audited as a `rank.scheme.manage` action; returns a
  created/updated/skipped summary.

**Why.** A coalition or multinational force is real: one personnel directory holds soldiers ranked in
different national systems, and operators need both **bootstrap without hand-entry** and **cross-national
comparability** (who is senior; what is equivalent). The existing scheme already expressed "parallel
ladders" as sibling `rank_categories`, but (a) a flat category list can't hold *branches within a
nation*, and (b) the single global `sort_order` is meaningless across nations. A dedicated `rank_system`
level plus the **NATO STANAG 2116** grade — the established real-world idiom for comparing ranks across
nations — fixes both without inventing a bespoke ordering. Presets-as-data keep the scheme operator-owned
while removing the tedium.

**Why not** (a) *multiple independent schemes* (`person → (scheme_id, rank_id)`): would **break
L-OneRankScheme** and still need the same grade comparator; the one-registry model already suffices.
(b) *A pairwise rank↔rank equivalence table*: high-maintenance and subjective where a published standard
(STANAG 2116) already exists; equivalence falls out of a shared `grade_code` for free. (c) *Auto-seed a
default ladder*: rank is deployment-specific (army vs university); presets stay opt-in.

**L-OneRankScheme is refined, not broken.** It still holds: **one** scheme registry, edited by the
instance admin, **never adopted per unit**. "Multinational" means that one registry now contains multiple
`rank_systems` — it does **not** mean multiple schemes or per-unit schemes. The lock's note below points
here.

**Consequence.** New table `rank_systems` and reference catalog `rank_grades`; `rank_categories` gains
`system_id` (denormalized to `rank_types`/`rank_ranks`); `rank_ranks` gains `grade_code`
([rank](../modules/rank.md)). New endpoints: `rank_systems` CRUD, `grade_code` on rank create/edit,
`GET /rank-grades`, and `POST /rank-scheme/import`; `GET /rank-scheme` now nests
`systems → categories → types → ranks`. New Objects `RankSystem` + `RankGrade` registered in
[ontology-mapping](../ontology-mapping.md) §1 (the rank tree roots at `RankSystem`). A person's
system is *derived* through `rank → type → category → system`; **`person` now holds one rank per
system** via the reified `person_ranks` link (see [D-Rank](#d-rank--rank-on-person-rank--permission),
which scoped "one rank" to "one rank per rank system"). Additive / expand-only. Lands as the scoped
**M15** ([milestones](../milestones.md)); promotes
open-question **DS-43** (non-military cross-system comparators). See [rank](../modules/rank.md).

---

### D-RLSLiveReach — RLS policies compute reach live in SQL; the GUC contract is O(1)

**Decision.** The RLS backstop's policies no longer consume an app-materialized unit list: they call
**`oikumenea.authz_unit_in_reach(unit uuid, wr boolean)`** — a planner-inlined `LANGUAGE sql STABLE`
predicate that semi-joins the subject's **live** role assignments (`authz_role_assignments ⋈
authz_roles ⋈ authz_role_permissions`, unexpired, role live, `'*.read'` for `wr=false` / any
non-read permission for `wr=true`) over the tenant closure (`subtree` scope over authority-bearing,
non-deleted graphs; `unit` scope = target only) — the exact SQL mirror of the domain `ReachSet`
semantics. The per-request GUC contract shrinks to **`app.person_id` + `app.is_instance_admin`**,
set and reset in **one round trip each** (`db.AcquireScoped`). App-side, nothing on the request path
materializes reach anymore: person/document read scope is a SQL semi-join (membership's
`VisiblePersonIDsForSubject` — adaptive sparse/dense plan shapes behind a capped reach-cardinality
probe — and `SubjectCanReadPerson`), and the shadow gate is a batch probe
(`ReadableUnitsForSubjectAmong`). `Service.EffectiveReach`/`RLSStateFor` are deleted;
`domain.ReachSet` survives as the pure, property-tested reference semantic and the oracle of the
randomized reach differential test (`internal/membership/reach_differential_integration_test.go`),
which pins Go ⇄ SQL ⇄ policy-predicate parity.

**Why.** Review-2026-07 **R-02**: a staff-level subject near an org root has reach ≈ the whole org.
Materializing it app-side and shipping it as comma-joined GUCs cost, **per request**: a 2.7 s
closure expansion, a 7.4 MB GUC payload, 4+4 `set_config` round trips, and linear `= ANY` scans in
every policy (measured, M46 harness at 10⁵ units / 10⁶ persons). Live policies cost an index probe
per row scanned (subject-idx + closure-PK), and every guarded read path is keyset-paged — measured
3–6 ms per page for the same subject; GUC payload is 40 bytes.

**Two deliberate semantic notes.** (1) *Stronger*: the backstop is now **exact under revocation** —
revoking an assignment hides rows on an already-pinned connection mid-request (asserted in
`rls_integration_test.go`), which also backstops D-AuthzGrantCache's ≤2 s decision staleness. (2)
*Never narrower*: the predicate reads only RLS-exempt tables (a read of a guarded table would
recurse into its own policy), so it cannot apply `ReachSet`'s soft-deleted-descendant refinement —
the backstop may pass rows keyed on a soft-deleted descendant unit that the authoritative app layer
excludes. The person/document hardening seam noted in D-RLSDefenseInDepth is now **one policy
away** (the reach predicate exists in SQL); still deliberately not shipped.

**Consequence.** **Amends D-RLSDefenseInDepth** (mechanism only; the backstop/authoritative split
stands). Migrations `0011`/`0023`/`0024`/`0026` edited in place (unreleased-slice convention;
`atlas.sum` re-hashed); `db.RLSState` is `{PersonID, IsInstanceAdmin}`; the RLS-exempt list gains
the `authz_*` tables incl. `authz_epoch`. Landed with **M47**.

**Extended by the machine reach arm (M55, migration `0042`).** A **service principal** (D-ServiceIdentities)
sets no `app.person_id`, so every policy denied it — that was M51's deliberate "no reach." M55 adds a
third GUC **`app.principal_id`** and a principal arm: an **org-confined** grant
(`authz_principal_grants`, `org_id NOT NULL`) authorizes that organization's RLS-guarded rows. The
recursion constraint (read only RLS-exempt tables) forbids joining `tenant_units` to learn a unit's
org, so `0042` adds a dedicated **RLS-exempt projection `authz_unit_org(unit_id → org_id)`**,
trigger-maintained from `tenant_units` (the `unit_id` FK is `DEFERRABLE INITIALLY DEFERRED` so the
BEFORE-INSERT projection write precedes the parent row). `authz_principal_org_in_reach(org, wr)` is the
org-direct grant primitive (read live → **revocation is immediate**, matching the person arm and the
M51 no-cache principal design); `authz_unit_in_reach`'s new arm resolves a **child** table's unit → org
via the projection, while the **`tenant_units`** policy uses the org-direct arm on the row's own
`org_id` column — the one case the projection can't serve, since a BEFORE-trigger write is invisible to
the same statement's `WITH CHECK`, and the case that lets a connector create a brand-new (edgeless)
unit. `org_id IS NULL` (instance-wide) grants confer **no** operational reach (blast-radius = the org).
`db.RLSState` gains `PrincipalID`; the person + admin arms are unchanged (an empty probe when
`app.principal_id` is unset — no hot-path regression). The person/document hardening seam noted above
stays unshipped.

---

### D-AuthzRequestContext — Authority state is fetched once per request and snapshotted on the context

**Decision.** The identity-federation authenticator resolves the subject's **authority state** —
instance-admin flag + active grants — **exactly once per request** (`authorization/application`
`ContextWithAuthority`) and attaches it to the request context as an opaque snapshot. Every
PDP-consuming call in that request (`Decide`, `DecideBatch`, `HoldsPermissionAnywhere`,
`EffectiveReach`, `FilterVisibleUnits`, and every `pep.Require*` gate built on them) consumes the
snapshot instead of re-running the grants join. A call about a **different** subject than the
request's falls through to a fresh resolve (the snapshot is keyed by person RID); out-of-request
callers (CLI, boot seeds, tests) carry no snapshot and behave as before.

**Why.** Review-2026-07 **R-01**: a typical guarded request executed the grants join
(`authz_role_assignments ⋈ authz_roles ⋈ authz_role_permissions`) + the instance-admin probe
**≥3–4×** (authenticator reach + every `Require*`), serialized in front of every handler — measured
at 4 grants-joins/request on the M46 harness. Authority state was already a snapshot-at-call-time;
making it a snapshot-at-request-start changes no semantics and deletes the multiplier.

**Consequence.** `middleware.RLSResolver` is replaced by `middleware.AuthorityResolver`
(`ContextWithAuthority(ctx, personID) (ctx, RLSState, error)`); the snapshot type never crosses the
module boundary. A guarded request issues **exactly one** authority fetch regardless of gate count
(asserted by `db.WithQueryCounter` in integration tests; `BenchmarkEnforceOnSnapshot` shows
decision cost independent of gate count). Landed with **M47**; see also
[D-AuthzGrantCache](#d-authzgrantcache--epoch-validated-per-process-grant-cache-2-s-revocation-bound)
for what "fetch" means across requests.

---

### D-AuthzGrantCache — Epoch-validated per-process grant cache (2 s revocation bound)

**Decision.** The per-request authority fetch (D-AuthzRequestContext) is served through a
**per-process cache** of `(subject → isAdmin, grants)` validated against a **single-row revocation
epoch** (`oikumenea.authz_epoch`) that **every authority-mutating transaction bumps** — grant/revoke
assignment, role permission-set edit, role delete, instance-admin grant/revoke, base-role re-sync,
person-merge subject repoint. Protocol: an entry validated less than **2 s** ago
(`grantCacheTTL`) is served with **zero** database reads; a staler entry is revalidated with **one**
single-row epoch read (unchanged epoch ⇒ keep grants; changed ⇒ full refetch); misses read the epoch
first, then fetch (concurrent bumps can only make an entry conservatively stale). Per-subject
singleflight collapses stampedes; the map is size-capped (drop-all at 10k entries).

**Why.** Review-2026-07 **R-01.2**: even fetched once per request, the grants join is per-request
DB load that scales with RPS; at national scale authorization dominates the query mix. The epoch
makes invalidation **exact** at the cost of one indexed single-row read per subject per TTL window,
instead of heuristic TTL-only staleness.

**Revocation-latency contract (replaces "a revoke takes effect on the next call").** A
grant/revoke/role-edit is visible: **immediately** in the process that performed it (the mutating
write resets its local cache after commit), and within **≤ 2 s** on every other replica. The RLS
backstop underneath is exact/live (D-RLSLiveReach), so a stale cached ALLOW cannot actually read
revoked-away rows on RLS-guarded tables — the cache bounds *decision* staleness, not *data*
exposure.

**Consequence.** New single-row table `authz_epoch` (derived counter, no RID — ontology-mapping
§4.3) in migration `0007`; `BumpAuthzEpoch` runs inside the mutating transactions
(`authorization/application/service.go`, `person_merge.go`); `grantcache.go` owns the protocol
(unit-tested: fresh-hit/revalidate/bump/reset/singleflight). Raising `grantCacheTTL` trades
revocation latency for epoch-read rate — change it only by amending this block.

### D-PersonSearch — Trigram-indexed directory search over names + variants, filtered in SQL

**Decision.** Directory search (`GET /persons?query=`) matches a case-insensitive substring against
a person's own name/code haystack **and** any of its per-person **name variants** (transliterations,
aka/aliases — D-PersonNamesCLDR), so a person is findable by a native-script or alias form, not only
the Latin `display_name`. Matching is served by a **pg_trgm GIN index** over a `STORED` generated
`search_text` column on both `person_persons` and `person_name_variants`. The text filter is applied
**in the database**, never in Go: the admin path is a dedicated `SearchPersons` query and the scoped
path folds the trigram predicate into the read-scope semi-join (`VisiblePersonIDsForSubjectSearch`,
extends **D-PersonReadScope** / D-RLSLiveReach). The two match branches (person haystack, variant
haystack) are a **UNION of id-sets** so each stays an index bitmap scan; the keyset stays on the
person RID.

**Why.** Review-2026-07 **R-06**: the previous `ILIKE '%q%'` over four un-indexed columns was a
sequential scan per keystroke at 1M persons, and the scoped path filtered the already-paginated page
**in Go** — so a scoped search could return an **empty page while `hasMore`** and paged through the
whole roster for a rare name. Folding the filter into SQL makes pages fill correctly and O(page); the
GIN index makes a ≥3-char query a bitmap scan. Searching variants closes a correctness gap (non-Latin
/ aliased names were unfindable). An `A OR EXISTS(subquery)` predicate is **not** index-able (it
seq-scans), which is why the match is a UNION semi-join and the empty-query "list" is a separate
query with no filter — keeping every plan index-served regardless of the prepared-statement plan mode.

**Consequence.** `pg_trgm` + `search_text` generated columns + two GIN indexes in migration `0005`;
`SearchPersons` (person) and `VisiblePersonIDsForSubjectSearch` (membership) queries; the Go-side
`matchesPersonQuery` filter is deleted. Sub-3-char queries fall back to a scan (pg_trgm needs a full
trigram) — acceptable for the rare short-prefix case. The membership→`person_persons` read in the
scoped search is a sanctioned cross-module read (like the existing authz/tenant reads in that query).

**Generalized (review-2026-08 R-21).** This is now the **standard** pattern for **every** typeahead /
substring-list surface, not just persons. Migration `0011_infra` extends it to language languoids, geo
locations, education institutions, education publications/scholarships, and companies; the same rules
apply everywhere:
- **The filtered list splits into an unfiltered `List<X>` query and a trigram `Search<X>` query**, the
  application branching on empty query. A `(@query = '' OR <ilike>)` guard is banned: under a generic
  prepared-statement plan Postgres can't prove `@query` is non-empty and falls back to a seq scan, so
  the guard silently defeats the GIN index. Splitting keeps the trigram predicate unconditional.
- **One GIN over a `search_text` haystack, not per-column indexes.** A multi-column match is folded into
  a single `lower(colA || ' ' || colB …)` haystack with one GIN trigram index, and `Search<X>` matches
  `search_text ILIKE '%q%'` — exactly `person_persons`. This is a **STORED generated column** (measured:
  a per-column BitmapOr *and* a bare expression index both lose to a seq scan at 30k rows, because the
  planner has no selectivity stats for an ILIKE over a bare expression and defaults to ~4% → a seq scan
  looks cheaper; a STORED column carries real `pg_stats`, so a rare substring is estimated correctly and
  the index is used). The column also appears in the sqlc model of every package that reads the table —
  an inert extra field on `tenant_organizations` (shared by institutions + companies), nothing selects
  it into a domain type. Single-column surfaces (`company_org_profiles.short_name`) index the real
  column directly (it already carries stats).
- **A match spanning a join** (company: the org haystack on `tenant_organizations` **and** `short_name`
  on the joined `company_org_profiles`) is a **UNION of id-sets**, never an OR across the join — an OR
  can't BitmapOr across tables, whereas each UNION arm stays a bitmap scan (the persons names+variants
  shape). The outer keyset then paginates by the driving RID.
- **No `OFFSET` pagination anywhere.** Keyset only. R-21 converted geo's three offset-paginated queries
  (the last in the codebase): `Search`/`Bbox` key on `id`; nearest-first `Near` keys on the
  `(distance, id)` sort pair (using exact `ST_Distance` in ORDER BY *and* the keyset comparison — the
  `<->` KNN operator's index-approximated order would skip rows at a page boundary; the distance is
  returned and carried in the opaque page token).

### D-AuditRetention — Monthly-partitioned audit ledger; retention is an operator act

**Decision.** The append-only audit ledger (`oikumenea.audit_log`, D-Audit) is **declaratively
RANGE-partitioned by month on `created_at`**. Its physical PK becomes `(id, created_at)` (the
partition key must be in every unique constraint; `id`-leading keeps a by-RID `GetAuditEntry` a PK
lookup) and its secondary index set is trimmed to what the read API serves —
`(created_at DESC, id DESC)` keyset, `(target_type, target_id)`, `(actor_person_id)`, `(unit_id)`
(the RLS read policy's key); the `actor_type` and `request_id` singles are dropped. Monthly
partitions are rolled forward idempotently at boot (`ensure_audit_partition`, advisory-locked) with a
`DEFAULT` catch-all backstop. **Retention is an operator policy, never automatic deletion**:
`audit.retention-months` (install config, default **0 = retain forever**, legal-hold-safe) records
intent, and the operator enforces it by running `detach_audit_partitions_before(cutoff)` then
dumping/dropping the detached partitions. D-Audit's **semantics are unchanged** — same-transaction
insert, append-only (`reject_mutation` still guards every partition), one row per Action.

**Why.** Review-2026-07 **R-07**: one unbounded table with 6 secondary indexes on every write's
critical path becomes the largest, hottest, ever-growing object, with no archival story. Monthly
partitioning bounds index/vacuum/backup cost to the live months and makes retention a metadata
`DETACH` rather than a mass `DELETE` over the biggest table; the index diet cuts per-write
maintenance. Legal-hold requirements belong to the operator, so retention is **config, not code** —
and defaults to keeping everything.

**Consequence.** Migration `0001` defines `audit_log` partitioned from the start (edited in place —
pre-release, the DB is rebuilt); `ensure_audit_partition` / `detach_audit_partitions_before` helpers
ship with it; the composition root rolls the window forward under the boot-seed advisory lock.
Automated scheduled retention enforcement is an explicit **open seam** (a future scheduler reads
`audit.retention-months`). See [audit](../modules/audit.md) for the partition + retention runbook.

---

### D-PersonModuleSplit — The `person` god module splits into core / profile / sensitive behind one `PersonService`

**Decision.** `internal/person` is split into **three internal Go modules behind ONE unchanged Conjure
`PersonService`**, along a **data-sensitivity + change-cadence** axis:

- **`internal/person` (core)** — identity, CLDR names + name variants, bio (`birthdate`/`date_of_death`/
  `sex`/`country_of_birth`), ranks (`HOLDS_RANK`), the read-scope projection (owns the `MembershipReader`
  seam), and the **merge/purge lifecycle orchestration**.
- **`internal/personprofile`** — citizenships, residences, addresses, contact channels
  (email/phone/call-sign + messenger links + social accounts), the `SPEAKS` languages link, the
  person↔person relationships, and the **non-encrypted** institutional ties (government positions,
  lobbying relationships, external references). Owns the `LocationLookup` seam (addresses).
- **`internal/personsensitive`** — everything **envelope-encrypted or `pii:special`**: physical
  identity / descriptions / distinguishing marks, the encrypted ethnicity link, the M35 overlays
  (crypto wallets, personality, inferred political leaning), watchlist matches + regulatory sanctions,
  and the **encrypted M33 party membership**. **The `crypto.Cipher` and all seal/unseal logic live
  here** (the R-09 headline — one module, one reviewer surface), plus the `ColorLookup` and
  `WatchlistLookup` seams.

One `internal/person/transport` package holds one `Service` that **composes the three application
services** and implements the single `PersonService`; each handler delegates to the owning service, and
`mapError` stays a single function over the one shared `domain` package.

**Delivered as PR-2a (application-layer split, move-only).** The three modules share a **single
`internal/person/domain` kernel** (all types, the error block, validators, seam interfaces) and a
**unified `internal/person/adapters` repository + `queries/person.sql`** — so merge/purge orchestration
and `GetPerson` child-hydration keep working through the shared repo. **No schema change, no migration,
no Conjure contract change**; the RID type codes and every ontology mapping are untouched (D-Ontology /
D-ResourceIdentifiers unaffected). A shared cycle-free `internal/person/appkit` leaf carries the tx +
audit-record plumbing all three modules reuse.

**Delivered as PR-2b step 1 (the purge fan-out / R-08 cross-module fix).** A **`PersonPurged{ID}` event
+ `SubscribeErase` helper** (leaf `internal/person/events`, mirroring `PersonMerged`/`SubscribeRepoint`)
now let each owning module erase its **own** rows on purge in the same transaction: `PurgePerson`
publishes `PersonPurged` after the person-owned scrub, and `education`/`company` moved their
person-referencing deletes out of `person`'s `repo.Purge` into their own `SubscribeErase` subscriptions
(`SubscribePersonPurge`). This **removes `person`'s inline cross-module purge writes** (`person` no
longer deletes `company_*`/`education_*` tables) — the R-08 module-table boundary, without an allowlist.
Behavior is identical (same rows, same tx); the event is published only on a real purge, never on the
merge-tombstone `Purge` of a stub (whose rows were already re-pointed by `PersonMerged`).

**Delivered as PR-2b step 2 (the dormant erasers wired — closes the purge PII gap).** The previously
deferred `document`/`finance`/`vehicle`/`religion` `ErasePerson*` methods are now activated as
`PersonPurged` subscribers (`SubscribePersonPurge`): because they crypto-erase + write a correlated audit
row they subscribe to `TypePersonPurged` **directly** (each delegates to an extracted `tx`-accepting
helper so it runs in the purge transaction), not via the raw-SQL `SubscribeErase`. Before this, a
`PurgePerson` left those modules' rows un-erased (the methods existed but nothing called them on purge) —
a real erasure gap now closed: a purge crypto-erases the person's documents/personal-codes, sole-held
finance accounts+cards, owned vehicle registrations, and encrypted religious affiliations in the same
transaction. Each records its audit row only when it actually erased something.

**Delivered as PR-2b step 3 (the data-layer split).** `queries/person.sql` (182 queries) and
`adapters/repository.go` (~3000 lines) split three ways by owning table: **core** keeps
`person_persons` / `person_ranks` / `person_name_variants`; **personprofile** owns citizenships,
residences, addresses, the contact channels, SPEAKS languages, person↔person relationships and the
non-encrypted institutional ties; **personsensitive** owns physical identity, ethnicity, party
membership, watchlist/sanctions and the M35 overlays. Each concern now has its **own `adapters/queries`
dir + generated sqlc package** (`personprofilesql` / `personsensitivesql`) and its **own repository
port** in the domain kernel (`Repository` / `ProfileRepository` / `SensitiveRepository`), implemented by
its own adapter (the shared pgtype/error plumbing is duplicated as adapter-local helpers so the concern
adapters import neither core's `adapters` nor each other). `domain.Person` is **lean** (the seven profile
child-slices dropped); the transport `GetPerson` handler composes them from the profile service
(`composeProfile`). The purge fan-out completes: core `repo.Purge` is **core-only** (name variants +
person PII scrub); `personprofile` / `personsensitive` erase (hard-delete or crypto-erase) their own rows
via `SubscribePersonPurge` in the purge transaction, and `MergePerson` publishes `PersonPurged` for the
tombstoned stub so its non-re-homed residuals are dropped. Two reviewed cross-module reads keep the
concerns off each other's tables: each concern verifies the parent exists via `PersonExists` on
`person_persons`, and `personsensitive`'s watchlist screening reads the PEP flag from `personprofile`
through a late-bound `PEPStatusReader` seam (never touching `person_government_positions` itself). The
**R-08 module-table lint gained per-`person_*`-table ownership** (exact table names override the shared
`person` prefix), with reviewed allowlist entries for the `PersonExists` guard and the geo/language
reference lookups. Every person-family hand-written file is now **< 800 lines** except the two large
concern adapters (`personprofile/adapters/repository.go`, `personsensitive/adapters/repository.go`),
which may sub-split further later.

**Fully delivered (2026-07-09).** The final documentation follow-up landed: the entity data-model
sections moved from [person.md](../modules/person.md) into the concern docs.
[person.md](../modules/person.md) now owns only the **core** entities (`person_persons`, the `HOLDS_RANK`
link, `person_name_variants`) plus the read-scope rule, the PII-governance purge summary, and the single
`PersonService` API surface; [personprofile.md](../modules/personprofile.md) owns the data model for
citizenships / residences / addresses / contact & social channels / `SPEAKS` languages / person↔person
relationships / non-encrypted institutional ties; [personsensitive.md](../modules/personsensitive.md)
owns the sensitive/encrypted overlays. Each `person_*` entity now has exactly one owning doc.

**Why.** Review-2026-07 **R-09**: `person` was a god module — bio + contacts + social + relationships +
physical identity + addresses + watchlists + overlays all piled onto the same five files (each M31–M35
milestone grew them), and the PII tiers concentrate in the biggest, hardest-to-review files. Splitting
by sensitivity puts the **entire envelope-crypto surface and its reviewers in one module**
(`personsensitive`), bounds merge-conflict and review blast radius, and stops unrelated concerns sharing
tx/repository helpers. Keeping one Conjure service means the split is invisible to every client.

**Consequence.** New `internal/personprofile` + `internal/personsensitive` (+ the shared
`internal/person/appkit`), each a full hexagonal module (`domain` kernel shared, own
`application`/`adapters`/`queries`). `cmd/oikumenea/main.go` registers all three, wires colors/watchlist →
sensitive and location → profile, binds the `PEPStatusReader` seam (sensitive → profile), subscribes each
concern's `SubscribePersonPurge`, and adds `profileSvc`+`sensitiveSvc` to the boot-time `MustBeBound` seam
slice (R-11). [personprofile.md](../modules/personprofile.md) and
[personsensitive.md](../modules/personsensitive.md) now **own the data model** for their entities;
[person.md](../modules/person.md) owns the **core** entities (`person_persons`, the `HOLDS_RANK` link,
`person_name_variants`) plus the read-scope rule, the PII-governance purge summary, and the single
`PersonService` API surface. Each `person_*` entity has exactly one owning doc.

---

### D-EventOutbox — Transactional outbox for the `notify` event class (extends the `pkg/events` bus)

Domain events split into two classes. **`atomic`** (the existing `events.Bus`) dispatches subscribers
**inside the publisher's transaction** — all-or-nothing with the originating write; this stays the
default and every event today is `atomic`. **`notify`** is a new class delivered **after commit, at
least once, out of process** via a **transactional outbox**: the producer enqueues one row on its own
write transaction (`events.OutboxWriter.PublishNotify` → `oikumenea.platform_outbox`, migration `0011_infra`),
so the event commits atomically with the write, and a dispatcher (`internal/platform/outbox`) drains the
queue after commit — claiming rows `FOR UPDATE SKIP LOCKED` (replica-safe, mirrors the hermenea worker,
D-Hermenea/R-13), retrying with exponential backoff, dead-lettering past `max_attempts`. Handlers are
**idempotent** (a crash between commit and dispatch re-delivers).

The event bus is **sealed** after boot (`Bus.Seal` in the composition root, after all modules wire their
subscribers): a later `Subscribe` **panics**. Adding an `atomic` subscriber widens every publisher's
transaction, so it is a **decision-level change**, not a routine wiring edit; an out-of-band side effect
that must not hold the write's locks is a `notify` handler on the dispatcher instead.

**Why.** Review-2026-07 **R-10**: the same-transaction bus is load-bearing across ten modules, so a
merge/purge is one giant transaction and every new `atomic` subscriber widens it; there was **no**
at-least-once channel for effects that *should* be async (webhooks, projections, cache/grant-epoch
invalidation), so everything defaulted into the write tx. The fix is not "make events async" —
same-transaction is correct for order-apply and merge/purge — but to **name the two classes** and provide
the missing `notify` channel + the boot-time guardrail, so the next async-shaped need is not jammed into
the write transaction by default.

**Consequence.** New `oikumenea.platform_outbox` table (infra, not an ontology entity; no RLS, mutable
status), `pkg/events.OutboxWriter` + `NotifyPublisher`, the `internal/platform/outbox` dispatcher (started
in `main.go`, wired into cleanup), and `Bus.Seal`. **No `notify` producers exist yet** — the outbox is a
live-but-empty proven seam (integration test: enqueue-on-commit durability + delivery + at-least-once
redelivery on handler failure). Classified in [patterns.md](patterns.md) *Domain events: atomic vs.
notify*. The multi-replica *notify* posture (rest of R-13) rides on this. Retention of dispatched/dead
rows is an operator concern (like audit partitions), an open seam.

---

### D-DataScope — What a deployment may hold; the product is a personnel directory + registry platform (owns the PCI-DSS posture)

**Decision.** go-oikumenea is a **personnel directory + multi-domain registry / intelligence
platform** — this **re-frames** the earlier "generic, domain-agnostic personnel & authorization
service (Keycloak-like)" label, which no longer describes the schema. Beyond identity/directory data
(names, ranks, positions, documents, declared attributes) a deployment MAY hold the following
**enumerated, bounded** data classes — this list *is* the boundary; adding a class outside it is a
decision-level change, not a routine migration:

| Class | Home | Tier |
| --- | --- | --- |
| Payment cards (**recoverable PAN**, no CVV) + bank accounts (IBAN) | D-Finance | `pii:special` (envelope-encrypted) |
| Crypto wallets (address, balance) | D-PersonOverlays | `pii:sensitive` |
| **Inferred** political leaning | D-PersonOverlays | `pii:special` (encrypted, one-active, crypto-erased) |
| Self-declared ethnicity | D-PhysicalIdentity | `pii:special` (envelope-encrypted) |
| Religious affiliation | D-ReligiousAffiliation | `pii:special` |
| Watchlist / sanctions / PEP matches | D-Watchlists | metadata-only snapshot |
| Physical identity, aliases, marks | D-PhysicalIdentity | `pii:sensitive` |
| Addresses / location | D-PersonAddresses / D-Location | `pii:contact` |

**PAN policy.** The **recoverable full PAN is deliberately retained** (D-Finance) — a registry
platform legitimately needs the full instrument, so this is not truncated to BIN + last4. The
consequence is owned, not waved away: **every deployment therefore falls into PCI-DSS Cardholder
Data Environment (CDE) scope** — the whole Postgres, the oikumenea binary, its host, and arguably
the operator's network segment become auditable CDE. **Envelope encryption does not de-scope this.**
This is an explicit **operator responsibility**, stated so at deploy time.

**Aggregation rule.** One `person` read must never unlock the join of ethnicity + religion + politics
+ finance + location. **Every `pii:special` surface is gated behind its own permission code** — this
is the load-bearing reason for the R-09 `person`→core/profile/sensitive split
([D-PersonModuleSplit](#d-personmodulesplit--the-person-god-module-splits-into-core--profile--sensitive-behind-one-personservice))
and per-module permission codes, not mere hygiene. **Enforced (R-14 audit, 2026-07-11):** the three
`pii:special` person surfaces read on their own codes — `person.ethnicity.read`,
`person.political_leaning.read`, `person.party_membership.read` — which are **not** in the base
`unit-reader` set and are **not** folded into `unit-manager`/`unit-admin`; they compose a standalone,
additive **`sensitive-reader`** base role, so reading Art.9 person data is always an explicit grant
(no graduated role unlocks the aggregation). Religion affiliation (`affiliation.manage`) and finance
(`finance.read`) were already isolated the same way; the person module was the one exception the audit
closed. **Residual:** `person_persons.attributes` is itself `pii:special` and is returned by the core
`person.read`; because it is a free-form bag, the boundary here is **policy** — Art.9 content belongs
in the typed special tables (ethnicity/politics/party), never the attributes bag — not a further code
split.

**Why.** Review-2026-08 **R-14**: the code already *is* a registry-platform data surface (PANs,
IBANs, wallets, inferred politics, ethnicity, religion, sanctions), while the stated identity was
"Keycloak-like." The honest move is to **name the boundary and own the compliance posture**, not to
keep a directory-shaped label over intelligence-grade data. Enumerating the permitted classes turns
unbounded scope-creep into a reviewable list; the per-code aggregation rule turns "we split the
person module" from hygiene into an enforced access boundary.

**Consequence.** The identity line in `CLAUDE.md` and [`../README.md`](../README.md) (and CLAUDE.md's
module map) are re-framed to match (this is the docs half of R-14/R-18). PCI-DSS CDE scope is a
documented operator responsibility. **Key rotation is a
named must-fix**: PCI Req 3.6 requires it, and R-22 shows the current envelope scheme cannot rotate
keys — so **full PAN without rotation is recorded compliance debt**, tracked as R-22 (Phase 11),
*strengthened* (not resolved) by this decision. **No schema change** flows from D-DataScope itself
(PAN kept as-is). The per-`pii:special`-code aggregation rule was **enforced** by the R-14 audit
(2026-07-11): the audit found the person module lumped ethnicity/politics/party under `person.read`
(which the base `unit-reader` holds) and closed it with three dedicated read codes + the
`sensitive-reader` base role (no migration — code-defined permissions, idempotent base-role reseed).
Relates to
[D-Finance](roadmap-decisions.md#d-finance--bank-accounts--payment-cards-banks-as-company-orgs),
[D-PersonOverlays](roadmap-decisions.md#d-personoverlays--financial-behavioral--psychological-overlays-extends-d-overlayfoundation-d-specialpii),
[D-Watchlists](roadmap-decisions.md#d-watchlists--live-lookup-sanctionspepinterpol-via-hermenea--a-regulatory-sanctions-overlay-extends-d-hermenea),
[D-SpecialPII](roadmap-decisions.md#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29),
[D-CryptoProvider](#d-cryptoprovider--pluggable-envelope-encryption-for-sensitive-pii-reshapes-ds-29),
and [D-PIITiers](#d-piitiers--5-tier-pii-classification-via-comment-on-column).

---

### D-VisibilityScope — One read-visibility interface, three canonical scope shapes, registered per object type

**Decision.** Row-level read visibility gets **one shared interface** in the authorization module
(`internal/authorization/scope`): `Visibility.ReadableIDs(ctx, subject, isAdmin, candidateIDs) →
ids` (order-preserving subset). Exactly **three canonical implementations** exist, matching the
three real policies already in the code — **person-scope** (the D-PersonReadScope membership
semi-join, via an injected batch probe), **unit-scope** (owning-unit mapping + the existing
shadow-gate `FilterVisibleUnits`), and **catalog-scope** (the endpoint's read-permission gate is
the entire decision; identity row trim). Every object type exposed on a **cross-type surface**
(unified search R‑26, generic links R‑27, …) MUST register a `Visibility` at composition time;
a cross-type surface with an unregistered type **fails composition** (boot), never serves
untrimmed rows. The adapter is **additive**: existing per-module endpoints keep their current
code paths; differential equality with each module's own list endpoint (same subject, same
fixtures) is the correctness contract, enforced by tests.

**Why.** Review-2026-09 **R‑30**: visibility was re-invented per module (person semi-join, tenant
shadow gate, catalog coarse gates), so no generic surface could answer "may this subject read
object X" — the load-bearing prerequisite for unified search and link traversal. This is the
honest cost of D-NoRLS (app-layer row security): the obligation concentrates into exactly this
adapter. Three named shapes — not a per-module bespoke fourth — keep the next module's choice a
classification, not a design.

**Consequence.** New `internal/authorization/scope` package (impls take injected funcs; the
authorization module still imports no other module — the composition root wires concretes). New
membership batch probe `SubjectReadablePersonsAmong` (the `= ANY` variant of the existing
point-probe reach predicate; no schema change). `pep.Enforcer` gains the non-erroring
`AllowedAnywhere` probe. New object types that want cross-type exposure declare their scope shape
at design time (alongside their Object/Link/Action kind, D-Ontology).

**Amendment (M58 ticket 4 follow-up) — organization reach is DERIVED from unit reach.** The
unit-scope shape above was, in practice, applied to **organizations** as well: tenant's org list
called `FilterVisibleUnits` on organization RIDs. That type-checks and reads plausibly, and it asks
the unit reach probe whether an ORGANIZATION rid is among the subject's readable **units** — a
question whose answer is always no, because `authz_role_assignments.target_unit_id` is
`NOT NULL REFERENCES tenant_units` and an organization can never be a grant target. The consequence
was not a policy anyone chose: a shadow organization was visible to an instance admin and to nobody
else, by accident of the assignment table's shape.

Organizations therefore get their **own** gate — `FilterVisibleOrgs` / `ReadableOrgsForSubjectAmong`,
with the same predicate folded into the org dashboard's scoped arm — under this rule: **an
organization is visible when any of its live units is in the subject's reach.** Derived, not granted.

*Why derivation rather than a new grant primitive.* The two alternatives — a nullable
`target_org_id` on the assignment, or an org-level `scope` value — both add a primitive to the PDP
for a question unit reach already answers. Derivation also **discloses nothing new**: `listUnits`
takes the org RID as a REQUIRED argument and gates the units rather than the organization, so a
subject with reach inside an org could already enumerate its units and was already holding its RID.
And it is precise — reaching one shadow org does not reveal another.

*Consequence.* `gateOrgs` is a sibling of `gateUnits` rather than a call into it, and deliberately
so: the two ask different questions, and a shared entry point taking a mode is exactly what let an
organization RID be passed to the unit probe unnoticed. `internal/tenant/transport/shadowgate_test.go`
pins which helper each shadow-bearing handler uses, and names an org handler calling `gateUnits` as
the original bug rather than a style slip.

**Extension (M58 ticket 5) — the organization gate covers the SIDECAR PROFILES, on all three read
surfaces.** A company and an institution ARE tenant organizations (M41 / D-UnifiedOrgGraph), so they
carry the organization's public/shadow bit — and their modules applied no gate at all, from M21 and
M20 respectively. `listCompanies` and `listInstitutions` joined `tenant_organizations` and never
looked at `visibility`; the point reads leaked the same rows one at a time; and unified **search**
was a third door, both types registered under the catalog (identity) scope.

All three now route through one helper per module over the same `FilterVisibleOrgs`, plus
`scope.NewOrgScope` for search. Two consequences worth stating because they are easy to get wrong:
a gated-out row is **NotFound, never 403** (`shadow` hides existence; a permission error confirms the
RID names something real), and the dashboard's scoped arm folds the predicate into **SQL** rather than
trimming the result — trimming after the fact is right for a page and wrong for a count. Each module
carries its own copy of the structural `shadowgate_test.go`, because the gate is applied per handler
in the transport and nothing outside that package can observe its absence.

---

### D-UnifiedSearch — One cross-type SearchService as a fan-in over the per-module trigram queries

**Decision.** Cross-type object search is **one Conjure service** (`api/search.conjure.yml`,
`SearchService.searchObjects` at `/search/v1/objects`) implemented as a **fan-in over the existing
per-module trigram search queries** (D-PersonSearch + the R‑21 generalization) — **not** a global
index table, not Elasticsearch, not a new ranking engine (all explicitly rejected by
review-2026-09 "NOT recommended"). Modules register a `SearchProvider` (object-type token, read
permission, keyset search func) **plus** a D-VisibilityScope `Visibility` in one composition-time
registry (`internal/search`); the registry is the same seam the R‑27 link facet will extend.
Per request: providers run in **fixed lexicographic type order**; a provider is **skipped
entirely** if the subject lacks its read permission (non-erroring probe); returned rows are
trimmed through the registered `Visibility` (person's provider searches pre-trimmed in SQL);
hits are `{rid, objectType, label, snippet}` — the RID is self-describing, so no per-type
response shapes. Pagination is a composite keyset token (per-provider cursors, base64url JSON);
provider cursors advance over **raw** (pre-trim) rows so trimming can shorten a page but never
skip a row. The endpoint itself requires only an authenticated subject — authorization is
entirely per-provider gate + per-row trim.

**Why.** Review-2026-09 **R‑26**: no unified search existed; the ⌘K palette substring-filtered
the first 100 rows per type in the browser — at registry scale a search box that silently finds
almost nothing, while the R‑21 trigram indexes sat unused by the first surface an operator
reaches. The Gotham-style entry point ("search anything, then navigate") starts here. Fan-in
over existing per-type queries keeps each arm on its proven index and keeps relevance/ranking
out of scope (trigram has no rank; deterministic grouped-by-type order instead).

**Consequence.** New thin `search` module (`internal/search`: no tables, no RIDs minted, no RID
service number, reads not audited — consistent with every other read path). Provider wiring is
composition glue (`cmd/oikumenea/search_providers.go`) closing over module services; initial
providers: person, languoid, location, institution, company, publication, scholarship. The web
palette (and progressively the console's other search boxes) consume the endpoint instead of
client-side fan-out caches. Relevance ranking, locale-map labels, and per-provider snippets
beyond a secondary line are named open seams in [modules/search.md](../modules/search.md).

---

### D-LinkTraversal — One generic getObjectLinks endpoint as a fan-in over a pkg/rid-derived link-descriptor registry

**Decision.** "What links does object X have?" is answered by **one Conjure service**
(`api/links.conjure.yml`, `LinkService.getObjectLinks` at `/links/v1/objects/{rid}/links`, plus a
depth-1 `searchAround`) implemented as a **fan-in (logical union) over the existing reified link
tables** — **not** a graph database, not a new join/edge table, not a client-side fan-out over the
web registry. Each module registers a **link `Descriptor`** (link type, table, the two endpoint
columns, per-end target object types, read permission) at composition time
(`cmd/oikumenea/link_descriptors.go` → `internal/links`); the descriptor set is the Go counterpart
of the web console's hand-authored `links[]`, but its link types are **validated against the
drift-proof pkg/rid link-type registry** (R-28) at Register time, and the engine's **`MustBeBound`
coverage assertion** (main.go's boot seam loop) fails startup if any kind=link RID type is neither
registered **nor** explicitly **exempt** — so a link table added by a future milestone appears in the
console **without editing `web/`**, or fails boot until wired. Per request: for the queried RID's
decoded object type the engine selects every incident link arm, runs one **keyset query per arm** on
that arm's existing endpoint index, **skips** an arm whose read permission the subject lacks
(`pep.AllowedAnywhere`, non-erroring), and **trims neighbor rows** through the neighbor object type's
D-VisibilityScope adapter (person → person-scope; unit → shadow gate; every other neighbor →
catalog scope, differential-equal to the owning module's coarse-gated list). Pagination is a
composite keyset token (per-arm cursors advancing over **raw** pre-trim rows, base64url JSON — a
trimmed page shortens but never skips). Polymorphic F-014 ends (`holder_kind`/`holder_id`) declare
one target per discriminator value (person `(6,1,1)` / company = tenant org `(4,1,6)`), closing
R-32's fix-sketch item 2. The endpoint requires only an authenticated subject; authorization is
entirely per-arm gate + per-row trim (an ungranted subject gets an empty result, not 403).

**Why.** Review-2026-09 **R-27**: no backend answered "links for object X", so the universal object
page and graph explorer fired ~19 HTTP GETs per object (one per hand-declared `links[]` collection),
swallowed per-endpoint errors to empty groups, and — worst for a registry/watchlist product — under-
reported relationships whenever a new link table outran the hand-maintained web registry. A
descriptor registry **derived from pkg/rid** cannot drift; a generic UNION over the already-indexed
link tables collapses the fan-out to O(1) and makes Gotham-style search-around reachable. Row
security stays in the app layer (D-NoRLS): the obligation lands on the same D-VisibilityScope adapter
Phase 14 built, which is why R-30 sequenced before this.

**Consequence.** New thin `links` module (`internal/links`: no tables, no RIDs minted, reads not
audited — like `search`). The one place **raw dynamic SQL** is justified in the codebase: a union
over a runtime-registered set of tables is not expressible in sqlc, so the engine interpolates
compile-time descriptor identifiers through `pgx.Identifier.Sanitize` with bound value params.
Descriptor wiring is composition glue closing over the pool + module services; the initial set covers
38 traversable link tables with 8 documented exemptions (encrypted/free-text/untyped-polymorphic
ends, `has_role`'s three-way assignment, `instance_admin`'s absent neighbor). The web
`resolveLinkGroups` (and the object page + graph explorer through it) consume the endpoint; the web
`links[]` arrays become display-only hints, no longer the traversal source of truth. Server-side
neighbor **labelers** are now delivered: each neighbor type registers a batch labeler that resolves a
`targetLabel` **locale→text map** (D-i18n) — person from its name variants, everything else via an
overlay over the base name + `i18n_translations` (`localization.NamesByID`) — so link/graph rows show a
real name, not the RID tail. Per-link-type (vs per-module) permission codes are delivered for the
person/ownership graphs (D-LinkPermissions) and remain a per-module rollout seam in
[modules/links.md](../modules/links.md).

**Depth-2 search-around — delivered (review-2026-09 follow-on, gated on measurements).** `searchAround`
takes an optional `depth` (1 default, clamped to 2); depth-2 returns each direct neighbor's own
neighbors as a flat list, each hop-2 `LinkRow` tagged `hop:2` and carrying `viaRid` (the intermediate
node), so "any path between these two objects?" is answered with one request. It stays **Postgres over
the existing link tables** (no graph DB): the walk is two sequential keyset phases sharing the page
budget — (1) drain the origin's hop-1 arms (the depth-1 engine, unchanged); (2) enumerate the trimmed
hop-1 neighbors as a **frontier in neighbor-RID order** (a distinct-neighbor keyset query per origin
arm, enumerated a **batch** at a time so a wide frontier does not re-scan the arms per node) and expand
each with an inner hop-2 `collect`. **Per-hop authorization is identical to depth-1** — the arm gate
and D-VisibilityScope trim run at *every* hop, reusing the same primitives, so a neighbor the subject
cannot read is neither returned nor expanded (no depth-2 bypass). The trivial backtrack edge to the
origin is excluded; genuine alternate 2-paths are kept (each row is an *edge* `via→neighbor`, not a
deduped node). Pagination is a distinct **v2** keyset token (origin cursors + a scalar frontier
high-water mark + the current node's cursors); v1 and v2 tokens never decode as each other. The
gate ("< 1 s for a 50-neighbor node, 2-hop, M49-scale") is met — ~0.4–0.5 s for a 50-node frontier
fanning into 767 neighbors on the 1M-person seed-scale dataset. Clearing it required two fixes to the
shared descriptor layer: (a) `parent_of`/`written_in` mark `NoSoftDelete` (their append-only tables
have no `deleted_at` — a latent depth-1 500 on unit/language expansion); (b) a new optional descriptor
equality **filter** (`FilterCol`/`FilterVal`, a bound param) applied to `member_of` as `status='active'`
so traversal matches the membership **partial** indexes (`…WHERE status='active' AND deleted_at IS
NULL`) and stays index-backed instead of seq-scanning 1M rows — it also scopes the graph to *current*
memberships. Verified by `cmd/oikumenea/links_integration_test.go` (`TestSearchAroundDepth2*`, incl.
the env-gated `TestSearchAroundDepth2Scale` gate measurement) and the domain token tests.

---

### D-ActionTypes — A checked action-type catalog behind the free-text audit_log.action

**Decision.** Action *types* become a **checked, enumerable catalog** in Go (`pkg/action`): each
registered action is `{code, service, targetType, permission}`, keyed by the stable dotted `code`
that `audit_log.action` already carries (`assignment.grant`, `unit.transition`, …). The catalog is
the **source of truth**, validated three ways: (1) **write-time** — `audit.Service.Record` rejects
an entry whose action is not registered (`action.Validate`; kept out of the stdlib-only
`audit/domain` so the domain stays dependency-free), so a typo (`assignment.granted`) or an
un-catalogued new action fails a test rather than drifting across modules; (2) **doc coherence** —
[ontology-mapping.md §3.1](../ontology-mapping.md) carries the catalog as a generated table that
`pkg/action/catalog_doc_test.go` asserts equal to the registry (R-28-style); (3) **discoverability**
— `AuditService.listActionTypes` (`GET /audit/v1/action-types`) serves it so a client (the
console's actions panel) enumerates actions and their gating permission instead of hard-coding them.
Each action also carries an optional **parameter schema** (R-29's deferred seam, now closed):
`ActionType.RequestType` names the Conjure request type carrying the action's inputs, package-qualified
(`oikumenea.authorization.GrantAssignmentRequest`), and the argument list is **derived from the Conjure
IR** — never hand-authored — by `tools/genactionparams` (`scripts/gen-action-params.sh` → the generated
`pkg/action/params_gen.go`), so it **cannot drift** from the contract (same single-source discipline as
R-28). `listActionTypes` returns them as `parameters: list<ActionParam{name, type, required, docs}>`,
rendered read-only in the audit action catalog and the `/o/[rid]` Actions panel. The schema is
**descriptive / discoverability only** — *not* enforced at write time (`audit.Record` still validates the
code, not arguments; validating arguments would thread request inputs into the audit writer across every
module — the wrong layer). Annotation is **expand-only**: an unannotated action reports no parameters, and
`TestRequestTypesResolve` fails if a named `RequestType` stops resolving in the IR.
It is **expand-only**: the audit RID stays the generic **kind=action / type_code 0** (D-Audit —
history is not rewritten); a new milestone adds catalog rows, and the coherence + validation tests
fail until it does. The [order](../modules/order.md) module is named the **reference pattern** (a
typed action with catalogued `effect`s applied all-or-nothing via events in one transaction).

**Why.** Review-2026-09 **R-29**: action *instances* existed (every audited write mints a kind=action
RID) but action *types* did not — the name was free text in `audit_log.action` with no read-time
contract, so nothing prevented cross-module drift, nothing let an operator or the console *discover*
what actions a type affords, and audit-analytics-by-type depended on string hygiene across ~26
modules. The repo already paid the cost of the pattern (per-action permission codes, audit rows,
endpoints) without the payoff of the catalog. A registry — **not a framework** — closes that at the
schema-discipline level the rest of the ontology already meets.

**Consequence.** New leaf `pkg/action` (imports only `pkg/rid`); the catalog was derived from the
actual emit sites (runtime capture of every audited write ∪ static scan) so it is complete against
what the code emits — the full integration suite is the standing completeness check (an unregistered
emitted action fails its module's test). `permission` is the **module-granular gating write
permission** (the finding's "required permission"); a dedicated per-action permission code, where a
module later defines one, swaps in with no other change. Retrofitting per-action RID `type_code`s
into historical audit rows is explicitly **not** done (kind=action/type 0 stays valid — expand-only).
Nested-body rendering is capped at **one level** by decision: the four nested request types are three
shallow all-flat objects (coordinate, order-item, link-identity — structured) plus the rank import
**tree** (deep + self-referential — JSON), and a full recursive form for the latter is worse UX than
JSON, so `ActionParam.fields` is emitted only for a one-level-structurable nest.
Per-action parameter schemas (originally a named open seam) are now delivered, IR-derived and
descriptive (above); annotating the remaining modules' `RequestType`s is incremental follow-on.

---

### D-ActionInvocation — An IR-derived endpoint binding per action, driving a generic console action runner

**Decision.** Extend the action catalog (D-ActionTypes) from *describing* an action to letting the
console **run** it. Each action gains an **endpoint binding** — `{method, path, pathParams}` — that is
**derived from the Conjure IR** by `tools/genactionendpoints` (`scripts/gen-action-params.sh` → the
generated `pkg/action/endpoints_gen.go`), the same single-source discipline as the R-29 parameter
schema: an action is joined to its endpoint by request-body type, with the code's verb+tokens
disambiguating the create/update pairs that share a body and a module-prefix scan resolving the
body-less deletes/lifecycle POSTs. The generator **fails the build** if an action does not resolve to
exactly one real endpoint (or a duplicate-binding guard fires) — so a contract change that breaks a
binding is caught in CI, not at runtime. `pkg/action.EndpointFor(code)` exposes it and
`AuditService.listActionTypes` returns it as `endpoint: optional<ActionEndpoint>`. The **web action
runner** (`components/ontology/ActionRunner.tsx`) builds the request from `parameters` + `endpoint`
and POSTs via the SDK's generic `request()` — no hand-authored URL. It renders **flat** params as
inputs, **`list<scalar>`** as repeatable inputs, and a **nested object one level deep** (or a list of
one) whose fields are all flat as a **structured sub-form** (`ActionParam.fields`, emitted by
`genactionparams`); a **deeper or self-referential** body (the rank import tree) carries no `fields` and
falls back to a raw-JSON editor. The object's own RID fills the single path param. It surfaces on the universal object page
(`/o/[rid]`) and — because that page redirects person/unit to bespoke routes — as a collapsed
"Actions (advanced)" panel on the person & unit pages. **Sub-resource** update/delete actions (≥2 path
params: the parent RID + a child id the object doesn't carry) stay non-inline — routed to the bespoke
row managers that already hold the child id. Regulated/secret params (**PAN, IBAN, inferred political
spectrum, wallet address**) are tagged `sensitivity` in the catalog (`pkg/action.paramSensitivity`, a
hand-authored **D-DataScope** policy fact) so the runner **masks** the input and runs an **advisory**
client validator; the server's `pkg/personalcode` validators remain authoritative.

**Why.** R-29 delivered the parameter schema but left invocation as a named open seam ("the framework
R-29 warned against"). The trap was a *hand-authored* action→URL map — reintroducing exactly the drift
R-29 killed. Deriving the binding from the IR and asserting it at build time avoids that: the runner is
*generated wiring*, not a parallel routing table. The result closes the loop from Foundry's ontology
benchmark — an object page where a catalogued Action is not just discoverable but runnable.

**Consequence.** Actions with **no invocable endpoint** are explicitly **exempt** (a test-pinned set,
mirrored in the generator): purge-cascade erasures (`*.erase`, emitted internally on `PersonPurged`) and
the bulk `import.*` ingestion plane (hermenea's job, not a per-object console action). The catalog is
never *narrower* than reality — a new action without a binding fails `TestActionEndpointCoverage` until
it is wired or exempted. Nested-object bodies (5 actions, all with richer **bespoke** forms) use the
JSON editor rather than a recursive schema pipeline — a deliberate bound, since those forms are the
better UX. `permission`-gating is unchanged: the runner shows every catalogued action, and the PDP
rejects a POST the subject may not make (surfaced as the endpoint's error).

---

### D-LinkPermissions — Per-relationship read codes gating the module endpoint and the traversal arm alike

**Decision.** A **relationship is its own disclosure**, separate from the objects it connects. Each
reified relationship that a deployment should be able to withhold gets its **own read permission code**,
and that one code gates **both** its dedicated module list endpoint **and** its
[link-traversal](../modules/links.md) arm (D-LinkTraversal) — so the bespoke page and the generic object
graph can never disagree about what a subject may see. Landed for the **person relationship graph** —
`person.partnership.read`, `person.kinship.read`, `person.guardianship.read`, `person.sponsorship.read`,
`person.next_of_kin.read`, `person.association.read`, `person.address.read` — and the two **ownership**
links: `finance.holder.read` (who holds an account) and `vehicle.registration.read` (who a vehicle is
registered to). They compose **additive, per-module base roles** (`person-relationship-reader`,
`finance-graph-reader`, `vehicle-graph-reader`), deliberately **outside** `unit-reader`/`-manager`/
`-admin` — exactly the D-DataScope (R-14) treatment of the Art.9 set: `person.read` lists people but no
longer reveals *who they are related to* or *where they live*; `finance.read` lists accounts but not
*whose* they are.

**Why.** The links engine originally gated every arm on the owning module's coarse read
(`person.read`, `finance.read`, …) — the named open seam in links.md. That makes the relationship graph
exactly as disclosed as the directory, which is wrong for a registry/intelligence platform: the personal
graph (kin, guardians, partners, home address) and asset ownership are the sensitive product, and the
coarse code conflated "may see this person" with "may see their whole network".

**Consequence.** Two boundaries are ratified rather than papered over. (1) **Aggregate-embedded links
keep the coarse code**: `holds_rank` and `speaks` are returned *inside* the person aggregate
(`getPerson`), so a separate arm code would hide them in the graph while the page still showed them —
incoherent, so they stay on `person.read`. (2) **Structural/reference links keep the coarse code**:
`parent_of` (the unit org tree), `written_in`, `unit_language`, `curriculum_item`,
`course_prerequisite`, `manufactured_by`, `has_industry`, `branch_of`, … have no personal subject; a
per-link code there restricts nothing and a restrictive role would hide the org tree from a plain
reader. The rule is therefore **"a code per relationship that is a distinct disclosure"**, not a code
per link table. The narrowing is real and intentional: a subject with only `unit-reader` sees fewer
links than before and must be granted the additive role — proven by `TestRelationshipCodeGate`
(cmd/oikumenea) and the base-role invariant `TestRelationshipReadsGatedByOwnCodes`. Remaining modules
(company ownership, education person-links, religion clergy) follow the same pattern where they warrant
a distinct disclosure; unwired links keep their module read with no engine change.

---

### D-Temporal — A three-tier link-history classification (native validity by default) plus getObjectHistory over the audit ledger

**Decision.** Every reified **Link** (kind=2) declares a **history tier** at design time, the same way
it declares its Object/Link/Action kind (D-Ontology). There are three tiers:

| Tier | What it means | Criterion | Where the history lives |
|---|---|---|---|
| **(a) native validity** — the **default** for relationship/state Links | the row carries its own truth-interval; history is never overwritten | the Link asserts a state that is *true over a period* (membership, ownership, a held rank, an affiliation, a residence) | on the row: `valid_from`/`valid_to` (NULL `valid_to` = active) — the **canonical pair** — or the grandfathered equivalents `effective_from`/`effective_to`, `granted_at`/`revoked_at`(+`expires_at`), `founded_on`, `awarded_on` (D-ResourceIdentifiers §4.1 already defines these **as** the validity pair) |
| **(b) object history** | an Object's change history is read back from the **audit ledger**, not a per-row interval | any audited Object (person, unit, …) | `oikumenea.audit_log (target_type, target_id)` + `before/after`, served by **`AuditService.getObjectHistory`** |
| **(c) history-exempt** | a reference/structural association whose change is a *correction*, not a dated historical event | validity would be noise (a linguistic fact, a catalog association, a structural edge within an already-versioned snapshot) | nothing — deliberately undated |

**Tier (a) is the mandate going forward:** a new relationship Link **must** carry native validity;
a new Link is tier-(c)-exempt only by an explicit, reasoned classification. The boundary is not prose
— it is **enforced executable**: `cmd/oikumenea/temporal_tiers_test.go` holds a `temporalTiers`
classification of **every** kind=Link RID type and fails the build if (1) a `pkg/rid` Link type is
unclassified (`TestTemporalTierCoverage` — the drift guard R-31 makes real), or (2) a tier-(a) Link's
migration DDL lacks a validity column (`TestValidityLinksHaveDatingColumn` — so "this Link is dated"
can never be an unbacked claim). This is the temporal analogue of the R-27 link-coverage assertion and
the R-28 RID coherence check.

**The tier-(c) exempt set is closed and small** (six kind=Link types — the *bounded* divergence R-31
demanded, replacing §4.1's open-ended hand-wave): `locale_language` (2,1) and `unit_language` (4,2)
(reference associations), `written_in` (13,1) (a Glottolog languoid↔script linguistic fact),
`curriculum_item` (14,5) and `course_prerequisite` (14,6) (structural facts inside an
already-versioned curriculum), and `has_ethnicity` (6,9) (an encrypted self-declared *attribute*, not
a dated edge). **Everything else is tier (a).** The gap was closed by adding `valid_from`/`valid_to` (+ an inline
range CHECK) to the **thirteen** previously-undated relationship Links — `parent_of`, `holds_rank`,
`kin_parent_of`, `next_of_kin`, `associated_with`, `speaks`, `beneficiary_of`, `succeeded_by`,
`branch_of`, `has_industry`, `located_at`, `classified_as`, `site_of` — **folded directly into each
table's original `CREATE TABLE`** in its existing migration and the dev/test DBs rebuilt, per the
unreleased-build-out convention (edit in place, not an ALTER migration — the same way R-32's shape
CHECKs were folded in). Existing dated Links keep their grandfathered column names (no churny rename);
the canonical `valid_from`/`valid_to` pair applies to these newly-dated Links. (Note: "thirteen" is
the count *newly folded in-place* here. In total **~17** Links carry the canonical `valid_from`/
`valid_to` pair — these thirteen plus four already dated from their original migrations: `lives_at`
(M32) and `party_membership`/`government_position`/`lobbying_rel` (M33). The authoritative,
build-enforced set of validity-bearing Links is `cmd/oikumenea/temporal_tiers_test.go`'s
`tierValidity` classification, not this prose list — an as-of reader must use all of them.) On a real post-release
upgrade the equivalent change would instead ship as an expand ALTER (`valid_from := created_at`,
`valid_to := deleted_at`); pre-release, the migrations are the source and are edited in place.

**The cheap read capability** ships with it: **`AuditService.getObjectHistory(rid)`**
(`GET /audit/v1/objects/{rid}/history`) — a token-paginated, reverse-chronological read over
`audit_log` filtered to `target_id = rid`, returning `{at, action, actor, targetType, outcome, before,
after}`. It is gated by **`audit.read`** (it is a convenience projection of `GET /audit?targetId=…`, so
a stronger whole-endpoint gate would be bypassable), but the **`before`/`after` payloads are redacted**
(nulled, `redacted=true`) unless the caller also holds the **sensitive-reader capability**
(`authorization.SensitiveReadPermissions()` — all of `person.{ethnicity,political_leaning,party_membership}.read`,
or instance admin). Rationale: a folded per-object timeline can surface pii up to the **D-DataScope**
special-category ceiling, so the payloads sit behind the same bar as reading that Art.9 data directly,
while the *timeline* (when/what/who) stays visible to any `audit.read` holder.

**Why.** Review-2026-09 **R-31**: the material for a Gotham-style dossier timeline existed — effective
dating on some Links, append-only lifecycle ledgers, and a per-object-queryable audit log with
`before/after` — but **no endpoint read any of it back as history**, and **§4.1 set no boundary** for
which Links owe validity, so each milestone re-decided ad hoc (some M18–M50 Links got `valid_from/to`,
some got nothing) and the divergence grew monotonically. For a registry whose identity includes
watchlists and sanctions, *when we knew something* is the line between an intelligence platform and a
CRUD directory — and it is unrecoverable for any table that overwrote state before dating was added.

**Consequence.** As-of *reconstruction* and full **bitemporality** (a second, transaction-time axis)
remain named seams — **not** shipped (and blanket bitemporality is explicitly *not* the plan; R-31
exists to *scope* history). The console consumes `getObjectHistory` for object timelines.

**As-of reconstruction — re-scoped (2026-07-23), because the original framing was infeasible.** The
seam was described as "folding `before`/`after` into a point-in-time view of a whole object." That is
**not achievable from the audit ledger** and never was: `before` is populated by **no** module (always
NULL), and `after` is a small hand-built **non-PII identifier map** (typically `{"id": …}`, sometimes
one changed status field) — updates frequently record only the id, not the changed values. The
`before/after` columns sit under the **`pii:special` ceiling** (D-DataScope), so real attribute values
cannot land there until the **DS-29** envelope seam ships. The ledger therefore answers "who changed
what-kind-of-thing, when" — **not** "what did each field equal at time T"; there is no snapshot and no
diff to fold. The seam is re-scoped to two honest, independent futures:
- **(i) Tier-(a) relationship-graph as-of — buildable, not built.** The ~17 validity-bearing Links
  (above; `temporal_tiers_test.go`'s `tierValidity` set) already store half-open `valid_from`/`valid_to`
  intervals, but **no query reads them as-of**
  today (only "active now", `valid_to IS NULL`, or for ordering). A point-in-time view of the
  *relationship graph* is a bounded future: an `asOf T` filter (`valid_from ≤ T AND (valid_to IS NULL
  OR valid_to > T)`) over those Links, naturally layered onto **D-LinkTraversal**
  (`getObjectLinks`/`searchAround`). This is the only axis with the data to reconstruct today.
- **(ii) Full-object *attribute* as-of — out of scope.** Reconstructing an object's scalar attribute
  state at T requires either **versioned/temporal primary tables** (which do not exist for most object
  types) or **full-snapshot audit** (blocked on DS-29 and a cross-cutting write-path change). Neither
  is planned here; recording it as a named seam is the point.

---

### D-EnvConfig — Environment variables override the YAML config, and the YAML file is optional

**Decision.** go-oikumenea boots 12-factor style: **every** install/runtime config field is overridable
by an **environment variable**, and the YAML file is **optional** — an absent file plus env vars is a
valid, fully env-only boot. This makes the open-source "clone, set a few env vars, run" path work
without authoring YAML. Applies to **both** binaries and the oikumenea **CLI** subcommands.

**Precedence** (highest wins): real process env → `.env` file (loaded at boot, only for keys not
already set) → YAML file → Go zero-value/defaults.

**Mechanism.** `pkg/config/envoverlay` (framework-free: stdlib + `gopkg.in/yaml.v3` only) derives the
env-var name from each field's **YAML path**, schema-driven via reflection over the config struct tags:
each path segment is upper-cased with dashes → `_` and joined with `_`, under a binary prefix
(`OIKUMENEA_` / `HERMENEA_`). Schema-derivation is what disambiguates dashed keys —
`crypto.local-dev.kek` → `OIKUMENEA_CRYPTO_LOCAL_DEV_KEK` (never `crypto/local/dev/kek`). The overlay
parses the file (or an empty mapping) into a `yaml.Node` tree, sets nodes for the env vars that are
present (type-preserving: strings quoted, numbers/bools tagged; a bad numeric/bool value fails boot
naming the var), and re-marshals. The bytes are handed to witchcraft via `WithInstallConfigProvider` /
`WithRuntimeConfigProviderFunc`, so **ECV decryption + unmarshal still run** on the result — env values
are plaintext and never match `enc:`, so they pass through ECV untouched. Runtime env overrides are
applied on each file-reload tick; env is static (a change needs a restart).

- **Slices** (`idp.issuers[]`, `crypto.local-dev.previous-keks[]`, hermenea `sources[]`) use indexed
  names (`OIKUMENEA_IDP_ISSUERS_0_HMAC_KEY`) with **per-index merge** — env overrides one element's
  field while YAML supplies the rest. Shrinking an array requires editing YAML; sparse indices
  materialize empty preceding elements, so keep indices contiguous.
- **Maps** (`modules`): `OIKUMENEA_MODULES_FINANCE_ENABLED=false`.
- **Database** may be set as a whole (`OIKUMENEA_POSTGRES_DSN`, which wins) **or** assembled from
  discrete parts (`OIKUMENEA_DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE`) into a libpq keyword string.
- **R-16 aliases retained:** the documented `OIKUMENEA_HERMENEA_TOKEN` / `HERMENEA_OIKUMENEA_TOKEN`
  names still work (registered as aliases → `hermenea.outbound-token` / `hermenea.inbound-token`); the
  canonical path-derived `…_OUTBOUND_TOKEN` / `…_INBOUND_TOKEN` win when both are set. The `Resolve*`
  accessors are now single struct-field reads (the env value is folded in at load).

**Tradeoff.** Env/`.env` secrets (KEK, blind-index key, HMAC keys, tokens) are **plaintext** in the
process environment — this is the standard 12-factor lane; ECV remains available for YAML-at-rest
secrets. `.env` is gitignored. The full generated env-var surface is
[`docs/reference/env-vars.md`](../reference/env-vars.md) (regenerated from the schema by a golden test —
a new config field cannot silently miss the doc).

---

### D-ObjectFacets — One per-object-type facet vocabulary driving both list filters and per-module stats endpoints (extends D-VisibilityScope, D-PersonReadScope; constrained by D-DataScope)

**Decision.** Each listable object type declares a **facet set** — one vocabulary, two consumers.
A `Facet` is `{key, kind ∈ enum|ref|date-range|bool|numeric-range, column, readPermission, buckets}`,
declared by the module that owns the table (the `internal/links` `Descriptor` shape: each module
describes its own table once). From that one declaration come:

1. **List filters** — **explicit, typed Conjure query args** on the owning module's list endpoint,
   one per facet, in the shape `listUnits(org, domain, unitKind, level, graph, parent, rootsOnly)`
   and `listExternalOrgs(query, kind, country, status)` already use. Not a generic filter DSL, not an
   opaque encoded `filter=` param: the args are typed in the generated SDKs and visible in OpenAPI.
2. **A per-module stats endpoint** — `GET /<module>/v1/stats/<collection>` (the path shape as built;
   see the M57 as-built note), taking **exactly the same
   filter args** as the list endpoint plus an optional `facets` CSV, returning `totalCount` plus a
   `list<FacetDistribution>` (`facet`, `buckets[{key, label, count}]`). One endpoint per module, so a
   whole dashboard is one round-trip; backed by **static sqlc `GROUP BY` queries**, one per
   (module, facet).

Parity between a declared facet and its query arg is a **build-time drift guard** derived from the
Conjure IR, the way `tools/genactionparams` derives per-action parameter schemas (R-29): a facet
without its arg, or an arg without its facet, fails the build.

**Four rules make aggregation safe:**

- **Counts are computed inside the visibility predicate**, never over a post-paged set. Person-scoped
  counts fold in the reach semi-join already proven at scale (`VisiblePersonIDsForSubjectSparse`/
  `Dense`, with `CountReadableUnitsCapped` picking the plan shape); unit-scoped counts must fold the
  shadow gate **into SQL** — `gateUnits` trims *after* the keyset page is cut, which is correct for a
  list (a short page, never a skipped row) and wrong for a count.
- **A facet exists only over a plaintext column.** Every envelope-encrypted `pii:special` value —
  ethnicity, party membership, inferred political leaning, religious affiliation, health detail,
  legal-record offence detail — has **no facet**. This is an invariant asserted at build time, not an
  accident of storage: there is no plaintext to `GROUP BY`, and [D-DataScope](#d-datascope--what-a-deployment-may-hold-the-product-is-a-personnel-directory--registry-platform-owns-the-pci-dss-posture)'s
  aggregation rule forbids the surface regardless.
- **A facet above `pii:basic` inherits its field's own read code**, and a subject lacking it gets that
  facet **omitted** from the response — never a zeroed bucket, never a 403. This is the search
  engine's "skip the provider, don't fail the request" behaviour (D-UnifiedSearch).
- **No bucket-size suppression.** Every counted row is a row the subject may already read and page
  through the same list endpoint under the same filters, so a k-anonymity floor would protect nothing
  it does not already protect. Stated explicitly so it is not re-litigated as an oversight.

**AMENDED (M58 ticket 2) — a fifth rule, and the one property it may relax.** The load-bearing
property is not that a distribution sums to `totalCount` but that **a bucket's count equals the number
of rows its own filter returns** — that is what makes a chart segment and a list filter the same act,
and it holds without exception. Summation is the *usual* consequence, and two shapes cannot deliver
it: a **closure** facet (grouping over a transitive-closure ancestor) and an **M:N** facet (grouping
over a join table) count each row once per bucket it belongs to. `Facet.NonPartitioning` carries the
reason such a facet overlaps, on the `Ledger` pattern — the reason is the declaration, so a second one
costs an argument rather than a copy-paste — and it exempts the summation assertion and **nothing
else**. Two build-time guards contain it: the reason must be substantive, and the facet's table must
not be the listed table (a row has one value in its own column, so a facet grouping one cannot
overlap; an exemption there is imitation, not need). A closure facet additionally confines its buckets
to the current candidate set, without which a single-valued ancestor filter would *widen* rather than
narrow when a bucket is clicked — breaking the very property this rule protects.

**AMENDED (M58 ticket 2) — "static sqlc `GROUP BY` queries" is the mechanism, not the requirement.**
Four modules (`religion`, `externalorg`, and the still-to-come `vehicle`, `finance`) build their SQL
at runtime by a documented per-module choice and ship no `queries/*.sql` at all, so the sqlc-shaped
parity guards cannot read them. The requirement — the list and the stats path apply ONE predicate, and
the aggregate has ONE definition — is unchanged; the proof becomes an **AST check** that both paths
call one shared filter builder plus a single named aggregate const (`pkg/facet/rawpgx_test.go`). The
sqlc-shaped coverage floors **defer** to that guard rather than exempting the types, so a registered
type is still required to be checked somewhere.

**Why.** The console shows every module as a flat list and nothing else: an operator cannot see age
structure, sex structure, status mix or rank distribution, and cannot narrow a list by anything
structural. The contract made that unavoidable — of ~90 list/search endpoints only five carry a real
filter set, there is no sort param anywhere, and **no page envelope carries a `totalCount`**. Counting
cannot be pushed to the console facade either: a facade would have to page the whole table to count
it, and the north star grants facades response shaping, not database reach. One vocabulary shared by
both consumers is what makes a chart click and a list filter the same act.

**Why not a generic cross-type analytics service** (the D-UnifiedSearch / D-LinkTraversal fan-in
shape, which this repo otherwise favours): (a) it needs a dynamic `GROUP BY` builder, a **second**
raw-dynamic-SQL engine after the links union — a cost that engine justified by a genuine union over a
runtime-registered table set, which aggregation does not need; (b) static per-facet sqlc queries stay
readable, `EXPLAIN`-able and index-tunable per module, which matters because the R-21 lesson is that
filter predicates decide plan shape; (c) each module already owns its visibility story, and
`scope.Visibility.ReadableIDs` — an id-list trim — is the wrong instrument for a count over 10⁶ rows.

**Consequence.** No new Go module, **no new tables, no RID service number, no minted RIDs, no audit
rows** — stats endpoints are reads (the `search` / `links` precedent). A new shared `pkg/listing`
kernel lands with the filters, retiring the 14+ copies of `encodeCursor`/`decodeCursor`, the 7+ copies
of `resolvePageSize`/`clampPageSize`, the 5+ redeclarations of `DefaultPageSize`/`MaxPageSize` (only
`audit` reads the runtime tunable today) and the lone `base64.StdEncoding` cursor in
`internal/religion`. Structural filters use the `sqlc.narg('x')::type IS NULL OR col = …` style proven
in `audit`/`tenant`, **not** the `sqlc.arg(x)::text = ''` sentinel — and never the
`(@query = '' OR <ilike>)` shape D-PersonSearch's R-21 generalization bans. Every filtered path is
`EXPLAIN`-checked against the M46 scale harness before its milestone is verified. The catalog of
facets and their charts lives in [`facets.md`](facets.md). Lands as **M56** (filters) and **M57**
(stats endpoints), rolled out across the remaining types in **M58**
([milestones](../milestones.md)). Additive / expand-only.

**As built (M56 ticket 2 — `person` and `unit`).** Three choices this decision left open, settled by
implementation and recorded so the remaining tickets do not re-litigate them:

- **The registry is `pkg/facet`, not per-module.** The IR-derived mirror and the drift test must live
  in the same package as the catalog, which is why `pkg/action`'s catalog+generator+test triad works
  and `internal/links` has no generator. "The `Descriptor` shape" is honoured as a shape; module
  ownership is carried by a mandatory `Module` field and enforced by the plaintext/table guards. The
  boot assertion `facet.Default.MustBeBound()` joins the composition root's seam loop.
- **`person.unitId` is subtree-expanding over every authority-bearing graph**, and `listPersons`
  gains a `graph` arg that narrows the expansion to one graph. `graph` is classified as a traversal
  arg, not a facet; it is rejected on its own. The default is deliberately the same closure set the
  read-scope predicate walks, so the filter cannot widen what a caller may see.
- **A LEDGER may be faceted, though an action may not.** The kind rule stands — an action
  *invocation* is not a collection — but the RECORD of one is: `audit_log`'s rows have identity,
  attributes and history, and they list and filter exactly like an object. They also have no RID type
  token, because each row's RID is minted by the service that PRODUCED the action (`kind=action`,
  generic type 0), so registering an `audit` type would make `rid.TokenOf` describe identifiers that
  never exist. `ObjectType.Ledger` is therefore a one-field escape from the token check that carries
  its own REASON, is refused for a token that is registered, and is held to at most one type by a
  guard — a second ledger is an argument to have here, not a precedent to follow (M58 ticket 1).
- **A PROFILE may be faceted, and it is the SECOND admission that a collection's rows carry no token
  of their own.** A company and an institution are sidecar rows keyed by a `company`- or
  `university`-domain tenant ORGANIZATION's RID (M41 / D-UnifiedOrgGraph), so their token is
  `organization` and it is not theirs alone. `ObjectType.Profile` names the token the rows are keyed
  BY. It is refused on a type that HAS a token, refused for a parent that is not a registered
  kind=object token, and refused alongside `Ledger` — they are different admissions and a type is at
  most one of them.
  Unlike `Ledger` it is **uncapped**, because the sidecar-on-organization shape is a pattern rather
  than an exception; what replaces the cap is that the claim is **checkable**. A ledger's "these rows
  have no token" is written down nowhere and must be argued; a profile's "these rows are keyed by
  that token" is a fact the DDL records, so the guard reads the migrations and asserts the profile
  table's primary key `REFERENCES` the profiled token's table. A companion guard pins that every
  profile hangs off the same parent today, so a profile of something else is a review moment
  (M58 ticket 5).
  Rejected: a `domain`-discriminated `organization` type (the facets would bind to
  `listOrganizations`, leaving the endpoints that actually serve these rows unfilterable), and new RID
  types for the profiles (a migration re-keying two sidecars and every table referencing
  `institution_id` as an org RID, against D-UnifiedOrgGraph's "a company IS a tenant organization").
- **A facet may name an OPEN value set (`KindCode`).** `audit.action` has a registry behind it
  (`pkg/action`, R-29) but no CHECK constraint, and `target_type` has neither. A `code` facet is
  ranked top-N like a ref and carries NO labels, because its key is its own label — which is also why
  it may not declare a `RefType`.
- **`unit.level` binds `levelMin`/`levelMax`, and the legacy scalar is *superseded*, not removed.**
  Until M57 ticket 3 the facet pinned the pre-existing scalar `level` via `ArgOverride` and the range
  args were "additive and deferred to when the bands are consumed". A dashboard consumes them: a band
  is a RANGE, and no single value expresses one, so the bars were readable and inert. The facet now
  binds the derived pair; the scalar is still shipped and still honoured, because **the contract is
  expand-only** (removing a query arg breaks every stored link and every client that still sends it),
  and the three predicates are **ANDed** so neither silently wins. It is classified `superseded` — a
  fourth non-facet class whose check is earned rather than written: the named successor must exist,
  must be a *range* facet over the *same* column, must not itself bind the arg, and must take the
  same Conjure type.

- **A CONTINUOUS filter is a fifth non-facet class (`ClassWindow`), and it SHIPS on `/stats`.**
  `location`'s radius and bounding box are predicates over the listed table, so they select a subset
  of the same population — unlike a tree walk, which selects a listing MODE and describes nothing
  about the registry. An aggregate that ignored a window would therefore describe a different world
  than the list beside it, with both numbers looking reasonable alone. So the class carries the
  opposite rule to `traversal`: it must appear on the stats endpoint (checked, as for `search`), and
  it is grouped by nothing, because a continuum has no chart order. Its contract type is pinned to
  `double`, which is what stops it becoming a place to hide an arg that could have been bucketed — an
  enum or a RID window is a facet (M58 ticket 6).
- **The aggregate ARMS of a type need not be visibility arms.** `location` has no owner and no
  public/shadow bit, so it has no scoped arm at all — but it ships FOUR aggregate queries, one per
  listing mode, because each mode is a different plan (plain keyset / GiST radius / GiST envelope /
  trigram bitmap) and a nullable spatial predicate is not index-served. `statsparity_test.go`'s
  byte-identity requirement applies unchanged: what the arms differ by is the candidate CTE, never the
  GROUP BY half (M58 ticket 6).
- **A reach trim may need a NARROWER reach than `authz_readable_units`.** Every scoped list before
  `assignment` trimmed with the generic `'%.read'` family, which is right where the endpoint has
  already checked its own read code and reach is only narrowing rows. On the grant table it would
  **widen**: generic read-reach is a strict superset of `assignment.read` reach, and `listAssignments`'
  per-unit arm has always demanded the narrow question, so borrowing the family would have shown
  grants that arm refuses. Migration 0023 adds permission-parameterised siblings of the 0017 trio
  (`authz_readable_units_with` and friends) which compare the code by **equality** — a LIKE-pattern
  argument would make `'%'`, i.e. every permission, expressible by a typo at a call site. The 0017
  trio is untouched: its plans are measured, and the differential test holds the family to one answer
  (M58 ticket 6).
- **A ref facet over a CLOSED, ORDERED catalog is a scale, and topN destroys it (`StrategyCatalog`).**
  Ranking by count is right for an open or large value set and wrong for one carrying a scale:
  `enrollment.degreeLevelId` points at the nine ISCED 2011 levels, and sorted by frequency that chart
  reads "Bachelor, Doctoral, Master". The strategy orders by the referenced catalog's own ordinal,
  emits **every** row including the zero-count ones (on a scale an absent level is information, where
  on a ranking an absent value is merely outside the top N), and has no `(other)` tail — a closed
  catalog has no tail to collapse. It is the ref analogue of `StrategyIdentity` and the same rule
  `KindEnum.Values` has carried since M56, reaching the case where the ordered set lives in a table
  rather than in a CHECK constraint. Like `Profile` and unlike `Ledger` its claim is **CHECKABLE**, so
  that is what replaces an argument: `Register` refuses it off a ref facet or without both
  `CatalogTable` and `CatalogOrder`, and a guard parses the migrations to assert the facet's column
  really FK-references that catalog and that the ordinal is really one of its columns. The zero-count
  buckets come from the module's SQL (a LEFT JOIN over the catalog), because the kernel cannot
  enumerate a catalog it has never read (M58 ticket 7).
- **A table with no RLS policy can still need the request-pinned connection.** The rule "list paths
  read through `db.RequestQuerier`" was written for tables carrying a policy; `enrollment` carries
  none, and its holder read scope probes `membership_memberships`, which does. Unpinned, the `app.*`
  GUCs are unset, every reach predicate matches nothing, and the endpoint answers 200 with an EMPTY
  page to a caller entitled to every row. What decides the rule is therefore not whether the LISTED
  table has a policy but whether any table the query TOUCHES does — the same reading `document` has
  needed since M56, made explicit after this ticket shipped the bug and the live run found it
  (M58 ticket 7).

**AMENDED (M59) — rule 2 has a LIST side, and it is deliberately not symmetric with the stats side.**
Rule 2 above says what a dashboard does with a facet the caller may not read: omit it, never a zeroed
bucket, never a 403. It never said what a LIST does when that same caller *filters* by it, because
until M59 no facet carried a code and the question could not arise. It arises now, and only one of the
three possible answers is honest:

- **Honour it** — the response then answers a question about an attribute the caller may not read.
  Repeated with different values it is an oracle: binary-searching `registrationCountry` recovers each
  vehicle's registration country exactly, which is precisely what `vehicle.registration.read` gates.
- **Ignore it** — the page silently contains rows the caller asked to exclude, and nothing in the
  response says so. A filter that fails OPEN is the dangerous direction, the same reasoning that makes
  an unparseable date bound an error rather than a dropped predicate.
- **Refuse it — 403, naming the code.** It discloses nothing the caller did not already supply, and it
  is the only answer that is true about what happened.

So a gated facet's **filter arg requires its code**; its **distribution is omitted** without it. The
asymmetry is the point: a 403 on `/stats` would leak the same bit the omission hides (the request
merely named a dimension), while a 403 on a filter leaks nothing (the request named a VALUE). The
decision is which code, and that is data: `ObjectType.FilterReadCodes(supplied)` returns the codes a
request needs given the args it actually carried, so a newly gated facet is covered by declaration
alone, and the module's existing PEP produces its own 403.

**AMENDED (M59) — a facet inherits the read code of the SURFACE it reads, not the tier of the column
it names.** Rule 2's mandatory arm keys on the column's `pii:` tier, which is necessary but not
sufficient, and M58 ticket 3 shipped the gap: `vehicle.registrationCountry` groups
`vehicle_registrations.country_id`, a `pii:none` column, so no guard asked for a code — while
`ListRegistrations` and `ListPersonVehicles` both require `vehicle.registration.read` from a base role
of their own. A caller with plain `vehicle.read` could therefore group and filter the fleet by
registration country, deriving one value at a time what those endpoints refuse to return. The tier
answers "how sensitive is this value"; a cross-table facet also discloses a RELATIONSHIP, and the
relationship has its own code (D-LinkPermissions). Both gated facets now inherit one
(`vehicle.registrationCountry` → `vehicle.registration.read`, `account.holderKind` →
`finance.holder.read`), and a guard holds every facet whose `Table` is a separately-gated surface to
carrying a code. Rule 2's stats-side and list-side behaviours are otherwise unchanged — this is about
which facets are *in scope* for them (M59).

**AMENDED (M59) — a dashboard chart may be drawn from ANOTHER module's stats endpoint.** "One endpoint
per module, so a whole dashboard is one round-trip" holds as the shape and stops being an absolute:
one catalogued component was never a distribution of the type whose dashboard it belongs to. The unit
dashboard's headcount-by-unit counts MEMBERSHIPS, so M57 left it unbuilt rather than draw it
dishonestly — the unit dashboard is org-scoped and `membershipStats` shipped no `org` arg, so the
chart would have mixed organizations. `ChartDef.source` names the endpoint, the registry type whose
facets label the buckets and whose LIST the segments link to, and `carry`: an **allowlist** of the host
dashboard's params that travel with the request. An allowlist rather than the whole filter set,
because forwarding a filter the source does not ship would silently widen the chart relative to the
dashboard around it; the build-time guard holds every carried param to being a real arg of BOTH
endpoints, and the chart's facet to being declared by the SOURCE. The cost is one extra request per
such chart, paid only by a dashboard that declares one (M59).

The **filter half** of "counts are computed inside the visibility predicate" is now real: one
`PersonFilter` drives both list paths, folded into the SQL of all five queries, because a Go-side
re-filter after the page is cut returns a short page with a `nextPageToken` (the R-06 failure). The
guards are two-directional and check the *class* of every non-facet arg, so they cannot decay into an
allowlist; `scripts/gen-action-params.sh --verify` keeps the generated mirror from going stale, which
would otherwise let the guards validate an old contract and pass.

**As built (M56 ticket 3 — `membership`, `order`, `document`), amending two points above.**

- **"Listable object type" means object OR reified link.** `membership` is `link__member_of`
  (`KindLink`), not a kind=object type, and it is the first faceted one; M58's `assignment` set is
  `link__has_role`, another. A reified link is a first-class row with its own identity, attributes
  and history ([D-Ontology](#d-ontology--every-entity-is-an-object-a-reified-link-or-an-audited-action)),
  so it lists and filters exactly like an object and there is no reason to exclude it. `pkg/facet`
  accepts object and link **tokens**; **actions remain non-listable** (an action is an audited
  invocation, not a collection), so the check stays a kind check rather than "anything in the
  registry". The token, not the bare name, is what a declaration carries — it is the key the console's
  ontology registry uses — and the token scheme moved into `pkg/rid` (`rid.TokenOf`) so the facet
  catalog and the console registry cannot name the same type differently.
- **A scoped list ships TWO plan shapes, dispatched on reach cardinality.** "Counts are computed
  inside the visibility predicate" is unchanged; *how* the reach is folded is not free, and the
  ticket-3 measurement showed neither form is safe alone. Materializing the reach set makes the
  planner drive from the reach side at large reach — it builds a ~10⁶-row hash and top-N sorts, so
  the `LIMIT` never terminates early (documents: 6 419 ms at 100 000-unit reach). A per-row point
  probe is the mirror image: 3.6–6.3 ms at that reach, 2 500–13 100 ms at reach 1. So the adapters
  dispatch on a capped reach count — precisely the sparse/dense split `VisiblePersonIDsForSubject*`
  has used since R-02.1, which this decision already cited for the person-scoped case and which now
  generalizes to every scoped list. The three reach forms are SQL **functions** defined once in
  migration `0017` (set / point probe / capped count), never inlined per query: their parity with the
  Go PDP oracle is the invariant the differential test exists to protect, and it is now held over
  four implementations at once. Numbers in
  [review-2026-07](review-2026-07.md#m56-ticket-3--top-level-list-endpoints-2026-07-29).
- **A top-level list carries no implicit status default.** `GET /memberships` returns every status,
  unlike the per-unit roster and per-person listing it joins. A hidden active-only filter would make
  M57's `totalCount` disagree with its own status distribution — the two consumers must see one
  world — and would leave ended rows unreachable through any endpoint. The consequence is a
  migration: every pre-existing `membership_memberships` index is partial on `status='active'`, so
  the status-agnostic paths needed keyset indexes (`0017`), making M56 the first ticket in this
  cluster with a non-`➖` `Migrated` gate.

**As built (M57 ticket 1 — the `person` and `unit` stats endpoints).** Four things this decision
specified differently, each settled by implementation or measurement and recorded so tickets 2–4 do
not re-litigate them:

- **The path is `/<module>/v1/stats/<collection>`, not `/<collection>/stats`.** The specified shape is
  **unroutable in this stack**: witchcraft serves on julienschmidt/httprouter, whose radix tree
  refuses a literal segment beside a wildcard at the same position, so `GET /persons/stats` next to
  `GET /persons/{personId}` panics **at registration** — server startup, not a request, and invisible
  to `go build` and every unit test. `/stats/<collection>` also generalizes better for the M58 modules
  that list several collections (`/finance/v1/stats/accounts`, `/stats/cards`). A guard now holds the
  whole contract: `internal/platform/transport/route_conflict_test.go` parses every `api/*.conjure.yml`
  route and fails on a same-method literal/wildcard sibling — keyed by METHOD, because httprouter
  keeps one tree per method (which is why the pre-existing `DELETE /rank-scheme/{level}/{nodeId}`
  beside `POST /rank-scheme/systems` is legal and must not be reported).
- **One static aggregate query per (module, ARM), not per (module, facet).** "Static sqlc `GROUP BY`
  queries, one per (module, facet)" would have meant 7 facets × the arms each module ships — 30-odd
  near-identical 40-line queries for `person` alone, and one round trip per facet. Instead each arm is
  ONE statement: a materialized candidate CTE carrying the list's filter block verbatim, then one
  `UNION ALL` branch per facet, each gated on a `want_<facet>` boolean so an unselected or unreadable
  facet is **skipped by the planner** (a one-time false filter) rather than merely dropped from the
  response. Staticness — the property this decision actually wanted, no dynamic `GROUP BY` builder —
  is unchanged, and the whole dashboard is one scan of one candidate set. The arms exist for plan
  reasons the ticket-3 measurement already established (a nullable trigram predicate is not indexable,
  R-21; the admin arm must carry no visibility predicate at all), and `pkg/facet/statsparity_test.go`
  holds their aggregate halves **byte-identical**, so a facet fixed in one arm cannot be forgotten in
  another.
- **A scoped aggregate needs only ONE plan shape**, unlike a scoped list. The sparse/dense dispatch
  exists because a `LIMIT` cannot terminate early once the planner drives from the reach side; an
  aggregate has no `LIMIT`, so the prediction was that the materialized reach set wins everywhere.
  Measured, at 10^6 persons: set form 8.3 ms / 79.8 ms / 7 144 ms at leaf / mid / root reach against
  the point probe's 12 926 / 17 066 / 24 869 ms. The dispatch — and its ~180 ms capped-count tax — is
  therefore deliberately absent here. Numbers in
  [review-2026-07](review-2026-07.md#m57-ticket-1--the-dashboard-aggregates-2026-07-29).
- **A ref facet declares what its RIDs point at.** `Facet.RefType` (a `pkg/rid` object token) is new,
  because a bucket key is a RID and an axis of RID tails is unreadable; the composition root resolves
  them through the **same D-LinkTraversal labelers** the link engine uses, so a unit is named
  identically in a graph row and in a chart segment, and a boot assertion fails startup for a ref
  facet whose target type has no resolver. `Register` also now refuses a type whose ref facets declare
  different `TopN`, since one query binds one `top_n` across its ref branches.

Bucket assembly (zero-filling an enum in CHART order, banding an age, collapsing a top-N tail,
surfacing NULLs as the mandatory `(unknown)` bucket) lives in the new stdlib-only `pkg/stats`, which
also owns the `facets` CSV selection and rule 2's omission. Its one invariant: **every counted row
lands in exactly one bucket** — an undeclared enum value is appended rather than dropped — so a facet
over the listed table's own column always sums to `totalCount`.

**As built (M57 ticket 2 — `link__member_of`, `order`, `document`), adding two things and correcting one.**

- **The arm convention lives in the kernel, not in five transports.** An empty subject means "no
  visibility predicate" to every stats query, and `pep.SubjectAuthority` returns `("", false)` for a
  MACHINE subject (M51: a principal has no person identity and no reach) — so a non-admin with no
  subject would have been handed whole-instance counts. Ticket 1's tenant transport encoded that
  collapse itself, which is exactly the shape of edit that turns into a leak. `stats.Compute` now owns
  it — pick the arm, run the module's aggregate, assemble, label — and **all five types route through
  it**: a non-admin with no subject reads nothing.
- **The set-vs-probe verdict is re-measured per table, not inherited.** Ticket 1's conclusion was
  reached on `person`, whose reach lands on the membership row; `document` reaches through the HOLDER
  and is the case that broke the set form for lists. Measured, the set form still wins at every reach
  (documents 25.7 / 218 / 4 322 ms against the probe's 12 447 / 15 771 / 23 651 ms), so all five types
  ship one scoped query.
- **A `dateTrunc` facet's `(unknown)` bucket is emitted even when empty**, like every other strategy's:
  an order register with no drafts must still show a zero draft backlog, or the chart changes shape as
  the data does. (Ticket 1 had no `dateTrunc` facet, so the kernel's inconsistency was unreachable.)

The one facet-level cost worth recording: a top-N over a HIGH-CARDINALITY ref column is expensive —
`link__member_of.personId` costs 8.6 s alone at 10^6 distinct persons, because the ranking window sorts
every group to keep fifteen. It is a FILTER facet, not one of the catalog's charts, and the dashboard as
drawn costs 1.3 s admin / 3.2 s root instead of 9.6 / 11.1 — which is what the `facets` CSV is for. The
bounded-top-N alternative is costed in
[review-2026-07](review-2026-07.md#m57-ticket-2--the-membership--order--document-dashboards-2026-07-30)
and deliberately not built.

---

### D-ConsoleDashboards — Every listable type gets a list view and a dashboard view over one URL-borne filter set (amends D-WebUI)

**Decision.** The console's generic explorer (`/explore/[type]`) gains a second view. **List** and
**dashboard** are two renderings of *one* request state, and that state lives **entirely in the URL**
— generalizing the `?view=tree` toggle units already have to `?view=dashboard`, and moving today's
`useState` quick-filter/sort out of `DataTable` into `searchParams`.

**Amended (M57 ticket 3, as built): the dashboard is the DEFAULT view** for every type that declares
one, and `?view=table` is the explicit opt-out. Opening a collection should answer *what is in here*
before *what is on page 1*: the aggregate describes the whole filtered set, while a keyset page
describes fifty rows and cannot be totalled (there is deliberately no `totalCount` on a list
envelope). The default view is the one that clears the `view` param, so a collection's canonical URL
stays paramless and a shared link never carries a redundant `?view=`. Types with no dashboard are
unaffected — their default remains the table, and no existing URL changes meaning. Both are driven from the existing
ontology registry (`web/src/lib/ontology/registry.ts`): `ObjectTypeDef` gains `filters?: FilterDef[]`
and `dashboard?: ChartDef[]`, siblings of the `ColumnDef`/`PropertyDef`/`LinkDef`/`ActionDef` arrays
already there — so a type joins both surfaces by a registry entry, not new pages.

Because the filter set is the URL, three things fall out rather than being built: toggling
list↔dashboard **preserves the filters**; a dashboard segment is an `<a>` to the same URL with one
more filter applied, so **click-to-filter is ordinary navigation**; and a filtered view is
shareable and bookmarkable.

Charts are **composed by hand over `@visx/scale` + `@visx/shape`** — a third focused library, joining
`cmdk` and `@xyflow/react`. `web/src/components/charts/` holds `BarChart`, `DonutChart`, `Histogram`,
`StatTile`, `Sparkline`, rendered as SVG server-side where the data allows.

**Why.** The dashboards must be *the same query* as the list or they mislead: a chart computed from
the first page of a keyset-paginated list is wrong past page 1, and a filter that does not survive the
toggle makes the two views separate tools rather than two lenses. URL-borne state is the smallest
mechanism that gets correctness, click-through and shareability at once, and it fits a
Server-Component console that already reads `searchParams`.

**Why visx** rather than hand-rolled SVG or a chart component library: buckets arrive
pre-aggregated from the stats endpoint, so what is actually needed is scales and shapes, not a chart
framework — visx supplies exactly that and leaves the markup ours, keeping the charts consistent with
the hand-rolled Tailwind primitives and with `GraphExplorer`'s per-type colour convention. A full
component library (recharts) would force every tile into a client island and pull in a layout engine
this does not need.

**Consequence.** **Amends [D-WebUI](#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)**,
whose "no broader component-library lock" stance stands but whose count of focused libraries goes from
two to three. Six modules with no registry entry today — company, vehicle, finance, religion,
education, external-org — get one, so they stop being bespoke `"use client"` pages that fetch a single
page of 100 and drop `nextPageToken`. Every new chrome string needs its four-locale entry in
`messages.ts` and must render through `<T>`/`tg()` (D-i18n). The UI still performs **no** authorization
decision and **no** visibility filtering locally: a facet the caller may not read simply does not come
back from the stats endpoint. Lands as **M56** (filters, list view) and **M57** (dashboards)
([milestones](../milestones.md)); no schema, no contract beyond D-ObjectFacets'.

---

### D-MultiIdPExamples — Public IdPs are supported in two documented topologies; an `oidc` issuer must pin an audience (extends D-JIT, amends D-WebUI)

**Decision.** Signing in with a public provider (Google, GitHub, Microsoft Entra ID, GitLab, Okta) is
supported in **two topologies**, both documented with working config in
[`deploy/oauth/README.md`](../../deploy/oauth/README.md):

- **A — brokered.** Providers federate into Keycloak; **Keycloak remains the sole issuer**, so
  `idp.issuers[]` is unchanged and the token's `sub` is the Keycloak user id. This is the **only**
  topology that supports **GitHub**.
- **B — direct.** Each provider is its own `idp.issuers[]` entry, validated by OIDC discovery + JWKS
  exactly like any other issuer. The console forwards that provider's **ID token**.

Three rules fall out, and all three are enforced, not merely documented:

1. **An `oidc` issuer MUST pin at least one audience; the service refuses to boot otherwise**
   (`GuardIssuerAudience`, alongside the existing symmetric- and reserved-issuer guards).
2. **An issuer accepts a SET of audiences** (`audience:` scalar and/or `audiences:` list, merged), and
   a token validates when its own `aud` **intersects** that set. The scalar spelling is **retained
   rather than replaced** by the list: it is the one that survives
   [D-EnvConfig](#d-envconfig--environment-variables-override-the-yaml-config-and-the-yaml-file-is-optional).
   The env overlay binds only *scalar* fields of a struct-slice element (`elementBindings` collects
   `sub.scalars`), so `OIKUMENEA_IDP_ISSUERS_N_AUDIENCE` exists while `…_AUDIENCES` cannot. An
   env-only deployment can therefore always pin **one** audience — enough to satisfy the boot guard —
   and only the multi-client case additionally requires the YAML file.
3. **The console records per provider which token it forwards** (`web/src/lib/auth/providers.ts`):
   Keycloak's `access_token`, every public IdP's `id_token`. Providers are offered **iff** their
   credentials are present in the environment.

Enrolment is unchanged: a first login on a new provider is an unknown `(issuer, subject)` and is
**rejected** ([D-JIT](#d-jit--just-in-time-provisioning-is-link-on-match-only)); an admin links it via
`POST /accounts/{id}/identities`.

**Why.** The `(issuer, subject)` model and the issuer list were already multi-IdP; what was missing was
that **the two hard parts are not symmetrical between providers**, and both failure modes are silent.

*GitHub is not OIDC.* It issues no ID token, publishes no JWKS, and its access tokens are opaque
`gho_*` strings. There is nothing a relying party can verify, so GitHub cannot be a direct issuer
without the service trusting an unverifiable bearer — which would break
[L-AuthzOnly](#carried-over-locks-settled-earlier-restated-for-self-containment). Brokering is not a
workaround but the correct seam: Keycloak does the OAuth2 dance and issues a real JWT. Rather than
weaken validation for one provider, the topology is documented as the answer.

*A public issuer's `iss` is shared.* `https://accounts.google.com` is the `iss` of every Google OAuth
client on earth, and a Google `sub` identifies the **account**, not the client. Verification therefore
turns entirely on `aud`. The previous code made the audience check *optional* (empty `audience` set
`SkipClientIDCheck`), which was harmless when the only issuers were a private Keycloak realm and a dev
HMAC key, and becomes an **authentication bypass** the moment a public issuer is added: an ID token
minted for any unrelated third-party application would carry an `iss`/`sub` this instance accepts and
resolve straight to the linked person. Fail-closed at boot is the only safe default, and it applies to
every `oidc` issuer because whether an issuer is shared is a property of the deployment, not something
inferable from the URL. The audience **set** exists because one public IdP legitimately serves several
clients of the *same* deployment (the console and a CLI register separately and get different `aud`
values); the alternative — a second issuer entry with the same `iss` — is unrepresentable, since `iss`
is the routing key.

*Which token carries the audience differs by provider.* Keycloak's realm audience mapper puts
`aud: oikumenea` on the **access** token; a public IdP puts its client id on the **ID** token. A
console that forwards the wrong one logs in successfully and then 401s on every API call — a failure
that looks like a permissions bug and is a token-selection bug. Making it a declared per-provider
field turns that into a one-line, reviewable statement.

**Consequence.** **Extends [D-JIT](#d-jit--just-in-time-provisioning-is-link-on-match-only)** (the
reject-unknown default is what makes a multi-provider deployment safe by default — no provider can
enrol itself) and **amends
[D-WebUI](#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)**, whose
single hardcoded Keycloak provider becomes an env-driven registry; the BFF contract (server-side code
exchange, browser never holds a token) is unchanged. **L-AuthzOnly is unchanged** — the service still
validates only, holds no credential, and issues no token.

**Breaking on upgrade:** a deployment with an `oidc` issuer and no `audience` will **fail to boot**
until it pins one. That is deliberate — such a config accepts tokens it should not — and is the one
exception to the non-destructive-upgrade guarantee in this cycle; see
[UPGRADING.md](../../UPGRADING.md).

Enforced by `GuardIssuerAudience` + `audienceAccepted` (`internal/identityfederation/middleware`),
`Issuer.AcceptedAudiences` (`internal/platform/config`), and the console registry; see
[identity-federation](../modules/identity-federation.md) and
[`deploy/oauth/README.md`](../../deploy/oauth/README.md). No schema change.

---

### Planned-tier decisions (M16–M26) → [roadmap-decisions.md](roadmap-decisions.md)

The decisions for the **not-yet-built planned tier** have been **moved out of this binding file**
into [`roadmap-decisions.md`](roadmap-decisions.md) (per the F-008 review finding), so this file
reflects only the built / in-progress surface (M0–M15) — what the code is held to. The moved
decisions remain authoritative for their verticals' design and become binding-against-code as each
milestone enters implementation. In milestone order they are:

- **D-Worker** (M16 — background-job runtime; promotes DS-25)
- **D-DataIngestion** (M17 — generic reference-data ingestion & connector framework)
- **D-Languages** (M18 — Glottolog-faithful language/writing-system registry)
- **D-Location** (M19 — shared standalone Location entity; PostGIS; app-derived MGRS; multi-format input)
- **D-Education** (M20 — institutions, structure & person bindings)
- **D-Companies** (M21 — legal-entity registry + ownership/affiliation graph)
- **D-Religion** (M22–M25 — multi-faith taxonomy, organization graphs & discovery)
- **D-ClergyCredential** (M23 — per-tradition clergy grades + reified credential Link)
- **D-ReligiousAffiliation** (M24 — lay affiliation as a reified `pii:special` Link)
- **D-SpecialPII** (M24 — envelope encryption extended to the `pii:special` tier)
- **D-GeoSubdivisions** (M26 — seeded ISO-3166-2 subnational-division registry)
- **D-Vehicles** (M26 — vehicle registry binding people & companies to vehicles)

## Carried-over locks (settled earlier; restated for self-containment)

These come from the high-level plan and are not re-litigated here.

- **L-AuthzOnly — AuthZ + directory only.** Authentication is delegated to an external IdP.
  go-oikumenea validates inbound identities and decides authorization; it **stores no
  credentials and issues no tokens**. See [identity-federation](../modules/identity-federation.md).
  **Amended by D-Hermenea** ([roadmap-decisions.md](roadmap-decisions.md)): one additional inbound
  auth path exists for the **`hermenea` service principal** — a **runtime shared secret**
  (`HERMENEA_OIKUMENEA_TOKEN`, ECV-refreshable, **never install config, never stored**) validated by
  comparison (the bootstrap-admin pattern) that maps to a principal holding exactly `import.manage`
  (instance scope), audited as a **`system`** actor. This is still "validates an inbound identity,
  stores no credential, issues no token" — the secret is operator-supplied at deploy time, not
  persisted.
- **L-AccountOptional — Person-centric, account optional.** `person` is the core aggregate; an
  `account` is an optional attachment. People who never log in are first-class.
- **L-SingleDomain — ~~Single~~ multi-domain per deployment.** Originally one instance = one domain
  (army OR church OR university), no org-type discriminator in data; `unit_kind` a descriptive label.
  **Refined by [D-Religion](roadmap-decisions.md#d-religion--a-multi-faith-religion-vertical-catalog-driven-taxonomy-organization-graphs--discovery-reverses-the-drafts-religion-drop-refines-l-singledomain):**
  the single domain may be **religion**, within which multiple religions/traditions coexist as
  **catalog data + units in graphs**. **Superseded for the org-structure scope by
  [D-TenantOrganizations](roadmap-decisions.md#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph)
  (M40):** one instance may now hold **multiple domains** (military/government/company/university/
  church/public-org) and multiple **organizations** within each — but the spirit holds: **no org-type
  discriminator is branched on in code.** Domain, organization, and `unit_kind` are descriptive
  **catalog rows** feeding listing/validation/UI, never a code switch and never a PDP input, exactly
  as D-RankSystems refined L-OneRankScheme.
- **L-UnitIsTenant — Tenant ≡ organizational unit.** A "tenant" is a node in the org graph.
  **Refined by [D-TenantOrganizations](roadmap-decisions.md#d-tenantorganizations--domains--organizations-a-multi-domain-tenant-over-the-unit-graph)
  (M40):** units now belong to a first-class **Organization** (the realm a person joins — US Army,
  Bundeswehr, KhNU), itself classified by a **Domain** catalog. The organization is the concrete
  top-level container; units remain the graph nodes within it.
- **L-OneRankScheme — One system-wide rank scheme**, edited by the instance admin, never
  adopted per unit. **Refined by [D-RankSystems](#d-ranksystems--multinational-rank-systems-standardized-grade-comparability-and-scheme-presets-extends-d-rank-refines-l-onerankscheme):**
  the one registry MAY contain multiple `rank_systems` (multinational) — still one scheme,
  instance-admin-managed, never per-unit; not multiple schemes.
- **L-Visibility — Shadow tenants.** `visibility ∈ {public, shadow}` on units. **Enforced (F-002,
  A-lite):** on the unit-result-set reads (`GET /units`, `…/ancestors`, `…/descendants`) a `public`
  unit is broadly discoverable while a `shadow` unit appears only when the subject's `*.read` reaches
  it — applied as the authoritative app-layer gate (`authorization.FilterVisibleUnits`, reached via
  `pep.FilterVisibleUnits`) and mirrored at the DB by a `tenant_units` public-read RLS policy
  (migration `0016`). `GET /units/{id}` stays gated by the per-unit `unit.read` decision, and
  membership/order/person/document reads remain reach-gated — i.e. broad public discovery is a
  **unit-read affordance only**; a public unit is discoverable in listings, but its roster/detail
  still needs reach. Extending public discovery to rosters/people is a deferred seam.
- **L-OperatorDB — Operator-owned Postgres**, schema **`oikumenea`**.
- **L-UpgradeSafe — Non-destructive, data-safe upgrades** are a first-class, tested guarantee.
- **L-Conventions — Schema conventions:** `TIMESTAMPTZ`, soft-delete, `set_updated_at()` triggers,
  `reject_mutation()` append-only guard, `TEXT`+`CHECK` enums. **Amended by D-ResourceIdentifiers** —
  PKs are no longer bare `uuid_v7()` UUIDs but composed URN `TEXT` RIDs via `new_rid()`; `uuid_v7()`
  is retained as the RID's crypto component.

### Explicitly dropped from `drafts/`

~~Religion-specific concepts (denominations, tradition families, the Nicene gate, ROC /
Russian-locale rules)~~ (**superseded by [D-Religion](roadmap-decisions.md#d-religion--a-multi-faith-religion-vertical-catalog-driven-taxonomy-organization-graphs--discovery-reverses-the-drafts-religion-drop-refines-l-singledomain)** —
re-adopted as a **multi-faith**, **catalog-driven** `religion` module: the dropped Christianity-shaped
concepts return generalized to *all* faiths with no hard-coded vocabulary, the Nicene gate replaced by
a generic data-driven org-policy, and the ROC/Russian-locale rules **not** carried over); the org-type
discriminator; per-tenant rank adoption; `content` (pages/blocks/i18n — stays in the consuming app); ~~`location`/PostGIS/geography~~ (**superseded by
[D-Location](roadmap-decisions.md#d-location--a-shared-standalone-location-entity-postgis-app-derived-mgrs-multi-format-input)** —
re-adopted as a shared, standalone `location` module because the army/university analytics scope genuinely
needs queryable geography, unlike the original church-discovery scope; H3 stays dropped — MGRS is derived
app-side, radius search uses PostGIS `ST_DWithin`); `vouching`/web-of-trust; content
`moderation`/policy engine; `integrations`/scrapers; the OAuth **credential vault** (auth is
delegated — we validate, we do not store secrets); `uber/fx`; ~~the Next.js UI (API-only)~~
(**superseded by [D-WebUI](#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)** —
re-adopted as an *optional, standalone* BFF that does not couple into the core, so the original
"don't bake a UI into the service" intent still holds); and all AWS/Supabase/Cloudflare specifics
(self-hostable instead). These appear in the docs only as "dropped" notes.
