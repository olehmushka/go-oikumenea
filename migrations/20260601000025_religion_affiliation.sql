-- 0025 religion affiliation (M24 — D-ReligiousAffiliation / D-SpecialPII): lay religious affiliation.
--
-- A person's LAY religious affiliation/belief (docs/modules/religion.md / D-ReligiousAffiliation). Unlike
-- a clergy credential (a public organizational fact, 0024), lay belief is GDPR Art. 9 `pii:special` and
-- carries a stricter regime: the affiliation's free-form value is ENVELOPE-ENCRYPTED at rest with a blind
-- index (D-SpecialPII extends the D-CryptoProvider sensitive-tier envelope UNCHANGED to pii:special), and
-- it is CRYPTO-ERASED on person purge (drop the wrapped DEK + ciphertext, keep the row as a tombstone).
--
-- The structural anchors (which faith / tradition / community / affiliation type) are plaintext FKs; the
-- optional free-form belief detail is the encrypted value. The same value_ciphertext / wrapped_dek /
-- key_ref / value_blind_index envelope shape as document_personal_codes (0009) is reused verbatim.
--
-- Binding design rule (D-Religion): affiliation types are CATALOG ROWS keyed to a religion_taxa node
-- (per tradition) or generic, never a CHECK enum. The only CHECK enum here is the lifecycle status.
--
-- Person-scoped (instance-global), like person_languages / person_partnerships → NO unit RLS.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id), 0003 tenant (tenant_units),
-- 0005 person (person_persons) and 0023 religion (religion_taxa).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (16,1,10,'affiliation_type'),
  (16,2,3,'affiliated_with');

-- ===================================================================================================
-- religion_affiliation_types — the per-tradition affiliation/milestone catalog (D-Code / D-i18n).
-- ===================================================================================================
CREATE TABLE oikumenea.religion_affiliation_types (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,10),  -- religion / object / affiliation_type
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  code               text NOT NULL,
  name               text NOT NULL,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order         integer,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT religion_affiliation_types_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=10)
);
CREATE UNIQUE INDEX religion_affiliation_types_code_active
  ON oikumenea.religion_affiliation_types (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_affiliation_types_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_affiliation_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_affiliation_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliation_types.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliation_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliation_types.name IS 'pii:none';

-- Generic + per-tradition seed.
INSERT INTO oikumenea.religion_affiliation_types (tradition_taxon_id, code, name, sort_order) VALUES
  (NULL,'adherent','Adherent',10),
  (NULL,'member','Member',20);
INSERT INTO oikumenea.religion_affiliation_types (tradition_taxon_id, code, name, sort_order)
SELECT t.id, v.code, v.name, v.so
FROM (VALUES
  ('christianity','catechumen','Catechumen',30),
  ('christianity','baptized','Baptized',40),
  ('christianity','confirmed','Confirmed',50),
  ('islam','shahada','Shahada',30),
  ('judaism','bar_bat_mitzvah','Bar / Bat Mitzvah',30)
) AS v(tradition_code, code, name, so)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL;

-- ===================================================================================================
-- religion_affiliations — the reified `pii:special` Link link__affiliated_with (Person → a religion /
-- tradition unit / community unit + an affiliation type). The optional belief detail is envelope-
-- encrypted; structural anchors are plaintext FKs. Crypto-erased on person purge.
-- ===================================================================================================
CREATE TABLE oikumenea.religion_affiliations (
  id                  uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,2,3),  -- religion / link / affiliated_with
  person_id           uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  religion_id         uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,    -- optional faith anchor
  tradition_unit_id   uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,     -- optional tradition/body
  community_unit_id   uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,     -- optional local community
  affiliation_type_id uuid NOT NULL REFERENCES oikumenea.religion_affiliation_types(id) ON DELETE RESTRICT,
  -- envelope-encrypted optional belief detail (D-SpecialPII; same shape as document_personal_codes).
  -- All four are NULL when no free-form value is supplied, or once crypto-erased on purge.
  value_ciphertext    bytea,
  wrapped_dek         bytea,
  key_ref             text,
  value_blind_index   bytea,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('active','lapsed','renounced')),
  effective_from      timestamptz NOT NULL DEFAULT now(),
  effective_to        timestamptz,
  source              text,
  confidence          text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  CONSTRAINT religion_affiliations_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3)
);
CREATE INDEX religion_affiliations_person_idx
  ON oikumenea.religion_affiliations (person_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_affiliations_type_idx
  ON oikumenea.religion_affiliations (affiliation_type_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_affiliations_religion_idx
  ON oikumenea.religion_affiliations (religion_id) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_affiliations_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_affiliations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
-- pii markers: structural anchors are pii:none; the value envelope columns are pii:special.
COMMENT ON COLUMN oikumenea.religion_affiliations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.religion_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.tradition_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.community_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.affiliation_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_affiliations.value_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.religion_affiliations.wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.religion_affiliations.value_blind_index IS 'pii:special';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0025_religion_affiliation', applied_at = now() WHERE singleton;
