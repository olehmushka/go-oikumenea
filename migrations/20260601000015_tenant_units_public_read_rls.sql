-- Migration 0015_tenant_units_public_read_rls: the shadow-visibility gate, unit scope (F-002, A-lite).
--
-- The shadow-visibility differentiator (L-Visibility, patterns.md) was inert: every authenticated
-- request pins a connection whose app.readable_units GUC holds the subject's grant-based reach, and the
-- migration-0012 tenant_units_reach policy admits a row only when it is in reach (or the subject is an
-- instance admin). A `public` unit outside reach was therefore dropped exactly like a `shadow` one, so
-- public and shadow behaved identically and the documented "public units are discoverable" rule was not
-- enforced anywhere.
--
-- This migration makes `visibility` mean something for the unit graph: a `public` unit is SELECT-able
-- regardless of reach (broadly discoverable), while a `shadow` unit still requires reach. It is wired
-- as a SECOND permissive FOR SELECT policy: PostgreSQL OR-combines permissive policies per command, so
-- a SELECT on tenant_units now passes if the row is in reach OR the subject is an instance admin
-- (tenant_units_reach) OR the row is public (this policy). Writes are untouched — tenant_units_reach
-- (FOR ALL) still governs INSERT/UPDATE/DELETE through its WITH CHECK / USING on app.writable_units, and
-- a FOR SELECT policy grants no write. The app-layer shadow gate (authorization.FilterVisibleUnits,
-- wired into the tenant list/ancestors/descendants reads) remains the AUTHORITATIVE pass; this policy is
-- its DB-level mirror (D-RLSDefenseInDepth).
--
-- A-lite boundary (decisions.md, L-Visibility note): broad public discovery is a UNIT-read affordance
-- only. person/document/membership/order reads stay reach-gated — a public unit is discoverable in unit
-- listings, but its roster/detail still needs reach. Expand-only; no data change; no destructive op.

CREATE POLICY tenant_units_public_read ON oikumenea.tenant_units
  FOR SELECT
  USING (visibility = 'public');

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0015_tenant_units_public_read_rls', applied_at = now() WHERE singleton;
