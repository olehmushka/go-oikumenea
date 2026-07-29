-- 0017_facet_lists — the top-level, facet-filtered list endpoints (M56 ticket 3 / D-ObjectFacets):
-- GET /memberships, GET /orders, GET /documents.
--
-- Two things the endpoints need that the schema does not yet have.
--
-- 1. `authz_readable_units(subject)` — the subject's effective readable reach as a SET, callable from
--    any module's query. Three modules now need the same reach set; the alternative was a third and
--    fourth verbatim copy of a 30-line UNION whose parity with the Go PDP oracle
--    (authorization/domain ReachSet + classify) is the single most safety-critical invariant in the
--    codebase — exactly the drift reach_differential_integration_test.go exists to police.
--
--    NOT the same thing as authz_unit_in_reach(unit, wr) (0011), which answers the reach question for
--    ONE row and is what the RLS policies call. That per-row form is the correlated-probe shape M56
--    ticket 2 measured at 242 ms; used uncorrelated (`unit_id IN (SELECT ... FROM
--    authz_readable_units(...))`) the planner evaluates this set ONCE and probes a hash — the same
--    33 ms shape that replaced it. Both forms stay: RLS needs the per-row predicate, the list
--    endpoints need the set.
--
-- 2. Keyset indexes for the new filtered paths. Every membership_memberships index is PARTIAL on
--    `status = 'active'` (0003), because until now every membership read was an active-roster read.
--    The top-level list deliberately carries NO implicit status filter — a hidden default would make
--    M57's totalCount disagree with its own status distribution — so an unfiltered or `status=ended`
--    listing matches no existing index and sequential-scans. That is the R-21 failure mode the
--    depth-2 search-around work hit (cost 23660 -> 48 once a filter column matched the index
--    predicate), so the indexes below are non-partial in status.

-- ---------------------------------------------------------------------------------------------------
-- The reach set.
--
-- PARITY CONTRACT with authorization/domain ReachSet + classify (pdp.go), identical to the inline CTE
-- in membership.sql's VisiblePersonIDsForSubject* (which keeps its copy — those queries are measured
-- and green, and rewriting them to call this would perturb a just-measured plan for no behaviour
-- change): an assignment contributes iff it is active (revoked_at IS NULL, unexpired), its role is
-- not deleted, and the role carries any '*.read' permission; a 'unit' grant reaches its target only;
-- a 'subtree' grant reaches target + non-deleted closure descendants ONLY over an authority-bearing,
-- non-deleted graph (a directory-only subtree grant contributes NOTHING, not even its target —
-- D-DirectoryGraphs).
--
-- Verified equal to the inline CTE, for randomized subjects, by
-- internal/membership/reach_differential_integration_test.go.
--
-- STABLE (not IMMUTABLE): it reads tables and now(). SECURITY INVOKER (the default) — this must NOT
-- become a privilege escalation: it is a projection of the caller's own reach, and every table it
-- reads (authz_*, tenant_graphs, tenant_unit_closure) is deliberately RLS-EXEMPT because the reach
-- predicate reads them to COMPUTE reach and a reach-keyed policy there would be circular (0005).
--
-- The subject is an explicit PARAMETER rather than a read of current_setting('app.person_id'):
-- these are ordinary application queries that already carry the subject, and a GUC read would make
-- the function untestable off the request path.
CREATE OR REPLACE FUNCTION oikumenea.authz_readable_units(subject uuid)
RETURNS SETOF uuid
LANGUAGE sql STABLE AS $$
  SELECT a.target_unit_id AS unit_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
  WHERE a.subject_person_id = subject
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
  WHERE a.subject_person_id = subject
    AND a.scope = 'subtree'
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_readable_units(uuid) TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- The same reach question asked as a POINT PROBE, keyed on an explicit subject.
--
-- Two shapes exist because neither wins everywhere — the sparse/dense split R-02.1 already found for
-- the visible-persons queries, measured again here for the ticket-3 lists at 10^6 memberships /
-- 2x10^5 orders / 6x10^5 documents (first filtered page, review-2026-07):
--
--                       set form (authz_readable_units)   point probe (this function)
--   leaf reach (1)                        1.3 ms                    2 500 - 13 100 ms
--   mid reach (658)                      27 - 122 ms                  128 - 162 ms
--   root reach (100 000)                640 - 6 400 ms                  3.6 - 6.3 ms
--
-- The set form materializes the reach and semi-joins it: right when the reach is small, and at root
-- it forces the planner to drive from the reach side, build a 9x10^5-row person hash and top-N sort
-- the result — so the LIMIT never terminates early. The point probe leaves the driving table in
-- keyset order and asks the question per candidate row: right when nearly every row qualifies (the
-- LIMIT then terminates almost immediately), catastrophic when almost none do.
--
-- The adapters dispatch on CountReadableUnitsCapped, exactly as the visible-persons path does.
--
-- It is a FUNCTION rather than a fourth inlined copy of the reach predicate because the parity
-- contract below is the single most safety-critical invariant in the codebase; the differential test
-- holds this function, authz_readable_units, the inline CTE and the Go PDP oracle to one answer.
--
-- Distinct from authz_unit_in_reach(unit, wr) (0011), which reads the subject from the app.person_id
-- GUC because it is called by RLS policies. This one takes the subject explicitly: these are ordinary
-- application queries that already carry it, and a GUC read would make the function untestable off
-- the request path.
CREATE OR REPLACE FUNCTION oikumenea.authz_unit_readable_by(unit uuid, subject uuid) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
    FROM oikumenea.authz_role_assignments a
    JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
    WHERE a.subject_person_id = subject
      AND a.revoked_at IS NULL
      AND (a.expires_at IS NULL OR a.expires_at > now())
      AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                  WHERE rp.role_id = a.role_id AND rp.permission_code LIKE '%.read')
      AND ((a.scope = 'unit' AND a.target_unit_id = unit)
        OR (a.scope = 'subtree'
            AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                        WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
            AND (a.target_unit_id = unit
              OR EXISTS (SELECT 1
                         FROM oikumenea.tenant_unit_closure c
                         JOIN oikumenea.tenant_units u ON u.id = c.descendant_id AND u.deleted_at IS NULL
                         WHERE c.graph_id = a.graph_id
                           AND c.ancestor_id = a.target_unit_id
                           AND c.descendant_id = unit))))
  )
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_unit_readable_by(uuid, uuid) TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- The capped reach-cardinality probe the adapters dispatch on: how many units can this subject read,
-- counting no further than `cap`. Cheap because it stops at the cap — the question is never "how
-- big is the reach" but "is it bigger than the threshold".
--
-- A function, again, so that three modules share one definition. membership's own
-- CountReadableUnitsCapped query predates this and keeps its inline copy: the visible-persons path it
-- serves is measured and green, and rewriting it to call this would perturb a proven plan for no
-- behaviour change. The differential test holds them to the same answer.
CREATE OR REPLACE FUNCTION oikumenea.authz_readable_unit_count(subject uuid, cap integer)
RETURNS bigint
LANGUAGE sql STABLE AS $$
  SELECT count(*) FROM (
    SELECT oikumenea.authz_readable_units(subject) LIMIT cap
  ) capped
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_readable_unit_count(uuid, integer) TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- Keyset indexes for the new list paths.
--
-- Shape: (<filter column>, id) WHERE deleted_at IS NULL. The trailing id serves the keyset
-- `id > @after ORDER BY id LIMIT n`, so a filtered page is an index range scan rather than a filter
-- over a full id scan; the partial predicate matches the queries' own `deleted_at IS NULL`.

-- membership_memberships: the existing person/unit/position indexes are all partial on
-- status='active' and stay (they serve the roster reads). These add the status-agnostic forms.
CREATE INDEX IF NOT EXISTS membership_memberships_unit_keyset_idx
  ON oikumenea.membership_memberships (unit_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS membership_memberships_person_keyset_idx
  ON oikumenea.membership_memberships (person_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS membership_memberships_status_keyset_idx
  ON oikumenea.membership_memberships (status, id) WHERE deleted_at IS NULL;

-- order_orders: order_orders_issued_idx is partial on status='issued' and keyed on the unit only.
CREATE INDEX IF NOT EXISTS order_orders_unit_keyset_idx
  ON oikumenea.order_orders (issuing_unit_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS order_orders_status_keyset_idx
  ON oikumenea.order_orders (status, id) WHERE deleted_at IS NULL;
-- The orderTypeId facet is an EXISTS over the items; it probes from the item side.
CREATE INDEX IF NOT EXISTS order_order_items_type_order_idx
  ON oikumenea.order_order_items (type_id, order_id);

-- document_documents: document_documents_type_idx exists but is neither keyset-ordered nor partial
-- on deleted_at.
CREATE INDEX IF NOT EXISTS document_documents_type_keyset_idx
  ON oikumenea.document_documents (type_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS document_documents_status_keyset_idx
  ON oikumenea.document_documents (status, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS document_documents_expires_idx
  ON oikumenea.document_documents (expires_on) WHERE deleted_at IS NULL AND expires_on IS NOT NULL;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0017_facet_lists', applied_at = now() WHERE singleton;
