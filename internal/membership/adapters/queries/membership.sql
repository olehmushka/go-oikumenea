-- Membership module queries (docs/modules/membership.md). Unit-owned billets (membership_positions)
-- and the reified person->unit belonging/filling Link (membership_memberships). RID PKs default at
-- the database. Positions/memberships are reversible: abolish/end flip status rather than delete. A
-- NULL narg leaves the stored value unchanged on update (COALESCE); `code` and unit are immutable.
-- Existence of the referenced person/unit/position/rank is validated by the FKs (mapped in the
-- adapter), so these queries carry no pre-check lookups.

-- ============================ positions ============================

-- name: InsertPosition :one
-- Create a billet in a unit (vacant). The tenant_units / rank_ranks FKs validate unit + required rank.
-- sort_order, omitted, appends after the unit's current max among active positions.
INSERT INTO oikumenea.membership_positions (unit_id, code, title, required_rank_id, sort_order)
VALUES (
  @unit_id, @code, @title, sqlc.narg('required_rank_id'),
  COALESCE(sqlc.narg('sort_order'), (
    SELECT COALESCE(MAX(sort_order), 0) + 1 FROM oikumenea.membership_positions
    WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  ))
)
RETURNING *;

-- name: GetPosition :one
SELECT * FROM oikumenea.membership_positions WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePosition :one
-- Partial update: a NULL narg leaves the value unchanged. `code` and unit_id are immutable; clearing
-- required_rank_id to NULL via this path is an open seam (COALESCE cannot set NULL).
UPDATE oikumenea.membership_positions SET
  title            = COALESCE(sqlc.narg('title'), title),
  required_rank_id = COALESCE(sqlc.narg('required_rank_id'), required_rank_id),
  sort_order       = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: AbolishPosition :one
-- Reversible status flip; only an active position can be abolished. The in-use guard (active filling)
-- is enforced in the application before this runs.
UPDATE oikumenea.membership_positions SET status = 'abolished'
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: ListPositionsByUnit :many
-- All active positions in a unit, keyset-paginated by RID.
SELECT * FROM oikumenea.membership_positions
WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListVacantPositionsByUnit :many
-- Active positions with NO active filling (vacancy = the derived predicate), keyset-paginated.
SELECT p.* FROM oikumenea.membership_positions p
WHERE p.unit_id = @unit_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.position_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- name: ListFilledPositionsByUnit :many
-- Active positions that HAVE an active filling, keyset-paginated.
SELECT p.* FROM oikumenea.membership_positions p
WHERE p.unit_id = @unit_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.position_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- ============================ memberships ============================

-- name: InsertMembership :one
-- Add a belonging/filling. The person/unit/position FKs validate existence; the one-holder and
-- plain-belonging partial-unique indexes enforce single occupancy (mapped in the adapter).
INSERT INTO oikumenea.membership_memberships (
  person_id, unit_id, position_id, order_item_id, effective_from
) VALUES (
  @person_id, @unit_id, sqlc.narg('position_id'), sqlc.narg('order_item_id'),
  COALESCE(sqlc.narg('effective_from')::timestamptz, now())
)
RETURNING *;

-- name: GetMembership :one
SELECT * FROM oikumenea.membership_memberships WHERE id = @id AND deleted_at IS NULL;

-- name: EndMembership :one
-- Reversible end: flip status to ended and stamp effective_to; only an active membership can end.
-- An order_item_id provenance pointer may be attached at end time (NULL narg leaves it unchanged).
UPDATE oikumenea.membership_memberships SET
  status        = 'ended',
  effective_to  = COALESCE(sqlc.narg('effective_to')::timestamptz, now()),
  order_item_id = COALESCE(sqlc.narg('order_item_id'), order_item_id)
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: GetActiveFillingByPosition :one
-- The current holder of a position (its single active filling), if any.
SELECT * FROM oikumenea.membership_memberships
WHERE position_id = @position_id AND status = 'active' AND deleted_at IS NULL;

-- name: GetActivePlainMembership :one
-- A person's active PLAIN belonging (no position) in a unit, if any — the target an order's
-- membership-end item ends when it names a unit but no position. The belonging index keeps it unique.
SELECT * FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND unit_id = @unit_id
  AND position_id IS NULL AND status = 'active' AND deleted_at IS NULL;

-- name: ListMembersByUnit :many
-- A unit's active memberships (its roster), keyset-paginated by RID.
SELECT * FROM oikumenea.membership_memberships
WHERE unit_id = @unit_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListMembershipsByPerson :many
-- A person's active memberships across units, keyset-paginated by RID.
SELECT * FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- ============================ top-level facet-filtered list (M56 ticket 3 / D-ObjectFacets) ============================
-- GET /memberships. Two shapes, ONE filter block, byte-identical between them: the admin path and the
-- reach-scoped path must select the same rows for the same filters, differing ONLY by the reach
-- predicate. sqlparity_test.go proves the block is present in both with no database.
--
-- NO implicit status filter, unlike ListMembersByUnit / ListMembershipsByPerson above: the top-level
-- listing returns every status, because a hidden default would make M57's totalCount disagree with
-- its own status distribution. Narrow with @status.
--
-- Every predicate is folded INTO the SQL, before the LIMIT (review-2026-07 R-06). A Go-side
-- re-filter after the page is cut would return a short page WITH a nextPageToken.
--
-- The `sqlc.narg('x')::type IS NULL OR col = ...` style is deliberate and load-bearing: R-21 BANS the
-- `(@arg = '' OR ...)` sentinel because the planner cannot prove the arg non-empty under a generic
-- prepared plan and sequential-scans.
--
-- effective_from is a timestamptz; the bounds are calendar dates, so the upper bound compares against
-- the END of the given day (< the next day) rather than midnight, which would silently exclude every
-- row on the boundary date.

-- name: ListMemberships :many
-- Instance-admin path: the whole membership set, keyset-paginated by RID.
SELECT * FROM oikumenea.membership_memberships m
WHERE m.deleted_at IS NULL
  AND (@after = '' OR m.id::text > @after)
  AND (sqlc.narg('unit_id')::uuid IS NULL OR m.unit_id = sqlc.narg('unit_id')::uuid)
  AND (sqlc.narg('person_id')::uuid IS NULL OR m.person_id = sqlc.narg('person_id')::uuid)
  AND (sqlc.narg('position_id')::uuid IS NULL OR m.position_id = sqlc.narg('position_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR m.status = sqlc.narg('status')::text)
  AND (sqlc.narg('effective_from_after')::date IS NULL
       OR m.effective_from >= sqlc.narg('effective_from_after')::date)
  AND (sqlc.narg('effective_from_before')::date IS NULL
       OR m.effective_from < (sqlc.narg('effective_from_before')::date + 1))
ORDER BY m.id
LIMIT @lim;

-- name: ListMembershipsForSubject :many
-- Read-scope path: the same set intersected with the subject's effective readable reach. The reach
-- set is UNCORRELATED — it reads only @subject_person_id, so the planner evaluates it once and probes
-- a hash, rather than re-deriving the closure per candidate row (M56 ticket 2: 242 ms -> 33 ms).
SELECT * FROM oikumenea.membership_memberships m
WHERE m.deleted_at IS NULL
  AND (@after = '' OR m.id::text > @after)
  AND (sqlc.narg('unit_id')::uuid IS NULL OR m.unit_id = sqlc.narg('unit_id')::uuid)
  AND (sqlc.narg('person_id')::uuid IS NULL OR m.person_id = sqlc.narg('person_id')::uuid)
  AND (sqlc.narg('position_id')::uuid IS NULL OR m.position_id = sqlc.narg('position_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR m.status = sqlc.narg('status')::text)
  AND (sqlc.narg('effective_from_after')::date IS NULL
       OR m.effective_from >= sqlc.narg('effective_from_after')::date)
  AND (sqlc.narg('effective_from_before')::date IS NULL
       OR m.effective_from < (sqlc.narg('effective_from_before')::date + 1))
  AND m.unit_id IN (SELECT oikumenea.authz_readable_units(@subject_person_id))
ORDER BY m.id
LIMIT @lim;

-- name: CountReadableUnitsForDispatch :one
-- The capped reach-cardinality probe the sparse/dense list dispatch reads (migration 0017). Capped,
-- because the question is never "how big is the reach" but "is it past the threshold".
SELECT oikumenea.authz_readable_unit_count(@subject_person_id, @cap::integer) AS n;

-- name: ListMembershipsForSubjectDense :many
-- DENSE-reach plan shape of the query above, byte-identical in its filter block and differing ONLY in
-- how reach is applied: a per-row point probe instead of a materialized reach set.
--
-- Why two shapes (measured, review-2026-07 — the same sparse/dense split R-02.1 found for the
-- visible-persons queries): materializing a 100 000-unit reach makes the planner drive from the reach
-- side, build a 9x10^5-row hash and top-N sort, so the LIMIT never terminates early (1 937 ms at root
-- reach). The point probe keeps membership_memberships in keyset order and asks per candidate row —
-- 6 ms at root, because nearly every row qualifies and the LIMIT is satisfied almost immediately. At
-- SPARSE reach the same probe is catastrophic (13 100 ms at reach 1) because almost no row qualifies
-- and it scans the table to find out, which is exactly why the adapter dispatches rather than picking.
SELECT * FROM oikumenea.membership_memberships m
WHERE m.deleted_at IS NULL
  AND (@after = '' OR m.id::text > @after)
  AND (sqlc.narg('unit_id')::uuid IS NULL OR m.unit_id = sqlc.narg('unit_id')::uuid)
  AND (sqlc.narg('person_id')::uuid IS NULL OR m.person_id = sqlc.narg('person_id')::uuid)
  AND (sqlc.narg('position_id')::uuid IS NULL OR m.position_id = sqlc.narg('position_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR m.status = sqlc.narg('status')::text)
  AND (sqlc.narg('effective_from_after')::date IS NULL
       OR m.effective_from >= sqlc.narg('effective_from_after')::date)
  AND (sqlc.narg('effective_from_before')::date IS NULL
       OR m.effective_from < (sqlc.narg('effective_from_before')::date + 1))
  AND oikumenea.authz_unit_readable_by(m.unit_id, @subject_person_id)
ORDER BY m.id
LIMIT @lim;

-- name: ReadableUnitIDsForSubject :many
-- The reach set as the SQL function projects it — the differential test's probe, asserting the
-- function agrees with the inline CTE below for randomized subjects (migration 0017's parity claim).
SELECT oikumenea.authz_readable_units(@subject_person_id)::uuid AS unit_id;

-- name: ActiveUnitIDsByPerson :many
-- The distinct units a person currently belongs to via ACTIVE memberships. The person/document
-- read-scope projection (D-PersonReadScope) intersects this set with the reader's effective readable
-- units to decide visibility.
SELECT DISTINCT unit_id FROM oikumenea.membership_memberships
WHERE person_id = @person_id AND status = 'active' AND deleted_at IS NULL
ORDER BY unit_id;

-- name: ActivePersonIDsInUnits :many
-- The distinct persons with an ACTIVE membership in any of the given units, keyset-paginated by person
-- RID. Powers the directory-list union (GET /persons) under D-PersonReadScope: the caller passes its
-- effective readable unit-set and pages the reachable roster.
SELECT DISTINCT person_id FROM oikumenea.membership_memberships
WHERE unit_id = ANY(@unit_ids::uuid[]) AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR person_id::text > @after)
ORDER BY person_id
LIMIT @lim;

-- ============================ reach semi-join (D-PersonReadScope, review-2026-07 R-02.1) ============================
-- The subject's effective readable reach computed IN the database — the reach set never leaves
-- Postgres (replaces the app-side ReachSet flatten + ANY(array) union). PARITY CONTRACT with
-- authorization/domain ReachSet + classify (pdp.go): an assignment contributes iff it is active
-- (revoked_at IS NULL, unexpired), its role is not deleted, and the role carries any '*.read'
-- permission; a 'unit' grant reaches its target only; a 'subtree' grant reaches target +
-- non-deleted closure descendants ONLY over an authority-bearing, non-deleted graph (a
-- directory-only subtree grant contributes NOTHING, not even its target — D-DirectoryGraphs).
-- Cross-module join precedent: ActiveGrantsForSubject ⋈ tenant_graphs (authorization.sql).
-- Verified against the Go oracle by the randomized differential test
-- (reach_differential_integration_test.go).

-- name: CountReadableUnitsCapped :one
-- Capped cardinality probe of the subject's readable reach (same parity contract): the adapter uses
-- it to pick the visible-persons plan shape — sparse reach drives from the unit set (semi-join),
-- dense reach drives a person-ordered scan with a correlated reach probe. Counting stops at @cap.
SELECT count(*) FROM (
  SELECT a.target_unit_id AS unit_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
  WHERE a.subject_person_id = @subject_person_id
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
    AND (a.scope = 'unit'
         OR EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                    WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL))
  UNION
  SELECT c.descendant_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r  ON r.id = a.role_id AND r.deleted_at IS NULL
  JOIN oikumenea.tenant_graphs g ON g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL
  JOIN oikumenea.tenant_unit_closure c
       ON c.graph_id = a.graph_id AND c.ancestor_id = a.target_unit_id
  JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
  WHERE a.subject_person_id = @subject_person_id
    AND a.scope = 'subtree'
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
  LIMIT @cap
) capped;

-- name: VisiblePersonIDsForSubjectSparse :many
-- SPARSE-reach plan shape: materialize the (small) readable unit set, semi-join memberships via the
-- unit index. O(|reach| + page) — wrong shape for a near-root subtree subject (the whole reach
-- materializes before the LIMIT), hence the adapter's cardinality dispatch.
WITH readable AS (
  SELECT a.target_unit_id AS unit_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
  WHERE a.subject_person_id = @subject_person_id
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
    AND (a.scope = 'unit'
         OR EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                    WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL))
  UNION
  SELECT c.descendant_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r  ON r.id = a.role_id AND r.deleted_at IS NULL
  JOIN oikumenea.tenant_graphs g ON g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL
  JOIN oikumenea.tenant_unit_closure c
       ON c.graph_id = a.graph_id AND c.ancestor_id = a.target_unit_id
  JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
  WHERE a.subject_person_id = @subject_person_id
    AND a.scope = 'subtree'
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
)
--
-- DRIVEN FROM person_persons in keyset order (M56). Two reasons, both measured on the M46 scale
-- world (10^6 persons / 10^5 units, reach 658):
--
--   * the facet predicates sit on the driving relation, so a filtered page terminates early instead
--     of materializing every in-reach membership and filtering afterwards. The membership-driven
--     form with a person join measured 120 ms unfiltered / 200 ms with a unit filter against
--     65 ms / 140 ms for this shape;
--   * `p.deleted_at IS NULL` closes a pre-existing hole: the old shape returned soft-deleted person
--     ids that ListPersonsByIDs then silently dropped, yielding a page SHORTER than pageSize while
--     still handing back a nextPageToken.
--
-- It also drops the DISTINCT + sort (a person with several in-reach memberships matched more than
-- once), and makes all three visibility shapes structurally identical — Search already drove from
-- person_persons, so this is the shape that was already proven.
SELECT p.id
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL
  AND (@after = '' OR p.id::text > @after)
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
  AND EXISTS (
    SELECT 1
    FROM oikumenea.membership_memberships m
    JOIN readable rd ON rd.unit_id = m.unit_id
    WHERE m.person_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL)
ORDER BY p.id
LIMIT @lim;

-- name: SubjectCanReadPerson :one
-- Point probe of the same reach predicate: does any of the person's active-membership units fall in
-- the subject's readable reach? (Same parity contract as VisiblePersonIDsForSubject.)
SELECT EXISTS (
  SELECT 1
  FROM oikumenea.membership_memberships m
  WHERE m.person_id = @person_id AND m.status = 'active' AND m.deleted_at IS NULL
    AND EXISTS (
      SELECT 1
      FROM oikumenea.authz_role_assignments a
      JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
      WHERE a.subject_person_id = @subject_person_id
        AND a.revoked_at IS NULL
        AND (a.expires_at IS NULL OR a.expires_at > now())
        AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                    WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
        AND ((a.scope = 'unit' AND a.target_unit_id = m.unit_id)
          OR (a.scope = 'subtree'
              AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                          WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
              AND (a.target_unit_id = m.unit_id
                OR EXISTS (SELECT 1
                           FROM oikumenea.tenant_unit_closure c
                           JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                           WHERE c.graph_id = a.graph_id
                             AND c.ancestor_id = a.target_unit_id
                             AND c.descendant_id = m.unit_id))))
    )
) AS can_read;

-- name: VisiblePersonIDsForSubjectDense :many
-- DENSE-reach plan shape: probe the reach predicate per candidate person — O(page) when most
-- memberships are in reach (near-root subtree subjects), pathological for tiny reach (every
-- non-matching row still probes), hence the adapter's cardinality dispatch. Same parity contract as
-- the sparse shape, which the reach differential test asserts directly.
--
-- Driven from person_persons in keyset order (M56), like the sparse and search shapes: the facet
-- predicates then sit on the driving relation. Measured on the M46 scale world at reach 10^5, the
-- membership-driven form took 816 ms with a unit filter against 30 ms for this shape, because it
-- walked memberships that the filter went on to discard.
SELECT p.id
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL
  AND (@after = '' OR p.id::text > @after)
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
  AND EXISTS (
    SELECT 1
    FROM oikumenea.membership_memberships m
    WHERE m.person_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
      AND EXISTS (
        SELECT 1
        FROM oikumenea.authz_role_assignments a
        JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
        WHERE a.subject_person_id = @subject_person_id
          AND a.revoked_at IS NULL
          AND (a.expires_at IS NULL OR a.expires_at > now())
          AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                      WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
          AND ((a.scope = 'unit' AND a.target_unit_id = m.unit_id)
            OR (a.scope = 'subtree'
                AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                            WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
                AND (a.target_unit_id = m.unit_id
                  OR EXISTS (SELECT 1
                             FROM oikumenea.tenant_unit_closure c
                             JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                             WHERE c.graph_id = a.graph_id
                               AND c.ancestor_id = a.target_unit_id
                               AND c.descendant_id = m.unit_id))))
      ))
ORDER BY p.id
LIMIT @lim;

-- name: VisiblePersonIDsForSubjectSearch :many
-- Scoped directory SEARCH (review R-06): the visible-set union narrowed by a non-empty trigram
-- @query. Lead with the (highly selective) trigram match so it stays a GIN bitmap scan — the UNION
-- of person + name-variant id sets keeps BOTH branches indexable (an `OR EXISTS(...)` predicate is
-- not) — then probe the subject's readable reach per candidate (the same predicate as
-- SubjectCanReadPerson). Because search is selective this needs no sparse/dense split. Keyset by
-- person RID so the page fills correctly and the cursor is stable.
SELECT p.id
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL
  AND (@after = '' OR p.id::text > @after)
  AND p.id IN (
    SELECT ps.id FROM oikumenea.person_persons ps
      WHERE ps.deleted_at IS NULL AND ps.search_text ILIKE '%' || @query || '%'
    UNION
    SELECT v.person_id FROM oikumenea.person_name_variants v
      WHERE v.search_text ILIKE '%' || @query || '%')
  -- The M56 facet block, identical to the other visibility shapes. It sits AFTER the selective
  -- trigram set so the GIN bitmap scan still leads and the facets narrow a small candidate set.
  AND (sqlc.narg('sex')::text IS NULL OR p.sex = sqlc.narg('sex')::text)
  AND (sqlc.narg('status')::text IS NULL OR p.status = sqlc.narg('status')::text)
  AND (sqlc.narg('birthdate_from')::date IS NULL OR p.birthdate >= sqlc.narg('birthdate_from')::date)
  AND (sqlc.narg('birthdate_to')::date IS NULL OR p.birthdate <= sqlc.narg('birthdate_to')::date)
  AND (sqlc.narg('country_of_birth_id')::uuid IS NULL OR p.country_of_birth_id = sqlc.narg('country_of_birth_id')::uuid)
  AND (sqlc.narg('rank_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL
          AND pr.rank_id = sqlc.narg('rank_id')::uuid))
  AND (sqlc.narg('has_account')::boolean IS NULL
       OR sqlc.narg('has_account')::boolean = EXISTS (
            SELECT 1 FROM oikumenea.account_accounts ac
            WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND (sqlc.narg('filter_unit_id')::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          -- The unit itself plus its closure descendants, as an UNCORRELATED set: this subquery
          -- reads only the two facet parameters, never p, so the planner evaluates it once and
          -- probes it as a hash. The correlated form (a closure lookup per candidate person)
          -- measured 242 ms at 10^6 persons against 33 ms for this shape, because it re-walked the
          -- closure for every row the keyset scan touched to fill a page.
          AND fm.unit_id IN (
            SELECT sqlc.narg('filter_unit_id')::uuid
            UNION
            SELECT c.descendant_id
            FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = sqlc.narg('filter_unit_id')::uuid
              AND g.deleted_at IS NULL
              AND g.is_authority_bearing
              AND (sqlc.narg('filter_graph')::text IS NULL OR g.code = sqlc.narg('filter_graph')::text))))
  AND EXISTS (
    SELECT 1
    FROM oikumenea.membership_memberships m
    WHERE m.person_id = p.id AND m.status = 'active' AND m.deleted_at IS NULL
      AND EXISTS (
        SELECT 1
        FROM oikumenea.authz_role_assignments a
        JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
        WHERE a.subject_person_id = @subject_person_id
          AND a.revoked_at IS NULL
          AND (a.expires_at IS NULL OR a.expires_at > now())
          AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                      WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
          AND ((a.scope = 'unit' AND a.target_unit_id = m.unit_id)
            OR (a.scope = 'subtree'
                AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                            WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
                AND (a.target_unit_id = m.unit_id
                  OR EXISTS (SELECT 1
                             FROM oikumenea.tenant_unit_closure c
                             JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                             WHERE c.graph_id = a.graph_id
                               AND c.ancestor_id = a.target_unit_id
                               AND c.descendant_id = m.unit_id))))
      )
  )
ORDER BY p.id
LIMIT @lim;

-- name: SubjectReadablePersonsAmong :many
-- Batch variant of SubjectCanReadPerson for the D-VisibilityScope person-scope adapter (R-30):
-- which of the candidate persons does the subject's '*.read' reach through an active membership?
-- Same PARITY CONTRACT as SubjectCanReadPerson / VisiblePersonIDsForSubject; unnest-probe shape
-- mirrors authorization's ReadableUnitsForSubjectAmong.
SELECT cand.person_id::uuid AS person_id
FROM unnest(@person_ids::uuid[]) AS cand(person_id)
WHERE EXISTS (
  SELECT 1
  FROM oikumenea.membership_memberships m
  WHERE m.person_id = cand.person_id AND m.status = 'active' AND m.deleted_at IS NULL
    AND EXISTS (
      SELECT 1
      FROM oikumenea.authz_role_assignments a
      JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
      WHERE a.subject_person_id = @subject_person_id
        AND a.revoked_at IS NULL
        AND (a.expires_at IS NULL OR a.expires_at > now())
        AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                    WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
        AND ((a.scope = 'unit' AND a.target_unit_id = m.unit_id)
          OR (a.scope = 'subtree'
              AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                          WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
              AND (a.target_unit_id = m.unit_id
                OR EXISTS (SELECT 1
                           FROM oikumenea.tenant_unit_closure c
                           JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                           WHERE c.graph_id = a.graph_id
                             AND c.ancestor_id = a.target_unit_id
                             AND c.descendant_id = m.unit_id))))
    )
);
