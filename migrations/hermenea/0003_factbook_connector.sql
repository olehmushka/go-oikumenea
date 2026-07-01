-- 0003 factbook connector — allow the `factbook` connector-type on import_sources (D-PhysicalIdentity
-- amendment, M43). The `factbook` StreamingConnector enumerates + stages the CIA World Factbook country
-- files for the runtime ethnicity import (internal/hermenea/connector/factbook.go). Expand-only.
ALTER TABLE hermenea.import_sources DROP CONSTRAINT import_sources_connector_type_check;
ALTER TABLE hermenea.import_sources ADD CONSTRAINT import_sources_connector_type_check
  CHECK (connector_type IN ('http', 'file', 'wof-sqlite', 'http-files', 'factbook'));
