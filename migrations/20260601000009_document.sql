-- 0009 document (M9).
--
-- Person-held papers and government personal codes (docs/modules/document.md): what a person HAS,
-- distinct from an order (an administrative act). Two parallel models split by D-Documents /
-- D-PersonalCodes:
--   * PAPERS  — document_documents, typed by the instance-admin document_document_types catalog
--               (passport, driver-license, military-id). Metadata only; number/issuer are pii:basic.
--   * CODES   — document_personal_codes, typed by the country-namespaced document_personal_code_schemes
--               catalog (ua-rnokpp, us-ssn). The value is pii:sensitive, ENVELOPE-ENCRYPTED at rest
--               (D-CryptoProvider): ciphertext + wrapped DEK + key_ref + a keyed-HMAC blind index for
--               equality lookup / cross-person uniqueness without decryption. The plaintext is never
--               stored; person purge crypto-erases it (drop wrapped_dek, null ciphertext).
--
-- A document/code carries NO authority (directory data, like rank/position); access is decided by the
-- PDP, scoped THROUGH THE HOLDER (D-PersonReadScope) + the shadow gate. Expand-only (L-UpgradeSafe);
-- depends on 0001 bootstrap (new_rid, set_updated_at, geo_countries), 0003 localization (translatable
-- type/scheme names join the i18n store), and 0006 person (the holder FK).
--
-- RID-keyed tables (document_document_types, document_documents, document_personal_codes) seed NO rows
-- here — that needs the app.environment GUC atlas does not set (D-RIDSeeding); the type catalog is
-- seeded at boot in document.Register. document_personal_code_schemes keeps a NATURAL `code` PK (the
-- D-ResourceIdentifiers carve-out, like geo_countries / i18n_locales) and IS seeded here.

-- document_document_types: the instance-admin catalog of PAPER kinds (D-Documents). RID Object with a
-- stable, locale-agnostic `code` (immutable by convention) + a translatable default-locale `name`.
CREATE TABLE oikumenea.document_document_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(10,1,1),  -- document / object / document_type
  code       text NOT NULL UNIQUE,               -- stable locale-agnostic identifier (D-Code)
  name       text NOT NULL,                      -- default-locale label; translatable via the i18n store
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT document_document_types_rid_shape
    CHECK (oikumenea.rid_service(id)=10 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER document_document_types_set_updated_at
  BEFORE UPDATE ON oikumenea.document_document_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.document_document_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_document_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_document_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_document_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_document_types.sort_order IS 'pii:none';

-- document_documents: a person-held PAPER of some type, with its number, issuer, issuing country, and
-- validity window. number/issuer are pii:basic; attributes is the pii:special CEILING (a grab-bag).
-- `status` (active|superseded|revoked) is admin-set, self-asserted, reversible, and ORTHOGONAL to
-- deleted_at (soft-delete of the record). person/type FKs are RESTRICT (a type in use is retired, not
-- hard-deleted; a held paper does not vanish with a hard person delete — purge erases instead).
CREATE TABLE oikumenea.document_documents (
  id               uuid PRIMARY KEY DEFAULT oikumenea.new_id(10,1,2),  -- document / object / document
  person_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  type_id          uuid NOT NULL REFERENCES oikumenea.document_document_types(id) ON DELETE RESTRICT,
  number           text,                          -- document number (passport no., licence no.)
  issuer           text,                          -- issuing authority (e.g. ДМС України)
  issuing_country  char(2) REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,  -- nullable (D-Geo)
  issued_on        date,
  expires_on       date,
  attributes       jsonb NOT NULL DEFAULT '{}',   -- long-tail per-type fields; pii:special CEILING
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded','revoked')),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz,

  CONSTRAINT document_documents_rid_shape
    CHECK (oikumenea.rid_service(id)=10 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);

CREATE TRIGGER document_documents_set_updated_at
  BEFORE UPDATE ON oikumenea.document_documents
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- A person does not hold the same numbered document twice (among active rows; D-Documents invariant).
CREATE UNIQUE INDEX document_documents_person_type_number_idx
  ON oikumenea.document_documents (person_id, type_id, number)
  WHERE number IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX document_documents_person_idx
  ON oikumenea.document_documents (person_id) WHERE deleted_at IS NULL;
CREATE INDEX document_documents_type_idx ON oikumenea.document_documents (type_id);

COMMENT ON COLUMN oikumenea.document_documents.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.number IS 'pii:basic';
COMMENT ON COLUMN oikumenea.document_documents.issuer IS 'pii:basic';
COMMENT ON COLUMN oikumenea.document_documents.issuing_country IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.issued_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.expires_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.attributes IS 'pii:special';
COMMENT ON COLUMN oikumenea.document_documents.status IS 'pii:none';

-- document_personal_code_schemes: the instance-admin catalog of country-namespaced national-identifier
-- schemes (D-PersonalCodes). Natural `code` PK (the D-ResourceIdentifiers carve-out — a seeded shared
-- reference registry FK'd by code, like geo_countries / i18n_locales). generic_category is the
-- cross-scheme join key ("list everyone's tax IDs"); validation_regex is the data-side FALLBACK behind
-- a compiled pkg/personalcode validator.
CREATE TABLE oikumenea.document_personal_code_schemes (
  code             text PRIMARY KEY,              -- the scheme id, e.g. ua-rnokpp, us-ssn (D-Code)
  country_iso      char(2) REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,  -- NOT NULL for national schemes
  generic_category text NOT NULL CHECK (generic_category IN
                     ('tax-id','national-id','social-insurance','health-insurance','residence-permit','other')),
  name             text NOT NULL,                 -- default-locale label; translatable via the i18n store
  validation_regex text,                          -- optional data-side fallback format check
  status           text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order       int,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at       timestamptz
);

CREATE TRIGGER document_personal_code_schemes_set_updated_at
  BEFORE UPDATE ON oikumenea.document_personal_code_schemes
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.document_personal_code_schemes.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.country_iso IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.generic_category IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.validation_regex IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.sort_order IS 'pii:none';

-- document_personal_codes: a person-held government identifier of some scheme. The value is pii:sensitive
-- and ENVELOPE-ENCRYPTED (D-CryptoProvider): value_ciphertext (AEAD of the value) + wrapped_dek (DEK
-- wrapped by the KMS-held KEK) + key_ref (the KEK id/version) + value_blind_index (keyed HMAC for
-- equality lookup / uniqueness). The plaintext is never stored. value_ciphertext / wrapped_dek are
-- NULLABLE so person purge can CRYPTO-ERASE (drop the wrapped DEK, null the ciphertext) while keeping
-- the row id as a tombstone; on active rows the app always sets them. Country derives from the scheme.
CREATE TABLE oikumenea.document_personal_codes (
  id                 uuid PRIMARY KEY DEFAULT oikumenea.new_id(10,1,3),  -- document / object / personal_code
  person_id          uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  scheme_code        text NOT NULL REFERENCES oikumenea.document_personal_code_schemes(code) ON DELETE RESTRICT,
  value_ciphertext   bytea,                       -- AEAD ciphertext of the value (NULL once crypto-erased)
  wrapped_dek        bytea,                       -- per-record DEK wrapped by the KEK (NULL once crypto-erased)
  key_ref            text NOT NULL,               -- KEK id + version that produced wrapped_dek
  value_blind_index  bytea NOT NULL,              -- keyed HMAC of the normalized value (opaque, not reversible)
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded','revoked')),
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,

  CONSTRAINT document_personal_codes_rid_shape
    CHECK (oikumenea.rid_service(id)=10 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);

CREATE TRIGGER document_personal_codes_set_updated_at
  BEFORE UPDATE ON oikumenea.document_personal_codes
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- The same code in the same scheme is not held cross-person (over the blind index, since the value is
-- ciphertext); among active rows only (D-PersonalCodes invariant).
CREATE UNIQUE INDEX document_personal_codes_scheme_value_idx
  ON oikumenea.document_personal_codes (scheme_code, value_blind_index)
  WHERE deleted_at IS NULL;
CREATE INDEX document_personal_codes_person_idx
  ON oikumenea.document_personal_codes (person_id) WHERE deleted_at IS NULL;
CREATE INDEX document_personal_codes_scheme_idx ON oikumenea.document_personal_codes (scheme_code);

COMMENT ON COLUMN oikumenea.document_personal_codes.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_codes.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_codes.scheme_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_codes.value_ciphertext IS 'pii:sensitive';
COMMENT ON COLUMN oikumenea.document_personal_codes.wrapped_dek IS 'secret';
COMMENT ON COLUMN oikumenea.document_personal_codes.key_ref IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_codes.value_blind_index IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_personal_codes.status IS 'pii:none';

-- Seed the personal-code scheme catalog (natural-key carve-out; D-RIDSeeding does not apply). A
-- representative country-namespaced set; the instance admin adds more via the API. Schemes with a
-- compiled pkg/personalcode validator carry no regex (the validator is authoritative); others get a
-- fallback regex. Country codes reference the ISO-3166 geo registry seeded in 0001.
INSERT INTO oikumenea.document_personal_code_schemes (code, country_iso, generic_category, name, validation_regex, sort_order) VALUES
  ('ua-rnokpp',         'UA', 'tax-id',          'РНОКПП',         NULL,                  0),
  ('ua-unzr',           'UA', 'national-id',     'УНЗР',           '^\d{8}-\d{5}$',      10),
  ('us-ssn',            'US', 'social-insurance','Social Security Number', NULL,          20),
  ('de-steuer-id',      'DE', 'tax-id',          'Steuer-ID',      '^\d{11}$',           30),
  ('it-codice-fiscale', 'IT', 'tax-id',          'Codice Fiscale', '^[A-Za-z0-9]{16}$',  40),
  ('pl-pesel',          'PL', 'national-id',     'PESEL',          NULL,                  50);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0009_document', applied_at = now() WHERE singleton;
