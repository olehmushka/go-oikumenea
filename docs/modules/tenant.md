# Module: tenant

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.tenant_*`

## Purpose

Owns the organization structure as a **two-tier model** (D-TenantOrganizations, M40): a **domain**
catalog (the *kind* of organization — military/government/company/university/church/public-org),
**organizations** (the realm a person joins — *US Army*, *Bundeswehr*, *KhNU*), and within each
organization a **graph of units**. A unit is a node — a brigade/battalion/platoon, or a
university/campus/department. The module stores domains, organizations, units, their parent→child
edges (a **DAG**: multi-parent, multi-root, **per organization**), a maintained transitive
**closure** for fast ancestor/descendant queries, each unit's/org's **visibility** (`public`/
`shadow`), and lifecycle state. One deployment hosts **multiple domains and multiple organizations**
side by side, all sharing the instance-global [person](person.md) directory so a person can serve
across organizations over time. It is the structural foundation the whole service hangs off; the
[authorization](authorization.md) PDP reads its closure to resolve inheritance. It does **not**
decide access itself, and **domain/organization are directory attributes — never PDP inputs**.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Domain`,
`Organization`, `UnitKind`, `Unit`, `Graph`. **Links:** `link__parent_of` (the unit edge, per graph)
and the derived `link__ancestor_of` (the closure). **Actions:** `CreateDomain`, `CreateOrganization`,
`TransitionOrganization`, `CreateUnit`, `TransitionUnit`, `RecodeUnit`, `AddEdge`/`RemoveEdge`, graph
management — each audited and keyed by its `action__<type>` RID.

- **Domain** — an org-kind **catalog** (D-Code/D-i18n): `military`/`government`/`company`/
  `university`/`church`/`public-org`, … Stable `code` + translatable `name`, `status`. Seeded by
  migration `0003_tenant` (RID seeding in migrations is valid post-F-014 — `new_id` reads no GUC),
  instance-admin-extensible, **never a `CHECK` enum**. Classifies organizations and units; a
  **directory attribute, never a PDP input**.
- **Organization** (aggregate root, the realm) — the concrete top-level entity a person joins
  (*US Army*, *Bundeswehr*, *KhNU*). `code` (D-Code), translatable `name`, `domain_id` (its kind),
  `visibility`, lifecycle `state`. Many organizations may share a domain. Owns units and per-org
  graphs. **A directory attribute, never a PDP input** — authority flows only through its graphs.
- **Unit kind** — a **domain-scoped catalog** replacing the former free-text `unit_kind`. Seeded by
  migration `0003_tenant` with a starter set per domain; the `military` set is a **universal echelon
  ladder** any armed force can use — a top governance tier (`ministry-of-defence`), `service-branch`,
  `command`, the joint/ground ladder (`army-group → army → corps → division → brigade → regiment →
  battalion → company → platoon → squad → fire-team`), plus air (`wing`/`air-group`/`air-squadron`/
  `flight`) and naval (`fleet`/`flotilla`/`naval-squadron`) echelons; university→{campus, faculty,
  department, chair}. Arm-specific naming variants (battery/troop = company/platoon) are **not** separate
  kinds — the per-unit `name` carries the actual title. `domain_id`, `code` (unique per domain),
  translatable `name`, optional `attr_schema` (validates a unit's `metadata` per kind).
- **Unit** — a node in an organization's graph: `org_id` (the owning realm), `domain_id` (its kind
  class — per-unit, defaults to the org's domain, **mixed-domain trees allowed**), optional
  `kind_id`, **optional** `code`, translatable `name`, optional ordinal `level`, `visibility`,
  lifecycle `state`, free-form `metadata`. A **codeless** unit is a non-separate sub-unit
  (D-UnitCodeLifecycle, M28).
- **Graph** — a **named hierarchy** over units (D-Graphs), normally **per organization** (`org_id`):
  each org is seeded `command` (the structural / administrative authority chain — the default,
  undeletable) and `operational` (mission / task-organization, OPCON-like). A graph with **`org_id`
  NULL is instance-global / cross-org** — used by the religion vertical's `tradition`/`affiliation`/
  `canonical` taxonomy graphs that span all faiths (D-TenantOrganizations, M40). Instance-admin-managed;
  `code` unique per org (or among global graphs) + translatable `name`. Each graph is independently a
  DAG; per-org closures/PDP isolate per org, while a global graph's closure may legitimately span orgs.
- **Organization lifecycle event** — append-only record of each org state transition (mirrors the
  unit lifecycle event).
- **Unit edge** — a directed `parent → child` relationship **within one graph**. Many per unit
  in either direction (DAG), across graphs.
- **Unit closure** — derived, **per graph**: every `(graph, ancestor, descendant, depth)` pair
  reachable through that graph's edges. Maintained automatically on edge change; not user-edited.
- **Unit lifecycle event** — append-only record of each state transition.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete, `TEXT`+`CHECK` enums)
per [conventions.md](../architecture/conventions.md).

**`tenant_domains`** (org-kind catalog — D-TenantOrganizations, M40; RID `4,1,5`)
- `id` PK · `code TEXT` (unique among active) · `name TEXT` (translatable) ·
  `status TEXT CHECK (status IN ('active','retired'))` · `sort_order` · timestamps + soft-delete.
- **`pdp_scoped BOOLEAN NOT NULL DEFAULT true`** (D-UnifiedOrgGraph, M41): `true` = an **operational**
  domain (military/government/public-org/church) whose units are reach-RLS-scoped and whose orgs auto-seed
  `command`/`operational` graphs; `false` = a **reference** domain (university/company) that is
  instance-global (public reads, app-permission writes, exempt from reach-RLS, no auto graph seed).
  Denormalized onto `tenant_units.pdp_scoped` (derived in SQL at insert) for the RLS predicate.
- Seeded at **boot** (RID-keyed; idempotent `ON CONFLICT (code) DO NOTHING`):
  `military`, `government`, `company`, `university`, `church`, `public-org`. Instance-admin-extensible.

**`tenant_unit_kinds`** (domain-scoped catalog — replaces free-text `unit_kind`; RID `4,1,7`)
- `id` PK · `domain_id` FK → `tenant_domains` · `code TEXT` (**unique per domain**,
  `UNIQUE (domain_id, code) WHERE deleted_at IS NULL`) · `name TEXT` (translatable) ·
  `attr_schema JSONB` (optional JSON schema validating a unit's `metadata` for this kind) ·
  `status` · `sort_order` · timestamps + soft-delete. Seeded at boot per domain.

**`tenant_organizations`** (the realm — D-TenantOrganizations, M40; RID `4,1,6`)
- `id` PK · `code TEXT NOT NULL` (D-Code; unique among active, immutable by convention) ·
  `name TEXT` (translatable) · `domain_id` FK → `tenant_domains` (its kind) ·
  `visibility TEXT CHECK (visibility IN ('public','shadow'))` ·
  `state TEXT CHECK (state IN ('active','suspended','archived'))` · `metadata JSONB` ·
  timestamps + soft-delete. **Not seeded** (deployment-created). Indexed on `(domain_id)`.

**`tenant_org_lifecycle_events`** (append-only org transition ledger; RID `4,1,8`)
- `id` PK · `org_id` FK · `from_state`/`to_state` · `reason` · `actor_person_id` · `request_id` ·
  `created_at`. Guarded by `reject_mutation()`. Mirrors `tenant_unit_lifecycle_events`.

**`tenant_units`**
- `id` PK
- `org_id` FK → `tenant_organizations` **NOT NULL** — the owning realm (M40). Immutable after create.
- `domain_id` FK → `tenant_domains` **NOT NULL** — the unit's kind class; **defaults to the org's
  domain** on create, may differ (**mixed-domain trees allowed**). A directory attribute, never a
  PDP input. Indexed `(org_id)` and `(org_id, domain_id)`.
- `kind_id` FK → `tenant_unit_kinds` (nullable) — the domain-scoped unit kind (replaces free-text
  `unit_kind`); must belong to the unit's domain. Never branched on in code.
- `code TEXT` — **optional**, **mutable** human-readable business identifier
  (D-UnitCodeLifecycle, M28, amending D-Code). `NULL` ⇒ a **non-separate sub-unit** (a line
  battalion/platoon with no independent designation); a value ⇒ a separate unit. Unique among
  active units **that have a code** (`UNIQUE WHERE deleted_at IS NULL AND code IS NOT NULL`).
  Set/corrected/cleared only via the audited recode op (below) — **not** the generic update patch.
  The stable machine handle external systems reference is the **RID**, not the code. (Replaces
  drafts' `slug` — an API-only service has no subdomains.)
- `name TEXT NOT NULL` — default-locale display name; **translatable** via the
  [localization](localization.md) store (returned as a `locale → text` map)
- `level SMALLINT` — optional **ordinal** for sort/filter (echelon in an army:
  team < … < battalion < brigade < corps; tier in a church; depth-class in a university).
  Promoted from `metadata` (DS-1 resolved). **Directory attribute only** — like rank, it is
  **never** an input to the PDP or the shadow gate, and it is **independent of closure depth**
  (a unit's `level` is intrinsic, not its position in any graph). The paired descriptive label
  is `unit_kind`; `level` is its sortable companion.
- `visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','shadow'))`
- `state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active','suspended','archived'))`
- `metadata JSONB NOT NULL DEFAULT '{}'`
- `created_at`, `updated_at`, `deleted_at`

**`tenant_graphs`** (named-hierarchy registry, **per organization** — D-Graphs / D-TenantOrganizations)
- `id` PK
- `org_id` FK → `tenant_organizations` **nullable** — the owning realm; graphs are per-org (M40).
  Each org is seeded its own `command` + `operational` graphs **in the same transaction as the org**
  (replacing the former boot-time *global* seed). **`org_id NULL` = an instance-global / cross-org
  graph** (the religion taxonomy graphs).
- `code TEXT NOT NULL` — **stable, locale-agnostic** identifier referenced by edges/closure/
  assignments (e.g. `command`, `operational`); **unique per org** among active rows
  (`UNIQUE (org_id, code) WHERE deleted_at IS NULL AND org_id IS NOT NULL`) and, separately, unique
  among active **global** graphs (`UNIQUE (code) WHERE deleted_at IS NULL AND org_id IS NULL`);
  immutable by convention
- `name TEXT NOT NULL` — default-locale display name; **translatable** via the
  [localization](localization.md) store
- `is_default BOOLEAN NOT NULL DEFAULT FALSE` — exactly one default **per org** (seeded `command`);
  the default is the graph a `subtree` grant uses when none is named
- `is_authority_bearing BOOLEAN NOT NULL DEFAULT TRUE` — whether the PDP cascades `subtree`
  grants over this graph (D-DirectoryGraphs). `FALSE` = **directory-only**: edges/closure are
  maintained for display/association but no cascade. `command` is **locked to TRUE**
  (`CHECK (code <> 'command' OR is_authority_bearing = TRUE)`).
- `created_at`, `updated_at`, `deleted_at`
- Seeded `command` (default, **undeletable**, authority-bearing) + `operational` (authority-bearing).
  Guards: ≥1 graph always exists; a graph with live edges or active `subtree` assignments cannot
  be deleted; the `is_authority_bearing` flag may flip TRUE→FALSE **only** when the graph has no
  active `subtree` assignments (same-shape guard as deletion); FALSE→TRUE is always safe.

**`tenant_unit_edges`** *(Link `link__parent_of`)*
- `id` PK — RID, `link__parent_of` entity-type slot
- `graph_id TEXT NOT NULL REFERENCES tenant_graphs(id) ON DELETE RESTRICT` — which hierarchy
  this edge belongs to (D-Graphs)
- `parent_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `child_id  TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `created_at`, `created_by` (person id, provenance)
- `UNIQUE (graph_id, parent_id, child_id)`; `CHECK (parent_id <> child_id)` (no self-loop).
  The same `parent → child` pair may exist in more than one graph.
- Cycle prevention enforced **per graph** in the application/SQL on insert (see Invariants).

**`tenant_unit_closure`** (derived; maintained on edge insert/delete, **per graph**)
- `graph_id TEXT NOT NULL REFERENCES tenant_graphs(id)`
- `ancestor_id TEXT NOT NULL`
- `descendant_id TEXT NOT NULL`
- `depth INT NOT NULL` (0 = self-row for each unit **participating in the graph's edges**, per graph)
- `PRIMARY KEY (graph_id, ancestor_id, descendant_id)`; indexed both directions
- Includes a reflexive `(g, u, u, 0)` row for **every unit that participates in graph g's edges**
  (an edge-less unit has no closure row in `g`) so "is U in the subtree of T in graph g" is one
  lookup. An edge change in graph K **incrementally adjusts only the affected rows of K** in the
  same transaction (M48, review R‑04): attach merges `anc*(parent) × desc*(child)` with a
  shortest-depth `LEAST` update; detach deletes and re-derives exactly that slice, then prunes
  orphaned reflexive rows. Per-graph closure maintenance is serialized by a row lock on the
  graph; the full recompute remains only as the D-ClosureIntegrity repair path.

**`tenant_closure_status`** (derived **diagnostic overlay**, one row per graph; not append-only,
not audited — D-ClosureDriftHealth)
- `graph_id TEXT PRIMARY KEY REFERENCES tenant_graphs(id) ON DELETE CASCADE`
- `last_checked_at TIMESTAMPTZ NOT NULL` — when `closure.verify` last ran for this graph
- `missing_count INT NOT NULL`, `extra_count INT NOT NULL` — closure rows the recompute found
  missing / spurious vs. the stored closure
- `in_drift BOOLEAN NOT NULL` — `missing_count > 0 OR extra_count > 0`
- `sample JSONB` — optional small drift sample for diagnostics
- `updated_at`
- Written (upserted) by **`POST /closure/verify`**; read by the **`closure-drift`** health reporter
  ([platform](platform.md)). All columns `pii:none` (graph-level counts, no person/unit PII).

**`tenant_unit_lifecycle_events`** (append-only; `reject_mutation()` guard)
- `id` PK, `unit_id`, `from_state`, `to_state`, `reason TEXT`, `actor_person_id`,
  `request_id`, `created_at`

**`tenant_unit_code_events`** (append-only; `reject_mutation()` guard — substrate for the
`RecodeUnit` Action, D-UnitCodeLifecycle M28)
- `id` PK (RID slot `4,1,4`), `unit_id`, `old_code TEXT` (nullable), `new_code TEXT` (nullable —
  both nullable so NULL↔value transitions are recorded: give a code, fix a code, clear a code),
  `reason TEXT`, `actor_person_id`, `request_id`, `created_at`. Appended in the **same transaction**
  as the `tenant_units.code` update — the queryable rename history.

## Conjure API surface

`TenantService` (all unit-scoped checks against the path unit):

| Op | Intent | Perm |
|---|---|---|
| `POST /units` | Create a unit | `unit.create` (instance or parent-subtree) |
| `GET /units/{id}` | Read one unit | `unit.read` at the unit (per-unit decision; reach required even for a `public` unit) |
| `PUT /units/{id}` | Update name/kind/level/metadata/visibility (**`code` excluded** — see recode) | `unit.update` |
| `PUT /units/{id}/code` | Set / correct / clear the unit `code` (body: `code?`, `reason?`); audited, appends `tenant_unit_code_events`; `409 Tenant:UnitCodeConflict` on collision (D-UnitCodeLifecycle) | `unit.recode` |
| `GET /units` | List/search units (token-paginated); filtered by the declared facets — `org` (required), `domain`, `unitKind`, `level`, `visibility`, `state`, `pdpScoped` — plus `query`, a trigram-indexed substring match on the unit's `code`/`name` haystack (migration 0022, extending [D-PersonSearch](../architecture/decisions.md)'s R-21 generalization; NARROWS only, the shadow gate still trims afterwards), and the `graph`/`parent`/`rootsOnly` traversal args, which select a hierarchy query and **ignore** the flat filters (M56, [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope)) | `unit.read` + shadow gate |
| `GET /stats/units` | Facet distributions over the **same** flat-listing filter args + an optional `facets` CSV (M57; [facets catalog](../architecture/facets.md)). The shadow gate is folded **into the SQL** — the list's post-page `gateUnits` trim is correct for a page and wrong for a count. The path is `/stats/units`, not `/units/stats`, because a literal segment beside `{unitId}` makes the router refuse to register the route (D-ObjectFacets as-built) | `unit.read` + shadow gate |

> **`visibility` narrows, it never widens** (M56 ticket 2). The shadow-visibility gate still trims the
> page *after* it is cut, so `visibility=shadow` for a caller without shadow reach returns an empty
> page — not an error, and not a leak. `graph` is **not** a facet: it selects which DAG
> `parent`/`rootsOnly` walk and adds no predicate to `tenant_units` (there is no column to filter or
> group by), so the drift guard classifies it as a traversal arg.

| `GET /organizations` | List organizations, token-paginated, filtered by the declared facets — `domain`, `visibility`, `state` (M58 ticket 4; [facets catalog](../architecture/facets.md)). Malformed values are `Tenant:OrganizationInvalid` | `organization.read` + shadow gate |
| `GET /organizations/{orgId}` | Read one organization. A `shadow` organization the caller cannot reach is `Tenant:OrganizationNotFound` — the **same** error an unknown RID gets, because `shadow` hides existence and a permission error would confirm the organization is real. **This gate did not exist before M58 ticket 4** although the contract claimed it: the list trimmed shadow orgs while the point read handed them over | `organization.read` + shadow gate |
| `GET /stats/organizations` | Facet distributions over the **same** filter args + an optional `facets` CSV. The shadow gate is folded **into the SQL**, as for units. No `org` arg: the organization registry is the instance's whole realm catalog, not one org's tree. Path is `/stats/organizations`, not `/organizations/stats` (router) | `organization.read` + shadow gate |

> **Organization reach is DERIVED from unit reach** (M58 ticket 4 follow-up; D-VisibilityScope
> amendment). A shadow organization is visible when **any of its live units** is in the subject's
> reach — organizations cannot be granted directly, because `authz_role_assignments.target_unit_id`
> is `NOT NULL REFERENCES tenant_units`.
>
> Until that amendment the org list called **`gateUnits`**, the *unit* gate, on organization rows. It
> type-checks and reads plausibly, and it asks the unit reach probe whether an ORGANIZATION rid is
> among the subject's readable **units** — always no. So a shadow org was instance-admin-only by
> accident of the assignment table's shape, not by decision. Organizations now have their own
> `gateOrgs` / `FilterVisibleOrgs`, a **sibling** of the unit gate rather than a call into it,
> precisely because the two ask different questions and a shared entry point is what hid the
> mismatch. The dashboard's scoped arm folds the same derivation into SQL.
>
> It discloses nothing new — `listUnits` takes the org RID as a REQUIRED arg and gates the units, so
> a subject with reach inside an org could already enumerate them — and it is precise: reaching one
> shadow org does not reveal another.
| `POST /units/{id}/edges` | Add a parent in a graph (body: `parentId`, `graph`) | `unit.edges.<graph>.manage` OR `unit.edges.manage` (D-EdgePerms) |
| `DELETE /units/{id}/edges?graph={g}&parentId={p}` | Detach from a parent in a graph | `unit.edges.<graph>.manage` OR `unit.edges.manage` (D-EdgePerms) |
| `GET /units/{id}/ancestors?graph={g}` | Ancestors in graph `g` (closure; default `command`) | `unit.read` + shadow gate |
| `GET /units/{id}/descendants?graph={g}` | Subtree in graph `g` (closure, token-paginated; default `command`) | `unit.read` + shadow gate |
| `POST /units/{id}/transition` | Lifecycle transition (suspend/archive/restore) | `unit.lifecycle` |
| `GET /units/{id}/languages` | List the unit's official/working languages (name as `locale→text` map; D-Languages M18) | `unit.read` |
| `PUT /units/{id}/languages` | Upsert an official/working-language link (body: `languageId`, `isOfficial`) | `unit.update` |
| `DELETE /units/{id}/languages/{languageId}` | Remove a unit language | `unit.update` |
| `POST /closure/verify?graph={g}` | Diff stored closure vs. edges → drift report (default: all graphs); also upserts the per-graph `tenant_closure_status` the `closure-drift` health reporter reads (D-ClosureIntegrity / D-ClosureDriftHealth) | `closure.rebuild` (instance) |
| `POST /closure/rebuild?graph={g}` | Truncate + recompute closure, one txn per graph (default: all graphs); audited write (D-ClosureIntegrity) | `closure.rebuild` (instance) |
| `GET /graphs` | List the graph registry | `graph.read` |
| `POST /graphs` | Add a graph (body: `code`, `name`, `isAuthorityBearing?` default TRUE) | `graph.manage` (instance) |
| `PUT /graphs/{id}` | Rename / set default / flip `isAuthorityBearing` (guarded) | `graph.manage` (instance) |
| `DELETE /graphs/{id}` | Delete a graph (blocked: `command`, or in-use) | `graph.manage` (instance) |

Edge ops name their **graph** (defaulting to `command` when omitted). Returns Conjure
`SerializableError` on failure; cycle attempts → `Tenant:UnitCycleDetected` (scoped to the
edge's graph).

## Dependencies

- **Calls:** [localization](localization.md) to assemble the `name` locale-map for **units and
  graphs** in responses and validate locale codes. Uses [platform](platform.md) for DB pool,
  config, logging; emits domain events (`UnitCreated`, `UnitEdgeAdded` (carries the graph),
  `UnitTransitioned`, `UnitDeleted`, `GraphChanged`) consumed by
  [authorization](authorization.md), [audit](audit.md), and [localization](localization.md)
  (the latter purges the unit's / graph's translations on delete).
- **Called by:** [authorization](authorization.md) (closure queries: ancestors/descendants
  for the PDP), [membership](membership.md) (validate unit exists / visibility),
  [audit](audit.md).

## Authorization touchpoints

Defines and is gated by: `unit.create`, `unit.read`, `unit.update`, `unit.recode` (the audited
code set/correct/clear — D-UnitCodeLifecycle),
`unit.edges.<graph>.manage` / `unit.edges.manage` (D-EdgePerms),
`unit.lifecycle` (all unit-scoped, the path unit) and the **graph-registry** permissions
`graph.read` (a reference read in `unit-reader`) + `graph.manage` (instance-scope). The **catalog +
realm** permissions (D-TenantOrganizations, M40) are `domain.read`/`domain.manage`,
`unit-kind.read`/`unit-kind.manage` (instance-scope), and `organization.read` (org-scoped reads,
shadow-gated) + `organization.create`/`organization.update`/`organization.lifecycle`. Read results
pass the **shadow-visibility gate** ([patterns.md](../architecture/patterns.md)). The module
never decides access — it calls the PDP. **`level`, `domain`, `org`, and `kind` are NOT consulted by
any authorization check** — authority flows only through role assignments over (now per-org) graphs.

## Invariants & safety

- **No cycles, per graph.** On edge insert, reject if `child_id` is already an ancestor of
  `parent_id` **in that edge's graph** (closure lookup) → `Tenant:UnitCycleDetected`. Each graph
  stays a DAG; a cross-graph cycle is legal (A commands B in `command` while B is over A in
  `operational`).
- **Closure is always consistent with edges, per graph.** Edge insert/delete incrementally
  adjusts the affected graph's closure rows in the same transaction (M48; only the
  `anc*(parent) × desc*(child)` slice is touched, under a per-graph lock). Invariant:
  each graph's closure — rows **and shortest-path depths** — equals the transitive closure of
  that graph's edges — asserted in tests (incl. the M48 random attach/detach differential
  against a BFS oracle),
  enforceable at runtime by the on-demand verify/rebuild operation (D-ClosureIntegrity), and
  **surfaced as a diagnostic** by the `closure-drift` health reporter (fed by `verify`'s persisted
  `tenant_closure_status`; diagnostic-only, does not gate readiness — D-ClosureDriftHealth). The
  integrity backstop against drift (maintenance bug, manual DB edit under L-OperatorDB, partial
  failure).
- **Multi-parent, multi-root are legal.** A unit may have 0..N parents **per graph**; a unit
  with 0 parents in a graph is a root of that graph; ≥1 root may exist. Creating a **root** unit
  (no parent) is an **instance-scoped** `unit.create` — so the **first** unit is created by the
  bootstrapped instance admin (the graphs start empty after install; D-Bootstrap), and is the
  first post-bootstrap action.
- **Graph registry guard, per organization** (D-TenantOrganizations, M40). Each organization is
  seeded its own `command` + `operational` graphs **in the same transaction as the org**; `command`
  is the per-org default, **locked authority-bearing** (`CHECK`), and **cannot be deleted**; at least
  one graph always exists **per org**; a graph with live edges or active `subtree` assignments cannot
  be deleted. A graph may also be **instance-global** (`org_id NULL`, e.g. religion's taxonomy graphs);
  global graph `code` is unique among active global graphs. `is_authority_bearing` may flip TRUE→FALSE only when no active `subtree` assignments
  reference the graph (D-DirectoryGraphs); FALSE→TRUE is always safe. Graph `code` is unique **per org**
  among active graphs and immutable by convention.
- **Organization & domain are directory-only.** An organization owns its units and graphs; a unit's
  `org_id` is immutable after create. `domain_id` is per-unit (defaults to the org's domain;
  **mixed-domain trees allowed** — edges carry no same-domain constraint). Org/domain/`kind` are
  **never** PDP or shadow-gate inputs. Domain/unit-kind/org `code` is `NOT NULL UNIQUE`
  (unit-kind: unique per domain), immutable by convention. Org lifecycle mirrors unit lifecycle
  (reversible, append-only `tenant_org_lifecycle_events`).
- **Listing is org-scoped.** `GET /units` **requires** `?org` (rejected with `Tenant:UnitInvalid`
  otherwise); `?domain`/`?unitKind`/`?level` are optional filters. Cross-org browsing is
  `GET /organizations?domain=`. A person's organization history is **derived** from their temporal
  unit memberships ([membership](membership.md)) projected through each unit's `org_id` — there is no
  person↔org affiliation table.
- **Lifecycle is reversible.** `archived` is soft (within grace) before any purge; transitions
  are append-only events.
- Unit **`code`** is **optional** (a codeless unit is a non-separate sub-unit) and **unique among
  active units that have a code**; it is **mutable** only through the audited recode op
  (`PUT /units/{id}/code`), which appends a `tenant_unit_code_events` row in the same transaction
  (D-UnitCodeLifecycle, M28). External systems reference the **RID**, not the code. `name` is
  a localized label (default-locale fallback + [localization](localization.md) store).
- **RLS backstop.** The unit-scoped tables (`tenant_units`, `tenant_unit_edges`) carry the
  defense-in-depth RLS policies keyed on `app.readable_units` / `app.writable_units`
  (D-RLSDefenseInDepth) — a backstop behind the authoritative PDP + shadow gate, not a replacement.

## Open seams / future

- Per-unit `metadata` JSONB remains the long-tail escape hatch (column-ize when shared); the
  first promotion, `level`, is now a real column (DS-1 resolved). Geo/location stays in
  `metadata` until a spatial-query need is concrete (that would mean PostGIS, a larger decision).
- Closure maintenance is incremental; an **on-demand** per-graph verify/rebuild ships as the
  recovery/integrity tool (D-ClosureIntegrity), and silent-drift **detection** is surfaced by the
  diagnostic **`closure-drift`** health reporter (fed by `verify`'s persisted `tenant_closure_status`;
  diagnostic-only, never gates readiness — D-ClosureDriftHealth). Detection cadence stays
  operator-cron-driven (a systemd timer / k8s CronJob calling `POST /closure/verify`); a **scheduled
  in-app auto-rebuild is ruled out** (the small, rarely-re-orged graph does not justify it — DS-2
  resolved, DS-25 stays parked).
- A `merged`/`split` lifecycle (drafts had it) is intentionally deferred; not in scope.
- The exact set of lifecycle states may grow via expand/contract; `TEXT`+`CHECK` makes that a
  non-destructive migration.
- A unit's **official/working language** (`tenant_unit_languages`: an `OFFICIAL_LANGUAGE` link to a
  languoid) landed with **M18 / D-Languages**; the languoid catalog is owned by the
  [language](language.md) module. The management UI is deferred.
