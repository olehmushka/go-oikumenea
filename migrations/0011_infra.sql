-- 0011_infra — merged domain migration (refactor: consolidated from 0036_events_outbox, 0037_search_generalization, 0038_person_health, 0039_service_identities, 0040_connector_plane, 0041_datapacks, 0042_rls_service_arm, 0043_login_security_log, 0044_login_event_rid_seed).

-- ===== merged from 0036_events_outbox =====
-- 0036 events outbox (review-2026-07 R-10 / D-EventOutbox).
--
-- The transactional outbox for the `notify` event class (docs/architecture/patterns.md): effects that
-- must NOT widen the write transaction (webhooks, projections, cache invalidation, R-01's grant-cache
-- epoch bump). A producer enqueues one row on its own write transaction (pkg/events.OutboxWriter), so the
-- event commits atomically with the originating write; the out-of-process dispatcher
-- (internal/platform/outbox) drains the queue AFTER COMMIT, at least once, claiming rows FOR UPDATE SKIP
-- LOCKED so multiple replicas share it safely (mirrors the hermenea worker, R-13).
--
-- The atomic Bus (pkg/events, same-transaction subscribers) is unchanged and stays the default; today
-- every domain event is `atomic`, so this table is a live-but-empty seam proven by tests until the first
-- `notify` producer lands. Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 bootstrap
-- helpers (uuid_v7, set_updated_at). This is infrastructure plumbing (like schema_version) — NOT an
-- ontology entity, so the id is a plain time-ordered uuid, not a new_id() RID.

CREATE TABLE oikumenea.platform_outbox (
  -- Chronologically-ordered surrogate key (uuid_v7 time component orders the queue by enqueue time).
  id              uuid PRIMARY KEY DEFAULT oikumenea.uuid_v7(),
  event_type      text NOT NULL,                 -- the notify event's dispatch key (Event.Type())
  payload         jsonb NOT NULL,                -- the JSON-marshaled concrete event
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','dispatched','dead')),
  attempts        integer NOT NULL DEFAULT 0,    -- delivery attempts so far
  max_attempts    integer NOT NULL DEFAULT 10,   -- dead-letter after this many failures
  next_attempt_at timestamptz NOT NULL DEFAULT now(),  -- earliest next delivery (backoff schedule)
  last_error      text,                          -- last handler error (diagnostics)
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  dispatched_at   timestamptz                    -- set when a row reaches 'dispatched'
);

-- Claim index: the dispatcher scans due, still-pending rows oldest-first; partial so the index stays
-- small as dispatched/dead rows accumulate (retention is an operator concern, like audit partitions).
CREATE INDEX platform_outbox_due_idx
  ON oikumenea.platform_outbox (next_attempt_at, id)
  WHERE status = 'pending';

CREATE TRIGGER platform_outbox_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_outbox
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- No RLS: the outbox is system-actor infrastructure, never subject to the per-request unit reach
-- (only the four D-RLSDefenseInDepth tables carry policies). No reject_mutation trigger: rows are
-- mutable by design (status transitions pending -> dispatched / dead).

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0037_search_generalization =====
-- 0037 search generalization (review-2026-08 R-21 / D-PersonSearch generalized).
--
-- July's R-06 (D-PersonSearch) made the person directory typeahead index-served: a pg_trgm GIN over a
-- generated search haystack turns an unanchored ILIKE '%q%' from a per-keystroke sequential scan into a
-- bitmap index scan. Five other typeahead/list surfaces kept the un-indexed pattern — language (27,177
-- languoids seeded at first boot by pinax), geo locations, education institutions, education
-- publications/scholarships, and companies. This migration extends the same trigram machinery to each.
--
-- Index strategy (docs/architecture/decisions.md, D-PersonSearch "Generalized"): a multi-column search is
-- served by a SINGLE GIN trigram index over a STORED generated `search_text` haystack — exactly the
-- person_persons pattern. EXPLAIN on 30k synthetic rows settled the shape: a two-index BitmapOr and an
-- EXPRESSION index over `col || ' ' || col` both LOSE to a seq scan, because the planner has no
-- selectivity statistics for an ILIKE (`~~*`) over a bare expression and defaults to ~4% → a seq scan
-- looks cheaper. A STORED column carries real pg_stats, so the planner estimates the true (low)
-- selectivity of a rare substring and uses the GIN index. The cost is one narrow generated column per
-- table (and its presence in the sqlc-generated model structs, which nothing reads) — worth it for a
-- search that is actually index-served. The application splits each filtered list into an unfiltered List
-- query and a trigram Search query so the predicate is never guarded by `(@q = '' OR …)` — which defeats
-- the index under a generic prepared-statement plan.
--
-- Expand-only (L-UpgradeSafe / D-Migrations): additive indexes + one generated column; pg_trgm already
-- exists (migration 0005). PURE DDL, seeds no rows (no app.environment GUC — D-RIDSeeding). Adding the
-- generated column rewrites location_locations once at apply time.

-- pg_trgm makes unanchored ILIKE '%q%' index-servable via GIN. Idempotent — created in 0005.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- 1. language_languoids (internal/language SearchLanguoids): STORED haystack over name + glottocode. code
-- is char(8) (8-char glottocodes, no padding); the concat casts it to text. Reference data: no deleted_at,
-- so no partial clause.
ALTER TABLE oikumenea.language_languoids
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(name || ' ' || code)) STORED;
CREATE INDEX language_languoids_search_trgm
  ON oikumenea.language_languoids USING gin (search_text gin_trgm_ops);
COMMENT ON COLUMN oikumenea.language_languoids.search_text IS 'pii:none';

-- 2. location_locations (internal/geo SearchLocationsByText): STORED haystack over the six address columns.
ALTER TABLE oikumenea.location_locations
  ADD COLUMN search_text text GENERATED ALWAYS AS (
    lower(coalesce(locality,'')     || ' ' || coalesce(admin_area_1,'') || ' ' ||
          coalesce(admin_area_2,'') || ' ' || coalesce(street,'')      || ' ' ||
          coalesce(mgrs,'')         || ' ' || coalesce(raw_address,''))) STORED;
CREATE INDEX location_locations_search_trgm
  ON oikumenea.location_locations USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.location_locations.search_text IS 'pii:none';

-- 3. tenant_organizations (shared: internal/education SearchInstitutions AND internal/company
-- SearchCompanies both search org code+name). STORED haystack over code + name; partial on active rows.
-- Both columns are NOT NULL. This column also surfaces in the sqlc model wherever the table is read — an
-- inert extra field (nothing selects it into a domain type).
ALTER TABLE oikumenea.tenant_organizations
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || name)) STORED;
CREATE INDEX tenant_organizations_search_trgm
  ON oikumenea.tenant_organizations USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_organizations.search_text IS 'pii:basic';

-- 4. company_org_profiles.short_name (internal/company SearchCompanies): short_name lives on the joined
-- profile table, so its match branch is a UNION arm — a GIN on the real column (which carries pg_stats),
-- indexed independently of the org haystack.
CREATE INDEX company_org_profiles_short_name_trgm
  ON oikumenea.company_org_profiles USING gin (short_name gin_trgm_ops) WHERE deleted_at IS NULL;

-- 5. education reference (internal/education SearchPublications/SearchScholarships): STORED haystack per
-- catalog over its two matched columns; partial on active rows. All four columns are NOT NULL.
ALTER TABLE oikumenea.education_publications
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || title)) STORED;
CREATE INDEX education_publications_search_trgm
  ON oikumenea.education_publications USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.education_publications.search_text IS 'pii:none';
ALTER TABLE oikumenea.education_scholarships
  ADD COLUMN search_text text GENERATED ALWAYS AS (lower(code || ' ' || name)) STORED;
CREATE INDEX education_scholarships_search_trgm
  ON oikumenea.education_scholarships USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.education_scholarships.search_text IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0038_person_health =====
-- 0038 person health & vulnerability records (M36 — D-HealthVulnerability). The draft's macro-category 8,
-- built last so it reuses the proven M31/M33/M35 special-PII envelope + the M29 legal_basis/audit
-- substrate. Category-level only (NO diagnosis text), NEVER inferred — the strictest gate.
--
--   person_health_records — a typed, category-level health/vulnerability record (hospitalization,
--                           mental_health, disability). GDPR Art. 9 special category → pii:special: the
--                           category-level `detail` (a coarse, functional-level note — NO diagnosis) is
--                           envelope-encrypted (ciphertext/wrapped_dek/key_ref/blind_index sealed in the
--                           application; NO plaintext value column) with a NOT-NULL legal_basis and a
--                           need-to-know read gate (person.health.read). One active row per (person, kind),
--                           refreshed in place. Crypto-erased on purge (drop the envelope, keep a tombstone).
--   person_insurance      — a person's insurance coverage (health/life/disability/ltc), provider +
--                           employer-sponsored flag + validity window. pii:sensitive (plaintext), gated on
--                           person.read. Hard-erased on purge.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons) and 0028 (platform_legal_basis_kinds).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,20,'health_record'),  -- person / object / health_record (M36, encrypted)
  (6,1,21,'insurance');      -- person / object / insurance     (M36)

-- ===================================================================================================
-- person_health_records — a category-level health/vulnerability record Object. Health is a GDPR Art. 9
-- special category, so the category-level `detail` value is envelope-encrypted (detail_ciphertext/
-- detail_wrapped_dek/detail_key_ref/detail_blind_index sealed in the application; NO plaintext detail
-- column). The coarse `kind` discriminator stays in plaintext so it drives the one-active-per-(person,kind)
-- index and stays queryable, but knowing a person has (say) a mental-health record is itself special, so
-- `kind` is marked pii:special. NEVER inferred. Crypto-erased on purge (drop the envelope, keep the row).
-- ===================================================================================================
CREATE TABLE oikumenea.person_health_records (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,20),  -- person / object / health_record
  person_id          uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  kind               text NOT NULL
                       CHECK (kind IN ('hospitalization','mental_health','disability')),
  -- envelope-encrypted category-level detail (a coarse, functional-level note — NO diagnosis). Always
  -- populated at creation, but NULLABLE so crypto-erase on purge can drop all four (the row survives).
  detail_ciphertext  bytea,
  detail_wrapped_dek bytea,
  detail_key_ref     text,
  detail_blind_index bytea,
  is_public_record   boolean NOT NULL DEFAULT false,                     -- true when derived from a public record
  assessed_at        date,
  legal_basis        text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  source             text NOT NULL DEFAULT 'imported'
                       CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence         text NOT NULL DEFAULT 'possible'
                       CHECK (confidence IN ('confirmed','probable','possible')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT person_health_records_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=20)
);

CREATE TRIGGER person_health_records_set_updated_at
  BEFORE UPDATE ON oikumenea.person_health_records
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_health_records_person_idx
  ON oikumenea.person_health_records (person_id) WHERE deleted_at IS NULL;
-- One active record per (person, kind) — refreshed in place.
CREATE UNIQUE INDEX person_health_records_person_kind
  ON oikumenea.person_health_records (person_id, kind) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_health_records.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_health_records.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_health_records.kind IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_health_records.detail_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_health_records.detail_wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_health_records.detail_key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_health_records.detail_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_health_records.is_public_record IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_health_records.assessed_at IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_health_records.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_health_records.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_health_records.confidence IS 'pii:none';

-- ===================================================================================================
-- person_insurance — a person's insurance coverage Object (plaintext pii:sensitive). Gated on person.read
-- (no dedicated need-to-know code, exactly like the M35 crypto-wallet/personality overlays). Hard-erased
-- on purge.
-- ===================================================================================================
CREATE TABLE oikumenea.person_insurance (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,21),  -- person / object / insurance
  person_id          uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  type               text NOT NULL
                       CHECK (type IN ('health','life','disability','ltc')),
  provider           text,                                               -- the insurer's name
  policy_reference   text,                                               -- an operator-held policy handle
  employer_sponsored boolean NOT NULL DEFAULT false,
  valid_from         date,
  valid_to           date,
  source             text NOT NULL DEFAULT 'imported'
                       CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence         text NOT NULL DEFAULT 'possible'
                       CHECK (confidence IN ('confirmed','probable','possible')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT person_insurance_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=21)
);

CREATE TRIGGER person_insurance_set_updated_at
  BEFORE UPDATE ON oikumenea.person_insurance
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_insurance_person_idx
  ON oikumenea.person_insurance (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_insurance.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_insurance.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_insurance.type IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_insurance.provider IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_insurance.policy_reference IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_insurance.employer_sponsored IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_insurance.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_insurance.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_insurance.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_insurance.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0039_service_identities =====
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

-- ===== merged from 0040_connector_plane =====
-- 0040 connector plane — the fleet registry (M53 — D-ConnectorPlane). oikumenea gains VISIBILITY over
-- the family of connectors that feed it, WITHOUT gaining orchestration: connectors keep their own
-- storage and scheduler and couple to the core over HTTP only (the D-Hermenea boundary, kept). The
-- core never schedules, triggers or retries a run; it records what a connector REPORTS.
--
-- Vocabulary note: a "connector" here is a WHOLE DEPLOYABLE AGENT beside the core (hermenea is the
-- first). That is a different, higher altitude than hermenea's internal `Fetcher` seam (a fetch
-- strategy — http/file/wof-sqlite), which was called `Connector` through M52 and was renamed in M53
-- precisely so these two never collide. hermenea's own `connector_type` column keeps its name.
--
-- Tables:
--   * OBJECT — connector_connectors: a registered agent. `code` is the stable handle (D-Code);
--              principal_id ties it to the M51 service principal it authenticates as, which is what
--              makes a report attributable. `last_seen_at` is a liveness hint, not a health verdict.
--   * OBJECT — connector_sources: one dataset a connector syncs. `object_type` names the core import
--              target (e.g. geo-places) for push-mode sources; NULL for sources that only feed the
--              connector's own lookups. Mirrors what the connector reports about itself — the
--              connector remains AUTHORITATIVE for execution, this is a read model.
--   * OBJECT — connector_sync_runs: one reported execution. A first-class Object with its own RID
--              (not an audit-only line) because the M53 exit bar requires a completed run to be
--              VISIBLE and queryable from the core; the audit ledger records the reporting action.
--
-- Sources are (connector, code) unique; runs are append-mostly (a connector opens a run, then closes
-- it with counts or an error). Nothing here is RLS-guarded: the registry is instance-plane operator
-- data with no person or organization dimension. Machine access to RLS-protected, ORGANIZATION-owned
-- data is deliberately NOT part of this milestone — that is the RLS service arm, M55.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id / platform_rid_types) and
-- 0039 (account_service_principals).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (20, 'connector');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (20,1,1,'connector'),         -- connector / object / connector         (a deployable agent)
  (20,1,2,'connector_source'),  -- connector / object / connector_source  (one dataset it syncs)
  (20,1,3,'sync_run');          -- connector / object / sync_run          (one reported execution)

-- ===================================================================================================
-- connector_connectors — the registered fleet. One row per agent (hermenea, a future HR scraper, …).
-- `principal_id` is the M51 service principal the agent authenticates as: reports and wiring reads are
-- attributable to it, and disabling the principal stops both at once (no separate kill switch here).
-- ===================================================================================================
CREATE TABLE oikumenea.connector_connectors (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,1),  -- connector / object / connector
  code         text NOT NULL,
  name         text NOT NULL,
  description  text,
  -- The machine identity this agent presents (M51 / D-ServiceIdentities). RESTRICT: a principal that
  -- names a registered connector must be disabled, not deleted, so its audit trail keeps resolving.
  principal_id uuid REFERENCES oikumenea.account_service_principals(id) ON DELETE RESTRICT,
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  -- Liveness hint: when the core last heard from this agent (registration or a run report). A stale
  -- value means "not heard from", NOT "unhealthy" — the core does not probe connectors.
  last_seen_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT connector_connectors_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
-- `code` unique among LIVE rows only (soft-delete then re-register must work) — the partial-unique
-- idiom used across the schema.
CREATE UNIQUE INDEX connector_connectors_code_active_idx
  ON oikumenea.connector_connectors (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX connector_connectors_principal_active_idx
  ON oikumenea.connector_connectors (principal_id)
  WHERE deleted_at IS NULL AND principal_id IS NOT NULL;
CREATE TRIGGER connector_connectors_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_connectors
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_connectors.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.description IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.principal_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.last_seen_at IS 'pii:none';

-- ===================================================================================================
-- connector_sources — the datasets each connector syncs, as REPORTED by the connector. This is a read
-- model: hermenea's own `hermenea.import_sources` (its DB) stays authoritative for execution, because
-- scheduling lives in the connector (D-ConnectorPlane rejects core-side orchestration).
-- ===================================================================================================
CREATE TABLE oikumenea.connector_sources (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,2),  -- connector / object / connector_source
  connector_id uuid NOT NULL REFERENCES oikumenea.connector_connectors(id) ON DELETE CASCADE,
  code         text NOT NULL,
  name         text NOT NULL,
  -- The core import target for push-mode sources (e.g. geo-places, language-scheme). Deliberately NOT
  -- an FK or CHECK against the handler registry: object types are code-defined in internal/dataimport,
  -- and a connector may report a source whose handler this core build does not have.
  object_type  text,
  -- The connector's own schedule string, verbatim, for display only. The core never parses or acts on
  -- it — scheduling stays inside the connector.
  schedule     text,
  enabled      boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT connector_sources_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX connector_sources_code_active_idx
  ON oikumenea.connector_sources (connector_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER connector_sources_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_sources
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_sources.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.connector_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.object_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.schedule IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.enabled IS 'pii:none';

-- ===================================================================================================
-- connector_sync_runs — one reported execution of a source. The connector opens a run (state=running)
-- and closes it (succeeded|failed) with counts. The core stores what it is told: these counts are the
-- CONNECTOR's account of its work, not something the core recomputes.
--
-- `external_run_id` is the connector's own run identifier (hermenea's import_runs.id, and the same
-- value the M49 chunked envelopes carry as `runId`), so an operator can correlate a core-side run row
-- with the connector's own ledger and with import provenance. Unique per connector so a re-report is
-- an update, not a duplicate — reporting must be idempotent under connector retries.
-- ===================================================================================================
CREATE TABLE oikumenea.connector_sync_runs (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,3),  -- connector / object / sync_run
  source_id       uuid NOT NULL REFERENCES oikumenea.connector_sources(id) ON DELETE CASCADE,
  external_run_id text,
  state           text NOT NULL CHECK (state IN ('running','succeeded','failed')),
  created_count   bigint NOT NULL DEFAULT 0,
  updated_count   bigint NOT NULL DEFAULT 0,
  skipped_count   bigint NOT NULL DEFAULT 0,
  -- Failure detail as reported. Free text: it is a connector's message, shown to operators verbatim.
  error           text,
  started_at      timestamptz NOT NULL DEFAULT now(),
  finished_at     timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT connector_sync_runs_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3),
  -- A finished run must say when it finished; a running one must not claim to have.
  CONSTRAINT connector_sync_runs_finished_shape
    CHECK ((state = 'running' AND finished_at IS NULL) OR (state <> 'running' AND finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX connector_sync_runs_external_idx
  ON oikumenea.connector_sync_runs (source_id, external_run_id) WHERE external_run_id IS NOT NULL;
-- The fleet view's query: most recent runs for a source.
CREATE INDEX connector_sync_runs_source_started_idx
  ON oikumenea.connector_sync_runs (source_id, started_at DESC);
CREATE TRIGGER connector_sync_runs_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_sync_runs
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_sync_runs.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.source_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.external_run_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.state IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.created_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.updated_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.skipped_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.error IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.started_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.finished_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0041_datapacks =====
-- 0041 data packs (M54 — D-DataPacks). The D-Pinax boot autoseeder generalizes from purely
-- `go:embed`-bundled presets to ALSO scanning an operator-mounted packs directory (`pinax.packs`),
-- beside the embedded set — same create-if-absent / fill-if-empty / never-delete pipeline, same
-- version gate. This migration records WHERE a seeded preset came from, so an operator can see which
-- pack (if any) supplied a given catalog slice.
--
-- Additive / expand-only (L-UpgradeSafe): one nullable column on the existing marker table. Embedded
-- presets keep `pack` NULL; a mounted pack's presets record the pack's directory name. No data moves,
-- no default backfill needed (NULL == embedded, which is what every existing row already is).
ALTER TABLE oikumenea.pinax_seed_state
  ADD COLUMN pack text;  -- NULL = embedded bundle; else the operator-mounted pack's name (D-DataPacks)

COMMENT ON COLUMN oikumenea.pinax_seed_state.pack IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0042_rls_service_arm =====
-- Migration 0042_rls_service_arm: the MACHINE reach arm of the RLS backstop (M55 — the deferred half
-- of D-ServiceIdentities / D-ConnectorPlane, split out of M53).
--
-- M51 gave a service principal flat (permission_code, org_id) grants but NO reach: the reach predicate
-- oikumenea.authz_unit_in_reach(unit, wr) computes reach from app.person_id (D-RLSLiveReach), and a
-- machine request sets no person id, so every person-shaped, RLS-guarded surface denies it. This
-- migration adds a PRINCIPAL arm keyed on a new app.principal_id GUC: an org-confined grant authorizes
-- reads/writes on THAT organization's RLS-guarded rows (scraped units, memberships, positions, orders),
-- and only that org's — a cross-org read/write is refused by the DB, not merely the PEP.
--
-- THE RECURSION PROBLEM (why this is its own milestone). The predicate may read ONLY RLS-exempt tables
-- (authz_*, tenant_graphs, tenant_unit_closure) — reading a policy-guarded table from inside a policy
-- recurses into that same policy. So the arm cannot join tenant_units to learn a unit's org, even
-- though tenant_units.org_id exists. Worse, the connector plane must be able to CREATE a brand-new
-- unit, and a just-inserted / edgeless unit has no tenant_unit_closure row yet (reflexive rows are
-- seeded only when an edge is added — internal/tenant ExtendClosureForEdge), so a closure-derived arm
-- could never satisfy the tenant_units INSERT WITH CHECK for a fresh unit.
--
-- RESOLUTION: a dedicated RLS-exempt PROJECTION authz_unit_org(unit_id -> org_id), trigger-maintained
-- from tenant_units. A BEFORE INSERT trigger populates the projection row before the tenant_units
-- WITH CHECK subquery evaluates, so even the unit being inserted in the current statement resolves its
-- org recursion-free. The principal arm is then an O(1) authz_principal_grants x authz_unit_org probe.
--
-- Expand-only (CREATE TABLE / TRIGGER / CREATE OR REPLACE FUNCTION only; no drops/narrowings). The
-- CREATE OR REPLACE keeps the existing GRANT EXECUTE on the function. Depends on 0011 (the backstop)
-- and 0039 (authz_principal_grants). Additive to a LIVE deployment: the person + admin arms are
-- unchanged, so no person hot path regresses; the new arm is inert until app.principal_id is set.

-- ---------------------------------------------------------------------------------------------------
-- authz_unit_org: the RLS-EXEMPT unit -> org projection the machine reach arm reads. A derived
-- structural mirror of tenant_units.org_id (like tenant_unit_closure it is a maintained projection, so
-- it carries NO RID). EXEMPT from RLS on purpose — the reach predicate READS it to COMPUTE reach; a
-- reach-keyed policy here would be circular (the exact recursion the projection exists to avoid). Holds
-- only (unit_id, org_id) — org membership of a unit, pii:none.
-- The unit_id FK is DEFERRABLE INITIALLY DEFERRED: the BEFORE INSERT trigger below populates the
-- projection row from inside the tenant_units INSERT — i.e. BEFORE the parent tenant_units row itself
-- lands — so an immediately-checked FK would fail. Deferring validation to COMMIT (by which point the
-- parent exists) keeps referential integrity + ON DELETE CASCADE while letting the projection row be
-- visible to the same-statement WITH CHECK that a principal-created unit depends on.
CREATE TABLE oikumenea.authz_unit_org (
  unit_id uuid PRIMARY KEY
            REFERENCES oikumenea.tenant_units(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
  org_id  uuid NOT NULL
            REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT
);
CREATE INDEX authz_unit_org_org_idx ON oikumenea.authz_unit_org (org_id);

COMMENT ON TABLE  oikumenea.authz_unit_org         IS 'RLS-exempt unit->org projection for the machine reach arm (M55); trigger-maintained from tenant_units. No RID (derived projection).';
COMMENT ON COLUMN oikumenea.authz_unit_org.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_unit_org.org_id  IS 'pii:none';

-- The app role (oikumenea_app, created NOBYPASSRLS in 0011) writes this projection via the trigger
-- below, which runs as the invoking role. 0011's ALTER DEFAULT PRIVILEGES already covers future
-- tables, but grant explicitly so the DML is correct regardless of who owns the migration.
GRANT SELECT, INSERT, UPDATE, DELETE ON oikumenea.authz_unit_org TO oikumenea_app;

-- Backfill existing units (empty on a fresh install). The migration runs as the owner/superuser, which
-- bypasses RLS, so this read of tenant_units sees every row. Soft-deleted units are included — a
-- projection row is harmless without an active grant, and keeping it avoids a resurrect-on-undelete gap.
INSERT INTO oikumenea.authz_unit_org (unit_id, org_id)
  SELECT id, org_id FROM oikumenea.tenant_units
  ON CONFLICT (unit_id) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- sync_unit_org: keep the projection in lockstep with tenant_units. BEFORE INSERT is load-bearing —
-- the projection row must exist before the tenant_units WITH CHECK subquery (which reads this table)
-- evaluates, so a principal CREATING a unit passes its own INSERT policy. UPDATE OF org_id is handled
-- defensively (a unit's org is immutable by convention, but a correction must not desync the mirror).
CREATE FUNCTION oikumenea.sync_unit_org() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO oikumenea.authz_unit_org (unit_id, org_id)
  VALUES (NEW.id, NEW.org_id)
  ON CONFLICT (unit_id) DO UPDATE SET org_id = EXCLUDED.org_id;
  RETURN NEW;
END$$;

CREATE TRIGGER tenant_units_sync_org
  BEFORE INSERT OR UPDATE OF org_id ON oikumenea.tenant_units
  FOR EACH ROW EXECUTE FUNCTION oikumenea.sync_unit_org();

-- ---------------------------------------------------------------------------------------------------
-- authz_principal_org_in_reach: THE machine reach primitive — does the app.principal_id principal hold
-- an org-CONFINED grant of the requested read/write class over organization `org`? Reads ONLY the
-- RLS-exempt authz_principal_grants:
--   * org_id IS NOT NULL — an INSTANCE-WIDE grant (org_id NULL) confers NO operational reach; it is for
--     reference catalogs (pdp_scoped=false units, already exempt via NOT pdp_scoped) and instance-scope
--     wiring reads. The organization is a scraper's blast-radius boundary (D-ServiceIdentities).
--   * revoked_at IS NULL — grants are read LIVE, so revocation is immediate (matches the M51 no-cache
--     principal-authority design and the person arm's exactness under revocation).
--   * (permission_code LIKE '%.read') = NOT wr — the exact mirror of the person arm's read/write split.
-- For a PERSON request app.principal_id is unset -> nullif('')::uuid IS NULL -> an empty probe, so the
-- person hot path (M47) does not regress. STABLE + single SELECT ⇒ the planner inlines it per policy.
CREATE FUNCTION oikumenea.authz_principal_org_in_reach(org uuid, wr boolean) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
    FROM oikumenea.authz_principal_grants pg
    WHERE pg.principal_id = nullif(current_setting('app.principal_id', true), '')::uuid
      AND pg.org_id = org
      AND pg.org_id IS NOT NULL
      AND pg.revoked_at IS NULL
      AND (pg.permission_code LIKE '%.read') = NOT wr
  )
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_principal_org_in_reach(uuid, boolean) TO oikumenea_app;

-- authz_unit_in_reach: add the PRINCIPAL arm for the UNIT-KEYED tables (membership_*, order_orders,
-- tenant_unit_*, audit_log — everything whose policy passes a unit id, not an org). The person + admin
-- arms are copied verbatim from 0011 (CREATE OR REPLACE cannot patch a subexpression) — DO NOT alter
-- their semantics. The new arm resolves the unit's org through the RLS-exempt authz_unit_org projection
-- and defers the grant test to authz_principal_org_in_reach. Every unit these child tables reference is
-- already committed, so its projection row is visible. (tenant_units' OWN insert is the one case the
-- projection can't serve — a BEFORE-trigger write is not visible to the same statement's WITH CHECK —
-- so the tenant_units policy below checks the row's org_id column directly instead.)
CREATE OR REPLACE FUNCTION oikumenea.authz_unit_in_reach(unit uuid, wr boolean) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
      OR EXISTS (
        SELECT 1
        FROM oikumenea.authz_role_assignments a
        JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
        WHERE a.subject_person_id = nullif(current_setting('app.person_id', true), '')::uuid
          AND a.revoked_at IS NULL
          AND (a.expires_at IS NULL OR a.expires_at > now())
          AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                      WHERE rp.role_id = a.role_id
                        AND (rp.permission_code LIKE '%.read') = NOT wr)
          AND ((a.scope = 'unit' AND a.target_unit_id = unit)
            OR (a.scope = 'subtree'
                AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                            WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
                AND (a.target_unit_id = unit
                  OR EXISTS (SELECT 1 FROM oikumenea.tenant_unit_closure c
                             WHERE c.graph_id = a.graph_id
                               AND c.ancestor_id = a.target_unit_id
                               AND c.descendant_id = unit))))
      )
      OR EXISTS (
        SELECT 1 FROM oikumenea.authz_unit_org uo
        WHERE uo.unit_id = unit
          AND oikumenea.authz_principal_org_in_reach(uo.org_id, wr)
      )
$$;

-- tenant_units: extend the reach policy with the org-DIRECT principal arm. A principal's reach on a unit
-- row is its org grant checked against the row's own org_id — this is what lets an org-confined connector
-- CREATE a brand-new unit (the projection row is not yet visible mid-INSERT) and confines every write to
-- its organization. Replaces the 0011 policy with a strict superset (an added OR arm). REFERENCE units
-- (pdp_scoped=false) stay instance-global as before.
DROP POLICY tenant_units_reach ON oikumenea.tenant_units;
CREATE POLICY tenant_units_reach ON oikumenea.tenant_units
  USING (NOT pdp_scoped
      OR oikumenea.authz_unit_in_reach(id, false)
      OR oikumenea.authz_principal_org_in_reach(org_id, false))
  WITH CHECK (NOT pdp_scoped
      OR oikumenea.authz_unit_in_reach(id, true)
      OR oikumenea.authz_principal_org_in_reach(org_id, true));

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0043_login_security_log =====
-- Migration 0043_login_security_log: the first-party login/IP security log (M37 / D-LoginSecurityLog).
--
-- A security telemetry stream on the identity-federation seam: the OIDC/JWKS validation middleware
-- already sees a validated request's account, client IP and user-agent, so it records a bounded,
-- de-duplicated login/IP history without the service becoming an auth provider (L-AuthzOnly holds — no
-- tokens issued, no credentials stored). NOT OSINT enrichment. pii:contact, retention-bounded (a plain
-- DELETE sweep — the stream is deduped, so no partitioning), and purge-erased with the person.
--
-- Volume: NOT a per-request firehose. The middleware de-dupes to ONE row per (account_id, context, ip)
-- per configurable window (default ~1h app-side) — a bump of last_seen_at + occurrence_count within the
-- window, else a new row. So the table stays a genuine security log, not a request log.
--
-- No RLS: like account_accounts (its parent), a login event has no unit_id and is not reach-scoped;
-- reads are gated by the app-layer PDP (account.security-log.read, instance-scope). Expand-only.
-- Depends on 0008 (account_accounts) and 0011 (the app-role default privileges cover this new table).

-- account_login_events — an account Object (9,1,4 = login_event; the next account object after
-- service_principal 9,1,3). Append-with-dedup; no soft-delete (a security log — erasure on purge is a
-- hard delete). resolved_* / is_vpn / is_tor are the IP-intelligence seam: NULL until a resolver ships
-- (deferred — a future hermenea connector), so the MVP records raw ip + user_agent.
CREATE TABLE oikumenea.account_login_events (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(9,1,4),  -- account / object / login_event
  account_id       uuid NOT NULL REFERENCES oikumenea.account_accounts(id) ON DELETE CASCADE,
  context          text NOT NULL CHECK (context IN ('login','activity','registration')),
  ip               inet NOT NULL,
  first_seen_at    timestamptz NOT NULL DEFAULT now(),
  last_seen_at     timestamptz NOT NULL DEFAULT now(),
  occurrence_count integer NOT NULL DEFAULT 1 CHECK (occurrence_count > 0),
  -- IP-intelligence seam (nullable; resolver deferred):
  resolved_country text,     -- ISO 3166-1 alpha-2 when resolved
  resolved_isp     text,
  is_vpn           boolean,
  is_tor           boolean,
  user_agent       text,

  CONSTRAINT account_login_events_rid_shape
    CHECK (oikumenea.rid_service(id)=9 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);

-- Dedup probe (find the recent (account, context, ip) row to bump) + the read keyset (account history,
-- newest first).
CREATE INDEX account_login_events_dedup_idx
  ON oikumenea.account_login_events (account_id, context, ip, last_seen_at DESC);
-- Retention sweep + the erase-by-account fan-out.
CREATE INDEX account_login_events_sweep_idx ON oikumenea.account_login_events (last_seen_at);

COMMENT ON COLUMN oikumenea.account_login_events.id               IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.account_id       IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.context          IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.ip               IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.first_seen_at    IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.last_seen_at     IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.occurrence_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.resolved_country IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.resolved_isp     IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_login_events.is_vpn           IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.is_tor           IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_login_events.user_agent       IS 'pii:contact';

-- delete_login_events_before: the retention sweep helper (D-LoginSecurityLog; mirrors the M50
-- detach_audit_partitions_before operator seam). Returns the number of rows deleted. Retention is an
-- operator policy (login-security.retention-days; 0 = retain forever) enforced by the app calling this.
CREATE FUNCTION oikumenea.delete_login_events_before(cutoff timestamptz) RETURNS bigint
LANGUAGE sql AS $$
  WITH del AS (
    DELETE FROM oikumenea.account_login_events WHERE last_seen_at < cutoff RETURNING 1
  )
  SELECT count(*) FROM del;
$$;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0044_login_event_rid_seed =====
-- Migration 0044_login_event_rid_seed: seed the `login_event` RID type into the DB registry (M37 fix).
--
-- M37's 0043 added `account_login_events` (account Object 9,1,4) and the Go registry entry
-- (pkg/rid/registry.go) + the ontology-mapping.md row, but OMITTED the third registry: the
-- oikumenea.platform_rid_types seed. The three must agree or the boot-time `rid.AssertMatches` drift
-- check fails (db=150 vs go=151). This forward-only fix adds the missing seed row (0043 is an
-- immutable, already-applied historical artifact, so the row lands here rather than by editing it).
-- Idempotent; expand-only.
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (9, 1, 4, 'login_event')
ON CONFLICT DO NOTHING;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0011_infra', applied_at = now() WHERE singleton;
