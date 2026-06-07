# go-oikumenea — architecture documentation

> **Audience: Claude Code.** These docs describe the architecture of a system that does
> not exist as code yet. They are written to be read by an AI agent that will later
> implement it. Every module doc is self-contained: you can read one without reading the
> others. When you implement, treat `architecture/decisions.md` as binding.

## What go-oikumenea is

A generic, domain-agnostic **personnel & authorization service** — Keycloak-like, but for
**hierarchical, multi-tenant organizations** (an army, a church, a university). It is:

- **API-first.** The contract is [Conjure](architecture/overview.md) IDL; clients and an
  [OpenAPI reference](api/README.md) are generated from it. The service ships **no UI of its
  own**, but an **optional, standalone** Next.js admin console
  ([`web-ui.md`](web-ui.md), [D-WebUI](architecture/decisions.md)) can be run beside it as a
  pure API consumer — opt-in, separately deployed, no coupling into the core.
- **AuthZ + directory, not authentication.** Authentication is delegated to an external
  identity provider (IdP). go-oikumenea validates inbound identities and **decides
  authorization** — it is a Policy Decision Point (PDP). It never stores credentials or
  issues tokens. See [`identity-federation`](modules/identity-federation.md).
- **Person-centric.** A `person` is the core aggregate (the personnel directory). A login
  `account` is an *optional* attachment — rosters of people who never log in are
  first-class.
- **Single-domain per deployment.** One running instance serves exactly one domain (army
  **or** church **or** university). There is no org-type discriminator in the data; the
  domain is a deployment property.
- **Self-hosted, operator-owned data.** The operator supplies their own PostgreSQL and
  credentials. Schema name: **`oikumenea`**.
- **Built on the Palantir OSS stack** (witchcraft / conjure / gödel + the observability
  libraries) — deliberately, as a reference implementation of that stack.

### The differentiator vs. Keycloak

Keycloak realms are flat and isolated. Here, **units form graphs** (a unit may have
**multiple parents** — a DAG, not a strict tree — across **several named hierarchies** such as
administrative `command` and operational chains, NATO ADCON/OPCON-style), role/permission grants
**inherit down a chosen graph** under explicit per-assignment scope, and units carry a
**public/shadow visibility** boundary. A real **PDP** resolves all of this, unioning authority
across graphs. That — *hierarchy + inheritance + visibility, decided by a PDP* — is the product.

## The modules

| Module | Responsibility |
|---|---|
| [tenant](modules/tenant.md) | The organization as **units** in multiple named hierarchies (per-graph DAGs, multi-parent), visibility, lifecycle. |
| [person](modules/person.md) | The instance-global **personnel directory**. CLDR names, citizenship & residence. Holds rank. Account-optional. |
| [membership](modules/membership.md) | `person ↔ unit` assignment. Carries **position** (the unit billet). |
| [document](modules/document.md) | Person-held **papers** (metadata) + **encrypted national-identifier codes** (passport, tax/social-insurance number). Catalog-typed. |
| [order](modules/order.md) | Administrative **orders** (наказ) — the legal basis for status changes (arrival, appointment, leave, transfer, discipline, duty). |
| [rank](modules/rank.md) | The single system-wide **rank scheme** (category → type → rank). Directory seniority only. |
| [authorization](modules/authorization.md) | RBAC + the **PDP**. Roles, code-defined permissions, scoped assignments, instance-admin. |
| [identity-federation](modules/identity-federation.md) | The external-IdP seam: accounts, external identities, inbound OIDC/JWKS validation. |
| [localization](modules/localization.md) | i18n: instance-admin-managed locales + the translation store for entity labels. |
| [platform](modules/platform.md) | witchcraft bootstrap, config, observability, schema bootstrap, country registry, crypto/KMS seam, boot-time schema-version check. |
| [audit](modules/audit.md) | Append-only audit trail of permission-sensitive actions. |

A **consumer** of the above (not a backend module), documented alongside them:

| Surface | Responsibility |
|---|---|
| [web-ui](web-ui.md) | **Optional** standalone Next.js **admin console** (port 8445). BFF over the public API; Keycloak login; no client-side authz. |

## Reading order for a new agent

1. **This file** — what it is, the module map.
2. [`glossary.md`](glossary.md) — the domain vocabulary. Read it before anything else; the
   module docs assume these terms.
3. [`architecture/overview.md`](architecture/overview.md) — the Palantir stack, the
   modular-monolith / hexagonal layering, the conceptual domain model, the PDP request path.
4. [`architecture/decisions.md`](architecture/decisions.md) — the binding decisions
   (what is locked and why). If code and a decision disagree, the code is wrong.
5. [`architecture/conventions.md`](architecture/conventions.md) — schema, Go/witchcraft,
   Conjure, and API conventions that every module follows.
6. [`ontology-mapping.md`](ontology-mapping.md) — the **binding Object / Link / Action registry**
   (D-Ontology): the authoritative catalog of the typed Objects, Links, and Actions the modules
   define. Module docs conform to it.
7. [`architecture/patterns.md`](architecture/patterns.md) — recurring cross-cutting patterns.
8. The relevant [`modules/*.md`](modules/) for the work at hand. Foundational order:
   **tenant → person → rank → membership → authorization → identity-federation**, with
   **document** and **order** (person-held papers / administrative acts) layered on
   person+membership, and **platform**, **localization**, and **audit** as cross-cutting.
9. [`architecture/upgrade-safety.md`](architecture/upgrade-safety.md) — the
   non-destructive-upgrade guarantee and the migration layout.
10. [`open-questions.md`](open-questions.md) — the live backlog for the next planning session: the
   deferred-seam list (parked items, each promotable to a milestone). Resolved seams are removed
   from it; their outcomes live in [`architecture/decisions.md`](architecture/decisions.md).
11. [`milestones.md`](milestones.md) — the implementation roadmap: the architecture sequenced into
   buildable, dependency-ordered milestones (M0…M11). A roadmap, not binding — `decisions.md` governs
   *what*, this governs *in what order*.

## Provenance

This design is derived from `drafts/` (a locked, religion-specific church-discovery design
called *FaithMap*) by extracting its reusable IAM core and **reversing** its `uber/fx` +
OpenAPI stack choice in favour of the Palantir OSS stack. `drafts/` is reference material
only — do not build from it directly. What was carried over vs. dropped is recorded in
[`architecture/decisions.md`](architecture/decisions.md).

## Status

Design-complete at the architecture level. **No application code exists yet.** The build
sequence is in [`milestones.md`](milestones.md) (M0…M11, dependency-ordered). Until then, when
asked to "find the code that does X", the answer is "it does not exist yet — the design is here."
