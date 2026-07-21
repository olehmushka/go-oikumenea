-- 0040 connector plane — the fleet registry (M53 — D-ConnectorPlane). oikumenea gains VISIBILITY over
-- the family of connectors that feed it, WITHOUT gaining orchestration: connectors keep their own
-- storage and scheduler and couple to the core over HTTP only (the D-Hermenea boundary, kept). The
-- core never schedules, triggers or retries a run; it records what a connector REPORTS.
--
-- Vocabulary note: a "connector" here is a WHOLE DEPLOYABLE AGENT beside the core (hermenea is the
-- first). That is a different, higher altitude than hermenea's internal `Fetcher` seam (a fetch
-- strategy — http/file/wof-sqlite), which was called `Connector` through M52 and was renamed in M53
-- precisely so these two never collide. hermenea's own `connector_type` column keeps its name.
--
-- Tables:
--   * OBJECT — connector_connectors: a registered agent. `code` is the stable handle (D-Code);
--              principal_id ties it to the M51 service principal it authenticates as, which is what
--              makes a report attributable. `last_seen_at` is a liveness hint, not a health verdict.
--   * OBJECT — connector_sources: one dataset a connector syncs. `object_type` names the core import
--              target (e.g. geo-places) for push-mode sources; NULL for sources that only feed the
--              connector's own lookups. Mirrors what the connector reports about itself — the
--              connector remains AUTHORITATIVE for execution, this is a read model.
--   * OBJECT — connector_sync_runs: one reported execution. A first-class Object with its own RID
--              (not an audit-only line) because the M53 exit bar requires a completed run to be
--              VISIBLE and queryable from the core; the audit ledger records the reporting action.
--
-- Sources are (connector, code) unique; runs are append-mostly (a connector opens a run, then closes
-- it with counts or an error). Nothing here is RLS-guarded: the registry is instance-plane operator
-- data with no person or organization dimension. Machine access to RLS-protected, ORGANIZATION-owned
-- data is deliberately NOT part of this milestone — that is the RLS service arm, M55.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id / platform_rid_types) and
-- 0039 (account_service_principals).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (20, 'connector');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (20,1,1,'connector'),         -- connector / object / connector         (a deployable agent)
  (20,1,2,'connector_source'),  -- connector / object / connector_source  (one dataset it syncs)
  (20,1,3,'sync_run');          -- connector / object / sync_run          (one reported execution)

-- ===================================================================================================
-- connector_connectors — the registered fleet. One row per agent (hermenea, a future HR scraper, …).
-- `principal_id` is the M51 service principal the agent authenticates as: reports and wiring reads are
-- attributable to it, and disabling the principal stops both at once (no separate kill switch here).
-- ===================================================================================================
CREATE TABLE oikumenea.connector_connectors (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,1),  -- connector / object / connector
  code         text NOT NULL,
  name         text NOT NULL,
  description  text,
  -- The machine identity this agent presents (M51 / D-ServiceIdentities). RESTRICT: a principal that
  -- names a registered connector must be disabled, not deleted, so its audit trail keeps resolving.
  principal_id uuid REFERENCES oikumenea.account_service_principals(id) ON DELETE RESTRICT,
  status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  -- Liveness hint: when the core last heard from this agent (registration or a run report). A stale
  -- value means "not heard from", NOT "unhealthy" — the core does not probe connectors.
  last_seen_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT connector_connectors_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
-- `code` unique among LIVE rows only (soft-delete then re-register must work) — the partial-unique
-- idiom used across the schema.
CREATE UNIQUE INDEX connector_connectors_code_active_idx
  ON oikumenea.connector_connectors (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX connector_connectors_principal_active_idx
  ON oikumenea.connector_connectors (principal_id)
  WHERE deleted_at IS NULL AND principal_id IS NOT NULL;
CREATE TRIGGER connector_connectors_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_connectors
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_connectors.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.description IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.principal_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_connectors.last_seen_at IS 'pii:none';

-- ===================================================================================================
-- connector_sources — the datasets each connector syncs, as REPORTED by the connector. This is a read
-- model: hermenea's own `hermenea.import_sources` (its DB) stays authoritative for execution, because
-- scheduling lives in the connector (D-ConnectorPlane rejects core-side orchestration).
-- ===================================================================================================
CREATE TABLE oikumenea.connector_sources (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,2),  -- connector / object / connector_source
  connector_id uuid NOT NULL REFERENCES oikumenea.connector_connectors(id) ON DELETE CASCADE,
  code         text NOT NULL,
  name         text NOT NULL,
  -- The core import target for push-mode sources (e.g. geo-places, language-scheme). Deliberately NOT
  -- an FK or CHECK against the handler registry: object types are code-defined in internal/dataimport,
  -- and a connector may report a source whose handler this core build does not have.
  object_type  text,
  -- The connector's own schedule string, verbatim, for display only. The core never parses or acts on
  -- it — scheduling stays inside the connector.
  schedule     text,
  enabled      boolean NOT NULL DEFAULT true,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT connector_sources_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE UNIQUE INDEX connector_sources_code_active_idx
  ON oikumenea.connector_sources (connector_id, code) WHERE deleted_at IS NULL;
CREATE TRIGGER connector_sources_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_sources
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_sources.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.connector_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.object_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.schedule IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sources.enabled IS 'pii:none';

-- ===================================================================================================
-- connector_sync_runs — one reported execution of a source. The connector opens a run (state=running)
-- and closes it (succeeded|failed) with counts. The core stores what it is told: these counts are the
-- CONNECTOR's account of its work, not something the core recomputes.
--
-- `external_run_id` is the connector's own run identifier (hermenea's import_runs.id, and the same
-- value the M49 chunked envelopes carry as `runId`), so an operator can correlate a core-side run row
-- with the connector's own ledger and with import provenance. Unique per connector so a re-report is
-- an update, not a duplicate — reporting must be idempotent under connector retries.
-- ===================================================================================================
CREATE TABLE oikumenea.connector_sync_runs (
  id              uuid PRIMARY KEY DEFAULT oikumenea.new_id(20,1,3),  -- connector / object / sync_run
  source_id       uuid NOT NULL REFERENCES oikumenea.connector_sources(id) ON DELETE CASCADE,
  external_run_id text,
  state           text NOT NULL CHECK (state IN ('running','succeeded','failed')),
  created_count   bigint NOT NULL DEFAULT 0,
  updated_count   bigint NOT NULL DEFAULT 0,
  skipped_count   bigint NOT NULL DEFAULT 0,
  -- Failure detail as reported. Free text: it is a connector's message, shown to operators verbatim.
  error           text,
  started_at      timestamptz NOT NULL DEFAULT now(),
  finished_at     timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT connector_sync_runs_rid_shape
    CHECK (oikumenea.rid_service(id)=20 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3),
  -- A finished run must say when it finished; a running one must not claim to have.
  CONSTRAINT connector_sync_runs_finished_shape
    CHECK ((state = 'running' AND finished_at IS NULL) OR (state <> 'running' AND finished_at IS NOT NULL))
);
CREATE UNIQUE INDEX connector_sync_runs_external_idx
  ON oikumenea.connector_sync_runs (source_id, external_run_id) WHERE external_run_id IS NOT NULL;
-- The fleet view's query: most recent runs for a source.
CREATE INDEX connector_sync_runs_source_started_idx
  ON oikumenea.connector_sync_runs (source_id, started_at DESC);
CREATE TRIGGER connector_sync_runs_set_updated_at
  BEFORE UPDATE ON oikumenea.connector_sync_runs
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.connector_sync_runs.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.source_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.external_run_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.state IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.created_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.updated_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.skipped_count IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.error IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.started_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.connector_sync_runs.finished_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0040_connector_plane', applied_at = now() WHERE singleton;
