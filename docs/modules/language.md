# Language module

> **Status: M18 — verified.** Binding decision: **D-Languages**
> ([roadmap-decisions.md](../architecture/roadmap-decisions.md)). The first NEW consumer of the M16
> [hermenea](hermenea.md) ingestion pipeline. Names are returned as `locale→text` maps (D-i18n via
> `NamesByID`); `i18n_locale_languages` is reconciled on import; the person/unit/locale language
> editors + a language browser ship in the web console.

## Purpose

Owns the world's languages as a **faithful model of Glottolog** (the standard genealogical reference) —
the `family → language → dialect` forest keyed by **glottocode** — their **writing systems** (ISO
15924), and the language ties on people, units, and locales. Language is a recurring analytics / linking
dimension (who speaks what; a unit's working language; a locale's canonical language), so it is modeled
as first-class reference data rather than a hand-curated list. It is **read-only at the API**: the
catalog is written exclusively by the hermenea import pipeline (`language-scheme` + `language-scripts`).

The languoid forest (the genealogical catalog) is **distinct from the localization module's supported
UI locales** (`i18n_locales`): a *locale* is a deployment's supported UI language (ISO 639-3, instance-
admin-managed); a *languoid* is a node in Glottolog's genealogy. `i18n_locale_languages` ties the two.

## Entities

| Entity | Kind | Notes |
|---|---|---|
| **Languoid** | Object (RID `13,1,1`) | one Glottolog node; `code` = glottocode (UNIQUE); `level ∈ family\|language\|dialect` |
| **WritingSystem** | Object (RID `13,1,2`) | one ISO-15924 script; `code` UNIQUE; migration-seeded |
| **WritingSystemScriptType** | Object (RID `13,1,3`) | closed catalog: logographic/syllabary/alphabet/abjad/abugida/featural |
| **LANGUAGE_SUBGROUP_OF** | structural FK (not a Link) | `language_languoids.parent_id` self-FK — strict tree; closure is `language_languoid_closure` |
| **WRITTEN_IN** | reified Link (RID `13,2,1`) | `language_writing_systems` (`is_primary`); import-loaded from CLDR |
| **SPEAKS** | reified Link (person RID `6,2,8`) | `person_languages` (`cefr_level`, `is_native`); `pii:basic`, purge-erased |
| **OFFICIAL_LANGUAGE** | reified Link (tenant RID `4,2,2`) | `tenant_unit_languages` (`is_official`) |
| **LOCALE_OF** | reified Link (i18n RID `2,2,1`) | `i18n_locale_languages` — one per locale |

## Data model

Migration `migrations/20260601000018_language.sql` (RID service **13**, schema `oikumenea`):

- **`language_languoids`** — RID `id` PK (F-014; `code` glottocode is a UNIQUE lookup key, like
  `geo_countries`); `level` CHECK; `name` (default-locale column; returned as a D-i18n `locale→text`
  map assembled via `LocalizationService.NamesByID`, entity type `languoid`); self-FK `parent_id … ON DELETE RESTRICT`
  (strict tree — Glottolog "father", **not** a reified Link); denormalized `family_code` (root family,
  derived in SQL via the closure); nullable UNIQUE `iso639_3`; `macroarea`; `latitude`/`longitude`
  (plain `double precision` — M18 precedes the PostGIS Location, D-Location); AES `status` CHECK
  (`not_endangered…extinct`); `glottolog_version` + `(source, source_version, imported_at)` provenance.
  A composite `UNIQUE (id, level)` lets `person_languages` FK against a level-constrained target.
  **Migration-seeded** with the ~50 most-spoken languages (`level='language'`, required columns only),
  mirroring the `geo_countries` bootstrap, so the catalog is usable before any import. The seed `code`s
  are real glottocodes, so the first `language-scheme` import **updates them in place** (no duplicates)
  and fills in `parent_id`/`iso639_3`/`family_code`/`source`.
- **`language_languoid_closure`** — derived transitive closure `(ancestor_id, descendant_id, depth)`,
  no RID (mirrors `tenant_unit_closure`); rebuilt in SQL at the end of every `language-scheme` import.
- **`language_languoid_countries`** — plain M:N → `geo_countries(id)` (CLDF `Country_IDs`), no RID.
- **`writing_system_script_types`** — RID-keyed catalog, **migration-seeded** (6 rows).
- **`writing_systems`** — RID-keyed, ISO-15924 `code` UNIQUE, `script_type` FK; **migration-seeded**
  with the living-language scripts (instance-admin-extensible; an import skips a link whose script is
  not yet seeded).
- **`language_writing_systems`** — reified M:N (`is_primary`), RID link, UNIQUE `(languoid, ws)`;
  **import-loaded from CLDR**.
- **`person_languages`** / **`tenant_unit_languages`** / **`i18n_locale_languages`** — the cross-module
  links (each carries its owning service's RID).

## Conjure endpoint sketch

`LanguageService` (`api/language.conjure.yml`, base `/language/v1`, `default-auth: header`) — read-only:

- `GET /languages?level=&family=&query=&limit=` → `LanguoidList` (code order; `limit` clamped; the
  catalog is ~27k so narrow with filters).
- `GET /languages/{id}` → `Languoid` (or `LanguoidNotFound`).
- `GET /writing-systems` → `WritingSystemList`.

Writes happen via the generic `POST /import/{objectType}` (object-types `language-scheme` /
`language-scripts`), owned by [dataimport](platform.md)/[hermenea](hermenea.md) — not this module.

## Dependencies

- **Calls:** [platform](platform.md) (pool), [authorization](authorization.md) (PEP — `language.read`).
- **Reads** `geo_countries` (the `language_languoid_countries` tie resolves ISO codes → country RIDs).
- **Called by:** [person](person.md) (`person_languages`), [tenant](tenant.md)
  (`tenant_unit_languages`), [localization](localization.md) (`i18n_locale_languages`) own those link
  tables; this module owns the languoid + writing-system read surface they reference.
- **Written by:** [hermenea](hermenea.md) via the import endpoint (D-Hermenea / D-DataIngestion).

## Authorization touchpoints

All reads require **`language.read`** (a base-reader permission; the registry is instance-global, not
unit-keyed). Imports require **`import.manage`** (instance scope), held by the `hermenea-importer`
service principal — enforced on the generic import endpoint, not here.

## Patterns

- **Reference-Object via import** (D-Hermenea): code-keyed, idempotent, non-destructive upsert;
  per-row provenance; recorded as a `system`-actor audited Action. Mirrors the geo precedent.
- **Strict-tree + closure** (the tenant pattern): structural `parent_id` FK + a maintained closure;
  `family_code` is the denormalized-FK-derived-in-SQL root.
- **Idempotency keyed on `source_version`** (the geo-places pattern): unchanged edition → skip. For the
  live `http-files` source, `source_version` is a checksum of the fetched upstream files, so an
  unchanged master is a no-op.
- **Parent-first load**: the hermenea `glottolog` mapper topologically orders by tree depth so the
  RESTRICT `parent_id` FK always resolves; the import handler rebuilds the closure + `family_code`
  once at the end of the batch (the whole scheme is one transaction).
- **Live-from-upstream (default) with offline fallback**: the bundled sources fetch fresh from upstream
  master each run via hermenea's `http-files` streaming connector — Glottolog CLDF (`languages.csv` +
  `values.csv`) and CLDR (`supplementalData.xml` + `iso-639-3.tab`) are transformed in Go (the
  `CLDFMapper` / `SupplementalMapper` port of `gen-presets.py`), emitting the whole forest as one page so
  the single-transaction closure rebuild holds. Air-gapped installs swap the source to the `file`
  connector + the bundled `deploy/language-presets/*.json`. Tracking master means a run depends on
  upstream reachability/format-stability; a failed run is logged + retried + dead-lettered and never
  corrupts the catalog (imports are transactional).

## Invariants

- A glottocode is unique and stable; `iso639_3` is unique when present (families/dialects have none).
- `parent_id` forms a strict tree (no cycles — Glottolog guarantees this; a parent outside the snapshot
  is dropped so the node lands top-level).
- `person_languages.language_id` always references a `level='language'` languoid (composite FK).
- Imports never delete; a superseded languoid is left in place (re-import only inserts/updates).

## Cross-module link endpoints (D-Languages, M18)

The languoid/writing-system read surface is this module's; the person/unit/locale **links** are owned
by their services and exposed there (each `name` is the languoid's `locale→text` map via `NamesByID`):

- **SPEAKS** ([person](person.md)): `GET|PUT|DELETE /person/v1/persons/{personId}/languages`
  (`languageId` constrained to `level='language'` by the composite FK; `cefrLevel`, `isNative`).
  `person_languages` is `pii:basic` and is erased on person purge.
- **OFFICIAL_LANGUAGE** ([tenant](tenant.md)): `GET|PUT|DELETE /tenant/v1/units/{unitId}/languages`
  (`isOfficial`).
- **LOCALE_OF** ([localization](localization.md)): read-only `GET /localization/v1/locale-languages`
  (the import-reconciled locale→languoid links; not directly editable).

## Open seams / future

- **Facets & dashboards (M58).** [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) lands filters + a stats endpoint + a console dashboard
  for this module's listable types: `GET /languages/stats`; the existing `level`/`family`/`parent`/`topLevel` args become the declared facet set, extended with `macroarea` and endangerment `status` — the status bar orders by **severity, not count**.
  Facets and proposed charts are catalogued in [facets.md](../architecture/facets.md).

- **Richer script sourcing**: `language_writing_systems` currently comes from CLDR `languageData`;
  finer orthography data (ScriptSource) could extend it via another `language-scripts` source.
- **Languoid name picker scale**: the web language picker queries the server `query` filter as you
  type (the catalog is ~27k); a future enhancement could add prefix/relevance ranking server-side.
