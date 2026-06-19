-- 0019 location (M19).
--
-- A shared, standalone place entity (docs/modules/location.md / D-Location). One row is a precise
-- point on Earth (a required GEOGRAPHY(POINT,4326) coordinate) plus a structured postal address over
-- the seeded geo_countries registry, with DB-derived MGRS + H3 spatial indexes. Anything with a
-- location (M20 education buildings/dorms, M21 company addresses, religion sites) references
-- location_locations(id) by FK and owns the meaning (visibility/precision/purpose) on its own link —
-- a location itself carries no owner. Lives on the existing `location` RID service (12) beside the
-- geo_countries / geo_places registry (M16). Expand-only (L-UpgradeSafe / D-Migrations); depends on
-- the 0000 schema bootstrap (new_id + geo_countries + PostGIS).
--
-- D-Location reverses the drafts/ geography drop: PostGIS was already pulled forward by D-GeoPlaces
-- (M16) for the WOF gazetteer; this migration adds the location point model on top of it. PostGIS is
-- the only operator-DB prerequisite (stock postgis image) — the radius/bbox queries use ST_DWithin on
-- the GiST-indexed geom. MGRS is derived in the application (pure Go), not the DB, and the canonical
-- coordinate can be supplied in several formats (lat/lon, MGRS, UTM, СК-42); the app converts each to
-- WGS84, derives MGRS, and records the original input in the source_coordinate JSONB column.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): two new object types on the existing `location` service (12),
-- plus its Action RID type — service 12 was read-only (geo) until now, so it had no kind=3 row.
-- pkg/rid mirrors the object types and asserts equality at boot (kind<>3 is excluded from the check).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (12,1,3,'location'),
  (12,1,4,'location_type'),
  (12,3,0,'action');

-- ---------------------------------------------------------------------------------------------------
-- location_location_types — a small instance-admin catalog of place purposes (building/address/online),
-- optional on a location; descriptive only, never branched on. RID-keyed (location service 12), code +
-- translatable name (i18n entity_type 'location_type'), soft-delete.
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE oikumenea.location_location_types (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(12,1,4),  -- location / object / location_type
  code        text NOT NULL,
  name        text NOT NULL,                  -- default-locale display name; translatable via the i18n store
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT location_location_types_rid_shape
    CHECK (oikumenea.rid_service(id)=12 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);
CREATE UNIQUE INDEX location_location_types_code_active
  ON oikumenea.location_location_types (code) WHERE deleted_at IS NULL;
CREATE TRIGGER location_location_types_set_updated_at
  BEFORE UPDATE ON oikumenea.location_location_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.location_location_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_location_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_location_types.name IS 'pii:none';

-- Seed a few generic place purposes (new_id reads no GUC, so seeding directly here is fine).
INSERT INTO oikumenea.location_location_types (code, name) VALUES
  ('building', 'Building'),
  ('address',  'Address'),
  ('online',   'Online');

-- ---------------------------------------------------------------------------------------------------
-- location_locations — the shared place object (D-Location). The required spine is `geom`
-- GEOGRAPHY(POINT,4326), built in SQL from the canonical WGS84 lat/lon the application resolves. `mgrs`
-- is derived in the application (pure Go) and written here on every coordinate change; `source_coordinate`
-- holds the original input exactly as supplied (its format + raw values), so a coordinate entered as
-- MGRS/UTM/СК-42 is preserved alongside the canonical point. The structured address sits over the
-- RID-keyed geo_countries registry (country_id RESTRICT). pii:none at rest — a coordinate becomes
-- locator data only when an owner links a person to it, and that tier lives on the owning link.
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE oikumenea.location_locations (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(12,1,3),  -- location / object / location
  geom              geography(Point, 4326) NOT NULL,    -- the authoritative coordinate (PostGIS); built from the resolved WGS84 lat/lon
  mgrs              text,                               -- app-derived (pure Go) from the coordinate; NULL for polar UPS points
  source_coordinate jsonb NOT NULL DEFAULT '{}',        -- the original input as supplied (format + raw values)
  country_id        uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  admin_area_1      text,                               -- state/oblast
  admin_area_2      text,                               -- county/raion
  locality          text,
  street            text,
  house_number      text,
  postal_code       text,
  raw_address       text,                               -- the unparsed address as supplied
  type_id           uuid REFERENCES oikumenea.location_location_types(id) ON DELETE RESTRICT,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT location_locations_rid_shape
    CHECK (oikumenea.rid_service(id)=12 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE TRIGGER location_locations_set_updated_at
  BEFORE UPDATE ON oikumenea.location_locations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE INDEX location_locations_geom_gist ON oikumenea.location_locations USING gist (geom);
CREATE INDEX location_locations_country   ON oikumenea.location_locations (country_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.location_locations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.geom IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.mgrs IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.source_coordinate IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.raw_address IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0019_location', applied_at = now() WHERE singleton;
