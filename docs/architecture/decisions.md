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
**Amended by D-Graphs** — edges are *typed* (each belongs to one named graph) and the closure
is maintained **per graph**; cycle prevention is per graph.

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
  recomputes only K's rows in the same transaction.
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
[authorization](../modules/authorization.md).

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

### D-Audit — Every write is audited; audit reads are permission-scoped

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
exemption** (OQ-5).

**Why.** Mirrors the graduated, namespace-scoped Kubernetes defaults (`view`/`edit`/`admin`, with
`cluster-admin` ≡ the instance plane) — a natural fit for `unit`/`subtree` scoping. Explicit reads +
a dedicated `auditor` keep the model uniform (everything is a granted permission) and serve
least-privilege / separation-of-duties. Inline localized labels are server-assembled, so gating the
reference reads breaks no normal rendering.

**Consequence.** See [authorization](../modules/authorization.md) (base-role enumeration, the
`rank.scheme.read` catalog addition, the `/authorize` permission). Base roles are immutable by
instance admins (`is_base`).

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

JSONB grab-bags (`person.attributes`, `audit.before`/`after`) are tagged at their **ceiling**
(`pii:special`) with a governance note: special-category data must **not** land there without
the envelope-encryption seam (**DS-29** for audit; a person-side equivalent). **Secrets**
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
See [conventions.md](conventions.md).

### D-PersonReadScope — A person's read scope projects through its memberships

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

---

### D-ResourceIdentifiers — Composed URN RIDs as primary keys (Objects, Links, Actions)

**Decision.** Every Object, Link, and Action is identified by a **composed, self-describing URN**
that **is its primary key** (Palantir-style resource identifiers). The grammar is fixed:

```
urn:oikumenea:<service>:<environment>:<entity_type>:<uuid>
```

- `urn` — literal scheme token; `oikumenea` — the company/app constant.
- `<service>` — the owning module, from the table-prefix vocabulary: `tenant`, `person`,
  `membership`, `document`, `order`, `rank`, `authz`, `account`, `i18n`, `audit`, `platform`.
- `<environment>` — deployment env (`prod`|`staging`|`dev`|`local`), from install config via
  `current_setting('app.environment')`. **Constant per database** for a self-hosted instance
  (L-SingleDomain), so all RIDs in a DB share the segment and FK joins are unaffected.
- `<entity_type>` — for an **Object**, its type (`unit`, `person`, `role-assignment`, …); for a
  **Link**, `link__<link_type>` (e.g. `link__has_role`, `link__parent_of`); for an **Action**,
  `action__<action_type>` (e.g. `action__issue_order`).
- `<uuid>` — a `uuid_v7()` (time-ordered) crypto component, lowercase canonical form.

PKs become `id TEXT PRIMARY KEY DEFAULT oikumenea.new_rid('<service>','<entity_type>')`; `new_rid()`
composes the URN from `uuid_v7()`. Foreign keys follow the PK type (`TEXT`). A per-table shape
`CHECK` (e.g. `id LIKE 'urn:oikumenea:tenant:%:unit:%'`) is the cheap structural guard.

- **Temporal Links** never encode validity in the RID (RIDs are immutable). A time-bounded Link
  carries `valid_from`/`valid_to` (NULL `valid_to` = currently active); the existing temporal columns
  *are* this mapping — membership `effective_from`/`effective_to` and assignment
  `granted_at`/`revoked_at`(+`expires_at`).
- **Action RID = the natural key of the `audit_log` row** that records it: the audit log is the action
  ledger keyed by action RID (D-Audit).
- **Object-set / resource-ref (reserved seam):** `urn:oikumenea:<service>:<env>:object-set:<uuid>` for
  named/saved object collections; the full RID is the portable cross-system reference handle.
- **Scope.** Tables keyed by `id` adopt the RID. Pre-existing **natural-key catalog tables**
  (`geo_countries.code`, `document_personal_code_schemes.code`, `i18n_locales.code`) and
  **composite-key** join/closure tables keep their keys (D-Geo / D-PersonalCodes / D-Code unchanged).

**Why.** Self-describing, globally-addressable keys are how Palantir organizes resources: an
identifier states its app, service, environment, and type without a lookup, makes Links and Actions
first-class addressable resources, and gives the audit ledger a stable action handle. B-tree ordering
on the URN preserves insert locality — only the trailing `uuid_v7()` varies within a
`(service,env,entity_type)` prefix, so append behavior matches the old uuid PK.

**Consequence. Reverses/supersedes L-Conventions' `uuid_v7()` PK** — PKs are now URN `TEXT` via
`new_rid()`; `uuid_v7()` is **retained** as the crypto component. PKs/FKs widen from `UUID` (16 B) to
`TEXT` (~70 B): larger indexes/joins, accepted for self-describing addressability. The
**D-RLSDefenseInDepth** GUC arrays change type (`app.readable_units`/`writable_units` `uuid[]` →
`text[]`; policy casts `::uuid[]` → `::text[]`). Coexists with **D-Code**: the RID is the *machine
resource handle*; `code` stays the stable, locale-agnostic *business* key. See
[conventions.md](conventions.md) (Resource identifiers) and
[ontology-mapping.md](../ontology-mapping.md).

---

### D-RIDSeeding — RID-keyed seed rows are seeded at boot, not in migrations

**Decision.** Reference rows whose PK is an RID (`new_rid('<svc>','<type>')`) are seeded **by the
application at startup**, idempotently, **not** by the Atlas migration that creates their table.
`new_rid()` reads the per-connection `app.environment` GUC (D-ResourceIdentifiers), which only the
application pool sets (`db.NewPool`'s `AfterConnect`); **Atlas's migration connection does not set
it**, so any migration that *inserts* a defaulted-RID row fails with `unrecognized configuration
parameter "app.environment"`. The seed therefore runs in the owning module's `Register(...)` on the
GUC-bearing pool, via `INSERT … ON CONFLICT (<natural-code>) … DO NOTHING` so it is safe on every
boot (and after an operator changes a seeded row, e.g. promotes a different default). Migrations
that seed reference data stay restricted to **natural-key** tables (`geo_countries`, `i18n_locales`
— the D-ResourceIdentifiers carve-out).

First applied by **tenant** (M3): the `command` (default, undeletable, locked authority-bearing) and
`operational` graphs are RID-keyed Objects, seeded in `tenant.Register`. It is the **precedent** for
the M7 base-role seeds (D-BaseRoles) and the M8 first-admin bootstrap (D-Bootstrap), which are
likewise RID-keyed and application-seeded.

**Why.** Only the application knows the deployment environment (from install config), so only it can
mint RIDs with the correct `<environment>` segment; baking a fallback environment into `new_rid()`
would stamp wrong-env RIDs when Atlas applies migrations in prod. Boot-time idempotent seeding keeps
RIDs correct, keeps migrations pure DDL + natural-key seeds, and needs no provisioning-time
`ALTER DATABASE … SET app.environment`.

**Consequence.** A migration's table-create and its seed rows are no longer co-located for RID-keyed
tables; the seed lives in module code and must be idempotent. Boot does one extra `INSERT … ON
CONFLICT` per seeded registry (negligible). See [conventions.md](conventions.md) (Resource
identifiers), [tenant](../modules/tenant.md), and [platform](../modules/platform.md).

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
- **L-Conventions — Schema conventions:** `TIMESTAMPTZ`, soft-delete, `set_updated_at()` triggers,
  `reject_mutation()` append-only guard, `TEXT`+`CHECK` enums. **Amended by D-ResourceIdentifiers** —
  PKs are no longer bare `uuid_v7()` UUIDs but composed URN `TEXT` RIDs via `new_rid()`; `uuid_v7()`
  is retained as the RID's crypto component.

### Explicitly dropped from `drafts/`

Religion-specific concepts (denominations, tradition families, the Nicene gate, ROC /
Russian-locale rules); the org-type discriminator; per-tenant rank adoption; `content`
(pages/blocks/i18n); `location`/PostGIS/H3/geography; `vouching`/web-of-trust; content
`moderation`/policy engine; `integrations`/scrapers; the OAuth **credential vault** (auth is
delegated — we validate, we do not store secrets); `uber/fx`; the Next.js UI (API-only); and
all AWS/Supabase/Cloudflare specifics (self-hostable instead). These appear in the docs only as
"dropped" notes.
