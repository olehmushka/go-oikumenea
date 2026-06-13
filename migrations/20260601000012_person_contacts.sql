-- 0012 person contacts + document attribute schema (M12).
--
-- Person enrichment (docs/modules/person.md, D-PersonContactChannels): three multi-valued contact /
-- identity channels on a person — emails, phones, call signs — each an effective child table that
-- mirrors person_citizenships / person_residences (RID PK, person_id CASCADE, soft-delete, is_primary,
-- set_updated_at, every column PII-tiered). Email/phone `kind` is an instance-admin catalog (natural
-- `code` PK, translatable name — D-Code/D-i18n), seeded here (natural-key carve-out; D-RIDSeeding does
-- not apply). The contact email is DISTINCT from the login email (account_accounts.email) — no FK.
--
-- Document per-type attribute schema (docs/modules/document.md, D-DocumentAttrSchema): a nullable
-- document_document_types.attr_schema declaring the fields a document's `attributes` may/must carry,
-- validated on write. The military-id type's schema is set at boot (the type rows are RID-keyed and
-- seeded at boot, D-RIDSeeding), not here.
--
-- Expanded personal-code schemes (D-PersonalCodes): additive RU/BY/LATAM scheme rows in the
-- natural-key document_personal_code_schemes catalog. Compiled pkg/personalcode validators carry no
-- regex (the validator is authoritative); regex-only schemes get a fallback. All country_iso values are
-- in the 0001-seeded geo_countries registry.
--
-- These contact tables have NO unit column (scoped through the holder per D-PersonReadScope), so — like
-- person_persons / document_documents — they are EXEMPT from the RLS app.readable_units backstop
-- (D-RLSDefenseInDepth); no RLS is enabled on them.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid,
-- set_updated_at, citext, geo_countries), 0006 person (person_persons), 0010 document
-- (document_document_types, document_personal_code_schemes). person_email_types / person_phone_types
-- are seeded here (natural-key); the contact rows are created through PersonService.

-- ============================ contact-kind catalogs ============================

-- person_email_types: instance-admin catalog of contact-email kinds (D-Code/D-i18n). Natural `code` PK
-- (carve-out, like document_personal_code_schemes / geo_countries). name is the default-locale label;
-- other locales live in the localization store (entity_type='email_type').
CREATE TABLE oikumenea.person_email_types (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER person_email_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_email_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.person_email_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_email_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_email_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_email_types.sort_order IS 'pii:none';

-- person_phone_types: instance-admin catalog of contact-phone kinds (entity_type='phone_type').
CREATE TABLE oikumenea.person_phone_types (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER person_phone_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_phone_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.person_phone_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phone_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phone_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phone_types.sort_order IS 'pii:none';

-- Seed the contact-kind catalogs (natural-key carve-out). The instance admin adds more via the API.
INSERT INTO oikumenea.person_email_types (code, name, sort_order) VALUES
  ('personal', 'Personal',  0),
  ('work',     'Work',     10),
  ('other',    'Other',    20);

INSERT INTO oikumenea.person_phone_types (code, name, sort_order) VALUES
  ('mobile', 'Mobile',  0),
  ('home',   'Home',   10),
  ('work',   'Work',   20),
  ('other',  'Other',  30);

-- ============================ contact channels ============================

-- person_emails: multi-valued contact email (D-PersonContactChannels). address is citext (the index is
-- therefore case-insensitive); provider is derived on write from the address domain (gmail.com→google).
-- One ACTIVE row per (person, address); is_primary marks at most one active. CASCADE on person delete;
-- erased on purge. pii:contact. DISTINCT from the login email — no FK to account_accounts.
CREATE TABLE oikumenea.person_emails (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,5),  -- person / object / email
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  type_code  text NOT NULL REFERENCES oikumenea.person_email_types(code) ON DELETE RESTRICT,
  address    citext NOT NULL,
  provider   text,                                -- derived on write; NULL when no mapping
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_emails_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=5)
);

CREATE TRIGGER person_emails_set_updated_at
  BEFORE UPDATE ON oikumenea.person_emails
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_emails_active_address_idx
  ON oikumenea.person_emails (person_id, address) WHERE deleted_at IS NULL;
CREATE INDEX person_emails_person_idx
  ON oikumenea.person_emails (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_emails.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_emails.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_emails.type_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_emails.address IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_emails.provider IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_emails.is_primary IS 'pii:none';

-- person_phones: multi-valued contact phone (D-PersonContactChannels). number is stored E.164-normalized
-- by PersonService (github.com/nyaruka/phonenumbers); country is derived from the number and FK'd to the
-- geo registry. Carrier/provider is NOT stored (not statically derivable — DS-40). One ACTIVE row per
-- (person, number); is_primary marks at most one active. CASCADE on person delete; erased on purge.
CREATE TABLE oikumenea.person_phones (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,6),  -- person / object / phone
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  type_code  text NOT NULL REFERENCES oikumenea.person_phone_types(code) ON DELETE RESTRICT,
  number     text NOT NULL,                       -- E.164-normalized
  country    char(2) REFERENCES oikumenea.geo_countries(code) ON DELETE RESTRICT,  -- derived; nullable
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_phones_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=6)
);

CREATE TRIGGER person_phones_set_updated_at
  BEFORE UPDATE ON oikumenea.person_phones
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_phones_active_number_idx
  ON oikumenea.person_phones (person_id, number) WHERE deleted_at IS NULL;
CREATE INDEX person_phones_person_idx
  ON oikumenea.person_phones (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_phones.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phones.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phones.type_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_phones.number IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_phones.country IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_phones.is_primary IS 'pii:none';

-- person_call_signs: multi-valued informal identifier / позивний (D-PersonContactChannels). call_sign is
-- NOT NULL, pii:basic, and UNIQUE per person among active rows. is_primary marks at most one active.
-- CASCADE on person delete; erased on purge.
CREATE TABLE oikumenea.person_call_signs (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,7),  -- person / object / call_sign
  person_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  call_sign  text NOT NULL,
  is_primary boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_call_signs_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=7)
);

CREATE TRIGGER person_call_signs_set_updated_at
  BEFORE UPDATE ON oikumenea.person_call_signs
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One active call sign per (person, value); the leading person_id also serves the list lookup.
CREATE UNIQUE INDEX person_call_signs_active_idx
  ON oikumenea.person_call_signs (person_id, call_sign) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_call_signs.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_call_signs.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_call_signs.call_sign IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_call_signs.is_primary IS 'pii:none';

-- ============================ document attribute schema (D) ============================

-- Per-document-type attribute schema (D-DocumentAttrSchema). Nullable: when set, a document's
-- `attributes` is validated against it on every write; when null, attributes is free-form. The
-- military-id type's schema is seeded at boot (RID-keyed type rows; D-RIDSeeding), not here.
ALTER TABLE oikumenea.document_document_types ADD COLUMN attr_schema jsonb;
COMMENT ON COLUMN oikumenea.document_document_types.attr_schema IS 'pii:none';

-- ============================ expanded personal-code schemes (C) ============================

-- Additive RU/BY/LATAM national-identifier schemes (D-PersonalCodes). Schemes with a compiled
-- pkg/personalcode checksum validator carry NO regex (the validator is authoritative); regex-only
-- schemes (ar-dni, co-cedula, by-personal-number) carry a fallback. country_iso values are all in the
-- 0001 geo registry.
INSERT INTO oikumenea.document_personal_code_schemes (code, country_iso, generic_category, name, validation_regex, sort_order) VALUES
  ('ru-inn',             'RU', 'tax-id',           'ИНН',             NULL,                                  60),
  ('ru-snils',           'RU', 'social-insurance', 'СНИЛС',           NULL,                                  70),
  ('by-personal-number', 'BY', 'national-id',      'Асабовы нумар',   '^\d{7}[A-Za-z]\d{3}[A-Za-z]{2}\d$',   80),
  ('br-cpf',             'BR', 'tax-id',           'CPF',             NULL,                                  90),
  ('ar-dni',             'AR', 'national-id',      'DNI',             '^\d{7,8}$',                          100),
  ('ar-cuil',            'AR', 'tax-id',           'CUIL',            NULL,                                 110),
  ('mx-curp',            'MX', 'national-id',      'CURP',            NULL,                                 120),
  ('mx-rfc',             'MX', 'tax-id',           'RFC',             NULL,                                 130),
  ('cl-rut',             'CL', 'national-id',      'RUT',             NULL,                                 140),
  ('co-cedula',          'CO', 'national-id',      'Cédula de Ciudadanía', '^\d{6,10}$',                   150);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0012_person_contacts', applied_at = now() WHERE singleton;
