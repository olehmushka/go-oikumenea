-- Membership module queries (docs/modules/membership.md). Unit-owned billets (membership_positions)
-- and the reified person->unit belonging/filling Link (membership_memberships). RID PKs default at
-- the database. Positions/memberships are reversible: abolish/end flip status rather than delete. A
-- NULL narg leaves the stored value unchanged on update (COALESCE); `code` and unit are immutable.
-- Existence of the referenced person/unit/position/rank is validated by the FKs (mapped in the
-- adapter), so these queries carry no pre-check lookups.

-- ============================ positions ============================

-- name: InsertPosition :one
-- Create a billet in a unit (vacant). The tenant_units / rank_ranks FKs validate unit + required rank.
-- sort_order, omitted, appends after the unit's current max among active positions.
INSERT INTO oikumenea.membership_positions (unit_id, code, title, required_rank_id, sort_order)
VALUES (
  @unit_id, @code, @title, sqlc.narg('required_rank_id'),
  COALESCE(sqlc.narg('sort_order'), (
    SELECT COALESCE(MAX(sort_order), 0) + 1 FROM oikumenea.membership_positions
    WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  ))
)
RETURNING *;

-- name: GetPosition :one
SELECT * FROM oikumenea.membership_positions WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePosition :one
-- Partial update: a NULL narg leaves the value unchanged. `code` and unit_id are immutable; clearing
-- required_rank_id to NULL via this path is an open seam (COALESCE cannot set NULL).
UPDATE oikumenea.membership_positions SET
  title            = COALESCE(sqlc.narg('title'), title),
  required_rank_id = COALESCE(sqlc.narg('required_rank_id'), required_rank_id),
  sort_order       = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: AbolishPosition :one
-- Reversible status flip; only an active position can be abolished. The in-use guard (active filling)
-- is enforced in the application before this runs.
UPDATE oikumenea.membership_positions SET status = 'abolished'
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: ListPositionsByUnit :many
-- All active positions in a unit, keyset-paginated by RID.
SELECT * FROM oikumenea.membership_positions
WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListVacantPositionsByUnit :many
-- Active positions with NO active filling (vacancy = the derived predicate), keyset-paginated.
SELECT p.* FROM oikumenea.membership_positions p
WHERE p.unit_id = @unit_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.position_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- name: ListFilledPositionsByUnit :many
-- Active positions that HAVE an active filling, keyset-paginated.
SELECT p.* FROM oikumenea.membership_positions p
WHERE p.unit_id = @unit_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.position_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- ============================ memberships ============================

-- name: InsertMembership :one
-- Add a belonging/filling. The person/unit/position FKs validate existence; the one-holder and
-- plain-belonging partial-unique indexes enforce single occupancy (mapped in the adapter).
INSERT INTO oikumenea.membership_memberships (
  person_id, unit_id, position_id, order_item_id, effective_from
) VALUES (
  @person_id, @unit_id, sqlc.narg('position_id'), sqlc.narg('order_item_id'),
  COALESCE(sqlc.narg('effective_from')::timestamptz, now())
)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM oikumenea.membership_memberships WHERE id = @id AND deleted_at IS NULL;

-- name: EndMembership :one
-- Reversible end: flip status to ended and stamp effective_to; only an active membership can end.
-- An order_item_id provenance pointer may be attached at end time (NULL narg leaves it unchanged).
UPDATE oikumenea.membership_memberships SET
  status        = 'ended',
  effective_to  = COALESCE(sqlc.narg('effective_to')::timestamptz, now()),
  order_item_id = COALESCE(sqlc.narg('order_item_id'), order_item_id)
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: GetActiveFillingByPosition :one
-- The current holder of a position (its single active filling), if any.
SELECT * FROM oikumenea.membership_memberships
WHERE position_id = @position_id AND status = 'active' AND deleted_at IS NULL;

-- name: GetActivePlainMembership :one
-- A person's active PLAIN belonging (no position) in a unit, if any — the target an order's
-- membership-end item ends when it names a unit but no position. The belonging index keeps it unique.
SELECT * FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND unit_id = @unit_id
  AND position_id IS NULL AND status = 'active' AND deleted_at IS NULL;

-- name: ListMembersByUnit :many
-- A unit's active memberships (its roster), keyset-paginated by RID.
SELECT * FROM oikumenea.membership_memberships
WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListMembershipsByPerson :many
-- A person's active memberships across units, keyset-paginated by RID.
SELECT * FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ActiveUnitIDsByPerson :many
-- The distinct units a person currently belongs to via ACTIVE memberships. The person/document
-- read-scope projection (D-PersonReadScope) intersects this set with the reader's effective readable
-- units to decide visibility.
SELECT DISTINCT unit_id FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND status = 'active' AND deleted_at IS NULL
ORDER BY unit_id;

-- name: ActivePersonIDsInUnits :many
-- The distinct persons with an ACTIVE membership in any of the given units, keyset-paginated by person
-- RID. Powers the directory-list union (GET /persons) under D-PersonReadScope: the caller passes its
-- effective readable unit-set and pages the reachable roster.
SELECT DISTINCT person_id FROM oikumenea.membership_memberships
WHERE unit_id = ANY(@unit_ids::uuid[]) AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR person_id::text > @after)
ORDER BY person_id
LIMIT @lim;
