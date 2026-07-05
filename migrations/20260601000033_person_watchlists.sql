-- 0033 person watchlists & regulatory exposure (M34 — D-Watchlists). Watchlist screening is NEVER stored
-- statically. Two surfaces:
--
--   person_watchlist_matches      — the persisted RESULT of a live screening check that runs OUT to the
--                                    hermenea companion (which owns egress to OFAC/EU/UN/INTERPOL + a ≤24h
--                                    cache). Only per-person MATCH METADATA lands here — never the lists.
--                                    One active row per person (a re-check refreshes it). pii:sensitive.
--                                    PEP is a snapshot of the M33 government-position derivation at check
--                                    time. Hard-deleted on purge (a transient screening result, not a
--                                    legal record).
--   person_regulatory_sanctions   — a structured regulatory-action overlay (regulator/action/amount/status).
--                                    Audited manual CRUD AND a hermenea import target (idempotent by
--                                    (person, external_id)). pii:sensitive; erased on purge.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons), 0028 (platform_legal_basis_kinds),
-- and 0001 (geo_countries). The M33 pep_trigger seam (person_government_positions) feeds the PEP snapshot.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,15,'watchlist_match'),        -- person / object / watchlist_match     (M34)
  (6,1,16,'regulatory_sanction');    -- person / object / regulatory_sanction (M34)

-- ===================================================================================================
-- person_watchlist_matches — the pii:sensitive Object watchlist_match. The persisted residue of a live
-- watchlist check (D-Watchlists): match METADATA only, never the underlying lists. One active row per
-- person; CheckWatchlists upserts it (partial-unique on person_id) so re-screening refreshes last_checked/
-- next_check_due in place rather than accumulating history. `pep` is a snapshot of the M33 pep_trigger
-- derivation captured at check time (the live external providers do not return PEP; it is derived locally).
-- Hard-deleted on person purge — a transient screening flag, not a record to crypto-tombstone.
-- ===================================================================================================
CREATE TABLE oikumenea.person_watchlist_matches (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,15),  -- person / object / watchlist_match
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  on_list        boolean NOT NULL DEFAULT false,                     -- any hit across the queried providers
  lists          text[] NOT NULL DEFAULT '{}',                       -- e.g. {OFAC_SDN, EU_CFSP, INTERPOL_RED}
  program        text,                                               -- e.g. sanctions program / notice class
  match_score    numeric,                                            -- 0..1 best-match score across providers
  pep            boolean NOT NULL DEFAULT false,                     -- PEP snapshot (M33 government positions)
  last_checked   timestamptz NOT NULL DEFAULT now(),
  next_check_due timestamptz,                                        -- when the ≤24h cache lapses upstream
  source         text NOT NULL DEFAULT 'imported'
                   CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence     text NOT NULL DEFAULT 'possible'
                   CHECK (confidence IN ('confirmed','probable','possible')),
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT person_watchlist_matches_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=15)
);

CREATE TRIGGER person_watchlist_matches_set_updated_at
  BEFORE UPDATE ON oikumenea.person_watchlist_matches
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One active screening result per person — CheckWatchlists refreshes rather than accumulates.
CREATE UNIQUE INDEX person_watchlist_matches_person
  ON oikumenea.person_watchlist_matches (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_watchlist_matches.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.on_list IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.lists IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.program IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.match_score IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.pep IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.last_checked IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.next_check_due IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_watchlist_matches.confidence IS 'pii:none';

-- ===================================================================================================
-- person_regulatory_sanctions — the pii:sensitive Object regulatory_sanction. A structured record of a
-- regulatory/enforcement action against a person (a licensed professional, a director, etc.), distinct from
-- the volatile live-lookup above: this is durable, operator-curated or hermenea-imported reference data.
-- Idempotent import keys on (person_id, external_id). legal_basis is optional (Art. 6/9) since a public
-- enforcement action is public-record but may carry special-category context.
-- ===================================================================================================
CREATE TABLE oikumenea.person_regulatory_sanctions (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,16),  -- person / object / regulatory_sanction
  person_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  regulator     text NOT NULL,                                      -- e.g. "SEC", "FCA", "NBU"
  action_type   text NOT NULL DEFAULT 'other'
                  CHECK (action_type IN ('fine','ban','license_revocation','warning','settlement','debarment','other')),
  amount        numeric,                                            -- monetary penalty, if any
  currency      text,                                               -- ISO-4217, when amount is present
  status        text NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active','appealed','overturned','expired','settled')),
  sanction_date date,
  source_url    text,
  external_id   text,                                               -- the id within the source system (import key)
  legal_basis   text REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  source        text NOT NULL DEFAULT 'operator_verified'
                  CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence    text NOT NULL DEFAULT 'possible'
                  CHECK (confidence IN ('confirmed','probable','possible')),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  CONSTRAINT person_regulatory_sanctions_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=16),
  CONSTRAINT person_regulatory_sanctions_amount_currency
    CHECK (amount IS NULL OR currency IS NOT NULL)
);

CREATE TRIGGER person_regulatory_sanctions_set_updated_at
  BEFORE UPDATE ON oikumenea.person_regulatory_sanctions
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_regulatory_sanctions_person_idx
  ON oikumenea.person_regulatory_sanctions (person_id) WHERE deleted_at IS NULL;
-- Idempotent hermenea import: one active row per (person, source id).
CREATE UNIQUE INDEX person_regulatory_sanctions_person_extid
  ON oikumenea.person_regulatory_sanctions (person_id, external_id)
  WHERE external_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.regulator IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.action_type IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.amount IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.currency IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.status IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.sanction_date IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.source_url IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.external_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_regulatory_sanctions.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0033_person_watchlists', applied_at = now() WHERE singleton;
