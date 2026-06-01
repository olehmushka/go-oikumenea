# Module: tenant

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.tenant_*`

## Purpose

Owns the organization as a **graph of units**. A unit is a node — a brigade/battalion/platoon,
or a university/campus/department. The module stores units, their parent→child edges (a **DAG**:
multi-parent, multi-root), a maintained transitive **closure** for fast ancestor/descendant
queries, each unit's **visibility** (`public`/`shadow`), and its lifecycle state. It is the
structural foundation the whole service hangs off; the [authorization](authorization.md) PDP
reads its closure to resolve inheritance. It does **not** decide access itself.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Unit`, `Graph`.
**Links:** `link__parent_of` (the unit edge, per graph) and the derived `link__ancestor_of` (the
closure). **Actions:** `CreateUnit`, `TransitionUnit`, `AddEdge`/`RemoveEdge`, graph management —
each audited and keyed by its `action__<type>` RID.

- **Unit** (aggregate root) — a node in the org graph: stable `code`, translatable `name`,
  optional `unit_kind` label, optional ordinal `level`, `visibility`, lifecycle `state`,
  free-form `metadata`.
- **Graph** — a **named hierarchy** over the units (D-Graphs): `command` (the structural /
  administrative authority chain — the default, undeletable) and `operational`
  (mission / task-organization, OPCON-like). Instance-admin-managed registry; stable `code` +
  translatable `name`. Each graph is independently a DAG.
- **Unit edge** — a directed `parent → child` relationship **within one graph**. Many per unit
  in either direction (DAG), across graphs.
- **Unit closure** — derived, **per graph**: every `(graph, ancestor, descendant, depth)` pair
  reachable through that graph's edges. Maintained automatically on edge change; not user-edited.
- **Unit lifecycle event** — append-only record of each state transition.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete, `TEXT`+`CHECK` enums)
per [conventions.md](../architecture/conventions.md).

**`tenant_units`**
- `id` PK
- `code TEXT NOT NULL` — **stable, locale-agnostic** identifier for external-system reference
  (D-Code); unique among active units (`UNIQUE WHERE deleted_at IS NULL`); immutable by
  convention. (Replaces drafts' `slug` — an API-only service has no subdomains.)
- `name TEXT NOT NULL` — default-locale display name; **translatable** via the
  [localization](localization.md) store (returned as a `locale → text` map)
- `unit_kind TEXT` — descriptive instance label (e.g. `battalion`); **not** branched on in
  code (see L-SingleDomain)
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

**`tenant_graphs`** (named-hierarchy registry; instance-admin-managed — D-Graphs)
- `id` PK
- `code TEXT NOT NULL` — **stable, locale-agnostic** identifier referenced by edges/closure/
  assignments (e.g. `command`, `operational`); unique among active rows; immutable by convention
- `name TEXT NOT NULL` — default-locale display name; **translatable** via the
  [localization](localization.md) store
- `is_default BOOLEAN NOT NULL DEFAULT FALSE` — exactly one default (seeded `command`); the
  default is the graph a `subtree` grant uses when none is named
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
- `depth INT NOT NULL` (0 = self-row for each unit, per graph)
- `PRIMARY KEY (graph_id, ancestor_id, descendant_id)`; indexed both directions
- Includes the reflexive `(g, u, u, 0)` row so "is U in the subtree of T in graph g" is one
  lookup. An edge change in graph K recomputes only K's rows in the same transaction.

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

## Conjure API surface

`TenantService` (all unit-scoped checks against the path unit):

| Op | Intent | Perm |
|---|---|---|
| `POST /units` | Create a unit | `unit.create` (instance or parent-subtree) |
| `GET /units/{id}` | Read one unit | `unit.read` + shadow gate |
| `PUT /units/{id}` | Update name/kind/level/metadata/visibility | `unit.update` |
| `GET /units` | List/search units (token-paginated; filterable by `level`) | `unit.read` + shadow gate |
| `POST /units/{id}/edges` | Add a parent in a graph (body: `parentId`, `graph`) | `unit.edges.<graph>.manage` OR `unit.edges.manage` (D-EdgePerms) |
| `DELETE /units/{id}/edges?graph={g}&parentId={p}` | Detach from a parent in a graph | `unit.edges.<graph>.manage` OR `unit.edges.manage` (D-EdgePerms) |
| `GET /units/{id}/ancestors?graph={g}` | Ancestors in graph `g` (closure; default `command`) | `unit.read` + shadow gate |
| `GET /units/{id}/descendants?graph={g}` | Subtree in graph `g` (closure, token-paginated; default `command`) | `unit.read` + shadow gate |
| `POST /units/{id}/transition` | Lifecycle transition (suspend/archive/restore) | `unit.lifecycle` |
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

Defines and is gated by: `unit.create`, `unit.read`, `unit.update`,
`unit.edges.<graph>.manage` / `unit.edges.manage` (D-EdgePerms),
`unit.lifecycle` (all unit-scoped, the path unit) and the **graph-registry** permissions
`graph.read` (a reference read in `unit-reader`) + `graph.manage` (instance-scope). Read results
pass the **shadow-visibility gate** ([patterns.md](../architecture/patterns.md)). The module
never decides access — it calls the PDP. `level` is **not** consulted by any check.

## Invariants & safety

- **No cycles, per graph.** On edge insert, reject if `child_id` is already an ancestor of
  `parent_id` **in that edge's graph** (closure lookup) → `Tenant:UnitCycleDetected`. Each graph
  stays a DAG; a cross-graph cycle is legal (A commands B in `command` while B is over A in
  `operational`).
- **Closure is always consistent with edges, per graph.** Edge insert/delete recomputes the
  affected graph's closure rows in the same transaction (incremental maintenance). Invariant:
  each graph's closure equals the transitive closure of that graph's edges — asserted in tests,
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
- **Graph registry guard.** `command` is seeded, is the default, is **locked authority-bearing**
  (`CHECK`), and **cannot be deleted**; at least one graph always exists; a graph with live edges
  or active `subtree` assignments cannot be deleted. `is_authority_bearing` may flip TRUE→FALSE
  only when no active `subtree` assignments reference the graph (D-DirectoryGraphs); FALSE→TRUE
  is always safe. Graph `code` is unique among active graphs and immutable by convention.
- **Lifecycle is reversible.** `archived` is soft (within grace) before any purge; transitions
  are append-only events.
- Unit **`code`** is unique among active units and stable/immutable by convention; `name` is
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
