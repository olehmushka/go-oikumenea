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

### 2. Start Postgres

```bash
docker compose -f docker-compose.dev.yml up -d
```

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
set -a; . ./.env; set +a
go test -tags=integration ./internal/...                 # integration tests (need a migrated DB)
```
