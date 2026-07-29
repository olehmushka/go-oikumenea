-- Order module queries (docs/modules/order.md). The order-type catalog (order_order_types, RID), the
-- order header (order_orders, RID, draft→issued→revoked) and its parent-scoped items
-- (order_order_items, RID, no deleted_at). RID PKs default at the database. A NULL narg leaves the
-- stored value unchanged on update (COALESCE). Existence of the referenced type/person/unit/position/
-- rank is validated by the FKs (mapped in the adapter), so these queries carry no pre-check lookups.

-- ============================ order types ============================

-- name: InsertOrderType :one
-- Add an order type. sort_order, omitted, appends after the catalog's current max among active types.
INSERT INTO oikumenea.order_order_types (code, name, category, effect, sort_order)
VALUES (
  @code, @name, @category, @effect,
  COALESCE(sqlc.narg('sort_order'), (
    SELECT COALESCE(MAX(sort_order), 0) + 1 FROM oikumenea.order_order_types
    WHERE status = 'active' AND deleted_at IS NULL
  ))
)
RETURNING *;

-- name: UpdateOrderType :one
-- Partial update: a NULL narg leaves the value unchanged. code/category/effect are immutable.
UPDATE oikumenea.order_order_types SET
  name       = COALESCE(sqlc.narg('name'), name),
  status     = COALESCE(sqlc.narg('status'), status),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: GetOrderType :one
SELECT * FROM oikumenea.order_order_types WHERE id = @id AND deleted_at IS NULL;

-- name: ListOrderTypes :many
-- The whole catalog (active + retired), deterministic seniority order.
SELECT * FROM oikumenea.order_order_types
WHERE deleted_at IS NULL
ORDER BY sort_order, code;

-- ============================ orders (header) ============================

-- name: InsertOrder :one
-- Create a draft order for an issuing unit. The tenant_units FK validates the unit; the partial-unique
-- (issuing_unit_id, number) index surfaces a duplicate number.
INSERT INTO oikumenea.order_orders (number, issued_on, issuing_unit_id)
VALUES (sqlc.narg('number'), sqlc.narg('issued_on'), @issuing_unit_id)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM oikumenea.order_orders WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateOrderHeader :one
-- Edit a draft order's header (number/date). Only a draft is editable (the status guard); a NULL narg
-- leaves the value unchanged.
UPDATE oikumenea.order_orders SET
  number    = COALESCE(sqlc.narg('number'), number),
  issued_on = COALESCE(sqlc.narg('issued_on'), issued_on)
WHERE id = @id AND status = 'draft' AND deleted_at IS NULL
RETURNING *;

-- name: MarkIssued :one
-- Lock a draft order (the headline legal-basis act); only a draft can be issued.
UPDATE oikumenea.order_orders SET status = 'issued'
WHERE id = @id AND status = 'draft' AND deleted_at IS NULL
RETURNING *;

-- name: MarkRevoked :one
-- Revoke an issued order (legal-status flip); records the optional revoking order. Only an issued order
-- can be revoked.
UPDATE oikumenea.order_orders SET
  status              = 'revoked',
  revoked_at          = now(),
  revoked_by_order_id = sqlc.narg('revoking_order_id')
WHERE id = @id AND status = 'issued' AND deleted_at IS NULL
RETURNING *;

-- ============================ top-level facet-filtered list (M56 ticket 3 / D-ObjectFacets) ============================
-- GET /orders. Two shapes, ONE filter block, byte-identical between them: the admin path and the
-- reach-scoped path must select the same rows for the same filters, differing ONLY by the reach
-- predicate. sqlparity_test.go proves the block is present in both with no database.
--
-- Every predicate is folded INTO the SQL, before the LIMIT (review-2026-07 R-06); a Go-side
-- re-filter after the page is cut would return a short page WITH a nextPageToken. The
-- `sqlc.narg('x')::type IS NULL OR col = ...` style is what R-21 requires — the `(@arg = '' OR ...)`
-- sentinel is banned because the planner cannot prove the arg non-empty under a generic plan.
--
-- order_type_id is an EXISTS over the ITEMS, never a join: an order's effect lives on its items, and
-- joining would multiply the order row across every matching item and corrupt the keyset page.

-- name: ListOrders :many
-- Instance-admin path: every order header, keyset-paginated by RID.
SELECT * FROM oikumenea.order_orders o
WHERE o.deleted_at IS NULL
  AND (@after = '' OR o.id::text > @after)
  AND (sqlc.narg('issuing_unit_id')::uuid IS NULL OR o.issuing_unit_id = sqlc.narg('issuing_unit_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR o.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR o.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR o.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('order_type_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.order_order_items i
        WHERE i.order_id = o.id AND i.type_id = sqlc.narg('order_type_id')::uuid))
ORDER BY o.id
LIMIT @lim;

-- name: ListOrdersForSubject :many
-- Read-scope path: the same set intersected with the subject's effective readable reach on the
-- ISSUING unit. The reach set is UNCORRELATED — it reads only @subject_person_id, so the planner
-- evaluates it once and probes a hash (M56 ticket 2: the correlated form measured 242 ms vs 33 ms).
SELECT * FROM oikumenea.order_orders o
WHERE o.deleted_at IS NULL
  AND (@after = '' OR o.id::text > @after)
  AND (sqlc.narg('issuing_unit_id')::uuid IS NULL OR o.issuing_unit_id = sqlc.narg('issuing_unit_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR o.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR o.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR o.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('order_type_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.order_order_items i
        WHERE i.order_id = o.id AND i.type_id = sqlc.narg('order_type_id')::uuid))
  AND o.issuing_unit_id IN (SELECT oikumenea.authz_readable_units(@subject_person_id))
ORDER BY o.id
LIMIT @lim;

-- name: CountReadableUnitsForDispatch :one
-- The capped reach-cardinality probe the sparse/dense list dispatch reads (migration 0017). Capped,
-- because the question is never "how big is the reach" but "is it past the threshold".
SELECT oikumenea.authz_readable_unit_count(@subject_person_id, @cap::integer) AS n;

-- name: ListOrdersForSubjectDense :many
-- DENSE-reach plan shape of the query above, byte-identical in its filter block and differing ONLY in
-- how reach is applied: a per-row point probe instead of a materialized reach set. See
-- ListMembershipsForSubjectDense and migration 0017 for the measured reason both shapes exist; the
-- adapter dispatches on CountReadableUnitsCapped.
SELECT * FROM oikumenea.order_orders o
WHERE o.deleted_at IS NULL
  AND (@after = '' OR o.id::text > @after)
  AND (sqlc.narg('issuing_unit_id')::uuid IS NULL OR o.issuing_unit_id = sqlc.narg('issuing_unit_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR o.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR o.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR o.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('order_type_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.order_order_items i
        WHERE i.order_id = o.id AND i.type_id = sqlc.narg('order_type_id')::uuid))
  AND oikumenea.authz_unit_readable_by(o.issuing_unit_id, @subject_person_id)
ORDER BY o.id
LIMIT @lim;

-- name: ListOrdersByUnit :many
-- An issuing unit's orders (headers only), keyset-paginated by RID.
SELECT * FROM oikumenea.order_orders
WHERE issuing_unit_id = @issuing_unit_id AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListOrdersByPerson :many
-- Orders affecting a person (via their items), keyset-paginated by RID.
SELECT o.* FROM oikumenea.order_orders o
WHERE o.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM oikumenea.order_order_items i
    WHERE i.order_id = o.id AND i.person_id = @person_id
  )
  AND (@after = '' OR o.id::text > @after)
ORDER BY o.id
LIMIT @lim;

-- ============================ order items (parent-scoped) ============================

-- name: InsertOrderItem :one
-- Add an item to an order. The order_order_types / person / unit / position / rank FKs validate the
-- references; effective_from/to are calendar dates.
INSERT INTO oikumenea.order_order_items
  (order_id, type_id, person_id, unit_id, position_id, rank_id, effective_from, effective_to, note)
VALUES (
  @order_id, @type_id, @person_id,
  sqlc.narg('unit_id'), sqlc.narg('position_id'), sqlc.narg('rank_id'),
  sqlc.narg('effective_from'), sqlc.narg('effective_to'), sqlc.narg('note')
)
RETURNING *;

-- name: GetOrderItems :many
-- An order's items, each joined to its type to carry the effect (drives issue-time event dispatch).
SELECT i.*, t.effect AS type_effect
FROM oikumenea.order_order_items i
JOIN oikumenea.order_order_types t ON t.id = i.type_id
WHERE i.order_id = @order_id
ORDER BY i.id;

-- name: DeleteOrderItems :exec
-- Hard-delete an order's items (draft edit replaces them; draft items have no legal weight / lifecycle).
DELETE FROM oikumenea.order_order_items WHERE order_id = @order_id;
