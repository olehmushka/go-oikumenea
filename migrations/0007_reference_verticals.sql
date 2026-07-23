-- 0007_reference_verticals — merged domain migration (refactor: consolidated from 0018_language, 0019_location, 0020_education, 0021_education_reference, 0022_company).

-- ===== merged from 0018_language =====
-- 0018 language (M18).
--
-- Languages & writing systems (docs/modules/language.md / D-Languages). A faithful model of the
-- Glottolog genealogical forest (languoids: family → language → dialect, keyed by glottocode), their
-- ISO-15924 writing systems, and the language ties on person / unit / locale. Turns "language" into a
-- queryable, linkable dimension. The first NEW consumer of the M16 hermenea ingestion pipeline:
-- the ~26k-languoid Glottolog snapshot and the CLDR language→script links arrive over the generic
-- POST /import/{objectType} endpoint (language-scheme + language-scripts object-types). Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id + geo_countries) and on
-- person (0005) / tenant (0003) / localization (0002) for the link tables.
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the writing-system + script-type reference rows are
-- seeded directly here. The Glottolog languoids are NEVER seeded in a migration (~26k rows is the
-- D-DataIngestion import's job); the language_writing_systems M:N is import-loaded too (from CLDR).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the new `language` service (13) + its object/link types, plus
-- the language link-types added to existing services (person/tenant/i18n). pkg/rid mirrors these and
-- asserts equality at boot (kind<>3), so they are added in both places together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (13, 'language');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- language objects
  (13,1,1,'languoid'),(13,1,2,'writing_system'),(13,1,3,'script_type'),
  -- language links
  (13,2,1,'written_in'),
  -- language Action RID (kind=3, excluded from the Go-mirror size check)
  (13,3,0,'action'),
  -- cross-module language link types on existing services
  (6,2,8,'speaks'),          -- person → languoid (person_languages)
  (4,2,2,'unit_language'),    -- tenant unit → languoid (tenant_unit_languages)
  (2,2,1,'locale_language');  -- i18n locale → languoid (i18n_locale_languages)

-- ---------------------------------------------------------------------------------------------------
-- language_languoids — the recursive Glottolog forest (D-Languages). ONE table (not a group/language
-- split), faithful to Glottolog's uniform languoid model. RID `id` PK (language service 13); the
-- 8-char `code` (glottocode) is the universal, stable external spine (UNIQUE). `parent_id` is a
-- structural containment self-FK ("father" — a strict tree, the geo_places/rank_types pattern, NOT a
-- reified Link). `family_code` is the denormalized root family, DERIVED IN SQL via the closure on
-- import (the denormalized-FK-derived-in-SQL pattern). `iso639_3` is the OPTIONAL ISO 639-3 attribute
-- (UNIQUE; families/dialects have no ISO code, so it can never be the PK). `status` is Glottolog's
-- graded AES endangerment (replaces a naïve `living` boolean). Representative `latitude`/`longitude`
-- are plain numeric — M18 precedes the PostGIS Location (D-Location). Provenance + glottolog_version
-- are the per-row D-DataIngestion lineage.
CREATE TABLE oikumenea.language_languoids (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(13,1,1),  -- language / object / languoid
  code             char(8) NOT NULL UNIQUE,        -- glottocode (e.g. stan1293); the universal external reference
  level            text NOT NULL CHECK (level IN ('family','language','dialect')),
  name             text NOT NULL,                  -- default-locale display name; translatable via the i18n store
  parent_id        uuid REFERENCES oikumenea.language_languoids(id) ON DELETE RESTRICT,  -- Glottolog "father" (strict tree)
  family_code      char(8),                        -- denormalized root family glottocode (derived in SQL via the closure)
  iso639_3         char(3) UNIQUE,                 -- optional ISO 639-3 (NULL for families/dialects/unlisted)
  macroarea        text,                           -- Glottolog macroarea (e.g. Eurasia, Africa)
  latitude         double precision,               -- representative point (plain numeric; M18 precedes PostGIS)
  longitude        double precision,
  status           text NOT NULL DEFAULT 'not_endangered'
                     CHECK (status IN ('not_endangered','threatened','shifting','moribund','nearly_extinct','extinct')),
  glottolog_version text,                          -- the Glottolog edition this row came from (e.g. 5.3)
  source           text,                           -- importing dataset id (e.g. glottolog)
  source_version   text,                           -- the source edition (idempotency key)
  imported_at      timestamptz,                    -- when the import upsert last touched this row
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT language_languoids_rid_shape
    CHECK (oikumenea.rid_service(id)=13 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1),
  -- composite-unique so a level-constrained link (person_languages) can FK against (id, level).
  CONSTRAINT language_languoids_id_level_uq UNIQUE (id, level)
);
CREATE TRIGGER language_languoids_set_updated_at
  BEFORE UPDATE ON oikumenea.language_languoids
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE INDEX language_languoids_parent_idx  ON oikumenea.language_languoids (parent_id);
CREATE INDEX language_languoids_level_idx   ON oikumenea.language_languoids (level);
CREATE INDEX language_languoids_family_idx  ON oikumenea.language_languoids (family_code);

COMMENT ON COLUMN oikumenea.language_languoids.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.level IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.family_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.iso639_3 IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.macroarea IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.latitude IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.longitude IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoids.status IS 'pii:none';

-- Bootstrap the ~50 most-spoken languages (level='language') so the catalog is usable before any
-- import — mirrors the geo_countries seed (schema_bootstrap). `code` is the real Glottolog glottocode
-- (the import's UNIQUE upsert key), so a later language-scheme import UPDATEs these rows in place (no
-- duplicates) and enriches them (parent_id / family_code / source). Required columns + iso639_3:
-- status defaults to 'not_endangered'. `iso639_3` is the Glottolog individual-language code (e.g.
-- cmn/arb/pes/swh, not the macrolanguage code), so it matches what the import would write (zero churn)
-- and lets the ukr/eng locale→languoid reconciliation resolve before any import. Names are the
-- Glottolog default-locale names (so the first import causes no churn).
INSERT INTO oikumenea.language_languoids (code, level, name, iso639_3) VALUES
  ('stan1293','language','English','eng'),
  ('mand1415','language','Mandarin Chinese','cmn'),
  ('hind1269','language','Hindi','hin'),
  ('stan1288','language','Spanish','spa'),
  ('stan1318','language','Standard Arabic','arb'),
  ('stan1290','language','French','fra'),
  ('beng1280','language','Bengali','ben'),
  ('port1283','language','Portuguese','por'),
  ('russ1263','language','Russian','rus'),
  ('urdu1245','language','Urdu','urd'),
  ('indo1316','language','Standard Indonesian','ind'),
  ('stan1295','language','German','deu'),
  ('nucl1643','language','Japanese','jpn'),
  ('swah1253','language','Swahili','swh'),
  ('mara1378','language','Marathi','mar'),
  ('telu1262','language','Telugu','tel'),
  ('nucl1301','language','Turkish','tur'),
  ('yuec1235','language','Yue Chinese','yue'),
  ('tami1289','language','Tamil','tam'),
  ('viet1252','language','Vietnamese','vie'),
  ('wuch1236','language','Wu Chinese','wuu'),
  ('kore1280','language','Korean','kor'),
  ('west2369','language','Western Farsi','pes'),
  ('haus1257','language','Hausa','hau'),
  ('egyp1253','language','Egyptian Arabic','arz'),
  ('java1254','language','Javanese','jav'),
  ('ital1282','language','Italian','ita'),
  ('west2386','language','Western Panjabi','pnb'),
  ('nucl1305','language','Kannada','kan'),
  ('guja1252','language','Gujarati','guj'),
  ('thai1261','language','Thai','tha'),
  ('amha1245','language','Amharic','amh'),
  ('panj1256','language','Eastern Panjabi','pan'),
  ('bhoj1244','language','Bhojpuri','bho'),
  ('nort2690','language','Northern Uzbek','uzn'),
  ('mala1464','language','Malayalam','mal'),
  ('nort2646','language','Northern Pashto','pbu'),
  ('nucl1310','language','Burmese','mya'),
  ('poli1260','language','Polish','pol'),
  ('ukra1253','language','Ukrainian','ukr'),
  ('yoru1245','language','Yoruba','yor'),
  ('oriy1255','language','Odia','ory'),
  ('cebu1242','language','Cebuano','ceb'),
  ('nepa1254','language','Nepali','npi'),
  ('dutc1256','language','Dutch','nld'),
  ('roma1327','language','Romanian','ron'),
  ('mait1250','language','Maithili','mai'),
  ('sinh1246','language','Sinhala','sin'),
  ('liuj1238','language','Liujiang Zhuang','zlj'),
  ('cent1989','language','Central Khmer','khm');

-- language_languoid_closure — derived transitive closure (mirrors tenant_unit_closure). A maintained
-- materialized relation, not a source of truth (ontology-mapping.md 4.3) → composite key, no RID.
-- Includes the reflexive (u,u,0) row, so "all languages under Indo-European" is one lookup and the
-- root family is the ancestor with parent_id IS NULL. Rebuilt in SQL on every language-scheme import.
CREATE TABLE oikumenea.language_languoid_closure (
  ancestor_id   uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE CASCADE,
  descendant_id uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE CASCADE,
  depth         integer NOT NULL,
  PRIMARY KEY (ancestor_id, descendant_id)
);
CREATE INDEX language_languoid_closure_descendant_idx
  ON oikumenea.language_languoid_closure (descendant_id);
COMMENT ON COLUMN oikumenea.language_languoid_closure.ancestor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoid_closure.descendant_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoid_closure.depth IS 'pii:none';

-- language_languoid_countries — plain M:N tie languoid → geo_countries (from CLDF Country_IDs,
-- D-Geo). A bare join (no own attributes/identity) → composite key, no RID (ontology-mapping.md: not
-- a reified Link). Populated on the language-scheme import.
CREATE TABLE oikumenea.language_languoid_countries (
  languoid_id uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE CASCADE,
  country_id  uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  PRIMARY KEY (languoid_id, country_id)
);
CREATE INDEX language_languoid_countries_country_idx
  ON oikumenea.language_languoid_countries (country_id);
COMMENT ON COLUMN oikumenea.language_languoid_countries.languoid_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_languoid_countries.country_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Writing systems (D-Languages). The script-type catalog + the ISO-15924 script registry are small and
-- stable → seeded directly here. The language→script M:N (`language_writing_systems`) is NOT seeded:
-- it is import-loaded from CLDR (the language-scripts object-type), since neither Glottolog nor
-- ISO-15924 carries the mapping.
-- ---------------------------------------------------------------------------------------------------

-- writing_system_script_types — closed catalog of the structural script families (D-Languages).
CREATE TABLE oikumenea.writing_system_script_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(13,1,3),  -- language / object / script_type
  code       text NOT NULL UNIQUE,             -- logographic | syllabary | alphabet | abjad | abugida | featural
  name       text NOT NULL,                    -- default-locale display name; translatable via the i18n store
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT writing_system_script_types_rid_shape
    CHECK (oikumenea.rid_service(id)=13 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE TRIGGER writing_system_script_types_set_updated_at
  BEFORE UPDATE ON oikumenea.writing_system_script_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.writing_system_script_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_system_script_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_system_script_types.name IS 'pii:none';

INSERT INTO oikumenea.writing_system_script_types (code, name, sort_order) VALUES
  ('logographic','Logographic',10),
  ('syllabary','Syllabary',20),
  ('alphabet','Alphabet',30),
  ('abjad','Abjad',40),
  ('abugida','Abugida',50),
  ('featural','Featural',60);

-- writing_systems — the ISO-15924 script registry (D-Languages). RID `id` PK; the 4-letter ISO-15924
-- `code` is the stable external lookup key (UNIQUE). `script_type` classifies it (FK to the catalog).
-- Seeded with the living-language scripts CLDR references; instance-admin-extensible (the import skips
-- a language→script link whose script code is not yet seeded, rather than failing).
CREATE TABLE oikumenea.writing_systems (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(13,1,2),  -- language / object / writing_system
  code        char(4) NOT NULL UNIQUE,         -- ISO 15924 (e.g. Latn, Cyrl, Hani)
  name        text NOT NULL,                   -- default-locale display name; translatable via the i18n store
  script_type text REFERENCES oikumenea.writing_system_script_types(code) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT writing_systems_rid_shape
    CHECK (oikumenea.rid_service(id)=13 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE TRIGGER writing_systems_set_updated_at
  BEFORE UPDATE ON oikumenea.writing_systems
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.writing_systems.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_systems.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_systems.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_systems.script_type IS 'pii:none';

INSERT INTO oikumenea.writing_systems (code, name, script_type) VALUES
  ('Latn','Latin','alphabet'),
  ('Cyrl','Cyrillic','alphabet'),
  ('Grek','Greek','alphabet'),
  ('Armn','Armenian','alphabet'),
  ('Geor','Georgian','alphabet'),
  ('Glag','Glagolitic','alphabet'),
  ('Runr','Runic','alphabet'),
  ('Ogam','Ogham','alphabet'),
  ('Goth','Gothic','alphabet'),
  ('Nkoo','N’Ko','alphabet'),
  ('Adlm','Adlam','alphabet'),
  ('Arab','Arabic','abjad'),
  ('Hebr','Hebrew','abjad'),
  ('Syrc','Syriac','abjad'),
  ('Samr','Samaritan','abjad'),
  ('Mand','Mandaic','abjad'),
  ('Phnx','Phoenician','abjad'),
  ('Thaa','Thaana','abjad'),
  ('Deva','Devanagari','abugida'),
  ('Beng','Bengali','abugida'),
  ('Guru','Gurmukhi','abugida'),
  ('Gujr','Gujarati','abugida'),
  ('Orya','Oriya','abugida'),
  ('Taml','Tamil','abugida'),
  ('Telu','Telugu','abugida'),
  ('Knda','Kannada','abugida'),
  ('Mlym','Malayalam','abugida'),
  ('Sinh','Sinhala','abugida'),
  ('Thai','Thai','abugida'),
  ('Laoo','Lao','abugida'),
  ('Tibt','Tibetan','abugida'),
  ('Mymr','Myanmar','abugida'),
  ('Khmr','Khmer','abugida'),
  ('Ethi','Ethiopic','abugida'),
  ('Cans','Unified Canadian Aboriginal Syllabics','abugida'),
  ('Tfng','Tifinagh','abugida'),
  ('Java','Javanese','abugida'),
  ('Bali','Balinese','abugida'),
  ('Hira','Hiragana','syllabary'),
  ('Kana','Katakana','syllabary'),
  ('Bopo','Bopomofo','syllabary'),
  ('Yiii','Yi','syllabary'),
  ('Cher','Cherokee','syllabary'),
  ('Vaii','Vai','syllabary'),
  ('Hani','Han','logographic'),
  ('Hans','Han (Simplified)','logographic'),
  ('Hant','Han (Traditional)','logographic'),
  ('Jpan','Japanese','logographic'),
  ('Egyp','Egyptian hieroglyphs','logographic'),
  ('Xsux','Cuneiform','logographic'),
  ('Hang','Hangul','featural'),
  ('Kore','Korean','featural');

-- language_writing_systems — reified M:N link languoid ↔ writing_system (link__written_in,
-- D-Languages). Carries `is_primary` (the language's main script), so it has its own identity → RID
-- PK. Import-loaded from CLDR (language-scripts object-type); idempotency is per (languoid, writing
-- system) pair.
CREATE TABLE oikumenea.language_writing_systems (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(13,2,1),  -- language / link / written_in
  languoid_id       uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE CASCADE,
  writing_system_id uuid NOT NULL REFERENCES oikumenea.writing_systems(id) ON DELETE RESTRICT,
  is_primary        boolean NOT NULL DEFAULT false,
  source            text,
  source_version    text,
  imported_at       timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT language_writing_systems_rid_shape
    CHECK (oikumenea.rid_service(id)=13 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1),
  CONSTRAINT language_writing_systems_unique UNIQUE (languoid_id, writing_system_id)
);
CREATE TRIGGER language_writing_systems_set_updated_at
  BEFORE UPDATE ON oikumenea.language_writing_systems
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE INDEX language_writing_systems_ws_idx
  ON oikumenea.language_writing_systems (writing_system_id);
COMMENT ON COLUMN oikumenea.language_writing_systems.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_writing_systems.languoid_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_writing_systems.writing_system_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.language_writing_systems.is_primary IS 'pii:none';

-- Bootstrap each bootstrapped language's primary writing system (is_primary=true) by joining the
-- glottocode→ISO-15924 pairs below to the seeded languoids and writing_systems. Mirrors the
-- language-scripts import (per (languoid, writing system) pair), so a later import is a no-op (or an
-- in-place enrichment); ON CONFLICT keeps the seed idempotent if re-run.
INSERT INTO oikumenea.language_writing_systems (languoid_id, writing_system_id, is_primary)
SELECT g.id, w.id, true
FROM (VALUES
  ('stan1293','Latn'),('mand1415','Hans'),('hind1269','Deva'),('stan1288','Latn'),
  ('stan1318','Arab'),('stan1290','Latn'),('beng1280','Beng'),('port1283','Latn'),
  ('russ1263','Cyrl'),('urdu1245','Arab'),('indo1316','Latn'),('stan1295','Latn'),
  ('nucl1643','Jpan'),('swah1253','Latn'),('mara1378','Deva'),('telu1262','Telu'),
  ('nucl1301','Latn'),('yuec1235','Hant'),('tami1289','Taml'),('viet1252','Latn'),
  ('wuch1236','Hans'),('kore1280','Kore'),('west2369','Arab'),('haus1257','Latn'),
  ('egyp1253','Arab'),('java1254','Latn'),('ital1282','Latn'),('west2386','Arab'),
  ('nucl1305','Knda'),('guja1252','Gujr'),('thai1261','Thai'),('amha1245','Ethi'),
  ('panj1256','Guru'),('bhoj1244','Deva'),('nort2690','Latn'),('mala1464','Mlym'),
  ('nort2646','Arab'),('nucl1310','Mymr'),('poli1260','Latn'),('ukra1253','Cyrl'),
  ('yoru1245','Latn'),('oriy1255','Orya'),('cebu1242','Latn'),('nepa1254','Deva'),
  ('dutc1256','Latn'),('roma1327','Latn'),('mait1250','Deva'),('sinh1246','Sinh'),
  ('liuj1238','Latn'),('cent1989','Khmr')
) AS m(glottocode, script)
JOIN oikumenea.language_languoids g ON g.code = m.glottocode
JOIN oikumenea.writing_systems  w ON w.code = m.script
ON CONFLICT (languoid_id, writing_system_id) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- Cross-module language links. Each lives here (one milestone, one migration) but carries its OWNING
-- service's RID (person 6 / tenant 4 / i18n 2), per ontology-mapping.md.
-- ---------------------------------------------------------------------------------------------------

-- person_languages — a person SPEAKS a language (link__speaks, D-Languages). `language_id` is
-- constrained to a level='language' languoid via the composite FK (language_id, language_level) →
-- language_languoids(id, level) with language_level pinned to 'language'. `cefr_level` is the optional
-- CEFR proficiency; `is_native` flags a mother tongue. pii:basic; CASCADE on person delete and erased
-- on purge (person.md purge sweep).
CREATE TABLE oikumenea.person_languages (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,8),  -- person / link / speaks
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  language_id    uuid NOT NULL,
  language_level text NOT NULL DEFAULT 'language' CHECK (language_level = 'language'),
  cefr_level     text CHECK (cefr_level IN ('A1','A2','B1','B2','C1','C2')),
  is_native      boolean NOT NULL DEFAULT false,
  -- native validity (D-Temporal, R-31): the interval this person speaks it; NULL valid_to = active.
  valid_from     timestamptz NOT NULL DEFAULT now(),
  valid_to       timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_languages_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=8),
  CONSTRAINT person_languages_is_language_fk
    FOREIGN KEY (language_id, language_level)
    REFERENCES oikumenea.language_languoids(id, level) ON DELETE RESTRICT
);
CREATE TRIGGER person_languages_set_updated_at
  BEFORE UPDATE ON oikumenea.person_languages
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE UNIQUE INDEX person_languages_active_idx
  ON oikumenea.person_languages (person_id, language_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.person_languages.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_languages.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_languages.language_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_languages.language_level IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_languages.cefr_level IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_languages.is_native IS 'pii:basic';

-- tenant_unit_languages — a unit's official/working language (link__unit_language, D-Languages).
CREATE TABLE oikumenea.tenant_unit_languages (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,2,2),  -- tenant / link / unit_language
  unit_id     uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE CASCADE,
  language_id uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE RESTRICT,
  is_official boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT tenant_unit_languages_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2)
);
CREATE TRIGGER tenant_unit_languages_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_unit_languages
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE UNIQUE INDEX tenant_unit_languages_active_idx
  ON oikumenea.tenant_unit_languages (unit_id, language_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_unit_languages.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_languages.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_languages.language_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_languages.is_official IS 'pii:none';

-- i18n_locale_languages — a supported locale's canonical language (link__locale_language,
-- D-Languages). Ties the ISO-639-3 UI locale to its Glottolog languoid (distinct concepts: a locale is
-- a supported UI language, a languoid is the genealogical node).
CREATE TABLE oikumenea.i18n_locale_languages (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(2,2,1),  -- i18n / link / locale_language
  locale      text NOT NULL REFERENCES oikumenea.i18n_locales(code) ON DELETE CASCADE,
  language_id uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE RESTRICT,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT i18n_locale_languages_rid_shape
    CHECK (oikumenea.rid_service(id)=2 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1),
  CONSTRAINT i18n_locale_languages_locale_unique UNIQUE (locale)
);
CREATE TRIGGER i18n_locale_languages_set_updated_at
  BEFORE UPDATE ON oikumenea.i18n_locale_languages
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.i18n_locale_languages.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locale_languages.locale IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locale_languages.language_id IS 'pii:none';

-- Bootstrap the locale→languoid link for the seeded locales (ukr/eng, migration 0002) by matching
-- their ISO-639-3 code to the bootstrapped languoid carrying that iso639_3 (ukr→ukra1253,
-- eng→stan1293). Same shape as ReconcileLocaleLanguages, so a later language-scheme import is a no-op.


-- Localized names for the writing-system catalog (D-i18n: all locales in every response). eng = the
-- catalog's English `name` made explicit (the default locale is ukr, so the `name` column is not
-- implicitly the eng label); ukr = hand-authored. entity_id is the RID `id` (the transport assembles
-- writing-system names keyed by id under entity_type 'writing_system'). Untranslated scripts keep
-- their English name in both locales (graceful fallback). Idempotent: re-running is a no-op.




-- pinax origin marker (D-Pinax, M45): 'seeded' = managed by the bundled `languages` / `writing-systems`
-- presets, 'operator' = created via the admin API. Migration-seeded rows are marked seeded; the runtime
-- default 'operator' applies to API-created rows. (The Glottolog forest is loaded via the import path,
-- whose handler stamps origin='seeded' — D-Pinax; this marks the migration-seeded skeleton + scripts.)
ALTER TABLE oikumenea.language_languoids ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
ALTER TABLE oikumenea.writing_systems    ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
UPDATE oikumenea.language_languoids SET origin = 'seeded';
UPDATE oikumenea.writing_systems    SET origin = 'seeded';
COMMENT ON COLUMN oikumenea.language_languoids.origin IS 'pii:none';
COMMENT ON COLUMN oikumenea.writing_systems.origin IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0019_location =====
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

-- ===== merged from 0020_education =====
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

-- ===== merged from 0021_education_reference =====
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
-- M41 / D-UnifiedOrgGraph: `institution_id` columns here reference tenant_organizations (the institution
-- org) and `owning_unit_id`/`unit_id` reference tenant_units — the column names are kept so the
-- reference-layer Go is unchanged, but an institution is a tenant org and a unit is a tenant unit.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0003 tenant (tenant_organizations / tenant_units),
-- 0020 education (education_degree_levels / education_programs / person_education_enrollments) and 0005 person.

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
  institution_id     uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  owning_unit_id     uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- nullable
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
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  owning_unit_id uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- nullable
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
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  centre_id      uuid REFERENCES oikumenea.education_research_centres(id) ON DELETE RESTRICT,  -- nullable
  unit_id        uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,             -- nullable
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
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  institution_id uuid REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- nullable
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
  institution_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  institution_id     uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  institution_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  institution_id uuid REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- nullable
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
  institution_id  uuid REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- set iff institution
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

-- ===== merged from 0022_company =====
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

-- M41 / D-UnifiedOrgGraph: a company IS a `company`-domain tenant organization (no own `company` object
-- RID); company-specific attributes live in the company_org_profiles sidecar (PK = the tenant org RID).
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- company objects
  (15,1,2,'legal_form'),(15,1,3,'registration_scheme'),(15,1,4,'industry_class'),
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

-- company_org_profiles — the company sidecar (M41 / D-UnifiedOrgGraph). A company IS a `company`-domain
-- tenant organization (its stable `code` and translatable registered name = the org's code + name); this
-- table carries the company-specific attributes keyed by the tenant org RID (PK = institution pattern,
-- mirrors education_org_profiles / religion_org_profiles — no own RID). short_name is a plain trading
-- name; ownership_category and legal_form are orthogonal axes (a private LLC vs a state-owned JSC).
CREATE TABLE oikumenea.company_org_profiles (
  company_id         uuid PRIMARY KEY REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
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
  deleted_at         timestamptz
);
CREATE INDEX company_org_profiles_country_idx
  ON oikumenea.company_org_profiles (country_id) WHERE deleted_at IS NULL;
CREATE INDEX company_org_profiles_legal_form_idx
  ON oikumenea.company_org_profiles (legal_form_id) WHERE deleted_at IS NULL;
CREATE TRIGGER company_org_profiles_set_updated_at
  BEFORE UPDATE ON oikumenea.company_org_profiles
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.company_org_profiles.company_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.short_name IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.legal_form_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.ownership_category IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.country_id IS 'pii:none';

-- company_registrations — a company's per-scheme registration identifier (mirrors document
-- personal_codes). identifier is the registered number; validated records whether it matched the
-- scheme's validator_pattern. Unique per (scheme, identifier) among active rows.
CREATE TABLE oikumenea.company_registrations (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(15,1,6),  -- company / object / registration
  company_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
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
  company_id         uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  industry_class_id  uuid NOT NULL REFERENCES oikumenea.company_industry_classes(id) ON DELETE RESTRICT,
  is_primary         boolean NOT NULL DEFAULT false,
  -- native validity (D-Temporal, R-31): the interval this industry classification holds; NULL = active.
  valid_from         timestamptz NOT NULL DEFAULT now(),
  valid_to           timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
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
  company_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  location_id uuid NOT NULL REFERENCES oikumenea.location_locations(id) ON DELETE RESTRICT,  -- M19
  role        text NOT NULL DEFAULT 'registered' CHECK (role IN ('registered','operating','branch')),
  -- native validity (D-Temporal, R-31): the interval this location holds; NULL valid_to = active.
  valid_from  timestamptz NOT NULL DEFAULT now(),
  valid_to    timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
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
  company_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
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
  company_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,  -- the founded company
  holder_kind text NOT NULL CHECK (holder_kind IN ('person','company')),
  holder_id   text NOT NULL,                  -- founder RID (person or company); polymorphic, no FK
  founded_on  date,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT company_foundings_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2),
  -- The polymorphic founder end carries no FK, so this shape CHECK is its only integrity on the id:
  -- a 'person' founder must be a person object RID (6,1,1); a 'company' founder must be a company-domain
  -- tenant ORGANIZATION RID (4,1,6) — M41/D-UnifiedOrgGraph. The ::uuid cast also rejects a malformed
  -- id. Existence stays app-enforced (R-32, review-2026-09).
  CONSTRAINT company_foundings_holder_shape CHECK (
    (holder_kind <> 'person'  OR (oikumenea.rid_service(holder_id::uuid)=6 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=1)) AND
    (holder_kind <> 'company' OR (oikumenea.rid_service(holder_id::uuid)=4 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=6))
  )
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
  company_id     uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,  -- the issuer
  holder_kind    text NOT NULL CHECK (holder_kind IN ('person','company')),
  holder_id      text NOT NULL,               -- owner RID (person or company); polymorphic, no FK
  stake_pct      numeric(7,4) CHECK (stake_pct IS NULL OR (stake_pct >= 0 AND stake_pct <= 100)),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT company_shareholdings_rid_shape
    CHECK (oikumenea.rid_service(id)=15 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3),
  -- The polymorphic holder end carries no FK, so this shape CHECK is its only integrity on the id:
  -- a 'person' holder must be a person object RID (6,1,1); a 'company' holder must be a company-domain
  -- tenant ORGANIZATION RID (4,1,6) — M41/D-UnifiedOrgGraph. The ::uuid cast also rejects a malformed
  -- id. Existence stays app-enforced (R-32, review-2026-09).
  CONSTRAINT company_shareholdings_holder_shape CHECK (
    (holder_kind <> 'person'  OR (oikumenea.rid_service(holder_id::uuid)=6 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=1)) AND
    (holder_kind <> 'company' OR (oikumenea.rid_service(holder_id::uuid)=4 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=6))
  )
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
  company_id   uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  ultimate_pct numeric(7,4) CHECK (ultimate_pct IS NULL OR (ultimate_pct >= 0 AND ultimate_pct <= 100)),
  declared     boolean NOT NULL DEFAULT true,
  -- native validity (D-Temporal, R-31): the interval this benefit holds; NULL valid_to = active.
  valid_from   timestamptz NOT NULL DEFAULT now(),
  valid_to     timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
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
  predecessor_id uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  successor_id   uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  kind           text NOT NULL DEFAULT 'reorganization'
                   CHECK (kind IN ('merger','reorganization','rename','acquisition','spinoff')),
  effective_on   date,
  -- native validity (D-Temporal, R-31): the interval this succession holds; NULL valid_to = active.
  valid_from     timestamptz NOT NULL DEFAULT now(),
  valid_to       timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
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
  branch_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  parent_id  uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE CASCADE,
  -- native validity (D-Temporal, R-31): the interval this branch-of holds; NULL valid_to = active.
  valid_from timestamptz NOT NULL DEFAULT now(),
  valid_to   timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
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

UPDATE oikumenea.schema_version SET revision = '0007_reference_verticals', applied_at = now() WHERE singleton;
