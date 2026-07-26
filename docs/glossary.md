# Glossary

The domain vocabulary used across every doc. Module docs assume these terms. Grouped
thematically; alphabetical index at the end.

---

## Organization

**Domain.** The *kind* of organization (D-TenantOrganizations, M40): `military`, `government`,
`company`, `university`, `church`, `public-org`, … A catalog row (stable `code` + translatable
`name`), instance-admin-extensible, **never a `CHECK` enum**. Classifies organizations and units; a
**directory attribute, never a PDP input**. One deployment may hold many domains. Owned by the
[tenant](modules/tenant.md) module.

**Organization (realm).** The concrete top-level entity a person joins — *US Army*, *Bundeswehr*,
*KhNU* (D-TenantOrganizations, M40). Many organizations may share a *domain* (US Army and Bundeswehr
are both `military`, distinct orgs). An organization owns its *units* and its own per-org *graphs*.
The person directory is **instance-global**, so one person can serve in several organizations over
different periods — *organizations sharing a directory, not isolated realms*. A directory attribute,
never a PDP input. Owned by the [tenant](modules/tenant.md) module.

**Unit.** A node in an *organization*'s graph — the thing this service historically called a
*tenant*. A brigade / regiment / battalion / platoon, or a university / campus / faculty /
department. Belongs to exactly one organization (`org_id`) and carries a per-unit *domain* (mixed-
domain trees allowed). All organizational nodes are units; there is no separate "group" concept.
Owned by the [tenant](modules/tenant.md) module.

**Unit kind.** A **domain-scoped catalog** describing a unit's level
(military→`brigade`/`battalion`; university→`faculty`/`department`) — D-TenantOrganizations (M40),
replacing the former free-text label. `code` unique per domain + translatable `name` + optional
`attr_schema` (validates a unit's `metadata`). **Descriptive data**, not a behavioral discriminator —
the code does not branch on it.

**Unit graph / DAG.** Units relate by parent→child edges. A unit may have **more than one
parent** (a directed acyclic graph), and there may be **more than one root** (units with no
parent). Cycles are forbidden. There is **more than one such graph** — see *Graph*.

**Graph (named hierarchy).** A named DAG over an *organization*'s units (D-Graphs), **per
organization** (D-TenantOrganizations, M40). Each organization is seeded its own `command` (the
structural / administrative authority chain — its default) and `operational` (mission /
task-organization, OPCON-like) graphs; the instance admin can add more. A graph with **no org**
(`org_id` NULL) is **instance-global / cross-org** — used by the religion vertical's taxonomy graphs
that span all faiths. An edge belongs to exactly one graph; the same unit pair may be related
differently in different graphs; per-org closures/PDP isolate per organization. Modelled on
NATO's distinction between **ADCON** (administrative control — `command`) and **OPCON / TACON**
(operational control — `operational`). Owned by the [tenant](modules/tenant.md) module.

**Authority-bearing.** Property of a graph whose `subtree` grants the PDP actually cascades over.
`tenant_graphs.is_authority_bearing` (D-DirectoryGraphs): TRUE = cascades; FALSE =
**directory-only** (edges and closure are maintained for display / association but no PDP cascade,
and `subtree` grants on it are rejected at write time). `command` is locked TRUE; the flag may
flip TRUE→FALSE only when no active `subtree` assignments reference the graph; FALSE→TRUE is
always safe. Models NATO **DIRLAUTH** / coordinating-authority relationships and matrix/affiliation
chains.

**Level.** An optional **ordinal** on a unit for sort/filter (echelon in an army; tier in a
church; depth-class in a university). A **directory attribute only** — like rank, it never feeds
the PDP, and it is independent of a unit's depth in any graph. Distinct from *unit kind* (its
descriptive label).

**Closure.** A maintained transitive-closure table, **per graph** (`graph → ancestor →
descendant`), that lets the PDP answer "is U a descendant of T in graph g?" in one indexed lookup
instead of walking edges. Reflexive `(g, u, u, 0)` self-rows cover only units **participating in
that graph's edges** (an edge-less unit has no closure row in `g`). Maintained incrementally on
edge change; an on-demand **verify**
(drift diff) / **rebuild** (recompute from edges) operation is the integrity backstop
against drift (D-ClosureIntegrity). Drift is also surfaced at runtime by the diagnostic
**`closure-drift`** health reporter — fed by `verify`'s persisted `tenant_closure_status`,
diagnostic-only (never gates readiness; D-ClosureDriftHealth).

**Visibility.** A unit is `public` (discoverable) or `shadow` (private, hidden from
discovery). A person may belong to several units, some public, some shadow. The
[authorization](modules/authorization.md) module gates reads on this.

**Lifecycle state.** A unit's status (`active`, `suspended`, `archived`, …). Transitions
are recorded as append-only events.

---

## People

**Person.** The core aggregate — an individual in the directory. **Instance-global**: one
record per individual for the whole deployment (not per-unit). Exists independently of any
login account and of any unit membership. Carries a canonical `display_name` plus the
**Unicode CLDR Person Names** structured parts (`given`, `given2`, `surname`, …; D-PersonNamesCLDR),
bio fields (`birthdate`, `date_of_death`, `sex`, `country_of_birth`), citizenships and residences.
A `date_of_death` is a **bio attribute, not a lifecycle state** — a deceased person stays an active
directory record (D-PersonBio). Owned by the [person](modules/person.md) module.

**person core / personprofile / personsensitive.** The **three Go modules** the person god-module split
into behind the **one** Conjure `PersonService` (**D-PersonModuleSplit**, review-2026-07 R-09), by data
sensitivity + change cadence: **core** ([person](modules/person.md)) — identity, CLDR names, bio, ranks,
read-scope, merge/purge orchestration; **[personprofile](modules/personprofile.md)** — contacts, social
channels, relationships, addresses, non-encrypted institutional ties; **[personsensitive](modules/personsensitive.md)**
— physical identity/ethnicity, overlays, watchlists + sanctions, encrypted party membership (the whole
`crypto.Cipher` / `pii:special` surface). A **code split only** — one `oikumenea.person_*` schema, one
API contract, and [person.md](modules/person.md) stays the single entity-model owner.

**Name (CLDR).** Person names follow the **Unicode CLDR Person Names** fixed field set; `display_name`
is authoritative and the structured parts are advisory (D-PersonNamesCLDR). There is **no dedicated
patronymic field** — the Slavic по-батькові / отчество lives in `given2`, and formal address ("Тарас
Григорович") is assembled by locale-aware formatting from `given` + `given2`.

**Country.** A seeded ISO-3166-1 alpha-2 entry in the `geo_countries` registry (stable `code` +
translatable `name`), referenced wherever a country appears (country of birth, citizenship, residence,
a paper's issuing country, a personal-code scheme's country). Instance-admin-extensible (D-Geo).

**Citizenship.** A person's nationality in a country, **effective-dated** (`acquired_on`/`lost_on`,
`basis`, `is_primary`); a person may hold **several** (D-Geo). Owned by [person](modules/person.md).

**Residence.** A person's effective-dated residence in a country/region (D-Geo). Owned by
[person](modules/person.md).

**Email (contact).** A person's email address — multi-valued, `pii:contact`, catalog-typed `kind`
(`personal`/`work`/…), with a derived `provider` and an `is_primary` flag (D-PersonContactChannels).
**Distinct from the login email** on an [account](modules/identity-federation.md). Owned by
[person](modules/person.md); erased on purge.

**Phone (contact).** A person's phone number — multi-valued, `pii:contact`, stored **E.164-normalized**
with a **derived country**, catalog-typed `kind`, `is_primary` (D-PersonContactChannels). Carrier/
provider is not stored (DS-40). Owned by [person](modules/person.md); erased on purge.

**Call sign (позивний).** An informal radio/identifier label on a person — multi-valued, `pii:basic`,
a required value, unique per person among active rows, `is_primary` (D-PersonContactChannels). Owned by
[person](modules/person.md); erased on purge.

**Email type / phone type.** Instance-admin-managed catalogs (stable `code` + translatable `name`)
naming the `kind` of a contact email/phone (D-PersonContactChannels, D-Code/D-i18n). Owned by
[person](modules/person.md).

**Platform.** Instance-admin catalog (stable `code` + translatable `name`, `category ∈
messenger|social`) of the messengers / social networks a person may appear on (D-PersonSocialChannels).
Owned by [person](modules/person.md).

**Messenger link.** A reachability annotation attaching an existing contact **phone or email** (XOR) to
a `messenger`-category platform — "this number is on Telegram" (D-PersonSocialChannels). Owned by
[person](modules/person.md); erased on purge.

**Social account.** A person's standalone account on a social/messenger platform, independent of any
phone/email (D-PersonSocialChannels). Keys on the platform's **immutable `platform_user_id`** (the
mutable `@handle` has its own rename **history**), records **profile** fields (`pii:contact`),
distinguishes **platform verification** (blue-check) from **operator verification**, and carries
**attribution provenance** — `source` (self_declared / operator_verified / imported) + `confidence`
(confirmed / probable / possible) — so a claimed account is a sourced, weighted assertion. **No**
social-graph metrics; free-text `bio`/location wait on DS-29. Owned by [person](modules/person.md);
erased on purge.

**Person↔person relationship.** A reified self-link between two **in-directory** persons
(D-PersonRelationships), each per-type and mirroring the membership temporal-link shape:
**partnership** (marriage/engagement, symmetric, ≤1 active per person), **kinship** (directional
`parent_of`, siblings derived), **guardianship** (guardian→ward), **sponsorship** (godparent / academic
advisor / military mentor), **next-of-kin** (an in-directory nomination, not a blood fact), and
**association** (associate / conflict-of-interest / no-contact). A **social link** (friend/follower) was
scoped but **deferred — not built** (no consumer / no source / redundant with association; see
decisions.md D-PersonRelationships). Authority **never** derives from a relationship — directory data
only. Owned by [person](modules/person.md); erased when **either** endpoint purges.

**Relation type.** Instance-admin catalog (stable `code` + translatable `name` + `category`) for the
open-ended person↔person relation labels — sponsorship / association / next-of-kin kinds
(D-PersonRelationships, D-Code/D-i18n). Owned by [person](modules/person.md).

**Account.** An *optional* login attachment to a person — at most one per person. People
without accounts (rosters, personnel who never sign in) are first-class. The account is the
person's **set of login points**: it may hold several external identities (e.g. Google +
Keycloak), and the person resolves to the same PDP context regardless of which IdP issued
the inbound token. Whether additional login points may be linked is operator-gated by the
`account.identity_linking.enabled` install config. Owned by
[identity-federation](modules/identity-federation.md).

**External identity.** A verified `(issuer, subject)` pair from an external IdP, linked to
an account — **one link per login point**. The basis on which an inbound token is mapped to a
person.

**Membership.** A `person ↔ unit` assignment — the org-belonging join. One person may hold
many memberships across many units (public and shadow). Optionally fills a position; carries
effective dates. Owned by the [membership](modules/membership.md) module.

**Position.** A **unit-owned billet** — a post belonging to one unit (e.g. *Commander*,
*Deputy*, *Dean*, *Chaplain*) that **exists whether or not anyone fills it**. Has a stable
`code`, a translatable title, and an optional required rank. A person fills it via a membership
that references it. Distinct from rank: rank is the person's standing across the whole org;
position is what they *do* in a specific unit. Owned by [membership](modules/membership.md).

**Vacancy.** A derived state: an active position with **no** active membership filling it. Not
a stored column — the closure of "active position, unfilled".

**Acting authority.** Temporary authority held while a substantive holder is absent, modeled as a
**time-bound role assignment** (`expires_at`) on the unit — *not* a position fill or a rank change
(D-TimeBoundGrants; [patterns.md](architecture/patterns.md), *Acting authority via time-bound role
assignment*). The substantive holder's membership/position is untouched; the grant lapses silently
on expiry.

**Dual-hatting.** One person carrying authority over two units (or two roles) at once — two
concurrent live role assignments; the union-across-graphs PDP sums them. Not a second position.

**Secondment.** Temporary authority on a **host** unit while the person's **home-unit** membership
persists — a bounded role assignment on the host, not a transfer or a new billet.

---

## Documents & orders

**Document.** A **paper** a **person holds** — passport, national ID card, driver's licence, military
ID. Attached to exactly one person; stores metadata only (number, issuer, issuing country, validity),
never binaries. Owned by the [document](modules/document.md) module. Distinct from a *personal code*
(an encrypted government identifier) and from an *order* (an act, not a possession).

**Document type.** The **instance-admin-managed catalog** entry naming a kind of **paper** (stable
`code` + translatable `name`), e.g. `passport`, `driver-license`. Like the rank scheme / locale
registry, it is reference data, not a code-defined enum. A type may carry an *attribute schema*.

**Document attribute schema.** An optional per-document-type declaration (`attr_schema` JSONB) of the
fields a document's `attributes` may/must carry (name → type/required/enum), validated on every
document write (D-DocumentAttrSchema). Used e.g. by `military-id` for VOS/fitness/mobilization fields;
when absent, `attributes` is free-form. Owned by the [document](modules/document.md) module.

**Personal code.** A government-issued **national identifier** a person holds — tax number (РНОКПП),
national ID (УНЗР), SSN, social-/health-insurance number. Belongs to a *personal-code scheme*; its
`value` is **`pii:sensitive`** and **envelope-encrypted at rest** (D-PersonalCodes / D-CryptoProvider).
Owned by the [document](modules/document.md) module.

**Personal-code scheme.** The **country-namespaced** catalog entry for a national-identifier kind
(stable `code` like `ua-rnokpp`, a `country_iso`, a semantic `generic_category` such as `tax-id`, an
optional `validation_regex`, translatable `name`). The code's country **derives from the scheme**.
Cross-scheme queries ("all tax IDs") join on `generic_category` (D-PersonalCodes).

**Order.** An administrative act (наказ) — the **legal basis** for a change in a person's status:
arrival, appointment, leave, transfer, discipline, duty roster. Issued by a unit; has a number, a
date, a `draft → issued → revoked` lifecycle, and one or more *order items*. Owned by the
[order](modules/order.md) module. On **issue**, its structural effects are **auto-applied** by
membership/person subscribers in the issue transaction, via domain events + provenance (D-OrderApply).

**Order type.** The **instance-admin-managed catalog** entry naming a kind of order (stable `code` +
translatable `name`), carrying an *order category* and an **effect** (`membership-start` /
`membership-end` / `rank-change` / `record-only`) that declares the downstream consequence.

**Order category.** One of the five Ukrainian-army "стройова частина" families an order type belongs
to: `personnel-list`, `appointment`, `leave-travel`, `discipline-incentive`, `duty-roster`.

**Order item.** One affected person/action within an order — the unit of effect. Targets exactly one
person (+ optional unit/position/rank per the type). Structural items (membership/rank) are cited as
**provenance** by the resulting change; `record-only` items (leave, trip, discipline, duty) are
authoritative as themselves.

---

## Rank

**Rank scheme.** The single, **system-wide** seniority ladder for the deployment, edited by
the instance admin (never adopted per-unit). Ordered levels:
**rank system → rank category → rank type → rank**. The one scheme may hold **several rank systems**
(multinational; D-RankSystems). Owned by the [rank](modules/rank.md) module.

**Rank system.** Top of the scheme: a national/organizational rank ladder (e.g. `ua-armed-forces`,
`us-armed-forces`, `nato`), optionally tied to a `country`. Lets one directory carry US and Ukrainian
ranks at once (D-RankSystems). Ordered.

**Rank category.** A branch within a rank system (e.g. `army`, `navy`, `marines`). Ordered.

**Rank type.** A grouping within a category (e.g. `officers`, `warrant`, `enlisted`).
Ordered, expresses the broad seniority band.

**Rank.** A specific grade (e.g. `private`, `sergeant`, `colonel`). Ordered, expresses exact
seniority **within a system**. A person holds at most **one rank per rank system** (the reified
`HOLDS_RANK` link / `person_ranks`); a single-system deployment thus holds at most one, a multi-track
one (university/church) may hold concurrent standings (D-Rank).

**Standardized grade (NATO STANAG 2116).** A locale-agnostic grade code (`OF-1`…`OF-10`, `OR-1`…`OR-9`,
warrant) optionally attached to a rank, drawn from the seeded `rank_grades` catalog. It is the
**cross-system comparator**: ranks with the same grade are *equivalent* (US `OF-5` ≈ UA `OF-5`) and grade
`tier`/`ordinal` gives cross-system seniority; absent grade ⇒ incomparable across systems (D-RankSystems).

**Rank preset.** A curated, opt-in template document for one rank-system subtree
(`system → categories → types → ranks`), bundled in-repo and applied by an idempotent, code-keyed
`POST /rank-scheme/import` so admins don't hand-build the ladder (D-RankSystems).

**Rank ≠ permission.** Rank is a **directory attribute** describing seniority. It grants no
authorization whatsoever. Authority comes only from role assignments.

---

## Authorization

**Atomic permission.** A code-defined, runtime-immutable permission string
(e.g. `unit.update`, `person.read`, `rank.scheme.manage`). The closed vocabulary lives in
code; adding one is a code change. Owned by [authorization](modules/authorization.md).

**Role.** A named, composed set of atomic permissions. **Base roles** are platform-defined;
**custom roles** are instance-defined. A role does not, by itself, target anything — it is
bound to a unit and scope by an assignment.

**Role assignment.** The tuple `(subject_person, role, target_unit, scope, graph)` with
provenance and optional expiry. The unit of authorization grant. `graph` names the hierarchy a
`subtree` grant cascades over (NULL for `unit` scope).

**Scope.** A property of a role assignment, one of:
- `unit` — the role's permissions apply **only at `target_unit`**. Descendants get
  **nothing — not even read**. Graph-independent.
- `subtree` — the role's permissions apply at `target_unit` **and all its descendants in the
  assignment's `graph`** (cascading across that one DAG; default `command`).

The `target_unit` is **independent of where the subject sits**: a low-placed person can hold
a `subtree` role on a high-level unit.

**PDP (Policy Decision Point).** The component that answers "may person P perform action A
on unit U?" It unions instance-admin permissions, `unit`-scoped grants at U, and
`subtree`-scoped grants on any ancestor of U **in each grant's own graph** (via that graph's
closure) — so authority unions **across graphs** (e.g. `command` + `operational`) — then applies
the shadow-visibility gate on reads. The request question is graph-agnostic; the graph lives on
the assignment. No per-permission filtering within an assignment; no cross-request caching
(a revoke takes effect immediately).

**Instance admin.** A holder of an **instance-level** authority scope, distinct from unit
role assignments. Manages the rank scheme, role definitions, supported locales & translations,
and global config. The "top-permission role" — bootstrapped at install (D-Bootstrap). An instance
admin is a `person` holding this plane; **"super admin" is colloquial for the same** — there is no
separate super-admin entity (D-Audit, OQ-1).

**Effective permissions.** The union of all permissions a person holds on a given unit,
computed by the PDP at decision time.

**Visibility scope.** The **one** shared row-level read-trim interface
(`ReadableIDs(subject, candidates) → readable subset`; **D-VisibilityScope**, review-2026-09
R‑30) with exactly three canonical shapes matching the system's real policies: **person scope**
(the D-PersonReadScope membership semi-join), **unit scope** (owning-unit mapping + the shadow
gate), **catalog scope** (the read-permission gate is the whole decision, no row trim). Every
object type on a cross-type surface (unified search, generic links) registers one at composition
— an unregistered type fails boot, never serves untrimmed. Additive: per-module endpoints keep
their own code paths; the contract is differential equality with them.

**Unified search.** The one cross-type `searchObjects` endpoint
([search](modules/search.md), **D-UnifiedSearch**, review-2026-09 R‑26): a **fan-in over the
per-module trigram search queries** (no global index), each provider skipped unless the subject
holds its read permission and trimmed through its visibility scope. Hits are `{rid, objectType,
label, snippet}` — the RID is self-describing, so one response shape serves every type.

---

## Localization (i18n)

**Code.** A **stable, locale-agnostic** machine identifier on a structural/catalog entity
(role, position, rank node, locale; optional on person). What external systems reference
in their own code. Operator-assigned, unique, immutable by convention. The permission string is
the degenerate case (it *is* the code). Distinct from the translatable `name`. **Exception
(D-UnitCodeLifecycle, M28):** a **unit** `code` is **optional** (a codeless unit is a non-separate
sub-unit) and **mutable** — a correctable human-readable ID; for units the stable handle external
systems reference is the **RID**, not the code.

**Color (catalog).** A structural, operator-managed color in a **per-domain palette** —
`eye` / `hair` / `vehicle` — owned by the [platform](modules/platform.md) module
(`platform_colors`, the platform module's first RID-bearing Object). Each color has a stable
`code`, a translatable `name`, and an optional `hex` swatch (nullable: biological
eye/hair colors are categories, not precise hex). Referenced by **hard FK** from
`vehicle_vehicles.color_id` and `person_physical_descriptions.eye_color_id`/`hair_color_id`; the
referencing module validates the color's domain in the application layer (a single-column FK can't
constrain the palette). Replaces the prior advisory free-text color fields (D-Color, M42).

**Ethnicity (catalog).** The `person_ethnicity_types` **hierarchical** reference catalog (parent +
closure, like the language forest) of ethnic groups, owned by the [person](modules/person.md) module —
plaintext reference data with a stable `code`, i18n `name`, optional `wikidata_id`, and **group-level**
M:N links to languages (Glottolog languoids — an *ethnolinguistic* association) and homeland
countries. **Default seed is empty** (ethnicity is contentious); an operator loads it on purpose via the
opt-in **CIA World Factbook** `ethnicity-scheme` import — fetched + parsed **live at runtime** by a
`factbook` hermenea connector + mapper (public domain; no committed preset) — a flat catalog of group names +
their homeland countries; the Factbook carries no hierarchy or language ties, so those schema features stay
unpopulated by this source. A person's **declared** ethnicity is a separate,
**envelope-encrypted** `pii:special` link (`person_ethnicities`); the catalog's group↔language tie is
**never** inferred onto a person — declared ≠ inferred (D-PhysicalIdentity, M31/M43).

**Locale.** A supported language for the deployment, identified by an ISO 639-3 code (e.g.
`ukr`, `eng`). The set is **instance-admin-managed** (seeded with `ukr` + `eng`, more can be
added). Owned by the [localization](modules/localization.md) module.

**Supported language.** Synonym for an enabled locale.

**Translation.** A localized value of a translatable field (`name`/`title`/`description`) of
some entity, stored in the [localization](modules/localization.md) translation store and
managed by the instance admin. Translatable fields are returned in every response as a
`locale → text` map (no Accept-Language negotiation).

**Transliteration.** A per-person alternate **name variant** for a locale/script (e.g. "Тарас" /
"Taras"). Person-managed data on the person record — *not* the instance-admin translation store.

---

## Cross-cutting

**Ontology.** The binding way the domain is modeled (D-Ontology): every persisted entity is an
**Object**, a **Link**, or an **Action**. The catalog of types lives in
[ontology-mapping.md](ontology-mapping.md); each module doc classifies its entities by these kinds.

**Object (type).** A thing with identity over time (`Unit`, `Person`, `Position`, `Order`, `Role`, …),
keyed by an RID whose `<entity_type>` slot is the Object type. The Palantir-ontology counterpart of a
domain aggregate/entity (D-Ontology).

**Link (type).** A relationship modeled as a first-class row (its own RID, `link__<type>`) when it
carries identity, attributes, or history — `HAS_ROLE`/role assignment, `MEMBER_OF`, `PARENT_OF`,
`HOLDS_RANK`. A relationship with none of those stays a plain FK column (D-Ontology). See *Link RID*.

**Action (type).** A named, audited mutation (`IssueOrder`, `GrantAssignment`, `CreateUnit`, …); the
[audit](modules/audit.md) row recording it is keyed by its Action RID (D-Ontology). See *Action RID*.

**RID (Resource Identifier).** The packed, self-describing **UUIDv8** that is every entity's primary
key (D-ResourceIdentifiers, amended F-014): a native `uuid` whose bits encode *app · service · kind ·
type · timestamp · random*, minted by `new_id(service,kind,type)` and decoded by the `rid_*` SQL
functions / `pkg/rid`. Its human, decomposable form is rendered from the bytes as
`oikumenea:<service>:<kind>:<type>:<uuid>`, never stored. It is the **machine resource handle**; the
entity's **code** stays the stable *business* key. Distinct kinds for Objects, Links, Actions — below.

**Link RID.** A RID whose packed **kind** is link, rendered with a `link__<link_type>` type token
(e.g. `link__has_role`), marking the row as a first-class relationship. Time-bounded Links carry
`valid_from`/`valid_to`; validity is never encoded in the RID itself (RIDs are immutable).

**Action RID.** A RID whose packed **kind** is action; its specific action *name* (e.g. `issue_order`)
lives in `audit_log.action`, not the RID. Each [audit](modules/audit.md) row is keyed by the Action RID
of the write it records — the audit log is the action ledger.

**Action type / action-type catalog (D-ActionTypes).** The machine catalog (`pkg/action`) of every
named audited write — each `{code, service, targetType, permission}` keyed by the dotted
`audit_log.action` code. It is the read-time contract the free-text action name lacked: audit writes
are validated against it, the [ontology-mapping §3.1](ontology-mapping.md) table is generated from it
and coherence-checked, and `AuditService.listActionTypes` serves it. Expand-only; the Action RID
stays generic kind=action/type 0.

**Service / type codes.** The numeric `service` (per module) and per-service `type` codes packed into
every RID, held in the seeded `platform_rid_services` / `platform_rid_types` registries and mirrored in
`pkg/rid`. The owning module is the RID's `service`; environment is no longer encoded (the URN
`<environment>` segment was dropped — it was constant per database, L-SingleDomain).

**Audit log.** An append-only record of permission-sensitive actions, correlated by
`request_id`. Never updated or deleted (guarded by a `reject_mutation()` trigger). Owned by
[audit](modules/audit.md).

**History tier (D-Temporal).** The classification every reified Link declares at design time:
**(a) native validity** — the default for relationship/state Links, which carry `valid_from`/`valid_to`
(or grandfathered `effective_from/to` · `granted_at/revoked_at` · `founded_on` · `awarded_on`);
**(b) object history** — read back from the audit ledger via `getObjectHistory`, no per-row interval;
**(c) history-exempt** — a reference/structural association deliberately undated (a closed six-member
set). Enforced by a build-time drift guard; the boundary R-31 made the ontology declare instead of
re-deciding ad hoc per milestone.

**Object history / `getObjectHistory`.** The reverse-chronological, token-paginated read of one
object's audited changes (audit rows with `target_id = rid`), gated by `audit.read` with `before`/`after`
change payloads **redacted unless the caller holds the sensitive-reader capability** (D-Temporal, R-31).

**Append-only / immutable event log.** A table whose rows are never updated or deleted;
current state is derived or kept in a separate mutable overlay. Used for audit and unit
lifecycle events.

**Reversibility.** Destructive actions are soft (a `deleted_at` / state flip with a grace
window), never immediate hard removal; a reversal is itself an audited action.

**Dormant seam.** A column or table shipped but intentionally unused (always NULL / empty),
reserved so a future capability is *additive*, not a rewrite (e.g. the password/2FA columns
reserved for a future full-IdP pivot).

**Expand / contract.** The migration discipline: a release only **adds**; removals happen in
a later, announced release after the old shape is unused. Guarantees data-safe upgrades.

**PII tier.** A column's data-sensitivity classification, recorded as `COMMENT ON COLUMN ... IS
'pii:<tier>'` (D-PIITiers): `pii:none` / `pii:basic` (identifying) / `pii:contact` (locator) /
`pii:sensitive` (national-identifier-class) / `pii:special` (GDPR Art. 9 special-category). JSONB
grab-bags are tagged at their ceiling (`pii:special`); secrets (e.g. `password_hash`) are marked
`secret`, a separate axis. `pii:sensitive` is the **"envelope-encrypt at rest" marker**
(D-CryptoProvider). See [conventions.md](architecture/conventions.md).

**Envelope encryption.** The at-rest protection for `pii:sensitive` values (D-CryptoProvider): the
value is **ciphertext in the DB**, encrypted by a per-record **data key (DEK)** that is itself
**wrapped by a key (KEK) held in an external KMS** and never in the DB. Erasure is **crypto-erase**
(destroy the wrapped DEK). The KMS backend (AWS KMS / GCP KMS / HashiCorp Vault / Azure Key Vault /
local-dev) is install config behind a pluggable **`KeyProvider`** seam.

**Blind index.** A keyed HMAC of a normalized sensitive value, stored alongside the ciphertext so the
value can be matched for **equality lookup / uniqueness without decryption** (D-CryptoProvider).

**RLS backstop.** The defense-in-depth Postgres Row-Level Security layer (D-RLSDefenseInDepth) that
mirrors the PDP-computed read/write reach via per-transaction `app.*` session GUCs. A backstop
behind the **authoritative** PDP + shadow gate (it guards the forgotten-filter bug class), **not**
the authorization model — which remains app-layer (D-NoRLS).

**PDP context.** The resolved `(person, [account])` plus request metadata that the transport
layer derives from a validated inbound token and passes to the PDP and to audit.

**Gate.** One step in the feature pipeline (idea → decided → designed → backend → migrated → ui →
verified); each gate has an **exit artifact** that proves it is passed (a `D-<Name>` block, a module
doc, an `internal/` module, a `migrations/` file, a `web/` page). Defined in
[development-process.md](development-process.md).

**Stage board.** The single scannable table in [milestones.md](milestones.md#stage-board) — one row
per `M#`, one column per *gate* — that is **authoritative for where a milestone sits**; the
per-milestone prose carries the detail. Every `✅` is grounded in a real artifact, never memory.

**TODO-N.** A raw, not-yet-weighed feature idea in `todo.md`, `## TODO-N · Title [status:
idea]`. It is not on the stage board; once promoted to a milestone it is marked `promoted→M#` and
then deleted. `todo.md` may legitimately not exist when nothing is pending. The parked-seam
counterpart for *known* future seams is **DS-N** in [open-questions.md](open-questions.md).

## Planned domains (M16–M54)

> Vocabulary for the [milestones.md](milestones.md) M16–M45 planned cluster (derived from the
> superbrain/OSINT draft + the original `todo.md`) and the **M51–M54 north-star topology cluster**
> ([north-star.md](architecture/north-star.md)). Designed and decision-backed
> ([roadmap-decisions.md](architecture/roadmap-decisions.md)); full module docs follow at
> implementation time (the **religion** vertical, the shared [location](modules/location.md), and the
> new [external-organizations](modules/external-organizations.md) docs already exist).

**Hermenea.** The **companion service** (a second binary, `cmd/hermenea`) that performs reference-data
ingestion + the background-job runtime **out of process**, with its **own PostgreSQL**, coupled to
oikumenea **only over HTTP** (D-Hermenea, M16 — supersedes D-Worker, folds D-DataIngestion/M17). It
fetches → stages → maps → **loads** datasets by calling oikumenea's `POST /import/{objectType}`. The
name pairs *hermenea* (interpretation) with *oikumenea*. See [hermenea](modules/hermenea.md).

**Pinax.** The **reference plane** (D-Pinax, M45) — the instance-global, read-mostly **world-model**
catalogs (`platform_colors`, `geo_countries`, `language_languoids` + `writing_systems`, `rank_*`,
`religion_taxa`, `person_ethnicity_types` + its links), grouped as a **naming convention** (not a new
RID service, not a separate schema/DB) distinct from the operational core and from the small structural
type/kind catalogs (which stay migration-seeded). Governed by one seeding contract: **bundled YAML seed
presets** `go:embed`-ed into oikumenea and self-**autoseeded** at boot (`pinax.autoseed`, default on)
through the same application import service the HTTP `/import` wraps — **create-if-absent /
fill-if-empty / never-delete**, version-gated via `pinax_seed_state`. A *pinax* (πίναξ) was an ancient
catalogue/register (Callimachus' *Pinakes*). See the [pinax plane note](architecture/pinax.md).

**`origin` (seeded / operator).** The plane-wide marker (D-Pinax) on every seeded reference table —
`origin ∈ {seeded, operator}`, default `operator`. The boot seeder writes `seeded`; ordinary API
inserts default `operator`. Re-seeding never touches an `operator` row (protects operator-added
catalog entries), and an explicit `oikumenea seed --reconcile` updates `seeded` rows only.

**Seed preset (bundled).** A versioned YAML reference dataset shipped inside the oikumenea binary
(languages, writing-systems + wiring, countries-enrichment, religions, ethnicities, ranks, colors),
with a manifest (`source` / `source_version` / `license` / `depends_on` / translation provenance).
The "seed" end of the one import pipeline — a `bundled_file` source, versus a remote hermenea
**connector** for the massive `geo_places` set (D-Pinax).

**Background worker (hermenea).** The cron **scheduler** + `worker_jobs` **queue** inside hermenea
(D-Hermenea, was the in-process D-Worker): at-least-once with idempotency keys, exponential backoff
(per-job-type), `FOR UPDATE SKIP LOCKED` claim, graceful drain, a job-health reporter. Promotes the
long-parked DS-25. Runs in hermenea's DB, not oikumenea's.

**Data ingestion / connector.** The generic reference-data pipeline (D-Hermenea, M16; pipeline shape
from D-DataIngestion): a **connector** (`http`/`file`) fetches an external source → **raw staging** →
a per-object-type **mapper** transforms it to a **canonical envelope** → oikumenea applies a
**code-keyed, idempotent, non-destructive upsert** with `import_runs` **lineage** + per-row provenance.
The connectors/mapper/scheduler live in **hermenea**; the upsert endpoint lives in oikumenea. The
right-sized analog of Palantir Foundry's Data Connection → Pipeline → Ontology mapping.

**Canonical envelope.** The interchange document hermenea POSTs to oikumenea's import endpoint:
`{ objectType, source, sourceVersion, license, generatedAt, records[] }`.

**Service principal.** A **machine subject** in the PDP — a registered non-person caller (a facade
with standing of its own, a connector) holding its own role assignments, audited as a **`system`**
actor attributable to the specific principal (D-ServiceIdentities, M51 — amends L-AuthzOnly). In the
target state a principal authenticates via the external IdP's **client-credentials** flow and is
resolved by the same OIDC/JWKS middleware as humans (`(issuer, subject) → principal`, admin-managed).
The original form — **`hermenea-importer`**, a **runtime shared secret** (`HERMENEA_OIKUMENEA_TOKEN`,
never stored) mapping to a principal holding exactly `import.manage` (D-Hermenea, M16) — survives as
the minimal-install fallback.

**North star.** The agreed **target-state topology** ([north-star.md](architecture/north-star.md),
2026-07-18): oikumenea as the **headless internal core** (the brain — persons, units, roles,
relationships, the PDP) behind unprivileged **facades**, fed by the **connector plane**, tailored by
**data packs**. Realized by D-HeadlessTopology / D-ServiceIdentities / D-ConnectorPlane / D-DataPacks
(M51–M54). A destination the stage board converges on, not a status claim.

**Facade.** A per-audience **BFF service** in front of the headless core (D-HeadlessTopology, M52) —
the admin console behind **console-bff**, a future HR app behind its own facade. Owns the browser
session and response shaping, speaks the Conjure API via the generated SDKs, and is **unprivileged**:
it always forwards the **end-user's IdP token** (no on-behalf-of), so the PDP decides against the
real user and a compromised facade can impersonate nobody. A facade is a *constraint*, not a
particular process boundary: **console-bff** is realized as the admin console's own Next.js server
tier rather than a separate binary (D-HeadlessTopology, M52 amendment).

**Connector plane.** The generalization of hermenea into a **family of connectors**
(D-ConnectorPlane, M53): each connector keeps its own storage + scheduler, couples over HTTP only,
and interacts in up to three modes — **push** (bulk import envelopes), **pull-wiring** (reads on the
wiring API), **on-demand lookup** (core→connector synchronous calls with a deadline, the generalized
M34 watchlist seam). oikumenea holds a **registry** (`Connector`/`Source` Objects + audited sync-run
reporting) for **visibility, not orchestration**.

**Wiring API.** The narrow, permission-gated **read surface for connectors** (D-ConnectorPlane, M53):
natural-key → RID resolution, reference-catalog reads, a connector's own sync cursors. Each surface
is its own permission code granted to a service principal — what a connector may see is a grant,
never a default.

**Data pack.** A **versioned bundle of seedable content** — a locale pack, a pinax-style world-model
preset, a catalog, a rank scheme — mounted by the operator and loaded by the boot autoseeder under
the D-Pinax invariants (create-if-absent / fill-if-empty / never-delete, version-gated). The unit of
deployment tailoring (D-DataPacks, M54): plugins are **data, not code** — no runtime code loading.
Paired with **per-module enable flags** (`modules.*.enabled`): a disabled vertical hides its API
surface but its schema still migrates.

**Languoid.** A node in the Glottolog genealogical forest (D-Languages, M18): `level ∈ family | language
| dialect`, keyed by its **glottocode**, optional ISO 639-3, with an AES endangerment **status**. The
recursive `language_languoids` table; a person `SPEAKS` a `language`-level languoid (with CEFR + native).

**Writing system.** An ISO 15924 script (D-Languages, M18) a languoid is `WRITTEN_IN`; classified by a
**script type** (logographic/syllabary/alphabet/abjad/abugida/featural).

**Location.** A shared, standalone place (D-Location, M19): a **required** WGS84 coordinate (PostGIS
`GEOGRAPHY` point) with an **app-derived MGRS** and the original input preserved in `source_coordinate`,
plus a structured postal address over the country registry. The coordinate may be supplied in several
formats (lat/lon, MGRS, UTM, СК-42); radius/bbox search uses PostGIS `ST_DWithin`. Education
buildings/dorms and company addresses reference it by FK.

**Educational institution.** An external reference org (D-Education, M20) — kindergarten…academy — with a
recursive internal **structure tree** (campus/faculty/department/chair), **buildings** (Locations), and
person bindings: **enrollment** (`STUDIED_AT`), **mentorship** (reuses M14 sponsorship with an education
context), **dorm stay**, and institution **positions**. Distinct from the deploying org's tenant units.

**Education reference layer.** The M20 extension (D-Education *Extension*) adding reference-grade facts:
**program** (degree offering), **course** (module), **curriculum version** + **curriculum item** (a
versioned course list), **course prerequisite** (cycle-guarded), **research centre/group**, **grant**,
**publication**, **governance body**, **policy**, **qualification** (a credential type; an awarded
**diploma** is a qualification award + a `diploma` document), **scholarship**, **accreditation event** —
plus person↔reference links (authorship, research/governance membership, grant holding, qualification &
scholarship awards). Reference data only — **not** an operational SIS (no terms/sections/grades).

**Company (legal entity).** A registered organization (D-Companies, M21): `legal_form` + orthogonal
`ownership_category`, multi-scheme **registration** (LEI spine), industry classification, positions, and
the **ownership/affiliation graph** — founders, shareholders (stake %), and **beneficial owner (UBO)**.

**Beneficial owner (UBO).** The ultimate person behind a company through ownership layers; stored as a
**declared** `BENEFICIARY_OF` link (computed chain-traversal is the parked DS-47).

**Registration scheme.** A catalog of company-identifier kinds (LEI ISO-17442 global spine, DUNS, UA
EDRPOU, VAT, US EIN), each with an optional validator regex — mirrors the person `PersonalCodeScheme`. A
**Registration** is one company's identifier under a scheme, flagged `validated` against that regex.

**Ownership/affiliation graph.** The company link cluster: **founding** (founder is a person or a
company), **shareholding** (`OWNS_STAKE`, polymorphic holder, stake %; company-holder edges form the
ownership DAG — a **subsidiary** is a company a parent holds stake in), **succession** (`SUCCEEDED_BY`
M&A/reorganization lineage), and **branch** (`BRANCH_OF`, a non-independent sub-unit). Legal form +
`ownership_category` are two orthogonal axes (a private LLC vs a state-owned JSC).

### Religion (M22–M25)

> The **multi-faith** religion vertical (D-Religion, M22–M25). **All faith vocabulary is catalog-driven**
> — every term below names a *catalog* seeded with cross-faith examples, never a hard-coded enum. Owned
> by the [religion](modules/religion.md) module (organizations reuse [tenant](modules/tenant.md) units;
> sites reuse [location](modules/location.md)).

**Taxon (faith taxonomy).** A node in the **recursive** faith taxonomy (D-Religion **refined**, M22) — a
`religion_taxa` row with a `parent_id` (NULL = a root religion) and a maintained `religion_taxa_closure`.
Each carries a catalog-driven **rank** (level marker) and an optional `wikidata_id`. Replaces the former
fixed `religion_religions`/`religion_tradition_families`/`religion_sub_traditions` catalogs.

**Taxon rank.** The ordered structural level a taxon sits at (a `religion_taxon_ranks` catalog row):
**religion** → **branch** → **tradition** → **sub-tradition** → **denomination**. Structural, not faith
vocabulary; a faith need not use every level (the closure carries true depth). E.g. *Christianity*
(religion) → *Eastern Orthodoxy* (branch) → *Orthodox Church of Ukraine* (denomination).

**Religion-type / theism classification.** A `religion_classifications` catalog row
(monotheistic/polytheistic/nontheistic/…) tagged M:N onto taxa; resolves **nearest-declared-wins** down
the closure, and a unit may **override** its inherited type. A faith may carry several (Hinduism =
monotheistic + polytheistic + monistic).

**Religious organization / worship community.** A religious body (denomination, jurisdiction, community,
mosque, monastery, …) modeled as a **tenant unit** placed in the religion graphs; its faith attributes
live in an **org profile** and its eligibility rules in a data-driven **org policy**. **Org kind** is the
catalog naming each organizational level per faith (a descriptive `unit_kind`, never branched on).

**Canonical / tradition / affiliation graph.** The three seeded religion **unit graphs**: `canonical`
(governance tree — **authority-bearing**, the PDP cascades here), `tradition` (taxonomic placement —
**directory-only**), `affiliation` (voluntary association DAG — **directory-only**, no admin inheritance).

**Clergy grade.** A per-tradition, ordered religious-functionary rank (bishop/presbyter/deacon;
imam/mufti/sheikh; rabbi/cantor; bhikkhu/lama; pujari/swami) — a `religion_clergy_grades` catalog;
`ordinal` orders only *within* a tradition (no cross-tradition comparator, DS-43). **Clergy credential**
is the reified `CLERGY_CREDENTIAL` link recording a person's standing (ordination/investiture/
recognition) — indelible where sacramental, and **never an authz input** (parallels rank, D-Rank).
**Clergy office** (pastor / imam-of-mosque / head-rabbi / abbot) is a [membership](modules/membership.md)
**position** with authority from a role assignment.

**Religious affiliation.** A person's lay belief tie to a religion/tradition/community — the reified
`AFFILIATED_WITH` link, **GDPR Art. 9 `pii:special`**, envelope-encrypted + blind-indexed (D-SpecialPII),
crypto-erased on purge. **Affiliation type** (adherent/member; catechumen/baptized/confirmed; shahada;
bar/bat-mitzvah) is a per-tradition catalog.

**Religious site.** A worship place — the reified `SITE_OF` link from an organization unit to a shared
**Location**, typed by a per-tradition **site type** (church/mosque/synagogue/temple/gurdwara/…), with
its own `visibility` and `public_precision`. **Public precision** is the privacy projection that coarsens
a published coordinate to an H3 cell (`exact`→point … `city`→~9 km … `hidden`→none) at read time — the
persecuted-community case. **Service schedule** is a site's recurring service (day/RRULE, time, IANA
timezone, service language, **service type**, mode). **Alias** is a search-only alternative name.

### Vehicle (M26)

**Plate region.** The structured home for a vehicle's registration region is the WOF **`geo_places`**
gazetteer (placetype=region, D-GeoPlaces, built in M16) — **not** a separate `geo_subdivisions` registry
(D-GeoSubdivisions was superseded; never built). A vehicle registration's `subdivision_id` is a
`geo_places` region RID, app-validated on write.

**Vehicle.** A physical vehicle (D-Vehicles, M26): a `vin` (unique among active), `manufacture_date`,
`color`, and long-tail `attributes`, typed by a **vehicle type** (a shallow taxonomy tree) and a
**vehicle model**. A **vehicle brand** is the marque, linked to its manufacturer **Company** by the
temporal `MANUFACTURED_BY` link.

**Vehicle registration.** The ownership+plate record — the reified temporal `REGISTERED_TO` link from a
**Vehicle** to a **polymorphic owner** (person **or** company), carrying the registration country,
the **plate region** (a `geo_places` region RID), plate `registration_number`, and **registration number type**
(regular/temporary/transit/…). Re-registration is a new row (the prior closed), so the link *is* the
ownership history; person-owned rows are `pii:basic`, holder-scoped, and purge-erased.

### OSINT person-intelligence enrichment (M29–M37)

> The [draft_superbrain_schema.md](draft_superbrain_schema.md) verdicts are the binding source; the
> cluster decisions are D-OverlayFoundation … D-LoginSecurityLog
> ([roadmap-decisions.md](architecture/roadmap-decisions.md)).

**Provisional person.** A minimal-PII `person_persons` row with `status=provisional` (D-OverlayFoundation,
M29) — a stub so every relationship/overlay edge points at a real node (an unresolved external person,
an emergency contact, a wallet-attribution target). Promoted/merged into a canonical person by the
audited **`MergePerson`** action (carries a `confidence`, re-homes edges). No automatic dedup.

**Attribution (`source`/`confidence`).** The reusable overlay column-set (D-OverlayFoundation) —
`source ∈ {self_declared, operator_verified, imported}`, `confidence ∈ {confirmed, probable, possible}`,
optional `as_of` — stamped on every overlay/attribution row. **Declared values and inferred values live
in separate column-spaces and are never merged.**

**Legal basis.** The structured `platform_legal_basis_kinds` catalog (GDPR Art. 6 lawful bases + Art. 9
conditions, M29), FK'd (NOT NULL) by every `pii:special` overlay, plus an optional free-text
justification — the queryable, enforceable lawful-processing record.

**External organization.** A party / government body / foreign-military formation / NGO / lobbying
registrant in the dedicated `external_organizations` registry (D-ExternalOrgs, M30, **RID service 18**) —
the node-space M33 institutional ties point at when the org is neither an operator **Unit** nor an M21
**Company**. Catalog-typed (`external_org_kinds`), provisional/resolved, hermenea-fed.

**Overlay.** A provenance-tagged store carrying `source`+`confidence` (vs an authoritative
operator-asserted attribute) — crypto wallets, inferred political leaning, regulatory sanctions,
external references. Distinguished from **DEVELOP** attributes (physical description, declared
ethnicity) which are first-party.

**Watchlist match.** The only persistable residue of a **live-lookup** sanctions/PEP/Interpol check
(D-Watchlists, M34): `on_list`/`lists[]`/`program`/`match_score`/`last_checked` — the lists themselves
are **never stored**, queried at request time **through hermenea** (≤24h cache). **PEP** status derives
from a held **government position** (M33).

**Special-category overlay.** A `pii:special` (GDPR Art. 9/Art. 10) person store — declared ethnicity
(M31), party membership (M33), inferred political leaning (M35), health records (M36), criminal/arrest/
court records (M38) — **envelope-encrypted** (D-SpecialPII) + `legal_basis` + full audit + (for health
and legal records) app-layer need-to-know; **never inferred** except the explicitly-isolated
political-leaning spectrum.

**Legal record.** A category-level criminal/arrest/court record (D-LegalRecords, M38, GDPR Art. 10):
`kind` (criminal_conviction/arrest/court_judgment) + a **mandatory `disposition`** (arrest ≠ guilt) +
envelope-encrypted coarse offence detail (no full charge sheet). Sealed/expunged records are
**suppressed** — retained but hidden behind `person.legal-record.read-suppressed`. Distinct from the
order module's internal `discipline-incentive` items (external judicial facts, not org discipline).

**Login security log.** First-party `account_login_events` (D-LoginSecurityLog, M37; account Object
`9,1,4`, `pii:contact`) — ip/context/resolved-country/vpn/tor recorded by the OIDC/JWKS validation
middleware per validated human request, **deduped** to one row per `(account, context, ip)` per window
(a bump, not a firehose). **Not** OSINT enrichment and **not** stored credentials (L-AuthzOnly holds).
Read on `account.security-log.read`; purge-erased + retention-swept.

**Finance module.** The planned `finance` module (D-Finance, M44 — RID service 19) holding **bank
accounts and payment cards** as authoritative, encrypted-at-rest directory data. Authoritative
first-party (unlike the M35 financial overlay), and a directory of accounts only — no balance/
transaction ledger. See [finance](modules/finance.md).

**Bank / financial institution.** A bank is **not** a bespoke entity — it is an existing
`company`-domain [tenant](modules/tenant.md) *Organization* (M21/M41) an account references as its
holding institution (D-Finance, M44).

**Bank account.** `finance_accounts` (D-Finance, M44): an account held at a *bank* by a person or
company; its **IBAN** is `pii:sensitive`, **envelope-encrypted** + blind-indexed (the
`document_personal_codes` pattern); carries `currency` (ISO 4217) + an *account type*.

**Account holder.** The `finance_account_holders` reified Link (`link__held_by`, M44): a **polymorphic**
person|company holder of a *bank account* with a `role` (`primary`/`joint`/`authorized_signer`),
effective-dated — the edge that expresses "person → bank account" and supports joint/corporate accounts.

**Payment card.** `finance_cards` (D-Finance, M44): a debit/credit card hanging off a *bank account*;
the full **PAN** is `pii:sensitive`, **envelope-encrypted** + blind-indexed, with a clear **BIN**
(first 6) + **last-4** for display. **CVV2/CVC2 is never stored** (PCI-DSS Req 3.2). Storing the PAN
brings the deployment into **PCI-DSS cardholder-data scope**.

---

## Alphabetical index

Account · Account holder · Action (type) · Action RID · Affiliation type · Append-only event log · Atomic permission · Audit log · Authority-bearing · Background worker · Bank / financial institution · Bank account · Beneficial owner (UBO) · Blind index ·
Call sign · Canonical envelope · Canonical graph · Citizenship · Clergy credential · Clergy grade · Clergy office · Closure · Code · Company (legal entity) · Connector plane · Country · Data ingestion / connector · Data pack · Document · Document attribute schema · Document type · Dormant seam ·
Education reference layer · Educational institution · Effective permissions · Email (contact) · Email type · Envelope encryption · Environment slot · Expand/contract · External identity · Facade · Finance module ·
Gate · Graph (named hierarchy) · Hermenea · History tier · Instance admin · Languoid · Level · Link (type) · Link RID · Locale · Location · Membership · Name (CLDR) ·
North star · Object (type) · Object history · Object-set · Ontology · Order ·
Order category · Order item · Order type · Org kind · Payment card · PDP · PDP context · Person · person core / personprofile / personsensitive · Personal code ·
Personal-code scheme · Phone (contact) · Phone type · PII tier · Pinax (reference plane) · Position · `origin` (seeded/operator) · Public precision · Public/shadow · Rank · Rank category · Rank preset ·
Rank scheme · Rank system · Rank type · Religion (faith) · Religious affiliation · Religious organization · Religious site · Residence · Reversibility · RID (Resource Identifier) · RLS backstop · Role · Role assignment · Scope ·
Service principal · Service schedule · Service type · Site type · Stage board · Standardized grade (NATO STANAG 2116) · Sub-tradition · Subdivision · Supported language · TODO-N · Tradition family · Translation · Transliteration · Unified search · Unit · Unit graph (DAG) · Unit kind ·
Vacancy · Vehicle · Vehicle brand · Vehicle registration · Visibility · Visibility scope · Wiring API · Writing system
