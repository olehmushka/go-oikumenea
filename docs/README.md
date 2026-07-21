# go-oikumenea — architecture documentation

> **Audience: Claude Code.** These docs describe the architecture of a system that is now
> **under active implementation** (a modular monolith: see `internal/`, `api/`, `migrations/`,
> `web/`). They remain the **source of truth** that the code is held to. Every module doc is
> self-contained: you can read one without reading the others. When you implement, treat
> `architecture/decisions.md` as binding, and follow the feature pipeline in
> [`development-process.md`](development-process.md) — the [stage board](milestones.md#stage-board)
> shows where each milestone sits.

## What go-oikumenea is

A **personnel directory + multi-domain registry / intelligence platform** for **hierarchical,
multi-tenant organizations** (an army, a church, a university, a government, a company). Its
authorization plane is Keycloak-like (a PDP over a unit graph); on top of the directory it is a
**registry** holding rich, permission-gated subject data whose boundary and compliance posture are
governed by **[D-DataScope](architecture/decisions.md)**. It is:

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
  domain is a deployment property. *(Refined for the religion vertical, D-Religion: the single
  domain may be "religion", within which many faiths/traditions coexist as **catalog data + units
  in graphs** — still no org-type discriminator branched on in code.)*
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
| [platform](modules/platform.md) | witchcraft bootstrap, config, observability, schema bootstrap, country registry + the WOF `geo_places` gazetteer (D-GeoPlaces, M16), crypto/KMS seam, boot-time schema-version check. Hosts the generic **`POST /import/{objectType}`** upsert endpoint + the `import.manage` **service principal** the hermenea companion calls (M16). |
| [audit](modules/audit.md) | Append-only audit trail of permission-sensitive actions. |
| [search](modules/search.md) | **Unified cross-type object search** (D-UnifiedSearch, review-2026-09): one `searchObjects` endpoint fanning in the per-module trigram queries, permission-gated + **visibility-trimmed** per type (D-VisibilityScope). Owns no tables. |
| [links](modules/links.md) | **Generic object-link traversal** (D-LinkTraversal, review-2026-09): one `getObjectLinks` + depth‑1 `searchAround` endpoint fanning in the reified link tables over a pkg/rid-derived, boot-asserted descriptor registry, per-arm permission-gated + neighbor **visibility-trimmed** (D-VisibilityScope). Owns no tables. |

> **person is internally three Go modules** behind the one Conjure `PersonService`
> (**D-PersonModuleSplit**, review-2026-07 R-09): [person](modules/person.md) core (identity, names,
> bio, ranks, read-scope, merge/purge) + [personprofile](modules/personprofile.md) (contacts, social,
> relationships, addresses, non-encrypted institutional ties) + [personsensitive](modules/personsensitive.md)
> (physical identity/ethnicity, overlays, watchlists, encrypted party — the whole crypto surface). A
> code split only: one schema, one API contract; [person.md](modules/person.md) stays the entity owner.

**Companion service** (a **second binary**, not an oikumenea `internal/` module):

| Service | Responsibility |
|---|---|
| [hermenea](modules/hermenea.md) *(M16, `cmd/hermenea`)* | Out-of-process **ingestion + scheduler**, its **own Postgres**, HTTP-only coupling: fetch (http/file connector) → raw staging → mapper → oikumenea `POST /import/{objectType}` idempotent upsert; cron + `worker_jobs` queue; `import_runs` lineage. **D-Hermenea** supersedes D-Worker and folds the M17 data-ingestion framework. |

**Client SDKs** (generated build outputs + thin hand-written façades, not `internal/` modules):

| SDK | Responsibility |
|---|---|
| [clients](modules/clients.md) *(M27, `clients/go/` + `clients/typescript/`)* | Unified **Go + TypeScript SDKs** generated from the same Conjure contract (**D-ClientSDK**); each adds a one-call façade (`client.New` / `createOikumeneaClient`) binding one base URL + token to every service, including the `hermenea`/`import` endpoints oikumenea proxies. The web console consumes the TS SDK. |

**Planned modules** (designed in [milestones.md](milestones.md) M16–M39 + [roadmap-decisions.md](architecture/roadmap-decisions.md); most module docs follow at implementation time — the **religion**, shared **location**, **company**, and **external-organizations** docs already exist):

| Module | Responsibility |
|---|---|
| [language](modules/language.md) *(M18 — backend + migrated, UI deferred)* | Languages & writing systems as a Glottolog-faithful registry (the full 5.3 forest, import-loaded) + ISO-15924 scripts; person/unit/locale language ties. |
| [location](modules/location.md) *(M19)* | A shared, standalone place entity: PostGIS coordinate + app-derived MGRS + multi-format coordinate input + structured address. Reused by education, company, and religion sites. |
| `education` *(M20)* | Educational institutions, internal structure, buildings, and person bindings (enrollment, mentorship, groups, dorm stays, positions). |
| [company](modules/company.md) *(M21)* | A legal-entity registry with the ownership/affiliation graph (legal form, registration, positions, founders, shareholders, UBO). |
| [religion](modules/religion.md) *(M22–M25)* | The **multi-faith** religion vertical: faith taxonomy (religions→traditions), organizations as tenant units in religion graphs, clergy grades/credentials, lay affiliation (`pii:special`), and discovery (sites→Location, schedules, search). Catalog-driven, no hard-coded faith vocabulary. |
| [vehicle](modules/vehicle.md) *(M26)* | A vehicle registry binding people & companies to vehicles: brand/model/type taxonomy, the vehicle (VIN), a temporal brand↔Company manufacturer link, and the ownership+plate registration (polymorphic person\|company owner, plate region via the shared WOF `geo_places` gazetteer). |
| [external-organizations](modules/external-organizations.md) *(M30)* | A registry of external organizations (party / government body / foreign military / NGO / lobbying registrant) — the node-space the M33 person↔org institutional ties point at when the org is neither an operator unit nor an M21 company. Catalog-typed, provisional/resolved, hermenea-fed. |
| [finance](modules/finance.md) *(M44)* | Bank accounts (envelope-encrypted **IBAN**) & payment cards (envelope-encrypted **PAN**, BIN/last-4 display, **no CVV**) as authoritative directory data; a polymorphic person\|company holder link; a **bank is a `company`-domain tenant-org** (M21/M41), not a new entity. |
| [connector](modules/connector.md) *(M53)* | The **connector plane** — the core-side registry of the connector fleet (`Connector`/`Source`/`SyncRun` Objects) that generalizes hermenea into one of a family of agents. **Visibility, not orchestration**: connectors self-register and report their sync runs; the core displays and audits, never schedules. Plus the pull-wiring read codes and the generalized on-demand-lookup seam (D-ConnectorPlane). |

The remaining planned milestones extend existing modules rather than adding new ones: the
**person-intelligence / OSINT-enrichment cluster** (M29, M31–M36; [draft_superbrain_schema.md](draft_superbrain_schema.md))
enriches [person](modules/person.md); the login security log (M37) extends
[identity-federation](modules/identity-federation.md). **M38** (criminal/legal records) and **M39**
(compensation/payroll) are **deferred stubs** — designed in their own later sessions.

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
4. [`architecture/decisions.md`](architecture/decisions.md) — the binding decisions for the built /
   in-progress surface (M0–M15): what is locked and why. If code and a decision disagree, the code is
   wrong. The **planned-tier (M16–M26)** decisions live in
   [`architecture/roadmap-decisions.md`](architecture/roadmap-decisions.md) (decided/designed, not yet
   built; binding once their milestone enters implementation). The
   **[north star](architecture/north-star.md)** records the target-state topology those decisions
   converge on (headless core · facades · service principals · connector plane · data packs;
   M51–M54).
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
   buildable, dependency-ordered milestones (M0…M26). A roadmap, not binding — `decisions.md` governs
   *what*, this governs *in what order*. Its **[stage board](milestones.md#stage-board)** is the
   scannable index of where every milestone sits in the pipeline.
12. [`development-process.md`](development-process.md) — the **feature pipeline**: the gates a feature
   passes (idea → decided → designed → backend → migrated → ui → verified), the runbook to advance one,
   and how the stage board is kept honest. Read it before starting or reporting on any feature.

## Provenance

This design is derived from `drafts/` (a locked, religion-specific church-discovery design
called *FaithMap*) by extracting its reusable IAM core and **reversing** its `uber/fx` +
OpenAPI stack choice in favour of the Palantir OSS stack. `drafts/` is reference material
only — do not build from it directly. What was carried over vs. dropped is recorded in
[`architecture/decisions.md`](architecture/decisions.md).

## Status

Design-complete at the architecture level. **No application code exists yet.** The build
sequence is in [`milestones.md`](milestones.md) (M0…M14, dependency-ordered). Until then, when
asked to "find the code that does X", the answer is "it does not exist yet — the design is here."
