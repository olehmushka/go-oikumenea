# Module: membership

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.membership_*`

## Purpose

Owns two related things: **positions** (the unit's billets) and **memberships** (people filling
them / belonging to units). A **position is a unit-owned billet** — it belongs to one unit and
**exists whether or not anyone fills it** (a **vacancy** is an active, unfilled position;
D-Position). A person **belongs** to a unit through a **membership**, which **optionally
references a position** (the billet that person holds); a membership can also be position-less
("just belongs to this unit"). One person may hold many memberships across many units, some
`public` and some `shadow`. Like rank, **position grants no authorization** — it is directory
data ([patterns.md](../architecture/patterns.md), Directory attribute vs. authorization).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Position` (a
billet that **exists while vacant**). **Links:** the temporal `link__member_of`/`link__fills` (the
membership, carrying `effective_from`/`effective_to`). **Actions:**
`CreatePosition`/`AbolishPosition`, `CreateMembership`/`EndMembership` — audited, `action__<type>`
RID; order-driven changes arrive via [order](order.md)'s `IssueOrder` events citing `order_item_id`.

- **Position** (aggregate root) — a billet belonging to a unit: a stable `code`, a translatable
  `title`, an optional `required_rank` (the establishment expectation), and a status. Vacant
  until filled.
- **Membership** — a person's belonging to a unit, optionally filling a position, with effective
  dates. The act of **filling** a position is a membership that references it.

Visibility is **not stored** on either entity — it **derives from the unit's** `visibility`
([tenant](tenant.md)).

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`membership_positions`** (unit-owned billets)
- `id` PK
- `unit_id TEXT NOT NULL REFERENCES tenant_units(id) ON DELETE RESTRICT` — the owning unit
- `code TEXT NOT NULL` — **stable, locale-agnostic** identifier (D-Code); unique within the unit
  (`UNIQUE (unit_id, code) WHERE deleted_at IS NULL`); immutable by convention
- `title TEXT NOT NULL` — default-locale title; **translatable** via [localization](localization.md)
- `required_rank_id TEXT REFERENCES rank_ranks(id) ON DELETE RESTRICT` — optional establishment
  expectation (advisory; not enforced against the filler's rank)
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','abolished'))`
- `sort_order INT`
- `created_at`, `updated_at`, `deleted_at`
- Index `(unit_id) WHERE status='active'`.

**`membership_memberships`** (belonging / filling) *(Link `link__member_of`)*
- `id` PK — RID, `link__member_of` entity-type slot
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `unit_id   TEXT NOT NULL REFERENCES tenant_units(id)   ON DELETE RESTRICT`
- `position_id TEXT REFERENCES membership_positions(id) ON DELETE RESTRICT` — **nullable**: set
  when this membership fills a billet, NULL for plain belonging. If set, its position must belong
  to `unit_id` (checked in the application).
- `order_item_id TEXT REFERENCES order_order_items(id) ON DELETE SET NULL` — **nullable**
  provenance pointer: the [order](order.md) item (наказ) that this fill/belonging cites as its
  legal basis (D-Orders). NULL when the membership was created without an order. `SET NULL` so the
  membership survives if the order item is removed; the act stays recorded in [audit](audit.md).
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended'))`
- `effective_from TIMESTAMPTZ NOT NULL DEFAULT now()`, `effective_to TIMESTAMPTZ`
- `created_at`, `updated_at`, `deleted_at`
- **One billet, one holder:** `UNIQUE (position_id) WHERE position_id IS NOT NULL
  AND status='active' AND deleted_at IS NULL` — a position has at most one active filling
  (multi-incumbent is a seam).
- Plain-belonging uniqueness: `UNIQUE (person_id, unit_id) WHERE position_id IS NULL
  AND status='active' AND deleted_at IS NULL`.
- Indexes: `(person_id) WHERE status='active'`, `(unit_id) WHERE status='active'`,
  `(position_id) WHERE status='active'`.

**Vacancy** is a derived state, not a column: an `active` position with **no** active membership
referencing it. `GET /units/{id}/positions?state=vacant` is the closure of this predicate.

## Conjure API surface

`MembershipService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /units/{unitId}/positions` | Create a billet in a unit (vacant) | `position.create` (on the unit) |
| `GET /units/{unitId}/positions` | List a unit's positions (filter `state=vacant\|filled`) | `position.read` + shadow gate |
| `GET /positions/{id}` | Read one position (+ current holder if any) | `position.read` + shadow gate |
| `PUT /positions/{id}` | Update title/required-rank/order | `position.update` |
| `POST /positions/{id}/abolish` | Abolish a billet (reversible) | `position.update` |
| `POST /memberships` | Add belonging (optionally filling a position) | `membership.create` (on the unit) |
| `POST /positions/{id}/fill` | Fill a vacant position with a person | `membership.create` (on the unit) |
| `POST /memberships/{id}/end` | End a membership → vacates its position | `membership.update` |
| `GET /units/{unitId}/members` | Roster of a unit (token-paginated) | `membership.read` + shadow gate |
| `GET /persons/{personId}/memberships` | A person's memberships | `membership.read` + shadow gate |

`title` is returned as a `locale → text` map. Filling an already-filled position →
`Membership:PositionAlreadyFilled`. Roster/by-unit reads enforce the **shadow-visibility gate**.

## Dependencies

- **Calls:** [person](person.md) (person exists), [tenant](tenant.md) (unit exists + visibility
  for the gate), [rank](rank.md) (validate `required_rank_id`), [localization](localization.md)
  (assemble the `title` locale-map; purge position translations on delete). Emits
  `PositionCreated`, `PositionAbolished`, `MembershipCreated`, `MembershipEnded` events.
  **Subscribes** to [order](order.md)'s `AppointmentOrdered` / `RemovalOrdered` events and applies
  the fill/end in the issue transaction, citing `order_item_id` (D-OrderApply).
- **Called by:** read surfaces listing people-by-unit / vacancies; [order](order.md) (an order item
  may target a position; a fill/end may carry an `order_item_id` provenance link); [audit](audit.md).

## Authorization touchpoints

Defines/gates: `position.create`, `position.read`, `position.update` and `membership.create`,
`membership.read`, `membership.update` — all **unit-scoped** against the relevant unit (a unit's
billets and roster are managed by admins with authority over that unit/subtree). Reads pass the
shadow gate. Position carries **no** authority (D-Position / D-Rank).

## Invariants & safety

- A position **belongs to exactly one unit** and exists independently of any person (vacancies
  are first-class).
- A membership requires an existing `person` and `unit`; if it references a position, that
  position must belong to the same unit.
- **At most one active filling per position** (single billet); plain belonging is unique per
  `(person, unit)`.
- Visibility is **derived** from the unit, never duplicated — no drift.
- A person may hold many memberships across public and shadow units simultaneously.
- **Reversible:** `membership.end` and `position.abolish` flip status (+ `effective_to`) rather
  than delete; ending a filling **vacates** the billet. Re-fill is a new membership, audited.
- A position with an active filling cannot be hard-deleted (`ON DELETE RESTRICT`).
- **RLS backstop.** `membership_positions` / `membership_memberships` carry the defense-in-depth
  RLS policies keyed on `app.readable_units` / `app.writable_units` (D-RLSDefenseInDepth) — behind
  the authoritative PDP + shadow gate, not a replacement.

## Open seams / future

- **Multi-incumbent positions** (a billet with several holders) — relax the one-holder unique
  index; reserved.
- **Standard-title catalog** (reusable, instance-defined titles that positions draw from) is a
  future additive layer; today each position carries its own translatable `title`.
- **Establishment control** (whether creating billets is unit-scoped, as here, or centrally
  instance-controlled) is a config seam; the default is unit-scoped management. **Strong future
  candidate** for the target domains — military establishment (TO&E/MTOE) and university position
  lines are typically owned by a **central manpower authority**, not minted per-unit (DS-11);
  still an additive config switch, so the default stays unit-scoped until an org needs it.
- Effective-dated / temporal membership history is supported by the date columns; richer
  temporal queries are additive.
- **Order provenance.** A fill/end may cite an [order](order.md) item via `order_item_id` (the
  наказ that authorized it, D-Orders). On order **issue**, membership applies the fill/end
  automatically from the order's `AppointmentOrdered` / `RemovalOrdered` events **in the issue
  transaction** (D-OrderApply) — it still runs its own write-path invariants, so a fill that hits the
  one-holder index surfaces `Membership:PositionAlreadyFilled` and **rolls back the whole issue**. A
  fill/end may also still be performed directly (without an order), leaving `order_item_id` NULL.
