-- 0003 tenant (M3).
--
-- The structural foundation: the organization as a graph of units (docs/modules/tenant.md /
-- D-Graphs). Units are Objects; the parent->child edge is a reified Link (link__parent_of), per
-- named hierarchy (graph); a maintained transitive closure answers ancestor/descendant in one
-- lookup for the M7 PDP. Multi-parent, multi-root DAGs, independently per graph. Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.
--
-- Seeding: this migration seeds the RID-keyed reference catalogs (tenant_domains + tenant_unit_kinds)
-- at the end, idempotently (INSERT ... ON CONFLICT DO NOTHING). Post-F-014 new_id() reads NO GUC
-- (app/service/type are caller-supplied codes — see 0000 schema_bootstrap), so migrations may seed
-- RID rows directly; the old "seed RID rows only at boot because new_rid needs the env GUC" constraint
-- (D-RIDSeeding) is now vestigial. The per-org command + operational graphs are NOT seeded here — they
-- are created with each organization (application.CreateOrganization, D-TenantOrganizations/M40).

-- ---------------------------------------------------------------------------------------------------
-- M40 / D-TenantOrganizations: a two-tier model above the unit DAG so one deployment hosts every kind
-- of hierarchical organization. domain = org-kind catalog; organization = the realm a person joins
-- (US Army / Bundeswehr / KhNU); unit_kind = domain-scoped catalog replacing free-text unit_kind.
-- These RID-keyed types extend the tenant service; pkg/rid mirrors them and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (4,1,5,'domain'),(4,1,6,'organization'),(4,1,7,'unit_kind'),(4,1,8,'org_lifecycle_event');

-- tenant_domains: the org-kind catalog (military/government/company/university/church/public-org, …).
-- Catalog Object (D-Code/D-i18n): stable `code` + default-locale `name` (translations in the i18n
-- store). Instance-admin-extensible, seeded below (RID-keyed; new_id needs no GUC post-F-014), NEVER a
-- CHECK enum. A DIRECTORY attribute that classifies organizations & units — never a PDP input.
CREATE TABLE oikumenea.tenant_domains (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,5),  -- tenant / object / domain
  code       text NOT NULL,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  -- pdp_scoped (D-UnifiedOrgGraph, M41): TRUE = an OPERATIONAL domain whose units are reach-RLS-scoped and
  -- whose orgs auto-seed command/operational graphs (military/government/public-org/church). FALSE = a
  -- REFERENCE domain (university/company): instance-global (public reads, app-permission writes, no
  -- reach-RLS, no auto graph seed). Denormalized onto tenant_units for the RLS predicate.
  pdp_scoped boolean NOT NULL DEFAULT true,
  sort_order integer,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT tenant_domains_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=5)
);
CREATE TRIGGER tenant_domains_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_domains
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE UNIQUE INDEX tenant_domains_code_active_idx
  ON oikumenea.tenant_domains (code) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_domains.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_domains.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_domains.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_domains.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_domains.pdp_scoped IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_domains.sort_order IS 'pii:none';

-- tenant_unit_kinds: a DOMAIN-SCOPED catalog replacing the free-text unit_kind (military→brigade/
-- battalion/platoon; university→faculty/department/chair). Optional `attr_schema` (JSON schema) may
-- validate a unit's `metadata` per kind (the document_types.attr_schema pattern). Seeded at BOOT.
CREATE TABLE oikumenea.tenant_unit_kinds (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,7),  -- tenant / object / unit_kind
  domain_id   uuid NOT NULL REFERENCES oikumenea.tenant_domains(id) ON DELETE RESTRICT,
  code        text NOT NULL,
  name        text NOT NULL,
  attr_schema jsonb,                 -- optional JSON schema validating a unit's metadata for this kind
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order  integer,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT tenant_unit_kinds_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=7)
);
CREATE TRIGGER tenant_unit_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_unit_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
-- `code` unique among active kinds OF THE SAME DOMAIN.
CREATE UNIQUE INDEX tenant_unit_kinds_domain_code_active_idx
  ON oikumenea.tenant_unit_kinds (domain_id, code) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.domain_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.attr_schema IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_kinds.sort_order IS 'pii:none';

-- tenant_organizations: the REALM — the concrete top-level entity a person joins (US Army, Bundeswehr,
-- KhNU). Many organizations may share a domain. `code` (D-Code: NOT NULL UNIQUE, immutable by
-- convention) + translatable `name`; `domain_id` classifies it; public/shadow visibility; reversible
-- lifecycle. Owns units (units.org_id) and per-org graphs (graphs.org_id). NOT seeded — created by the
-- deployment. A DIRECTORY attribute, never a PDP input; authority flows only through its graphs.
CREATE TABLE oikumenea.tenant_organizations (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,6),  -- tenant / object / organization
  code        text NOT NULL,
  name        text NOT NULL,
  domain_id   uuid NOT NULL REFERENCES oikumenea.tenant_domains(id) ON DELETE RESTRICT,
  visibility  text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','shadow')),
  state       text NOT NULL DEFAULT 'active' CHECK (state IN ('active','suspended','archived')),
  metadata    jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT tenant_organizations_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=6)
);
CREATE TRIGGER tenant_organizations_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_organizations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
CREATE UNIQUE INDEX tenant_organizations_code_active_idx
  ON oikumenea.tenant_organizations (code) WHERE deleted_at IS NULL;
CREATE INDEX tenant_organizations_domain_idx
  ON oikumenea.tenant_organizations (domain_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.tenant_organizations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.domain_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.visibility IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_organizations.metadata IS 'pii:none';

-- tenant_org_lifecycle_events: append-only record of each organization state transition (mirrors
-- tenant_unit_lifecycle_events). Guarded by reject_mutation(); keyed by its own event RID.
CREATE TABLE oikumenea.tenant_org_lifecycle_events (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,8),  -- tenant / object / org_lifecycle_event
  org_id          uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  from_state      text NOT NULL,
  to_state        text NOT NULL,
  reason          text,
  actor_person_id uuid,
  request_id      text NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_org_lifecycle_events_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=8)
);
CREATE TRIGGER tenant_org_lifecycle_events_reject_mutation
  BEFORE UPDATE OR DELETE ON oikumenea.tenant_org_lifecycle_events
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();
CREATE INDEX tenant_org_lifecycle_events_org_idx
  ON oikumenea.tenant_org_lifecycle_events (org_id, created_at DESC);
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.from_state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.to_state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.reason IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.actor_person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.tenant_org_lifecycle_events.request_id IS 'pii:none';

-- tenant_units: a node in the org graph (D-Graphs). `code` is an OPTIONAL, mutable, locale-agnostic
-- human-readable business ID (D-UnitCodeLifecycle, M28, amending D-Code); the RID is the stable
-- external machine handle. `name` is the default-locale fallback (translations in the i18n store, M2).
-- `level`/`unit_kind` are DIRECTORY attributes only — never PDP inputs (tenant.md). Visibility is
-- the read-time public/shadow gate (M7). Lifecycle: active/suspended/archived (reversible).
CREATE TABLE oikumenea.tenant_units (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,1),  -- tenant / object / unit
  org_id      uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- owning realm (M40)
  domain_id   uuid NOT NULL REFERENCES oikumenea.tenant_domains(id) ON DELETE RESTRICT,         -- per-unit kind class; defaults to the org's domain; mixed trees allowed (M40)
  kind_id     uuid REFERENCES oikumenea.tenant_unit_kinds(id) ON DELETE RESTRICT,               -- domain-scoped unit kind (replaces free-text unit_kind); optional (M40)
  code        text,                          -- optional; NULL = non-separate sub-unit; mutable via the recode op; unique among active coded units (index below)
  name        text NOT NULL,                 -- default-locale display name; translatable via i18n store
  level       smallint,                      -- optional ordinal for sort/filter; never a PDP/gate input
  visibility  text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','shadow')),
  state       text NOT NULL DEFAULT 'active' CHECK (state IN ('active','suspended','archived')),
  -- pdp_scoped: denormalized from the unit's domain (D-UnifiedOrgGraph, M41). FALSE = a reference unit
  -- (university/company) — instance-global, exempt from the reach-RLS predicate. Derived in SQL at insert.
  pdp_scoped  boolean NOT NULL DEFAULT true,
  metadata    jsonb NOT NULL DEFAULT '{}',
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,

  CONSTRAINT tenant_units_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER tenant_units_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_units
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` is unique among active (non-deleted) units THAT HAVE one; codeless units (NULL) are
-- non-separate sub-units and never collide (D-UnitCodeLifecycle, M28).
CREATE UNIQUE INDEX tenant_units_code_active_idx
  ON oikumenea.tenant_units (code) WHERE deleted_at IS NULL AND code IS NOT NULL;
CREATE INDEX tenant_units_level_idx ON oikumenea.tenant_units (level) WHERE deleted_at IS NULL;
-- Listing is org-scoped (GET /units requires ?org); the domain filter is a secondary cross-cut (M40).
CREATE INDEX tenant_units_org_idx ON oikumenea.tenant_units (org_id) WHERE deleted_at IS NULL;
CREATE INDEX tenant_units_org_domain_idx ON oikumenea.tenant_units (org_id, domain_id) WHERE deleted_at IS NULL;

-- Unit labels are organizational, not personal data (D-PIITiers).
COMMENT ON COLUMN oikumenea.tenant_units.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.domain_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.kind_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.level IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.visibility IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.pdp_scoped IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.metadata IS 'pii:none';

-- tenant_graphs: the named-hierarchy registry (D-Graphs / D-DirectoryGraphs). Each graph is
-- independently a DAG over the units. `command` is the default + undeletable + locked
-- authority-bearing; `operational` is authority-bearing. Both are seeded at boot (see header).
CREATE TABLE oikumenea.tenant_graphs (
  id                   uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,2),  -- tenant / object / graph
  org_id               uuid REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,  -- owning realm; NULL = instance-global/cross-org graph (e.g. religion taxonomy) (M40)
  code                 text NOT NULL,            -- stable, locale-agnostic (e.g. command, operational)
  name                 text NOT NULL,            -- default-locale display name; translatable via i18n store
  is_default           boolean NOT NULL DEFAULT false,  -- the graph a subtree grant uses when none is named (per org)
  is_authority_bearing boolean NOT NULL DEFAULT true,   -- whether the PDP cascades subtree grants over it
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,

  CONSTRAINT tenant_graphs_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2),
  -- command is locked authority-bearing (tenant.md): it may never be made directory-only.
  CONSTRAINT tenant_graphs_command_authority CHECK (code <> 'command' OR is_authority_bearing)
);

CREATE TRIGGER tenant_graphs_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_graphs
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` unique among active graphs OF THE SAME ORG (org_id NOT NULL) AND, separately, among active
-- instance-global graphs (org_id NULL — religion's taxonomy graphs). Two partial-unique indexes (M40).
CREATE UNIQUE INDEX tenant_graphs_org_code_active_idx
  ON oikumenea.tenant_graphs (org_id, code) WHERE deleted_at IS NULL AND org_id IS NOT NULL;
CREATE UNIQUE INDEX tenant_graphs_global_code_active_idx
  ON oikumenea.tenant_graphs (code) WHERE deleted_at IS NULL AND org_id IS NULL;
-- At most one default per org, and at most one default among global graphs (NULL bucket via sentinel).
CREATE UNIQUE INDEX tenant_graphs_one_default_idx
  ON oikumenea.tenant_graphs (COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid))
  WHERE is_default AND deleted_at IS NULL;
CREATE INDEX tenant_graphs_org_idx ON oikumenea.tenant_graphs (org_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.tenant_graphs.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_graphs.org_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_graphs.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_graphs.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_graphs.is_default IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_graphs.is_authority_bearing IS 'pii:none';

-- tenant_unit_edges: the reified parent->child Link, per graph (link__parent_of). Many per unit in
-- either direction (DAG); the same parent->child pair may exist across graphs. Hard-deleted on
-- detach (an edge has no independent life). Cycle prevention is enforced per graph in the
-- application on insert (via the closure); the closure is recomputed in the same txn.
CREATE TABLE oikumenea.tenant_unit_edges (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,2,1),  -- tenant / link / parent_of
  graph_id   uuid NOT NULL REFERENCES oikumenea.tenant_graphs(id) ON DELETE RESTRICT,
  parent_id  uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  child_id   uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by uuid,   -- person RID provenance (nullable; identity lands in M8)

  CONSTRAINT tenant_unit_edges_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1),
  CONSTRAINT tenant_unit_edges_no_self_loop CHECK (parent_id <> child_id),
  CONSTRAINT tenant_unit_edges_unique UNIQUE (graph_id, parent_id, child_id)
);

CREATE INDEX tenant_unit_edges_parent_idx ON oikumenea.tenant_unit_edges (graph_id, parent_id);
CREATE INDEX tenant_unit_edges_child_idx  ON oikumenea.tenant_unit_edges (graph_id, child_id);

COMMENT ON COLUMN oikumenea.tenant_unit_edges.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_edges.graph_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_edges.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_edges.child_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_edges.created_by IS 'pii:basic';

-- tenant_unit_closure: derived, maintained per graph on every edge change (link__ancestor_of). A
-- materialized derived relation, not a source of truth (ontology-mapping.md 4.3) — so it has a
-- composite key, no RID. Includes the reflexive (g,u,u,0) row for every unit that participates in
-- the graph's edges, so "is U in the subtree of T in graph g" is one lookup. depth = the shortest
-- path length in a multi-parent DAG.
CREATE TABLE oikumenea.tenant_unit_closure (
  graph_id      uuid NOT NULL REFERENCES oikumenea.tenant_graphs(id) ON DELETE RESTRICT,
  ancestor_id   uuid NOT NULL,
  descendant_id uuid NOT NULL,
  depth         integer NOT NULL,

  PRIMARY KEY (graph_id, ancestor_id, descendant_id)
);

-- The PK indexes ancestor->descendant (subtree/descendant lookups); this covers the reverse
-- (descendant->ancestor) lookups used for ancestors.
CREATE INDEX tenant_unit_closure_descendant_idx
  ON oikumenea.tenant_unit_closure (graph_id, descendant_id);

COMMENT ON COLUMN oikumenea.tenant_unit_closure.graph_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_closure.ancestor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_closure.descendant_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_closure.depth IS 'pii:none';

-- tenant_closure_status: derived diagnostic overlay, one row per graph (D-ClosureDriftHealth). NOT
-- append-only, NOT audited. Upserted by POST /closure/verify; read by the closure-drift health
-- reporter (platform). Graph-level counts only, no person/unit PII.
CREATE TABLE oikumenea.tenant_closure_status (
  graph_id        uuid PRIMARY KEY REFERENCES oikumenea.tenant_graphs(id) ON DELETE CASCADE,
  last_checked_at timestamptz NOT NULL DEFAULT now(),
  missing_count   integer NOT NULL DEFAULT 0,   -- closure rows the recompute found missing vs stored
  extra_count     integer NOT NULL DEFAULT 0,   -- spurious stored rows the recompute did not produce
  in_drift        boolean NOT NULL DEFAULT false,
  sample          jsonb,                         -- optional small drift sample for diagnostics
  updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER tenant_closure_status_set_updated_at
  BEFORE UPDATE ON oikumenea.tenant_closure_status
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.tenant_closure_status.graph_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_closure_status.missing_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_closure_status.extra_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_closure_status.in_drift IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_closure_status.sample IS 'pii:none';

-- tenant_unit_lifecycle_events: append-only record of each unit state transition (tenant.md).
-- Guarded by reject_mutation(); keyed by its own event RID.
CREATE TABLE oikumenea.tenant_unit_lifecycle_events (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,3),  -- tenant / object / unit_lifecycle_event
  unit_id         uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  from_state      text NOT NULL,
  to_state        text NOT NULL,
  reason          text,
  actor_person_id uuid,           -- the person who transitioned the unit (nullable until M8)
  request_id      text NOT NULL,  -- correlation key shared with logs/metrics/traces/audit
  created_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_unit_lifecycle_events_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);

CREATE TRIGGER tenant_unit_lifecycle_events_reject_mutation
  BEFORE UPDATE OR DELETE ON oikumenea.tenant_unit_lifecycle_events
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

CREATE INDEX tenant_unit_lifecycle_events_unit_idx
  ON oikumenea.tenant_unit_lifecycle_events (unit_id, created_at DESC);

COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.from_state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.to_state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.reason IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.actor_person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.tenant_unit_lifecycle_events.request_id IS 'pii:none';

-- tenant_unit_code_events: append-only ledger of each unit `code` set/correct/clear via the audited
-- recode op (D-UnitCodeLifecycle, M28). old_code/new_code are both nullable to record NULL<->value
-- transitions (a codeless unit gaining a code; a code being cleared). Guarded by reject_mutation();
-- keyed by its own event RID. The RID is the external handle, so old codes need not stay resolvable.
CREATE TABLE oikumenea.tenant_unit_code_events (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,4),  -- tenant / object / unit_code_event
  unit_id         uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  old_code        text,           -- the code before the change (NULL = was codeless)
  new_code        text,           -- the code after the change (NULL = cleared)
  reason          text,
  actor_person_id uuid,           -- the person who recoded the unit (nullable until M8)
  request_id      text NOT NULL,  -- correlation key shared with logs/metrics/traces/audit
  created_at      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT tenant_unit_code_events_rid_shape
    CHECK (oikumenea.rid_service(id)=4 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);

CREATE TRIGGER tenant_unit_code_events_reject_mutation
  BEFORE UPDATE OR DELETE ON oikumenea.tenant_unit_code_events
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

CREATE INDEX tenant_unit_code_events_unit_idx
  ON oikumenea.tenant_unit_code_events (unit_id, created_at DESC);

COMMENT ON COLUMN oikumenea.tenant_unit_code_events.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.old_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.new_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.reason IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.actor_person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.tenant_unit_code_events.request_id IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Reference-catalog seed (D-TenantOrganizations, M40 / M41). Idempotent (ON CONFLICT DO NOTHING) so a
-- re-applied migration or a manually-seeded test DB is a no-op. Domains first, then unit kinds (which
-- resolve domain_id by joining on the domain code).
--
-- pdp_scoped (D-UnifiedOrgGraph, M41): operational domains (military/government/public-org/church) are
-- reach-RLS-scoped + auto-seed per-org graphs; reference domains (university/company) are instance-global.
INSERT INTO oikumenea.tenant_domains (code, name, pdp_scoped, sort_order) VALUES
  ('military',   'Military',             true,   0),
  ('government', 'Government',           true,  10),
  ('company',    'Company',              false, 20),
  ('university', 'University',           false, 30),
  ('church',     'Church',              true,  40),
  ('public-org', 'Public Organization', true,  50)
ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;

-- Starter domain-scoped unit kinds. `name` is the default-locale text (the i18n store can translate
-- later). Instance admins add more via the unit-kind catalog API. The `military` set is a universal
-- echelon ladder usable by any armed force: a top governance tier (Ministry/Department of Defence),
-- service branches & commands, the joint/ground ladder (army-group → … → fire-team), plus air & naval
-- echelons. Arm-specific naming variants (battery/troop = company/platoon) are NOT separate kinds — the
-- per-unit name carries the actual title.
INSERT INTO oikumenea.tenant_unit_kinds (domain_id, code, name, sort_order)
SELECT d.id, k.code, k.name, k.sort_order
FROM (VALUES
  -- military — governance / strategic
  ('military',   'ministry-of-defence', 'Ministry / Department of Defence',  0),
  ('military',   'armed-forces',        'Armed Forces',                     10),
  ('military',   'service-branch',      'Service Branch',                   20),
  ('military',   'command',             'Command',                          30),
  -- military — joint / ground ladder
  ('military',   'army-group',          'Army Group / Front',               40),
  ('military',   'army',                'Field Army',                       50),
  ('military',   'corps',               'Corps',                            60),
  ('military',   'division',            'Division',                         70),
  ('military',   'brigade',             'Brigade',                          80),
  ('military',   'regiment',            'Regiment',                         90),
  ('military',   'battalion',           'Battalion',                       100),
  ('military',   'company',             'Company',                         110),
  ('military',   'platoon',             'Platoon',                         120),
  ('military',   'squad',               'Squad / Section',                 130),
  ('military',   'fire-team',           'Fire Team / Crew',                140),
  -- military — air
  ('military',   'wing',                'Wing',                            200),
  ('military',   'air-group',           'Group (Air)',                     210),
  ('military',   'air-squadron',        'Squadron (Air)',                  220),
  ('military',   'flight',              'Flight',                          230),
  -- military — naval
  ('military',   'fleet',               'Fleet',                           300),
  ('military',   'flotilla',            'Flotilla',                        310),
  ('military',   'naval-squadron',      'Squadron (Naval)',                320),
  -- government
  ('government', 'ministry',   'Ministry',       0),
  ('government', 'agency',     'Agency',        10),
  ('government', 'department', 'Department',    20),
  -- company
  ('company',    'division',   'Division',       0),
  ('company',    'team',       'Team',          10),
  -- university
  ('university', 'campus',     'Campus',         0),
  ('university', 'institute',  'Institute',      5),
  ('university', 'faculty',    'Faculty',       10),
  ('university', 'department', 'Department',    20),
  ('university', 'chair',      'Chair',         30),
  -- church
  ('church',     'diocese',    'Diocese',        0),
  ('church',     'parish',     'Parish',        10),
  -- public-org
  ('public-org', 'chapter',    'Chapter',        0)
) AS k(domain_code, code, name, sort_order)
JOIN oikumenea.tenant_domains d ON d.code = k.domain_code AND d.deleted_at IS NULL
ON CONFLICT (domain_id, code) WHERE deleted_at IS NULL DO NOTHING;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0003_tenant', applied_at = now() WHERE singleton;
