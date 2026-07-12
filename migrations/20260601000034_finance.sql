-- 0034 finance — bank accounts & payment cards (M44 — D-Finance). Authoritative, operator-asserted
-- FIRST-PARTY directory data (NOT an OSINT overlay: no source/confidence attribution). A person or
-- company holds bank accounts; each account is held at a BANK — which is NOT a new entity but an
-- existing `company`-domain tenant_organizations row (M21/M41). Cards hang off an account. The sensitive
-- identifiers (IBAN, card PAN) are ENVELOPE-ENCRYPTED at rest, exactly like document_personal_codes
-- (D-CryptoProvider): ciphertext + wrapped DEK + key_ref + a keyed-HMAC blind index for uniqueness /
-- equality lookup without decryption. The plaintext is never stored; person purge crypto-erases it.
--
-- Tables:
--   * CATALOGS — finance_account_types (current/savings/…) + finance_card_networks (visa/…): RID Objects
--                with a stable `code` + translatable `name`; SEEDED here (new_id reads no GUC — F-014).
--   * OBJECT   — finance_accounts: institution_id -> tenant_organizations (the bank), encrypted IBAN,
--                currency (ISO 4217, plain TEXT — no currency table), account_type_id, status. The RID
--                is the external handle (no separate `code`).
--   * OBJECT   — finance_cards: account_id -> finance_accounts (STRUCTURAL containment FK, CASCADE —
--                like order_items -> orders, NOT a reified Link), encrypted PAN + clear bin/last_four,
--                network_id, card_type (debit|credit), optional expiry + cardholder_person_id. There is
--                NO CVV/CVC column, ever (PCI-DSS Req 3.2 prohibits storing it after authorization).
--   * LINK     — finance_account_holders (link__held_by): the OWNERSHIP edge. Polymorphic holder
--                (holder_kind person|company + holder_id text, no FK — F-014, RID self-describes, like
--                vehicle_registrations.owner_id); role primary|joint|authorized_signer; temporal. Joint
--                and corporate accounts + account transfers all fall out of this edge.
--
-- Person-held finance rows are holder-scoped (D-PersonReadScope) + erased on person purge (a finance
-- PersonPurged subscriber; there is no person_id FK to CASCADE through on accounts). Expand-only
-- (L-UpgradeSafe / D-Migrations); depends on 0000 (new_id / platform_rid_types), 0003 tenant
-- (tenant_organizations), and 0005 person (person_persons). NB: storing the full PAN brings the
-- deployment into PCI-DSS cardholder-data scope; DS-54 (BIN+last-4-only) is the out-of-scope alternative.

-- ---------------------------------------------------------------------------------------------------
-- RID registry (D-ResourceIdentifiers). pkg/rid mirrors these and asserts equality at boot.
-- ---------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.platform_rid_services (code, module) VALUES (19, 'finance');

INSERT INTO oikumenea.platform_rid_types (service_code, kind, type_code, type_name) VALUES
  (19,1,1,'account'),        -- finance / object / account
  (19,1,2,'card'),           -- finance / object / card
  (19,1,3,'account_type'),   -- finance / object / account_type   (catalog)
  (19,1,4,'card_network'),   -- finance / object / card_network   (catalog)
  (19,2,1,'held_by');        -- finance / link   / held_by        (ownership)

-- ===================================================================================================
-- finance_account_types — the instance-admin catalog of account kinds (D-Code / D-i18n). RID Object,
-- stable `code` + translatable default-locale `name`. Seeded with a baseline; instance-extensible.
-- ===================================================================================================
CREATE TABLE oikumenea.finance_account_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(19,1,3),  -- finance / object / account_type
  code       text NOT NULL UNIQUE,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT finance_account_types_rid_shape
    CHECK (oikumenea.rid_service(id)=19 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);
CREATE TRIGGER finance_account_types_set_updated_at
  BEFORE UPDATE ON oikumenea.finance_account_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.finance_account_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_account_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_account_types.name IS 'pii:none';

INSERT INTO oikumenea.finance_account_types (code, name, sort_order) VALUES
  ('current',  'Current account',  10),
  ('savings',  'Savings account',  20),
  ('deposit',  'Deposit account',  30),
  ('loan',     'Loan account',     40),
  ('card',     'Card account',     50);

-- ===================================================================================================
-- finance_card_networks — the instance-admin catalog of payment-card networks (D-Code / D-i18n).
-- ===================================================================================================
CREATE TABLE oikumenea.finance_card_networks (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(19,1,4),  -- finance / object / card_network
  code       text NOT NULL UNIQUE,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  CONSTRAINT finance_card_networks_rid_shape
    CHECK (oikumenea.rid_service(id)=19 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=4)
);
CREATE TRIGGER finance_card_networks_set_updated_at
  BEFORE UPDATE ON oikumenea.finance_card_networks
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
COMMENT ON COLUMN oikumenea.finance_card_networks.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_card_networks.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_card_networks.name IS 'pii:none';

INSERT INTO oikumenea.finance_card_networks (code, name, sort_order) VALUES
  ('visa',        'Visa',             10),
  ('mastercard',  'Mastercard',       20),
  ('amex',        'American Express',  30),
  ('discover',    'Discover',          40),
  ('unionpay',    'UnionPay',          50),
  ('jcb',         'JCB',               60),
  ('mir',         'Mir',               70);

-- ===================================================================================================
-- finance_accounts — the bank account (Object account). institution_id is the holding bank (a
-- `company`-domain tenant_organizations row, RESTRICT — a bank in use is not hard-deleted). The IBAN is
-- pii:sensitive and ENVELOPE-ENCRYPTED (D-CryptoProvider): iban_ciphertext (AEAD of the value) +
-- iban_wrapped_dek (DEK wrapped by the KEK) + key_ref (the KEK id/version) + iban_blind_index (keyed
-- HMAC for equality/uniqueness). The plaintext is never stored. iban_ciphertext / iban_wrapped_dek are
-- NULLABLE so person purge can CRYPTO-ERASE (drop the wrapped DEK, null the ciphertext) while keeping
-- the row + its blind index. currency is a plain ISO-4217 TEXT (no currency table — M45 exclusion).
-- ===================================================================================================
CREATE TABLE oikumenea.finance_accounts (
  id                uuid PRIMARY KEY DEFAULT oikumenea.new_id(19,1,1),  -- finance / object / account
  institution_id    uuid NOT NULL REFERENCES oikumenea.tenant_organizations(id) ON DELETE RESTRICT,
  iban_ciphertext   bytea,                        -- AEAD ciphertext of the IBAN (NULL once crypto-erased)
  iban_wrapped_dek  bytea,                        -- per-record DEK wrapped by the KEK (NULL once crypto-erased)
  key_ref           text NOT NULL,                -- KEK id + version that produced iban_wrapped_dek
  iban_blind_index  bytea NOT NULL,               -- keyed HMAC of the normalized IBAN (opaque, not reversible)
  currency          text,                         -- ISO 4217 (e.g. UAH, USD); nullable, plain text
  account_type_id   uuid REFERENCES oikumenea.finance_account_types(id) ON DELETE RESTRICT,
  status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active','closed','frozen')),
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,
  CONSTRAINT finance_accounts_rid_shape
    CHECK (oikumenea.rid_service(id)=19 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);
CREATE TRIGGER finance_accounts_set_updated_at
  BEFORE UPDATE ON oikumenea.finance_accounts
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
-- The IBAN is unique among ACTIVE (non-deleted) accounts, by blind index (equality without decryption).
CREATE UNIQUE INDEX finance_accounts_iban_active
  ON oikumenea.finance_accounts (iban_blind_index) WHERE deleted_at IS NULL;
CREATE INDEX finance_accounts_institution_idx
  ON oikumenea.finance_accounts (institution_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.finance_accounts.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_accounts.institution_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_accounts.iban_ciphertext IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.finance_accounts.iban_wrapped_dek IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.finance_accounts.key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_accounts.iban_blind_index IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_accounts.currency IS 'pii:none';

-- ===================================================================================================
-- finance_cards — a payment card, contained by exactly one account (account_id CASCADE — a structural
-- containment FK, like order_items -> orders; NOT a reified Link). The full PAN is pii:sensitive and
-- envelope-encrypted exactly like the account IBAN, WITH display-only clear bin (first 6) + last_four.
-- card_type is debit|credit; expiry + a named cardholder_person_id are optional. There is NO CVV column.
-- ===================================================================================================
CREATE TABLE oikumenea.finance_cards (
  id                   uuid PRIMARY KEY DEFAULT oikumenea.new_id(19,1,2),  -- finance / object / card
  account_id           uuid NOT NULL REFERENCES oikumenea.finance_accounts(id) ON DELETE CASCADE,
  pan_ciphertext       bytea,                     -- AEAD ciphertext of the PAN (NULL once crypto-erased)
  pan_wrapped_dek      bytea,                     -- per-record DEK wrapped by the KEK (NULL once crypto-erased)
  key_ref              text NOT NULL,             -- KEK id + version that produced pan_wrapped_dek
  pan_blind_index      bytea NOT NULL,            -- keyed HMAC of the normalized PAN (opaque)
  bin                  char(6),                   -- display-only issuer id (first 6 of the PAN), clear
  last_four            char(4),                   -- display-only last 4 of the PAN, clear
  network_id           uuid REFERENCES oikumenea.finance_card_networks(id) ON DELETE RESTRICT,
  card_type            text NOT NULL CHECK (card_type IN ('debit','credit')),
  expiry_month         int CHECK (expiry_month BETWEEN 1 AND 12),
  expiry_year          int CHECK (expiry_year BETWEEN 2000 AND 2100),
  cardholder_person_id uuid REFERENCES oikumenea.person_persons(id) ON DELETE SET NULL,
  status               text NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked','expired')),
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,
  CONSTRAINT finance_cards_rid_shape
    CHECK (oikumenea.rid_service(id)=19 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);
CREATE TRIGGER finance_cards_set_updated_at
  BEFORE UPDATE ON oikumenea.finance_cards
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
-- The PAN is unique among ACTIVE (non-deleted) cards, by blind index.
CREATE UNIQUE INDEX finance_cards_pan_active
  ON oikumenea.finance_cards (pan_blind_index) WHERE deleted_at IS NULL;
CREATE INDEX finance_cards_account_idx
  ON oikumenea.finance_cards (account_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.finance_cards.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.account_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.pan_ciphertext IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.finance_cards.pan_wrapped_dek IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.finance_cards.key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.pan_blind_index IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.bin IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.last_four IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_cards.cardholder_person_id IS 'pii:none';

-- ===================================================================================================
-- finance_account_holders — the OWNERSHIP edge (link__held_by). account_id CASCADE. The holder is a
-- person OR a company: holder_kind discriminates, holder_id is the holder RID (text, no FK —
-- polymorphic, F-014, like vehicle_registrations.owner_id). role is primary|joint|authorized_signer;
-- temporal (effective_from/effective_to). Person-holder rows are pii:basic, holder-scoped, and erased
-- on person purge. At most one ACTIVE primary holder per account.
-- ===================================================================================================
CREATE TABLE oikumenea.finance_account_holders (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(19,2,1),  -- finance / link / held_by
  account_id     uuid NOT NULL REFERENCES oikumenea.finance_accounts(id) ON DELETE CASCADE,
  holder_kind    text NOT NULL CHECK (holder_kind IN ('person','company')),
  holder_id      text NOT NULL,         -- holder RID (person or company); polymorphic, no FK
  role           text NOT NULL DEFAULT 'primary' CHECK (role IN ('primary','joint','authorized_signer')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_to   timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,
  CONSTRAINT finance_account_holders_rid_shape
    CHECK (oikumenea.rid_service(id)=19 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=1),
  -- The polymorphic holder end carries no FK, so this shape CHECK is its only integrity on the id:
  -- a 'person' holder must be a person object RID (6,1,1); a 'company' holder must be a company-domain
  -- tenant ORGANIZATION RID (4,1,6) — M41/D-UnifiedOrgGraph: a company IS a tenant org, no own company
  -- object RID. The ::uuid cast also rejects a malformed id. Existence stays app-enforced (R-32).
  CONSTRAINT finance_account_holders_holder_shape CHECK (
    (holder_kind <> 'person'  OR (oikumenea.rid_service(holder_id::uuid)=6 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=1)) AND
    (holder_kind <> 'company' OR (oikumenea.rid_service(holder_id::uuid)=4 AND oikumenea.rid_kind(holder_id::uuid)=1 AND oikumenea.rid_type(holder_id::uuid)=6))
  )
);
CREATE TRIGGER finance_account_holders_set_updated_at
  BEFORE UPDATE ON oikumenea.finance_account_holders
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();
-- At most one ACTIVE primary holder per account (a joint/authorized_signer set is unbounded).
CREATE UNIQUE INDEX finance_account_holders_primary_active
  ON oikumenea.finance_account_holders (account_id)
  WHERE role = 'primary' AND effective_to IS NULL AND deleted_at IS NULL;
CREATE INDEX finance_account_holders_account_idx
  ON oikumenea.finance_account_holders (account_id) WHERE deleted_at IS NULL;
CREATE INDEX finance_account_holders_holder_idx
  ON oikumenea.finance_account_holders (holder_kind, holder_id) WHERE deleted_at IS NULL;
COMMENT ON COLUMN oikumenea.finance_account_holders.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_account_holders.account_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_account_holders.holder_kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.finance_account_holders.holder_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.finance_account_holders.role IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0034_finance', applied_at = now() WHERE singleton;
