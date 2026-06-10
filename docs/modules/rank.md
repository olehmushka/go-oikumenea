# Module: rank

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.rank_*`

## Purpose

Owns the **single, system-wide rank scheme** for the deployment: a **rank system** at the top, a
**rank category** within each system, a **tree of rank types** within each category (a type may have
child types), and **ranks on the leaf types**. There is exactly one scheme per instance (L-OneRankScheme,
never adopted per-unit), edited only by the **instance admin** (instance-scope authority —
[patterns.md](../architecture/patterns.md)). The one scheme may contain **several rank systems at once**
so a multinational/coalition directory can carry e.g. US and Ukrainian ranks together
([D-RankSystems](../architecture/decisions.md)). A rank is a **directory attribute describing
seniority and grants no authorization** (D-Rank); the PDP never reads it. A `person` points at
one `rank` (its system is *derived* through `rank → type → category → system`).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — four **Objects** forming the
one ordered scheme: `Rank system`, `Rank category`, `Rank type`, `Rank` (a person holds one via the
`HOLDS_RANK` link — never an authz input), plus the seeded reference catalog `Rank grade` (the
cross-system comparator). **Actions:** create/edit/reorder/soft-delete of scheme nodes + preset import
(`rank.scheme.manage`) — audited, `action__<type>` RID.

- **Rank system** — top level, ordered: a national/organizational rank ladder (e.g. `ua-armed-forces`,
  `us-armed-forces`, `nato`); optional `country` (`NULL` for supranational). One scheme holds many
  systems (D-RankSystems); a single-nation deployment just has one.
- **Rank category** — a branch within a system, ordered (e.g. `army`, `navy`, `marines`; or, in a
  university deployment, `academic`, `administrative`).
- **Rank type** — a band within a category, ordered, forming a **tree**: a type may nest under
  another type of the same category (e.g. `officers` → `junior`, `senior`, `general`; or
  `professorial` → `assistant`, `associate`). A root type sits directly under the category.
- **Rank** — a specific grade within a **leaf** type, ordered for exact seniority (e.g. `private`,
  `corporal`, `sergeant`; or `assistant_professor`, `associate_professor`, `full_professor`). Carries an
  optional **standardized `grade_code`** (NATO STANAG 2116) for cross-system comparability.

The scheme is a strict containment tree: each rank belongs to one leaf type, each type to one parent
type or (for a root type) one category, each category to one system. **Ranks live on leaf types only** —
a type with child types holds no ranks, and a type with ranks gains no children. Ordering at every level
expresses seniority **within a system**; comparison **across** systems uses the standardized grade
(below).

## Data model

Conventions per [conventions.md](../architecture/conventions.md). The scheme tables are
instance-config catalogs (low row counts, instance-admin-managed). Each level has a stable
locale-agnostic **`code`** (external reference; D-Code) and a **translatable `name`**
(default-locale fallback in the column + the [localization](localization.md) store; returned
as a `locale → text` map). `system_id` is **denormalized down** the tree (onto types and ranks) exactly
as `category_id` already is, so grouping, sibling code-uniqueness, and seniority need no recursive walk
(D-RankSystems).

**`rank_systems`** (top level — D-RankSystems)
- `id` PK, `code TEXT NOT NULL UNIQUE`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `country TEXT REFERENCES geo_countries(code)` — the national origin; `NULL` for supranational (NATO/UN)
- `created_at`, `updated_at`, `deleted_at`

**`rank_categories`**
- `id` PK
- `system_id TEXT NOT NULL REFERENCES rank_systems(id) ON DELETE RESTRICT` — the owning system
- `code TEXT NOT NULL`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `UNIQUE (system_id, code)` among active — unique among siblings within the system; timestamps + `deleted_at`

**`rank_types`**
- `id` PK
- `system_id TEXT NOT NULL` — denormalized root system (equals the category's `system_id`)
- `category_id TEXT NOT NULL REFERENCES rank_categories(id) ON DELETE RESTRICT` — the **root**
  category, carried (denormalized) on every type so a nested type's `category_id` equals its parent's
- `parent_type_id TEXT REFERENCES rank_types(id) ON DELETE RESTRICT` — the parent type; `NULL` for a
  root type of the category. `CHECK (parent_type_id <> id)`
- `code TEXT NOT NULL`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `UNIQUE (category_id, COALESCE(parent_type_id,''), code)` among active — unique among **siblings**
  (same category + same parent); timestamps + `deleted_at`

**`rank_ranks`**
- `id` PK
- `system_id TEXT NOT NULL` — denormalized root system (equals the type's `system_id`)
- `type_id TEXT NOT NULL REFERENCES rank_types(id) ON DELETE RESTRICT` — a **leaf** type
- `code TEXT NOT NULL`, `name TEXT NOT NULL`, `sort_order INT NOT NULL`
- `abbreviation TEXT` — optional short form (e.g. `SGT`)
- `grade_code TEXT REFERENCES rank_grades(code)` — optional standardized cross-system grade (D-RankSystems)
- `UNIQUE (type_id, code)`; timestamps + `deleted_at`

**`rank_grades`** (seeded reference catalog — the cross-system comparator; D-RankSystems / D-Geo carve-out)
- `code TEXT PRIMARY KEY` — NATO STANAG 2116 grade (`OF-1`…`OF-10`, `OF(D)`, `OR-1`…`OR-9`, warrant)
- `tier TEXT NOT NULL CHECK (tier IN ('officer','warrant','enlisted'))`
- `ordinal INT NOT NULL` — order within the comparability scale
- `name TEXT NOT NULL`; **migration-seeded** (natural key → no D-RIDSeeding GUC issue), instance-global,
  immutable-by-convention

`sort_order` is unique within its parent (the sibling group); reorders rewrite `sort_order` for the
affected siblings in one transaction.

**Seniority & equivalence.**
- *Within a system:* the structural order `(system.sort_order, category.sort_order, the type sort_order
  path down the tree, rank.sort_order)`.
- *Across systems — equivalence:* two ranks are equivalent when they share a `grade_code` (US `OF-5` ≈
  UA `OF-5`).
- *Across systems — seniority:* compare `grade.tier` then `grade.ordinal`.
- If either rank has no `grade_code`, the pair is **incomparable across systems** — the pure
  `isSenior(a, b)` helper returns *unknown*, never a wrong answer.

## Conjure API surface

`RankService` — **reads use `rank.scheme.read` (an explicit grant, bundled in the `unit-reader`
base role — D-BaseRoles); writes are instance-scope only.**

| Op | Intent | Perm |
|---|---|---|
| `GET /rank-scheme` | Read the whole scheme (systems → categories → types → ranks) | `rank.scheme.read` |
| `GET /rank-grades` | Read the standardized-grade comparator catalog (codes + tier/ordinal) | `rank.scheme.read` |
| `POST /rank-scheme/systems` | Add a rank system | `rank.scheme.manage` (instance) |
| `PUT /rank-scheme/systems/{id}` | Edit/reorder a system | `rank.scheme.manage` |
| `POST /rank-scheme/categories` | Add a category under a system (`systemId`) | `rank.scheme.manage` |
| `PUT /rank-scheme/categories/{id}` | Edit/reorder a category | `rank.scheme.manage` |
| `POST /rank-scheme/types` | Add a type under a category (`categoryId`) or nested under a parent type (`parentTypeId`) | `rank.scheme.manage` |
| `PUT /rank-scheme/types/{id}` | Edit/reorder a type | `rank.scheme.manage` |
| `POST /rank-scheme/ranks` | Add a rank under a type (optional `gradeCode`) | `rank.scheme.manage` |
| `PUT /rank-scheme/ranks/{id}` | Edit/reorder a rank (incl. `gradeCode`) | `rank.scheme.manage` |
| `DELETE /rank-scheme/{level}/{id}` | Soft-delete a scheme node (blocked if in use) | `rank.scheme.manage` |
| `POST /rank-scheme/import` | Import a preset `rank_system` subtree (idempotent, code-keyed upsert) | `rank.scheme.manage` |

**Presets (D-RankSystems).** A *preset* is a curated document for one `rank_system` subtree
(`system → categories → types → ranks`, each with `code`/`name`/`sort_order`/`grade_code`), shipped
in-repo as **opt-in** reference data (e.g. `deploy/rank-presets/{ua-armed-forces,us-armed-forces,
nato-generic}.json`) — never auto-seeded (rank stays deployment-specific). `POST /rank-scheme/import`
applies one preset as a **code-keyed idempotent upsert in one transaction** (RIDs minted at import per
D-RIDSeeding; re-import updates `name`/`sort_order`, never duplicates) and is **additive/upsert only —
it never deletes an in-use node**; it returns a created/updated/skipped summary. Shape:

```jsonc
{ "system": { "code": "us-armed-forces", "name": "US Armed Forces", "country": "US", "sortOrder": 0,
  "categories": [ { "code": "army", "name": "Army", "sortOrder": 0,
    "types": [ { "code": "officers", "name": "Officers", "sortOrder": 0,
      "ranks": [ { "code": "o6", "name": "Colonel", "abbreviation": "COL", "gradeCode": "OF-5", "sortOrder": 50 } ] } ] } ] } }
```

## Dependencies

- **Calls:** [localization](localization.md) (assemble `name` locale-maps; purge translations
  on scheme-node delete), [platform](platform.md) for infra (the seeded `geo_countries` registry backs a
  rank system's optional `country`; D-Geo). Emits `RankSchemeChanged` events
  (consumed by [audit](audit.md) and [localization](localization.md)).
- **Called by:** [person](person.md) (validates a `rank_id` on assign). No other module reads
  rank for behavior — by design.

## Authorization touchpoints

Defines/gates: `rank.scheme.read` (in the `unit-reader` base role; an explicit grant, not an
implicit allow — D-BaseRoles), `rank.scheme.manage` (**instance-scope** — the top-permission
plane, never a unit assignment). Editing the scheme is the canonical instance-admin capability.

## Invariants & safety

- **Exactly one scheme** per deployment (no scheme-per-unit table; the catalog *is* the
  scheme). The one scheme may hold **multiple `rank_systems`** (multinational; D-RankSystems) — still one
  registry, instance-admin-managed, never per-unit (L-OneRankScheme refined, not broken).
- **Containment is strict:** rank → leaf type → … → root type → category → system, each parent existing
  and not soft-deleted; `system_id` denormalized down the tree equals the root system at every level.
  **Ranks attach to leaf types only** (a type with active child types holds no ranks, and a type with
  active ranks gains no child types — enforced in the application).
- **Standardized grade is validated** against the seeded `rank_grades` catalog (an unknown `grade_code`
  is rejected); it is **optional** — a non-military system leaves it `NULL` and has no cross-system
  comparator (L-SingleDomain: no org-type discriminator). Cross-system seniority/equivalence read the
  grade, never the intra-system `sort_order`.
- **Preset import is idempotent and non-destructive:** code-keyed upsert in one transaction; re-import
  updates labels/order and **never deletes** an in-use node (D-RankSystems).
- **`parent_type_id` is set at creation and immutable** (like `code`); reparenting a type is an open
  seam. Because a fresh type has no children, no cycle can form.
- **A scheme node in use cannot be hard-deleted.** A category with active types, or a type with
  active ranks or active child types, is blocked. `rank_ranks` referenced by a
  `person_persons.rank_id` is `ON DELETE RESTRICT`; soft-delete is also blocked while
  referenced (checked in the application).
- **`sort_order` unique within parent** (the sibling group), contiguous after a reorder.
- **Rank ≠ permission** — no code path consults rank to authorize ([patterns.md](../architecture/patterns.md),
  Directory attribute vs. authorization).

## Open seams / future

- Position (the unit billet) is intentionally **not** here — it lives in
  [membership](membership.md) (D-Rank). Rank = whole-org seniority; position = unit role.
- A seniority-comparison helper (`isSenior(a, b)`) is a pure domain function (intra-system structural
  order; cross-system via `grade.tier`/`ordinal`; *unknown* when a grade is absent); exposed only if a
  caller needs it.
- **Multinational is modeled as multiple `rank_systems` within the one scheme** (D-RankSystems), **not**
  multiple schemes (the one-scheme lock holds). A truly separate second *registry* remains out of scope.
- The standardized-grade comparator is **military (NATO STANAG 2116)**; academic/ecclesiastical
  deployments have no published cross-system grade, so `grade_code` stays `NULL` and cross-system
  comparison is N/A there (parked as **DS-43**).
- Reparenting a type / moving a category between systems is an open seam (`parent_type_id` / `system_id`
  immutable after creation).
