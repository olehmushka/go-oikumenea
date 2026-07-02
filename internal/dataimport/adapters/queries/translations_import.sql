-- pinax translations import (D-Pinax, M45 + D-i18n). Seeds the polymorphic i18n_translations store
-- (entity_type, entity_id, field, locale) -> text for reference-catalog entities from the bundled
-- `translations` preset. entity_id is resolved per entity_type from the entity's natural key(s) (below);
-- RID-keyed entities (languoid, writing_system, religion_taxon, rank_*) resolve code -> RID, while
-- code-keyed entities (country, ethnicity_type) use the code directly. Inserts are CREATE-IF-ABSENT
-- (ON CONFLICT DO NOTHING) so a re-seed never clobbers an operator-corrected translation (the store has
-- no provenance column — open seam).

-- name: UpsertTranslationSeed :exec
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
VALUES (sqlc.arg(entity_type)::text, sqlc.arg(entity_id)::text, sqlc.arg(field)::text,
        sqlc.arg(locale)::text, sqlc.arg(text)::text)
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- name: ResolveLanguoidRID :one
SELECT id FROM oikumenea.language_languoids WHERE code = sqlc.arg(code)::text;

-- name: ResolveReligionTaxonRID :one
SELECT id FROM oikumenea.religion_taxa WHERE code = sqlc.arg(code)::text AND deleted_at IS NULL;

-- name: ResolveColorRID :one
-- Color translations key on the color RID (platform catalog read path), resolved from (domain, code).
SELECT id FROM oikumenea.platform_colors
WHERE domain = sqlc.arg(domain)::text AND code = sqlc.arg(code)::text AND deleted_at IS NULL;

-- name: ResolveRankCategoryRID :one
SELECT c.id
FROM oikumenea.rank_categories c
JOIN oikumenea.rank_systems s ON s.id = c.system_id AND s.deleted_at IS NULL
WHERE s.code = sqlc.arg(system_code)::text AND c.code = sqlc.arg(category_code)::text
  AND c.deleted_at IS NULL;

-- name: ResolveRankTypeRID :one
SELECT t.id
FROM oikumenea.rank_types t
JOIN oikumenea.rank_categories c ON c.id = t.category_id AND c.deleted_at IS NULL
JOIN oikumenea.rank_systems s ON s.id = t.system_id AND s.deleted_at IS NULL
WHERE s.code = sqlc.arg(system_code)::text AND c.code = sqlc.arg(category_code)::text
  AND t.code = sqlc.arg(type_code)::text AND t.deleted_at IS NULL;

-- name: ResolveRankRID :one
SELECT r.id
FROM oikumenea.rank_ranks r
JOIN oikumenea.rank_types t ON t.id = r.type_id AND t.deleted_at IS NULL
JOIN oikumenea.rank_categories c ON c.id = t.category_id AND c.deleted_at IS NULL
JOIN oikumenea.rank_systems s ON s.id = r.system_id AND s.deleted_at IS NULL
WHERE s.code = sqlc.arg(system_code)::text AND c.code = sqlc.arg(category_code)::text
  AND r.code = sqlc.arg(rank_code)::text AND r.deleted_at IS NULL;
