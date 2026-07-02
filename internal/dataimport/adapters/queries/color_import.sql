-- colors import upsert (D-Color + D-Pinax, M45). The per-domain platform_colors palette arrives over
-- POST /import/colors and is upserted (domain, code)-keyed, idempotently, non-destructively: insert when
-- absent, update only when name/hex changed, skip otherwise — never deletes. Provenance is not stamped on
-- platform_colors (no source columns); origin='seeded' marks preset ownership (D-Pinax). The bundled
-- `colors` preset seeds the eye/hair/vehicle palettes plus the new rank/religion/ethnicity/country
-- palettes that the seeded reference catalogs point at via color_id.

-- name: GetColor :one
SELECT name, hex FROM oikumenea.platform_colors
WHERE domain = $1 AND code = $2 AND deleted_at IS NULL;

-- name: InsertColorImport :exec
INSERT INTO oikumenea.platform_colors (domain, code, name, hex, sort_order, origin)
VALUES (sqlc.arg(domain)::text, sqlc.arg(code)::text, sqlc.arg(name)::text,
        NULLIF(sqlc.arg(hex)::text, ''), sqlc.narg(sort_order)::int,
        'seeded');  -- import-path rows are pinax seeded-owned (D-Pinax, M45)

-- name: UpdateColorImport :exec
UPDATE oikumenea.platform_colors SET
  name       = sqlc.arg(name)::text,
  hex        = NULLIF(sqlc.arg(hex)::text, ''),
  sort_order = sqlc.narg(sort_order)::int,
  status     = 'active'
WHERE domain = sqlc.arg(domain)::text AND code = sqlc.arg(code)::text AND deleted_at IS NULL;
