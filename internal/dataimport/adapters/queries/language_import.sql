-- language-scheme + language-scripts import upserts (D-Languages, M18). The Glottolog languoid forest
-- and the CLDR language→writing-system links arrive over POST /import/{objectType} and are upserted
-- code-keyed, idempotently, and non-destructively. Languoids are keyed on source_version (a re-import
-- with the same Glottolog edition skips); parent + country natural keys resolve to RIDs in SQL.

-- name: GetLanguoidVersion :one
SELECT source_version FROM oikumenea.language_languoids WHERE code = $1;

-- name: InsertLanguoidImport :exec
-- Resolve the parent glottocode to its RID; a non-empty parent that does not resolve falls back to a
-- sentinel uuid so the parent_id FK fails loudly (RESTRICT) rather than silently NULLing a real but
-- not-yet-loaded parent (records must arrive parent-first).
INSERT INTO oikumenea.language_languoids (
  code, level, name, parent_id, iso639_3, macroarea, latitude, longitude, status,
  glottolog_version, source, source_version, imported_at)
SELECT sqlc.arg(code)::text,
       sqlc.arg(level)::text,
       sqlc.arg(name)::text,
       CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
            ELSE COALESCE((SELECT p.id FROM oikumenea.language_languoids p WHERE p.code = sqlc.arg(parent_code)::text),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       NULLIF(sqlc.arg(iso639_3)::text, ''),
       NULLIF(sqlc.arg(macroarea)::text, ''),
       sqlc.narg(latitude)::double precision,
       sqlc.narg(longitude)::double precision,
       sqlc.arg(status)::text,
       NULLIF(sqlc.arg(source_version)::text, ''),
       sqlc.arg(source)::text,
       sqlc.arg(source_version)::text,
       sqlc.arg(imported_at)::timestamptz;

-- name: UpdateLanguoidImport :exec
UPDATE oikumenea.language_languoids SET
  level             = sqlc.arg(level)::text,
  name              = sqlc.arg(name)::text,
  parent_id         = CASE WHEN NULLIF(sqlc.arg(parent_code)::text, '') IS NULL THEN NULL
                           ELSE COALESCE((SELECT p.id FROM oikumenea.language_languoids p WHERE p.code = sqlc.arg(parent_code)::text),
                                         '00000000-0000-0000-0000-000000000000'::uuid) END,
  iso639_3          = NULLIF(sqlc.arg(iso639_3)::text, ''),
  macroarea         = NULLIF(sqlc.arg(macroarea)::text, ''),
  latitude          = sqlc.narg(latitude)::double precision,
  longitude         = sqlc.narg(longitude)::double precision,
  status            = sqlc.arg(status)::text,
  glottolog_version = NULLIF(sqlc.arg(source_version)::text, ''),
  source            = sqlc.arg(source)::text,
  source_version    = sqlc.arg(source_version)::text,
  imported_at       = sqlc.arg(imported_at)::timestamptz
WHERE code = sqlc.arg(code)::text;

-- name: DeleteLanguoidCountries :exec
DELETE FROM oikumenea.language_languoid_countries
WHERE languoid_id = (SELECT id FROM oikumenea.language_languoids WHERE code = $1);

-- name: InsertLanguoidCountry :exec
-- Insert one languoid↔country tie, resolving both natural keys to RIDs. A country code that does not
-- resolve yields no row (the SELECT is empty) — the tie is silently dropped rather than failing.
INSERT INTO oikumenea.language_languoid_countries (languoid_id, country_id)
SELECT l.id, c.id
FROM oikumenea.language_languoids l, oikumenea.geo_countries c
WHERE l.code = sqlc.arg(code)::text AND c.code = sqlc.arg(country_code)::text
ON CONFLICT (languoid_id, country_id) DO NOTHING;

-- name: ClearLanguoidClosure :exec
-- Truncate the closure ahead of a rebuild. Kept SEPARATE from the INSERT (not a single DELETE+INSERT
-- CTE) so the rebuild never self-conflicts on the unique (ancestor_id, descendant_id) PK when the
-- closure already holds rows that get re-inserted (a re-import of a changed edition). Both run in the
-- import transaction, so the clear+rebuild stays atomic.
DELETE FROM oikumenea.language_languoid_closure;

-- name: RebuildLanguoidClosure :exec
-- Recompute the full transitive closure (mirrors tenant_unit_closure): reflexive (u,u,0) plus every
-- ancestor→descendant pair down the strict tree. Run after ClearLanguoidClosure, once at the end of a
-- language-scheme import.
WITH RECURSIVE c AS (
  SELECT id AS ancestor_id, id AS descendant_id, 0 AS depth FROM oikumenea.language_languoids
  UNION ALL
  SELECT c.ancestor_id, l.id, c.depth + 1
  FROM c JOIN oikumenea.language_languoids l ON l.parent_id = c.descendant_id
)
INSERT INTO oikumenea.language_languoid_closure (ancestor_id, descendant_id, depth)
SELECT ancestor_id, descendant_id, depth FROM c;

-- name: RebuildLanguoidFamilyCodes :exec
-- Derive each languoid's root-family glottocode from the closure (the ancestor with parent_id IS NULL;
-- exactly one per tree, the reflexive row for a root family itself).
UPDATE oikumenea.language_languoids l SET family_code = root.code
FROM (
  SELECT c.descendant_id, a.code
  FROM oikumenea.language_languoid_closure c
  JOIN oikumenea.language_languoids a ON a.id = c.ancestor_id
  WHERE a.parent_id IS NULL
) root
WHERE l.id = root.descendant_id
  AND l.family_code IS DISTINCT FROM root.code;

-- name: ReconcileLocaleLanguages :exec
-- Populate the i18n_locale_languages link (D-Languages / D-i18n): match each supported UI locale's
-- ISO-639-3 code to the Glottolog languoid carrying that iso639_3 (e.g. ukr→Ukrainian, eng→English).
-- Self-healing and idempotent — run once at the end of a language-scheme import, after languoids exist.
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT loc.code, x.id
FROM oikumenea.i18n_locales loc
JOIN oikumenea.language_languoids x ON x.iso639_3 = loc.code
ON CONFLICT (locale) DO UPDATE SET language_id = EXCLUDED.language_id, updated_at = now();

-- name: ResolveLanguoidByISO :one
SELECT id FROM oikumenea.language_languoids WHERE iso639_3 = $1;

-- name: ResolveWritingSystemByCode :one
SELECT id FROM oikumenea.writing_systems WHERE code = $1;

-- name: GetLanguageWritingSystemPrimary :one
SELECT is_primary FROM oikumenea.language_writing_systems
WHERE languoid_id = $1 AND writing_system_id = $2;

-- name: InsertLanguageWritingSystem :exec
INSERT INTO oikumenea.language_writing_systems (
  languoid_id, writing_system_id, is_primary, source, source_version, imported_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: UpdateLanguageWritingSystem :exec
UPDATE oikumenea.language_writing_systems SET
  is_primary     = sqlc.arg(is_primary),
  source         = sqlc.arg(source),
  source_version = sqlc.arg(source_version),
  imported_at    = sqlc.arg(imported_at)
WHERE languoid_id = sqlc.arg(languoid_id) AND writing_system_id = sqlc.arg(writing_system_id);
