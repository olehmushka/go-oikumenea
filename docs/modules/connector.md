# Module: connector

> **Status: building (M53 / D-ConnectorPlane).** The registry (this module's core), the wiring API,
> and the generalized lookup seam land in M53. Binding design lives in
> [roadmap-decisions.md](../architecture/roadmap-decisions.md) (D-ConnectorPlane); the target topology
> is [north-star.md](../architecture/north-star.md) plane 4.

## Purpose

The **connector plane** generalizes [hermenea](hermenea.md) from *the* companion into **one of a
family of connectors** — external services that feed or answer for oikumenea, each with its own
storage and scheduler, coupled to the core **over HTTP only** (the D-Hermenea boundary, kept). This
module is the core-side **registry** of that fleet: `Connector` and `Source` become first-class
Objects, and connectors report their `SyncRun`s so operators can see what has been happening from the
core, where they already look.

The governing property is **visibility, not orchestration**. The core records what a connector
*reports*; it never schedules, triggers, pauses or retries a run — scheduling lives inside the
connector. D-ConnectorPlane rejects core-side orchestration outright (its alternative (a)), because it
would put connector operational state and outbound coupling into the core and widen the blast radius.

**Altitude note.** "Connector" here is a *whole deployable agent*. hermenea's internal `Fetcher` seam
(a fetch strategy — http/file/wof-sqlite) carried this name through M52 and was renamed in M53 so the
two never collide (see [hermenea](hermenea.md) → Patterns).

## Entities & aggregates

- **`Connector`** (Object, RID `20,1,1`) — a registered agent. Stable `code`; `principal_id` → the
  M51 [service principal](identity-federation.md) it authenticates as (what makes a report
  attributable); `status ∈ {active, disabled}`; `last_seen_at` liveness hint; soft-delete.
- **`ConnectorSource`** (Object, RID `20,1,2`) — one dataset a connector syncs, as reported. `code`
  unique within its connector; `object_type` names the core import target for push-mode sources
  (NULL for lookup-only sources); `schedule` is the connector's own string, verbatim, never parsed.
  A **read model**: the connector's own source table stays authoritative for execution.
- **`SyncRun`** (Object, RID `20,1,3`) — one reported execution. `state ∈ {running, succeeded,
  failed}`; connector-supplied `created`/`updated`/`skipped` counts; `external_run_id` (the
  connector's own run id — hermenea's `import_runs.id`, the same value M49 chunked envelopes carry as
  `runId`) correlates a run with the connector's ledger and per-row import provenance.

## Data model

`migrations/20260601000040_connector_plane.sql` (RID service **20**):

- `connector_connectors` — RID PK via `new_id(20,1,1)`; `code` partial-unique among live rows;
  `principal_id` FK → `account_service_principals` **ON DELETE RESTRICT** (a principal naming a
  connector must be disabled, not deleted, so audit keeps resolving); `set_updated_at()`; soft-delete.
- `connector_sources` — `new_id(20,1,2)`; `connector_id` FK **CASCADE**; `(connector_id, code)`
  partial-unique; soft-delete (not hard, so a source's run history survives a rename/retire).
- `connector_sync_runs` — `new_id(20,1,3)`; `source_id` FK **CASCADE**; `(source_id, external_run_id)`
  partial-unique so a re-report is idempotent; a CHECK ties `finished_at` to a terminal `state`.

Nothing here is RLS-guarded — the registry is instance-plane operator data with no person or
organization dimension.

## Conjure API surface

`api/connectors.conjure.yml` → `ConnectorService`, base-path `/connectors/v1`. Two audiences:

- **Machine self-service** (a connector, gated on codes it holds): `PUT /registration`
  (`registerConnector` — idempotent, binds the row to the **calling** principal, replaces the declared
  source set) and `POST /sync-runs` (`reportSyncRun` — idempotent on `(source, externalRunId)`). A
  connector never names another principal; the core takes it from the request context.
- **Operator reads** (`connector.read`): `GET /connectors`, `GET /connectors/{id}`,
  `GET /connectors/{id}/sources`, `GET /sync-runs` (fleet or per-source, newest first).

There is deliberately **no** endpoint to trigger, pause or retry a run.

### The other two modes (same milestone, other modules)

D-ConnectorPlane names three interaction modes; this module owns the registry that underpins them:

- **push** — bulk data in via the existing chunked `POST /import/{objectType}` ([dataimport], M49),
  unchanged. `import.manage` stays the push gate.
- **pull-wiring** — connector → core *reads* on a narrow, permission-gated **wiring API**
  (natural-key → RID resolution, reference-catalog reads, own cursors). Each surface is its own
  instance-scope code — `wiring.resolve`, `wiring.catalog.read`, `wiring.cursor.read` — held by a
  service principal as a grant, never a default.
- **on-demand lookup** — core → connector synchronous calls with a mandatory deadline (R-12): the M34
  [watchlist](../modules/hermenea.md) check generalized into one typed per-kind connector-call seam.
  The shared `internal/connectorcall` package owns the two invariants every such call needs — the
  mandatory deadline (applied in `Dial`, so a call site cannot omit it) and the null-object discipline
  (an unconfigured kind is an explicit "disabled" implementation, never a nil seam). Each lookup KIND
  is a per-kind interface owned by its consumer's domain (person's `WatchlistLookup` is the first),
  late-bound in `main.go`. **Binding a new lookup kind is still a `main.go` line** — the seam is
  late-bound by design; `connectorcall` removes the transport boilerplate, not that line.

## Dependencies

- [identity-federation](identity-federation.md) — the M51 service principal a connector authenticates
  as; `principal_id` FKs to it.
- [authorization](authorization.md) — the per-surface permission codes and the `RequireService` PEP.
- [audit](audit.md) — every write records an Action under the reporting principal (M51 machine actor).
- [platform](platform.md) — the RID registry, the pool.
- Registers [hermenea](hermenea.md) as the first connector.

## Authorization touchpoints

- `connector.register` / `connector.report` — self-service codes, held by machine subjects; a human is
  denied by `RequireService`. Instance-scope (a connector has no unit or, in M53, organization reach).
- `connector.read` — operator fleet-view code, satisfied for an instance admin (RequireServiceOrPerson).
- `wiring.*` — the pull-wiring read codes; instance-scope, one per surface.

## Invariants

- **A registry row is always bound to a real principal, taken from the caller** — never from the wire.
  A connector cannot register or report as another (the anti-impersonation guard: a foreign code under
  a different principal is a CONFLICT).
- **Reporting is idempotent** on `(source, externalRunId)` — connectors retry with backoff.
- **The core never orchestrates.** No trigger/retry/pause exists, by design.
- **Registration is convergent**: re-registering replaces the declared source set (declared-away
  sources are retired), so a connector reconciles the registry by simply re-registering at boot.

## Open seams

- **Org-owned operational data + the RLS service arm is M55**, not here. M53's wiring API is *reads*
  over instance-wide reference data; a connector still gets no reach into RLS-protected,
  organization-owned data (scraped units, memberships, …). See D-ServiceIdentities and M55.
- **A new *lookup* connector needs a line in `main.go`** to bind its seam (late binding, by design), so
  the "second connector, zero core changes" property holds for push and pull-wiring connectors.
- Additional connector *fetch strategies* (jdbc-sql, object-store) remain DS-44, inside hermenea.
