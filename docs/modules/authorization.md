# Module: authorization

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.authz_*`

## Purpose

The centerpiece. Owns the **RBAC model** and the **Policy Decision Point (PDP)** — the engine
that answers *"may person P perform action A on unit U?"*. This is the product's differentiator
vs. Keycloak: decisions are computed over the **unit graph** ([tenant](tenant.md)'s closure),
with **per-assignment scope** inheritance (D-Inherit), a separate **instance-admin** plane
(D-InstanceAdmin), and a **shadow-visibility gate** on reads. Permissions are **code-defined**;
roles and assignments are **data** (policy-as-data, enforcement-as-code). Authority comes
**only** from assignments here — never from rank or position.

## Entities & aggregates

- **Atomic permission** — a code-defined string; the closed vocabulary lives in Go
  (`internal/authorization/domain/permissions.go`), not the DB. Adding one is a code change.
- **Role** — a named set of permission codes. **Base roles** are seeded/platform-defined;
  **custom roles** are instance-defined.
- **Role assignment** — `(subject_person, role, target_unit, scope)` + provenance + optional
  expiry. The unit of granted authority.
- **Instance admin** — a person holding the instance-wide authority plane.

## The atomic permission catalog (code-defined)

Generic, domain-agnostic. Representative set (final list is fixed in code):

- **unit:** `unit.read`, `unit.create`, `unit.update`, `unit.edges.manage`, `unit.lifecycle`
- **person:** `person.read`, `person.create`, `person.update`, `person.rank.assign`,
  `person.lifecycle`, `person.purge`
- **membership:** `membership.read`, `membership.create`, `membership.update`
- **position** (unit-scoped — billets belong to a unit): `position.read`, `position.create`,
  `position.update`
- **authz:** `role.read`, `role.create`, `role.update`, `role.delete`, `assignment.read`,
  `assignment.grant`, `assignment.revoke`
- **audit:** `audit.read`
- **i18n** (instance-scope): `locale.read`, `locale.manage`, `translation.read`,
  `translation.manage`
- **instance-scope** (only meaningful on the instance-admin plane): `rank.scheme.manage`,
  `instance.config`, `instance.admin.manage` (plus the i18n `*.manage` above)

Permissions are grouped by resource; the `*.read` family is what the **shadow gate** consults
on read paths.

## Data model

Conventions per [conventions.md](../architecture/conventions.md). Permissions are **not** a
table (they are code); everything else is data.

**`authz_roles`**
- `id` PK
- `code TEXT NOT NULL UNIQUE` — **stable, locale-agnostic** identifier for external reference
  (D-Code); immutable by convention
- `name TEXT NOT NULL`, `description TEXT` — default-locale labels; **translatable** via the
  [localization](localization.md) store (returned as `locale → text` maps)
- `is_base BOOLEAN NOT NULL DEFAULT FALSE` — base roles are not instance-editable
- `created_at`, `updated_at`, `deleted_at`

**`authz_role_permissions`** (role → permission-code membership)
- `role_id UUID NOT NULL REFERENCES authz_roles(id) ON DELETE CASCADE`
- `permission_code TEXT NOT NULL` — validated against the code catalog at write time
- `PRIMARY KEY (role_id, permission_code)`

**`authz_role_assignments`**
- `id` PK
- `subject_person_id UUID NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `role_id UUID NOT NULL REFERENCES authz_roles(id) ON DELETE RESTRICT`
- `target_unit_id UUID NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `scope TEXT NOT NULL CHECK (scope IN ('unit','subtree'))`
- `granted_by UUID REFERENCES person_persons(id) ON DELETE SET NULL`, `granted_at`
- `revoked_at TIMESTAMPTZ`, `revoked_by UUID REFERENCES person_persons(id) ON DELETE SET NULL`
- `expires_at TIMESTAMPTZ` — **dormant seam**, evaluated at decision time once exercised
- `created_at`, `updated_at`
- Active uniqueness: `UNIQUE (subject_person_id, role_id, target_unit_id, scope)
  WHERE revoked_at IS NULL`
- Indexes: `(subject_person_id) WHERE revoked_at IS NULL`,
  `(target_unit_id) WHERE revoked_at IS NULL`

**`authz_instance_admins`**
- `id` PK
- `person_id UUID NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `granted_by`, `granted_at`, `revoked_at`, `revoked_by`
- `UNIQUE (person_id) WHERE revoked_at IS NULL`
- (Optional refinement: an instance-admin *role* selecting a subset of instance permissions;
  default is full instance-scope. Seam reserved.)

## The PDP algorithm

`authorize(person, action, unitID) → ALLOW | DENY`:

```
1. If person is an active instance admin and `action` is an instance-scope permission
   they hold  → ALLOW.            (instance plane; unit-independent)

2. Collect the person's active role assignments
   (revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())).

3. effective := ∅
   For each assignment (role, target, scope):
       perms := permissions(role)            // code lookup
       if scope = 'unit'    and target = unitID:                 effective ∪= perms
       if scope = 'subtree' and isAncestorOrSelf(target, unitID): effective ∪= perms
   // isAncestorOrSelf is ONE closure lookup: tenant_unit_closure
   //   (ancestor_id = target, descendant_id = unitID) exists.

4. If `action` ∈ effective:
       if action is a read on a SHADOW unit → ALLOW only if a *.read in `effective`
          actually reaches unitID (it does, by construction of step 3) → ALLOW
       else → ALLOW
   else → DENY.
```

Key properties:
- **Union semantics:** effective permissions are the union over all reaching assignments
  (multi-parent DAG: a unit may be reachable from several `subtree` grants; all contribute).
- **`unit` scope leaks nothing downward** — a `unit` grant at T contributes to T only, never to
  T's children (not even read).
- **`target_unit` is independent of the subject's placement** — the algorithm never consults
  where the person sits or their rank.
- **No per-permission filtering** inside an assignment; the whole role's permission set applies
  at its scope.
- **No cross-request caching.** A decision may be memoized within one request; a revoke or role
  edit takes effect on the next request immediately.

### The shadow-visibility gate

For read endpoints that return units / memberships / rosters, after the permission decision the
result set is filtered: rows belonging to a `shadow` unit are dropped unless the PDP grants a
`*.read` reaching that unit. `public` units appear subject to normal read permission. This is a
second pass over results, owned here and called by [tenant](tenant.md) and
[membership](membership.md). See [patterns.md](../architecture/patterns.md).

## Conjure API surface

`AuthorizationService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /authorize` | Decide `(person, action, unit)` → allow/deny | authenticated; self or `assignment.read` |
| `POST /authorize/batch` | Batch decisions / return effective permission set | authenticated |
| `POST /roles` | Create a custom role | `role.create` (instance) |
| `GET /roles` / `GET /roles/{id}` | List/read roles | `role.read` |
| `PUT /roles/{id}` | Edit a custom role's permissions | `role.update` (instance) |
| `DELETE /roles/{id}` | Soft-delete a custom role (blocked if assigned) | `role.delete` (instance) |
| `POST /assignments` | Grant `(person, role, unit, scope)` | `assignment.grant` (on target unit) |
| `DELETE /assignments/{id}` | Revoke (reversible flip) | `assignment.revoke` |
| `GET /assignments` | List assignments (by person/unit, token-paginated) | `assignment.read` |
| `POST /instance-admins` | Grant instance-admin | `instance.admin.manage` (instance) |
| `DELETE /instance-admins/{id}` | Revoke instance-admin | `instance.admin.manage` (instance) |

`/authorize` is the seam other modules' transport layers call (in-process) before each guarded
operation, and that external PEPs can call over HTTP. Denied → Conjure
`Authorization:PermissionDenied`.

## Dependencies

- **Calls:** [tenant](tenant.md) (closure: `isAncestorOrSelf`, and unit visibility for the
  gate). [person](person.md) (subject exists). [localization](localization.md) (assemble role
  `name`/`description` locale-maps; purge translations on role delete). [platform](platform.md)
  for infra. Emits `RoleChanged`, `AssignmentGranted`, `AssignmentRevoked`,
  `InstanceAdminChanged` events.
- **Called by:** **every** module's transport layer (the PDP check before guarded ops) and
  external policy-enforcement points. [audit](audit.md) records all writes here.

## Authorization touchpoints

Defines the whole catalog above. Self-gates role/assignment/instance-admin management:
`role.*`, `assignment.*` are unit- or instance-scoped as noted; `instance.admin.manage` is
instance-scope only. Guards against privilege escalation: granting an assignment requires
`assignment.grant` reaching the target unit; managing roles/instance-admins is instance-plane.

## Invariants & safety

- **Permissions exist only in code.** A write to `authz_role_permissions` with an unknown code
  is rejected. The authorization surface is always visible in a diff.
- **Base roles are immutable** by instance admins (`is_base = TRUE`).
- **No self-escalation:** a person cannot grant themselves instance-admin or a role they lack
  the authority to grant; checked against the PDP on the grant path.
- **Reversibility:** assignments and instance-admin grants are revoked by flipping
  `revoked_at`, never deleted — history is preserved for [audit](audit.md).
- **Decisions are pure functions of current data** (assignments + closure + code catalog) — no
  hidden state, no rank, no position.

## Open seams / future

- `expires_at` on assignments is shipped **dormant**; activating time-bound grants is additive
  (evaluate in step 2).
- An optional **instance-admin role** (subset of instance permissions) is reserved; default is
  full instance-scope.
- A **decision-explain** response (which assignment(s) produced ALLOW) is a natural addition to
  `/authorize/batch` for debugging/audit.
- Postgres RLS is **not** used (D-NoRLS) but remains an additive hardening seam if a deployment
  ever needs DB-level defense in depth.
