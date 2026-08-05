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

## Setting up an identity provider (Docker)

go-oikumenea **never authenticates** — it validates a **JWT** somebody else issued and decides
authorization (L-AuthzOnly). So "setting up auth" means pointing it at an issuer you trust and telling
it which audience is yours. Everything below is env-var only: the packaged `app` service mounts
`deploy/install.docker.yml` **read-only**, so you configure issuers with `OIKUMENEA_*` variables
rather than editing a file inside the image (D-EnvConfig).

Full per-provider walkthroughs — including registering the OAuth app in Google/GitHub/Entra/GitLab/Okta
consoles — are in [`deploy/oauth/README.md`](deploy/oauth/README.md). This section is the Docker
deployment shape.

### First: which variation are you in?

```bash
curl -fsS https://YOUR-PROVIDER/.well-known/openid-configuration | jq '{issuer, jwks_uri}'
```

| Result | Variation |
| --- | --- |
| JSON with `issuer` + `jwks_uri` | **A** or **B** — your choice |
| 404 / HTML / error (e.g. **GitHub**) | **C** only — it is OAuth2 without OIDC, so nothing it issues is verifiable |

### Variation A — one OIDC provider, direct

The common case: Google, Entra, Okta, GitLab, Keycloak, Authentik… Two containers need to agree.

```yaml
# docker-compose.override.yml
services:
  app:
    environment:
      # `_0_` is the first idp.issuers[] entry; use _1_, _2_ … for more (variation B).
      OIKUMENEA_IDP_ISSUERS_0_ISSUER: "https://accounts.google.com"   # EXACTLY as discovery printed it
      OIKUMENEA_IDP_ISSUERS_0_TYPE: "oidc"
      OIKUMENEA_IDP_ISSUERS_0_AUDIENCE: "<client-id>"                 # MANDATORY — see below
      OIKUMENEA_IDP_ISSUERS_0_LABEL: "Google"                         # display name in the console

  console-bff:
    environment:
      AUTH_URL: "https://console.example.org"        # the PUBLIC origin browsers use
      AUTH_SECRET: "<openssl rand -base64 32>"
      AUTH_GOOGLE_ID: "<client-id>"                  # the SAME client id as the audience above
      AUTH_GOOGLE_SECRET: "<client-secret>"
```

Register the redirect URI **`<AUTH_URL>/api/auth/callback/google`** with the provider.

Named variables exist for `AUTH_KEYCLOAK_*`, `AUTH_GOOGLE_*`, `AUTH_ENTRA_*`, `AUTH_GITLAB_*` and
`AUTH_OKTA_*`. For anything else use a **generic slot** — no image rebuild, no code change:

```yaml
      AUTH_OIDC_ACME_ISSUER: "https://id.acme.example/application/o/oikumenea/"
      AUTH_OIDC_ACME_ID: "…"
      AUTH_OIDC_ACME_SECRET: "…"
      AUTH_OIDC_ACME_LABEL: "Acme SSO"     # callback: <AUTH_URL>/api/auth/callback/acme
```

**Two rules the deployment will enforce on you:**

- **`AUDIENCE` is mandatory and the container will refuse to start without it.** A public IdP's `iss`
  is shared by every application registered with it, so the audience is what binds a token to *your*
  deployment. Set it to the OAuth **client id**, not `oikumenea`.
- **The issuer string must equal the token's `iss` byte for byte** — it is the routing key. A trailing
  slash, `http` vs `https`, or a missing `/v2.0` (Entra) means every login 401s.

### Variation B — several providers at once

Add issuer entries by index; the console offers a button per provider whose credentials are present.

```yaml
  app:
    environment:
      OIKUMENEA_IDP_ISSUERS_0_ISSUER: "https://accounts.google.com"
      OIKUMENEA_IDP_ISSUERS_0_TYPE: "oidc"
      OIKUMENEA_IDP_ISSUERS_0_AUDIENCE: "<google-client-id>"
      OIKUMENEA_IDP_ISSUERS_0_LABEL: "Google"
      OIKUMENEA_IDP_ISSUERS_1_ISSUER: "https://login.microsoftonline.com/<tenant>/v2.0"
      OIKUMENEA_IDP_ISSUERS_1_TYPE: "oidc"
      OIKUMENEA_IDP_ISSUERS_1_AUDIENCE: "<entra-client-id>"
      OIKUMENEA_IDP_ISSUERS_1_LABEL: "Corporate Entra ID"
```

One person may hold **one login point per provider** on a single account, resolving to the same
directory record whichever they use.

> If one issuer serves several clients of the *same* deployment (a console **and** a CLI registered
> separately), it accepts a SET of audiences — but only the YAML file can express it (`audiences: [a, b]`).
> There is no `…_AUDIENCES` env var, because the env overlay binds only scalar fields of a list
> element. Mount your own `install.yml` over `/app/var/conf/install.yml` for that case.

### Variation C — brokered (and the only way to use GitHub)

Put an IdP that speaks OIDC in front (Keycloak, Authentik, Auth0…), federate the providers into it, and
point go-oikumenea at **that** issuer only. GitHub, Discord, Slack and friends reach you this way.

```yaml
  app:
    environment:
      OIKUMENEA_IDP_ISSUERS_0_ISSUER: "https://sso.example.org/realms/oikumenea"
      OIKUMENEA_IDP_ISSUERS_0_TYPE: "oidc"
      OIKUMENEA_IDP_ISSUERS_0_AUDIENCE: "oikumenea"
      OIKUMENEA_IDP_ISSUERS_0_LABEL: "Company SSO"
```

go-oikumenea needs **nothing** per upstream provider: the broker mints every token, so `iss` stays the
broker and `sub` is the **broker's** user id, never Google's or GitHub's. The dev stack ships a worked
example — `scripts/keycloak-brokers.sh` configures Google/GitHub/Entra/GitLab/Okta on the bundled
Keycloak realm from environment credentials.

### Then: enrolling people

A verified token still has to map to a **person**. Unknown identities are **rejected** — the service is
a personnel directory first, so a first login is a *link*, never a *create* (D-JIT).

- **Default — link explicitly.** The person signs in once and is refused; the log line
  `rejected unknown token identity` carries the `issuer`, `subject` and `email`, and an admin links it
  in the console (*Persons → the person → Account → Link external identity*).
- **When you only know people's email addresses**, invert it — create a login-less account carrying the
  address and let the first sign-in attach itself:

  ```yaml
        OIKUMENEA_IDP_JIT_ENABLED: "true"
        OIKUMENEA_IDP_JIT_CLAIM: "email"
        OIKUMENEA_IDP_JIT_MATCH: "account-email"
        OIKUMENEA_ACCOUNT_IDENTITY_LINKING_ENABLED: "false"   # optional: cap one login point per person
  ```

  It requires a **verified** email (`email_verified` true) and still never creates a person. Understand
  what it delegates: every issuer you configure becomes trusted to verify addresses honestly, so anyone
  proving that address at any of them reaches the account.

### Troubleshooting

| Symptom | Cause |
| --- | --- |
| Container exits: *"pins no audience"* | An `oidc` issuer has no `AUDIENCE`. Deliberate and fail-closed — see above. |
| Console logs in, then *"Your session has expired"* | That banner renders on **any 401**. Either the identity is not linked yet (expected — enrol it), or the audience/issuer does not match. |
| Every login 401s from the start | `iss` mismatch — compare the configured string with the discovery document character by character. |
| Provider button missing from the login page | Its `AUTH_<PROVIDER>_ID`/`_SECRET` are unset on **console-bff**. |
| `redirect_uri_mismatch` at the provider | `AUTH_URL` is not the public origin, or the callback path was not registered. |

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

**Signing in with Google, GitHub, Entra ID, GitLab or Okta** —
[`deploy/oauth/README.md`](deploy/oauth/README.md) has working recipes in two topologies: *brokered*
through Keycloak (one issuer, and the only route for GitHub, which is OAuth2 without OIDC and so
publishes nothing this service could verify) and *direct* (one `idp.issuers[]` entry per provider).

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
