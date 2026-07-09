-- 0035 person financial / behavioural / psychological overlays (M35 — D-PersonOverlays). The draft's
-- macro-categories 4 + 5, modelled as three per-type person overlays, each carrying the
-- D-OverlayFoundation attribution column-set (source/confidence). Two are plaintext pii:sensitive; the
-- inferred political-leaning overlay is pii:special and envelope-encrypted (same shape as the M31
-- person_ethnicities / M33 person_party_memberships), and is a SEPARATE table from the declared M33 party
-- membership — the declared and the inferred are NEVER merged (D-PersonOverlays).
--
--   person_crypto_wallets     — a crypto-wallet attribution (address/chain/method/balance). pii:sensitive
--                               (the address itself is public on-chain data, but ATTRIBUTING it to a person
--                               is sensitive). Hard-erased on purge. Synergy with the M34 sanctions screen.
--   person_personality        — a declared / formally-assessed personality profile (MBTI / Big-Five / DISC /
--                               Enneagram). pii:sensitive. DECLARED SURVEY OR FORMAL HR ASSESSMENT ONLY —
--                               the `method` CHECK forbids text-inference. Hard-erased on purge.
--   person_political_leaning  — an INFERRED political leaning (spectrum ∈ [-1,1], inference sources). GDPR
--                               Art. 9 (political opinion) → pii:special: the spectrum value is
--                               envelope-encrypted (ciphertext/wrapped_dek/key_ref/blind_index sealed in the
--                               application; NO plaintext value) with a NOT-NULL legal_basis. One active row
--                               per person, refreshed in place. Crypto-erased on purge. NEVER merged with the
--                               declared M33 person_party_memberships.
--
-- Compensation / payroll is out of scope here (a separate operational-HR module, M39 — deferred).
--
-- Person-scoped (instance-global) — NO unit RLS. Expand-only (L-UpgradeSafe / D-Migrations). Depends on
-- 0000 (new_id / platform_rid_types), 0005 person (person_persons) and 0028 (platform_legal_basis_kinds).

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (6,1,17,'crypto_wallet'),      -- person / object / crypto_wallet    (M35)
  (6,1,18,'personality'),        -- person / object / personality      (M35)
  (6,1,19,'political_leaning');  -- person / object / political_leaning (M35, encrypted)

-- ===================================================================================================
-- person_crypto_wallets — a crypto-wallet attribution Object. The wallet address is public blockchain
-- data (stored in plaintext, queryable for the M34 sanctioned-wallet cross-check), but the LINK to a
-- person is pii:sensitive. attribution_method records HOW the wallet was tied to the person.
-- ===================================================================================================
CREATE TABLE oikumenea.person_crypto_wallets (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,17),  -- person / object / crypto_wallet
  person_id          uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  address            text NOT NULL,                                       -- the on-chain wallet address
  chain              text NOT NULL DEFAULT 'ethereum'
                       CHECK (chain IN ('bitcoin','ethereum','solana','tron','bnb','polygon','monero','other')),
  attribution_method text NOT NULL DEFAULT 'other'
                       CHECK (attribution_method IN ('exchange_kyc','blockchain_analysis','self_declared','leak','public_post','other')),
  balance_usd_approx double precision,                                    -- last-known approximate USD balance
  first_seen         date,
  last_seen          date,
  source             text NOT NULL DEFAULT 'imported'
                       CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence         text NOT NULL DEFAULT 'possible'
                       CHECK (confidence IN ('confirmed','probable','possible')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  CONSTRAINT person_crypto_wallets_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=17)
);

CREATE TRIGGER person_crypto_wallets_set_updated_at
  BEFORE UPDATE ON oikumenea.person_crypto_wallets
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_crypto_wallets_person_idx
  ON oikumenea.person_crypto_wallets (person_id) WHERE deleted_at IS NULL;
-- Dedup an active (person, chain, address) — one attribution row per wallet.
CREATE UNIQUE INDEX person_crypto_wallets_person_addr
  ON oikumenea.person_crypto_wallets (person_id, chain, address) WHERE deleted_at IS NULL;
-- Cross-check ("which persons hold this address") without scanning by person.
CREATE INDEX person_crypto_wallets_addr_idx
  ON oikumenea.person_crypto_wallets (chain, address) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_crypto_wallets.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.address IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.chain IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.attribution_method IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.balance_usd_approx IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.first_seen IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.last_seen IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_crypto_wallets.confidence IS 'pii:none';

-- ===================================================================================================
-- person_personality — a declared or formally-assessed personality profile Object. The `method` CHECK
-- enforces the D-PersonOverlays rule: DECLARED SURVEY OR FORMAL HR ASSESSMENT ONLY — there is no
-- NLP-from-text inference path. framework is the instrument family; result is its typed output (e.g. an
-- MBTI code "INTJ", a Big-Five summary, an Enneagram type). pii:sensitive; hard-erased on purge.
-- ===================================================================================================
CREATE TABLE oikumenea.person_personality (
  id           uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,18),  -- person / object / personality
  person_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  framework    text NOT NULL DEFAULT 'mbti'
                 CHECK (framework IN ('mbti','big_five','disc','enneagram','other')),
  result       text NOT NULL,                                      -- e.g. "INTJ", "O80 C60 E40 A55 N30", "Type 5"
  instrument   text,                                               -- the specific test/assessment used
  method       text NOT NULL DEFAULT 'self_declared_survey'
                 CHECK (method IN ('self_declared_survey','hr_assessment')),
  assessed_at  date,
  source       text NOT NULL DEFAULT 'self_declared'
                 CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence   text NOT NULL DEFAULT 'possible'
                 CHECK (confidence IN ('confirmed','probable','possible')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz,
  CONSTRAINT person_personality_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=18)
);

CREATE TRIGGER person_personality_set_updated_at
  BEFORE UPDATE ON oikumenea.person_personality
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_personality_person_idx
  ON oikumenea.person_personality (person_id) WHERE deleted_at IS NULL;
-- One active profile per framework per person.
CREATE UNIQUE INDEX person_personality_person_framework
  ON oikumenea.person_personality (person_id, framework) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_personality.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_personality.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_personality.framework IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_personality.result IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.person_personality.instrument IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_personality.method IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_personality.assessed_at IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_personality.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_personality.confidence IS 'pii:none';

-- ===================================================================================================
-- person_political_leaning — an INFERRED political leaning Object. Political opinion is a GDPR Art. 9
-- special category, so the inferred spectrum value is envelope-encrypted (ciphertext/wrapped_dek/key_ref/
-- blind_index sealed in the application; NO plaintext value column) with a NOT-NULL legal_basis. The
-- inference methodology (inference_sources) and confidence stay in plaintext (the sensitive datum is the
-- LEANING itself, not the fact that it was inferred). ONE active row per person, refreshed in place;
-- crypto-erased on purge (drop the envelope, keep the row tombstone). This is a SEPARATE table from the
-- declared M33 person_party_memberships and is NEVER merged with it (D-PersonOverlays).
-- ===================================================================================================
CREATE TABLE oikumenea.person_political_leaning (
  id                  uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,19),  -- person / object / political_leaning
  person_id           uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  -- envelope-encrypted inferred spectrum (a signed decimal string in [-1,1]). Always populated at creation,
  -- but NULLABLE so crypto-erase on purge can drop all four (the row survives as a tombstone).
  leaning_ciphertext  bytea,
  leaning_wrapped_dek bytea,
  leaning_key_ref     text,
  leaning_blind_index bytea,
  inference_sources   text[] NOT NULL DEFAULT '{}',                       -- methodology, e.g. {social_media,voting_record}
  assessed_at         date,
  legal_basis         text NOT NULL REFERENCES oikumenea.platform_legal_basis_kinds(code) ON UPDATE RESTRICT,
  confidence          text NOT NULL DEFAULT 'possible'
                        CHECK (confidence IN ('confirmed','probable','possible')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  CONSTRAINT person_political_leaning_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=19)
);

CREATE TRIGGER person_political_leaning_set_updated_at
  BEFORE UPDATE ON oikumenea.person_political_leaning
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One active inferred leaning per person (refreshed in place).
CREATE UNIQUE INDEX person_political_leaning_person
  ON oikumenea.person_political_leaning (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_political_leaning.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_political_leaning.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_political_leaning.leaning_ciphertext IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_political_leaning.leaning_wrapped_dek IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_political_leaning.leaning_key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_political_leaning.leaning_blind_index IS 'pii:special';
COMMENT ON COLUMN oikumenea.person_political_leaning.inference_sources IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_political_leaning.assessed_at IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_political_leaning.legal_basis IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_political_leaning.confidence IS 'pii:none';

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0035_person_overlays', applied_at = now() WHERE singleton;
