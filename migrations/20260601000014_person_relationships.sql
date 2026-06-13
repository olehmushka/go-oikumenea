-- 0014 person↔person relationships (M14).
--
-- Person enrichment (docs/modules/person.md, D-PersonRelationships): family and social structure
-- between two in-directory persons, modelled as PER-TYPE reified self-links (Person → Person, D-Ontology
-- link__<type>) — never one generic table, never a bare FK. Each mirrors the membership_memberships
-- temporal-link shape (RID PK, soft-delete, set_updated_at, effective interval + status where a lifecycle
-- applies), is instance-global, holder-scoped on read (D-PersonReadScope), audited on write, and erased
-- when EITHER endpoint person purges. Authority NEVER derives from a relationship (D-Rank) — directory data.
--
--   person_relation_types  — instance-admin catalog for the open-ended relation labels (natural `code` PK
--                            carve-out, like person_platforms; category sponsorship|association|next_of_kin;
--                            translatable name in the localization store entity_type='relation_type').
--   person_partnerships     — marriage + engagement, symmetric canonical pair; link__partnered_with.
--   person_kinships         — directional parent_of (siblings derived); link__kin_parent_of.
--   person_guardianships    — guardian → ward (distinct from blood kin); link__guardian_of.
--   person_sponsorships     — godparent / advisor / mentor (catalog-typed); link__sponsor_of.
--   person_next_of_kin      — in-directory nomination + priority; link__next_of_kin.
--   person_associations     — associate / COI / no-contact, symmetric; link__associated_with.
--
-- (A friend/follower person_social_links / link__social_tie was scoped but DEFERRED — see decisions.md
-- D-PersonRelationships: no consumer, no authoritative source, and redundant with person_associations
-- for the actionable COI/no-contact case; it returns only with a real account-level model.)
--
-- These tables have NO unit column (scoped through the holder per D-PersonReadScope), so — like
-- person_persons / person_emails / person_social_accounts — they are EXEMPT from the RLS
-- app.readable_units backstop (D-RLSDefenseInDepth); no RLS is enabled on them.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid, set_updated_at),
-- 0006 person (person_persons). person_relation_types is seeded here (natural-key); the link rows are
-- created through PersonService.

-- ============================ relation-type catalog ============================

-- person_relation_types: instance-admin catalog of open-ended person↔person relation labels (D-Code/D-i18n).
-- Natural `code` PK (carve-out, like person_platforms). name is the default-locale label; other locales
-- live in the localization store (entity_type='relation_type'). category scopes which link type a label
-- applies to (sponsorship/association/next_of_kin); fixed lifecycle statuses (partnership, kinship) stay
-- TEXT+CHECK on their own tables and do NOT use this catalog.
CREATE TABLE oikumenea.person_relation_types (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  category   text NOT NULL CHECK (category IN ('sponsorship','association','next_of_kin')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER person_relation_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_relation_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.person_relation_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.category IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.sort_order IS 'pii:none';

-- Seed the relation-type catalog (natural-key carve-out). The instance admin adds more via the API.
-- Sponsorship labels are required (person_sponsorships.relation_code is NOT NULL).
INSERT INTO oikumenea.person_relation_types (code, name, category, sort_order) VALUES
  ('godparent',        'Godparent',         'sponsorship',   0),
  ('academic_advisor', 'Academic advisor',  'sponsorship',  10),
  ('military_mentor',  'Military mentor',    'sponsorship',  20),
  ('spouse',           'Spouse',             'next_of_kin',  30),
  ('parent',           'Parent',             'next_of_kin',  40),
  ('child',            'Child',              'next_of_kin',  50),
  ('sibling',          'Sibling',            'next_of_kin',  60),
  ('next_of_kin_other','Other (next of kin)','next_of_kin',  70),
  ('colleague',        'Colleague',          'association',  80),
  ('business_associate','Business associate','association',  90);

-- ============================ partnerships (marriage + engagement) ============================

-- person_partnerships: marriage AND engagement folded into one lifecycle (D-PersonRelationships). A
-- SYMMETRIC pair stored in canonical order (person_id_a < person_id_b, CHECK; no self-pair). At most one
-- active engaged-or-married row per person — the active-pair partial-unique index below stops a duplicate
-- between the SAME two people; the broader "one active per person with anyone" is enforced in the
-- application (a partial-unique index cannot span both columns). effective_to NULL = ongoing. Link
-- link__partnered_with. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_partnerships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,2),  -- person / link / partnered_with
  person_id_a    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  person_id_b    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  status         text NOT NULL CHECK (status IN ('engaged','married','divorced','widowed','annulled','dissolved')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_partnerships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2),
  CONSTRAINT person_partnerships_canonical_pair CHECK (person_id_a < person_id_b)
);

CREATE TRIGGER person_partnerships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_partnerships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_partnerships_active_pair_idx
  ON oikumenea.person_partnerships (person_id_a, person_id_b)
  WHERE deleted_at IS NULL AND status IN ('engaged','married');
CREATE INDEX person_partnerships_person_a_idx
  ON oikumenea.person_partnerships (person_id_a) WHERE deleted_at IS NULL;
CREATE INDEX person_partnerships_person_b_idx
  ON oikumenea.person_partnerships (person_id_b) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_partnerships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.person_id_a IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.person_id_b IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_partnerships.effective_to IS 'pii:basic';

-- ============================ kinships (directional parentage) ============================

-- person_kinships: directional blood/legal parentage (parent_id → child_id; D-PersonRelationships).
-- Siblings are DERIVED (shared parent), never stored. Distinct RID from tenant's unit link__parent_of.
-- Link link__kin_parent_of. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_kinships (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,3),  -- person / link / kin_parent_of
  parent_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  child_id   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disestablished')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_kinships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3),
  CONSTRAINT person_kinships_no_self CHECK (parent_id <> child_id)
);

CREATE TRIGGER person_kinships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_kinships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_kinships_active_pair_idx
  ON oikumenea.person_kinships (parent_id, child_id) WHERE deleted_at IS NULL;
CREATE INDEX person_kinships_parent_idx
  ON oikumenea.person_kinships (parent_id) WHERE deleted_at IS NULL;
CREATE INDEX person_kinships_child_idx
  ON oikumenea.person_kinships (child_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_kinships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.child_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.status IS 'pii:none';

-- ============================ guardianships (legal guardian → ward) ============================

-- person_guardianships: legal guardian → ward, distinct from blood parent_of (D-PersonRelationships).
-- relation_code is an optional catalog label. effective_to NULL = ongoing. Link link__guardian_of.
-- Erased when either endpoint purges.
CREATE TABLE oikumenea.person_guardianships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,4),  -- person / link / guardian_of
  guardian_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  ward_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code  text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_guardianships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=4),
  CONSTRAINT person_guardianships_no_self CHECK (guardian_id <> ward_id)
);

CREATE TRIGGER person_guardianships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_guardianships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_guardianships_active_pair_idx
  ON oikumenea.person_guardianships (guardian_id, ward_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_guardianships_guardian_idx
  ON oikumenea.person_guardianships (guardian_id) WHERE deleted_at IS NULL;
CREATE INDEX person_guardianships_ward_idx
  ON oikumenea.person_guardianships (ward_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_guardianships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.guardian_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.ward_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_guardianships.effective_to IS 'pii:basic';

-- ============================ sponsorships (godparent / advisor / mentor) ============================

-- person_sponsorships: sponsor → sponsored, catalog-typed relation kind (godparent / academic advisor /
-- military mentor; D-PersonRelationships). relation_code is REQUIRED and must reference a
-- category='sponsorship' relation type (the category is enforced in the application). effective_to NULL =
-- ongoing. Link link__sponsor_of. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_sponsorships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,5),  -- person / link / sponsor_of
  sponsor_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  sponsored_id   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code  text NOT NULL REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_sponsorships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=5),
  CONSTRAINT person_sponsorships_no_self CHECK (sponsor_id <> sponsored_id)
);

CREATE TRIGGER person_sponsorships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_sponsorships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_sponsorships_active_idx
  ON oikumenea.person_sponsorships (sponsor_id, sponsored_id, relation_code)
  WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_sponsorships_sponsor_idx
  ON oikumenea.person_sponsorships (sponsor_id) WHERE deleted_at IS NULL;
CREATE INDEX person_sponsorships_sponsored_idx
  ON oikumenea.person_sponsorships (sponsored_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_sponsorships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.sponsor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.sponsored_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_sponsorships.effective_to IS 'pii:basic';

-- ============================ next of kin (in-directory nomination) ============================

-- person_next_of_kin: subject → contact, both in-directory (D-PersonRelationships). A NOMINATION (with a
-- priority ordering), not a blood fact; external free-text contacts are out of scope (both ends must be
-- directory persons). relation_code is an optional category='next_of_kin' catalog label (enforced in the
-- application). Link link__next_of_kin. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_next_of_kin (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,6),  -- person / link / next_of_kin
  subject_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  contact_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  priority      int  NOT NULL DEFAULT 1,
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','withdrawn')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_next_of_kin_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=6),
  CONSTRAINT person_next_of_kin_no_self CHECK (subject_id <> contact_id)
);

CREATE TRIGGER person_next_of_kin_set_updated_at
  BEFORE UPDATE ON oikumenea.person_next_of_kin
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_next_of_kin_active_pair_idx
  ON oikumenea.person_next_of_kin (subject_id, contact_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_next_of_kin_subject_idx
  ON oikumenea.person_next_of_kin (subject_id) WHERE deleted_at IS NULL;
CREATE INDEX person_next_of_kin_contact_idx
  ON oikumenea.person_next_of_kin (contact_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_next_of_kin.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.subject_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.contact_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.priority IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.status IS 'pii:none';

-- ============================ associations (associate / COI / no-contact) ============================

-- person_associations: symmetric associate ↔ associate (canonical pair person_id_a < person_id_b;
-- D-PersonRelationships). kind partitions plain association / conflict-of-interest / prohibited-contact
-- (discipline). relation_code is an optional category='association' catalog label (enforced in the
-- application). Link link__associated_with. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_associations (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,7),  -- person / link / associated_with
  person_id_a   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  person_id_b   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  kind          text NOT NULL CHECK (kind IN ('associate','coi','no_contact')),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_associations_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=7),
  CONSTRAINT person_associations_canonical_pair CHECK (person_id_a < person_id_b)
);

CREATE TRIGGER person_associations_set_updated_at
  BEFORE UPDATE ON oikumenea.person_associations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_associations_active_pair_idx
  ON oikumenea.person_associations (person_id_a, person_id_b, kind) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_associations_person_a_idx
  ON oikumenea.person_associations (person_id_a) WHERE deleted_at IS NULL;
CREATE INDEX person_associations_person_b_idx
  ON oikumenea.person_associations (person_id_b) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_associations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.person_id_a IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.person_id_b IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.status IS 'pii:none';

-- (person_social_links / link__social_tie deferred — see the header note and decisions.md.)

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0014_person_relationships', applied_at = now() WHERE singleton;
