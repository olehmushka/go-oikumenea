# Module: external-organizations

> **Status: planned (M30 / D-ExternalOrgs).** Designed, not yet built — no `internal/` or
> `migrations/` artifacts exist yet. Binding design lives in
> [roadmap-decisions.md](../architecture/roadmap-decisions.md) (D-ExternalOrgs).

## Purpose

A registry of **external organizations** a person is tied to but which the deploying org neither owns
nor commands — political parties, government bodies, foreign military formations, NGOs, lobbying
registrants/clients. It is the **node-space** the M33 institutional-tie edges
([person](person.md)) point at when the org side is *not* one of the operator's own
[tenant](tenant.md) units and *not* a for-profit legal entity in the M21 [company](company.md)
registry. Faith-/sector-agnostic, catalog-typed, provenance-tagged.

This module exists because external orgs are conceptually distinct from both (a) the operator's own
unit DAG — which is authority-bearing through the PDP — and (b) commercial legal entities. Mixing a
party or a ministry into either mis-types it; a single registry with a `kind` catalog keeps every
affiliation edge pointing at one coherent node-space (D-ExternalOrgs).

## Entities & aggregates

- **`ExternalOrganization`** (Object, RID `18,1,1`) — RID PK; translatable `name`; `kind` →
  `external_org_kinds`; optional `country` → `geo_countries`; optional `wikidata_id` (concordance);
  **provisional/resolved `status`** + the D-OverlayFoundation attribution columns; soft-delete.
- **`ExternalOrgKind`** (catalog Object, RID `18,1,2`) — instance-admin catalog
  (`code`/translatable `name`): `party | government_body | military | ngo | registrant | other`.

(Final RID type codes are allocated in `pkg/rid` + migration `0000` when the module is built — service
**18** is reserved here. The person↔org **links** live on the person service, M33.)

## Data model

- `external_org_kinds` — `code TEXT NOT NULL UNIQUE`, translatable `name` (`entity_type='external_org_kind'`),
  migration-seeded; `TEXT`+`CHECK` discipline per [conventions.md](../architecture/conventions.md).
- `external_organizations` — RID PK via `new_id()`; `name` (default-locale column +
  `i18n_translations`, `entity_type='external_organization'`); `kind_id` FK;
  `country_id` nullable FK → `geo_countries`; `wikidata_id` nullable; `status ∈ {provisional, resolved}`;
  `source`/`confidence`/`as_of` (D-OverlayFoundation); `TIMESTAMPTZ` audit columns; `deleted_at`
  soft-delete; `set_updated_at()` trigger.
- A hermenea import target (`external-organizations` object-type) — Wikidata / public registries feed
  it via the generic `POST /import/{objectType}` upsert (M16), stamping per-row provenance.

## Conjure API surface

`api/external-organizations.conjure.yml` → `ExternalOrganizationService`:

- `GET /external-orgs` (list/filter by `kind`/`country`/`status`), `GET /external-orgs/{id}`,
  `POST /external-orgs`, `PUT /external-orgs/{id}`, `DELETE /external-orgs/{id}` (soft-delete).
- `GET /external-org-kinds` (catalog read); kind management is instance-scope.
- Names returned as a `locale→text` map (D-i18n), assembled via `LocalizationService.NamesByID`.

## Dependencies

- [localization](localization.md) — translatable `name` assembly.
- [platform](platform.md) — `geo_countries` registry, RID service registry, the import endpoint.
- [audit](audit.md) — every write records an Action.
- Consumed by [person](person.md) (M33 institutional ties) and reused alongside
  [company](company.md) (corporate ties) + [tenant](tenant.md) (internal-unit ties).

## Authorization touchpoints

- Reads: `externalorg.read`; catalog + org writes: `externalorg.manage` (instance-scope — this is
  reference/registry data, not person PII, so no holder-scoping).
- Provisional→resolved promotion and merge follow the D-OverlayFoundation manual-resolution pattern,
  audited.

## Patterns

- **Catalog-driven kind** (D-Code) — mirrors `religion_org_kinds` / `company_legal_forms`; no
  per-type tables, no hard-coded org vocabulary in schema.
- **Provenance-tagged registry** — provisional stubs + `source`/`confidence` so an imported or
  unresolved org is first-class and later promotable (D-OverlayFoundation).
- **RID-keyed shared registry** (F-014) — consumers reference by `id`; resolve a `code`/`wikidata_id`
  → RID via the read API, like `geo_countries`.

## Invariants & safety

- `kind_id` always resolves to a seeded catalog row; `country_id` (when present) is a seeded
  `geo_countries` row.
- A `resolved` org has no unmerged provisional duplicate pointing at it (merge re-homes edges).
- External orgs **never** enter the tenant closure / PDP graph — they are directory nodes only.

## Open seams / future

- Org↔org structure (a party's regional branches, a ministry's agencies) — deferred; add a reified
  `parent_of` link + closure only if a consumer needs hierarchy.
- Reconciling an external org that later becomes a tenant unit or an M21 company (promotion across
  registries) — manual re-home for now.
