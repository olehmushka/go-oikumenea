-- Hermenea companion-service queries (M16 / D-Hermenea), over hermenea's OWN database.

-- ============================ sources ============================

-- name: UpsertSource :one
INSERT INTO hermenea.import_sources (code, name, connector_type, object_type, locator, cron, enabled)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (code) DO UPDATE SET
  name = EXCLUDED.name, connector_type = EXCLUDED.connector_type, object_type = EXCLUDED.object_type,
  locator = EXCLUDED.locator, cron = EXCLUDED.cron, enabled = EXCLUDED.enabled
RETURNING id, code, name, connector_type, object_type, locator, cron, enabled, credentials_ref, created_at, updated_at, deleted_at;

-- name: ListSources :many
SELECT id, code, name, connector_type, object_type, locator, cron, enabled, credentials_ref, created_at, updated_at, deleted_at
FROM hermenea.import_sources
WHERE deleted_at IS NULL
ORDER BY code;

-- name: GetSourceByCode :one
SELECT id, code, name, connector_type, object_type, locator, cron, enabled, credentials_ref, created_at, updated_at, deleted_at
FROM hermenea.import_sources
WHERE code = $1 AND deleted_at IS NULL;

-- ============================ schedules ============================

-- name: UpsertSchedule :exec
INSERT INTO hermenea.worker_schedules (source_id, cron, enabled)
VALUES ($1, $2, true)
ON CONFLICT DO NOTHING;

-- name: ListEnabledSchedules :many
SELECT sch.id, sch.source_id, sch.cron, sch.last_enqueued_at, src.code AS source_code
FROM hermenea.worker_schedules sch
JOIN hermenea.import_sources src ON src.id = sch.source_id
WHERE sch.enabled AND src.enabled AND src.deleted_at IS NULL;

-- name: TouchSchedule :exec
UPDATE hermenea.worker_schedules SET last_enqueued_at = now() WHERE id = $1;

-- ============================ worker_jobs ============================

-- name: EnqueueJob :one
INSERT INTO hermenea.worker_jobs (job_type, idempotency_key, source_id, payload, max_attempts, run_after)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (idempotency_key) DO UPDATE SET idempotency_key = EXCLUDED.idempotency_key
RETURNING id, status;

-- name: ClaimJob :one
UPDATE hermenea.worker_jobs
SET status = 'running', locked_by = $1, locked_at = now(), attempts = attempts + 1
WHERE id = (
  SELECT id FROM hermenea.worker_jobs
  WHERE status = 'queued' AND run_after <= now()
  ORDER BY run_after
  FOR UPDATE SKIP LOCKED
  LIMIT 1
)
RETURNING id, job_type, idempotency_key, source_id, payload, status, attempts, max_attempts, run_after, locked_by, locked_at, last_error, created_at, updated_at;

-- name: MarkJobSucceeded :exec
UPDATE hermenea.worker_jobs
SET status = 'succeeded', last_error = NULL, locked_by = NULL, locked_at = NULL
WHERE id = $1;

-- name: RescheduleJob :exec
UPDATE hermenea.worker_jobs
SET status = 'queued', run_after = $2, last_error = $3, locked_by = NULL, locked_at = NULL
WHERE id = $1;

-- name: DeadLetterJob :exec
UPDATE hermenea.worker_jobs
SET status = 'dead', last_error = $2, locked_by = NULL, locked_at = NULL
WHERE id = $1;

-- name: RequeueStaleRunning :exec
UPDATE hermenea.worker_jobs
SET status = 'queued', locked_by = NULL, locked_at = NULL
WHERE status = 'running' AND locked_at < $1;

-- name: ListJobs :many
SELECT id, job_type, idempotency_key, source_id, payload, status, attempts, max_attempts, run_after, locked_by, locked_at, last_error, created_at, updated_at
FROM hermenea.worker_jobs
ORDER BY created_at DESC
LIMIT $1;

-- name: CountUnhealthyJobs :one
SELECT count(*) FROM hermenea.worker_jobs WHERE status = 'dead';

-- ============================ raw batches ============================

-- name: InsertRawBatch :one
INSERT INTO hermenea.import_raw_batches (source_id, source_version, checksum, payload, fetched_at)
VALUES ($1, $2, $3, $4, now())
RETURNING id;

-- name: InsertRawBatchRef :one
-- A large streamed source staged on disk (D-GeoPlaces): the body is a file reference, not inline bytes.
INSERT INTO hermenea.import_raw_batches (source_id, source_version, checksum, staged_path, fetched_at)
VALUES ($1, $2, $3, $4, now())
RETURNING id;

-- ============================ runs ============================

-- name: StartRun :one
INSERT INTO hermenea.import_runs (source_id, raw_batch_id, source_version, status)
VALUES ($1, $2, $3, 'running')
RETURNING id;

-- name: FinishRun :exec
UPDATE hermenea.import_runs
SET status = $2, created_count = $3, updated_count = $4, skipped_count = $5, error = $6, finished_at = now()
WHERE id = $1;

-- name: ListRuns :many
SELECT id, source_id, raw_batch_id, source_version, status, created_count, updated_count, skipped_count, error, started_at, finished_at, created_at, updated_at
FROM hermenea.import_runs
ORDER BY started_at DESC
LIMIT $1;
