-- 0029 external organizations (M30).
--
-- The external-organizations domain (docs/modules/external-organizations.md / D-ExternalOrgs): a
-- registry of external organizations a person is tied to but which the deploying org neither owns nor
-- commands — political parties, government bodies, foreign military formations, NGOs, lobbying
-- registrants/clients. It is the NODE-SPACE the M33 institutional-tie edges point at when the org side
-- is neither one of the operator's own tenant_units (authority-bearing through the PDP) nor a for-profit
-- legal entity in the M21 company registry. Faith-/sector-agnostic, catalog-typed, provenance-tagged.
--
-- Each row is catalog-typed (external_org_kinds: party | government_body | military | ngo | registrant |
-- other), carries the D-OverlayFoundation (M29) provisional/resolved status + the source/confidence/as_of
-- attribution column-set (docs/architecture/conventions.md §Attribution), an optional country ->
-- geo_countries, an optional wikidata_id concordance (the hermenea import natural key), and a translatable
-- name. It is a hermenea import target (Wikidata / public registries) via the generic
-- POST /import/external-organizations endpoint (M16).
--
-- new_id() reads no GUC (D-ResourceIdentifiers), so the kind catalog rows are seeded directly here.
-- Organizations are created through ExternalOrganizationService or the hermenea import handler.
--
-- RLS: external orgs are external reference data, instance-global, not scoped against tenant_units (like
-- company / education / location / vehicle), so no RLS is enabled. External orgs NEVER enter the tenant
-- closure / PDP graph — they are directory nodes only.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 schema bootstrap (new_id +
-- geo_countries).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers): the new `external_organization` service (18) + its object/action
-- types. pkg/rid mirrors these and asserts equality at boot (kind<>3), so they are added in both places
-- together.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (18, 'external_organization');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  -- external-organization objects
  (18,1,1,'external_organization'),(18,1,2,'external_org_kind'),
  -- external-organization Action RID (kind=3, excluded from the Go-mirror size check)
  (18,3,0,'action');

-- ===================================================================================================
-- Reference catalog (D-Code / D-i18n): code + translatable name (default-locale here, other locales in
-- the localization store). Instance-admin-managed; seeded with the D-ExternalOrgs starter set,
-- instance-extensible. Mirrors religion_org_kinds / vehicle_types.
-- ===================================================================================================

-- external_org_kinds — the org-kind catalog (party / government_body / military / ngo / registrant / other).
CREATE TABLE oikumenea.external_org_kinds (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(18,1,2),  -- external_organization / object / external_org_kind
  code       text NOT NULL,
  name       text NOT NULL,                 -- default-locale display name; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT external_org_kinds_rid_shape
    CHECK (oikumenea.rid_service(id)=18 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX external_org_kinds_code_active
  ON oikumenea.external_org_kinds (code) WHERE deleted_at IS NULL;
CREATE TRIGGER external_org_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.external_org_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.external_org_kinds.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_org_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_org_kinds.name IS 'pii:none';

INSERT INTO oikumenea.external_org_kinds (code, name, sort_order) VALUES
  ('party','Political party',10),
  ('government_body','Government body',20),
  ('military','Military formation',30),
  ('ngo','Non-governmental organization',40),
  ('registrant','Lobbying registrant / client',50),
  ('other','Other',60);

-- ===================================================================================================
-- Object: the external organization.
-- ===================================================================================================

-- external_organizations — an external organization at registry grade. kind_id classifies it; name is the
-- default-locale display name (translatable via the i18n store). code is an optional stable handle
-- (D-Code; the RID is the primary external handle). country_id is the org's country (nullable).
-- wikidata_id is the optional Wikidata Q-id concordance, unique among active rows when present (the
-- hermenea import natural key). status carries the D-OverlayFoundation provisional/resolved lifecycle;
-- source/confidence/as_of are the M29 attribution column-set, reused verbatim.
CREATE TABLE oikumenea.external_organizations (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(18,1,1),  -- external_organization / object / external_organization
  kind_id     uuid NOT NULL REFERENCES oikumenea.external_org_kinds(id) ON DELETE RESTRICT,
  name        text NOT NULL,                -- default-locale display name; translatable via the i18n store
  code        text,                         -- optional stable handle (D-Code); unique-active when present
  country_id  uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable
  wikidata_id text,                         -- optional Wikidata Q-id concordance; unique-active; import key
  status      text NOT NULL DEFAULT 'resolved' CHECK (status IN ('provisional','resolved')),
  -- D-OverlayFoundation attribution column-set (docs/architecture/conventions.md §Attribution), verbatim:
  source      text NOT NULL DEFAULT 'operator_verified'
                CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence  text NOT NULL DEFAULT 'possible'
                CHECK (confidence IN ('confirmed','probable','possible')),
  as_of       timestamptz,                  -- optional: when the asserted value was observed/true
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CONSTRAINT external_organizations_rid_shape
    CHECK (oikumenea.rid_service(id)=18 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE UNIQUE INDEX external_organizations_code_active
  ON oikumenea.external_organizations (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE UNIQUE INDEX external_organizations_wikidata_active
  ON oikumenea.external_organizations (wikidata_id) WHERE deleted_at IS NULL AND wikidata_id IS NOT NULL;
CREATE INDEX external_organizations_kind_idx
  ON oikumenea.external_organizations (kind_id) WHERE deleted_at IS NULL;
CREATE INDEX external_organizations_country_idx
  ON oikumenea.external_organizations (country_id) WHERE deleted_at IS NULL;
CREATE INDEX external_organizations_status_idx
  ON oikumenea.external_organizations (status) WHERE deleted_at IS NULL;
CREATE TRIGGER external_organizations_set_updated_at
  BEFORE UPDATE ON oikumenea.external_organizations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.external_organizations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.kind_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.wikidata_id IS 'pii:none';
