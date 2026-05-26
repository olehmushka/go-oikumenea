# Module: rank

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.rank_*`

## Purpose

Owns the **single, system-wide rank scheme** for the deployment: a three-level ordered ladder
**rank category → rank type → rank**. There is exactly one scheme per instance (L-OneRankScheme,
never adopted per-unit), edited only by the **instance admin** (instance-scope authority —
[patterns.md](../architecture/patterns.md)). A rank is a **directory attribute describing
seniority and grants no authorization** (D-Rank); the PDP never reads it. A `person` points at
one `rank`.

## Entities & aggregates

- **Rank category** — top level, ordered (e.g. `army`, `navy`, `marines`; or, in a university
  deployment, `academic`, `administrative`).
- **Rank type** — a band within a category, ordered (e.g. `officers`, `warrant`, `enlisted`;
  or `professorial`, `lecturer`).
- **Rank** — a specific grade within a type, ordered for exact seniority (e.g. `private`,
  `corporal`, `sergeant`; or `assistant_professor`, `associate_professor`, `full_professor`).

The three are a strict containment tree (each rank belongs to one type, each type to one
category). Ordering at every level expresses seniority.

## Data model

Conventions per [conventions.md](../architecture/conventions.md). All three are
instance-config catalogs (low row counts, instance-admin-managed). Each level has a stable
locale-agnostic **`code`** (external reference; D-Code) and a **translatable `name`**
(default-locale fallback in the column + the [localization](localization.md) store; returned
as a `locale → text` map).

**`rank_categories`**
- `id` PK, `code TEXT NOT NULL UNIQUE`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`,
  `created_at`, `updated_at`, `deleted_at`

**`rank_types`**
- `id` PK
- `category_id UUID NOT NULL REFERENCES rank_categories(id) ON DELETE RESTRICT`
- `code TEXT NOT NULL`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `UNIQUE (category_id, code)`; timestamps + `deleted_at`

**`rank_ranks`**
- `id` PK
- `type_id UUID NOT NULL REFERENCES rank_types(id) ON DELETE RESTRICT`
- `code TEXT NOT NULL`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `abbreviation TEXT` — optional short form (e.g. `SGT`)
- `UNIQUE (type_id, code)`; timestamps + `deleted_at`

`sort_order` is unique within its parent; reorders rewrite `sort_order` for the affected
siblings in one transaction. Seniority comparison across the whole scheme is
`(category.sort_order, type.sort_order, rank.sort_order)`.

## Conjure API surface

`RankService` — **reads are broadly allowed; writes are instance-scope only.**

| Op | Intent | Perm |
|---|---|---|
| `GET /rank-scheme` | Read the whole scheme (categories → types → ranks) | `rank.scheme.read` |
| `POST /rank-scheme/categories` | Add a category | `rank.scheme.manage` (instance) |
| `PUT /rank-scheme/categories/{id}` | Edit/reorder a category | `rank.scheme.manage` |
| `POST /rank-scheme/types` | Add a type under a category | `rank.scheme.manage` |
| `PUT /rank-scheme/types/{id}` | Edit/reorder a type | `rank.scheme.manage` |
| `POST /rank-scheme/ranks` | Add a rank under a type | `rank.scheme.manage` |
| `PUT /rank-scheme/ranks/{id}` | Edit/reorder a rank | `rank.scheme.manage` |
| `DELETE /rank-scheme/{level}/{id}` | Soft-delete a scheme node (blocked if in use) | `rank.scheme.manage` |

## Dependencies

- **Calls:** [localization](localization.md) (assemble `name` locale-maps; purge translations
  on scheme-node delete), [platform](platform.md) for infra. Emits `RankSchemeChanged` events
  (consumed by [audit](audit.md) and [localization](localization.md)).
- **Called by:** [person](person.md) (validates a `rank_id` on assign). No other module reads
  rank for behavior — by design.

## Authorization touchpoints

Defines/gates: `rank.scheme.read` (broad), `rank.scheme.manage` (**instance-scope** — the
top-permission plane, never a unit assignment). Editing the scheme is the canonical
instance-admin capability.

## Invariants & safety

- **Exactly one scheme** per deployment (no scheme-per-unit table; the catalog *is* the
  scheme).
- **Containment is strict:** rank → type → category, each parent existing and not soft-deleted.
- **A scheme node in use cannot be hard-deleted.** `rank_ranks` referenced by a
  `person_persons.rank_id` is `ON DELETE RESTRICT`; soft-delete is also blocked while
  referenced (checked in the application).
- **`sort_order` unique within parent**, contiguous after a reorder.
- **Rank ≠ permission** — no code path consults rank to authorize ([patterns.md](../architecture/patterns.md),
  Directory attribute vs. authorization).

## Open seams / future

- Position (the unit billet) is intentionally **not** here — it lives in
  [membership](membership.md) (D-Rank). Rank = whole-org seniority; position = unit role.
- A seniority-comparison helper (`isSenior(a, b)`) is a pure domain function; exposed only if a
  caller needs it (none does today).
- Multiple parallel schemes (e.g. one per branch as separate top-levels) are already expressible
  via multiple `rank_categories`; a truly separate second scheme is out of scope (one scheme
  lock).
