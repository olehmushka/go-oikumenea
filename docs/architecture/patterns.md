# Cross-cutting patterns

Recurring shapes used across modules. Each is named once here; module docs reference them by
name. A pattern earns a place here when it appears in ≥2 modules and its name is meaningful
without a tutorial.

---

## PDP over a closure-backed DAG

Authorization decisions are computed by walking the **unit graphs**, not stored per
(person, unit). The [tenant](../modules/tenant.md) module maintains a transitive-closure
table **per named graph** (D-Graphs); the [authorization](../modules/authorization.md) PDP
resolves a decision by unioning:

- instance-admin permissions (unit-independent),
- `unit`-scoped grants whose `target_unit` is exactly the queried unit (graph-independent),
- `subtree`-scoped grants whose `target_unit` is an **ancestor of the queried unit in that
  grant's graph** (one indexed closure lookup keyed by `(graph, ancestor, descendant)`),

then applying the shadow-visibility gate on read actions. Authority **unions across graphs**
(a unit's `command` chain and its `operational` commander both contribute). No per-permission
filtering inside an assignment; no cross-request caching (a revoke is immediate). This is
drafts' "admin inheritance," generalized from a tree to **several named DAGs** and made
**explicit per assignment**.

Owners: [authorization](../modules/authorization.md), [tenant](../modules/tenant.md).

---

## Instance-scope vs. unit-scope authority

Two distinct authority planes:

- **Unit-scope** — role assignments bound to a `target_unit` (+ `unit`/`subtree` scope; a
  `subtree` grant names the graph it cascades over). Governs everything *about the organization*
  (units, people, memberships, and a unit's positions/billets).
- **Instance-scope** — the instance-admin plane, unit-independent. Governs everything *about
  the deployment itself*: the rank scheme, role definitions, supported locales & translations,
  global config.

A capability lives in exactly one plane. Editing the rank scheme is instance-scope; archiving
a battalion is unit-scope. Keeping them separate is what lets a deeply-placed unit admin be
powerful within their subtree yet unable to touch instance-wide config.

Owners: [authorization](../modules/authorization.md), [rank](../modules/rank.md),
[membership](../modules/membership.md).

---

## Policy-as-data, enforcement-as-code

The **vocabulary of permissions is code** (a closed, compiled, reviewable set — adding one
is a diff). The **bindings are data** (roles compose permissions; assignments bind roles to
units; both are rows). Adding an exclusion or a new role is a data change; adding a new *kind*
of permission is a code change. This keeps the authorization surface visible in code review
while letting operators reconfigure without a deploy.

Owner: [authorization](../modules/authorization.md).

---

## Directory attribute vs. authorization

**Rank** describes a person's seniority for the whole organization; it is **directory data**
and grants **no** authority. **Position** describes what a person does in a specific unit;
also directory data, no authority. Authority comes *only* from role assignments. This
separation is load-bearing: a four-star general has no API permissions unless someone granted
them a role. Documented so no implementer is tempted to branch authorization on rank.

Owners: [rank](../modules/rank.md), [person](../modules/person.md),
[membership](../modules/membership.md), [authorization](../modules/authorization.md).

---

## Acting authority via time-bound role assignment

The temporal corollary of *Directory attribute vs. authorization*. **Acting command, dual-hatting,
and secondment are modeled as a time-bound role assignment — never as a position fill.** A position
is *establishment* (a billet); a role assignment is *authority*. So:

- **Acting.** While the substantive holder is absent (e.g. on leave), grant the stand-in a role
  assignment on the same unit with an `expires_at` (D-TimeBoundGrants). The substantive holder's
  membership/position is **untouched** — the single-billet unique index never fights the acting
  case, because acting is *not* a second fill. When the bound passes the grant **lapses silently**
  (PDP decision-time; no event, no sweep); authority reverts with no edit to the billet.
- **Dual-hatting.** Two concurrent live assignments on different units (or different roles) — the
  union-across-graphs PDP already sums them.
- **Secondment.** A bounded assignment on the host unit while the home-unit membership persists.

Because authority comes *only* from assignments, none of these require vacating a billet, minting a
temporary position, or branching on rank. Showing *both* substantive and acting incumbents **on the
billet itself** is the separate multi-incumbent seam (see [membership](../modules/membership.md)),
not this pattern.

Owners: [authorization](../modules/authorization.md), [membership](../modules/membership.md). See
[decisions.md](decisions.md) D-TimeBoundGrants.

---

## Stable code vs. translatable name

Every structural/catalog entity has two identifiers with different jobs: a stable,
locale-agnostic **`code`** (what external systems reference in their own code — operator-assigned,
unique, immutable by convention) and a human-facing, **translatable `name`** (the default-locale
value in the entity's own column, with other locales in the i18n store). Editing or translating
the name never changes the code, so external references never break.

**Unit exception (D-UnitCodeLifecycle, M28):** a [tenant](../modules/tenant.md) **unit** `code` is
**optional and mutable** (a correctable human-readable ID; a codeless unit is a non-separate
sub-unit) — for units the stable handle external systems reference is the **RID**, not the code.
All other structural/catalog entities follow the immutable-by-convention rule above.

Owners: every module with a structural/catalog entity (tenant, authorization, membership/
position, rank, localization, document, order). See [conventions.md](conventions.md) (Code vs. name).

---

## Translatable label (i18n)

A human-facing label (`name`/`title`/`description`) is rendered in **all enabled locales**: the
owning entity stores the **default-locale** value in its own `name` column (the fallback), and
the [localization](../modules/localization.md) translation store holds the other locales. The
transport layer assembles a **`locale → text` map** for every response — **no Accept-Language
negotiation**. Supported locales are instance-admin-managed data (seeded `ukr` + `eng`). Person
name transliteration is the related-but-separate per-record mechanism (not the admin store).

Owner: [localization](../modules/localization.md); used by
[tenant](../modules/tenant.md), [rank](../modules/rank.md), [membership](../modules/membership.md),
[authorization](../modules/authorization.md), [document](../modules/document.md),
[order](../modules/order.md).

---

## Immutable event log + mutable overlay

History is recorded append-only; current state is a separate column/table. Unit
**lifecycle events** are append-only while `tenant_units.state` holds the current state; the
**audit log** is pure append-only. Append-only tables are guarded by `reject_mutation()`
(see [conventions.md](conventions.md)). Reversal of an action is a *new* event referencing
the original, never an in-place edit.

Owners: [audit](../modules/audit.md), [tenant](../modules/tenant.md).

---

## Audit-on-write

Every **write** (state mutation) records an audit entry in the **same transaction** as the change —
the audit row and the mutation share one fate, so there is no orphan audit and no missing audit.
Denied write attempts are recorded too (`outcome='denied'`). **Reads are not audited.** Because
every entity is audited on write, the audit query is symmetrically filterable by **every audited
entity type** (read ↔ write symmetry): *"the history of person X"*, *"everything that happened in
tenant T"*. This is the runtime obligation behind the storage shape in *Immutable event log +
mutable overlay*; see [decisions.md](decisions.md) D-Audit.

Owners: [audit](../modules/audit.md) + **every** mutating module.

---

## Domain events: atomic vs. notify

Cross-module mutations flow through domain events (`pkg/events`), never a direct cross-module write.
There are **two classes**, and choosing the wrong one is a correctness/scale bug:

- **`atomic`** — the default. Subscribers run **inside the publisher's transaction** (`events.Bus`,
  synchronous, handed the publisher's `pgx.Tx`): the originating write and every effect share one fate.
  Use when the effect *must* be all-or-nothing with the write (order-apply re-homing, merge re-point,
  purge fan-out). **Every event today is `atomic`.** Adding an `atomic` subscriber **widens every
  publisher's transaction** (longer lock lists, more fate-sharing), so it is a **decision-level change**,
  not a routine wiring edit. The bus is **sealed** after boot (`Bus.Seal` in the composition root); a
  `Subscribe` afterwards **panics** (review-2026-07 R-10) — subscribers are wired once, before serving.

  **Purge/merge fan-out width baseline (review R-24).** The person purge/merge fan-out is the widest
  `atomic` transaction: **19 subscriptions across 13 modules** (`cmd/oikumenea/main.go` —
  `SubscribePersonPurge` erases a module's person-owned rows in the purge tx; `SubscribePersonEvents`
  re-homes rows on a merge; the M37 addition is identity-federation's `SubscribePersonPurge`, erasing a
  purged person's login-security-log rows). Two signals keep this from growing invisibly (the M31–M45 pattern of
  subscribers accruing with no number moving): (1) `TestPersonFanoutWidthGuard`
  (`cmd/oikumenea`) pins the subscription count — a module joining the fan-out fails it, forcing a
  conscious decision + a baseline bump; (2) `TestPurgeWidthBudget` (`internal/person`) records the
  actual SQL width of a representative person-family purge — **35 statements** (personprofile
  hard-delete + personsensitive crypto-erase, 2026-07-12) — and asserts a **generous budget (80, ~2×)**
  as a step-change tripwire (catches an N+1 or a heavy new erase handler, not normal drift). What is
  **auto-guarded**: the person-family SQL width + the subscription count. What stays **decision-gated**
  (the whole-fan-out cross-module SQL width isn't measured — it would need most of the composition
  root): use the count guard + this rule when adding a subscriber.
- **`notify`** — after-commit, at-least-once, out of process, via the **transactional outbox**
  (`oikumenea.platform_outbox`, migration 0011_infra; D-EventOutbox). The producer enqueues on its own write
  tx (`events.OutboxWriter.PublishNotify`) so the event commits atomically with the write; the dispatcher
  (`internal/platform/outbox`) drains the queue after commit — claiming rows `FOR UPDATE SKIP LOCKED`
  (replica-safe, like the hermenea worker), retrying with backoff, dead-lettering past max attempts.
  Handlers must be **idempotent** (a crash between commit and dispatch re-delivers). Use for effects that
  must **not** widen the write transaction: webhooks, projections, cache/epoch invalidation, external
  calls. **No `notify` producers exist yet** — the outbox is a live-but-empty proven seam.

The rule of thumb: *"must this effect roll back with the write?"* → `atomic`; *"is this an out-of-band
side effect that should not hold the write's locks?"* → `notify`.

Owners: [platform](../modules/platform.md) (the bus, the outbox dispatcher) + every publishing module;
see [decisions.md](decisions.md) D-EventOutbox and [overview.md](overview.md).

---

## Reversibility everywhere

Destructive operations are soft and reversible within a grace window, never immediate hard
removal:

- units → `suspended` / `archived` state (reversible), hard-purge only after grace,
- persons → deactivate → grace → purge,
- assignments → `revoked_at` flip (the row is retained for audit),

and the reversal is itself an audited action. Encourages decisive operation because mistakes
are correctable, and protects against catastrophic loss.

Owners: [tenant](../modules/tenant.md), [person](../modules/person.md),
[authorization](../modules/authorization.md), [document](../modules/document.md) (soft-delete +
`status` flips), [order](../modules/order.md) (revoke-not-delete).

---

## Shadow-visibility gate

A unit is `public` or `shadow`. A caller sees a `shadow` unit (and memberships/roster derived from
it) only if the PDP grants them a read permission reaching that unit; `public` units are discoverable
subject to normal read permission.

**Enforcement (F-002, A-lite).** This is realized as the authoritative app-layer second pass
(`authorization.FilterVisibleUnits`, reached via `pep.FilterVisibleUnits`) on the **unit-result-set
reads** — tenant `GET /units`, `…/ancestors`, `…/descendants` — applied *after* the permission
decision and mirrored at the DB by a `tenant_units` public-read RLS policy. Other read surfaces
enforce visibility through the **reach projection** rather than this explicit pass: unit-keyed tables
(membership, order) via the reach-keyed RLS policies, and person/document via the read-scope
projection (D-PersonReadScope). Consequently broad `public` discovery is currently a **unit-read
affordance only** — a person/roster in a public unit is *not* broadly readable; it still needs reach.
Extending public discovery to rosters/people is a deferred seam.

Owners: [authorization](../modules/authorization.md), [tenant](../modules/tenant.md),
[membership](../modules/membership.md), [audit](../modules/audit.md),
[document](../modules/document.md), [order](../modules/order.md).

---

## Dormant seam

Ship a column/table intentionally unused (always NULL/empty), reserved so a future capability
is additive rather than a schema rewrite. Used for the future **full-IdP pivot** (password /
2FA / session columns reserved on accounts, kept NULL while authentication is delegated). A
dormant seam is documented as such in the owning module's *Open seams* section.

Owners: [identity-federation](../modules/identity-federation.md),
[authorization](../modules/authorization.md).

---

## Expand / contract migrations

Every release only **adds** (new tables/columns, backfill, dual-write/read). Removal of an
old shape happens in a *later, announced* release after it is unused. Combined with the
`atlas migrate lint` destructive-change gate and CI upgrade tests, this is the mechanism
behind the non-destructive-upgrade guarantee. Full detail in
[upgrade-safety.md](upgrade-safety.md).

Owner: [upgrade-safety.md](upgrade-safety.md); every module's migrations comply.

---

## Declared facet: one dimension, two consumers

A **facet** is one filterable, groupable dimension of an object type — `{key, kind, column,
readPermission, buckets}` — declared **once** by the module that owns the table and consumed
**twice**: as a typed optional query arg on that module's list endpoint, and as a `groupBy` key on
its `GET /<module>/v1/stats/<collection>` endpoint. Both take the same names and the same values, so
a chart segment and a list filter are the same act; the console keeps the whole set in the URL, so
toggling list↔dashboard carries the filters across and clicking a chart is navigation that adds one.
Parity between a facet and its query arg is a build-time drift guard derived from the Conjure IR.

Owners: every module with a listable collection. Binding rationale:
[D-ObjectFacets](decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope);
per-type catalog: [facets.md](facets.md).

---

## Visibility-safe aggregation

A count must be computed **inside** the same predicate that decides which rows the caller may read —
never as a tally over an already-paged result. The distinction is easy to miss because both shapes
look correct on a list: the shadow gate (`FilterVisibleUnits`, applied *after* the keyset page is
cut) legitimately returns a **short page** and never a skipped row, which is right for pagination and
wrong for a count. Person-scoped aggregates therefore fold in the reach semi-join
(`VisiblePersonIDsForSubject*`), unit-scoped aggregates fold the shadow gate into SQL, and
catalog-scoped aggregates need only the endpoint's read gate — the three shapes of
[D-VisibilityScope](decisions.md#d-visibilityscope--one-read-visibility-interface-three-canonical-scope-shapes-registered-per-object-type),
whose `ReadableIDs` id-list trim is the wrong instrument at aggregate scale. The correctness contract
is **differential**, as it is for the trim itself: for a fixed subject and filter set the aggregate's
`totalCount` equals the number of rows exhaustively paged from the owning module's own list endpoint.

Two data-protection rules ride along: a facet may name only a **plaintext** column (envelope-encrypted
`pii:special` values have no facet — nothing to group, and D-DataScope's aggregation rule forbids the
surface), and a facet above `pii:basic` inherits its field's own read code, being **omitted** from the
response for a caller without it rather than zeroed or refused.

Owners: [authorization](../modules/authorization.md), [person](../modules/person.md),
[tenant](../modules/tenant.md), and every module with a `/stats` endpoint.

---

## Maintenance rule

Add a pattern here when it appears in ≥2 modules, its name is self-explanatory, and the
owning module(s) are stable enough to cite. Remove it when it collapses into a single module
and is no longer cross-cutting.
