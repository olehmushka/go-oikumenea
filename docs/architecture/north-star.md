# North star — oikumenea as the headless directory brain

> The **target-state architecture** the project is converging on, agreed in the 2026-07-18 design
> session. It is a *destination*, not a status report: the [stage board](../milestones.md#stage-board)
> says what exists; this file says where the topology is going and why. The four decisions that make
> it binding — **D-HeadlessTopology**, **D-ServiceIdentities**, **D-ConnectorPlane**, **D-DataPacks**
> — live in [roadmap-decisions.md](roadmap-decisions.md) and are sequenced as milestones
> **M51–M54** ([milestones.md](../milestones.md)).

## The one-paragraph statement

**oikumenea is the brain of a system of services**: the single source of truth for persons (users
*and* actors who never log in), organizations and units across domains, roles and authority, and
the reified relationships between all of them (D-Ontology). It **decides** authorization (the PDP)
but **never performs** authentication — identity is proven by external IdPs (Keycloak, Okta,
Google, Microsoft, GitHub, Apple, …) and only *validated* here (L-AuthzOnly). In the target state
it is a **headless internal service**: no consumer talks to it directly; every audience reaches it
through its own **facade**, every data feed reaches it through a **connector**, and deployments
tailor it with **data packs** — never with runtime code.

## The five planes

```
                 ┌─────────────────────────── identity plane (external) ───────────────────────────┐
                 │        Keycloak · Okta · Google · Microsoft · GitHub · Apple · …                 │
                 │   humans: OIDC login → user JWT      machines: client-credentials → service JWT  │
                 └───────────────┬────────────────────────────────────┬─────────────────────────────┘
                                 │ user token (passthrough)           │ service token
   public network                ▼                                    ▼
  ───────────────┬──── access plane ─────────────┬──────────── ingestion plane ────────────────────
                 │  console-bff │ hr-app │ …     │   hermenea │ future connectors …
                 │  (facades: session, shaping,  │   (own DB, own scheduler; push imports,
                 │   caching — zero authority)   │    pull wiring reads, answer lookups)
   internal      ▼                               ▼
  ─────────── Conjure API ──────────── import / wiring / lookup APIs ──────────────────────────────
                 ┌──────────────────────── core plane — oikumenea ─────────────────────────────────┐
                 │  ontology · person directory · tenant unit-graphs · authorization + PDP         │
                 │  verticals (finance, religion, vehicles, watchlists, …) · audit · i18n · search  │
                 │  sole owner of its Postgres — validates every token, decides every request       │
                 └───────────────────────────────▲──────────────────────────────────────────────────┘
                                                 │ boot autoseed (create-if-absent, version-gated)
                 └──── extension plane: data packs (locale packs, pinax presets, catalogs) ────────┘
```

### 1 · Core plane — oikumenea

The headless modular monolith, unchanged in substance: ontology (Object / Link / Action), the
instance-global person directory, the multi-domain tenant unit-graph, RBAC + the PDP, the
registry verticals, localization, audit, unified search, and the generic import endpoint. Sole
owner of its PostgreSQL. **The verticals stay core** — finance, watchlists, religion, vehicles,
physical identity, … are built-in modules behind their own permission codes (D-DataScope), not
plugins. What changes is exposure: in the target topology oikumenea listens on an **internal
network only** and is never reachable from outside the deployment (D-HeadlessTopology).

### 2 · Identity plane — external IdPs

Unchanged for humans: authentication is fully delegated; oikumenea stores no credentials, issues
no tokens, and validates inbound OIDC JWTs against the configured issuers' JWKS
([identity-federation](../modules/identity-federation.md)). **New:** machine callers — facades
that need their own standing (e.g. health probes) and connectors calling the wiring API — obtain
tokens from the *same external IdP* via the OAuth2 **client-credentials** flow and are validated
by the *same middleware*, resolving to a **service principal** instead of a person
(D-ServiceIdentities, generalizing the M16 `hermenea-importer`). The no-credentials stance is
preserved: the IdP owns client secrets, oikumenea only verifies signatures.

### 3 · Access plane — facades

Every human-facing or product-facing consumer reaches oikumenea through its own **facade** (a
BFF): the admin console behind **console-bff**, a future HR app behind its own facade, and so on.
Facades speak the Conjure API through the generated SDKs ([clients](../modules/clients.md), M27)
and may own the browser session, shape and aggregate responses, and cache — but they are
**unprivileged**: they always forward the **end-user's token**, oikumenea re-validates it and runs
the PDP against the real user. A facade holds no credential that widens access, so a compromised
facade can impersonate nobody (no confused deputy). There is **no on-behalf-of assertion** — a
facade cannot tell oikumenea "act as person X" (D-HeadlessTopology).

"Facade" names a set of constraints, not a mandatory extra process. **console-bff** is realized as the
admin console's own Next.js server tier — it already owned the httpOnly session and already forwarded
the user's bearer, so M52 recognized it rather than adding a redundant binary (D-HeadlessTopology,
M52 amendment).

### 4 · Ingestion plane — connectors

hermenea generalizes into a **family of connectors** — external services that feed or answer for
oikumenea, each with its own storage and scheduler, coupled to the core **only over HTTP**
(D-Hermenea's boundary, kept). oikumenea gains a **connector registry** — connectors and their
sources become first-class, monitorable Objects — and the contract names three interaction modes
(D-ConnectorPlane):

- **push** — bulk data in via the chunked, resumable `POST /import/{objectType}` envelopes (M49);
  the workhorse, unchanged.
- **pull-wiring** — connector → oikumenea *reads* on a narrow, permission-gated **wiring API**
  (resolve natural keys to RIDs, read reference catalogs, fetch sync cursors), authenticated as a
  service principal. What a connector may see is a grant, not a default.
- **on-demand lookup** — oikumenea → connector synchronous calls with a deadline (the M34
  watchlist check, generalized into one typed connector-call seam).

Scheduling stays inside the connector; the registry gives oikumenea **visibility, not
orchestration** — connectors report sync runs, oikumenea displays and audits them.

### 5 · Extension plane — data packs

A deployment is tailored by **data, not code**. A plugin is a **data pack**: a versioned bundle
of seedable content — locale packs, pinax-style world-model presets, catalogs, rank schemes —
loaded create-if-absent / fill-if-empty / never-delete by the boot autoseeder (the D-Pinax
machinery, generalized from bundled-only to operator-mounted packs), plus a **per-module enable
flag** so an installation can switch verticals off (a disabled module hides its API surface;
its schema still migrates — schema presence is not capability). There is **no runtime code
loading**: Go links statically and the Conjure contract is generated at build time, so code-level
extension means building from source — a documented seam, not a product feature (D-DataPacks).

## What is already true vs. what changes

| Already true today | Changes on the way to the north star |
|---|---|
| AuthN delegated; oikumenea validates tokens, stores no credentials (L-AuthzOnly) | Machine clients join the same path via client-credentials → service principals (M51) |
| PDP over the unit DAG is the centerpiece (D-Inherit, D-TenantOrganizations) | Nothing — explicitly reaffirmed |
| Persons are account-optional; never-logged-in actors are first-class (L-AccountOptional) | Nothing |
| Verticals are built-in, permission-gated modules (D-DataScope) | Nothing — they stay core; only an enable-flag surface is added (M54) |
| The console talks to oikumenea's public API directly (D-WebUI) | Console moves behind **console-bff**; oikumenea leaves the public network (M52) |
| hermenea pushes imports; one hard-wired sync egress (M34); shared-secret auth (D-Hermenea) | Connector registry + wiring API + generalized lookup seam; service-principal auth for the fleet (M53) |
| pinax presets are `go:embed`-bundled; locales are migration-seeded (D-Pinax) | Presets generalize to mountable, versioned **data packs**; per-module enable flags (M54) |

## Non-goals — locked out, not merely deferred

- **oikumenea never becomes an IdP.** No passwords, no token issuance, no MFA. The dormant
  account columns stay dormant.
- **Authorization is never externalized.** No external policy engine decides for oikumenea; the
  PDP over the unit graph *is* the product (reaffirms the OpenFGA deferral).
- **The verticals are never demoted to plugins.** The registry surface is the identity of the
  platform (D-DataScope), not an extension.
- **Facades gain no authority.** No on-behalf-of, no facade-side authorization, no client-side
  trust — every decision stays in the core, per real user.
- **Connectors never touch the core database.** HTTP is the only coupling, in both directions.
- **No runtime code loading.** Extension is data packs and facades/connectors — never `.so`
  files, embedded interpreters, or a plugin marketplace.

## Sequencing

Milestones, in dependency order (each leaves the system deployable; details on the
[stage board](../milestones.md#stage-board)):

1. **M51 — service identities** (D-ServiceIdentities): client-credentials validation + the
   service-principal registry. Prerequisite for both facades and the connector fleet.
2. **M52 — console facade** (D-HeadlessTopology): console-bff; oikumenea off the public network
   in the compose topology; passthrough proven end-to-end.
3. **M53 — connector plane** (D-ConnectorPlane): registry, wiring API, sync-run reporting,
   the generalized lookup seam; hermenea becomes the first *registered* connector.
4. **M54 — data packs & enable flags** (D-DataPacks): pack format, autoseeder generalization,
   per-module enable flags; first pack = a locale pack.
