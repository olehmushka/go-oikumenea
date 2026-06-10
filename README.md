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

Other useful Atlas commands (all need `.env` sourced first):

```bash
atlas migrate status --env local         # show applied vs pending migrations
atlas migrate hash   --env local         # refresh migrations/atlas.sum after editing a migration
atlas migrate lint   --env local         # destructive-change gate (uses an ephemeral Docker dev DB)
```

> Adding a migration: write `migrations/<timestamp>_<name>.sql` (pure DDL, ending with the
> `schema_version` marker `UPDATE`), bump `ExpectedSchemaRevision` in
> `internal/platform/db/schemaversion.go`, then run `atlas migrate hash --env local`.

### 4. Run the server

```bash
./godelw run                             # or: go run ./cmd/oikumenea serve
```

Readiness is served on the management port and goes green only when the DB schema matches the
binary's expected revision:

```bash
curl -sk https://localhost:8444/status/readiness
# {"ready":true,"schemaRevision":"0006_person","expectedSchemaRevision":"0006_person"}
```

### 5. Tests

```bash
go test ./...                                            # unit tests
./scripts/setup-test-db.sh                               # create + migrate the dedicated test DB (once)
set -a; . ./.env; set +a
go test -tags=integration ./internal/...                 # integration tests (need a migrated DB)
```

The integration tests run against a **dedicated `oikumenea_test` database** (`$OIKUMENEA_TEST_DSN`),
separate from the dev/operator DB (`$DATABASE_URL`), so a test run never pollutes the data the
running server + web console read. `setup-test-db.sh` is idempotent; pass `--reset` to rebuild the
test DB from scratch after editing a migration.

## Client SDK & API reference

The API is Conjure-first (`api/*.conjure.yml`). A typed **Go client SDK** is generated from that same
contract into the nested module [`client/`](client/README.md) — `go get
github.com/olegamysk/go-oikumenea/client`. An **OpenAPI** reference is generated from the same IR in CI
(see [`docs/api/README.md`](docs/api/README.md)). Both derive from one contract, so they cannot drift
from the server.

## Web UI (optional)

An optional **Next.js admin console** ([`web/`](web/README.md), [`docs/web-ui.md`](docs/web-ui.md))
runs on **port 8445** beside the API. It is opt-in and a pure API consumer (a Backend-for-Frontend
with Keycloak login) — the server is unchanged whether or not it runs.

```bash
# local dev (with the dev Postgres + Keycloak + server already up — see deploy/keycloak/README.md):
cd web && cp .env.example .env.local && npm install && npm run dev   # http://localhost:8445

# or production-shaped, opt-in via the `ui` compose profile (default `up` does NOT start it):
docker compose --profile ui up --build
```
