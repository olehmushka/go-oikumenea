-- 0013_locale_eng — self-contained eng locale: registers the locale, links its languoid, and seeds
-- all eng translation rows. Adding a new locale = drop in one more 00NN_locale_<code>.sql after this.
-- (Extracted from the former 0002/0009/0018/0023-0026/0045 migrations by the migration refactor.)

-- locale registration
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order) VALUES
  ('eng', 'English',    true, false, 10);

-- link this locale to its Glottolog languoid (was in 0018)
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT 'eng', x.id FROM oikumenea.language_languoids x WHERE x.iso639_3 = 'eng' ON CONFLICT DO NOTHING;

-- eng translations
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'personal_code_scheme', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('ua-rnokpp',         'eng', 'Individual Tax Number (RNOKPP)'),
  ('ua-unzr',           'eng', 'Unique Record Number (UNZR)'),
  ('us-ssn',            'eng', 'Social Security Number'),
  ('de-steuer-id',      'eng', 'Tax ID (Steuer-ID)'),
  ('it-codice-fiscale', 'eng', 'Tax Code (Codice Fiscale)'),
  ('pl-pesel',          'eng', 'PESEL')
) AS v(code, locale, text)
JOIN oikumenea.document_personal_code_schemes s ON s.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'writing_system', w.id::text, 'name', 'eng', w.name
FROM oikumenea.writing_systems w
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon_rank', r.id::text, 'name', 'eng', r.name
FROM oikumenea.religion_taxon_ranks r
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_classification', c.id::text, 'name', 'eng', c.name
FROM oikumenea.religion_classifications c
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_org_kind', k.id::text, 'name', 'eng', k.name
FROM oikumenea.religion_org_kinds k
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_policy_kind', p.id::text, 'name', 'eng', p.name
FROM oikumenea.religion_policy_kinds p
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon', t.id::text, 'name', 'eng', t.name
FROM oikumenea.religion_taxa t
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', 'eng', c.name
FROM oikumenea.religion_grade_categories c
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', 'eng', g.name
FROM oikumenea.religion_clergy_grades g
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_affiliation_type', a.id::text, 'name', 'eng', a.name
FROM oikumenea.religion_affiliation_types a
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', 'eng', s.name
FROM oikumenea.religion_site_types s
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', 'eng', s.name
FROM oikumenea.religion_service_types s
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'domain', id::text, 'name', 'eng', name FROM oikumenea.tenant_domains WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'unit_kind', id::text, 'name', 'eng', name FROM oikumenea.tenant_unit_kinds WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'relation_type', code, 'name', 'eng', name FROM oikumenea.person_relation_types
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'email_type', code, 'name', 'eng', name FROM oikumenea.person_email_types
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'phone_type', code, 'name', 'eng', name FROM oikumenea.person_phone_types
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'document_type', id::text, 'name', 'eng', name FROM oikumenea.document_document_types WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'location_type', id::text, 'name', 'eng', name FROM oikumenea.location_location_types WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_institution_kind', id::text, 'name', 'eng', name FROM oikumenea.education_institution_kinds WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_degree_level', id::text, 'name', 'eng', name FROM oikumenea.education_degree_levels WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_legal_form', id::text, 'name', 'eng', name FROM oikumenea.company_legal_forms WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_registration_scheme', id::text, 'name', 'eng', name FROM oikumenea.company_registration_schemes WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_industry_class', id::text, 'name', 'eng', name FROM oikumenea.company_industry_classes WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'external_org_kind', id::text, 'name', 'eng', name FROM oikumenea.external_org_kinds WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'finance_account_type', id::text, 'name', 'eng', name FROM oikumenea.finance_account_types WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_type', id::text, 'name', 'eng', name FROM oikumenea.vehicle_types WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_registration_number_type', id::text, 'name', 'eng', name FROM oikumenea.vehicle_registration_number_types WHERE deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

UPDATE oikumenea.schema_version SET revision = '0013_locale_eng', applied_at = now() WHERE singleton;
