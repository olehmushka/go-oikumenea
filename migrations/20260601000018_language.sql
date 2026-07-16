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
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT loc.code, x.id
FROM oikumenea.i18n_locales loc
JOIN oikumenea.language_languoids x ON x.iso639_3 = loc.code
ON CONFLICT (locale) DO UPDATE SET language_id = EXCLUDED.language_id, updated_at = now();

-- Localized names for the writing-system catalog (D-i18n: all locales in every response). eng = the
-- catalog's English `name` made explicit (the default locale is ukr, so the `name` column is not
-- implicitly the eng label); ukr = hand-authored. entity_id is the RID `id` (the transport assembles
-- writing-system names keyed by id under entity_type 'writing_system'). Untranslated scripts keep
-- their English name in both locales (graceful fallback). Idempotent: re-running is a no-op.
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'writing_system', w.id::text, 'name', 'eng', w.name
FROM oikumenea.writing_systems w
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'writing_system', w.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('Latn','Латиниця'),
  ('Cyrl','Кирилиця'),
  ('Grek','Грецьке письмо'),
  ('Armn','Вірменське письмо'),
  ('Geor','Грузинське письмо'),
  ('Glag','Глаголиця'),
  ('Runr','Руни'),
  ('Ogam','Огам'),
  ('Goth','Готське письмо'),
  ('Nkoo','Н’Ко'),
  ('Adlm','Адлам'),
  ('Arab','Арабиця'),
  ('Hebr','Єврейське письмо'),
  ('Syrc','Сирійське письмо'),
  ('Samr','Самаритянське письмо'),
  ('Mand','Мандейське письмо'),
  ('Phnx','Фінікійське письмо'),
  ('Thaa','Тана'),
  ('Deva','Деванагарі'),
  ('Beng','Бенгальське письмо'),
  ('Guru','Гурмукхі'),
  ('Gujr','Гуджараті'),
  ('Orya','Орія'),
  ('Taml','Тамільське письмо'),
  ('Telu','Телугу'),
  ('Knda','Каннада'),
  ('Mlym','Малаялам'),
  ('Sinh','Сингальське письмо'),
  ('Thai','Тайське письмо'),
  ('Laoo','Лаоське письмо'),
  ('Tibt','Тибетське письмо'),
  ('Mymr','Бірманське письмо'),
  ('Khmr','Кхмерське письмо'),
  ('Ethi','Ефіопське письмо'),
  ('Cans','Канадське складове письмо'),
  ('Tfng','Тіфінаг'),
  ('Java','Яванське письмо'),
  ('Bali','Балійське письмо'),
  ('Hira','Хіраґана'),
  ('Kana','Катакана'),
  ('Bopo','Бопомофо'),
  ('Yiii','Письмо ї'),
  ('Cher','Черокі'),
  ('Vaii','Письмо ваї'),
  ('Hani','Ієрогліфи хань'),
  ('Hans','Спрощені ієрогліфи'),
  ('Hant','Традиційні ієрогліфи'),
  ('Jpan','Японське письмо'),
  ('Egyp','Єгипетські ієрогліфи'),
  ('Xsux','Клинопис'),
  ('Hang','Хангиль'),
  ('Kore','Корейське письмо')
) AS v(code, text)
JOIN oikumenea.writing_systems w ON w.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

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
UPDATE oikumenea.schema_version SET revision = '0018_language', applied_at = now() WHERE singleton;
