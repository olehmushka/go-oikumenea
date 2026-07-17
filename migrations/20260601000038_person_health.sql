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
UPDATE oikumenea.schema_version SET revision = '0038_person_health', applied_at = now() WHERE singleton;
