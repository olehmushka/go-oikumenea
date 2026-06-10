-- 0005 rank (M4 + M15).
--
-- The single, system-wide rank scheme (L-OneRankScheme / D-Rank / D-RankSystems / docs/modules/rank.md):
-- a rank SYSTEM at the top (a national/organizational ladder — one scheme may hold several at once for a
-- coalition directory, D-RankSystems), a rank category within each system, a TREE of rank types within
-- each category (a type may have child types via parent_type_id), and ranks on the LEAF types — strict
-- containment throughout. Each level is an instance-config catalog Object (D-Ontology), keyed by a rank
-- RID; `code` is the stable, locale-agnostic external reference (D-Code), `name` the default-locale
-- fallback (other locales in the i18n store, M2). `system_id` is denormalized DOWN onto categories,
-- types, and ranks (exactly as category_id is) so grouping, sibling code-uniqueness, and seniority need
-- no recursive walk. Intra-system seniority is the lexical order of (system.sort_order, category.sort_order,
-- the type sort_order path down the tree, rank.sort_order); CROSS-system comparability comes from a rank's
-- optional standardized grade_code (NATO STANAG 2116 — the seeded rank_grades catalog below). A rank is a
-- DIRECTORY attribute and never an authz input — the PDP never reads it. Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects + geo_countries (D-Geo).
--
-- This migration's scheme tables (systems/categories/types/ranks) are PURE DDL: they seed NO rows. The
-- scheme is deployment-specific (army ranks for an army, academic ranks for a university —
-- L-SingleDomain), so it is populated by the instance admin through RankService (or POST
-- /rank-scheme/import from an opt-in preset), not shipped. (The RID PKs would also need app.environment
-- at migration time, which atlas's connection does not set — D-RIDSeeding.) The ONE exception is
-- rank_grades: a standardized reference catalog with a NATURAL key (the STANAG code), so it is
-- migration-seeded (no RID, no GUC — the D-Geo reference-registry carve-out).
--
-- `sort_order` is application-managed within each parent (append = max+1; reorder = an explicit set).
-- It is intentionally NOT a DB uniqueness constraint: a partial-unique index could not be DEFERRED,
-- so swapping two siblings' orders in one txn would transiently violate it. Reads order by
-- (sort_order, code), so seniority is deterministic even with a duplicate sort_order mid-reorder.

-- rank_systems: the top level (D-RankSystems) — a national/organizational rank ladder
-- (e.g. ua-armed-forces, us-armed-forces, nato). One scheme may hold several at once (a coalition
-- directory); a single-nation deployment just has one. `country` is the national origin (ISO-3166 via
-- geo_countries, D-Geo) and is NULL for a supranational system (NATO/UN). ON DELETE RESTRICT on the
-- category->system FK keeps a system with active categories from being removed.
CREATE TABLE oikumenea.rank_systems (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','system'),
  code       text NOT NULL,                 -- stable, locale-agnostic; unique among active (index below)
  name       text NOT NULL,                 -- default-locale display name; translatable via i18n store
  sort_order integer NOT NULL,              -- order among active systems (app-managed)
  country    text REFERENCES oikumenea.geo_countries(code), -- national origin; NULL = supranational
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT rank_systems_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:system:%')
);

CREATE TRIGGER rank_systems_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_systems
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX rank_systems_code_active_idx
  ON oikumenea.rank_systems (code) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_systems.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_systems.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_systems.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_systems.sort_order IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_systems.country IS 'pii:none';

-- rank_grades: the standardized cross-system comparability scale (D-RankSystems) — NATO STANAG 2116.
-- A seeded reference catalog with a NATURAL key (the grade code), immutable by convention: no
-- soft-delete, no RID. A rank's optional grade_code FKs here. Cross-system EQUIVALENCE = same grade_code;
-- cross-system SENIORITY = tier (enlisted < warrant < officer, ordered in the app) then ordinal (ordered
-- WITHIN a tier here). A non-military system simply leaves grade_code NULL and has no comparator.
CREATE TABLE oikumenea.rank_grades (
  code    text PRIMARY KEY,                 -- STANAG 2116 grade (OF(D), OF-1..OF-10, WO-1..WO-5, OR-1..OR-9)
  tier    text NOT NULL CHECK (tier IN ('officer','warrant','enlisted')),
  ordinal integer NOT NULL,                 -- order within the tier (junior -> senior)
  name    text NOT NULL                     -- generic, nation-neutral grade label
);

COMMENT ON COLUMN oikumenea.rank_grades.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_grades.tier IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_grades.ordinal IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_grades.name IS 'pii:none';

-- Seed the STANAG 2116 grades (natural key -> safe in a migration, D-RIDSeeding carve-out). Names are
-- deliberately nation-neutral: a NATO code maps to different rank titles per nation, so only the
-- standardized grade is recorded here.
INSERT INTO oikumenea.rank_grades (code, tier, ordinal, name) VALUES
  ('OR-1',  'enlisted', 1, 'Other rank grade 1'),
  ('OR-2',  'enlisted', 2, 'Other rank grade 2'),
  ('OR-3',  'enlisted', 3, 'Other rank grade 3'),
  ('OR-4',  'enlisted', 4, 'Other rank grade 4'),
  ('OR-5',  'enlisted', 5, 'Other rank grade 5'),
  ('OR-6',  'enlisted', 6, 'Other rank grade 6'),
  ('OR-7',  'enlisted', 7, 'Other rank grade 7'),
  ('OR-8',  'enlisted', 8, 'Other rank grade 8'),
  ('OR-9',  'enlisted', 9, 'Other rank grade 9'),
  ('WO-1',  'warrant',  1, 'Warrant officer grade 1'),
  ('WO-2',  'warrant',  2, 'Warrant officer grade 2'),
  ('WO-3',  'warrant',  3, 'Warrant officer grade 3'),
  ('WO-4',  'warrant',  4, 'Warrant officer grade 4'),
  ('WO-5',  'warrant',  5, 'Warrant officer grade 5'),
  ('OF(D)', 'officer',  0, 'Officer candidate'),
  ('OF-1',  'officer',  1, 'Officer grade 1'),
  ('OF-2',  'officer',  2, 'Officer grade 2'),
  ('OF-3',  'officer',  3, 'Officer grade 3'),
  ('OF-4',  'officer',  4, 'Officer grade 4'),
  ('OF-5',  'officer',  5, 'Officer grade 5'),
  ('OF-6',  'officer',  6, 'Officer grade 6'),
  ('OF-7',  'officer',  7, 'Officer grade 7'),
  ('OF-8',  'officer',  8, 'Officer grade 8'),
  ('OF-9',  'officer',  9, 'Officer grade 9'),
  ('OF-10', 'officer', 10, 'Officer grade 10');

-- rank_categories: a branch within a system (e.g. army/navy, or academic/administrative), ordered. Every
-- category belongs to one rank_system (ON DELETE RESTRICT keeps a system with active categories intact);
-- `code` is unique among active SIBLINGS within the system.
CREATE TABLE oikumenea.rank_categories (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','category'),
  system_id  text NOT NULL REFERENCES oikumenea.rank_systems(id) ON DELETE RESTRICT,
  code       text NOT NULL,                 -- stable, locale-agnostic; unique among active siblings within the system
  name       text NOT NULL,                 -- default-locale display name; translatable via i18n store
  sort_order integer NOT NULL,              -- seniority ordinal among active categories of the system (app-managed)
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT rank_categories_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:category:%')
);

CREATE TRIGGER rank_categories_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_categories
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX rank_categories_code_active_idx
  ON oikumenea.rank_categories (system_id, code) WHERE deleted_at IS NULL;
CREATE INDEX rank_categories_system_idx
  ON oikumenea.rank_categories (system_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_categories.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.system_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.sort_order IS 'pii:none';

-- rank_types: a band within a category (e.g. officers/warrant/enlisted), forming a TREE — a type may
-- nest under another type of the same category via parent_type_id (NULL = a root type of its
-- category). Ranks attach to LEAF types only (enforced in the application). category_id is carried on
-- EVERY type (denormalized to the root category) so grouping, code-uniqueness, and seniority need no
-- recursive lookup; a nested type's category_id equals its parent's. ON DELETE RESTRICT (on both the
-- category FK and the self-FK) keeps the containment tree intact: a category with active types, or a
-- type with active child types, cannot be removed.
CREATE TABLE oikumenea.rank_types (
  id             text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','type'),
  system_id      text NOT NULL,             -- denormalized root system (equals the category's system_id)
  category_id    text NOT NULL REFERENCES oikumenea.rank_categories(id) ON DELETE RESTRICT,
  parent_type_id text REFERENCES oikumenea.rank_types(id) ON DELETE RESTRICT, -- NULL = root type of its category
  code           text NOT NULL,             -- stable, locale-agnostic; unique among active siblings (same category + parent)
  name           text NOT NULL,             -- default-locale display name; translatable via i18n store
  sort_order     integer NOT NULL,          -- seniority ordinal among active siblings (app-managed)
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT rank_types_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:type:%'),
  CONSTRAINT rank_types_no_self_parent CHECK (parent_type_id IS NULL OR parent_type_id <> id)
);

CREATE TRIGGER rank_types_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` unique among active SIBLINGS: same category AND same parent. parent_type_id is a URN RID,
-- never '', so COALESCE(..., '') is a safe sentinel for "root of category" and keeps two roots of the
-- same category from sharing a code (NULLs would otherwise compare distinct). COALESCE is immutable,
-- so this is index-legal on any PG version.
CREATE UNIQUE INDEX rank_types_code_active_idx
  ON oikumenea.rank_types (category_id, COALESCE(parent_type_id, ''), code) WHERE deleted_at IS NULL;
CREATE INDEX rank_types_category_idx
  ON oikumenea.rank_types (category_id) WHERE deleted_at IS NULL;
CREATE INDEX rank_types_parent_idx
  ON oikumenea.rank_types (parent_type_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.system_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.category_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.parent_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.sort_order IS 'pii:none';

-- rank_ranks: a specific grade within a type (e.g. private/corporal/sergeant), ordered for exact
-- seniority. ON DELETE RESTRICT: a type with active ranks cannot be removed; person_persons.rank_id
-- will likewise RESTRICT (M5) so a held rank cannot be deleted.
CREATE TABLE oikumenea.rank_ranks (
  id           text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','rank'),
  system_id    text NOT NULL,              -- denormalized root system (equals the type's system_id)
  type_id      text NOT NULL REFERENCES oikumenea.rank_types(id) ON DELETE RESTRICT,
  code         text NOT NULL,              -- stable, locale-agnostic; unique within type among active
  name         text NOT NULL,             -- default-locale display name; translatable via i18n store
  abbreviation text,                       -- optional short form (e.g. SGT); locale-agnostic
  grade_code   text REFERENCES oikumenea.rank_grades(code), -- optional standardized cross-system grade (D-RankSystems)
  sort_order   integer NOT NULL,          -- seniority ordinal among active siblings (app-managed)
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,

  CONSTRAINT rank_ranks_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:rank:%')
);

CREATE TRIGGER rank_ranks_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_ranks
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX rank_ranks_code_active_idx
  ON oikumenea.rank_ranks (type_id, code) WHERE deleted_at IS NULL;
CREATE INDEX rank_ranks_type_idx
  ON oikumenea.rank_ranks (type_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_ranks.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.system_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.abbreviation IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.grade_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.sort_order IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0005_rank', applied_at = now() WHERE singleton;
