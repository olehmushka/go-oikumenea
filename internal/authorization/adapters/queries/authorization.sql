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

-- name: ListAssignmentsBySubject :many
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments
WHERE subject_person_id = @subject_person_id AND revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR id::text > sqlc.arg('after')::text)
ORDER BY id
LIMIT @lim;

-- name: ListAssignmentsByUnit :many
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments
WHERE target_unit_id = @target_unit_id AND revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR id::text > sqlc.arg('after')::text)
ORDER BY id
LIMIT @lim;

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
