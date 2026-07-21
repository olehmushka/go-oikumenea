# Module: hermenea (companion service)

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> (**D-Hermenea**) · [platform](platform.md) (the oikumenea import endpoint).
> Table prefix (its **own** DB): `hermenea.*`

> **Hermenea is a separate deployable, not an `internal/` module of oikumenea.** It is a second binary
> (`cmd/hermenea`) with its **own PostgreSQL**, its **own Atlas migrations** (`migrations/hermenea/`),
> and its **own ontology/RIDs**. It couples to oikumenea **only over HTTP** — it never opens
> oikumenea's database. This doc follows the module template for consistency, but the "data model" is
> hermenea's DB and the "API surface" spans both hermenea's endpoints and the oikumenea endpoint it
> calls.

## Purpose

Owns **reference-data ingestion** and the **background-job runtime** that drives it (D-Hermenea, which
**supersedes D-Worker** and **folds D-DataIngestion / M17 into M16**). Hermenea **fetches** external
reference datasets over HTTP (or a bundled file), **stages** the raw payload, **maps** it to a canonical
envelope, and **loads** it into oikumenea by calling oikumenea's public `POST /import/{objectType}`
endpoint — a **code-keyed, idempotent, non-destructive** upsert. Syncs run on hermenea's **own cron**
or are **pushed on demand** by an oikumenea admin action (`POST /sync/{source}`). It mirrors Palantir
Foundry's *Data Connection → Pipeline → Ontology mapping* stages, right-sized for a self-hosted Go
service (no Spark).

The split is deliberate (D-Hermenea *Why*): ingestion is failure-prone, bursty, and pulls in outbound
network + parser surface that must **not** share a process or database with the PDP. Hermenea is
independently deployable/restartable/scalable; oikumenea's public import API is the **only** coupling.

## Entities & aggregates

**Ontology kinds** (hermenea's own registry namespace; D-Ontology shape reused) — four **Objects** in
hermenea's DB: `Import source`, `Import raw batch`, `Import run`, `Worker job`; plus `Worker schedule`
(cron registration). **Actions** are hermenea-local job executions (recorded in `worker_jobs` /
`import_runs`, not in oikumenea's audit log). The **imported rows** land as oikumenea Objects through
the import endpoint, carrying `(source, source_version, imported_at)` **provenance** and audited there
as `system`-actor Actions.

- **Import source** — a registered external dataset: `type ∈ {http, file, wof-sqlite, http-files}`
  (DS-44 parks `jdbc-sql`/`object-store`), a connector-specific locator (URL, bundled path, or a
  whitespace-separated URL list for `http-files`), an `object_type`
  (which oikumenea import target it feeds, e.g. `geo-places`), optional credentials (via the M0
  crypto seam), and an optional cron schedule.
- **Import raw batch** — one fetched payload landed **verbatim** (checksum, `source_version`,
  `fetched_at`), re-mappable without re-fetch. A large **streamed** source (the `wof-sqlite` path,
  D-GeoPlaces) records a `staged_path` file reference instead of inline `payload` bytes.
- **Import run** — the lineage ledger for one map+load: source, version, record counts
  (created/updated/skipped), checksum, status, errors.
- **Worker job** — one unit of queued work (`fetch`, `map`, `load`, or a composite `sync`):
  idempotency key, `run_after`, attempts, `max_attempts`, backoff state, `locked_by`/`locked_at`,
  `last_error`, status.
- **Worker schedule** — a cron registration (`source` + cron spec) the scheduler enqueues from.

## Data model

Hermenea's own `hermenea` schema, same conventions as oikumenea where they apply (RID PKs via a reused
`new_id()`/`pkg/rid`, `TIMESTAMPTZ` UTC, `set_updated_at()`, `TEXT`+`CHECK` enums, soft-delete on the
registry tables). The queue/ledger tables are **append-or-update operational** tables (not soft-deleted).

**`hermenea.import_sources`**
- `id` PK (RID), `code TEXT NOT NULL UNIQUE`, `name TEXT NOT NULL`
- `connector_type TEXT NOT NULL CHECK (connector_type IN ('http','file','wof-sqlite','http-files'))`
  (`http-files` added by migration `0002`, D-Languages M18)
- `object_type TEXT NOT NULL` — the oikumenea import target (e.g. `geo-places`)
- `locator TEXT NOT NULL` — URL (http / wof-sqlite `.db.bz2`), bundled path (file), or a
  whitespace-separated URL list (http-files)
- `cron TEXT` — optional cron spec; `NULL` = trigger-only
- `enabled BOOLEAN NOT NULL DEFAULT TRUE`
- `credentials_ref TEXT` — optional crypto-seam reference; timestamps + `deleted_at`

**`hermenea.import_raw_batches`**
- `id` PK (RID), `source_id TEXT NOT NULL REFERENCES import_sources(id) ON DELETE RESTRICT`
- `source_version TEXT`, `checksum TEXT NOT NULL`
- `payload BYTEA` (inline body, small http/file sources) **xor** `staged_path TEXT` (on-disk file
  reference for large streamed sources — the `wof-sqlite` path); a CHECK requires one to be present
- `fetched_at TIMESTAMPTZ NOT NULL`; `created_at`

**`hermenea.import_runs`**
- `id` PK (RID), `source_id TEXT NOT NULL REFERENCES import_sources(id)`
- `raw_batch_id TEXT REFERENCES import_raw_batches(id)`, `source_version TEXT`
- `status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed'))`
- `created_count INT`, `updated_count INT`, `skipped_count INT`, `error TEXT`
- `started_at TIMESTAMPTZ NOT NULL`, `finished_at TIMESTAMPTZ`; timestamps

**`hermenea.worker_jobs`** (the queue + ledger — D-Hermenea / ex-D-Worker)
- `id` PK (RID), `job_type TEXT NOT NULL` (`sync`/`fetch`/`map`/`load`/`heartbeat`)
- `idempotency_key TEXT NOT NULL UNIQUE` — dedupe on enqueue
- `source_id TEXT REFERENCES import_sources(id)`, `payload JSONB`
- `status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed','dead'))`
- `attempts INT NOT NULL DEFAULT 0`, `max_attempts INT NOT NULL`
- `run_after TIMESTAMPTZ NOT NULL DEFAULT now()` — backoff/scheduling gate
- `locked_by TEXT`, `locked_at TIMESTAMPTZ`, `last_error TEXT`; timestamps
- Claim query: `SELECT … WHERE status='queued' AND run_after<=now() ORDER BY run_after FOR UPDATE SKIP LOCKED LIMIT n`

**`hermenea.worker_schedules`**
- `id` PK (RID), `source_id TEXT NOT NULL REFERENCES import_sources(id)`, `cron TEXT NOT NULL`,
  `enabled BOOLEAN NOT NULL DEFAULT TRUE`, `last_enqueued_at TIMESTAMPTZ`; timestamps

**oikumenea side (its DB, expand-only).** `oikumenea.geo_countries` (and every later import target)
gains nullable provenance columns `source TEXT`, `source_version TEXT`, `imported_at TIMESTAMPTZ`,
stamped from the envelope on each upsert (D-DataIngestion lineage, retained under D-Hermenea).

## Conjure API surface

**Hermenea — `HermeneaService`** (`api/hermenea.conjure.yml`). All endpoints require the
`OIKUMENEA_HERMENEA_TOKEN` bearer (the only caller is oikumenea's push-trigger client + operators).

| Op | Intent |
|---|---|
| `POST /sync/{source}` | Enqueue a sync for a registered source (the **push trigger** from oikumenea); returns the `worker_job` id |
| `GET /sources` | List registered import sources |
| `POST /sources` / `PUT /sources/{id}` | Register / edit a source (incl. its cron) |
| `GET /runs` | List import-run lineage (paginated) |
| `GET /jobs` | List worker jobs (status/attempts/last_error) |
| `POST /watchlist/check` | **Live watchlist screening** (D-Watchlists, M34) — the first **synchronous** surface oikumenea calls: hermenea owns egress to OFAC/EU/UN/INTERPOL + a ≤24h cache (`hermenea.watchlist_cache`), returns per-person **match metadata only**. The real INTERPOL Red Notices connector ships; sanctions providers are a documented stub |

**Oikumenea — import endpoint** (in [platform](platform.md), the only write path hermenea uses):

| Op | Intent | Perm |
|---|---|---|
| `POST /import/{objectType}` | Idempotent, non-destructive code-keyed upsert of a **canonical envelope** into the target catalog; one txn; audited as `system` | `import.manage` (instance) |

**Canonical envelope** (hermenea produces, oikumenea consumes):

```jsonc
{ "objectType": "geo-countries", "source": "iso-3166", "sourceVersion": "2024", "license": "…",
  "generatedAt": "2026-06-13T00:00:00Z",
  "records": [ { "code": "UA", "name": "Ukraine", "alpha3": "UKR", "numeric": "804" } ],
  // Chunked runs (R-05 / M49): a large dataset arrives as sequential ~5k-record chunks, each its own
  // envelope + oikumenea transaction, ended by a trailing (possibly empty) isLast finalize chunk that
  // runs the object-type's batch finalizers. All three absent = single-shot (pre-M49 semantics).
  "runId": "…", "seq": 3, "isLast": false }
```

## Dependencies

- **Hermenea calls** oikumenea's **public HTTP API only** — `POST /import/{objectType}` (via the
  **generated oikumenea Conjure client**), authorized by `HERMENEA_OIKUMENEA_TOKEN`. It opens **no**
  oikumenea DB connection.
- **Connector-plane self-registration (M53 / D-ConnectorPlane).** Hermenea also calls the core's
  `PUT /connectors/v1/registration` at boot (its row + declared sources) and
  `POST /connectors/v1/sync-runs` on each run's open/close, so it appears in the core connector
  registry with completed runs an operator can see. Both reuse the **same base URL and shared secret**
  as the import loader (`internal/hermenea/reporter`), so there is no second credential. Reporting is
  **best-effort**: a failure logs and is dropped — the core registry is a read model and never gates
  hermenea's own execution (*visibility, not orchestration*). Hermenea stays authoritative for
  execution; the registry mirrors what it reports.
- **Oikumenea calls** hermenea's `POST /sync/{source}` (push trigger) via a thin HTTP client,
  authorized by `OIKUMENEA_HERMENEA_TOKEN`, from an admin/console action.
- Hermenea reuses platform-kernel packages where shared (`pkg/rid`, `pkg/crypto` for credential refs,
  the witchcraft/observability stack) but has its own composition root and DB pool.

## Authorization touchpoints

- **`import.manage`** — a new **instance-scope** oikumenea permission gating `POST /import/{objectType}`.
- **Service principal** — `HERMENEA_OIKUMENEA_TOKEN` (ECV-refreshable runtime secret) authenticates a
  `hermenea-importer` principal that holds `import.manage` plus the M53 connector-plane self-service
  codes `connector.register` + `connector.report` (all instance-scope, boot-seeded together — **not**
  the `wiring.*` read codes, since hermenea pushes and reports but does not pull-wire); resolved by a
  shared-secret auth path beside the OIDC `Authenticator`, audited as a **`system`** actor (audit
  actor-shape CHECK already allows `person | system`). Amends **L-AuthzOnly**
  ([decisions.md](../architecture/decisions.md)) — no credential is stored; the operator supplies the
  secret at deploy time, validated by comparison (bootstrap-admin pattern).
- **Two trust directions, two secrets** — `HERMENEA_OIKUMENEA_TOKEN` (import) and
  `OIKUMENEA_HERMENEA_TOKEN` (trigger), each scoped to one direction to bound blast radius. On the
  oikumenea side both flow through install config with directional names —
  `hermenea.inbound-token` (hermenea→oikumenea import) and `hermenea.outbound-token`
  (oikumenea→hermenea trigger), ECV-encryptable — resolved in one place each
  (`config.Hermenea.ResolveInboundToken` / `ResolveOutboundToken`), which still honour the two env
  vars above as overrides for the cross-service compose contract (review R-16). The `cmd/hermenea`
  companion reads the same two env vars directly.

## Patterns

- **Fetcher seam** — `Fetcher.Fetch(ctx, source) → RawBatch`; HTTP(S) + the degenerate `file`
  fetcher. New source types (DS-44) are new `Fetcher` implementations, not new call sites.

  > **Naming (M53 / D-ConnectorPlane).** This seam was `Connector` through M52. The connector plane
  > gives "connector" a higher meaning in the core — a whole deployable agent, of which hermenea is
  > one — so hermenea's *fetch strategy* is a **`Fetcher`**. The names **at rest did not move**:
  > the column `import_sources.connector_type`, the install key `connector-type:`, and the Conjure
  > field `connectorType` are unchanged (renaming them would break every deployment's config for no
  > benefit), so `ConnectorType: s.FetcherType` in the adapters is correct, not a leftover.
- **Live multi-file transform** (`http-files`, D-Languages M18) — a `StreamingFetcher` whose
  `locator` is a whitespace-separated **URL list**, each streamed to a staged temp directory by basename
  (no 16 MiB cap; a descriptive User-Agent avoids upstream 403s). The paired `PagedMapper` transforms
  the raw upstream in Go and emits a **single page** (whole forest) so a multi-file source that needs
  one transaction works. Used for languages: `glottolog` CLDF (`languages.csv` + `values.csv`) and CLDR
  (`supplementalData.xml` + `iso-639-3.tab`) are fetched fresh from upstream master each run — the live
  Go port of `deploy/language-presets/gen-presets.py` (which remains the offline/`file` fallback).
- **Streaming fetcher + paged mapper** (D-GeoPlaces) — for sources too large for a single in-memory
  batch, a `StreamingFetcher.Stage(...) → StagedSource` lands the body to disk (the `wof-sqlite`
  fetcher fetches a `.db.bz2`, bzip2-decompresses, stages a temp SQLite file), and a
  `PagedMapper.MapPaged(staged, emit)` walks it **parent-first** emitting bounded pages, each loaded as
  its own canonical envelope (one `import_runs` row aggregates the page counts). The 16 MiB cap and the
  in-memory `Fetch`/`Map` path are untouched for small http/file sources.
- **Chunked runs + resume cursor** (R-05 / M49) — the loader sends any dataset larger than one chunk
  (`oikumenea.chunk-size`, default 5000) as a **chunked run**: sequential `(runId, seq, isLast)`
  envelopes, one oikumenea transaction each, a trailing empty finalize chunk, and a **finite**
  per-request deadline (`oikumenea.http-timeout-ms`, default 120 s — the old
  `WithHTTPTimeout(0)`/whole-dataset POST is retired). After every acknowledged chunk the worker
  persists `worker_jobs.resume_seq` + `resume_checksum`; a retried attempt (crash of either side,
  job-timeout overrun) re-stages the source and **skips already-acked chunks** while the staged
  checksum still matches — a changed source resets the cursor (full, still-idempotent re-run).
  oikumenea stays stateless per chunk; chunk replay is safe by natural-key idempotency.
- **Mapper registry** — per `object_type`, mirrors the oikumenea upsert registry (which mirrors
  `pkg/events.Bus`): raw records → canonical envelope (or paged via `RegisterPagedMapper`). Adding an
  import target = registering a mapper (hermenea) + an upsert handler (oikumenea); no framework change.
- **Idempotent re-sync** — code-keyed upsert; re-running a sync over unchanged data is a no-op
  (created/updated/skipped reported). Ingest ≠ audited edit: a bulk import is one `system` Action, not
  N user edits.
- **Queue claim** — `FOR UPDATE SKIP LOCKED` for at-least-once concurrency without a broker;
  `worker.concurrency` fans out N claim loops in one process (R-13 / M49; default 1), and the same
  claim is replica-safe across processes; exponential backoff with **per-job-type** config;
  dead-letter after `max_attempts`.

## Invariants & safety

- **No direct DB coupling** — hermenea writes oikumenea **only** through `POST /import/{objectType}`.
  This is the load-bearing separation invariant; a hermenea DB connection to oikumenea is a defect.
- **Idempotent + non-destructive** — the import upsert never deletes a target row; a mismatch is
  reported, not destructive. Enqueue dedupes on `idempotency_key`; execution is at-least-once, so
  handlers must tolerate re-delivery.
- **Graceful drain** — in-flight jobs finish (or re-queue) on shutdown; witchcraft-managed lifecycle.
- **Provenance everywhere** — every imported oikumenea row carries `(source, source_version,
  imported_at)`; every map+load has an `import_runs` row.
- **Failures surface** — a failed fetch/map/load lands in `last_error` + a job-health reporter
  (hermenea) and, for the load step, an oikumenea `system`-actor audit entry.

## Open seams / future

- **More connector types** (`jdbc-sql`, `object-store`) — **DS-44**, new `Connector` impls.
- **First real connector** is the **Who's-On-First geo gazetteer** (M16, D-GeoPlaces): the `wof-sqlite`
  streaming connector + the `geo-places` paged mapper load country/region/county/locality globally
  (rolled out per country, Ukraine first). `geo-countries` (ISO-3166) is the simpler proving consumer —
  an in-memory `geocountries.Mapper` over the degenerate `file` connector, fed by the bundled
  `deploy/geo-presets/iso-3166.json` source (seeded in `var/conf/hermenea-install.yml`), verified
  end-to-end in compose. **M18** (Glottolog languages) is the next over the HTTP connector.
- **WOF out-of-distribution parents (resolved).** WOF parents some places to an ancestor outside the
  per-country dist — e.g. **Crimea** carries a `country_id` not present in the Ukraine admin dist, which
  failed the whole region page on `geo_places.parent_id` RESTRICT (SQLSTATE 23503). The paged mapper now
  tracks the `wof_id`s it has emitted and **omits a `parentId` whose target isn't in the imported set**,
  so such a place lands top-level (NULL parent) rather than aborting the page; its own descendants still
  attach beneath it. The full UA gazetteer (35k places) loads end-to-end; oikumenea keeps the RESTRICT FK
  as defence in depth.
- **Multi-process / leader election** — out of scope; hermenea is single-process (the broker question
  stays **DS-26**).
- **Other DS-25 beneficiaries** (audit-retention partitioning DS-28, future-dated order effects, expiry
  sweeps) can run as hermenea jobs later but call oikumenea over HTTP, not the DB.
