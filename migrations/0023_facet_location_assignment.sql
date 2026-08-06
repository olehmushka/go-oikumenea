-- 0023_facet_location_assignment — the last two M58 types (ticket 6 / D-ObjectFacets): `location` and
-- `assignment` (link__has_role).
--
-- Additive and expand-only: three functions, four indexes, eight column comments. Nothing is
-- rewritten, nothing is dropped.
--
-- Two things the endpoints need that the schema does not have.
--
-- 1. A reach probe keyed on ONE permission code. Every reach helper the codebase has —
--    authz_readable_units / authz_unit_readable_by / authz_readable_unit_count (0017), and
--    authz_unit_in_reach (0011) — asks whether the subject holds ANY '%.read' code on a unit. That is
--    the right question for a module gated on its own read code, where the endpoint has already
--    checked `membership.read` and reach is only trimming rows. It is the WRONG question for
--    `listAssignments`: generic read-reach is a strict superset of assignment.read-reach, so trimming
--    the grant table with it would let a caller holding, say, `person.read` over a unit and
--    `assignment.read` somewhere else see grants they cannot see today. The endpoint would have got
--    WIDER by acquiring a filter. See §1 below.
--
-- 2. Keyset and grouping indexes for the new unconditional list. `listAssignments` could previously be
--    asked only by subject or by unit, and authz_role_assignments has exactly those two indexes; the
--    unconditional list pages by RID and the dashboard groups by role and graph, none of which any
--    existing index serves. All are PARTIAL on `revoked_at IS NULL`, which is not a hidden default
--    leaking into the schema but the population both surfaces describe (the active-only rule stands —
--    M58 ticket 3).
--
-- pii: the sweep came back DIRTY for the fifth time in six tickets. location_locations.type_id — a
-- FACETED column — carried no classification at all, and neither did the six address-part columns it
-- sits beside, even though raw_address and the search_text that folds all of them are both classified.
-- Corrected in §3; the values are unchanged, only the schema's account of them.

-- ---------------------------------------------------------------------------------------------------
-- §1. Reach, asked for one permission code.
--
-- These are the 0017 trio with `rp.permission_code LIKE '%.read'` replaced by
-- `rp.permission_code = permission`. Everything else — the PARITY CONTRACT with authorization/domain
-- ReachSet + classify (pdp.go) — is carried over verbatim: an assignment contributes iff it is active
-- (revoked_at IS NULL, unexpired), its role is not deleted, and the role carries the named permission;
-- a 'unit' grant reaches its target only; a 'subtree' grant reaches target + non-deleted closure
-- descendants ONLY over an authority-bearing, non-deleted graph (a directory-only subtree grant
-- contributes NOTHING, not even its target — D-DirectoryGraphs).
--
-- EXACT equality rather than a LIKE pattern parameter, deliberately: a pattern argument would make
-- `'%'` — every permission, i.e. reach wider than any read code grants — expressible by a typo at a
-- call site. The generic family question already has three functions of its own; this one answers a
-- different question and cannot be talked into answering that one.
--
-- The 0017 trio is left UNTOUCHED rather than reimplemented in terms of these. Its comments record
-- that its plans are measured, and rewriting a proven plan for no behaviour change is exactly what
-- those comments warn against; the differential test holds the family to one answer.
--
-- STABLE (they read tables and now()) and SECURITY INVOKER (the default) — a projection of the
-- caller's own reach, never an escalation. Every table read here is deliberately RLS-EXEMPT, because
-- the reach predicate reads them to COMPUTE reach and a reach-keyed policy there would be circular
-- (0005).

-- The set form: the subject's reach for one permission, for an uncorrelated semi-join.
CREATE OR REPLACE FUNCTION oikumenea.authz_readable_units_with(subject uuid, permission text)
RETURNS SETOF uuid
LANGUAGE sql STABLE AS $$
  SELECT a.target_unit_id AS unit_id
  FROM oikumenea.authz_role_assignments a
  JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
  WHERE a.subject_person_id = subject
    AND a.revoked_at IS NULL
    AND (a.expires_at IS NULL OR a.expires_at > now())
    AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                WHERE rp.role_id = a.role_id AND rp.permission_code = permission)
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
                WHERE rp.role_id = a.role_id AND rp.permission_code = permission)
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_readable_units_with(uuid, text) TO oikumenea_app;

-- The point-probe form, for the DENSE-reach plan shape (0017 §2 measured why both exist: the set form
-- is right at leaf reach and 640-6 400 ms at root, the probe is right at root and 2 500-13 100 ms at
-- leaf, so the adapter dispatches on the capped count rather than picking).
CREATE OR REPLACE FUNCTION oikumenea.authz_unit_readable_with(unit uuid, subject uuid, permission text)
RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
    FROM oikumenea.authz_role_assignments a
    JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
    WHERE a.subject_person_id = subject
      AND a.revoked_at IS NULL
      AND (a.expires_at IS NULL OR a.expires_at > now())
      AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                  WHERE rp.role_id = a.role_id AND rp.permission_code = permission)
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
GRANT EXECUTE ON FUNCTION oikumenea.authz_unit_readable_with(uuid, uuid, text) TO oikumenea_app;

-- The capped cardinality probe the dispatch reads. Counting stops at `cap`: the question is never
-- "how big is the reach" but "is it past the threshold".
CREATE OR REPLACE FUNCTION oikumenea.authz_readable_unit_count_with(subject uuid, permission text, cap integer)
RETURNS bigint
LANGUAGE sql STABLE AS $$
  SELECT count(*) FROM (
    SELECT oikumenea.authz_readable_units_with(subject, permission) LIMIT cap
  ) capped
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_readable_unit_count_with(uuid, text, integer) TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- §2. Indexes for the new list and dashboard paths.
--
-- Shape follows 0017: (<filter column>, id) so a filtered page is an index range scan rather than a
-- filter over a full id scan, PARTIAL on the predicate the queries themselves carry.

-- authz_role_assignments: the existing subject/target indexes (0004) are the two arms the endpoint
-- used to require. The unconditional list keysets on id alone, and the dashboard groups by role_id and
-- graph_id — none of which those two serve.
CREATE INDEX IF NOT EXISTS authz_role_assignments_active_id_idx
  ON oikumenea.authz_role_assignments (id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS authz_role_assignments_role_id_idx
  ON oikumenea.authz_role_assignments (role_id, id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS authz_role_assignments_graph_id_idx
  ON oikumenea.authz_role_assignments (graph_id, id) WHERE revoked_at IS NULL AND graph_id IS NOT NULL;

-- location_locations: country_id already has an index (0007); type_id, the other facet, has none.
CREATE INDEX IF NOT EXISTS location_locations_type_idx
  ON oikumenea.location_locations (type_id) WHERE deleted_at IS NULL AND type_id IS NOT NULL;

-- ---------------------------------------------------------------------------------------------------
-- §3. The pii sweep's findings.
--
-- location_locations: type_id is a facet source and must be classified for the plaintext guard to
-- have anything to check; the six address parts are the columns search_text (already `pii:none`) is
-- generated FROM, so leaving them unclassified said nothing about data the schema had already
-- accounted for twice. `pii:none` throughout, matching D-Location's rule and the module doc: a place
-- is not personal data, and a coordinate becomes locator data only when an owner links a person to it
-- — that tier lives on the owning link, not here.
COMMENT ON COLUMN oikumenea.location_locations.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.admin_area_1 IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.admin_area_2 IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.locality IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.street IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.house_number IS 'pii:none';
COMMENT ON COLUMN oikumenea.location_locations.postal_code IS 'pii:none';

-- location_location_types: a catalog label's lifecycle flag.
COMMENT ON COLUMN oikumenea.location_location_types.status IS 'pii:none';

-- authz_role_assignments needed nothing: subject_person_id and target_unit_id are already `pii:basic`
-- and every other faceted column (role_id, scope, graph_id) is already `pii:none`. Recorded because
-- "the sweep found nothing here" is a result, and the next ticket should not have to re-derive it.

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0023_facet_location_assignment', applied_at = now() WHERE singleton;
