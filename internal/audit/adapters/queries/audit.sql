-- Audit module queries (docs/modules/audit.md). The audit log is append-only: insert + read only,
-- never UPDATE/DELETE (reject_mutation() guards it at the DB).

-- name: InsertAuditEntry :exec
-- Records one Action in the same transaction as the write it describes (D-Audit). The id is the
-- supplied Action RID (action__<type>); the row commits iff the caller's transaction commits.
INSERT INTO oikumenea.audit_log (
  id, actor_type, actor_person_id, subsystem,
  action, target_type, target_id, unit_id,
  request_id, before, after, outcome
) VALUES (
  @id, @actor_type, @actor_person_id, @subsystem,
  @action, @target_type, @target_id, @unit_id,
  @request_id, @before, @after, @outcome
);

-- name: GetAuditEntry :one
-- Reads one entry by its Action RID.
SELECT * FROM oikumenea.audit_log WHERE id = @id;

-- name: QueryAuditLog :many
-- Filterable, keyset-paginated read over the log (D-Audit: filterable by every audited entity
-- type). Ordered newest-first by the (created_at, id) cursor. A NULL filter matches everything.
-- page_limit is fetched as N+1 by the caller to detect a further page.
SELECT * FROM oikumenea.audit_log
WHERE (sqlc.narg('actor_person_id')::uuid IS NULL OR actor_person_id = sqlc.narg('actor_person_id')::uuid)
  AND (sqlc.narg('actor_type')::text     IS NULL OR actor_type      = sqlc.narg('actor_type'))
  AND (sqlc.narg('target_type')::text    IS NULL OR target_type     = sqlc.narg('target_type'))
  AND (sqlc.narg('target_id')::text      IS NULL OR target_id       = sqlc.narg('target_id'))
  AND (sqlc.narg('unit_id')::uuid        IS NULL OR unit_id         = sqlc.narg('unit_id')::uuid)
  AND (sqlc.narg('action')::text         IS NULL OR action          = sqlc.narg('action'))
  AND (sqlc.narg('outcome')::text        IS NULL OR outcome         = sqlc.narg('outcome'))
  AND (sqlc.narg('since')::timestamptz   IS NULL OR created_at      >= sqlc.narg('since'))
  AND (sqlc.narg('until')::timestamptz   IS NULL OR created_at      <= sqlc.narg('until'))
  AND (
    sqlc.narg('cursor_id')::uuid IS NULL
    OR (created_at, id) < (sqlc.narg('cursor_created_at')::timestamptz, sqlc.narg('cursor_id')::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('page_limit');
