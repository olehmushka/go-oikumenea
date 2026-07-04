-- 0032 person institutional & political ties (M33 — D-InstitutionalTies). The draft's macro-category 7,
-- modelled as per-type reified person↔organization affiliation edges (not one generic "affiliation" blob),
-- so each edge keeps its own PII tier and attribute set — mirroring the M14 relationship pattern. Every
-- edge carries the D-OverlayFoundation attribution column-set (source/confidence). The org side is usually
-- an external body (M30 external_organizations); for OSINT-ingested rows it is a free-text label, optionally
-- resolvable to a polymorphic org RID later.
--
--   person_party_memberships       — party affiliation (political opinion). pii:special (GDPR Art. 9):
--                                    the party itself is envelope-encrypted (same shape as the M31
--                                    person_ethnicities / religion_affiliations), NOT-NULL legal_basis.
--   person_government_positions    — held public office; pep_trigger (auto-true, persists post-office) feeds
--                                    the M34 PEP check. pii:basic.
--   person_lobbying_relationships  — registrant↔client lobbying filings. pii:basic.
--   person_external_references     — reference object: wikipedia/news/registry/… links about a person
--                                    (mirrors person_social_accounts; a hermenea import target). pii:basic.
--
-- Foreign / historical military service reuses membership against external_organizations military stubs +
-- rank (no new table here); emergency contacts reuse the M14 person_relation_types catalog — this migration
-- only seeds the 'emergency' label (no new entity). Inferred political leaning is NOT here — it is a separate
-- M35 pii:special overlay, never merged with the declared party membership below.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons), 0014 (person_relation_types),
-- 0028 (platform_legal_basis_kinds), and 0001 (geo_countries).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,14,'external_reference'),    -- person / object / external_reference (M33)
  (6,2,11,'party_membership'),      -- person / link   / party_membership   (M33, encrypted)
  (6,2,12,'government_position'),   -- person / link   / government_position (M33)
  (6,2,13,'lobbying_rel');          -- person / link   / lobbying_rel        (M33)

-- ===================================================================================================
-- person_party_memberships — the reified pii:special Link link__party_membership. Political opinion is a
-- GDPR Art. 9 special category, so the party identity is envelope-encrypted (ciphertext/wrapped_dek/
-- key_ref/blind_index sealed in the application; NO plaintext party FK) with a NOT-NULL legal_basis. Role,
-- dates and attribution stay in plaintext (the sensitive datum is WHICH party, not the membership role).
-- Crypto-erased on purge (drop the envelope, keep the row tombstone).
-- ===================================================================================================
CREATE TABLE oikumenea.person_party_memberships (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,11),  -- person / link / party_membership
  person_id         uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  -- envelope-encrypted party identity (a party name or an external_organizations RID). Always populated at
  -- creation, but NULLABLE so crypto-erase on purge can drop all four (the row survives as a tombstone).
  party_ciphertext  bytea,
  party_wrapped_dek bytea,
  party_key_ref     text,
  party_blind_index bytea,
  role              text NOT NULL DEFAULT 'member'
                      CHECK (role IN ('member','official','candidate','donor','supporter','other')),
  valid_from        date,
  valid_to          date,          -- NULL = current
  legal_basis       text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  source            text NOT NULL DEFAULT 'operator_verified'
                      CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence        text NOT NULL DEFAULT 'possible'
                      CHECK (confidence IN ('confirmed','probable','possible')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT person_party_memberships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=11)
);

CREATE TRIGGER person_party_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_party_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_party_memberships_person_idx
  ON oikumenea.person_party_memberships (person_id) WHERE deleted_at IS NULL;
-- Blind-index lookup ("who else belongs to this party") without decrypting.
CREATE INDEX person_party_memberships_blind_idx
  ON oikumenea.person_party_memberships (party_blind_index) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_party_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.party_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_party_memberships.role IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_party_memberships.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_party_memberships.confidence IS 'pii:none';

-- ===================================================================================================
-- person_government_positions — the reified pii:basic Link link__government_position. Public office is
-- public-record, not special-category, so it is stored in plaintext. pep_trigger defaults true and PERSISTS
-- after the position ends (valid_to set) — a former official stays a PEP; the M34 watchlist check derives
-- PEP status from these rows. org_id is an optional, unvalidated polymorphic reference to the resolved body
-- (external_organizations | company | tenant_unit); body is the always-present free-text label.
-- ===================================================================================================
CREATE TABLE oikumenea.person_government_positions (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,12),  -- person / link / government_position
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  title        text NOT NULL,                                       -- e.g. "Minister of Defence", "Senator"
  body         text NOT NULL,                                       -- e.g. "Ministry of Defence", "US Senate"
  org_id       uuid,                                                -- optional resolved body RID (polymorphic; no FK)
  country_id   uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  level        text NOT NULL DEFAULT 'national'
                 CHECK (level IN ('international','national','regional','local')),
  role_type    text,                                                -- e.g. "elected", "appointed", "career_civil_service"
  valid_from   date,
  valid_to     date,          -- NULL = current
  pep_trigger  boolean NOT NULL DEFAULT true,                       -- politically-exposed person (persists post-office)
  source       text NOT NULL DEFAULT 'operator_verified'
                 CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence   text NOT NULL DEFAULT 'possible'
                 CHECK (confidence IN ('confirmed','probable','possible')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT person_government_positions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=12)
);

CREATE TRIGGER person_government_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_government_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_government_positions_person_idx
  ON oikumenea.person_government_positions (person_id) WHERE deleted_at IS NULL;
-- PEP derivation ("is this person politically exposed") reads active pep_trigger rows by person.
CREATE INDEX person_government_positions_pep_idx
  ON oikumenea.person_government_positions (person_id) WHERE pep_trigger AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_government_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.title IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.body IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.level IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.role_type IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_government_positions.pep_trigger IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_government_positions.confidence IS 'pii:none';

-- ===================================================================================================
-- person_lobbying_relationships — the reified pii:basic Link link__lobbying_rel. A public lobbying filing:
-- the person as registrant lobbying on behalf of a client before a legislative body, on a set of issues.
-- Free-text org sides (public-registry data); filing_id/source_url anchor the provenance.
-- ===================================================================================================
CREATE TABLE oikumenea.person_lobbying_relationships (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,13),  -- person / link / lobbying_rel
  person_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  registrant       text NOT NULL,                                      -- the registered lobbyist/firm
  client           text,                                               -- on whose behalf
  legislative_body text,                                               -- e.g. "US Congress", "European Parliament"
  issues           text[] NOT NULL DEFAULT '{}',                       -- policy areas
  filing_id        text,                                               -- the public filing identifier
  source_url       text,
  valid_from       date,
  valid_to         date,
  source           text NOT NULL DEFAULT 'operator_verified'
                     CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence       text NOT NULL DEFAULT 'possible'
                     CHECK (confidence IN ('confirmed','probable','possible')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,
  CONSTRAINT person_lobbying_relationships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=13)
);

CREATE TRIGGER person_lobbying_relationships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_lobbying_relationships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_lobbying_relationships_person_idx
  ON oikumenea.person_lobbying_relationships (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_lobbying_relationships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.registrant IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.client IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.legislative_body IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.issues IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.filing_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.source_url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.valid_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.valid_to IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_lobbying_relationships.confidence IS 'pii:none';

-- ===================================================================================================
-- person_external_references — the pii:basic Object external_reference. A pointer to an off-platform source
-- about a person (wikipedia/news/registry/social/court/other). Mirrors person_social_accounts and is a
-- hermenea import target; disputed flags a contested reference. Not a Link (it points at a URL, not a
-- reified relationship with another Object).
-- ===================================================================================================
CREATE TABLE oikumenea.person_external_references (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,14),  -- person / object / external_reference
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  kind         text NOT NULL DEFAULT 'other'
                 CHECK (kind IN ('wikipedia','news','registry','social','court','academic','other')),
  url          text NOT NULL,
  external_id  text,                                                -- the id within the source system
  categories   text[] NOT NULL DEFAULT '{}',
  last_checked timestamptz,
  disputed     boolean NOT NULL DEFAULT false,
  source       text NOT NULL DEFAULT 'imported'
                 CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence   text NOT NULL DEFAULT 'possible'
                 CHECK (confidence IN ('confirmed','probable','possible')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT person_external_references_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=14)
);

CREATE TRIGGER person_external_references_set_updated_at
  BEFORE UPDATE ON oikumenea.person_external_references
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_external_references_person_idx
  ON oikumenea.person_external_references (person_id) WHERE deleted_at IS NULL;
-- Dedup an active (person, url) — one reference row per source link.
CREATE UNIQUE INDEX person_external_references_person_url
  ON oikumenea.person_external_references (person_id, url) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_external_references.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.external_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.categories IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_external_references.last_checked IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.disputed IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_external_references.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Emergency contacts (D-InstitutionalTies): NO new entity — add an 'emergency' label to the M14
-- person_relation_types catalog (category next_of_kin), so an emergency contact is an ordinary
-- person↔person next-of-kin relationship.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.person_relation_types (code, name, category, sort_order) VALUES
  ('emergency', 'Emergency contact', 'next_of_kin', 75)
ON CONFLICT (code) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0032_person_institutional_ties', applied_at = now() WHERE singleton;
