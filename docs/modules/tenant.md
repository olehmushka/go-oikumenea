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

- **Unit** (aggregate root) — a node in the org graph: stable `code`, translatable `name`,
  optional `unit_kind` label, `visibility`, lifecycle `state`, free-form `metadata`.
- **Unit edge** — a directed `parent → child` relationship. Many per unit in either
  direction (DAG).
- **Unit closure** — derived: every `(ancestor, descendant, depth)` pair reachable through
  edges. Maintained automatically on edge change; not user-edited.
- **Unit lifecycle event** — append-only record of each state transition.

## Data model

Conventions (UUIDv7 PKs, `TIMESTAMPTZ`, `set_updated_at`, soft-delete, `TEXT`+`CHECK` enums)
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
- `visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','shadow'))`
- `state TEXT NOT NULL DEFAULT 'active' CHECK (state IN ('active','suspended','archived'))`
- `metadata JSONB NOT NULL DEFAULT '{}'`
- `created_at`, `updated_at`, `deleted_at`

**`tenant_unit_edges`**
- `id` PK
- `parent_id UUID NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `child_id  UUID NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `created_at`, `created_by` (person id, provenance)
- `UNIQUE (parent_id, child_id)`; `CHECK (parent_id <> child_id)` (no self-loop)
- Cycle prevention enforced in the application/SQL on insert (see Invariants).

**`tenant_unit_closure`** (derived; maintained on edge insert/delete)
- `ancestor_id UUID NOT NULL`
- `descendant_id UUID NOT NULL`
- `depth INT NOT NULL` (0 = self-row for each unit)
- `PRIMARY KEY (ancestor_id, descendant_id)`; indexed both directions
- Includes the reflexive `(u, u, 0)` row so "is U in the subtree of T" is one lookup.

**`tenant_unit_lifecycle_events`** (append-only; `reject_mutation()` guard)
- `id` PK, `unit_id`, `from_state`, `to_state`, `reason TEXT`, `actor_person_id`,
  `request_id`, `created_at`

## Conjure API surface

`TenantService` (all unit-scoped checks against the path unit):

| Op | Intent | Perm |
|---|---|---|
| `POST /units` | Create a unit | `unit.create` (instance or parent-subtree) |
| `GET /units/{id}` | Read one unit | `unit.read` + shadow gate |
| `PUT /units/{id}` | Update name/kind/metadata/visibility | `unit.update` |
| `GET /units` | List/search units (token-paginated) | `unit.read` + shadow gate |
| `POST /units/{id}/edges` | Add a parent (attach to a parent unit) | `unit.edges.manage` |
| `DELETE /units/{id}/edges/{parentId}` | Detach from a parent | `unit.edges.manage` |
| `GET /units/{id}/ancestors` | Ancestors (closure) | `unit.read` + shadow gate |
| `GET /units/{id}/descendants` | Subtree (closure, token-paginated) | `unit.read` + shadow gate |
| `POST /units/{id}/transition` | Lifecycle transition (suspend/archive/restore) | `unit.lifecycle` |

Returns Conjure `SerializableError` on failure; cycle attempts → `Tenant:UnitCycleDetected`.

## Dependencies

- **Calls:** [localization](localization.md) to assemble the `name` locale-map in responses
  and validate locale codes. Uses [platform](platform.md) for DB pool, config, logging; emits
  domain events (`UnitCreated`, `UnitEdgeAdded`, `UnitTransitioned`, `UnitDeleted`) consumed by
  [authorization](authorization.md), [audit](audit.md), and [localization](localization.md)
  (the latter purges the unit's translations on delete).
- **Called by:** [authorization](authorization.md) (closure queries: ancestors/descendants
  for the PDP), [membership](membership.md) (validate unit exists / visibility),
  [audit](audit.md).

## Authorization touchpoints

Defines and is gated by: `unit.create`, `unit.read`, `unit.update`, `unit.edges.manage`,
`unit.lifecycle`. All checks are unit-scoped (the path unit). Read results pass the
**shadow-visibility gate** ([patterns.md](../architecture/patterns.md)). The module never
decides access — it calls the PDP.

## Invariants & safety

- **No cycles.** On edge insert, reject if `child_id` is already an ancestor of `parent_id`
  (closure lookup) → `Tenant:UnitCycleDetected`. This keeps the graph a DAG.
- **Closure is always consistent with edges.** Edge insert/delete recomputes affected closure
  rows in the same transaction (incremental maintenance). Tested invariant: closure equals the
  transitive closure of edges.
- **Multi-parent, multi-root are legal.** A unit may have 0..N parents; a unit with 0 parents
  is a root; ≥1 root may exist.
- **Lifecycle is reversible.** `archived` is soft (within grace) before any purge; transitions
  are append-only events.
- Unit **`code`** is unique among active units and stable/immutable by convention; `name` is
  a localized label (default-locale fallback + [localization](localization.md) store).

## Open seams / future

- Per-unit `metadata` JSONB is the long-tail escape hatch (column-ize when shared).
- Closure maintenance is incremental; if edge churn ever dominates, a periodic full rebuild
  job is an additive seam.
- A `merged`/`split` lifecycle (drafts had it) is intentionally deferred; not in scope.
- The exact set of lifecycle states may grow via expand/contract; `TEXT`+`CHECK` makes that a
  non-destructive migration.
