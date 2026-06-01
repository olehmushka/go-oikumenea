# Glossary

The domain vocabulary used across every doc. Module docs assume these terms. Grouped
thematically; alphabetical index at the end.

---

## Organization

**Unit.** A node in the organization graph — the thing this service calls a *tenant*. A
brigade / regiment / battalion / platoon, or a university / campus / faculty / department.
All organizational entities are units; there is no separate "group" concept. Owned by the
[tenant](modules/tenant.md) module.

**Unit kind.** An optional, instance-configured label on a unit describing its level
(e.g. `brigade`, `battalion`, or `university`, `department`). It is **descriptive data**,
not a behavioral discriminator — the code does not branch on it. Distinct from drafts'
`tenant_type`, which is dropped.

**Unit graph / DAG.** Units relate by parent→child edges. A unit may have **more than one
parent** (a directed acyclic graph), and there may be **more than one root** (units with no
parent). Cycles are forbidden. There is **more than one such graph** — see *Graph*.

**Graph (named hierarchy).** A named DAG over the units (D-Graphs). The deployment ships
`command` (the structural / administrative authority chain — the default) and `operational`
(mission / task-organization, OPCON-like); the instance admin can add more. An edge belongs to
exactly one graph; the same unit pair may be related differently in different graphs. Modelled on
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
instead of walking edges. Maintained incrementally on edge change; an on-demand **verify**
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
bio fields (`birthdate`, `sex`, `country_of_birth`), citizenships and residences. Owned by the
[person](modules/person.md) module.

**Name (CLDR).** Person names follow the **Unicode CLDR Person Names** fixed field set; `display_name`
is authoritative and the structured parts are advisory (D-PersonNamesCLDR). There is **no dedicated
patronymic field** — the Slavic по-батькові / отчество lives in `given2`, and formal address ("Олег
Володимирович") is assembled by locale-aware formatting from `given` + `given2`.

**Country.** A seeded ISO-3166-1 alpha-2 entry in the `geo_countries` registry (stable `code` +
translatable `name`), referenced wherever a country appears (country of birth, citizenship, residence,
a paper's issuing country, a personal-code scheme's country). Instance-admin-extensible (D-Geo).

**Citizenship.** A person's nationality in a country, **effective-dated** (`acquired_on`/`lost_on`,
`basis`, `is_primary`); a person may hold **several** (D-Geo). Owned by [person](modules/person.md).

**Residence.** A person's effective-dated residence in a country/region (D-Geo). Owned by
[person](modules/person.md).

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

---

## Documents & orders

**Document.** A **paper** a **person holds** — passport, national ID card, driver's licence, military
ID. Attached to exactly one person; stores metadata only (number, issuer, issuing country, validity),
never binaries. Owned by the [document](modules/document.md) module. Distinct from a *personal code*
(an encrypted government identifier) and from an *order* (an act, not a possession).

**Document type.** The **instance-admin-managed catalog** entry naming a kind of **paper** (stable
`code` + translatable `name`), e.g. `passport`, `driver-license`. Like the rank scheme / locale
registry, it is reference data, not a code-defined enum.

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
the instance admin (never adopted per-unit). Three ordered levels:
**rank category → rank type → rank**. Owned by the [rank](modules/rank.md) module.

**Rank category.** Top of the scheme (e.g. `army`, `navy`, `marines`). Ordered.

**Rank type.** A grouping within a category (e.g. `officers`, `warrant`, `enlisted`).
Ordered, expresses the broad seniority band.

**Rank.** A specific grade (e.g. `private`, `sergeant`, `colonel`). Ordered, expresses exact
seniority. A person holds **one** rank.

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

---

## Localization (i18n)

**Code.** A **stable, locale-agnostic** machine identifier on a structural/catalog entity
(unit, role, position, rank node, locale; optional on person). What external systems reference
in their own code. Operator-assigned, unique, immutable by convention. The permission string is
the degenerate case (it *is* the code). Distinct from the translatable `name`.

**Locale.** A supported language for the deployment, identified by an ISO 639-3 code (e.g.
`ukr`, `eng`). The set is **instance-admin-managed** (seeded with `ukr` + `eng`, more can be
added). Owned by the [localization](modules/localization.md) module.

**Supported language.** Synonym for an enabled locale.

**Translation.** A localized value of a translatable field (`name`/`title`/`description`) of
some entity, stored in the [localization](modules/localization.md) translation store and
managed by the instance admin. Translatable fields are returned in every response as a
`locale → text` map (no Accept-Language negotiation).

**Transliteration.** A per-person alternate **name variant** for a locale/script (e.g. "Олег" /
"Oleh"). Person-managed data on the person record — *not* the instance-admin translation store.

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

**RID (Resource Identifier).** The composed, self-describing URN that is every entity's primary key
(D-ResourceIdentifiers): `urn:oikumenea:<service>:<environment>:<entity_type>:<uuid>`, generated by
`new_rid()` with a `uuid_v7()` crypto component. It is the **machine resource handle**; the entity's
**code** stays the stable *business* key. Distinct slots for Objects, Links, and Actions — see below.

**Link RID.** An RID whose `<entity_type>` slot is prefixed `link__<link_type>` (e.g.
`link__has_role`), marking the row as a first-class relationship. Time-bounded Links additionally carry
`valid_from`/`valid_to`; validity is never encoded in the RID itself (RIDs are immutable).

**Action RID.** An RID whose slot is prefixed `action__<action_type>` (e.g. `action__issue_order`).
Each [audit](modules/audit.md) row is keyed by the Action RID of the write it records — the audit log
is the action ledger.

**Environment slot.** The `<environment>` URN segment (`prod`/`staging`/`dev`/`local`), sourced from
install config (`app.environment`). Constant per self-hosted database (L-SingleDomain); maps to
Palantir's stack/branch concept.

**Object-set (seam).** A reserved RID form `urn:oikumenea:<service>:<env>:object-set:<uuid>` for
named/saved collections of objects — a future capability, not yet specified.

**Audit log.** An append-only record of permission-sensitive actions, correlated by
`request_id`. Never updated or deleted (guarded by a `reject_mutation()` trigger). Owned by
[audit](modules/audit.md).

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

---

## Alphabetical index

Account · Action (type) · Action RID · Append-only event log · Atomic permission · Audit log · Authority-bearing · Blind index ·
Citizenship · Closure · Code · Country · Document · Document type · Dormant seam ·
Effective permissions · Envelope encryption · Environment slot · Expand/contract · External identity ·
Graph (named hierarchy) · Instance admin · Level · Link (type) · Link RID · Locale · Membership · Name (CLDR) ·
Object (type) · Object-set · Ontology · Order ·
Order category · Order item · Order type · PDP · PDP context · Person · Personal code ·
Personal-code scheme · PII tier · Position · Public/shadow · Rank · Rank category · Rank scheme ·
Rank type · Residence · Reversibility · RID (Resource Identifier) · RLS backstop · Role · Role assignment · Scope ·
Supported language · Translation · Transliteration · Unit · Unit graph (DAG) · Unit kind ·
Vacancy · Visibility
