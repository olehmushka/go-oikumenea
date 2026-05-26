# Module: audit

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.audit_*`

## Purpose

Owns the **append-only audit trail** of permission-sensitive actions across the service. Every
grant/revoke, role/rank-scheme edit, unit lifecycle transition, membership change, and account
link is recorded as an immutable row, correlated by `request_id` with logs, metrics, and traces.
This is both an operational necessity and a deliberate **governance showcase** for the kind of
auditability Palantir-grade deployments expect. Audit emits via `audit2log` **and** persists to
`oikumenea.audit_log`; the table is guarded against mutation.

## Entities & aggregates

- **Audit entry** — an immutable record: who did what to which target, when, in which request,
  with before/after context.

## Data model

Conventions per [conventions.md](../architecture/conventions.md). Append-only — guarded by the
`oikumenea.reject_mutation()` `BEFORE UPDATE OR DELETE` trigger.

**`audit_log`** (append-only)
- `id` PK (`uuid_v7()` — also gives chronological ordering)
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `actor_person_id UUID` — the subject who acted (nullable for system actions)
- `action TEXT NOT NULL` — e.g. `assignment.grant`, `unit.transition`, `rank.scheme.update`
- `target_type TEXT NOT NULL` — e.g. `unit`, `person`, `role_assignment`, `account`
- `target_id UUID`
- `unit_id UUID` — the unit context of the action where applicable (for scoped audit reads)
- `request_id TEXT NOT NULL` — the correlation key shared with logs/metrics/traces
- `before JSONB`, `after JSONB` — state snapshot / change payload (no secrets; PII minimized)
- `outcome TEXT NOT NULL DEFAULT 'success' CHECK (outcome IN ('success','denied','error'))`

No `updated_at`/`deleted_at` (immutable). Indexes: `(actor_person_id)`, `(target_type,
target_id)`, `(unit_id)`, `(created_at)`, `(request_id)`.

## How modules record audit

- The **audit application service** exposes `Record(ctx, entry)` called by other modules'
  application services **inside the same transaction** as the mutation, so the audit row commits
  iff the change commits (no orphan or missing audit).
- Alternatively, modules emit domain events ([overview.md](../architecture/overview.md)) that an
  audit subscriber persists; the in-transaction path is preferred for permission-sensitive
  writes to guarantee atomicity.
- `request_id` is pulled from the request context (set by [platform](platform.md)).

### What must be audited (non-negotiable)

Role create/update/delete; assignment grant/revoke; instance-admin grant/revoke; unit lifecycle
transitions; rank-scheme edits; position-catalog edits; membership create/update/end; account
and external-identity link/unlink; person purge. Denied attempts on these are recorded with
`outcome='denied'`.

## Conjure API surface

`AuditService` (read-only — there is no write endpoint; writes happen in-process):

| Op | Intent | Perm |
|---|---|---|
| `GET /audit` | Query the log (filter by actor/target/unit/time, token-paginated) | `audit.read` |
| `GET /audit/{id}` | Read one entry | `audit.read` |

Audit reads are unit-scoped where a `unit_id` is present (an admin reads audit reaching their
subtree); instance-scope admins read all. Reads pass the shadow gate for `shadow`-unit context.

## Dependencies

- **Calls:** [platform](platform.md) (DB, `audit2log`, request context). Reads no domain
  module.
- **Called by:** **every** module's application layer (`Record(...)`), in-transaction. Consumes
  domain events as a backstop.

## Authorization touchpoints

Defines/gates: `audit.read`. Audit recording is not itself permission-gated (it is a side effect
of an already-authorized — or explicitly denied — action); reading the log is.

## Invariants & safety

- **Append-only.** No `UPDATE`/`DELETE` from application code; `reject_mutation()` enforces it
  at the DB. A correction is a **new** entry, never an edit.
- **Atomic with the change** for in-transaction recording — the audit row and the mutation share
  a fate.
- **No secrets, minimal PII** in `before`/`after`; person references are by id (the
  [person](person.md) purge tombstone keeps ids resolvable-or-redacted after erasure).
- `request_id` ties every entry to the full request across logs/metrics/traces.

## Open seams / future

- **Retention via partitioning:** `audit_log` is a natural range-partition-by-`created_at`
  target; a partition + cold-archive policy is an additive seam (carried concept from drafts,
  not built yet).
- **PII envelope** (encrypt attributed free-text with per-row keys, erase by key deletion) is a
  reserved enhancement if richer PII ever lands in audit payloads.
- A streaming/export sink (SIEM) sits naturally behind `audit2log`.
