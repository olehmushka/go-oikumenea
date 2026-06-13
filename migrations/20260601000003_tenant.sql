-- 0003 tenant (M3).
--
-- The structural foundation: the organization as a graph of units (docs/modules/tenant.md /
-- D-Graphs). Units are Objects; the parent->child edge is a reified Link (link__parent_of), per
-- named hierarchy (graph); a maintained transitive closure answers ancestor/descendant in one
-- lookup for the M7 PDP. Multi-parent, multi-root DAGs, independently per graph. Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.
--
-- This migration is PURE DDL: it seeds NO rows. The command + operational graphs are RID-keyed
-- Objects, and new_rid() reads the per-connection app.environment GUC that atlas's migration
-- connection does not set. So RID-keyed seed rows are seeded at BOOT by the application
-- (tenant.Register), on the GUC-bearing pool, idempotently (INSERT ... ON CONFLICT (code) DO
-- NOTHING) — only the app knows the real environment. See docs/architecture/decisions.md
-- (boot-time idempotent seeding of RID-keyed reference rows); precedent for M7 base-roles / M8
-- first-admin.

-- tenant_units: a node in the org graph (D-Graphs). `code` is the stable, locale-agnostic external
-- reference (D-Code); `name` is the default-locale fallback (translations in the i18n store, M2).
-- `level`/`unit_kind` are DIRECTORY attributes only — never PDP inputs (tenant.md). Visibility is
-- the read-time public/shadow gate (M7). Lifecycle: active/suspended/archived (reversible).
CREATE TABLE oikumenea.tenant_units (
  id          uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,1),  -- tenant / object / unit
  code        text NOT NULL,                 -- stable, locale-agnostic; unique among active (index below)
  name        text NOT NULL,                 -- default-locale display name; translatable via i18n store
  unit_kind   text,                          -- descriptive label (e.g. battalion); never branched on
  level       smallint,                      -- optional ordinal for sort/filter; never a PDP/gate input
  visibility  text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','shadow')),
  state       text NOT NULL DEFAULT 'active' CHECK (state IN ('active','suspended','archived')),
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

-- `code` is unique among active (non-deleted) units; immutable by convention (D-Code).
CREATE UNIQUE INDEX tenant_units_code_active_idx
  ON oikumenea.tenant_units (code) WHERE deleted_at IS NULL;
CREATE INDEX tenant_units_level_idx ON oikumenea.tenant_units (level) WHERE deleted_at IS NULL;

-- Unit labels are organizational, not personal data (D-PIITiers).
COMMENT ON COLUMN oikumenea.tenant_units.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.unit_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.level IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.visibility IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.state IS 'pii:none';
COMMENT ON COLUMN oikumenea.tenant_units.metadata IS 'pii:none';

-- tenant_graphs: the named-hierarchy registry (D-Graphs / D-DirectoryGraphs). Each graph is
-- independently a DAG over the units. `command` is the default + undeletable + locked
-- authority-bearing; `operational` is authority-bearing. Both are seeded at boot (see header).
CREATE TABLE oikumenea.tenant_graphs (
  id                   uuid PRIMARY KEY DEFAULT oikumenea.new_id(4,1,2),  -- tenant / object / graph
  code                 text NOT NULL,            -- stable, locale-agnostic (e.g. command, operational)
  name                 text NOT NULL,            -- default-locale display name; translatable via i18n store
  is_default           boolean NOT NULL DEFAULT false,  -- the graph a subtree grant uses when none is named
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

-- `code` unique among active graphs; at most one default among active graphs (the application
-- clears the previous default in the same txn when promoting a new one).
CREATE UNIQUE INDEX tenant_graphs_code_active_idx
  ON oikumenea.tenant_graphs (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenant_graphs_one_default_idx
  ON oikumenea.tenant_graphs (is_default) WHERE is_default AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.tenant_graphs.id IS 'pii:none';
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

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0003_tenant', applied_at = now() WHERE singleton;
