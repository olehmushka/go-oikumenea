-- 0015_locale_por — self-contained por locale: registers the locale, links its languoid, and seeds
-- all por translation rows. Adding a new locale = drop in one more 00NN_locale_<code>.sql after this.
-- (Extracted from the former 0002/0009/0018/0023-0026/0045 migrations by the migration refactor.)

-- locale registration
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order) VALUES
  ('por', 'Português (Brasil)',      true, false, 30);

-- link this locale to its Glottolog languoid (was in 0018)
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT 'por', x.id FROM oikumenea.language_languoids x WHERE x.iso639_3 = 'por' ON CONFLICT DO NOTHING;

-- por translations
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon', t.id::text, 'name', 'por', v.text
FROM (VALUES
  -- Religions (roots)
  ('christianity','Cristianismo'),
  ('islam','Islã'),
  ('judaism','Judaísmo'),
  ('hinduism','Hinduísmo'),
  ('buddhism','Budismo'),
  ('sikhism','Sikhismo'),
  ('jainism','Jainismo'),
  ('bahai','Fé bahá''í'),
  ('shinto','Xintoísmo'),
  ('taoism','Taoismo'),
  ('confucianism','Confucionismo'),
  ('zoroastrianism','Zoroastrismo'),
  ('atheism','Ateísmo'),
  ('agnosticism','Agnosticismo'),
  ('traditional','Religiões tradicionais e indígenas'),
  ('other','Outro / não classificado'),
  -- Branches
  ('catholicism','Catolicismo'),
  ('eastern_orthodoxy','Ortodoxia Oriental'),
  ('oriental_orthodoxy','Ortodoxia Oriental Antiga'),
  ('church_of_the_east','Igreja do Oriente'),
  ('protestantism','Protestantismo'),
  ('restorationism','Restauracionismo e não trinitários'),
  ('independent_christianity','Cristianismo independente / não denominacional'),
  ('sunni','Islã sunita'),
  ('shia','Islã xiita'),
  ('ibadi','Islã ibadita'),
  ('sufism','Sufismo'),
  ('ahmadiyya','Ahmadia'),
  ('orthodox_judaism','Judaísmo ortodoxo'),
  ('conservative_judaism','Judaísmo conservador'),
  ('reform_judaism','Judaísmo reformista'),
  ('reconstructionist_judaism','Judaísmo reconstrucionista'),
  ('karaite_judaism','Judaísmo caraíta'),
  ('vaishnavism','Vaishnavismo'),
  ('shaivism','Xivaísmo'),
  ('shaktism','Shaktismo'),
  ('smartism','Smartismo'),
  ('theravada','Teravada'),
  ('mahayana','Mahayana'),
  ('vajrayana','Vajrayana'),
  ('digambara','Digambara'),
  ('svetambara','Svetambara'),
  -- Traditions
  ('latin_church','Igreja Latina'),
  ('eastern_catholic','Igrejas Católicas Orientais'),
  ('lutheranism','Luteranismo'),
  ('reformed','Reformada (calvinismo)'),
  ('anglicanism','Anglicanismo'),
  ('anabaptism','Anabatismo'),
  ('baptist','Batista'),
  ('methodism','Metodismo'),
  ('pentecostalism','Pentecostalismo'),
  ('adventism','Adventismo'),
  ('holiness','Movimento de santidade'),
  ('evangelicalism','Evangelicalismo'),
  ('quakerism','Quacrismo (Amigos)'),
  -- Sub-traditions
  ('hanafi','Hanafita'),
  ('maliki','Malikita'),
  ('shafii','Xafiíta'),
  ('hanbali','Hambalita'),
  ('twelver','Duodecimano'),
  ('ismailism','Ismaelismo'),
  ('zaidiyyah','Zaidismo'),
  ('hasidic','Judaísmo hassídico'),
  ('modern_orthodox','Judaísmo ortodoxo moderno'),
  ('haredi','Judaísmo haredi'),
  ('presbyterianism','Presbiterianismo'),
  ('congregationalism','Congregacionalismo'),
  ('continental_reformed','Reformada continental'),
  -- Denominations
  ('ecumenical_patriarchate','Patriarcado Ecumênico de Constantinopla'),
  ('church_of_greece','Igreja da Grécia'),
  ('russian_orthodox_church','Igreja Ortodoxa Russa'),
  ('serbian_orthodox_church','Igreja Ortodoxa Sérvia'),
  ('romanian_orthodox_church','Igreja Ortodoxa Romena'),
  ('bulgarian_orthodox_church','Igreja Ortodoxa Búlgara'),
  ('georgian_orthodox_church','Igreja Ortodoxa Georgiana'),
  ('orthodox_church_of_ukraine','Igreja Ortodoxa da Ucrânia'),
  ('orthodox_church_in_america','Igreja Ortodoxa na América'),
  ('coptic_orthodox_church','Igreja Ortodoxa Copta'),
  ('armenian_apostolic_church','Igreja Apostólica Armênia'),
  ('ethiopian_orthodox_tewahedo','Igreja Ortodoxa Etíope Tewahedo'),
  ('syriac_orthodox_church','Igreja Ortodoxa Siríaca'),
  ('malankara_orthodox_church','Igreja Ortodoxa Síria de Malankara'),
  ('assyrian_church_of_the_east','Igreja Assíria do Oriente'),
  ('ancient_church_of_the_east','Antiga Igreja do Oriente'),
  ('ukrainian_greek_catholic_church','Igreja Greco-Católica Ucraniana'),
  ('maronite_church','Igreja Maronita'),
  ('melkite_greek_catholic_church','Igreja Greco-Católica Melquita'),
  ('chaldean_catholic_church','Igreja Católica Caldeia'),
  ('syro_malabar_church','Igreja Siro-Malabar'),
  ('armenian_catholic_church','Igreja Católica Armênia'),
  ('elca','Igreja Evangélica Luterana na América'),
  ('lcms','Igreja Luterana – Sínodo de Missouri'),
  ('southern_baptist_convention','Convenção Batista do Sul'),
  ('united_methodist_church','Igreja Metodista Unida'),
  ('church_of_england','Igreja da Inglaterra'),
  ('episcopal_church_usa','Igreja Episcopal (EUA)'),
  ('assemblies_of_god','Assembleias de Deus'),
  ('seventh_day_adventist_church','Igreja Adventista do Sétimo Dia'),
  ('lds_church','Igreja de Jesus Cristo dos Santos dos Últimos Dias'),
  ('jehovahs_witnesses','Testemunhas de Jeová')
) AS v(code, text)
JOIN oikumenea.religion_taxa t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('major_orders','por','Ordens maiores'),
  ('minor_orders','por','Ordens menores'),
  ('religious_leadership','por','Liderança religiosa'),
  ('clergy','por','Clero'),
  ('monastic','por','Monástico'),
  ('priestly','por','Sacerdotal')
) AS v(code, locale, text)
JOIN oikumenea.religion_grade_categories c ON c.code = v.code AND c.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('bishop','por','Bispo'),
  ('presbyter','por','Presbítero / sacerdote'),
  ('deacon','por','Diácono'),
  ('subdeacon','por','Subdiácono'),
  ('reader','por','Leitor'),
  ('mufti','por','Mufti'),
  ('imam','por','Imã'),
  ('sheikh','por','Xeique'),
  ('rabbi','por','Rabino'),
  ('cantor','por','Cantor (hazã)'),
  ('bhikkhu','por','Bhikkhu'),
  ('lama','por','Lama'),
  ('pujari','por','Pujari'),
  ('swami','por','Swami')
) AS v(code, locale, text)
JOIN oikumenea.religion_clergy_grades g ON g.code = v.code AND g.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_affiliation_type', a.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('adherent','por','Aderente'),
  ('member','por','Membro'),
  ('catechumen','por','Catecúmeno'),
  ('baptized','por','Batizado'),
  ('confirmed','por','Crismado'),
  ('shahada','por','Chahada'),
  ('bar_bat_mitzvah','por','Bar / Bat mitzvá')
) AS v(code, locale, text)
JOIN oikumenea.religion_affiliation_types a ON a.code = v.code AND a.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('office','por','Escritório'),
  ('online','por','Online'),
  ('mission','por','Missão'),
  ('shrine','por','Santuário'),
  ('church','por','Igreja'),
  ('cathedral','por','Catedral'),
  ('chapel','por','Capela'),
  ('monastery','por','Mosteiro'),
  ('mosque','por','Mesquita'),
  ('synagogue','por','Sinagoga'),
  ('temple','por','Templo'),
  ('gurdwara','por','Gurdwara')
) AS v(code, locale, text)
JOIN oikumenea.religion_site_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('main','por','Serviço principal'),
  ('youth','por','Serviço jovem'),
  ('prayer','por','Oração'),
  ('special','por','Serviço especial'),
  ('daily_mass','por','Missa diária'),
  ('jumua','por','Oração de sexta-feira (jumuʿah)'),
  ('shabbat','por','Serviço de Shabat'),
  ('puja','por','Puja'),
  ('meditation','por','Meditação')
) AS v(code, locale, text)
JOIN oikumenea.religion_service_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'domain', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('military','por','Militar'),
  ('government','por','Governo'),
  ('company','por','Empresa'),
  ('university','por','Universidade'),
  ('church','por','Igreja'),
  ('public-org','por','Organização pública')
) AS v(code, locale, text)
JOIN oikumenea.tenant_domains t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'unit_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('ministry-of-defence','por','Ministério da Defesa'),
  ('armed-forces','por','Forças Armadas'),
  ('service-branch','por','Ramo das forças'),
  ('command','por','Comando'),
  ('army-group','por','Grupo de Exércitos / Frente'),
  ('army','por','Exército de campanha'),
  ('corps','por','Corpo'),
  ('division','por','Divisão'),
  ('brigade','por','Brigada'),
  ('regiment','por','Regimento'),
  ('battalion','por','Batalhão'),
  ('company','por','Companhia'),
  ('platoon','por','Pelotão'),
  ('squad','por','Esquadra / Seção'),
  ('fire-team','por','Equipe de fogo / Guarnição'),
  ('wing','por','Ala'),
  ('air-group','por','Grupo (Aéreo)'),
  ('air-squadron','por','Esquadrão (Aéreo)'),
  ('flight','por','Esquadrilha'),
  ('fleet','por','Frota'),
  ('flotilla','por','Flotilha'),
  ('naval-squadron','por','Esquadrão (Naval)'),
  ('ministry','por','Ministério'),
  ('agency','por','Agência'),
  ('department','por','Departamento'),
  ('team','por','Equipe'),
  ('campus','por','Campus'),
  ('institute','por','Instituto'),
  ('faculty','por','Faculdade'),
  ('chair','por','Cátedra'),
  ('diocese','por','Diocese'),
  ('parish','por','Paróquia'),
  ('chapter','por','Capítulo')
) AS v(code, locale, text)
JOIN oikumenea.tenant_unit_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'relation_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('godparent','por','Padrinho/Madrinha'),
  ('academic_advisor','por','Orientador acadêmico'),
  ('military_mentor','por','Mentor militar'),
  ('spouse','por','Cônjuge'),
  ('parent','por','Progenitor'),
  ('child','por','Filho/a'),
  ('sibling','por','Irmão/ã'),
  ('next_of_kin_other','por','Outro (parente próximo)'),
  ('colleague','por','Colega'),
  ('business_associate','por','Sócio comercial'),
  ('emergency','por','Contato de emergência')
) AS v(code, locale, text)
JOIN oikumenea.person_relation_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'email_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('personal','por','Pessoal'),
  ('work','por','Trabalho'),
  ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.person_email_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'phone_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('mobile','por','Celular'),
  ('home','por','Residencial'),
  ('work','por','Trabalho'),
  ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.person_phone_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'personal_code_scheme', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('ua-rnokpp','por','Número de identificação fiscal individual (RNOKPP)'),
  ('ua-unzr','por','Número de registro único (UNZR)'),
  ('us-ssn','por','Número de Seguro Social'),
  ('de-steuer-id','por','Identificação fiscal (Steuer-ID)'),
  ('it-codice-fiscale','por','Código fiscal (Codice Fiscale)')
) AS v(code, locale, text)
JOIN oikumenea.document_personal_code_schemes t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'document_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('passport','por','Passaporte'),
  ('national-id','por','Documento nacional de identidade'),
  ('tax-id','por','Documento de identificação fiscal'),
  ('social-insurance','por','Cartão de seguro social'),
  ('driver-license','por','Carteira de habilitação'),
  ('military-id','por','Documento de identidade militar')
) AS v(code, locale, text)
JOIN oikumenea.document_document_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'location_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('building','por','Prédio'),
  ('address','por','Endereço'),
  ('online','por','On-line')
) AS v(code, locale, text)
JOIN oikumenea.location_location_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_institution_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('kindergarten','por','Jardim de infância'),
  ('school','por','Escola'),
  ('lyceum','por','Liceu'),
  ('gymnasium','por','Ginásio'),
  ('vocational','por','Escola profissionalizante'),
  ('college','por','Colégio'),
  ('institute','por','Instituto'),
  ('university','por','Universidade'),
  ('academy','por','Academia')
) AS v(code, locale, text)
JOIN oikumenea.education_institution_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_degree_level', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('isced-0','por','Educação infantil'),
  ('isced-1','por','Ensino fundamental (anos iniciais)'),
  ('isced-2','por','Ensino fundamental (anos finais)'),
  ('isced-3','por','Ensino médio'),
  ('isced-4','por','Educação pós-secundária não terciária'),
  ('isced-5','por','Educação superior de ciclo curto'),
  ('isced-6','por','Bacharelado ou equivalente'),
  ('isced-7','por','Mestrado ou equivalente'),
  ('isced-8','por','Doutorado ou equivalente')
) AS v(code, locale, text)
JOIN oikumenea.education_degree_levels t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_legal_form', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('llc','por','Sociedade de responsabilidade limitada'),
  ('jsc','por','Sociedade anônima'),
  ('plc','por','Sociedade anônima de capital aberto'),
  ('sole-proprietor','por','Empresário individual'),
  ('state-enterprise','por','Empresa estatal')
) AS v(code, locale, text)
JOIN oikumenea.company_legal_forms t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_registration_scheme', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('lei','por','Identificador de entidade jurídica (ISO 17442)'),
  ('duns','por','Número D-U-N-S (Dun & Bradstreet)'),
  ('ua-edrpou','por','Código EDRPOU (Ucrânia)'),
  ('vat','por','Número de IVA'),
  ('us-ein','por','Número de identificação do empregador (EUA)')
) AS v(code, locale, text)
JOIN oikumenea.company_registration_schemes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_industry_class', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('nace-a','por','Agricultura, silvicultura e pesca'),
  ('nace-c','por','Indústria de transformação'),
  ('nace-f','por','Construção'),
  ('nace-g','por','Comércio atacadista e varejista'),
  ('nace-j','por','Informação e comunicação'),
  ('nace-k','por','Atividades financeiras e de seguros'),
  ('nace-m','por','Atividades profissionais, científicas e técnicas'),
  ('nace-q','por','Saúde humana e serviços sociais')
) AS v(code, locale, text)
JOIN oikumenea.company_industry_classes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'external_org_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('party','por','Partido político'),
  ('government_body','por','Órgão governamental'),
  ('military','por','Formação militar'),
  ('ngo','por','Organização não governamental'),
  ('registrant','por','Registrante de lobby / cliente'),
  ('other','por','Outro')
) AS v(code, locale, text)
JOIN oikumenea.external_org_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'finance_account_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('current','por','Conta corrente'),
  ('savings','por','Conta poupança'),
  ('deposit','por','Conta de depósito'),
  ('loan','por','Conta de empréstimo'),
  ('card','por','Conta de cartão')
) AS v(code, locale, text)
JOIN oikumenea.finance_account_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('car','por','Automóvel'),
  ('truck','por','Caminhão'),
  ('motorcycle','por','Motocicleta'),
  ('bus','por','Ônibus'),
  ('trailer','por','Reboque'),
  ('special','por','Veículo especial')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_registration_number_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('regular','por','Comum'),
  ('temporary','por','Temporário'),
  ('transit','por','Trânsito'),
  ('diplomatic','por','Diplomático'),
  ('military','por','Militar'),
  ('old','por','Antigo / histórico')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_registration_number_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

UPDATE oikumenea.schema_version SET revision = '0015_locale_por', applied_at = now() WHERE singleton;
