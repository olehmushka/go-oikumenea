-- Tenant module queries (docs/modules/tenant.md). Units as a DAG per graph + a maintained
-- transitive closure recomputed in the write transaction on every edge change. Units/graphs
-- soft-delete; edges hard-delete on detach; the closure is derived (no RID).

-- ============================ units ============================

-- name: InsertUnit :one
-- Create a unit. The RID PK defaults at the database; the partial-unique code guards duplicates.
INSERT INTO oikumenea.tenant_units (code, name, unit_kind, level, visibility, metadata)
VALUES (@code, @name, sqlc.narg('unit_kind'), sqlc.narg('level'), @visibility, @metadata)
RETURNING *;

-- name: GetUnit :one
SELECT * FROM oikumenea.tenant_units WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateUnit :one
-- Partial update: a NULL narg leaves the stored value unchanged (COALESCE). `code` is immutable.
UPDATE oikumenea.tenant_units SET
  name       = COALESCE(sqlc.narg('name'), name),
  unit_kind  = COALESCE(sqlc.narg('unit_kind'), unit_kind),
  level      = COALESCE(sqlc.narg('level'), level),
  visibility = COALESCE(sqlc.narg('visibility'), visibility),
  metadata   = COALESCE(sqlc.narg('metadata'), metadata)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetUnitState :one
UPDATE oikumenea.tenant_units SET state = @state
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListUnits :many
-- Keyset pagination over the time-ordered RID (id), optional level filter.
SELECT * FROM oikumenea.tenant_units
WHERE deleted_at IS NULL
  AND (sqlc.narg('level')::smallint IS NULL OR level = sqlc.narg('level')::smallint)
  AND (sqlc.narg('after')::uuid IS NULL OR id > sqlc.narg('after')::uuid)
ORDER BY id
LIMIT @lim;

-- ============================ graphs ============================

-- name: InsertGraph :one
INSERT INTO oikumenea.tenant_graphs (code, name, is_authority_bearing)
VALUES (@code, @name, @is_authority_bearing)
RETURNING *;

-- name: GetGraphByID :one
SELECT * FROM oikumenea.tenant_graphs WHERE id = @id AND deleted_at IS NULL;

-- name: GetGraphByCode :one
SELECT * FROM oikumenea.tenant_graphs WHERE code = @code AND deleted_at IS NULL;

-- name: ListGraphs :many
SELECT * FROM oikumenea.tenant_graphs WHERE deleted_at IS NULL ORDER BY created_at, code;

-- name: ClearDefaultGraphs :exec
-- Unset is_default on every active graph (run before promoting a new default).
UPDATE oikumenea.tenant_graphs SET is_default = false WHERE is_default AND deleted_at IS NULL;

-- name: UpdateGraph :one
UPDATE oikumenea.tenant_graphs SET
  name                 = COALESCE(sqlc.narg('name'), name),
  is_default           = COALESCE(sqlc.narg('is_default'), is_default),
  is_authority_bearing = COALESCE(sqlc.narg('is_authority_bearing'), is_authority_bearing)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteGraph :one
UPDATE oikumenea.tenant_graphs SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CountActiveGraphs :one
SELECT count(*)::int AS active_count FROM oikumenea.tenant_graphs WHERE deleted_at IS NULL;

-- name: GraphHasLiveEdges :one
SELECT EXISTS(
  SELECT 1 FROM oikumenea.tenant_unit_edges WHERE graph_id = @graph_id
) AS has_edges;

-- ============================ edges ============================

-- name: InsertEdge :one
INSERT INTO oikumenea.tenant_unit_edges (graph_id, parent_id, child_id, created_by)
VALUES (@graph_id, @parent_id, @child_id, sqlc.narg('created_by'))
RETURNING *;

-- name: DeleteEdge :execrows
DELETE FROM oikumenea.tenant_unit_edges
WHERE graph_id = @graph_id AND parent_id = @parent_id AND child_id = @child_id;

-- ============================ closure ============================

-- name: ClosureHasPath :one
-- Is `ancestor_id` an ancestor of `descendant_id` in this graph? Used for the cycle guard
-- (a new parent->child edge is a cycle iff the child already reaches the parent).
SELECT EXISTS(
  SELECT 1 FROM oikumenea.tenant_unit_closure
  WHERE graph_id = @graph_id AND ancestor_id = @ancestor_id AND descendant_id = @descendant_id
) AS reachable;

-- name: DeleteClosureForGraph :exec
DELETE FROM oikumenea.tenant_unit_closure WHERE graph_id = @graph_id;

-- name: RebuildClosureForGraph :exec
-- Recompute one graph's full transitive closure from its edges, in the caller's transaction.
-- Reflexive (g,u,u,0) rows for every unit appearing in the graph's edges, then descend; collapse
-- multi-path DAG depths to the shortest with MIN(depth). Cycle-free by construction (guarded).
WITH RECURSIVE
  nodes AS (
    SELECT parent_id AS u FROM oikumenea.tenant_unit_edges WHERE graph_id = @graph_id
    UNION
    SELECT child_id FROM oikumenea.tenant_unit_edges WHERE graph_id = @graph_id
  ),
  reach AS (
    SELECT u AS ancestor_id, u AS descendant_id, 0 AS depth FROM nodes
    UNION ALL
    SELECT r.ancestor_id, e.child_id, r.depth + 1
    FROM reach r
    JOIN oikumenea.tenant_unit_edges e
      ON e.graph_id = @graph_id AND e.parent_id = r.descendant_id
  )
INSERT INTO oikumenea.tenant_unit_closure (graph_id, ancestor_id, descendant_id, depth)
SELECT @graph_id::uuid, ancestor_id, descendant_id, min(depth)::int
FROM reach
GROUP BY ancestor_id, descendant_id;

-- name: VerifyClosureForGraph :one
-- Diff the stored closure against a freshly computed one (pair membership), returning the counts
-- and a small sample for the drift report. Does not modify the stored closure.
WITH RECURSIVE
  nodes AS (
    SELECT te.parent_id AS u FROM oikumenea.tenant_unit_edges te WHERE te.graph_id = @graph_id
    UNION
    SELECT te.child_id FROM oikumenea.tenant_unit_edges te WHERE te.graph_id = @graph_id
  ),
  reach AS (
    SELECT u AS ancestor_id, u AS descendant_id, 0 AS depth FROM nodes
    UNION ALL
    SELECT r.ancestor_id, e.child_id, r.depth + 1
    FROM reach r
    JOIN oikumenea.tenant_unit_edges e
      ON e.graph_id = @graph_id AND e.parent_id = r.descendant_id
  ),
  expected AS (
    SELECT DISTINCT ancestor_id, descendant_id FROM reach
  ),
  stored AS (
    SELECT tc.ancestor_id, tc.descendant_id FROM oikumenea.tenant_unit_closure tc WHERE tc.graph_id = @graph_id
  ),
  missing AS (SELECT ancestor_id, descendant_id FROM expected EXCEPT SELECT ancestor_id, descendant_id FROM stored),
  extra   AS (SELECT ancestor_id, descendant_id FROM stored   EXCEPT SELECT ancestor_id, descendant_id FROM expected)
SELECT
  (SELECT count(*) FROM missing)::int AS missing_count,
  (SELECT count(*) FROM extra)::int   AS extra_count,
  (SELECT coalesce(jsonb_agg(s), '[]'::jsonb) FROM (
     (SELECT 'missing'::text AS kind, ancestor_id, descendant_id FROM missing LIMIT 5)
     UNION ALL
     (SELECT 'extra'::text AS kind, ancestor_id, descendant_id FROM extra LIMIT 5)
   ) s) AS sample;

-- name: UpsertClosureStatus :exec
INSERT INTO oikumenea.tenant_closure_status (graph_id, last_checked_at, missing_count, extra_count, in_drift, sample)
VALUES (@graph_id, now(), @missing_count, @extra_count, @in_drift, @sample)
ON CONFLICT (graph_id) DO UPDATE SET
  last_checked_at = now(),
  missing_count   = EXCLUDED.missing_count,
  extra_count     = EXCLUDED.extra_count,
  in_drift        = EXCLUDED.in_drift,
  sample          = EXCLUDED.sample;

-- name: ListAncestors :many
-- Ancestors of @unit_id in @graph_id (strict; excludes self), nearest first.
SELECT u.id, u.code, u.name, u.visibility, c.depth
FROM oikumenea.tenant_unit_closure c
JOIN oikumenea.tenant_units u ON u.id = c.ancestor_id AND u.deleted_at IS NULL
WHERE c.graph_id = @graph_id AND c.descendant_id = @unit_id AND c.depth > 0
ORDER BY c.depth, u.code;

-- name: ListDescendants :many
-- The subtree of @unit_id in @graph_id (strict; excludes self), keyset-paginated by descendant id.
SELECT u.id, u.code, u.name, u.visibility, c.depth
FROM oikumenea.tenant_unit_closure c
JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
WHERE c.graph_id = @graph_id AND c.ancestor_id = @unit_id AND c.depth > 0
  AND (sqlc.narg('after')::uuid IS NULL OR c.descendant_id > sqlc.narg('after')::uuid)
ORDER BY c.descendant_id
LIMIT @lim;

-- ============================ lifecycle ============================

-- name: InsertLifecycleEvent :exec
INSERT INTO oikumenea.tenant_unit_lifecycle_events
  (unit_id, from_state, to_state, reason, actor_person_id, request_id)
VALUES (@unit_id, @from_state, @to_state, sqlc.narg('reason'), sqlc.narg('actor_person_id'), @request_id);
