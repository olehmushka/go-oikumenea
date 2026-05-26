# Architecture overview

How go-oikumenea is built as code: the stack, the layering, how modules talk, and the
two request paths (a normal mutation, and a PDP decision). Read [`../glossary.md`](../glossary.md)
first.

---

## Technology stack

The whole service stands on the **Palantir OSS stack**. This is deliberate — the service is
meant to read as a reference implementation of that stack. (This reverses the `drafts/`
choice of `uber/fx` + generic OpenAPI; see [`decisions.md`](decisions.md) D-Stack.)

| Concern | Choice | Notes |
|---|---|---|
| Language | **Go** | — |
| API contract | **Conjure** IDL | `*.conjure.yml` per module. The single source of truth for the API. |
| Codegen / build | **gödel** + `godel-conjure-plugin` | Conjure IR → Go server interfaces + client structs. |
| HTTP server | **witchcraft-go-server** | `witchcraft.Server`; conjure routes registered into `wrouter`. |
| Clients | **conjure-go-runtime** | Typed clients for internal composition and integration callers. |
| Logging | **witchcraft-go-logging** | `svc1log` (service), `req2log` (request), `evt2log` (events), `audit2log` (audit module). |
| Tracing | **witchcraft-go-tracing** | Zipkin-style spans; `X-B3-*` propagation. |
| Metrics | **pkg/metrics** (`witchcraft-go-metrics`) | Tagged metric registry; RED discipline. |
| Errors | **werror** | Structured errors with safe/unsafe params; surfaced as Conjure `SerializableError`. |
| Config | **encrypted-config-value (ECV)** + **pkg/refreshable** | `var/conf/install.yml` + `runtime.yml`; live reload; operator-supplied DB DSN + IdP config. |
| Health | **witchcraft-go-health** | Readiness/liveness reporters (DB reachability, schema-version check). |
| DB driver | **pgx** | No ORM (witchcraft prescribes none). Operator-owned credentials. |
| Typed queries | **sqlc** | Compile-time-checked SQL; generates Go from `.sql` against the schema. |
| Migrations | **Atlas** (versioned) + `atlas migrate lint` | One repo-root `migrations/` dir; destructive-change gate. See [`upgrade-safety.md`](upgrade-safety.md). |
| API docs | Conjure IR → **OpenAPI** + generated reference site | Comes essentially for free from the contract. |
| Packaging | **Docker** + `docker-compose` (Postgres + service) | Self-hostable; no cloud-vendor coupling. |

---

## Architecture style — modular monolith, extraction-ready

One binary, one deploy, internally separated into **bounded contexts (modules)** with
explicit boundaries that would support later extraction without a rewrite. Each module owns
a coherent slice of the domain and a table-prefix in the `oikumenea` schema
(`oikumenea.tenant_*`, `oikumenea.person_*`, …).

### Hexagonal layering, mapped onto witchcraft

Every module uses the same four layers, defined by *who triggers whom*:

```
transport  →  application  →  domain  →  adapters
                                 ↑          │
                                 └─ interfaces ┘
```

- **domain** — pure logic: entities, value objects, aggregate roots, invariants, domain
  events. No I/O, no framework imports. The domain declares the interfaces it needs from the
  outside world.
- **application** — orchestrators (application services) that compose domain operations with
  adapter calls to fulfil a use case. **Flat** — no CQRS command/query split.
- **adapters** — implementations of domain-declared interfaces: pgx/sqlc repositories, the
  event publisher, the external JWKS client. Depend on infrastructure, never the reverse.
- **transport (port invokers)** — drive the domain from outside: the **conjure-generated
  server interfaces** (HTTP handlers), CLI commands, scheduled-job handlers. Resolve the PDP
  context, call an application service, map results/errors to the Conjure contract.

The dependency rule: `transport → application → domain → (domain interfaces ← adapters)`.
No cycles.

### Per-module package layout

```
internal/<module>/
├── domain/         entities, value objects, aggregate roots, domain events, invariants
├── application/    application services (flat)
├── adapters/       pgx/sqlc repositories, event publisher, external clients
├── transport/      conjure server-interface implementations, job handlers
└── module.go       wires this module's providers + registers its conjure routes
```

The composition root is `cmd/oikumenea/main.go`: it builds the `witchcraft.Server`, wires
the shared [platform](../modules/platform.md) services, then each module's `module.go`.

### Conjure-first flow

```
*.conjure.yml  ──gödel──▶  generated Go (server interfaces + client structs + OpenAPI)
                                │
              transport implements the server interface; clients consume the structs
```

The contract is authored first. Handlers implement generated interfaces, so the compiler
enforces that transport matches the contract. The OpenAPI reference site is generated from
the same IR.

---

## Inter-module communication

- **Cross-module queries** are synchronous, read-only, **direct calls through the target
  module's application-service interface** (the Go interface, not over HTTP). Example: the
  [authorization](../modules/authorization.md) PDP asks [tenant](../modules/tenant.md)
  "is U a descendant of T?".
- **Cross-module mutations** are published as **domain events** (in-process event bus,
  backed by an outbox/queue seam) and processed by the subscribing module in its own
  transaction. Example: a unit-archived event prompts authorization to flag affected
  assignments.
- **Same-module work** stays in-process, typically in one transaction.

Rule of thumb: *queries cross freely; mutations publish events*. This is what keeps the
monolith extraction-ready — extraction turns the event bus into a real broker and the direct
query calls into network calls, with no domain restructuring.

---

## Conceptual domain model

```
                            identity-federation
            ┌──────────────────────────────────────────────┐
            │  ExternalIdentity(issuer, subject)            │
            │        │ links                                │
            │     Account ──0..1── Person                   │
            └────────────────────────┼─────────────────────┘
                                     │  (Person is instance-global; account optional)
                         person      │
            ┌────────────────────────┼─────────────────────┐
            │  Person ──holds──▶ Rank (one, from scheme)    │       rank
            └────────────────────────┼─────────────────────┘   ┌─────────────────────────┐
                                     │                         │ RankCategory (ordered)  │
                        membership   │                         │   └─ RankType (ordered) │
            ┌────────────────────────┼───────────────────┐    │        └─ Rank (ordered) │
            │  Membership(person, unit, position,         │    └─────────────────────────┘
            │             effective_from/to)              │
            │        position ─▶ Position (catalog)        │
            └────────────────────────┼───────────────────┘
                                     │
                          tenant     ▼
            ┌──────────────────────────────────────────────┐
            │  Unit (visibility: public|shadow, kind?,      │
            │        state)                                 │
            │     edges: unit ──< parent-of >── unit  (DAG, │
            │            multi-parent, multi-root)          │
            │     closure(ancestor, descendant, depth)      │
            └────────────────────────┬─────────────────────┘
                                     │  closure feeds the PDP
                     authorization   ▼
            ┌──────────────────────────────────────────────┐
            │  Role(atomic permissions[code-defined])       │
            │  RoleAssignment(person, role, target_unit,    │
            │                 scope: unit|subtree)          │
            │  InstanceAdmin(person)  — instance-wide scope │
            │                                               │
            │  PDP(person, action, unit) →                  │
            │     instance-admin perms                      │
            │   ∪ unit-scoped grants at unit                │
            │   ∪ subtree-scoped grants on any ancestor     │
            │   then shadow-visibility gate on reads        │
            └──────────────────────────────────────────────┘
```

Every permission-sensitive transition writes to the [audit](../modules/audit.md) log. The
translatable labels of units, ranks, positions, and roles are localized by the
[localization](../modules/localization.md) module: each entity keeps a stable `code` plus a
default-locale `name`, and responses return all locales as a `locale → text` map (no
Accept-Language negotiation).

---

## The two request paths

### A. A normal mutation (e.g. `PUT /units/{id}`)

1. **transport** receives the request. The inbound bearer token was validated by the
   federation middleware ([identity-federation](../modules/identity-federation.md)),
   producing a **PDP context** `(person, account, request_id)`.
2. transport calls the **PDP**: `authorize(person, "unit.update", unitID)`. On deny → Conjure
   `PermissionDenied` error.
3. On allow, transport calls the tenant **application service**, which runs domain logic +
   the repository adapter inside one transaction.
4. The application emits a **domain event** for cross-module reactions; the [audit](../modules/audit.md)
   write happens in the same transaction as the mutation.
5. transport maps the result to the Conjure response (or a `SerializableError`).

### B. A PDP decision served to an external caller (`POST /authorize`)

1. transport validates the token → PDP context.
2. The authorization application service resolves the decision: collect the person's active
   assignments + instance-admin status; for the requested unit, union `unit`-scoped grants at
   U and `subtree`-scoped grants on every ancestor of U (closure lookup against
   [tenant](../modules/tenant.md)); apply the shadow gate for read actions.
3. Returns an allow/deny (+ the effective permission set for batch/explain variants).
   Results are cacheable **per request** but never across requests — a revoke is immediate.

---

## Where the runtime concerns live

| Concern | Owner doc |
|---|---|
| Stack choices, layering, communication | this file |
| Schema, Go, Conjure, API conventions | [conventions.md](conventions.md) |
| Cross-cutting patterns | [patterns.md](patterns.md) |
| Binding decisions | [decisions.md](decisions.md) |
| Bootstrap, config, observability, schema-version check, `pkg/` | [platform](../modules/platform.md) |
| Migration layout + upgrade safety | [upgrade-safety.md](upgrade-safety.md) |
