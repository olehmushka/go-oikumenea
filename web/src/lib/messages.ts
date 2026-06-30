/**
 * Static UI-chrome message catalog (D-i18n). Dynamic, data-borne labels come from the API as
 * `locale → text` maps (see i18n.ts / pickLabel); this catalog covers the *interface* strings that
 * are not data — navigation, section headings, and common chrome/actions — so they translate when the
 * UI locale switches.
 *
 * Keys are stable, dot-namespaced strings. `t(key)` resolves against the module-global active UI
 * locale (set by the root layout on the server and LocaleProvider on the client); pass an explicit
 * locale to override. Missing keys fall back to English, then to the key itself, so a partial catalog
 * degrades gracefully. Server components call `t(key, getActiveLocale())`; client components use the
 * `useT()` hook (re-renders on switch).
 */

import { getActiveLocale, type UiLocale } from "./i18n";

type Catalog = Record<string, string>;

const eng: Catalog = {
  // app shell
  "app.console": "admin console",
  "nav.signOut": "Sign out",
  "nav.search": "Search…",
  // nav sections
  "nav.section.explore": "Explore",
  "nav.section.tools": "Tools",
  "nav.overview": "Overview",
  "nav.overview.hint": "whoami & recents",
  // nav tools
  "nav.ontology": "Ontology",
  "nav.ontology.hint": "type registry",
  "nav.authorize": "Authorize",
  "nav.authorize.hint": "PDP check",
  "nav.roles": "Roles & access",
  "nav.roles.hint": "RBAC + assignments",
  "nav.organizations": "Organizations",
  "nav.organizations.hint": "realms (per domain)",
  "nav.graphs": "Graph admin",
  "nav.graphs.hint": "hierarchies + closure",
  "nav.memberships": "Memberships",
  "nav.memberships.hint": "person ↔ unit",
  "nav.orders": "Orders",
  "nav.orders.hint": "наказ",
  "nav.documents": "Documents",
  "nav.documents.hint": "papers & catalogs",
  "nav.ranks": "Ranks",
  "nav.ranks.hint": "rank scheme",
  "nav.locations": "Locations",
  "nav.locations.hint": "places + radius search",
  "nav.education": "Education",
  "nav.education.hint": "institutions + structure",
  "nav.companies": "Companies",
  "nav.companies.hint": "legal entities + ownership",
  "nav.vehicles": "Vehicles",
  "nav.vehicles.hint": "vehicle registry + ownership",
  "nav.externalOrgs": "External orgs",
  "nav.externalOrgs.hint": "parties, government, NGOs",
  "nav.religion": "Religion",
  "nav.religion.hint": "faith taxonomy + organization",
  "nav.localization": "Localization",
  "nav.localization.hint": "locales",
  "nav.legalBasis": "Legal basis",
  "nav.legalBasis.hint": "GDPR lawful-basis catalog",
  "nav.imports": "Imports",
  "nav.imports.hint": "hermenea ingestion",
  "nav.audit": "Audit",
  "nav.audit.hint": "log",
};

const ukr: Catalog = {
  // app shell
  "app.console": "адмінконсоль",
  "nav.signOut": "Вийти",
  "nav.search": "Пошук…",
  // nav sections
  "nav.section.explore": "Огляд",
  "nav.section.tools": "Інструменти",
  "nav.overview": "Огляд",
  "nav.overview.hint": "хто я та недавні",
  // nav tools
  "nav.ontology": "Онтологія",
  "nav.ontology.hint": "реєстр типів",
  "nav.authorize": "Авторизація",
  "nav.authorize.hint": "перевірка PDP",
  "nav.roles": "Ролі та доступ",
  "nav.roles.hint": "RBAC + призначення",
  "nav.organizations": "Організації",
  "nav.organizations.hint": "реалми (за доменом)",
  "nav.graphs": "Адмін графів",
  "nav.graphs.hint": "ієрархії + замикання",
  "nav.memberships": "Членства",
  "nav.memberships.hint": "особа ↔ підрозділ",
  "nav.orders": "Накази",
  "nav.orders.hint": "наказ",
  "nav.documents": "Документи",
  "nav.documents.hint": "папери та каталоги",
  "nav.ranks": "Звання",
  "nav.ranks.hint": "схема звань",
  "nav.locations": "Локації",
  "nav.locations.hint": "місця + пошук за радіусом",
  "nav.education": "Освіта",
  "nav.education.hint": "заклади + структура",
  "nav.companies": "Компанії",
  "nav.companies.hint": "юридичні особи + власність",
  "nav.vehicles": "Транспорт",
  "nav.vehicles.hint": "реєстр транспорту + власність",
  "nav.externalOrgs": "Зовнішні організації",
  "nav.externalOrgs.hint": "партії, державні органи, НУО",
  "nav.religion": "Релігія",
  "nav.religion.hint": "таксономія віри + організація",
  "nav.localization": "Локалізація",
  "nav.localization.hint": "локалі",
  "nav.legalBasis": "Правова підстава",
  "nav.legalBasis.hint": "каталог підстав GDPR",
  "nav.imports": "Імпорти",
  "nav.imports.hint": "завантаження hermenea",
  "nav.audit": "Аудит",
  "nav.audit.hint": "журнал",
};

const CATALOGS: Record<string, Catalog> = { eng, ukr };

/**
 * Translate a static-chrome key. Resolves against the active UI locale by default (override with
 * `locale`), falling back to English then the key itself.
 */
export function t(key: string, locale: string = getActiveLocale()): string {
  return CATALOGS[locale]?.[key] ?? eng[key] ?? key;
}

/**
 * Glossary keyed by the *English source string* (not a key). It covers the ontology registry's static
 * vocabulary — type labels (label/labelPlural), table column headers, property/link/action labels —
 * plus the repeated workspace chrome (Detail, Properties, Filter…, etc.). The render points pass the
 * English text through `tg()`, which returns the active-locale translation or the English text
 * unchanged (graceful fallback for anything not yet translated). Only the non-English locales need an
 * entry; English is the identity. This keeps the registry itself pure/locale-agnostic (D-Ontology).
 */
const glossaryUkr: Record<string, string> = {
  // ── registry type labels (singular / plural) ──
  "Person": "Особа", "Persons": "Особи",
  "Unit": "Підрозділ", "Units": "Підрозділи",
  "Order": "Наказ", "Orders": "Накази",
  "Order type": "Тип наказу", "Order types": "Типи наказів",
  "Role": "Роль", "Roles": "Ролі",
  "Assignment": "Призначення ролі", "Assignments": "Призначення ролей",
  "Graph": "Граф", "Graphs": "Графи",
  "Document": "Документ", "Documents": "Документи",
  "Document type": "Тип документа", "Document types": "Типи документів",
  "Personal-code scheme": "Схема особистого коду", "Personal-code schemes": "Схеми особистих кодів",
  "Locale": "Локаль", "Locales": "Локалі",
  "Position": "Посада", "Positions": "Посади",
  "Social account": "Обліковий запис у соцмережі", "Social accounts": "Облікові записи в соцмережах",
  "Messenger link": "Зв’язок у месенджері", "Messenger links": "Зв’язки в месенджерах",
  "Platform": "Платформа", "Platforms": "Платформи",
  "Relation type": "Тип відносин", "Relation types": "Типи відносин",
  "Rank": "Звання", "Ranks": "Звання",
  "Rank system": "Система звань", "Rank systems": "Системи звань",
  "Language": "Мова", "Languages": "Мови",
  "Writing system": "Система письма", "Writing systems": "Системи письма",
  "Location": "Локація", "Locations": "Локації",
  "Location type": "Тип локації", "Location types": "Типи локацій",
  "Institution": "Заклад", "Institutions": "Заклади",
  "Education unit": "Освітній підрозділ", "Education units": "Освітні підрозділи",
  "Building": "Будівля", "Buildings": "Будівлі",
  "Study group": "Навчальна група", "Study groups": "Навчальні групи",
  "Group": "Група", "Groups": "Групи",
  "Education position": "Освітня посада", "Education positions": "Освітні посади",
  "Program": "Програма", "Programs": "Програми",
  "Course": "Курс", "Courses": "Курси",
  "Curriculum version": "Версія навчального плану", "Curriculum versions": "Версії навчального плану",
  "Research centre": "Дослідний центр", "Research centres": "Дослідні центри",
  "Research group": "Дослідна група", "Research groups": "Дослідні групи",
  "Grant": "Грант", "Grants": "Гранти",
  "Publication": "Публікація", "Publications": "Публікації",
  "Governance body": "Орган управління", "Governance bodies": "Органи управління",
  "Policy": "Політика", "Policies": "Політики",
  "Qualification": "Кваліфікація", "Qualifications": "Кваліфікації",
  "Scholarship": "Стипендія", "Scholarships": "Стипендії",
  "Accreditation event": "Подія акредитації", "Accreditation events": "Події акредитації",
  // ── column headers / property labels ──
  "Code": "Код", "Name": "Назва", "Status": "Статус", "Kind": "Вид", "Type": "Тип",
  "Title": "Назва", "Description": "Опис", "Category": "Категорія", "Level": "Рівень",
  "State": "Стан", "Visibility": "Видимість", "Country": "Країна", "Number": "Номер",
  "Sex": "Стать", "Birthdate": "Дата народження", "Display name": "Відображуване ім’я",
  "Given": "Ім’я", "Surname": "Прізвище", "Patronymic": "По батькові",
  "Country of birth": "Країна народження", "Abbr": "Скор.", "Abbreviation": "Скорочення",
  "Grade": "Ступінь", "Grade (STANAG)": "Ступінь (STANAG)", "Permissions": "Дозволи",
  "Base": "Базова", "Base role": "Базова роль", "Scope": "Область", "Subject": "Суб’єкт",
  "Subject person": "Суб’єкт (особа)", "Target": "Ціль", "Target unit": "Цільовий підрозділ",
  "Granted at": "Надано", "Expires at": "Спливає", "Revoked at": "Відкликано",
  "Issued on": "Видано", "Issuer": "Видавець", "Issuing country": "Країна видачі",
  "Issuing unit": "Підрозділ видачі", "Items": "Пункти", "Expires on": "Дійсний до",
  "Default": "За замовчуванням", "Enabled": "Увімкнено", "Directory-only": "Лише довідник",
  "Handle": "Нік", "Confidence": "Впевненість", "Source": "Джерело", "Verified": "Підтверджено",
  "Verified at": "Підтверджено о", "Platform-verified": "Підтверджено платформою",
  "Profile URL": "URL профілю", "Channel": "Канал", "Phone": "Телефон", "Email": "Ел. пошта",
  "Primary": "Основний", "Glottocode": "Глоттокод", "ISO 639-3": "ISO 639-3",
  "Family": "Родина", "Macroarea": "Макрорегіон", "Script type": "Тип письма",
  "MGRS": "MGRS", "Lat": "Шир.", "Lng": "Довг.", "Latitude": "Широта", "Longitude": "Довгота",
  "Locality": "Населений пункт", "Street": "Вулиця", "Admin area": "Адмін. одиниця",
  "Postal code": "Поштовий індекс", "Raw address": "Адреса (як є)", "Source format": "Формат джерела",
  "Holder": "Власник", "Parent": "Батьківський", "Mode": "Режим", "Delivery": "Формат",
  "Delivery mode": "Формат навчання", "Credit hours": "Кредитні години",
  "Duration (years)": "Тривалість (років)", "Founded": "Засновано", "Closed": "Закрито",
  "Admission year": "Рік вступу", "Year": "Рік", "Focus area": "Сфера досліджень",
  "Funder": "Фінансувальник", "Funding source": "Джерело фінансування", "Amount": "Сума",
  "Frequency": "Періодичність", "DOI": "DOI", "Venue": "Видання", "Published": "Опубліковано",
  "Mandate": "Мандат", "Body": "Орган", "Outcome": "Результат", "Reviewed": "Розглянуто",
  "Effective": "Чинний", "Effective from": "Чинний від", "Framework": "Рамка",
  "Framework level": "Рівень рамки", "Awarding body": "Орган присвоєння", "Version": "Версія",
  "Prerequisites": "Передумови", "Issuing": "Видача",
  // ── link group labels ──
  "Memberships": "Членства", "Members": "Учасники", "Parents (ancestors)": "Батьківські (предки)",
  "Children (descendants)": "Дочірні (нащадки)", "Partnerships": "Партнерства", "Kin": "Родичі",
  "Guardianships": "Опікунства", "Sponsorships": "Спонсорства", "Next of kin": "Найближчі родичі",
  "Associations": "Зв’язки", "Education": "Освіта", "Dormitory stays": "Проживання в гуртожитку",
  "Appointments": "Призначення", "Research memberships": "Членства в дослідних групах",
  "Grant holdings": "Гранти", "Governance memberships": "Членства в органах управління",
  "Publication authorships": "Авторства публікацій", "Qualification awards": "Присвоєння кваліфікацій",
  "Scholarship awards": "Присудження стипендій",
  // ── action labels ──
  "Deactivate": "Деактивувати", "Reactivate": "Реактивувати", "Issue": "Видати", "Revoke": "Відкликати",
  // ── workspace chrome ──
  "Detail": "Деталі", "Properties": "Властивості", "Links": "Зв’язки", "No links.": "Немає зв’язків.",
  "Loading…": "Завантаження…", "Full view →": "Повний перегляд →",
  "Open in graph →": "Відкрити у графі →", "No matching rows.": "Немає відповідних рядків.",
  "Filter": "Фільтр", "of": "з", "selected": "вибрано", "Clear": "Очистити", "New": "Створити",
  "All": "Усі", "Indexing…": "Індексація…", "Type to search…": "Почніть вводити…",
  "No matches.": "Немає збігів.", "Open": "Відкрити", "Navigate": "Навігація", "Actions": "Дії",
  "Objects": "Об’єкти", "Nothing here yet.": "Поки що порожньо.",
  // command-palette specifics
  "Overview": "Огляд", "Ontology": "Онтологія", "Authorize": "Авторизація",
  "Localization": "Локалізація", "Audit": "Аудит",
  "Search objects, jump to a view, or paste a RID…": "Шукайте об’єкти, переходьте до розділів або вставте RID…",
  "New person": "Нова особа", "New unit": "Новий підрозділ", "Authorize check": "Перевірка доступу",
  "Ontology browser": "Браузер онтології", "Rebuild unit closure": "Перебудувати замикання підрозділів",
  "create": "створити", "PDP": "PDP", "types": "типи", "tenant": "організація",
  // ontology browser + overview page chrome
  "Browse →": "Перегляд →",
  "Every entity is a typed Object or reified Link, keyed by a self-describing RID. This is the registry that powers search, the explorer, and link traversal.":
    "Кожна сутність — це типізований Об’єкт або реіфікований Зв’язок, ідентифікований самоописовим RID. Це реєстр, що живить пошук, провідник і навігацію зв’язками.",
  "Press ⌘K anywhere to search objects, jump to a view, or paste a RID.":
    "Натисніть ⌘K будь-де, щоб шукати об’єкти, переходити до розділів або вставити RID.",
  "Signed in as": "Ви увійшли як", "Account": "Обліковий запис", "Service": "Сервіс",
  "Schema": "Схема", "Workspace": "Робоча область", "Jump to": "Перейти до",
  "Authentication is delegated to Keycloak; the service resolved this token to the person above and decides authorization per request (the PDP).":
    "Автентифікація делегована Keycloak; сервіс зіставив цей токен із зазначеною особою та ухвалює рішення про авторизацію для кожного запиту (PDP).",
  "The personnel directory": "Довідник персоналу", "Browse the unit DAG": "Огляд DAG підрозділів",
  "RBAC roles": "Ролі RBAC", "The type registry": "Реєстр типів",
  "Run a PDP decision": "Виконати рішення PDP", "Permission-sensitive log": "Журнал чутливих до дозволів дій",
  // authorize page
  "Ask the PDP a single question: may this person perform this action on this unit? This is the same decision the service makes for every API request.":
    "Поставте PDP одне запитання: чи може ця особа виконати цю дію над цим підрозділом? Це те саме рішення, яке сервіс ухвалює для кожного запиту до API.",
  "Subject person *": "Суб’єкт (особа) *", "Action (permission code) *": "Дія (код дозволу) *",
  "Search a person…": "Пошук особи…", "(optional) search a unit…": "(необов’язково) пошук підрозділу…",
  "Deciding…": "Обчислення…", "Run decision": "Виконати рішення",
  "ALLOW": "ДОЗВОЛЕНО", "DENY": "ЗАБОРОНЕНО",
  "Granted via the instance-admin plane.": "Надано через площину адміністратора екземпляра.",
  "Contributing assignments": "Призначення, що вплинули",
  // roles & access page
  "Roles & access": "Ролі та доступ",
  "RBAC: code-defined permissions packaged into roles, then granted as scoped assignments. Authority comes only from assignments — never rank or position.":
    "RBAC: визначені в коді дозволи, об’єднані в ролі та надані як обмежені призначення. Повноваження походять лише від призначень — ніколи від звання чи посади.",
  "base": "базова", "No roles.": "Немає ролей.", "Remove": "Видалити", "Go": "Перейти",
  // ── shared form vocabulary ──
  "Cancel": "Скасувати", "Save": "Зберегти", "Saving…": "Збереження…", "Creating…": "Створення…",
  "Create": "Створити", "Add": "Додати", "Adding…": "Додавання…", "Edit": "Редагувати",
  "Update": "Оновити", "Updating…": "Оновлення…", "Close": "Закрити", "Back": "Назад", "Next page →": "Наступна сторінка →",
  "Code *": "Код *", "Name *": "Назва *", "Title *": "Назва *",
  // person create
  "Create person": "Створити особу",
  "Create a directory entry. A login account is optional and attached later.":
    "Створіть запис у довіднику. Обліковий запис для входу необов’язковий і додається пізніше.",
  "Display name *": "Відображуване ім’я *", "Ivan Petrenko": "Іван Петренко", "auto from name": "авто з імені",
  "Date of death": "Дата смерті", "Sex (ISO 5218)": "Стать (ISO 5218)",
  "0 — not known": "0 — невідомо", "1 — male": "1 — чоловіча", "2 — female": "2 — жіноча",
  "9 — not applicable": "9 — не застосовно",
  // unit create
  "Create unit": "Створити підрозділ",
  "Create a unit. Optionally pick a parent to nest it under (you can also manage edges later from the unit's detail page).":
    "Створіть підрозділ. За бажанням оберіть батьківський, щоб вкласти його (зв’язки можна керувати пізніше зі сторінки підрозділу).",
  "Stable, locale-agnostic identifier.": "Стабільний, незалежний від локалі ідентифікатор.",
  "Headquarters": "Штаб", "command / department / faculty": "командування / відділ / факультет",
  "Parent unit (optional)": "Батьківський підрозділ (необов’язково)", "Search a parent…": "Пошук батьківського…",
  "Nests this unit under the chosen parent.": "Вкладає цей підрозділ під обраний батьківський.",
  // ranks
  "Rank scheme": "Схема звань",
  "The system-wide rank scheme: system → category → type → rank (directory seniority only — rank is never authority). NATO STANAG-2116 grade codes give cross-system equivalence. Add, edit, delete, and import below.":
    "Загальносистемна схема звань: система → категорія → тип → звання (лише старшинство в довіднику — звання ніколи не є повноваженням). Коди ступенів NATO STANAG-2116 дають міжсистемну відповідність. Додавайте, редагуйте, видаляйте та імпортуйте нижче.",
  "The rank scheme is empty — add a system below, or import a preset.":
    "Схема звань порожня — додайте систему нижче або імпортуйте пресет.",
  // rank scheme manager (inline field placeholders + buttons)
  "name": "назва", "code": "код", "priority": "пріоритет", "abbr": "скор.", "grade": "ступінь",
  "grade…": "ступінь…", "country": "країна", "country (opt.)": "країна (необов.)",
  "Add system": "Додати систему", "Add category": "Додати категорію", "Add type": "Додати тип",
  "Add sub-type": "Додати підтип", "Add rank": "Додати звання",
  "New sub-type:": "Новий підтип:", "New rank:": "Нове звання:", "New type:": "Новий тип:",
  "New category:": "Нова категорія:", "supranational": "наднаціональна",
  "Import preset…": "Імпортувати пресет…", "Import rank-system preset": "Імпорт пресету системи звань",
  "Importing…": "Імпортування…", "Import": "Імпортувати",
  "Paste a preset of shape": "Вставте пресет у форматі",
  "Import is idempotent: existing codes are updated, new ones created.":
    "Імпорт ідемпотентний: наявні коди оновлюються, нові — створюються.",
  "Imported — created": "Імпортовано — створено", "updated": "оновлено", "skipped": "пропущено",
  // memberships
  "person ↔ unit belonging and the positions they hold. Look up by unit or by person.":
    "належність особа ↔ підрозділ та посади, які вони обіймають. Шукайте за підрозділом або особою.",
  "By unit": "За підрозділом", "By person": "За особою", "No memberships found.": "Членств не знайдено.",
  "From": "Від",
  // membership + position forms
  "Add membership": "Додати членство",
  "Belong this person to the unit. Pick a position to assign them to that billet, or leave it empty for plain belonging.":
    "Додайте належність цієї особи до підрозділу. Оберіть посаду, щоб призначити її на цей штат, або залиште порожнім для простої належності.",
  "Position (optional)": "Посада (необов’язково)", "— plain belonging —": "— проста належність —",
  "Effective from (optional)": "Чинний від (необов’язково)", "End": "Завершити",
  "Create position": "Створити посаду", "auto from title": "авто з назви",
  "title (e.g. Commanding Officer)": "назва (напр. Командир)", "title": "назва", "sort order": "порядок сортування",
  "Fill": "Заповнити", "person…": "особа…", "Assign": "Призначити", "Abolish": "Ліквідувати",
  // orders
  "Administrative orders (наказ) — the legal basis for status changes. Effects apply on issue, with provenance.":
    "Адміністративні накази — правова підстава для змін статусу. Наслідки застосовуються при виданні, з простежуваністю.",
  "No orders for this unit.": "Немає наказів для цього підрозділу.",
  "Pick an issuing unit above to list or create orders.": "Оберіть підрозділ видачі вище, щоб переглянути або створити накази.",
  // documents
  "Catalogs for person-held papers and national-identifier schemes. A person's actual documents and (encrypted) codes live on their detail page.":
    "Каталоги документів особи та схем національних ідентифікаторів. Фактичні документи й (зашифровані) коди особи — на її сторінці.",
  "No document types.": "Немає типів документів.", "No personal-code schemes.": "Немає схем особистих кодів.",
  // audit
  "Audit log": "Журнал аудиту",
  "Append-only trail of permission-sensitive actions. Reads are themselves permission-scoped.":
    "Незмінний журнал чутливих до дозволів дій. Самі перегляди теж обмежені дозволами.",
  "Filter by action": "Фільтр за дією", "e.g. assignment.grant": "напр. assignment.grant",
  "No audit entries.": "Немає записів аудиту.", "Time": "Час", "Action": "Дія", "Actor": "Виконавець",
  "system": "система",
  // order forms
  "New order (наказ)": "Новий наказ",
  "Created in DRAFT. Effects apply only when you issue it.":
    "Створено як ЧЕРНЕТКА. Наслідки застосовуються лише коли ви видасте наказ.",
  "order number": "номер наказу", "item type…": "тип пункту…", "subject person…": "суб’єкт (особа)…",
  "item unit (optional)…": "підрозділ пункту (необов.)…", "note (optional)": "примітка (необов.)",
  "Create order": "Створити наказ", "Edit draft": "Редагувати чернетку", "number": "номер",
  "retired": "знятий", "category…": "категорія…", "effect…": "ефект…", "Add order type": "Додати тип наказу",
  // document catalog forms
  "Add scheme": "Додати схему", "regex (optional)": "регулярний вираз (необов.)",
  "code (e.g. ua-rnokpp)": "код (напр. ua-rnokpp)", "category (e.g. tax-id)": "категорія (напр. tax-id)",
  // graphs page
  "Graph admin": "Адміністрування графів",
  "Named unit hierarchies and the transitive closure that feeds the PDP.":
    "Іменовані ієрархії підрозділів та транзитивне замикання, що живить PDP.",
  "Closure": "Замикання",
  "Rebuild or verify the materialized transitive-closure table (descendant/ancestor reach).":
    "Перебудуйте або перевірте матеріалізовану таблицю транзитивного замикання (досяжність нащадків/предків).",
  // localization page
  "Instance-admin-managed supported locales. Every translatable label is returned in all locales — there is no Accept-Language negotiation.":
    "Підтримувані локалі, керовані адміністратором екземпляра. Кожен перекладний підпис повертається в усіх локалях — без узгодження Accept-Language.",
  "default": "за замовчуванням", "enabled": "увімкнено", "disabled": "вимкнено", "No locales.": "Немає локалей.",
  "Canonical languages": "Канонічні мови", "Languoid": "Мовоїд",
  "Each locale's canonical Glottolog language (D-Languages). Read-only — reconciled by the language-scheme import (matching the locale's ISO 639-3 code to a languoid).":
    "Канонічна мова кожної локалі за Glottolog (D-Languages). Лише для читання — узгоджується імпортом мовної схеми (зіставлення коду ISO 639-3 локалі з мовоїдом).",
  "No canonical languages linked yet — run the language-scheme import.":
    "Канонічні мови ще не прив’язані — запустіть імпорт мовної схеми.",
  // shared select components
  "Search…": "Пошук…", "Search a language…": "Пошук мови…", "No matches": "Немає збігів",
  "Searching…": "Пошук…", "(failed to load list)": "(не вдалося завантажити список)", "clear": "очистити",
  "command (default)": "командування (за замовч.)", "Rebuild": "Перебудувати", "Verify": "Перевірити",
  // graph manager + locale forms
  "Add graph": "Додати граф", "(directory-only)": "(лише довідник)",
  "Add locale": "Додати локаль", "ISO 639-3 code, e.g.": "Код ISO 639-3, напр.",
  "display name": "відображувана назва", "Disable": "Вимкнути", "Enable": "Увімкнути",
  // unit detail forms
  "Edit unit": "Редагувати підрозділ", "Suspend": "Призупинити", "Archive": "Архівувати",
  "Restore": "Відновити", "(unchanged)": "(без змін)",
  "No languages set.": "Мови не встановлено.", " · official": " · офіційна", " · working": " · робоча",
  "official": "офіційна",
  // unit detail page
  "Unit detail, graph neighbourhood, and positions.": "Деталі підрозділу, оточення у графі та посади.",
  "All units": "Усі підрозділи", "Details": "Деталі", "ID": "ID", "Ancestors": "Предки", "Descendants": "Нащадки",
  "No parents (a root unit).": "Немає батьківських (кореневий підрозділ).", "No descendants.": "Немає нащадків.",
  "Manage unit": "Керування підрозділом",
  "Edit details, or move it through its lifecycle (archive is the equivalent of delete).":
    "Редагуйте деталі або проведіть через життєвий цикл (архівування — еквівалент видалення).",
  "Edges": "Зв’язки (ребра)",
  "Nest this unit under a parent (creates a child relationship in the chosen graph).":
    "Вкладіть цей підрозділ під батьківський (створює дочірній зв’язок в обраному графі).",
  "The unit's official / working languages (D-Languages).":
    "Офіційні / робочі мови підрозділу (D-Languages).",
  "vacant": "вакантна", "abolished": "ліквідована", "filled": "заповнена",
  "No positions defined for this unit.": "Для цього підрозділу не визначено посад.",
  // edge manager
  "Parents": "Батьківські", "Add parent": "Додати батьківський", "Search a parent unit…": "Пошук батьківського підрозділу…",
  "No parents in this graph (a root unit).": "Немає батьківських у цьому графі (кореневий підрозділ).",
  "Children": "Дочірні", "Add child": "Додати дочірній", "Search a child unit…": "Пошук дочірнього підрозділу…",
  "No children in this graph.": "Немає дочірніх у цьому графі.",
  "Lists show the graph's ancestors / descendants; remove targets a direct edge (a transitive relation must be detached at its own edge).":
    "Списки показують предків / нащадків графа; видалення стосується прямого ребра (транзитивний зв’язок треба від’єднати на його власному ребрі).",
  // order detail + graph page
  "item": "пункт", "No items.": "Немає пунктів.",
  "Issuing an order applies its effects synchronously and records provenance; revoking is the legal counter-act.":
    "Видання наказу застосовує його наслідки синхронно та фіксує походження; відкликання — правовий контрзахід.",
  "Relationship graph": "Граф зв’язків",
  "Click a node to fan out its links; double-click to open it.": "Клацніть вузол, щоб розгорнути зв’язки; подвійний клік — відкрити.",
  "Object view →": "Перегляд об’єкта →", "Not a valid RID.": "Недійсний RID.",
  // person detail page
  "All persons": "Усі особи", "Identity": "Ідентичність", "Contact channels": "Канали зв’язку",
  "Social & messenger": "Соцмережі та месенджери", "Citizenship & residence": "Громадянство та місце проживання",
  "Name variants": "Варіанти імені", "Relationships": "Стосунки",
  "Documents & personal codes": "Документи та особисті коди",
  "No memberships.": "Немає членств.", "No orders reference this person.": "Жоден наказ не посилається на цю особу.",
  "Loading relationships, documents, memberships…": "Завантаження стосунків, документів, членств…",
  // religion page
  "The multi-faith taxonomy (religion → branch → tradition → sub-tradition → denomination) and the religion-type classifications. Organizations reuse tenant units; per-unit faith profiles live on the unit object view.":
    "Багатоконфесійна таксономія (релігія → гілка → традиція → піднапрям → деномінація) та класифікації типів релігій. Організації використовують підрозділи; профілі віри окремих підрозділів — на сторінці підрозділу.",
  "all ranks": "усі ранги", "all faiths": "усі віри", "Search": "Пошук", "code or name": "код або назва",
  "Select a taxon to see its resolved classification and edit its theism tags.":
    "Виберіть таксон, щоб побачити його класифікацію та редагувати теги теїзму.",
  "Clergy grades": "Ступені духовенства", "No grades.": "Немає ступенів.",
  "Affiliation types": "Типи належності", "No types.": "Немає типів.",
  "Clergy roster (by org unit)": "Реєстр духовенства (за оргпідрозділом)", "org unit RID": "RID оргпідрозділу",
  "Lookup": "Знайти", "Enter a unit RID.": "Введіть RID підрозділу.",
  "No credentials conferred by this unit.": "Цей підрозділ не надав посвідчень.",
  "No taxa match.": "Немає відповідних таксонів.", "Wikidata": "Wikidata", "Add taxon": "Додати таксон",
  "— rank —": "— ранг —", "— root religion (no parent) —": "— коренева релігія (без батьківської) —",
  "Effective religion-type (resolved)": "Ефективний тип релігії (визначено)",
  "none (no ancestor declares one)": "немає (жоден предок не оголошує)",
  "Declare tags on this taxon (overrides inherited)": "Оголосіть теги на цьому таксоні (перевизначає успадковані)",
  "Set declared tags": "Встановити оголошені теги",
  "Site types": "Типи об’єктів", "Service types": "Типи служінь", "None.": "Немає.",
  "Discovery search": "Пошук у каталозі", "lat": "шир.", "lng": "довг.", "radius m": "радіус, м",
  "language (ISO 639-3)": "мова (ISO 639-3)", "any day": "будь-який день", "name / alias": "назва / псевдонім",
  "online / hybrid only": "лише онлайн / гібрид", "No public sites match.": "Немає публічних об’єктів.",
  "Sunday": "Неділя", "Monday": "Понеділок", "Tuesday": "Вівторок", "Wednesday": "Середа",
  "Thursday": "Четвер", "Friday": "П’ятниця", "Saturday": "Субота",
  "Sites & aliases (by org unit)": "Об’єкти та псевдоніми (за оргпідрозділом)", "Load": "Завантажити",
  "Sites": "Об’єкти", "No sites.": "Немає об’єктів.", "Precision": "Точність", "Coord": "Координати",
  "location RID": "RID локації", "— site type —": "— тип об’єкта —", "primary site": "основний об’єкт",
  "Add site": "Додати об’єкт", "Aliases (search-only)": "Псевдоніми (лише для пошуку)",
  "No aliases.": "Немає псевдонімів.", "alias text": "текст псевдоніма", "Add alias": "Додати псевдонім",
  // locations page
  "The shared place entity (D-Location): a precise coordinate with a structured address. The coordinate can be entered in several formats (lat/lon, MGRS, UTM, СК-42); the server converts it to WGS84 and derives the MGRS in the application.":
    "Спільна сутність місця (D-Location): точна координата зі структурованою адресою. Координату можна ввести в кількох форматах (lat/lon, MGRS, UTM, СК-42); сервер конвертує її у WGS84 та обчислює MGRS у застосунку.",
  "Create a location": "Створити локацію", "Coordinate format": "Формат координат", "Country *": "Країна *",
  "House no.": "Буд. №", "Create location": "Створити локацію", "Created —": "Створено —", "open": "відкрити",
  "MGRS *": "MGRS *", "Zone *": "Зона *", "Hemisphere": "Півкуля", "Easting *": "Схід (E) *", "Northing *": "Північ (N) *",
  "Easting (Y) *": "Схід (Y) *", "Northing (X) *": "Північ (X) *", "СК-42 grid *": "СК-42 сітка *",
  "Full numeric reference: zone northing easting (metres).": "Повне числове посилання: зона північ схід (метри).",
  "Latitude *": "Широта *", "Longitude *": "Довгота *", "Radius search": "Пошук за радіусом",
  "Radius (m)": "Радіус (м)", "Search nearby": "Шукати поблизу",
  "Enter a centre point and radius to find nearby locations.": "Введіть центральну точку та радіус, щоб знайти локації поблизу.",
  "No locations within the radius.": "Немає локацій у межах радіуса.",
  "Kyiv": "Київ", "Kyiv City": "Київ (місто)", "Maidan Nezalezhnosti": "Майдан Незалежності",
  // education page
  "External reference institutions (where people studied/taught) and their internal structure tree. Distinct from the deploying org's tenant units.":
    "Зовнішні довідкові заклади (де люди навчалися/викладали) та їхнє внутрішнє дерево структури. Відмінні від підрозділів організації.",
  "Reference layer": "Довідковий шар", "None yet.": "Поки що порожньо.", "Create an institution": "Створити заклад",
  "KPI": "КПІ", "Kind *": "Вид *", "Create institution": "Створити заклад", "Created": "Створено",
  "No institutions yet.": "Поки що немає закладів", "— structure": "— структура",
  "Units (tree)": "Підрозділи (дерево)", "No units.": "Немає підрозділів.", "name (FIOT)": "назва (ФІОТ)",
  "kind…": "вид…", "— top-level —": "— верхній рівень —", "Add unit": "Додати підрозділ", "No positions.": "Немає посад.",
  // companies page
  "A legal-entity registry — identity, legal form, registrations, locations, positions, and the ownership/affiliation graph. External reference data, independent of the deploying org's units.":
    "Реєстр юридичних осіб — ідентичність, організаційна форма, реєстрації, локації, посади та граф власності/зв’язків. Зовнішні довідкові дані, незалежні від підрозділів організації.",
  "Register a company": "Зареєструвати компанію", "Legal name *": "Юридична назва *", "Short name": "Коротка назва",
  "Acme LLC": "ТОВ «Акме»", "Acme": "Акме", "Legal form *": "Організаційна форма *", "Ownership": "Власність",
  "Register company": "Зареєструвати компанію", "filter…": "фільтр…", "No companies.": "Немає компаній.",
  "Legal name": "Юридична назва", "— registry": "— реєстр", "Registrations": "Реєстрації",
  "unvalidated": "неперевірено", "remove": "видалити", "scheme…": "схема…", "identifier": "ідентифікатор",
  "Industries": "Галузі", "(primary)": "(основна)", "class…": "клас…", "primary": "основна",
  "Search a location…": "Пошук локації…", "Appoint to": "Призначити на", "Appoint": "Призначити", "cancel": "скасувати",
  "title (CEO)": "назва (CEO)", "fill": "заповнити",
  "Ownership & affiliation graph": "Граф власності та зв’язків",
  "Shareholders (stakes held IN this company)": "Акціонери (частки в цій компанії)",
  "Holdings (subsidiaries — stakes this company holds)": "Володіння (дочірні — частки цієї компанії)",
  "Founders": "Засновники", "Beneficial owners (UBO)": "Кінцеві бенефіціари (UBO)",
  " · declared": " · задекларовано", " · computed": " · обчислено", "Branches": "Філії",
  "branch company…": "компанія-філія…", "Successions": "Правонаступництва", "successor…": "правонаступник…",
  "company": "компанія", "person": "особа", "Search a company…": "Пошук компанії…",
  // person companies/education managers
  "No company affiliations.": "Немає корпоративних зв’язків.", "Employment": "Працевлаштування",
  "Shareholdings": "Частки", "Beneficial owner of": "Кінцевий бенефіціар", "now": "зараз",
  "Read-only. Company links are recorded from the Companies workspace.":
    "Лише для читання. Корпоративні зв’язки записуються в розділі «Компанії».",
  // person education manager
  "Education relationships": "Освітні зв’язки", "Enrollments": "Зарахування",
  "Appointments (read-only)": "Призначення (лише читання)", "pick institution first": "спершу оберіть заклад",
  "(current)": "(поточний)", "Institution *": "Заклад *",
  "Degree level…": "Рівень освіти…", "Status…": "Статус…", "Field of study": "Галузь знань",
  "Student number": "Номер студента", "Qualification awarded": "Присвоєна кваліфікація",
  "Save enrollment": "Зберегти зарахування", "Add enrollment": "Додати зарахування", "Dormitory": "Гуртожиток",
  "Room": "Кімната", "Save stay": "Зберегти проживання", "Add stay": "Додати проживання",
  "Publication *": "Публікація *", "Author order": "Порядок автора", "corresponding author": "кореспондуючий автор",
  "Save authorship": "Зберегти авторство", "Add authorship": "Додати авторство", "Role…": "Роль…",
  "Add holding": "Додати володіння", "Link enrollment…": "Прив’язати зарахування…",
  "GPA": "Середній бал", "with distinction": "з відзнакою", "Save award": "Зберегти нагороду",
  "Add award": "Додати нагороду", "Scholarship *": "Стипендія *",
  "Read-only. Teaching/admin positions are filled and ended from the Education institution view.":
    "Лише для читання. Викладацькі/адмін. посади заповнюються та завершуються в розділі закладу освіти.",
  // person forms (identity, channels, relationships, clergy, affiliation)
  "Edit person": "Редагувати особу", "Purge": "Видалити дані (purge)",
  "Set rank": "Встановити звання", "— none —": "— немає —",
  "Emails": "Електронні адреси", "Phones": "Телефони", "Call signs": "Позивні", "Citizenships": "Громадянства",
  "Residences": "Місця проживання", "Personal codes": "Особисті коди", "Languages spoken": "Мови, якими володіє",
  "Clergy credentials": "Посвідчення духовенства", "Religious affiliation": "Релігійна належність",
  "Kinships (parent → child)": "Спорідненість (батько → дитина)",
  "type…": "тип…", "basis…": "підстава…", "birth": "народження", "descent": "походження",
  "naturalization": "натуралізація", "other": "інше", "region (optional)": "регіон (необов.)",
  "call sign": "позивний", "identifier value": "значення ідентифікатора", "platform…": "платформа…",
  "handle": "нік", "self-declared": "самозаявлено", "operator-verified": "перевірено оператором",
  "imported": "імпортовано", "platform-verified": "перевірено платформою", "history": "історія",
  "hide": "сховати", "loading…": "завантаження…", "no rename history": "немає історії перейменувань",
  "messenger…": "месенджер…", "phone or email…": "телефон або ел. пошта…",
  "counterpart person…": "особа-контрагент…", "CEFR…": "CEFR…", "native": "рідна",
  "conferring org unit…": "оргпідрозділ, що надає…", "Reinstate": "Поновити",
  "Special-category data (GDPR Art. 9) — encrypted at rest.": "Дані особливої категорії (GDPR ст. 9) — шифруються в спокої.",
  "detail (optional)": "деталь (необов.)", "faith (optional)…": "віра (необов.)…",
  "relation…": "відносини…", "relation (optional)": "відносини (необов.)",
  "Pick a subject person or a target unit (exactly one) to list assignments.":
    "Виберіть суб’єкта (особу) або цільовий підрозділ (рівно один), щоб переглянути призначення.",
  "Delete": "Видалити",
  "Filter by subject person": "Фільтр за суб’єктом (особою)", "Filter by target unit": "Фільтр за цільовим підрозділом",
  "Expires": "Спливає",
  "No roles yet.": "Поки що немає ролей.", "No assignments match.": "Немає відповідних призначень.",
  "active": "активне", "revoked": "відкликане",
};

const GLOSSARY: Record<string, Record<string, string>> = { ukr: glossaryUkr };

/**
 * Translate a registry/chrome string by its English source text. Returns the active-locale (or
 * `locale`-override) translation, or the original English text when there is no entry — so untranslated
 * strings degrade to English rather than to a key. Use this at render points that show the ontology
 * registry's labels/headers and the shared workspace chrome.
 */
export function tg(text: string | undefined | null, locale: string = getActiveLocale()): string {
  if (!text) return text ?? "";
  return GLOSSARY[locale]?.[text] ?? text;
}

/** The UI locales the chrome catalog covers (for completeness checks/tests). */
export const CATALOG_LOCALES: UiLocale[] = ["eng", "ukr"];
