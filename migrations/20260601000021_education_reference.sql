-- 0021 education reference layer (M20 extension).
--
-- Adopts the reference-grade slice of docs/university_ontology.md into the existing education domain
-- (docs/modules/education.md / D-Education), WITHOUT becoming an operational university SIS. Added as
-- EXTERNAL reference data + person↔reference directory links, mirroring the 0020 shapes exactly:
--   * Reference objects (programs, courses, curriculum versions, research centres/groups, grants,
--     publications, governance bodies, policies, qualifications, scholarships, accreditation events) —
--     RID PK via new_id(14,1,N), code + translatable name, lifecycle, soft-delete. Like education
--     institutions, these are external reference entities, never scoped against tenant_units → no RLS.
--   * Reified junction links (curriculum items, course prerequisites) — own RID, RESTRICT endpoints.
--   * Person-binding links (publication authorship, research-group membership, grant holding,
--     governance membership, awarded qualification, scholarship award) — person_id CASCADE, effective-
--     dated, pii:basic, holder-scoped (D-PersonReadScope) and ERASED on person purge (like
--     person_education_enrollments). Carry no unit column → EXEMPT from the readable_units backstop; no
--     RLS is enabled on them.
--
-- DELIBERATELY OUT OF SCOPE (kept external-reference, not operational SIS): academic terms / calendars,
-- course sections, section-level enrollment with grades, assessments, GPA / grading — and the
-- Person→Student/StaffMember subtype split and bi-temporal validity from the source doc (the repo keeps
-- a single person + directory attributes + effective-dated links + soft-delete; D-Person / D-Ontology).
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so this migration adds nothing to seed beyond the RID
-- registry rows; all reference rows are created through EducationService at runtime. The `diploma`
-- document type is seeded at boot in internal/document/module.go (ON CONFLICT), not here.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0020 education (education_institutions /
-- education_units / education_degree_levels / person_education_enrollments) and 0005 person.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): extend the education service (14) with the reference-layer
-- object/link types. pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- education reference objects
  (14,1,9,'program'),(14,1,10,'course'),(14,1,11,'curriculum_version'),
  (14,1,12,'research_centre'),(14,1,13,'research_group'),(14,1,14,'grant'),
  (14,1,15,'publication'),(14,1,16,'governance_body'),(14,1,17,'policy'),
  (14,1,18,'qualification'),(14,1,19,'scholarship'),(14,1,20,'accreditation_event'),
  -- education reference links
  (14,2,5,'curriculum_item'),(14,2,6,'course_prerequisite'),(14,2,7,'authored_publication'),
  (14,2,8,'member_of_research_group'),(14,2,9,'holds_grant'),(14,2,10,'member_of_governance_body'),
  (14,2,11,'awarded_qualification'),(14,2,12,'awarded_scholarship');

-- ===================================================================================================
-- Curriculum & courses (reference catalog). Program → owning unit; Course → owning unit; a versioned
-- CurriculumVersion snapshots a program's requirements; CurriculumItem (reified link) places a Course in
-- a version; CoursePrerequisite (reified self-link) chains courses. No terms/sections/grades.
-- ===================================================================================================

-- education_programs — a degree/diploma/certificate program offered by an institution (optionally a unit).
CREATE TABLE oikumenea.education_programs (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,9),  -- education / object / program
  institution_id     uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  owning_unit_id     uuid REFERENCES oikumenea.education_units(id) ON DELETE RESTRICT,  -- nullable
  degree_level_id    uuid REFERENCES oikumenea.education_degree_levels(id) ON DELETE RESTRICT,  -- ISCED; nullable
  code               text NOT NULL,
  name               text NOT NULL,              -- default-locale display name; translatable via the i18n store
  mode               text NOT NULL DEFAULT 'full_time'
                       CHECK (mode IN ('full_time','part_time','online','hybrid','intensive')),
  duration_years     numeric(4,1),
  credit_hours_total integer,
  state              text NOT NULL DEFAULT 'active' CHECK (state IN ('active','retired')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT education_programs_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=9)
);
CREATE UNIQUE INDEX education_programs_institution_code_active
  ON oikumenea.education_programs (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_programs_institution_idx
  ON oikumenea.education_programs (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_programs_unit_idx ON oikumenea.education_programs (owning_unit_id);
CREATE TRIGGER education_programs_set_updated_at
  BEFORE UPDATE ON oikumenea.education_programs
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_programs.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_programs.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_programs.owning_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_programs.degree_level_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_programs.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_programs.name IS 'pii:none';

-- education_courses — a unit of study / module / subject owned by an institution (optionally a unit).
CREATE TABLE oikumenea.education_courses (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,10),  -- education / object / course
  institution_id uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  owning_unit_id uuid REFERENCES oikumenea.education_units(id) ON DELETE RESTRICT,  -- nullable
  code           text NOT NULL,
  title          text NOT NULL,                 -- default-locale title; translatable via the i18n store
  credit_hours   integer,
  level          integer,                       -- 100..900 (undergraduate to doctoral); free numeric
  description    text,
  delivery_mode  text NOT NULL DEFAULT 'in_person'
                   CHECK (delivery_mode IN ('in_person','online','hybrid','lab','clinic','field_work','studio')),
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_courses_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=10)
);
CREATE UNIQUE INDEX education_courses_institution_code_active
  ON oikumenea.education_courses (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_courses_institution_idx
  ON oikumenea.education_courses (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_courses_unit_idx ON oikumenea.education_courses (owning_unit_id);
CREATE TRIGGER education_courses_set_updated_at
  BEFORE UPDATE ON oikumenea.education_courses
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_courses.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_courses.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_courses.owning_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_courses.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_courses.title IS 'pii:none';

-- education_curriculum_versions — a versioned snapshot of a program's requirements.
CREATE TABLE oikumenea.education_curriculum_versions (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,11),  -- education / object / curriculum_version
  program_id     uuid NOT NULL REFERENCES oikumenea.education_programs(id) ON DELETE RESTRICT,
  version_code   text NOT NULL,                  -- e.g. 2024-v1
  effective_from date,
  effective_to   date,
  status         text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','retired')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_curriculum_versions_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=11)
);
CREATE UNIQUE INDEX education_curriculum_versions_program_code_active
  ON oikumenea.education_curriculum_versions (program_id, version_code) WHERE deleted_at IS NULL;
CREATE INDEX education_curriculum_versions_program_idx
  ON oikumenea.education_curriculum_versions (program_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_curriculum_versions_set_updated_at
  BEFORE UPDATE ON oikumenea.education_curriculum_versions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_curriculum_versions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_curriculum_versions.program_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_curriculum_versions.version_code IS 'pii:none';

-- education_curriculum_items — reified link (curriculum_item): a Course placed in a CurriculumVersion
-- with required/elective + credit/year metadata.
CREATE TABLE oikumenea.education_curriculum_items (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,5),  -- education / link / curriculum_item
  version_id        uuid NOT NULL REFERENCES oikumenea.education_curriculum_versions(id) ON DELETE RESTRICT,
  course_id         uuid NOT NULL REFERENCES oikumenea.education_courses(id) ON DELETE RESTRICT,
  is_required       boolean NOT NULL DEFAULT true,
  year_of_study     integer,                     -- 1-based
  credit_allocation integer,
  semester_slot     integer,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT education_curriculum_items_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=5)
);
CREATE UNIQUE INDEX education_curriculum_items_version_course_active
  ON oikumenea.education_curriculum_items (version_id, course_id) WHERE deleted_at IS NULL;
CREATE INDEX education_curriculum_items_course_idx ON oikumenea.education_curriculum_items (course_id);
CREATE TRIGGER education_curriculum_items_set_updated_at
  BEFORE UPDATE ON oikumenea.education_curriculum_items
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_curriculum_items.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_curriculum_items.version_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_curriculum_items.course_id IS 'pii:none';

-- education_course_prerequisites — reified self-link (course_prerequisite): course requires another.
CREATE TABLE oikumenea.education_course_prerequisites (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,6),  -- education / link / course_prerequisite
  course_id        uuid NOT NULL REFERENCES oikumenea.education_courses(id) ON DELETE RESTRICT,
  required_course_id uuid NOT NULL REFERENCES oikumenea.education_courses(id) ON DELETE RESTRICT,
  kind             text NOT NULL DEFAULT 'required'
                     CHECK (kind IN ('required','recommended','corequisite')),
  min_grade        text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT education_course_prerequisites_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=6),
  CONSTRAINT education_course_prerequisites_not_self CHECK (course_id <> required_course_id)
);
CREATE UNIQUE INDEX education_course_prerequisites_pair_active
  ON oikumenea.education_course_prerequisites (course_id, required_course_id) WHERE deleted_at IS NULL;
CREATE INDEX education_course_prerequisites_required_idx
  ON oikumenea.education_course_prerequisites (required_course_id);
CREATE TRIGGER education_course_prerequisites_set_updated_at
  BEFORE UPDATE ON oikumenea.education_course_prerequisites
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_course_prerequisites.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_course_prerequisites.course_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_course_prerequisites.required_course_id IS 'pii:none';

-- ===================================================================================================
-- Research (reference). Centres/groups are reference orgs of an institution; grants and publications
-- are reference outputs; people connect via the person-binding links further below.
-- ===================================================================================================

-- education_research_centres — a research centre / institute / lab of an institution.
CREATE TABLE oikumenea.education_research_centres (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,12),  -- education / object / research_centre
  institution_id uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  code           text NOT NULL,
  name           text NOT NULL,
  kind           text NOT NULL DEFAULT 'centre'
                   CHECK (kind IN ('centre','institute','lab','cluster','hub')),
  funding_source text,
  founded_on     date,
  dissolved_on   date,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','dissolved')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_research_centres_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=12)
);
CREATE UNIQUE INDEX education_research_centres_institution_code_active
  ON oikumenea.education_research_centres (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_research_centres_institution_idx
  ON oikumenea.education_research_centres (institution_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_research_centres_set_updated_at
  BEFORE UPDATE ON oikumenea.education_research_centres
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_research_centres.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_centres.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_centres.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_centres.name IS 'pii:none';

-- education_research_groups — a smaller research cluster under a centre and/or unit of an institution.
CREATE TABLE oikumenea.education_research_groups (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,13),  -- education / object / research_group
  institution_id uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  centre_id      uuid REFERENCES oikumenea.education_research_centres(id) ON DELETE RESTRICT,  -- nullable
  unit_id        uuid REFERENCES oikumenea.education_units(id) ON DELETE RESTRICT,             -- nullable
  code           text NOT NULL,
  name           text NOT NULL,
  focus_area     text,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disbanded')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_research_groups_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=13)
);
CREATE UNIQUE INDEX education_research_groups_institution_code_active
  ON oikumenea.education_research_groups (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_research_groups_centre_idx ON oikumenea.education_research_groups (centre_id);
CREATE INDEX education_research_groups_unit_idx ON oikumenea.education_research_groups (unit_id);
CREATE TRIGGER education_research_groups_set_updated_at
  BEFORE UPDATE ON oikumenea.education_research_groups
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_research_groups.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_groups.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_groups.centre_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_groups.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_groups.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_research_groups.name IS 'pii:none';

-- education_grants — a research/operational funding grant held by an institution.
CREATE TABLE oikumenea.education_grants (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,14),  -- education / object / grant
  institution_id uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  code           text NOT NULL,
  title          text NOT NULL,
  funder         text,
  funder_ref     text,
  amount         numeric(18,2),
  currency       text CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),  -- ISO 4217
  start_on       date,
  end_on         date,
  status         text NOT NULL DEFAULT 'awarded'
                   CHECK (status IN ('applied','awarded','active','no_cost_extension','closed','rejected')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_grants_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=14)
);
CREATE UNIQUE INDEX education_grants_institution_code_active
  ON oikumenea.education_grants (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_grants_institution_idx
  ON oikumenea.education_grants (institution_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_grants_set_updated_at
  BEFORE UPDATE ON oikumenea.education_grants
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_grants.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_grants.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_grants.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_grants.title IS 'pii:none';

-- education_publications — an academic publication. institution_id is nullable (a work may not tie to
-- one of the referenced institutions); `code` is the stable app key (globally unique among active).
CREATE TABLE oikumenea.education_publications (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,15),  -- education / object / publication
  institution_id uuid REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,  -- nullable
  code           text NOT NULL,
  title          text NOT NULL,
  kind           text NOT NULL DEFAULT 'journal_article'
                   CHECK (kind IN ('journal_article','conference_paper','book','book_chapter','report',
                                   'thesis','dissertation','preprint','patent')),
  doi            text,
  venue          text,                           -- journal / conference name
  published_on   date,
  open_access    boolean NOT NULL DEFAULT false,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_publications_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=15)
);
CREATE UNIQUE INDEX education_publications_code_active
  ON oikumenea.education_publications (code) WHERE deleted_at IS NULL;
CREATE INDEX education_publications_institution_idx ON oikumenea.education_publications (institution_id);
CREATE TRIGGER education_publications_set_updated_at
  BEFORE UPDATE ON oikumenea.education_publications
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_publications.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_publications.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_publications.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_publications.title IS 'pii:none';

-- ===================================================================================================
-- Governance & policy (reference).
-- ===================================================================================================

-- education_governance_bodies — board / senate / council / committee / advisory of an institution.
CREATE TABLE oikumenea.education_governance_bodies (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,16),  -- education / object / governance_body
  institution_id uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  code           text NOT NULL,
  name           text NOT NULL,
  kind           text NOT NULL DEFAULT 'committee'
                   CHECK (kind IN ('board','senate','council','committee','advisory','working_group')),
  mandate        text,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','dissolved')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_governance_bodies_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=16)
);
CREATE UNIQUE INDEX education_governance_bodies_institution_code_active
  ON oikumenea.education_governance_bodies (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_governance_bodies_institution_idx
  ON oikumenea.education_governance_bodies (institution_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_governance_bodies_set_updated_at
  BEFORE UPDATE ON oikumenea.education_governance_bodies
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_governance_bodies.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_governance_bodies.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_governance_bodies.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_governance_bodies.name IS 'pii:none';

-- education_policies — an institutional rule/regulation, optionally approved by a governance body and
-- superseding an earlier policy (self-FK SET NULL).
CREATE TABLE oikumenea.education_policies (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,17),  -- education / object / policy
  institution_id     uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  governance_body_id uuid REFERENCES oikumenea.education_governance_bodies(id) ON DELETE RESTRICT,  -- nullable
  supersedes_id      uuid REFERENCES oikumenea.education_policies(id) ON DELETE SET NULL,           -- nullable
  code               text NOT NULL,
  title              text NOT NULL,
  kind               text NOT NULL DEFAULT 'academic'
                       CHECK (kind IN ('academic','financial','hr','safety','research','student_conduct',
                                       'privacy','admissions')),
  effective_on       date,
  expiry_on          date,
  document_url       text,
  status             text NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','active','superseded','archived')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT education_policies_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=17)
);
CREATE UNIQUE INDEX education_policies_institution_code_active
  ON oikumenea.education_policies (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_policies_institution_idx
  ON oikumenea.education_policies (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_policies_body_idx ON oikumenea.education_policies (governance_body_id);
CREATE TRIGGER education_policies_set_updated_at
  BEFORE UPDATE ON oikumenea.education_policies
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_policies.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_policies.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_policies.governance_body_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_policies.supersedes_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_policies.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_policies.title IS 'pii:none';

-- ===================================================================================================
-- Credentials & accreditation (reference). Qualification is a credential TYPE (NQF/EQF/ISCED framework);
-- the per-person award is person_education_qualifications below. AccreditationEvent is a review cycle
-- against an institution or a program.
-- ===================================================================================================

-- education_qualifications — a formally awardable qualification (degree) classification.
CREATE TABLE oikumenea.education_qualifications (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,18),  -- education / object / qualification
  institution_id  uuid NOT NULL REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,
  program_id      uuid REFERENCES oikumenea.education_programs(id) ON DELETE RESTRICT,        -- nullable
  degree_level_id uuid REFERENCES oikumenea.education_degree_levels(id) ON DELETE RESTRICT,   -- ISCED; nullable
  code            text NOT NULL,
  name            text NOT NULL,                 -- e.g. "Bachelor of Science in Computer Science"
  framework_code  text,                          -- NQF | EQF | ISCED | NZQF | AQF …
  framework_level text,                          -- the level code within that framework
  awarding_body   text,                          -- may differ from the institution
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  CONSTRAINT education_qualifications_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=18)
);
CREATE UNIQUE INDEX education_qualifications_institution_code_active
  ON oikumenea.education_qualifications (institution_id, code) WHERE deleted_at IS NULL;
CREATE INDEX education_qualifications_institution_idx
  ON oikumenea.education_qualifications (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_qualifications_program_idx ON oikumenea.education_qualifications (program_id);
CREATE TRIGGER education_qualifications_set_updated_at
  BEFORE UPDATE ON oikumenea.education_qualifications
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_qualifications.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_qualifications.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_qualifications.program_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_qualifications.degree_level_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_qualifications.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_qualifications.name IS 'pii:none';

-- education_scholarships — a financial award scheme. institution_id nullable (external schemes); `code`
-- is the stable app key (globally unique among active).
CREATE TABLE oikumenea.education_scholarships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,19),  -- education / object / scholarship
  institution_id uuid REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,  -- nullable
  code           text NOT NULL,
  name           text NOT NULL,
  kind           text NOT NULL DEFAULT 'merit'
                   CHECK (kind IN ('merit','need_based','athletic','research','government','external','bursary')),
  amount         numeric(18,2),
  currency       text CHECK (currency IS NULL OR currency ~ '^[A-Z]{3}$'),  -- ISO 4217
  frequency      text NOT NULL DEFAULT 'annual'
                   CHECK (frequency IN ('once_off','annual','per_semester','monthly')),
  renewable      boolean NOT NULL DEFAULT false,
  conditions     text,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT education_scholarships_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=19)
);
CREATE UNIQUE INDEX education_scholarships_code_active
  ON oikumenea.education_scholarships (code) WHERE deleted_at IS NULL;
CREATE INDEX education_scholarships_institution_idx ON oikumenea.education_scholarships (institution_id);
CREATE TRIGGER education_scholarships_set_updated_at
  BEFORE UPDATE ON oikumenea.education_scholarships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_scholarships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_scholarships.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_scholarships.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_scholarships.name IS 'pii:none';

-- education_accreditation_events — an accreditation review cycle against an institution OR a program
-- (exactly one set, matching entity_kind). Event-like: no code uniqueness.
CREATE TABLE oikumenea.education_accreditation_events (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,1,20),  -- education / object / accreditation_event
  entity_kind     text NOT NULL CHECK (entity_kind IN ('institution','program')),
  institution_id  uuid REFERENCES oikumenea.education_institutions(id) ON DELETE RESTRICT,  -- set iff institution
  program_id      uuid REFERENCES oikumenea.education_programs(id) ON DELETE RESTRICT,      -- set iff program
  body            text,                          -- accrediting organization name
  body_country_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,           -- nullable
  outcome         text NOT NULL DEFAULT 'pending'
                    CHECK (outcome IN ('granted','renewed','conditional','withdrawn','pending','deferred')),
  review_on       date,
  effective_from  date,
  effective_to    date,
  notes           text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  CONSTRAINT education_accreditation_events_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=20),
  CONSTRAINT education_accreditation_events_target CHECK (
    (entity_kind = 'institution' AND institution_id IS NOT NULL AND program_id IS NULL) OR
    (entity_kind = 'program'     AND program_id     IS NOT NULL AND institution_id IS NULL)
  )
);
CREATE INDEX education_accreditation_events_institution_idx
  ON oikumenea.education_accreditation_events (institution_id) WHERE deleted_at IS NULL;
CREATE INDEX education_accreditation_events_program_idx
  ON oikumenea.education_accreditation_events (program_id) WHERE deleted_at IS NULL;
CREATE TRIGGER education_accreditation_events_set_updated_at
  BEFORE UPDATE ON oikumenea.education_accreditation_events
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.education_accreditation_events.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_accreditation_events.entity_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_accreditation_events.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_accreditation_events.program_id IS 'pii:none';

-- ===================================================================================================
-- Person bindings — reified temporal Links (D-Ontology). Holder-scoped on read (D-PersonReadScope),
-- ERASED on person purge (person.repository sweep); carry the education service's RID. No RLS (no unit).
-- ===================================================================================================

-- person_publication_authorships — a person AUTHORED a publication.
CREATE TABLE oikumenea.person_publication_authorships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,7),  -- education / link / authored_publication
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  publication_id uuid NOT NULL REFERENCES oikumenea.education_publications(id) ON DELETE RESTRICT,
  author_order   integer,                        -- 1-based position
  corresponding  boolean NOT NULL DEFAULT false,
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_publication_authorships_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=7)
);
CREATE UNIQUE INDEX person_publication_authorships_pair_active
  ON oikumenea.person_publication_authorships (person_id, publication_id) WHERE deleted_at IS NULL;
CREATE INDEX person_publication_authorships_person_idx
  ON oikumenea.person_publication_authorships (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_publication_authorships_publication_idx
  ON oikumenea.person_publication_authorships (publication_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_publication_authorships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_publication_authorships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_publication_authorships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_publication_authorships.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_publication_authorships.publication_id IS 'pii:basic';

-- person_research_memberships — a person is a MEMBER_OF_RESEARCH_GROUP.
CREATE TABLE oikumenea.person_research_memberships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,8),  -- education / link / member_of_research_group
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  group_id       uuid NOT NULL REFERENCES oikumenea.education_research_groups(id) ON DELETE RESTRICT,
  role           text,                           -- e.g. lead / member / affiliate
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_research_memberships_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=8)
);
CREATE INDEX person_research_memberships_person_idx
  ON oikumenea.person_research_memberships (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_research_memberships_group_idx
  ON oikumenea.person_research_memberships (group_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_research_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_research_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_research_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_research_memberships.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_research_memberships.group_id IS 'pii:basic';

-- person_grant_holdings — a person HOLDS_GRANT (PI / co-investigator / researcher).
CREATE TABLE oikumenea.person_grant_holdings (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,9),  -- education / link / holds_grant
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  grant_id       uuid NOT NULL REFERENCES oikumenea.education_grants(id) ON DELETE RESTRICT,
  role           text NOT NULL DEFAULT 'pi'
                   CHECK (role IN ('pi','co_investigator','researcher','administrator')),
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_grant_holdings_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=9)
);
CREATE INDEX person_grant_holdings_person_idx
  ON oikumenea.person_grant_holdings (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_grant_holdings_grant_idx
  ON oikumenea.person_grant_holdings (grant_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_grant_holdings_set_updated_at
  BEFORE UPDATE ON oikumenea.person_grant_holdings
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_grant_holdings.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_grant_holdings.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_grant_holdings.grant_id IS 'pii:basic';

-- person_governance_memberships — a person is a MEMBER_OF_GOVERNANCE_BODY.
CREATE TABLE oikumenea.person_governance_memberships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,10),  -- education / link / member_of_governance_body
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  body_id        uuid NOT NULL REFERENCES oikumenea.education_governance_bodies(id) ON DELETE RESTRICT,
  role_in_body   text,                           -- e.g. Chair / Secretary / Member
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_governance_memberships_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=10)
);
CREATE INDEX person_governance_memberships_person_idx
  ON oikumenea.person_governance_memberships (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_governance_memberships_body_idx
  ON oikumenea.person_governance_memberships (body_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_governance_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_governance_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_governance_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_governance_memberships.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_governance_memberships.body_id IS 'pii:basic';

-- person_education_qualifications — a person was AWARDED_QUALIFICATION (the diploma award). Optionally
-- ties to the enrollment it concluded (SET NULL on enrollment erase so a purge never orphans this row's
-- FK — though the whole row is purge-erased anyway).
CREATE TABLE oikumenea.person_education_qualifications (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,11),  -- education / link / awarded_qualification
  person_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  qualification_id uuid NOT NULL REFERENCES oikumenea.education_qualifications(id) ON DELETE RESTRICT,
  enrollment_id    uuid REFERENCES oikumenea.person_education_enrollments(id) ON DELETE SET NULL,  -- nullable
  awarded_on       date,
  with_distinction boolean NOT NULL DEFAULT false,
  gpa              numeric(4,3),
  status           text NOT NULL DEFAULT 'awarded' CHECK (status IN ('awarded','revoked')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT person_education_qualifications_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=11)
);
CREATE INDEX person_education_qualifications_person_idx
  ON oikumenea.person_education_qualifications (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_education_qualifications_qualification_idx
  ON oikumenea.person_education_qualifications (qualification_id) WHERE deleted_at IS NULL;
CREATE INDEX person_education_qualifications_enrollment_idx
  ON oikumenea.person_education_qualifications (enrollment_id);
CREATE TRIGGER person_education_qualifications_set_updated_at
  BEFORE UPDATE ON oikumenea.person_education_qualifications
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_education_qualifications.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_education_qualifications.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_qualifications.qualification_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_qualifications.enrollment_id IS 'pii:basic';

-- person_scholarship_awards — a person was AWARDED_SCHOLARSHIP.
CREATE TABLE oikumenea.person_scholarship_awards (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(14,2,12),  -- education / link / awarded_scholarship
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  scholarship_id uuid NOT NULL REFERENCES oikumenea.education_scholarships(id) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active','suspended','terminated','completed')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_scholarship_awards_rid_shape
    CHECK (oikumenea.rid_service(id)=14 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=12)
);
CREATE INDEX person_scholarship_awards_person_idx
  ON oikumenea.person_scholarship_awards (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_scholarship_awards_scholarship_idx
  ON oikumenea.person_scholarship_awards (scholarship_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_scholarship_awards_set_updated_at
  BEFORE UPDATE ON oikumenea.person_scholarship_awards
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_scholarship_awards.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_scholarship_awards.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_scholarship_awards.scholarship_id IS 'pii:basic';

-- ===================================================================================================
-- Enrollment ⇄ program link + student number (D-Education, catalog+person-link). Additive nullable
-- columns on the 0020 person_education_enrollments (which program a person studied; their student no.).
-- ===================================================================================================
ALTER TABLE oikumenea.person_education_enrollments
  ADD COLUMN program_id     uuid REFERENCES oikumenea.education_programs(id) ON DELETE SET NULL,
  ADD COLUMN student_number text;
COMMENT ON COLUMN oikumenea.person_education_enrollments.program_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_education_enrollments.student_number IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0021_education_reference', applied_at = now() WHERE singleton;
