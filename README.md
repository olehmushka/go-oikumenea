# go-oikumenea

A generic, domain-agnostic **personnel directory + multi-domain registry / authorization service**
for hierarchical, multi-tenant organizations (army, church, university, government, company). Think
"Keycloak, but with a real policy-decision point over an org graph, and rich per-domain registries" —
API-only, self-hosted, on operator-owned PostgreSQL, built on the Palantir OSS stack (witchcraft /
conjure / gödel).

- **Authorization plane, not an IdP.** Authentication is delegated to your external IdP; oikumenea
  validates inbound identities and **decides** authorization (a PDP). It never stores credentials or
  issues tokens.
- **A PDP over a unit DAG.** Units can have multiple parents; role assignments are scoped
  `unit | subtree`; reads are trimmed by public/shadow visibility.
- **One directory, many organizations.** Multiple domains (military/university/…) and organizations
  (US Army, Bundeswehr, KhNU) share one instance-global person directory.
- **Rich registries behind per-code permissions** — documents, finance, watchlist/sanctions matches,
  physical identity, addresses, and more, each gated by its own permission.

The architecture specification is the **source of truth** — start at [`docs/README.md`](docs/README.md).

---

## Quick start

**Prerequisites:** [Go](https://go.dev) (version in [`go.mod`](go.mod)), [Docker](https://docs.docker.com/get-docker/),
the [Atlas CLI](https://atlasgo.io/getting-started#installation), and `make`. The bundled gödel
wrapper (`./godelw`) needs no install. (Node.js is only needed for the optional web console.)

```bash
git clone https://github.com/olegamysk/go-oikumenea && cd go-oikumenea
cp .env.example .env          # local defaults; tweak ports/DSNs if 5432 is taken

make dev-up                   # start Postgres + a local Keycloak (OIDC IdP) in Docker
make migrate                  # apply database migrations

# create the non-superuser login role the server connects as (one-time, RLS backstop)
psql "$DATABASE_URL" -c "CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;"

make run                      # start the server: API on :8443, management on :8444
```

Confirm it is healthy (readiness goes green only when the DB schema matches the binary):

```bash
curl -sk https://localhost:8444/status/readiness
# {"ready":true,"schemaRevision":"…","expectedSchemaRevision":"…"}
```

Run `make help` at any time to see every task.

> **Prefer containers?** `make docker-up` builds and starts the packaged stack from
> [`docker-compose.yml`](docker-compose.yml) instead. In that topology the core publishes **no** host
> port (it sits behind the console facade on `:8445`, M52 / D-HeadlessTopology).

---

## Configuration

The server is configured by **environment variables**, a YAML file, or both — whichever you prefer
(D-EnvConfig):

- **Environment variables** override everything and the **YAML file is optional**, so an env-only boot
  works (great for `docker run` and 12-factor deploys). Each field maps to an env var derived from its
  config path under the `OIKUMENEA_` prefix (`crypto.local-dev.kek` → `OIKUMENEA_CRYPTO_LOCAL_DEV_KEK`).
- **`var/conf/install.yml`** (witchcraft) holds the same settings as a file; secrets can be
  ECV-encrypted at rest.
- **Precedence:** real env `>` `.env` (auto-loaded at boot) `>` YAML `>` built-in defaults.

A minimal env-only boot:

```bash
export OIKUMENEA_ENVIRONMENT=local
export OIKUMENEA_DB_HOST=localhost OIKUMENEA_DB_USER=oikumenea OIKUMENEA_DB_PASSWORD=dev OIKUMENEA_DB_NAME=postgres
export OIKUMENEA_LOGGING_LEVEL=info
oikumenea serve
```

The database can be one `OIKUMENEA_POSTGRES_DSN` or the discrete
`OIKUMENEA_DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` parts. **The full, generated list of every
variable is [`docs/reference/env-vars.md`](docs/reference/env-vars.md).** The companion `hermenea`
service uses the same scheme under the `HERMENEA_` prefix.

---

## Common tasks

Everything runs through `make` (which delegates to `./godelw`, Atlas, and docker compose):

| Task | Command |
| --- | --- |
| Start / stop the dev stack (Postgres + Keycloak) | `make dev-up` / `make dev-down` |
| Apply migrations | `make migrate` (and `make migrate-hermenea`) |
| Run the server / the hermenea companion | `make run` / `make run-hermenea` |
| Build both binaries | `make build` |
| Build container images | `make docker-build` / `make docker-build-hermenea` |
| Format, lint, test | `make format` · `make lint` · `make test` |
| Full CI bundle | `make verify` |
| Reset the dev DB (clean + migrate + re-seed admin) | `make db-reset` |
| Web console (optional) | `make web-install && make web-dev` |

See `make help` for the complete list.

---

## Using the API

The API is delegated-auth: send a bearer token from your IdP. In local dev, mint one from the bundled
Keycloak and call the API as the seeded admin:

```bash
TOKEN=$(scripts/keycloak-token.sh)                       # OAuth2 password grant (default user: admin)
curl -sk https://localhost:8443/identity/v1/whoami -H "Authorization: Bearer $TOKEN"
```

The full hands-on login/token recipe is in [`deploy/keycloak/README.md`](deploy/keycloak/README.md).

**Contracts & SDKs.** The API is Conjure-first (`api/*.conjure.yml`). From that one contract come a
typed **Go SDK** ([`clients/go/`](clients/go/README.md)), a **TypeScript SDK**
([`clients/typescript/`](clients/typescript/README.md)), and an **OpenAPI** reference
([`docs/api/README.md`](docs/api/README.md)) — none can drift from the server.

**Data ingestion (`hermenea`).** An optional companion service ingests external reference datasets
(country codes, gazetteers, watchlists, …) and loads them into oikumenea over HTTP. It has its own
database and migrations and couples only through the public import endpoint. See
[`docs/modules/hermenea.md`](docs/modules/hermenea.md) and the run guide in
[`CONTRIBUTING.md`](CONTRIBUTING.md#running-the-hermenea-companion).

---

## Web console (optional)

An optional **Next.js admin console** ([`web/`](web/README.md), [`docs/web-ui.md`](docs/web-ui.md))
runs on **port 8445**. It is a pure API consumer — a Backend-for-Frontend with Keycloak login — so the
server is unchanged whether or not it runs. In the packaged compose topology its `console-bff` tier is
the only public port (M52 / D-HeadlessTopology): it forwards the end-user's own token to a core that is
off the public network, holding no credential that widens access.

```bash
make web-install && make web-dev                          # http://localhost:8445 (dev)
docker compose --profile ui up --build                    # production-shaped (opt-in profile)
```

---

## Project layout

| Path | What |
| --- | --- |
| [`cmd/`](cmd/) | The two binaries: `oikumenea` (core) and `hermenea` (ingestion companion) |
| [`internal/<module>/`](internal/) | The modules (tenant, person, authorization, …), hexagonal layering |
| [`api/`](api/) | Conjure API definitions (the contract) |
| [`migrations/`](migrations/) | Versioned Atlas migrations (one schema `oikumenea`; `migrations/hermenea/` for the companion) |
| [`pkg/`](pkg/) | Framework-free shared kernel (RID, crypto, config overlay, …) |
| [`clients/`](clients/) | Generated Go + TypeScript SDKs |
| [`web/`](web/) | Optional Next.js admin console |
| [`docs/`](docs/README.md) | **The source of truth** — architecture, decisions, module docs |

---

## Contributing

Development setup, the migration workflow, code generation, testing, and coding conventions are all in
**[`CONTRIBUTING.md`](CONTRIBUTING.md)**. In short: `make verify` runs what CI runs, and
[`docs/architecture/decisions.md`](docs/architecture/decisions.md) is **binding** — if the code and a
recorded decision disagree, the code is wrong.

## License

The **software** is licensed under the **Apache License 2.0** — see [`LICENSE`](LICENSE). Source
files carry an SPDX header (`./godelw license` applies them; `make verify` enforces them).

The **bundled reference data** is not. The `pinax` presets
([`internal/pinax/presets/`](internal/pinax/presets/)) are `go:embed`-ed into the binary and are
derived from third-party datasets under their own terms — **two of them are share-alike**
(`countries.yaml` is ODbL-1.0, `religions.yaml` is CC-BY-SA-4.0). Redistributing this software, a
build of it, or a database derived from it means complying with those terms as well as Apache-2.0.

- Required attribution: [`NOTICE`](NOTICE)
- Per-dataset obligations, and how to build without the share-alike data:
  [`docs/reference/data-licenses.md`](docs/reference/data-licenses.md)

Data fetched at runtime by [hermenea](docs/modules/hermenea.md) connectors (Who's On First,
Wikidata, INTERPOL, sanctions lists) is never compiled in and is the **operator's** licensing
decision — the INTERPOL and sanctions feeds in particular carry restrictions no license here grants.
