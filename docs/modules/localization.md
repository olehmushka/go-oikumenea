# Module: localization

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.i18n_*`

## Purpose

Owns **i18n** for the deployment: the **supported-locale registry** and the **translation
store** for the human-facing, **translatable `name`/title/description** of other modules'
structural entities. i18n is a required feature (D-i18n): a deployment ships with **`ukr` and
`eng`** out of the box, and the **instance admin can add more locales and supply translations**.
Translatable text is **always returned in every response** as a locale→text map — there is **no
Accept-Language negotiation** (D-i18n). This is distinct from each entity's stable
locale-agnostic **`code`** (D-Code), which lives on the entity itself and is what external
systems reference.

What this module does **not** own: the `code` of any entity (those live on the owning entity);
**person name transliteration** (that is per-person data in [person](person.md), not
admin-managed translations).

## Entities & aggregates

- **Locale** — a supported language for the deployment: ISO 639-3 code, display name, enabled
  flag, default flag, ordering. Instance-admin-managed; seeded with `ukr` + `eng`.
- **Translation** — one `(entity_type, entity_id, field, locale) → text` row: the localized
  value of a named translatable field of some other module's entity.

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`i18n_locales`**
- `id` PK
- `code TEXT NOT NULL UNIQUE` — ISO 639-3 (e.g. `ukr`, `eng`); locale-agnostic identifier
- `name TEXT NOT NULL` — endonym/display name (e.g. "Українська", "English")
- `enabled BOOLEAN NOT NULL DEFAULT TRUE`
- `is_default BOOLEAN NOT NULL DEFAULT FALSE` — exactly one default among enabled locales
- `sort_order INT NOT NULL`
- `created_at`, `updated_at`, `deleted_at`
- Partial unique: `UNIQUE (is_default) WHERE is_default AND deleted_at IS NULL` is not directly
  expressible; enforce "exactly one default enabled locale" in the application + a constraint
  trigger.

**`i18n_translations`** (shared, polymorphic store)
- `id` PK
- `entity_type TEXT NOT NULL` — e.g. `unit`, `rank_category`, `rank_type`, `rank`, `position`,
  `role`
- `entity_id UUID NOT NULL` — the owning entity's id (polymorphic; no FK — see Invariants)
- `field TEXT NOT NULL` — the translatable field key, e.g. `name`, `title`, `description`
- `locale TEXT NOT NULL REFERENCES i18n_locales(code) ON UPDATE RESTRICT`
- `text TEXT NOT NULL`
- `created_at`, `updated_at`
- `UNIQUE (entity_type, entity_id, field, locale)`
- Indexes: `(entity_type, entity_id)` (the hot read — fetch all translations of one entity),
  `(locale)`.

Each translatable entity also keeps a non-translatable **fallback** value in its own `name`
column (the value in the default locale) so a response is never empty even before translations
are entered; the locale→text map in responses is `{default_locale: name} ∪ i18n_translations`.

## Conjure API surface

`LocalizationService`:

| Op | Intent | Perm |
|---|---|---|
| `GET /locales` | List supported locales | `locale.read` |
| `POST /locales` | Add a locale | `locale.manage` (instance) |
| `PUT /locales/{code}` | Enable/disable, rename, set default, reorder | `locale.manage` (instance) |
| `GET /translations/{entityType}/{entityId}` | All translations of one entity (for editing) | `translation.read` |
| `PUT /translations/{entityType}/{entityId}` | Upsert translations (one or many fields/locales) | `translation.manage` (instance) |

Other modules do **not** call these endpoints to render — they assemble locale→text maps
in-process via the localization application service (below).

## How other modules use it

- A translatable response field is a **map `locale → text`** (Conjure object), built by the
  owning module's transport from `entity.name` (default-locale fallback) + the entity's
  `i18n_translations` rows. Helper on the localization application service:
  `TranslationsFor(entityType, entityIds[], fields[]) → map keyed by (id, field) → (locale→text)`
  for batch list endpoints.
- On entity create/update, the owning module writes the default-locale value to its own `name`
  column; translations are managed separately by the instance admin via `LocalizationService`.

## Dependencies

- **Calls:** [platform](platform.md) for infra; subscribes to other modules' delete events to
  purge orphaned translations.
- **Called by:** [tenant](tenant.md), [rank](rank.md), [membership](membership.md) (positions),
  [authorization](authorization.md) (roles) — in-process, to assemble localized responses and to
  validate `locale` codes. [audit](audit.md) records locale/translation changes.

## Authorization touchpoints

Defines/gates (all in authorization's code catalog): `locale.read`, `locale.manage`
(**instance-scope**), `translation.read`, `translation.manage` (**instance-scope**). Managing
languages and translations is an instance-admin capability — "super admin manages it."

## Invariants & safety

- **At least one enabled locale, exactly one default** among enabled locales (constraint
  trigger).
- **`ukr` + `eng` seeded** at install; the operator may add/disable others but cannot leave zero
  enabled.
- **No domain language is hardcoded-excluded** — drafts' "no Russian locale" rule is dropped;
  the operator chooses (D-i18n).
- **Polymorphic `entity_id` has no DB FK** (it spans many tables). Integrity is kept by: the
  owning module emitting a delete event that localization consumes to remove that entity's
  translations; orphan translations are otherwise harmless (never read without their entity).
- Translations reference a valid, existing locale `code`.

## Open seams / future

- A per-entity typed translation table (FK-integrity) could replace the shared polymorphic store
  if the polymorphic model ever proves limiting — an additive, mechanical migration.
- A bulk import/export (e.g. for translation tooling / TMS) sits naturally on
  `LocalizationService`.
- If Accept-Language negotiation is ever wanted (it is intentionally **not** today), it is an
  additive read option layered over the same store.
