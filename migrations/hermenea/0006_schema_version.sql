-- Hermenea boot-time schema-version marker (architecture review R-25 / August 2026).
--
-- oikumenea fail-closes at boot against a stale schema (review-2026-07 R-15): it derives its
-- ExpectedSchemaRevision from the embedded migrations and refuses to serve if the DB's
-- oikumenea.schema_version row does not match. hermenea — the companion whose whole job is long,
-- unattended ingestion runs — had no equivalent: deployed against an un-migrated DB it failed at
-- first job claim with a raw SQL error mid-run instead of at boot with a clear message.
--
-- This migration adds the single-row marker table hermenea derives its gate from, mirroring
-- oikumenea.schema_version (migrations/20260601000000_schema_bootstrap.sql). Each FUTURE hermenea
-- migration bumps it with:
--   UPDATE hermenea.schema_version SET revision = '000N_<name>', applied_at = now() WHERE singleton;
-- The Go side (internal/hermenea/db/schemaversion.go) derives the expected revision from the embedded
-- migration bodies (the same regex-scan oikumenea uses), so adding a migration needs no Go change.

CREATE TABLE hermenea.schema_version (
  singleton  boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  revision   text NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO hermenea.schema_version (singleton, revision) VALUES (true, '0006_schema_version');
