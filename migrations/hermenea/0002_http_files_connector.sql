-- Allow the `http-files` connector type (D-Languages, M18 — live language import). It is a multi-URL
-- HTTP streaming connector that stages the raw Glottolog CLDF + CLDR/ISO-639 inputs to disk for the live
-- Go transform (CLDFMapper / SupplementalMapper). Additive, non-destructive (expand-only): widen the
-- import_sources.connector_type CHECK. The constraint is recreated by its auto-generated name.
ALTER TABLE hermenea.import_sources DROP CONSTRAINT import_sources_connector_type_check;
ALTER TABLE hermenea.import_sources ADD CONSTRAINT import_sources_connector_type_check
  CHECK (connector_type IN ('http', 'file', 'wof-sqlite', 'http-files'));
