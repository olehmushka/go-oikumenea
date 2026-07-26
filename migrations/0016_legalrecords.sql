-- 0016_legalrecords — criminal / arrest / court records (M16 draft macro-category 6.1–6.3;
-- D-LegalRecords). The last in-scope milestone of the person-intelligence cluster (M29–M37): a
-- category-level record of a person's criminal, arrest and court-judgment history, built last so it
-- reuses the proven M31/M33/M35/M36 special-PII envelope + the M29 legal_basis/audit substrate.
--
-- Criminal-conviction/offence data is GDPR Art. 10 (arguably the strictest class), so — amending the
-- draft's pii:sensitive tag — the record rides the SAME pii:special envelope as M36 health: the
-- category-level `detail` (a coarse offence/charge category — NEVER a full charge sheet) is
-- envelope-encrypted (ciphertext/wrapped_dek/key_ref/blind_index sealed in the application; NO
-- plaintext value column) with a NOT-NULL legal_basis (Art. 10) and a need-to-know read gate
-- (person.legal-record.read). Crypto-erased on purge (drop the envelope, keep a tombstone).
--
-- Two requirements carried from the draft distinguish this from M36:
--   * `disposition` is MANDATORY (arrest ≠ guilt) — a coarse outcome discriminator, plaintext but
--     pii:special like `kind`, validated app-side against a closed set.
--   * expungement / sealing SUPPRESSION — a sealed/expunged record is RETAINED (legal + audit basis)
--     but hidden from the normal person.legal-record.read gate; only a caller who additionally holds
--     person.legal-record.read-suppressed (or an instance admin) sees suppressed rows. Enforced in the
--     application/transport read path (the sensitive-reader redaction pattern), not in SQL policy.
--
-- Distinct from the internal discipline-incentive orders (order module, DS-36): those are the org's
-- own reprimand/gratitude record-only order items; these are external judicial facts.
--
-- Jurisdiction is a hard FK to geo_countries (D-Geo — never a bare country string); the
-- jurisdiction-specific display/storage rules (Ban-the-Box, FCRA lookback) are an OPEN SEAM — the
-- data hook (jurisdiction + suppression) lands here, the rule engine does not.
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends
-- on 0001 (new_id / platform_rid_types / geo_countries), 0003 (person_persons) and 0009
-- (platform_legal_basis_kinds). Applies after the locale files (0012–0015): no new i18n rows (the
-- kind/disposition/suppressed_reason discriminators are enums rendered client-side).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors this and asserts equality at boot
-- (rid.AssertMatches). The three registries — this seed, pkg/rid/registry.go and ontology-mapping.md —
-- MUST agree or boot fails (the M37 login_event gotcha).
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,22,'legal_record')  -- person / object / legal_record (M38, encrypted)
ON CONFLICT DO NOTHING;

-- ===================================================================================================
-- person_legal_records — a category-level criminal/arrest/court record Object (GDPR Art. 10). The
-- category-level offence `detail` is envelope-encrypted (detail_ciphertext/detail_wrapped_dek/
-- detail_key_ref/detail_blind_index sealed in the application; NO plaintext detail column). The coarse
-- `kind` and mandatory `disposition` discriminators stay plaintext so they drive queries, but knowing
-- someone has (say) a criminal conviction is itself special, so both are marked pii:special. A person
-- may hold many records (no one-active-per-kind uniqueness). Crypto-erased on purge (drop the
-- envelope, keep the row tombstone).
-- ===================================================================================================
CREATE TABLE oikumenea.person_legal_records (
  id                  uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,22),  -- person / object / legal_record
  person_id           uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  kind                text NOT NULL
                        CHECK (kind IN ('criminal_conviction','arrest','court_judgment')),
  -- MANDATORY outcome (arrest ≠ guilt). Closed set, also validated app-side.
  disposition         text NOT NULL
                        CHECK (disposition IN
                          ('convicted','acquitted','dismissed','pending','sealed','expunged','no_charges')),
  -- envelope-encrypted category-level offence/charge (a coarse category — NO full charge sheet).
  -- Always populated at creation, but NULLABLE so crypto-erase on purge can drop all four (row survives).
  detail_ciphertext   bytea,
  detail_wrapped_dek  bytea,
  detail_key_ref      text,
  detail_blind_index  bytea,
  -- jurisdiction (D-Geo hard FK; nullable). The subnational subdivision + display-rule engine are a seam.
  jurisdiction_country_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,
  occurred_at         date,                                                -- offence / arrest date
  disposition_date    date,                                               -- when the disposition was reached
  -- expungement / sealing suppression: a suppressed row is retained but hidden from the normal read gate.
  is_suppressed       boolean NOT NULL DEFAULT false,
  suppressed_reason   text CHECK (suppressed_reason IN ('sealed','expunged')),
  legal_basis         text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  source              text NOT NULL DEFAULT 'imported'
                        CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence          text NOT NULL DEFAULT 'possible'
                        CHECK (confidence IN ('confirmed','probable','possible')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  -- suppressed_reason is present iff the row is suppressed.
  CONSTRAINT person_legal_records_suppression_ck
    CHECK ((is_suppressed = false AND suppressed_reason IS NULL)
        OR (is_suppressed = true  AND suppressed_reason IS NOT NULL)),
  CONSTRAINT person_legal_records_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=22)
);

CREATE TRIGGER person_legal_records_set_updated_at
  BEFORE UPDATE ON oikumenea.person_legal_records
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_legal_records_person_idx
  ON oikumenea.person_legal_records (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_legal_records.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_legal_records.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_legal_records.kind IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_legal_records.disposition IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_legal_records.detail_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_legal_records.detail_wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_legal_records.detail_key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_legal_records.detail_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_legal_records.jurisdiction_country_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_legal_records.occurred_at IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_legal_records.disposition_date IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_legal_records.is_suppressed IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_legal_records.suppressed_reason IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_legal_records.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_legal_records.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_legal_records.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0016_legalrecords', applied_at = now() WHERE singleton;
