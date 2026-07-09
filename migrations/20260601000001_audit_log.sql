-- 0001 audit log (M1).
--
-- The append-only Action ledger every later write commits into (D-Audit / docs/modules/audit.md).
-- Each row records one Action and is keyed by that Action's RID (action__<type>; D-Ontology):
-- the audit log IS the action ledger. Append-only — guarded by oikumenea.reject_mutation().
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.
--
-- Physical layout (review-2026-07 R-07): the ledger is the largest, hottest, ever-growing table, so
-- it is DECLARATIVELY RANGE-PARTITIONED BY MONTH on created_at. This bounds index/vacuum/backup cost
-- to the live months and makes retention a partition detach (D-AuditRetention), not a mass DELETE.
-- The D-Audit SEMANTICS are unchanged — same-transaction insert, append-only, one row per Action;
-- only the storage shape and the index set (trimmed to what the read API serves) change here.

CREATE TABLE oikumenea.audit_log (
  -- PK = the Action RID of the write this row records (D-ResourceIdentifiers / D-Audit):
  -- self-describing and chronologically ordered via its uuid_v7() component. Supplied by the
  -- producing module's application service, not defaulted here. Declarative partitioning requires
  -- the partition key in every unique/PK constraint, so the PK is (id, created_at); id leads so
  -- GetAuditEntry (WHERE id = …) stays a PK lookup. id is globally unique by construction (uuid_v7).
  id              uuid NOT NULL,

  created_at      timestamptz NOT NULL DEFAULT now(),

  -- The two actor kinds (D-Audit). There is no super_admin kind — an instance admin is a person.
  actor_type      text NOT NULL CHECK (actor_type IN ('person','system')),
  -- The person who acted (person RID); NOT NULL for person actions, NULL for system (CHECK below).
  actor_person_id uuid,
  -- For system actions, the originating source (bootstrap, recover-admin, purge-worker,
  -- closure-rebuild, event-subscriber, …); NOT NULL for system actions, NULL otherwise (CHECK below).
  subsystem       text,

  action          text NOT NULL,  -- e.g. assignment.grant, unit.transition, rank.scheme.update
  target_type     text NOT NULL,  -- e.g. unit, person, role_assignment, account, graph
  -- target_id is POLYMORPHIC: a RID uuid for RID-keyed entities OR a natural code (locale, country,
  -- scheme) for catalog entities — so it is TEXT, not uuid (D-ResourceIdentifiers carve-out).
  target_id       text,           -- the acted-on entity's id (RID uuid text or a natural code)
  unit_id         uuid,           -- unit context where applicable (for scoped audit reads)

  request_id      text NOT NULL,  -- correlation key shared with logs/metrics/traces

  -- State snapshot / change payload. No secrets; PII minimized. pii:special CEILING (D-PIITiers):
  -- special-category data must NOT land here until the envelope seam (DS-29) ships.
  before          jsonb,
  after           jsonb,

  outcome         text NOT NULL DEFAULT 'success' CHECK (outcome IN ('success','denied','error')),

  PRIMARY KEY (id, created_at),

  -- The Action RID shape: every audit key is an Action RID (rid_kind = 3; D-Ontology / conventions).
  CONSTRAINT audit_log_action_rid_shape CHECK (oikumenea.rid_kind(id) = 3),

  -- Actor-shape CHECK — the two kinds, mutually exclusive (D-Audit).
  CONSTRAINT audit_log_actor_shape CHECK (
    (actor_type = 'person' AND actor_person_id IS NOT NULL AND subsystem IS NULL)
    OR
    (actor_type = 'system' AND actor_person_id IS NULL AND subsystem IS NOT NULL)
  )
) PARTITION BY RANGE (created_at);

-- Append-only: no UPDATE/DELETE from application code; corrections are new entries (D-Audit). A
-- BEFORE ROW trigger on the partitioned parent cascades to every current and future partition
-- (PG 13+; the deployment runs PG 16).
CREATE TRIGGER audit_log_reject_mutation
  BEFORE UPDATE OR DELETE ON oikumenea.audit_log
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

-- Filter/correlation indexes (docs/modules/audit.md), trimmed to what the read API serves
-- (review-2026-07 R-07 index diet): keyset cursor, target lookup, actor lookup, and the unit_id the
-- RLS read policy probes. The former actor_type single (2 distinct values) and request_id single are
-- dropped — a request-id correlation rides the time-range + created_at index; re-add if it goes hot.
-- Indexes declared on the parent auto-propagate to every partition.
CREATE INDEX audit_log_created_at_id_idx ON oikumenea.audit_log (created_at DESC, id DESC);
CREATE INDEX audit_log_actor_person_idx  ON oikumenea.audit_log (actor_person_id);
CREATE INDEX audit_log_target_idx        ON oikumenea.audit_log (target_type, target_id);
CREATE INDEX audit_log_unit_idx          ON oikumenea.audit_log (unit_id);

-- ensure_audit_partition: idempotently create the monthly partition covering month_start's month,
-- with UTC-aligned [m_start, m_end) bounds (created_at is UTC). Called at migration time for the
-- current + next month and re-called at every oikumenea boot (advisory-locked) to roll the window
-- forward. CREATE TABLE IF NOT EXISTS makes it a no-op once the partition exists.
-- SECURITY DEFINER so the boot-time roll-forward can run as this function's owner (the schema owner,
-- which holds CREATE on schema oikumenea). The application role (oikumenea_app) only holds USAGE, so an
-- INVOKER-mode CREATE TABLE would fail 42501; the pinned search_path keeps the definer-rights body safe.
CREATE OR REPLACE FUNCTION oikumenea.ensure_audit_partition(month_start date)
RETURNS void LANGUAGE plpgsql
SECURITY DEFINER SET search_path = pg_catalog, oikumenea AS $$
DECLARE
  m_start date := date_trunc('month', month_start)::date;
  m_end   date := (date_trunc('month', month_start) + interval '1 month')::date;
  part    text := 'audit_log_y' || to_char(m_start, 'YYYY') || 'm' || to_char(m_start, 'MM');
BEGIN
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS oikumenea.%I PARTITION OF oikumenea.audit_log FOR VALUES FROM (%L) TO (%L)',
    part,
    (m_start::timestamp AT TIME ZONE 'UTC'),
    (m_end::timestamp   AT TIME ZONE 'UTC'));
END;
$$;

-- detach_audit_partitions_before: operator retention helper (D-AuditRetention). Detaches every fully
-- past monthly partition whose upper bound is <= cutoff so the operator can dump + drop it under
-- their own legal-hold policy. It DETACHES only (never drops) — data loss is an explicit operator
-- act. The DEFAULT partition and any partition still covering >= cutoff are left attached. Returns
-- the detached partition names.
CREATE OR REPLACE FUNCTION oikumenea.detach_audit_partitions_before(cutoff date)
RETURNS SETOF text LANGUAGE plpgsql AS $$
DECLARE
  rec record;
BEGIN
  FOR rec IN
    SELECT c.relname
    FROM pg_inherits i
    JOIN pg_class c   ON c.oid = i.inhrelid
    JOIN pg_class p   ON p.oid = i.inhparent
    JOIN pg_namespace n ON n.oid = p.relnamespace
    WHERE n.nspname = 'oikumenea' AND p.relname = 'audit_log'
      AND pg_get_expr(c.relpartbound, c.oid) <> 'DEFAULT'
      -- upper bound of a monthly RANGE partition, parsed back to a date, strictly at/below cutoff
      AND (substring(pg_get_expr(c.relpartbound, c.oid) FROM 'TO \(''([0-9-]+)')::date) <= cutoff
  LOOP
    EXECUTE format('ALTER TABLE oikumenea.audit_log DETACH PARTITION oikumenea.%I', rec.relname);
    RETURN NEXT rec.relname;
  END LOOP;
END;
$$;

-- Safety-net catch-all so an insert never fails for a not-yet-created month; boot-time roll-forward
-- keeps live writes in their own monthly partitions, so DEFAULT should stay empty in practice.
CREATE TABLE oikumenea.audit_log_default PARTITION OF oikumenea.audit_log DEFAULT;

-- Seed the current and next month at migration time (whenever the migration runs).
SELECT oikumenea.ensure_audit_partition(CURRENT_DATE);
SELECT oikumenea.ensure_audit_partition((date_trunc('month', CURRENT_DATE) + interval '1 month')::date);

COMMENT ON COLUMN oikumenea.audit_log.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.created_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.actor_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.actor_person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.audit_log.subsystem IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.action IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.target_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.target_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.request_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.audit_log.before IS 'pii:special';
COMMENT ON COLUMN oikumenea.audit_log.after IS 'pii:special';
COMMENT ON COLUMN oikumenea.audit_log.outcome IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0001_audit_log', applied_at = now() WHERE singleton;
