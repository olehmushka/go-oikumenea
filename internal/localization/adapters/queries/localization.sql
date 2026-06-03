-- Localization module queries (docs/modules/localization.md). The supported-locale registry and
-- the polymorphic translation store (D-i18n). Locales soft-delete; translations are upserted and
-- purged with their owning entity (future event seam).

-- name: ListLocales :many
-- All supported locales in display order.
SELECT * FROM oikumenea.i18n_locales
WHERE deleted_at IS NULL
ORDER BY sort_order, code;

-- name: GetLocaleByCode :one
-- One locale by its ISO 639-3 code.
SELECT * FROM oikumenea.i18n_locales
WHERE code = @code AND deleted_at IS NULL;

-- name: InsertLocale :one
-- Add a supported locale. The RID PK defaults at the database; the unique code guards duplicates.
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order)
VALUES (@code, @name, @enabled, @is_default, @sort_order)
RETURNING *;

-- name: ClearDefaultLocales :exec
-- Unset is_default on every enabled, non-deleted locale (run before promoting a new default).
UPDATE oikumenea.i18n_locales
SET is_default = false
WHERE is_default AND deleted_at IS NULL;

-- name: UpdateLocale :one
-- Partial update: a NULL narg leaves the stored value unchanged (COALESCE).
UPDATE oikumenea.i18n_locales SET
  name       = COALESCE(sqlc.narg('name'), name),
  enabled    = COALESCE(sqlc.narg('enabled'), enabled),
  is_default = COALESCE(sqlc.narg('is_default'), is_default),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE code = @code AND deleted_at IS NULL
RETURNING *;

-- name: ExistingLocaleCodes :many
-- The subset of the given codes that are known, enabled, non-deleted locales.
SELECT code FROM oikumenea.i18n_locales
WHERE code = ANY(@codes::text[]) AND enabled AND deleted_at IS NULL;

-- name: GetTranslationsForEntity :many
-- All translations of one entity (for editing), ordered for a stable response.
SELECT * FROM oikumenea.i18n_translations
WHERE entity_type = @entity_type AND entity_id = @entity_id
ORDER BY field, locale;

-- name: UpsertTranslation :exec
-- Insert or replace one (entity_type, entity_id, field, locale) -> text row.
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
VALUES (@entity_type, @entity_id, @field, @locale, @text)
ON CONFLICT (entity_type, entity_id, field, locale)
DO UPDATE SET text = EXCLUDED.text;

-- name: TranslationsForBatch :many
-- Batch read for the in-process assembly helper: every translation of the given entities' fields.
SELECT * FROM oikumenea.i18n_translations
WHERE entity_type = @entity_type
  AND entity_id = ANY(@entity_ids::text[])
  AND field = ANY(@fields::text[]);
