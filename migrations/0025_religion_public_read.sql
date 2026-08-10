-- 0025_religion_public_read — closes GH-34: the religion discovery RLS backstop ignored PUBLIC rows.
--
-- GH-33 (0021-era fix, fedc094) made `GET /religion/v1/discovery/sites` dispatch through
-- RequireServiceOrPerson, so a service principal holding instance-wide `religion.read` (org_id IS
-- NULL) now passes the API-layer PEP check. But the RLS backstop on `religion_sites` still zeroed
-- the result set: `authz_principal_org_in_reach` (0011_infra.sql) requires `pg.org_id = org AND
-- pg.org_id IS NOT NULL`, so an instance-wide grant structurally cannot satisfy ANY org's reach. That
-- exclusion is deliberate (D-ServiceIdentities / D-RLSLiveReach, M55, migration 0042: "an
-- INSTANCE-WIDE grant confers no operational reach — blast-radius = the org") and this migration does
-- NOT touch it: order_orders, membership_positions, and every other org-reach-gated table keep the
-- exact same boundary.
--
-- The actual bug is narrower: `religion.SearchSites` (internal/religion/adapters/discovery.go) already
-- hard-codes `s.visibility = 'public'` in its own WHERE clause — discovery, by construction, only ever
-- asks for PUBLIC sites. RLS blocking those rows too contradicts what the app layer already decided,
-- which is exactly backwards for a PDP-MIRROR backstop (D-RLSDefenseInDepth: "RLS mirrors the
-- PDP-computed reach, it does not replace it"). There is already a precedent for this exact shape:
-- `tenant_units_public_read` (0006_person_ext.sql) is a second, SELECT-only permissive policy —
-- `USING (visibility = 'public')` — that Postgres OR-combines with the main reach policy, making
-- `public` units broadly discoverable regardless of caller/grant while writes and non-public rows stay
-- reach-gated. religion_sites already carries the same `visibility` shape (public/unlisted/private).
--
-- So: three new FOR SELECT-only permissive policies, one per table the discovery query touches.
-- religion_service_schedules and religion_aliases have no visibility column of their own — they
-- inherit their site's, via an EXISTS on religion_sites — because SearchSites' day/language/online
-- filters (EXISTS on schedules) and query= fuzzy match (EXISTS on aliases) would otherwise silently
-- zero out for an instance-wide-only principal even though the site itself is public. Writes are
-- untouched: the existing *_reach policies (FOR ALL, reach-gated WITH CHECK) still govern
-- INSERT/UPDATE/DELETE exactly as before; a FOR SELECT policy grants no write.
--
-- Expand-only (L-UpgradeSafe / D-Migrations): three new policies, nothing dropped or rewritten.
-- atlas migrate lint: additive-only by construction, no destructive change for the gate to find.

CREATE POLICY religion_sites_public_read ON oikumenea.religion_sites
  FOR SELECT
  USING (visibility = 'public');

CREATE POLICY religion_service_schedules_public_read ON oikumenea.religion_service_schedules
  FOR SELECT
  USING (EXISTS (SELECT 1 FROM oikumenea.religion_sites s
                 WHERE s.id = site_id AND s.visibility = 'public' AND s.deleted_at IS NULL));

CREATE POLICY religion_aliases_public_read ON oikumenea.religion_aliases
  FOR SELECT
  USING (EXISTS (SELECT 1 FROM oikumenea.religion_sites s
                 WHERE s.org_unit_id = unit_id AND s.visibility = 'public' AND s.deleted_at IS NULL));

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0025_religion_public_read', applied_at = now() WHERE singleton;
