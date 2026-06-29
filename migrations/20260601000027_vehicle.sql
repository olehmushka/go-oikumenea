-- 0027 vehicle (M26).
--
-- The vehicle domain (docs/modules/vehicle.md / D-Vehicles): a generic vehicle registry binding people
-- AND companies to vehicles in one queryable graph. Scoped to STRUCTURAL registry data — a brand/model/
-- type taxonomy, the physical vehicle (VIN), the brand->manufacturer link, and the ownership+plate
-- registration record. Volatile vehicle intelligence (insurance/inspection/accidents/telematics) is
-- PARKED (DS-52), and column-izing stabilized specs out of `attributes` is parked (DS-53).
--
-- Mirrors the company templates: catalogs (types tree / brands / models / plate-number types), the
-- vehicle Object, and reified Links. The plate REGION FK targets the shared WOF geo_places gazetteer
-- (placetype='region') built in M16 — D-GeoPlaces SUPERSEDED the originally-planned geo_subdivisions
-- registry, so no new geography table is created here. Country FKs target the RID-keyed geo_countries.
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the catalog reference rows (types, number types) are
-- seeded directly here. Brands/models and the vehicle/registration/manufacturer rows are created through
-- VehicleService.
--
-- Polymorphic owners (a registration's owner is a person OR a company) are carried as
-- (owner_kind, owner_id TEXT) WITHOUT a FK — F-014 / D-RIDSeeding keep polymorphic target ids as text
-- (the RID self-describes its service/kind), exactly like company foundings/shareholdings. Person-owned
-- registrations are pii:basic and erased on person purge by the vehicle module's PersonPurged subscriber
-- (mirroring the document module — there is no person_id FK to CASCADE through).
--
-- RLS: vehicle entities are external reference data, instance-global, not scoped against tenant_units
-- (like company / education / location), so no RLS is enabled.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id +
-- geo_countries + geo_places), 0005 person (person_persons) and 0022 company (tenant_organizations).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the new `vehicle` service (17) + its object/link/action types.
-- pkg/rid mirrors these and asserts equality at boot (kind<>3), so they are added in both places together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (17, 'vehicle');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- vehicle objects
  (17,1,1,'vehicle'),(17,1,2,'vehicle_type'),(17,1,3,'vehicle_brand'),(17,1,4,'vehicle_model'),
  (17,1,5,'registration_number_type'),
  -- vehicle links
  (17,2,1,'manufactured_by'),(17,2,2,'registered_to'),
  -- vehicle Action RID (kind=3, excluded from the Go-mirror size check)
  (17,3,0,'action');

-- ===================================================================================================
-- Reference catalogs (D-Code / D-i18n): code + translatable name (default-locale here, other locales in
-- the localization store). Instance-admin-managed; seeded with a starter set, instance-extensible.
-- ===================================================================================================

-- vehicle_types — the vehicle taxonomy as a shallow TREE (car/truck/motorcycle/bus/trailer/special…):
-- parent_id self-FK + denormalized root_id, NO maintained closure (the rank_types pattern — a structural
-- containment FK, not a reified Link). A NULL parent_id is a root; root_id points at the tree root.
CREATE TABLE oikumenea.vehicle_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,1,2),  -- vehicle / object / vehicle_type
  code       text NOT NULL,
  name       text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  parent_id  uuid REFERENCES oikumenea.vehicle_types(id) ON DELETE RESTRICT,  -- NULL = root
  root_id    uuid REFERENCES oikumenea.vehicle_types(id) ON DELETE RESTRICT,  -- denormalized tree root
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT vehicle_types_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX vehicle_types_code_active
  ON oikumenea.vehicle_types (code) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_types_parent_idx
  ON oikumenea.vehicle_types (parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_types_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_types.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_types.root_id IS 'pii:none';

-- Seed the root types (flat starter set; each is its own root). root_id is backfilled to self below.
INSERT INTO oikumenea.vehicle_types (code, name, sort_order) VALUES
  ('car','Car',10),
  ('truck','Truck',20),
  ('motorcycle','Motorcycle',30),
  ('bus','Bus',40),
  ('trailer','Trailer',50),
  ('special','Special vehicle',60);
-- Roots point root_id at themselves (denormalized tree-root pointer, the rank_types pattern).
UPDATE oikumenea.vehicle_types SET root_id = id WHERE parent_id IS NULL AND root_id IS NULL;

-- vehicle_registration_number_types — the plate-type catalog (regular/temporary/transit/diplomatic/
-- military/old…).
CREATE TABLE oikumenea.vehicle_registration_number_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,1,5),  -- vehicle / object / registration_number_type
  code       text NOT NULL,
  name       text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT vehicle_registration_number_types_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=5)
);
CREATE UNIQUE INDEX vehicle_registration_number_types_code_active
  ON oikumenea.vehicle_registration_number_types (code) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_registration_number_types_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_registration_number_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_registration_number_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registration_number_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registration_number_types.name IS 'pii:none';

INSERT INTO oikumenea.vehicle_registration_number_types (code, name, sort_order) VALUES
  ('regular','Regular',10),
  ('temporary','Temporary',20),
  ('transit','Transit',30),
  ('diplomatic','Diplomatic',40),
  ('military','Military',50),
  ('old','Old / historic',60);

-- vehicle_brands — the marque (Toyota/BMW…). country_id is the country of origin (nullable). Created
-- through VehicleService (not migration-seeded — brand registries ride the M16 hermenea connectors).
CREATE TABLE oikumenea.vehicle_brands (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,1,3),  -- vehicle / object / vehicle_brand
  code       text NOT NULL,
  name       text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  country_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (origin)
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT vehicle_brands_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE UNIQUE INDEX vehicle_brands_code_active
  ON oikumenea.vehicle_brands (code) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_brands_country_idx
  ON oikumenea.vehicle_brands (country_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_brands_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_brands
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_brands.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_brands.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_brands.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_brands.country_id IS 'pii:none';

-- vehicle_models — a model under a brand (containment FK). generation + the manufacture window are
-- structural specs. code is unique within the brand among active rows.
CREATE TABLE oikumenea.vehicle_models (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,1,4),  -- vehicle / object / vehicle_model
  brand_id          uuid NOT NULL REFERENCES oikumenea.vehicle_brands(id) ON DELETE RESTRICT,
  code              text NOT NULL,
  name              text NOT NULL,          -- default-locale display name; translatable via the i18n store
  generation        text,
  manufacture_start date,
  manufacture_end   date,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order        integer,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT vehicle_models_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);
CREATE UNIQUE INDEX vehicle_models_brand_code_active
  ON oikumenea.vehicle_models (brand_id, code) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_models_brand_idx
  ON oikumenea.vehicle_models (brand_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_models_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_models
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_models.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_models.brand_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_models.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_models.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_models.generation IS 'pii:none';

-- ===================================================================================================
-- Object: the physical vehicle.
-- ===================================================================================================

-- vehicle_vehicles — a physical vehicle at registry grade. type_id classifies it; model_id is the
-- optional make/model. vin is the normalized Vehicle Identification Number (nullable for VIN-less
-- vehicles), unique among active rows when present (pii:basic). attributes is the long-tail spec
-- grab-bag (DS-53 will column-ize stabilized fields). The RID is the external handle (no separate code).
CREATE TABLE oikumenea.vehicle_vehicles (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,1,1),  -- vehicle / object / vehicle
  type_id          uuid NOT NULL REFERENCES oikumenea.vehicle_types(id) ON DELETE RESTRICT,
  model_id         uuid REFERENCES oikumenea.vehicle_models(id) ON DELETE RESTRICT,  -- nullable (unknown model)
  vin              text,                     -- normalized VIN; nullable; unique among active when present
  color            text,
  manufacture_date date,
  attributes       jsonb NOT NULL DEFAULT '{}'::jsonb,
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','scrapped','exported')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT vehicle_vehicles_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX vehicle_vehicles_vin_active
  ON oikumenea.vehicle_vehicles (vin) WHERE deleted_at IS NULL AND vin IS NOT NULL;
CREATE INDEX vehicle_vehicles_type_idx
  ON oikumenea.vehicle_vehicles (type_id) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_vehicles_model_idx
  ON oikumenea.vehicle_vehicles (model_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_vehicles_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_vehicles
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_vehicles.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_vehicles.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_vehicles.model_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_vehicles.vin IS 'pii:basic';
COMMENT ON COLUMN oikumenea.vehicle_vehicles.color IS 'pii:none';

-- ===================================================================================================
-- Reified Links (D-Ontology): the brand->manufacturer tie and the ownership+plate registration record.
-- ===================================================================================================

-- vehicle_brand_manufacturers — a brand is MANUFACTURED_BY a company (link__manufactured_by). Temporal:
-- a marque's manufacturer changes with acquisitions, so effective_from/effective_to record the window.
-- Both ends are real FKs (brand + the manufacturer, a `company`-domain tenant organization — M41).
CREATE TABLE oikumenea.vehicle_brand_manufacturers (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,2,1),  -- vehicle / link / manufactured_by
  brand_id       uuid NOT NULL REFERENCES oikumenea.vehicle_brands(id) ON DELETE CASCADE,
  company_id     uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- M21/M41 company org
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT vehicle_brand_manufacturers_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1)
);
CREATE INDEX vehicle_brand_manufacturers_brand_idx
  ON oikumenea.vehicle_brand_manufacturers (brand_id) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_brand_manufacturers_company_idx
  ON oikumenea.vehicle_brand_manufacturers (company_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_brand_manufacturers_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_brand_manufacturers
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_brand_manufacturers.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_brand_manufacturers.brand_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_brand_manufacturers.company_id IS 'pii:none';

-- vehicle_registrations — the ownership+plate record (link__registered_to). The owner is a person OR a
-- company: owner_kind discriminates, owner_id is the owner RID (text, no FK — polymorphic, F-014, like
-- company foundings). country_id is the registering country; subdivision_id is the OPTIONAL plate region
-- -> the shared WOF geo_places gazetteer (placetype='region', app-validated on write — D-GeoPlaces).
-- registration_number is the plate, unique among active rows PER COUNTRY. Temporal + status: a
-- re-registration is a NEW row (the prior one closed), so registration IS the ownership history.
-- Person-owned rows are pii:basic, holder-scoped, and erased on person purge (vehicle PersonPurged
-- subscriber — there is no person_id FK to CASCADE through).
CREATE TABLE oikumenea.vehicle_registrations (
  id                  uuid PRIMARY KEY DEFAULT oikumenea.new_id(17,2,2),  -- vehicle / link / registered_to
  vehicle_id          uuid NOT NULL REFERENCES oikumenea.vehicle_vehicles(id) ON DELETE CASCADE,
  owner_kind          text NOT NULL CHECK (owner_kind IN ('person','company')),
  owner_id            text NOT NULL,         -- owner RID (person or company); polymorphic, no FK
  country_id          uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  subdivision_id      uuid REFERENCES oikumenea.geo_places(id) ON DELETE RESTRICT,  -- plate region (placetype=region)
  registration_number text NOT NULL,
  number_type_id      uuid REFERENCES oikumenea.vehicle_registration_number_types(id) ON DELETE RESTRICT,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('active','closed')),
  effective_from      timestamptz NOT NULL DEFAULT now(),
  effective_to        timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  CONSTRAINT vehicle_registrations_rid_shape
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
);
-- A plate number is unique among ACTIVE registrations within a country.
CREATE UNIQUE INDEX vehicle_registrations_country_plate_active
  ON oikumenea.vehicle_registrations (country_id, registration_number)
  WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX vehicle_registrations_vehicle_idx
  ON oikumenea.vehicle_registrations (vehicle_id) WHERE deleted_at IS NULL;
CREATE INDEX vehicle_registrations_owner_idx
  ON oikumenea.vehicle_registrations (owner_kind, owner_id) WHERE deleted_at IS NULL;
CREATE TRIGGER vehicle_registrations_set_updated_at
  BEFORE UPDATE ON oikumenea.vehicle_registrations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.vehicle_registrations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registrations.vehicle_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registrations.owner_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registrations.owner_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.vehicle_registrations.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registrations.subdivision_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_registrations.registration_number IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0027_vehicle', applied_at = now() WHERE singleton;
