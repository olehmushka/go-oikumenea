-- 0012_locale_ukr — self-contained ukr locale: registers the locale, links its languoid, and seeds
-- all ukr translation rows. Adding a new locale = drop in one more 00NN_locale_<code>.sql after this.
-- (Extracted from the former 0002/0009/0018/0023-0026/0045 migrations by the migration refactor.)

-- locale registration
INSERT INTO oikumenea.i18n_locales (code, name, enabled, is_default, sort_order) VALUES
  ('ukr', 'Українська', true, true,  0);

-- link this locale to its Glottolog languoid (was in 0018)
INSERT INTO oikumenea.i18n_locale_languages (locale, language_id)
SELECT 'ukr', x.id FROM oikumenea.language_languoids x WHERE x.iso639_3 = 'ukr' ON CONFLICT DO NOTHING;

-- ukr translations
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'personal_code_scheme', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('ua-rnokpp',         'ukr', 'РНОКПП'),
  ('ua-unzr',           'ukr', 'УНЗР'),
  ('us-ssn',            'ukr', 'Номер соціального страхування'),
  ('de-steuer-id',      'ukr', 'Податковий номер (Steuer-ID)'),
  ('it-codice-fiscale', 'ukr', 'Податковий код (Codice Fiscale)'),
  ('pl-pesel',          'ukr', 'PESEL')
) AS v(code, locale, text)
JOIN oikumenea.document_personal_code_schemes s ON s.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'writing_system', w.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('Latn','Латиниця'),
  ('Cyrl','Кирилиця'),
  ('Grek','Грецьке письмо'),
  ('Armn','Вірменське письмо'),
  ('Geor','Грузинське письмо'),
  ('Glag','Глаголиця'),
  ('Runr','Руни'),
  ('Ogam','Огам'),
  ('Goth','Готське письмо'),
  ('Nkoo','Н’Ко'),
  ('Adlm','Адлам'),
  ('Arab','Арабиця'),
  ('Hebr','Єврейське письмо'),
  ('Syrc','Сирійське письмо'),
  ('Samr','Самаритянське письмо'),
  ('Mand','Мандейське письмо'),
  ('Phnx','Фінікійське письмо'),
  ('Thaa','Тана'),
  ('Deva','Деванагарі'),
  ('Beng','Бенгальське письмо'),
  ('Guru','Гурмукхі'),
  ('Gujr','Гуджараті'),
  ('Orya','Орія'),
  ('Taml','Тамільське письмо'),
  ('Telu','Телугу'),
  ('Knda','Каннада'),
  ('Mlym','Малаялам'),
  ('Sinh','Сингальське письмо'),
  ('Thai','Тайське письмо'),
  ('Laoo','Лаоське письмо'),
  ('Tibt','Тибетське письмо'),
  ('Mymr','Бірманське письмо'),
  ('Khmr','Кхмерське письмо'),
  ('Ethi','Ефіопське письмо'),
  ('Cans','Канадське складове письмо'),
  ('Tfng','Тіфінаг'),
  ('Java','Яванське письмо'),
  ('Bali','Балійське письмо'),
  ('Hira','Хіраґана'),
  ('Kana','Катакана'),
  ('Bopo','Бопомофо'),
  ('Yiii','Письмо ї'),
  ('Cher','Черокі'),
  ('Vaii','Письмо ваї'),
  ('Hani','Ієрогліфи хань'),
  ('Hans','Спрощені ієрогліфи'),
  ('Hant','Традиційні ієрогліфи'),
  ('Jpan','Японське письмо'),
  ('Egyp','Єгипетські ієрогліфи'),
  ('Xsux','Клинопис'),
  ('Hang','Хангиль'),
  ('Kore','Корейське письмо')
) AS v(code, text)
JOIN oikumenea.writing_systems w ON w.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon_rank', r.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('religion','Релігія'),
  ('branch','Гілка'),
  ('tradition','Традиція'),
  ('sub_tradition','Піднапрям'),
  ('denomination','Деномінація')
) AS v(code, text)
JOIN oikumenea.religion_taxon_ranks r ON r.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_classification', c.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('monotheistic','Монотеїстична'),
  ('polytheistic','Політеїстична'),
  ('henotheistic','Генотеїстична'),
  ('monistic','Моністична'),
  ('nontheistic','Нетеїстична'),
  ('pantheistic','Пантеїстична'),
  ('panentheistic','Панентеїстична'),
  ('animistic','Анімістична'),
  ('dualistic','Дуалістична'),
  ('deistic','Деїстична'),
  ('agnostic','Агностична'),
  ('atheistic','Атеїстична')
) AS v(code, text)
JOIN oikumenea.religion_classifications c ON c.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_org_kind', k.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('denomination','Деномінація'),
  ('jurisdiction','Юрисдикція'),
  ('diocese','Єпархія'),
  ('deanery','Деканат'),
  ('parish','Парафія'),
  ('congregation','Конгрегація'),
  ('mission','Місія'),
  ('monastery','Монастир'),
  ('community','Спільнота'),
  ('mosque_community','Мусульманська громада'),
  ('temple_community','Храмова громада'),
  ('council','Рада / Асоціація')
) AS v(code, text)
JOIN oikumenea.religion_org_kinds k ON k.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_policy_kind', p.id::text, 'name', 'ukr', v.text
FROM (VALUES
  ('excludes_child_creation','Виключає створення дочірніх'),
  ('excluded_body','Виключений орган')
) AS v(code, text)
JOIN oikumenea.religion_policy_kinds p ON p.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_taxon', t.id::text, 'name', 'ukr', v.text
FROM (VALUES
  -- Religions (roots)
  ('christianity','Християнство'),
  ('islam','Іслам'),
  ('judaism','Юдаїзм'),
  ('hinduism','Індуїзм'),
  ('buddhism','Буддизм'),
  ('sikhism','Сикхізм'),
  ('jainism','Джайнізм'),
  ('bahai','Віра Багаї'),
  ('shinto','Синтоїзм'),
  ('taoism','Даосизм'),
  ('confucianism','Конфуціанство'),
  ('zoroastrianism','Зороастризм'),
  ('atheism','Атеїзм'),
  ('agnosticism','Агностицизм'),
  ('traditional','Традиційні та корінні релігії'),
  ('other','Інше / некласифіковане'),
  -- Branches
  ('catholicism','Католицтво'),
  ('eastern_orthodoxy','Православ’я'),
  ('oriental_orthodoxy','Давньосхідні церкви'),
  ('church_of_the_east','Церква Сходу'),
  ('protestantism','Протестантизм'),
  ('restorationism','Реставраціонізм'),
  ('independent_christianity','Незалежне християнство'),
  ('sunni','Сунізм'),
  ('shia','Шиїзм'),
  ('ibadi','Ібадизм'),
  ('sufism','Суфізм'),
  ('ahmadiyya','Ахмадія'),
  ('orthodox_judaism','Ортодоксальний юдаїзм'),
  ('conservative_judaism','Консервативний юдаїзм'),
  ('reform_judaism','Реформістський юдаїзм'),
  ('reconstructionist_judaism','Реконструктивістський юдаїзм'),
  ('karaite_judaism','Караїмський юдаїзм'),
  ('vaishnavism','Вайшнавізм'),
  ('shaivism','Шиваїзм'),
  ('shaktism','Шактизм'),
  ('smartism','Смартизм'),
  ('theravada','Тхеравада'),
  ('mahayana','Махаяна'),
  ('vajrayana','Ваджраяна'),
  ('digambara','Дигамбара'),
  ('svetambara','Шветамбара'),
  -- Traditions
  ('latin_church','Латинська церква'),
  ('eastern_catholic','Східнокатолицькі церкви'),
  ('lutheranism','Лютеранство'),
  ('reformed','Реформатство (кальвінізм)'),
  ('anglicanism','Англіканство'),
  ('anabaptism','Анабаптизм'),
  ('baptist','Баптизм'),
  ('methodism','Методизм'),
  ('pentecostalism','П’ятдесятництво'),
  ('adventism','Адвентизм'),
  ('holiness','Рух святості'),
  ('evangelicalism','Євангелізм'),
  ('quakerism','Квакерство'),
  -- Sub-traditions
  ('hanafi','Ханафітський мазгаб'),
  ('maliki','Малікітський мазгаб'),
  ('shafii','Шафіїтський мазгаб'),
  ('hanbali','Ханбалітський мазгаб'),
  ('twelver','Дванадесятники'),
  ('ismailism','Ісмаїлізм'),
  ('zaidiyyah','Зейдизм'),
  ('hasidic','Хасидизм'),
  ('modern_orthodox','Сучасний ортодоксальний юдаїзм'),
  ('haredi','Харедим'),
  ('presbyterianism','Пресвітеріанство'),
  ('congregationalism','Конгрегаціоналізм'),
  ('continental_reformed','Континентальне реформатство'),
  -- Denominations
  ('ecumenical_patriarchate','Вселенський патріархат Константинополя'),
  ('church_of_greece','Елладська православна церква'),
  ('russian_orthodox_church','Російська православна церква'),
  ('serbian_orthodox_church','Сербська православна церква'),
  ('romanian_orthodox_church','Румунська православна церква'),
  ('bulgarian_orthodox_church','Болгарська православна церква'),
  ('georgian_orthodox_church','Грузинська православна церква'),
  ('orthodox_church_of_ukraine','Православна церква України'),
  ('orthodox_church_in_america','Православна церква в Америці'),
  ('coptic_orthodox_church','Коптська православна церква'),
  ('armenian_apostolic_church','Вірменська апостольська церква'),
  ('ethiopian_orthodox_tewahedo','Ефіопська православна церква Тевахедо'),
  ('syriac_orthodox_church','Сирійська православна церква'),
  ('malankara_orthodox_church','Маланкарська православна сирійська церква'),
  ('assyrian_church_of_the_east','Ассирійська церква Сходу'),
  ('ancient_church_of_the_east','Давня церква Сходу'),
  ('ukrainian_greek_catholic_church','Українська греко-католицька церква'),
  ('maronite_church','Маронітська церква'),
  ('melkite_greek_catholic_church','Мелькітська греко-католицька церква'),
  ('chaldean_catholic_church','Халдейська католицька церква'),
  ('syro_malabar_church','Сиро-малабарська церква'),
  ('armenian_catholic_church','Вірменська католицька церква'),
  ('elca','Євангелічно-лютеранська церква в Америці'),
  ('lcms','Лютеранська церква — Міссурійський синод'),
  ('southern_baptist_convention','Південна баптистська конвенція'),
  ('united_methodist_church','Об’єднана методистська церква'),
  ('church_of_england','Церква Англії'),
  ('episcopal_church_usa','Єпископальна церква (США)'),
  ('assemblies_of_god','Асамблеї Бога'),
  ('seventh_day_adventist_church','Церква адвентистів сьомого дня'),
  ('lds_church','Церква Ісуса Христа Святих останніх днів'),
  ('jehovahs_witnesses','Свідки Єгови')
) AS v(code, text)
JOIN oikumenea.religion_taxa t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_grade_category', c.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('major_orders','ukr','Вищі свячення'),
  ('minor_orders','ukr','Нижчі свячення'),
  ('religious_leadership','ukr','Релігійне провідництво'),
  ('clergy','ukr','Духовенство'),
  ('monastic','ukr','Чернецтво'),
  ('priestly','ukr','Жрецтво')
) AS v(code, locale, text)
JOIN oikumenea.religion_grade_categories c ON c.code = v.code AND c.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_clergy_grade', g.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('bishop','ukr','Єпископ'),
  ('presbyter','ukr','Пресвітер / священник'),
  ('deacon','ukr','Диякон'),
  ('subdeacon','ukr','Іподиякон'),
  ('reader','ukr','Читець'),
  ('mufti','ukr','Муфтій'),
  ('imam','ukr','Імам'),
  ('sheikh','ukr','Шейх'),
  ('rabbi','ukr','Рабин'),
  ('cantor','ukr','Кантор (хаззан)'),
  ('bhikkhu','ukr','Бгіккху'),
  ('lama','ukr','Лама'),
  ('pujari','ukr','Пуджарі'),
  ('swami','ukr','Свамі')
) AS v(code, locale, text)
JOIN oikumenea.religion_clergy_grades g ON g.code = v.code AND g.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_affiliation_type', a.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('adherent','ukr','Прихильник'),
  ('member','ukr','Член'),
  ('catechumen','ukr','Катехумен'),
  ('baptized','ukr','Хрещений'),
  ('confirmed','ukr','Конфірмований'),
  ('shahada','ukr','Шахада'),
  ('bar_bat_mitzvah','ukr','Бар / Бат-міцва')
) AS v(code, locale, text)
JOIN oikumenea.religion_affiliation_types a ON a.code = v.code AND a.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_site_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('office','ukr','Офіс'),
  ('online','ukr','Онлайн'),
  ('mission','ukr','Місія'),
  ('shrine','ukr','Святиня'),
  ('church','ukr','Церква'),
  ('cathedral','ukr','Собор'),
  ('chapel','ukr','Каплиця'),
  ('monastery','ukr','Монастир'),
  ('mosque','ukr','Мечеть'),
  ('synagogue','ukr','Синагога'),
  ('temple','ukr','Храм'),
  ('gurdwara','ukr','Гурдвара')
) AS v(code, locale, text)
JOIN oikumenea.religion_site_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'religion_service_type', s.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('main','ukr','Головне богослужіння'),
  ('youth','ukr','Молодіжне богослужіння'),
  ('prayer','ukr','Молитва'),
  ('special','ukr','Особливе богослужіння'),
  ('daily_mass','ukr','Щоденна меса'),
  ('jumua','ukr','П’ятнична молитва (джума)'),
  ('shabbat','ukr','Шабатнє богослужіння'),
  ('puja','ukr','Пуджа'),
  ('meditation','ukr','Медитація')
) AS v(code, locale, text)
JOIN oikumenea.religion_service_types s ON s.code = v.code AND s.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'domain', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('military','ukr','Військовий'),
  ('government','ukr','Державний'),
  ('company','ukr','Компанія'),
  ('university','ukr','Університет'),
  ('church','ukr','Церква'),
  ('public-org','ukr','Громадська організація')
) AS v(code, locale, text)
JOIN oikumenea.tenant_domains t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'unit_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('ministry-of-defence','ukr','Міністерство оборони'),
  ('armed-forces','ukr','Збройні сили'),
  ('service-branch','ukr','Вид збройних сил'),
  ('command','ukr','Командування'),
  ('army-group','ukr','Група армій / Фронт'),
  ('army','ukr','Польова армія'),
  ('corps','ukr','Корпус'),
  ('division','ukr','Дивізія'),
  ('brigade','ukr','Бригада'),
  ('regiment','ukr','Полк'),
  ('battalion','ukr','Батальйон'),
  ('company','ukr','Рота'),
  ('platoon','ukr','Взвод'),
  ('squad','ukr','Відділення'),
  ('fire-team','ukr','Бойова група / Обслуга'),
  ('wing','ukr','Авіакрило'),
  ('air-group','ukr','Авіагрупа'),
  ('air-squadron','ukr','Авіаескадрилья'),
  ('flight','ukr','Ланка'),
  ('fleet','ukr','Флот'),
  ('flotilla','ukr','Флотилія'),
  ('naval-squadron','ukr','Ескадра (ВМС)'),
  ('ministry','ukr','Міністерство'),
  ('agency','ukr','Агентство'),
  ('department','ukr','Департамент'),
  ('team','ukr','Команда'),
  ('campus','ukr','Кампус'),
  ('institute','ukr','Інститут'),
  ('faculty','ukr','Факультет'),
  ('chair','ukr','Кафедра'),
  ('diocese','ukr','Єпархія'),
  ('parish','ukr','Парафія'),
  ('chapter','ukr','Осередок')
) AS v(code, locale, text)
JOIN oikumenea.tenant_unit_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'relation_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('godparent','ukr','Хрещений батько/мати'),
  ('academic_advisor','ukr','Науковий керівник'),
  ('military_mentor','ukr','Військовий наставник'),
  ('spouse','ukr','Подружжя'),
  ('parent','ukr','Батько/мати'),
  ('child','ukr','Дитина'),
  ('sibling','ukr','Брат/сестра'),
  ('next_of_kin_other','ukr','Інше (найближчі родичі)'),
  ('colleague','ukr','Колега'),
  ('business_associate','ukr','Діловий партнер'),
  ('emergency','ukr','Екстрений контакт')
) AS v(code, locale, text)
JOIN oikumenea.person_relation_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'email_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('personal','ukr','Особистий'),
  ('work','ukr','Робочий'),
  ('other','ukr','Інший')
) AS v(code, locale, text)
JOIN oikumenea.person_email_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'phone_type', v.code, 'name', v.locale, v.text
FROM (VALUES
  ('mobile','ukr','Мобільний'),
  ('home','ukr','Домашній'),
  ('work','ukr','Робочий'),
  ('other','ukr','Інший')
) AS v(code, locale, text)
JOIN oikumenea.person_phone_types t ON t.code = v.code
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'document_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('passport','ukr','Паспорт'),
  ('national-id','ukr','Посвідчення особи'),
  ('tax-id','ukr','Податковий документ'),
  ('social-insurance','ukr','Картка соціального страхування'),
  ('driver-license','ukr','Посвідчення водія'),
  ('military-id','ukr','Військовий квиток'),
  ('diploma','ukr','Диплом')
) AS v(code, locale, text)
JOIN oikumenea.document_document_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'location_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('building','ukr','Будівля'),
  ('address','ukr','Адреса'),
  ('online','ukr','Онлайн')
) AS v(code, locale, text)
JOIN oikumenea.location_location_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_institution_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('kindergarten','ukr','Дитячий садок'),
  ('school','ukr','Школа'),
  ('lyceum','ukr','Ліцей'),
  ('gymnasium','ukr','Гімназія'),
  ('vocational','ukr','Професійно-технічний заклад'),
  ('college','ukr','Коледж'),
  ('institute','ukr','Інститут'),
  ('university','ukr','Університет'),
  ('academy','ukr','Академія')
) AS v(code, locale, text)
JOIN oikumenea.education_institution_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'education_degree_level', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('isced-0','ukr','Дошкільна освіта'),
  ('isced-1','ukr','Початкова освіта'),
  ('isced-2','ukr','Базова середня освіта'),
  ('isced-3','ukr','Повна середня освіта'),
  ('isced-4','ukr','Післясередня нетретинна освіта'),
  ('isced-5','ukr','Короткий цикл вищої освіти'),
  ('isced-6','ukr','Бакалавр або еквівалент'),
  ('isced-7','ukr','Магістр або еквівалент'),
  ('isced-8','ukr','Доктор філософії або еквівалент')
) AS v(code, locale, text)
JOIN oikumenea.education_degree_levels t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_legal_form', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('llc','ukr','Товариство з обмеженою відповідальністю'),
  ('jsc','ukr','Акціонерне товариство'),
  ('plc','ukr','Публічне акціонерне товариство'),
  ('sole-proprietor','ukr','Фізична особа-підприємець'),
  ('state-enterprise','ukr','Державне підприємство')
) AS v(code, locale, text)
JOIN oikumenea.company_legal_forms t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_registration_scheme', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('lei','ukr','Ідентифікатор юридичної особи (ISO 17442)'),
  ('duns','ukr','Номер D-U-N-S (Dun & Bradstreet)'),
  ('ua-edrpou','ukr','Код ЄДРПОУ'),
  ('vat','ukr','Номер платника ПДВ'),
  ('us-ein','ukr','Ідентифікаційний номер роботодавця (США)')
) AS v(code, locale, text)
JOIN oikumenea.company_registration_schemes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'company_industry_class', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('nace-a','ukr','Сільське, лісове та рибне господарство'),
  ('nace-c','ukr','Переробна промисловість'),
  ('nace-f','ukr','Будівництво'),
  ('nace-g','ukr','Оптова та роздрібна торгівля'),
  ('nace-j','ukr','Інформація та телекомунікації'),
  ('nace-k','ukr','Фінансова та страхова діяльність'),
  ('nace-m','ukr','Професійна, наукова та технічна діяльність'),
  ('nace-q','ukr','Охорона здоровʼя та соціальна допомога')
) AS v(code, locale, text)
JOIN oikumenea.company_industry_classes t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'external_org_kind', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('party','ukr','Політична партія'),
  ('government_body','ukr','Державний орган'),
  ('military','ukr','Військове формування'),
  ('ngo','ukr','Неурядова організація'),
  ('registrant','ukr','Лобіст / клієнт'),
  ('other','ukr','Інше')
) AS v(code, locale, text)
JOIN oikumenea.external_org_kinds t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'finance_account_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('current','ukr','Поточний рахунок'),
  ('savings','ukr','Ощадний рахунок'),
  ('deposit','ukr','Депозитний рахунок'),
  ('loan','ukr','Кредитний рахунок'),
  ('card','ukr','Картковий рахунок')
) AS v(code, locale, text)
JOIN oikumenea.finance_account_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('car','ukr','Легковий автомобіль'),
  ('truck','ukr','Вантажівка'),
  ('motorcycle','ukr','Мотоцикл'),
  ('bus','ukr','Автобус'),
  ('trailer','ukr','Причіп'),
  ('special','ukr','Спеціальний транспорт')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;
INSERT INTO oikumenea.i18n_translations (entity_type, entity_id, field, locale, text)
SELECT 'vehicle_registration_number_type', t.id::text, 'name', v.locale, v.text
FROM (VALUES
  ('regular','ukr','Звичайний'),
  ('temporary','ukr','Тимчасовий'),
  ('transit','ukr','Транзитний'),
  ('diplomatic','ukr','Дипломатичний'),
  ('military','ukr','Військовий'),
  ('old','ukr','Старий / історичний')
) AS v(code, locale, text)
JOIN oikumenea.vehicle_registration_number_types t ON t.code = v.code AND t.deleted_at IS NULL
ON CONFLICT (entity_type, entity_id, field, locale) DO NOTHING;

UPDATE oikumenea.schema_version SET revision = '0012_locale_ukr', applied_at = now() WHERE singleton;
