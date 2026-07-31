-- 0018_facet_dashboards — the external-organization and religion-taxon dashboards (M58 ticket 2 /
-- D-ObjectFacets, D-ConsoleDashboards).
--
-- Two things the new facets need that the schema does not yet have. Both are additive; there is no
-- new table and no new column, because both types were already fully modelled — this milestone adds
-- a second READING of columns that already exist.
--
-- 1. pii classification comments on four external_organizations columns. D-ObjectFacets rule 2 is
--    enforced by a build-time guard (pkg/facet/plaintext_test.go) that parses the migration DDL and
--    refuses a facet whose column carries no `COMMENT ON COLUMN ... IS 'pii:<tier>'`: a facet over an
--    unclassified column is exactly the case the rule exists to catch, so an omitted comment must
--    fail rather than default. M30 commented the identifying columns (id/kind_id/name/code/country_id/
--    wikidata_id) and left the status + attribution set unclassified, which was invisible until
--    something faceted them. All four are pii:none — an external organization is an institution, not
--    a person, and these describe the ASSERTION's provenance rather than any data subject.
--
-- 2. Indexes for the two new aggregate shapes. The taxonomy is small (hundreds of rows) but the
--    closure is not — it is quadratic in tree depth, and the subtree facet groups over it.

-- ---------------------------------------------------------------------------------------------------
-- 1. Classify the external_organizations facet columns (D-OverlayFoundation attribution column-set).
COMMENT ON COLUMN oikumenea.external_organizations.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.confidence IS 'pii:none';
COMMENT ON COLUMN oikumenea.external_organizations.as_of IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- 2. Aggregate support.

-- external_organizations: the list had one index per identifying column; the dashboard groups by the
-- attribution set and bounds by as_of.
CREATE INDEX IF NOT EXISTS external_organizations_kind_idx
  ON oikumenea.external_organizations (kind_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS external_organizations_country_idx
  ON oikumenea.external_organizations (country_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS external_organizations_as_of_idx
  ON oikumenea.external_organizations (as_of) WHERE deleted_at IS NULL AND as_of IS NOT NULL;

-- religion_taxa_closure: the subtree facet groups by ancestor_id over the PROPER descendants
-- (depth > 0), and the same predicate is the `parent` filter's click-through. The existing indexes
-- are the composite PK (ancestor_id, descendant_id) and a descendant_id index; neither excludes the
-- reflexive depth-0 row, which is half of a shallow tree's rows.
CREATE INDEX IF NOT EXISTS religion_taxa_closure_proper_idx
  ON oikumenea.religion_taxa_closure (ancestor_id, descendant_id) WHERE depth > 0;

-- religion_taxon_classifications: the classification facet resolves the NEAREST DECLARING ancestor,
-- so it probes this table from the taxon side once per closure row rather than scanning it.
CREATE INDEX IF NOT EXISTS religion_taxon_classifications_taxon_idx
  ON oikumenea.religion_taxon_classifications (taxon_id, classification_id);

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0018_facet_dashboards', applied_at = now() WHERE singleton;
