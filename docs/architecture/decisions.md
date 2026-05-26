# Decisions

The binding architectural decisions. **If code and a decision here disagree, the code is
wrong.** Each entry: the decision, why, and the consequence. Two groups: decisions
**resolved this session**, and **carried-over locks** (settled earlier, restated here so this
file is self-contained).

Format note: these are intentionally lightweight ADR entries, not the full `drafts/`
ADR ceremony. They will be the seed for a `docs/decisions/` ADR set if the project later
wants per-file ADRs.

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

### D-Rank — Rank on person; rank ≠ permission

**Decision.** A `person` holds **one rank** drawn from the single system-wide scheme. **Rank is
a directory attribute and grants no authorization** — authority comes only from role
assignments. (Position is covered by D-Position.)

**Why.** Military/academic reality: rank (Sergeant, Professor) is a person's standing across the
whole organization. Coupling it to permissions would make authorization implicit and
unauditable.

**Consequence.** `person.rank_id` → [rank](../modules/rank.md). The PDP never reads rank.

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
roles, manage supported locales & translations, edit global config. Bootstrapped at install.

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

**Consequence.** No `SET LOCAL app.tenant_id` GUC dance, no per-table RLS policies. RLS stays
available as a future hardening seam only. Departs from `drafts/` ADR-0018 §3.6. See
[conventions.md](conventions.md).

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
below one enabled). The translatable `name`/title/description of units, ranks (category/type/
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
label).

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

---

## Carried-over locks (settled earlier; restated for self-containment)

These come from the high-level plan and are not re-litigated here.

- **L-AuthzOnly — AuthZ + directory only.** Authentication is delegated to an external IdP.
  go-oikumenea validates inbound identities and decides authorization; it **stores no
  credentials and issues no tokens**. See [identity-federation](../modules/identity-federation.md).
- **L-AccountOptional — Person-centric, account optional.** `person` is the core aggregate; an
  `account` is an optional attachment. People who never log in are first-class.
- **L-SingleDomain — Single domain per deployment.** One instance = one domain (army OR church
  OR university). **No org-type discriminator** in data; `unit_kind` is a descriptive label
  only.
- **L-UnitIsTenant — Tenant ≡ organizational unit.** A "tenant" is a node in the org graph.
- **L-OneRankScheme — One system-wide rank scheme**, edited by the instance admin, never
  adopted per unit.
- **L-Visibility — Shadow tenants.** `visibility ∈ {public, shadow}` on units.
- **L-OperatorDB — Operator-owned Postgres**, schema **`oikumenea`**.
- **L-UpgradeSafe — Non-destructive, data-safe upgrades** are a first-class, tested guarantee.
- **L-Conventions — Schema conventions:** `uuid_v7()` PKs, `TIMESTAMPTZ`, soft-delete,
  `set_updated_at()` triggers, `reject_mutation()` append-only guard, `TEXT`+`CHECK` enums.

### Explicitly dropped from `drafts/`

Religion-specific concepts (denominations, tradition families, the Nicene gate, ROC /
Russian-locale rules); the org-type discriminator; per-tenant rank adoption; `content`
(pages/blocks/i18n); `location`/PostGIS/H3/geography; `vouching`/web-of-trust; content
`moderation`/policy engine; `integrations`/scrapers; the OAuth **credential vault** (auth is
delegated — we validate, we do not store secrets); `uber/fx`; the Next.js UI (API-only); and
all AWS/Supabase/Cloudflare specifics (self-hostable instead). These appear in the docs only as
"dropped" notes.
