-- Migration 0011_rls_backstop: the live-reach Row-Level-Security backstop (D-RLSDefenseInDepth,
-- reshaped by D-RLSLiveReach — review-2026-07 R-02.2).
--
-- RLS is enabled here as a DB-level defense-in-depth backstop. The app-layer PDP + shadow gate
-- remain AUTHORITATIVE; RLS only guards the forgotten-filter bug class (a SELECT/INSERT that skips
-- the PDP/gate). Policies compute reach LIVE via oikumenea.authz_unit_in_reach(unit, wr) — a
-- semi-join over the subject's role assignments + the tenant closure — keyed on two O(1) GUCs the
-- application sets per request (internal/platform/db): app.person_id + app.is_instance_admin. The
-- former comma-joined app.readable_units / app.writable_units unit-list GUCs are GONE (they scaled
-- with org size per request — multi-MB for a staff-level subject). Because the policies read the
-- authority tables directly, the backstop is EXACT under revocation (stronger than the old
-- snapshot-at-request-start GUCs). An instance admin is expressed via the app.is_instance_admin
-- GUC flag, NEVER a DB superuser — the app role created here lacks BYPASSRLS.
--
-- upgrade-safety.md stages RLS as permissive-then-tighten for a LIVE, already-released deployment (so a
-- policy tightening cannot outrun the GUC plumbing). go-oikumenea has never been released, so this
-- migration ships the GUC wiring (in the same release) and the tightened policies ATOMICALLY: on a
-- fresh install there is no window in which the policy outruns the plumbing. The staged rollout
-- re-applies for any post-v1 RLS change. Expand-only (CREATE ROLE / GRANT / ENABLE RLS / CREATE
-- POLICY only; no drops/narrowings). Depends on 0001–0011.

-- ---------------------------------------------------------------------------------------------------
-- The non-superuser application role. Migrations run as the owner/superuser (which bypasses RLS); the
-- application connects as a login role that is a MEMBER of this group role (see UPGRADING.md / .env),
-- so the policies below apply to it. NOLOGIN + NOBYPASSRLS: a group role, never a login/superuser.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'oikumenea_app') THEN
    CREATE ROLE oikumenea_app NOLOGIN NOBYPASSRLS;
  END IF;
END$$;

GRANT USAGE ON SCHEMA oikumenea TO oikumenea_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA oikumenea TO oikumenea_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA oikumenea TO oikumenea_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA oikumenea TO oikumenea_app;
-- Future objects (forward-compatible; this is the last v1 migration but keeps the grant correct if a
-- later migration adds a table without re-granting).
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO oikumenea_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT USAGE, SELECT ON SEQUENCES TO oikumenea_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT EXECUTE ON FUNCTIONS TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- authz_unit_in_reach: the live reach predicate every policy calls (D-RLSLiveReach). PARITY CONTRACT
-- with authorization/domain ReachSet (pdp.go): an assignment contributes iff active (revoked_at IS
-- NULL, unexpired), its role live, and the role carries a '*.read' permission (wr=false) / any
-- non-read permission (wr=true — the exact mirror of the Go classify()); 'unit' scope reaches its
-- target only; 'subtree' reaches target + closure descendants ONLY over an authority-bearing,
-- non-deleted graph (a directory-only subtree grant confers nothing, not even its target).
--
-- LANGUAGE sql + STABLE + a single SELECT ⇒ the planner inlines it into each policy as a
-- parameterized sub-plan: an authz_role_assignments_subject_idx probe (k = the subject's
-- assignments, typically ≤ tens) + one closure-PK probe per subtree assignment — exact index hits
-- per row scanned, and every RLS-guarded read path is keyset-paged, so rows scanned ≈ page.
--
-- The function reads ONLY RLS-exempt tables (authz_*, tenant_graphs, tenant_unit_closure) — a read
-- of a policy-guarded table here would recurse into its own policy. Consequence: it cannot apply
-- ReachSet's soft-deleted-DESCENDANT refinement (that needs tenant_units), so the backstop is
-- deliberately NEVER NARROWER than PDP reach — it may additionally pass rows keyed on a soft-deleted
-- descendant unit; the authoritative app layer excludes them. current_setting(name, true) returns
-- NULL when the GUC was never set; nullif('') maps the reset value to NULL, so a non-pinned
-- connection simply sees no rows.
CREATE FUNCTION oikumenea.authz_unit_in_reach(unit uuid, wr boolean) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
      OR EXISTS (
        SELECT 1
        FROM oikumenea.authz_role_assignments a
        JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
        WHERE a.subject_person_id = nullif(current_setting('app.person_id', true), '')::uuid
          AND a.revoked_at IS NULL
          AND (a.expires_at IS NULL OR a.expires_at > now())
          AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                      WHERE rp.role_id = a.role_id
                        AND (rp.permission_code LIKE '%.read') = NOT wr)
          AND ((a.scope = 'unit' AND a.target_unit_id = unit)
            OR (a.scope = 'subtree'
                AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                            WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
                AND (a.target_unit_id = unit
                  OR EXISTS (SELECT 1 FROM oikumenea.tenant_unit_closure c
                             WHERE c.graph_id = a.graph_id
                               AND c.ancestor_id = a.target_unit_id
                               AND c.descendant_id = unit))))
      )
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_unit_in_reach(uuid, boolean) TO oikumenea_app;

-- Policy predicate shorthand (inlined per table; PostgreSQL has no policy macros):
--   read   := oikumenea.authz_unit_in_reach(<col>, false)   -- admin short-circuit is inside
--   write  := oikumenea.authz_unit_in_reach(<col>, true)
--
-- EXEMPT (no RLS): tenant_unit_closure + tenant_closure_status + tenant_graphs + the authz_* tables
-- incl. authz_epoch (the reach predicate READS these to COMPUTE reach — a reach-keyed policy there
-- would be circular; tenant_graphs is an instance-level catalog anyway).
-- person_persons / document_documents / order_order_items (and person child tables like person_ranks,
-- the HOLDS_RANK link) have no unit column and are scoped by the app-layer PDP through a unit-scoped
-- parent/holder (D-PersonReadScope / D-RLSDefenseInDepth); since R-02.1 the reach predicate exists in
-- SQL, so a membership-semi-join policy for them is one policy away — a noted hardening seam, still
-- not shipped (D-RLSLiveReach).

-- tenant_units: keyed on the unit's own id. REFERENCE units (pdp_scoped=false — university/company,
-- D-UnifiedOrgGraph M41) are instance-global: they are exempt from the reach predicate (reads are gated
-- by the public-read policy + the app-layer permission; writes by the instance-scoped `*.manage` perm).
ALTER TABLE oikumenea.tenant_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_units FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_units_reach ON oikumenea.tenant_units
  USING (NOT pdp_scoped OR oikumenea.authz_unit_in_reach(id, false))
  WITH CHECK (NOT pdp_scoped OR oikumenea.authz_unit_in_reach(id, true));

-- tenant_unit_edges: visible/writable if EITHER endpoint is in reach.
ALTER TABLE oikumenea.tenant_unit_edges ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_edges FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_edges_reach ON oikumenea.tenant_unit_edges
  USING (oikumenea.authz_unit_in_reach(parent_id, false) OR oikumenea.authz_unit_in_reach(child_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(parent_id, true) OR oikumenea.authz_unit_in_reach(child_id, true));

-- tenant_unit_lifecycle_events: append-only (reject_mutation guards U/D); keyed on unit_id.
ALTER TABLE oikumenea.tenant_unit_lifecycle_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_lifecycle_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_lifecycle_events_reach ON oikumenea.tenant_unit_lifecycle_events
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- membership_positions: keyed on the owning unit_id.
ALTER TABLE oikumenea.membership_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_positions_reach ON oikumenea.membership_positions
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- membership_memberships: keyed on unit_id (the unit the person belongs to / fills a billet in).
ALTER TABLE oikumenea.membership_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_memberships_reach ON oikumenea.membership_memberships
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- order_orders: keyed on issuing_unit_id (D-Orders — every order is unit-issued).
ALTER TABLE oikumenea.order_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.order_orders FORCE ROW LEVEL SECURITY;
CREATE POLICY order_orders_reach ON oikumenea.order_orders
  USING (oikumenea.authz_unit_in_reach(issuing_unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(issuing_unit_id, true));

-- audit_log: a READ backstop only (the dangerous leak is reading another unit's audit history). Writes
-- are append-only (reject_mutation guards U/D) and originate from BOTH request transactions (the
-- pinned conn) AND system paths (first-admin bootstrap, boot seeds) that have no unit reach, so the
-- INSERT policy is permissive — the app, not RLS, governs what is written. NULL unit_id rows (system /
-- instance-plane events) are visible only to an instance admin.
ALTER TABLE oikumenea.audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.audit_log FORCE ROW LEVEL SECURITY;
-- NULL unit_id rows (system / instance-plane events) stay admin-only: authz_unit_in_reach(NULL, ·)
-- is the admin flag OR a never-matching EXISTS.
CREATE POLICY audit_log_read ON oikumenea.audit_log FOR SELECT
  USING (oikumenea.authz_unit_in_reach(unit_id, false));
CREATE POLICY audit_log_append ON oikumenea.audit_log FOR INSERT
  WITH CHECK (true);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0011_rls', applied_at = now() WHERE singleton;
