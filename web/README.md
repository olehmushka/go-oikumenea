# go-oikumenea — web admin console (optional)

A standalone **Next.js** admin console for go-oikumenea, served on **port 8445**. It is an
**optional, opt-in** consumer of the API — it adds no Go code, no Conjure contract, and no schema.
See [`docs/web-ui.md`](../docs/web-ui.md) and the binding decisions
[D-WebUI](../docs/architecture/decisions.md) and
[D-HeadlessTopology](../docs/architecture/roadmap-decisions.md).

## How it works (BFF = console-bff, the facade)

This app's **server tier is `console-bff`** — the first facade of the headless topology (M52 /
D-HeadlessTopology). In the packaged compose topology oikumenea publishes **no host port**; this
facade is the only thing on the public network:

```
browser ──(httpOnly session)──▶ console-bff = Next.js (:8445)
                                     │  (end-user's Bearer, forwarded unchanged)
                                     ▼
                          oikumenea API — internal network only
```

The facade is **unprivileged**: it forwards the end-user's own token and makes no on-behalf-of
assertion, so oikumenea re-validates that token and runs the PDP against the real user. It holds no
credential that widens access — a compromised facade can impersonate nobody.

- **Keycloak login** via Auth.js (NextAuth v5), OIDC Authorization-Code flow, exchanged
  **server-side**. The browser never holds a token.
- A single **BFF proxy** (`/api/oikumenea/[...path]`) attaches the bearer and forwards to the
  API — so there is **no CORS** on the Go app.
- The API layer is the **typed TypeScript SDK** `oikumenea-client` (`../clients/typescript`, a
  `file:` dep generated from the Conjure contract — D-ClientSDK), so the UI cannot drift from the
  contract. Build it with `npm run gen:sdk` (also run by `predev`/`prebuild`).
- **No client-side authorization**: the console asks the PDP (`/authorization/v1/authorize`)
  and renders what the API returns; it never filters for visibility itself.

## Run it (local dev — recommended)

Bring up the dev infra + the API first (see [`deploy/keycloak/README.md`](../deploy/keycloak/README.md)):
Postgres + Keycloak via `docker compose -f ../docker-compose.dev.yml up -d`, migrations applied,
and the server running on `:8443`. Re-import the realm so the `oikumenea-web` client exists.

Then:

```bash
cd web
cp .env.example .env.local        # dev defaults already match the dev Keycloak + API
npm install
npm run dev                       # http://localhost:8445  (runs gen:api first)
```

Sign in with **admin / admin**. The dev `.env.local` sets `NODE_TLS_REJECT_UNAUTHORIZED=0`
so the Node server trusts the API's self-signed cert — **dev only**.

### Using the console

It's an **object workspace** (see [`docs/web-ui.md`](../docs/web-ui.md)):

- **⌘K / Ctrl-K** opens the command palette anywhere — search objects across types, jump to a view,
  run a quick action, or paste a RID to open it directly.
- **Explore** (sidebar) lists each object type as a filterable, sortable, multi-select table; click a
  row for a detail **drawer** (properties, links, inline actions) without leaving the list.
- **`/o/<rid>`** is the universal object view; **`/graph/<rid>`** is a traversable relationship graph;
  **/ontology** browses the whole type registry.

## Run it (Docker, production-shaped)

```bash
# from repo root — opt-in via the `ui` profile (default `up` does NOT start it):
docker compose --profile ui up --build
open http://localhost:8445
```

In this topology oikumenea is unreachable from the host — `:8443` and `:8444` are unpublished, and
`:8445` (this facade) is the only public port. `API_BASE_URL` is the compose-internal `https://app:8443`.

Set the env in `docker-compose.yml`'s `console-bff` service for your environment — crucially
`AUTH_KEYCLOAK_ISSUER` (a Keycloak reachable from **both** the browser and the container, with
the same URL), `AUTH_SECRET`, and `AUTH_KEYCLOAK_SECRET`. That compose ships no Keycloak (the
IdP is external); for an all-in-one local demo, prefer the dev path above.

## Environment

See [`.env.example`](.env.example). `API_BASE_URL` is **required** (no default — console-bff throws at
startup if unset, so a misconfigured facade fails loudly rather than reaching for a port that is not
there). Keys: `API_BASE_URL`, `AUTH_SECRET`, `AUTH_URL`,
`AUTH_KEYCLOAK_ID` / `AUTH_KEYCLOAK_SECRET` / `AUTH_KEYCLOAK_ISSUER`, and (dev) `NODE_TLS_REJECT_UNAUTHORIZED`.

## Notes & caveats

- **Issuer hostname.** The browser redirect and the server-side token exchange use the *same*
  issuer URL. In containers that means the URL must resolve identically from the browser and
  from the Next.js server (dev sidesteps this by running the UI on the host).
- **Non-admin logins.** The dev `admin` resolves via the fixed-`sub` bootstrap binding. Other
  users need provisioned accounts or JIT enabled (D-JIT) on the service.
- The TS SDK (`../clients/typescript`) is generated from the contract; rebuild it with `npm run
  gen:sdk` after the API changes (regenerate its sources via `scripts/gen-ts-client.sh`).
