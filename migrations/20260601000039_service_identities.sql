-- 0039 service identities (M51) — machine subjects: the service-principal registry + the flat
-- per-principal grants that carry their authority (D-ServiceIdentities, docs/architecture/north-star.md).
--
-- A SERVICE PRINCIPAL is a machine caller — a facade with standing of its own (M52), or a connector
-- (M53). It authenticates through the SAME external IdP and the SAME OIDC/JWKS middleware as a human,
-- using the OAuth2 client-credentials grant, and resolves to a principal instead of a person.
-- L-AuthzOnly is untouched: the IdP owns the client secret; oikumenea stores no credentials and issues
-- no tokens. This generalizes the M16 `hermenea-importer` (D-Hermenea) from a token-mapped singleton
-- into a registry.
--
-- WHY GRANTS AND NOT ROLES. A principal holds flat `(permission_code, org_id)` grants, never a role
-- assignment, because two properties of the built system forbid the "give the machine a role" reading:
--   * authz_role_assignments.subject_person_id is a hard person_persons FK (0007); and
--   * the PDP satisfies INSTANCE-SCOPE permissions — which import.manage and every M53 wiring code
--     are — only for instance admins, never through a role (internal/authorization/domain/pdp.go,
--     D-InstanceAdmin). A principal given a role could therefore not import at all.
-- Flat grants also keep machines out of the unit DAG, so the M47 reach/RLS hot path (D-RLSLiveReach,
-- D-AuthzGrantCache) is untouched. The blast-radius boundary is the ORGANIZATION, not the unit:
-- org_id NULL = instance-wide (reference catalogs), a named org confines a connector to that org's
-- data (D-TenantOrganizations) — a church scraper cannot rewrite another organization's structure.
--
-- NO REACH IN M51. A service request sets no app.person_id GUC, so every person-shaped PEP path denies
-- a principal at its existing empty-subject guard. Machine access to RLS-protected, organization-owned
-- data (scraped units, memberships, clergy, university staff) needs an RLS service arm and lands with
-- M53 / D-ConnectorPlane. The org_id column ships now so M53 retrofits nothing.
--
-- Three changes, all expand-only (L-UpgradeSafe / D-Migrations):
--   * account_service_principals — Object ServicePrincipal (9,1,3), owned by identity-federation.
--   * authz_principal_grants     — Link PRINCIPAL_GRANT (8,2,3), owned by authorization.
--   * audit_log.actor_principal_id — a `system` actor that names itself (no third actor_type).
-- Depends on 0000 bootstrap (new_id/set_updated_at/platform_rid_types), 0001 audit_log, 0003 tenant
-- (tenant_organizations), 0007 authorization, 0008 identity-federation (account_external_identities).

-- ---------------------------------------------------------------------------------------------------
-- RID types. Three registries must agree or rid.AssertMatches fails the boot (pkg/rid/registry.go,
-- this seed, docs/ontology-mapping.md). Services 8 (authz) and 9 (account) already exist.
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (9, 1, 3, 'service_principal'),
  (8, 2, 3, 'principal_grant')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- account_service_principals: the (issuer, subject) -> machine-subject registry (Object
-- ServicePrincipal). Same key shape as account_external_identities, so one uniform resolution path and
-- no IdP-specific claim names.
--
-- Unlike account_external_identities this table is NOT append-only (name/description/client_id/status
-- are editable), so it carries no reject_mutation() guard; (issuer, subject) immutability is enforced
-- in the application service, which returns a typed 409 rather than a raw constraint error.
CREATE TABLE oikumenea.account_service_principals (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(9,1,3),  -- account / object / service_principal
  -- Stable, locale-agnostic machine name (D-Code) — what operators and the audit ledger reference.
  code        text NOT NULL,
  name        text NOT NULL,
  description text,
  -- The IdP `iss` and `sub` of the client-credentials token. `subject` names a MACHINE, not a person,
  -- so it is pii:none here — unlike account_external_identities.subject (pii:basic).
  issuer      text NOT NULL,
  subject     text NOT NULL,
  -- Optional display label projected from the token's azp / client_id claim, so an operator can see
  -- which IdP client a principal is. NEVER an authorization input: the identity key is (issuer, subject).
  client_id   text,
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT account_service_principals_rid_shape
    CHECK (oikumenea.rid_service(id)=9 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);

CREATE TRIGGER account_service_principals_set_updated_at
  BEFORE UPDATE ON oikumenea.account_service_principals
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX account_service_principals_identity_active_idx
  ON oikumenea.account_service_principals (issuer, subject) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX account_service_principals_code_active_idx
  ON oikumenea.account_service_principals (code) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.account_service_principals.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.description IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.issuer IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.subject IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.client_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_service_principals.status IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- An (issuer, subject) is a PERSON identity XOR a MACHINE principal — never both, or a token would
-- resolve to two different subjects. Two UNIQUE indexes on two tables cannot express that, so
-- symmetric guards reject a collision from either side. The application pre-checks and returns a typed
-- 409; these triggers are the backstop.
--
-- LIMITATION: at READ COMMITTED these are not race-free against a simultaneous insert into the
-- counterpart table (neither statement sees the other's uncommitted row). Accepted: both sides are
-- instance-admin mutations at human rate, and the failure mode is a duplicate registration an admin
-- can delete — not a privilege escalation, since resolution tries the person path first and a
-- colliding principal would simply never resolve.
CREATE OR REPLACE FUNCTION oikumenea.reject_principal_identity_collision() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM oikumenea.account_external_identities e
     WHERE e.issuer = NEW.issuer AND e.subject = NEW.subject
  ) THEN
    RAISE EXCEPTION 'issuer/subject % / % is already a person external identity', NEW.issuer, NEW.subject
      USING ERRCODE = 'unique_violation',
            CONSTRAINT = 'account_service_principals_identity_collision';
  END IF;
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION oikumenea.reject_identity_principal_collision() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM oikumenea.account_service_principals p
     WHERE p.issuer = NEW.issuer AND p.subject = NEW.subject AND p.deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'issuer/subject % / % is already a service principal', NEW.issuer, NEW.subject
      USING ERRCODE = 'unique_violation',
            CONSTRAINT = 'account_external_identities_principal_collision';
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER account_service_principals_no_identity_collision
  BEFORE INSERT OR UPDATE OF issuer, subject ON oikumenea.account_service_principals
  FOR EACH ROW WHEN (NEW.deleted_at IS NULL)
  EXECUTE FUNCTION oikumenea.reject_principal_identity_collision();

CREATE TRIGGER account_external_identities_no_principal_collision
  BEFORE INSERT ON oikumenea.account_external_identities
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_identity_principal_collision();

-- ---------------------------------------------------------------------------------------------------
-- authz_principal_grants: the reified PRINCIPAL_GRANT link (principal -> permission code), owned by
-- authorization. Flat by construction — no target unit, no scope, no graph: a machine has no reach.
--
-- permission_code is validated in the APPLICATION against the Go catalog (domain.Catalog()), exactly
-- as authz_role_permissions is — the permission catalog is CODE, never a table (D-Code / 0007).
--
-- org_id NULL = instance-wide (reference-catalog imports, M53 wiring codes); a named organization
-- confines the principal to that org's data. Instance-plane data like authz_roles: no RLS.
CREATE TABLE oikumenea.authz_principal_grants (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(8,2,3),  -- authz / link / principal_grant
  principal_id    uuid NOT NULL REFERENCES oikumenea.account_service_principals(id) ON DELETE RESTRICT,
  permission_code text NOT NULL,
  org_id          uuid REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  granted_by      uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  granted_at      timestamptz NOT NULL DEFAULT now(),
  revoked_at      timestamptz,
  revoked_by      uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,

  CONSTRAINT authz_principal_grants_rid_shape
    CHECK (oikumenea.rid_service(id)=8 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3)
);

-- Two partial uniques: a NULL org_id does not de-duplicate under a plain UNIQUE (NULLs are distinct),
-- so the instance-wide case needs its own index.
CREATE UNIQUE INDEX authz_principal_grants_instance_active_idx
  ON oikumenea.authz_principal_grants (principal_id, permission_code)
  WHERE org_id IS NULL AND revoked_at IS NULL;
CREATE UNIQUE INDEX authz_principal_grants_org_active_idx
  ON oikumenea.authz_principal_grants (principal_id, permission_code, org_id)
  WHERE org_id IS NOT NULL AND revoked_at IS NULL;
-- The per-request authority fetch reads every active grant of one principal.
CREATE INDEX authz_principal_grants_principal_active_idx
  ON oikumenea.authz_principal_grants (principal_id) WHERE revoked_at IS NULL;

COMMENT ON COLUMN oikumenea.authz_principal_grants.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_principal_grants.principal_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_principal_grants.permission_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_principal_grants.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_principal_grants.granted_by IS 'pii:basic';
COMMENT ON COLUMN oikumenea.authz_principal_grants.revoked_by IS 'pii:basic';

-- ---------------------------------------------------------------------------------------------------
-- Audit: a principal is a `system` actor that NAMES ITSELF. D-Audit's two actor kinds are binding —
-- no third actor_type — so this is an additive column under the existing system arm. Declared on the
-- partitioned parent, so it cascades to every current and future partition.
--
-- The replacement CHECK is strictly WEAKER than the shipped one (it only adds a NULL requirement on
-- the person arm, where the column is already NULL for every existing row), so no row can fail
-- validation. `subsystem` keeps its meaning: the originating subsystem, not the actor.
ALTER TABLE oikumenea.audit_log ADD COLUMN actor_principal_id uuid;

ALTER TABLE oikumenea.audit_log DROP CONSTRAINT audit_log_actor_shape;
ALTER TABLE oikumenea.audit_log ADD CONSTRAINT audit_log_actor_shape CHECK (
  (actor_type = 'person' AND actor_person_id IS NOT NULL AND subsystem IS NULL AND actor_principal_id IS NULL)
  OR
  (actor_type = 'system' AND actor_person_id IS NULL AND subsystem IS NOT NULL)
);

CREATE INDEX audit_log_actor_principal_idx
  ON oikumenea.audit_log (actor_principal_id) WHERE actor_principal_id IS NOT NULL;

COMMENT ON COLUMN oikumenea.audit_log.actor_principal_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0039_service_identities', applied_at = now() WHERE singleton;
