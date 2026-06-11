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
  ('telegram',  'Telegram',  'messenger',  0),
  ('whatsapp',  'WhatsApp',  'messenger', 10),
  ('signal',    'Signal',    'messenger', 20),
  ('viber',     'Viber',     'messenger', 30),
  ('instagram', 'Instagram', 'social',    40),
  ('linkedin',  'LinkedIn',  'social',    50),
  ('x',         'X',         'social',    60),
  ('facebook',  'Facebook',  'social',    70);

-- ============================ messenger links (layer a) ============================

-- person_messenger_links: annotates an existing phone OR email with reachability on a messenger
-- platform (D-PersonSocialChannels). Exactly one of phone_id/email_id is non-null (XOR CHECK); both
-- CASCADE when the underlying channel is hard-deleted. platform_code is write-time restricted to a
-- category='messenger' platform (enforced in the application + domain; the FK only checks existence).
-- One active link per (phone_id, platform_code) / (email_id, platform_code). Erased on person purge.
CREATE TABLE oikumenea.person_messenger_links (
  id            text PRIMARY KEY DEFAULT oikumenea.new_rid('person','messenger_link'),
  phone_id      text REFERENCES oikumenea.person_phones(id) ON DELETE CASCADE,
  email_id      text REFERENCES oikumenea.person_emails(id) ON DELETE CASCADE,
  platform_code text NOT NULL REFERENCES oikumenea.person_platforms(code) ON DELETE RESTRICT,
  is_primary    boolean NOT NULL DEFAULT false,
  verified_at   timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,

  CONSTRAINT person_messenger_links_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:messenger_link:%'),
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
  id                      text PRIMARY KEY DEFAULT oikumenea.new_rid('person','social_account'),
  person_id               text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE CASCADE,
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

  CONSTRAINT person_social_accounts_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:social_account:%')
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
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('person','social_handle'),
  account_id text NOT NULL REFERENCES oikumenea.person_social_accounts(id) ON DELETE CASCADE,
  handle     text NOT NULL,
  valid_from timestamptz NOT NULL,
  valid_to   timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT person_social_account_handles_rid_shape CHECK (id LIKE 'urn:oikumenea:person:%:social_handle:%')
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
UPDATE oikumenea.schema_version SET revision = '0013_person_social_channels', applied_at = now() WHERE singleton;
