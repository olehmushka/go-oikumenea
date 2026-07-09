-- Resume cursor for chunked import runs (R-05 / M49). The worker records, per job, the last chunk
-- seq oikumenea acknowledged (HTTP 200) together with the checksum of the staged source it belongs
-- to. A retried attempt re-stages the source and — when the checksum still matches — skips
-- re-sending chunks with seq <= resume_seq; a changed checksum invalidates the cursor (full re-run,
-- still safe: chunk replay is idempotent server-side, so the cursor is a crash-resume optimization,
-- not a correctness requirement). Additive expand migration (upgrade-safety: no destructive change).
ALTER TABLE hermenea.worker_jobs
  ADD COLUMN resume_seq bigint NOT NULL DEFAULT 0,
  ADD COLUMN resume_checksum text;

COMMENT ON COLUMN hermenea.worker_jobs.resume_seq IS 'last oikumenea-acked chunk seq of the current chunked run (0 = none)';
COMMENT ON COLUMN hermenea.worker_jobs.resume_checksum IS 'staged-source checksum the cursor belongs to (mismatch on retry resets the cursor)';
