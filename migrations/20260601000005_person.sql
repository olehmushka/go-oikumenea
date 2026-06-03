-- 0006 person (M5).
--
-- The personnel directory (docs/modules/person.md): the core aggregate of the whole service. A
-- person is instance-global (one record per individual, never per-unit — D-PersonGlobal), exists
-- independently of any login account (L-AccountOptional) and of any unit membership, and holds at
-- most ONE rank — a DIRECTORY attribute that grants no authority (D-Rank); the PDP never reads it.
-- person_persons is the system's primary PII store, so every column is tiered with COMMENT ON
-- COLUMN ... IS 'pii:<tier>' (D-PIITiers) and the lifecycle carries a crypto-erase PURGE path.
--
-- Names follow the Unicode CLDR Person Names fixed field set (D-PersonNamesCLDR): display_name is
-- authoritative, the structured parts are advisory and used for locale-aware formatting. There is
-- NO dedicated patronymic field — the Slavic по-батькові lives in given2, and formal Slavic address
-- is assembled from given + given2. Anything rarer (Arabic nasab, 4+ surnames, clan/tribal) is not
-- typed: it rides in display_name (+ a per-locale variant). Transliterations are per-person data
-- (person_name_variants), NOT the instance-admin localization store (D-i18n).
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap (new_rid,
-- set_updated_at, geo_countries — D-Geo), 0003 localization (i18n_locales) and 0005 rank
-- (rank_ranks). This migration is PURE DDL: it seeds NO rows (so no app.environment GUC is needed at
-- migration time — D-RIDSeeding). Persons are created through PersonService.

-- person_persons: the aggregate root — one record per individual, account-optional, instance-global.
CREATE TABLE oikumenea.person_persons (
  id               text PRIMARY KEY DEFAULT oikumenea.new_rid('person','person'),
  code             text,                          -- OPTIONAL stable, locale-agnostic external id (personnel/service number); unique among active
  display_name     text NOT NULL,                 -- the canonical full name form; authoritative for search/display

  -- Unicode CLDR Person Names fixed field set (all optional; advisory — display_name is authoritative).
  title            text,                          -- honorific / title prefix (Dr., Rev., Ms.)
  given            text,                          -- first / forename
  given2           text,                          -- second given name; also holds Slavic по-батькові / Icelandic patronymic
  surname          text,                          -- primary surname
  surname_prefix   text,                          -- nobiliary / genealogical particle (van, von, de, bin)
  surname2         text,                          -- second surname (Hispanic / Lusophone)
  generation       text,                          -- generational suffix (Jr., Sr., III)
  credentials      text,                          -- post-nominal credentials (PhD, MD)
  preferred        text,                          -- known-as / nickname

  birthdate        date,                          -- calendar day of birth (a DATE, not an instant); nullable
  sex              text NOT NULL DEFAULT 'not_known'
                     CHECK (sex IN ('not_known','male','female','not_applicable')),  -- biological sex, ISO/IEC 5218
  country_of_birth char(2) REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,  -- nullable (D-Geo)
  attributes       jsonb NOT NULL DEFAULT '{}',   -- long-tail directory fields; pii:special CEILING (grab-bag)

  rank_id          text REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,        -- one rank, nullable; RESTRICT so a held rank cannot be deleted

  status           text NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active','deactivated','purged')),
  deactivated_at   timestamptz,                   -- set on deactivate; cleared on reactivate
  purge_after      timestamptz,                   -- reversibility window end; purge refuses before it

  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT person_persons_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:person:%')
);

CREATE TRIGGER person_persons_set_updated_at
  BEFORE UPDATE ON oikumenea.person_persons
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_persons_code_active_idx
  ON oikumenea.person_persons (code) WHERE code IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX person_persons_rank_idx
  ON oikumenea.person_persons (rank_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_persons.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.code IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.display_name IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.title IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.given IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.given2 IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.surname IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.surname_prefix IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.surname2 IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.generation IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.credentials IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.preferred IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.birthdate IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.sex IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.country_of_birth IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.attributes IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_persons.rank_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.deactivated_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.purge_after IS 'pii:none';

-- person_name_variants: per-person transliteration / alternate name forms (e.g. ukr native + eng
-- Latin). A variant is a FULL name form, so it carries the same CLDR structured parts. CASCADE on
-- person delete; locale FK to the i18n registry. UNIQUE (person_id, locale).
CREATE TABLE oikumenea.person_name_variants (
  id             text PRIMARY KEY DEFAULT oikumenea.new_rid('person','name_variant'),
  person_id      text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  locale         text NOT NULL REFERENCES oikumenea.i18n_locales(code) ON UPDATE RESTRICT,
  display_name   text NOT NULL,
  title          text,
  given          text,
  given2         text,
  surname        text,
  surname_prefix text,
  surname2       text,
  generation     text,
  credentials    text,
  preferred      text,
  is_primary     boolean NOT NULL DEFAULT false,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT person_name_variants_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:name_variant:%'),
  CONSTRAINT person_name_variants_person_locale_uniq UNIQUE (person_id, locale)
);

CREATE TRIGGER person_name_variants_set_updated_at
  BEFORE UPDATE ON oikumenea.person_name_variants
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_name_variants_person_idx ON oikumenea.person_name_variants (person_id);

COMMENT ON COLUMN oikumenea.person_name_variants.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.locale IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.display_name IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.title IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.given IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.given2 IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.surname IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.surname_prefix IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.surname2 IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.generation IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.credentials IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.preferred IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_name_variants.is_primary IS 'pii:none';

-- person_citizenships: effective-dated nationality; a person may hold several (D-Geo). One ACTIVE
-- citizenship per (person, country). is_primary marks at most one. CASCADE on person delete.
CREATE TABLE oikumenea.person_citizenships (
  id          text PRIMARY KEY DEFAULT oikumenea.new_rid('person','citizenship'),
  person_id   text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  country     char(2) NOT NULL REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,
  basis       text NOT NULL DEFAULT 'other'
                CHECK (basis IN ('birth','descent','naturalization','other')),
  acquired_on date,
  lost_on     date,
  is_primary  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT person_citizenships_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:citizenship:%')
);

CREATE TRIGGER person_citizenships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_citizenships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_citizenships_active_country_idx
  ON oikumenea.person_citizenships (person_id, country) WHERE lost_on IS NULL AND deleted_at IS NULL;
CREATE INDEX person_citizenships_person_idx
  ON oikumenea.person_citizenships (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_citizenships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_citizenships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_citizenships.country IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.basis IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.acquired_on IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.lost_on IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.is_primary IS 'pii:none';

-- person_residences: effective-dated residence history (D-Geo). Locator data → pii:contact.
-- CASCADE on person delete.
CREATE TABLE oikumenea.person_residences (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('person','residence'),
  person_id  text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  country    char(2) NOT NULL REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,
  region     text,                               -- optional sub-national region / locality (free text)
  valid_from date NOT NULL,
  valid_to   date,                               -- NULL = current
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_residences_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:residence:%')
);

CREATE TRIGGER person_residences_set_updated_at
  BEFORE UPDATE ON oikumenea.person_residences
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_residences_person_idx
  ON oikumenea.person_residences (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_residences.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_residences.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_residences.country IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.region IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.valid_from IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.valid_to IS 'pii:contact';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0006_person', applied_at = now() WHERE singleton;
