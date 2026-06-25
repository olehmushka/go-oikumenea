-- 0028 overlay foundation (M29).
--
-- The OSINT-enrichment substrate (docs/architecture/roadmap-decisions.md, D-OverlayFoundation): the
-- cross-cutting machinery every later overlay milestone (M30-M37) rides as a thin slice. M29 lands three
-- things; only one needs a new table here:
--
--   1. Provisional persons — person_persons.status gains 'provisional' (a minimal-PII stub so every
--      relationship/overlay edge points at a real node). Added by editing 0005 in place (the status CHECK);
--      no DDL here. Resolution is the MergePerson audited action (re-homes edges, tombstones the stub).
--   2. Attribution convention — the source/confidence/as_of column-set (already used by
--      D-PersonSocialChannels), formalized in conventions.md for verbatim reuse by M30-M37. Documentation
--      only; no DDL.
--   3. legal_basis (structured) — THIS migration: a seeded platform_legal_basis_kinds catalog, FK-referenced
--      by every future pii:special overlay store (the FK consumers arrive in M31+).
--
-- platform_legal_basis_kinds is a platform-owned reference catalog with a natural `code` PK (D-Code
-- carve-out, exactly the person_platforms shape): GDPR Article 6 lawful bases and the Article 9
-- special-category processing conditions, partitioned by `article`. Instance-extensible via the API.
--
-- Expand-only (L-UpgradeSafe / D-Migrations); depends on the 0001 schema bootstrap (set_updated_at).
-- Seeded here (natural-key carve-out — new_id() reads no GUC, D-RIDSeeding).

-- platform_legal_basis_kinds: the structured lawful-basis catalog (D-OverlayFoundation). `article`
-- partitions the Art. 6 lawful bases from the Art. 9 special-category conditions; `name` is the
-- default-locale label (other locales in the localization store). A gated/special-category overlay row
-- references a code here (NOT NULL on pii:special stores) + an optional free-text justification note.
CREATE TABLE oikumenea.platform_legal_basis_kinds (
  code       text PRIMARY KEY,
  name       text NOT NULL,
  article    text NOT NULL CHECK (article IN ('art6','art9')),  -- art6 = lawful basis; art9 = special-category condition
  status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired')),
  sort_order int,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER platform_legal_basis_kinds_set_updated_at
  BEFORE UPDATE ON oikumenea.platform_legal_basis_kinds
  FOR EACH ROW EXECUTE FUNCTION oikumenea.set_updated_at();

COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.code IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.name IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.article IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.status IS 'pii:none';
COMMENT ON COLUMN oikumenea.platform_legal_basis_kinds.sort_order IS 'pii:none';

-- Seed the lawful-basis catalog (natural-key carve-out). The instance admin adds more via the API.
INSERT INTO oikumenea.platform_legal_basis_kinds (code, name, article, sort_order) VALUES
  -- GDPR Article 6 — lawful bases for processing
  ('consent',                    'Consent',                                'art6', 10),
  ('contract',                   'Performance of a contract',              'art6', 20),
  ('legal_obligation',           'Legal obligation',                       'art6', 30),
  ('vital_interests',            'Vital interests',                        'art6', 40),
  ('public_task',                'Public task',                            'art6', 50),
  ('legitimate_interest',        'Legitimate interests',                   'art6', 60),
  -- GDPR Article 9(2) — conditions for processing special categories of data
  ('explicit_consent',           'Explicit consent',                       'art9', 110),
  ('employment_law',             'Employment, social security & social protection law', 'art9', 120),
  ('vital_interests_art9',       'Vital interests (data subject incapable of consent)', 'art9', 130),
  ('not_for_profit_body',        'Not-for-profit body processing',         'art9', 140),
  ('made_public_by_subject',     'Data manifestly made public by the subject', 'art9', 150),
  ('legal_claims',               'Establishment, exercise or defence of legal claims', 'art9', 160),
  ('substantial_public_interest','Substantial public interest',            'art9', 170),
  ('health_care',                'Preventive or occupational medicine / health care', 'art9', 180),
  ('public_health',              'Public health',                          'art9', 190),
  ('archiving_research',         'Archiving, scientific/historical research or statistics', 'art9', 200);

-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
UPDATE oikumenea.schema_version SET revision = '0028_overlay_foundation', applied_at = now() WHERE singleton;
