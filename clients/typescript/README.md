# go-oikumenea TypeScript SDK

A typed TypeScript client for the go-oikumenea API. The per-service clients under `src/generated` are
**generated from the same Conjure contract** (`api/*.conjure.yml`) as the server and the Go SDK
(`client/`), so the SDK cannot drift from the API (**D-ClientSDK** / **D-Conjure**). This is the TS
peer of the Go SDK — published and versioned independently.

```
npm install oikumenea-client
```

> The package name (`oikumenea-client`) and `version` in `package.json` are placeholders — set the
> real scoped name and version before the first `npm publish`.

## Layout

- `src/generated/**` — generated per-service clients + types (one namespace per module: `person`,
  `tenant`, …, `hermenea`, `dataimport`). **Generated, never hand-edited.**
- `src/index.ts` — the only hand-written source: the unified façade `createOikumeneaClient(...)` that
  wires every generated service onto one shared HTTP bridge.

## Usage

```ts
import { createOikumeneaClient } from "oikumenea-client";

const client = createOikumeneaClient({
  baseUrl: "https://localhost:8443",
  token: process.env.OIKUMENEA_TOKEN, // OIDC/JWT bearer; see scripts/keycloak-token.sh
});

// Who am I? (resolves the token -> person/account)
const who = await client.identityFederation.whoami();

// List the directory
const page = await client.person.listPersons();

// Reach hermenea THROUGH oikumenea — oikumenea reverse-proxies /hermenea/v1/* (D-Hermenea)
const runs = await client.hermenea.listRuns();
```

### Behind a BFF (browser)

The token must not live in the browser. Point `baseUrl` at a same-origin proxy that injects the
bearer server-side and omit `token` — exactly how the web console uses this SDK:

```ts
const client = createOikumeneaClient({ baseUrl: "/api/oikumenea" });
```

`baseUrl`, `token` and `fetch` all accept suppliers, so callers can refresh tokens per request or
swap the fetch implementation (Node fetch, a proxying fetch, etc.).

### Errors

Service methods throw the Conjure error envelope (`conjure-client`'s thrown errors carry the
`errorCode`/`errorName`/`parameters` of the server's `SerializableError`). Catch and branch on
`errorName`.

## Regenerating

`src/generated` is produced from the Conjure contract by the repo-root script:

```
npm run gen          # = bash ../../scripts/gen-ts-client.sh
npm run gen:verify   # drift gate: fails if src/generated is stale vs the contract
```

The script extracts the Conjure IR via `tools/ir2openapi -dump-ir` (no JVM), rewrites the IR's
package names to the 3-segment form conjure-typescript requires (a derived-artifact transform that
does not touch the contract or the wire format — see `scripts/rewrite-ir-packages.mjs`), then runs
`conjure-typescript`.

## Build

```
npm run build       # tsc -> dist/
npm run typecheck
```
