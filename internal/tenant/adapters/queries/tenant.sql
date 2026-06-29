-- Tenant module queries (docs/modules/tenant.md). Two-tier model (D-TenantOrganizations, M40):
-- domains (org-kind catalog) -> organizations (the realm) -> units as a DAG per graph + a maintained
-- transitive closure recomputed in the write transaction on every edge change. Graphs are per-org
-- (org_id), with org_id NULL = an instance-global/cross-org graph (religion taxonomy). Units/graphs/
-- orgs soft-delete; edges hard-delete on detach; the closure is derived (no RID).

-- ============================ domains (org-kind catalog) ============================

-- name: InsertDomain :one
INSERT INTO oikumenea.tenant_domains (code, name, sort_order)
VALUES (@code, @name, sqlc.narg('sort_order'))
RETURNING *;

-- name: GetDomain :one
SELECT * FROM oikumenea.tenant_domains WHERE id = @id AND deleted_at IS NULL;

-- name: GetDomainByCode :one
SELECT * FROM oikumenea.tenant_domains WHERE code = @code AND deleted_at IS NULL;

-- name: ListDomains :many
SELECT * FROM oikumenea.tenant_domains WHERE deleted_at IS NULL ORDER BY sort_order NULLS LAST, code;

-- name: UpdateDomain :one
UPDATE oikumenea.tenant_domains SET
  name       = COALESCE(sqlc.narg('name'), name),
  status     = COALESCE(sqlc.narg('status'), status),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CountActiveDomainsByCode :one
SELECT count(*)::int AS code_count FROM oikumenea.tenant_domains
WHERE code = @code AND deleted_at IS NULL AND id <> @exclude_id;

-- ============================ unit kinds (domain-scoped catalog) ============================

-- name: InsertUnitKind :one
INSERT INTO oikumenea.tenant_unit_kinds (domain_id, code, name, attr_schema, sort_order)
VALUES (@domain_id, @code, @name, sqlc.narg('attr_schema'), sqlc.narg('sort_order'))
RETURNING *;

-- name: GetUnitKind :one
SELECT * FROM oikumenea.tenant_unit_kinds WHERE id = @id AND deleted_at IS NULL;

-- name: ListUnitKinds :many
SELECT * FROM oikumenea.tenant_unit_kinds
WHERE domain_id = @domain_id AND deleted_at IS NULL
ORDER BY sort_order NULLS LAST, code;

-- name: UpdateUnitKind :one
UPDATE oikumenea.tenant_unit_kinds SET
  name        = COALESCE(sqlc.narg('name'), name),
  attr_schema = COALESCE(sqlc.narg('attr_schema'), attr_schema),
  status      = COALESCE(sqlc.narg('status'), status),
  sort_order  = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CountActiveUnitKindsByCode :one
SELECT count(*)::int AS code_count FROM oikumenea.tenant_unit_kinds
WHERE domain_id = @domain_id AND code = @code AND deleted_at IS NULL AND id <> @exclude_id;

-- ============================ organizations (the realm) ============================

-- name: InsertOrganization :one
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id, visibility, metadata)
VALUES (@code, @name, @domain_id, @visibility, @metadata)
RETURNING *;

-- name: GetOrganization :one
SELECT * FROM oikumenea.tenant_organizations WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateOrganization :one
UPDATE oikumenea.tenant_organizations SET
  name       = COALESCE(sqlc.narg('name'), name),
  domain_id  = COALESCE(sqlc.narg('domain_id'), domain_id),
  visibility = COALESCE(sqlc.narg('visibility'), visibility),
  metadata   = COALESCE(sqlc.narg('metadata'), metadata)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetOrgState :one
UPDATE oikumenea.tenant_organizations SET state = @state
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListOrganizations :many
-- Keyset pagination over the time-ordered RID (id), optional domain filter.
SELECT * FROM oikumenea.tenant_organizations
WHERE deleted_at IS NULL
  AND (sqlc.narg('domain_id')::uuid IS NULL OR domain_id = sqlc.narg('domain_id')::uuid)
  AND (sqlc.narg('after')::uuid IS NULL OR id > sqlc.narg('after')::uuid)
ORDER BY id
LIMIT @lim;

-- name: CountActiveOrgsByCode :one
SELECT count(*)::int AS code_count FROM oikumenea.tenant_organizations
WHERE code = @code AND deleted_at IS NULL AND id <> @exclude_id;

-- name: InsertOrgLifecycleEvent :exec
INSERT INTO oikumenea.tenant_org_lifecycle_events
  (org_id, from_state, to_state, reason, actor_person_id, request_id)
VALUES (@org_id, @from_state, @to_state, sqlc.narg('reason'), sqlc.narg('actor_person_id'), @request_id);

-- ============================ units ============================

-- name: InsertUnit :one
-- Create a unit (D-TenantOrganizations, M40): org_id + domain_id required, kind_id optional.
-- `code` is optional (NULL = a non-separate sub-unit; D-UnitCodeLifecycle).
-- pdp_scoped is DERIVED in SQL from the unit's domain (D-UnifiedOrgGraph, M41) so the RLS predicate can
-- exempt reference (university/company) units without a join.
INSERT INTO oikumenea.tenant_units (org_id, domain_id, kind_id, code, name, level, visibility, pdp_scoped, metadata)
SELECT @org_id, @domain_id, sqlc.narg('kind_id'), sqlc.narg('code'), @name, sqlc.narg('level'), @visibility,
       COALESCE((SELECT d.pdp_scoped FROM oikumenea.tenant_domains d WHERE d.id = @domain_id), true), @metadata
RETURNING *;

-- name: GetUnit :one
SELECT * FROM oikumenea.tenant_units WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateUnit :one
-- Partial update: a NULL narg leaves the stored value unchanged (COALESCE). `code` and `org_id` are
-- immutable here (code via SetUnitCode; org is fixed at create).
UPDATE oikumenea.tenant_units SET
  name       = COALESCE(sqlc.narg('name'), name),
  domain_id  = COALESCE(sqlc.narg('domain_id'), domain_id),
  kind_id    = COALESCE(sqlc.narg('kind_id'), kind_id),
  level      = COALESCE(sqlc.narg('level'), level),
  visibility = COALESCE(sqlc.narg('visibility'), visibility),
  -- re-derive pdp_scoped when the domain changes (mixed-tree re-classification, M41)
  pdp_scoped = COALESCE(
    (SELECT d.pdp_scoped FROM oikumenea.tenant_domains d WHERE d.id = COALESCE(sqlc.narg('domain_id'), tenant_units.domain_id)),
    tenant_units.pdp_scoped),
  metadata   = COALESCE(sqlc.narg('metadata'), metadata)
WHERE tenant_units.id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetUnitState :one
UPDATE oikumenea.tenant_units SET state = @state
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetUnitCode :one
-- Set/correct/clear a unit's code (D-UnitCodeLifecycle). A NULL narg clears the code; the partial
-- unique index guards collisions among active coded units (the app pre-checks for a friendly 409).
UPDATE oikumenea.tenant_units SET code = sqlc.narg('code')
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: CountActiveUnitsByCode :one
-- Count active units already holding @code, excluding @exclude_id (the unit being recoded). Drives
-- the friendly ErrUnitCodeConflict pre-check before the partial-unique index would reject the write.
SELECT count(*)::int AS code_count FROM oikumenea.tenant_units
WHERE code = @code AND deleted_at IS NULL AND id <> @exclude_id;

-- name: ListUnits :many
-- Keyset pagination over the time-ordered RID (id), REQUIRED org scope + optional domain/kind/level.
SELECT * FROM oikumenea.tenant_units
WHERE deleted_at IS NULL
  AND org_id = @org_id
  AND (sqlc.narg('domain_id')::uuid IS NULL OR domain_id = sqlc.narg('domain_id')::uuid)
  AND (sqlc.narg('kind_id')::uuid IS NULL OR kind_id = sqlc.narg('kind_id')::uuid)
  AND (sqlc.narg('level')::smallint IS NULL OR level = sqlc.narg('level')::smallint)
  AND (sqlc.narg('after')::uuid IS NULL OR id > sqlc.narg('after')::uuid)
ORDER BY id
LIMIT @lim;

-- name: ListChildUnits :many
-- Direct children of @parent_id within graph @graph_id (the immediate edges, not the closure subtree).
-- Keyset-paginated by the child unit's RID (id). Used for expand-on-click hierarchy browsing.
SELECT u.* FROM oikumenea.tenant_units u
JOIN oikumenea.tenant_unit_edges e
  ON e.child_id = u.id AND e.graph_id = @graph_id AND e.parent_id = @parent_id
WHERE u.deleted_at IS NULL
  AND (sqlc.narg('after')::uuid IS NULL OR u.id > sqlc.narg('after')::uuid)
ORDER BY u.id
LIMIT @lim;

-- name: ListRootUnits :many
-- The org's top-level units in graph @graph_id: active units with no parent edge in the graph
-- (includes still-unattached units). Keyset-paginated by id. Used as the roots of the unit tree.
SELECT u.* FROM oikumenea.tenant_units u
WHERE u.deleted_at IS NULL
  AND u.org_id = @org_id
  AND NOT EXISTS (
    SELECT 1 FROM oikumenea.tenant_unit_edges e
    WHERE e.graph_id = @graph_id AND e.child_id = u.id
  )
  AND (sqlc.narg('after')::uuid IS NULL OR u.id > sqlc.narg('after')::uuid)
ORDER BY u.id
LIMIT @lim;

-- ============================ graphs (per-org; org_id NULL = global) ============================

-- name: InsertGraph :one
INSERT INTO oikumenea.tenant_graphs (org_id, code, name, is_default, is_authority_bearing)
VALUES (sqlc.narg('org_id'), @code, @name, @is_default, @is_authority_bearing)
RETURNING *;

-- name: GetGraphByID :one
SELECT * FROM oikumenea.tenant_graphs WHERE id = @id AND deleted_at IS NULL;

-- name: GetGraphForOrgByCode :one
-- Resolve a graph by code within an organization, preferring the org's own graph and falling back to
-- an instance-global graph (org_id NULL). When @org_id is NULL only global graphs match. The ORDER BY
-- puts the org-specific row (org_id IS NULL = false) ahead of the global one.
SELECT * FROM oikumenea.tenant_graphs
WHERE code = @code AND deleted_at IS NULL
  AND (org_id = sqlc.narg('org_id') OR org_id IS NULL)
ORDER BY (org_id IS NULL)
LIMIT 1;

-- name: ListGraphsForOrg :many
-- An organization's graphs plus the instance-global graphs (org_id NULL). When @org_id is NULL,
-- returns only the global graphs.
SELECT * FROM oikumenea.tenant_graphs
WHERE deleted_at IS NULL AND (org_id = sqlc.narg('org_id') OR org_id IS NULL)
ORDER BY (org_id IS NULL), created_at, code;

-- name: ClearDefaultGraphsForOrg :exec
-- Unset is_default on the org's active graphs (run before promoting a new default within the org).
UPDATE oikumenea.tenant_graphs SET is_default = false
WHERE is_default AND deleted_at IS NULL AND org_id = @org_id;

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

-- name: CountActiveGraphsForOrg :one
-- Active graph count for the per-org "at least one graph remains" guard (NULL org_id = globals).
SELECT count(*)::int AS active_count FROM oikumenea.tenant_graphs
WHERE deleted_at IS NULL AND org_id IS NOT DISTINCT FROM sqlc.narg('org_id');

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

-- name: ListGraphIDs :many
-- All active graph ids (used to verify/rebuild every graph when none is named).
SELECT id FROM oikumenea.tenant_graphs WHERE deleted_at IS NULL ORDER BY created_at, code;

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

-- ============================ code events (D-UnitCodeLifecycle, M28) ============================

-- name: InsertUnitCodeEvent :exec
INSERT INTO oikumenea.tenant_unit_code_events
  (unit_id, old_code, new_code, reason, actor_person_id, request_id)
VALUES (@unit_id, sqlc.narg('old_code'), sqlc.narg('new_code'), sqlc.narg('reason'), sqlc.narg('actor_person_id'), @request_id);

-- name: ListUnitCodeEvents :many
-- A unit's code-change history, newest first.
SELECT id, unit_id, old_code, new_code, reason, actor_person_id, request_id, created_at
FROM oikumenea.tenant_unit_code_events
WHERE unit_id = @unit_id
ORDER BY created_at DESC, id DESC;

-- ============================ unit languages (D-Languages, M18) ============================

-- name: ListUnitLanguages :many
-- A unit's official/working languages joined to the languoid for its default-locale display name
-- (transport assembles the locale->text map). Official first, then by name.
SELECT ul.id, ul.unit_id, ul.language_id, ul.is_official, l.name AS language_name
FROM oikumenea.tenant_unit_languages ul
JOIN oikumenea.language_languoids l ON l.id = ul.language_id
WHERE ul.unit_id = @unit_id AND ul.deleted_at IS NULL
ORDER BY ul.is_official DESC, l.name, ul.id;

-- name: GetUnitLanguage :one
SELECT ul.id, ul.unit_id, ul.language_id, ul.is_official, l.name AS language_name
FROM oikumenea.tenant_unit_languages ul
JOIN oikumenea.language_languoids l ON l.id = ul.language_id
WHERE ul.unit_id = @unit_id AND ul.language_id = @language_id AND ul.deleted_at IS NULL;

-- name: InsertUnitLanguage :exec
INSERT INTO oikumenea.tenant_unit_languages (unit_id, language_id, is_official)
VALUES (@unit_id, @language_id, @is_official);

-- name: UpdateUnitLanguage :exec
UPDATE oikumenea.tenant_unit_languages SET is_official = @is_official
WHERE unit_id = @unit_id AND language_id = @language_id AND deleted_at IS NULL;

-- name: DeleteUnitLanguage :one
UPDATE oikumenea.tenant_unit_languages SET deleted_at = now()
WHERE unit_id = @unit_id AND language_id = @language_id AND deleted_at IS NULL
RETURNING id;
