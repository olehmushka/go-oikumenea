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

### Configuration (ECV + `pkg/refreshable`)

- **Install config** (`var/conf/install.yml`): operator-supplied **Postgres DSN**, IdP
  issuer/JWKS/audience, server ports/TLS. Secrets ECV-encrypted.
- **Runtime config** (`var/conf/runtime.yml`): hot-reloadable tunables (default page size,
  log level, grace windows) read through `refreshable`.
- The operator owns the DB and its credentials; nothing secret lives in the repo or the DB.

### Database access (pgx + sqlc)

- Constructs and owns the **pgx pool** against the operator DSN; exposes transaction helpers so
  application services control transaction boundaries.
- sqlc-generated query code lives per-module under `adapters/`; platform provides the pool and
  the `pgx.Tx` plumbing, not the queries.

### Schema bootstrap (first Atlas migration)

Creates the shared objects all modules rely on (see [conventions.md](../architecture/conventions.md)):

- the **`oikumenea` schema** and the `citext` extension;
- `oikumenea.uuid_v7()` — time-ordered UUID generator used as every PK default;
- `oikumenea.set_updated_at()` — `BEFORE UPDATE` trigger function for `updated_at`;
- `oikumenea.reject_mutation()` — `BEFORE UPDATE OR DELETE` guard for append-only tables;
- `oikumenea.schema_version` — the single-row table recording the applied schema revision.

### Boot-time schema-version check

On startup, compares `oikumenea.schema_version` to the revision the binary expects:

- DB **older** → run migrations (operator-gated) or refuse, per config;
- DB **newer/unknown** → **refuse readiness** (a witchcraft-go-health reporter reports unready)
  rather than risk writing against an unknown schema.

This is the runtime half of the non-destructive-upgrade guarantee
([upgrade-safety.md](../architecture/upgrade-safety.md)).

### Observability (the Palantir libraries)

- **Logging:** `svc1log`/`req2log`/`evt2log` from witchcraft-go-logging; structured params; the
  `request_id`/trace id flows on context into every log line and into [audit](audit.md).
- **Metrics:** `pkg/metrics` tagged registry; RED discipline on every endpoint plus domain
  counters (e.g. `pdp.decisions{result}`).
- **Tracing:** witchcraft-go-tracing spans; `X-B3-*` propagation.
- **Health:** witchcraft-go-health reporters — DB reachability + schema-version check.
- **Errors:** `werror` safe/unsafe params; mapped to Conjure `SerializableError` at transport.

### Shared kernel (`pkg/`)

Cross-cutting primitives with **no domain logic**:
- `pkg/id` — UUIDv7 helpers (mirrors the SQL `uuid_v7()`),
- `pkg/errors` — werror conventions + Conjure error mapping helpers,
- `pkg/pagination` — opaque page-token encode/decode,
- `pkg/events` — the in-process event bus + outbox seam,
- `pkg/locale` — ISO 639-3 validation + default-locale fallback helpers (used by
  [localization](localization.md) and label-bearing modules),
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
- No secrets in repo or DB; the operator's DSN/IdP config is supplied at deploy time.
- Shared SQL objects (`uuid_v7`, `set_updated_at`, `reject_mutation`) exist before any module
  table migration runs (migration ordering).

## Open seams / future

- The `pkg/events` bus is in-process with an outbox seam; extracting a module later turns it
  into a real broker without domain changes ([overview.md](../architecture/overview.md)).
- A background **job/worker** runtime (for scheduled purges, closure rebuilds) is an additive
  platform component; not required for the synchronous core.
- OpenTelemetry export is a drop-in behind the tracing seam.
