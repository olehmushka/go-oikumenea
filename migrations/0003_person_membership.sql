-- 0003_person_membership — merged domain migration (refactor: consolidated from 0005_person, 0006_membership).

-- ===== merged from 0005_person =====
-- 0005 person (M5).
--
-- The personnel directory (docs/modules/person.md): the core aggregate of the whole service. A
-- person is instance-global (one record per individual, never per-unit — D-PersonGlobal), exists
-- independently of any login account (L-AccountOptional) and of any unit membership, and holds at
-- most ONE rank PER RANK SYSTEM via the person_ranks link table — a DIRECTORY attribute that grants
-- no authority (D-Rank); the PDP never reads it.
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

-- Trigram matching for the directory typeahead (review R-06): pg_trgm makes an unanchored
-- ILIKE '%q%' index-servable via GIN. Idempotent — the extension may already exist.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- person_persons: the aggregate root — one record per individual, account-optional, instance-global.
CREATE TABLE oikumenea.person_persons (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,1),  -- person / object / person
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
  country_of_birth_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (D-Geo); ISO code resolved in SQL
  attributes       jsonb NOT NULL DEFAULT '{}',   -- long-tail directory fields; pii:special CEILING (grab-bag)

  -- rank is NOT a column: a person holds one rank PER RANK SYSTEM via the person_ranks link below (D-Rank).

  status           text NOT NULL DEFAULT 'active'
                     CHECK (status IN ('active','deactivated','purged','provisional')),
                     -- 'provisional' = a minimal-PII stub node (D-OverlayFoundation, M29): an unresolved
                     -- external/edge target awaiting MergePerson, which promotes/merges it into a canonical
                     -- person (re-homing its edges) and tombstones the stub as 'purged'.
  deactivated_at   timestamptz,                   -- set on deactivate; cleared on reactivate
  purge_after      timestamptz,                   -- reversibility window end; purge refuses before it

  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  -- Denormalized lowercased search haystack over the same fields the directory search matched
  -- historically (display_name / code / given / surname). STORED + trigram-indexed so the
  -- typeahead ILIKE '%q%' is a bitmap index scan, not a seq scan, at directory scale (review R-06).
  search_text      text GENERATED ALWAYS AS (
                     lower(coalesce(display_name,'') || ' ' || coalesce(code,'') || ' ' ||
                           coalesce(given,'') || ' ' || coalesce(surname,''))) STORED,

  CONSTRAINT person_persons_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER person_persons_set_updated_at
  BEFORE UPDATE ON oikumenea.person_persons
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_persons_code_active_idx
  ON oikumenea.person_persons (code) WHERE code IS NOT NULL AND deleted_at IS NULL;

-- Directory search (review R-06): GIN trigram over the generated haystack, partial on active rows.
CREATE INDEX person_persons_search_trgm_idx
  ON oikumenea.person_persons USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;

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
COMMENT ON COLUMN oikumenea.person_persons.country_of_birth_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_persons.attributes IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_persons.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.deactivated_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.purge_after IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_persons.search_text IS 'pii:basic';

-- person_ranks: the reified HOLDS_RANK link — a person holds one rank PER RANK SYSTEM (D-Rank,
-- extended by D-RankSystems). Rank is a DIRECTORY attribute that grants no authority; the PDP never
-- reads it. system_id is denormalized from the rank (derived in SQL on write from rank_ranks.system_id)
-- so the one-per-system uniqueness needs no join. ON DELETE RESTRICT on rank_id/system_id so a held
-- rank/system cannot be deleted; CASCADE on person delete. As a reified Link the RID entity_type token
-- is link__holds_rank (D-Ontology).
CREATE TABLE oikumenea.person_ranks (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,1),  -- person / link / holds_rank
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  system_id  uuid NOT NULL REFERENCES oikumenea.rank_systems(id) ON DELETE RESTRICT,  -- denormalized from the rank
  rank_id    uuid NOT NULL REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,
  -- native validity (D-Temporal, R-31): the interval this rank is held; NULL valid_to = active.
  valid_from timestamptz NOT NULL DEFAULT now(),
  valid_to   timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_ranks_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER person_ranks_set_updated_at
  BEFORE UPDATE ON oikumenea.person_ranks
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One ACTIVE rank per (person, system) — the core invariant of "one rank per rank system".
CREATE UNIQUE INDEX person_ranks_person_system_active_idx
  ON oikumenea.person_ranks (person_id, system_id) WHERE deleted_at IS NULL;
CREATE INDEX person_ranks_person_idx
  ON oikumenea.person_ranks (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_ranks_rank_idx
  ON oikumenea.person_ranks (rank_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_ranks.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ranks.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ranks.system_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ranks.rank_id IS 'pii:none';

-- person_name_variants: per-person transliteration / alternate name forms (e.g. ukr native + eng
-- Latin). A variant is a FULL name form, so it carries the same CLDR structured parts. CASCADE on
-- person delete; locale FK to the i18n registry. UNIQUE (person_id, locale).
CREATE TABLE oikumenea.person_name_variants (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,2),  -- person / object / name_variant
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
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

  -- Search haystack for this name form (review R-06): lets the directory typeahead find a person by
  -- a native-script transliteration or an alias/aka, not just the canonical Latin display name.
  search_text    text GENERATED ALWAYS AS (
                   lower(coalesce(display_name,'') || ' ' || coalesce(given,'') || ' ' ||
                         coalesce(surname,'') || ' ' || coalesce(preferred,''))) STORED,

  CONSTRAINT person_name_variants_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2),
  CONSTRAINT person_name_variants_person_locale_uniq UNIQUE (person_id, locale)
);

CREATE TRIGGER person_name_variants_set_updated_at
  BEFORE UPDATE ON oikumenea.person_name_variants
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_name_variants_person_idx ON oikumenea.person_name_variants (person_id);
-- Alias / transliteration search (review R-06): GIN trigram over the variant haystack.
CREATE INDEX person_name_variants_search_trgm_idx
  ON oikumenea.person_name_variants USING gin (search_text gin_trgm_ops);

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
COMMENT ON COLUMN oikumenea.person_name_variants.search_text IS 'pii:basic';

-- person_citizenships: effective-dated nationality; a person may hold several (D-Geo). One ACTIVE
-- citizenship per (person, country). is_primary marks at most one. CASCADE on person delete.
CREATE TABLE oikumenea.person_citizenships (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,3),  -- person / object / citizenship
  person_id   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  country_id  uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- ISO code resolved in SQL
  basis       text NOT NULL DEFAULT 'other'
                CHECK (basis IN ('birth','descent','naturalization','other')),
  acquired_on date,
  lost_on     date,
  is_primary  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT person_citizenships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);

CREATE TRIGGER person_citizenships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_citizenships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_citizenships_active_country_idx
  ON oikumenea.person_citizenships (person_id, country_id) WHERE lost_on IS NULL AND deleted_at IS NULL;
CREATE INDEX person_citizenships_person_idx
  ON oikumenea.person_citizenships (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_citizenships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_citizenships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_citizenships.country_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.basis IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.acquired_on IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.lost_on IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_citizenships.is_primary IS 'pii:none';

-- person_residences: effective-dated residence history (D-Geo). Locator data → pii:contact.
-- CASCADE on person delete.
CREATE TABLE oikumenea.person_residences (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,4),  -- person / object / residence
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  country_id uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- ISO code resolved in SQL
  region     text,                               -- optional sub-national region / locality (free text)
  valid_from date NOT NULL,
  valid_to   date,                               -- NULL = current
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_residences_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);

CREATE TRIGGER person_residences_set_updated_at
  BEFORE UPDATE ON oikumenea.person_residences
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_residences_person_idx
  ON oikumenea.person_residences (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_residences.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_residences.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_residences.country_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.region IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.valid_from IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_residences.valid_to IS 'pii:contact';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0006_membership =====
-- 0006 membership (M6).
--
-- People belonging to / filling billets in units (docs/modules/membership.md). Two related things:
--   * POSITIONS (membership_positions) — a unit-owned billet (an Object that EXISTS while vacant;
--     D-Position). It belongs to exactly one unit, carries a stable `code` + a translatable `title`,
--     an optional establishment `required_rank_id`, and a reversible active/abolished status.
--   * MEMBERSHIPS (membership_memberships) — the reified Link link__member_of: a person's belonging
--     to a unit, OPTIONALLY filling a position, effective-dated. Filling a billet is a membership
--     that references it; a position-less membership is plain belonging.
--
-- Neither entity stores visibility — it DERIVES from the owning unit's visibility (tenant.md); the
-- shadow-visibility read gate lands with the PDP (M7). Position grants NO authority (D-Position /
-- D-Rank) — it is directory data, never a PDP input.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid,
-- set_updated_at), 0004 tenant (tenant_units), 0005 rank (rank_ranks) and 0006 person
-- (person_persons). This migration is PURE DDL: it seeds NO rows (so no app.environment GUC is
-- needed at migration time — D-RIDSeeding). Positions/memberships are created through
-- MembershipService.

-- membership_positions: a unit-owned billet (D-Position). Exists vacant; `code` is the stable,
-- locale-agnostic identifier (unique within the unit among active rows); `title` is the
-- default-locale fallback (translations in the i18n store, M2). required_rank_id is the
-- establishment expectation — ADVISORY, never enforced against any filler's rank.
CREATE TABLE oikumenea.membership_positions (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(7,1,1),  -- membership / object / position
  unit_id          uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- the owning unit
  code             text NOT NULL,                 -- stable, locale-agnostic; unique within the unit among active
  title            text NOT NULL,                 -- default-locale title; translatable via the i18n store
  required_rank_id uuid REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,  -- optional, advisory establishment rank
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','abolished')),
  sort_order       integer,                       -- app-managed display order within the unit
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT membership_positions_rid_shape
    CHECK (oikumenea.rid_service(id)=7 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER membership_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.membership_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` is unique within a unit among active (non-deleted) positions; immutable by convention (D-Code).
CREATE UNIQUE INDEX membership_positions_unit_code_active_idx
  ON oikumenea.membership_positions (unit_id, code) WHERE deleted_at IS NULL;
CREATE INDEX membership_positions_unit_active_idx
  ON oikumenea.membership_positions (unit_id) WHERE status = 'active' AND deleted_at IS NULL;

-- Billet/establishment labels are organizational, not personal data (D-PIITiers).
COMMENT ON COLUMN oikumenea.membership_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.title IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.required_rank_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.sort_order IS 'pii:none';

-- membership_memberships: the reified person->unit belonging/filling Link (link__member_of). A
-- membership requires an existing person and unit; if it references a position, that position must
-- belong to the same unit (checked in the application). Reversible: end flips status + sets
-- effective_to rather than deleting; ending a filling VACATES the billet.
CREATE TABLE oikumenea.membership_memberships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(7,2,1),  -- membership / link / member_of
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  unit_id        uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  position_id    uuid REFERENCES oikumenea.membership_positions(id) ON DELETE RESTRICT,  -- NULL = plain belonging
  -- order_item_id: provenance pointer to the order (наказ) item this fill/belonging cites as its
  -- legal basis (D-Orders). The order module is M10, so the FK (-> order_order_items, ON DELETE SET
  -- NULL) is added then; today it is a free-standing nullable RID column (open seam).
  order_item_id  uuid,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT membership_memberships_rid_shape
    CHECK (oikumenea.rid_service(id)=7 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER membership_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.membership_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One billet, one holder: a position has at most one ACTIVE filling (multi-incumbent is a seam).
CREATE UNIQUE INDEX membership_memberships_one_holder_idx
  ON oikumenea.membership_memberships (position_id)
  WHERE position_id IS NOT NULL AND status = 'active' AND deleted_at IS NULL;
-- Plain belonging is unique per (person, unit) among active position-less memberships.
CREATE UNIQUE INDEX membership_memberships_belonging_idx
  ON oikumenea.membership_memberships (person_id, unit_id)
  WHERE position_id IS NULL AND status = 'active' AND deleted_at IS NULL;

CREATE INDEX membership_memberships_person_idx
  ON oikumenea.membership_memberships (person_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX membership_memberships_unit_idx
  ON oikumenea.membership_memberships (unit_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX membership_memberships_position_idx
  ON oikumenea.membership_memberships (position_id) WHERE status = 'active' AND deleted_at IS NULL;

-- A membership links a person to a unit (the association is organizational); it is not itself a PII
-- store. person_id is a stable id (pii:none), but the FACT of belonging to a specific unit is
-- mildly identifying, so the link end and its dates are tiered pii:basic.
COMMENT ON COLUMN oikumenea.membership_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.unit_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.position_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.order_item_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.effective_to IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0003_person_membership', applied_at = now() WHERE singleton;
