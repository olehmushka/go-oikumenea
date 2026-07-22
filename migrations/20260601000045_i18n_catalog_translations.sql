-- 0045_i18n_catalog_translations — backfill spa/por (+ukr) translations for the operator-facing
-- reference catalogs that were seeded English-only (D-i18n: every translatable label is returned in
-- all enabled locales — ukr/eng/spa/por). Each module's read path already assembles a locale->text map
-- via localization.NamesByID/LabelsByID keyed by (entity_type, entity_id); the maps were carrying only
-- the English `name` column fallback because no non-English rows existed. This migration inserts them.
--
-- entity_id keying matches each transport's NamesByID call site: RID-keyed catalogs join the catalog
-- table to resolve the row RID (id::text); code-keyed catalogs (person contact/relation/scheme) use the
-- natural code. Idempotent — ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING — so it
-- never clobbers an operator-corrected translation and is safe to re-run. `id` uses its default
-- new_id(2,1,1). Reference data already multilingual via the pinax `translations` preset (countries,
-- languages, scripts, colors, religion taxa, ranks) and the religion migrations is NOT re-seeded here.
-- Brand-name catalogs (person_platforms, finance_card_networks) are intentionally omitted — a brand is
-- identical across locales, so the English fallback is already correct.

-- ------------------------------------------------------------------------------------------------
-- eng backfill (REQUIRED first): localization.LabelsByID seeds each entity's map with the DEFAULT
-- locale (ukr) -> the caller's English column value, then overlays translation rows. Adding a `ukr`
-- row below overrides that seed, which would leave the map with NO `eng` key — so an English UI would
-- fall through to Ukrainian. Seeding an explicit `eng` row (= the English `name` column) for every
-- translated catalog keeps English present. Create-if-absent, so 0009's personal_code_scheme eng rows
-- (a curated English label distinct from the native column) are not clobbered.
-- ------------------------------------------------------------------------------------------------
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

-- ------------------------------------------------------------------------------------------------
-- tenant_domains (entity_type 'domain', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'domain', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('military','ukr','Військовий'),   ('military','spa','Militar'),              ('military','por','Militar'),
  ('government','ukr','Державний'),  ('government','spa','Gobierno'),           ('government','por','Governo'),
  ('company','ukr','Компанія'),      ('company','spa','Empresa'),               ('company','por','Empresa'),
  ('university','ukr','Університет'), ('university','spa','Universidad'),        ('university','por','Universidade'),
  ('church','ukr','Церква'),         ('church','spa','Iglesia'),                ('church','por','Igreja'),
  ('public-org','ukr','Громадська організація'), ('public-org','spa','Organización pública'), ('public-org','por','Organização pública')
) AS v(code, locale, text)
JOIN oikumenea.tenant_domains t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- tenant_unit_kinds (entity_type 'unit_kind', RID-keyed). A code shared across domains (division,
-- department) resolves to every matching row and shares one translation — acceptable for these.
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'unit_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('ministry-of-defence','ukr','Міністерство оборони'), ('ministry-of-defence','spa','Ministerio de Defensa'), ('ministry-of-defence','por','Ministério da Defesa'),
  ('armed-forces','ukr','Збройні сили'),   ('armed-forces','spa','Fuerzas Armadas'),   ('armed-forces','por','Forças Armadas'),
  ('service-branch','ukr','Вид збройних сил'), ('service-branch','spa','Rama de servicio'), ('service-branch','por','Ramo das forças'),
  ('command','ukr','Командування'),        ('command','spa','Comando'),                ('command','por','Comando'),
  ('army-group','ukr','Група армій / Фронт'), ('army-group','spa','Grupo de Ejércitos / Frente'), ('army-group','por','Grupo de Exércitos / Frente'),
  ('army','ukr','Польова армія'),          ('army','spa','Ejército de campaña'),       ('army','por','Exército de campanha'),
  ('corps','ukr','Корпус'),                ('corps','spa','Cuerpo'),                   ('corps','por','Corpo'),
  ('division','ukr','Дивізія'),            ('division','spa','División'),              ('division','por','Divisão'),
  ('brigade','ukr','Бригада'),             ('brigade','spa','Brigada'),                ('brigade','por','Brigada'),
  ('regiment','ukr','Полк'),               ('regiment','spa','Regimiento'),           ('regiment','por','Regimento'),
  ('battalion','ukr','Батальйон'),         ('battalion','spa','Batallón'),            ('battalion','por','Batalhão'),
  ('company','ukr','Рота'),                ('company','spa','Compañía'),              ('company','por','Companhia'),
  ('platoon','ukr','Взвод'),               ('platoon','spa','Pelotón'),               ('platoon','por','Pelotão'),
  ('squad','ukr','Відділення'),            ('squad','spa','Escuadra / Sección'),      ('squad','por','Esquadra / Seção'),
  ('fire-team','ukr','Бойова група / Обслуга'), ('fire-team','spa','Equipo de fuego / Dotación'), ('fire-team','por','Equipe de fogo / Guarnição'),
  ('wing','ukr','Авіакрило'),              ('wing','spa','Ala'),                       ('wing','por','Ala'),
  ('air-group','ukr','Авіагрупа'),         ('air-group','spa','Grupo (Aéreo)'),       ('air-group','por','Grupo (Aéreo)'),
  ('air-squadron','ukr','Авіаескадрилья'), ('air-squadron','spa','Escuadrón (Aéreo)'), ('air-squadron','por','Esquadrão (Aéreo)'),
  ('flight','ukr','Ланка'),                ('flight','spa','Escuadrilla'),            ('flight','por','Esquadrilha'),
  ('fleet','ukr','Флот'),                  ('fleet','spa','Flota'),                    ('fleet','por','Frota'),
  ('flotilla','ukr','Флотилія'),           ('flotilla','spa','Flotilla'),             ('flotilla','por','Flotilha'),
  ('naval-squadron','ukr','Ескадра (ВМС)'), ('naval-squadron','spa','Escuadrón (Naval)'), ('naval-squadron','por','Esquadrão (Naval)'),
  ('ministry','ukr','Міністерство'),       ('ministry','spa','Ministerio'),           ('ministry','por','Ministério'),
  ('agency','ukr','Агентство'),            ('agency','spa','Agencia'),                ('agency','por','Agência'),
  ('department','ukr','Департамент'),      ('department','spa','Departamento'),       ('department','por','Departamento'),
  ('team','ukr','Команда'),                ('team','spa','Equipo'),                    ('team','por','Equipe'),
  ('campus','ukr','Кампус'),               ('campus','spa','Campus'),                  ('campus','por','Campus'),
  ('institute','ukr','Інститут'),          ('institute','spa','Instituto'),           ('institute','por','Instituto'),
  ('faculty','ukr','Факультет'),           ('faculty','spa','Facultad'),              ('faculty','por','Faculdade'),
  ('chair','ukr','Кафедра'),               ('chair','spa','Cátedra'),                 ('chair','por','Cátedra'),
  ('diocese','ukr','Єпархія'),             ('diocese','spa','Diócesis'),              ('diocese','por','Diocese'),
  ('parish','ukr','Парафія'),              ('parish','spa','Parroquia'),              ('parish','por','Paróquia'),
  ('chapter','ukr','Осередок'),            ('chapter','spa','Capítulo'),              ('chapter','por','Capítulo')
) AS v(code, locale, text)
JOIN oikumenea.tenant_unit_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- person_relation_types (entity_type 'relation_type', CODE-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'relation_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('godparent','ukr','Хрещений батько/мати'), ('godparent','spa','Padrino/Madrina'), ('godparent','por','Padrinho/Madrinha'),
  ('academic_advisor','ukr','Науковий керівник'), ('academic_advisor','spa','Asesor académico'), ('academic_advisor','por','Orientador acadêmico'),
  ('military_mentor','ukr','Військовий наставник'), ('military_mentor','spa','Mentor militar'), ('military_mentor','por','Mentor militar'),
  ('spouse','ukr','Подружжя'),      ('spouse','spa','Cónyuge'),      ('spouse','por','Cônjuge'),
  ('parent','ukr','Батько/мати'),   ('parent','spa','Progenitor'),   ('parent','por','Progenitor'),
  ('child','ukr','Дитина'),         ('child','spa','Hijo/a'),        ('child','por','Filho/a'),
  ('sibling','ukr','Брат/сестра'),  ('sibling','spa','Hermano/a'),   ('sibling','por','Irmão/ã'),
  ('next_of_kin_other','ukr','Інше (найближчі родичі)'), ('next_of_kin_other','spa','Otro (familiar cercano)'), ('next_of_kin_other','por','Outro (parente próximo)'),
  ('colleague','ukr','Колега'),     ('colleague','spa','Colega'),    ('colleague','por','Colega'),
  ('business_associate','ukr','Діловий партнер'), ('business_associate','spa','Socio comercial'), ('business_associate','por','Sócio comercial'),
  ('emergency','ukr','Екстрений контакт'), ('emergency','spa','Contacto de emergencia'), ('emergency','por','Contato de emergência')
) AS v(code, locale, text)
JOIN oikumenea.person_relation_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- person_email_types / person_phone_types (CODE-keyed; distinct entity_type so shared codes are fine)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'email_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('personal','ukr','Особистий'), ('personal','spa','Personal'), ('personal','por','Pessoal'),
  ('work','ukr','Робочий'),       ('work','spa','Trabajo'),      ('work','por','Trabalho'),
  ('other','ukr','Інший'),        ('other','spa','Otro'),        ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.person_email_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'phone_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('mobile','ukr','Мобільний'), ('mobile','spa','Móvil'),   ('mobile','por','Celular'),
  ('home','ukr','Домашній'),    ('home','spa','Casa'),      ('home','por','Residencial'),
  ('work','ukr','Робочий'),     ('work','spa','Trabajo'),   ('work','por','Trabalho'),
  ('other','ukr','Інший'),      ('other','spa','Otro'),     ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.person_phone_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- document_personal_code_schemes (entity_type 'personal_code_scheme', CODE-keyed) — eng/ukr already
-- seeded in 0009; add spa/por (pl-pesel is the same token in all locales, so it is left to fallback).
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'personal_code_scheme', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('ua-rnokpp','spa','Número de identificación fiscal individual (RNOKPP)'), ('ua-rnokpp','por','Número de identificação fiscal individual (RNOKPP)'),
  ('ua-unzr','spa','Número de registro único (UNZR)'), ('ua-unzr','por','Número de registro único (UNZR)'),
  ('us-ssn','spa','Número de Seguro Social'),          ('us-ssn','por','Número de Seguro Social'),
  ('de-steuer-id','spa','Identificación fiscal (Steuer-ID)'), ('de-steuer-id','por','Identificação fiscal (Steuer-ID)'),
  ('it-codice-fiscale','spa','Código fiscal (Codice Fiscale)'), ('it-codice-fiscale','por','Código fiscal (Codice Fiscale)')
) AS v(code, locale, text)
JOIN oikumenea.document_personal_code_schemes t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- document_document_types (entity_type 'document_type', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'document_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('passport','ukr','Паспорт'),          ('passport','spa','Pasaporte'),           ('passport','por','Passaporte'),
  ('national-id','ukr','Посвідчення особи'), ('national-id','spa','Documento nacional de identidad'), ('national-id','por','Documento nacional de identidade'),
  ('tax-id','ukr','Податковий документ'), ('tax-id','spa','Documento de identificación fiscal'), ('tax-id','por','Documento de identificação fiscal'),
  ('social-insurance','ukr','Картка соціального страхування'), ('social-insurance','spa','Tarjeta de seguro social'), ('social-insurance','por','Cartão de seguro social'),
  ('driver-license','ukr','Посвідчення водія'), ('driver-license','spa','Licencia de conducir'), ('driver-license','por','Carteira de habilitação'),
  ('military-id','ukr','Військовий квиток'), ('military-id','spa','Documento de identidad militar'), ('military-id','por','Documento de identidade militar'),
  ('diploma','ukr','Диплом')
) AS v(code, locale, text)
JOIN oikumenea.document_document_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- location_location_types (entity_type 'location_type', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'location_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('building','ukr','Будівля'), ('building','spa','Edificio'),  ('building','por','Prédio'),
  ('address','ukr','Адреса'),   ('address','spa','Dirección'),  ('address','por','Endereço'),
  ('online','ukr','Онлайн'),    ('online','spa','En línea'),    ('online','por','On-line')
) AS v(code, locale, text)
JOIN oikumenea.location_location_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- education_institution_kinds (entity_type 'education_institution_kind', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_institution_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('kindergarten','ukr','Дитячий садок'), ('kindergarten','spa','Jardín de infancia'), ('kindergarten','por','Jardim de infância'),
  ('school','ukr','Школа'),        ('school','spa','Escuela'),              ('school','por','Escola'),
  ('lyceum','ukr','Ліцей'),        ('lyceum','spa','Liceo'),                ('lyceum','por','Liceu'),
  ('gymnasium','ukr','Гімназія'),  ('gymnasium','spa','Gimnasio'),          ('gymnasium','por','Ginásio'),
  ('vocational','ukr','Професійно-технічний заклад'), ('vocational','spa','Escuela vocacional'), ('vocational','por','Escola profissionalizante'),
  ('college','ukr','Коледж'),      ('college','spa','Colegio'),             ('college','por','Colégio'),
  ('institute','ukr','Інститут'),  ('institute','spa','Instituto'),         ('institute','por','Instituto'),
  ('university','ukr','Університет'), ('university','spa','Universidad'),    ('university','por','Universidade'),
  ('academy','ukr','Академія'),    ('academy','spa','Academia'),            ('academy','por','Academia')
) AS v(code, locale, text)
JOIN oikumenea.education_institution_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- education_degree_levels (entity_type 'education_degree_level', RID-keyed) — ISCED 2011
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_degree_level', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('isced-0','ukr','Дошкільна освіта'),   ('isced-0','spa','Educación de la primera infancia'), ('isced-0','por','Educação infantil'),
  ('isced-1','ukr','Початкова освіта'),   ('isced-1','spa','Educación primaria'),   ('isced-1','por','Ensino fundamental (anos iniciais)'),
  ('isced-2','ukr','Базова середня освіта'), ('isced-2','spa','Educación secundaria inferior'), ('isced-2','por','Ensino fundamental (anos finais)'),
  ('isced-3','ukr','Повна середня освіта'), ('isced-3','spa','Educación secundaria superior'), ('isced-3','por','Ensino médio'),
  ('isced-4','ukr','Післясередня нетретинна освіта'), ('isced-4','spa','Educación postsecundaria no terciaria'), ('isced-4','por','Educação pós-secundária não terciária'),
  ('isced-5','ukr','Короткий цикл вищої освіти'), ('isced-5','spa','Educación terciaria de ciclo corto'), ('isced-5','por','Educação superior de ciclo curto'),
  ('isced-6','ukr','Бакалавр або еквівалент'), ('isced-6','spa','Grado o equivalente'), ('isced-6','por','Bacharelado ou equivalente'),
  ('isced-7','ukr','Магістр або еквівалент'), ('isced-7','spa','Máster o equivalente'), ('isced-7','por','Mestrado ou equivalente'),
  ('isced-8','ukr','Доктор філософії або еквівалент'), ('isced-8','spa','Doctorado o equivalente'), ('isced-8','por','Doutorado ou equivalente')
) AS v(code, locale, text)
JOIN oikumenea.education_degree_levels t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- company_legal_forms (entity_type 'company_legal_form', RID-keyed) — the generic forms; the German
-- 'gmbh' and the UA-specific native-named forms are left to their (proper-noun) column fallback.
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_legal_form', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('llc','ukr','Товариство з обмеженою відповідальністю'), ('llc','spa','Sociedad de responsabilidad limitada'), ('llc','por','Sociedade de responsabilidade limitada'),
  ('jsc','ukr','Акціонерне товариство'), ('jsc','spa','Sociedad anónima'), ('jsc','por','Sociedade anônima'),
  ('plc','ukr','Публічне акціонерне товариство'), ('plc','spa','Sociedad anónima cotizada'), ('plc','por','Sociedade anônima de capital aberto'),
  ('sole-proprietor','ukr','Фізична особа-підприємець'), ('sole-proprietor','spa','Empresario individual'), ('sole-proprietor','por','Empresário individual'),
  ('state-enterprise','ukr','Державне підприємство'), ('state-enterprise','spa','Empresa estatal'), ('state-enterprise','por','Empresa estatal')
) AS v(code, locale, text)
JOIN oikumenea.company_legal_forms t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- company_registration_schemes (entity_type 'company_registration_scheme', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_registration_scheme', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('lei','ukr','Ідентифікатор юридичної особи (ISO 17442)'), ('lei','spa','Identificador de entidad jurídica (ISO 17442)'), ('lei','por','Identificador de entidade jurídica (ISO 17442)'),
  ('duns','ukr','Номер D-U-N-S (Dun & Bradstreet)'), ('duns','spa','Número D-U-N-S (Dun & Bradstreet)'), ('duns','por','Número D-U-N-S (Dun & Bradstreet)'),
  ('ua-edrpou','ukr','Код ЄДРПОУ'), ('ua-edrpou','spa','Código EDRPOU (Ucrania)'), ('ua-edrpou','por','Código EDRPOU (Ucrânia)'),
  ('vat','ukr','Номер платника ПДВ'), ('vat','spa','Número de IVA'), ('vat','por','Número de IVA'),
  ('us-ein','ukr','Ідентифікаційний номер роботодавця (США)'), ('us-ein','spa','Número de identificación patronal (EE. UU.)'), ('us-ein','por','Número de identificação do empregador (EUA)')
) AS v(code, locale, text)
JOIN oikumenea.company_registration_schemes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- company_industry_classes (entity_type 'company_industry_class', RID-keyed) — NACE starter set
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_industry_class', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('nace-a','ukr','Сільське, лісове та рибне господарство'), ('nace-a','spa','Agricultura, silvicultura y pesca'), ('nace-a','por','Agricultura, silvicultura e pesca'),
  ('nace-c','ukr','Переробна промисловість'), ('nace-c','spa','Industria manufacturera'), ('nace-c','por','Indústria de transformação'),
  ('nace-f','ukr','Будівництво'), ('nace-f','spa','Construcción'), ('nace-f','por','Construção'),
  ('nace-g','ukr','Оптова та роздрібна торгівля'), ('nace-g','spa','Comercio al por mayor y al por menor'), ('nace-g','por','Comércio atacadista e varejista'),
  ('nace-j','ukr','Інформація та телекомунікації'), ('nace-j','spa','Información y comunicación'), ('nace-j','por','Informação e comunicação'),
  ('nace-k','ukr','Фінансова та страхова діяльність'), ('nace-k','spa','Actividades financieras y de seguros'), ('nace-k','por','Atividades financeiras e de seguros'),
  ('nace-m','ukr','Професійна, наукова та технічна діяльність'), ('nace-m','spa','Actividades profesionales, científicas y técnicas'), ('nace-m','por','Atividades profissionais, científicas e técnicas'),
  ('nace-q','ukr','Охорона здоровʼя та соціальна допомога'), ('nace-q','spa','Actividades sanitarias y de servicios sociales'), ('nace-q','por','Saúde humana e serviços sociais')
) AS v(code, locale, text)
JOIN oikumenea.company_industry_classes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- external_org_kinds (entity_type 'external_org_kind', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'external_org_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('party','ukr','Політична партія'), ('party','spa','Partido político'), ('party','por','Partido político'),
  ('government_body','ukr','Державний орган'), ('government_body','spa','Órgano gubernamental'), ('government_body','por','Órgão governamental'),
  ('military','ukr','Військове формування'), ('military','spa','Formación militar'), ('military','por','Formação militar'),
  ('ngo','ukr','Неурядова організація'), ('ngo','spa','Organización no gubernamental'), ('ngo','por','Organização não governamental'),
  ('registrant','ukr','Лобіст / клієнт'), ('registrant','spa','Registrante de cabildeo / cliente'), ('registrant','por','Registrante de lobby / cliente'),
  ('other','ukr','Інше'), ('other','spa','Otro'), ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.external_org_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- finance_account_types (entity_type 'finance_account_type', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'finance_account_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('current','ukr','Поточний рахунок'), ('current','spa','Cuenta corriente'), ('current','por','Conta corrente'),
  ('savings','ukr','Ощадний рахунок'),  ('savings','spa','Cuenta de ahorros'), ('savings','por','Conta poupança'),
  ('deposit','ukr','Депозитний рахунок'), ('deposit','spa','Cuenta de depósito'), ('deposit','por','Conta de depósito'),
  ('loan','ukr','Кредитний рахунок'),   ('loan','spa','Cuenta de préstamo'),  ('loan','por','Conta de empréstimo'),
  ('card','ukr','Картковий рахунок'),   ('card','spa','Cuenta de tarjeta'),   ('card','por','Conta de cartão')
) AS v(code, locale, text)
JOIN oikumenea.finance_account_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- vehicle_types (entity_type 'vehicle_type', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('car','ukr','Легковий автомобіль'), ('car','spa','Automóvil'),   ('car','por','Automóvel'),
  ('truck','ukr','Вантажівка'),        ('truck','spa','Camión'),     ('truck','por','Caminhão'),
  ('motorcycle','ukr','Мотоцикл'),     ('motorcycle','spa','Motocicleta'), ('motorcycle','por','Motocicleta'),
  ('bus','ukr','Автобус'),             ('bus','spa','Autobús'),      ('bus','por','Ônibus'),
  ('trailer','ukr','Причіп'),          ('trailer','spa','Remolque'), ('trailer','por','Reboque'),
  ('special','ukr','Спеціальний транспорт'), ('special','spa','Vehículo especial'), ('special','por','Veículo especial')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ------------------------------------------------------------------------------------------------
-- vehicle_registration_number_types (entity_type 'vehicle_registration_number_type', RID-keyed)
-- ------------------------------------------------------------------------------------------------
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_registration_number_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('regular','ukr','Звичайний'),     ('regular','spa','Regular'),      ('regular','por','Comum'),
  ('temporary','ukr','Тимчасовий'),  ('temporary','spa','Temporal'),   ('temporary','por','Temporário'),
  ('transit','ukr','Транзитний'),    ('transit','spa','Tránsito'),     ('transit','por','Trânsito'),
  ('diplomatic','ukr','Дипломатичний'), ('diplomatic','spa','Diplomático'), ('diplomatic','por','Diplomático'),
  ('military','ukr','Військовий'),   ('military','spa','Militar'),     ('military','por','Militar'),
  ('old','ukr','Старий / історичний'), ('old','spa','Antiguo / histórico'), ('old','por','Antigo / histórico')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_registration_number_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------
-- Advance the single-row schema-version marker the boot-time readiness gate reads (upgrade-safety.md).
-- ExpectedSchemaRevision derives from this literal (schemaversion.go), so no Go bump is needed.
UPDATE oikumenea.schema_version SET revision = '0045_i18n_catalog_translations', applied_at = now() WHERE singleton;
