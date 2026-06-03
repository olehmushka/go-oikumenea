-- Rank module queries (docs/modules/rank.md). The one system-wide scheme: rank_categories ->
-- rank_types -> rank_ranks, strict containment, all soft-deleting. `code` is immutable; `sort_order`
-- is app-managed (omit on insert to append last = max active sibling order + 1). Reads order by
-- (sort_order, code) so seniority is deterministic.

-- ============================ categories ============================

-- name: InsertCategory :one
-- Create a category. The RID PK defaults at the database; sort_order defaults to appended-last.
INSERT INTO oikumenea.rank_categories (code, name, sort_order)
VALUES (@code, @name, COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0) FROM oikumenea.rank_categories WHERE deleted_at IS NULL)
))
RETURNING *;

-- name: GetCategory :one
SELECT * FROM oikumenea.rank_categories WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateCategory :one
-- Partial update: a NULL narg leaves the stored value unchanged (COALESCE). `code` is immutable.
UPDATE oikumenea.rank_categories SET
  name       = COALESCE(sqlc.narg('name'), name),
  sort_order = COALESCE(sqlc.narg('sort_order')::int, sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteCategory :one
UPDATE oikumenea.rank_categories SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id;

-- name: ListCategories :many
SELECT * FROM oikumenea.rank_categories WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: CountActiveTypesInCategory :one
SELECT count(*)::int AS type_count
FROM oikumenea.rank_types WHERE category_id = @category_id AND deleted_at IS NULL;

-- ============================ types ============================

-- name: InsertType :one
INSERT INTO oikumenea.rank_types (category_id, code, name, sort_order)
VALUES (@category_id, @code, @name, COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0)
     FROM oikumenea.rank_types WHERE category_id = @category_id AND deleted_at IS NULL)
))
RETURNING *;

-- name: GetType :one
SELECT * FROM oikumenea.rank_types WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateType :one
UPDATE oikumenea.rank_types SET
  name       = COALESCE(sqlc.narg('name'), name),
  sort_order = COALESCE(sqlc.narg('sort_order')::int, sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteType :one
UPDATE oikumenea.rank_types SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id;

-- name: ListTypes :many
-- All active types across the scheme; the transport groups by category_id when assembling the tree.
SELECT * FROM oikumenea.rank_types WHERE deleted_at IS NULL ORDER BY category_id, sort_order, code;

-- name: CountActiveRanksInType :one
SELECT count(*)::int AS rank_count
FROM oikumenea.rank_ranks WHERE type_id = @type_id AND deleted_at IS NULL;

-- ============================ ranks ============================

-- name: InsertRank :one
INSERT INTO oikumenea.rank_ranks (type_id, code, name, abbreviation, sort_order)
VALUES (@type_id, @code, @name, sqlc.narg('abbreviation'), COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0)
     FROM oikumenea.rank_ranks WHERE type_id = @type_id AND deleted_at IS NULL)
))
RETURNING *;

-- name: GetRank :one
SELECT * FROM oikumenea.rank_ranks WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateRank :one
-- A NULL narg leaves the value unchanged; abbreviation cannot be cleared via this path (open seam).
UPDATE oikumenea.rank_ranks SET
  name         = COALESCE(sqlc.narg('name'), name),
  abbreviation = COALESCE(sqlc.narg('abbreviation'), abbreviation),
  sort_order   = COALESCE(sqlc.narg('sort_order')::int, sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteRank :one
UPDATE oikumenea.rank_ranks SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id;

-- name: ListRanks :many
-- All active ranks across the scheme; the transport groups by type_id when assembling the tree.
SELECT * FROM oikumenea.rank_ranks WHERE deleted_at IS NULL ORDER BY type_id, sort_order, code;
