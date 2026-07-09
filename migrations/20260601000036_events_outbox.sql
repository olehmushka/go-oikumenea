-- 0036 events outbox (review-2026-07 R-10 / D-EventOutbox).
--
-- The transactional outbox for the `notify` event class (docs/architecture/patterns.md): effects that
-- must NOT widen the write transaction (webhooks, projections, cache invalidation, R-01's grant-cache
-- epoch bump). A producer enqueues one row on its own write transaction (pkg/events.OutboxWriter), so the
-- event commits atomically with the originating write; the out-of-process dispatcher
-- (internal/platform/outbox) drains the queue AFTER COMMIT, at least once, claiming rows FOR UPDATE SKIP
-- LOCKED so multiple replicas share it safely (mirrors the hermenea worker, R-13).
--
-- The atomic Bus (pkg/events, same-transaction subscribers) is unchanged and stays the default; today
-- every domain event is `atomic`, so this table is a live-but-empty seam proven by tests until the first
-- `notify` producer lands. Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0000 bootstrap
-- helpers (uuid_v7, set_updated_at). This is infrastructure plumbing (like schema_version) — NOT an
-- ontology entity, so the id is a plain time-ordered uuid, not a new_id() RID.

CREATE TABLE oikumenea.platform_outbox (
  -- Chronologically-ordered surrogate key (uuid_v7 time component orders the queue by enqueue time).
  id              uuid PRIMARY KEY DEFAULT oikumenea.uuid_v7(),
  event_type      text NOT NULL,                 -- the notify event's dispatch key (Event.Type())
  payload         jsonb NOT NULL,                -- the JSON-marshaled concrete event
  status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','dispatched','dead')),
  attempts        integer NOT NULL DEFAULT 0,    -- delivery attempts so far
  max_attempts    integer NOT NULL DEFAULT 10,   -- dead-letter after this many failures
  next_attempt_at timestamptz NOT NULL DEFAULT now(),  -- earliest next delivery (backoff schedule)
  last_error      text,                          -- last handler error (diagnostics)
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  dispatched_at   timestamptz                    -- set when a row reaches 'dispatched'
);

-- Claim index: the dispatcher scans due, still-pending rows oldest-first; partial so the index stays
-- small as dispatched/dead rows accumulate (retention is an operator concern, like audit partitions).
CREATE INDEX platform_outbox_due_idx
  ON oikumenea.platform_outbox (next_attempt_at, id)
  WHERE status = 'pending';

CREATE TRIGGER platform_outbox_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_outbox
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- No RLS: the outbox is system-actor infrastructure, never subject to the per-request unit reach
-- (only the four D-RLSDefenseInDepth tables carry policies). No reject_mutation trigger: rows are
-- mutable by design (status transitions pending -> dispatched / dead).

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0036_events_outbox', applied_at = now() WHERE singleton;
