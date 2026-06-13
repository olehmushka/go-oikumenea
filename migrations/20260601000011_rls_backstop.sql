-- Migration 0011_rls_backstop: the PDP-mirror Row-Level-Security backstop (D-RLSDefenseInDepth).
--
-- RLS is enabled here as a DB-level defense-in-depth backstop that mirrors the PDP-computed read/write
-- reach. The app-layer PDP + shadow gate remain AUTHORITATIVE; RLS only guards the forgotten-filter bug
-- class (a SELECT/INSERT that skips the PDP/gate), not PDP-logic errors (it trusts the app-supplied
-- reach in the app.* GUCs). The application sets those GUCs per request on a pinned connection
-- (internal/platform/db AcquireScoped); an instance admin is expressed via the app.is_instance_admin
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
-- Policy predicate shorthand (inlined per table; PostgreSQL has no policy macros):
--   admin  := coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
--   read   := <col> = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[])
--   write  := <col> = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[])
-- current_setting(name, true) returns NULL (not an error) when the GUC was never set on the
-- connection; nullif(..., '') maps the reset/empty-reach value to NULL too, so a non-pinned or
-- empty-reach connection simply sees no rows rather than failing the ::uuid[] cast. uuid unit values
-- contain no commas, so comma-joining is unambiguous.
--
-- EXEMPT (no RLS): tenant_unit_closure + tenant_closure_status (the PDP READS the closure to COMPUTE
-- reach — a reach-keyed policy there would be circular) and tenant_graphs (instance-level catalog).
-- person_persons / document_documents / order_order_items (and person child tables like person_ranks,
-- the HOLDS_RANK link) have no unit column and are scoped by the app-layer PDP through a unit-scoped
-- parent/holder (D-PersonReadScope / D-RLSDefenseInDepth); a reach-join policy for them is a noted
-- hardening seam, not shipped.

-- tenant_units: keyed on the unit's own id.
ALTER TABLE oikumenea.tenant_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_units FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_units_reach ON oikumenea.tenant_units
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- tenant_unit_edges: visible/writable if EITHER endpoint is in reach.
ALTER TABLE oikumenea.tenant_unit_edges ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_edges FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_edges_reach ON oikumenea.tenant_unit_edges
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR parent_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[])
         OR child_id  = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR parent_id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[])
         OR child_id  = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- tenant_unit_lifecycle_events: append-only (reject_mutation guards U/D); keyed on unit_id.
ALTER TABLE oikumenea.tenant_unit_lifecycle_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_lifecycle_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_lifecycle_events_reach ON oikumenea.tenant_unit_lifecycle_events
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- membership_positions: keyed on the owning unit_id.
ALTER TABLE oikumenea.membership_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_positions_reach ON oikumenea.membership_positions
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- membership_memberships: keyed on unit_id (the unit the person belongs to / fills a billet in).
ALTER TABLE oikumenea.membership_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_memberships_reach ON oikumenea.membership_memberships
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- order_orders: keyed on issuing_unit_id (D-Orders — every order is unit-issued).
ALTER TABLE oikumenea.order_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.order_orders FORCE ROW LEVEL SECURITY;
CREATE POLICY order_orders_reach ON oikumenea.order_orders
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR issuing_unit_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]))
  WITH CHECK (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR issuing_unit_id = ANY (string_to_array(nullif(current_setting('app.writable_units', true), ''), ',')::uuid[]));

-- audit_log: a READ backstop only (the dangerous leak is reading another unit's audit history). Writes
-- are append-only (reject_mutation guards U/D) and originate from BOTH request transactions (the
-- pinned conn) AND system paths (first-admin bootstrap, boot seeds) that have no unit reach, so the
-- INSERT policy is permissive — the app, not RLS, governs what is written. NULL unit_id rows (system /
-- instance-plane events) are visible only to an instance admin.
ALTER TABLE oikumenea.audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.audit_log FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_log_read ON oikumenea.audit_log FOR SELECT
  USING (coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
         OR unit_id = ANY (string_to_array(nullif(current_setting('app.readable_units', true), ''), ',')::uuid[]));
CREATE POLICY audit_log_append ON oikumenea.audit_log FOR INSERT
  WITH CHECK (true);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0011_rls', applied_at = now() WHERE singleton;
