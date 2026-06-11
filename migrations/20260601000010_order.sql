-- 0010 order (M10).
--
-- Administrative orders (наказ): the formal acts that are the LEGAL BASIS for a change in a person's
-- status (docs/modules/order.md, D-Orders). Where a document records what a person HAS, an order
-- records an ACT the organization performs (arrival, appointment, leave, transfer, discipline, duty).
-- Three tables:
--   * order_order_types  — the instance-admin catalog of order kinds (RID Object; stable `code` +
--                          translatable `name`), each carrying a `category` (the five UA-army families)
--                          and an `effect` (membership-start | membership-end | rank-change |
--                          record-only) that drives which target columns an item must carry and which
--                          intent event issue emits.
--   * order_orders       — the order header (наказ): number, date, issuing unit, draft→issued→revoked
--                          lifecycle (mutable while draft; LOCKED on issue — corrections are amending
--                          orders, undo is a revoking order; reversibility, not the append-only guard).
--   * order_order_items  — one affected person/act within an order; PARENT-SCOPED (no deleted_at, no
--                          independent lifecycle — reads resolve items only through a non-deleted parent).
--
-- An order takes effect on other modules ONLY via domain events + provenance (the locked
-- cross-module-mutation rule, D-OrderApply): on issue, each structural item emits an intent event a
-- membership/person subscriber applies IN THE ISSUE TRANSACTION, citing membership_memberships.order_item_id.
-- That forward-referenced provenance column (added nullable, no FK, in 0006 membership) gets its FK here.
--
-- An order carries NO authority (directory/record data, like rank/position); access is decided by the
-- PDP, unit-scoped on issuing_unit_id + the shadow gate. Expand-only (L-UpgradeSafe); depends on 0001
-- bootstrap (new_rid, set_updated_at), 0003 localization (translatable type names), 0003 tenant (issuing
-- unit), 0005 rank (target rank), 0006 person + membership (target person/position; the provenance FK).
--
-- RID-keyed tables seed NO rows here — that needs the app.environment GUC atlas does not set
-- (D-RIDSeeding); the order-type catalog is seeded at boot in order.Register.

-- order_order_types: the instance-admin catalog of order kinds (D-Orders). RID Object with a stable,
-- locale-agnostic `code` (immutable by convention) + a translatable default-locale `name`. `category`
-- is the five UA-army "стройова частина" families; `effect` is the downstream consequence of items of
-- this type (it determines the required target columns — enforced in the application — and the intent
-- event issue emits).
CREATE TABLE oikumenea.order_order_types (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('order','order_type'),
  code       text NOT NULL UNIQUE,               -- stable locale-agnostic identifier (D-Code)
  name       text NOT NULL,                      -- default-locale label; translatable via the i18n store
  category   text NOT NULL CHECK (category IN
               ('personnel-list','appointment','leave-travel','discipline-incentive','duty-roster')),
  effect     text NOT NULL CHECK (effect IN
               ('membership-start','membership-end','rank-change','record-only')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT order_order_types_rid_shape CHECK (id LIKE 'urn:oikumenea:order:%:order_type:%')
);

CREATE TRIGGER order_order_types_set_updated_at
  BEFORE UPDATE ON oikumenea.order_order_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.order_order_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.category IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.effect IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.sort_order IS 'pii:none';

-- order_orders: the order header (наказ). issuing_unit_id is NOT NULL — every order is unit-issued (no
-- instance-level orders), which anchors the unit-scope authz check and the RLS predicate (both key on
-- issuing_unit_id; D-Orders, I-5). status is the draft→issued→revoked lifecycle (the only post-issue
-- transition is issued→revoked, recording revoked_by_order_id + revoked_at).
CREATE TABLE oikumenea.order_orders (
  id                  text PRIMARY KEY DEFAULT oikumenea.new_rid('order','order'),
  number              text,                       -- order number (unique within issuing unit; nullable)
  issued_on           date,                       -- the order's date
  issuing_unit_id     text NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  status              text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','issued','revoked')),
  revoked_by_order_id text REFERENCES oikumenea.order_orders(id),  -- the later order that revoked this one
  revoked_at          timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,

  CONSTRAINT order_orders_rid_shape CHECK (id LIKE 'urn:oikumenea:order:%:order:%')
);

CREATE TRIGGER order_orders_set_updated_at
  BEFORE UPDATE ON oikumenea.order_orders
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- An order number is unique within its issuing unit (among non-deleted rows; D-Orders invariant).
CREATE UNIQUE INDEX order_orders_unit_number_idx
  ON oikumenea.order_orders (issuing_unit_id, number)
  WHERE number IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX order_orders_issued_idx
  ON oikumenea.order_orders (issuing_unit_id) WHERE status = 'issued' AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.order_orders.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.number IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.issued_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.issuing_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.status IS 'pii:none';

-- order_order_items: one affected person/act within an order (the unit of effect). PARENT-SCOPED — no
-- deleted_at and no lifecycle of its own (added/removed only while the parent is draft; locked on
-- issue; visible only through a non-deleted parent). ON DELETE CASCADE off the parent is FK-integrity
-- insurance for the (design-forbidden) hard delete of a parent, not a routine path. Which target
-- columns are required is checked in the application against the type's `effect`. `note` is the only
-- pii:basic field; person/unit/position/rank are pii:none id references.
CREATE TABLE oikumenea.order_order_items (
  id             text PRIMARY KEY DEFAULT oikumenea.new_rid('order','order_item'),
  order_id       text NOT NULL REFERENCES oikumenea.order_orders(id) ON DELETE CASCADE,
  type_id        text NOT NULL REFERENCES oikumenea.order_order_types(id) ON DELETE RESTRICT,
  person_id      text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  unit_id        text REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,        -- target unit (nullable)
  position_id    text REFERENCES oikumenea.membership_positions(id) ON DELETE RESTRICT, -- target billet (nullable)
  rank_id        text REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,           -- target rank (nullable)
  effective_from date,
  effective_to   date,
  note           text,                            -- free-text detail (reason, reference); minimized
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT order_order_items_rid_shape CHECK (id LIKE 'urn:oikumenea:order:%:order_item:%')
);

CREATE TRIGGER order_order_items_set_updated_at
  BEFORE UPDATE ON oikumenea.order_order_items
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX order_order_items_order_idx  ON oikumenea.order_order_items (order_id);
CREATE INDEX order_order_items_person_idx ON oikumenea.order_order_items (person_id);

COMMENT ON COLUMN oikumenea.order_order_items.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.order_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.position_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.rank_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.effective_from IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.effective_to IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.note IS 'pii:basic';

-- Resolve the forward-referenced provenance pointer from 0006 membership: a fill/end may cite the
-- order item it realizes (D-OrderApply). Added nullable + no FK there (order_order_items did not yet
-- exist); the FK lands now as ON DELETE SET NULL so hard-deleting an order's items (FK insurance only)
-- nulls the provenance rather than blocking — the membership row is the authoritative fact.
ALTER TABLE oikumenea.membership_memberships
  ADD CONSTRAINT membership_memberships_order_item_id_fkey
  FOREIGN KEY (order_item_id) REFERENCES oikumenea.order_order_items(id) ON DELETE SET NULL;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0010_order', applied_at = now() WHERE singleton;
