-- 0019_facet_vehicle_finance — the vehicle, account and card dashboards (M58 ticket 3 /
-- D-ObjectFacets, D-ConsoleDashboards).
--
-- Additive, exactly like 0018: no new table and no new column. All three types were already fully
-- modelled; this milestone adds a second READING of columns that already exist.
--
-- 1. pii classification comments on seven columns across three tables. D-ObjectFacets rule 2 is
--    enforced by a build-time guard (pkg/facet/plaintext_test.go) that parses this directory and
--    refuses a facet whose column carries no `COMMENT ON COLUMN ... IS 'pii:<tier>'` — an omitted
--    comment must FAIL rather than default, since a facet over an unclassified column is precisely
--    what the rule exists to catch.
--
--    The same shape 0018 found in external_organizations, and worth naming because it has now
--    recurred three times: a module classifies its IDENTIFYING and its SENSITIVE columns carefully
--    (vehicle commented id/type_id/model_id/vin/color; finance commented every crypto column and both
--    PAN display fragments) and leaves the lifecycle/classification columns — status, dates, catalog
--    FKs — unclassified. Nothing reads them, so nothing notices, until a facet groups by one. The
--    omission is invisible by construction until the facet vocabulary reaches the table.
--
--    All seven are pii:none. A vehicle is not a data subject; a card's network and type describe the
--    INSTRUMENT, not its holder. Note what is deliberately NOT reclassified: finance_cards.pan_* stay
--    pii:sensitive and are unfaceted, and bin/last_four stay pii:none but are still not facets — they
--    identify one card rather than describing a population (D-DataScope, PCI-DSS CDE scope).
--
-- 2. Indexes for the three new aggregate shapes.

-- ---------------------------------------------------------------------------------------------------
-- 1. Classify the facet columns.

-- vehicle_vehicles: M31 commented id/type_id/model_id/vin and M42 added color_id; the lifecycle
-- columns were left unclassified.
COMMENT ON COLUMN oikumenea.vehicle_vehicles.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.vehicle_vehicles.manufacture_date IS 'pii:none';

-- finance_accounts (M44): the crypto column-set was classified, the catalog FK and status were not.
COMMENT ON COLUMN oikumenea.finance_accounts.account_type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_accounts.status IS 'pii:none';

-- finance_cards (M44): likewise. card_type and status describe the instrument's kind and lifecycle.
COMMENT ON COLUMN oikumenea.finance_cards.network_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.card_type IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.status IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- 2. Aggregate support.
--
-- Each dashboard runs ONE aggregate over a candidate CTE, so what it wants is a cheap scan of the
-- live rows plus the grouped columns. The existing indexes on all three tables are the identity and
-- FK lookups the per-row reads need (vin, type_id, model_id, color_id, institution_id, account_id);
-- none covers a grouped column.

-- vehicle_vehicles: status tiles and the fleet-age histogram both group columns with no index.
CREATE INDEX IF NOT EXISTS vehicle_vehicles_status_idx
  ON oikumenea.vehicle_vehicles (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS vehicle_vehicles_manufacture_date_idx
  ON oikumenea.vehicle_vehicles (manufacture_date)
  WHERE deleted_at IS NULL AND manufacture_date IS NOT NULL;

-- vehicle_registrations: the registrationCountry facet probes the ACTIVE registration by vehicle and
-- groups by country. The shipped indexes are (vehicle_id) and (owner_kind, owner_id); neither serves
-- the country grouping, and the partial ACTIVE predicate is what keeps the semi-join to one row per
-- vehicle rather than the whole ownership history.
CREATE INDEX IF NOT EXISTS vehicle_registrations_active_country_idx
  ON oikumenea.vehicle_registrations (vehicle_id, country_id)
  WHERE status = 'active' AND deleted_at IS NULL;

-- finance_accounts: currency and status are both grouped and both unindexed. account_type_id is the
-- third grouped column.
CREATE INDEX IF NOT EXISTS finance_accounts_status_idx
  ON oikumenea.finance_accounts (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS finance_accounts_currency_idx
  ON oikumenea.finance_accounts (currency) WHERE deleted_at IS NULL AND currency IS NOT NULL;
CREATE INDEX IF NOT EXISTS finance_accounts_account_type_idx
  ON oikumenea.finance_accounts (account_type_id) WHERE deleted_at IS NULL;

-- finance_cards: the registry list is a NEW top-level keyset over this table (M58 ticket 3 — cards
-- were previously reachable only per-account, which the account_id index served). All three facet
-- columns are grouped by the dashboard.
CREATE INDEX IF NOT EXISTS finance_cards_network_idx
  ON oikumenea.finance_cards (network_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS finance_cards_status_idx
  ON oikumenea.finance_cards (status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS finance_cards_card_type_idx
  ON oikumenea.finance_cards (card_type) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0019_facet_vehicle_finance', applied_at = now() WHERE singleton;
