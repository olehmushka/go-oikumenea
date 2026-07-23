-- 0004_authz_identity — merged domain migration (refactor: consolidated from 0007_authorization, 0008_identity_federation).

-- ===== merged from 0007_authorization =====
-- 0007 authorization (M7) — the centerpiece: RBAC + the Policy Decision Point (PDP).
--
-- Owns the role/assignment/instance-admin DATA (docs/modules/authorization.md). The atomic
-- PERMISSION catalog is CODE, not a table (domain/permissions.go) — a write to
-- authz_role_permissions with a code outside that catalog is rejected in the application; the
-- closed permission vocabulary is always visible in a diff (D-Ontology ratified divergence).
--
-- Five tables, all expand-only (L-UpgradeSafe / D-Migrations):
--   * authz_roles                — Object Role (code + translatable name; is_base seeded roles).
--   * authz_role_permissions     — Role -> permission-code membership (plain FK rows, no RID).
--   * authz_role_assignments     — the reified Link link__has_role: (subject, role, target_unit,
--                                  scope, graph) + provenance + optional expiry. THE centerpiece.
--   * authz_instance_admins      — the reified Link link__instance_admin: the instance-wide plane.
--   * authz_epoch                — single-row revocation-epoch counter for the per-process grant
--                                  cache (D-AuthzGrantCache, M47) — derived, no RID.
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
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(8,1,1),  -- authz / object / role
  code        text NOT NULL,                 -- stable, locale-agnostic; unique among active rows
  name        text NOT NULL,                 -- default-locale label; translatable via the i18n store
  description text,                           -- default-locale label; translatable via the i18n store
  is_base     boolean NOT NULL DEFAULT false, -- seeded base roles; not instance-editable
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT authz_roles_rid_shape
    CHECK (oikumenea.rid_service(id)=8 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
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
  role_id         uuid NOT NULL REFERENCES oikumenea.authz_roles(id) ON DELETE CASCADE,
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
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(8,2,1),  -- authz / link / has_role
  subject_person_id uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  role_id           uuid NOT NULL REFERENCES oikumenea.authz_roles(id)   ON DELETE RESTRICT,
  target_unit_id    uuid NOT NULL REFERENCES oikumenea.tenant_units(id)  ON DELETE RESTRICT,
  scope             text NOT NULL CHECK (scope IN ('unit','subtree')),
  -- graph_id: the hierarchy a subtree grant cascades over (D-Graphs). NULL iff scope='unit'.
  graph_id          uuid REFERENCES oikumenea.tenant_graphs(id) ON DELETE RESTRICT,
  granted_by        uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL, -- NULL for bootstrap (D-Bootstrap)
  granted_at        timestamptz NOT NULL DEFAULT now(),
  revoked_at        timestamptz,             -- reversible flip; never deleted (history for audit)
  revoked_by        uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  expires_at        timestamptz,             -- optional time bound; evaluated at decision time, silent lapse
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT authz_role_assignments_rid_shape
    CHECK (oikumenea.rid_service(id)=8 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1),
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
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(8,2,2),  -- authz / link / instance_admin
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  granted_by uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL, -- NULL for bootstrap
  granted_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  revoked_by uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT authz_instance_admins_rid_shape
    CHECK (oikumenea.rid_service(id)=8 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
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

-- authz_epoch: single-row revocation-epoch counter (D-AuthzGrantCache, M47 / review-2026-07 R-01.2).
-- Every authority-mutating transaction (grant/revoke assignment, role permission edit/delete,
-- instance-admin grant/revoke, base-role re-sync, person-merge repoint) bumps it; the per-process
-- grant cache validates a stale entry with ONE single-row read instead of re-running the grants
-- join. A derived counter, not an entity — composite-free single row, no RID (ontology-mapping 4.3).
CREATE TABLE oikumenea.authz_epoch (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  epoch     bigint  NOT NULL DEFAULT 0
);
INSERT INTO oikumenea.authz_epoch (singleton, epoch) VALUES (true, 0);
COMMENT ON COLUMN oikumenea.authz_epoch.epoch IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0008_identity_federation =====
-- 0008 identity-federation (M8) — the external-IdP seam: optional login accounts + the verified
-- external identities they federate (docs/modules/identity-federation.md).
--
-- go-oikumenea does NOT authenticate (L-AuthzOnly): it stores no credentials and issues no tokens.
-- It owns the optional login `account` (an attachment to a person) and the `(issuer, subject)`
-- external identities linked to it; inbound-token validation (OIDC discovery + JWKS) is middleware
-- (code, not a table) that maps a verified token -> external identity -> account -> person -> PDP
-- subject. The first instance admin is seeded out-of-band from install config (D-Bootstrap), creating
-- the first account + external_identity in one transaction.
--
-- Two tables, both expand-only (L-UpgradeSafe / D-Migrations):
--   * account_accounts             — Object Account: the optional login attachment (<=1 active per
--                                    person, HAS_ACCOUNT). Carries status + a DORMANT seam of
--                                    credential columns (CHECK-NULL) for a future full-IdP pivot.
--   * account_external_identities  — Object External identity: a verified (issuer, subject) pair
--                                    federating to an account (FEDERATES). Globally unique;
--                                    immutable once created (no UPDATE), removed by unlink (DELETE).
--
-- This migration is PURE DDL: it seeds NO rows. The first-admin account/identity is seeded at BOOT
-- (or by the recover-admin CLI) by the app on the GUC-bearing pool (D-Bootstrap / D-RIDSeeding) — not
-- here, where atlas's connection has no app.environment GUC for new_rid(). Depends on 0001 bootstrap
-- (new_rid/set_updated_at/reject_mutation, citext) and 0006 person (person_persons).

-- account_accounts: an optional login attachment to exactly one person (Object Account). A person may
-- have zero accounts (roster-only, L-AccountOptional) or one active account. Tokens/passwords are
-- NEVER stored while auth is delegated; the dormant credential columns are CHECK-enforced NULL until a
-- future "become a full IdP" pivot ships (additive, not a rewrite — patterns.md Dormant seam).
CREATE TABLE oikumenea.account_accounts (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(9,1,1),  -- account / object / account
  person_id       uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  email           citext,                         -- optional, as asserted by the IdP; unique among active when set
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  -- Dormant seam (always NULL while auth is delegated — L-AuthzOnly). password_hash is `secret`
  -- (a separate axis from the pii: tiers — D-PIITiers), not a credential we keep today.
  password_hash   text,
  mfa_enrolled_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,

  CONSTRAINT account_accounts_rid_shape
    CHECK (oikumenea.rid_service(id)=9 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1),
  -- Dormant credential columns stay NULL until the full-IdP pivot ships (then this CHECK is dropped).
  CONSTRAINT account_accounts_dormant_credentials CHECK (password_hash IS NULL AND mfa_enrolled_at IS NULL)
);

CREATE TRIGGER account_accounts_set_updated_at
  BEFORE UPDATE ON oikumenea.account_accounts
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- At most one active account per person (HAS_ACCOUNT, <=1 active).
CREATE UNIQUE INDEX account_accounts_person_active_idx
  ON oikumenea.account_accounts (person_id) WHERE deleted_at IS NULL;
-- The IdP-asserted email is unique among active accounts when present.
CREATE UNIQUE INDEX account_accounts_email_active_idx
  ON oikumenea.account_accounts (email) WHERE email IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.account_accounts.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_accounts.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.account_accounts.email IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_accounts.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_accounts.password_hash IS 'secret';
COMMENT ON COLUMN oikumenea.account_accounts.mfa_enrolled_at IS 'pii:none';

-- account_external_identities: a verified (issuer, subject) login point federating to one account
-- (Object External identity / FEDERATES). The schema permits MANY rows per account_id — one per login
-- point (e.g. a Google identity AND a Keycloak identity for the same person) — and the cap on
-- ADDITIONAL links is enforced in the application at link time (account.identity_linking.enabled),
-- NOT by a DB constraint, so flipping that config is reversible without a migration. No token columns:
-- access/refresh tokens are never persisted. The row is immutable once created (an UPDATE guard); an
-- unlink is a hard DELETE (there is no deleted_at — the FEDERATES link either exists or is removed).
CREATE TABLE oikumenea.account_external_identities (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(9,1,2),  -- account / object / external_identity
  account_id uuid NOT NULL REFERENCES oikumenea.account_accounts(id) ON DELETE CASCADE,
  issuer     text NOT NULL,                       -- the IdP `iss`
  subject    text NOT NULL,                       -- the IdP `sub` (pseudonymous identifier)
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT account_external_identities_rid_shape
    CHECK (oikumenea.rid_service(id)=9 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);

-- Immutable once created: block UPDATE (an identity's (issuer, subject, account) never changes) while
-- still permitting the unlink hard-DELETE. reject_mutation() raises on any firing, so attach it to the
-- UPDATE event only (cf. audit_log, which guards UPDATE OR DELETE for a true append-only ledger).
CREATE TRIGGER account_external_identities_no_update
  BEFORE UPDATE ON oikumenea.account_external_identities
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

-- A given external identity maps to exactly one account, globally.
CREATE UNIQUE INDEX account_external_identities_issuer_subject_idx
  ON oikumenea.account_external_identities (issuer, subject);
CREATE INDEX account_external_identities_account_idx
  ON oikumenea.account_external_identities (account_id);

COMMENT ON COLUMN oikumenea.account_external_identities.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.account_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.issuer IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.subject IS 'pii:basic';
COMMENT ON COLUMN oikumenea.account_external_identities.created_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0004_authz_identity', applied_at = now() WHERE singleton;
