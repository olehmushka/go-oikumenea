# Conventions

The rules every module follows. Module docs reference this file instead of repeating
boilerplate. If a module diverges, it says so explicitly and why.

---

## Schema conventions (PostgreSQL)

Carried over from `drafts/` (proven), adapted to the `oikumenea` schema.

- **One schema: `oikumenea`.** All application tables live there. `public` stays empty.
  The operator owns the database; we connect with operator-supplied credentials.
- **Per-module table prefixes** keep boundaries visible in the schema:
  `oikumenea.tenant_*`, `oikumenea.person_*`, `oikumenea.membership_*`, `oikumenea.rank_*`,
  `oikumenea.authz_*`, `oikumenea.account_*` (identity-federation), `oikumenea.audit_*`.
- **Primary keys:** `id UUID PRIMARY KEY DEFAULT oikumenea.uuid_v7()` everywhere. UUIDv7 is
  time-ordered, so it indexes well as a PK. The `uuid_v7()` function is created by the
  [platform](../modules/platform.md) schema bootstrap.
- **Timestamps:** `TIMESTAMPTZ`, stored in UTC. Never naive `timestamp`.
- **Every mutable table** has `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` and
  `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`, with a `BEFORE UPDATE` trigger calling
  `oikumenea.set_updated_at()`.
- **Soft delete:** user-facing entities carry `deleted_at TIMESTAMPTZ`. "Active" predicates
  and partial unique indexes use `WHERE deleted_at IS NULL`. Hard deletes are not issued
  from application code.
- **Append-only tables** (audit log, lifecycle events) are guarded by a
  `BEFORE UPDATE OR DELETE` trigger calling `oikumenea.reject_mutation()`, which raises.
- **Enums are `TEXT` + `CHECK`**, never native Postgres `ENUM` types — so the set can evolve
  with an `expand/contract` migration rather than an `ALTER TYPE`.
- **Foreign keys are explicit about `ON DELETE`.** Prefer `ON DELETE RESTRICT` for
  references that must not silently vanish; `SET NULL` for provenance pointers (e.g.
  `granted_by`); `CASCADE` only where the child has no independent life.
- **`citext`** for case-insensitive text (e.g. emails on accounts). Extension created
  by the platform bootstrap.
- **JSONB escape hatch** (`attributes`, `metadata`) is for the genuine long tail. A PR adding
  a key must justify why it is not a real column; if more than one caller needs it, it is a
  column. Index a JSONB key only once it is a proven hot lookup.

### Code vs. name — stable identifiers vs. translatable labels

Every structural/catalog entity (unit, role, position, rank category/type/rank, locale) carries
two distinct things (D-Code):

- **`code TEXT NOT NULL UNIQUE`** — a **stable, locale-agnostic machine identifier** that
  external systems reference in their own code. Operator-assigned (or seeded), unique, and
  **immutable by convention** — changing a code breaks external references. Permissions are the
  degenerate case: the permission string *is* the code (no separate row).
- **`name` (+ `title`/`description` where relevant)** — the **human-facing, translatable**
  label. The value stored in the entity's own `name` column is the **default-locale** text
  (the fallback); all other locales live in the [localization](../modules/localization.md)
  translation store.

`person` carries an **optional** `code` (e.g. a personnel/service number); person *names* are
not codes and not admin-translated (see i18n below).

### i18n

i18n is a required feature (D-i18n), owned by [localization](../modules/localization.md):

- **Supported locales are instance-admin-managed data**, seeded with **`ukr` + `eng`** (ISO
  639-3 codes). The instance admin may add more or disable ones (never below one enabled). No
  language is hardcoded-excluded — drafts' "no Russian locale" rule is **dropped**.
- **Translatable fields are returned in every response as a `locale → text` map** (a Conjure
  object), assembled from the entity's default-locale `name` plus its translation rows. **There
  is no Accept-Language negotiation** — the client receives all locales and picks.
- The owning module writes the default-locale value to its own `name` column on create/update;
  additional-locale translations are managed separately by the instance admin via
  `LocalizationService`.
- **Person name transliteration** is the one exception: it is **per-person data** (locale/script
  name variants on the person record), not the admin translation store — see
  [person](../modules/person.md).

### No Row-Level Security for unit isolation

Unlike `drafts/`, go-oikumenea does **not** use Postgres RLS to isolate units. A deployment
serves **one organization**; units are not mutually-distrusting SaaS tenants. Authorization
is enforced by the **application-layer PDP** plus the **shadow-visibility gate** on reads.
RLS remains an available future hardening seam but is not part of the model. See
[decisions.md](decisions.md) D-NoRLS.

---

## Go / witchcraft conventions

- **Layering** per [overview.md](overview.md): `transport → application → domain → adapters`,
  domain owns its interfaces. No framework imports in `domain/`.
- **Logging — `witchcraft-go-logging`.** Use `svc1log` from the request context for service
  logs; never `fmt.Println`/stdlib `log`. Request logging is `req2log` (wired by witchcraft).
  Structured key/value params, not interpolated strings. Every log line carries the
  `request_id` (a.k.a. trace id) automatically via context.
- **Audit — `audit2log`.** The [audit](../modules/audit.md) module emits via `audit2log`
  *and* persists to `oikumenea.audit_log`. Permission-sensitive actions must be audited.
- **Errors — `werror`.** Wrap with `werror.Wrap(err, "msg", werror.SafeParam(...),
  werror.UnsafeParam(...))`. **Safe** params may appear in logs/responses; **unsafe** params
  (PII, secrets) are logged redacted and never returned. Transport maps domain errors to
  Conjure `SerializableError` (see API conventions).
- **Metrics — `pkg/metrics`.** Tagged registry from context. RED discipline on endpoints
  (rate, errors, duration) plus domain counters (e.g. `pdp.decisions{result}`).
- **Tracing — `witchcraft-go-tracing`.** Spans around application-service calls and DB
  round-trips; propagate `X-B3-*`.
- **Health — `witchcraft-go-health`.** Reporters for DB reachability and the
  **schema-version check** (the service is unhealthy/refuses readiness against an
  unknown/newer schema — see [upgrade-safety.md](upgrade-safety.md)).
- **Config — ECV + `pkg/refreshable`.** Static install config in `var/conf/install.yml`
  (DB DSN, IdP issuer/JWKS URLs); runtime-tunable values in `var/conf/runtime.yml` read
  through `refreshable` so they hot-reload. Secrets are ECV-encrypted. Operator supplies the
  DB DSN and IdP config; **no credentials are stored in the DB or in the repo**.
- **DB access — pgx + sqlc.** Queries are authored as `.sql` and compiled by sqlc into typed
  Go in each module's `adapters/`. Repositories accept a `pgx.Tx`/pool so the application
  layer controls transaction boundaries. No ORM.

---

## Conjure conventions

- **One `*.conjure.yml` per module**, namespaced by module
  (`api/tenant.conjure.yml`, `api/authorization.conjure.yml`, …).
- **Naming:** services `TenantService`, `AuthorizationService`, …; objects in `PascalCase`;
  fields `camelCase`; enums `UPPER_SNAKE`. IDs are `rid`/`uuid` aliases.
- **Errors** are declared as Conjure error types (see below) with safe-arg params.
- Generated code lands under `internal/<module>/transport` (server interfaces) and a shared
  `generated/clients` for client structs. **Generated files are never hand-edited.**
- The transport layer implements the generated server interface; the compiler enforces the
  contract.

---

## API conventions

- **Error envelope = Conjure `SerializableError`.** Every error has a stable `errorCode`
  (one of the Conjure categories: `INVALID_ARGUMENT`, `NOT_FOUND`, `PERMISSION_DENIED`,
  `CONFLICT`, `FAILED_PRECONDITION`, `INTERNAL`, …), a service-specific `errorName`
  (e.g. `Authorization:PermissionDenied`, `Tenant:UnitCycleDetected`), a unique
  `errorInstanceId`, and safe `parameters`. Unsafe details stay in logs, keyed by
  `errorInstanceId`.
- **Pagination = token-based.** List endpoints take `pageSize` + opaque `pageToken` and
  return `nextPageToken` (empty when exhausted). No offset pagination. Default page size is a
  runtime tunable.
- **Authentication header → PDP context.** Endpoints take a bearer token
  (`Authorization: Bearer <jwt>`). The federation middleware validates it (OIDC/JWKS) and
  resolves the PDP context *before* the handler runs; handlers receive
  `(person, account, request_id)` from context. Endpoints that do not require a subject
  (health, OpenAPI) are explicitly unauthenticated.
- **Authorization is explicit per endpoint.** Each operation states the atomic permission it
  checks and the unit it checks against (see each module's *Authorization touchpoints*).
  Read endpoints additionally pass through the shadow-visibility gate.
- **Time** is RFC 3339 / `TIMESTAMPTZ` in payloads.
- **Localized fields** are returned as a `locale → text` map (all enabled locales, no
  negotiation); see the i18n convention above and [localization](../modules/localization.md).
  Stable references between systems use the entity **`code`**, never the localized `name`.
- **Idempotency:** create endpoints that can be safely retried accept an optional
  client-supplied idempotency key where it matters (e.g. assignment grants).

---

## Naming quick-reference

| Thing | Convention | Example |
|---|---|---|
| Schema | `oikumenea` | `oikumenea.tenant_units` |
| Table | `<module>_<plural>` | `oikumenea.authz_role_assignments` |
| PK | `id UUID DEFAULT oikumenea.uuid_v7()` | — |
| Stable code | `code TEXT NOT NULL UNIQUE` (locale-agnostic, external ref) | `unit.code = "1-bn"` |
| Localized label | `name` (default-locale fallback) + i18n store | response: `{ukr, eng}` |
| Conjure service | `<Module>Service` | `MembershipService` |
| Atomic permission | `<resource>.<verb>[.<qualifier>]` | `unit.update`, `rank.scheme.manage` |
| Conjure error name | `<Module>:<Error>` | `Tenant:UnitCycleDetected` |
| Module Go package | `internal/<module>` | `internal/authorization` |
