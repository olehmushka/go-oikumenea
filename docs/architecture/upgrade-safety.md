# Upgrade safety & migration layout

The flagship operational guarantee: **`git pull` + upgrade never loses operator data.** The
operator owns their PostgreSQL, can back it up and inspect it at any time; on top of that, the
release process is designed so that no upgrade can silently destroy data. This is both a
product promise and a Palantir-grade compliance posture. Decision reference:
[decisions.md](decisions.md) D-Migrations + L-UpgradeSafe.

---

## Migration layout

- **Tool: [Atlas](https://atlasgo.io/), versioned mode.** Not declarative auto-diffing —
  versioned migrations are deterministic, reviewable, and forward-only, which is what the
  no-data-loss guarantee requires.
- **One location: `migrations/` at the repo root.** A single, linearly-versioned directory
  for the whole service (no per-module migration dirs — that complexity from `drafts/` is
  dropped). Files are timestamp-or-sequence ordered, e.g.
  `migrations/20260601090000_create_tenant_units.sql`, with an `atlas.sum` integrity file.
- **`atlas.hcl`** at the repo root configures the dev database (for diff/lint), the migration
  dir, and the `oikumenea` schema scope.
- **sqlc reads the same schema** so generated Go types and the migrations never diverge.
- **Authoring loop:** edit a schema HCL/SQL desired-state (or write the migration directly),
  `atlas migrate diff` against the dev DB to generate the versioned file, review the SQL,
  `atlas migrate lint` locally, commit.

The first migration creates the schema bootstrap objects owned by
[platform](../modules/platform.md): the `oikumenea` schema, `uuid_v7()`, `set_updated_at()`,
`reject_mutation()`, and the `citext` extension, plus the **schema-version table**
(`oikumenea.schema_version`).

---

## The four guarantees

### 1. Expand / contract (parallel-change) only

A release only **adds**: new tables, new nullable columns, new indexes, backfills, and
dual-write/dual-read code. **Removing** or **narrowing** anything (drop column/table, tighten a
`CHECK`, add `NOT NULL` without a default, rename) happens **only in a later release**, after
the old shape is provably unused, and is announced in `UPGRADING.md`.

A column rename, for example, is three releases: (a) add the new column + dual-write; (b)
backfill + switch reads; (c) drop the old column once nothing reads it.

**Enabling RLS is an expand/contract change (D-RLSDefenseInDepth).** Turning on Row-Level
Security can lock the application out of its own tables, so it ships in stages: (a) enable RLS on
the target tables with a **permissive policy** while the per-transaction `app.*` GUC seam
([platform](../modules/platform.md)) is deployed and verified setting the GUCs on every request;
(b) **tighten** the policies to the `app.readable_units`/`app.writable_units` predicates once the
GUCs are reliably present; ensure the application DB role lacks `BYPASSRLS` only after (b). The
`atlas migrate lint` review must treat a policy tightening as the contract step (announced in
`UPGRADING.md`), since a tightening that outruns the GUC plumbing is an availability regression.

> **First release (revision `0012_rls`).** Because go-oikumenea has **never been released**, the GUC
> wiring and the tightened policies ship **atomically in one revision** — there is no live deployment
> for a tightening to outrun, so the (a)/(b) staging collapses on a fresh install. The staged
> permissive→tighten rollout above applies to any **post-v1** RLS change. See
> [decisions.md](decisions.md) D-RLSDefenseInDepth *Enablement timing*.

### 2. Destructive-change gate in CI

`atlas migrate lint` runs on every PR with destructive-change detection enabled. Any
drop/narrowing **fails the build**. Override requires an explicit, reviewed annotation in the
migration and a matching `UPGRADING.md` entry — there is no silent path to data loss.

### 3. Upgrade tests in CI

A CI job spins a fresh Postgres, applies migrations **from each prior released version**, seeds
representative data, then applies migrations up to `HEAD` and asserts **invariants hold and row
counts are preserved** (nothing silently dropped). This catches a destructive change that
slipped past lint and verifies real upgrade paths, not just clean installs.

### 4. Boot-time schema-version check

`oikumenea.schema_version` records the schema revision the running binary expects. On startup
the [platform](../modules/platform.md) module compares the DB's applied revision to the
binary's expected range:

- DB **older** than the binary expects → the binary may run migrations (operator-gated) or
  refuse, per config.
- DB **newer/unknown** than the binary expects → **refuse to start** (a witchcraft health
  reporter reports unready) rather than risk writing against a schema it does not understand.

This prevents a rolled-back binary from corrupting a forward-migrated database.

---

## Defense in depth from the domain model

Even setting migrations aside, the data model resists accidental loss:

- **Soft delete** (`deleted_at`) on user-facing entities — a delete is reversible.
- **Reversibility** windows on units (`suspended`/`archived` before purge) and persons
  (deactivate → grace → purge).
- **Append-only audit** — every permission-sensitive action is recoverable as history,
  guarded by `reject_mutation()`.

See [patterns.md](patterns.md) (Reversibility, Immutable event log).

---

## Operator guidance (to land in `UPGRADING.md` at implementation time)

- Always `pg_dump` before upgrading (the operator owns the DB; this is their cheap insurance).
- Read the `UPGRADING.md` entry for the target version — it lists any contract step
  (deprecations becoming removals) and the expand/contract timeline.
- Apply migrations as a pre-start step; the boot-time check is the backstop, not the plan.
- Roll back the binary freely *within* a schema revision range; do **not** run an older binary
  against a newer schema (the boot check will refuse it).
