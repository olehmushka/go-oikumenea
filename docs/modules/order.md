# Module: order

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.order_*`

## Purpose

Owns **administrative orders** (наказ) — the formal acts that are the **legal basis** for a change in
a person's status (D-Orders). Where a [document](document.md) records *what a person holds*, an order
records *an act the organization performs*: arrival, appointment, leave, transfer, discipline, duty
roster. An order is issued by a **unit**, carries a number and date, and contains one or more
**order items**, each targeting a person and (where the type calls for it) a unit/position/rank.

Modelled on the Ukrainian-army **"накази по стройовій частині"** — five families captured as the
order-type **category** (D-Orders):

1. **personnel lists** — arrival / enrollment; removal (transfer / discharge / demobilization);
2. **appointments** — appoint to a (vacant) position; dismiss from a position;
3. **leave & travel** — annual / family / medical leave; business trip; training;
4. **discipline & incentives** — reprimand; rank deprivation; rank award; gratitude; bonus;
5. **duty rosters** — daily detail, duty officer & assistants.

Like document types, the order **type** vocabulary is an **instance-admin-managed catalog** (stable
`code` + translatable `name`, D-Code / D-i18n), so each deployment adds its own order kinds without a
code change.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Order`,
`Order type` (catalog), `Order item` (parent-scoped; bears the `TARGETS` link via its `target_*` FKs).
**Actions:** `CreateOrder`, `AddOrderItem`, **`IssueOrder`** and `RevokeOrder` — audited,
`action__<type>` RID. `IssueOrder` is the strongest ontology fit: one atomic Action whose effects are
emitted as domain events applied in the same transaction, citing `order_item_id` provenance.

- **Order type** (catalog) — a stable `code` + translatable `name`, plus a **`category`** (the five
  families) and an **`effect`** declaring the downstream consequence of items of that type
  (`membership-start` | `membership-end` | `rank-change` | `record-only`).
- **Order** (aggregate root) — the order header (наказ): number, date, issuing unit, lifecycle.
- **Order item** — one affected person/action within an order (the unit of effect).

(People live in [person](person.md); units/positions in [tenant](tenant.md)/[membership](membership.md);
ranks in [rank](rank.md). An order only *points* at them. Translatable type `name`s live in the
[localization](localization.md) store.)

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`order_order_types`** (instance-admin catalog)
- `id` PK
- `code TEXT NOT NULL UNIQUE` — stable, locale-agnostic identifier (D-Code), immutable by convention
  (e.g. `arrival`, `appoint`, `dismiss`, `leave-annual`, `business-trip`, `reprimand`, `rank-award`,
  `duty-detail`) — `pii:none`
- `name TEXT NOT NULL` — default-locale label; **translatable** via [localization](localization.md)
- `category TEXT NOT NULL CHECK (category IN ('personnel-list','appointment','leave-travel',
  'discipline-incentive','duty-roster'))` — the five UA-army families
- `effect TEXT NOT NULL CHECK (effect IN ('membership-start','membership-end','rank-change',
  'record-only'))` — the downstream consequence of items of this type (drives which target columns
  an item must carry; see invariants)
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`
- `sort_order INT`
- `created_at`, `updated_at`, `deleted_at`

Seeded mapping: arrival → `membership-start`; removal/dismiss/discharge → `membership-end`;
rank-award/rank-deprivation → `rank-change`; leave / business-trip / reprimand / gratitude / duty →
`record-only`.

**`order_orders`** (the order header / наказ)
- `id` PK
- `number TEXT` — the order number (within issuing unit + year) — `pii:none`
- `issued_on DATE` — the order's date — `pii:none`
- `issuing_unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT` — the unit that
  issues it; **`NOT NULL`** — every order is unit-issued (no instance-level orders), which anchors the
  unit-scope authz check and the RLS predicate (both key on `issuing_unit_id`; D-Orders, I-5)
- `status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','issued','revoked'))`
- `revoked_by_order_id TEXT REFERENCES order_orders(id)` — the later order that revoked this one
- `revoked_at TIMESTAMPTZ`
- `created_at`, `updated_at`, `deleted_at`
- **Uniqueness:** `UNIQUE (issuing_unit_id, number) WHERE number IS NOT NULL AND deleted_at IS NULL`
  (an order number is unique within its issuing unit; operators that re-use numbers per year scope
  the number string accordingly).
- Index `(issuing_unit_id) WHERE status='issued'`.

**`order_order_items`** (one affected person/action)
- `id` PK
- `order_id TEXT NOT NULL REFERENCES order_orders(id) ON DELETE CASCADE` — child of its order
- `type_id TEXT NOT NULL REFERENCES order_order_types(id) ON DELETE RESTRICT`
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT` — the affected person
- `unit_id TEXT REFERENCES tenant_units(id) ON DELETE RESTRICT` — target unit (for
  arrival/transfer/appointment); nullable
- `position_id TEXT REFERENCES membership_positions(id) ON DELETE RESTRICT` — target billet (for
  appoint/dismiss); nullable
- `rank_id TEXT REFERENCES rank_ranks(id) ON DELETE RESTRICT` — target rank (for rank-change);
  nullable
- `effective_from DATE`, `effective_to DATE` — when the act takes/ceases effect (`DATE`s) — `pii:none`
- `note TEXT` — free-text detail (reason, reference) — **`pii:basic`** (minimized; no secrets)
- `created_at`, `updated_at` — **no `deleted_at`**: an item has **no independent lifecycle**, it is
  **parent-scoped** (see *Lifecycle & immutability*). Reads resolve items only through a non-deleted
  parent order; `ON DELETE CASCADE` is the FK-integrity backstop for the (design-forbidden) hard
  delete of a parent, not a routine path.
- Index `(person_id)`, `(order_id)`.

Which target columns are required is checked in the application against the type's `effect`
(`membership-start`/`-end` need `unit_id` and/or `position_id`; `rank-change` needs `rank_id`;
`record-only` needs none). All columns except `note` are `pii:none` (D-PIITiers).

### Lifecycle & immutability

An order is **mutable while `draft`** (header + items may be edited). On **issue** it becomes a legal
document: the header and its items are **locked** (corrections are made by an *amending* order; a
mistake is undone by a **revoking** order that sets `revoked_by_order_id`), **and its structural
effects are auto-applied in the issue transaction** (see *How an order takes effect*; D-OrderApply). This follows the existing
**reversibility** pattern (status flip + audit), not the hard `reject_mutation()` append-only guard —
hard legal immutability of issued orders is a noted hardening seam. The one permitted post-issue
mutation is `issued → revoked` (+ `revoked_at`, `revoked_by_order_id`), itself audited.

**Items are parent-scoped** — they have no `deleted_at` and no lifecycle of their own. While the order
is `draft`, items may be freely added/removed/edited as part of editing the draft (a removed draft
item is genuinely gone — the draft has no legal weight yet). On **issue** the items lock with the
header; an issued item is never independently deleted — a mistake is undone by **revoking the whole
order**. Soft-deleting an order (`order_orders.deleted_at`) therefore takes its items with it
logically: item reads always join the parent and honor the parent's `deleted_at`/`status`.

### PII governance

`order_order_items.note` is the only `pii:basic` field (it may name/describe a person); every other
column is `pii:none` (structured references by id). Unlike [document](document.md), which **erases**
its PII on `PersonPurged`, **order does not subscribe to `PersonPurged` to erase `note`** — and the
asymmetry is deliberate. An **issued order is an immutable legal record** (наказ, the *legal basis* for
a status change; "issued is locked", above), so it follows the **same tombstone exception as the
[audit](audit.md) log**: the record is **retained intact** through a person purge, with PII held to the
"minimized, no secrets" discipline and the purged person kept **resolvable-or-redacted via the
[person](person.md) purge tombstone** (the item's `person_id` is a `pii:none` id reference, not erased).
Retention rests on the legal-basis/recordkeeping ground, not on convenience; this is why orders differ
from documents (mutable, self-asserted metadata that *is* erased). Log order/person identifiers with
`werror.UnsafeParam` (redacted), never raw.

## How an order takes effect (provenance + events)

Per the locked rule that **cross-module mutations flow through domain events** (extraction-ready;
[overview.md](../architecture/overview.md)), an order does **not** synchronously write into other
modules' tables. Instead, structural effects are **auto-applied on issue** by each owning module's
subscriber, **within the issue transaction** (D-OrderApply):

- **Issuing an order emits `OrderIssued`** plus one **granular, effect-typed event per item**, by the
  item type's `effect`. These are the order's *intent* events (`*Ordered`); each subscriber then
  emits its own module's *fact* event:

  | Item `effect` | Order emits | Subscriber → action | Provenance |
  |---|---|---|---|
  | `membership-start` | `AppointmentOrdered` | [membership](membership.md) creates the membership (fills the position / plain belonging) → emits `MembershipCreated` | `membership_memberships.order_item_id` FK |
  | `membership-end` | `RemovalOrdered` | [membership](membership.md) ends the target membership → emits `MembershipEnded` | order item cited (FK on the ended row) |
  | `rank-change` | `RankChangeOrdered` | [person](person.md) sets `rank_id` → emits `PersonRankChanged` | audit payload (rank is a person *column*, not a row) |
  | `record-only` | *(none)* | authoritative as the order item itself — no downstream write | — |

- **`record-only` items** (leave, business trip, discipline, duty roster) are **authoritative as the
  order item itself** — they have no other module representation today.

**Same-transaction, all-or-nothing.** The subscribers run synchronously in the issue transaction, so
the order row and every effect share one fate. If any effect violates a target module's invariant
(e.g. the position is already filled), the **whole issue rolls back**, the order stays `draft`, and
that module's domain error surfaces (e.g. `Membership:PositionAlreadyFilled`). Each module keeps
enforcing its own invariants in its own write path. Effects land **at issue**;
`effective_from`/`effective_to` are **legal metadata only**, not a scheduler trigger (future-dated
*scheduled* application is parked behind the worker runtime — DS-25). Subscriber writes audit as
`actor_type='system', subsystem='event-subscriber'`, correlated to the issuer's `order.issue` row by
`request_id` ([audit](audit.md)). This needs **neither** the worker runtime (DS-25) **nor** a broker
(DS-26) — synchronous in-process dispatch is the in-process `pkg/events` bus.

## Conjure API surface

`OrderService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /units/{unitId}/orders` | Create a draft order (+ items) for an issuing unit | `order.create` (on the unit) |
| `PUT /orders/{id}` | Edit a draft order/items (rejected once issued) | `order.create` (on the unit) |
| `POST /orders/{id}/issue` | Issue a draft order (locks it; emits `OrderIssued` + per-item effect events; auto-applies structural effects in the same txn) | `order.issue` |
| `POST /orders/{id}/revoke` | Revoke an issued order (via a revoking order) | `order.revoke` |
| `GET /units/{unitId}/orders` | List a unit's orders (token-paginated) | `order.read` + shadow gate |
| `GET /orders/{id}` | Read one order (+ items) | `order.read` + shadow gate |
| `GET /persons/{personId}/orders` | Orders affecting a person (via items) | `order.read` + shadow gate |
| `GET /order-types` | List the type catalog | `order.type.read` |
| `POST /order-types` | Add an order type | `order.type.manage` |
| `PUT /order-types/{id}` | Edit an order type (code immutable by convention) | `order.type.manage` |

Type `name` is returned as a `locale → text` map. Order reads/writes are **unit-scoped on
`issuing_unit_id`** and pass the **shadow-visibility gate**. Editing or issuing an already-issued
order → `Order:AlreadyIssued`; revoking a non-issued order → `Order:NotIssued`. **Issue is
all-or-nothing**: if an auto-applied effect violates a target invariant the whole issue rolls back
(the order stays `draft`) and that module's error surfaces (e.g. `Membership:PositionAlreadyFilled`).
**Revoke does not reverse applied effects** — it is a legal-status flip; undo is expressed by the
revoking order's own items.

## Dependencies

- **Calls:** [person](person.md) (item person exists), [tenant](tenant.md) (issuing/target unit
  exists + visibility for the gate), [membership](membership.md) (target position),
  [rank](rank.md) (target rank), [localization](localization.md) (assemble the type `name`
  locale-map). [platform](platform.md) for infra. Emits `OrderIssued`, `OrderRevoked` (and per-item)
  events.
- **Called by:** read surfaces listing a unit's / person's orders; [membership](membership.md) (a
  fill/end may cite an `order_item_id` as provenance); [audit](audit.md).

## Authorization touchpoints

Defines/gates: `order.create`, `order.read`, `order.issue`, `order.revoke` — all **unit-scoped on the
issuing unit** (a unit's orders are managed by admins with authority over that unit/subtree); reads
pass the shadow gate. Plus the instance-plane catalog permissions `order.type.manage` (write) +
`order.type.read` (reference read). The module never decides access — it calls the PDP, and **never
reads rank/position to make a decision** (D-Rank / D-Position). An order is the legal *record* of an
act; it does not itself confer authority.

## Patterns

- **Stable code vs translatable name** for `order_types` (D-Code + D-i18n).
- **Instance-scope catalog vs unit-scope data**: the type catalog is instance-admin-managed; orders
  are unit-issued and unit/subtree-scoped.
- **Cross-module mutation via events** ([overview.md](../architecture/overview.md)): on issue, orders
  emit granular per-item effect events that membership/person subscribers apply **in the issue
  transaction**, citing the order item as provenance — no synchronous cross-module write (D-OrderApply).
- **Audit-on-write**: order create/issue/revoke and type-catalog edits record in-transaction
  ([audit](audit.md), D-Audit). Issue/revoke are the headline legal-basis events.
- **Reversibility**: an issued order is undone by a revoking order, not a delete.

## Invariants & safety

- An order belongs to **one issuing unit** (`issuing_unit_id NOT NULL` — **no instance-level
  orders**) and has **≥1 item**; each item targets **exactly one person**. The non-null issuing unit
  is what makes every `order.*` permission's unit-scope decision and the RLS `issuing_unit_id`
  predicate well-defined.
- **Draft is editable; issued is locked.** The only post-issue transition is `issued → revoked`
  (audited, via a revoking order).
- **Items are parent-scoped, never independently deleted.** Order items carry no `deleted_at`; their
  lifecycle is the order's (added/removed only while `draft`; locked on issue; visible only through a
  non-deleted parent). `ON DELETE CASCADE` is FK-integrity insurance, not a routine deletion path.
- The target columns an item must carry are determined by its type's **`effect`**
  (`membership-start`/`-end` → unit/position; `rank-change` → rank; `record-only` → none).
- An order takes effect on other modules **only via events + provenance links**, never a synchronous
  cross-module write; `record-only` items are authoritative as themselves.
- **Issue is all-or-nothing** (D-OrderApply): all structural effects are applied by subscribers in
  the issue transaction; any effect that violates a target invariant rolls the whole issue back (the
  order stays `draft`). Effects land at issue — `effective_from`/`effective_to` are legal metadata,
  not a scheduler trigger.
- **Revoke does not cascade**: revoking an issued order flips its legal status only and does **not**
  auto-reverse applied effects; reversal is carried by the revoking order's own items.
- An order item's `note` is `pii:basic` (minimized, no secrets); person/unit references are by id, so
  the [person](person.md) purge tombstone keeps them resolvable-or-redacted after erasure. Orders are
  **not** purge-erased (unlike [document](document.md)): an issued order is an immutable legal record
  and follows the [audit](audit.md)-style **tombstone exception** (retained intact; see *PII
  governance*).
- The type catalog is **instance-admin-managed**; codes are immutable by convention; a type in use
  cannot be hard-deleted (it is *retired*).
- An order grants **no** authorization.
- **RLS backstop.** `order_orders` carries the defense-in-depth RLS policy keyed on `issuing_unit_id`
  against `app.readable_units` / `app.writable_units` (D-RLSDefenseInDepth), mirroring the
  `membership_*` pattern — behind the authoritative PDP + shadow gate, not a replacement.
  `order_order_items` has no unit column and is **parent-scoped**: it is exempt from the direct
  predicate and reached only through its (covered) parent `order_orders` (a parent reach-join is a
  noted hardening seam).

## Open seams / future

- **Future-dated / scheduled effects** — auto-apply is **on issue** (D-OrderApply); applying an item
  at its `effective_from` instead (a future-dated наказ that takes effect later) is parked behind the
  worker runtime (open-questions DS-25).
- **First-class leave/absence** entity — today leave/business-trip are `record-only` order items
  (DS-35).
- **First-class discipline/incentive** records — today reprimand/gratitude/bonus are `record-only`
  items (DS-36).
- **Duty-roster as ephemeral assignments** — today the daily detail is a `record-only` item (DS-37).
- **Hard legal immutability** of issued orders (a `reject_mutation()`-guarded append-only form) is a
  hardening seam beyond the by-convention lock.
