-- 0026 religion discovery (M25 — D-Religion discovery surface): sites, service schedules, aliases.
--
-- The discovery substrate over the religion vertical (docs/modules/religion.md). It makes religious
-- organizations FINDABLE — where they meet (a site = a worship-community unit ↔ a shared location),
-- when they serve (per-site service schedules), and under what names (search-only aliases) — with
-- privacy-preserving spatial search. A FaithMap-style discovery/CMS app consumes this; the CMS itself
-- stays in that app.
--
-- Sites reuse the shared `location_locations` entity (D-Location, M19 / migration 0019) by FK — they do
-- NOT duplicate coordinates. The PUBLISH-PRECISION projection (`public_precision`) is applied APP-SIDE in
-- Go (coordinate rounding), NOT in the DB: D-Location was amended 2026-06-17 to DROP H3 entirely (the
-- stock postgis image is used; radius search is PostGIS ST_DWithin on the GiST index), so the original
-- "coarsen to an H3 cell" sketch is replaced by app-side rounding. The full coordinate stays in
-- location_locations; coarsening is a read-time projection on the site link, never a stored loss.
--
-- Binding design rule (D-Religion): site/service types are CATALOG ROWS keyed to a religion_taxa node
-- (per tradition) or generic, never a CHECK enum. The only CHECK enums here are fixed lifecycle/visibility
-- statuses and the alias-type / service-mode vocabularies (structural, not faith vocabulary).
--
-- Unit-scoped (org_unit_id / unit_id) tables carry the RLS backstop (D-RLSDefenseInDepth), like
-- religion_org_classifications / religion_clergy_credentials. Catalogs are reference data: NO RLS.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id), 0003 tenant (tenant_units),
-- 0019 location (location_locations) and 0023 religion (religion_taxa).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (16,1,11,'site_type'),
  (16,1,12,'service_type'),
  (16,1,13,'service_schedule'),
  (16,1,14,'alias'),
  (16,2,4,'site_of');

-- ===================================================================================================
-- religion_site_types — the per-tradition site/place catalog (D-Code / D-i18n).
-- ===================================================================================================
CREATE TABLE oikumenea.religion_site_types (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,11),  -- religion / object / site_type
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  code               text NOT NULL,
  name               text NOT NULL,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order         integer,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT religion_site_types_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=11)
);
CREATE UNIQUE INDEX religion_site_types_code_active
  ON oikumenea.religion_site_types (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_site_types_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_site_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_site_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_site_types.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_site_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_site_types.name IS 'pii:none';

-- Generic + per-tradition seed (cross-faith examples).
INSERT INTO oikumenea.religion_site_types (tradition_taxon_id, code, name, sort_order) VALUES
  (NULL,'office','Office',100),
  (NULL,'online','Online',110),
  (NULL,'mission','Mission',120),
  (NULL,'shrine','Shrine',130);
INSERT INTO oikumenea.religion_site_types (tradition_taxon_id, code, name, sort_order)
SELECT t.id, v.code, v.name, v.so
FROM (VALUES
  ('christianity','church','Church',10),
  ('christianity','cathedral','Cathedral',20),
  ('christianity','chapel','Chapel',30),
  ('christianity','monastery','Monastery',40),
  ('islam','mosque','Mosque',10),
  ('judaism','synagogue','Synagogue',10),
  ('hinduism','temple','Temple',10),
  ('buddhism','temple','Temple',10),
  ('sikhism','gurdwara','Gurdwara',10)
) AS v(tradition_code, code, name, so)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL;

-- ===================================================================================================
-- religion_service_types — the per-tradition service/observance catalog (D-Code / D-i18n).
-- ===================================================================================================
CREATE TABLE oikumenea.religion_service_types (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,12),  -- religion / object / service_type
  tradition_taxon_id uuid REFERENCES oikumenea.religion_taxa(id) ON DELETE RESTRICT,  -- NULL = generic
  code               text NOT NULL,
  name               text NOT NULL,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order         integer,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT religion_service_types_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=12)
);
CREATE UNIQUE INDEX religion_service_types_code_active
  ON oikumenea.religion_service_types (tradition_taxon_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_service_types_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_service_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_service_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_service_types.tradition_taxon_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_service_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_service_types.name IS 'pii:none';

-- Generic + per-tradition seed.
INSERT INTO oikumenea.religion_service_types (tradition_taxon_id, code, name, sort_order) VALUES
  (NULL,'main','Main service',10),
  (NULL,'youth','Youth service',20),
  (NULL,'prayer','Prayer',30),
  (NULL,'special','Special service',40);
INSERT INTO oikumenea.religion_service_types (tradition_taxon_id, code, name, sort_order)
SELECT t.id, v.code, v.name, v.so
FROM (VALUES
  ('christianity','daily_mass','Daily Mass',50),
  ('islam','jumua','Friday (Jumuʿah) prayer',50),
  ('judaism','shabbat','Shabbat service',50),
  ('hinduism','puja','Puja',50),
  ('buddhism','meditation','Meditation',50)
) AS v(tradition_code, code, name, so)
JOIN oikumenea.religion_taxa t ON t.code = v.tradition_code AND t.deleted_at IS NULL;

-- i18n: site-type ("Типи об’єктів") + service-type ("Типи служінь") names in every enabled locale. eng
-- is the seed name column; ukr/spa/por are curated. The default-locale (ukr) row overrides the English
-- name column that LabelsByID otherwise assigns to the default locale. Codes shared across traditions
-- (e.g. 'temple') resolve to every matching row via the JOIN — the shared label applies to all.
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', 'eng', s.name
FROM oikumenea.religion_site_types s
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('office','ukr','Офіс'),          ('office','spa','Oficina'),      ('office','por','Escritório'),
  ('online','ukr','Онлайн'),        ('online','spa','En línea'),     ('online','por','Online'),
  ('mission','ukr','Місія'),        ('mission','spa','Misión'),      ('mission','por','Missão'),
  ('shrine','ukr','Святиня'),       ('shrine','spa','Santuario'),    ('shrine','por','Santuário'),
  ('church','ukr','Церква'),        ('church','spa','Iglesia'),      ('church','por','Igreja'),
  ('cathedral','ukr','Собор'),      ('cathedral','spa','Catedral'),  ('cathedral','por','Catedral'),
  ('chapel','ukr','Каплиця'),       ('chapel','spa','Capilla'),      ('chapel','por','Capela'),
  ('monastery','ukr','Монастир'),   ('monastery','spa','Monasterio'),('monastery','por','Mosteiro'),
  ('mosque','ukr','Мечеть'),        ('mosque','spa','Mezquita'),     ('mosque','por','Mesquita'),
  ('synagogue','ukr','Синагога'),   ('synagogue','spa','Sinagoga'),  ('synagogue','por','Sinagoga'),
  ('temple','ukr','Храм'),          ('temple','spa','Templo'),       ('temple','por','Templo'),
  ('gurdwara','ukr','Гурдвара'),    ('gurdwara','spa','Gurdwara'),   ('gurdwara','por','Gurdwara')
) AS v(code, locale, text)
JOIN oikumenea.religion_site_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', 'eng', s.name
FROM oikumenea.religion_service_types s
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('main','ukr','Головне богослужіння'),   ('main','spa','Servicio principal'), ('main','por','Serviço principal'),
  ('youth','ukr','Молодіжне богослужіння'),('youth','spa','Servicio juvenil'),  ('youth','por','Serviço jovem'),
  ('prayer','ukr','Молитва'),              ('prayer','spa','Oración'),          ('prayer','por','Oração'),
  ('special','ukr','Особливе богослужіння'),('special','spa','Servicio especial'),('special','por','Serviço especial'),
  ('daily_mass','ukr','Щоденна меса'),     ('daily_mass','spa','Misa diaria'),  ('daily_mass','por','Missa diária'),
  ('jumua','ukr','П’ятнична молитва (джума)'),('jumua','spa','Oración del viernes (yumu''a)'),('jumua','por','Oração de sexta-feira (jumuʿah)'),
  ('shabbat','ukr','Шабатнє богослужіння'),('shabbat','spa','Servicio de Shabat'),('shabbat','por','Serviço de Shabat'),
  ('puja','ukr','Пуджа'),                  ('puja','spa','Puyá'),               ('puja','por','Puja'),
  ('meditation','ukr','Медитація'),        ('meditation','spa','Meditación'),   ('meditation','por','Meditação')
) AS v(code, locale, text)
JOIN oikumenea.religion_service_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ===================================================================================================
-- religion_sites — the reified Link link__site_of (worship-community Unit ↔ a shared Location). One
-- shared location may be published at different precisions by different owners (the precision lives on
-- the site link, not the location). Unit-scoped via org_unit_id → carries the RLS backstop (below).
-- ===================================================================================================
CREATE TABLE oikumenea.religion_sites (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,2,4),  -- religion / link / site_of
  org_unit_id      uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  location_id      uuid NOT NULL REFERENCES oikumenea.location_locations(id) ON DELETE RESTRICT,
  site_type_id     uuid NOT NULL REFERENCES oikumenea.religion_site_types(id) ON DELETE RESTRICT,
  visibility       text NOT NULL DEFAULT 'public'
                   CHECK (visibility IN ('public','unlisted','private')),
  public_precision text NOT NULL DEFAULT 'exact'
                   CHECK (public_precision IN ('exact','street','neighborhood','city','hidden')),
  is_primary       boolean NOT NULL DEFAULT false,
  -- native validity (D-Temporal, R-31): the interval this site-of holds; NULL valid_to = active.
  valid_from       timestamptz NOT NULL DEFAULT now(),
  valid_to         timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT religion_sites_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=4)
);
-- Exactly one primary site per org unit (among active rows).
CREATE UNIQUE INDEX religion_sites_one_primary
  ON oikumenea.religion_sites (org_unit_id) WHERE is_primary AND deleted_at IS NULL;
CREATE INDEX religion_sites_unit_idx
  ON oikumenea.religion_sites (org_unit_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_sites_location_idx
  ON oikumenea.religion_sites (location_id) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_sites_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_sites
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_sites.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_sites.org_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_sites.location_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_sites.site_type_id IS 'pii:none';

-- ===================================================================================================
-- religion_service_schedules — per-site recurring service times (weekly default day OR an RRULE subset).
-- A site's services drive the discovery language/day/online filters. Cascades with its site.
-- ===================================================================================================
CREATE TABLE oikumenea.religion_service_schedules (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,13),  -- religion / object / service_schedule
  site_id         uuid NOT NULL REFERENCES oikumenea.religion_sites(id) ON DELETE CASCADE,
  service_type_id uuid NOT NULL REFERENCES oikumenea.religion_service_types(id) ON DELETE RESTRICT,
  day_of_week     smallint CHECK (day_of_week BETWEEN 0 AND 6),  -- 0=Sunday … 6=Saturday; NULL when rrule-driven
  rrule           text,                                          -- an RRULE subset; NULL when day_of_week-driven
  start_time      time,
  end_time        time,
  timezone        text NOT NULL,                                 -- IANA zone (e.g. Europe/Kyiv), not a UTC offset
  language        text,                                          -- service language (ISO 639-3); drives the language filter
  mode            text NOT NULL DEFAULT 'in_person'
                  CHECK (mode IN ('in_person','online','hybrid')),
  meeting_url     text,                                          -- required by the application when mode in (online,hybrid)
  description     text,                                          -- translatable
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  CONSTRAINT religion_service_schedules_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=13),
  -- a schedule is recurring either by weekday or by RRULE — at least one is present.
  CONSTRAINT religion_service_schedules_recurrence
    CHECK (day_of_week IS NOT NULL OR rrule IS NOT NULL)
);
CREATE INDEX religion_service_schedules_site_idx
  ON oikumenea.religion_service_schedules (site_id) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_service_schedules_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_service_schedules
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_service_schedules.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_service_schedules.site_id IS 'pii:none';

-- ===================================================================================================
-- religion_aliases — search-only alternative names for an org unit (NEVER displayed). They let a query
-- match a nickname / abbreviation / historical name / common misspelling / transliteration regardless
-- of the searcher's locale. Cascades with its unit.
-- ===================================================================================================
CREATE TABLE oikumenea.religion_aliases (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(16,1,14),  -- religion / object / alias
  unit_id    uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE CASCADE,
  alias_text text NOT NULL,
  alias_type text NOT NULL
             CHECK (alias_type IN ('nickname','abbreviation','historical','misspelling','transliteration')),
  locale     text,  -- optional per-locale alias (ISO 639-3)
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT religion_aliases_rid_shape
    CHECK (oikumenea.rid_service(id)=16 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=14)
);
CREATE INDEX religion_aliases_unit_idx
  ON oikumenea.religion_aliases (unit_id) WHERE deleted_at IS NULL;
CREATE INDEX religion_aliases_text_idx
  ON oikumenea.religion_aliases (lower(alias_text)) WHERE deleted_at IS NULL;
CREATE TRIGGER religion_aliases_set_updated_at
  BEFORE UPDATE ON oikumenea.religion_aliases
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.religion_aliases.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_aliases.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_aliases.alias_text IS 'pii:none';

-- ===================================================================================================
-- RLS backstop (D-RLSDefenseInDepth) on the unit-scoped discovery tables. The app-layer PDP over the
-- canonical graph + the shadow gate remain AUTHORITATIVE; the catalogs (site/service types) are
-- reference data and carry NO RLS. religion_service_schedules is reached through its site's org_unit_id.
-- ===================================================================================================
ALTER TABLE oikumenea.religion_sites ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.religion_sites FORCE ROW LEVEL SECURITY;
CREATE POLICY religion_sites_reach ON oikumenea.religion_sites
  USING (oikumenea.authz_unit_in_reach(org_unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(org_unit_id, true));

ALTER TABLE oikumenea.religion_aliases ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.religion_aliases FORCE ROW LEVEL SECURITY;
CREATE POLICY religion_aliases_reach ON oikumenea.religion_aliases
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

ALTER TABLE oikumenea.religion_service_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.religion_service_schedules FORCE ROW LEVEL SECURITY;
CREATE POLICY religion_service_schedules_reach ON oikumenea.religion_service_schedules
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR EXISTS (SELECT 1 FROM oikumenea.religion_sites s
                    WHERE s.id = site_id
                      AND oikumenea.authz_unit_in_reach(s.org_unit_id, false)))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR EXISTS (SELECT 1 FROM oikumenea.religion_sites s
                    WHERE s.id = site_id
                      AND oikumenea.authz_unit_in_reach(s.org_unit_id, true)));

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0026_religion_discovery', applied_at = now() WHERE singleton;
