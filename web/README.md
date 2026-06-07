# go-oikumenea — web admin console (optional)

A standalone **Next.js** admin console for go-oikumenea, served on **port 8445**. It is an
**optional, opt-in** consumer of the public API — it adds no Go code, no Conjure contract, and
no schema. See [`docs/web-ui.md`](../docs/web-ui.md) and the binding decision
[D-WebUI](../docs/architecture/decisions.md).

## How it works (BFF)

```
browser ──(httpOnly session)──▶ Next.js (:8445) ──(Bearer)──▶ oikumenea API (:8443)
```

- **Keycloak login** via Auth.js (NextAuth v5), OIDC Authorization-Code flow, exchanged
  **server-side**. The browser never holds a token.
- A single **BFF proxy** (`/api/oikumenea/[...path]`) attaches the bearer and forwards to the
  API — so there is **no CORS** on the Go app.
- Types are **generated** from `../docs/api/openapi/openapi.json`
  (`src/lib/api/schema.d.ts`, via `npm run gen:api`) — the UI cannot drift from the contract.
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

## Run it (Docker, production-shaped)

```bash
# from repo root — opt-in via the `ui` profile (default `up` does NOT start it):
docker compose --profile ui up --build
open http://localhost:8445
```

Set the env in `docker-compose.yml`'s `web` service for your environment — crucially
`AUTH_KEYCLOAK_ISSUER` (a Keycloak reachable from **both** the browser and the container, with
the same URL), `AUTH_SECRET`, and `AUTH_KEYCLOAK_SECRET`. That compose ships no Keycloak (the
IdP is external); for an all-in-one local demo, prefer the dev path above.

## Environment

See [`.env.example`](.env.example). Keys: `API_BASE_URL`, `AUTH_SECRET`, `AUTH_URL`,
`AUTH_KEYCLOAK_ID` / `AUTH_KEYCLOAK_SECRET` / `AUTH_KEYCLOAK_ISSUER`, and (dev) `NODE_TLS_REJECT_UNAUTHORIZED`.

## Notes & caveats

- **Issuer hostname.** The browser redirect and the server-side token exchange use the *same*
  issuer URL. In containers that means the URL must resolve identically from the browser and
  from the Next.js server (dev sidesteps this by running the UI on the host).
- **Non-admin logins.** The dev `admin` resolves via the fixed-`sub` bootstrap binding. Other
  users need provisioned accounts or JIT enabled (D-JIT) on the service.
- `src/lib/api/schema.d.ts` is generated; refresh with `npm run gen:api` after the API changes.
