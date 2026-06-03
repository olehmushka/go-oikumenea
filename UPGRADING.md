# Upgrading go-oikumenea

Operator guidance for safe, non-destructive upgrades (L-UpgradeSafe / D-Migrations; see
`docs/architecture/upgrade-safety.md`).

- Always `pg_dump` before upgrading — the operator owns the database; this is cheap insurance.
- Apply migrations as a pre-start step (`atlas migrate apply --env <env>`). The boot-time
  schema-version check is the backstop, not the plan: the service **refuses readiness** against a
  schema revision it does not recognize.
- Roll back the binary freely *within* a schema revision range; do **not** run an older binary
  against a newer schema (the boot check will refuse it).
- Each entry below lists the schema revision, what it adds, and any contract (removal) step.

## Revisions

### `0001_schema_bootstrap`
- **Adds (expand-only):** the `oikumenea` schema; `citext` + `pgcrypto` extensions; the shared
  functions `uuid_v7()`, `new_rid()`, `set_updated_at()`, `reject_mutation()`; the single-row
  `schema_version` marker; and the seeded ISO-3166-1 alpha-2 `geo_countries` registry.
- **Contract steps:** none (initial revision).
- The binary expects this revision via `db.ExpectedSchemaRevision`.
