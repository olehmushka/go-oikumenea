-- 0020 education (M20; unified onto the tenant org-graph — M41 / D-UnifiedOrgGraph).
--
-- The education domain (docs/modules/education.md / D-Education): reference institutions (where a person
-- studied / taught), their internal structure + buildings, and the person bindings (enrollments,
-- dormitory stays, education positions).
--
-- M41 / D-UnifiedOrgGraph REVERSES the original D-Education choice of a dedicated education_institutions
-- table + education_units tree + education_unit_closure. An institution is now a tenant ORGANIZATION
-- (domain=`university`, pdp_scoped=false → instance-global reference, public reads, app-perm writes) with
-- an education_org_profiles SIDECAR (keyed by the org RID, no own RID — mirrors religion_org_profiles).
-- Its internal structure is tenant_units in the org's `structure` graph, and the transitive closure is
-- tenant_unit_closure — there is no second closure engine. Unit kinds are tenant_unit_kinds (the
-- `university` domain seeds campus/institute/faculty/department/chair). Buildings / groups / positions /
-- appointments and the person-binding link rows are still education-owned, but now point at tenant
-- organization / unit RIDs. Buildings/dorms reference the M19 location_locations by FK.
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the catalog reference rows (institution kinds, ISCED
-- degree levels) are seeded directly here. Org profiles / buildings / groups / positions and the
-- person-binding link rows are created through EducationService (which delegates org/unit structure to
-- the tenant service) at runtime.
--
-- RLS: the person-binding link tables (person_education_enrollments / person_dormitory_stays) are
-- holder-scoped (D-PersonReadScope) — like person_languages / person_relationships, they carry no unit
-- column and are EXEMPT from the app.readable_units backstop (D-RLSDefenseInDepth); no RLS is enabled.
-- The org-profile / building / group / position tables hang off instance-global reference orgs/units
-- (pdp_scoped=false; the tenant reach-RLS exempts reference units — D-UnifiedOrgGraph), so they carry no
-- RLS either; app-permission is the write gate.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id +
-- geo_countries), 0003 tenant (tenant_organizations + tenant_units + tenant_unit_kinds), 0005 person
-- (person_persons + 0014 person_sponsorships) and 0019 location (location_locations).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the `education` service (14) + its object/link/action types.
-- pkg/rid mirrors these and asserts equality at boot (kind<>3), so they are added in both places
-- together. NOTE (M41): institution / education_unit / unit_kind objects and the education_unit_parent_of
-- link are GONE — an institution is a tenant organization, a unit is a tenant unit, unit kinds are
-- tenant_unit_kinds, and the parent edge is a tenant graph edge.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (14, 'education');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- education objects
  (14,1,3,'building'),(14,1,4,'group'),
  (14,1,5,'education_position'),(14,1,6,'institution_kind'),(14,1,8,'degree_level'),
  -- education links
  (14,2,2,'studied_at'),
  (14,2,3,'resided_in_dormitory'),(14,2,4,'holds_education_position'),
  -- education Action RID (kind=3, excluded from the Go-mirror size check)
  (14,3,0,'action');

-- ===================================================================================================
-- Reference catalogs (D-Code / D-i18n): code + translatable name (default-locale here, other locales in
-- the localization store). Instance-admin-managed kinds; the ISCED degree-level scale is migration-seeded.
-- (Unit kinds are no longer here — they are tenant_unit_kinds under the `university` domain.)
-- ===================================================================================================

-- education_institution_kinds — kindergarten / school / lyceum / college / institute / university /
-- academy … (instance-admin catalog; mirrors location_location_types). Describes the institution org.
CREATE TABLE oikumenea.education_institution_kinds (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,6),  -- education / object / institution_kind
  code       text NOT NULL,
  name       text NOT NULL,                  -- default-locale display name; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT education_institution_kinds_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=6)
);
CREATE UNIQUE INDEX education_institution_kinds_code_active
  ON oikumenea.education_institution_kinds (code) WHERE deleted_at IS NULL;
CREATE TRIGGER education_institution_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.education_institution_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_institution_kinds.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_institution_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_institution_kinds.name IS 'pii:none';

INSERT INTO oikumenea.education_institution_kinds (code, name, sort_order) VALUES
  ('kindergarten','Kindergarten',10),
  ('school','School',20),
  ('lyceum','Lyceum',30),
  ('gymnasium','Gymnasium',40),
  ('vocational','Vocational school',50),
  ('college','College',60),
  ('institute','Institute',70),
  ('university','University',80),
  ('academy','Academy',90);

-- education_degree_levels — the ISCED 2011 levels 0–8 (UNESCO standard scale), migration-seeded
-- reference catalog. isced_level is the numeric ISCED code (used for ordering / cross-system queries).
CREATE TABLE oikumenea.education_degree_levels (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,8),  -- education / object / degree_level
  code        text NOT NULL,
  name        text NOT NULL,
  isced_level integer NOT NULL,              -- ISCED 2011 level 0..8
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order  integer,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT education_degree_levels_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=8)
);
CREATE UNIQUE INDEX education_degree_levels_code_active
  ON oikumenea.education_degree_levels (code) WHERE deleted_at IS NULL;
CREATE TRIGGER education_degree_levels_set_updated_at
  BEFORE UPDATE ON oikumenea.education_degree_levels
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_degree_levels.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_degree_levels.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_degree_levels.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_degree_levels.isced_level IS 'pii:none';

INSERT INTO oikumenea.education_degree_levels (code, name, isced_level, sort_order) VALUES
  ('isced-0','Early childhood education',0,0),
  ('isced-1','Primary education',1,1),
  ('isced-2','Lower secondary education',2,2),
  ('isced-3','Upper secondary education',3,3),
  ('isced-4','Post-secondary non-tertiary education',4,4),
  ('isced-5','Short-cycle tertiary education',5,5),
  ('isced-6','Bachelor or equivalent',6,6),
  ('isced-7','Master or equivalent',7,7),
  ('isced-8','Doctoral or equivalent',8,8);

-- ===================================================================================================
-- Institution profile sidecar (D-UnifiedOrgGraph): the education-specific attributes of a `university`-
-- domain tenant ORGANIZATION. Keyed by the org RID (an extension of the Organization object — no own
-- RID; mirrors religion_org_profiles). The org carries code / name / visibility; the profile carries the
-- institution kind, country, founding/closing dates, and the education lifecycle state.
-- ===================================================================================================
-- `institution_id` is the tenant organization RID (an `university`-domain org); it names the profile's
-- owner the way every other education table names it, so the education Go keeps one vocabulary.
CREATE TABLE oikumenea.education_org_profiles (
  institution_id uuid PRIMARY KEY REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  kind_id        uuid NOT NULL REFERENCES oikumenea.education_institution_kinds(id) ON DELETE RESTRICT,
  country_id     uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (international/online)
  founded_on     date,
  closed_on      date,
  state          text NOT NULL DEFAULT 'active' CHECK (state IN ('active','closed','merged')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE INDEX education_org_profiles_kind_idx ON oikumenea.education_org_profiles (kind_id) WHERE deleted_at IS NULL;
CREATE INDEX education_org_profiles_country_idx ON oikumenea.education_org_profiles (country_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_org_profiles_set_updated_at
  BEFORE UPDATE ON oikumenea.education_org_profiles
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_org_profiles.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_org_profiles.kind_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_org_profiles.country_id IS 'pii:none';

-- ===================================================================================================
-- Objects: buildings, groups — hang off the tenant organization / unit RIDs.
-- ===================================================================================================

-- education_buildings — a physical building of an institution (optionally a specific unit/campus),
-- located via the shared M19 location_locations (nullable — geocode later). `kind` distinguishes a
-- dormitory (the dorm-stay target) from academic/administrative/etc. buildings. The institution_id is a tenant org RID and unit_id a tenant unit RID
-- organization; unit_id → tenant unit (M41).
CREATE TABLE oikumenea.education_buildings (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,3),  -- education / object / building
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  unit_id        uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- nullable
  location_id    uuid REFERENCES oikumenea.location_locations(id) ON DELETE RESTRICT,  -- M19; nullable
  code           text NOT NULL,
  name           text NOT NULL,
  kind           text NOT NULL DEFAULT 'academic'
                   CHECK (kind IN ('academic','dormitory','administrative','library','sports','other')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_buildings_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE UNIQUE INDEX education_buildings_institution_code_active
  ON oikumenea.education_buildings (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_buildings_institution_idx
  ON oikumenea.education_buildings (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_buildings_location_idx ON oikumenea.education_buildings (location_id);
CREATE TRIGGER education_buildings_set_updated_at
  BEFORE UPDATE ON oikumenea.education_buildings
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_buildings.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.location_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_buildings.kind IS 'pii:none';

-- education_groups — a cohort (study group) under a tenant unit, with an admission year.
CREATE TABLE oikumenea.education_groups (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,4),  -- education / object / group
  unit_id        uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  code           text NOT NULL,
  name           text NOT NULL,
  admission_year integer,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','graduated','disbanded')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_groups_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);
CREATE UNIQUE INDEX education_groups_unit_code_active
  ON oikumenea.education_groups (unit_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_groups_unit_idx ON oikumenea.education_groups (unit_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_groups_set_updated_at
  BEFORE UPDATE ON oikumenea.education_groups
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_groups.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_groups.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_groups.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_groups.name IS 'pii:none';

-- ===================================================================================================
-- Positions & appointments ("like a military") — mirrors membership_positions / membership_memberships.
-- A position is an org/unit-owned billet that exists while vacant; an appointment is its one-holder,
-- effective-dated filling (reversible via status flip).
-- ===================================================================================================

-- education_positions — a billet owned by an institution org (rector) or one of its tenant units (dean,
-- head-of-chair, professor line). Vacant until an appointment fills it. `code` unique within the org.
CREATE TABLE oikumenea.education_positions (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,5),  -- education / object / education_position
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  unit_id        uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- NULL = institution-level
  code           text NOT NULL,
  title          text NOT NULL,                 -- default-locale title; translatable via the i18n store
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','abolished')),
  sort_order     integer,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_positions_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=5)
);
CREATE UNIQUE INDEX education_positions_institution_code_active
  ON oikumenea.education_positions (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_positions_institution_idx
  ON oikumenea.education_positions (institution_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX education_positions_unit_idx
  ON oikumenea.education_positions (unit_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE TRIGGER education_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.education_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_positions.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_positions.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_positions.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_positions.title IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_positions.status IS 'pii:none';

-- education_appointments — a person holds (fills) an education position (link__holds_education_position).
-- One holder per position among active rows; reversible (end flips status + sets effective_to). pii:basic
-- (it identifies a person's teaching/admin role); erased on person purge via the person purge sweep.
CREATE TABLE oikumenea.education_appointments (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,4),  -- education / link / holds_education_position
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  position_id    uuid NOT NULL REFERENCES oikumenea.education_positions(id) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_appointments_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=4)
);
-- One billet, one holder: a position has at most one ACTIVE appointment.
CREATE UNIQUE INDEX education_appointments_one_holder_idx
  ON oikumenea.education_appointments (position_id)
  WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX education_appointments_person_idx
  ON oikumenea.education_appointments (person_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE TRIGGER education_appointments_set_updated_at
  BEFORE UPDATE ON oikumenea.education_appointments
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_appointments.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_appointments.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.education_appointments.position_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_appointments.status IS 'pii:none';

-- ===================================================================================================
-- Person bindings — reified temporal Links (D-Ontology). Holder-scoped on read (D-PersonReadScope),
-- erased on person purge; carry the education service's RID per ontology-mapping.md.
-- ===================================================================================================

-- person_education_enrollments — a person STUDIED_AT an institution org (optionally a tenant unit + study
-- group), with an ISCED degree level, field/specialty, status, and the qualification awarded. Mirrors the
-- membership temporal link. pii:basic; CASCADE on person delete + purge-erased. The institution_id is a tenant org RID and unit_id a tenant unit RID
-- organization; unit_id → tenant unit (M41).
CREATE TABLE oikumenea.person_education_enrollments (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,2),  -- education / link / studied_at
  person_id       uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  institution_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  unit_id         uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,   -- nullable
  group_id        uuid REFERENCES oikumenea.education_groups(id) ON DELETE RESTRICT,  -- nullable
  degree_level_id uuid REFERENCES oikumenea.education_degree_levels(id) ON DELETE RESTRICT,  -- ISCED; nullable
  field_of_study  text,                          -- specialty / programme name
  status          text NOT NULL DEFAULT 'enrolled'
                    CHECK (status IN ('enrolled','graduated','withdrawn','expelled','on_leave')),
  qualification   text,                          -- the qualification/degree awarded (e.g. "MSc Computer Science")
  effective_from  date,
  effective_to    date,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  CONSTRAINT person_education_enrollments_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
);
CREATE INDEX person_education_enrollments_person_idx
  ON oikumenea.person_education_enrollments (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_education_enrollments_institution_idx
  ON oikumenea.person_education_enrollments (institution_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_education_enrollments_set_updated_at
  BEFORE UPDATE ON oikumenea.person_education_enrollments
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_education_enrollments.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_education_enrollments.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.institution_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.unit_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.group_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.degree_level_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.field_of_study IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.qualification IS 'pii:basic';

-- person_dormitory_stays — a person RESIDED_IN_DORMITORY: a dedicated stay (person ↔ dorm building,
-- room, period). Distinct from a generic person_residence (carries room/occupancy). pii:contact (it is
-- locator data); CASCADE on person delete + purge-erased.
CREATE TABLE oikumenea.person_dormitory_stays (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,3),  -- education / link / resided_in_dormitory
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  building_id    uuid NOT NULL REFERENCES oikumenea.education_buildings(id) ON DELETE RESTRICT,
  room           text,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_dormitory_stays_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3)
);
CREATE INDEX person_dormitory_stays_person_idx
  ON oikumenea.person_dormitory_stays (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_dormitory_stays_building_idx
  ON oikumenea.person_dormitory_stays (building_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_dormitory_stays_set_updated_at
  BEFORE UPDATE ON oikumenea.person_dormitory_stays
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_dormitory_stays.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_dormitory_stays.person_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_dormitory_stays.building_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_dormitory_stays.room IS 'pii:contact';

-- ===================================================================================================
-- Sponsorship education context (D-Education): extend M14 person_sponsorships with an OPTIONAL education
-- context (enrollment ref + role) — no new link type. The two columns are nullable and additive.
-- ===================================================================================================
ALTER TABLE oikumenea.person_sponsorships
  ADD COLUMN enrollment_id  uuid REFERENCES oikumenea.person_education_enrollments(id) ON DELETE SET NULL,
  ADD COLUMN education_role text CHECK (education_role IN ('professor','tutor','curator','advisor'));
COMMENT ON COLUMN oikumenea.person_sponsorships.enrollment_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.education_role IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0020_education', applied_at = now() WHERE singleton;
