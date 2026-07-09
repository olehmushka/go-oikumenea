-- 0024 religion clergy (M23 — D-ClergyCredential): clergy grades & credentials.
--
-- The clergy slice of the religion vertical (docs/modules/religion.md / D-ClergyCredential). A person's
-- ordination / investiture / recognition is a PUBLIC directory fact (never an authorization input —
-- parallel to D-Rank). It is NOT the linear `rank` scheme: ordination is per-tradition, often non-linear,
-- and sacramental/indelible where applicable, so it lives here as a per-tradition ordered catalog + a
-- reified Link.
--
-- Binding design rule (D-Religion): NO faith's vocabulary is hard-coded. Grade categories, grades and
-- office types are CATALOG ROWS keyed to a `religion_taxa` node (the retired `tradition_family_id` is now
-- a `tradition_taxon_id` FK), never a CHECK enum. The only CHECK enums here are fixed lifecycle statuses.
--
-- Offices themselves are NOT a new table: a clergy office (pastor of a parish, imam of a mosque) is a
-- membership Position owned by the org unit, typed by religion_office_types, with authority via an
-- authorization role assignment. religion_office_types is seeded here for that future use.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id), 0003 tenant (tenant_units),
-- 0005 person (person_persons), 0011 (the RLS app role) and 0023 religion (religion_taxa).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): new religion object/link types. pkg/rid mirrors these and
-- asserts equality at boot (kind<>3), so they are added in both places together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (16,1,7,'grade_category'),
  (16,1,8,'clergy_grade'),
  (16,1,9,'office_type'),
  (16,2,2,'clergy_credential');

-- ===================================================================================================
-- Catalogs (D-Code / D-i18n): code + translatable name. Keyed to a religion_taxa node (tradition), or
-- generic (NULL). Instance-admin-managed.
-- ===================================================================================================

-- religion_grade_categories — a per-tradition grouping of grades (generic; replaces a fixed major/minor
-- enum). e.g. Christianity → major_orders / minor_orders.
CREATE TABLE oikumenea.religion_grade_categories (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,7),  -- religion / object / grade_category
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  code              text NOT NULL,
  name              text NOT NULL,
  ordinal           integer,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order        integer,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT religion_grade_categories_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=7)
);
CREATE UNIQUE INDEX religion_grade_categories_code_active
  ON oikumenea.religion_grade_categories (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_grade_categories_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_grade_categories
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_grade_categories.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_grade_categories.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_grade_categories.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_grade_categories.name IS 'pii:none';

-- religion_clergy_grades — an ordered, per-tradition catalog (bishop/presbyter/deacon; imam/mufti/sheikh;
-- rabbi/cantor; bhikkhu/lama; pujari/swami). `ordinal` orders ONLY within a tradition — there is NO
-- cross-tradition comparator (DS-43 parked).
CREATE TABLE oikumenea.religion_clergy_grades (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,8),  -- religion / object / clergy_grade
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  grade_category_id  uuid NOT NULL REFERENCES oikumenea.religion_grade_categories(id) ON DELETE RESTRICT,
  code               text NOT NULL,
  name               text NOT NULL,
  ordinal            integer NOT NULL,  -- seniority within the tradition (no cross-tradition meaning)
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order         integer,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT religion_clergy_grades_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=8)
);
CREATE UNIQUE INDEX religion_clergy_grades_code_active
  ON oikumenea.religion_clergy_grades (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE INDEX religion_clergy_grades_category_idx
  ON oikumenea.religion_clergy_grades (grade_category_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_clergy_grades_tradition_idx
  ON oikumenea.religion_clergy_grades (tradition_taxon_id) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_clergy_grades_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_clergy_grades
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_clergy_grades.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_grades.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_grades.grade_category_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_grades.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_grades.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_grades.ordinal IS 'pii:none';

-- religion_office_types — catalog naming clergy offices (filled later as membership Positions).
CREATE TABLE oikumenea.religion_office_types (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,9),  -- religion / object / office_type
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  code               text NOT NULL,
  name               text NOT NULL,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order         integer,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT religion_office_types_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=9)
);
CREATE UNIQUE INDEX religion_office_types_code_active
  ON oikumenea.religion_office_types (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_office_types_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_office_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_office_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_office_types.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_office_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_office_types.name IS 'pii:none';

-- ===================================================================================================
-- religion_clergy_credentials — the reified Link link__clergy_credential (Person → ClergyGrade within a
-- tradition/organization body). INDELIBLE where sacramental: revocation/laicization is a status flip,
-- never a hard delete. A person may hold several (concurrent/successive grades, multiple traditions).
-- Unit-scoped via org_unit_id → carries the RLS backstop (below), like religion_org_classifications.
-- ===================================================================================================
CREATE TABLE oikumenea.religion_clergy_credentials (
  id                     uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,2,2),  -- religion / link / clergy_credential
  person_id              uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  clergy_grade_id        uuid NOT NULL REFERENCES oikumenea.religion_clergy_grades(id) ON DELETE RESTRICT,
  org_unit_id            uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  granted_on             date,
  conferred_by_person_id uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  status                 text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','revoked')),
  effective_from         timestamptz NOT NULL DEFAULT now(),
  effective_to           timestamptz,
  source                 text,
  confidence             text,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  deleted_at             timestamptz,
  CONSTRAINT religion_clergy_credentials_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
);
CREATE INDEX religion_clergy_credentials_person_idx
  ON oikumenea.religion_clergy_credentials (person_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_clergy_credentials_unit_idx
  ON oikumenea.religion_clergy_credentials (org_unit_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_clergy_credentials_grade_idx
  ON oikumenea.religion_clergy_credentials (clergy_grade_id) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_clergy_credentials_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_clergy_credentials
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.clergy_grade_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.org_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.granted_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_clergy_credentials.conferred_by_person_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Curated seed: per-tradition grade categories, grades and office types, keyed by root-religion taxon
-- code (christianity/islam/judaism/buddhism/hinduism — all seeded by 0023). Resolved by JOIN on code.
-- ---------------------------------------------------------------------------------------------------

-- Grade categories.
INSERT INTO oikumenea.religion_grade_categories (tradition_taxon_id, code, name, ordinal, sort_order)
SELECT t.id, v.code, v.name, v.ord, v.ord
FROM (VALUES
  ('christianity','major_orders','Major orders',10),
  ('christianity','minor_orders','Minor orders',20),
  ('islam','religious_leadership','Religious leadership',10),
  ('judaism','clergy','Clergy',10),
  ('buddhism','monastic','Monastic',10),
  ('hinduism','priestly','Priestly',10)
) AS v(tradition_code, code, name, ord)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL;

-- Grades (resolve tradition taxon + grade category by code).
INSERT INTO oikumenea.religion_clergy_grades (tradition_taxon_id, grade_category_id, code, name, ordinal, sort_order)
SELECT t.id, gc.id, v.code, v.name, v.ord, v.ord
FROM (VALUES
  ('christianity','major_orders','bishop','Bishop',10),
  ('christianity','major_orders','presbyter','Presbyter / Priest',20),
  ('christianity','major_orders','deacon','Deacon',30),
  ('christianity','minor_orders','subdeacon','Subdeacon',40),
  ('christianity','minor_orders','reader','Reader',50),
  ('islam','religious_leadership','mufti','Mufti',10),
  ('islam','religious_leadership','imam','Imam',20),
  ('islam','religious_leadership','sheikh','Sheikh',30),
  ('judaism','clergy','rabbi','Rabbi',10),
  ('judaism','clergy','cantor','Cantor (Hazzan)',20),
  ('buddhism','monastic','bhikkhu','Bhikkhu',10),
  ('buddhism','monastic','lama','Lama',20),
  ('hinduism','priestly','pujari','Pujari',10),
  ('hinduism','priestly','swami','Swami',20)
) AS v(tradition_code, cat_code, code, name, ord)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL
JOIN oikumenea.religion_grade_categories gc ON gc.code = v.cat_code AND gc.tradition_taxon_id = t.id AND gc.deleted_at IS NULL;

-- Office types (some generic — tradition NULL).
INSERT INTO oikumenea.religion_office_types (tradition_taxon_id, code, name, sort_order)
SELECT t.id, v.code, v.name, v.so
FROM (VALUES
  ('christianity','pastor','Pastor',10),
  ('christianity','rector','Rector',20),
  ('christianity','chaplain','Chaplain',30),
  ('christianity','abbot','Abbot',40),
  ('islam','imam_of_mosque','Imam of mosque',10),
  ('judaism','head_rabbi','Head rabbi',10)
) AS v(tradition_code, code, name, so)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL;

INSERT INTO oikumenea.religion_office_types (tradition_taxon_id, code, name, sort_order) VALUES
  (NULL,'head_priest','Head priest',100);

-- i18n: clergy-grade ("Ступені духовенства") + grade-category names in every enabled locale. eng is the
-- seed name column; ukr/spa/por are curated. The default-locale (ukr) row overrides the English name
-- column that LabelsByID otherwise assigns to the default locale (localization/service.go).
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', 'eng', c.name
FROM oikumenea.religion_grade_categories c
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('major_orders','ukr','Вищі свячення'),               ('major_orders','spa','Órdenes mayores'),        ('major_orders','por','Ordens maiores'),
  ('minor_orders','ukr','Нижчі свячення'),              ('minor_orders','spa','Órdenes menores'),        ('minor_orders','por','Ordens menores'),
  ('religious_leadership','ukr','Релігійне провідництво'),('religious_leadership','spa','Liderazgo religioso'),('religious_leadership','por','Liderança religiosa'),
  ('clergy','ukr','Духовенство'),                       ('clergy','spa','Clero'),                        ('clergy','por','Clero'),
  ('monastic','ukr','Чернецтво'),                       ('monastic','spa','Monástico'),                  ('monastic','por','Monástico'),
  ('priestly','ukr','Жрецтво'),                         ('priestly','spa','Sacerdotal'),                 ('priestly','por','Sacerdotal')
) AS v(code, locale, text)
JOIN oikumenea.religion_grade_categories c ON c.code = v.code AND c.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', 'eng', g.name
FROM oikumenea.religion_clergy_grades g
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('bishop','ukr','Єпископ'),           ('bishop','spa','Obispo'),                ('bishop','por','Bispo'),
  ('presbyter','ukr','Пресвітер / священник'),('presbyter','spa','Presbítero / sacerdote'),('presbyter','por','Presbítero / sacerdote'),
  ('deacon','ukr','Диякон'),            ('deacon','spa','Diácono'),               ('deacon','por','Diácono'),
  ('subdeacon','ukr','Іподиякон'),      ('subdeacon','spa','Subdiácono'),         ('subdeacon','por','Subdiácono'),
  ('reader','ukr','Читець'),            ('reader','spa','Lector'),                ('reader','por','Leitor'),
  ('mufti','ukr','Муфтій'),             ('mufti','spa','Muftí'),                  ('mufti','por','Mufti'),
  ('imam','ukr','Імам'),                ('imam','spa','Imán'),                    ('imam','por','Imã'),
  ('sheikh','ukr','Шейх'),              ('sheikh','spa','Jeque'),                 ('sheikh','por','Xeique'),
  ('rabbi','ukr','Рабин'),              ('rabbi','spa','Rabino'),                 ('rabbi','por','Rabino'),
  ('cantor','ukr','Кантор (хаззан)'),   ('cantor','spa','Cantor (jazán)'),        ('cantor','por','Cantor (hazã)'),
  ('bhikkhu','ukr','Бгіккху'),          ('bhikkhu','spa','Bhikkhu'),              ('bhikkhu','por','Bhikkhu'),
  ('lama','ukr','Лама'),                ('lama','spa','Lama'),                    ('lama','por','Lama'),
  ('pujari','ukr','Пуджарі'),           ('pujari','spa','Pujari'),                ('pujari','por','Pujari'),
  ('swami','ukr','Свамі'),              ('swami','spa','Suami'),                  ('swami','por','Swami')
) AS v(code, locale, text)
JOIN oikumenea.religion_clergy_grades g ON g.code = v.code AND g.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ===================================================================================================
-- RLS backstop (D-RLSDefenseInDepth) on religion_clergy_credentials (unit-scoped via org_unit_id).
-- Mirrors religion_org_classifications. The app-layer PDP over the canonical graph remains AUTHORITATIVE;
-- the catalogs (categories/grades/office types) are reference data and carry NO RLS.
-- ===================================================================================================
ALTER TABLE oikumenea.religion_clergy_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.religion_clergy_credentials FORCE ROW LEVEL SECURITY;
CREATE POLICY religion_clergy_credentials_reach ON oikumenea.religion_clergy_credentials
  USING (oikumenea.authz_unit_in_reach(org_unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(org_unit_id, true));

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0024_religion_clergy', applied_at = now() WHERE singleton;
