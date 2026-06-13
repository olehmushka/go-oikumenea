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
- The operator owns the DB and its credentials; nothing secret lives in the repo or the DB.

### Database access (pgx + sqlc)

- Constructs and owns the **pgx pool** against the operator DSN; exposes transaction helpers so
  application services control transaction boundaries.
- sqlc-generated query code lives per-module under `adapters/`; platform provides the pool and
  the `pgx.Tx` plumbing, not the queries.
- **RLS GUC seam (D-RLSDefenseInDepth):** the transaction helper sets the per-transaction session
  GUCs from the request's PDP context at txn begin — `SET LOCAL app.person_id`,
  `app.is_instance_admin`, `app.readable_units`, `app.writable_units` (the PDP read/write reach
  from [authorization](authorization.md)). This feeds the RLS backstop; the application DB role is
  provisioned **without `BYPASSRLS`**.

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
- **`geo_countries`** *(the one ontology **Object** platform owns — D-Ontology; natural `code` PK,
  not an RID)* — the seeded **ISO-3166-1 alpha-2 country registry** (D-Geo): `code CHAR(2)`
  PK, default-locale `name` (translatable via the [localization](localization.md) store,
  `entity_type='country'`), `status`, `sort_order`. A shared reference table (like `uuid_v7()`) FK'd
  by [person](person.md) (`country_of_birth`, citizenships, residences) and
  [document](document.md) (paper `issuing_country`, personal-code scheme `country_iso`). Seeded from
  ISO-3166 and instance-admin-extensible (`country.manage`) for historical/edge-case entities.

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
- **Metrics:** `pkg/metrics` tagged registry; RED discipline on every endpoint plus domain
  counters (e.g. `pdp.decisions{result}`).
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

### Shared kernel (`pkg/`)

Cross-cutting primitives with **no domain logic**:
- `pkg/id` — UUIDv7 helpers (mirrors the SQL `uuid_v7()`),
- `pkg/errors` — werror conventions + Conjure error mapping helpers,
- `pkg/pagination` — opaque page-token encode/decode,
- `pkg/events` — the in-process event bus + outbox seam; for a mutation, subscribers run
  **synchronously within the originating transaction** (so e.g. order auto-apply effects share the
  issue txn — D-OrderApply),
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

- The `pkg/events` bus is in-process (subscribers run in the originating transaction) with an outbox
  seam; extracting a module later turns it into a real broker without domain changes
  ([overview.md](../architecture/overview.md), DS-26).
- A background **job/worker** runtime (for scheduled purges, expiry sweeps, partition maintenance)
  is an additive platform component; not required for the synchronous core. (A *scheduled* closure
  rebuild is **not** among these — it was ruled out; closure repair stays on-demand and drift
  detection is the diagnostic `closure-drift` reporter — D-ClosureDriftHealth.)
- OpenTelemetry export is a drop-in behind the tracing seam.
- The `KeyProvider` crypto seam (D-CryptoProvider) protects `pii:sensitive` today; extending envelope
  encryption to `pii:special` person fields and audit `before`/`after` payloads reuses the same seam
  but is parked (DS-29).
