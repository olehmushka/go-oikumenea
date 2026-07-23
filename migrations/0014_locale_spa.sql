-- 0014_locale_spa — self-contained spa locale: registers the locale, links its languoid, and seeds
-- all spa translation rows. Adding a new locale = drop in one more 00NN_locale_<code>.sql after this.
-- (Extracted from the former 0002/0009/0018/0023-0026/0045 migrations by the migration refactor.)

-- locale registration
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order) VALUES
  ('spa', 'Español (Latinoamérica)', true, false, 20);

-- link this locale to its Glottolog languoid (was in 0018)
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT 'spa', x.id FROM oikumenea.language_languoids x WHERE x.iso639_3 = 'spa' ON CONFLICT DO NOTHING;

-- spa translations
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon', t.id::text, 'name', 'spa', v.text
FROM (VALUES
  -- Religions (roots)
  ('christianity','Cristianismo'),
  ('islam','Islam'),
  ('judaism','Judaísmo'),
  ('hinduism','Hinduismo'),
  ('buddhism','Budismo'),
  ('sikhism','Sijismo'),
  ('jainism','Jainismo'),
  ('bahai','Fe bahá''í'),
  ('shinto','Sintoísmo'),
  ('taoism','Taoísmo'),
  ('confucianism','Confucianismo'),
  ('zoroastrianism','Zoroastrismo'),
  ('atheism','Ateísmo'),
  ('agnosticism','Agnosticismo'),
  ('traditional','Religiones tradicionales e indígenas'),
  ('other','Otro / sin clasificar'),
  -- Branches
  ('catholicism','Catolicismo'),
  ('eastern_orthodoxy','Ortodoxia oriental'),
  ('oriental_orthodoxy','Ortodoxia oriental antigua'),
  ('church_of_the_east','Iglesia del Oriente'),
  ('protestantism','Protestantismo'),
  ('restorationism','Restauracionismo y no trinitarios'),
  ('independent_christianity','Cristianismo independiente / no confesional'),
  ('sunni','Islam suní'),
  ('shia','Islam chií'),
  ('ibadi','Islam ibadí'),
  ('sufism','Sufismo'),
  ('ahmadiyya','Ahmadía'),
  ('orthodox_judaism','Judaísmo ortodoxo'),
  ('conservative_judaism','Judaísmo conservador'),
  ('reform_judaism','Judaísmo reformista'),
  ('reconstructionist_judaism','Judaísmo reconstruccionista'),
  ('karaite_judaism','Judaísmo caraíta'),
  ('vaishnavism','Vaishnavismo'),
  ('shaivism','Shivaísmo'),
  ('shaktism','Shaktismo'),
  ('smartism','Smartismo'),
  ('theravada','Theravada'),
  ('mahayana','Mahayana'),
  ('vajrayana','Vajrayana'),
  ('digambara','Digambara'),
  ('svetambara','Svetambara'),
  -- Traditions
  ('latin_church','Iglesia latina'),
  ('eastern_catholic','Iglesias católicas orientales'),
  ('lutheranism','Luteranismo'),
  ('reformed','Reformada (calvinismo)'),
  ('anglicanism','Anglicanismo'),
  ('anabaptism','Anabaptismo'),
  ('baptist','Bautista'),
  ('methodism','Metodismo'),
  ('pentecostalism','Pentecostalismo'),
  ('adventism','Adventismo'),
  ('holiness','Movimiento de santidad'),
  ('evangelicalism','Evangelicalismo'),
  ('quakerism','Cuaquerismo (Amigos)'),
  -- Sub-traditions
  ('hanafi','Hanafí'),
  ('maliki','Malikí'),
  ('shafii','Shafi''í'),
  ('hanbali','Hanbalí'),
  ('twelver','Duodecimano'),
  ('ismailism','Ismailismo'),
  ('zaidiyyah','Zaidismo'),
  ('hasidic','Judaísmo jasídico'),
  ('modern_orthodox','Judaísmo ortodoxo moderno'),
  ('haredi','Judaísmo haredí'),
  ('presbyterianism','Presbiterianismo'),
  ('congregationalism','Congregacionalismo'),
  ('continental_reformed','Reformada continental'),
  -- Denominations
  ('ecumenical_patriarchate','Patriarcado Ecuménico de Constantinopla'),
  ('church_of_greece','Iglesia de Grecia'),
  ('russian_orthodox_church','Iglesia ortodoxa rusa'),
  ('serbian_orthodox_church','Iglesia ortodoxa serbia'),
  ('romanian_orthodox_church','Iglesia ortodoxa rumana'),
  ('bulgarian_orthodox_church','Iglesia ortodoxa búlgara'),
  ('georgian_orthodox_church','Iglesia ortodoxa georgiana'),
  ('orthodox_church_of_ukraine','Iglesia ortodoxa de Ucrania'),
  ('orthodox_church_in_america','Iglesia ortodoxa en América'),
  ('coptic_orthodox_church','Iglesia ortodoxa copta'),
  ('armenian_apostolic_church','Iglesia apostólica armenia'),
  ('ethiopian_orthodox_tewahedo','Iglesia ortodoxa etíope Tewahedo'),
  ('syriac_orthodox_church','Iglesia ortodoxa siríaca'),
  ('malankara_orthodox_church','Iglesia ortodoxa siria de Malankara'),
  ('assyrian_church_of_the_east','Iglesia asiria del Oriente'),
  ('ancient_church_of_the_east','Antigua Iglesia del Oriente'),
  ('ukrainian_greek_catholic_church','Iglesia greco-católica ucraniana'),
  ('maronite_church','Iglesia maronita'),
  ('melkite_greek_catholic_church','Iglesia greco-católica melquita'),
  ('chaldean_catholic_church','Iglesia católica caldea'),
  ('syro_malabar_church','Iglesia siro-malabar'),
  ('armenian_catholic_church','Iglesia católica armenia'),
  ('elca','Iglesia Evangélica Luterana en América'),
  ('lcms','Iglesia Luterana – Sínodo de Misuri'),
  ('southern_baptist_convention','Convención Bautista del Sur'),
  ('united_methodist_church','Iglesia Metodista Unida'),
  ('church_of_england','Iglesia de Inglaterra'),
  ('episcopal_church_usa','Iglesia Episcopal (EE. UU.)'),
  ('assemblies_of_god','Asambleas de Dios'),
  ('seventh_day_adventist_church','Iglesia Adventista del Séptimo Día'),
  ('lds_church','Iglesia de Jesucristo de los Santos de los Últimos Días'),
  ('jehovahs_witnesses','Testigos de Jehová')
) AS v(code, text)
JOIN oikumenea.religion_taxa t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('major_orders','spa','Órdenes mayores'),
  ('minor_orders','spa','Órdenes menores'),
  ('religious_leadership','spa','Liderazgo religioso'),
  ('clergy','spa','Clero'),
  ('monastic','spa','Monástico'),
  ('priestly','spa','Sacerdotal')
) AS v(code, locale, text)
JOIN oikumenea.religion_grade_categories c ON c.code = v.code AND c.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('bishop','spa','Obispo'),
  ('presbyter','spa','Presbítero / sacerdote'),
  ('deacon','spa','Diácono'),
  ('subdeacon','spa','Subdiácono'),
  ('reader','spa','Lector'),
  ('mufti','spa','Muftí'),
  ('imam','spa','Imán'),
  ('sheikh','spa','Jeque'),
  ('rabbi','spa','Rabino'),
  ('cantor','spa','Cantor (jazán)'),
  ('bhikkhu','spa','Bhikkhu'),
  ('lama','spa','Lama'),
  ('pujari','spa','Pujari'),
  ('swami','spa','Suami')
) AS v(code, locale, text)
JOIN oikumenea.religion_clergy_grades g ON g.code = v.code AND g.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_affiliation_type', a.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('adherent','spa','Adherente'),
  ('member','spa','Miembro'),
  ('catechumen','spa','Catecúmeno'),
  ('baptized','spa','Bautizado'),
  ('confirmed','spa','Confirmado'),
  ('shahada','spa','Shahada'),
  ('bar_bat_mitzvah','spa','Bar / Bat mitzvá')
) AS v(code, locale, text)
JOIN oikumenea.religion_affiliation_types a ON a.code = v.code AND a.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('office','spa','Oficina'),
  ('online','spa','En línea'),
  ('mission','spa','Misión'),
  ('shrine','spa','Santuario'),
  ('church','spa','Iglesia'),
  ('cathedral','spa','Catedral'),
  ('chapel','spa','Capilla'),
  ('monastery','spa','Monasterio'),
  ('mosque','spa','Mezquita'),
  ('synagogue','spa','Sinagoga'),
  ('temple','spa','Templo'),
  ('gurdwara','spa','Gurdwara')
) AS v(code, locale, text)
JOIN oikumenea.religion_site_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('main','spa','Servicio principal'),
  ('youth','spa','Servicio juvenil'),
  ('prayer','spa','Oración'),
  ('special','spa','Servicio especial'),
  ('daily_mass','spa','Misa diaria'),
  ('jumua','spa','Oración del viernes (yumu''a)'),
  ('shabbat','spa','Servicio de Shabat'),
  ('puja','spa','Puyá'),
  ('meditation','spa','Meditación')
) AS v(code, locale, text)
JOIN oikumenea.religion_service_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'domain', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('military','spa','Militar'),
  ('government','spa','Gobierno'),
  ('company','spa','Empresa'),
  ('university','spa','Universidad'),
  ('church','spa','Iglesia'),
  ('public-org','spa','Organización pública')
) AS v(code, locale, text)
JOIN oikumenea.tenant_domains t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'unit_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('ministry-of-defence','spa','Ministerio de Defensa'),
  ('armed-forces','spa','Fuerzas Armadas'),
  ('service-branch','spa','Rama de servicio'),
  ('command','spa','Comando'),
  ('army-group','spa','Grupo de Ejércitos / Frente'),
  ('army','spa','Ejército de campaña'),
  ('corps','spa','Cuerpo'),
  ('division','spa','División'),
  ('brigade','spa','Brigada'),
  ('regiment','spa','Regimiento'),
  ('battalion','spa','Batallón'),
  ('company','spa','Compañía'),
  ('platoon','spa','Pelotón'),
  ('squad','spa','Escuadra / Sección'),
  ('fire-team','spa','Equipo de fuego / Dotación'),
  ('wing','spa','Ala'),
  ('air-group','spa','Grupo (Aéreo)'),
  ('air-squadron','spa','Escuadrón (Aéreo)'),
  ('flight','spa','Escuadrilla'),
  ('fleet','spa','Flota'),
  ('flotilla','spa','Flotilla'),
  ('naval-squadron','spa','Escuadrón (Naval)'),
  ('ministry','spa','Ministerio'),
  ('agency','spa','Agencia'),
  ('department','spa','Departamento'),
  ('team','spa','Equipo'),
  ('campus','spa','Campus'),
  ('institute','spa','Instituto'),
  ('faculty','spa','Facultad'),
  ('chair','spa','Cátedra'),
  ('diocese','spa','Diócesis'),
  ('parish','spa','Parroquia'),
  ('chapter','spa','Capítulo')
) AS v(code, locale, text)
JOIN oikumenea.tenant_unit_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'relation_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('godparent','spa','Padrino/Madrina'),
  ('academic_advisor','spa','Asesor académico'),
  ('military_mentor','spa','Mentor militar'),
  ('spouse','spa','Cónyuge'),
  ('parent','spa','Progenitor'),
  ('child','spa','Hijo/a'),
  ('sibling','spa','Hermano/a'),
  ('next_of_kin_other','spa','Otro (familiar cercano)'),
  ('colleague','spa','Colega'),
  ('business_associate','spa','Socio comercial'),
  ('emergency','spa','Contacto de emergencia')
) AS v(code, locale, text)
JOIN oikumenea.person_relation_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'email_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('personal','spa','Personal'),
  ('work','spa','Trabajo'),
  ('other','spa','Otro')
) AS v(code, locale, text)
JOIN oikumenea.person_email_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'phone_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('mobile','spa','Móvil'),
  ('home','spa','Casa'),
  ('work','spa','Trabajo'),
  ('other','spa','Otro')
) AS v(code, locale, text)
JOIN oikumenea.person_phone_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'personal_code_scheme', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('ua-rnokpp','spa','Número de identificación fiscal individual (RNOKPP)'),
  ('ua-unzr','spa','Número de registro único (UNZR)'),
  ('us-ssn','spa','Número de Seguro Social'),
  ('de-steuer-id','spa','Identificación fiscal (Steuer-ID)'),
  ('it-codice-fiscale','spa','Código fiscal (Codice Fiscale)')
) AS v(code, locale, text)
JOIN oikumenea.document_personal_code_schemes t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'document_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('passport','spa','Pasaporte'),
  ('national-id','spa','Documento nacional de identidad'),
  ('tax-id','spa','Documento de identificación fiscal'),
  ('social-insurance','spa','Tarjeta de seguro social'),
  ('driver-license','spa','Licencia de conducir'),
  ('military-id','spa','Documento de identidad militar')
) AS v(code, locale, text)
JOIN oikumenea.document_document_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'location_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('building','spa','Edificio'),
  ('address','spa','Dirección'),
  ('online','spa','En línea')
) AS v(code, locale, text)
JOIN oikumenea.location_location_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_institution_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('kindergarten','spa','Jardín de infancia'),
  ('school','spa','Escuela'),
  ('lyceum','spa','Liceo'),
  ('gymnasium','spa','Gimnasio'),
  ('vocational','spa','Escuela vocacional'),
  ('college','spa','Colegio'),
  ('institute','spa','Instituto'),
  ('university','spa','Universidad'),
  ('academy','spa','Academia')
) AS v(code, locale, text)
JOIN oikumenea.education_institution_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_degree_level', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('isced-0','spa','Educación de la primera infancia'),
  ('isced-1','spa','Educación primaria'),
  ('isced-2','spa','Educación secundaria inferior'),
  ('isced-3','spa','Educación secundaria superior'),
  ('isced-4','spa','Educación postsecundaria no terciaria'),
  ('isced-5','spa','Educación terciaria de ciclo corto'),
  ('isced-6','spa','Grado o equivalente'),
  ('isced-7','spa','Máster o equivalente'),
  ('isced-8','spa','Doctorado o equivalente')
) AS v(code, locale, text)
JOIN oikumenea.education_degree_levels t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_legal_form', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('llc','spa','Sociedad de responsabilidad limitada'),
  ('jsc','spa','Sociedad anónima'),
  ('plc','spa','Sociedad anónima cotizada'),
  ('sole-proprietor','spa','Empresario individual'),
  ('state-enterprise','spa','Empresa estatal')
) AS v(code, locale, text)
JOIN oikumenea.company_legal_forms t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_registration_scheme', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('lei','spa','Identificador de entidad jurídica (ISO 17442)'),
  ('duns','spa','Número D-U-N-S (Dun & Bradstreet)'),
  ('ua-edrpou','spa','Código EDRPOU (Ucrania)'),
  ('vat','spa','Número de IVA'),
  ('us-ein','spa','Número de identificación patronal (EE. UU.)')
) AS v(code, locale, text)
JOIN oikumenea.company_registration_schemes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_industry_class', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('nace-a','spa','Agricultura, silvicultura y pesca'),
  ('nace-c','spa','Industria manufacturera'),
  ('nace-f','spa','Construcción'),
  ('nace-g','spa','Comercio al por mayor y al por menor'),
  ('nace-j','spa','Información y comunicación'),
  ('nace-k','spa','Actividades financieras y de seguros'),
  ('nace-m','spa','Actividades profesionales, científicas y técnicas'),
  ('nace-q','spa','Actividades sanitarias y de servicios sociales')
) AS v(code, locale, text)
JOIN oikumenea.company_industry_classes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'external_org_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('party','spa','Partido político'),
  ('government_body','spa','Órgano gubernamental'),
  ('military','spa','Formación militar'),
  ('ngo','spa','Organización no gubernamental'),
  ('registrant','spa','Registrante de cabildeo / cliente'),
  ('other','spa','Otro')
) AS v(code, locale, text)
JOIN oikumenea.external_org_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'finance_account_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('current','spa','Cuenta corriente'),
  ('savings','spa','Cuenta de ahorros'),
  ('deposit','spa','Cuenta de depósito'),
  ('loan','spa','Cuenta de préstamo'),
  ('card','spa','Cuenta de tarjeta')
) AS v(code, locale, text)
JOIN oikumenea.finance_account_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('car','spa','Automóvil'),
  ('truck','spa','Camión'),
  ('motorcycle','spa','Motocicleta'),
  ('bus','spa','Autobús'),
  ('trailer','spa','Remolque'),
  ('special','spa','Vehículo especial')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_registration_number_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('regular','spa','Regular'),
  ('temporary','spa','Temporal'),
  ('transit','spa','Tránsito'),
  ('diplomatic','spa','Diplomático'),
  ('military','spa','Militar'),
  ('old','spa','Antiguo / histórico')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_registration_number_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

UPDATE oikumenea.schema_version SET revision = '0014_locale_spa', applied_at = now() WHERE singleton;
