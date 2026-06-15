# Continue M16 — Hermenea: drive to the `verified` gate

## Context

M16 (the **hermenea** ingestion + scheduler companion, D-Hermenea — supersedes D-Worker, absorbs
M17) is **backend + migrated** and stops one gate short of done. Everything builds and unit-tests
pass; the full out-of-process pipeline already exists:

- **hermenea binary** (`cmd/hermenea`): connectors (`http`/`file`/`wof-sqlite` streaming), worker
  queue + cron scheduler (backoff, dead-letter, graceful drain — `internal/hermenea/runtime`),
  orchestrator (`fetch→stage→map→load` + `import_runs` lineage — `application/service.go`), HTTP
  loader, WOF paged mapper (`wof/mapper.go`), transport + shared-secret auth, config.
- **oikumenea `dataimport`**: generic `POST /import/{objectType}` (`api/import.conjure.yml`),
  `GeoCountriesHandler` + `GeoPlacesHandler` (idempotent, provenance-stamped, audited as `system`,
  country-enrichment), PostGIS `geo_places` + `geo_countries` schema, `migrations/hermenea/0001` +
  `20260601000017_import_provenance.sql`.

The stage board (`docs/milestones.md`) marks M16 `🚧 verifying` with "e2e verify pending". The gaps
blocking `verified`, all confirmed in the source:

1. **No integration tests** for `hermenea` or `dataimport` (every other module has a
   `*_integration_test.go`); the streaming WOF path and the upsert endpoint have only pure unit
   coverage.
2. **No docker-compose wiring** for hermenea — no second Postgres DB, no migrate-hermenea step, no
   service, no shared secrets on `app`+`hermenea`; it can't yet "boot, migrate, demo on its own".
3. **geo-countries mapper unregistered** in hermenea (`module.go`: "geo-countries lands in a
   follow-up") — its absence blocks the simplest network-free e2e (the `file` connector path).
4. Stage board / `CLAUDE.md` status not yet flipped to `verified`.

**Outcome:** close all four gaps, verify both fixture-based (CI-repeatable) **and** with a real WOF
Ukraine run (genuine demo), and record M16 as `verified`.

## Approach

### 1. geo-countries mapper + bundled file source (closes the "follow-up" gap)

- **New** `internal/hermenea/geocountries/mapper.go` — a `domain.Mapper` (`Map(raw) → []map[string]any`,
  signature at `internal/hermenea/domain/hermenea.go:125`). It decodes the `file`/`http` connector's
  payload (a canonical-envelope JSON `{records:[{code,name,alpha3,numeric}]}`) into records the
  `dataimport` `GeoCountriesHandler` already consumes (`code`,`name`). Mirror the `wof` package layout
  + a `mapper_test.go`.
- **Register** it in `internal/hermenea/module.go` beside the paged WOF mapper:
  `svc.RegisterMapper(geocountries.ObjectType, geocountries.Mapper{})`.
- **New bundled preset** `deploy/geo-presets/iso-3166.json` (opt-in asset, mirrors
  `deploy/rank-presets/`) — the canonical envelope for the seeded ISO-3166 set, so the `file`
  connector has a real local payload with no network.
- **Add a `file` source** `geo-countries-iso3166` (connector `file`, object-type `geo-countries`,
  locator the preset path) to `var/conf/hermenea-install.yml`, cron `@weekly`, enabled — this is the
  "simpler proving consumer" the M16 milestone text already names as first consumer.

### 2. Integration tests (fixture-based, `//go:build integration`)

Follow the existing harness exactly (DSN from env, `pdb.NewPool`, `t.Cleanup`) — pattern at
`internal/rank/rank_integration_test.go:1-55`.

- **New** `internal/dataimport/dataimport_integration_test.go` (against `oikumenea_test`, PostGIS):
  - geo-countries: import → `Created`; re-import unchanged → `Skipped` (idempotent); changed name →
    `Updated`; `geo_countries.source/source_version/imported_at` provenance stamped; one `system`-actor
    audit row per import (assert via the audit store).
  - geo-places: parent-first insert of a tiny country→region→locality set → upsert counts;
    `placetype=country` enriches the matching `geo_countries` row (`wof_id`, geometry) in the same tx;
    a forward parent reference fails the whole tx (RESTRICT); re-import same `source_version` → all
    `Skipped`.
- **New** `internal/hermenea/hermenea_integration_test.go` (against a new `hermenea_test` DB):
  - `SeedSource` → `TriggerSync` enqueues a `worker_jobs` row; a duplicate trigger in the same second
    folds to one job (idempotency key); `ProcessJob` with a **stub `domain.Loader`** (in-test fake,
    no oikumenea needed) runs `file`-connector geo-countries fetch→map→load and writes a succeeded
    `import_runs` row with counts; a stub loader returning an error finishes the run `failed` and the
    job reschedules with backoff.
  - WOF paged path: a small helper builds a tiny SQLite fixture (the `spr` + `geojson` tables the
    mapper reads — `wof/mapper.go:54`) for ~3 parent-first features, run `GeoPlacesMapper.MapPaged`
    through a capturing emit, assert parent-first record order + field mapping. (Pure-mapper unit
    coverage already exists in `wof/mapper_test.go`; this adds the real-SQLite `MapPaged` leg.)
- **Extend** `scripts/setup-test-db.sh` to also create + migrate a `hermenea_test` DB
  (`HERMENEA_TEST_DSN`, `atlas migrate apply` with `migrations/hermenea`) alongside `oikumenea_test`,
  reusing the existing create-if-absent + `--reset` plumbing.

### 3. docker-compose wiring (boot / migrate / demo on its own)

In **`docker-compose.yml`** (prod-shaped) add, mirroring the existing `postgres`/`migrate`/`app`
trio:
- a second database for hermenea (a `hermenea` DB on the same Postgres instance, created in
  `init-role`/a small init step, **or** the simplest: a `POSTGRES`-init `CREATE DATABASE hermenea`);
- a **`migrate-hermenea`** step (`arigaio/atlas` `migrate apply --dir file:///migrations/hermenea`
  against the `hermenea` DB);
- a **`hermenea`** service built from a new **`Dockerfile.hermenea`** (clone of `Dockerfile`, building
  `./cmd/hermenea`; CGO-free works — the WOF SQLite driver is `modernc.org/sqlite`), depending on
  `migrate-hermenea` + `app`, mounting `var/conf/hermenea-install.yml`, exposing 9443/9444;
- the **two shared secrets** as env on both services: `OIKUMENEA_HERMENEA_TOKEN` (trigger) +
  `HERMENEA_OIKUMENEA_TOKEN` (import), insecure local defaults, read by `cmd/hermenea/main.go:27-28`
  and the oikumenea push-trigger client / importer principal.

`docker-compose.dev.yml` already got the PostGIS image bump; add an optional `hermenea` DB note/DB
there too so the host-run binary has its DB.

### 4. Real WOF Ukraine run + record verification

- Bring the stack up, confirm the seeded `geo-countries-iso3166` `file` source syncs on boot/trigger
  (network-free), then flip on `wof-geo-ua` (already in `hermenea-install.yml`, raising
  `worker.job-timeout-ms`) to perform the real download → stage → paged load into `geo_places` +
  country enrichment. Capture `import_runs` lineage + provenance + a re-run no-op.
- **Update docs in the same pass:** `docs/milestones.md` stage board M16 → `✅ verified` (and the
  prose note); `docs/modules/hermenea.md` drop the "geo-countries follow-up" seam; `CLAUDE.md`
  repository-status line; ground every `✅` in a real artifact.

## Critical files

- New: `internal/hermenea/geocountries/mapper.go` (+ `mapper_test.go`),
  `deploy/geo-presets/iso-3166.json`, `Dockerfile.hermenea`,
  `internal/dataimport/dataimport_integration_test.go`,
  `internal/hermenea/hermenea_integration_test.go` (+ SQLite fixture helper).
- Edit: `internal/hermenea/module.go`, `var/conf/hermenea-install.yml`, `scripts/setup-test-db.sh`,
  `docker-compose.yml`, `docker-compose.dev.yml`, `docs/milestones.md`, `docs/modules/hermenea.md`,
  `CLAUDE.md`.
- Reuse (do **not** reimplement): `application.GeoCountriesHandler`/`GeoPlacesHandler`
  (`internal/dataimport/application/service.go`), `connector.Default()` (`file`/`wof-sqlite`),
  the `domain.Store`/`Loader`/`Mapper` interfaces, `pdb.NewPool`, the audit store/`Record` pattern.

## Verification

1. **Build + unit:** `go build ./...` && `go test ./internal/hermenea/... ./internal/dataimport/...`
   (all green, including the new geo-countries mapper test).
2. **Integration (fixture, CI-repeatable):**
   `./scripts/setup-test-db.sh --reset` (now provisions `hermenea_test` too), then
   `set -a; . ./.env; set +a; go test -tags=integration ./internal/dataimport/... ./internal/hermenea/...`
   — asserts idempotency, provenance, audit, country-enrichment, RESTRICT, lineage, backoff.
3. **Stack boot:** `docker compose up --build` → `app` (8444) + `hermenea` (9444) readiness green;
   migrations + migrate-hermenea complete; the `geo-countries-iso3166` `file` source loads on boot.
4. **Push trigger:** `POST https://localhost:9443/sync/geo-countries-iso3166` with
   `OIKUMENEA_HERMENEA_TOKEN` → returns a job id; `GET /runs` shows succeeded lineage; a re-run
   reports all-`Skipped` (idempotent); a bad token → 401.
5. **Real WOF demo:** enable `wof-geo-ua`, trigger, watch `geo_places` populate (country→…→locality)
   + `geo_countries` UA enriched with `wof_id`/geometry; `import_runs` aggregates page counts;
   re-trigger = no-op.
6. **Drain:** stop hermenea mid-job → in-flight job finishes/re-queues cleanly (graceful drain).
