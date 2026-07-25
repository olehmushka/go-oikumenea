# Module: platform

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [overview](../architecture/overview.md) · [upgrade-safety](../architecture/upgrade-safety.md)
> Schema-level objects (no single table prefix; owns shared `oikumenea` objects + `pkg/`)

## Purpose

The cross-cutting foundation every domain module depends on. **Not a domain module** — it owns
the **witchcraft bootstrap + composition root**, configuration, observability wiring, the
database access layer, the **schema bootstrap** (the shared `oikumenea` SQL objects), the
**boot-time schema-version check**, and the shared kernel `pkg/`. It is the only place that
imports the witchcraft framework directly; domain modules stay framework-free in their `domain/`
layers.

## Responsibilities & components

### Bootstrap / composition root (`cmd/oikumenea/main.go`)

- Builds the `witchcraft.Server` (witchcraft-go-server), loads ECV-encrypted config, registers
  the conjure routes of every module into `wrouter`, and wires the shared services below into
  each module's `module.go`.
- Installs the [identity-federation](identity-federation.md) **token-validation middleware**
  ahead of authenticated routes.
- Hosts the operator **CLI subcommands** (`serve` is the default; `bootstrap-admin` /
  `recover-admin` are the break-glass admin-recovery commands — see *First-admin bootstrap* below).

### Configuration (ECV + `pkg/refreshable`)

- **Install config** (`var/conf/install.yml`): operator-supplied **Postgres DSN**, IdP
  issuer/JWKS/audience, server ports/TLS, the **`bootstrap_admin`** block
  (`{ issuer, subject | email, display_name, person_code? }`) consumed by the first-admin
  bootstrap below, the **`account.identity_linking.enabled`** boolean (default `true`)
  consumed by [identity-federation](identity-federation.md) to gate linking additional login
  points (e.g. Google + Keycloak) on the same account, and the **`crypto`** block selecting the
  **KMS key-provider backend** (`provider: aws-kms | gcp-kms | vault-transit | azure-kv | local-dev`
  + its endpoint/key-id/credentials) consumed by the crypto seam below (D-CryptoProvider). Secrets
  ECV-encrypted.
- **Runtime config** (`var/conf/runtime.yml`): hot-reloadable tunables (default page size,
  log level, grace windows, the `closure_drift.max_age` staleness window for the `closure-drift`
  health reporter — default ~26h; `0` disables the staleness check) read through `refreshable`.
- **Data packs & module flags** (`pinax.packs`, `modules.*.enabled` — D-DataPacks, M54): `pinax.packs`
  is an optional operator-mounted directory scanned at boot **beside** the `go:embed`-ed pinax presets
  (same version-gated, create-if-absent pipeline; a name collision with an embedded or another pack's
  preset fails boot — packs are additive). `modules.<name>.enabled` (default `true`) switches an
  **enrichment vertical** off — one of `finance`/`religion`/`vehicle`/`externalorg`/`company`/
  `education`; core + depended-on reference modules are always on. A disabled module registers **no
  routes** (→ 404) and its permission codes are **not grantable** (a role/principal grant naming one is
  rejected as unknown), but its **schema still migrates** — so re-enabling is a config flip, never a
  migration. See [config.go](../../internal/platform/config/config.go) `ModuleEnabled` /
  `DisabledModulePrefixes` and the composition-root gating in `main.go`.
- **Login security log** (`login-security.*` — D-LoginSecurityLog, M37): `trust-forwarded-for` (default
  off) tells the core to log the facade-set `X-Forwarded-For` as the client IP rather than `RemoteAddr`
  — set it on **only** behind a facade that sets an authoritative XFF (D-HeadlessTopology amended).
  `retention-days` (default `0` = retain forever) bounds the retention sweep; `dedup-window-seconds`
  (default `3600`) collapses repeat `(account, context, ip)` occurrences into one row.
- The operator owns the DB and its credentials; nothing secret lives in the repo or the DB.

### Database access (pgx + sqlc)

- Constructs and owns the **pgx pool** against the operator DSN; exposes transaction helpers so
  application services control transaction boundaries.
- sqlc-generated query code lives per-module under `adapters/`; platform provides the pool and
  the `pgx.Tx` plumbing, not the queries.
- **RLS GUC seam (D-RLSDefenseInDepth / D-RLSLiveReach):** per authenticated request the
  authenticator installs a **lazy** RLS-scoped connection holder (`db.WithLazyConn`, R-03): the
  connection is pinned and the **three** O(1) GUCs — `app.person_id` + `app.is_instance_admin` +
  `app.principal_id` — are set in **one** `set_config` round trip only when a handler first touches an
  RLS-consuming module (`db.RequestQuerier`/`db.RequestDBTX`), and released after the response iff
  acquired. The RLS policies compute reach live from those GUCs (`oikumenea.authz_unit_in_reach`); no
  unit-list GUC exists. A request subject is **either** a person (`app.person_id`, optionally the admin
  flag) **or** a machine principal (`app.principal_id`) — never both; the unset one is an empty probe.
  System paths (`db.RunAsSystem`) still pin eagerly with the admin flag. The application DB role is
  provisioned **without `BYPASSRLS`**.
- **Machine reach arm (M55 / D-ServiceIdentities, migration `0042`):** a service principal set no
  `app.person_id`, so it had no reach. `app.principal_id` + `authz_principal_org_in_reach(org, wr)`
  (an org-direct check of `authz_principal_grants`, read live → immediate revocation) give an
  **org-confined** grant reach into that organization's RLS-guarded rows and only that org's. Because
  the reach predicate may read only RLS-exempt tables, `0042` adds a dedicated **RLS-exempt projection
  `authz_unit_org(unit_id → org_id)`**, trigger-maintained from `tenant_units` (its `unit_id` FK is
  `DEFERRABLE INITIALLY DEFERRED` so the BEFORE-INSERT projection write can precede the parent row):
  the child-table arm resolves a unit → org through the projection, while the `tenant_units` policy
  checks the row's own `org_id` (the one case the projection can't serve mid-INSERT — this is what
  lets a connector create a brand-new, edgeless unit). `org_id NULL` (instance-wide) grants confer no
  operational reach. `db.RLSState` is `{PersonID, IsInstanceAdmin, PrincipalID}`.
- **Query-count tracer (M46 measurement harness):** every pool built by `db.NewPool` carries a
  `pgx.QueryTracer` that is a no-op unless a test attaches a counter via `db.WithQueryCounter` —
  integration tests assert per-request statement budgets with it (`db.AssertQueryCount`).

### Schema bootstrap (first Atlas migration)

Creates the shared objects all modules rely on (see [conventions.md](../architecture/conventions.md)):

- the **`oikumenea` schema** and the `citext` extension;
- `oikumenea.uuid_v7()` — time-ordered UUID helper;
- `oikumenea.new_id(service, kind, type)` — mints the packed **UUIDv8 RID** used as every PK default
  (D-ResourceIdentifiers, amended F-014); reads no GUC. Paired with the `rid_app/service/kind/type()`
  decoders and the seeded `platform_rid_services` / `platform_rid_types` registries;
- `oikumenea.set_updated_at()` — `BEFORE UPDATE` trigger function for `updated_at`;
- `oikumenea.reject_mutation()` — `BEFORE UPDATE OR DELETE` guard for append-only tables;
- `oikumenea.schema_version` — the single-row table recording the applied schema revision;
- **`geo_countries`** *(an ontology **Object** of the **location** service — RID-keyed, F-014; defined
  in the bootstrap migration but owned by [location](location.md))* — the seeded **ISO-3166-1 alpha-2
  country registry** (D-Geo): `id uuid` RID PK (`new_id(12,1,1)`), `code CHAR(2) NOT NULL UNIQUE` (the
  external lookup key), default-locale `name` (translatable via the [localization](localization.md)
  store, `entity_type='country'`), `status`, `sort_order`. A shared reference table FK'd **by `id`** by
  [person](person.md) (`country_of_birth_id`, citizenships, residences, phones) and
  [document](document.md) (paper `issuing_country_id`, personal-code scheme `country_id`) and
  [rank](rank.md) (`rank_systems.country_id`). Clients resolve a code → RID via `GET /geo/countries`
  (location `GeoService`, `country.read`). Seeded from ISO-3166 and instance-admin-extensible
  (`country.manage`). **WOF-enriched** (D-GeoPlaces): additive `wof_id` + PostGIS
  `geom`/`centroid`/`bbox` + `iso_a3`/`numeric_code`, mirrored from the country's `geo_places` record.
- **`geo_places`** *(location-service Object — RID-keyed, F-014; D-GeoPlaces)* — the **Who's-On-First
  administrative gazetteer** (country/region/county/locality): `id uuid` RID PK (`new_id(12,1,2)`),
  `wof_id BIGINT NOT NULL UNIQUE` (the WOF concordance/import key), `placetype`, `parent_id uuid`
  self-FK → `geo_places(id)` (tree), denormalized `country_id` → `geo_countries(id)`, translatable
  `name` (`entity_type='geo_place'`), `population`, `hierarchy`/`concordances` JSONB, `status`,
  **PostGIS** `geom`/`centroid`/`bbox` (GeoJSON via `ST_AsGeoJSON`), and
  `(source, source_version, imported_at)` provenance. Loaded over `POST /import/geo-places` by the
  hermenea `wof-sqlite` connector, which streams natural keys and resolves `wof_id`/`code → id` in SQL.
  **Supersedes** the planned ISO-3166-2 `geo_subdivisions` (D-GeoSubdivisions). PostGIS is enabled in
  the bootstrap migration (pulled forward from D-Location/M19).
- **Spatial extension prerequisite (D-Location, M19).** The operator DB must carry **PostGIS** (enabled
  in the bootstrap migration) — it backs both the WOF `geo_places` gazetteer (D-GeoPlaces) and the
  `location_locations` point model + its `ST_DWithin` radius/bbox queries. The **stock
  `postgis/postgis:16-3.4` image suffices**: MGRS is derived in the application (pure Go) and there is no
  H3, so no h3-pg or custom image is needed. The **readiness gate checks PostGIS is installed** and
  refuses readiness otherwise (so a non-PostGIS image is caught at boot, not at the first spatial write).

Later migrations **enable RLS** on unit-scoped tables and create the PDP-mirror policies keyed on
the `app.*` GUCs (D-RLSDefenseInDepth), staged permissive-first then tightened
([upgrade-safety.md](../architecture/upgrade-safety.md)). The owner/migration role may bypass RLS;
the runtime application role may not.

### Boot-time schema-version check

On startup, compares `oikumenea.schema_version` to the revision the binary expects:

- DB **older** → run migrations (operator-gated) or refuse, per config;
- DB **newer/unknown** → **refuse readiness** (a witchcraft-go-health reporter reports unready)
  rather than risk writing against an unknown schema.

This is the runtime half of the non-destructive-upgrade guarantee
([upgrade-safety.md](../architecture/upgrade-safety.md)).

### First-admin bootstrap (first boot, idempotent)

On startup, **if no instance admin exists**, the service seeds the first one from the
`bootstrap_admin` install config (D-Bootstrap): in one transaction it creates a
[person](person.md), an [account + external identity](identity-federation.md) bound to the
configured IdP `(issuer, subject)`, and an `authz_instance_admins` grant
([authorization](authorization.md)). It is **idempotent** — once any instance admin exists the
block is ignored. Because auth is delegated (L-AuthzOnly), this seeds an **IdP identity binding,
not a credential**. The seed writes audit as `actor_type='system', subsystem='bootstrap'`
([audit](audit.md)). The unit graph is left **empty**; the seeded admin builds it via the API.

**Recovery CLI (`bootstrap-admin` / `recover-admin`).** Beyond the first-boot config-seed, the
composition root exposes idempotent CLI subcommands that **reuse the same seed transaction** to
(re)establish an instance admin — the supported break-glass path for a **lost sole instance
admin**, replacing raw DB surgery (D-Bootstrap). They are gated on *no active instance admin
exists* OR an explicit `--force`, respect the boot-time schema-version check (refuse against an
unknown/newer schema), and are **operator-host-gated** — possession of operator DB/host access is
the authorization. Writes audit as `actor_type='system', subsystem='recover-admin'`.

### Crypto / key-provider seam (D-CryptoProvider)

Envelope encryption for `pii:sensitive` data (today: [document](document.md) personal-code values),
behind a **pluggable `KeyProvider`** so the KMS vendor is install config, not a code dependency:

- **`KeyProvider` interface** — `Wrap(dek) → wrappedDEK`, `Unwrap(wrappedDEK) → dek`, `KeyRef()`
  (the active KEK id + version). Backends: **`aws-kms`, `gcp-kms`, `vault-transit`, `azure-kv`,
  `local-dev`**, selected by the install-config `crypto` block. The **KEK never leaves the KMS**; the
  app DB holds only ciphertext + `wrapped_dek` + `key_ref` + a `value_blind_index`.
- **`pkg/crypto`** — envelope **wrap/unwrap** (per-record DEK; AEAD encrypt of the value locally),
  a **keyed-HMAC blind index** for equality lookup / uniqueness without decryption, and a
  short-TTL **unwrapped-DEK cache** (KMS is on the unwrap/read path only). **Crypto-erase** =
  destroy the `wrapped_dek` (person purge).
- The operator owns the KMS (L-OperatorDB-style); no key material in the repo or DB. Scope today is
  `pii:sensitive` only; extending to `pii:special` / audit payloads is parked (DS-29).

### Validation registry (`pkg/personalcode`, D-PersonalCodes)

A compiled, reviewable **validator registry** for national-identifier schemes, keyed on the scheme
(e.g. UA-RNOKPP checksum, IT codice fiscale, US-SSN format) — "enforcement-as-code" alongside the
permission catalog. [document](document.md) calls it on personal-code create/update; precedence is
**code validator > the scheme's catalog `validation_regex` > accept-with-warning**.

### Observability (the Palantir libraries)

- **Logging:** `svc1log`/`req2log`/`evt2log` from witchcraft-go-logging; structured params; the
  `request_id`/trace id flows on context into every log line and into [audit](audit.md).
- **Metrics:** `pkg/metrics` tagged registry (the one witchcraft emits from via `metric.1`).
  witchcraft's built-in `metric.1` request metrics come for free; the deliberate application-metric
  surface over the hot seams is listed under **Application metrics & alarm signals** below. Emitters
  read the registry with `metrics.FromContext(ctx)` — no plumbing beyond the request/boot context
  (the fallback is the same `DefaultMetricsRegistry` witchcraft emits from).
- **Tracing:** witchcraft-go-tracing spans; `X-B3-*` propagation.
- **Health:** witchcraft-go-health reporters split two ways — **readiness-gating** (DB reachability,
  schema-version check) and **diagnostic-only**. The **`closure-drift`** reporter (D-ClosureDriftHealth)
  is diagnostic-only: it reads [tenant](tenant.md)'s `tenant_closure_status` (the persisted result of
  on-demand `POST /closure/verify`) and reports ERROR on drift / WARNING when a graph is never-verified
  or stale beyond the refreshable `closure_drift.max_age` window / HEALTHY otherwise. It is wired into
  `GET /status/health` but **deliberately excluded from `/status/readiness` and `/status/liveness`** —
  a drifted closure must not pull the pod from rotation (the PDP keeps serving off the stored closure).
  It does **not** recompute on scrape (operator-refresh only).
- **Errors:** `werror` safe/unsafe params; mapped to Conjure `SerializableError` at transport.

### Application metrics & alarm signals (R-20)

Every July scale-fix turned a visible failure (slow requests) into a fast path guarded by machinery
with *new, invisible* failure modes. These four hot seams emit a thin metric surface on the
witchcraft registry so those failure modes are observable; the right-hand column is the
alarm-worthy signal an operator should page/warn on.

| Metric (tags) | Seam | Alarm-worthy signal |
|---|---|---|
| `authz.grantcache.hits` / `.misses` / `.revalidations` / `.resets` (counters), `.entries` (gauge) | grant cache (`internal/authorization`) | steady-state **hit rate < ~90 %** ⇒ TTL misconfig or an epoch-bump storm (looks like "the DB got slow") |
| `outbox.dispatched` / `.retried` / `.dead` (counters), `outbox.pending` / `outbox.oldest_pending_age_seconds` (gauges) | outbox dispatcher (`internal/platform/outbox`) | **`outbox.dead` > 0** (a `notify` event exhausted its retries — silent data-flow loss; also logged as a distinct WARN `outbox dispatcher: event dead-lettered`); **`oldest_pending_age_seconds` > a few × `PollInterval`** ⇒ dispatcher wedged |
| `dataimport.chunk_seconds` (timer), `dataimport.rows.merged` / `.skipped` (counters) — all tagged `object_type` | import orchestrator (`internal/dataimport`) | per-object-type chunk latency climbing, or a long run merging **0** rows (connector/mapper broken) |
| `tenant.closure.edit_seconds` (timer, tagged `op=add|remove`) | closure maintenance (`internal/tenant`) | edit latency scaling with **graph size** rather than the affected slice ⇒ the M48 incremental property regressed |

Not a metric but the same class of "silent until it bites" signal: **audit partitions older than
`audit.retention-months`** (`config.Audit.RetentionMonths`) accumulating. Retention is a deliberate
operator act (D-AuditRetention) with no built-in enforcer, so the presence of over-window partitions
is the reminder to run the prune — check it in the same ops sweep as the gauges above.

### Shared kernel (`pkg/`)

Cross-cutting primitives with **no domain logic**:
- `pkg/id` — UUIDv7 helpers (mirrors the SQL `uuid_v7()`),
- `pkg/errors` — werror conventions + Conjure error mapping helpers,
- `pkg/pagination` — opaque page-token encode/decode,
- `pkg/events` — the domain-event seam, **two classes** (D-EventOutbox; patterns.md *Domain events:
  atomic vs. notify*): **`atomic`** (`Bus`) subscribers run **synchronously within the originating
  transaction** (so e.g. order auto-apply effects share the issue txn — D-OrderApply), the default and
  the only class in use today; **`notify`** (`OutboxWriter` → the `oikumenea.platform_outbox` table,
  migration `0011_infra`) enqueues on the write txn and is delivered **after commit, at least once** by the
  `internal/platform/outbox` dispatcher (below). The `Bus` is **sealed** after boot — a later `Subscribe`
  panics (R-10),
- `internal/platform/outbox` — the outbox **dispatcher**: polls `platform_outbox`, claims rows
  `FOR UPDATE SKIP LOCKED` (replica-safe, mirrors the hermenea worker), delivers to registered `notify`
  handlers, retries with backoff, dead-letters past `max_attempts`; started in `main.go`. No `notify`
  producers exist yet — a live-but-empty proven seam (R-10 / D-EventOutbox),
- `pkg/locale` — ISO 639-3 validation + default-locale fallback helpers (used by
  [localization](localization.md) and label-bearing modules),
- `pkg/crypto` — envelope wrap/unwrap behind the `KeyProvider` seam, blind-index HMAC, DEK cache
  (D-CryptoProvider; used by [document](document.md) for personal-code values),
- `pkg/personalcode` — the national-identifier validator registry (D-PersonalCodes),
- `pkg/config` — refreshable accessors.

## Conjure / endpoint surface

Platform owns no domain endpoints. It exposes operational surfaces:
- `GET /status/health`, `GET /status/liveness`, `GET /status/readiness` (witchcraft health),
- the generated **OpenAPI** reference site (from the Conjure IR of all modules),
- (optionally) `GET /status/version` reporting binary + schema revision.

These are unauthenticated by design.

### Generic import endpoint (D-Hermenea / ex-D-DataIngestion)

Platform also hosts the **reference-data import endpoint** the [hermenea](hermenea.md) companion calls:

| Op | Intent | Perm |
|---|---|---|
| `POST /import/{objectType}` | Idempotent, **non-destructive, code-keyed** upsert of a **canonical envelope** into the target catalog, in one transaction, audited as a `system` actor; stamps `(source, source_version, imported_at)` provenance on each row. A large dataset arrives as a **chunked run** (optional `runId`/`seq`/`isLast` envelope fields, R‑05/M49): one transaction per ~5k-record chunk, batch finalizers on the `isLast` chunk, the high-volume object-types applying each chunk as one set-based UNNEST merge; the server stays stateless per chunk (replay-safe) | `import.manage` (instance) |

It runs over an **upsert registry** (mirrors `pkg/events.Bus`): each importable object-type registers a
handler at composition time — `geo-countries` is the first (M16). Authorization uses the
**`hermenea-importer` service principal**: a **shared-secret** auth path (`HERMENEA_OIKUMENEA_TOKEN`,
ECV-refreshable) beside the OIDC `Authenticator` resolves to a principal holding **exactly**
`import.manage`, audited as `system` (L-AuthzOnly amendment; see [hermenea](hermenea.md)). The reverse
push trigger (`POST /sync/{source}` on hermenea, `OIKUMENEA_HERMENEA_TOKEN`) is a thin outbound HTTP
client wired here.

### Legal-basis catalog (D-OverlayFoundation, M29)

Platform owns the cross-cutting **GDPR lawful-basis catalog** `platform_legal_basis_kinds` — a natural
`code` PK reference table (like `geo_countries`) seeded with the **Article 6** lawful bases (consent,
contract, legal_obligation, vital_interests, public_task, legitimate_interest) and the **Article 9**
special-category conditions, partitioned by an `article` (`art6`/`art9`) column. It is referenced by FK
from every future `pii:special` overlay store (the FK is **NOT NULL** there — M31+), so special-category
processing is gated on a structured lawful basis rather than prose (see the *attribution convention* in
[conventions.md](../architecture/conventions.md)). The `PlatformCatalogService` exposes it:

| Op | Intent | Perm |
|---|---|---|
| `GET /platform/v1/legal-basis-kinds` | List the catalog | `legal-basis.read` |
| `PUT /platform/v1/legal-basis-kinds/{code}` | Add/update an entry (audited) | `legal-basis.manage` (instance) |

It is composed by `platform.RegisterCatalog` (after the audit service + PEP enforcer exist), not in
`Bootstrap`.

### Color catalog (D-Color, M42)

Platform also owns the cross-cutting **color catalog** `platform_colors` — its **first RID-bearing
Object** (RID service 1, object type `1,1,1`). Unlike the natural-key legal-basis table, colors carry a
RID (the FK target) and a translatable `name`. It is a **per-domain palette**: a `domain` discriminator
(`eye` / `hair` / `vehicle`, TEXT+CHECK) with `UNIQUE(domain, code)`, a stable `code`, an i18n `name`
(localization store, entity `color`, keyed by the RID since `code` is unique only per-domain), and a
**nullable** `hex` swatch (biological eye/hair colors are categories, not precise hex). Seeded with
eye/hair/vehicle baselines. Referenced by **hard FK** from `vehicle_vehicles.color_id` and
`person_physical_descriptions.eye_color_id`/`hair_color_id` (`ON DELETE RESTRICT`); the referencing
modules validate the color's `domain` in their application layer (a single-column FK can't constrain the
palette). The same `PlatformCatalogService` exposes it:

| Op | Intent | Perm |
|---|---|---|
| `GET /platform/v1/colors?domain=` | List a palette (or all) | `color.read` (reader-tier) |
| `PUT /platform/v1/colors` | Add/update a color, upsert on `(domain, code)` (audited) | `color.manage` (instance) |

## Dependencies

- **Calls:** nothing domain-side. Provides infrastructure to **every** module.
- **Called by:** every module (DB pool, config, logging, metrics, events, pagination).
  [identity-federation](identity-federation.md) middleware is installed here.

## Invariants & safety

- Domain `domain/` layers never import witchcraft; framework lives only here and in
  `transport/`.
- The service **refuses to run** against an unknown/newer schema (boot check).
- No secrets in repo or DB; the operator's DSN/IdP config is supplied at deploy time. The
  **encryption KEK lives only in the external KMS** (D-CryptoProvider); the DB holds ciphertext +
  wrapped DEK + key reference, never plaintext `pii:sensitive` values or the KEK.
- Shared SQL objects (`uuid_v7`, `set_updated_at`, `reject_mutation`, `geo_countries`) exist before
  any module table migration runs (migration ordering).

## Open seams / future

- The `pkg/events` bus is in-process (`atomic` subscribers run in the originating transaction); the
  **`notify` transactional outbox** now exists (D-EventOutbox / R-10 — `platform_outbox` + dispatcher)
  but has **no producers yet** (every event is `atomic` today). Extracting a module later turns the seam
  into a real broker without domain changes ([overview.md](../architecture/overview.md), DS-26). Outbox
  **retention** (pruning dispatched/dead rows) is an operator concern, still an open seam.
- The background **job/worker** runtime moved **out of process** into the [hermenea](hermenea.md)
  companion service (**D-Hermenea supersedes D-Worker**): scheduled syncs, the job queue, and the
  `worker_jobs` ledger live in hermenea's own DB, not in oikumenea. Other DS-25 beneficiaries
  (scheduled purges, expiry sweeps, partition maintenance) can run as hermenea jobs calling oikumenea
  over HTTP. (A *scheduled* closure rebuild is still **not** among these — ruled out; closure repair
  stays on-demand and drift detection is the diagnostic `closure-drift` reporter — D-ClosureDriftHealth.)
- OpenTelemetry export is a drop-in behind the tracing seam.
- The `KeyProvider` crypto seam (D-CryptoProvider) protects `pii:sensitive` today; extending envelope
  encryption to `pii:special` person fields and audit `before`/`after` payloads reuses the same seam
  but is parked (DS-29).
