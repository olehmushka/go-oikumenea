# web-ui — optional standalone admin console

> **Not a backend module.** This is an **optional consumer** of the public API, documented here
> because it is a first-class (if removable) surface. It adds **no Go code, no Conjure contract, no
> schema**, and no new port to the `oikumenea` binary. Binding rationale: [D-WebUI](architecture/decisions.md).
> Lives in `web/` (a standalone Next.js app), served on **port 8445**, run only when you opt in.

## Purpose

Give operators of a hierarchical organization (army / church / university) a **human-usable admin
console** for the directory and the authorization graph, without coupling a UI into the service. The
console is a thin, generated client over the existing HTTP API — it can do nothing the API does not
already expose, and the API is unaware of it.

## Architecture (Backend-for-Frontend = `console-bff`, the facade)

The Next.js server tier **is `console-bff`** — the first facade of the headless topology (M52 /
[D-HeadlessTopology](architecture/roadmap-decisions.md)). In the packaged compose topology oikumenea
publishes no host port; this facade is the only thing on the public network.

```
  browser ──(httpOnly session cookie)──▶ console-bff = Next.js server (:8445)
     ▲                                         │
     └────────── Keycloak login ◀──────────────┘   (OIDC Authorization-Code flow, server-side)
                                               │
                            (end-user's Bearer, forwarded unchanged)
                                               ▼
                              oikumenea API — internal network only
```

The facade is **unprivileged**: it forwards the end-user's own token and makes no on-behalf-of
assertion, so oikumenea re-validates that token and runs the PDP against the real user. It holds no
credential that widens access — its environment carries Keycloak *client* config (which authenticates
users) and no service token of its own.

- **Standalone Next.js** (App Router, TypeScript, Tailwind), its own Node process. Default
  deployments do not run it; it is the `ui`-profiled `console-bff` `docker-compose` service / a
  `web/` dev server.
- **BFF, not a SPA-to-API call.** The browser talks **only** to the Next.js server. A single proxy
  route (`/api/oikumenea/[...path]`) reads the server session, attaches `Authorization: Bearer
  <access_token>`, and forwards to `API_BASE_URL`. **The browser never holds a token and never calls
  the API directly** — so there is **no CORS surface** on the Go app.
- **Two server-side paths to the core, one browser path.** Server Components call oikumenea directly
  via `oikumenea()`; the browser goes through the proxy route. Both attach the same session bearer and
  both originate *inside* the facade process on the internal network, so this is not a second exposure
  path — the browser still has exactly one way in (D-HeadlessTopology, M52 amendment).
- **Generated, typed client — the single point of contact.** The web console reaches the backend
  **only** through the **TypeScript SDK** `oikumenea-client` (`clients/typescript/`, a `file:`
  dependency) generated from the Conjure contract (**D-ClientSDK** — see the
  [clients module doc](modules/clients.md)). There are **no ad-hoc `fetch` calls** to the API.
  `web/src/lib/api/client.ts` builds the browser client against the BFF
  (`createOikumeneaClient({ baseUrl: "/api/oikumenea" })`, tokenless); Server Components bind the
  session bearer via `oikumenea()` in `web/src/lib/api/server.ts`. Components call **typed methods**
  (`api.person.getPerson(id)`) or the SDK's generic `api.request(method, path)` escape hatch for the
  data-driven ontology registry (paths without a dedicated method); the thin `apiGet`/`apiSend`/
  `mutate`/`bffGet` helpers are convenience wrappers over that one SDK transport. SDK errors are
  `ConjureError`s, normalized for display by `web/src/lib/api/errors.ts` (`errorInfo`). The SDK's
  generated clients are **never hand-edited** (same stance as Conjure/sqlc output), so the UI cannot
  drift from the contract. *(This supersedes the earlier in-`web` `openapi-typescript` →
  `schema.d.ts` + `openapi-fetch` layer.)*

## Authentication & authorization flow

1. Unauthenticated browser → `/login` → **Auth.js** (NextAuth v5) Keycloak provider starts the OIDC
   **Authorization-Code** flow against the realm (`oikumenea`, confidential client `oikumenea-web`).
2. Keycloak authenticates the user and redirects back; Auth.js exchanges the code server-side and
   stores `access_token` / `refresh_token` / `expires_at` in an **httpOnly** JWT session. The session
   exposed to the browser carries display identity only — never a token.
3. The access token carries `aud: oikumenea` (Keycloak audience mapper), so the service's inbound
   validator ([identity-federation](modules/identity-federation.md)) accepts it unchanged. The UI
   **authenticates at the IdP**; the service still validates and decides — [L-AuthzOnly](architecture/decisions.md)
   holds. Tokens are refreshed against the Keycloak token endpoint on expiry.
4. **Authorization is the PDP's job, not the UI's.** Affordances are gated by asking
   [authorization](modules/authorization.md): `POST /authorization/v1/authorize` (one
   `person × action × unit`), or `/authorize/batch` with `explain` where the caller holds
   `assignment.read`. The UI **never** branches on rank/position, and **never** filters for shadow
   visibility — it renders exactly what the API returns.

## API consumed (all 11 modules)

The full admin console covers the whole surface ([OpenAPI reference](api/README.md)): units (the
[tenant](modules/tenant.md) DAG — tree, ancestors/descendants, edges, visibility),
[persons](modules/person.md) (directory, CLDR names, citizenships/residences, contact channels,
lifecycle), [memberships & positions](modules/membership.md), the [rank](modules/rank.md) scheme,
[roles/assignments/instance-admins](modules/authorization.md) plus an interactive authorize check,
[documents](modules/document.md) & personal codes, [orders](modules/order.md) (наказ — issue/revoke),
[localization](modules/localization.md) (locales + translation editor), and the
[audit](modules/audit.md) log viewer.

## Dependencies

- The running `oikumenea` API at `API_BASE_URL` (server-to-server from the Next.js process; the
  compose-internal `https://app:8443`, or the host binary in dev). `API_BASE_URL` is **required** —
  console-bff throws at startup if it is unset rather than defaulting to a host port that the packaged
  topology no longer publishes.
- At least one configured IdP. The bundled default is a Keycloak realm with the confidential
  `oikumenea-web` client (`deploy/keycloak/`); the login screen renders a button per provider whose
  credentials are present in the console environment (D-MultiIdPExamples), so Google/Entra/GitLab/Okta
  can be offered alongside or instead of it — see [`deploy/oauth/README.md`](../deploy/oauth/README.md).
  Whichever provider signs a session in, the BFF forwards the token that provider's issuer entry pins
  an audience on (Keycloak: the access token; public IdPs: the ID token).
- Build-time: the committed [`docs/api/openapi/openapi.json`](api/openapi/openapi.json) (kept fresh
  from Conjure by `scripts/gen-openapi.sh`).

## Object-centric workspace (the primary surface)

The console is organised around **objects and their links**, not modules — the UI analogue of
[D-Ontology](ontology-mapping.md). One **ontology registry** (`web/src/lib/ontology/registry.ts`)
describes each Object/Link type once (its list/detail endpoints, table columns, properties, link
collections, and inline actions); that single data-driven config powers every surface below, so
adding a type is a registry entry, not new pages.

- **Self-describing RID routing.** Every entity's id is a composed URN whose `entity_type` slot
  encodes its kind (`parseRid` in `web/src/lib/ontology/rid.ts`). So *any* RID resolves to its type
  with no server lookup — the basis for paste-a-RID navigation and the universal object view.
- **Command palette (⌘K).** A global omnibox (`cmdk`) that navigates, runs quick actions, resolves a
  pasted RID directly, and **searches objects** through the server-side unified search endpoint
  (`SearchService.searchObjects`, [D-UnifiedSearch](architecture/decisions.md) / [search](modules/search.md)) —
  debounced, permission-gated and visibility-trimmed by the backend. *(This supersedes the earlier
  client-side fan-out that filtered the first page of each listable type in the browser.)*
- **Object explorer** (`/explore/[type]`). A dense, filterable, sortable, **multi-selectable** table
  with a **detail drawer** (click a row → properties + links + inline actions on the right, without
  leaving the list) and **bulk actions** that loop existing single-entity mutations.
- **Two views per type: list and dashboard** (M56–M58,
  [D-ConsoleDashboards](architecture/decisions.md)). `/explore/[type]` renders either a table or a
  **dashboard** of charts (`?view=dashboard`, generalizing the `?view=tree` toggle units already
  have). Both are renderings of **one request state, and that state lives entirely in the URL** — so
  toggling between them preserves the filter set, a chart segment is an `<a>` that adds one more
  filter (**click-to-filter is ordinary navigation**), and a filtered view is shareable. The
  registry carries both halves: `filters?: FilterDef[]` and `dashboard?: ChartDef[]` on
  `ObjectTypeDef`, so a type joins both surfaces by a registry entry. Buckets arrive pre-aggregated
  from the owning module's `/stats` endpoint ([D-ObjectFacets](architecture/decisions.md); the
  per-type facet and chart catalog is [architecture/facets.md](architecture/facets.md)) — the console
  never aggregates a page of rows itself, because a chart computed from page 1 of a keyset list is
  wrong.
- **Universal object view** (`/o/[rid]`). One page for any object: header (type/RID/title), property
  list, and a grouped, traversable **Links panel**. It redirects to the richer bespoke editors
  (`/persons/[id]`, `/units/[id]`, `/orders/[id]`) where those exist, and renders generically for
  every other type.
- **Ontology browser** (`/ontology`) and **relationship graph** (`/graph/[rid]`, `@xyflow/react`):
  the registry as a human-facing type catalog, and a lazily-expanded node/edge graph for traversing
  the unit DAG and a person's/​unit's links.

Three small, focused libraries back this: `cmdk` (palette), `@xyflow/react` (graph) and `@visx/*`
(chart **scales and shapes only** — the chart markup in `web/src/components/charts/` is ours:
`BarChart`, `DonutChart`, `Histogram`, `StatTile`, `Sparkline`). The table, drawer, and object
primitives are hand-rolled on the existing Tailwind classes.

## Patterns

- **Locale-map fields** ([D-i18n](architecture/decisions.md)). Translatable labels arrive as
  `locale → text` maps; a `pickLabel(map, uiLocale)` helper renders per a UI-locale switch with
  fallback (seeded `ukr`/`eng`). Editors write the **whole map** back. Person names use the
  per-person transliteration variants, not the admin translation store.
- **Conjure error envelope.** API failures are relayed as the `SerializableError` JSON
  (`{errorCode, errorName, parameters}`) and surfaced via a shared error toaster.
- **Token-paginated lists** ([D-Conjure](architecture/decisions.md)) — list views thread the
  page token, not offsets. No list envelope carries a total; a count comes from the type's `/stats`
  endpoint, never from counting a page.
- **URL-borne view state** ([D-ConsoleDashboards](architecture/decisions.md)). Filters, the
  list/dashboard toggle, sort and the page token all live in `searchParams`, not component state —
  which is what makes a view shareable, refresh-safe, and identical across the two renderings.

## Invariants

- The browser never receives or stores an API/IdP token; every API call transits the BFF proxy.
- The UI performs **no authorization decision** locally and **no visibility filtering** locally.
- The TypeScript SDK's generated clients (`clients/typescript/src/generated`) are generated, never
  hand-edited; `web/` consumes the SDK rather than re-deriving types.
- The Go service, its Conjure contract, and the DB schema are unchanged by anything here; removing
  `web/` and the optional compose service leaves a working, untouched deployment.

## Open seams

- **Production IdP hostname.** Dev uses `http://localhost:8080/realms/oikumenea` for both the browser
  redirect and server token-exchange. A containerized prod deployment must resolve the same issuer URL
  from both browser and server (documented in `web/README.md`).
- **Non-admin account provisioning.** The dev `admin` user resolves via the fixed-`sub` bootstrap
  binding; broader login requires provisioned accounts or enabling JIT ([D-JIT](architecture/decisions.md)).
- ~~**Bespoke module pages.**~~ **CLOSED (M58 tickets 2, 3 and 5).** All six — religion and
  external-org (ticket 2), vehicle and finance (ticket 3), company and education (ticket 5) — now
  have an ontology-registry entry and route browsing through the generic explorer, with a filter bar
  and a dashboard. What stays on each module page is EDITING: creation and the panels richer than the
  generic action runner. Each keeps a bounded one-page table that says so, replacing the old shape
  that fetched 100 rows and dropped `nextPageToken` — presenting a truncation as a registry.
- **Sort.** The contract has **no sort param anywhere**, so column sorting stays client-side over the
  current page. Server-side ordering is additive on top of the facet vocabulary, not part of it.
- **Cross-type dashboards.** Every dashboard is single-type by construction (per-module `/stats`
  endpoints, D-ObjectFacets) — no cross-tab, and no roll-up spanning types.
- **Design system.** Tailwind + hand-rolled primitives, plus three focused libraries — `cmdk` (command
  palette), `@xyflow/react` (relationship graph) and `@visx/*` (chart scales/shapes). No broader
  component-library lock.
