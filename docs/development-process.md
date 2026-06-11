# Development process

> Reads: [README](README.md) · [milestones.md](milestones.md) ·
> [open-questions.md](open-questions.md) · [architecture/decisions.md](architecture/decisions.md) ·
> [architecture/conventions.md](architecture/conventions.md) ·
> [architecture/upgrade-safety.md](architecture/upgrade-safety.md).

**Audience: Claude Code.** This is the canonical, repeatable path every feature takes — from a raw
idea to verified, shipped code — and the single place its current stage is recorded. Read this when
you start, advance, or report on any feature. The roadmap ([milestones.md](milestones.md)) sequences
the work; this file says *how* a unit of work moves through the pipeline and *how its stage is tracked*.

## Why this exists

The artifacts of a feature are spread across many files by design (a raw idea, a parked seam, a
binding decision, a module doc, a milestone, Go code, a migration, a UI page). Without one model
tying them together it is easy to lose track of state — e.g. backend shipped but the console left
stale. This process makes the **stage of every feature scannable at a glance** (the
[stage board](milestones.md#stage-board)) and gives the exact gate-by-gate steps to advance one.

## The seven states, six gates

A feature moves left-to-right. Each **gate** has an **exit artifact** (the proof the gate is passed)
and the **docs you touch** to pass it. A feature is "at" the furthest gate it has fully passed.

| State → | Exit artifact (proof the gate passed) | Docs to touch |
|---|---|---|
| **idea** | `## TODO-N · Title [status: idea]` in `todo.md` | `todo.md` only |
| **decided** | a binding `D-<Name>` block in [`decisions.md`](architecture/decisions.md); if the work began as a parked seam, retire its `DS-N` from [`open-questions.md`](open-questions.md) | `decisions.md`, `open-questions.md` |
| **designed** | the module doc ([`modules/*.md`](modules/), fixed template) written/updated; [`glossary.md`](glossary.md) + [`ontology-mapping.md`](ontology-mapping.md) updated; an `M#` row added to [`milestones.md`](milestones.md); the `TODO-N` entry marked `promoted→M#` then deleted | `milestones.md`, `modules/`, `glossary.md`, `ontology-mapping.md`, `todo.md` |
| **backend** | Go module under `internal/<module>/` + `api/<module>.conjure.yml` (server interfaces/clients are **generated**, never hand-edited — D-Conjure / D-Stack) | code |
| **migrated** | one versioned Atlas migration in `migrations/` (expand-only, lint-gated — [upgrade-safety.md](architecture/upgrade-safety.md)); dev DB rebuilt | code |
| **ui** | page(s) under `web/` — or **N/A** (`➖`) for a feature with no console surface (e.g. platform/hardening work) | code |
| **verified** | the milestone's **exit criteria** met: tests pass and the slice boots/migrates/demos end-to-end | stage board → `✅` |

`idea` features are **not** on the [stage board](milestones.md#stage-board) — they live in
`todo.md` until they earn an `M#` at the **designed** gate. The board holds only `M#`
features. `decided` and `designed` can land in either order in practice, but a feature is not
`designed` until both its decision and its `M#` row exist.

## Two entry points

Every feature converges at the **decided** gate from one of two sources:

1. **A raw idea** — captured in `todo.md` (the brainstorm scratchpad). Use this for
   net-new domain ideas that have not yet been weighed.
2. **A parked seam** — a `DS-N` entry in [`open-questions.md`](open-questions.md) (a known future
   seam with a recorded *default* and *trigger*). Promoting a seam is the same as promoting an idea,
   minus the brainstorm step.

When the decision is written, the originating `TODO-N` / `DS-N` is **retired** (see below).

## The stage board (single source of truth for *stage*)

[`milestones.md` → Stage board](milestones.md#stage-board) is the **authoritative index of stage**:
one row per `M#`, one column per gate. Per-milestone prose in the same file carries the **detail**
(revision references, scope notes, resolved open questions) — the board says *where it is*, the prose
says *what it is*. Keep them consistent; when they disagree, the board (grounded in real artifacts)
wins, and the prose should be corrected.

Legend: `✅` done · `🚧` in progress · `⬜` not started · `➖` not applicable.

**Ground every `✅` in a real artifact.** A `✅` under *Migrated* means a file exists in
`migrations/`; under *UI* means a page exists in `web/`. Never mark a gate from memory or intent —
verify the artifact. (Memory can go stale: a "UI drift" note may already be resolved by later work,
or a `delivered` prose line may hide a missing surface. Check.)

## Runbook — advancing a feature

These steps map 1:1 to the gates. Do them in order; stop at whichever gate the current task targets,
and update the [stage board](milestones.md#stage-board) cell you just passed.

1. **Capture (→ idea).** Add `## TODO-N · Title [status: idea]` to `todo.md` with a
   freeform body. Create the file if absent; its absence means "no pending ideas".
2. **Decide (→ decided).** Weigh it; if accepted, write a `D-<Name>` block to
   [`decisions.md`](architecture/decisions.md) in the existing `Decision / Why / Consequence` shape.
   If it came from a `DS-N` seam, **remove** that entry from [`open-questions.md`](open-questions.md)
   (numbers are never reused). Refine an existing decision via an `Amended by D-<Name>` chain rather
   than diverging.
3. **Design (→ designed).** Write or extend the owning [`modules/*.md`](modules/) doc to the fixed
   template (**purpose → entities → data model → Conjure sketch → dependencies → authorization
   touchpoints → patterns → invariants → open seams**); keep each entity owned by **exactly one**
   module. Register new Objects/Links/Actions in [`ontology-mapping.md`](ontology-mapping.md) and add
   terms to [`glossary.md`](glossary.md). Add the `M#` row to [`milestones.md`](milestones.md) (table,
   prose section, **and** stage-board row). Mark the `TODO-N` `promoted→M#`, then delete it once the
   `M#` row exists.
4. **Build backend (→ backend).** Add/extend the `api/<module>.conjure.yml` contract first, generate,
   then implement `internal/<module>/` across `transport → application → domain → adapters`. Follow
   [`conventions.md`](architecture/conventions.md). Audit-on-write for permission-sensitive mutations.
5. **Migrate (→ migrated).** Add one expand-only versioned migration to `migrations/` per
   [`upgrade-safety.md`](architecture/upgrade-safety.md); pass the destructive-change lint gate; rebuild
   the dev DB.
6. **Build UI (→ ui).** Add the console surface under `web/` (BFF over the public API). If the feature
   has no console surface, mark the gate `➖`.
7. **Verify (→ verified).** Meet the milestone's exit criteria: tests pass; the slice boots, migrates,
   and demos. Flip the board row to `✅` across the line.

After any step, re-run the coherence check (the link-checker in [`CLAUDE.md`](../CLAUDE.md)) when docs
changed.

## Promotion & retirement rules

- **`TODO-N`** — stable per-idea id; `[status: idea]` → `[status: promoted→M#]` when a milestone is
  written → **deleted** from `todo.md` once the `M#` row exists. `todo.md` may legitimately not exist.
- **`DS-N`** — a parked seam; when promoted to a decision it is **removed** from
  [`open-questions.md`](open-questions.md) (its outcome lives in `decisions.md`); the number is never
  reused or renumbered (gaps are expected).
- **`D-<Name>`** — binding once written. It is not "pending/proposed"; parked work lives in
  `open-questions.md`, not as a draft decision. Supersede via `Amended by D-<Name>`.
- **`M#`** — append-only roadmap ids; a milestone's life is tracked by its **stage-board row**, not by
  deletion.

## Where each artifact lives (quick map)

| Stage | Artifact | Location |
|---|---|---|
| idea | raw idea | `docs/todo.md` (`TODO-N`) — optional, may not exist |
| (parked) | future seam | [`docs/open-questions.md`](open-questions.md) (`DS-N`) |
| decided | binding decision | [`docs/architecture/decisions.md`](architecture/decisions.md) (`D-<Name>`) |
| designed | domain model | [`docs/modules/*.md`](modules/), [`ontology-mapping.md`](ontology-mapping.md), [`glossary.md`](glossary.md) |
| designed | roadmap + stage | [`docs/milestones.md`](milestones.md) (`M#` row + [stage board](milestones.md#stage-board)) |
| backend | code + contract | `internal/<module>/`, `api/<module>.conjure.yml` |
| migrated | schema | `migrations/<timestamp>_<desc>.sql` |
| ui | console | `web/` |
