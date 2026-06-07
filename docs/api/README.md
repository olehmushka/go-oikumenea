# API reference (OpenAPI) & client SDK

The API source of truth is the **Conjure IDL** under [`api/`](../../api) — one `*.conjure.yml` per
module (D-Conjure / D-Stack). The gödel `conjure-plugin` compiles those to IR and generates the Go
server interfaces + clients into `internal/conjure` (never hand-edited). Two consumable artifacts are
derived from that **same** contract, so neither can drift from the running server:

- a **Go client SDK** — the publishable nested module [`client/`](../../client) (see below);
- an **OpenAPI v3** document — the human-facing rendering (see *Generating the OpenAPI documents*).

## Go client SDK

A typed Go client is generated client-only into the nested module
[`client/`](../../client/README.md) (module path `github.com/olegamysk/go-oikumenea/client`) by a second
`conjure-plugin` project (`godel/config/conjure-plugin.yml`). External code imports it directly:

```bash
go get github.com/olegamysk/go-oikumenea/client@latest
```

```go
hc, _ := client.Dial("https://host:8443")             // + client.WithInsecureSkipVerify() for dev
who, _ := identityfederation.NewIdentityFederationServiceClient(hc).Whoami(ctx, token)
```

The `internal/conjure` copy stays for in-repo use (it is import-restricted); the `client/` module is the
public one. Full usage + auth notes in [`client/README.md`](../../client/README.md).

## Conjure sources

| Module | Source | Base path |
|---|---|---|
| audit | [`api/audit.conjure.yml`](../../api/audit.conjure.yml) | `/audit/v1` |
| localization | [`api/localization.conjure.yml`](../../api/localization.conjure.yml) | `/localization/v1` |
| tenant | [`api/tenant.conjure.yml`](../../api/tenant.conjure.yml) | `/tenant/v1` |
| rank | [`api/rank.conjure.yml`](../../api/rank.conjure.yml) | `/rank/v1` |
| person | [`api/person.conjure.yml`](../../api/person.conjure.yml) | `/person/v1` |
| membership | [`api/membership.conjure.yml`](../../api/membership.conjure.yml) | `/membership/v1` |
| authorization | [`api/authorization.conjure.yml`](../../api/authorization.conjure.yml) | `/authorization/v1` |
| identity-federation | [`api/identity-federation.conjure.yml`](../../api/identity-federation.conjure.yml) | `/identity/v1` |
| document | [`api/document.conjure.yml`](../../api/document.conjure.yml) | `/document/v1` |
| order | [`api/order.conjure.yml`](../../api/order.conjure.yml) | `/order/v1` |
| platform | [`api/platform.conjure.yml`](../../api/platform.conjure.yml) | (shared types / ops) |

## OpenAPI document

The committed spec is [`docs/api/openapi/openapi.json`](openapi/openapi.json) — a single OpenAPI 3.0.3
document covering every service (112 operations across the 11 services above), with a `bearerAuth`
security scheme and the `SerializableError` envelope as the default error response.

Conjure has **no official OpenAPI generator**, so the repo ships its own emitter,
[`tools/ir2openapi`](../../tools/ir2openapi), wrapped by [`scripts/gen-openapi.sh`](../../scripts/gen-openapi.sh):

```bash
scripts/gen-openapi.sh        # -> docs/api/openapi/openapi.json   (pure Go; no JVM, no network)
```

The tool gets the Conjure **IR** from **godel** — `godel conjure-publish` compiles `api/` to IR and the
tool captures it from a tiny in-process sink — then converts IR → OpenAPI. So generation is fully
self-contained (no Palantir JVM tools, which have no OpenAPI emitter anyway).

CI ([`.github/workflows/api-docs.yml`](../../.github/workflows/api-docs.yml)) regenerates on changes
under `api/`, **fails if the committed spec is stale**, and publishes a **Redoc-rendered reference to
GitHub Pages**. Render locally with:

```bash
npx @redocly/cli build-docs docs/api/openapi/openapi.json -o openapi.html
```

> The Go server interfaces (`internal/conjure`) and the [client SDK](../../client) are generated from
> the same contract, so the spec, the SDK, and the running server cannot drift.

## Conventions reflected in every endpoint

- **Errors:** Conjure `SerializableError` envelope (`errorCode` / `errorName` / `parameters`); see
  [architecture/conventions.md](../architecture/conventions.md).
- **Pagination:** opaque `nextPageToken` keyset pagination on list endpoints.
- **i18n:** translatable labels are returned as `locale → text` maps, every supported locale in every
  response — no `Accept-Language` negotiation (D-i18n).
- **Auth:** every endpoint takes a bearer token (validated by the identity-federation middleware);
  authorization is decided by the PDP (`/authorization/v1/authorize`).
