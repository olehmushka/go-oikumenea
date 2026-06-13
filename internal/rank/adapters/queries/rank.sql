-- Rank module queries (docs/modules/rank.md). The one system-wide scheme: rank_systems ->
-- rank_categories -> rank_types (a TREE via parent_type_id) -> rank_ranks (on leaf types), strict
-- containment, all soft-deleting. `system_id` is denormalized down onto categories/types/ranks
-- (D-RankSystems) and derived from the parent on insert. `code` is immutable; `sort_order` is
-- app-managed (omit on insert to append last = max active sibling order + 1, scoped to the sibling
-- group). Reads order by (sort_order, code) so seniority is deterministic. `grade_code` (on a rank)
-- is the optional standardized cross-system grade (NATO STANAG 2116; the rank_grades catalog).

-- ============================ systems ============================

-- name: InsertSystem :one
-- Create a rank system (the top level). The RID PK defaults at the database; sort_order appends last.
INSERT INTO oikumenea.rank_systems (code, name, sort_order, country)
VALUES (@code, @name, COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0) FROM oikumenea.rank_systems WHERE deleted_at IS NULL)
), sqlc.narg('country'))
RETURNING *;

-- name: GetSystem :one
SELECT * FROM oikumenea.rank_systems WHERE id = @id AND deleted_at IS NULL;

-- name: GetSystemByCode :one
SELECT * FROM oikumenea.rank_systems WHERE code = @code AND deleted_at IS NULL;

-- name: UpdateSystem :one
-- Partial update: a NULL narg leaves the stored value unchanged. `code` is immutable; `country`
-- cannot be cleared via this path (open seam).
UPDATE oikumenea.rank_systems SET
  name       = COALESCE(sqlc.narg('name'), name),
  sort_order = COALESCE(sqlc.narg('sort_order')::int, sort_order),
  country    = COALESCE(sqlc.narg('country'), country)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteSystem :one
UPDATE oikumenea.rank_systems SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING id;

-- name: ListSystems :many
SELECT * FROM oikumenea.rank_systems WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: CountActiveCategoriesInSystem :one
SELECT count(*)::int AS category_count
FROM oikumenea.rank_categories WHERE system_id = @system_id AND deleted_at IS NULL;

-- ============================ grades ============================

-- name: ListGrades :many
-- The standardized cross-system comparability catalog (NATO STANAG 2116), seeded in the migration.
-- Ordered by the comparability scale: tier (enlisted, warrant, officer) then ordinal.
SELECT * FROM oikumenea.rank_grades
ORDER BY CASE tier WHEN 'enlisted' THEN 0 WHEN 'warrant' THEN 1 ELSE 2 END, ordinal;

-- name: GetGrade :one
SELECT * FROM oikumenea.rank_grades WHERE code = @code;

-- ============================ categories ============================

-- name: InsertCategory :one
-- Create a category under a system. The RID PK defaults at the database; sort_order appends last among
-- the system's active categories.
INSERT INTO oikumenea.rank_categories (system_id, code, name, sort_order)
VALUES (@system_id, @code, @name, COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0)
     FROM oikumenea.rank_categories WHERE system_id = @system_id AND deleted_at IS NULL)
))
RETURNING *;

-- name: GetCategory :one
SELECT * FROM oikumenea.rank_categories WHERE id = @id AND deleted_at IS NULL;

-- name: GetCategoryByCodeInSystem :one
SELECT * FROM oikumenea.rank_categories
WHERE system_id = @system_id AND code = @code AND deleted_at IS NULL;

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
-- parent_type_id NULL = a root type of the category. system_id is denormalized from the category (so a
-- nested type's system equals its root category's). sort_order appends last among ACTIVE SIBLINGS (same
-- category + same parent), comparing parent with IS NOT DISTINCT FROM so NULL matches NULL.
INSERT INTO oikumenea.rank_types (system_id, category_id, parent_type_id, code, name, sort_order)
VALUES (
  (SELECT system_id FROM oikumenea.rank_categories WHERE id = @category_id),
  @category_id, sqlc.narg('parent_type_id')::uuid, @code, @name, COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0)
     FROM oikumenea.rank_types
     WHERE category_id = @category_id
       AND parent_type_id IS NOT DISTINCT FROM sqlc.narg('parent_type_id')::uuid
       AND deleted_at IS NULL)
))
RETURNING *;

-- name: GetType :one
SELECT * FROM oikumenea.rank_types WHERE id = @id AND deleted_at IS NULL;

-- name: GetTypeByCodeInParent :one
-- Find a type by code among its active siblings (same category + same parent), matching NULL parent
-- with NULL via IS NOT DISTINCT FROM.
SELECT * FROM oikumenea.rank_types
WHERE category_id = @category_id
  AND parent_type_id IS NOT DISTINCT FROM sqlc.narg('parent_type_id')::uuid
  AND code = @code AND deleted_at IS NULL;

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
-- All active types across the scheme; the transport weaves the per-category tree from parent_type_id.
-- Ordered by (category_id, sort_order, code) so siblings within any parent come out in seniority order.
SELECT * FROM oikumenea.rank_types WHERE deleted_at IS NULL ORDER BY category_id, sort_order, code;

-- name: CountActiveRanksInType :one
SELECT count(*)::int AS rank_count
FROM oikumenea.rank_ranks WHERE type_id = @type_id AND deleted_at IS NULL;

-- name: CountActiveChildTypes :one
-- Active direct child types of a type — used for the leaf-only rule (no ranks under a type with
-- children) and the delete-in-use check (a type with active children cannot be removed).
SELECT count(*)::int AS child_count
FROM oikumenea.rank_types WHERE parent_type_id = @parent_type_id AND deleted_at IS NULL;

-- ============================ ranks ============================

-- name: InsertRank :one
-- system_id is denormalized from the owning type (so a rank's system equals its type's). grade_code is
-- the optional standardized cross-system grade (validated against rank_grades by the application).
INSERT INTO oikumenea.rank_ranks (system_id, type_id, code, name, abbreviation, grade_code, sort_order)
VALUES (
  (SELECT system_id FROM oikumenea.rank_types WHERE id = @type_id),
  @type_id, @code, @name, sqlc.narg('abbreviation'), sqlc.narg('grade_code'), COALESCE(
  sqlc.narg('sort_order')::int,
  (SELECT COALESCE(max(sort_order) + 1, 0)
     FROM oikumenea.rank_ranks WHERE type_id = @type_id AND deleted_at IS NULL)
))
RETURNING *;

-- name: GetRank :one
SELECT * FROM oikumenea.rank_ranks WHERE id = @id AND deleted_at IS NULL;

-- name: GetRankByCodeInType :one
SELECT * FROM oikumenea.rank_ranks
WHERE type_id = @type_id AND code = @code AND deleted_at IS NULL;

-- name: UpdateRank :one
-- A NULL narg leaves the value unchanged; abbreviation and grade_code cannot be cleared via this path
-- (open seam).
UPDATE oikumenea.rank_ranks SET
  name         = COALESCE(sqlc.narg('name'), name),
  abbreviation = COALESCE(sqlc.narg('abbreviation'), abbreviation),
  grade_code   = COALESCE(sqlc.narg('grade_code'), grade_code),
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
