-- 0008 identity-federation (M8) — the external-IdP seam: optional login accounts + the verified
-- external identities they federate (docs/modules/identity-federation.md).
--
-- go-oikumenea does NOT authenticate (L-AuthzOnly): it stores no credentials and issues no tokens.
-- It owns the optional login `account` (an attachment to a person) and the `(issuer, subject)`
-- external identities linked to it; inbound-token validation (OIDC discovery + JWKS) is middleware
-- (code, not a table) that maps a verified token -> external identity -> account -> person -> PDP
-- subject. The first instance admin is seeded out-of-band from install config (D-Bootstrap), creating
-- the first account + external_identity in one transaction.
--
-- Two tables, both expand-only (L-UpgradeSafe / D-Migrations):
--   * account_accounts             — Object Account: the optional login attachment (<=1 active per
--                                    person, HAS_ACCOUNT). Carries status + a DORMANT seam of
--                                    credential columns (CHECK-NULL) for a future full-IdP pivot.
--   * account_external_identities  — Object External identity: a verified (issuer, subject) pair
--                                    federating to an account (FEDERATES). Globally unique;
--                                    immutable once created (no UPDATE), removed by unlink (DELETE).
--
-- This migration is PURE DDL: it seeds NO rows. The first-admin account/identity is seeded at BOOT
-- (or by the recover-admin CLI) by the app on the GUC-bearing pool (D-Bootstrap / D-RIDSeeding) — not
-- here, where atlas's connection has no app.environment GUC for new_rid(). Depends on 0001 bootstrap
-- (new_rid/set_updated_at/reject_mutation, citext) and 0006 person (person_persons).

-- account_accounts: an optional login attachment to exactly one person (Object Account). A person may
-- have zero accounts (roster-only, L-AccountOptional) or one active account. Tokens/passwords are
-- NEVER stored while auth is delegated; the dormant credential columns are CHECK-enforced NULL until a
-- future "become a full IdP" pivot ships (additive, not a rewrite — patterns.md Dormant seam).
CREATE TABLE oikumenea.account_accounts (
  id              text PRIMARY KEY DEFAULT oikumenea.new_rid('account','account'),
  person_id       text NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  email           citext,                         -- optional, as asserted by the IdP; unique among active when set
  status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  -- Dormant seam (always NULL while auth is delegated — L-AuthzOnly). password_hash is `secret`
  -- (a separate axis from the pii: tiers — D-PIITiers), not a credential we keep today.
  password_hash   text,
  mfa_enrolled_at timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,

  CONSTRAINT account_accounts_rid_shape CHECK (id LIKE 'urn:oikumenea:account:%:account:%'),
  -- Dormant credential columns stay NULL until the full-IdP pivot ships (then this CHECK is dropped).
  CONSTRAINT account_accounts_dormant_credentials CHECK (password_hash IS NULL AND mfa_enrolled_at IS NULL)
);

CREATE TRIGGER account_accounts_set_updated_at
  BEFORE UPDATE ON oikumenea.account_accounts
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- At most one active account per person (HAS_ACCOUNT, <=1 active).
CREATE UNIQUE INDEX account_accounts_person_active_idx
  ON oikumenea.account_accounts (person_id) WHERE deleted_at IS NULL;
-- The IdP-asserted email is unique among active accounts when present.
CREATE UNIQUE INDEX account_accounts_email_active_idx
  ON oikumenea.account_accounts (email) WHERE email IS NOT NULL AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.account_accounts.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_accounts.person_id IS 'pii:basic';
COMMENT ON COLUMN oikumenea.account_accounts.email IS 'pii:contact';
COMMENT ON COLUMN oikumenea.account_accounts.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_accounts.password_hash IS 'secret';
COMMENT ON COLUMN oikumenea.account_accounts.mfa_enrolled_at IS 'pii:none';

-- account_external_identities: a verified (issuer, subject) login point federating to one account
-- (Object External identity / FEDERATES). The schema permits MANY rows per account_id — one per login
-- point (e.g. a Google identity AND a Keycloak identity for the same person) — and the cap on
-- ADDITIONAL links is enforced in the application at link time (account.identity_linking.enabled),
-- NOT by a DB constraint, so flipping that config is reversible without a migration. No token columns:
-- access/refresh tokens are never persisted. The row is immutable once created (an UPDATE guard); an
-- unlink is a hard DELETE (there is no deleted_at — the FEDERATES link either exists or is removed).
CREATE TABLE oikumenea.account_external_identities (
  id         text PRIMARY KEY DEFAULT oikumenea.new_rid('account','external_identity'),
  account_id text NOT NULL REFERENCES oikumenea.account_accounts(id) ON DELETE CASCADE,
  issuer     text NOT NULL,                       -- the IdP `iss`
  subject    text NOT NULL,                       -- the IdP `sub` (pseudonymous identifier)
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT account_external_identities_rid_shape CHECK (id LIKE 'urn:oikumenea:account:%:external_identity:%')
);

-- Immutable once created: block UPDATE (an identity's (issuer, subject, account) never changes) while
-- still permitting the unlink hard-DELETE. reject_mutation() raises on any firing, so attach it to the
-- UPDATE event only (cf. audit_log, which guards UPDATE OR DELETE for a true append-only ledger).
CREATE TRIGGER account_external_identities_no_update
  BEFORE UPDATE ON oikumenea.account_external_identities
  FOR EACH ROW EXECUTE FUNCTION oikumenea.reject_mutation();

-- A given external identity maps to exactly one account, globally.
CREATE UNIQUE INDEX account_external_identities_issuer_subject_idx
  ON oikumenea.account_external_identities (issuer, subject);
CREATE INDEX account_external_identities_account_idx
  ON oikumenea.account_external_identities (account_id);

COMMENT ON COLUMN oikumenea.account_external_identities.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.account_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.issuer IS 'pii:none';
COMMENT ON COLUMN oikumenea.account_external_identities.subject IS 'pii:basic';
COMMENT ON COLUMN oikumenea.account_external_identities.created_at IS 'pii:none';

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0008_identity_federation', applied_at = now() WHERE singleton;
