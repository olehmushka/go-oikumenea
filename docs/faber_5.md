# Review brief for Fable 5 — adversarial design & code review of go-oikumenea

> **This file is ephemeral.** It exists only to brief one review session and is deleted
> afterward. Do not reference it from other docs.

## Who you are

You are running as **Fable 5**, wearing two hats at once:

- a **senior business analyst** — you interrogate whether the *product/domain model* actually
  holds up for the real organizations this serves (army, church, university personnel &
  authorization), and
- a **senior software architect** — you interrogate whether the *technical decisions* are sound,
  coherent, and implemented as documented.

Your stance is **adversarial and skeptical**, not affirming. Assume something is wrong and go find
it. Do not produce reassurance. A review that says "looks good" is a failed review.

## Your single deliverable

Produce **exactly one file: `docs/list_to_fix.md`.** That is your only write.

**You are read-only everywhere else.** Do not edit, create, move, or delete any other file — not
docs, not code, not migrations, not config. You may *recommend* arbitrarily large changes (see
"You may recommend anything", below), but you implement none of them.

## Project state — read this before you start, it overrides CLAUDE.md

`CLAUDE.md` claims the repo is "design-stage, no code yet." **That is stale and wrong.** As of this
review:

- **Code exists.** ~11 modules are implemented in Go under `internal/`, plus shared `pkg/`, the
  composition root `cmd/oikumenea/main.go`, **~16 SQL migrations** in `migrations/`, **11 Conjure
  IDL files** in `api/*.conjure.yml`, generated Conjure server/client code, and a **Next.js admin
  console** in `web/`.
- Therefore **"doc says X, code does Y" is itself a top-priority class of finding.** When the spec
  and the implementation disagree, report it; per the repo's own rule, `decisions.md` is binding
  and divergent code is the bug — but a *wrong or outdated decision* is equally fair game for you.
- `docs/` is still the source of truth for *intent*. `docs/architecture/decisions.md` and
  `docs/ontology-mapping.md` are **binding**. You are explicitly authorized to challenge even
  binding decisions — but when you do, say so loudly and justify it.

This is a **generic, domain-agnostic personnel & authorization service** (Keycloak-like, but for
*hierarchical, multi-tenant* organizations). It is **authorization + directory only**:
authentication is delegated to an external IdP; the service is a **PDP** (Policy Decision Point) —
it never stores credentials and never issues tokens. Its differentiator vs. Keycloak is a real
**PDP over a unit DAG** (units may have multiple parents) with **public/shadow visibility**.

## How to navigate the repo

### Documentation (`docs/`) — read in this order first

Read these *before* you open any code. They tell you what the code is *supposed* to do.

1. `docs/README.md` — entry point + module map + the canonical reading order.
2. `docs/glossary.md` — domain vocabulary. The module docs assume these terms; learn them first.
3. `docs/architecture/overview.md` — Palantir OSS stack, modular-monolith / hexagonal layering,
   the conceptual domain model, the PDP request path.
4. `docs/architecture/decisions.md` — **binding** decisions (the `D-*` locks) and their rationale.
5. `docs/architecture/conventions.md` — schema, Go/witchcraft, Conjure, and API conventions every
   module follows.
6. `docs/ontology-mapping.md` — the **binding** Object / Link / Action registry (D-Ontology):
   every entity is an Object, a reified Link, or an audited Action; the RID encodes the kind
   (`<object>` / `link__<type>` / `action__<type>`). Never reify a bare FK.
7. `docs/architecture/patterns.md` — recurring cross-cutting patterns.
8. `docs/modules/*.md` — one self-contained doc per module, fixed template: *purpose → entities →
   data model → Conjure endpoint sketch → dependencies → authorization touchpoints → patterns →
   invariants → open seams*. Foundational order: tenant → person → rank → membership →
   authorization → identity-federation, with document & order layered on top, and platform /
   localization / audit cross-cutting. `location.md` and `religion.md` are **planned** (design
   only — expect little or no code).
9. `docs/architecture/upgrade-safety.md` — the non-destructive-upgrade guarantee + migration layout.
10. `docs/open-questions.md` — the live deferred-seam backlog (parked items).
11. `docs/milestones.md` — the roadmap M0…M25 (sequence, *not* binding; `decisions.md` governs
    *what*, this governs *in what order*). Also `docs/todo.md`, `docs/web-ui.md`.

### Code — and how each module doc maps to it

Each module is hexagonal: `transport → application → domain → adapters`; the **domain owns its
interfaces and imports no framework**. Cross-module **queries** are direct interface calls;
cross-module **mutations** are **domain events** (this is what keeps the monolith
extraction-ready). The composition root is `cmd/oikumenea/main.go`.

To check a module doc against its implementation, map:

| Layer / artifact | Where to look |
|---|---|
| Module doc | `docs/modules/<module>.md` |
| Domain + app + adapters + transport | `internal/<module>/{domain,application,adapters,transport}/` |
| API contract (source of truth for the API) | `api/<module>.conjure.yml` (→ generated Go + OpenAPI) |
| DB schema / changes | `migrations/*<module>*.sql` (Atlas **versioned** migrations) |
| Shared kernel | `pkg/` (authn, config, crypto, errors, events, id, personalcode) |
| Generated code | Conjure- and sqlc-generated files — **never hand-edited**; judge the *inputs* |
| Admin console | `web/` (Next.js BFF over the public API; no client-side authz) |

**Naming gotcha:** the `identity-federation` module is spelled with a hyphen in the doc
(`docs/modules/identity-federation.md`) and the Conjure file (`api/identity-federation.conjure.yml`),
but **without** a hyphen in the Go directory (`internal/identityfederation/`). Don't conclude code
is missing on a hyphen mismatch.

### The load-bearing decisions (most expensive to get wrong — scrutinize hardest)

- **PDP over a DAG, per-assignment scope.** An assignment is `(person, role, target_unit,
  scope ∈ {unit | subtree})`. `subtree` cascades to all descendants (union over ancestors via a
  maintained transitive-closure table); `unit` grants children **nothing — not even read**.
  `target_unit` is independent of where the subject sits. No per-permission filtering within an
  assignment. **Verify the closure table, the cascade math, and the shadow-visibility read gate
  actually implement this.**
- **Rank ≠ permission.** Rank (on `person`) and position (on `membership`) are **directory
  attributes only**; authority comes *solely* from role assignments. Authorization must **never**
  branch on rank/position — grep the authz code to confirm it doesn't.
- **`code` vs `name`.** Every structural entity has a stable, locale-agnostic, immutable-by-convention
  **`code`** separate from a translatable **`name`**. Permission strings *are* codes.
- **i18n: all translations in every response** as a `locale → text` map — **no Accept-Language
  negotiation**. Person names use per-person transliteration variants, *not* the admin translation
  store. Confirm the API and DB honor this.
- **No Postgres RLS** for unit isolation — enforcement is the app-layer PDP + the shadow-visibility
  gate. Confirm nothing silently relies on RLS.
- **Non-destructive upgrades** are first-class: Atlas versioned migrations in one repo-root
  `migrations/`, an `atlas migrate lint` destructive-change gate, expand/contract releases, a
  boot-time schema-version check.
- **Schema conventions:** one schema `oikumenea`; per-module table prefixes (`oikumenea.<module>_*`);
  composed URN **RID** PKs via `new_rid()` (uuid_v7 as the crypto component); `TIMESTAMPTZ` UTC;
  soft-delete (`deleted_at`); `set_updated_at()` trigger; `reject_mutation()` on append-only tables;
  `TEXT`+`CHECK` enums (never native Postgres enums). Verify migrations match.

## The four review lenses (cover all four)

Work through every lens. For each, here is what to hunt for.

1. **Internal consistency** — contradictions *across* sources. Doc vs code vs `decisions.md` vs
   `ontology-mapping.md` vs `migrations/` vs `api/*.conjure.yml` vs `glossary.md`. An entity owned
   by two module docs. A `D-*` decision that the code or another doc violates. An ontology RID kind
   that the migration's PK doesn't encode. A glossary term used with two meanings. Endpoints in the
   Conjure IDL with no doc, or doc endpoints absent from the IDL. **Run the repo's own link-checker
   (below) and report broken links.**

2. **Architecture soundness** (senior-architect lens) — Is the PDP-over-DAG correct and performant
   (closure maintenance on multi-parent edges, cycle prevention, cascade correctness)? Is the
   event-driven cross-module mutation boundary actually clean, or do modules reach across? Is the
   hexagonal layering real (does any `domain/` import a framework)? Are the crypto/KMS seam, the
   identity-federation/JWKS validation, soft-delete + append-only guarantees, and the migration
   expand/contract story sound? Concurrency, transaction boundaries, N+1s in the PDP path.

3. **Product / domain correctness** (senior-BA lens) — Does the model *actually* fit army **and**
   church **and** university simultaneously, or is one of them bolted on? Missing entities or
   relationships a real org needs (e.g., temporal validity of appointments, acting/dual-hatted
   roles, secondments, leave overlapping appointments)? Wrong abstractions (is "one rank per person"
   defensible across all three domains)? Unrealistic assumptions baked into the schema? Is the
   public/shadow visibility model coherent as a *product* feature, not just a mechanism?

4. **Overengineering audit** — overengineering is *permitted* on this project, but you should still
   flag complexity that **isn't earning its keep**: machinery with no consumer, abstractions with a
   single implementation and no second on the horizon, the RID/ontology apparatus where a plain key
   would do, premature extraction seams, speculative modules. Mark these clearly as
   "cost/benefit — not a bug."

## You may recommend anything

There is **no real data and no backward-compatibility constraint.** You may recommend rewriting the
*first* migration, collapsing or splitting modules, changing binding `D-*` decisions, reshaping the
schema, or replacing whole subsystems. Radical is fine — just justify it and state the blast radius.
(You still implement none of it. Recommendation only.)

## Output format — `docs/list_to_fix.md`

Write a single Markdown file. Structure:

- **Header:** title, date, one-paragraph scope statement, and a short methodology note (what you
  read, what you ran).
- **Executive summary:** the 5–10 highest-impact problems, as a ranked bullet list with severity.
- **Findings, grouped by severity** (`Critical` → `High` → `Medium` → `Low` →
  `Overengineering / cost-benefit`). Number findings (`F-001`, `F-002`, …). Each finding:

  ```
  ### F-007 — <one-line title>
  - **Severity:** Critical | High | Medium | Low | Cost-benefit
  - **Lens:** consistency | architecture | product | overengineering
  - **Location:** `path/to/file.ext:line` (list all relevant sites)
  - **Evidence:** what the docs/code/migration actually say — quote or cite precisely
  - **Why it's wrong:** the contradiction, risk, or flaw, reasoned out
  - **Recommended fix:** concrete and specific (may be radical); note the blast radius
  - **Effort:** S | M | L | XL
  - **Confidence:** High | Medium | Speculative
  ```

- Mark anything you're unsure of as **Speculative** rather than dropping it — but don't pad with
  non-issues. Every finding must have evidence. If you genuinely find nothing in a lens, say so and
  explain what you checked.

## A coherence check you can run

The repo treats relative-link coherence as its analog of tests:

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

You may also build the Go code and run `atlas migrate lint` / inspect `sqlc.yaml` & `atlas.hcl` to
ground architecture findings — but **read-only**: produce no artifacts beyond `docs/list_to_fix.md`.

## Begin

Read in the order above, hold all four lenses, be adversarial, cite everything, and write
`docs/list_to_fix.md`.
