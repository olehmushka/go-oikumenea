-- Migration 0017_import_provenance: per-row import lineage on reference catalogs (M16 / D-Hermenea).
--
-- The hermenea companion loads reference data through oikumenea's generic POST /import/{objectType}
-- endpoint (docs/modules/hermenea.md, docs/modules/platform.md). Each imported row carries WHERE it
-- came from: `source` (the dataset id, e.g. iso-3166), `source_version` (its edition), and
-- `imported_at` (when the upsert ran). These are stamped from the canonical envelope on every upsert
-- and are the per-row half of the D-DataIngestion lineage (the run-level ledger lives in hermenea's
-- own DB).
--
-- M16's first importable catalog is `geo_countries` (ISO-3166). The columns are nullable: rows seeded
-- by the bootstrap migration (0000) predate any import and simply have NULL provenance until a sync
-- refreshes them. Future importable catalogs add the same three columns in their own migrations.
--
-- Expand-only: three additive nullable columns, no data change, no destructive op, no backfill.

ALTER TABLE oikumenea.geo_countries ADD COLUMN source         text;        -- importing dataset id (e.g. iso-3166)
ALTER TABLE oikumenea.geo_countries ADD COLUMN source_version text;        -- the source edition/version
ALTER TABLE oikumenea.geo_countries ADD COLUMN imported_at    timestamptz; -- when the import upsert last touched this row
COMMENT ON COLUMN oikumenea.geo_countries.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.source_version IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.imported_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0017_import_provenance', applied_at = now() WHERE singleton;
