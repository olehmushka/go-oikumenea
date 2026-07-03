-- 0031 person addresses (M32 — D-PersonAddresses). A precise, effective-dated, geocoded address history
-- for a person, layered over the shared M19 Location entity (location_locations, RID service 12). This is
-- distinct from person_residences (0005), which stays for country-grade legal/citizenship residence; an
-- address is the PostGIS-backed overlay that dedups against shared location rows and enables spatial
-- queries ("everyone near point X").
--
--   person_addresses — a reified Link link__lives_at (Person → a location_locations row): role
--   (home|work|mailing|other), effective dates, is_primary (one active primary per person), a
--   privacy_seeking signal (a mailing address that differs from home is itself a datum), and the
--   D-OverlayFoundation attribution column-set (source/confidence). pii:contact → hard-deleted on purge.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons), and 0019 location (location_locations).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,2,10,'lives_at');  -- person / link / lives_at (M32)

-- ===================================================================================================
-- person_addresses — the reified pii:contact Link link__lives_at. The address content lives on the
-- shared location_locations row (ON DELETE RESTRICT — a location cannot vanish under a live address);
-- this table carries the person↔location relationship's own identity, role, validity, and attribution.
-- ===================================================================================================
CREATE TABLE oikumenea.person_addresses (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,10),  -- person / link / lives_at
  person_id       uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  location_id     uuid NOT NULL REFERENCES oikumenea.location_locations(id) ON DELETE RESTRICT,  -- M19
  role            text NOT NULL CHECK (role IN ('home','work','mailing','other')),
  valid_from      date NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
  valid_to        date,          -- NULL = current
  is_primary      boolean NOT NULL DEFAULT false,
  privacy_seeking boolean NOT NULL DEFAULT false,  -- a mailing address ≠ home is itself a signal
  source          text NOT NULL DEFAULT 'operator_verified'
                    CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence      text NOT NULL DEFAULT 'possible'
                    CHECK (confidence IN ('confirmed','probable','possible')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  CONSTRAINT person_addresses_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=10)
);

CREATE TRIGGER person_addresses_set_updated_at
  BEFORE UPDATE ON oikumenea.person_addresses
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_addresses_person_idx
  ON oikumenea.person_addresses (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_addresses_location_idx
  ON oikumenea.person_addresses (location_id) WHERE deleted_at IS NULL;
-- At most one active primary address per person (the application demotes the prior primary in-tx).
CREATE UNIQUE INDEX person_addresses_one_primary
  ON oikumenea.person_addresses (person_id) WHERE is_primary AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_addresses.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.location_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_addresses.role IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.valid_from IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_addresses.valid_to IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_addresses.is_primary IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.privacy_seeking IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_addresses.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0031_person_addresses', applied_at = now() WHERE singleton;
