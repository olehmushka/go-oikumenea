-- 0022_tenant_unit_search — server-side text search over units (extends R-21 / D-PersonSearch's
-- trigram generalization to the unit tree).
--
-- Additive and expand-only: one GENERATED column plus one index. No data is rewritten by hand — the
-- column is computed by Postgres for every existing row as part of the ADD COLUMN.
--
-- WHY. The console's unit picker (web/src/components/UnitSelect.tsx, reached through EntitySelect)
-- loaded ONE page and filtered it in the browser, because `listUnits` accepted no text predicate. That
-- is the same defect just fixed for the person directory, where a 304-row directory left 104 people
-- unselectable: past one page the picker silently reports "No matches" for units that plainly exist.
-- A unit tree is exactly the kind of thing that grows past a page in a real deployment (0021 shipped
-- the dashboards that make large trees pleasant to browse; this makes them searchable), so the fix has
-- to be a real indexed predicate rather than a bigger page.
--
-- The haystack is `code || name`, mirroring tenant_organizations (0011_infra §3) so the two levels of
-- the org graph are searched alike. Two differences from that table, both forced by this one's DDL:
--
--   * `code` is NULLABLE here (0002_tenant_rank: NULL = a non-separate sub-unit), so it is coalesced.
--     Without that, `lower(code || ' ' || name)` is NULL for every codeless unit and those rows drop
--     out of the index entirely — the failure would look exactly like the bug being fixed.
--   * the index is PARTIAL on `deleted_at IS NULL`, since units are soft-deleted and searches only
--     ever run over live rows.
--
-- pii: a unit's code and name are organizational identifiers, not personal data — `pii:none`, matching
-- how the same columns are classified on tenant_organizations' own haystack.

ALTER TABLE oikumenea.tenant_units
  ADD COLUMN search_text text GENERATED ALWAYS AS (
    lower(coalesce(code, '') || ' ' || name)) STORED;

CREATE INDEX tenant_units_search_trgm
  ON oikumenea.tenant_units USING gin (search_text gin_trgm_ops) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.tenant_units.search_text IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0022_tenant_unit_search', applied_at = now() WHERE singleton;
