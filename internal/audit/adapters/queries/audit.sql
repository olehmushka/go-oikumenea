-- Audit module queries (docs/modules/audit.md). The audit log is append-only: insert + read only,
-- never UPDATE/DELETE (reject_mutation() guards it at the DB).

-- name: InsertAuditEntry :exec
-- Records one Action in the same transaction as the write it describes (D-Audit). The id is the
-- supplied Action RID (action__<type>); the row commits iff the caller's transaction commits.
INSERT INTO oikumenea.audit_log (
  id, actor_type, actor_person_id, subsystem, actor_principal_id,
  action, target_type, target_id, unit_id,
  request_id, before, after, outcome
) VALUES (
  @id, @actor_type, @actor_person_id, @subsystem, sqlc.narg('actor_principal_id'),
  @action, @target_type, @target_id, @unit_id,
  @request_id, @before, @after, @outcome
);

-- name: GetAuditEntry :one
-- Reads one entry by its Action RID.
SELECT * FROM oikumenea.audit_log WHERE id = @id;

-- name: EnsureAuditPartitions :exec
-- Roll the monthly partition window forward (review-2026-07 R-07): idempotently create the current
-- and next month's partition so live inserts always land in a real partition, never the DEFAULT
-- catch-all. Called at every boot under the boot-seed advisory lock (replica-safe, no-op once made).
SELECT oikumenea.ensure_audit_partition(CURRENT_DATE),
       oikumenea.ensure_audit_partition((date_trunc('month', CURRENT_DATE) + interval '1 month')::date);

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

-- ==================== the ledger's dashboard aggregate (M58 ticket 1) ====================

-- name: AuditStats :many
-- Every selected facet's distribution plus the total, in ONE round-trip and ONE scan of the candidate
-- set (M58 / D-ObjectFacets). The candidate CTE carries QueryAuditLog's filter block VERBATIM — minus
-- the keyset cursor, which is a page boundary and not a filter — so the list and the dashboard see one
-- world. A branch whose want_* flag is false is SKIPPED by the planner, not merely dropped from the
-- response, so asking for two facets costs two facets.
--
-- ONE ARM, unlike the five M57 types, and the reason is worth stating rather than inferring: audit
-- visibility is the RLS policy on audit_log (a unit_id reach probe; NULL-unit rows admin-only), not an
-- app-layer reach predicate the query folds in. There is therefore no subject to pass and no scoped
-- twin — but the read MUST run on the request-pinned connection, where the app.* GUCs the policy reads
-- are set. On the bare pool this same statement returns a confident ZERO.
--
-- created_at is the PARTITION KEY (monthly range partitions, review-2026-07 R-07): a since/until bound
-- prunes whole partitions, and without one this scans the entire ledger. That is why the console sends
-- a default window as a real filter rather than this query inventing one.
--
-- Day grain, not month: an audit trail is read day by day, and a monthly bar hides the spike an
-- auditor is looking for.
WITH cand AS MATERIALIZED (
  SELECT actor_type, actor_person_id, action, target_type, target_id, unit_id, outcome, created_at
  FROM oikumenea.audit_log
  WHERE (sqlc.narg('actor_person_id')::uuid IS NULL OR actor_person_id = sqlc.narg('actor_person_id')::uuid)
    AND (sqlc.narg('actor_type')::text     IS NULL OR actor_type      = sqlc.narg('actor_type'))
    AND (sqlc.narg('target_type')::text    IS NULL OR target_type     = sqlc.narg('target_type'))
    AND (sqlc.narg('target_id')::text      IS NULL OR target_id       = sqlc.narg('target_id'))
    AND (sqlc.narg('unit_id')::uuid        IS NULL OR unit_id         = sqlc.narg('unit_id')::uuid)
    AND (sqlc.narg('action')::text         IS NULL OR action          = sqlc.narg('action'))
    AND (sqlc.narg('outcome')::text        IS NULL OR outcome         = sqlc.narg('outcome'))
    AND (sqlc.narg('since')::timestamptz   IS NULL OR created_at      >= sqlc.narg('since'))
    AND (sqlc.narg('until')::timestamptz   IS NULL OR created_at      <= sqlc.narg('until'))
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'actorType'::text, c.actor_type::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_actor_type')::boolean GROUP BY 2
UNION ALL
SELECT 'outcome'::text, c.outcome::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_outcome')::boolean GROUP BY 2
UNION ALL
SELECT 'actorPersonId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.actor_person_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_actor_person_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'unitId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.unit_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_unit_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'action'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.action::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_action')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'targetType'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.target_type::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_target_type')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'targetId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.target_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_target_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'createdAt'::text, to_char(date_trunc('day', c.created_at), 'YYYY-MM-DD'), count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_created_at')::boolean GROUP BY 2;
