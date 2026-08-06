-- Authorization module queries (M7). Roles, role-permission membership, role assignments
-- (link__has_role), and instance-admin grants (link__instance_admin). Compiled by sqlc into authzsql.
-- The permission CATALOG is code, not data — these queries only persist membership + assignments.

-- ============================ roles ============================

-- name: InsertRole :one
INSERT INTO oikumenea.authz_roles (code, name, description, is_base)
VALUES (@code, @name, sqlc.narg('description'), @is_base)
RETURNING id, code, name, description, is_base, created_at, updated_at;

-- name: SeedRole :one
-- Idempotent base-role seed (D-RIDSeeding): insert if the code is free among active rows, else
-- return the existing row. ON CONFLICT needs the partial-unique index's predicate, which a plain
-- ON CONFLICT cannot target, so we guard with WHERE NOT EXISTS and fall back to a select in the app.
INSERT INTO oikumenea.authz_roles (code, name, description, is_base)
SELECT @code, @name, sqlc.narg('description'), true
WHERE NOT EXISTS (
  SELECT 1 FROM oikumenea.authz_roles WHERE code = @code AND deleted_at IS NULL
)
RETURNING id, code, name, description, is_base, created_at, updated_at;

-- name: GetRole :one
SELECT id, code, name, description, is_base, created_at, updated_at
FROM oikumenea.authz_roles
WHERE id = @id AND deleted_at IS NULL;

-- name: GetRoleByCode :one
SELECT id, code, name, description, is_base, created_at, updated_at
FROM oikumenea.authz_roles
WHERE code = @code AND deleted_at IS NULL;

-- name: ListRoles :many
SELECT id, code, name, description, is_base, created_at, updated_at
FROM oikumenea.authz_roles
WHERE deleted_at IS NULL
  AND (sqlc.arg('after')::text = '' OR id::text > sqlc.arg('after')::text)
ORDER BY id
LIMIT @lim;

-- name: UpdateRole :one
-- Partial update of name/description (COALESCE keeps the stored value when the arg is NULL). Code and
-- is_base are immutable; permissions are replaced separately.
UPDATE oikumenea.authz_roles
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description)
WHERE id = @id AND deleted_at IS NULL
RETURNING id, code, name, description, is_base, created_at, updated_at;

-- name: SoftDeleteRole :exec
UPDATE oikumenea.authz_roles SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: RoleHasActiveAssignments :one
SELECT EXISTS(
  SELECT 1 FROM oikumenea.authz_role_assignments
  WHERE role_id = @role_id AND revoked_at IS NULL
) AS in_use;

-- ============================ role permissions ============================

-- name: GetRolePermissions :many
SELECT permission_code FROM oikumenea.authz_role_permissions
WHERE role_id = @role_id ORDER BY permission_code;

-- name: DeleteRolePermissions :exec
DELETE FROM oikumenea.authz_role_permissions WHERE role_id = @role_id;

-- name: InsertRolePermission :exec
INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code)
VALUES (@role_id, @permission_code)
ON CONFLICT (role_id, permission_code) DO NOTHING;

-- ============================ assignments ============================

-- name: InsertAssignment :one
INSERT INTO oikumenea.authz_role_assignments
  (subject_person_id, role_id, target_unit_id, scope, graph_id, granted_by, expires_at)
VALUES (@subject_person_id, @role_id, @target_unit_id, @scope,
        sqlc.narg('graph_id'), sqlc.narg('granted_by'), sqlc.narg('expires_at'))
RETURNING id, subject_person_id, role_id, target_unit_id, scope, graph_id,
          granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at;

-- name: GetAssignment :one
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments
WHERE id = @id;

-- name: RevokeAssignment :one
-- Reversible flip: set revoked_at/by only on a still-active row (idempotent guard via the WHERE).
UPDATE oikumenea.authz_role_assignments
SET revoked_at = now(), revoked_by = sqlc.narg('revoked_by')
WHERE id = @id AND revoked_at IS NULL
RETURNING id, subject_person_id, role_id, target_unit_id, scope, graph_id,
          granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at;

-- ============================ the assignment list (M58 ticket 6 / D-ObjectFacets) ============================
-- GET /assignments. THREE shapes, ONE filter block, byte-identical between them: the admin path and
-- the two reach-scoped paths must select the same rows for the same filters, differing ONLY by how
-- reach is applied. sqlparity_test.go proves the block is present in all of them with no database.
--
-- This replaces ListAssignmentsBySubject / ListAssignmentsByUnit, which were not two filters but two
-- ENDPOINTS wearing one name: exactly one of them had to be supplied, so there was no way to ask for
-- the grants. Both are now ordinary predicates in the block below.
--
-- ACTIVE ONLY, and unlike the top-level membership list this default STANDS (decided M58 ticket 3):
-- an ended membership is ordinary directory history, while a revoked grant is a security artefact
-- whose reachability is an authz read-surface decision rather than a facet-vocabulary one. The
-- consequence is written into the contract — totalCount counts ACTIVE grants — and there is no
-- `active` facet, because a distribution whose every row is active is a chart with one bar.
--
-- REACH IS ASKED FOR `assignment.read` AND NOTHING ELSE. Every other module's scoped list trims with
-- authz_readable_units(subject), which asks whether the subject holds ANY '%.read' code on the unit;
-- that is right there, where the endpoint has already checked its own read code and reach is only
-- narrowing rows. Here it would WIDEN: generic read-reach is a strict superset of assignment.read
-- reach, so a caller holding `person.read` over a unit and `assignment.read` somewhere else would be
-- handed grants that today's per-unit arm refuses them. The 0023 `_with` functions ask the narrow
-- question; passing anything but 'assignment.read' at the call sites is what
-- assignment_reach_test.go refuses.
--
-- The `sqlc.narg('x')::type IS NULL OR col = ...` style is deliberate and load-bearing: R-21 BANS the
-- `(@arg = '' OR ...)` sentinel because the planner cannot prove the arg non-empty under a generic
-- prepared plan and sequential-scans.

-- name: ListAssignments :many
-- Instance-admin path: every active grant, keyset-paginated by RID.
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments a
WHERE a.revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR a.id::text > sqlc.arg('after')::text)
  AND (sqlc.narg('subject_person_id')::uuid IS NULL OR a.subject_person_id = sqlc.narg('subject_person_id')::uuid)
  AND (sqlc.narg('target_unit_id')::uuid IS NULL OR a.target_unit_id = sqlc.narg('target_unit_id')::uuid)
  AND (sqlc.narg('role_id')::uuid IS NULL OR a.role_id = sqlc.narg('role_id')::uuid)
  AND (sqlc.narg('scope')::text IS NULL OR a.scope = sqlc.narg('scope')::text)
  AND (sqlc.narg('graph_id')::uuid IS NULL OR a.graph_id = sqlc.narg('graph_id')::uuid)
ORDER BY a.id
LIMIT @lim;

-- name: ListAssignmentsForSubject :many
-- SPARSE-reach path: the same set intersected with the subject's assignment.read reach. The reach set
-- is UNCORRELATED — it reads only @subject_person_id and @permission — so the planner evaluates it
-- once and probes a hash rather than re-deriving the closure per candidate row.
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments a
WHERE a.revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR a.id::text > sqlc.arg('after')::text)
  AND (sqlc.narg('subject_person_id')::uuid IS NULL OR a.subject_person_id = sqlc.narg('subject_person_id')::uuid)
  AND (sqlc.narg('target_unit_id')::uuid IS NULL OR a.target_unit_id = sqlc.narg('target_unit_id')::uuid)
  AND (sqlc.narg('role_id')::uuid IS NULL OR a.role_id = sqlc.narg('role_id')::uuid)
  AND (sqlc.narg('scope')::text IS NULL OR a.scope = sqlc.narg('scope')::text)
  AND (sqlc.narg('graph_id')::uuid IS NULL OR a.graph_id = sqlc.narg('graph_id')::uuid)
  AND a.target_unit_id IN (SELECT oikumenea.authz_readable_units_with(@reader_person_id, @permission))
ORDER BY a.id
LIMIT @lim;

-- name: ListAssignmentsForSubjectDense :many
-- DENSE-reach plan shape of the query above, byte-identical in its filter block and differing ONLY in
-- how reach is applied: a per-row point probe instead of a materialized reach set. The adapter
-- dispatches on the capped count (0017 §2 measured why neither shape wins everywhere).
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments a
WHERE a.revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR a.id::text > sqlc.arg('after')::text)
  AND (sqlc.narg('subject_person_id')::uuid IS NULL OR a.subject_person_id = sqlc.narg('subject_person_id')::uuid)
  AND (sqlc.narg('target_unit_id')::uuid IS NULL OR a.target_unit_id = sqlc.narg('target_unit_id')::uuid)
  AND (sqlc.narg('role_id')::uuid IS NULL OR a.role_id = sqlc.narg('role_id')::uuid)
  AND (sqlc.narg('scope')::text IS NULL OR a.scope = sqlc.narg('scope')::text)
  AND (sqlc.narg('graph_id')::uuid IS NULL OR a.graph_id = sqlc.narg('graph_id')::uuid)
  AND oikumenea.authz_unit_readable_with(a.target_unit_id, @reader_person_id, @permission)
ORDER BY a.id
LIMIT @lim;

-- name: CountAssignmentReadableUnitsCapped :one
-- The capped reach-cardinality probe the sparse/dense dispatch reads, for assignment.read.
SELECT oikumenea.authz_readable_unit_count_with(@reader_person_id, @permission, @cap::integer) AS n;

-- ============================ assignment dashboard (M58 ticket 6 / D-ObjectFacets) ============================
-- TWO arms, {instance-admin, reach-scoped}, and no search twin: authz_role_assignments carries no
-- search_text haystack (a grant has no name of its own — its parts are all RIDs), so there is nothing
-- for an R-21 split to split.
--
-- The scoped arm uses the SET form only. M57 measured that a scoped AGGREGATE, unlike a scoped LIST,
-- has no early-terminating LIMIT to lose: it must visit every candidate row whatever the reach size,
-- so the point probe's dense-reach advantage does not exist and a dense twin would be two plans for
-- one answer.
--
-- The aggregate half is byte-identical across both arms (statsparity_test.go), or an admin and a
-- scoped caller would be shown different distributions of the same world.

-- name: AssignmentStats :many
-- The INSTANCE-ADMIN arm: no reach predicate at all, which is why it is a separate query rather than
-- the scoped one with a flag.
WITH cand AS MATERIALIZED (
  SELECT a.subject_person_id, a.role_id, a.target_unit_id, a.scope, a.graph_id
  FROM oikumenea.authz_role_assignments a
  WHERE a.revoked_at IS NULL
    AND (sqlc.narg('subject_person_id')::uuid IS NULL OR a.subject_person_id = sqlc.narg('subject_person_id')::uuid)
    AND (sqlc.narg('target_unit_id')::uuid IS NULL OR a.target_unit_id = sqlc.narg('target_unit_id')::uuid)
    AND (sqlc.narg('role_id')::uuid IS NULL OR a.role_id = sqlc.narg('role_id')::uuid)
    AND (sqlc.narg('scope')::text IS NULL OR a.scope = sqlc.narg('scope')::text)
    AND (sqlc.narg('graph_id')::uuid IS NULL OR a.graph_id = sqlc.narg('graph_id')::uuid)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'subjectPersonId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.subject_person_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_subject_person_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'roleId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.role_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_role_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'targetUnitId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.target_unit_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_target_unit_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'scope'::text, c.scope::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_scope')::boolean GROUP BY 2
UNION ALL
SELECT 'graphId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.graph_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_graph_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: AssignmentStatsForSubject :many
-- The reach-scoped arm. Identical but for the reach predicate, which is the same set form
-- ListAssignmentsForSubject uses and asks for the same one permission.
WITH cand AS MATERIALIZED (
  SELECT a.subject_person_id, a.role_id, a.target_unit_id, a.scope, a.graph_id
  FROM oikumenea.authz_role_assignments a
  WHERE a.revoked_at IS NULL
    AND (sqlc.narg('subject_person_id')::uuid IS NULL OR a.subject_person_id = sqlc.narg('subject_person_id')::uuid)
    AND (sqlc.narg('target_unit_id')::uuid IS NULL OR a.target_unit_id = sqlc.narg('target_unit_id')::uuid)
    AND (sqlc.narg('role_id')::uuid IS NULL OR a.role_id = sqlc.narg('role_id')::uuid)
    AND (sqlc.narg('scope')::text IS NULL OR a.scope = sqlc.narg('scope')::text)
    AND (sqlc.narg('graph_id')::uuid IS NULL OR a.graph_id = sqlc.narg('graph_id')::uuid)
    AND a.target_unit_id IN (SELECT oikumenea.authz_readable_units_with(@reader_person_id, @permission))
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'subjectPersonId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.subject_person_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_subject_person_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'roleId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.role_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_role_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'targetUnitId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.target_unit_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_target_unit_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'scope'::text, c.scope::text, count(*)::bigint
FROM cand c WHERE sqlc.arg('want_scope')::boolean GROUP BY 2
UNION ALL
SELECT 'graphId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.graph_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_graph_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: ActiveGrantsForSubject :many
-- The subject's active assignments joined with each role's permission codes and the graph code, for
-- the PDP. revoked_at IS NULL AND decision-time expiry filter (D-TimeBoundGrants). The application
-- groups rows by assignment id into ActiveGrants.
SELECT a.id, a.role_id, r.code AS role_code, a.target_unit_id, a.scope,
       a.graph_id, g.code AS graph_code, rp.permission_code
FROM oikumenea.authz_role_assignments a
JOIN oikumenea.authz_roles r            ON r.id = a.role_id AND r.deleted_at IS NULL
JOIN oikumenea.authz_role_permissions rp ON rp.role_id = a.role_id
LEFT JOIN oikumenea.tenant_graphs g      ON g.id = a.graph_id
WHERE a.subject_person_id = @subject_person_id
  AND a.revoked_at IS NULL
  AND (a.expires_at IS NULL OR a.expires_at > now())
ORDER BY a.id, rp.permission_code;

-- ============================ instance admins ============================

-- name: InsertInstanceAdmin :one
INSERT INTO oikumenea.authz_instance_admins (person_id, granted_by)
VALUES (@person_id, sqlc.narg('granted_by'))
RETURNING id, person_id, granted_by, granted_at, revoked_at, revoked_by, created_at, updated_at;

-- name: GetInstanceAdmin :one
SELECT id, person_id, granted_by, granted_at, revoked_at, revoked_by, created_at, updated_at
FROM oikumenea.authz_instance_admins
WHERE id = @id;

-- name: RevokeInstanceAdmin :one
UPDATE oikumenea.authz_instance_admins
SET revoked_at = now(), revoked_by = sqlc.narg('revoked_by')
WHERE id = @id AND revoked_at IS NULL
RETURNING id, person_id, granted_by, granted_at, revoked_at, revoked_by, created_at, updated_at;

-- name: IsActiveInstanceAdmin :one
SELECT EXISTS(
  SELECT 1 FROM oikumenea.authz_instance_admins
  WHERE person_id = @person_id AND revoked_at IS NULL
) AS is_admin;

-- name: HasActiveInstanceAdmin :one
-- Whether ANY active instance admin exists. Gates the idempotent first-admin bootstrap (D-Bootstrap):
-- the seed runs only when no instance admin exists yet (or under an explicit --force).
SELECT EXISTS(
  SELECT 1 FROM oikumenea.authz_instance_admins WHERE revoked_at IS NULL
) AS has_admin;

-- ============================ revocation epoch (D-AuthzGrantCache) ============================

-- name: ReadAuthzEpoch :one
-- One single-row read: the grant cache validates a stale entry against this counter instead of
-- re-running the grants join (review-2026-07 R-01.2).
SELECT epoch FROM oikumenea.authz_epoch WHERE singleton;

-- name: BumpAuthzEpoch :exec
-- Called INSIDE every authority-mutating transaction so a grant/revoke/role edit invalidates every
-- process's cache exactly (the bump commits atomically with the mutation).
UPDATE oikumenea.authz_epoch SET epoch = epoch + 1 WHERE singleton;

-- ============================ reach probe (shadow gate, review-2026-07 R-02.1) ============================

-- name: ReadableUnitsForSubjectAmong :many
-- Batch reach probe for the shadow-visibility gate (FilterVisibleUnits): which of the candidate
-- units does the subject's '*.read' reach? Same PARITY CONTRACT as membership's
-- VisiblePersonIDsForSubject (mirrors domain ReachSet — active unexpired assignment, live role, any
-- '*.read' permission; unit scope → target only; subtree scope → target + non-deleted descendants
-- over an authority-bearing, non-deleted graph). Cross-module closure/graph join precedent:
-- ActiveGrantsForSubject above.
SELECT cand.unit_id::uuid AS unit_id
FROM unnest(@unit_ids::uuid[]) AS cand(unit_id)
WHERE EXISTS (
  SELECT 1
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
  WHERE a.subject_person_id = @subject_person_id
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
    AND ((a.scope = 'unit' AND a.target_unit_id = cand.unit_id)
      OR (a.scope = 'subtree'
          AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                      WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
          AND (a.target_unit_id = cand.unit_id
            OR EXISTS (SELECT 1
                       FROM oikumenea.tenant_unit_closure c
                       JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                       WHERE c.graph_id = a.graph_id
                         AND c.ancestor_id = a.target_unit_id
                         AND c.descendant_id = cand.unit_id))))
);

-- name: ReadableOrgsForSubjectAmong :many
-- Batch reach probe for the ORGANIZATION shadow gate (FilterVisibleOrgs). An organization is
-- reachable when ANY of its live units is — D-VisibilityScope as amended after M58 ticket 4.
--
-- Why derived rather than granted: `authz_role_assignments.target_unit_id` is NOT NULL and REFERENCES
-- tenant_units, so an organization RID can never appear in a grant. Before this query the org reach
-- set was empty BY CONSTRUCTION and a shadow organization was visible to an instance admin and to
-- nobody else — not a policy anyone chose, just the shape of the assignment table showing through.
--
-- It leaks nothing new. `listUnits` takes the org RID as a REQUIRED argument and gates the units, not
-- the organization, so a subject with reach into an org's units can already enumerate them and is
-- already holding that org's RID. What changes is that the organization now says so.
--
-- The reach set itself is authz_readable_units, the same STABLE set function the unit dashboards
-- probe — so this stays ONE definition of reach, read through one more join, rather than a second
-- reach semantic that would have to be kept in step.
SELECT cand.org_id::uuid AS org_id
FROM unnest(@org_ids::uuid[]) AS cand(org_id)
WHERE EXISTS (
  SELECT 1
  FROM oikumenea.tenant_units u
  WHERE u.org_id = cand.org_id
    AND u.deleted_at IS NULL
    AND u.id IN (SELECT oikumenea.authz_readable_units(@subject_person_id))
);

-- ============================ principal grants (M51) ============================
-- The machine-subject authority plane (D-ServiceIdentities). A grant is FLAT — no target unit, no
-- scope, no graph — because a service principal has no unit reach by construction. org_id NULL means
-- instance-wide; a named organization confines a connector to that org's data.
--
-- permission_code is validated in the application against the Go catalog, exactly as
-- authz_role_permissions is: the permission catalog is CODE, never a table (D-Code).

-- name: InsertPrincipalGrant :one
-- The two partial unique indexes (instance-wide vs org-scoped) backstop double-granting; the
-- principal FK backstops existence.
INSERT INTO oikumenea.authz_principal_grants (principal_id, permission_code, org_id, granted_by)
VALUES (@principal_id, @permission_code, sqlc.narg('org_id'), sqlc.narg('granted_by'))
RETURNING *;

-- name: GetPrincipalGrant :one
SELECT * FROM oikumenea.authz_principal_grants WHERE id = @id;

-- name: RevokePrincipalGrant :one
-- Revoke-flip, never a delete: the grant's history stays readable (D-Audit). Already-revoked rows are
-- excluded so a second revoke is a no-op the application maps to not-found.
UPDATE oikumenea.authz_principal_grants
SET revoked_at = now(), revoked_by = sqlc.narg('revoked_by')
WHERE id = @id AND revoked_at IS NULL
RETURNING *;

-- name: ListPrincipalGrants :many
SELECT * FROM oikumenea.authz_principal_grants
WHERE principal_id = @principal_id AND revoked_at IS NULL
ORDER BY permission_code, id;

-- name: ActiveGrantsForPrincipal :many
-- The per-request authority fetch for a machine subject — the service-side counterpart of
-- ActiveGrantsForSubject. Flat rows: the PDP is not involved (a principal decision is a grant match,
-- not a DAG traversal).
SELECT permission_code, org_id
FROM oikumenea.authz_principal_grants
WHERE principal_id = @principal_id AND revoked_at IS NULL
ORDER BY permission_code;
