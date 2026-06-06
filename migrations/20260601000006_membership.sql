-- 0007 membership (M6).
--
-- People belonging to / filling billets in units (docs/modules/membership.md). Two related things:
--   * POSITIONS (membership_positions) — a unit-owned billet (an Object that EXISTS while vacant;
--     D-Position). It belongs to exactly one unit, carries a stable `code` + a translatable `title`,
--     an optional establishment `required_rank_id`, and a reversible active/abolished status.
--   * MEMBERSHIPS (membership_memberships) — the reified Link link__member_of: a person's belonging
--     to a unit, OPTIONALLY filling a position, effective-dated. Filling a billet is a membership
--     that references it; a position-less membership is plain belonging.
--
-- Neither entity stores visibility — it DERIVES from the owning unit's visibility (tenant.md); the
-- shadow-visibility read gate lands with the PDP (M7). Position grants NO authority (D-Position /
-- D-Rank) — it is directory data, never a PDP input.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid,
-- set_updated_at), 0004 tenant (tenant_units), 0005 rank (rank_ranks) and 0006 person
-- (person_persons). This migration is PURE DDL: it seeds NO rows (so no app.environment GUC is
-- needed at migration time — D-RIDSeeding). Positions/memberships are created through
-- MembershipService.

-- membership_positions: a unit-owned billet (D-Position). Exists vacant; `code` is the stable,
-- locale-agnostic identifier (unique within the unit among active rows); `title` is the
-- default-locale fallback (translations in the i18n store, M2). required_rank_id is the
-- establishment expectation — ADVISORY, never enforced against any filler's rank.
CREATE TABLE oikumenea.membership_positions (
  id               text PRIMARY KEY DEFAULT oikumenea.new_rid('membership','position'),
  unit_id          text NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,  -- the owning unit
  code             text NOT NULL,                 -- stable, locale-agnostic; unique within the unit among active
  title            text NOT NULL,                 -- default-locale title; translatable via the i18n store
  required_rank_id text REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,  -- optional, advisory establishment rank
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','abolished')),
  sort_order       integer,                       -- app-managed display order within the unit
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT membership_positions_rid_shape CHECK (id LIKE 'urn:oikumenea:membership:%:position:%')
);

CREATE TRIGGER membership_positions_set_updated_at
  BEFORE UPDATE ON oikumenea.membership_positions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- `code` is unique within a unit among active (non-deleted) positions; immutable by convention (D-Code).
CREATE UNIQUE INDEX membership_positions_unit_code_active_idx
  ON oikumenea.membership_positions (unit_id, code) WHERE deleted_at IS NULL;
CREATE INDEX membership_positions_unit_active_idx
  ON oikumenea.membership_positions (unit_id) WHERE status = 'active' AND deleted_at IS NULL;

-- Billet/establishment labels are organizational, not personal data (D-PIITiers).
COMMENT ON COLUMN oikumenea.membership_positions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.title IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.required_rank_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_positions.sort_order IS 'pii:none';

-- membership_memberships: the reified person->unit belonging/filling Link (link__member_of). A
-- membership requires an existing person and unit; if it references a position, that position must
-- belong to the same unit (checked in the application). Reversible: end flips status + sets
-- effective_to rather than deleting; ending a filling VACATES the billet.
CREATE TABLE oikumenea.membership_memberships (
  id             text PRIMARY KEY DEFAULT oikumenea.new_rid('membership','link__member_of'),
  person_id      text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  unit_id        text NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  position_id    text REFERENCES oikumenea.membership_positions(id) ON DELETE RESTRICT,  -- NULL = plain belonging
  -- order_item_id: provenance pointer to the order (наказ) item this fill/belonging cites as its
  -- legal basis (D-Orders). The order module is M10, so the FK (-> order_order_items, ON DELETE SET
  -- NULL) is added then; today it is a free-standing nullable RID column (open seam).
  order_item_id  text,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT membership_memberships_rid_shape CHECK (id LIKE 'urn:oikumenea:membership:%:link\_\_member_of:%')
);

CREATE TRIGGER membership_memberships_set_updated_at
  BEFORE UPDATE ON oikumenea.membership_memberships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One billet, one holder: a position has at most one ACTIVE filling (multi-incumbent is a seam).
CREATE UNIQUE INDEX membership_memberships_one_holder_idx
  ON oikumenea.membership_memberships (position_id)
  WHERE position_id IS NOT NULL AND status = 'active' AND deleted_at IS NULL;
-- Plain belonging is unique per (person, unit) among active position-less memberships.
CREATE UNIQUE INDEX membership_memberships_belonging_idx
  ON oikumenea.membership_memberships (person_id, unit_id)
  WHERE position_id IS NULL AND status = 'active' AND deleted_at IS NULL;

CREATE INDEX membership_memberships_person_idx
  ON oikumenea.membership_memberships (person_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX membership_memberships_unit_idx
  ON oikumenea.membership_memberships (unit_id) WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX membership_memberships_position_idx
  ON oikumenea.membership_memberships (position_id) WHERE status = 'active' AND deleted_at IS NULL;

-- A membership links a person to a unit (the association is organizational); it is not itself a PII
-- store. person_id is a stable id (pii:none), but the FACT of belonging to a specific unit is
-- mildly identifying, so the link end and its dates are tiered pii:basic.
COMMENT ON COLUMN oikumenea.membership_memberships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.unit_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.position_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.order_item_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.membership_memberships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.membership_memberships.effective_to IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0007_membership', applied_at = now() WHERE singleton;
