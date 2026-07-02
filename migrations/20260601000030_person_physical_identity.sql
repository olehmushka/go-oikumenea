-- 0030 person physical identity (M31 — D-PhysicalIdentity) + structural color (M42 — D-Color) + ethnicity
-- taxonomy (M43 — D-PhysicalIdentity amendment). These three uncommitted milestones are SQUASHED into one
-- migration: tables are created in their FINAL shape (dependencies first) so there are no create-then-alter
-- churn cycles. The end-state schema is byte-identical to applying the former 0030+0031+0032 in sequence.
--
-- Moving parts:
--   1. ALIASES fold into the existing person_name_variants via a variant_kind discriminator
--      (transliteration | aka | former_legal | maiden | pseudonym | cover) — NO new table; the original
--      one-transliteration-per-locale rule becomes PARTIAL to variant_kind='transliteration'.
--   2. platform_colors (M42 / D-Color) — a platform-owned RID-bearing per-domain color catalog
--      (eye | hair | vehicle), referenced by HARD FK. Created FIRST so the person + vehicle FKs resolve.
--   3. person_physical_descriptions — effective-dated height/weight/build/blood_type (pii:basic) +
--      eye_color_id/hair_color_id HARD FKs -> platform_colors (no free-text color columns).
--   4. person_distinguishing_marks — tattoo/scar/piercing/birthmark; pii:special CEILING.
--   5. person_ethnicity_types (M31 catalog, M43 hierarchy) — self-declared vocabulary with parent_id +
--      transitive closure + group-level language/country M:N ties + Wikidata anchor + import provenance.
--   6. person_ethnicities — the reified pii:special link; the declared catalog code is ENVELOPE-ENCRYPTED
--      with a blind index (D-SpecialPII) + NOT NULL legal_basis; crypto-erased on purge. Biometrics EXCLUDED.
--   7. vehicle_vehicles gains color_id (HARD FK -> platform_colors); its legacy free-text color is dropped
--      (vehicle_vehicles is from committed 0027, so this stays an ALTER).
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only where it adds (L-UpgradeSafe / D-Migrations);
-- depends on 0000 (new_id / platform_rid_types), 0005 person (person_persons / person_name_variants),
-- 0018 language (language_languoids), 0027 vehicle (vehicle_vehicles), 0028 overlay
-- (platform_legal_basis_kinds), and the seeded geo_countries.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot (kind<>3).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (1,1,1,'color'),                    -- platform / object / color (M42)
  (6,1,11,'physical_description'),
  (6,1,12,'distinguishing_mark'),
  (6,1,13,'ethnicity_type'),
  (6,2,9,'has_ethnicity');

-- ===================================================================================================
-- platform_colors — the per-domain color catalog (D-Color / D-Code / D-i18n). Created BEFORE the person
-- + vehicle tables that hard-FK it. `name` is the default-locale label (full i18n overlaid from the
-- localization store, keyed by the color RID); `hex` is a NULLABLE representative swatch.
-- ===================================================================================================
CREATE TABLE oikumenea.platform_colors (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(1,1,1),  -- platform / object / color
  domain     text NOT NULL CHECK (domain IN ('eye','hair','vehicle','rank','religion','ethnicity','country')),  -- +pinax palettes (D-Pinax, M45)
  code       text NOT NULL,
  name       text NOT NULL,  -- default-locale label; full i18n overlaid from the localization store
  hex        text CHECK (hex IS NULL OR hex ~ '^#[0-9A-Fa-f]{6}$'),  -- nullable representative swatch
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT platform_colors_rid_shape
    CHECK (oikumenea.rid_service(id)=1 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX platform_colors_domain_code_active
  ON oikumenea.platform_colors (domain, code) WHERE deleted_at IS NULL;
CREATE INDEX platform_colors_domain_idx
  ON oikumenea.platform_colors (domain) WHERE deleted_at IS NULL;
CREATE TRIGGER platform_colors_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_colors
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.platform_colors.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.domain IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_colors.hex IS 'pii:none';

-- Seed the baseline palettes. eye/hair are categorical (no hex); vehicle carries representative hex.
INSERT INTO oikumenea.platform_colors (domain, code, name, hex, sort_order) VALUES
  -- eye colors (categorical)
  ('eye', 'amber',  'Amber',  NULL, 10),
  ('eye', 'blue',   'Blue',   NULL, 20),
  ('eye', 'brown',  'Brown',  NULL, 30),
  ('eye', 'gray',   'Gray',   NULL, 40),
  ('eye', 'green',  'Green',  NULL, 50),
  ('eye', 'hazel',  'Hazel',  NULL, 60),
  ('eye', 'black',  'Black',  NULL, 70),
  -- hair colors (categorical)
  ('hair', 'auburn', 'Auburn', NULL, 10),
  ('hair', 'black',  'Black',  NULL, 20),
  ('hair', 'blonde', 'Blonde', NULL, 30),
  ('hair', 'brown',  'Brown',  NULL, 40),
  ('hair', 'gray',   'Gray',   NULL, 50),
  ('hair', 'red',    'Red',    NULL, 60),
  ('hair', 'white',  'White',  NULL, 70),
  ('hair', 'bald',   'Bald',   NULL, 80),
  -- vehicle colors (with representative hex swatches)
  ('vehicle', 'black',  'Black',  '#000000', 10),
  ('vehicle', 'white',  'White',  '#FFFFFF', 20),
  ('vehicle', 'gray',   'Gray',   '#808080', 30),
  ('vehicle', 'silver', 'Silver', '#C0C0C0', 40),
  ('vehicle', 'blue',   'Blue',   '#0033A0', 50),
  ('vehicle', 'red',    'Red',    '#C8102E', 60),
  ('vehicle', 'green',  'Green',  '#2E7D32', 70),
  ('vehicle', 'yellow', 'Yellow', '#F2C200', 80),
  ('vehicle', 'brown',  'Brown',  '#6D4C41', 90);

-- ===================================================================================================
-- 1. Aliases fold into person_name_variants (no new table). This ALTERs a table from committed 0005.
-- ===================================================================================================
ALTER TABLE oikumenea.person_name_variants
  ADD COLUMN variant_kind text NOT NULL DEFAULT 'transliteration'
    CHECK (variant_kind IN ('transliteration','aka','former_legal','maiden','pseudonym','cover')),
  ADD COLUMN source     text,   -- attribution (D-OverlayFoundation) — alias provenance
  ADD COLUMN confidence text;

COMMENT ON COLUMN oikumenea.person_name_variants.variant_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_name_variants.confidence IS 'pii:none';

-- The original UNIQUE (person_id, locale) modelled "one canonical transliteration per locale". With
-- aliases now sharing the table that rule must apply ONLY to transliterations; aliases are unconstrained
-- (a person may have several AKAs / cover names). Replace the total constraint with a partial index.
ALTER TABLE oikumenea.person_name_variants
  DROP CONSTRAINT person_name_variants_person_locale_uniq;
CREATE UNIQUE INDEX person_name_variants_translit_uniq
  ON oikumenea.person_name_variants (person_id, locale) WHERE variant_kind = 'transliteration';

-- ===================================================================================================
-- 2. person_physical_descriptions — effective-dated physical description (pii:basic). eye/hair color are
--    HARD FKs into platform_colors (D-Color); the FK columns are LAST (parity with the former ALTER-add
--    ordering, so the generated row shape is unchanged).
-- ===================================================================================================
CREATE TABLE oikumenea.person_physical_descriptions (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,11),  -- person / object / physical_description
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  height_cm      integer CHECK (height_cm IS NULL OR (height_cm > 0 AND height_cm < 300)),
  weight_kg      integer CHECK (weight_kg IS NULL OR (weight_kg > 0 AND weight_kg < 700)),
  build          text,   -- advisory free text (e.g. slim, athletic, heavy)
  blood_type     text CHECK (blood_type IS NULL OR blood_type IN
                   ('A+','A-','B+','B-','AB+','AB-','O+','O-','unknown')),
  effective_from date NOT NULL DEFAULT (now() AT TIME ZONE 'UTC'),
  effective_to   date,   -- NULL = current
  source         text,
  confidence     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  eye_color_id   uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT,  -- domain='eye' (D-Color)
  hair_color_id  uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT,  -- domain='hair' (D-Color)
  CONSTRAINT person_physical_descriptions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=11)
);

CREATE TRIGGER person_physical_descriptions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_physical_descriptions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_physical_descriptions_person_idx
  ON oikumenea.person_physical_descriptions (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_physical_descriptions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.height_cm IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.weight_kg IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.build IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.blood_type IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.effective_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.eye_color_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_physical_descriptions.hair_color_id IS 'pii:basic';

-- ===================================================================================================
-- 3. person_distinguishing_marks — tattoos/scars/piercings/birthmarks (pii:special CEILING).
-- ===================================================================================================
CREATE TABLE oikumenea.person_distinguishing_marks (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,12),  -- person / object / distinguishing_mark
  person_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  kind          text NOT NULL CHECK (kind IN ('tattoo','scar','piercing','birthmark')),
  body_location text,   -- e.g. left forearm; free text
  description   text,   -- the mark's appearance; a tattoo may reveal Art. 9 data -> pii:special
  source        text,
  confidence    text,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT person_distinguishing_marks_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=12)
);

CREATE TRIGGER person_distinguishing_marks_set_updated_at
  BEFORE UPDATE ON oikumenea.person_distinguishing_marks
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_distinguishing_marks_person_idx
  ON oikumenea.person_distinguishing_marks (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_distinguishing_marks.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.kind IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.body_location IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_distinguishing_marks.description IS 'pii:special';

-- ===================================================================================================
-- 4a. person_ethnicity_types — the open, instance-admin-managed declared-ethnicity vocabulary, promoted
--     to a HIERARCHICAL catalog (M43 / D-PhysicalIdentity amendment): parent_id self-FK + transitive
--     closure (below) + Wikidata anchor + import provenance. Plaintext (a controlled vocabulary, not a
--     person's datum); a person's SELECTION is the Art. 9 datum, encrypted in person_ethnicities. Seeded
--     EMPTY (ethnicity is contentious; loaded on purpose via the opt-in `ethnicity-scheme` import). The
--     hierarchy/provenance columns are LAST (parity with the former ALTER-add ordering).
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicity_types (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,13),  -- person / object / ethnicity_type
  code           text NOT NULL,
  name           text NOT NULL,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order     integer,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  parent_id      uuid REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE RESTRICT,  -- "" = root (M43)
  wikidata_id    text,
  source         text,
  source_version text,
  imported_at    timestamptz,
  CONSTRAINT person_ethnicity_types_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=13)
);
CREATE UNIQUE INDEX person_ethnicity_types_code_active
  ON oikumenea.person_ethnicity_types (code) WHERE deleted_at IS NULL;
CREATE INDEX person_ethnicity_types_parent_idx
  ON oikumenea.person_ethnicity_types (parent_id) WHERE deleted_at IS NULL;
CREATE TRIGGER person_ethnicity_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_ethnicity_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_ethnicity_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.wikidata_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.source_version IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.imported_at IS 'pii:none';

-- ===================================================================================================
-- 4b. person_ethnicities — the reified pii:special Link link__has_ethnicity (Person → a declared
--     ethnicity). The declared value (the catalog code) is ENVELOPE-ENCRYPTED with a blind index so the
--     Art. 9 datum never sits in plaintext; value_blind_index = BlindIndex(code) enables equality search
--     ("who declared ethnicity X") without decryption. There is NO plaintext FK to person_ethnicity_types
--     for that reason — the application validates the code against the catalog before sealing. A NOT NULL
--     legal_basis records the lawful basis (D-OverlayFoundation art9 condition). Crypto-erased on purge.
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicities (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,9),  -- person / link / has_ethnicity
  person_id         uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  -- envelope-encrypted declared ethnicity code (D-SpecialPII; same shape as religion_affiliations).
  -- Always populated at creation (the application requires a value), but NULLABLE so crypto-erase on
  -- purge can drop all four (the row survives as a tombstone).
  value_ciphertext  bytea,
  wrapped_dek       bytea,
  key_ref           text,
  value_blind_index bytea,
  legal_basis       text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  source            text,
  confidence        text,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT person_ethnicities_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=9)
);
CREATE INDEX person_ethnicities_person_idx
  ON oikumenea.person_ethnicities (person_id) WHERE deleted_at IS NULL;
CREATE INDEX person_ethnicities_blind_idx
  ON oikumenea.person_ethnicities (value_blind_index) WHERE deleted_at IS NULL;
CREATE TRIGGER person_ethnicities_set_updated_at
  BEFORE UPDATE ON oikumenea.person_ethnicities
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.person_ethnicities.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.value_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.value_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_ethnicities.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicities.status IS 'pii:none';

-- ===================================================================================================
-- 4c. Ethnicity taxonomy aux tables (M43): the transitive closure + the group-level ethnolinguistic
--     (→ language_languoids) and homeland (→ geo_countries) M:N ties. Bare relations — composite keys,
--     NO RID (mirror language_languoid_closure / _countries). Group metadata; NEVER inferred onto a person.
-- ===================================================================================================
CREATE TABLE oikumenea.person_ethnicity_type_closure (
  ancestor_id   uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  descendant_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  depth         integer NOT NULL,
  PRIMARY KEY (ancestor_id, descendant_id)
);
CREATE INDEX person_ethnicity_type_closure_descendant_idx
  ON oikumenea.person_ethnicity_type_closure (descendant_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.ancestor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.descendant_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_closure.depth IS 'pii:none';

CREATE TABLE oikumenea.person_ethnicity_type_languages (
  ethnicity_type_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  language_id       uuid NOT NULL REFERENCES oikumenea.language_languoids(id) ON DELETE RESTRICT,
  PRIMARY KEY (ethnicity_type_id, language_id)
);
CREATE INDEX person_ethnicity_type_languages_language_idx
  ON oikumenea.person_ethnicity_type_languages (language_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_languages.ethnicity_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_languages.language_id IS 'pii:none';

CREATE TABLE oikumenea.person_ethnicity_type_countries (
  ethnicity_type_id uuid NOT NULL REFERENCES oikumenea.person_ethnicity_types(id) ON DELETE CASCADE,
  country_id        uuid NOT NULL REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  PRIMARY KEY (ethnicity_type_id, country_id)
);
CREATE INDEX person_ethnicity_type_countries_country_idx
  ON oikumenea.person_ethnicity_type_countries (country_id);
COMMENT ON COLUMN oikumenea.person_ethnicity_type_countries.ethnicity_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_type_countries.country_id IS 'pii:none';

-- ===================================================================================================
-- 5. vehicle_vehicles.color (text) -> color_id (HARD FK into platform_colors, D-Color). vehicle_vehicles
--    is from committed 0027, so this stays an ALTER: add color_id, best-effort backfill, drop color.
-- ===================================================================================================
ALTER TABLE oikumenea.vehicle_vehicles
  ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
UPDATE oikumenea.vehicle_vehicles v
  SET color_id = c.id
  FROM oikumenea.platform_colors c
  WHERE c.domain = 'vehicle' AND c.deleted_at IS NULL
    AND v.color IS NOT NULL AND lower(btrim(v.color)) = c.code;
ALTER TABLE oikumenea.vehicle_vehicles DROP COLUMN color;
CREATE INDEX vehicle_vehicles_color_idx
  ON oikumenea.vehicle_vehicles (color_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.vehicle_vehicles.color_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- pinax origin marker (D-Pinax, M45): 'seeded' = managed by a bundled preset (`colors` palettes;
-- `ethnicities` catalog via the import path), 'operator' = created via the admin API. platform_colors'
-- migration-seeded palettes are seeded-owned; person_ethnicity_types seeds empty (default 'operator',
-- and the ethnicity import handler stamps origin='seeded' on loaded rows — D-Pinax).
ALTER TABLE oikumenea.platform_colors        ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
ALTER TABLE oikumenea.person_ethnicity_types ADD COLUMN origin text NOT NULL DEFAULT 'operator' CHECK (origin IN ('seeded','operator'));
UPDATE oikumenea.platform_colors SET origin = 'seeded';
COMMENT ON COLUMN oikumenea.platform_colors.origin IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.origin IS 'pii:none';

-- pinax structural color (D-Pinax, M45 + D-Color): a nullable display color on the seeded reference
-- catalogs, referencing the shared platform_colors palette by hard FK. These ALTERs live here (not in
-- each table's own earlier migration) because platform_colors is created above in THIS migration — a
-- forward reference from 0000/0004/0023 would fail. Palette rows for the new domains arrive via the
-- bundled `colors` preset; the domain match (e.g. a country references a 'country'-domain color) is
-- validated app-side via the D-Color ColorLookup, as with eye/hair/vehicle.
ALTER TABLE oikumenea.geo_countries          ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.rank_ranks             ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.religion_taxa          ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
ALTER TABLE oikumenea.person_ethnicity_types ADD COLUMN color_id uuid REFERENCES oikumenea.platform_colors(id) ON DELETE RESTRICT;
COMMENT ON COLUMN oikumenea.geo_countries.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.rank_ranks.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.religion_taxa.color_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_ethnicity_types.color_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0030_person_physical_identity', applied_at = now() WHERE singleton;
