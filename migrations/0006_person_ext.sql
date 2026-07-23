-- 0006_person_ext — merged domain migration (refactor: consolidated from 0012_person_contacts, 0013_person_social_channels, 0014_person_relationships, 0015_tenant_units_public_read_rls, 0016_person_date_of_death, 0017_import_provenance).

-- ===== merged from 0012_person_contacts =====
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
-- (carve-out, like document_personal_code_schemes / i18n_locales). name is the default-locale label;
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
  country_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- derived; nullable; ISO code resolved in SQL
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
COMMENT ON COLUMN oikumenea.person_phones.country_id IS 'pii:contact';
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
INSERT INTO oikumenea.document_personal_code_schemes (code, country_id, generic_category, name, validation_regex, sort_order)
SELECT v.code, c.id, v.generic_category, v.name, v.validation_regex, v.sort_order
FROM (VALUES
  ('ru-inn',             'RU', 'tax-id',           'ИНН',             NULL::text,                            60),
  ('ru-snils',           'RU', 'social-insurance', 'СНИЛС',           NULL,                                  70),
  ('by-personal-number', 'BY', 'national-id',      'Асабовы нумар',   '^\d{7}[A-Za-z]\d{3}[A-Za-z]{2}\d$',   80),
  ('br-cpf',             'BR', 'tax-id',           'CPF',             NULL,                                  90),
  ('ar-dni',             'AR', 'national-id',      'DNI',             '^\d{7,8}$',                          100),
  ('ar-cuil',            'AR', 'tax-id',           'CUIL',            NULL,                                 110),
  ('mx-curp',            'MX', 'national-id',      'CURP',            NULL,                                 120),
  ('mx-rfc',             'MX', 'tax-id',           'RFC',             NULL,                                 130),
  ('cl-rut',             'CL', 'national-id',      'RUT',             NULL,                                 140),
  ('co-cedula',          'CO', 'national-id',      'Cédula de Ciudadanía', '^\d{6,10}$',                   150)
) AS v(code, country_iso, generic_category, name, validation_regex, sort_order)
JOIN oikumenea.geo_countries c ON c.code = v.country_iso;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0013_person_social_channels =====
-- 0013 person social & messenger channels (M13).
--
-- Person enrichment (docs/modules/person.md, D-PersonSocialChannels): a person's messenger
-- reachability + social-network presence, in two additive layers over the M12 contact channels plus an
-- instance-admin platform catalog. All follow the existing person child-table pattern (RID PK,
-- parent FK CASCADE, soft-delete, is_primary, set_updated_at, every column PII-tiered), holder-scoped
-- on read (D-PersonReadScope), audited on write, erased on person purge.
--
--   person_platforms              — instance-admin catalog (natural `code` PK carve-out, like
--                                   person_email_types; category messenger|social; translatable name in
--                                   the localization store entity_type='platform').
--   person_messenger_links        — layer a: reachability over an existing phone OR email (XOR FK), on a
--                                   `messenger`-category platform. Link link__reachable_on.
--   person_social_accounts        — layer b: a standalone catalog-typed handle with a stable platform id
--                                   (immutable) vs mutable handle, platform-vs-operator verification, and
--                                   source/confidence attribution on the HOLDS_ACCOUNT link. Object
--                                   PersonSocialAccount.
--   person_social_account_handles — handle-rename history (temporal) so a rename never breaks the link.
--
-- DS-29-gated: the social account's free-text `bio` + `self_declared_location` are pii:sensitive and are
-- NOT created here (they wait on the envelope-encryption seam). No time-series social-graph metrics are
-- stored (excluded outright; D-PersonSocialChannels).
--
-- These tables have NO unit column (scoped through the holder per D-PersonReadScope), so — like
-- person_persons / person_emails — they are EXEMPT from the RLS app.readable_units backstop
-- (D-RLSDefenseInDepth); no RLS is enabled on them.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid, set_updated_at,
-- citext), 0006 person (person_persons), 0013 person contacts (person_emails, person_phones).
-- person_platforms is seeded here (natural-key); the channel rows are created through PersonService.

-- ============================ platform catalog ============================

-- person_platforms: instance-admin catalog of social networks / messengers (D-Code/D-i18n). Natural
-- `code` PK (carve-out, like person_email_types). name is the default-locale label; other locales live
-- in the localization store (entity_type='platform'). category partitions reachability (messenger) from
-- standalone-account (social) platforms.
CREATE TABLE oikumenea.person_platforms (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  category   text NOT NULL CHECK (category IN ('messenger','social')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER person_platforms_set_updated_at
  BEFORE UPDATE ON oikumenea.person_platforms
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.person_platforms.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_platforms.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_platforms.category IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_platforms.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_platforms.sort_order IS 'pii:none';

-- Seed the platform catalog (natural-key carve-out). The instance admin adds more via the API.
INSERT INTO oikumenea.person_platforms (code, name, category, sort_order) VALUES
  ('telegram',      'Telegram',      'messenger',  0),
  ('whatsapp',      'WhatsApp',      'messenger', 10),
  ('signal',        'Signal',        'messenger', 20),
  ('viber',         'Viber',         'messenger', 30),
  ('threema',       'Threema',       'messenger', 32),
  ('milchat',       'MilChat',       'messenger', 34),
  ('instagram',     'Instagram',     'social',    40),
  ('linkedin',      'LinkedIn',      'social',    50),
  ('x',             'X',             'social',    60),
  ('facebook',      'Facebook',      'social',    70),
  ('vkontakte',     'VKontakte',     'social',    80),
  ('odnoklassniki', 'Odnoklassniki', 'social',    90),
  ('bluesky',       'Bluesky',       'social',   100),
  ('mastodon',      'Mastodon',      'social',   110);

-- ============================ messenger links (layer a) ============================

-- person_messenger_links: annotates an existing phone OR email with reachability on a messenger
-- platform (D-PersonSocialChannels). Exactly one of phone_id/email_id is non-null (XOR CHECK); both
-- CASCADE when the underlying channel is hard-deleted. platform_code is write-time restricted to a
-- category='messenger' platform (enforced in the application + domain; the FK only checks existence).
-- One active link per (phone_id, platform_code) / (email_id, platform_code). Erased on person purge.
CREATE TABLE oikumenea.person_messenger_links (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,8),  -- person / object / messenger_link
  phone_id      uuid REFERENCES oikumenea.person_phones(id) ON DELETE CASCADE,
  email_id      uuid REFERENCES oikumenea.person_emails(id) ON DELETE CASCADE,
  platform_code text NOT NULL REFERENCES oikumenea.person_platforms(code) ON DELETE RESTRICT,
  is_primary    boolean NOT NULL DEFAULT false,
  verified_at   timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_messenger_links_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=8),
  CONSTRAINT person_messenger_links_channel_xor CHECK ((phone_id IS NOT NULL) <> (email_id IS NOT NULL))
);

CREATE TRIGGER person_messenger_links_set_updated_at
  BEFORE UPDATE ON oikumenea.person_messenger_links
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_messenger_links_active_phone_idx
  ON oikumenea.person_messenger_links (phone_id, platform_code) WHERE deleted_at IS NULL AND phone_id IS NOT NULL;
CREATE UNIQUE INDEX person_messenger_links_active_email_idx
  ON oikumenea.person_messenger_links (email_id, platform_code) WHERE deleted_at IS NULL AND email_id IS NOT NULL;
CREATE INDEX person_messenger_links_phone_idx
  ON oikumenea.person_messenger_links (phone_id) WHERE deleted_at IS NULL AND phone_id IS NOT NULL;
CREATE INDEX person_messenger_links_email_idx
  ON oikumenea.person_messenger_links (email_id) WHERE deleted_at IS NULL AND email_id IS NOT NULL;

COMMENT ON COLUMN oikumenea.person_messenger_links.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_messenger_links.phone_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_messenger_links.email_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_messenger_links.platform_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_messenger_links.is_primary IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_messenger_links.verified_at IS 'pii:none';

-- ============================ social accounts (layer b) ============================

-- person_social_accounts: a standalone social-network account, independent of any phone/email
-- (D-PersonSocialChannels). platform_user_id is the platform's IMMUTABLE internal id (the durable key,
-- nullable when unknown); handle is the MUTABLE current @handle (rename history in
-- person_social_account_handles). platform_verified ("blue-check") is distinct from
-- verified_by_operator_at (operator confirmation). source/confidence carry the analytics-grade
-- attribution of the HOLDS_ACCOUNT claim. Erased on person purge.
--
-- DS-29-gated (NOT created here): free-text bio + self_declared_location (pii:sensitive).
CREATE TABLE oikumenea.person_social_accounts (
  id                      uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,9),  -- person / object / social_account
  person_id               uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  platform_code           text NOT NULL REFERENCES oikumenea.person_platforms(code) ON DELETE RESTRICT,
  platform_user_id        text,                                 -- immutable durable key; null when unknown
  handle                  text NOT NULL,                        -- mutable current @handle
  display_name            text,
  profile_url             text,                                 -- derived on write
  language                text,
  platform_verified       boolean NOT NULL DEFAULT false,       -- platform "blue-check"
  verified_by_operator_at timestamptz,                          -- operator confirmation; distinct
  source                  text NOT NULL CHECK (source IN ('self_declared','operator_verified','imported')),
  confidence              text NOT NULL DEFAULT 'possible' CHECK (confidence IN ('confirmed','probable','possible')),
  is_primary              boolean NOT NULL DEFAULT false,
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  deleted_at              timestamptz,

  CONSTRAINT person_social_accounts_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=9)
);

CREATE TRIGGER person_social_accounts_set_updated_at
  BEFORE UPDATE ON oikumenea.person_social_accounts
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- One active row per (person, platform, stable id) when the id is known, else per (person, platform,
-- lower(handle)). The two partial-unique indexes are mutually exclusive on platform_user_id null-ness.
CREATE UNIQUE INDEX person_social_accounts_active_uid_idx
  ON oikumenea.person_social_accounts (person_id, platform_code, platform_user_id)
  WHERE deleted_at IS NULL AND platform_user_id IS NOT NULL;
CREATE UNIQUE INDEX person_social_accounts_active_handle_idx
  ON oikumenea.person_social_accounts (person_id, platform_code, lower(handle))
  WHERE deleted_at IS NULL AND platform_user_id IS NULL;
CREATE INDEX person_social_accounts_person_idx
  ON oikumenea.person_social_accounts (person_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_social_accounts.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.platform_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.platform_user_id IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_accounts.handle IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_accounts.display_name IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_accounts.profile_url IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_accounts.language IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_accounts.platform_verified IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.verified_by_operator_at IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.confidence IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_accounts.is_primary IS 'pii:none';

-- ============================ social account handle history ============================

-- person_social_account_handles: the rename history of a social account's @handle (D-PersonSocialChannels)
-- so a rename never breaks the account link. valid_to IS NULL marks the current handle. CASCADE when the
-- account is hard-deleted; erased on person purge (via the account cascade + explicit DeleteAll).
CREATE TABLE oikumenea.person_social_account_handles (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,1,10),  -- person / object / social_handle
  account_id uuid NOT NULL REFERENCES oikumenea.person_social_accounts(id) ON DELETE CASCADE,
  handle     text NOT NULL,
  valid_from timestamptz NOT NULL,
  valid_to   timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_social_account_handles_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=10)
);

CREATE TRIGGER person_social_account_handles_set_updated_at
  BEFORE UPDATE ON oikumenea.person_social_account_handles
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX person_social_account_handles_account_idx
  ON oikumenea.person_social_account_handles (account_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_social_account_handles.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_account_handles.account_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_account_handles.handle IS 'pii:contact';
COMMENT ON COLUMN oikumenea.person_social_account_handles.valid_from IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_social_account_handles.valid_to IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0014_person_relationships =====
-- 0014 person↔person relationships (M14).
--
-- Person enrichment (docs/modules/person.md, D-PersonRelationships): family and social structure
-- between two in-directory persons, modelled as PER-TYPE reified self-links (Person → Person, D-Ontology
-- link__<type>) — never one generic table, never a bare FK. Each mirrors the membership_memberships
-- temporal-link shape (RID PK, soft-delete, set_updated_at, effective interval + status where a lifecycle
-- applies), is instance-global, holder-scoped on read (D-PersonReadScope), audited on write, and erased
-- when EITHER endpoint person purges. Authority NEVER derives from a relationship (D-Rank) — directory data.
--
--   person_relation_types  — instance-admin catalog for the open-ended relation labels (natural `code` PK
--                            carve-out, like person_platforms; category sponsorship|association|next_of_kin;
--                            translatable name in the localization store entity_type='relation_type').
--   person_partnerships     — marriage + engagement, symmetric canonical pair; link__partnered_with.
--   person_kinships         — directional parent_of (siblings derived); link__kin_parent_of.
--   person_guardianships    — guardian → ward (distinct from blood kin); link__guardian_of.
--   person_sponsorships     — godparent / advisor / mentor (catalog-typed); link__sponsor_of.
--   person_next_of_kin      — in-directory nomination + priority; link__next_of_kin.
--   person_associations     — associate / COI / no-contact, symmetric; link__associated_with.
--
-- (A friend/follower person_social_links / link__social_tie was scoped but DEFERRED — see decisions.md
-- D-PersonRelationships: no consumer, no authoritative source, and redundant with person_associations
-- for the actionable COI/no-contact case; it returns only with a real account-level model.)
--
-- These tables have NO unit column (scoped through the holder per D-PersonReadScope), so — like
-- person_persons / person_emails / person_social_accounts — they are EXEMPT from the RLS
-- app.readable_units backstop (D-RLSDefenseInDepth); no RLS is enabled on them.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on 0001 schema bootstrap (new_rid, set_updated_at),
-- 0006 person (person_persons). person_relation_types is seeded here (natural-key); the link rows are
-- created through PersonService.

-- ============================ relation-type catalog ============================

-- person_relation_types: instance-admin catalog of open-ended person↔person relation labels (D-Code/D-i18n).
-- Natural `code` PK (carve-out, like person_platforms). name is the default-locale label; other locales
-- live in the localization store (entity_type='relation_type'). category scopes which link type a label
-- applies to (sponsorship/association/next_of_kin); fixed lifecycle statuses (partnership, kinship) stay
-- TEXT+CHECK on their own tables and do NOT use this catalog.
CREATE TABLE oikumenea.person_relation_types (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  category   text NOT NULL CHECK (category IN ('sponsorship','association','next_of_kin')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER person_relation_types_set_updated_at
  BEFORE UPDATE ON oikumenea.person_relation_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.person_relation_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.category IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_relation_types.sort_order IS 'pii:none';

-- Seed the relation-type catalog (natural-key carve-out). The instance admin adds more via the API.
-- Sponsorship labels are required (person_sponsorships.relation_code is NOT NULL).
INSERT INTO oikumenea.person_relation_types (code, name, category, sort_order) VALUES
  ('godparent',        'Godparent',         'sponsorship',   0),
  ('academic_advisor', 'Academic advisor',  'sponsorship',  10),
  ('military_mentor',  'Military mentor',    'sponsorship',  20),
  ('spouse',           'Spouse',             'next_of_kin',  30),
  ('parent',           'Parent',             'next_of_kin',  40),
  ('child',            'Child',              'next_of_kin',  50),
  ('sibling',          'Sibling',            'next_of_kin',  60),
  ('next_of_kin_other','Other (next of kin)','next_of_kin',  70),
  ('colleague',        'Colleague',          'association',  80),
  ('business_associate','Business associate','association',  90);

-- ============================ partnerships (marriage + engagement) ============================

-- person_partnerships: marriage AND engagement folded into one lifecycle (D-PersonRelationships). A
-- SYMMETRIC pair stored in canonical order (person_id_a < person_id_b, CHECK; no self-pair). At most one
-- active engaged-or-married row per person — the active-pair partial-unique index below stops a duplicate
-- between the SAME two people; the broader "one active per person with anyone" is enforced in the
-- application (a partial-unique index cannot span both columns). effective_to NULL = ongoing. Link
-- link__partnered_with. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_partnerships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,2),  -- person / link / partnered_with
  person_id_a    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  person_id_b    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  status         text NOT NULL CHECK (status IN ('engaged','married','divorced','widowed','annulled','dissolved')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_partnerships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=2),
  CONSTRAINT person_partnerships_canonical_pair CHECK (person_id_a < person_id_b)
);

CREATE TRIGGER person_partnerships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_partnerships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_partnerships_active_pair_idx
  ON oikumenea.person_partnerships (person_id_a, person_id_b)
  WHERE deleted_at IS NULL AND status IN ('engaged','married');
CREATE INDEX person_partnerships_person_a_idx
  ON oikumenea.person_partnerships (person_id_a) WHERE deleted_at IS NULL;
CREATE INDEX person_partnerships_person_b_idx
  ON oikumenea.person_partnerships (person_id_b) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_partnerships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.person_id_a IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.person_id_b IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_partnerships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_partnerships.effective_to IS 'pii:basic';

-- ============================ kinships (directional parentage) ============================

-- person_kinships: directional blood/legal parentage (parent_id → child_id; D-PersonRelationships).
-- Siblings are DERIVED (shared parent), never stored. Distinct RID from tenant's unit link__parent_of.
-- Link link__kin_parent_of. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_kinships (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,3),  -- person / link / kin_parent_of
  parent_id  uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  child_id   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disestablished')),
  -- native validity (D-Temporal, R-31): the interval this kinship holds; NULL valid_to = active.
  valid_from timestamptz NOT NULL DEFAULT now(),
  valid_to   timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_kinships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=3),
  CONSTRAINT person_kinships_no_self CHECK (parent_id <> child_id)
);

CREATE TRIGGER person_kinships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_kinships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_kinships_active_pair_idx
  ON oikumenea.person_kinships (parent_id, child_id) WHERE deleted_at IS NULL;
CREATE INDEX person_kinships_parent_idx
  ON oikumenea.person_kinships (parent_id) WHERE deleted_at IS NULL;
CREATE INDEX person_kinships_child_idx
  ON oikumenea.person_kinships (child_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_kinships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.parent_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.child_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_kinships.status IS 'pii:none';

-- ============================ guardianships (legal guardian → ward) ============================

-- person_guardianships: legal guardian → ward, distinct from blood parent_of (D-PersonRelationships).
-- relation_code is an optional catalog label. effective_to NULL = ongoing. Link link__guardian_of.
-- Erased when either endpoint purges.
CREATE TABLE oikumenea.person_guardianships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,4),  -- person / link / guardian_of
  guardian_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  ward_id        uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code  text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_guardianships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=4),
  CONSTRAINT person_guardianships_no_self CHECK (guardian_id <> ward_id)
);

CREATE TRIGGER person_guardianships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_guardianships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_guardianships_active_pair_idx
  ON oikumenea.person_guardianships (guardian_id, ward_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_guardianships_guardian_idx
  ON oikumenea.person_guardianships (guardian_id) WHERE deleted_at IS NULL;
CREATE INDEX person_guardianships_ward_idx
  ON oikumenea.person_guardianships (ward_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_guardianships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.guardian_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.ward_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_guardianships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_guardianships.effective_to IS 'pii:basic';

-- ============================ sponsorships (godparent / advisor / mentor) ============================

-- person_sponsorships: sponsor → sponsored, catalog-typed relation kind (godparent / academic advisor /
-- military mentor; D-PersonRelationships). relation_code is REQUIRED and must reference a
-- category='sponsorship' relation type (the category is enforced in the application). effective_to NULL =
-- ongoing. Link link__sponsor_of. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_sponsorships (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,5),  -- person / link / sponsor_of
  sponsor_id     uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  sponsored_id   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code  text NOT NULL REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  status         text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  effective_from date,
  effective_to   date,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz,

  CONSTRAINT person_sponsorships_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=5),
  CONSTRAINT person_sponsorships_no_self CHECK (sponsor_id <> sponsored_id)
);

CREATE TRIGGER person_sponsorships_set_updated_at
  BEFORE UPDATE ON oikumenea.person_sponsorships
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_sponsorships_active_idx
  ON oikumenea.person_sponsorships (sponsor_id, sponsored_id, relation_code)
  WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_sponsorships_sponsor_idx
  ON oikumenea.person_sponsorships (sponsor_id) WHERE deleted_at IS NULL;
CREATE INDEX person_sponsorships_sponsored_idx
  ON oikumenea.person_sponsorships (sponsored_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_sponsorships.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.sponsor_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.sponsored_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_sponsorships.effective_from IS 'pii:basic';
COMMENT ON COLUMN oikumenea.person_sponsorships.effective_to IS 'pii:basic';

-- ============================ next of kin (in-directory nomination) ============================

-- person_next_of_kin: subject → contact, both in-directory (D-PersonRelationships). A NOMINATION (with a
-- priority ordering), not a blood fact; external free-text contacts are out of scope (both ends must be
-- directory persons). relation_code is an optional category='next_of_kin' catalog label (enforced in the
-- application). Link link__next_of_kin. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_next_of_kin (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,6),  -- person / link / next_of_kin
  subject_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  contact_id    uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  priority      int  NOT NULL DEFAULT 1,
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','withdrawn')),
  -- native validity (D-Temporal, R-31): the interval this next-of-kin holds; NULL valid_to = active.
  valid_from    timestamptz NOT NULL DEFAULT now(),
  valid_to      timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_next_of_kin_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=6),
  CONSTRAINT person_next_of_kin_no_self CHECK (subject_id <> contact_id)
);

CREATE TRIGGER person_next_of_kin_set_updated_at
  BEFORE UPDATE ON oikumenea.person_next_of_kin
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_next_of_kin_active_pair_idx
  ON oikumenea.person_next_of_kin (subject_id, contact_id) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_next_of_kin_subject_idx
  ON oikumenea.person_next_of_kin (subject_id) WHERE deleted_at IS NULL;
CREATE INDEX person_next_of_kin_contact_idx
  ON oikumenea.person_next_of_kin (contact_id) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_next_of_kin.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.subject_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.contact_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.priority IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_next_of_kin.status IS 'pii:none';

-- ============================ associations (associate / COI / no-contact) ============================

-- person_associations: symmetric associate ↔ associate (canonical pair person_id_a < person_id_b;
-- D-PersonRelationships). kind partitions plain association / conflict-of-interest / prohibited-contact
-- (discipline). relation_code is an optional category='association' catalog label (enforced in the
-- application). Link link__associated_with. Erased when either endpoint purges.
CREATE TABLE oikumenea.person_associations (
  id            uuid PRIMARY KEY DEFAULT oikumenea.new_id(6,2,7),  -- person / link / associated_with
  person_id_a   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  person_id_b   uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
  relation_code text REFERENCES oikumenea.person_relation_types(code) ON DELETE RESTRICT,
  kind          text NOT NULL CHECK (kind IN ('associate','coi','no_contact')),
  status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','ended')),
  -- native validity (D-Temporal, R-31): the interval this association holds; NULL valid_to = active.
  valid_from    timestamptz NOT NULL DEFAULT now(),
  valid_to      timestamptz CHECK (valid_to IS NULL OR valid_to >= valid_from),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_associations_rid_shape
    CHECK (oikumenea.rid_service(id)=6 AND oikumenea.rid_kind(id)=2 AND oikumenea.rid_type(id)=7),
  CONSTRAINT person_associations_canonical_pair CHECK (person_id_a < person_id_b)
);

CREATE TRIGGER person_associations_set_updated_at
  BEFORE UPDATE ON oikumenea.person_associations
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE UNIQUE INDEX person_associations_active_pair_idx
  ON oikumenea.person_associations (person_id_a, person_id_b, kind) WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX person_associations_person_a_idx
  ON oikumenea.person_associations (person_id_a) WHERE deleted_at IS NULL;
CREATE INDEX person_associations_person_b_idx
  ON oikumenea.person_associations (person_id_b) WHERE deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.person_associations.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.person_id_a IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.person_id_b IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.relation_code IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.kind IS 'pii:none';
COMMENT ON COLUMN oikumenea.person_associations.status IS 'pii:none';

-- (person_social_links / link__social_tie deferred — see the header note and decisions.md.)

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0015_tenant_units_public_read_rls =====
-- Migration 0015_tenant_units_public_read_rls: the shadow-visibility gate, unit scope (F-002, A-lite).
--
-- The shadow-visibility differentiator (L-Visibility, patterns.md) was inert: every authenticated
-- request pins a connection whose app.readable_units GUC holds the subject's grant-based reach, and the
-- migration-0012 tenant_units_reach policy admits a row only when it is in reach (or the subject is an
-- instance admin). A `public` unit outside reach was therefore dropped exactly like a `shadow` one, so
-- public and shadow behaved identically and the documented "public units are discoverable" rule was not
-- enforced anywhere.
--
-- This migration makes `visibility` mean something for the unit graph: a `public` unit is SELECT-able
-- regardless of reach (broadly discoverable), while a `shadow` unit still requires reach. It is wired
-- as a SECOND permissive FOR SELECT policy: PostgreSQL OR-combines permissive policies per command, so
-- a SELECT on tenant_units now passes if the row is in reach OR the subject is an instance admin
-- (tenant_units_reach) OR the row is public (this policy). Writes are untouched — tenant_units_reach
-- (FOR ALL) still governs INSERT/UPDATE/DELETE through its WITH CHECK / USING on app.writable_units, and
-- a FOR SELECT policy grants no write. The app-layer shadow gate (authorization.FilterVisibleUnits,
-- wired into the tenant list/ancestors/descendants reads) remains the AUTHORITATIVE pass; this policy is
-- its DB-level mirror (D-RLSDefenseInDepth).
--
-- A-lite boundary (decisions.md, L-Visibility note): broad public discovery is a UNIT-read affordance
-- only. person/document/membership/order reads stay reach-gated — a public unit is discoverable in unit
-- listings, but its roster/detail still needs reach. Expand-only; no data change; no destructive op.

CREATE POLICY tenant_units_public_read ON oikumenea.tenant_units
  FOR SELECT
  USING (visibility = 'public');

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0016_person_date_of_death =====
-- Migration 0016_person_date_of_death: the M12 date-of-death bio field (D-PersonBio amendment).
--
-- D-PersonBio's M12 amendment mandates a nullable `date_of_death DATE` on person_persons, alongside
-- `birthdate`/`sex`/`country_of_birth`. Death is a BIO ATTRIBUTE, not a lifecycle state: it does NOT
-- transition `status` to `deactivated`/`purged` (a deceased person stays an active directory record;
-- status is orthogonal). It is a full-precision calendar DATE (not a TIMESTAMPTZ instant) — like
-- `birthdate`, partial/approximate dates ride the parked DS-38 seam. The column is `pii:basic` and is
-- on the person purge erasure list (NULLed by PurgePerson with the other bio fields).
--
-- Expand-only: a single additive nullable column. No data change, no destructive op, no backfill.

ALTER TABLE oikumenea.person_persons ADD COLUMN date_of_death date;  -- nullable bio date (a DATE, not an instant)
COMMENT ON COLUMN oikumenea.person_persons.date_of_death IS 'pii:basic';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0017_import_provenance =====
-- Migration 0017_import_provenance: per-row import lineage on reference catalogs (M16 / D-Hermenea).
--
-- The hermenea companion loads reference data through oikumenea's generic POST /import/{objectType}
-- endpoint (docs/modules/hermenea.md, docs/modules/platform.md). Each imported row carries WHERE it
-- came from: `source` (the dataset id, e.g. iso-3166), `source_version` (its edition), and
-- `imported_at` (when the upsert ran). These are stamped from the canonical envelope on every upsert
-- and are the per-row half of the D-DataIngestion lineage (the run-level ledger lives in hermenea's
-- own DB).
--
-- M16's first importable catalog is `geo_countries` (ISO-3166). The columns are nullable: rows seeded
-- by the bootstrap migration (0000) predate any import and simply have NULL provenance until a sync
-- refreshes them. Future importable catalogs add the same three columns in their own migrations.
--
-- Expand-only: three additive nullable columns, no data change, no destructive op, no backfill.

ALTER TABLE oikumenea.geo_countries ADD COLUMN source         text;        -- importing dataset id (e.g. iso-3166)
ALTER TABLE oikumenea.geo_countries ADD COLUMN source_version text;        -- the source edition/version
ALTER TABLE oikumenea.geo_countries ADD COLUMN imported_at    timestamptz; -- when the import upsert last touched this row
COMMENT ON COLUMN oikumenea.geo_countries.source IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.source_version IS 'pii:none';
COMMENT ON COLUMN oikumenea.geo_countries.imported_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0006_person_ext', applied_at = now() WHERE singleton;
