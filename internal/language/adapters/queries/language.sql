-- Language module queries (docs/modules/language.md; D-Languages, M18). Read-only lookups over the
-- RID-keyed Glottolog languoid forest + ISO-15924 writing-system registry. The catalog is written by
-- the hermenea import pipeline (language-scheme / language-scripts), not here.

-- name: ListLanguoids :many
-- Languoids in code order, optionally filtered by level, root family (family_code), and a
-- name/glottocode substring. The empty-string sentinel disables each filter; the limit is clamped by
-- the application.
SELECT id, code, level, name, parent_id, family_code, iso639_3, macroarea, status
FROM oikumenea.language_languoids
WHERE (sqlc.arg(level)::text = '' OR level = sqlc.arg(level)::text)
  AND (sqlc.arg(family)::text = '' OR family_code = sqlc.arg(family)::text)
  AND (sqlc.arg(q)::text = '' OR name ILIKE '%' || sqlc.arg(q)::text || '%' OR code ILIKE sqlc.arg(q)::text || '%')
ORDER BY code
LIMIT sqlc.arg(lim)::int;

-- name: GetLanguoid :one
SELECT id, code, level, name, parent_id, family_code, iso639_3, macroarea, status
FROM oikumenea.language_languoids WHERE id = $1;

-- name: ListWritingSystems :many
SELECT id, code, name, script_type FROM oikumenea.writing_systems ORDER BY code;
