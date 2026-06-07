# Conventions

The rules every module follows. Module docs reference this file instead of repeating
boilerplate. If a module diverges, it says so explicitly and why.

---

## Schema conventions (PostgreSQL)

Carried over from `drafts/` (proven), adapted to the `oikumenea` schema.

- **One schema: `oikumenea`.** All application tables live there. `public` stays empty.
  The operator owns the database; we connect with operator-supplied credentials.
- **Per-module table prefixes** keep boundaries visible in the schema:
  `oikumenea.tenant_*`, `oikumenea.person_*`, `oikumenea.membership_*`, `oikumenea.document_*`,
  `oikumenea.order_*`, `oikumenea.rank_*`, `oikumenea.authz_*`, `oikumenea.account_*`
  (identity-federation), `oikumenea.audit_*`.
- **Primary keys:** composed **URN resource identifiers** (RIDs) — `id TEXT PRIMARY KEY DEFAULT
  oikumenea.new_rid('<service>','<entity_type>')` everywhere (D-ResourceIdentifiers). See the
  *Resource identifiers* subsection below for the grammar. `uuid_v7()` is **retained** as the RID's
  time-ordered crypto component (so the PK still appends in insert order); both `uuid_v7()` and
  `new_rid()` are created by the [platform](../modules/platform.md) schema bootstrap. Foreign keys
  follow the PK type (`TEXT`).
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

### Resource identifiers (RIDs)

Every Object, Link, and Action is keyed by a **composed, self-describing URN** (D-ResourceIdentifiers):

```
urn:oikumenea:<service>:<environment>:<entity_type>:<uuid>
```

- `<service>` — the owning module (the table-prefix name: `tenant`, `person`, `membership`,
  `document`, `order`, `rank`, `authz`, `account`, `i18n`, `audit`, `platform`).
- `<environment>` — `prod`|`staging`|`dev`|`local`, from install config via
  `current_setting('app.environment')`; constant per database (L-SingleDomain).
- `<entity_type>` — for an **Object**, its type (`unit`, `person`, `role-assignment`, …); for a
  **Link**, `link__<link_type>` (e.g. `link__has_role`, `link__parent_of`); for an **Action**,
  `action__<action_type>` (e.g. `action__issue_order`).
- `<uuid>` — a `uuid_v7()` (time-ordered), lowercase.

```sql
-- generator (platform bootstrap)
CREATE FUNCTION oikumenea.new_rid(service text, entity_type text) RETURNS text
  LANGUAGE sql VOLATILE AS $$
    SELECT 'urn:oikumenea:' || service || ':' || current_setting('app.environment')
        || ':' || entity_type || ':' || oikumenea.uuid_v7()::text $$;

-- per-table usage + cheap shape guard
id TEXT PRIMARY KEY DEFAULT oikumenea.new_rid('tenant','unit'),
CONSTRAINT unit_rid_shape CHECK (id LIKE 'urn:oikumenea:tenant:%:unit:%')
```

- **B-tree locality is preserved:** within a `(service, env, entity_type)` prefix only the trailing
  `uuid_v7()` varies, so inserts still append in time order.
- **Temporal Links** never encode validity in the RID (RIDs are immutable). A time-bounded Link
  carries `valid_from`/`valid_to` (NULL `valid_to` = active); existing temporal columns
  (`effective_from`/`effective_to`, `granted_at`/`revoked_at`+`expires_at`) **are** that pair.
- **Action RID = the `audit_log` row key** (the audit log is the action ledger; D-Audit).
- **RID vs `code`:** the RID is the *machine resource handle*; the entity **`code`** stays the stable,
  locale-agnostic *business* key (D-Code). Both coexist.
- **Reserved seam:** `urn:oikumenea:<service>:<env>:object-set:<uuid>` for named object collections.

### Ontology modeling (Object / Link / Action)

Every persisted entity is classified as exactly one ontology kind (D-Ontology); each module doc
states the kind in its *entities* section and keys the table with the matching RID slot:

- **Object** — a thing with identity over time. `<entity_type>` = the Object type (`unit`, `person`).
- **Link** — a relationship reified as its own row **when it carries identity, attributes, or
  history** (an assignment's scope/provenance, a membership's effective dates, an edge's graph).
  `<entity_type>` = `link__<link_type>`. A relationship with none of those stays a **plain FK
  column**, not a Link table — do not reify bare references.
- **Action** — a named, audited mutation. `<entity_type>` = `action__<action_type>`; the
  [audit](../modules/audit.md) row is keyed by the Action RID (the audit log is the action ledger).

The authoritative catalog of which Objects/Links/Actions exist is
[ontology-mapping.md](../ontology-mapping.md) (D-Ontology); this file owns the schema mechanics (RID
grammar above, temporal-Link columns, code-vs-name), and the module docs own per-entity detail.

### PII classification (`COMMENT ON COLUMN`)

Every PII-bearing column is classified with a machine-parseable comment (D-PIITiers), so tooling
(and an `atlas migrate lint`-style check) can assert that new PII columns are tiered:

```sql
COMMENT ON COLUMN oikumenea.person_persons.display_name IS 'pii:basic';
```

Fixed 5-tier vocabulary (`pii:sensitive` added by D-CryptoProvider):

| Tier | Meaning | Examples |
|---|---|---|
| `pii:none` | not personal data | codes (non-personal), FKs, enums, timestamps, `geo_countries` |
| `pii:basic` | identifying personal data | `display_name`, CLDR name parts (`given`, `given2`, `surname`, …), `birthdate`, `sex`, `country_of_birth`, citizenship, personnel `code`, IdP `subject`, document `number`/`issuer` |
| `pii:contact` | contact / locator data | `email`, phone, address, residence |
| `pii:sensitive` | **national-identifier-class** government codes | tax number, national ID, social-/health-insurance number (`document_personal_codes.value`) |
| `pii:special` | **GDPR Art. 9** special-category | religion, health, biometrics, ethnicity |

- **`pii:sensitive` ⇒ envelope-encrypt at rest.** The tier is the machine-parseable marker that a
  column must be stored via the envelope-encryption seam (D-CryptoProvider): ciphertext + wrapped
  DEK + `key_ref` + a keyed `value_blind_index` for lookup; the KEK lives in an external KMS. It
  sits between `pii:contact` and `pii:special` in handling strictness and is kept distinct from
  `pii:special` (different legal regime; the envelope-at-rest obligation attaches here specifically).
- **JSONB grab-bags** (`person.attributes`, `audit.before`/`after`) are tagged at their **ceiling**
  (`pii:special`) with a note: special-category data must **not** be placed there without the
  envelope-encryption seam ([open-questions](../open-questions.md) DS-29 for audit; the
  `pii:sensitive` envelope under D-CryptoProvider ships, the `pii:special` extension stays parked).
- **Secrets** (the dormant `account.password_hash`) are marked `secret` — a separate axis, **not**
  a `pii:` tier.
- This static classification is the companion to the two runtime PII controls: `werror.UnsafeParam`
  log redaction (below) and the [person](../modules/person.md) **purge** (erasure). Applied
  instance-wide — see [person](../modules/person.md),
  [identity-federation](../modules/identity-federation.md), [document](../modules/document.md),
  [order](../modules/order.md), [audit](../modules/audit.md). The [document](../modules/document.md)
  module hooks the person purge via the `PersonPurged` event to erase document PII.

### Country registry & personal-code schemes

Two reference tables follow the registry pattern (D-Geo / D-PersonalCodes):

- **`geo_countries`** — a seeded ISO-3166-1 alpha-2 country registry (`code CHAR(2)` PK,
  translatable `name`, `status`, `sort_order`), owned/seeded by [platform](../modules/platform.md)
  as a shared reference table (like `uuid_v7()`), FK'd from every column that names a country
  (`person.country_of_birth`, citizenships, residences, personal-code scheme `country_iso`).
- **`document_personal_code_schemes`** — the country-namespaced national-identifier catalog
  (`code` PK like `ua-rnokpp`, `country_iso` FK, `generic_category` for cross-scheme queries,
  optional `validation_regex`, translatable `name`). Distinct from the generic
  `document_document_types` (papers); see [document](../modules/document.md).

### Code vs. name — stable identifiers vs. translatable labels

Every structural/catalog entity (unit, role, position, rank category/type/rank, locale, country,
personal-code scheme) carries two distinct things (D-Code):

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

### Row-Level Security: not the authz model, but a defense-in-depth backstop

Unlike `drafts/`, go-oikumenea does **not** use Postgres RLS as the **isolation/authorization
model** — a deployment serves **one organization**, units are not mutually-distrusting SaaS
tenants, and the **application-layer PDP** + **shadow-visibility gate** on reads are and remain
**authoritative** (D-NoRLS).

RLS **is** enabled as a DB-level **defense-in-depth backstop** that mirrors the PDP-computed reach
(D-RLSDefenseInDepth). It guards the *forgotten-filter* bug class (a query that skips the PDP/gate),
not PDP-logic errors. The contract:

- The application sets per-**transaction** session GUCs at txn begin (via `SET LOCAL`, auto-reset):
  `app.person_id`, `app.is_instance_admin` (bool), `app.readable_units` (text[] of unit RIDs — PDP
  read reach), `app.writable_units` (text[] — write reach). The values come from the request's PDP
  context; the per-txn GUC seam lives in [platform](../modules/platform.md).
- RLS policies on unit-scoped tables use those GUCs:
  `USING (current_setting('app.is_instance_admin')::bool
          OR id|unit_id = ANY(current_setting('app.readable_units')::text[]))`; write policies use
  `app.writable_units`.
- The **application DB role must not hold `BYPASSRLS`**; instance-admin is expressed via the GUC
  flag, never a DB superuser. Schema migrations run as the owner/migration role.

See [decisions.md](decisions.md) D-NoRLS + D-RLSDefenseInDepth.

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
- **Health — `witchcraft-go-health`.** Reporters split into **readiness-gating** — DB reachability
  and the **schema-version check** (the service is unhealthy/refuses readiness against an
  unknown/newer schema — see [upgrade-safety.md](upgrade-safety.md)) — and **diagnostic-only**, e.g.
  the `closure-drift` reporter (D-ClosureDriftHealth), which surfaces a problem in `/status/health`
  **without** failing `/status/readiness` (it must not take the pod out of rotation).
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
  fields `camelCase`; enums `UPPER_SNAKE`. IDs are an `Rid` string alias (the URN format above) —
  Object, Link, and Action references all carry the full RID, never a bare uuid.
- **Errors** are declared as Conjure error types (see below) with safe-arg params.
- Generated code lands under **`internal/conjure`** (server interfaces + `RegisterRoutes` + clients,
  consumed in-repo) and, for the **publishable Go SDK**, a **nested module `client/`** (module path
  `…/go-oikumenea/client`) emitted client-only from the same IR by a second `conjure-plugin` project —
  so external code can `go get` it (the `internal/` copy is import-restricted). Both derive from the
  same `api/*.conjure.yml`, so they cannot drift. **Generated files are never hand-edited.** See
  [../api/README.md](../api/README.md) and [client/README.md](../../client/README.md).
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
| PK (RID) | `id TEXT DEFAULT oikumenea.new_rid('<svc>','<type>')` | `urn:oikumenea:tenant:prod:unit:0192f3a1-…` |
| Link RID | `link__<link_type>` in the entity-type slot | `urn:oikumenea:authz:prod:link__has_role:0192…` |
| Action RID | `action__<action_type>` in the entity-type slot | `urn:oikumenea:order:prod:action__issue_order:0192…` |
| Conjure ID type | `Rid` string alias (URN format) | — |
| Stable code | `code TEXT NOT NULL UNIQUE` (locale-agnostic, external ref) | `unit.code = "1-bn"` |
| Localized label | `name` (default-locale fallback) + i18n store | response: `{ukr, eng}` |
| Conjure service | `<Module>Service` | `MembershipService` |
| Atomic permission | `<resource>.<verb>[.<qualifier>]` | `unit.update`, `rank.scheme.manage` |
| Conjure error name | `<Module>:<Error>` | `Tenant:UnitCycleDetected` |
| Module Go package | `internal/<module>` | `internal/authorization` |
