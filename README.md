# go-oikumenea

A generic, domain-agnostic **personnel & authorization service** (Keycloak-like, for hierarchical
multi-tenant organizations). API-only, self-hosted, operator-owned PostgreSQL, built on the Palantir
OSS stack (witchcraft / conjure / gödel).

The architecture specification is the source of truth — start at [`docs/README.md`](docs/README.md).

## Local development

### Prerequisites

- Go (see `go.mod`) and the bundled gödel wrapper (`./godelw`)
- Docker (for the local Postgres and Atlas's ephemeral lint/dev DB)
- [Atlas](https://atlasgo.io) CLI (`atlas`) for migrations

### 1. Configure the environment

The **server** reads its config from `var/conf/install.yml` (witchcraft). The out-of-process
**tooling** (Atlas, docker compose, integration tests) reads a gitignored `.env`:

```bash
cp .env.example .env        # tweak ports/DSNs for your machine if 5432 is taken
```

Key variables (see `.env.example`): `DATABASE_URL` (Atlas target), `OIKUMENEA_TEST_DSN`
(integration tests), `OIKU_DB_HOST_PORT` (compose host port), `OIKU_ENVIRONMENT`.

### 2. Start Postgres (and Keycloak)

```bash
docker compose -f docker-compose.dev.yml up -d
```

This brings up Postgres **and a local Keycloak** (a real OIDC IdP for manual auth testing, on
`:8080`, with a realm auto-imported). For the full hands-on login/token recipe — minting a token and
calling the API as the bootstrapped admin — see [`deploy/keycloak/README.md`](deploy/keycloak/README.md).

### 3. Run migrations

Migrations are **versioned** Atlas migrations in the repo-root [`migrations/`](migrations/) dir
(one schema `oikumenea`, expand-only, non-destructive — see
[`docs/architecture/upgrade-safety.md`](docs/architecture/upgrade-safety.md)). The `atlas.hcl` env
`local` targets `$DATABASE_URL`.

```bash
set -a; . ./.env; set +a                 # export DATABASE_URL (and friends) from .env
atlas migrate apply --env local          # apply all pending migrations to the local DB
```

The migrations create the non-superuser `oikumenea_app` group role (RLS backstop, D-RLSDefenseInDepth);
the **server** connects as a login member of it, so create that role **once** after the first apply:

```bash
psql "$DATABASE_URL" -c "CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;"
```

Other useful Atlas commands (all need `.env` sourced first):

```bash
atlas migrate status --env local         # show applied vs pending migrations
atlas migrate hash   --env local         # refresh migrations/atlas.sum after editing a migration
atlas migrate lint   --env local         # destructive-change gate (uses an ephemeral Docker dev DB)
```

> Adding a migration: write `migrations/<timestamp>_<name>.sql` (pure DDL, ending with the
> `schema_version` marker `UPDATE`), bump `ExpectedSchemaRevision` in
> `internal/platform/db/schemaversion.go`, then run `atlas migrate hash --env local`.

### 4. Run the server (oikumenea)

```bash
./godelw run                             # or: go run ./cmd/oikumenea serve
```

Readiness is served on the management port and goes green only when the DB schema matches the
binary's expected revision:

```bash
curl -sk https://localhost:8444/status/readiness
# {"ready":true,"schemaRevision":"0017_import_provenance","expectedSchemaRevision":"0017_import_provenance"}
```

The server reads its config from `var/conf/install.yml` (DSN, IdP, bootstrap admin). The API listens on
`:8443` and the management/health port on `:8444`. If you plan to run the **hermenea** companion
(next section), also export `HERMENEA_OIKUMENEA_TOKEN` before starting the server so it accepts
hermenea's import calls — see section 5.

### 5. Run the hermenea companion service (no Docker)

[hermenea](docs/modules/hermenea.md) (**M16 / D-Hermenea**) is a **second binary** (`cmd/hermenea`)
that ingests external reference datasets and loads them into oikumenea over HTTP. It has its **own
PostgreSQL database** and its **own Atlas migrations** (`migrations/hermenea/`), and couples to
oikumenea **only** through the public `POST /import/{objectType}` endpoint — it never touches
oikumenea's database. Run it as a plain host process beside the server (no container needed); the only
shared infrastructure is the same local Postgres.

Two runtime shared secrets (env vars, **not** install config) bound the trust in each direction:
`HERMENEA_OIKUMENEA_TOKEN` authorizes hermenea's import calls *into* oikumenea (the `hermenea-importer`
principal), and `OIKUMENEA_HERMENEA_TOKEN` authenticates push triggers *into* hermenea's `/sync`. They
ship as insecure local defaults in `.env.example`.

**5a. Create + migrate hermenea's own database** (one-time):

```bash
set -a; . ./.env; set +a                                   # exports HERMENEA_DATABASE_URL + the tokens
psql "$DATABASE_URL" -c "CREATE DATABASE hermenea;"        # its OWN DB, separate from oikumenea's
atlas migrate apply --env hermenea                         # applies migrations/hermenea ($HERMENEA_DATABASE_URL)
```

> **If your oikumenea dev DB predates M16**, the import targets live in the bootstrap migration
> (`oikumenea.geo_places` + the PostGIS extension + `geo_countries` WOF columns), which Atlas will not
> re-apply to an already-bootstrapped DB — geo-places imports then 500 with `relation
> "oikumenea.geo_places" does not exist`. Rebuild the dev schema once (drops app data, re-runs every
> migration, re-seeds the admin — D-Bootstrap): `./scripts/reset-dev-db.sh`.

**5b. Start oikumenea with the importer secret** (terminal A) — the server validates hermenea's import
calls against this exact value:

```bash
set -a; . ./.env; set +a
export HERMENEA_OIKUMENEA_TOKEN                            # from .env; the server reads it at boot
./godelw run                                               # or: go run ./cmd/oikumenea serve
```

**5c. Start hermenea** (terminal B) — run it from the **repo root** (it reads
`var/conf/hermenea-install.yml` + bundled presets by relative path). It needs **both** secrets — the
importer token must match the server's:

```bash
set -a; . ./.env; set +a
go run ./cmd/hermenea                                      # serves https://localhost:9443 (+ :9444 mgmt)
curl -sk https://localhost:9444/status/readiness          # -> {}  (ready)
```

`var/conf/hermenea-install.yml` points hermenea at its DB (`…/hermenea`) and at oikumenea
(`https://localhost:8443`, `insecure-skip-verify: true` for the self-signed dev cert), and declares two
import sources, both seeded idempotently at boot:

- `geo-countries-iso3166` — the bundled `deploy/geo-presets/iso-3166.json` over the degenerate **file**
  connector → `oikumenea.geo_countries`. Network-free; the quickest thing to verify.
- `wof-geo-ua` — the **Who's-On-First** Ukraine admin gazetteer over the **wof-sqlite** streaming
  connector → `oikumenea.geo_places` (PostGIS). It is **enabled with a weekly cron**, so the scheduler
  fires it on boot and downloads a ~62 MB `.db.bz2` (the full Ukraine backfill is ~35k places). Set its
  `enabled: false` in `var/conf/hermenea-install.yml` if you only want the quick geo-countries check.

**5d. Trigger a sync on demand** (the push trigger) and watch the lineage:

```bash
# enqueue the bundled geo-countries sync (base path /hermenea/v1; bearer = OIKUMENEA_HERMENEA_TOKEN)
curl -sk -X POST https://localhost:9443/hermenea/v1/sync/geo-countries-iso3166 \
  -H "Authorization: Bearer $OIKUMENEA_HERMENEA_TOKEN"

# inspect the run lineage / registered sources / worker jobs
curl -sk https://localhost:9443/hermenea/v1/runs    -H "Authorization: Bearer $OIKUMENEA_HERMENEA_TOKEN"
curl -sk https://localhost:9443/hermenea/v1/sources -H "Authorization: Bearer $OIKUMENEA_HERMENEA_TOKEN"

# confirm it landed in oikumenea (provenance stamped on imported rows)
psql "$DATABASE_URL" -c \
  "SELECT code, name, source, source_version FROM oikumenea.geo_countries WHERE source IS NOT NULL LIMIT 5;"
```

A re-trigger over unchanged data is an idempotent no-op (the run reports all-`skipped`); a missing/bad
bearer is rejected `401`. To exercise the WOF path, trigger `wof-geo-ua` the same way and watch
`oikumenea.geo_places` populate (`SELECT placetype, count(*) FROM oikumenea.geo_places GROUP BY 1;`).

> Ports at a glance (running from source, as above): oikumenea `:8443` API / `:8444` mgmt;
> hermenea `:9443` API / `:9444` mgmt.
>
> In the packaged `docker-compose.yml` topology this differs: oikumenea publishes **no host port**
> (M52 / D-HeadlessTopology) and is reachable only from inside the compose network. The console
> facade `:8445` is the public entry point.

### 6. Tests

```bash
go test ./...                                            # unit tests
./scripts/setup-test-db.sh                               # create + migrate the dedicated test DBs (once)
set -a; . ./.env; set +a
go test -tags=integration ./internal/...                 # integration tests (need migrated DBs)
```

The integration tests run against **dedicated test databases** — `oikumenea_test` (`$OIKUMENEA_TEST_DSN`)
and, for the hermenea companion, `hermenea_test` (`$HERMENEA_TEST_DSN`) — both separate from the
dev/operator DBs (`$DATABASE_URL` / `$HERMENEA_DATABASE_URL`), so a test run never pollutes the data the
running server + web console read. `setup-test-db.sh` provisions **both** test DBs (oikumenea's full
schema + `migrations/hermenea`); it is idempotent — pass `--reset` to rebuild them from scratch after
editing a migration.

## Client SDK & API reference

The API is Conjure-first (`api/*.conjure.yml`). Two typed client SDKs are generated from that same
contract: a **Go SDK** in the nested module [`clients/go/`](clients/go/README.md) — `go get
github.com/olegamysk/go-oikumenea/clients/go` — and a **TypeScript SDK** in
[`clients/typescript/`](clients/typescript/README.md) (D-ClientSDK). Each adds a one-call façade
(`client.New` / `createOikumeneaClient`) over every service, including the `hermenea` endpoints
oikumenea proxies. An **OpenAPI** reference is generated from the same IR in CI (see
[`docs/api/README.md`](docs/api/README.md)). All derive from one contract, so they cannot drift from
the server.

## Web UI (optional)

An optional **Next.js admin console** ([`web/`](web/README.md), [`docs/web-ui.md`](docs/web-ui.md))
runs on **port 8445**. It is opt-in and a pure API consumer (a Backend-for-Frontend with Keycloak
login) — the server is unchanged whether or not it runs.

Its server tier is **console-bff**, the first facade of the headless topology (M52 /
D-HeadlessTopology): in the packaged compose topology it is the only public port, forwarding the
end-user's own token to an oikumenea that is off the public network. It is unprivileged — it holds no
credential that widens access and makes no on-behalf-of assertion.

```bash
# local dev (with the dev Postgres + Keycloak + server already up — see deploy/keycloak/README.md):
cd web && cp .env.example .env.local && npm install && npm run dev   # http://localhost:8445

# or production-shaped, opt-in via the `ui` compose profile (default `up` does NOT start it):
docker compose --profile ui up --build
```
