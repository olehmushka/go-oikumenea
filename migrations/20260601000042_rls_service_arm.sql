-- Migration 0042_rls_service_arm: the MACHINE reach arm of the RLS backstop (M55 — the deferred half
-- of D-ServiceIdentities / D-ConnectorPlane, split out of M53).
--
-- M51 gave a service principal flat (permission_code, org_id) grants but NO reach: the reach predicate
-- oikumenea.authz_unit_in_reach(unit, wr) computes reach from app.person_id (D-RLSLiveReach), and a
-- machine request sets no person id, so every person-shaped, RLS-guarded surface denies it. This
-- migration adds a PRINCIPAL arm keyed on a new app.principal_id GUC: an org-confined grant authorizes
-- reads/writes on THAT organization's RLS-guarded rows (scraped units, memberships, positions, orders),
-- and only that org's — a cross-org read/write is refused by the DB, not merely the PEP.
--
-- THE RECURSION PROBLEM (why this is its own milestone). The predicate may read ONLY RLS-exempt tables
-- (authz_*, tenant_graphs, tenant_unit_closure) — reading a policy-guarded table from inside a policy
-- recurses into that same policy. So the arm cannot join tenant_units to learn a unit's org, even
-- though tenant_units.org_id exists. Worse, the connector plane must be able to CREATE a brand-new
-- unit, and a just-inserted / edgeless unit has no tenant_unit_closure row yet (reflexive rows are
-- seeded only when an edge is added — internal/tenant ExtendClosureForEdge), so a closure-derived arm
-- could never satisfy the tenant_units INSERT WITH CHECK for a fresh unit.
--
-- RESOLUTION: a dedicated RLS-exempt PROJECTION authz_unit_org(unit_id -> org_id), trigger-maintained
-- from tenant_units. A BEFORE INSERT trigger populates the projection row before the tenant_units
-- WITH CHECK subquery evaluates, so even the unit being inserted in the current statement resolves its
-- org recursion-free. The principal arm is then an O(1) authz_principal_grants x authz_unit_org probe.
--
-- Expand-only (CREATE TABLE / TRIGGER / CREATE OR REPLACE FUNCTION only; no drops/narrowings). The
-- CREATE OR REPLACE keeps the existing GRANT EXECUTE on the function. Depends on 0011 (the backstop)
-- and 0039 (authz_principal_grants). Additive to a LIVE deployment: the person + admin arms are
-- unchanged, so no person hot path regresses; the new arm is inert until app.principal_id is set.

-- ---------------------------------------------------------------------------------------------------
-- authz_unit_org: the RLS-EXEMPT unit -> org projection the machine reach arm reads. A derived
-- structural mirror of tenant_units.org_id (like tenant_unit_closure it is a maintained projection, so
-- it carries NO RID). EXEMPT from RLS on purpose — the reach predicate READS it to COMPUTE reach; a
-- reach-keyed policy here would be circular (the exact recursion the projection exists to avoid). Holds
-- only (unit_id, org_id) — org membership of a unit, pii:none.
-- The unit_id FK is DEFERRABLE INITIALLY DEFERRED: the BEFORE INSERT trigger below populates the
-- projection row from inside the tenant_units INSERT — i.e. BEFORE the parent tenant_units row itself
-- lands — so an immediately-checked FK would fail. Deferring validation to COMMIT (by which point the
-- parent exists) keeps referential integrity + ON DELETE CASCADE while letting the projection row be
-- visible to the same-statement WITH CHECK that a principal-created unit depends on.
CREATE TABLE oikumenea.authz_unit_org (
  unit_id uuid PRIMARY KEY
            REFERENCES oikumenea.tenant_units(id) ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
  org_id  uuid NOT NULL
            REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT
);
CREATE INDEX authz_unit_org_org_idx ON oikumenea.authz_unit_org (org_id);

COMMENT ON TABLE  oikumenea.authz_unit_org         IS 'RLS-exempt unit->org projection for the machine reach arm (M55); trigger-maintained from tenant_units. No RID (derived projection).';
COMMENT ON COLUMN oikumenea.authz_unit_org.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.authz_unit_org.org_id  IS 'pii:none';

-- The app role (oikumenea_app, created NOBYPASSRLS in 0011) writes this projection via the trigger
-- below, which runs as the invoking role. 0011's ALTER DEFAULT PRIVILEGES already covers future
-- tables, but grant explicitly so the DML is correct regardless of who owns the migration.
GRANT SELECT, INSERT, UPDATE, DELETE ON oikumenea.authz_unit_org TO oikumenea_app;

-- Backfill existing units (empty on a fresh install). The migration runs as the owner/superuser, which
-- bypasses RLS, so this read of tenant_units sees every row. Soft-deleted units are included — a
-- projection row is harmless without an active grant, and keeping it avoids a resurrect-on-undelete gap.
INSERT INTO oikumenea.authz_unit_org (unit_id, org_id)
  SELECT id, org_id FROM oikumenea.tenant_units
  ON CONFLICT (unit_id) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- sync_unit_org: keep the projection in lockstep with tenant_units. BEFORE INSERT is load-bearing —
-- the projection row must exist before the tenant_units WITH CHECK subquery (which reads this table)
-- evaluates, so a principal CREATING a unit passes its own INSERT policy. UPDATE OF org_id is handled
-- defensively (a unit's org is immutable by convention, but a correction must not desync the mirror).
CREATE FUNCTION oikumenea.sync_unit_org() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO oikumenea.authz_unit_org (unit_id, org_id)
  VALUES (NEW.id, NEW.org_id)
  ON CONFLICT (unit_id) DO UPDATE SET org_id = EXCLUDED.org_id;
  RETURN NEW;
END$$;

CREATE TRIGGER tenant_units_sync_org
  BEFORE INSERT OR UPDATE OF org_id ON oikumenea.tenant_units
  FOR EACH ROW EXECUTE FUNCTION oikumenea.sync_unit_org();

-- ---------------------------------------------------------------------------------------------------
-- authz_principal_org_in_reach: THE machine reach primitive — does the app.principal_id principal hold
-- an org-CONFINED grant of the requested read/write class over organization `org`? Reads ONLY the
-- RLS-exempt authz_principal_grants:
--   * org_id IS NOT NULL — an INSTANCE-WIDE grant (org_id NULL) confers NO operational reach; it is for
--     reference catalogs (pdp_scoped=false units, already exempt via NOT pdp_scoped) and instance-scope
--     wiring reads. The organization is a scraper's blast-radius boundary (D-ServiceIdentities).
--   * revoked_at IS NULL — grants are read LIVE, so revocation is immediate (matches the M51 no-cache
--     principal-authority design and the person arm's exactness under revocation).
--   * (permission_code LIKE '%.read') = NOT wr — the exact mirror of the person arm's read/write split.
-- For a PERSON request app.principal_id is unset -> nullif('')::uuid IS NULL -> an empty probe, so the
-- person hot path (M47) does not regress. STABLE + single SELECT ⇒ the planner inlines it per policy.
CREATE FUNCTION oikumenea.authz_principal_org_in_reach(org uuid, wr boolean) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT EXISTS (
    SELECT 1
    FROM oikumenea.authz_principal_grants pg
    WHERE pg.principal_id = nullif(current_setting('app.principal_id', true), '')::uuid
      AND pg.org_id = org
      AND pg.org_id IS NOT NULL
      AND pg.revoked_at IS NULL
      AND (pg.permission_code LIKE '%.read') = NOT wr
  )
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_principal_org_in_reach(uuid, boolean) TO oikumenea_app;

-- authz_unit_in_reach: add the PRINCIPAL arm for the UNIT-KEYED tables (membership_*, order_orders,
-- tenant_unit_*, audit_log — everything whose policy passes a unit id, not an org). The person + admin
-- arms are copied verbatim from 0011 (CREATE OR REPLACE cannot patch a subexpression) — DO NOT alter
-- their semantics. The new arm resolves the unit's org through the RLS-exempt authz_unit_org projection
-- and defers the grant test to authz_principal_org_in_reach. Every unit these child tables reference is
-- already committed, so its projection row is visible. (tenant_units' OWN insert is the one case the
-- projection can't serve — a BEFORE-trigger write is not visible to the same statement's WITH CHECK —
-- so the tenant_units policy below checks the row's org_id column directly instead.)
CREATE OR REPLACE FUNCTION oikumenea.authz_unit_in_reach(unit uuid, wr boolean) RETURNS boolean
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
      OR EXISTS (
        SELECT 1 FROM oikumenea.authz_unit_org uo
        WHERE uo.unit_id = unit
          AND oikumenea.authz_principal_org_in_reach(uo.org_id, wr)
      )
$$;

-- tenant_units: extend the reach policy with the org-DIRECT principal arm. A principal's reach on a unit
-- row is its org grant checked against the row's own org_id — this is what lets an org-confined connector
-- CREATE a brand-new unit (the projection row is not yet visible mid-INSERT) and confines every write to
-- its organization. Replaces the 0011 policy with a strict superset (an added OR arm). REFERENCE units
-- (pdp_scoped=false) stay instance-global as before.
DROP POLICY tenant_units_reach ON oikumenea.tenant_units;
CREATE POLICY tenant_units_reach ON oikumenea.tenant_units
  USING (NOT pdp_scoped
      OR oikumenea.authz_unit_in_reach(id, false)
      OR oikumenea.authz_principal_org_in_reach(org_id, false))
  WITH CHECK (NOT pdp_scoped
      OR oikumenea.authz_unit_in_reach(id, true)
      OR oikumenea.authz_principal_org_in_reach(org_id, true));

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0042_rls_service_arm', applied_at = now() WHERE singleton;
