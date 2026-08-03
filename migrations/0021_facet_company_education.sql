-- 0021_facet_company_education — the company and institution dashboards (M58 ticket 5 /
-- D-ObjectFacets, D-ConsoleDashboards).
--
-- Additive, exactly like 0018-0020: no new table and no new column. Both types were already fully
-- modelled — they are SIDECAR PROFILES on tenant_organizations (M41 / D-UnifiedOrgGraph), which is
-- why neither has a RID type token of its own and why the facet catalog had to grow a `Profile` arm
-- to say so. This milestone adds a second READING of columns that already exist.
--
-- 1. pii classification comments, and the sweep came back DIRTY again. Ticket 4's came back clean and
--    recorded that "the obligation stands, only the answer changed"; here the answer changed back.
--    Both sidecars classified their identifying and referencing columns at creation
--    (0007_reference_verticals.sql:715-717, 1821-1825) and left the LIFECYCLE columns alone — the
--    same shape 0018 and 0019 found, and for the same reason: a module classifies what it thinks of
--    as data and forgets what it thinks of as bookkeeping. A facet does not make that distinction,
--    and pkg/facet/plaintext_test.go fails a facet whose column carries no classification, because an
--    omitted comment must FAIL rather than default.
--
--    Both dates and both states are pii:none: a company's founding year and a university's closure
--    are public-register facts about a legal entity, not about a person. Nothing here approaches
--    pii:basic, so every facet leaves ReadPermission empty and the endpoint's own read code is the
--    whole decision (D-ObjectFacets rule 2).
--
-- 2. Indexes for the two new aggregate shapes and the new filter shapes. Both sidecars already index
--    their ref columns (legal_form_id / country_id; kind_id / country_id); `state` and `founded_on`
--    are new on both counts and had no index at all.

-- ---------------------------------------------------------------------------------------------------
-- company_org_profiles.
COMMENT ON COLUMN oikumenea.company_org_profiles.founded_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.dissolved_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.company_org_profiles.state IS 'pii:none';

CREATE INDEX IF NOT EXISTS company_org_profiles_state_idx
  ON oikumenea.company_org_profiles (state) WHERE deleted_at IS NULL;
-- Partial on NOT NULL: rows with no founding date are the (unknown) bucket, which is deliberately not
-- clickable — nothing ever filters on `founded_on IS NULL`, so those rows have no business here.
CREATE INDEX IF NOT EXISTS company_org_profiles_founded_on_idx
  ON oikumenea.company_org_profiles (founded_on) WHERE deleted_at IS NULL AND founded_on IS NOT NULL;

-- company_industry_assignments: the `industryClass` facet is confined to the PRIMARY assignment, so
-- that it partitions honestly (facets.md — an exemption being available is not a reason to take it).
-- The existing one_primary_active index is keyed on company_id, which serves the per-company probe
-- and the uniqueness that MAKES the partition true; the dashboard reads the other way round, grouping
-- primaries BY class, and had no index for it.
CREATE INDEX IF NOT EXISTS company_industry_assignments_primary_class_idx
  ON oikumenea.company_industry_assignments (industry_class_id)
  WHERE is_primary AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------------------------------
-- education_org_profiles.
COMMENT ON COLUMN oikumenea.education_org_profiles.founded_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_org_profiles.closed_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.education_org_profiles.state IS 'pii:none';

CREATE INDEX IF NOT EXISTS education_org_profiles_state_idx
  ON oikumenea.education_org_profiles (state) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS education_org_profiles_founded_on_idx
  ON oikumenea.education_org_profiles (founded_on) WHERE deleted_at IS NULL AND founded_on IS NOT NULL;

-- ---------------------------------------------------------------------------------------------------
-- Nothing is needed for the SHADOW GATE this ticket also closed. Both scoped arms reach
-- tenant_organizations by RID and probe reach through tenant_units.org_id, and
-- tenant_units_org_idx (org_id) WHERE deleted_at IS NULL already matches that predicate exactly —
-- the same finding M58 ticket 4 recorded when organization reach became derived.

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0021_facet_company_education', applied_at = now() WHERE singleton;
