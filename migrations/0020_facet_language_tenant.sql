-- 0020_facet_language_tenant — the languoid and organization dashboards (M58 ticket 4 /
-- D-ObjectFacets, D-ConsoleDashboards).
--
-- Additive, exactly like 0018 and 0019: no new table and no new column. Both types were already
-- fully modelled; this milestone adds a second READING of columns that already exist.
--
-- 1. NO pii classification comments — and that is the finding, not an omission. Three tickets
--    running found faceted columns with no `COMMENT ON COLUMN ... IS 'pii:<tier>'` (0018 on
--    external_organizations, 0019 on seven columns across three tables), because a module classifies
--    its identifying and sensitive columns carefully and leaves the lifecycle ones alone. This time
--    the sweep came back clean: language_languoids.{level, family_code, macroarea, status} were
--    classified pii:none at creation (0007_reference_verticals.sql:83,86,88,91) and
--    tenant_organizations.{domain_id, visibility, state} at creation (0002_tenant_rank.sql:124-126).
--
--    The sweep still had to be RUN. pkg/facet/plaintext_test.go parses this directory and fails a
--    facet whose column carries no classification — an omitted comment must FAIL rather than
--    default, because a facet over an unclassified column is precisely what the rule exists to
--    catch. What changed here is only the answer, not the obligation.
--
--    Note also what is deliberately NOT faceted: tenant_organizations.search_text is pii:basic and
--    language_languoids.search_text is pii:none, and both are substring haystacks rather than
--    distributions. They serve the `query` search arg; grouping by one would be meaningless.
--
-- 2. Indexes for the two new aggregate shapes and the four new filter shapes. Every one of these
--    columns is now BOTH grouped by a dashboard branch and filtered by a chart segment's
--    click-through, which is the pair that makes an index worth its write cost here.

-- ---------------------------------------------------------------------------------------------------
-- language_languoids. `level` and `family_code` are already indexed (0007_reference_verticals.sql:78-79)
-- and `search_text` has its pg_trgm GIN (0011_infra.sql:85). `status` and `macroarea` are new on both
-- counts and had no index at all.
--
-- 27k rows makes the UNFILTERED aggregate a cheap sequential scan either way; these serve the
-- FILTERED dashboard and the filtered list — which is where a chart segment's click-through lands, and
-- therefore the path the facets exist for.
CREATE INDEX IF NOT EXISTS language_languoids_status_idx
  ON oikumenea.language_languoids (status);
-- Partial on NOT NULL: 142 rows have no macroarea and they are the (unknown) bucket, which is
-- deliberately not clickable — nothing ever filters on `macroarea IS NULL`, so those rows have no
-- business in the index.
CREATE INDEX IF NOT EXISTS language_languoids_macroarea_idx
  ON oikumenea.language_languoids (macroarea) WHERE macroarea IS NOT NULL;

-- ---------------------------------------------------------------------------------------------------
-- tenant_organizations. The domain FK is indexed already (0002_tenant_rank.sql); `visibility` and
-- `state` are new filters and new grouped columns. Partial on the live rows, matching every other
-- tenant index — and matching the `deleted_at IS NULL` both aggregate arms open with.
CREATE INDEX IF NOT EXISTS tenant_organizations_visibility_idx
  ON oikumenea.tenant_organizations (visibility) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS tenant_organizations_state_idx
  ON oikumenea.tenant_organizations (state) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0020_facet_language_tenant', applied_at = now() WHERE singleton;
