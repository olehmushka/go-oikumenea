-- 0005 rank (M4).
--
-- The single, system-wide rank scheme (L-OneRankScheme / D-Rank / docs/modules/rank.md): a
-- three-level ordered ladder rank category -> rank type -> rank, strict containment. Each level is
-- an instance-config catalog Object (D-Ontology), keyed by a rank RID; `code` is the stable,
-- locale-agnostic external reference (D-Code), `name` the default-locale fallback (other locales in
-- the i18n store, M2). Seniority is the lexical order of (category.sort_order, type.sort_order,
-- rank.sort_order). A rank is a DIRECTORY attribute and never an authz input — the PDP never reads
-- it. Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.
--
-- This migration is PURE DDL: it seeds NO rows. The scheme is deployment-specific (army ranks for an
-- army, academic ranks for a university — L-SingleDomain), so it is populated by the instance admin
-- through RankService, not shipped. (The RID PKs would also need app.environment at migration time,
-- which atlas's connection does not set — D-RIDSeeding.)
--
-- `sort_order` is application-managed within each parent (append = max+1; reorder = an explicit set).
-- It is intentionally NOT a DB uniqueness constraint: a partial-unique index could not be DEFERRED,
-- so swapping two siblings' orders in one txn would transiently violate it. Reads order by
-- (sort_order, code), so seniority is deterministic even with a duplicate sort_order mid-reorder.

-- rank_categories: top level of the scheme (e.g. army/navy, or academic/administrative), ordered.
CREATE TABLE oikumenea.rank_categories (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','category'),
  code       text NOT NULL,                 -- stable, locale-agnostic; unique among active (index below)
  name       text NOT NULL,                 -- default-locale display name; translatable via i18n store
  sort_order integer NOT NULL,              -- seniority ordinal among active categories (app-managed)
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT rank_categories_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:category:%')
);

CREATE TRIGGER rank_categories_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_categories
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX rank_categories_code_active_idx
  ON oikumenea.rank_categories (code) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_categories.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_categories.sort_order IS 'pii:none';

-- rank_types: a band within a category (e.g. officers/warrant/enlisted), ordered. ON DELETE RESTRICT
-- keeps the containment tree intact; a category in use (has active types) cannot be removed.
CREATE TABLE oikumenea.rank_types (
  id          text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','type'),
  category_id text NOT NULL REFERENCES oikumenea.rank_categories(id) ON DELETE RESTRICT,
  code        text NOT NULL,                -- stable, locale-agnostic; unique within category among active
  name        text NOT NULL,               -- default-locale display name; translatable via i18n store
  sort_order  integer NOT NULL,            -- seniority ordinal among active siblings (app-managed)
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT rank_types_rid_shape CHECK (id LIKE 'urn:oikumenea:rank:%:type:%')
);

CREATE TRIGGER rank_types_set_updated_at
  BEFORE UPDATE ON oikumenea.rank_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX rank_types_code_active_idx
  ON oikumenea.rank_types (category_id, code) WHERE deleted_at IS NULL;
CREATE INDEX rank_types_category_idx
  ON oikumenea.rank_types (category_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.rank_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.category_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_types.sort_order IS 'pii:none';

-- rank_ranks: a specific grade within a type (e.g. private/corporal/sergeant), ordered for exact
-- seniority. ON DELETE RESTRICT: a type with active ranks cannot be removed; person_persons.rank_id
-- will likewise RESTRICT (M5) so a held rank cannot be deleted.
CREATE TABLE oikumenea.rank_ranks (
  id           text PRIMARY KEY DEFAULT oikumenea.new_rid('rank','rank'),
  type_id      text NOT NULL REFERENCES oikumenea.rank_types(id) ON DELETE RESTRICT,
  code         text NOT NULL,              -- stable, locale-agnostic; unique within type among active
  name         text NOT NULL,             -- default-locale display name; translatable via i18n store
  abbreviation text,                       -- optional short form (e.g. SGT); locale-agnostic
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
COMMENT ON COLUMN oikumenea.rank_ranks.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.abbreviation IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.sort_order IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0005_rank', applied_at = now() WHERE singleton;
