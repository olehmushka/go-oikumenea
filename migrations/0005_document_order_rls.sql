-- 0005_document_order_rls — merged domain migration (refactor: consolidated from 0009_document, 0010_order, 0011_rls_backstop).

-- ===== merged from 0009_document =====
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
  issuing_country_id uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- nullable (D-Geo); ISO code resolved in SQL
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
COMMENT ON COLUMN oikumenea.document_documents.issuing_country_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.issued_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.expires_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.document_documents.attributes IS 'pii:special';
COMMENT ON COLUMN oikumenea.document_documents.status IS 'pii:none';

-- document_personal_code_schemes: the instance-admin catalog of country-namespaced national-identifier
-- schemes (D-PersonalCodes). Natural `code` PK (the D-ResourceIdentifiers carve-out — a seeded shared
-- reference registry FK'd by code, like i18n_locales). generic_category is the
-- cross-scheme join key ("list everyone's tax IDs"); validation_regex is the data-side FALLBACK behind
-- a compiled pkg/personalcode validator.
CREATE TABLE oikumenea.document_personal_code_schemes (
  code             text PRIMARY KEY,              -- the scheme id, e.g. ua-rnokpp, us-ssn (D-Code)
  country_id       uuid REFERENCES oikumenea.geo_countries(id) ON DELETE RESTRICT,  -- national scheme's country (ISO code resolved in SQL)
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
COMMENT ON COLUMN oikumenea.document_personal_code_schemes.country_id IS 'pii:none';
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
INSERT INTO oikumenea.document_personal_code_schemes (code, country_id, generic_category, name, validation_regex, sort_order)
SELECT v.code, c.id, v.generic_category, v.name, v.validation_regex, v.sort_order
FROM (VALUES
  ('ua-rnokpp',         'UA', 'tax-id',          'РНОКПП',         NULL::text,            0),
  ('ua-unzr',           'UA', 'national-id',     'УНЗР',           '^\d{8}-\d{5}$',      10),
  ('us-ssn',            'US', 'social-insurance','Social Security Number', NULL,          20),
  ('de-steuer-id',      'DE', 'tax-id',          'Steuer-ID',      '^\d{11}$',           30),
  ('it-codice-fiscale', 'IT', 'tax-id',          'Codice Fiscale', '^[A-Za-z0-9]{16}$',  40),
  ('pl-pesel',          'PL', 'national-id',     'PESEL',          NULL,                  50)
) AS v(code, country_iso, generic_category, name, validation_regex, sort_order)
JOIN oikumenea.geo_countries c ON c.code = v.country_iso;

-- Localized names for the personal-code scheme catalog (D-i18n: all locales in every response). The
-- catalog `name` column carries the native label; we seed explicit eng + ukr translations so both UI
-- locales resolve correctly. entity_id is the scheme `code` (the transport assembles scheme names
-- keyed by code under entity_type 'personal_code_scheme'). Idempotent: re-running is a no-op.


-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0010_order =====
-- 0010 order (M10).
--
-- Administrative orders (наказ): the formal acts that are the LEGAL BASIS for a change in a person's
-- status (docs/modules/order.md, D-Orders). Where a document records what a person HAS, an order
-- records an ACT the organization performs (arrival, appointment, leave, transfer, discipline, duty).
-- Three tables:
--   * order_order_types  — the instance-admin catalog of order kinds (RID Object; stable `code` +
--                          translatable `name`), each carrying a `category` (the five UA-army families)
--                          and an `effect` (membership-start | membership-end | rank-change |
--                          record-only) that drives which target columns an item must carry and which
--                          intent event issue emits.
--   * order_orders       — the order header (наказ): number, date, issuing unit, draft→issued→revoked
--                          lifecycle (mutable while draft; LOCKED on issue — corrections are amending
--                          orders, undo is a revoking order; reversibility, not the append-only guard).
--   * order_order_items  — one affected person/act within an order; PARENT-SCOPED (no deleted_at, no
--                          independent lifecycle — reads resolve items only through a non-deleted parent).
--
-- An order takes effect on other modules ONLY via domain events + provenance (the locked
-- cross-module-mutation rule, D-OrderApply): on issue, each structural item emits an intent event a
-- membership/person subscriber applies IN THE ISSUE TRANSACTION, citing membership_memberships.order_item_id.
-- That forward-referenced provenance column (added nullable, no FK, in 0006 membership) gets its FK here.
--
-- An order carries NO authority (directory/record data, like rank/position); access is decided by the
-- PDP, unit-scoped on issuing_unit_id + the shadow gate. Expand-only (L-UpgradeSafe); depends on 0001
-- bootstrap (new_rid, set_updated_at), 0003 localization (translatable type names), 0003 tenant (issuing
-- unit), 0005 rank (target rank), 0006 person + membership (target person/position; the provenance FK).
--
-- RID-keyed tables seed NO rows here — that needs the app.environment GUC atlas does not set
-- (D-RIDSeeding); the order-type catalog is seeded at boot in order.Register.

-- order_order_types: the instance-admin catalog of order kinds (D-Orders). RID Object with a stable,
-- locale-agnostic `code` (immutable by convention) + a translatable default-locale `name`. `category`
-- is the five UA-army "стройова частина" families; `effect` is the downstream consequence of items of
-- this type (it determines the required target columns — enforced in the application — and the intent
-- event issue emits).
CREATE TABLE oikumenea.order_order_types (
  id         uuid PRIMARY KEY DEFAULT oikumenea.new_id(11,1,1),  -- order / object / order_type
  code       text NOT NULL UNIQUE,               -- stable locale-agnostic identifier (D-Code)
  name       text NOT NULL,                      -- default-locale label; translatable via the i18n store
  category   text NOT NULL CHECK (category IN
               ('personnel-list','appointment','leave-travel','discipline-incentive','duty-roster')),
  effect     text NOT NULL CHECK (effect IN
               ('membership-start','membership-end','rank-change','record-only')),
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,

  CONSTRAINT order_order_types_rid_shape
    CHECK (oikumenea.rid_service(id)=11 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=1)
);

CREATE TRIGGER order_order_types_set_updated_at
  BEFORE UPDATE ON oikumenea.order_order_types
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.order_order_types.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.category IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.effect IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_types.sort_order IS 'pii:none';

-- order_orders: the order header (наказ). issuing_unit_id is NOT NULL — every order is unit-issued (no
-- instance-level orders), which anchors the unit-scope authz check and the RLS predicate (both key on
-- issuing_unit_id; D-Orders, I-5). status is the draft→issued→revoked lifecycle (the only post-issue
-- transition is issued→revoked, recording revoked_by_order_id + revoked_at).
CREATE TABLE oikumenea.order_orders (
  id                  uuid PRIMARY KEY DEFAULT oikumenea.new_id(11,1,2),  -- order / object / order
  number              text,                       -- order number (unique within issuing unit; nullable)
  issued_on           date,                       -- the order's date
  issuing_unit_id     uuid NOT NULL REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,
  status              text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','issued','revoked')),
  revoked_by_order_id uuid REFERENCES oikumenea.order_orders(id),  -- the later order that revoked this one
  revoked_at          timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,

  CONSTRAINT order_orders_rid_shape
    CHECK (oikumenea.rid_service(id)=11 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=2)
);

CREATE TRIGGER order_orders_set_updated_at
  BEFORE UPDATE ON oikumenea.order_orders
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

-- An order number is unique within its issuing unit (among non-deleted rows; D-Orders invariant).
CREATE UNIQUE INDEX order_orders_unit_number_idx
  ON oikumenea.order_orders (issuing_unit_id, number)
  WHERE number IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX order_orders_issued_idx
  ON oikumenea.order_orders (issuing_unit_id) WHERE status = 'issued' AND deleted_at IS NULL;

COMMENT ON COLUMN oikumenea.order_orders.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.number IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.issued_on IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.issuing_unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_orders.status IS 'pii:none';

-- order_order_items: one affected person/act within an order (the unit of effect). PARENT-SCOPED — no
-- deleted_at and no lifecycle of its own (added/removed only while the parent is draft; locked on
-- issue; visible only through a non-deleted parent). ON DELETE CASCADE off the parent is FK-integrity
-- insurance for the (design-forbidden) hard delete of a parent, not a routine path. Which target
-- columns are required is checked in the application against the type's `effect`. `note` is the only
-- pii:basic field; person/unit/position/rank are pii:none id references.
CREATE TABLE oikumenea.order_order_items (
  id             uuid PRIMARY KEY DEFAULT oikumenea.new_id(11,1,3),  -- order / object / order_item
  order_id       uuid NOT NULL REFERENCES oikumenea.order_orders(id) ON DELETE CASCADE,
  type_id        uuid NOT NULL REFERENCES oikumenea.order_order_types(id) ON DELETE RESTRICT,
  person_id      uuid NOT NULL REFERENCES oikumenea.person_persons(id) ON DELETE RESTRICT,
  unit_id        uuid REFERENCES oikumenea.tenant_units(id) ON DELETE RESTRICT,        -- target unit (nullable)
  position_id    uuid REFERENCES oikumenea.membership_positions(id) ON DELETE RESTRICT, -- target billet (nullable)
  rank_id        uuid REFERENCES oikumenea.rank_ranks(id) ON DELETE RESTRICT,           -- target rank (nullable)
  effective_from date,
  effective_to   date,
  note           text,                            -- free-text detail (reason, reference); minimized
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT order_order_items_rid_shape
    CHECK (oikumenea.rid_service(id)=11 AND oikumenea.rid_kind(id)=1 AND oikumenea.rid_type(id)=3)
);

CREATE TRIGGER order_order_items_set_updated_at
  BEFORE UPDATE ON oikumenea.order_order_items
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

CREATE INDEX order_order_items_order_idx  ON oikumenea.order_order_items (order_id);
CREATE INDEX order_order_items_person_idx ON oikumenea.order_order_items (person_id);

COMMENT ON COLUMN oikumenea.order_order_items.id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.order_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.type_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.person_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.unit_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.position_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.rank_id IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.effective_from IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.effective_to IS 'pii:none';
COMMENT ON COLUMN oikumenea.order_order_items.note IS 'pii:basic';

-- Resolve the forward-referenced provenance pointer from 0006 membership: a fill/end may cite the
-- order item it realizes (D-OrderApply). Added nullable + no FK there (order_order_items did not yet
-- exist); the FK lands now as ON DELETE SET NULL so hard-deleting an order's items (FK insurance only)
-- nulls the provenance rather than blocking — the membership row is the authoritative fact.
ALTER TABLE oikumenea.membership_memberships
  ADD CONSTRAINT membership_memberships_order_item_id_fkey
  FOREIGN KEY (order_item_id) REFERENCES oikumenea.order_order_items(id) ON DELETE SET NULL;

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

-- ===== merged from 0011_rls_backstop =====
-- Migration 0011_rls_backstop: the live-reach Row-Level-Security backstop (D-RLSDefenseInDepth,
-- reshaped by D-RLSLiveReach — review-2026-07 R-02.2).
--
-- RLS is enabled here as a DB-level defense-in-depth backstop. The app-layer PDP + shadow gate
-- remain AUTHORITATIVE; RLS only guards the forgotten-filter bug class (a SELECT/INSERT that skips
-- the PDP/gate). Policies compute reach LIVE via oikumenea.authz_unit_in_reach(unit, wr) — a
-- semi-join over the subject's role assignments + the tenant closure — keyed on two O(1) GUCs the
-- application sets per request (internal/platform/db): app.person_id + app.is_instance_admin. The
-- former comma-joined app.readable_units / app.writable_units unit-list GUCs are GONE (they scaled
-- with org size per request — multi-MB for a staff-level subject). Because the policies read the
-- authority tables directly, the backstop is EXACT under revocation (stronger than the old
-- snapshot-at-request-start GUCs). An instance admin is expressed via the app.is_instance_admin
-- GUC flag, NEVER a DB superuser — the app role created here lacks BYPASSRLS.
--
-- upgrade-safety.md stages RLS as permissive-then-tighten for a LIVE, already-released deployment (so a
-- policy tightening cannot outrun the GUC plumbing). go-oikumenea has never been released, so this
-- migration ships the GUC wiring (in the same release) and the tightened policies ATOMICALLY: on a
-- fresh install there is no window in which the policy outruns the plumbing. The staged rollout
-- re-applies for any post-v1 RLS change. Expand-only (CREATE ROLE / GRANT / ENABLE RLS / CREATE
-- POLICY only; no drops/narrowings). Depends on 0001–0011.

-- ---------------------------------------------------------------------------------------------------
-- The non-superuser application role. Migrations run as the owner/superuser (which bypasses RLS); the
-- application connects as a login role that is a MEMBER of this group role (see UPGRADING.md / .env),
-- so the policies below apply to it. NOLOGIN + NOBYPASSRLS: a group role, never a login/superuser.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'oikumenea_app') THEN
    CREATE ROLE oikumenea_app NOLOGIN NOBYPASSRLS;
  END IF;
END$$;

GRANT USAGE ON SCHEMA oikumenea TO oikumenea_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA oikumenea TO oikumenea_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA oikumenea TO oikumenea_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA oikumenea TO oikumenea_app;
-- Future objects (forward-compatible; this is the last v1 migration but keeps the grant correct if a
-- later migration adds a table without re-granting).
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO oikumenea_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT USAGE, SELECT ON SEQUENCES TO oikumenea_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA oikumenea GRANT EXECUTE ON FUNCTIONS TO oikumenea_app;

-- ---------------------------------------------------------------------------------------------------
-- authz_unit_in_reach: the live reach predicate every policy calls (D-RLSLiveReach). PARITY CONTRACT
-- with authorization/domain ReachSet (pdp.go): an assignment contributes iff active (revoked_at IS
-- NULL, unexpired), its role live, and the role carries a '*.read' permission (wr=false) / any
-- non-read permission (wr=true — the exact mirror of the Go classify()); 'unit' scope reaches its
-- target only; 'subtree' reaches target + closure descendants ONLY over an authority-bearing,
-- non-deleted graph (a directory-only subtree grant confers nothing, not even its target).
--
-- LANGUAGE sql + STABLE + a single SELECT ⇒ the planner inlines it into each policy as a
-- parameterized sub-plan: an authz_role_assignments_subject_idx probe (k = the subject's
-- assignments, typically ≤ tens) + one closure-PK probe per subtree assignment — exact index hits
-- per row scanned, and every RLS-guarded read path is keyset-paged, so rows scanned ≈ page.
--
-- The function reads ONLY RLS-exempt tables (authz_*, tenant_graphs, tenant_unit_closure) — a read
-- of a policy-guarded table here would recurse into its own policy. Consequence: it cannot apply
-- ReachSet's soft-deleted-DESCENDANT refinement (that needs tenant_units), so the backstop is
-- deliberately NEVER NARROWER than PDP reach — it may additionally pass rows keyed on a soft-deleted
-- descendant unit; the authoritative app layer excludes them. current_setting(name, true) returns
-- NULL when the GUC was never set; nullif('') maps the reset value to NULL, so a non-pinned
-- connection simply sees no rows.
CREATE FUNCTION oikumenea.authz_unit_in_reach(unit uuid, wr boolean) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.is_instance_admin', true), '') = 'true'
      OR EXISTS (
        SELECT 1
        FROM oikumenea.authz_role_assignments a
        JOIN oikumenea.authz_roles r ON r.id = a.role_id AND r.deleted_at IS NULL
        WHERE a.subject_person_id = nullif(current_setting('app.person_id', true), '')::uuid
          AND a.revoked_at IS NULL
          AND (a.expires_at IS NULL OR a.expires_at > now())
          AND EXISTS (SELECT 1 FROM oikumenea.authz_role_permissions rp
                      WHERE rp.role_id = a.role_id
                        AND (rp.permission_code LIKE '%.read') = NOT wr)
          AND ((a.scope = 'unit' AND a.target_unit_id = unit)
            OR (a.scope = 'subtree'
                AND EXISTS (SELECT 1 FROM oikumenea.tenant_graphs g
                            WHERE g.id = a.graph_id AND g.is_authority_bearing AND g.deleted_at IS NULL)
                AND (a.target_unit_id = unit
                  OR EXISTS (SELECT 1 FROM oikumenea.tenant_unit_closure c
                             WHERE c.graph_id = a.graph_id
                               AND c.ancestor_id = a.target_unit_id
                               AND c.descendant_id = unit))))
      )
$$;
GRANT EXECUTE ON FUNCTION oikumenea.authz_unit_in_reach(uuid, boolean) TO oikumenea_app;

-- Policy predicate shorthand (inlined per table; PostgreSQL has no policy macros):
--   read   := oikumenea.authz_unit_in_reach(<col>, false)   -- admin short-circuit is inside
--   write  := oikumenea.authz_unit_in_reach(<col>, true)
--
-- EXEMPT (no RLS): tenant_unit_closure + tenant_closure_status + tenant_graphs + the authz_* tables
-- incl. authz_epoch (the reach predicate READS these to COMPUTE reach — a reach-keyed policy there
-- would be circular; tenant_graphs is an instance-level catalog anyway).
-- person_persons / document_documents / order_order_items (and person child tables like person_ranks,
-- the HOLDS_RANK link) have no unit column and are scoped by the app-layer PDP through a unit-scoped
-- parent/holder (D-PersonReadScope / D-RLSDefenseInDepth); since R-02.1 the reach predicate exists in
-- SQL, so a membership-semi-join policy for them is one policy away — a noted hardening seam, still
-- not shipped (D-RLSLiveReach).

-- tenant_units: keyed on the unit's own id. REFERENCE units (pdp_scoped=false — university/company,
-- D-UnifiedOrgGraph M41) are instance-global: they are exempt from the reach predicate (reads are gated
-- by the public-read policy + the app-layer permission; writes by the instance-scoped `*.manage` perm).
ALTER TABLE oikumenea.tenant_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_units FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_units_reach ON oikumenea.tenant_units
  USING (NOT pdp_scoped OR oikumenea.authz_unit_in_reach(id, false))
  WITH CHECK (NOT pdp_scoped OR oikumenea.authz_unit_in_reach(id, true));

-- tenant_unit_edges: visible/writable if EITHER endpoint is in reach.
ALTER TABLE oikumenea.tenant_unit_edges ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_edges FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_edges_reach ON oikumenea.tenant_unit_edges
  USING (oikumenea.authz_unit_in_reach(parent_id, false) OR oikumenea.authz_unit_in_reach(child_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(parent_id, true) OR oikumenea.authz_unit_in_reach(child_id, true));

-- tenant_unit_lifecycle_events: append-only (reject_mutation guards U/D); keyed on unit_id.
ALTER TABLE oikumenea.tenant_unit_lifecycle_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.tenant_unit_lifecycle_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_unit_lifecycle_events_reach ON oikumenea.tenant_unit_lifecycle_events
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- membership_positions: keyed on the owning unit_id.
ALTER TABLE oikumenea.membership_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_positions_reach ON oikumenea.membership_positions
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- membership_memberships: keyed on unit_id (the unit the person belongs to / fills a billet in).
ALTER TABLE oikumenea.membership_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.membership_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY membership_memberships_reach ON oikumenea.membership_memberships
  USING (oikumenea.authz_unit_in_reach(unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(unit_id, true));

-- order_orders: keyed on issuing_unit_id (D-Orders — every order is unit-issued).
ALTER TABLE oikumenea.order_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.order_orders FORCE ROW LEVEL SECURITY;
CREATE POLICY order_orders_reach ON oikumenea.order_orders
  USING (oikumenea.authz_unit_in_reach(issuing_unit_id, false))
  WITH CHECK (oikumenea.authz_unit_in_reach(issuing_unit_id, true));

-- audit_log: a READ backstop only (the dangerous leak is reading another unit's audit history). Writes
-- are append-only (reject_mutation guards U/D) and originate from BOTH request transactions (the
-- pinned conn) AND system paths (first-admin bootstrap, boot seeds) that have no unit reach, so the
-- INSERT policy is permissive — the app, not RLS, governs what is written. NULL unit_id rows (system /
-- instance-plane events) are visible only to an instance admin.
ALTER TABLE oikumenea.audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE oikumenea.audit_log FORCE ROW LEVEL SECURITY;
-- NULL unit_id rows (system / instance-plane events) stay admin-only: authz_unit_in_reach(NULL, ·)
-- is the admin flag OR a never-matching EXISTS.
CREATE POLICY audit_log_read ON oikumenea.audit_log FOR SELECT
  USING (oikumenea.authz_unit_in_reach(unit_id, false));
CREATE POLICY audit_log_append ON oikumenea.audit_log FOR INSERT
  WITH CHECK (true);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).

UPDATE oikumenea.schema_version SET revision = '0005_document_order_rls', applied_at = now() WHERE singleton;
