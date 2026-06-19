-- 0022 company (M21).
--
-- The company domain (docs/modules/company.md / D-Companies): a generic, domain-agnostic legal-entity
-- registry over person + the M19 location foundation. Scoped to STRUCTURAL registry data — identity,
-- legal form, multi-jurisdiction registration, locations, positions, and the ownership/affiliation
-- graph (founders, shareholders, beneficial owners, parent/subsidiary, succession) — so people and
-- companies link into one queryable graph. Volatile intelligence (financials/court/tax/sanctions,
-- web/contact, ownership closure / computed-UBO) is PARKED (DS-45/46/47), not built here.
--
-- Mirrors the education templates: catalogs (legal forms / registration schemes / industry classes),
-- objects (companies, registrations, positions), positions + one-holder effective-dated appointments
-- (membership pattern), and reified Links for the ownership graph. Companies reference the shared M19
-- location_locations by FK (company_locations), and the RID-keyed geo_countries registry by FK.
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the catalog reference rows (legal forms,
-- registration schemes, industry classes) are seeded directly here. Companies / registrations /
-- positions / appointments and the ownership-graph link rows are created through CompanyService.
--
-- Polymorphic holders (a founder / shareholder is a person OR a company) are carried as
-- (holder_kind, holder_id TEXT) WITHOUT a FK — F-014 / D-RIDSeeding keep polymorphic target ids as
-- text (the RID self-describes its service/kind). Beneficiaries (UBO) are always natural persons, so
-- they carry a real person_id FK (CASCADE) and are erased on person purge.
--
-- RLS: company entities are external reference data, instance-global, not scoped against tenant_units
-- (like education / location), so no RLS is enabled. The person-referencing link rows (appointments,
-- person-holder foundings/shareholdings, beneficiaries) are holder-scoped and purge-erased via the
-- person purge sweep (D-PIITiers).
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id +
-- geo_countries), 0005 person (person_persons) and 0019 location (location_locations).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the new `company` service (15) + its object/link/action types.
-- pkg/rid mirrors these and asserts equality at boot (kind<>3), so they are added in both places together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (15, 'company');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- company objects
  (15,1,1,'company'),(15,1,2,'legal_form'),(15,1,3,'registration_scheme'),(15,1,4,'industry_class'),
  (15,1,5,'company_position'),(15,1,6,'registration'),
  -- company links
  (15,2,1,'holds_company_position'),(15,2,2,'founded'),(15,2,3,'owns_stake'),(15,2,4,'beneficiary_of'),
  (15,2,5,'succeeded_by'),(15,2,6,'branch_of'),(15,2,7,'has_industry'),(15,2,8,'located_at'),
  -- company Action RID (kind=3, excluded from the Go-mirror size check)
  (15,3,0,'action');

-- ===================================================================================================
-- Reference catalogs (D-Code / D-i18n): code + translatable name (default-locale here, other locales in
-- the localization store). Instance-admin-managed; seeded with a starter set, instance-extensible.
-- ===================================================================================================

-- company_legal_forms — per-country legal forms (ТОВ/ПАТ/ФОП, LLC/JSC/GmbH/PLC …). country_id is
-- nullable (a generic, jurisdiction-agnostic form). `abbreviation` is the short legal designator.
CREATE TABLE oikumenea.company_legal_forms (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,2),  -- company / object / legal_form
  code         text NOT NULL,
  name         text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  abbreviation text,
  country_id   uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (generic form)
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order   integer,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT company_legal_forms_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX company_legal_forms_code_active
  ON oikumenea.company_legal_forms (code) WHERE deleted_at IS NULL;
CREATE TRIGGER company_legal_forms_set_updated_at
  BEFORE UPDATE ON oikumenea.company_legal_forms
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_legal_forms.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_legal_forms.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_legal_forms.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_legal_forms.abbreviation IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_legal_forms.country_id IS 'pii:none';

INSERT INTO oikumenea.company_legal_forms (code, name, abbreviation, sort_order) VALUES
  ('llc','Limited liability company','LLC',10),
  ('jsc','Joint-stock company','JSC',20),
  ('plc','Public limited company','PLC',30),
  ('gmbh','Gesellschaft mit beschränkter Haftung','GmbH',40),
  ('sole-proprietor','Sole proprietor',NULL,50),
  ('state-enterprise','State enterprise',NULL,60);
-- UA-specific forms (resolve country_id from the seeded geo_countries registry).
INSERT INTO oikumenea.company_legal_forms (code, name, abbreviation, country_id, sort_order)
SELECT v.code, v.name, v.abbr, c.id, v.so
FROM (VALUES
  ('ua-tov','Товариство з обмеженою відповідальністю','ТОВ',110),
  ('ua-pat','Публічне акціонерне товариство','ПАТ',120),
  ('ua-fop','Фізична особа-підприємець','ФОП',130)
) AS v(code,name,abbr,so)
CROSS JOIN LATERAL (SELECT id FROM oikumenea.geo_countries WHERE code = 'UA' LIMIT 1) c;

-- company_registration_schemes — per-scheme registration identifier kinds (mirrors
-- document_personal_code_schemes, D-PersonalCodes). validator_pattern is a POSIX regex the identifier
-- must match (NULL = no validation). is_global marks the worldwide spine (LEI, ISO 17442).
CREATE TABLE oikumenea.company_registration_schemes (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,3),  -- company / object / registration_scheme
  code              text NOT NULL,
  name              text NOT NULL,
  validator_pattern text,                     -- POSIX regex; NULL = accept any
  is_global         boolean NOT NULL DEFAULT false,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order        integer,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT company_registration_schemes_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE UNIQUE INDEX company_registration_schemes_code_active
  ON oikumenea.company_registration_schemes (code) WHERE deleted_at IS NULL;
CREATE TRIGGER company_registration_schemes_set_updated_at
  BEFORE UPDATE ON oikumenea.company_registration_schemes
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_registration_schemes.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registration_schemes.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registration_schemes.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registration_schemes.validator_pattern IS 'pii:none';

INSERT INTO oikumenea.company_registration_schemes (code, name, validator_pattern, is_global, sort_order) VALUES
  ('lei','Legal Entity Identifier (ISO 17442)','^[A-Z0-9]{18}[0-9]{2}$',true,10),
  ('duns','Dun & Bradstreet D-U-N-S Number','^[0-9]{9}$',true,20),
  ('ua-edrpou','Ukraine EDRPOU code','^[0-9]{8}$',false,30),
  ('vat','VAT registration number',NULL,false,40),
  ('us-ein','US Employer Identification Number','^[0-9]{2}-?[0-9]{7}$',false,50);

-- company_industry_classes — economic-activity classification (NACE/ISIC/KVED). Flat starter set; the
-- hierarchy is parked (DS-47-adjacent) — instance-extensible. `system` names the classification scheme.
CREATE TABLE oikumenea.company_industry_classes (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,4),  -- company / object / industry_class
  code       text NOT NULL,
  name       text NOT NULL,
  system     text NOT NULL DEFAULT 'nace' CHECK (system IN ('nace','isic','kved')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT company_industry_classes_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);
CREATE UNIQUE INDEX company_industry_classes_code_active
  ON oikumenea.company_industry_classes (code) WHERE deleted_at IS NULL;
CREATE TRIGGER company_industry_classes_set_updated_at
  BEFORE UPDATE ON oikumenea.company_industry_classes
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_industry_classes.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_classes.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_classes.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_classes.system IS 'pii:none';

INSERT INTO oikumenea.company_industry_classes (code, name, system, sort_order) VALUES
  ('nace-a','Agriculture, forestry and fishing','nace',10),
  ('nace-c','Manufacturing','nace',20),
  ('nace-f','Construction','nace',30),
  ('nace-g','Wholesale and retail trade','nace',40),
  ('nace-j','Information and communication','nace',50),
  ('nace-k','Financial and insurance activities','nace',60),
  ('nace-m','Professional, scientific and technical activities','nace',70),
  ('nace-q','Human health and social work activities','nace',80);

-- ===================================================================================================
-- Objects: companies, registrations, positions.
-- ===================================================================================================

-- company_companies — a legal entity at registry grade. `code` is the stable external key (unique among
-- active); legal_name is the translatable registered name; short_name is a plain trading/short name.
-- ownership_category and legal_form are orthogonal axes (a private LLC vs a state-owned JSC).
CREATE TABLE oikumenea.company_companies (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,1),  -- company / object / company
  code               text NOT NULL,
  legal_name         text NOT NULL,           -- default-locale registered name; translatable via the i18n store
  short_name         text,                    -- plain trading/short name
  legal_form_id      uuid NOT NULL REFERENCES oikumenea.company_legal_forms(id) ON DELETE RESTRICT,
  ownership_category text NOT NULL DEFAULT 'private'
                       CHECK (ownership_category IN ('private','public','state_owned','municipal','foreign','mixed')),
  country_id         uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (multinational)
  founded_on         date,
  dissolved_on       date,
  state              text NOT NULL DEFAULT 'active' CHECK (state IN ('active','dissolved','merged')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT company_companies_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX company_companies_code_active
  ON oikumenea.company_companies (code) WHERE deleted_at IS NULL;
CREATE INDEX company_companies_country_idx
  ON oikumenea.company_companies (country_id) WHERE deleted_at IS NULL;
CREATE INDEX company_companies_legal_form_idx
  ON oikumenea.company_companies (legal_form_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_companies_set_updated_at
  BEFORE UPDATE ON oikumenea.company_companies
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_companies.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.legal_name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.short_name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.legal_form_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.ownership_category IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_companies.country_id IS 'pii:none';

-- company_registrations — a company's per-scheme registration identifier (mirrors document
-- personal_codes). identifier is the registered number; validated records whether it matched the
-- scheme's validator_pattern. Unique per (scheme, identifier) among active rows.
CREATE TABLE oikumenea.company_registrations (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,6),  -- company / object / registration
  company_id uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  scheme_id  uuid NOT NULL REFERENCES oikumenea.company_registration_schemes(id) ON DELETE RESTRICT,
  identifier text NOT NULL,
  validated  boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT company_registrations_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=6)
);
CREATE UNIQUE INDEX company_registrations_scheme_identifier_active
  ON oikumenea.company_registrations (scheme_id, identifier) WHERE deleted_at IS NULL;
CREATE INDEX company_registrations_company_idx
  ON oikumenea.company_registrations (company_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_registrations_set_updated_at
  BEFORE UPDATE ON oikumenea.company_registrations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_registrations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registrations.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registrations.scheme_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_registrations.identifier IS 'pii:none';

-- company_industry_assignments — a company's economic-activity classifications (M:N), one primary +
-- secondaries.
CREATE TABLE oikumenea.company_industry_assignments (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,7),  -- company / link / has_industry
  company_id         uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  industry_class_id  uuid NOT NULL REFERENCES oikumenea.company_industry_classes(id) ON DELETE RESTRICT,
  is_primary         boolean NOT NULL DEFAULT false,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT company_industry_assignments_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=7)
);
CREATE UNIQUE INDEX company_industry_assignments_unique_active
  ON oikumenea.company_industry_assignments (company_id, industry_class_id) WHERE deleted_at IS NULL;
-- At most one primary industry per company among active rows.
CREATE UNIQUE INDEX company_industry_assignments_one_primary_active
  ON oikumenea.company_industry_assignments (company_id) WHERE is_primary AND deleted_at IS NULL;
CREATE TRIGGER company_industry_assignments_set_updated_at
  BEFORE UPDATE ON oikumenea.company_industry_assignments
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_industry_assignments.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_assignments.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_assignments.industry_class_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_industry_assignments.is_primary IS 'pii:none';

-- company_locations — a company's addresses, located via the shared M19 location_locations. `role`
-- distinguishes the registered office from operating sites / branches.
CREATE TABLE oikumenea.company_locations (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,8),  -- company / link / located_at
  company_id  uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  location_id uuid NOT NULL REFERENCES oikumenea.location_locations(id) ON DELETE RESTRICT,  -- M19
  role        text NOT NULL DEFAULT 'registered' CHECK (role IN ('registered','operating','branch')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT company_locations_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=8)
);
CREATE INDEX company_locations_company_idx
  ON oikumenea.company_locations (company_id) WHERE deleted_at IS NULL;
CREATE INDEX company_locations_location_idx ON oikumenea.company_locations (location_id);
CREATE TRIGGER company_locations_set_updated_at
  BEFORE UPDATE ON oikumenea.company_locations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_locations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_locations.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_locations.location_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_locations.role IS 'pii:none';

-- company_positions — a company-owned billet (CEO/director/employee line); vacant until an appointment
-- fills it (mirrors membership / education positions). `code` unique within the company.
CREATE TABLE oikumenea.company_positions (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,5),  -- company / object / company_position
  company_id uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE RESTRICT,
  code       text NOT NULL,
  title      text NOT NULL,                  -- default-locale title; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','abolished')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT company_positions_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=5)
);
CREATE UNIQUE INDEX company_positions_company_code_active
  ON oikumenea.company_positions (company_id, code) WHERE deleted_at IS NULL;
CREATE INDEX company_positions_company_idx
  ON oikumenea.company_positions (company_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE TRIGGER company_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.company_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_positions.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_positions.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_positions.title IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_positions.status IS 'pii:none';

-- ===================================================================================================
-- Position link: appointments (mirrors membership / education appointments). One holder per billet.
-- ===================================================================================================

-- company_appointments — a person holds (fills) a company position (link__holds_company_position).
-- One holder per position among active rows; reversible (end flips status + sets effective_to).
-- pii:basic (identifies a person's employment role); erased on person purge.
CREATE TABLE oikumenea.company_appointments (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,1),  -- company / link / holds_company_position
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  position_id    uuid NOT NULL REFERENCES oikumenea.company_positions(id) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT company_appointments_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX company_appointments_one_holder_idx
  ON oikumenea.company_appointments (position_id)
  WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX company_appointments_person_idx
  ON oikumenea.company_appointments (person_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE TRIGGER company_appointments_set_updated_at
  BEFORE UPDATE ON oikumenea.company_appointments
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_appointments.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_appointments.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.company_appointments.position_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_appointments.status IS 'pii:none';

-- ===================================================================================================
-- Equity / ownership links (the ownership/affiliation graph). Reified Links (D-Ontology).
-- Polymorphic holders (person|company) are (holder_kind, holder_id TEXT, no FK) — F-014.
-- ===================================================================================================

-- company_foundings — who FOUNDED a company (link__founded). The founder is a person OR a company.
-- holder_kind discriminates; holder_id is the founder RID (text, no FK — polymorphic). When the founder
-- is a person it is pii:basic and erased on person purge.
CREATE TABLE oikumenea.company_foundings (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,2),  -- company / link / founded
  company_id  uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,  -- the founded company
  holder_kind text NOT NULL CHECK (holder_kind IN ('person','company')),
  holder_id   text NOT NULL,                  -- founder RID (person or company); polymorphic, no FK
  founded_on  date,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT company_foundings_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
);
CREATE INDEX company_foundings_company_idx
  ON oikumenea.company_foundings (company_id) WHERE deleted_at IS NULL;
CREATE INDEX company_foundings_holder_idx
  ON oikumenea.company_foundings (holder_kind, holder_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_foundings_set_updated_at
  BEFORE UPDATE ON oikumenea.company_foundings
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_foundings.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_foundings.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_foundings.holder_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_foundings.holder_id IS 'pii:basic';

-- company_shareholdings — a holder OWNS_STAKE in a company (link__owns_stake). Polymorphic holder
-- (person|company); company-holder edges form the ownership DAG. stake_pct is the percentage held;
-- effective-dated. Person-holder rows are pii:basic and erased on person purge.
CREATE TABLE oikumenea.company_shareholdings (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,3),  -- company / link / owns_stake
  company_id     uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,  -- the issuer
  holder_kind    text NOT NULL CHECK (holder_kind IN ('person','company')),
  holder_id      text NOT NULL,               -- owner RID (person or company); polymorphic, no FK
  stake_pct      numeric(7,4) CHECK (stake_pct IS NULL OR (stake_pct >= 0 AND stake_pct <= 100)),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT company_shareholdings_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3)
);
CREATE INDEX company_shareholdings_company_idx
  ON oikumenea.company_shareholdings (company_id) WHERE deleted_at IS NULL;
CREATE INDEX company_shareholdings_holder_idx
  ON oikumenea.company_shareholdings (holder_kind, holder_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_shareholdings_set_updated_at
  BEFORE UPDATE ON oikumenea.company_shareholdings
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_shareholdings.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_shareholdings.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_shareholdings.holder_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_shareholdings.holder_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.company_shareholdings.stake_pct IS 'pii:none';

-- company_beneficiaries — a natural person is the ultimate beneficial owner of a company
-- (link__beneficiary_of, UBO). Always a person (real person_id FK, CASCADE). ultimate_pct is the
-- declared ultimate ownership; `declared` records whether it is registry-declared (vs computed).
-- pii:basic; erased on person purge.
CREATE TABLE oikumenea.company_beneficiaries (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,4),  -- company / link / beneficiary_of
  company_id   uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  ultimate_pct numeric(7,4) CHECK (ultimate_pct IS NULL OR (ultimate_pct >= 0 AND ultimate_pct <= 100)),
  declared     boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT company_beneficiaries_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=4)
);
CREATE INDEX company_beneficiaries_company_idx
  ON oikumenea.company_beneficiaries (company_id) WHERE deleted_at IS NULL;
CREATE INDEX company_beneficiaries_person_idx
  ON oikumenea.company_beneficiaries (person_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_beneficiaries_set_updated_at
  BEFORE UPDATE ON oikumenea.company_beneficiaries
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_beneficiaries.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_beneficiaries.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_beneficiaries.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.company_beneficiaries.ultimate_pct IS 'pii:none';

-- company_successions — M&A / reorganization lineage: predecessor SUCCEEDED_BY successor
-- (link__succeeded_by). Both ends are companies.
CREATE TABLE oikumenea.company_successions (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,5),  -- company / link / succeeded_by
  predecessor_id uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  successor_id   uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  kind           text NOT NULL DEFAULT 'reorganization'
                   CHECK (kind IN ('merger','reorganization','rename','acquisition','spinoff')),
  effective_on   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT company_successions_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=5),
  CONSTRAINT company_successions_distinct CHECK (predecessor_id <> successor_id)
);
CREATE INDEX company_successions_predecessor_idx
  ON oikumenea.company_successions (predecessor_id) WHERE deleted_at IS NULL;
CREATE INDEX company_successions_successor_idx
  ON oikumenea.company_successions (successor_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_successions_set_updated_at
  BEFORE UPDATE ON oikumenea.company_successions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_successions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_successions.predecessor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_successions.successor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_successions.kind IS 'pii:none';

-- company_branches — a non-independent sub-unit BRANCH_OF a parent company (link__branch_of). Both
-- ends are companies (a branch is itself registered as a company row).
CREATE TABLE oikumenea.company_branches (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,2,6),  -- company / link / branch_of
  branch_id  uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  parent_id  uuid NOT NULL REFERENCES oikumenea.company_companies(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT company_branches_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=6),
  CONSTRAINT company_branches_distinct CHECK (branch_id <> parent_id)
);
CREATE UNIQUE INDEX company_branches_unique_active
  ON oikumenea.company_branches (branch_id, parent_id) WHERE deleted_at IS NULL;
CREATE INDEX company_branches_parent_idx
  ON oikumenea.company_branches (parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_branches_set_updated_at
  BEFORE UPDATE ON oikumenea.company_branches
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_branches.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_branches.branch_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_branches.parent_id IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0022_company', applied_at = now() WHERE singleton;
