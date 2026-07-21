-- Migration 0044_login_event_rid_seed: seed the `login_event` RID type into the DB registry (M37 fix).
--
-- M37's 0043 added `account_login_events` (account Object 9,1,4) and the Go registry entry
-- (pkg/rid/registry.go) + the ontology-mapping.md row, but OMITTED the third registry: the
-- oikumenea.platform_rid_types seed. The three must agree or the boot-time `rid.AssertMatches` drift
-- check fails (db=150 vs go=151). This forward-only fix adds the missing seed row (0043 is an
-- immutable, already-applied historical artifact, so the row lands here rather than by editing it).
-- Idempotent; expand-only.
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (9, 1, 4, 'login_event')
ON CONFLICT DO NOTHING;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0044_login_event_rid_seed', applied_at = now() WHERE singleton;
