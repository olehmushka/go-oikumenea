# Contributing to go-oikumenea

Thanks for hacking on go-oikumenea! This guide covers the development workflow. For what the service
*is* and how to run it, see the [README](README.md); for the architecture, start at
[`docs/README.md`](docs/README.md).

**The docs are the source of truth.** [`docs/architecture/decisions.md`](docs/architecture/decisions.md)
is **binding**: if code and a recorded decision disagree, the code is wrong — change the decision (with
rationale) in that file, don't diverge in code. Every feature moves through the fixed pipeline in
[`docs/development-process.md`](docs/development-process.md) (idea → decided → designed → backend →
migrated → ui → verified), tracked on the [stage board](docs/milestones.md#stage-board).

---

## Prerequisites

- [Go](https://go.dev) — version pinned in [`go.mod`](go.mod); the bundled gödel wrapper (`./godelw`)
  needs no separate install.
- [Docker](https://docs.docker.com/get-docker/) — for the local Postgres/Keycloak and Atlas's ephemeral
  lint DB.
- [Atlas CLI](https://atlasgo.io/getting-started#installation) — migrations.
- `make`, `psql`. Node.js only for the optional web console.

## Environment

```bash
cp .env.example .env          # gitignored; local-dev defaults
```

`.env` feeds the out-of-process tooling (Atlas via `$DATABASE_URL`, docker compose, the integration
tests via `$OIKUMENEA_TEST_DSN`) and is exported to `make` recipes. The **server** also auto-loads
`.env` at boot but only consumes `OIKUMENEA_*` / `HERMENEA_*` variables (see
[`docs/reference/env-vars.md`](docs/reference/env-vars.md)). Every variable is documented inline in
[`.env.example`](.env.example).

Bring up the stack and database:

```bash
make dev-up                   # Postgres + Keycloak
make migrate                  # apply oikumenea migrations
psql "$DATABASE_URL" -c "CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;"
```

> The migrations create the non-superuser `oikumenea_app` group role (RLS backstop,
> D-RLSDefenseInDepth); the server connects as a login member of it, so create that login role once
> after the first `make migrate`.

## The development loop

| Step | Command | Notes |
| --- | --- | --- |
| Format | `make format` | gofmt + imports, via gödel |
| Lint | `make lint` | golangci-lint, via gödel |
| Unit tests | `make test` | |
| Everything CI runs | `make verify` | format-check + lint + test + license headers |
| Run the server | `make run` | API `:8443`, management `:8444` |

`make help` lists all targets.

## Database & migrations

Migrations are **versioned** Atlas migrations in the repo-root [`migrations/`](migrations/) dir (one
schema `oikumenea`, **expand-only, non-destructive** — see
[`docs/architecture/upgrade-safety.md`](docs/architecture/upgrade-safety.md)). `migrations/hermenea/`
holds the companion's separate set.

```bash
make migrate            # apply pending oikumenea migrations (Atlas env local)
make migrate-hermenea   # apply pending hermenea migrations (Atlas env hermenea)
make migrate-status     # applied vs pending
make migrate-lint       # destructive-change gate (ephemeral Docker dev DB)
make db-reset           # drop app data, re-apply every migration, re-seed the admin
```

**Adding a migration:**

1. Write `migrations/<timestamp>_<name>.sql` — pure DDL, ending with the `schema_version` marker
   `UPDATE`.
2. Bump `ExpectedSchemaRevision` in `internal/platform/db/schemaversion.go`.
3. `atlas migrate hash --env local` to refresh `migrations/atlas.sum`.
4. `make migrate-lint` to check the destructive-change gate.

## Code generation

The API is **Conjure-first** (`api/*.conjure.yml`) — generated code is **never hand-edited**.

```bash
make generate     # regenerate Go server/client code (internal/conjure) from the Conjure contract
make openapi      # regenerate the OpenAPI reference (docs/api/openapi/openapi.json)
make sdk          # regenerate the TypeScript SDK (clients/typescript)
```

## Testing

Unit tests (`make test`) need nothing external. **Integration tests** run against **dedicated test
databases** (`oikumenea_test` / `hermenea_test`), kept separate from the dev DB so a test run never
pollutes operator data:

```bash
make test-db                                  # create + migrate the test DBs (idempotent; once)
make test-integration                         # go test -tags=integration ./internal/...
```

`make test-db` wraps `./scripts/setup-test-db.sh` — pass `--reset` to it directly to rebuild the test
DBs after editing a migration.

## Running the hermenea companion

[hermenea](docs/modules/hermenea.md) (M16 / D-Hermenea) is a **second binary** (`cmd/hermenea`) that
ingests external reference datasets and loads them into oikumenea over HTTP (`POST /import/{objectType}`)
— it has its **own database** and **own migrations** and never touches oikumenea's DB. Two runtime
shared secrets (env vars, not install config) bound the trust in each direction:
`HERMENEA_OIKUMENEA_TOKEN` authorizes hermenea's import calls *into* oikumenea (the `hermenea-importer`
principal); `OIKUMENEA_HERMENEA_TOKEN` authenticates push triggers *into* hermenea's `/sync`. Both ship
as insecure local defaults in `.env`.

**One-time — create and migrate hermenea's own database:**

```bash
psql "$DATABASE_URL" -c "CREATE DATABASE hermenea;"
make migrate-hermenea
```

**Run both** (the importer token must match on both processes; `.env` provides it):

```bash
make run                # terminal A — oikumenea (reads the tokens from .env at boot)
make run-hermenea       # terminal B — hermenea, from the repo root (reads var/conf/hermenea-install.yml)
curl -sk https://localhost:9444/status/readiness   # -> {}  (ready)
```

**Trigger a sync and watch the lineage** (base path `/hermenea/v1`; bearer = `OIKUMENEA_HERMENEA_TOKEN`):

```bash
curl -sk -X POST https://localhost:9443/hermenea/v1/sync/geo-countries-iso3166 \
  -H "Authorization: Bearer $OIKUMENEA_HERMENEA_TOKEN"
curl -sk https://localhost:9443/hermenea/v1/runs -H "Authorization: Bearer $OIKUMENEA_HERMENEA_TOKEN"

# confirm it landed in oikumenea (provenance stamped on imported rows)
psql "$DATABASE_URL" -c \
  "SELECT code, name, source FROM oikumenea.geo_countries WHERE source IS NOT NULL LIMIT 5;"
```

`var/conf/hermenea-install.yml` declares the seeded import sources. `geo-countries-iso3166` (the
bundled ISO-3166 file → `geo_countries`) is network-free and the quickest check; `wof-geo-ua` (the
Who's-On-First Ukraine gazetteer → PostGIS `geo_places`) downloads ~62 MB on its weekly cron — set its
`enabled: false` to skip it.

> If your oikumenea dev DB predates M16, the geo import targets live in the bootstrap migration and
> won't be re-applied to an already-bootstrapped DB (imports then 500 with `relation
> "oikumenea.geo_places" does not exist`). Rebuild the dev schema once with `make db-reset`.

## Conventions

- **Read before doing:** [`docs/glossary.md`](docs/glossary.md),
  [`docs/architecture/conventions.md`](docs/architecture/conventions.md) (schema / Go / Conjure / API
  conventions), and the relevant [`docs/modules/*.md`](docs/modules/).
- Modules are **hexagonal**: `transport → application → domain → adapters`; the domain owns its
  interfaces and imports no framework. Cross-module **queries** are direct interface calls; cross-module
  **mutations** are domain events.
- Every structural entity has a stable, locale-agnostic **`code`** separate from its translatable
  **`name`**; translatable labels are returned as a `locale → text` map (no Accept-Language
  negotiation).
- Conjure- and sqlc-generated code is never hand-edited.
- **Docs coherence** is the analog of tests for the docs. After editing docs, check links resolve:

  ```bash
  cd docs && python3 - <<'PY'
  import re,os,glob
  bad=0
  for md in glob.glob('**/*.md',recursive=True):
      base=os.path.dirname(md)
      for m in re.finditer(r'\]\(([^)]+)\)',open(md).read()):
          link=m.group(1).split('#')[0]
          if link and not link.startswith('http') and not os.path.exists(os.path.normpath(os.path.join(base,link))):
              print(f"broken: {md} -> {link}"); bad+=1
  print("links OK" if bad==0 else f"{bad} broken")
  PY
  ```

## Submitting changes

1. Branch off `main`.
2. Make your change; keep the docs in step (update `decisions.md` if it's a decision, `glossary.md` /
   `README.md` module map when entities move, and the [stage board](docs/milestones.md#stage-board)
   when a milestone advances a gate).
3. Run `make verify` (and `make test-integration` if you touched DB-backed code) — it must be green.
4. Open a PR with a clear description; reference the relevant `D-<Name>` decision or milestone.

## Licensing

The software is **Apache-2.0** ([`LICENSE`](LICENSE)). By submitting a contribution you agree it is
licensed under those terms — this is the inbound-equals-outbound rule of Apache-2.0 §5; there is no
separate CLA.

- **New source files need the SPDX header.** Run `./godelw license` and it stamps them; `make verify`
  fails without it. Generated trees (`internal/conjure`, sqlc output, `clients/go/oikumenea`,
  `clients/typescript`) are excluded in
  [`godel/config/license-plugin.yml`](godel/config/license-plugin.yml) — never hand-stamp them, the
  next regeneration would strip the header and break the build.
- **Adding a `pinax` preset carries obligations.** The presets are `go:embed`-ed, so their upstream
  license ships with every binary. Set the `license:` front-matter field, add the row to
  [`docs/reference/data-licenses.md`](docs/reference/data-licenses.md), and put any required
  attribution in [`NOTICE`](NOTICE). `TestPresetLicensesAreDocumented` fails the build if the doc
  falls out of step. **Do not add a dataset whose terms you have not read** — two of the current
  presets are share-alike, and that reaches downstream redistributors.
