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

Every revision through `0011_order` is **expand-only** (new tables/columns/indexes/constraints and
boot/migration seeds; no drops or narrowings) and has **no contract steps**. `0012_rls` enables the
RLS backstop (see its entry). The binary's expected revision is `db.ExpectedSchemaRevision`.

### `0001_schema_bootstrap`
- **Adds (expand-only):** the `oikumenea` schema; `citext` + `pgcrypto` extensions; the shared
  functions `uuid_v7()`, `new_rid()`, `set_updated_at()`, `reject_mutation()`; the single-row
  `schema_version` marker; and the seeded ISO-3166-1 alpha-2 `geo_countries` registry.
- **Contract steps:** none (initial revision).

### `0002_audit_log`
- **Adds (expand-only):** `audit_log` — the append-only Action ledger (`reject_mutation()` guard,
  Action-RID PK, actor-shape CHECK).
- **Contract steps:** none.

### `0003_localization`
- **Adds (expand-only):** `i18n_locales` (seeded `ukr` + `eng`) and the polymorphic
  `i18n_translations` store.
- **Contract steps:** none.

### `0004_tenant`
- **Adds (expand-only):** `tenant_units`, `tenant_graphs`, `tenant_unit_edges`, the derived
  `tenant_unit_closure` + `tenant_closure_status`, and the append-only `tenant_unit_lifecycle_events`.
  The `command`/`operational` graphs are boot-seeded by the app, not the migration (D-RIDSeeding).
- **Contract steps:** none.

### `0005_rank`
- **Adds (expand-only):** `rank_categories` → `rank_types` → `rank_ranks` (the single ordered scheme).
- **Contract steps:** none.

### `0006_person`
- **Adds (expand-only):** `person_persons` (CLDR names, `birthdate`, ISO-5218 `sex`, lifecycle),
  `person_name_variants`, `person_citizenships`, `person_residences`.
- **Contract steps:** none.

### `0007_membership`
- **Adds (expand-only):** `membership_positions` (unit-owned billets) and `membership_memberships`
  (belonging/filling), including a nullable `order_item_id` provenance column **without** its FK (the
  referenced table does not exist yet — the FK lands in `0011_order`).
- **Contract steps:** none.

### `0008_authorization`
- **Adds (expand-only):** `authz_roles` (+ `authz_role_permissions`), `authz_role_assignments`,
  `authz_instance_admins`. Base roles are boot-seeded by the app (D-BaseRoles / D-RIDSeeding).
- **Contract steps:** none.

### `0009_identity_federation`
- **Adds (expand-only):** `account_accounts` and `account_external_identities` (the latter
  immutable-but-unlinkable: `reject_mutation()` on UPDATE only, unlink = hard DELETE).
- **Contract steps:** none.

### `0010_document`
- **Adds (expand-only):** `document_document_types`, `document_documents`, the migration-seeded
  natural-key `document_personal_code_schemes`, and `document_personal_codes` (envelope-encrypted
  value + blind index; ciphertext/DEK nullable for crypto-erase).
- **Contract steps:** none.

### `0011_order`
- **Adds (expand-only):** `order_order_types`, `order_orders`, `order_order_items`, and the
  forward-referenced FK `membership_memberships.order_item_id → order_order_items(id) ON DELETE SET
  NULL` (adds a constraint; not a narrowing).
- **Contract steps:** none.

### `0012_rls`
- **Adds:** the Row-Level-Security backstop (D-RLSDefenseInDepth) — the non-superuser group role
  `oikumenea_app` (+ schema/table/function GRANTs), and `ENABLE`+`FORCE ROW LEVEL SECURITY` with
  reach-keyed policies on the unit-scoped tables (`tenant_units`, `tenant_unit_edges`,
  `tenant_unit_lifecycle_events`, `membership_positions`, `membership_memberships`, `order_orders`,
  and a read-only policy on `audit_log`). The policies read the per-request `app.readable_units` /
  `app.writable_units` / `app.is_instance_admin` GUCs the application sets on a pinned connection.
- **Operator actions (required):**
  - Create (or repoint) the **application login role** as a member of `oikumenea_app`, and run the
    service as it — e.g. `CREATE ROLE oikumenea LOGIN PASSWORD '…' IN ROLE oikumenea_app;` — then set
    `postgres.dsn` to it. The application DB role **must not** hold `BYPASSRLS` or be a superuser, or
    the backstop is silently bypassed.
  - Keep running **migrations** as the owner/superuser (migrations create the role + policies and must
    not be subject to them).
- **Contract step note:** enabling RLS is normally staged permissive→tighten (`upgrade-safety.md`).
  For this first release the GUC wiring ships in the same revision as the tightened policies, so there
  is no permissive interim; **post-v1 RLS changes follow the staged rollout.**
