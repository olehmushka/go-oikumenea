-- 0009_enrichment — merged domain migration (refactor: consolidated from 0027_vehicle, 0028_overlay_foundation, 0029_external_orgs, 0030_person_physical_identity, 0031_person_addresses, 0032_person_institutional_ties, 0033_person_watchlists).

-- ===== merged from 0027_vehicle =====
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
    CHECK (oikumenea.rid_service(id)=17 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2),
  -- The polymorphic owner end carries no FK, so this shape CHECK is its only integrity on the id:
  -- a 'person' owner must be a person object RID (6,1,1); a 'company' owner must be a company-domain
  -- tenant ORGANIZATION RID (4,1,6) — M41/D-UnifiedOrgGraph. The ::uuid cast also rejects a malformed
  -- id. Existence stays app-enforced (R-32, review-2026-09).
  CONSTRAINT vehicle_registrations_owner_shape CHECK (
    (owner_kind <> 'person'  OR (oikumenea.rid_service(owner_id::uuid)=6 AND oikumenea.rid_kind(owner_id::uuid)=1 AND oikumenea.rid_type(owner_id::uuid)=1)) AND
    (owner_kind <> 'company' OR (oikumenea.rid_service(owner_id::uuid)=4 AND oikumenea.rid_kind(owner_id::uuid)=1 AND oikumenea.rid_type(owner_id::uuid)=6))
  )
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

-- ===== merged from 0028_overlay_foundation =====
-- 0028 overlay foundation (M29).
--
-- The OSINT-enrichment substrate (docs/architecture/roadmap-decisions.md, D-OverlayFoundation): the
-- cross-cutting machinery every later overlay milestone (M30-M37) rides as a thin slice. M29 lands three
-- things; only one needs a new table here:
--
--   1. Provisional persons — person_persons.status gains 'provisional' (a minimal-PII stub so every
--      relationship/overlay edge points at a real node). Added by editing 0005 in place (the status CHECK);
--      no DDL here. Resolution is the MergePerson audited action (re-homes edges, tombstones the stub).
--   2. Attribution convention — the source/confidence/as_of column-set (already used by
--      D-PersonSocialChannels), formalized in conventions.md for verbatim reuse by M30-M37. Documentation
--      only; no DDL.
--   3. legal_basis (structured) — THIS migration: a seeded platform_legal_basis_kinds catalog, FK-referenced
--      by every future pii:special overlay store (the FK consumers arrive in M31+).
--
-- platform_legal_basis_kinds is a platform-owned reference catalog with a natural `code` PK (D-Code
-- carve-out, exactly the person_platforms shape): GDPR Article 6 lawful bases and the Article 9
-- special-category processing conditions, partitioned by `article`. Instance-extensible via the API.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap (set_updated_at).
-- Seeded here (natural-key carve-out — new_id() reads no GUC, D-RIDSeeding).

-- platform_legal_basis_kinds: the structured lawful-basis catalog (D-OverlayFoundation). `article`
-- partitions the Art. 6 lawful bases from the Art. 9 special-category conditions; `name` is the
-- default-locale label (other locales in the localization store). A gated/special-category overlay row
-- references a code here (NOT NULL on pii:special stores) + an optional free-text justification note.
CREATE TABLE oikumenea.platform_legal_basis_kinds (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  article    text NOT NULL CHECK (article IN ('art6','art9')),  -- art6 = lawful basis; art9 = special-category condition
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER platform_legal_basis_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_legal_basis_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.article IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.sort_order IS 'pii:none';

-- Seed the lawful-basis catalog (natural-key carve-out). The instance admin adds more via the API.
INSERT INTO oikumenea.platform_legal_basis_kinds (code, name, article, sort_order) VALUES
  -- GDPR Article 6 — lawful bases for processing
  ('consent',                    'Consent',                                'art6', 10),
  ('contract',                   'Performance of a contract',              'art6', 20),
  ('legal_obligation',           'Legal obligation',                       'art6', 30),
  ('vital_interests',            'Vital interests',                        'art6', 40),
  ('public_task',                'Public task',                            'art6', 50),
  ('legitimate_interest',        'Legitimate interests',                   'art6', 60),
  -- GDPR Article 9(2) — conditions for processing special categories of data
  ('explicit_consent',           'Explicit consent',                       'art9', 110),
  ('employment_law',             'Employment, social security & social protection law', 'art9', 120),
  ('vital_interests_art9',       'Vital interests (data subject incapable of consent)', 'art9', 130),
  ('not_for_profit_body',        'Not-for-profit body processing',         'art9', 140),
  ('made_public_by_subject',     'Data manifestly made public by the subject', 'art9', 150),
  ('legal_claims',               'Establishment, exercise or defence of legal claims', 'art9', 160),
  ('substantial_public_interest','Substantial public interest',            'art9', 170),
  ('health_care',                'Preventive or occupational medicine / health care', 'art9', 180),
  ('public_health',              'Public health',                          'art9', 190),
  ('archiving_research',         'Archiving, scientific/historical research or statistics', 'art9', 200);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0029_external_orgs =====
-- 0029 external organizations (M30).
--
-- The external-organizations domain (docs/modules/external-organizations.md / D-ExternalOrgs): a
-- registry of external organizations a person is tied to but which the deploying org neither owns nor
-- commands — political parties, government bodies, foreign military formations, NGOs, lobbying
-- registrants/clients. It is the NODE-SPACE the M33 institutional-tie edges point at when the org side
-- is neither one of the operator's own tenant_units (authority-bearing through the PDP) nor a for-profit
-- legal entity in the M21 company registry. Faith-/sector-agnostic, catalog-typed, provenance-tagged.
--
-- Each row is catalog-typed (external_org_kinds: party | government_body | military | ngo | registrant |
-- other), carries the D-OverlayFoundation (M29) provisional/resolved status + the source/confidence/as_of
-- attribution column-set (docs/architecture/conventions.md §Attribution), an optional country ->
-- geo_countries, an optional wikidata_id concordance (the hermenea import natural key), and a translatable
-- name. It is a hermenea import target (Wikidata / public registries) via the generic
-- POST /import/external-organizations endpoint (M16).
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the kind catalog rows are seeded directly here.
-- Organizations are created through ExternalOrganizationService or the hermenea import handler.
--
-- RLS: external orgs are external reference data, instance-global, not scoped against tenant_units (like
-- company / education / location / vehicle), so no RLS is enabled. External orgs NEVER enter the tenant
-- closure / PDP graph — they are directory nodes only.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id +
-- geo_countries).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the new `external_organization` service (18) + its object/action
-- types. pkg/rid mirrors these and asserts equality at boot (kind<>3), so they are added in both places
-- together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (18, 'external_organization');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- external-organization objects
  (18,1,1,'external_organization'),(18,1,2,'external_org_kind'),
  -- external-organization Action RID (kind=3, excluded from the Go-mirror size check)
  (18,3,0,'action');

-- ===================================================================================================
-- Reference catalog (D-Code / D-i18n): code + translatable name (default-locale here, other locales in
-- the localization store). Instance-admin-managed; seeded with the D-ExternalOrgs starter set,
-- instance-extensible. Mirrors religion_org_kinds / vehicle_types.
-- ===================================================================================================

-- external_org_kinds — the org-kind catalog (party / government_body / military / ngo / registrant / other).
CREATE TABLE oikumenea.external_org_kinds (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(18,1,2),  -- external_organization / object / external_org_kind
  code       text NOT NULL,
  name       text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT external_org_kinds_rid_shape
    CHECK (oikumenea.rid_service(id)=18 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX external_org_kinds_code_active
  ON oikumenea.external_org_kinds (code) WHERE deleted_at IS NULL;
CREATE TRIGGER external_org_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.external_org_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.external_org_kinds.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_org_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_org_kinds.name IS 'pii:none';

INSERT INTO oikumenea.external_org_kinds (code, name, sort_order) VALUES
  ('party','Political party',10),
  ('government_body','Government body',20),
  ('military','Military formation',30),
  ('ngo','Non-governmental organization',40),
  ('registrant','Lobbying registrant / client',50),
  ('other','Other',60);

-- ===================================================================================================
-- Object: the external organization.
-- ===================================================================================================

-- external_organizations — an external organization at registry grade. kind_id classifies it; name is the
-- default-locale display name (translatable via the i18n store). code is an optional stable handle
-- (D-Code; the RID is the primary external handle). country_id is the org's country (nullable).
-- wikidata_id is the optional Wikidata Q-id concordance, unique among active rows when present (the
-- hermenea import natural key). status carries the D-OverlayFoundation provisional/resolved lifecycle;
-- source/confidence/as_of are the M29 attribution column-set, reused verbatim.
CREATE TABLE oikumenea.external_organizations (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(18,1,1),  -- external_organization / object / external_organization
  kind_id     uuid NOT NULL REFERENCES oikumenea.external_org_kinds(id) ON DELETE RESTRICT,
  name        text NOT NULL,                -- default-locale display name; translatable via the i18n store
  code        text,                         -- optional stable handle (D-Code); unique-active when present
  country_id  uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable
  wikidata_id text,                         -- optional Wikidata Q-id concordance; unique-active; import key
  status      text NOT NULL DEFAULT 'resolved' CHECK (status IN ('provisional','resolved')),
  -- D-OverlayFoundation attribution column-set (docs/architecture/conventions.md §Attribution), verbatim:
  source      text NOT NULL DEFAULT 'operator_verified'
                CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence  text NOT NULL DEFAULT 'possible'
                CHECK (confidence IN ('confirmed','probable','possible')),
  as_of       timestamptz,                  -- optional: when the asserted value was observed/true
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT external_organizations_rid_shape
    CHECK (oikumenea.rid_service(id)=18 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX external_organizations_code_active
  ON oikumenea.external_organizations (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE UNIQUE INDEX external_organizations_wikidata_active
  ON oikumenea.external_organizations (wikidata_id) WHERE deleted_at IS NULL AND wikidata_id IS NOT NULL;
CREATE INDEX external_organizations_kind_idx
  ON oikumenea.external_organizations (kind_id) WHERE deleted_at IS NULL;
CREATE INDEX external_organizations_country_idx
  ON oikumenea.external_organizations (country_id) WHERE deleted_at IS NULL;
CREATE INDEX external_organizations_status_idx
  ON oikumenea.external_organizations (status) WHERE deleted_at IS NULL;
CREATE TRIGGER external_organizations_set_updated_at
  BEFORE UPDATE ON oikumenea.external_organizations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.external_organizations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.kind_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.wikidata_id IS 'pii:none';

-- ===== merged from 0030_person_physical_identity =====
-- 0030 person physical identity (M31 — D-PhysicalIdentity) + structural color (M42 — D-Color) + ethnicity
-- taxonomy (M43 — D-PhysicalIdentity amendment). These three uncommitted milestones are SQUASHED into one
-- migration: tables are created in their FINAL shape (dependencies first) so there are no create-then-alter
-- churn cycles. The end-state schema is byte-identical to applying the former 0030+0031+0032 in sequence.
--
-- Moving parts:
--   1. ALIASES fold into the existing person_name_variants via a variant_kind discriminator
--      (transliteration | aka | former_legal | maiden | pseudonym | cover) — NO new table; the original
--      one-transliteration-per-locale rule becomes PARTIAL to variant_kind='transliteration'.
--   2. platform_colors (M42 / D-Color) — a platform-owned RID-bearing per-domain color catalog
--      (eye | hair | vehicle), referenced by HARD FK. Created FIRST so the person + vehicle FKs resolve.
--   3. person_physical_descriptions — effective-dated height/weight/build/blood_type (pii:basic) +
--      eye_color_id/hair_color_id HARD FKs -> platform_colors (no free-text color columns).
--   4. person_distinguishing_marks — tattoo/scar/piercing/birthmark; pii:special CEILING.
--   5. person_ethnicity_types (M31 catalog, M43 hierarchy) — self-declared vocabulary with parent_id +
--      transitive closure + group-level language/country M:N ties + Wikidata anchor + import provenance.
--   6. person_ethnicities — the reified pii:special link; the declared catalog code is ENVELOPE-ENCRYPTED
--      with a blind index (D-SpecialPII) + NOT NULL legal_basis; crypto-erased on purge. Biometrics EXCLUDED.
--   7. vehicle_vehicles gains color_id (HARD FK -> platform_colors); its legacy free-text color is dropped
--      (vehicle_vehicles is from committed 0027, so this stays an ALTER).
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only where it adds (L-UpgradeSafe / D-Migrations);
-- depends on 0000 (new_id / platform_rid_types), 0005 person (person_persons / person_name_variants),
-- 0018 language (language_languoids), 0027 vehicle (vehicle_vehicles), 0028 overlay
-- (platform_legal_basis_kinds), and the seeded geo_countries.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (1,1,1,'color'),                    -- platform / object / color (M42)
  (6,1,11,'physical_description'),
  (6,1,12,'distinguishing_mark'),
  (6,1,13,'ethnicity_type'),
  (6,2,9,'has_ethnicity');

-- ===================================================================================================
-- platform_colors — the per-domain color catalog (D-Color / D-Code / D-i18n). Created BEFORE the person
-- + vehicle tables that hard-FK it. `name` is the default-locale label (full i18n overlaid from the
-- localization store, keyed by the color RID); `hex` is a NULLABLE representative swatch.
-- ===================================================================================================
CREATE TABLE oikumenea.platform_colors (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(1,1,1),  -- platform / object / color
  domain     text NOT NULL CHECK (domain IN ('eye','hair','vehicle','rank','religion','ethnicity','country')),  -- +pinax palettes (D-Pinax, M45)
  code       text NOT NULL,
  name       text NOT NULL,  -- default-locale label; full i18n overlaid from the localization store
  hex        text CHECK (hex IS NULL OR hex ~ '^#[0-9A-Fa-f]{6}$'),  -- nullable representative swatch
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT platform_colors_rid_shape
    CHECK (oikumenea.rid_service(id)=1 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX platform_colors_domain_code_active
  ON oikumenea.platform_colors (domain, code) WHERE deleted_at IS NULL;
CREATE INDEX platform_colors_domain_idx
  ON oikumenea.platform_colors (domain) WHERE deleted_at IS NULL;
CREATE TRIGGER platform_colors_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_colors
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.platform_colors.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.domain IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.hex IS 'pii:none';

-- Seed the baseline palettes. eye/hair are categorical (no hex); vehicle carries representative hex.
INSERT INTO oikumenea.platform_colors (domain, code, name, hex, sort_order) VALUES
  -- eye colors (categorical)
  ('eye', 'amber',  'Amber',  NULL, 10),
  ('eye', 'blue',   'Blue',   NULL, 20),
  ('eye', 'brown',  'Brown',  NULL, 30),
  ('eye', 'gray',   'Gray',   NULL, 40),
  ('eye', 'green',  'Green',  NULL, 50),
  ('eye', 'hazel',  'Hazel',  NULL, 60),
  ('eye', 'black',  'Black',  NULL, 70),
  -- hair colors (categorical)
  ('hair', 'auburn', 'Auburn', NULL, 10),
  ('hair', 'black',  'Black',  NULL, 20),
  ('hair', 'blonde', 'Blonde', NULL, 30),
  ('hair', 'brown',  'Brown',  NULL, 40),
  ('hair', 'gray',   'Gray',   NULL, 50),
  ('hair', 'red',    'Red',    NULL, 60),
  ('hair', 'white',  'White',  NULL, 70),
  ('hair', 'bald',   'Bald',   NULL, 80),
  -- vehicle colors (with representative hex swatches)
  ('vehicle', 'black',  'Black',  '#000000', 10),
  ('vehicle', 'white',  'White',  '#FFFFFF', 20),
  ('vehicle', 'gray',   'Gray',   '#808080', 30),
  ('vehicle', 'silver', 'Silver', '#C0C0C0', 40),
  ('vehicle', 'blue',   'Blue',   '#0033A0', 50),
  ('vehicle', 'red',    'Red',    '#C8102E', 60),
  ('vehicle', 'green',  'Green',  '#2E7D32', 70),
  ('vehicle', 'yellow', 'Yellow', '#F2C200', 80),
  ('vehicle', 'brown',  'Brown',  '#6D4C41', 90);

-- ===================================================================================================
-- 1. Aliases fold into person_name_variants (no new table). This ALTERs a table from committed 0005.
-- ===================================================================================================
ALTER TABLE oikumenea.person_name_variants
  ADD COLUMN variant_kind text NOT NULL DEFAULT 'transliteration'
    CHECK (variant_kind IN ('transliteration','aka','former_legal','maiden','pseudonym','cover')),
  ADD COLUMN source     text,   -- attribution (D-OverlayFoundation) — alias provenance
  ADD COLUMN confidence text;

COMMENT ON COLUMN oikumenea.person_name_variants.variant_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.confidence IS 'pii:none';

-- The original UNIQUE (person_id, locale) modelled "one canonical transliteration per locale". With
-- aliases now sharing the table that rule must apply ONLY to transliterations; aliases are unconstrained
-- (a person may have several AKAs / cover names). Replace the total constraint with a partial index.
ALTER TABLE oikumenea.person_name_variants
  DROP CONSTRAINT person_name_variants_person_locale_uniq;
CREATE UNIQUE INDEX person_name_variants_translit_uniq
  ON oikumenea.person_name_variants (person_id, locale) WHERE variant_kind = 'transliteration';

-- ===================================================================================================
-- 2. person_physical_descriptions — effective-dated physical description (pii:basic). eye/hair color are
--    HARD FKs into platform_colors (D-Color); the FK columns are LAST (parity with the former ALTER-add
--    ordering, so the generated row shape is unchanged).
-- ===================================================================================================
CREATE TABLE oikumenea.person_physical_descriptions (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,11),  -- person / object / physical_description
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  height_cm      integer CHECK (height_cm IS NULL OR (height_cm > 0 AND height_cm < 300)),
  weight_kg      integer CHECK (weight_kg IS NULL OR (weight_kg > 0 AND weight_kg < 700)),
  build          text,   -- advisory free text (e.g. slim, athletic, heavy)
  blood_type     text CHECK (blood_type IS NULL OR blood_type IN
                   ('A+','A-','B+','B-','AB+','AB-','O+','O-','unknown')),
  effective_from date NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
  effective_to   date,   -- NULL = current
  source         text,
  confidence     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  eye_color_id   uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT,  -- domain='eye' (D-Color)
  hair_color_id  uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT,  -- domain='hair' (D-Color)
  CONSTRAINT person_physical_descriptions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=11)
);

CREATE TRIGGER person_physical_descriptions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_physical_descriptions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_physical_descriptions_person_idx
  ON oikumenea.person_physical_descriptions (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_physical_descriptions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.height_cm IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.weight_kg IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.build IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.blood_type IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.effective_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.eye_color_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.hair_color_id IS 'pii:basic';

-- ===================================================================================================
-- 3. person_distinguishing_marks — tattoos/scars/piercings/birthmarks (pii:special CEILING).
-- ===================================================================================================
CREATE TABLE oikumenea.person_distinguishing_marks (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,12),  -- person / object / distinguishing_mark
  person_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  kind          text NOT NULL CHECK (kind IN ('tattoo','scar','piercing','birthmark')),
  body_location text,   -- e.g. left forearm; free text
  description   text,   -- the mark's appearance; a tattoo may reveal Art. 9 data -> pii:special
  source        text,
  confidence    text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT person_distinguishing_marks_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=12)
);

CREATE TRIGGER person_distinguishing_marks_set_updated_at
  BEFORE UPDATE ON oikumenea.person_distinguishing_marks
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_distinguishing_marks_person_idx
  ON oikumenea.person_distinguishing_marks (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_distinguishing_marks.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.kind IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.body_location IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.description IS 'pii:special';

-- ===================================================================================================
-- 4a. person_ethnicity_types — the open, instance-admin-managed declared-ethnicity vocabulary, promoted
--     to a HIERARCHICAL catalog (M43 / D-PhysicalIdentity amendment): parent_id self-FK + transitive
--     closure (below) + Wikidata anchor + import provenance. Plaintext (a controlled vocabulary, not a
--     person's datum); a person's SELECTION is the Art. 9 datum, encrypted in person_ethnicities. Seeded
--     EMPTY (ethnicity is contentious; loaded on purpose via the opt-in `ethnicity-scheme` import). The
--     hierarchy/provenance columns are LAST (parity with the former ALTER-add ordering).
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicity_types (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,13),  -- person / object / ethnicity_type
  code           text NOT NULL,
  name           text NOT NULL,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order     integer,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  parent_id      uuid REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE RESTRICT,  -- "" = root (M43)
  wikidata_id    text,
  source         text,
  source_version text,
  imported_at    timestamptz,
  CONSTRAINT person_ethnicity_types_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=13)
);
CREATE UNIQUE INDEX person_ethnicity_types_code_active
  ON oikumenea.person_ethnicity_types (code) WHERE deleted_at IS NULL;
CREATE INDEX person_ethnicity_types_parent_idx
  ON oikumenea.person_ethnicity_types (parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_ethnicity_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_ethnicity_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_ethnicity_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.wikidata_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.source_version IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.imported_at IS 'pii:none';

-- ===================================================================================================
-- 4b. person_ethnicities — the reified pii:special Link link__has_ethnicity (Person → a declared
--     ethnicity). The declared value (the catalog code) is ENVELOPE-ENCRYPTED with a blind index so the
--     Art. 9 datum never sits in plaintext; value_blind_index = BlindIndex(code) enables equality search
--     ("who declared ethnicity X") without decryption. There is NO plaintext FK to person_ethnicity_types
--     for that reason — the application validates the code against the catalog before sealing. A NOT NULL
--     legal_basis records the lawful basis (D-OverlayFoundation art9 condition). Crypto-erased on purge.
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicities (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,9),  -- person / link / has_ethnicity
  person_id         uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  -- envelope-encrypted declared ethnicity code (D-SpecialPII; same shape as religion_affiliations).
  -- Always populated at creation (the application requires a value), but NULLABLE so crypto-erase on
  -- purge can drop all four (the row survives as a tombstone).
  value_ciphertext  bytea,
  wrapped_dek       bytea,
  key_ref           text,
  value_blind_index bytea,
  legal_basis       text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  source            text,
  confidence        text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT person_ethnicities_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=9)
);
CREATE INDEX person_ethnicities_person_idx
  ON oikumenea.person_ethnicities (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_ethnicities_blind_idx
  ON oikumenea.person_ethnicities (value_blind_index) WHERE deleted_at IS NULL;
CREATE TRIGGER person_ethnicities_set_updated_at
  BEFORE UPDATE ON oikumenea.person_ethnicities
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_ethnicities.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.value_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.value_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.status IS 'pii:none';

-- ===================================================================================================
-- 4c. Ethnicity taxonomy aux tables (M43): the transitive closure + the group-level ethnolinguistic
--     (→ language_languoids) and homeland (→ geo_countries) M:N ties. Bare relations — composite keys,
--     NO RID (mirror language_languoid_closure / _countries). Group metadata; NEVER inferred onto a person.
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicity_type_closure (
  ancestor_id   uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  descendant_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  depth         integer NOT NULL,
  PRIMARY KEY (ancestor_id, descendant_id)
);
CREATE INDEX person_ethnicity_type_closure_descendant_idx
  ON oikumenea.person_ethnicity_type_closure (descendant_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.ancestor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.descendant_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.depth IS 'pii:none';

CREATE TABLE oikumenea.person_ethnicity_type_languages (
  ethnicity_type_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  language_id       uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE RESTRICT,
  PRIMARY KEY (ethnicity_type_id, language_id)
);
CREATE INDEX person_ethnicity_type_languages_language_idx
  ON oikumenea.person_ethnicity_type_languages (language_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_languages.ethnicity_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_languages.language_id IS 'pii:none';

CREATE TABLE oikumenea.person_ethnicity_type_countries (
  ethnicity_type_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  country_id        uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  PRIMARY KEY (ethnicity_type_id, country_id)
);
CREATE INDEX person_ethnicity_type_countries_country_idx
  ON oikumenea.person_ethnicity_type_countries (country_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_countries.ethnicity_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_countries.country_id IS 'pii:none';

-- ===================================================================================================
-- 5. vehicle_vehicles.color (text) -> color_id (HARD FK into platform_colors, D-Color). vehicle_vehicles
--    is from committed 0027, so this stays an ALTER: add color_id, best-effort backfill, drop color.
-- ===================================================================================================
ALTER TABLE oikumenea.vehicle_vehicles
  ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
UPDATE oikumenea.vehicle_vehicles v
  SET color_id = c.id
  FROM oikumenea.platform_colors c
  WHERE c.domain = 'vehicle' AND c.deleted_at IS NULL
    AND v.color IS NOT NULL AND lower(btrim(v.color)) = c.code;
ALTER TABLE oikumenea.vehicle_vehicles DROP COLUMN color;
CREATE INDEX vehicle_vehicles_color_idx
  ON oikumenea.vehicle_vehicles (color_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.vehicle_vehicles.color_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- pinax origin marker (D-Pinax, M45): 'seeded' = managed by a bundled preset (`colors` palettes;
-- `ethnicities` catalog via the import path), 'operator' = created via the admin API. platform_colors'
-- migration-seeded palettes are seeded-owned; person_ethnicity_types seeds empty (default 'operator',
-- and the ethnicity import handler stamps origin='seeded' on loaded rows — D-Pinax).
ALTER TABLE oikumenea.platform_colors        ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
ALTER TABLE oikumenea.person_ethnicity_types ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
UPDATE oikumenea.platform_colors SET origin = 'seeded';
COMMENT ON COLUMN oikumenea.platform_colors.origin IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.origin IS 'pii:none';

-- pinax structural color (D-Pinax, M45 + D-Color): a nullable display color on the seeded reference
-- catalogs, referencing the shared platform_colors palette by hard FK. These ALTERs live here (not in
-- each table's own earlier migration) because platform_colors is created above in THIS migration — a
-- forward reference from 0000/0004/0023 would fail. Palette rows for the new domains arrive via the
-- bundled `colors` preset; the domain match (e.g. a country references a 'country'-domain color) is
-- validated app-side via the D-Color ColorLookup, as with eye/hair/vehicle.
ALTER TABLE oikumenea.geo_countries          ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.rank_ranks             ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.religion_taxa          ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.person_ethnicity_types ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
COMMENT ON COLUMN oikumenea.geo_countries.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_taxa.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.color_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0031_person_addresses =====
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

-- ===== merged from 0032_person_institutional_ties =====
-- 0032 person institutional & political ties (M33 — D-InstitutionalTies). The draft's macro-category 7,
-- modelled as per-type reified person↔organization affiliation edges (not one generic "affiliation" blob),
-- so each edge keeps its own PII tier and attribute set — mirroring the M14 relationship pattern. Every
-- edge carries the D-OverlayFoundation attribution column-set (source/confidence). The org side is usually
-- an external body (M30 external_organizations); for OSINT-ingested rows it is a free-text label, optionally
-- resolvable to a polymorphic org RID later.
--
--   person_party_memberships       — party affiliation (political opinion). pii:special (GDPR Art. 9):
--                                    the party itself is envelope-encrypted (same shape as the M31
--                                    person_ethnicities / religion_affiliations), NOT-NULL legal_basis.
--   person_government_positions    — held public office; pep_trigger (auto-true, persists post-office) feeds
--                                    the M34 PEP check. pii:basic.
--   person_lobbying_relationships  — registrant↔client lobbying filings. pii:basic.
--   person_external_references     — reference object: wikipedia/news/registry/… links about a person
--                                    (mirrors person_social_accounts; a hermenea import target). pii:basic.
--
-- Foreign / historical military service reuses membership against external_organizations military stubs +
-- rank (no new table here); emergency contacts reuse the M14 person_relation_types catalog — this migration
-- only seeds the 'emergency' label (no new entity). Inferred political leaning is NOT here — it is a separate
-- M35 pii:special overlay, never merged with the declared party membership below.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons), 0014 (person_relation_types),
-- 0028 (platform_legal_basis_kinds), and 0001 (geo_countries).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,14,'external_reference'),    -- person / object / external_reference (M33)
  (6,2,11,'party_membership'),      -- person / link   / party_membership   (M33, encrypted)
  (6,2,12,'government_position'),   -- person / link   / government_position (M33)
  (6,2,13,'lobbying_rel');          -- person / link   / lobbying_rel        (M33)

-- ===================================================================================================
-- person_party_memberships — the reified pii:special Link link__party_membership. Political opinion is a
-- GDPR Art. 9 special category, so the party identity is envelope-encrypted (ciphertext/wrapped_dek/
-- key_ref/blind_index sealed in the application; NO plaintext party FK) with a NOT-NULL legal_basis. Role,
-- dates and attribution stay in plaintext (the sensitive datum is WHICH party, not the membership role).
-- Crypto-erased on purge (drop the envelope, keep the row tombstone).
-- ===================================================================================================
CREATE TABLE oikumenea.person_party_memberships (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,11),  -- person / link / party_membership
  person_id         uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  -- envelope-encrypted party identity (a party name or an external_organizations RID). Always populated at
  -- creation, but NULLABLE so crypto-erase on purge can drop all four (the row survives as a tombstone).
  party_ciphertext  bytea,
  party_wrapped_dek bytea,
  party_key_ref     text,
  party_blind_index bytea,
  role              text NOT NULL DEFAULT 'member'
                      CHECK (role IN ('member','official','candidate','donor','supporter','other')),
  valid_from        date,
  valid_to          date,          -- NULL = current
  legal_basis       text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  source            text NOT NULL DEFAULT 'operator_verified'
                      CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence        text NOT NULL DEFAULT 'possible'
                      CHECK (confidence IN ('confirmed','probable','possible')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT person_party_memberships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=11)
);

CREATE TRIGGER person_party_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_party_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_party_memberships_person_idx
  ON oikumenea.person_party_memberships (person_id) WHERE deleted_at IS NULL;
-- Blind-index lookup ("who else belongs to this party") without decrypting.
CREATE INDEX person_party_memberships_blind_idx
  ON oikumenea.person_party_memberships (party_blind_index) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_party_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.role IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.confidence IS 'pii:none';

-- ===================================================================================================
-- person_government_positions — the reified pii:basic Link link__government_position. Public office is
-- public-record, not special-category, so it is stored in plaintext. pep_trigger defaults true and PERSISTS
-- after the position ends (valid_to set) — a former official stays a PEP; the M34 watchlist check derives
-- PEP status from these rows. org_id is an optional, unvalidated polymorphic reference to the resolved body
-- (external_organizations | company | tenant_unit); body is the always-present free-text label.
-- ===================================================================================================
CREATE TABLE oikumenea.person_government_positions (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,12),  -- person / link / government_position
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  title        text NOT NULL,                                       -- e.g. "Minister of Defence", "Senator"
  body         text NOT NULL,                                       -- e.g. "Ministry of Defence", "US Senate"
  org_id       uuid,                                                -- optional resolved body RID (polymorphic; no FK)
  country_id   uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  level        text NOT NULL DEFAULT 'national'
                 CHECK (level IN ('international','national','regional','local')),
  role_type    text,                                                -- e.g. "elected", "appointed", "career_civil_service"
  valid_from   date,
  valid_to     date,          -- NULL = current
  pep_trigger  boolean NOT NULL DEFAULT true,                       -- politically-exposed person (persists post-office)
  source       text NOT NULL DEFAULT 'operator_verified'
                 CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence   text NOT NULL DEFAULT 'possible'
                 CHECK (confidence IN ('confirmed','probable','possible')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT person_government_positions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=12)
);

CREATE TRIGGER person_government_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_government_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_government_positions_person_idx
  ON oikumenea.person_government_positions (person_id) WHERE deleted_at IS NULL;
-- PEP derivation ("is this person politically exposed") reads active pep_trigger rows by person.
CREATE INDEX person_government_positions_pep_idx
  ON oikumenea.person_government_positions (person_id) WHERE pep_trigger AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_government_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.title IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.body IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.level IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.role_type IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.pep_trigger IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.confidence IS 'pii:none';

-- ===================================================================================================
-- person_lobbying_relationships — the reified pii:basic Link link__lobbying_rel. A public lobbying filing:
-- the person as registrant lobbying on behalf of a client before a legislative body, on a set of issues.
-- Free-text org sides (public-registry data); filing_id/source_url anchor the provenance.
-- ===================================================================================================
CREATE TABLE oikumenea.person_lobbying_relationships (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,13),  -- person / link / lobbying_rel
  person_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  registrant       text NOT NULL,                                      -- the registered lobbyist/firm
  client           text,                                               -- on whose behalf
  legislative_body text,                                               -- e.g. "US Congress", "European Parliament"
  issues           text[] NOT NULL DEFAULT '{}',                       -- policy areas
  filing_id        text,                                               -- the public filing identifier
  source_url       text,
  valid_from       date,
  valid_to         date,
  source           text NOT NULL DEFAULT 'operator_verified'
                     CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence       text NOT NULL DEFAULT 'possible'
                     CHECK (confidence IN ('confirmed','probable','possible')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT person_lobbying_relationships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=13)
);

CREATE TRIGGER person_lobbying_relationships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_lobbying_relationships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_lobbying_relationships_person_idx
  ON oikumenea.person_lobbying_relationships (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_lobbying_relationships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.registrant IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.client IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.legislative_body IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.issues IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.filing_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.source_url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.confidence IS 'pii:none';

-- ===================================================================================================
-- person_external_references — the pii:basic Object external_reference. A pointer to an off-platform source
-- about a person (wikipedia/news/registry/social/court/other). Mirrors person_social_accounts and is a
-- hermenea import target; disputed flags a contested reference. Not a Link (it points at a URL, not a
-- reified relationship with another Object).
-- ===================================================================================================
CREATE TABLE oikumenea.person_external_references (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,14),  -- person / object / external_reference
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  kind         text NOT NULL DEFAULT 'other'
                 CHECK (kind IN ('wikipedia','news','registry','social','court','academic','other')),
  url          text NOT NULL,
  external_id  text,                                                -- the id within the source system
  categories   text[] NOT NULL DEFAULT '{}',
  last_checked timestamptz,
  disputed     boolean NOT NULL DEFAULT false,
  source       text NOT NULL DEFAULT 'imported'
                 CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence   text NOT NULL DEFAULT 'possible'
                 CHECK (confidence IN ('confirmed','probable','possible')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT person_external_references_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=14)
);

CREATE TRIGGER person_external_references_set_updated_at
  BEFORE UPDATE ON oikumenea.person_external_references
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_external_references_person_idx
  ON oikumenea.person_external_references (person_id) WHERE deleted_at IS NULL;
-- Dedup an active (person, url) — one reference row per source link.
CREATE UNIQUE INDEX person_external_references_person_url
  ON oikumenea.person_external_references (person_id, url) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_external_references.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.external_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.categories IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.last_checked IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.disputed IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Emergency contacts (D-InstitutionalTies): NO new entity — add an 'emergency' label to the M14
-- person_relation_types catalog (category next_of_kin), so an emergency contact is an ordinary
-- person↔person next-of-kin relationship.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.person_relation_types (code, name, category, sort_order) VALUES
  ('emergency', 'Emergency contact', 'next_of_kin', 75)
ON CONFLICT (code) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0033_person_watchlists =====
-- 0033 person watchlists & regulatory exposure (M34 — D-Watchlists). Watchlist screening is NEVER stored
-- statically. Two surfaces:
--
--   person_watchlist_matches      — the persisted RESULT of a live screening check that runs OUT to the
--                                    hermenea companion (which owns egress to OFAC/EU/UN/INTERPOL + a ≤24h
--                                    cache). Only per-person MATCH METADATA lands here — never the lists.
--                                    One active row per person (a re-check refreshes it). pii:sensitive.
--                                    PEP is a snapshot of the M33 government-position derivation at check
--                                    time. Hard-deleted on purge (a transient screening result, not a
--                                    legal record).
--   person_regulatory_sanctions   — a structured regulatory-action overlay (regulator/action/amount/status).
--                                    Audited manual CRUD AND a hermenea import target (idempotent by
--                                    (person, external_id)). pii:sensitive; erased on purge.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons), 0028 (platform_legal_basis_kinds),
-- and 0001 (geo_countries). The M33 pep_trigger seam (person_government_positions) feeds the PEP snapshot.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,15,'watchlist_match'),        -- person / object / watchlist_match     (M34)
  (6,1,16,'regulatory_sanction');    -- person / object / regulatory_sanction (M34)

-- ===================================================================================================
-- person_watchlist_matches — the pii:sensitive Object watchlist_match. The persisted residue of a live
-- watchlist check (D-Watchlists): match METADATA only, never the underlying lists. One active row per
-- person; CheckWatchlists upserts it (partial-unique on person_id) so re-screening refreshes last_checked/
-- next_check_due in place rather than accumulating history. `pep` is a snapshot of the M33 pep_trigger
-- derivation captured at check time (the live external providers do not return PEP; it is derived locally).
-- Hard-deleted on person purge — a transient screening flag, not a record to crypto-tombstone.
-- ===================================================================================================
CREATE TABLE oikumenea.person_watchlist_matches (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,15),  -- person / object / watchlist_match
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  on_list        boolean NOT NULL DEFAULT false,                     -- any hit across the queried providers
  lists          text[] NOT NULL DEFAULT '{}',                       -- e.g. {OFAC_SDN, EU_CFSP, INTERPOL_RED}
  program        text,                                               -- e.g. sanctions program / notice class
  match_score    numeric,                                            -- 0..1 best-match score across providers
  pep            boolean NOT NULL DEFAULT false,                     -- PEP snapshot (M33 government positions)
  last_checked   timestamptz NOT NULL DEFAULT now(),
  next_check_due timestamptz,                                        -- when the ≤24h cache lapses upstream
  source         text NOT NULL DEFAULT 'imported'
                   CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence     text NOT NULL DEFAULT 'possible'
                   CHECK (confidence IN ('confirmed','probable','possible')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_watchlist_matches_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=15)
);

CREATE TRIGGER person_watchlist_matches_set_updated_at
  BEFORE UPDATE ON oikumenea.person_watchlist_matches
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One active screening result per person — CheckWatchlists refreshes rather than accumulates.
CREATE UNIQUE INDEX person_watchlist_matches_person
  ON oikumenea.person_watchlist_matches (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_watchlist_matches.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.on_list IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.lists IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.program IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.match_score IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.pep IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.last_checked IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.next_check_due IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.confidence IS 'pii:none';

-- ===================================================================================================
-- person_regulatory_sanctions — the pii:sensitive Object regulatory_sanction. A structured record of a
-- regulatory/enforcement action against a person (a licensed professional, a director, etc.), distinct from
-- the volatile live-lookup above: this is durable, operator-curated or hermenea-imported reference data.
-- Idempotent import keys on (person_id, external_id). legal_basis is optional (Art. 6/9) since a public
-- enforcement action is public-record but may carry special-category context.
-- ===================================================================================================
CREATE TABLE oikumenea.person_regulatory_sanctions (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,16),  -- person / object / regulatory_sanction
  person_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  regulator     text NOT NULL,                                      -- e.g. "SEC", "FCA", "NBU"
  action_type   text NOT NULL DEFAULT 'other'
                  CHECK (action_type IN ('fine','ban','license_revocation','warning','settlement','debarment','other')),
  amount        numeric,                                            -- monetary penalty, if any
  currency      text,                                               -- ISO-4217, when amount is present
  status        text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','appealed','overturned','expired','settled')),
  sanction_date date,
  source_url    text,
  external_id   text,                                               -- the id within the source system (import key)
  legal_basis   text REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  source        text NOT NULL DEFAULT 'operator_verified'
                  CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence    text NOT NULL DEFAULT 'possible'
                  CHECK (confidence IN ('confirmed','probable','possible')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT person_regulatory_sanctions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=16),
  CONSTRAINT person_regulatory_sanctions_amount_currency
    CHECK (amount IS NULL OR currency IS NOT NULL)
);

CREATE TRIGGER person_regulatory_sanctions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_regulatory_sanctions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_regulatory_sanctions_person_idx
  ON oikumenea.person_regulatory_sanctions (person_id) WHERE deleted_at IS NULL;
-- Idempotent hermenea import: one active row per (person, source id).
CREATE UNIQUE INDEX person_regulatory_sanctions_person_extid
  ON oikumenea.person_regulatory_sanctions (person_id, external_id)
  WHERE external_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.regulator IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.action_type IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.amount IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.currency IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.status IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.sanction_date IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.source_url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.external_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0009_enrichment', applied_at = now() WHERE singleton;
