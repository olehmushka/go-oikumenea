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

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Role`. **Links:**
`link__has_role` (the **reified `Role assignment`** — the centerpiece, carrying scope/provenance/
validity) and `link__instance_admin`. **Not an Object:** an *atomic permission* is **code, not data**
(a ratified divergence). **Actions:** `CreateRole`/`UpdateRole`/`DeleteRole`,
`GrantAssignment`/`RevokeAssignment`, `GrantInstanceAdmin`/`RevokeInstanceAdmin` — audited,
`action__<type>` RID. A PDP *decision* is a read, not an Action.

- **Atomic permission** — a code-defined string; the closed vocabulary lives in Go
  (`internal/authorization/domain/permissions.go`), not the DB. Adding one is a code change.
- **Role** — a named set of permission codes. **Base roles** are seeded/platform-defined;
  **custom roles** are instance-defined.
- **Role assignment** — `(subject_person, role, target_unit, scope, graph)` + provenance +
  optional expiry. The unit of granted authority. `graph` names which hierarchy a `subtree`
  grant cascades over (NULL for `unit` scope); D-Graphs.
- **Instance admin** — a person holding the instance-wide authority plane.

## The atomic permission catalog (code-defined)

Generic, domain-agnostic. Representative set (final list is fixed in code):

- **unit:** `unit.read`, `unit.create`, `unit.update`, `unit.lifecycle`
- **unit edges (per graph; D-EdgePerms):** `unit.edges.manage` (broad fallback / custom graphs),
  `unit.edges.command.manage`, `unit.edges.operational.manage`
- **person:** `person.read`, `person.create`, `person.update`, `person.rank.assign`,
  `person.lifecycle`, `person.purge`
- **membership:** `membership.read`, `membership.create`, `membership.update`
- **position** (unit-scoped — billets belong to a unit): `position.read`, `position.create`,
  `position.update`
- **document** (scoped through the holder per D-PersonReadScope; D-Documents): `document.read`,
  `document.create`, `document.update`, `document.delete`; `document.type.read` (reference read;
  `document.type.manage` is instance-scope below)
- **personal-code** (national identifiers, `pii:sensitive`; scoped through the holder per
  D-PersonReadScope; D-PersonalCodes): `personal-code.read`, `personal-code.create`,
  `personal-code.update`, `personal-code.delete`; `personal-code-scheme.read` (reference read;
  `personal-code-scheme.manage` is instance-scope below)
- **order** (unit-scoped on the issuing unit; D-Orders): `order.read`, `order.create`,
  `order.issue`, `order.revoke`; `order.type.read` (reference read; `order.type.manage` is
  instance-scope below)
- **authz:** `role.read`, `role.create`, `role.update`, `role.delete`, `assignment.read`,
  `assignment.grant`, `assignment.revoke`
- **audit:** `audit.read`
- **rank:** `rank.scheme.read` (broad read of the rank scheme; `rank.scheme.manage` is
  instance-scope below)
- **graph:** `graph.read` (read the named-hierarchy registry; `graph.manage` is instance-scope
  below) — D-Graphs
- **i18n:** `locale.read`, `translation.read` (reads); `locale.manage`, `translation.manage`
  (instance-scope)
- **instance-scope** (only meaningful on the instance-admin plane): `rank.scheme.manage`,
  `graph.manage`, `closure.rebuild` (on-demand closure verify/rebuild — D-ClosureIntegrity),
  `document.type.manage`, `order.type.manage`, `personal-code-scheme.manage` (D-PersonalCodes),
  `country.manage` (D-Geo; the `geo_countries` registry), `instance.config`,
  `instance.admin.manage` (plus the i18n `*.manage` above)

Permissions are grouped by resource; the `*.read` family is what the **shadow gate** consults
on read paths.

## Base roles (seeded)

Four `is_base = TRUE`, **unit-scoped** roles ship seeded (D-BaseRoles), defined in code alongside the
catalog and assignable with `unit` or `subtree` scope. They graduate like the Kubernetes
`view`/`edit`/`admin` defaults; the instance-admin plane is the `cluster-admin` analog (not a role).

| Base role | Permission codes |
|---|---|
| **`unit-reader`** | `unit.read`, `person.read`, `membership.read`, `position.read`, `document.read`, `personal-code.read`, `order.read`, `role.read`, `assignment.read`, `rank.scheme.read`, `graph.read`, `document.type.read`, `personal-code-scheme.read`, `order.type.read`, `locale.read`, `translation.read` |
| **`unit-manager`** | *reader* + `unit.create`, `unit.update`, `person.create`, `person.update`, `person.rank.assign`, `membership.create`, `membership.update`, `position.create`, `position.update`, `document.create`, `document.update`, `personal-code.create`, `personal-code.update`, `order.create` |
| **`unit-admin`** | *manager* + `unit.edges.manage` (broad — covers all graphs incl. custom; D-EdgePerms), `unit.lifecycle`, `person.lifecycle`, `person.purge`, `document.delete`, `personal-code.delete`, `order.issue`, `order.revoke`, `assignment.grant`, `assignment.revoke` |
| **`auditor`** | `audit.read` only (separation of duties; assign alongside `unit-reader` to resolve referenced entities) |

Instance-only permissions (`role.create/update/delete`, `rank.scheme.manage`, `graph.manage`,
`closure.rebuild`, `document.type.manage`, `order.type.manage`, `personal-code-scheme.manage`,
`country.manage`, `locale.manage`, `translation.manage`, `instance.config`,
`instance.admin.manage`) are **never** in a base role —
they are held on the instance-admin plane. **Read is an explicit grant**: there is no implicit
"authenticated ⇒ may read" exemption; grant broad read by assigning `unit-reader` at a root with
`subtree` scope.

The `document.*` / `personal-code.*` / `order.*` rows above **amend D-BaseRoles** (document/
personal-code/order reads fold into `unit-reader`; create/update into `unit-manager`;
`document.delete` / `personal-code.delete` / `order.issue` / `order.revoke` into `unit-admin`) — a
deliberate, documented extension consistent with the graduated model. `personal-code.*` mirrors
`document.*` exactly; the values themselves stay `pii:sensitive` and envelope-encrypted regardless of
who can read them (D-PersonalCodes / D-CryptoProvider).

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
- `role_id TEXT NOT NULL REFERENCES authz_roles(id) ON DELETE CASCADE`
- `permission_code TEXT NOT NULL` — validated against the code catalog at write time
- `PRIMARY KEY (role_id, permission_code)`

**`authz_role_assignments`** *(Link `link__has_role` — the reified assignment)*
- `id` PK — RID, `link__has_role` entity-type slot
- `subject_person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `role_id TEXT NOT NULL REFERENCES authz_roles(id) ON DELETE RESTRICT`
- `target_unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT`
- `scope TEXT NOT NULL CHECK (scope IN ('unit','subtree'))`
- `graph_id TEXT REFERENCES tenant_graphs(id) ON DELETE RESTRICT` — the hierarchy a `subtree`
  grant cascades over (D-Graphs); **NULL iff `scope='unit'`**:
  `CHECK ((scope = 'subtree') = (graph_id IS NOT NULL))`. A grant omitting the graph defaults to
  the registry's default graph (`command`).
- `granted_by TEXT REFERENCES person_persons(id) ON DELETE SET NULL`, `granted_at`
- `revoked_at TIMESTAMPTZ`, `revoked_by TEXT REFERENCES person_persons(id) ON DELETE SET NULL`
- `expires_at TIMESTAMPTZ` — **optional** time bound; evaluated at decision time in the PDP
  (D-TimeBoundGrants). NULL = no expiry. Lapse is **silent** (decision-time; no event, no sweep).
- `created_at`, `updated_at`
- Active uniqueness: `UNIQUE (subject_person_id, role_id, target_unit_id, scope, graph_id)
  WHERE revoked_at IS NULL` — keyed on `revoked_at` only, so an **expired-not-revoked** row still
  occupies its tuple. **Renewal is an update** of `expires_at` on that row; re-granting an
  identical expired tuple requires revoking the stale row first (D-TimeBoundGrants).
- Indexes: `(subject_person_id) WHERE revoked_at IS NULL`,
  `(target_unit_id) WHERE revoked_at IS NULL`

**Acting authority (worked example).** Acting command, dual-hatting, and secondment are modeled
here as **time-bound role assignments**, never as a position fill ([patterns.md](../architecture/patterns.md),
*Acting authority via time-bound role assignment*; D-TimeBoundGrants). The substantive CO of
Battalion B holds `command-role` `subtree`@B with no expiry. She goes on leave — her
**membership/position is untouched** (leave does not vacate the billet). To cover the gap, the
deputy gets `POST /assignments` for `command-role` `subtree`@B with `expiresAt` = the leave end:
he now carries the full authority reaching B's subtree, with **no billet change** and no
`Membership:PositionAlreadyFilled` (the one-holder index is a [membership](membership.md) concern,
and acting is not a second fill). On `expiresAt` the grant lapses silently (PDP step 2); extending
the cover is an **update** of `expires_at` on that row. **Dual-hatting** is two concurrent live
assignments on different units (the union-across-graphs PDP sums them); **secondment** is a bounded
assignment on the host unit while the home-unit membership persists.

**`authz_instance_admins`**
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `granted_by`, `granted_at`, `revoked_at`, `revoked_by` — `granted_by` is **NULL for the install
  bootstrap grant** (no granter exists yet; origin is the `system`/`bootstrap` audit row — D-Bootstrap)
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
   For each assignment (role, target, scope, graph):
       perms := permissions(role)            // code lookup
       if scope = 'unit'    and target = unitID:                        effective ∪= perms
       if scope = 'subtree' and graph.is_authority_bearing
                            and isAncestorOrSelf(graph, target, unitID): effective ∪= perms
   // isAncestorOrSelf is ONE closure lookup: tenant_unit_closure
   //   (graph_id = graph, ancestor_id = target, descendant_id = unitID) exists.
   // The graph comes from the assignment, NOT the request — the question
   //   authorize(person, action, unit) is unchanged (D-Graphs).
   // Directory-only graphs (is_authority_bearing = FALSE) are skipped here as
   //   defense-in-depth; the write path already rejects subtree grants on them
   //   (D-DirectoryGraphs).

4. If `action` ∈ effective:
       if action is a read on a SHADOW unit → ALLOW only if a *.read in `effective`
          actually reaches unitID (it does, by construction of step 3) → ALLOW
       else → ALLOW
   else → DENY.
```

Key properties:
- **Union semantics, across graphs:** effective permissions are the union over all reaching
  assignments — multi-parent within a graph (a unit reachable from several `subtree` grants) and
  **across graphs** (a unit's `command` administrative chain and its `operational` commander both
  contribute; D-Graphs). The **decision-explain** mode (DS-16, below) reports *which assignment and
  graph* produced ALLOW.
- **`unit` scope leaks nothing downward** — a `unit` grant at T contributes to T only, never to
  T's children (not even read).
- **`target_unit` is independent of the subject's placement** — the algorithm never consults
  where the person sits or their rank.
- **No per-permission filtering** inside an assignment; the whole role's permission set applies
  at its scope.
- **No cross-request caching.** A decision may be memoized within one request; a revoke or role
  edit takes effect on the next request immediately.

### The shadow-visibility gate

After the permission decision a `shadow` unit is visible only when the PDP grants a `*.read` reaching
it, while `public` units appear subject to normal read permission. This is owned here as
`FilterVisibleUnits` (reached via `pep.FilterVisibleUnits`) and wired as the authoritative second
pass on [tenant](tenant.md)'s unit-result-set reads — `GET /units`, `…/ancestors`, `…/descendants`
(F-002, A-lite), mirrored by the `tenant_units` public-read RLS policy. Other read surfaces enforce
visibility through the reach projection instead: membership/order via the reach-keyed RLS policies,
person/document via the read-scope projection (D-PersonReadScope) — so broad `public` discovery is a
unit-read affordance only and does not yet extend to rosters/people. See
[patterns.md](../architecture/patterns.md).

### RLS backstop (defense-in-depth)

The PDP + shadow gate remain the **authoritative** enforcement; on top of them, Postgres RLS is
enabled as a **DB-level backstop** that mirrors the PDP-computed reach (D-RLSDefenseInDepth). To
feed it, the PDP exposes an **effective read/write unit-set** for the subject: expand each
`subtree` read/write-bearing assignment over its graph's closure, union the `unit`-scope targets —
the **same reach** the shadow gate uses. The composition root pushes that set into per-transaction
session GUCs (`app.person_id`, `app.is_instance_admin`, `app.readable_units`, `app.writable_units`);
RLS policies on unit-scoped tables enforce membership so a query that forgets the PDP/gate cannot
leak rows outside the computed set. RLS **trusts the app-supplied set** — it is a backstop against
forgotten filters, not against PDP-logic errors. The per-txn GUC seam lives in
[platform](platform.md); see [conventions.md](../architecture/conventions.md) and
[upgrade-safety.md](../architecture/upgrade-safety.md) (staged enablement).

## Conjure API surface

`AuthorizationService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /authorize` | Decide `(person, action, unit)` → allow/deny | `assignment.read` |
| `POST /authorize/batch` | Batch decisions / effective set; opt-in `explain` mode (DS-16) | `assignment.read` |
| `POST /roles` | Create a custom role | `role.create` (instance) |
| `GET /roles` / `GET /roles/{id}` | List/read roles | `role.read` |
| `PUT /roles/{id}` | Edit a custom role's permissions | `role.update` (instance) |
| `DELETE /roles/{id}` | Soft-delete a custom role (blocked if assigned) | `role.delete` (instance) |
| `POST /assignments` | Grant `(person, role, unit, scope, graph)` + optional `expiresAt` (D-TimeBoundGrants) | `assignment.grant` (on target unit) |
| `DELETE /assignments/{id}` | Revoke (reversible flip) | `assignment.revoke` |
| `GET /assignments` | List assignments (by person/unit, token-paginated) | `assignment.read` |
| `POST /instance-admins` | Grant instance-admin | `instance.admin.manage` (instance) |
| `DELETE /instance-admins/{id}` | Revoke instance-admin | `instance.admin.manage` (instance) |

`/authorize` is the seam other modules' transport layers call (in-process) before each guarded
operation, and that external PEPs can call over HTTP. Denied → Conjure
`Authorization:PermissionDenied`.

**Decision-explain mode (DS-16).** `POST /authorize/batch` takes an optional request flag
`explain` (default `false`). When set, each decision carries an `explanation`: for **ALLOW**, the
contributing assignment id(s), role `code`, target unit, `scope`, and graph `code` (the union may
name several across graphs); for **DENY**, that the requested permission was absent from the
effective set. Gated by the endpoint's existing `assignment.read` — explain exposes assignment
structure, so it sits behind the same permission, not a "self" exemption.

The HTTP `/authorize` endpoints require **`assignment.read`** — there is **no "self" exemption** for
asking about one's own access (D-BaseRoles, OQ-5). Consequence: authorization self-introspection is
not an end-user capability; a "what can I do here?" UI is driven by a PEP/service holding
`assignment.read`. In-process module→PDP calls before guarded ops are internal and unaffected (not
over this HTTP permission boundary); `/whoami` ([identity-federation](identity-federation.md)) still
returns identity, just not authorization decisions.

## Dependencies

- **Calls:** [tenant](tenant.md) (per-graph closure: `isAncestorOrSelf(graph, …)`, unit
  visibility for the gate, and validating an assignment's `graph` against the registry).
  [person](person.md) (subject exists). [localization](localization.md) (assemble role
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
  the authority to grant; checked against the PDP on the grant path. The **only** exception is the
  install bootstrap (D-Bootstrap), which seeds the first instance admin out-of-band before any
  admin exists, recorded as a `system`/`bootstrap` action.
- **No subtree grants on directory-only graphs** (D-DirectoryGraphs). `POST /assignments` rejects
  `(scope='subtree', graph=G)` where `G.is_authority_bearing = FALSE` with
  `Authorization:NonAuthorityBearingGraph`. `unit`-scope grants are graph-independent and are
  unaffected. The PDP's step-3 filter is the read-side counterpart.
- **Edge mutations require a per-graph or broad edge permission** (D-EdgePerms). Adding or
  removing an edge in graph `g` requires either `unit.edges.<g>.manage` or the broad
  `unit.edges.manage` in the effective set at the path unit. Custom graphs (no specific code yet)
  fall through to the broad permission; the seeded `command` / `operational` graphs each have
  their per-graph code.
- **Reversibility:** assignments and instance-admin grants are revoked by flipping
  `revoked_at`, never deleted — history is preserved for [audit](audit.md).
- **Decisions are pure functions of current data** (assignments + closure + code catalog) — no
  hidden state, no rank, no position.

## Open seams / future

- **Time-bound grants are live** (`expires_at`, D-TimeBoundGrants) — see the data model. **Decision-
  explain** is live on `/authorize/batch` (DS-16) — see the API surface. The **RLS backstop**
  (D-RLSDefenseInDepth) is shipped — see *RLS backstop (defense-in-depth)* above.
- An optional **instance-admin role** (subset of instance permissions) is reserved; default is
  full instance-scope (DS-18).
