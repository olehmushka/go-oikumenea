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

Every revision through `0010_order` is **expand-only** (new tables/columns/indexes/constraints and
boot/migration seeds; no drops or narrowings) and has **no contract steps**. `0011_rls` enables the
RLS backstop (see its entry). The binary's expected revision is `db.ExpectedSchemaRevision`.

### `0000_schema_bootstrap`
- **Adds (expand-only):** the `oikumenea` schema; `citext` + `pgcrypto` extensions; the shared
  functions `uuid_v7()`, `new_rid()`, `set_updated_at()`, `reject_mutation()`; the single-row
  `schema_version` marker; and the seeded ISO-3166-1 alpha-2 `geo_countries` registry.
- **Contract steps:** none (initial revision).

### `0001_audit_log`
- **Adds (expand-only):** `audit_log` — the append-only Action ledger (`reject_mutation()` guard,
  Action-RID PK, actor-shape CHECK).
- **Contract steps:** none.

### `0002_localization`
- **Adds (expand-only):** `i18n_locales` (seeded `ukr` + `eng`) and the polymorphic
  `i18n_translations` store.
- **Contract steps:** none.

### `0003_tenant`
- **Adds (expand-only):** `tenant_units`, `tenant_graphs`, `tenant_unit_edges`, the derived
  `tenant_unit_closure` + `tenant_closure_status`, and the append-only `tenant_unit_lifecycle_events`.
  The `command`/`operational` graphs are boot-seeded by the app, not the migration (D-RIDSeeding).
- **Contract steps:** none.

### `0004_rank`
- **Adds (expand-only):** `rank_categories` → `rank_types` → `rank_ranks` (the single ordered scheme).
- **Contract steps:** none.

### `0005_person`
- **Adds (expand-only):** `person_persons` (CLDR names, `birthdate`, ISO-5218 `sex`, lifecycle),
  `person_name_variants`, `person_citizenships`, `person_residences`.
- **Contract steps:** none.

### `0006_membership`
- **Adds (expand-only):** `membership_positions` (unit-owned billets) and `membership_memberships`
  (belonging/filling), including a nullable `order_item_id` provenance column **without** its FK (the
  referenced table does not exist yet — the FK lands in `0010_order`).
- **Contract steps:** none.

### `0007_authorization`
- **Adds (expand-only):** `authz_roles` (+ `authz_role_permissions`), `authz_role_assignments`,
  `authz_instance_admins`. Base roles are boot-seeded by the app (D-BaseRoles / D-RIDSeeding).
- **Contract steps:** none.

### `0008_identity_federation`
- **Adds (expand-only):** `account_accounts` and `account_external_identities` (the latter
  immutable-but-unlinkable: `reject_mutation()` on UPDATE only, unlink = hard DELETE).
- **Contract steps:** none.

### `0009_document`
- **Adds (expand-only):** `document_document_types`, `document_documents`, the migration-seeded
  natural-key `document_personal_code_schemes`, and `document_personal_codes` (envelope-encrypted
  value + blind index; ciphertext/DEK nullable for crypto-erase).
- **Contract steps:** none.

### `0010_order`
- **Adds (expand-only):** `order_order_types`, `order_orders`, `order_order_items`, and the
  forward-referenced FK `membership_memberships.order_item_id → order_order_items(id) ON DELETE SET
  NULL` (adds a constraint; not a narrowing).
- **Contract steps:** none.

### `0011_rls`
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

### `0022_tenant_unit_search`
- **Adds (expand-only):** `tenant_units.search_text` — a `GENERATED ALWAYS AS ... STORED` trigram
  haystack over `lower(coalesce(code,'') || ' ' || name)` — plus the partial GIN index
  `tenant_units_search_trgm` (`WHERE deleted_at IS NULL`). Postgres computes the column for existing
  rows as part of the `ADD COLUMN`; there is no backfill step and no data is rewritten by hand.
- **Enables:** the `query` arg on `GET /units` and `GET /stats/units`, so the console's unit picker
  searches server-side instead of filtering one page in the browser. `code` is coalesced because it is
  nullable (NULL = a non-separate sub-unit); without that every codeless unit would have a NULL
  haystack and drop out of the index entirely.
- **Contract steps:** none.

## Configuration changes

Breaking changes to `install.yml` that are **not** schema revisions. The boot-time schema check will
not catch these — the service refuses to start with an explanatory error instead.

### `idp.issuers[]` — an `oidc` issuer must pin an audience (D-MultiIdPExamples)

- **What changed:** an issuer with `type: oidc` and no `audience` (or `audiences`) now makes the
  service **fail to start**. Previously an empty audience skipped the `aud` check entirely.
- **Why it is not optional:** a public IdP's `iss` is shared by every application registered with it —
  `https://accounts.google.com` is the same value for every Google OAuth client, and a Google `sub`
  identifies the account rather than the client. With no `aud` check, an ID token minted for an
  unrelated third-party application carries an issuer/subject the instance accepts and resolves to the
  linked person. The audience is what binds a token to *this* relying party.
- **Operator action (required if you hit it):** set `audience` to the OAuth client id this deployment
  receives tokens for. If several clients of the same deployment use one issuer (a console and a CLI
  registering separately), list them all:
  ```yaml
  idp:
    issuers:
      - issuer: "https://accounts.google.com"
        type: oidc
        audience: "<console-client-id>.apps.googleusercontent.com"
      - issuer: "https://idp.example/realms/x"
        type: oidc
        audiences: ["console-client", "cli-client"]   # token validates if `aud` matches ANY
  ```
  A token validates when its own `aud` **intersects** the configured set. `hs256` issuers are exempt
  (local/dev only, deployment-private key) and need no change.
- **Configuring by environment variable:** `OIKUMENEA_IDP_ISSUERS_<N>_AUDIENCE` sets the scalar, which
  is enough to satisfy the guard. There is deliberately no `…_AUDIENCES` — the env overlay binds only
  scalar fields of a struct-slice element — so the multi-client case requires the YAML file.
- **Also new, additive:** an optional `label` per issuer, a display name surfaced by
  `GET /identity/v1/issuers` for binding UIs. Cosmetic; never an identity or authorization input.
- See [`deploy/oauth/README.md`](deploy/oauth/README.md) for per-provider recipes.
