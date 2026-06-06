# API reference (OpenAPI)

The API source of truth is the **Conjure IDL** under [`api/`](../../api) — one `*.conjure.yml` per
module (D-Conjure / D-Stack). The gödel `conjure-plugin` compiles those to IR and generates the Go
server interfaces + clients into `internal/conjure` (never hand-edited). The **OpenAPI** document is a
*generated artifact* derived from the same Conjure IR, so the spec and the running server cannot drift.

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

## Generating the OpenAPI documents

OpenAPI is produced from the Conjure **IR** by the Palantir `conjure` compiler's OpenAPI emitter. The
two steps (compile YAML → IR, then IR → OpenAPI) are:

```bash
# 1. Compile the Conjure YAML sources to IR (one combined IR for the api/ directory).
conjure compile api out/conjure-ir.json

# 2. Emit OpenAPI v3 from the IR into docs/api/openapi/.
conjure-openapi generate --ir out/conjure-ir.json --output docs/api/openapi/
```

> **Note (M11):** the OpenAPI documents are **not committed** in this repo yet — the Palantir
> `conjure` / `conjure-openapi` compilers are JVM tools not available in the current build sandbox, so
> the artifacts cannot be generated here. The generation is wired as the documented command above and
> should run as a release step (and a CI docs job) once the compiler is on the build image. The Go
> server interfaces in `internal/conjure` ARE generated and committed, so the contract is already
> enforced in code; OpenAPI is the human-facing rendering of that same contract.

## Conventions reflected in every endpoint

- **Errors:** Conjure `SerializableError` envelope (`errorCode` / `errorName` / `parameters`); see
  [architecture/conventions.md](../architecture/conventions.md).
- **Pagination:** opaque `nextPageToken` keyset pagination on list endpoints.
- **i18n:** translatable labels are returned as `locale → text` maps, every supported locale in every
  response — no `Accept-Language` negotiation (D-i18n).
- **Auth:** every endpoint takes a bearer token (validated by the identity-federation middleware);
  authorization is decided by the PDP (`/authorization/v1/authorize`).
