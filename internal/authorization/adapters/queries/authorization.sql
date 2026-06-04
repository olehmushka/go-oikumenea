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
  AND (sqlc.arg('after')::text = '' OR id > sqlc.arg('after')::text)
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
  AND (sqlc.arg('after')::text = '' OR id > sqlc.arg('after')::text)
ORDER BY id
LIMIT @lim;

-- name: ListAssignmentsByUnit :many
SELECT id, subject_person_id, role_id, target_unit_id, scope, graph_id,
       granted_by, granted_at, revoked_at, revoked_by, expires_at, created_at, updated_at
FROM oikumenea.authz_role_assignments
WHERE target_unit_id = @target_unit_id AND revoked_at IS NULL
  AND (sqlc.arg('after')::text = '' OR id > sqlc.arg('after')::text)
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
