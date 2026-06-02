-- 0002 audit log (M1).
--
-- The append-only Action ledger every later write commits into (D-Audit / docs/modules/audit.md).
-- Each row records one Action and is keyed by that Action's RID (action__<type>; D-Ontology):
-- the audit log IS the action ledger. Append-only — guarded by oikumenea.reject_mutation().
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap objects.

CREATE TABLE oikumenea.audit_log (
  -- PK = the Action RID of the write this row records (D-ResourceIdentifiers / D-Audit):
  -- self-describing and chronologically ordered via its uuid_v7() component. Supplied by the
  -- producing module's application service, not defaulted here.
  id              text PRIMARY KEY,

  created_at      timestamptz NOT NULL DEFAULT now(),

  -- The two actor kinds (D-Audit). There is no super_admin kind — an instance admin is a person.
  actor_type      text NOT NULL CHECK (actor_type IN ('person','system')),
  -- The person who acted (person RID); NOT NULL for person actions, NULL for system (CHECK below).
  actor_person_id text,
  -- For system actions, the originating source (bootstrap, recover-admin, purge-worker,
  -- closure-rebuild, event-subscriber, …); NOT NULL for system actions, NULL otherwise (CHECK below).
  subsystem       text,

  action          text NOT NULL,  -- e.g. assignment.grant, unit.transition, rank.scheme.update
  target_type     text NOT NULL,  -- e.g. unit, person, role_assignment, account, graph
  target_id       text,           -- the acted-on entity's RID (Object/Link/Action URN)
  unit_id         text,           -- unit context where applicable (for scoped audit reads)

  request_id      text NOT NULL,  -- correlation key shared with logs/metrics/traces

  -- State snapshot / change payload. No secrets; PII minimized. pii:special CEILING (D-PIITiers):
  -- special-category data must NOT land here until the envelope seam (DS-29) ships.
  before          jsonb,
  after           jsonb,

  outcome         text NOT NULL DEFAULT 'success' CHECK (outcome IN ('success','denied','error')),

  -- The Action RID shape: every audit key is an action__<type> RID (D-Ontology / conventions).
  CONSTRAINT audit_log_action_rid_shape CHECK (id LIKE 'urn:oikumenea:%:action\_\_%'),

  -- Actor-shape CHECK — the two kinds, mutually exclusive (D-Audit).
  CONSTRAINT audit_log_actor_shape CHECK (
    (actor_type = 'person' AND actor_person_id IS NOT NULL AND subsystem IS NULL)
    OR
    (actor_type = 'system' AND actor_person_id IS NULL AND subsystem IS NOT NULL)
  )
);

-- Append-only: no UPDATE/DELETE from application code; corrections are new entries (D-Audit).
CREATE TRIGGER audit_log_reject_mutation
  BEFORE UPDATE OR DELETE ON oikumenea.audit_log
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

-- Filter/correlation indexes (docs/modules/audit.md). created_at is the chronological read order;
-- the (created_at, id) keyset is the pagination cursor.
CREATE INDEX audit_log_created_at_id_idx ON oikumenea.audit_log (created_at DESC, id DESC);
CREATE INDEX audit_log_actor_person_idx  ON oikumenea.audit_log (actor_person_id);
CREATE INDEX audit_log_actor_type_idx    ON oikumenea.audit_log (actor_type);
CREATE INDEX audit_log_target_idx        ON oikumenea.audit_log (target_type, target_id);
CREATE INDEX audit_log_unit_idx          ON oikumenea.audit_log (unit_id);
CREATE INDEX audit_log_request_idx       ON oikumenea.audit_log (request_id);

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
UPDATE oikumenea.schema_version SET revision = '0002_audit_log', applied_at = now() WHERE singleton;
