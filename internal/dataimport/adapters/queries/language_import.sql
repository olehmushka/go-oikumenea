-- language-scheme + language-scripts import upserts (D-Languages, M18). The Glottolog languoid forest
-- and the CLDR language→writing-system links arrive over POST /import/{objectType} and are upserted
-- code-keyed, idempotently, and non-destructively. Languoids are keyed on source_version (a re-import
-- with the same Glottolog edition skips); parent + country natural keys resolve to RIDs in SQL.

-- name: BulkUpsertLanguoids :many
-- Set-based chunk merge (R-05): one INSERT … SELECT over the chunk's parallel arrays replaces the
-- per-record loop. Insert absent glottocodes, update rows whose stored source_version differs from
-- the incoming edition, and leave the rest untouched — never deletes. parent_id is deliberately NOT
-- written here (Glottolog families nest arbitrarily deep, so a parent may sit in the same chunk):
-- BulkSetLanguoidParents resolves it in a second pass over the rows this merge reports as
-- created/updated. Latitude/longitude cross as text ('' = NULL) so one array can carry absent values.
WITH r AS (
  SELECT unnest(@codes::text[])      AS code,
         unnest(@levels::text[])     AS level,
         unnest(@names::text[])      AS name,
         unnest(@iso639_3s::text[])  AS iso639_3,
         unnest(@macroareas::text[]) AS macroarea,
         unnest(@latitudes::text[])  AS latitude,
         unnest(@longitudes::text[]) AS longitude,
         unnest(@statuses::text[])   AS status
)
INSERT INTO oikumenea.language_languoids (
  code, level, name, iso639_3, macroarea, latitude, longitude, status,
  glottolog_version, source, source_version, imported_at, origin)
SELECT r.code,
       r.level,
       r.name,
       NULLIF(r.iso639_3, ''),
       NULLIF(r.macroarea, ''),
       NULLIF(r.latitude, '')::double precision,
       NULLIF(r.longitude, '')::double precision,
       r.status,
       NULLIF(@source_version::text, ''),
       @source::text,
       @source_version::text,
       @imported_at::timestamptz,
       'seeded'::text  -- import-path rows are pinax seeded-owned (D-Pinax, M45)
FROM r
ON CONFLICT (code) DO UPDATE SET
  level             = EXCLUDED.level,
  name              = EXCLUDED.name,
  iso639_3          = EXCLUDED.iso639_3,
  macroarea         = EXCLUDED.macroarea,
  latitude          = EXCLUDED.latitude,
  longitude         = EXCLUDED.longitude,
  status            = EXCLUDED.status,
  glottolog_version = EXCLUDED.glottolog_version,
  source            = EXCLUDED.source,
  source_version    = EXCLUDED.source_version,
  imported_at       = EXCLUDED.imported_at
WHERE oikumenea.language_languoids.source_version IS DISTINCT FROM EXCLUDED.source_version
RETURNING code, (xmax = 0) AS inserted;

-- name: BulkInsertLanguoidsAbsent :many
-- The CreateOnly (pinax boot-autoseed, D-Pinax) variant of BulkUpsertLanguoids: insert absent rows,
-- NEVER touch an existing one (no conflict update at all). Returns the created codes.
WITH r AS (
  SELECT unnest(@codes::text[])      AS code,
         unnest(@levels::text[])     AS level,
         unnest(@names::text[])      AS name,
         unnest(@iso639_3s::text[])  AS iso639_3,
         unnest(@macroareas::text[]) AS macroarea,
         unnest(@latitudes::text[])  AS latitude,
         unnest(@longitudes::text[]) AS longitude,
         unnest(@statuses::text[])   AS status
)
INSERT INTO oikumenea.language_languoids (
  code, level, name, iso639_3, macroarea, latitude, longitude, status,
  glottolog_version, source, source_version, imported_at, origin)
SELECT r.code,
       r.level,
       r.name,
       NULLIF(r.iso639_3, ''),
       NULLIF(r.macroarea, ''),
       NULLIF(r.latitude, '')::double precision,
       NULLIF(r.longitude, '')::double precision,
       r.status,
       NULLIF(@source_version::text, ''),
       @source::text,
       @source_version::text,
       @imported_at::timestamptz,
       'seeded'::text
FROM r
ON CONFLICT (code) DO NOTHING
RETURNING code;

-- name: BulkSetLanguoidParents :exec
-- Second pass of the chunk merge: resolve each touched languoid's parent glottocode to its RID. Runs
-- after the bulk upsert in the same transaction, so a parent inserted by this very chunk resolves. A
-- non-empty parent that does not resolve falls back to the sentinel uuid so the parent_id FK fails
-- loudly (RESTRICT) rather than silently NULLing a real, but not-yet-loaded, parent reference.
UPDATE oikumenea.language_languoids l SET
  parent_id = CASE WHEN r.parent_code = '' THEN NULL
                   ELSE COALESCE((SELECT p.id FROM oikumenea.language_languoids p WHERE p.code = r.parent_code),
                                 '00000000-0000-0000-0000-000000000000'::uuid) END
FROM (SELECT unnest(@codes::text[])        AS code,
             unnest(@parent_codes::text[]) AS parent_code) r
WHERE l.code = r.code;

-- name: BulkDeleteLanguoidCountries :exec
-- Clear the country ties of every touched languoid ahead of the bulk re-insert (the set-based
-- ReplaceCountries half; both run in the chunk transaction).
DELETE FROM oikumenea.language_languoid_countries
WHERE languoid_id IN (SELECT id FROM oikumenea.language_languoids WHERE code = ANY(@codes::text[]));

-- name: BulkInsertLanguoidCountries :exec
-- Re-insert the touched languoids' country ties from flattened (code, country) pairs, resolving both
-- natural keys to RIDs. A country code that does not resolve yields no row (the join drops it) — the
-- tie is silently dropped rather than failing, matching InsertLanguoidCountry.
INSERT INTO oikumenea.language_languoid_countries (languoid_id, country_id)
SELECT l.id, c.id
FROM (SELECT unnest(@codes::text[])         AS code,
             unnest(@country_codes::text[]) AS country_code) r
JOIN oikumenea.language_languoids l ON l.code = r.code
JOIN oikumenea.geo_countries c ON c.code = r.country_code
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
