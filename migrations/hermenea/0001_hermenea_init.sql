-- Hermenea companion-service schema bootstrap (M16 / D-Hermenea).
--
-- Hermenea is a SEPARATE service with its OWN PostgreSQL (docs/modules/hermenea.md): it never shares
-- oikumenea's database and couples to oikumenea only over HTTP. This is its first (and so far only)
-- migration — the ingestion registry + raw staging + lineage ledger + the background-job queue that
-- the cron scheduler and worker run on. Plain `uuid` PKs (gen_random_uuid) — hermenea has its own
-- identity space and does not use oikumenea's RID machinery.
--
-- Tables:
--   import_sources       — registered external datasets (http|file), their oikumenea object-type target
--                          and optional cron.
--   import_raw_batches    — fetched payloads landed verbatim (re-mappable without re-fetch).
--   import_runs           — map+load lineage (counts, status, errors).
--   worker_jobs           — the at-least-once job queue (idempotency key, attempts, backoff via run_after,
--                          SKIP LOCKED claim) + ledger (status/last_error).
--   worker_schedules      — cron registrations the scheduler enqueues from.

CREATE SCHEMA IF NOT EXISTS hermenea;

-- updated_at maintenance trigger (hermenea's own copy; mirrors oikumenea.set_updated_at).
CREATE OR REPLACE FUNCTION hermenea.set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================ import_sources ============================
CREATE TABLE hermenea.import_sources (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code            text NOT NULL UNIQUE,
  name            text NOT NULL,
  connector_type  text NOT NULL CHECK (connector_type IN ('http','file')),
  object_type     text NOT NULL,
  locator         text NOT NULL,
  cron            text,
  enabled         boolean NOT NULL DEFAULT true,
  credentials_ref text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz
);
CREATE TRIGGER import_sources_set_updated_at
  BEFORE UPDATE ON hermenea.import_sources
  FOR EACH ROW EXECUTE FUNCTION hermenea.set_updated_at();

-- ============================ import_raw_batches ============================
CREATE TABLE hermenea.import_raw_batches (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id      uuid NOT NULL REFERENCES hermenea.import_sources(id) ON DELETE RESTRICT,
  source_version text,
  checksum       text NOT NULL,
  payload        bytea NOT NULL,
  fetched_at     timestamptz NOT NULL DEFAULT now(),
  created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_raw_batches_source_idx ON hermenea.import_raw_batches (source_id, fetched_at DESC);

-- ============================ import_runs ============================
CREATE TABLE hermenea.import_runs (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id      uuid NOT NULL REFERENCES hermenea.import_sources(id) ON DELETE RESTRICT,
  raw_batch_id   uuid REFERENCES hermenea.import_raw_batches(id) ON DELETE SET NULL,
  source_version text,
  status         text NOT NULL CHECK (status IN ('running','succeeded','failed')),
  created_count  integer NOT NULL DEFAULT 0,
  updated_count  integer NOT NULL DEFAULT 0,
  skipped_count  integer NOT NULL DEFAULT 0,
  error          text,
  started_at     timestamptz NOT NULL DEFAULT now(),
  finished_at    timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_runs_source_idx ON hermenea.import_runs (source_id, started_at DESC);
CREATE TRIGGER import_runs_set_updated_at
  BEFORE UPDATE ON hermenea.import_runs
  FOR EACH ROW EXECUTE FUNCTION hermenea.set_updated_at();

-- ============================ worker_jobs ============================
CREATE TABLE hermenea.worker_jobs (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type        text NOT NULL,
  idempotency_key text NOT NULL UNIQUE,
  source_id       uuid REFERENCES hermenea.import_sources(id) ON DELETE SET NULL,
  payload         jsonb,
  status          text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','dead')),
  attempts        integer NOT NULL DEFAULT 0,
  max_attempts    integer NOT NULL DEFAULT 5,
  run_after       timestamptz NOT NULL DEFAULT now(),
  locked_by       text,
  locked_at       timestamptz,
  last_error      text,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now()
);
-- The claim index: queued jobs ready to run, oldest first (FOR UPDATE SKIP LOCKED).
CREATE INDEX worker_jobs_claim_idx ON hermenea.worker_jobs (run_after) WHERE status = 'queued';
CREATE TRIGGER worker_jobs_set_updated_at
  BEFORE UPDATE ON hermenea.worker_jobs
  FOR EACH ROW EXECUTE FUNCTION hermenea.set_updated_at();

-- ============================ worker_schedules ============================
CREATE TABLE hermenea.worker_schedules (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id        uuid NOT NULL REFERENCES hermenea.import_sources(id) ON DELETE CASCADE,
  cron             text NOT NULL,
  enabled          boolean NOT NULL DEFAULT true,
  last_enqueued_at timestamptz,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER worker_schedules_set_updated_at
  BEFORE UPDATE ON hermenea.worker_schedules
  FOR EACH ROW EXECUTE FUNCTION hermenea.set_updated_at();
