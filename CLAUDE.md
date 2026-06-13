# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository status: implemented through M15, design-led

This repo is a **modular monolith under active implementation**. Code exists: a `go.mod`, the
`internal/<module>/` Go modules, the `api/*.conjure.yml` contracts, the versioned `migrations/`,
and an optional Next.js console under `web/`. The foundation milestones **M0–M11** are delivered
and the person/rank enrichment milestones **M13–M15** are delivered; **M12** is scoped/in progress;
**M16–M26** are designed (decisions landed) but not yet built. To answer "what stage is feature X
in?" read the **[stage board](docs/milestones.md#stage-board)** — it is the scannable index.

`docs/` remains the **source of truth**, and `docs/architecture/decisions.md` is **binding**: if
code and a decision recorded there disagree, **the code is wrong**. Change a decision by editing
that file (with rationale), not by diverging in code. The **planned-tier (M16–M26)** decisions —
decided/designed but not yet built — live in `docs/architecture/roadmap-decisions.md` (binding
against code once their milestone enters implementation).

**Every feature follows a fixed pipeline** — idea → decided → designed → backend → migrated → ui →
verified. Read **`docs/development-process.md`** before starting, advancing, or reporting on any
feature; it defines the gates and the runbook, and the stage board records each milestone's position.

The docs were derived from a now-deleted FaithMap (church-discovery) source. Prose that mentions
"drafts" / "FaithMap" is **historical provenance only** — that source is gone and is not needed;
do not go looking for a `drafts/` directory.

## What the service is

**go-oikumenea** — a generic, domain-agnostic **personnel & authorization service** (Keycloak-like,
for *hierarchical, multi-tenant* organizations: army / church / university). API-only, self-hosted,
operator-owned PostgreSQL. It is **authorization + directory only**: authentication is delegated to
an external IdP; the service validates inbound identities and **decides** authorization (it is a
PDP). It never stores credentials and never issues tokens.

Differentiator vs. Keycloak: a real **PDP over a unit graph** (units may have multiple parents — a
DAG) with **public/shadow visibility** — not flat, isolated realms.

## Read before doing anything

`docs/README.md` is the entry point and gives the canonical reading order. In short:

1. `docs/glossary.md` — domain vocabulary (the module docs assume these terms).
2. `docs/architecture/decisions.md` — the binding decisions (M0–M15) + carried-over locks;
   `docs/architecture/roadmap-decisions.md` for the planned-tier (M16–M26) decisions.
3. `docs/architecture/conventions.md` — schema / Go-witchcraft / Conjure / API conventions.
4. `docs/ontology-mapping.md` — the binding Object/Link/Action type registry (D-Ontology).
5. `docs/architecture/patterns.md` — cross-cutting patterns.
6. The relevant `docs/modules/*.md` for the task.
7. `docs/development-process.md` — the feature pipeline (gates + runbook) and how stage is tracked;
   `docs/milestones.md` (incl. its **stage board**) for where each milestone sits.

Each `docs/modules/*.md` is self-contained and follows a fixed template: **purpose → entities →
data model → Conjure endpoint sketch → dependencies → authorization touchpoints → patterns →
invariants → open seams**. Preserve that template when adding or editing a module doc, and keep
each module doc readable in isolation.

## Architecture in one screen

Modular monolith, extraction-ready, on the **Palantir OSS stack** (witchcraft / conjure / gödel +
observability libs); this reverses the original `uber/fx` + OpenAPI choice. Hexagonal layering per
module: `transport → application → domain → adapters` — the domain owns its interfaces and imports
no framework. Cross-module **queries** are direct interface calls; cross-module **mutations** are
domain events (keeps the monolith extraction-ready). Planned composition root:
`cmd/oikumenea/main.go`.

Eleven modules (`docs/modules/`):

- **tenant** — units as a DAG (multi-parent, multi-root) + a maintained transitive-closure table;
  `public`/`shadow` visibility; lifecycle. The closure feeds the PDP.
- **person** — instance-global personnel directory; account-optional; holds exactly one rank.
  Carries structured names (incl. patronymic), `birthdate`, ISO-5218 `sex`.
- **membership** — person↔unit belonging; owns **positions** (unit-owned billets that can be vacant).
- **document** — person-held identity papers + personal codes (passport, tax/social-insurance
  number); catalog-typed (`document_types`), metadata only.
- **order** — administrative orders (наказ): the legal basis for status changes (arrival,
  appointment, leave, transfer, discipline, duty); catalog-typed; effects via events + provenance.
- **rank** — the single system-wide rank scheme (category → type → rank, ordered).
- **authorization** — RBAC + the **PDP** (the centerpiece): code-defined permissions, scoped
  assignments, the instance-admin plane.
- **identity-federation** — the external-IdP seam (accounts, external identities, inbound OIDC/JWKS
  validation → PDP context).
- **localization** — i18n: instance-admin-managed supported locales + the translation store.
- **platform** — witchcraft bootstrap, config (ECV + refreshable), observability, schema bootstrap,
  boot-time schema-version check, shared kernel `pkg/`.
- **audit** — append-only audit log of permission-sensitive actions.

## Load-bearing decisions (easy to get wrong)

- **PDP over a DAG, per-assignment scope.** An assignment is `(person, role, target_unit,
  scope∈{unit|subtree})`. `subtree` cascades to all descendants (union over ancestors via the
  closure); `unit` grants children **nothing — not even read**. `target_unit` is independent of
  where the subject sits. No per-permission filtering within an assignment.
- **Rank ≠ permission.** Rank (on `person`) and position (on `membership`) are **directory
  attributes**; authority comes *only* from role assignments. Never branch authorization on
  rank/position.
- **`code` vs `name`.** Every structural entity has a stable, locale-agnostic, immutable-by-convention
  **`code`** (what external systems reference) separate from a **translatable `name`**. Permission
  strings *are* codes.
- **Ontology is the binding model (D-Ontology).** Every entity is an **Object**, a reified **Link**
  (a relationship with its own identity/attributes/history), or an audited **Action**; the **RID**
  encodes the kind (`<object>` / `link__<type>` / `action__<type>`). `docs/ontology-mapping.md` is
  the binding type registry; module docs classify their entities by kind. Never reify a bare FK.
- **i18n: all translations in every response.** Translatable labels are returned as a
  `locale → text` map — **no Accept-Language negotiation**. Supported locales are
  instance-admin-managed (seeded `ukr` + `eng`, ISO 639-3). Person names use per-person
  transliteration variants, *not* the admin translation store.
- **No Postgres RLS** for unit isolation (one org, not mutually-distrusting tenants) — enforcement
  is the app-layer PDP + the shadow-visibility gate on reads.
- **Non-destructive upgrades** are a first-class guarantee: Atlas **versioned** migrations in one
  repo-root `migrations/` dir, an `atlas migrate lint` destructive-change gate, expand/contract
  releases, and a boot-time schema-version check. See `docs/architecture/upgrade-safety.md`.
- **Schema conventions:** one schema **`oikumenea`**; per-module table prefixes
  (`oikumenea.<module>_*`); composed URN **RID** PKs via `new_rid()` (D-ResourceIdentifiers —
  `uuid_v7()` retained as the RID's crypto component); `TIMESTAMPTZ` UTC; soft-delete (`deleted_at`);
  `set_updated_at()` trigger; `reject_mutation()` guard on append-only tables; `TEXT`+`CHECK`
  enums (never native Postgres enums).

## Toolchain

The pinned stack (see `docs/architecture/overview.md`) is:
Go + **gödel** build with `godel-conjure-plugin`; **Conjure** IDL as the API source of truth
(`*.conjure.yml` → generated Go server interfaces/clients + OpenAPI); **witchcraft-go-server**;
**pgx + sqlc** for DB access; **Atlas** versioned migrations; Docker + docker-compose for
packaging. Conjure/sqlc-generated code is never hand-edited.

## Working in the repo

Features move through the pipeline in `docs/development-process.md` (idea → decided → designed →
backend → migrated → ui → verified); update the **stage board** in `docs/milestones.md` for the gate
you pass, and ground every `✅` in a real artifact (a `migrations/` file, a `web/` page, a `D-<Name>`
block) — never from memory.

Coherence is the analog of "tests" for the docs. After editing docs, check that relative links
resolve:

```bash
cd docs && python3 - <<'PY'
import re,os,glob
bad=0
for md in glob.glob('**/*.md',recursive=True):
    base=os.path.dirname(md)
    for m in re.finditer(r'\]\(([^)]+)\)',open(md).read()):
        link=m.group(1).split('#')[0]
        if link and not link.startswith('http') and not os.path.exists(os.path.normpath(os.path.join(base,link))):
            print(f"broken: {md} -> {link}"); bad+=1
print("links OK" if bad==0 else f"{bad} broken")
PY
```

When a doc change introduces or moves a domain entity, keep it owned by **exactly one** module
doc, reflect the change in `decisions.md` if it is a decision, and update `glossary.md` +
`README.md` (module map / reading order) in the same pass. When a milestone advances a gate, update
its `docs/milestones.md` **stage board** row in the same pass.
