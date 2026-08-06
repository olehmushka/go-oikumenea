# Module: clients (Go + TypeScript SDKs)

> Reads: [conventions](../architecture/conventions.md) (Conjure §) ·
> [roadmap-decisions](../architecture/roadmap-decisions.md) (**D-ClientSDK**) ·
> [decisions](../architecture/decisions.md) (**D-Conjure**, **D-WebUI**) ·
> [hermenea](hermenea.md) (the proxied companion).
> No table prefix — this is **client tooling**, not a data module.

> **The SDKs are generated build outputs + thin hand-written façades, not an `internal/` module.**
> They own no schema, no RIDs, no endpoints of their own. This doc follows the module template for
> consistency, but "data model" is the generation pipeline and "API surface" is the façade each SDK
> exposes over the generated per-service clients.

## Purpose

Distribute the go-oikumenea API as **two symmetric, published client SDKs** — Go and TypeScript —
**generated from the single source of truth** (`api/*.conjure.yml`) so they cannot drift from the
server or each other (**D-ClientSDK**, extending **D-Conjure**). Each SDK adds a **unified façade**: one
constructor binds a base URL + bearer token to **every** service, so callers get one typed client for
the whole API instead of assembling per-service clients by hand. The TypeScript SDK is the API layer
the [web console](../web-ui.md) consumes.

Because oikumenea reverse-proxies the [hermenea](hermenea.md) control/read API at `/hermenea/v1/*`
(**D-Hermenea**), the hermenea endpoints are **members of both façades** — a single client + base URL
reaches both oikumenea-native and hermenea-proxied endpoints. No database coupling is added; the SDKs
simply expose the existing HTTP proxy.

## Entities & aggregates

None. The SDKs reflect every module's Objects/Links/Actions but introduce no new ontology
([ontology-mapping](../ontology-mapping.md) is unchanged).

## Data model (the generation pipeline)

One Conjure contract feeds three generated surfaces; none is hand-edited:

```
api/*.conjure.yml ──(godel conjure)──> internal/conjure/**           server interfaces + RegisterRoutes
                  ──(godel conjure, project "client")──> clients/go/oikumenea/<module>   Go SDK clients
                  ──(godel, via tools/ir2openapi -dump-ir)──> Conjure IR
                        ──(rewrite-ir-packages.mjs)──> IR (3-seg packages)
                              ──(conjure-typescript)──> clients/typescript/src/generated   TS SDK clients
```

- **Go SDK** — `clients/go/` (nested Go module, `github.com/olehmushka/go-oikumenea/clients/go`, versioned
  independently). Generated per-service clients under `clients/go/oikumenea/<module>`; hand-written files:
  `clients/go/dial.go` (`Dial`) and `clients/go/client.go` (the façade).
- **TypeScript SDK** — `clients/typescript/` (npm package). Generated per-service clients under
  `src/generated`; hand-written files: `src/index.ts` (the façade) and `scripts/rewrite-ir-packages.mjs`.
- **IR extraction** — `tools/ir2openapi -dump-ir <path>` reuses the existing offline IR capture (godel
  `conjure-publish` to an in-process sink — no JVM, no network) to dump the raw Conjure IR. The same IR
  also drives the OpenAPI doc.
- **The package rewrite** — conjure-typescript rejects packages with < 3 dot-segments; the contract uses
  2 (`oikumenea.<module>`). `rewrite-ir-packages.mjs` rewrites them to `oikumenea.api.<module>` **in the
  IR only** — a derived-artifact transform that leaves endpoint paths, service names and JSON shapes (the
  wire contract) untouched.
- **Generation scripts** — `scripts/gen-ts-client.sh` (Go SDK regenerates via `godel conjure`). Both
  have a verify/drift gate: `godel conjure --verify` and `scripts/gen-ts-client.sh --verify`.

## API surface (the façades)

Both façades expose one field/method per service, including `hermenea` and `import`:

| | Go (`clients/go/client.go`) | TypeScript (`clients/typescript/src/index.ts`) |
|---|---|---|
| Construct | `client.New(baseURL, token, opts…)` · `client.NewWithTokenProvider(baseURL, provider, opts…)` | `createOikumeneaClient({ baseUrl, token?, fetch?, userAgent? })` |
| Call (native) | `c.Person.ListPersons(ctx)` | `c.person.listPersons()` |
| Call (proxied) | `c.Hermenea.ListRuns(ctx)` | `c.hermenea.listRuns()` |
| Transport | one `Dial` (conjure-go-runtime httpclient) | one `DefaultHttpApiBridge` (conjure-client) |

`baseUrl`/`token` accept suppliers; the TS `fetch` is overridable (Node fetch, a proxying fetch, or the
browser's). `Platform` is unauthenticated (ops endpoints) so it is not token-bound.

## Dependencies

- **The Conjure contract** (`api/*.conjure.yml`) — the only source of truth.
- **godel-conjure-plugin** (Go gen), **conjure-typescript** + **conjure-client** runtime (TS gen),
  **tools/ir2openapi** (IR dump).
- **web** depends on the TS SDK as a `file:../clients/typescript` dependency
  ([web-ui](../web-ui.md)): the browser client targets the BFF (`/api/oikumenea`, tokenless); Server
  Components bind the session bearer via `oikumenea()`.

## Authorization touchpoints

None of its own. The SDKs carry a bearer token; **the server makes every authorization decision** (the
PDP). The SDKs never see permissions, scopes, or the shadow-visibility gate.

## Patterns

- **Generated + façade.** Per-service clients are generated; the one-call aggregate is the only
  hand-written client code (mirrors how `dial.go` is the only hand-written Go-SDK file).
- **Proxy-as-member.** A reverse-proxied companion (hermenea) appears as an ordinary façade member — no
  special-casing in the SDK.
- **Derived-artifact transform.** Adapting the IR for a downstream generator (the package rewrite) is
  done on the IR, never on the contract.

## Invariants & safety

- Generated code is **never hand-edited**; the `--verify` gates fail CI if `clients/go/` or
  `clients/typescript/src/generated` drift from the contract.
- The SDKs add **no new server behavior, schema, or endpoint** — additive / tooling-only.
- The package rewrite must not change the wire contract (paths/service names/JSON); it only renames
  code-organization packages in the IR.

## Open seams / future

- **Publishing.** Tagging + publishing to npm and pkg.go.dev (and choosing the final scoped npm name /
  version policy) is a follow-up; in-repo consumption (the web app, the Go smoke test) verifies the SDKs
  meanwhile.
- **More languages.** The same IR could drive Python/Kotlin/etc. SDKs if needed.
- **Façade ergonomics.** Optional cross-service helpers (pagination iterators, retry/QoS presets) could
  live in the hand-written façade layer without touching generated code.
