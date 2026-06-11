-- 0007 authorization (M7) — the centerpiece: RBAC + the Policy Decision Point (PDP).
--
-- Owns the role/assignment/instance-admin DATA (docs/modules/authorization.md). The atomic
-- PERMISSION catalog is CODE, not a table (domain/permissions.go) — a write to
-- authz_role_permissions with a code outside that catalog is rejected in the application; the
-- closed permission vocabulary is always visible in a diff (D-Ontology ratified divergence).
--
-- Four tables, all expand-only (L-UpgradeSafe / D-Migrations):
--   * authz_roles                — Object Role (code + translatable name; is_base seeded roles).
--   * authz_role_permissions     — Role -> permission-code membership (plain FK rows, no RID).
--   * authz_role_assignments     — the reified Link link__has_role: (subject, role, target_unit,
--                                  scope, graph) + provenance + optional expiry. THE centerpiece.
--   * authz_instance_admins      — the reified Link link__instance_admin: the instance-wide plane.
--
-- This migration is PURE DDL: it seeds NO rows. The four base roles (D-BaseRoles) are RID-keyed, so
-- they are seeded at BOOT by authz.Register on the GUC-bearing pool (D-RIDSeeding) — not here, where
-- atlas's connection has no app.environment GUC for new_rid(). Authority comes ONLY from assignments
-- here; rank (person) and position (membership) are directory attributes and are never PDP inputs
-- (D-Rank / D-Position). Depends on 0001 bootstrap (new_rid/set_updated_at), 0004 tenant
-- (tenant_units, tenant_graphs), 0006 person (person_persons).

-- authz_roles: a named set of permission codes (Object Role). `code` is the stable, locale-agnostic
-- identifier external systems reference (D-Code); `name`/`description` are default-locale fallbacks,
-- translatable via the i18n store (M2). Base roles (is_base) are seeded and immutable by instance
-- admins.
CREATE TABLE oikumenea.authz_roles (
  id          text PRIMARY KEY DEFAULT oikumenea.new_rid('authz','role'),
  code        text NOT NULL,                 -- stable, locale-agnostic; unique among active rows
  name        text NOT NULL,                 -- default-locale label; translatable via the i18n store
  description text,                           -- default-locale label; translatable via the i18n store
  is_base     boolean NOT NULL DEFAULT false, -- seeded base roles; not instance-editable
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT authz_roles_rid_shape CHECK (id LIKE 'urn:oikumenea:authz:%:role:%')
);

CREATE TRIGGER authz_roles_set_updated_at
  BEFORE UPDATE ON oikumenea.authz_roles
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` is unique among active (non-deleted) roles; immutable by convention (D-Code).
CREATE UNIQUE INDEX authz_roles_code_active_idx
  ON oikumenea.authz_roles (code) WHERE deleted_at IS NULL;

-- Role definitions are organizational metadata, not personal data (D-PIITiers).
COMMENT ON COLUMN oikumenea.authz_roles.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_roles.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_roles.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_roles.description IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_roles.is_base IS 'pii:none';

-- authz_role_permissions: a role's membership of code-defined permissions. No RID (a plain
-- GRANTS link, carrying no identity/attributes/history — stays a composite-PK row, not a reified
-- Link). `permission_code` is validated against the code catalog at write time in the application.
CREATE TABLE oikumenea.authz_role_permissions (
  role_id         text NOT NULL REFERENCES oikumenea.authz_roles(id) ON DELETE CASCADE,
  permission_code text NOT NULL,             -- validated against domain/permissions.go at write time
  PRIMARY KEY (role_id, permission_code)
);

COMMENT ON COLUMN oikumenea.authz_role_permissions.role_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_permissions.permission_code IS 'pii:none';

-- authz_role_assignments: the reified Link link__has_role — the unit of granted authority and the
-- PDP's core input (D-Inherit / D-Graphs). (subject_person, role, target_unit, scope, graph) +
-- provenance (granted_by/at, revoked_by/at) + optional decision-time expiry (D-TimeBoundGrants).
-- graph_id names the hierarchy a `subtree` grant cascades over and is NULL iff scope='unit'
-- (a `unit` grant is graph-independent). target_unit is independent of where the subject sits.
CREATE TABLE oikumenea.authz_role_assignments (
  id                text PRIMARY KEY DEFAULT oikumenea.new_rid('authz','link__has_role'),
  subject_person_id text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  role_id           text NOT NULL REFERENCES oikumenea.authz_roles(id)   ON DELETE RESTRICT,
  target_unit_id    text NOT NULL REFERENCES oikumenea.tenant_units(id)  ON DELETE RESTRICT,
  scope             text NOT NULL CHECK (scope IN ('unit','subtree')),
  -- graph_id: the hierarchy a subtree grant cascades over (D-Graphs). NULL iff scope='unit'.
  graph_id          text REFERENCES oikumenea.tenant_graphs(id) ON DELETE RESTRICT,
  granted_by        text REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL, -- NULL for bootstrap (D-Bootstrap)
  granted_at        timestamptz NOT NULL DEFAULT now(),
  revoked_at        timestamptz,             -- reversible flip; never deleted (history for audit)
  revoked_by        text REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  expires_at        timestamptz,             -- optional time bound; evaluated at decision time, silent lapse
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT authz_role_assignments_rid_shape CHECK (id LIKE 'urn:oikumenea:authz:%:link\_\_has_role:%'),
  -- NULL iff scope='unit' — a subtree grant always names its graph; a unit grant never does.
  CONSTRAINT authz_role_assignments_graph_scope CHECK ((scope = 'subtree') = (graph_id IS NOT NULL))
);

CREATE TRIGGER authz_role_assignments_set_updated_at
  BEFORE UPDATE ON oikumenea.authz_role_assignments
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- Active-uniqueness keyed on revoked_at ONLY (D-TimeBoundGrants): an expired-not-revoked row still
-- occupies its tuple, so renewal is an UPDATE of expires_at and re-granting an identical expired
-- tuple requires revoking the stale row first. graph_id NULL distinguishes a unit grant's tuple.
CREATE UNIQUE INDEX authz_role_assignments_active_idx
  ON oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope, graph_id)
  WHERE revoked_at IS NULL;
CREATE INDEX authz_role_assignments_subject_idx
  ON oikumenea.authz_role_assignments (subject_person_id) WHERE revoked_at IS NULL;
CREATE INDEX authz_role_assignments_target_idx
  ON oikumenea.authz_role_assignments (target_unit_id) WHERE revoked_at IS NULL;

-- The fact that a specific person holds authority over a unit is mildly identifying; the assignment
-- itself is organizational. Subject/grantor person ids are stable ids (pii:none).
COMMENT ON COLUMN oikumenea.authz_role_assignments.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.subject_person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.authz_role_assignments.role_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.target_unit_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.authz_role_assignments.scope IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.graph_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.granted_by IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.granted_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.revoked_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.revoked_by IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_role_assignments.expires_at IS 'pii:none';

-- authz_instance_admins: the reified Link link__instance_admin — a person holding the instance-wide
-- authority plane (D-InstanceAdmin), distinct from any unit assignment. granted_by is NULL for the
-- install bootstrap grant (no granter exists yet — D-Bootstrap; origin lives in the bootstrap audit
-- row). Reversible (revoked_at flip), never deleted.
CREATE TABLE oikumenea.authz_instance_admins (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('authz','link__instance_admin'),
  person_id  text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  granted_by text REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL, -- NULL for bootstrap
  granted_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  revoked_by text REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT authz_instance_admins_rid_shape CHECK (id LIKE 'urn:oikumenea:authz:%:link\_\_instance_admin:%')
);

CREATE TRIGGER authz_instance_admins_set_updated_at
  BEFORE UPDATE ON oikumenea.authz_instance_admins
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- At most one active instance-admin grant per person.
CREATE UNIQUE INDEX authz_instance_admins_person_active_idx
  ON oikumenea.authz_instance_admins (person_id) WHERE revoked_at IS NULL;

COMMENT ON COLUMN oikumenea.authz_instance_admins.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_instance_admins.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.authz_instance_admins.granted_by IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_instance_admins.granted_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_instance_admins.revoked_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_instance_admins.revoked_by IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0007_authorization', applied_at = now() WHERE singleton;
