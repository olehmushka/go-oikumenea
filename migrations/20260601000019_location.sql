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
-- (M16) for the WOF gazetteer; this migration adds the h3-pg extensions, an MGRS derivation function,
-- and the location point model. The operator DB must carry the custom Postgres image (Dockerfile.postgres
-- builds postgis + h3 + h3_postgis); the boot-time readiness gate checks the extensions are present.

-- ---------------------------------------------------------------------------------------------------
-- Extensions (D-Location). PostGIS is already enabled by the bootstrap (0000); h3 + h3_postgis are the
-- new operator-DB prerequisites the custom image provisions. Installed into the search_path's public
-- schema like postgis, so their functions (h3_lat_lng_to_cell, ST_*) resolve unqualified.
-- ---------------------------------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS postgis_raster;  -- h3_postgis depends on it (PostGIS 3.x split raster out)
CREATE EXTENSION IF NOT EXISTS h3;
CREATE EXTENSION IF NOT EXISTS h3_postgis;

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
-- location_mgrs(geography) — DB-derived MGRS string from a point (D-Location). PostGIS does the
-- projection math (ST_Transform into the point's UTM zone for easting/northing); plpgsql does the
-- MGRS lettering (zone + latitude band + 100km grid square + 5-digit easting/northing = 1m precision).
-- Polar regions (lat outside [-80,84]) use UPS, which is out of scope → NULL (documented seam).
-- The standard MGRS/UTM algorithm; validated against fixtures in the geo integration tests.
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION oikumenea.location_mgrs(g geography)
RETURNS text
LANGUAGE plpgsql
IMMUTABLE
AS $$
DECLARE
  lon          double precision;
  lat          double precision;
  zone         int;
  epsg         int;
  utm          geometry;
  easting      double precision;
  northing     double precision;
  band_letters constant text := 'CDEFGHJKLMNPQRSTUVWX';  -- 20 latitude bands, 8 deg each, from 80S
  band_idx     int;
  band         text;
  col_letters  text;
  col_idx      int;
  row_letters  constant text := 'ABCDEFGHJKLMNPQRSTUV';  -- 20 row letters (skip I, O)
  row_offset   int;
  row_idx      int;
BEGIN
  IF g IS NULL THEN
    RETURN NULL;
  END IF;
  lon := ST_X(g::geometry);
  lat := ST_Y(g::geometry);
  IF lat < -80 OR lat > 84 THEN
    RETURN NULL;  -- UPS polar region, out of scope
  END IF;

  zone := floor((lon + 180) / 6)::int + 1;
  IF zone > 60 THEN zone := 60; END IF;  -- lon = 180.0 edge folds into zone 60

  -- UTM EPSG for the zone: 326xx north, 327xx south (the 327xx codes apply the 10,000,000m false
  -- northing, so southern points come back with a positive northing as MGRS expects).
  IF lat >= 0 THEN epsg := 32600 + zone; ELSE epsg := 32700 + zone; END IF;
  utm      := ST_Transform(g::geometry, epsg);
  easting  := ST_X(utm);
  northing := ST_Y(utm);

  -- latitude band letter
  band_idx := floor((lat + 80) / 8)::int;
  IF band_idx > 19 THEN band_idx := 19; END IF;  -- band X spans 72..84 (12 deg)
  band := substr(band_letters, band_idx + 1, 1);

  -- 100km square column letter: one of three 8-letter sets, chosen by zone mod 3, indexed by easting.
  CASE zone % 3
    WHEN 1 THEN col_letters := 'ABCDEFGH';
    WHEN 2 THEN col_letters := 'JKLMNPQR';
    ELSE        col_letters := 'STUVWXYZ';
  END CASE;
  col_idx := floor(easting / 100000)::int - 1;  -- easting in [100000,900000) -> [0,7]

  -- 100km square row letter: 20-letter cycle, offset by 5 for even-numbered zones.
  IF zone % 2 = 0 THEN row_offset := 5; ELSE row_offset := 0; END IF;
  row_idx := (floor(northing / 100000)::int + row_offset) % 20;

  RETURN zone::text || band
       || substr(col_letters, col_idx + 1, 1)
       || substr(row_letters, row_idx + 1, 1)
       || lpad(((floor(easting)::bigint % 100000))::text, 5, '0')
       || lpad(((floor(northing)::bigint % 100000))::text, 5, '0');
END;
$$;

-- location_derive_indexes() — recompute the derived spatial columns from geom on every insert/update,
-- so mgrs + the H3 cells are authoritative-from-geometry and can never be hand-set or drift (the
-- D-Location invariant). H3 cells come from h3_postgis (h3_lat_lng_to_cell) at a fixed resolution set
-- (~9km / 1.2km / 150m / 20m), stored in full regardless of any owner's publish precision.
CREATE OR REPLACE FUNCTION oikumenea.location_derive_indexes()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.mgrs      := oikumenea.location_mgrs(NEW.geom);
  NEW.h3_res_5  := h3_lat_lng_to_cell(NEW.geom::geometry, 5)::text;
  NEW.h3_res_7  := h3_lat_lng_to_cell(NEW.geom::geometry, 7)::text;
  NEW.h3_res_9  := h3_lat_lng_to_cell(NEW.geom::geometry, 9)::text;
  NEW.h3_res_11 := h3_lat_lng_to_cell(NEW.geom::geometry, 11)::text;
  RETURN NEW;
END;
$$;

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
-- GEOGRAPHY(POINT,4326); mgrs + the H3 cells are DB-derived (the trigger below) and never written by
-- the app. The structured address sits over the RID-keyed geo_countries registry (country_id RESTRICT).
-- pii:none at rest — a coordinate becomes locator data only when an owner links a person to it, and
-- that tier lives on the owning link, not here.
-- ---------------------------------------------------------------------------------------------------
CREATE TABLE oikumenea.location_locations (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(12,1,3),  -- location / object / location
  geom          geography(Point, 4326) NOT NULL,    -- the authoritative coordinate (PostGIS); address-only is out of scope
  mgrs          text,                               -- DB-derived from geom (trigger); never hand-set
  h3_res_5      text,                               -- DB-derived H3 cell, ~9km
  h3_res_7      text,                               -- ~1.2km
  h3_res_9      text,                               -- ~150m
  h3_res_11     text,                               -- ~20m
  country_id    uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  admin_area_1  text,                               -- state/oblast
  admin_area_2  text,                               -- county/raion
  locality      text,
  street        text,
  house_number  text,
  postal_code   text,
  raw_address   text,                               -- the unparsed address as supplied
  type_id       uuid REFERENCES oikumenea.location_location_types(id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT location_locations_rid_shape
    CHECK (oikumenea.rid_service(id)=12 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE TRIGGER location_locations_derive_indexes
  BEFORE INSERT OR UPDATE OF geom ON oikumenea.location_locations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.location_derive_indexes();
CREATE TRIGGER location_locations_set_updated_at
  BEFORE UPDATE ON oikumenea.location_locations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE INDEX location_locations_geom_gist ON oikumenea.location_locations USING gist (geom);
CREATE INDEX location_locations_h3_res_5  ON oikumenea.location_locations (h3_res_5);
CREATE INDEX location_locations_h3_res_7  ON oikumenea.location_locations (h3_res_7);
CREATE INDEX location_locations_h3_res_9  ON oikumenea.location_locations (h3_res_9);
CREATE INDEX location_locations_h3_res_11 ON oikumenea.location_locations (h3_res_11);
CREATE INDEX location_locations_country   ON oikumenea.location_locations (country_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.location_locations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.geom IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.mgrs IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.raw_address IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0019_location', applied_at = now() WHERE singleton;
