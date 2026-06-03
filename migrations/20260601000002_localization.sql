-- 0003 localization (M2).
--
-- The i18n module (D-i18n / docs/modules/localization.md): the instance-admin-managed supported-
-- locale registry + the polymorphic translation store. Translatable `name`/title/description of
-- other modules' structural entities live here as (entity_type, entity_id, field, locale) -> text
-- rows; every response returns all enabled locales as a locale->text map (no Accept-Language
-- negotiation). Both tables are Objects (D-Ontology), keyed by an i18n RID. Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.

-- i18n_locales: the supported-language registry. Seeded ukr + eng; instance-admin-extensible.
-- `code` is the stable, locale-agnostic ISO 639-3 identifier (D-Code) and serves as the natural
-- PRIMARY KEY — a D-ResourceIdentifiers carve-out matching geo_countries: a seeded shared reference
-- registry FK'd by code (i18n_translations.locale references it), not an RID-keyed runtime entity.
-- (Seeding an RID PK would also require app.environment at migration time, which atlas does not set.)
CREATE TABLE oikumenea.i18n_locales (
  code       text PRIMARY KEY,                 -- ISO 639-3 (e.g. ukr, eng); locale-agnostic natural key
  name       text NOT NULL,                    -- endonym/display name (e.g. "Українська", "English")
  enabled    boolean NOT NULL DEFAULT true,
  is_default boolean NOT NULL DEFAULT false,   -- exactly one default among enabled locales (trigger)
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER i18n_locales_set_updated_at
  BEFORE UPDATE ON oikumenea.i18n_locales
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.i18n_locales.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locales.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locales.enabled IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locales.is_default IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_locales.sort_order IS 'pii:none';

-- i18n_enforce_default(): the locale-registry invariant (docs/modules/localization.md). Among the
-- enabled, non-deleted locales there must be at least one and exactly one default. Run as a
-- DEFERRABLE INITIALLY DEFERRED constraint trigger so it is checked once at COMMIT — this lets a
-- multi-row seed and a "switch the default" update pass through an intermediate state. The
-- application is the primary guard; this is the backstop.
CREATE OR REPLACE FUNCTION oikumenea.i18n_enforce_default() RETURNS trigger
  LANGUAGE plpgsql AS $$
DECLARE
  enabled_count integer;
  default_count integer;
BEGIN
  SELECT count(*) FILTER (WHERE enabled),
         count(*) FILTER (WHERE enabled AND is_default)
    INTO enabled_count, default_count
    FROM oikumenea.i18n_locales
   WHERE deleted_at IS NULL;

  IF enabled_count < 1 THEN
    RAISE EXCEPTION 'at least one locale must be enabled'
      USING ERRCODE = 'check_violation';
  END IF;
  IF default_count <> 1 THEN
    RAISE EXCEPTION 'exactly one enabled locale must be the default (found %)', default_count
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NULL;
END;
$$;

CREATE CONSTRAINT TRIGGER i18n_locales_enforce_default
  AFTER INSERT OR UPDATE OR DELETE ON oikumenea.i18n_locales
  DEFERRABLE INITIALLY DEFERRED
  FOR EACH ROW EXECUTE FUNCTION oikumenea.i18n_enforce_default();

-- i18n_translations: the shared, polymorphic translation store. `entity_id` is polymorphic and
-- carries NO foreign key (it spans every translatable module); orphans are purged via the owning
-- module's delete/retire event once those producers + the events bus land (open seam). `locale`
-- references the registry by code.
CREATE TABLE oikumenea.i18n_translations (
  id          text PRIMARY KEY DEFAULT oikumenea.new_rid('i18n','translation'),
  entity_type text NOT NULL,   -- e.g. unit, graph, rank_category, rank, position, role, country
  entity_id   text NOT NULL,   -- the owning entity's id (polymorphic; no FK — see localization.md)
  field       text NOT NULL,   -- the translatable field key: name, title, description
  locale      text NOT NULL REFERENCES oikumenea.i18n_locales(code) ON UPDATE RESTRICT,
  text        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT i18n_translations_rid_shape CHECK (id LIKE 'urn:oikumenea:i18n:%:translation:%'),
  CONSTRAINT i18n_translations_unique UNIQUE (entity_type, entity_id, field, locale)
);

CREATE TRIGGER i18n_translations_set_updated_at
  BEFORE UPDATE ON oikumenea.i18n_translations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- Hot read: fetch all translations of one entity. Plus a locale index for cleanup-by-locale.
CREATE INDEX i18n_translations_entity_idx ON oikumenea.i18n_translations (entity_type, entity_id);
CREATE INDEX i18n_translations_locale_idx ON oikumenea.i18n_translations (locale);

COMMENT ON COLUMN oikumenea.i18n_translations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_translations.entity_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_translations.entity_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_translations.field IS 'pii:none';
COMMENT ON COLUMN oikumenea.i18n_translations.locale IS 'pii:none';
-- Translatable labels of structural/catalog entities (unit/rank/role names) are not personal data.
COMMENT ON COLUMN oikumenea.i18n_translations.text IS 'pii:none';

-- Seed the out-of-the-box locales (D-i18n): ukr (default) + eng, both enabled.
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order) VALUES
  ('ukr', 'Українська', true, true,  0),
  ('eng', 'English',    true, false, 10);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0003_localization', applied_at = now() WHERE singleton;
