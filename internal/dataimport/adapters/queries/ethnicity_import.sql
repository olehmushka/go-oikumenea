-- ethnicity-scheme import upserts (D-PhysicalIdentity amendment, M43). A hierarchical ethnicity
-- taxonomy with group-level language + country links arrives over POST /import/ethnicity-scheme and is
-- upserted code-keyed, idempotently, non-destructively (mirrors language-scheme). Records arrive
-- parent-first; parent + language + country natural keys resolve to RIDs in SQL. The catalog is plaintext
-- reference data — the person's SELECTION (person_ethnicities) is unaffected by this import.

-- name: GetEthnicityVersion :one
SELECT source_version FROM oikumenea.person_ethnicity_types WHERE code = $1 AND deleted_at IS NULL;

-- name: InsertEthnicityImport :exec
-- Resolve the parent code to its RID; a non-empty parent that does not resolve falls back to a sentinel
-- uuid so the parent_id FK fails loudly (RESTRICT) rather than silently NULLing a not-yet-loaded parent
-- (records must arrive parent-first).
INSERT INTO oikumenea.person_ethnicity_types (
  code, name, parent_id, wikidata_id, status, source, source_version, imported_at)
SELECT sqlc.arg(code)::text,
       sqlc.arg(name)::text,
       CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
            ELSE COALESCE((SELECT p.id FROM oikumenea.person_ethnicity_types p
                           WHERE p.code = sqlc.arg(parent_code)::text AND p.deleted_at IS NULL),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       NULLIF(sqlc.arg(wikidata_id)::text, ''),
       'active',
       sqlc.arg(source)::text,
       sqlc.arg(source_version)::text,
       sqlc.arg(imported_at)::timestamptz;

-- name: UpdateEthnicityImport :exec
UPDATE oikumenea.person_ethnicity_types SET
  name           = sqlc.arg(name)::text,
  parent_id      = CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
                        ELSE COALESCE((SELECT p.id FROM oikumenea.person_ethnicity_types p
                                       WHERE p.code = sqlc.arg(parent_code)::text AND p.deleted_at IS NULL),
                                      '00000000-0000-0000-0000-000000000000'::uuid) END,
  wikidata_id    = NULLIF(sqlc.arg(wikidata_id)::text, ''),
  status         = 'active',
  source         = sqlc.arg(source)::text,
  source_version = sqlc.arg(source_version)::text,
  imported_at    = sqlc.arg(imported_at)::timestamptz
WHERE code = sqlc.arg(code)::text AND deleted_at IS NULL;

-- name: DeleteEthnicityLanguages :exec
DELETE FROM oikumenea.person_ethnicity_type_languages
WHERE ethnicity_type_id = (SELECT id FROM oikumenea.person_ethnicity_types WHERE code = $1 AND deleted_at IS NULL);

-- name: InsertEthnicityLanguage :exec
-- Insert one ethnicity↔language tie, resolving both natural keys to RIDs. The language key may be a
-- Glottolog code OR an ISO-639-3 code (the mapper projects whichever Wikidata carries). A key that does
-- not resolve yields no row (empty SELECT) — the tie is silently dropped (resilient), not fatal.
INSERT INTO oikumenea.person_ethnicity_type_languages (ethnicity_type_id, language_id)
SELECT e.id, l.id
FROM oikumenea.person_ethnicity_types e, oikumenea.language_languoids l
WHERE e.code = sqlc.arg(code)::text AND e.deleted_at IS NULL
  AND (l.code = sqlc.arg(language_key)::text OR l.iso639_3 = sqlc.arg(language_key)::text)
ON CONFLICT (ethnicity_type_id, language_id) DO NOTHING;

-- name: DeleteEthnicityCountries :exec
DELETE FROM oikumenea.person_ethnicity_type_countries
WHERE ethnicity_type_id = (SELECT id FROM oikumenea.person_ethnicity_types WHERE code = $1 AND deleted_at IS NULL);

-- name: InsertEthnicityCountry :exec
-- Insert one ethnicity↔country tie, resolving both natural keys. A country code that does not resolve
-- yields no row — silently dropped rather than failing.
INSERT INTO oikumenea.person_ethnicity_type_countries (ethnicity_type_id, country_id)
SELECT e.id, c.id
FROM oikumenea.person_ethnicity_types e, oikumenea.geo_countries c
WHERE e.code = sqlc.arg(code)::text AND e.deleted_at IS NULL
  AND c.code = sqlc.arg(country_code)::text
ON CONFLICT (ethnicity_type_id, country_id) DO NOTHING;

-- name: ClearEthnicityClosure :exec
-- Truncate the closure ahead of a rebuild (kept separate from the INSERT so the rebuild never
-- self-conflicts on the PK). Runs in the import transaction, so clear+rebuild stays atomic.
DELETE FROM oikumenea.person_ethnicity_type_closure;

-- name: RebuildEthnicityClosure :exec
-- Recompute the full transitive closure (mirrors language_languoid_closure): reflexive (e,e,0) plus
-- every ancestor→descendant pair down the tree. Run after ClearEthnicityClosure, once at import end.
WITH RECURSIVE c AS (
  SELECT id AS ancestor_id, id AS descendant_id, 0 AS depth
  FROM oikumenea.person_ethnicity_types WHERE deleted_at IS NULL
  UNION ALL
  SELECT c.ancestor_id, e.id, c.depth + 1
  FROM c JOIN oikumenea.person_ethnicity_types e ON e.parent_id = c.descendant_id AND e.deleted_at IS NULL
)
INSERT INTO oikumenea.person_ethnicity_type_closure (ancestor_id, descendant_id, depth)
SELECT ancestor_id, descendant_id, depth FROM c;
