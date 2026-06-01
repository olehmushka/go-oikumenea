# Module: audit

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.audit_*`

## Purpose

Owns the **append-only audit trail of every write** (state mutation) across the service (D-Audit).
Every create/update, grant/revoke, role/rank-scheme edit, unit lifecycle transition, membership
change, account link, and localization edit is recorded as an immutable row, correlated by
`request_id` with logs, metrics, and traces. This is both an operational necessity and a deliberate
**governance showcase** for the kind of auditability Palantir-grade deployments expect. Audit emits
via `audit2log` **and** persists to `oikumenea.audit_log`; the table is guarded against mutation.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — one **Object:** the append-only
`AuditEntry`, which **is the Action ledger**: each row records one **Action** and is keyed by that
Action's RID (`action__<type>`; D-Audit). The audit module defines **no** Actions of its own.

- **Audit entry** — an immutable record: who did what to which target, when, in which request,
  with before/after context.

## Data model

Conventions per [conventions.md](../architecture/conventions.md). Append-only — guarded by the
`oikumenea.reject_mutation()` `BEFORE UPDATE OR DELETE` trigger.

**`audit_log`** (append-only)
- `id TEXT` PK = the **Action RID** (`action__<type>`) of the write it records
  (D-ResourceIdentifiers / D-Audit) — self-describing, and chronologically ordered via its
  `uuid_v7()` component
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `actor_type TEXT NOT NULL CHECK (actor_type IN ('person','system'))` — the two actor kinds
  (D-Audit; there is no `super_admin` kind — an instance admin is a `person`)
- `actor_person_id TEXT` — the person who acted (person RID); **NOT NULL when `actor_type='person'`, NULL when
  `'system'`** (CHECK below)
- `subsystem TEXT` — for `system` actions, the originating source (`bootstrap`, `recover-admin`,
  `purge-worker`, `closure-rebuild`, `event-subscriber`, …); **NOT NULL when `actor_type='system'`,
  NULL otherwise**
- `action TEXT NOT NULL` — e.g. `assignment.grant`, `unit.transition`, `rank.scheme.update`
- `target_type TEXT NOT NULL` — e.g. `unit`, `person`, `role_assignment`, `account`, `graph`
- `target_id TEXT` — the acted-on entity's RID (an Object/Link/Action URN)
- `unit_id TEXT` — the unit context of the action where applicable (for scoped audit reads)
- `request_id TEXT NOT NULL` — the correlation key shared with logs/metrics/traces
- `before JSONB`, `after JSONB` — state snapshot / change payload (no secrets; PII minimized) —
  `pii:special` **(ceiling)**: a grab-bag may carry up to special-category data, so tagged at the
  ceiling (D-PIITiers); special-category PII must **not** land here until the envelope seam (DS-29)
  ships
- `outcome TEXT NOT NULL DEFAULT 'success' CHECK (outcome IN ('success','denied','error'))`

Actor-shape CHECK (the two kinds, D-Audit): `(actor_type='person' AND actor_person_id IS NOT NULL
AND subsystem IS NULL)` OR `(actor_type='system' AND actor_person_id IS NULL AND subsystem IS NOT
NULL)`. The install **bootstrap** seed (D-Bootstrap) records as `actor_type='system',
subsystem='bootstrap'`.

An **event-driven cross-module reaction write** — e.g. [document](document.md)'s `PersonPurged`
subscriber erasing the person's document PII, or [order](order.md)'s auto-apply on issue
([membership](membership.md) fill/end and [person](person.md) rank change driven by the per-item
effect events, D-OrderApply) — records as `actor_type='system'`, `actor_person_id=NULL`,
**`subsystem='event-subscriber'`**. It is **not** attributed to the human who triggered the
originating action; instead it is correlated to that action — which carries its own
`person`-attributed row (e.g. the admin's `person.purge` or `order.issue`) — by the shared
**`request_id`** (in-process events run within the originating request context, and for order
auto-apply the same transaction). This is the general rule for any write performed by
an event subscriber rather than by a direct authenticated request.

No `updated_at`/`deleted_at` (immutable). Indexes: `(actor_person_id)`, `(actor_type)`,
`(target_type, target_id)`, `(unit_id)`, `(created_at)`, `(request_id)`.

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
transitions; rank-scheme edits; position-catalog edits; membership create/update/end; document
create/update/delete and document-type edits; order create/**issue**/**revoke** and order-type
edits (issue/revoke are the headline legal-basis events); account and external-identity link/unlink;
person purge; on-demand closure **rebuild** (target a `graph`; D-ClosureIntegrity — closure *verify*
is read-only and not audited). Denied attempts on these are recorded with `outcome='denied'`.

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
