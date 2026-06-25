-- Language module queries (docs/modules/language.md; D-Languages, M18). Read-only lookups over the
-- RID-keyed Glottolog languoid forest + ISO-15924 writing-system registry. The catalog is written by
-- the hermenea import pipeline (language-scheme / language-scripts), not here.

-- name: ListLanguoids :many
-- Languoids in code order, optionally filtered by level, root family (family_code), immediate parent
-- (one tree level), top-level-only, and a name/glottocode substring. The empty-string / false
-- sentinels disable each filter; the limit is clamped by the application. has_children flags whether
-- the node has any non-dialect child, so a tree browser can show an expand affordance only where it
-- leads somewhere (family → language; languages whose only children are dialects read as leaves).
SELECT l.id, l.code, l.level, l.name, l.parent_id, l.family_code, l.iso639_3, l.macroarea, l.status,
  EXISTS (
    SELECT 1 FROM oikumenea.language_languoids c
    WHERE c.parent_id = l.id AND c.level <> 'dialect'
  ) AS has_children
FROM oikumenea.language_languoids l
WHERE (sqlc.arg(level)::text = '' OR l.level = sqlc.arg(level)::text)
  AND (sqlc.arg(family)::text = '' OR l.family_code = sqlc.arg(family)::text)
  AND (sqlc.arg(parent)::text = '' OR l.parent_id::text = sqlc.arg(parent)::text)
  AND (NOT sqlc.arg(top_level)::bool OR l.parent_id IS NULL)
  AND (sqlc.arg(q)::text = '' OR l.name ILIKE '%' || sqlc.arg(q)::text || '%' OR l.code ILIKE sqlc.arg(q)::text || '%')
ORDER BY l.code
LIMIT sqlc.arg(lim)::int;

-- name: GetLanguoid :one
SELECT l.id, l.code, l.level, l.name, l.parent_id, l.family_code, l.iso639_3, l.macroarea, l.status,
  EXISTS (
    SELECT 1 FROM oikumenea.language_languoids c
    WHERE c.parent_id = l.id AND c.level <> 'dialect'
  ) AS has_children
FROM oikumenea.language_languoids l WHERE l.id = $1;

-- name: ListWritingSystems :many
SELECT id, code, name, script_type FROM oikumenea.writing_systems ORDER BY code;
