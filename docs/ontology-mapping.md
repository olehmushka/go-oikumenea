# Ontology mapping (Object / Link / Action lens)

> Reads: [glossary](glossary.md) · [architecture/decisions.md](architecture/decisions.md) ·
> [architecture/patterns.md](architecture/patterns.md) · the [modules/](modules/) docs.

This is the **binding Object / Link / Action registry** ([D-Ontology](architecture/decisions.md)). It
is the authoritative catalog of which typed **Objects** (each with a stable RID `__rid`), first-class
**Links** (reified relationships, not foreign keys), and audited **Actions** the service defines.
Module docs **conform** to it: each classifies its entities by the kind named here.

**Source-of-truth split:** this file owns the *catalog* (which types exist + their kind); the
[modules/](modules/) docs own the *detail* (columns, RID shape, lifecycle, invariants, endpoints);
[decisions.md](architecture/decisions.md) **wins on any genuine conflict** — log such a conflict in
[open-questions.md](open-questions.md), not by editing here. Each row cites the module doc it derives
from. The catalog is intentionally lossy: it shows the *kind and shape*, not every column.

Ontology ↔ schema dictionary (holds for every row below, so it is stated once):

| Ontology field | go-oikumenea schema (per [conventions.md](architecture/conventions.md)) |
|---|---|
| `__rid` | the **packed UUIDv8** primary key (`id uuid`) via `new_id(service,kind,type)` (D-ResourceIdentifiers); immutable, self-describing (decodes to app/service/kind/type). See [Identifier scheme](#identifier-scheme-rids) |
| `object_type` / `link_type` | the entity's packed **type code** (per service; kind distinguishes object vs link); also the table (per-module prefix `oikumenea.<module>_*`) |
| `created_at` / `updated_at` | `TIMESTAMPTZ` UTC + the `set_updated_at()` trigger |
| `status` | the entity's `status`/`state` enum (`TEXT`+`CHECK`) and/or `deleted_at` soft-delete |
| `source` | **mostly absent as a column** — provenance lives in [order](modules/order.md) refs + the [audit](modules/audit.md) actor; see [§4.4](#4-ratified-divergences-from-the-ontology-ideal) |
| Action `audit_log` | the append-only [audit](modules/audit.md) entry written in the same transaction |
| code vs name | every structural Object has a stable locale-agnostic **`code`** + a translatable **`name`** ([localization](modules/localization.md)) |

---

## Identifier scheme (RIDs)

Every row below is keyed by a **packed UUIDv8 RID** (D-ResourceIdentifiers, amended F-014; full layout
in [conventions.md](architecture/conventions.md#resource-identifiers-rids)): a native `uuid` whose bits
encode *app · service · kind · type · timestamp · random*. **Kind** is object / link / action; **type**
is a per-service numeric code held in `platform_rid_types` (mirrored in `pkg/rid`) — this table is its
authoritative *list*, with each row's owning module = its service. The human, decomposable form is
rendered from the bytes as `oikumenea:<service>:<kind>:<type>:<uuid>` (e.g. `oikumenea:authz:link:
has_role:…`), never stored. Temporal Links additionally carry `valid_from`/`valid_to` (the existing
`effective_from`/`effective_to`, `granted_at`/`revoked_at`+`expires_at` columns); the audit row
recording an Action is keyed by its Action RID (kind=action) — the audit log is the action ledger.

## 1. Object Types

Real-world entities with identity over time → Objects.

| Object Type | Module | `code`/`name`? | Lifecycle / removal | Notes |
|---|---|---|---|---|
| `Unit` | [tenant](modules/tenant.md) | **optional**, **mutable** `code` (unique among active **coded** units) | `state` (active/suspended/archived) + soft-delete | codeless ⇒ non-separate sub-unit; code set/cleared via audited `RecodeUnit` (history in `tenant_unit_code_events`, RID `4,1,4`); external ref is the **RID**, not `code` (D-UnitCodeLifecycle, M28); belongs to an `Organization` (`org_id`) + a `Domain` (`domain_id`, per-unit — mixed trees) + a `UnitKind` (`kind_id`); `visibility` public/shadow; `level`, `domain`, `org`, `kind` are **directory attributes**, never PDP inputs (D-TenantOrganizations, M40) |
| `Graph` | [tenant](modules/tenant.md) | yes | soft-delete (`command` undeletable) | named hierarchy, **per organization** (`org_id`, nullable — NULL = instance-global/cross-org graph, e.g. religion taxonomy); `is_authority_bearing` gates PDP cascade (D-TenantOrganizations, M40) |
| `Domain` *(M40)* | [tenant](modules/tenant.md) | `code` (NOT NULL UNIQUE) | `status` active/retired + soft-delete | org-kind catalog (RID `4,1,5`): military/government/company/university/church/public-org; instance-admin-extensible, **never a CHECK enum**; classifies organizations & units; **never a PDP input** (D-TenantOrganizations) |
| `Organization` *(M40)* | [tenant](modules/tenant.md) | `code` (NOT NULL UNIQUE) | `state` (active/suspended/archived) + soft-delete | the realm a person joins (RID `4,1,6`): US Army / Bundeswehr / KhNU; `domain_id`, `visibility` public/shadow; owns units + per-org graphs; **directory attribute, never a PDP input** (D-TenantOrganizations) |
| `UnitKind` *(M40)* | [tenant](modules/tenant.md) | `code` (unique per domain) | `status` + soft-delete | domain-scoped catalog (RID `4,1,7`) replacing free-text `unit_kind`; optional `attr_schema` validates unit `metadata` per kind (D-TenantOrganizations) |
| `OrgLifecycleEvent` *(M40)* | [tenant](modules/tenant.md) | none | append-only (`reject_mutation()`) | organization state-transition ledger (RID `4,1,8`); mirrors `unit_lifecycle_event` |
| `Person` | [person](modules/person.md) | optional `code` | `status` (active/deactivated/purged) + soft-delete; **crypto-erase on purge** | instance-global; CLDR structured names (patronymic in `given2`); `birthdate`, ISO-5218 `sex` |
| `PersonEmail` / `PersonPhone` / `PersonCallSign` | [person](modules/person.md) | no | soft-delete; **erased on purge** | effective person child rows; email/phone `pii:contact`, call sign `pii:basic`; each unique per person among active; `is_primary` |
| `PersonEmailType` / `PersonPhoneType` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalogs for the contact-channel `kind` |
| `PersonSocialAccount` | [person](modules/person.md) | no | soft-delete; **erased on purge** | standalone social handle; stable `platform_user_id` vs mutable `handle` (history in `PersonSocialAccountHandle`); `pii:contact`; `platform_verified` vs `verified_by_operator_at` (D-PersonSocialChannels) |
| `PersonSocialAccountHandle` | [person](modules/person.md) | no | temporal (`valid_from`/`valid_to`) + soft-delete | handle-rename history so a rename never breaks links |
| `PersonPhysicalDescription` *(M31)* | [person](modules/person.md) | no | effective-dated + soft-delete; **erased on purge** | `person_physical_descriptions` (`6,1,11`): height/weight/eye/hair/build/`blood_type`; `pii:basic`; D-PhysicalIdentity |
| `PersonDistinguishingMark` *(M31)* | [person](modules/person.md) | no | soft-delete; **erased on purge** | `person_distinguishing_marks` (`6,1,12`): tattoo/scar/piercing/birthmark; `pii:special` ceiling (a mark can reveal Art. 9 data) |
| `PersonExternalReference` *(M33)* | [person](modules/person.md) | no | soft-delete; **erased on purge** | `person_external_references` (`6,1,14`): off-platform pointers (wikipedia/news/registry/social/court/academic) with `url`/`external_id`/`categories`/`last_checked`/`disputed`; mirrors `PersonSocialAccount`; a hermenea import target; `pii:basic`; `source`/`confidence`; D-InstitutionalTies |
| `PersonEthnicityType` *(M31; hierarchical M43)* | [person](modules/person.md) | yes (`code`/`name` i18n) | `status` + soft-delete | `person_ethnicity_types` (`6,1,13`): instance-admin-managed declared-ethnicity vocabulary; plaintext (a controlled vocabulary, not a person's datum). **M43 (D-Color-style promotion, D-PhysicalIdentity amendment):** `parent_id` self-FK + `person_ethnicity_type_closure` (RID-less, like `language_languoid_closure`) + `wikidata_id`; group-level M:N `person_ethnicity_type_languages` (→ `language_languoids`) + `person_ethnicity_type_countries` (→ `geo_countries`) — bare associations, no RID; opt-in **CIA World Factbook** `ethnicity-scheme` import — fetched + parsed live at runtime (public domain; flat catalog + homeland-country ties — no hierarchy/language from this source). The group↔language tie is **never inferred onto a person** |
| `Platform` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalog of messengers/social networks; `category ∈ messenger\|social` |
| `RelationType` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalog for open-ended person↔person relation labels (`category ∈ sponsorship\|association\|next_of_kin`) |
| `Position` | [membership](modules/membership.md) | yes (unique per unit) | `status` (active/abolished) + soft-delete | unit-owned billet; an Object that **exists while vacant** — not just a link end |
| `Document` / `DocumentType` | [document](modules/document.md) | type has `code`/`name` | `status` + soft-delete | papers, metadata only; type is an instance-admin catalog |
| `PersonalCode` / `PersonalCodeScheme` | [document](modules/document.md) | scheme has `code`/`name` | `status` + soft-delete; crypto-erase | value is `pii:sensitive`, **envelope-encrypted** + blind-indexed |
| `Order` (наказ) / `OrderType` | [order](modules/order.md) | type has `code`/`name` | `Order`: draft→issued→revoked (issued is immutable) | the legal act; `OrderType.effect` declares the downstream consequence |
| `OrderItem` | [order](modules/order.md) | no | parent-scoped (no own `deleted_at`) | one affected person/action; the unit of effect + provenance |
| `RankSystem` / `RankCategory` / `RankType` / `Rank` | [rank](modules/rank.md) | yes | soft-delete (RESTRICT if held) | single system-wide ordered scheme, now rooted at `RankSystem` (a national/organizational ladder — multinational; D-RankSystems); types form a **tree** (`parent_type_id` self-FK — a structural containment FK like `system_id`/`category_id`/`type_id`, **not** a reified Link), ranks on leaf types; a rank carries an optional standardized `grade_code` → `RankGrade` |
| `RankGrade` | [rank](modules/rank.md) | `code` = NATO STANAG 2116 grade (`OF-1`…`OF-10`, `OR-1`…`OR-9`, warrant) | seeded reference registry | the cross-system comparability scale (`tier`/`ordinal`); migration-seeded, natural `code` key (D-RankSystems carve-out) |
| `Role` | [authorization](modules/authorization.md) | yes | soft-delete | `is_base` roles immutable; permissions are **code**, not rows |
| `Assignment` | [authorization](modules/authorization.md) | no | revoke-flip + optional `expires_at` | a **reified Link** — see [§2](#2-link-types) |
| `InstanceAdmin` | [authorization](modules/authorization.md) | no | revoke-flip | the instance-wide authority plane |
| `Account` / `ExternalIdentity` | [identity-federation](modules/identity-federation.md) | no | account soft-delete; identity append-only | `(issuer, subject)` globally unique; account optional per person |
| `ServicePrincipal` *(M51)* | [identity-federation](modules/identity-federation.md) | yes (`code`) | `status` (active/disabled) + soft-delete | `account_service_principals` (`9,1,3`): a **machine** subject (facade / connector) authenticated by the IdP's client-credentials grant on the same `(issuer, subject)` key as `ExternalIdentity` — an `(issuer, subject)` is one **or** the other, enforced by symmetric triggers. `client_id` is a display label, never an authz input. Holds no role assignment and **no unit reach**; authority is `PRINCIPAL_GRANT`. D-ServiceIdentities |
| `Locale` / `Translation` | [localization](modules/localization.md) | locale `code` is ISO 639-3 | locale soft-delete | the translatable-`name` store |
| `Country` | [location](modules/location.md) | **RID** (location svc); `code` = ISO-3166-1 α2 is a `UNIQUE` lookup key | status | RID-keyed shared registry (F-014); consumers reference by `id`; resolve a code → RID via `GET /geo/countries` (D-Geo) |
| `AuditEntry` | [audit](modules/audit.md) | no | **append-only** (`reject_mutation()`) | not an endpoint; written in-transaction |
| `ImportSource` / `ImportRawBatch` / `ImportRun` / `WorkerJob` / `WorkerSchedule` *(M16, hermenea's **own** DB)* | [hermenea](modules/hermenea.md) | source/job `code`/key | source soft-delete; raw/run/job append-or-update | the companion service's ingestion + job-runtime objects — **not** oikumenea Objects; coupled to oikumenea only via the `POST /import/{objectType}` upsert (which stamps `(source, source_version, imported_at)` provenance on the target rows); D-Hermenea (supersedes D-Worker, folds D-DataIngestion) |
| `Languoid` *(M18)* | [language](modules/language.md) | **RID** (language svc); `code` = glottocode; nullable unique `iso639_3` | reference (import) | recursive Glottolog forest, `level ∈ family\|language\|dialect`; `parent_id` strict-tree FK (not a Link); denormalized `family_code` (derived via the closure); AES `status`; import-loaded via hermenea `language-scheme`; D-Languages |
| `WritingSystem` / `WritingSystemScriptType` *(M18)* | [language](modules/language.md) | **RID** (language svc); `code` (ISO 15924 / catalog) | seeded reference | scripts + script-type catalog, both migration-seeded; `language_writing_systems` (`WRITTEN_IN`) import-loaded from CLDR; D-Languages |
| `Location` *(M19)* | `location` | no | soft-delete | required `GEOGRAPHY(POINT,4326)`; app-derived MGRS; multi-format coordinate input + `source_coordinate`; structured address over `geo_countries`; D-Location |
| `EducationBuilding` / `EducationGroup` *(M20)* | [education](modules/education.md) | building/group `code` | soft-delete | education objects (RID service 14, types 3/4); hang off a tenant `Organization` / `Unit`; buildings → `Location`; D-Education. **M41 / D-UnifiedOrgGraph:** an institution is a tenant `Organization` (domain=`university`) + an `education_org_profiles` sidecar; a unit is a tenant `Unit` (the `EducationInstitution`/`EducationUnit` objects + `education_unit_closure` are gone) |
| `EducationPosition` *(M20)* | [education](modules/education.md) | yes (per institution) | `status` + soft-delete | institution/unit-owned billet, vacant-first (mirrors `Position`) |
| `EducationInstitutionKind` / `EducationUnitKind` / `EducationDegreeLevel` *(M20)* | [education](modules/education.md) | yes (`code`/`name`) | `status` + soft-delete | catalogs; degree levels migration-seeded ISCED 2011 (0–8) |
| `Program` / `Course` / `CurriculumVersion` *(M20 ref)* | [education](modules/education.md) | `code` (per institution / program) | lifecycle + soft-delete | reference curriculum catalog (RID `14,1,9/10/11`); D-Education reference layer |
| `ResearchCentre` / `ResearchGroup` / `Grant` / `Publication` *(M20 ref)* | [education](modules/education.md) | `code` | `status`/lifecycle + soft-delete | reference research entities (RID `14,1,12..15`) |
| `GovernanceBody` / `Policy` / `Qualification` / `Scholarship` / `AccreditationEvent` *(M20 ref)* | [education](modules/education.md) | `code` (events: none) | lifecycle + soft-delete | reference governance/credentials (RID `14,1,16..20`) |
| `Company` *(M21, M41)* | [company](modules/company.md) / [tenant](modules/tenant.md) | `code` | soft-delete | legal entity = a `company`-domain `tenant_organization` (RID `4,1,6`) + a `company_org_profiles` sidecar keyed by that RID (M41 / D-UnifiedOrgGraph — **no own `15,1,1`**); `legal_form` + `ownership_category` (two axes); translatable `legal_name` = the org name; D-Companies |
| `CompanyPosition` *(M21)* | [company](modules/company.md) | yes (per company) | `status` + soft-delete | company-owned billet (RID `15,1,5`; mirrors `Position`) |
| `Registration` *(M21)* | [company](modules/company.md) | per `(scheme, identifier)` | soft-delete | a company's per-scheme registration id (RID `15,1,6`; mirrors `PersonalCode`); `validated` against the scheme pattern |
| `CompanyLegalForm` / `CompanyRegistrationScheme` / `CompanyIndustryClass` *(M21)* | [company](modules/company.md) | yes (`code`/`name`) | `status` + soft-delete | catalogs (RID `15,1,2/3/4`); registration schemes mirror `PersonalCodeScheme` (LEI ISO-17442 spine, per-scheme validators) |
| `LocationType` *(M19)* | `location` | yes (`code`/`name`) | `status` + soft-delete | optional place-purpose catalog beside `Location`; D-Location |
| `Taxon` *(M22)* | [religion](modules/religion.md) | `code` | soft-delete | the **recursive** faith taxonomy node (RID service 16, `16,1,1`); `parent_id` self-FK + `religion_taxa_closure`; `rank_id` level marker; denormalized root `religion_id`; optional `wikidata_id`; D-Religion (refined) |
| `TaxonRank` / `Classification` *(M22)* | [religion](modules/religion.md) | yes (`code`/`name`) | soft-delete | the ordered level scaffold (`16,1,2`: religion→branch→tradition→sub_tradition→denomination) + the religion-type/theism catalog (`16,1,3`), tagged M:N onto taxa |
| `OrgKind` / `PolicyKind` / `OrgPolicy` *(M22)* | [religion](modules/religion.md) | kinds have `code`/`name` | soft-delete | org nodes **reuse `Unit`**; `OrgKind` (`16,1,4`), `PolicyKind` (`16,1,5`), `OrgPolicy` (`16,1,6`); `OrgProfile` is a 1:1 Unit extension keyed by the unit RID (no own RID) |
| `ClergyGrade` / `GradeCategory` / `OfficeType` *(M23)* | [religion](modules/religion.md) | yes (`code`/`name`) | soft-delete | **per-tradition** ordered clergy catalog (`16,1,8` / `16,1,7` / `16,1,9`; `tradition_taxon_id` → `religion_taxa`); no cross-tradition comparator (DS-43); offices **reuse `Position`**; D-ClergyCredential |
| `AffiliationType` *(M24)* | [religion](modules/religion.md) | yes (`code`/`name`) | soft-delete | per-tradition lay-affiliation catalog (`16,1,10`); D-ReligiousAffiliation |
| `SiteType` / `ServiceType` *(M25)* | [religion](modules/religion.md) | yes (`code`/`name`) | soft-delete | per-tradition discovery catalogs (`16,1,11` / `16,1,12`; church/mosque/synagogue/temple…; main/prayer…) |
| `ServiceSchedule` / `Alias` *(M25)* | [religion](modules/religion.md) | no | soft-delete | per-site recurring service times (`16,1,13`); search-only alternative names (`16,1,14`, never displayed) |
| `GeoPlace` *(M16)* | [location](modules/location.md) | **RID** (location svc); `wof_id` (Who's-On-First id) is a `UNIQUE` concordance key | status (`active`/`retired`) | WOF admin gazetteer (country/region/county/locality); RID-keyed (F-014); `parent_id uuid` self-FK (tree), denormalized `country_id` → `Country`; PostGIS `geom`/`centroid`/`bbox`; import resolves `wof_id`/`code` → RID in SQL; import-loaded via hermenea `wof-sqlite` connector; D-GeoPlaces |
| ~~`GeoSubdivision`~~ *(superseded by `GeoPlace`/D-GeoPlaces)* | [platform](modules/platform.md) | ~~`code` = ISO 3166-2~~ | — | **not built** — ISO-3166-2 subdivisions are subsumed by the richer WOF `geo_places` (D-GeoPlaces); D-GeoSubdivisions |
| `Vehicle` *(M26)* | `vehicle` | optional | soft-delete | physical vehicle; `vin` unique among active (nullable, `pii:basic`); `type_id`/`model_id`; `attributes` JSONB; D-Vehicles |
| `VehicleBrand` / `VehicleModel` / `VehicleType` *(M26)* | `vehicle` | yes (`code`/`name`) | `status` + soft-delete | brand (`country` of origin); model (`brand_id` + generation/manufacture window); type taxonomy **tree** (`parent_id` self-FK + denormalized root, no closure — the `RankType` pattern) |
| `VehicleRegistrationNumberType` *(M26)* | `vehicle` | yes (`code`/`name`) | `status` + soft-delete | plate-type catalog (regular/temporary/transit/diplomatic/military/old…) |
| `LegalBasisKind` *(planned, M29)* | [platform](modules/platform.md) | yes (`code`/`name`) | seeded | `platform_legal_basis_kinds` — GDPR Art. 6 lawful bases + Art. 9 conditions; FK'd by every gated/special overlay; D-OverlayFoundation |
| `Color` *(M42)* | [platform](modules/platform.md) | yes (`code` + `name` i18n) | seeded + soft-delete | `platform_colors` — per-domain palette (`eye`/`hair`/`vehicle`), nullable `hex` swatch; **RID service 1, `1,1,1`** (platform's first Object); hard-FK'd by `vehicle_vehicles.color_id` + `person_physical_descriptions.eye_color_id`/`hair_color_id`; D-Color |
| `ExternalOrganization` | [external-organizations](modules/external-organizations.md) | optional | provisional/resolved + soft-delete | party/government/military/NGO/registrant; **RID service 18**; `kind`, optional `country`/`wikidata_id`; attribution; D-ExternalOrgs |
| `Account` *(M44)* | [finance](modules/finance.md) | RID only | `status` + soft-delete | bank account; **RID service 19, `19,1,1`**; `institution_id` → a `company`-domain `tenant_organizations` row (the bank); **envelope-encrypted IBAN** (`pii:sensitive`, blind index unique among active); `currency` (ISO 4217); D-Finance |
| `Card` *(M44)* | [finance](modules/finance.md) | RID only | `status` + soft-delete | payment card; `19,1,2`; `account_id` → `Account` (**containment** FK, CASCADE); **envelope-encrypted PAN** + clear `bin`/`last_four`; `card_type` debit\|credit; **no CVV column ever** (PCI Req 3.2); optional named `cardholder_person_id`; D-Finance |
| `FinanceAccountType` / `FinanceCardNetwork` *(M44)* | [finance](modules/finance.md) | yes (`code`/`name`) | `status` + soft-delete | account-kind (current/savings/…) + card-network (visa/…) catalogs (`19,1,3` / `19,1,4`), instance-extensible; D-Finance |
| `ExternalOrgKind` | [external-organizations](modules/external-organizations.md) | yes (`code`/`name`) | seeded | catalog (`party\|government_body\|military\|ngo\|registrant\|other`) |
| `PhysicalDescription` / `DistinguishingMark` *(planned, M31)* | [person](modules/person.md) | no | effective-dated + soft-delete | `person_physical_descriptions` (height/weight/eye/hair/build/blood_type, `pii:basic`); marks (`pii:special` ceiling); D-PhysicalIdentity |
| `EthnicityType` *(planned, M31)* | [person](modules/person.md) | yes (`code`/`name`) | catalog | open catalog for declared ethnicity (the value link is `pii:special`, encrypted); D-PhysicalIdentity |
| `ExternalReference` *(planned, M33)* | [person](modules/person.md) | no | soft-delete | `person_external_references` (wikipedia/news/registry; mirrors `SocialAccount`); hermenea target; D-InstitutionalTies |
| `WatchlistMatch` (M34) | [person](modules/person.md) | no | soft-delete | `person_watchlist_matches` (`6,1,15`, one active row/person — **match-metadata only**, never the lists; `pii:sensitive`; live via hermenea + ≤24h cache; hard-erased on purge); D-Watchlists |
| `RegulatorySanction` (M34) | [person](modules/person.md) | no | soft-delete | `person_regulatory_sanctions` (`6,1,16`, `pii:sensitive`, idempotent by `(person, externalId)`; hermenea import target); D-Watchlists |
| `CryptoWallet` / `Personality` / `PoliticalLeaning` *(planned, M35)* | [person](modules/person.md) | no | soft-delete | wallet attribution + declared personality (`pii:sensitive`); **inferred** political-leaning (`pii:special`, never merged with declared); D-PersonOverlays |
| `HealthRecord` / `Insurance` *(planned, M36)* | [person](modules/person.md) | no | soft-delete | category-level health (`pii:special`, no diagnosis, never inferred) + insurance (`pii:sensitive`); D-HealthVulnerability |
| `AccountLoginEvent` *(planned, M37)* | [identity-federation](modules/identity-federation.md) | no | retention-bounded + purge-erased | first-party login/IP security log on the account seam (`pii:contact`); D-LoginSecurityLog |
| `BankAccount` *(planned, M44)* | [finance](modules/finance.md) | no | `status` + soft-delete; **crypto-erase on purge** | `finance_accounts` (**RID service 19**, `19,1,1`); `institution_id` → a `company`-domain `tenant_organizations` (the bank, M41); **envelope-encrypted IBAN** + blind index (unique among active), `currency` (ISO 4217), `account_type_id`; `pii:sensitive` value; D-Finance |
| `PaymentCard` *(planned, M44)* | [finance](modules/finance.md) | no | soft-delete; **crypto-erase on purge** | `finance_cards` (`19,1,2`); `account_id` → `finance_accounts` (containment FK); **envelope-encrypted PAN** + display `bin`/`last_four`, `card_type ∈ {debit,credit}`, `network_id`, optional expiry + `cardholder_person_id`; **no CVV column** (PCI-DSS Req 3.2); `pii:sensitive`; D-Finance |
| `AccountType` / `CardNetwork` *(planned, M44)* | [finance](modules/finance.md) | yes (`code`/`name`) | `status` + soft-delete | catalogs (`19,1,3` / `19,1,4`): account kinds (current/savings/deposit/loan…) + card networks (visa/mastercard/amex…) |

(Planned-cluster RID type codes are allocated in `pkg/rid` + migration `0000` on build — person
service object 11+ / link 9+, account/platform as noted, external-organizations = service 18,
**finance = service 19** (M44).)

**Non-Objects (correctly):** `Atomic permission` is **code, not data** — a closed vocabulary in Go,
not a table ([authorization](modules/authorization.md)). `Vacancy` is a **derived predicate** (active
position, no active filling), not a stored row ([membership](modules/membership.md)). The `Unit
closure` is a **materialized derived relation**, treated here as a Link set ([§2](#2-link-types)),
not a source-of-truth Object.

---

## 2. Link Types

Relationships with their own identity, attributes, or history → Links (not FK columns). go-oikumenea
already models its load-bearing relationships as join/edge tables, so most map cleanly. Each Link's
RID packs kind=link plus its per-service type code; rendered, the type token is `link__<link_type>` in
lower_snake (e.g. the `PARENT_OF` row → `link__parent_of`, `HAS_ROLE` → `link__has_role`); temporal
Links additionally carry `valid_from`/`valid_to`
([Identifier scheme](#identifier-scheme-rids)).

Since **review-2026-09 Phase 15 (D-LinkTraversal, R‑27)** these Links are **generically traversable**
through one API (`LinkService.getObjectLinks`): each link table registers a `Descriptor` — validated
against this registry — so "what links does object X have?" is answered by a fan-in over the tables
below, without a per-relationship endpoint. The **polymorphic F‑014 ends**
(`held_by`/`registered_to`/`founded`/`owns_stake` — `holder_kind`/`holder_id text`, no FK) declare
one target per discriminator (person `(6,1,1)` / company = tenant org `(4,1,6)`), the declaration
generic traversal needs since it cannot infer a no-FK text edge from the schema. A handful of
non-RID / multi-ended / encrypted-end Links (`locale_language`, `has_ethnicity`, `party_membership`,
`government_position`, `lobbying_rel`, `has_role`, `instance_admin`, `affiliated_with`) are
**explicitly exempt** from traversal — see [modules/links.md](modules/links.md).

| Link Type | From → To | Module | Carries | Temporal? |
|---|---|---|---|---|
| `PARENT_OF` (per graph) | `Unit` → `Unit` | [tenant](modules/tenant.md) | `graph_id`, `created_by` provenance | created-only; multi-parent DAG, no validity interval |
| `ANCESTOR_OF` (derived) | `Unit` → `Unit` | [tenant](modules/tenant.md) | `graph_id`, `depth` | materialized closure; recomputed on edge change |
| `IN_UNIT` | `Position` → `Unit` | [membership](modules/membership.md) | — | structural |
| `MEMBER_OF` / `FILLS` | `Person` → `Unit` (opt. `Position`) | [membership](modules/membership.md) | `position_id` (nullable), `order_item_id` provenance | **yes — `effective_from`/`effective_to`** + `status` |
| `HAS_ROLE` @ scope (the **`Assignment`**) | `Person` → `Role`, scoped to `target_unit` | [authorization](modules/authorization.md) | `scope ∈ {unit,subtree}`, `graph_id`, `granted_by`, `expires_at` | grant/revoke + decision-time expiry |
| `GRANTS` | `Role` → `Permission`(code) | [authorization](modules/authorization.md) | — | code-validated membership |
| `PRINCIPAL_GRANT` *(M51)* | `ServicePrincipal` → `Permission`(code) | [authorization](modules/authorization.md) | `org_id` (**NULL = instance-wide**, else confines to that organization), `granted_by` | grant/revoke flip; **flat — no unit, no scope, no graph**: a machine has no reach (D-ServiceIdentities) |
| `HOLDS_RANK` | `Person` → `Rank` (in a `RankSystem`) | [person](modules/person.md) | `system_id` (derived) | **one per rank system** (`person_ranks`, reified); **directory attribute — never an authz input** |
| `HAS_ACCOUNT` | `Person` → `Account` | [identity-federation](modules/identity-federation.md) | ≤1 active | — |
| `FEDERATES` | `Account` → `ExternalIdentity` | [identity-federation](modules/identity-federation.md) | `(issuer, subject)` | identity row append-only |
| `HOLDS_DOCUMENT` / `HOLDS_CODE` | `Person` → `Document`/`PersonalCode` | [document](modules/document.md) | — | `status`; scoped through the holder |
| `OF_TYPE` / `OF_SCHEME` | `Document`/`PersonalCode` → catalog | [document](modules/document.md) | — | — |
| `HOLDS_EMAIL` / `HOLDS_PHONE` / `HOLDS_CALL_SIGN` | `Person` → email/phone/call-sign | [person](modules/person.md) | `is_primary`; email `provider`, phone `country` (derived) | scoped through the holder |
| `OF_EMAIL_TYPE` / `OF_PHONE_TYPE` | email/phone → type catalog | [person](modules/person.md) | — | — |
| `REACHABLE_ON` | `PersonEmail`/`PersonPhone` → `Platform` | [person](modules/person.md) | XOR phone/email, `is_primary`, `verified_at` | — (messenger-category only) |
| `HOLDS_ACCOUNT` | `Person` → `PersonSocialAccount` | [person](modules/person.md) | **`source` (self_declared/operator_verified/imported) + `confidence` (confirmed/probable/possible)** — a sourced, weighted attribution claim | `status`; scoped through the holder; see §4.4 |
| `PARTNERED_WITH` | `Person` → `Person` | [person](modules/person.md) | symmetric (canonical pair); `status ∈ engaged\|married\|divorced\|widowed\|annulled\|dissolved` | **yes — `effective_from`/`effective_to`** |
| `KIN_PARENT_OF` | `Person` → `Person` | [person](modules/person.md) | directional `parent_of`; `status ∈ active\|disestablished` | siblings derived, not stored |
| `GUARDIAN_OF` | `Person` → `Person` | [person](modules/person.md) | `relation_code`, `status` | **yes — effective interval** |
| `SPONSOR_OF` | `Person` → `Person` | [person](modules/person.md) | catalog `relation_code` (godparent/advisor/mentor) | **yes — effective interval** |
| `NEXT_OF_KIN` | `Person` → `Person` | [person](modules/person.md) | in-directory nomination, `relation_code`, `priority` | — |
| `ASSOCIATED_WITH` | `Person` → `Person` | [person](modules/person.md) | symmetric; `kind ∈ associate\|coi\|no_contact`, `relation_code` | — (COI / no-contact) |
| `SOCIAL_TIE` *(deferred — not built)* | `Person` → `Person` | [person](modules/person.md) | `status ∈ active\|archived` | scoped friend/follower tie, **cut from M14** (no consumer / no source / redundant with `ASSOCIATED_WITH`); see decisions.md D-PersonRelationships |
| `ISSUED_BY` | `Order` → `Unit` | [order](modules/order.md) | — | anchors authz + RLS |
| `TARGETS` | `OrderItem` → `Person`(+`Unit`/`Position`/`Rank`) | [order](modules/order.md) | `effect`, `effective_from/to` (legal metadata) | — |
| `CAUSED_BY` (provenance) | `Membership`/rank change → `OrderItem` | [membership](modules/membership.md) / [order](modules/order.md) | `order_item_id` | the наказ that authorized the change |
| `REVOKED_BY` | `Order` → `Order` | [order](modules/order.md) | — | the revoking order (legal trail) |
| `TRANSLATES` | `Translation` → entity (polymorphic) | [localization](modules/localization.md) | `entity_type`, `field`, `locale` | no FK; kept consistent by event subscription |
| `LANGUAGE_SUBGROUP_OF` *(M18)* | `Languoid` → `Languoid` | [language](modules/language.md) | structural; `family_code` denormalized | strict tree, a containment FK — *not* a reified Link (closure is `ANCESTOR_OF`-style, `language_languoid_closure`) |
| `WRITTEN_IN` *(M18)* | `Languoid` → `WritingSystem` | [language](modules/language.md) | `is_primary`; RID link (`13,2,1`) | `language_writing_systems`; import-loaded from CLDR |
| `SPEAKS` *(M18)* | `Person` → `Languoid` (level=language) | [language](modules/language.md)/[person](modules/person.md) | `cefr_level`, `is_native`; `pii:basic` | `person_languages` (person RID link `6,2,8`); scoped through the holder; purge-erased; `level='language'` enforced by composite FK |
| `HAS_ETHNICITY` *(M31)* | `Person` → declared ethnicity (`PersonEthnicityType` code) | [person](modules/person.md) | `legal_basis` (NOT NULL, Art. 9), `status`, `source`/`confidence`; **`pii:special`** | `person_ethnicities` (person RID link `6,2,9`); the declared code is **envelope-encrypted + blind-indexed** (no plaintext FK); self-declared only; **crypto-erased** on purge; D-PhysicalIdentity / D-SpecialPII |
| `LIVES_AT` *(M32)* | `Person` → `Location` | [person](modules/person.md)/[location](modules/location.md) | `role∈{home,work,mailing,other}`, `valid_from`/`valid_to`, `is_primary` (one active/person), `privacy_seeking`, `source`/`confidence`; **`pii:contact`** | `person_addresses` (person RID link `6,2,10`) → `location_locations` (M19); purge **hard-deletes**; distinct from `person_residences` (legal residence); D-PersonAddresses |
| `PARTY_MEMBERSHIP` *(M33)* | `Person` → party (external org / free-text) | [person](modules/person.md) | `role`, `valid_from`/`valid_to`, `legal_basis` (NOT NULL, Art. 9), `status`, `source`/`confidence`; **`pii:special`** | `person_party_memberships` (person RID link `6,2,11`); the party identity is **envelope-encrypted + blind-indexed** (political opinion; no plaintext FK); **crypto-erased** on purge; separate from the inferred M35 political leaning (never merged); D-InstitutionalTies |
| `GOVERNMENT_POSITION` *(M33)* | `Person` → government body (external org / free-text) | [person](modules/person.md) | `title`, `body`, optional polymorphic `org_id`, `country_id`, `level`, `role_type`, `valid_from`/`valid_to`, **`pep_trigger`** (persists post-office), `source`/`confidence`; **`pii:basic`** | `person_government_positions` (person RID link `6,2,12`); feeds the M34 PEP check; purge **hard-deletes**; D-InstitutionalTies |
| `LOBBYING_REL` *(M33)* | `Person` (registrant) → client / legislative body | [person](modules/person.md) | `registrant`, `client`, `legislative_body`, `issues[]`, `filing_id`, `source_url`, `valid_from`/`valid_to`, `source`/`confidence`; **`pii:basic`** | `person_lobbying_relationships` (person RID link `6,2,13`); public-registry data; purge **hard-deletes**; D-InstitutionalTies |
| `OFFICIAL_LANGUAGE` *(M18)* | `Unit` → `Languoid` | [language](modules/language.md)/[tenant](modules/tenant.md) | `is_official` | `tenant_unit_languages` (tenant RID link `4,2,2`) |
| `LOCALE_OF` *(M18)* | `Locale` → `Languoid` | [language](modules/language.md)/[localization](modules/localization.md) | canonical language of a locale | `i18n_locale_languages` (i18n RID link `2,2,1`); one per locale |
| `STUDIED_AT` *(M20)* | `Person` → `EducationInstitution` (opt. unit/group) | [education](modules/education.md) | `degree_level`, field, status, qualification; `pii:basic` | **temporal** (effective-dated); `person_education_enrollments` (education RID link `14,2,2`); purge-erased; mirrors `MEMBER_OF` |
| `RESIDED_IN_DORMITORY` *(M20)* | `Person` → `EducationBuilding` | [education](modules/education.md) | room, period; `pii:contact` | **temporal**; `person_dormitory_stays` (`14,2,3`); purge-erased |
| `HOLDS_EDUCATION_POSITION` *(M20)* | `Person` → `EducationPosition` | [education](modules/education.md) | one-holder | **temporal**; `education_appointments` (`14,2,4`); mirrors `FILLS` |
| `CURRICULUM_ITEM` / `COURSE_PREREQUISITE` *(M20 ref)* | `CurriculumVersion` → `Course` / `Course` → `Course` | [education](modules/education.md) | required/elective; prereq kind | reified junction links (RID `14,2,5/6`); prereq is cycle-guarded |
| `AUTHORED_PUBLICATION` / `MEMBER_OF_RESEARCH_GROUP` / `HOLDS_GRANT` *(M20 ref)* | `Person` → `Publication` / `ResearchGroup` / `Grant` | [education](modules/education.md) | order/role; `pii:basic` | **temporal**; person links (RID `14,2,7/8/9`); purge-erased |
| `MEMBER_OF_GOVERNANCE_BODY` / `AWARDED_QUALIFICATION` / `AWARDED_SCHOLARSHIP` *(M20 ref)* | `Person` → `GovernanceBody` / `Qualification` / `Scholarship` | [education](modules/education.md) | role; diploma award; `pii:basic` | **temporal**; person links (RID `14,2,10/11/12`); purge-erased |
| `SPONSOR_OF` (education context) *(M20)* | `Person` → `Person` | [person](modules/person.md) | optional `enrollment_id` ref + `education_role ∈ professor/tutor/curator/advisor` | **extends M14 `SPONSOR_OF`** — two nullable columns, no new link type (D-Education) |
| `HOLDS_COMPANY_POSITION` *(M21)* | `Person` → `CompanyPosition` | [company](modules/company.md) | one-holder; `pii:basic` | **temporal**; `company_appointments` (`15,2,1`); mirrors `FILLS`; purge-erased |
| `FOUNDED` *(M21)* | `Person`\|`Company` → `Company` | [company](modules/company.md) | founder (person or company); **polymorphic holder** (`holder_kind`+`holder_id`, no FK) | `company_foundings` (`15,2,2`); person-holder rows `pii:basic`, purge-erased |
| `OWNS_STAKE` *(M21)* | `Person`\|`Company` → `Company` | [company](modules/company.md) | stake %; **polymorphic holder** | **temporal**; `company_shareholdings` (`15,2,3`); company-holder edges form the ownership DAG; person-holder rows purge-erased |
| `BENEFICIARY_OF` *(M21)* | `Person` → `Company` | [company](modules/company.md) | ultimate %, declared-vs-computed; `pii:basic` | UBO; `company_beneficiaries` (`15,2,4`); purge-erased; computed traversal is DS-47 |
| `SUCCEEDED_BY` *(M21)* | `Company` → `Company` | [company](modules/company.md) | M&A/reorganization lineage (`kind`) | `company_successions` (`15,2,5`) |
| `BRANCH_OF` *(M21)* | `Company` → `Company` | [company](modules/company.md) | non-independent sub-unit | `company_branches` (`15,2,6`); distinct from a subsidiary |
| `HAS_INDUSTRY` *(M21)* | `Company` → `CompanyIndustryClass` | [company](modules/company.md) | M:N, one primary | `company_industry_assignments` (`15,2,7`) |
| `LOCATED_AT` *(M21)* | `Company` → `Location` | [company](modules/company.md) | `role ∈ registered\|operating\|branch` | `company_locations` (`15,2,8`) → shared M19 `Location` |
| `CLASSIFIED_AS` *(M22)* | `Unit` → `Taxon` | [religion](modules/religion.md) | M:N, one primary (`is_primary` partial-unique); `source`/`confidence` | reified `religion_org_classifications` (`16,2,1`); a unit's faith classification tags; **never an authz input** |
| `CLERGY_CREDENTIAL` *(M23)* | `Person` → `ClergyGrade` (in an org `Unit`) | [religion](modules/religion.md) | reified `religion_clergy_credentials` (`16,2,2`); `granted_on`, conferrer, `status ∈ active\|suspended\|revoked`, `source`/`confidence` | **temporal**; indelible where sacramental; **never an authz input** (parallels `HOLDS_RANK`) |
| `AFFILIATED_WITH` *(M24)* | `Person` → religion/tradition/community `Unit` | [religion](modules/religion.md) | reified `religion_affiliations` (`16,2,3`); `affiliation_type`, **`pii:special`** envelope-encrypted value + blind index, `source`/`confidence` | **temporal**; crypto-erased on purge; never an authz input; D-ReligiousAffiliation / D-SpecialPII |
| `SITE_OF` *(M25)* | `Unit` → `Location` | [religion](modules/religion.md) | reified `religion_sites` (`16,2,4`); `site_type`, `visibility`, `public_precision`, `is_primary` (one per unit) | shared `Location` by FK; precision coarsened **app-side** at read time (H3 dropped — `domain.Coarsen` rounding) |
| `MANUFACTURED_BY` *(M26)* | `VehicleBrand` → `Company` | `vehicle` | manufacturer of a marque | **temporal** (`effective_from`/`effective_to`) — changes with acquisitions |
| `REGISTERED_TO` *(M26)* | `Vehicle` → `Person`\|`Company` | `vehicle` | **polymorphic owner** (person XOR company); `country` → `geo_countries`, `subdivision` → `geo_places` (plate region, placetype=region), `registration_number` (unique active per country), `number_type` | **temporal** + `status`; the ownership+plate record (re-registration = new row); person-owned rows `pii:basic`, holder-scoped, purge-erased; D-Vehicles |
| `HELD_BY` *(M44)* | `Account` → `Person`\|`Company` | [finance](modules/finance.md) | reified `finance_account_holders` (`19,2,1`); **polymorphic holder** (person XOR company, `holder_id` text, no FK); `role` primary\|joint\|authorized_signer | **temporal**; the ownership edge (joint/corporate accounts fall out of it); person-holder rows `pii:basic`, holder-scoped; a person purge crypto-erases solely-held accounts + cards; D-Finance |
| `HAS_ETHNICITY` *(planned, M31)* | `Person` → `EthnicityType` | [person](modules/person.md) | declared-only; **`pii:special`** envelope-encrypted + `legal_basis` | crypto-erased on purge; never inferred; D-PhysicalIdentity |
| `RESIDES_AT` *(planned, M32)* | `Person` → `Location` | [person](modules/person.md) | `role ∈ {home,work,mailing,other}`, `is_primary`, `privacy_seeking`; `pii:contact` | **temporal**; `person_addresses` → shared M19 `Location`; work address derivable from unit; purge-erased; D-PersonAddresses |
| `PARTY_MEMBER_OF` *(planned, M33)* | `Person` → `ExternalOrganization` | [person](modules/person.md) | role/dates; **`pii:special`** (Art. 9) envelope + `legal_basis` | **temporal**; `source`/`confidence`; never merged with inferred leaning; D-InstitutionalTies |
| `HOLDS_GOVERNMENT_POSITION` *(planned, M33)* | `Person` → `ExternalOrganization` | [person](modules/person.md) | title/body/level; **`pep_trigger`** (auto-true, persists post-office); `pii:basic` | **temporal**; feeds M34 PEP; D-InstitutionalTies |
| `LOBBYING_FOR` *(planned, M33)* | `Person` → `ExternalOrganization` | [person](modules/person.md) | registrant/client, issues[], filing_id, source_url; `pii:basic` | `source`/`confidence`; D-InstitutionalTies |
| `SERVED_IN` *(planned, M33)* | `Person` → `ExternalOrganization` (military) | [person](modules/person.md) / [membership](modules/membership.md) | reuse the membership shape + rank; `units[]`/`deployments[]`/`discharge_type`/`clearance_level` (latter two `pii:sensitive`) | **temporal**; foreign/historical military service against an external-org stub; D-InstitutionalTies |
| `HELD_BY` *(planned, M44)* | `BankAccount` → `Person`\|`Company` | [finance](modules/finance.md) | reified `finance_account_holders` (`19,2,1`); **polymorphic holder** (`holder_kind`+`holder_id`, no FK — F-014); `role ∈ {primary,joint,authorized_signer}` | **temporal**; joint/corporate accounts; person-holder rows `pii:basic`, holder-scoped, purge-erased; a card→account is a **containment FK, not a Link**; D-Finance |

The `Assignment` is the centerpiece and deserves emphasis: an ontology would model it as a **reified
Link** `(subject, role, target_unit, scope, graph)`. Two non-obvious semantics
([authorization](modules/authorization.md)):

- `scope=subtree` cascades to all descendants **via the graph's closure** (union across ancestors,
  and across graphs); `scope=unit` grants children **nothing — not even read**.
- `target_unit` is **independent of where the subject sits** — the edge is not "subject's placement";
  it is an explicit grant pointing anywhere in the graph.

---

## 3. Actions

All writes are named, auditable Actions; the [order](modules/order.md) module + the in-process event
bus + the [audit](modules/audit.md) log already form an Action-shaped spine. Each Action is addressable
by an Action RID (kind=action; the specific action *name*, e.g. `issue_order` / `grant_assignment`,
lives in `audit_log.action`), and the
[audit](modules/audit.md) row that records it is keyed by that RID — so the audit log *is* the action
ledger ([Identifier scheme](#identifier-scheme-rids)).

- **Direct CRUD:** `CreateUnit`, `AddEdge`/`RemoveEdge`, `TransitionUnit`, `CreatePerson`,
  `AssignRank`, `CreatePosition`/`AbolishPosition`, `CreateMembership`/`EndMembership`,
  `AttachDocument`/`AttachPersonalCode`, `UpsertEmail`/`UpsertPhone`/`UpsertCallSign` (+ their
  deletes), `CreateRole`, `GrantAssignment`/`RevokeAssignment`,
  `GrantInstanceAdmin`, `CreateAccount`/`LinkExternalIdentity`, rank/locale/catalog edits.
- **Planned (M16–M26):** `RunImport` (**M16, D-Hermenea**) — oikumenea's `POST /import/{objectType}`
  applies a bulk **code-keyed upsert** in one txn, emitted as a **`system`-actor** audited Action (the
  ingest≠edit boundary); the scheduler/queue Actions (`ScheduleJob`/`RunJob`) live in **hermenea's own**
  ledger (`worker_jobs`/`import_runs`), not oikumenea's audit log;
  `CreateLanguoid`/`ImportLanguageScheme`, `UpsertPersonLanguage` (M18);
  `CreateLocation`/`UpdateLocation`/`DeleteLocation` (M19, built);
  `CreateInstitution`/`CreateEnrollment`/`RecordDormStay`/`AppointEducationPosition` (M20);
  `CreateCompany`/`RecordShareholding`/`RecordBeneficiary`/`AppointCompanyPosition` (M21);
  `ConferCredential`/`SuspendCredential`/`AppointClergy` (M23), `RecordAffiliation` (M24),
  `AddSite`/`AddSchedule`/`AddAlias` (M25, built) — the religion vertical (D-Religion);
  `CreateVehicle`/`RegisterVehicle` (transfer-as-history)/`CloseRegistration` + vehicle-catalog edits
  (M26, built — D-Vehicles; plate region → the WOF `geo_places` gazetteer, D-GeoPlaces);
  `RecodeUnit` (**M28, D-UnitCodeLifecycle**) — the audited set/correct/clear of a unit `code`,
  recorded in the append-only `tenant_unit_code_events` ledger (RID slot `4,1,4`); `CreateUnit` now
  accepts an optional code.
- **Planned (M29–M37 — OSINT-enrichment cluster):** `MergePerson` (**M29, D-OverlayFoundation**) — the
  audited manual promote/merge of a `provisional` person into a canonical one, re-homing edges
  (`PersonMerged` event); `CreateExternalOrganization` (M30); `RecordEthnicity`/`RecordPhysicalDescription`
  (M31); `RecordAddress` (M32); `RecordPartyMembership`/`RecordGovernmentPosition`/`RecordLobbying`/
  `RecordForeignService`/`AddExternalReference` (M33); `CheckWatchlists` (**M34** — the live-lookup that
  persists only match-metadata) + `RecordRegulatorySanction` (M34); `RecordCryptoWallet`/
  `RecordPersonality`/`RecordPoliticalLeaning` (M35); `RecordHealthRecord`/`RecordInsurance` (M36);
  `RecordLoginEvent` (M37, `system`-actor). Every special-category write carries a `legal_basis` and is
  fully audited (D-OverlayFoundation / D-SpecialPII).
- **Order-driven effects (the strongest ontology fit):** `IssueOrder` is one Action whose effects are
  **emitted as domain events** (`AppointmentOrdered`, `RemovalOrdered`, `RankChangeOrdered`) that
  membership/person subscribers apply **in the same transaction**, citing `order_item_id` provenance.
  It is **all-or-nothing**: any effect that violates an invariant rolls back the whole issue.
  `RevokeOrder` flips legal status only and does **not** auto-reverse applied effects (undo is a new
  revoking order) — note this is a deliberate non-reversal, unlike the ontology default.
- **Cross-module rule:** cross-module **mutations are events**; cross-module **queries are direct
  interface calls** ([decisions.md](architecture/decisions.md)). So Actions cross module boundaries as
  events (keeping the monolith extraction-ready); reads do not.
- **Audit:** every permission-sensitive Action writes an `AuditEntry` in-transaction (`outcome ∈
  {success,denied,error}`); system-initiated effects record `actor_type='system'` with a `subsystem`
  (`bootstrap`, `event-subscriber`, `closure-rebuild`, …), correlated by `request_id`.

---


### 3.1 The action-type catalog (D-ActionTypes)

Until review-2026-09 R-29, the Action *names* above were free text in `audit_log.action` with no
read-time contract (D-Audit deliberately keeps the Action RID at the generic kind=action
`type_code 0`). **D-ActionTypes** turns them into a checked, enumerable catalog: the binding
source is the Go registry `pkg/action` (mirrored to the machine table below and served by
`AuditService.listActionTypes`). Every audited write is validated against it at write time
(`audit.Service.Record`), so a typo or an un-catalogued action fails a test rather than drifting.
The [order](modules/order.md) module is the **reference pattern** — a typed action
(`order_order_types.effect`) whose all-or-nothing effects apply via domain events in one
transaction; `order.issue` / `order.revoke` are its catalog entries. This table is generated
from `pkg/action` and coherence-checked (`pkg/action/catalog_doc_test.go`); it is **expand-only**.
Beyond the four columns below, an action optionally carries a **parameter schema** (R-29): where a
row names its Conjure request type (`ActionType.RequestType`), `listActionTypes` returns the argument
list (`{name, type, required, docs}`) **derived from the Conjure IR** (`tools/genactionparams`), so it
cannot drift from the contract. Descriptive only (discoverability), not write-time validation.

<!-- BEGIN action-catalog (generated from pkg/action — do not hand-edit; keep in sync via the coherence test) -->
| Action code | Service | Target type | Permission |
|---|---|---|---|
| `account.create` | account | `account` | `instance.admin.manage` |
| `account.disable` | account | `account` | `instance.admin.manage` |
| `assignment.grant` | authz | `assignment` | `assignment.grant` |
| `assignment.revoke` | authz | `assignment` | `assignment.revoke` |
| `closure.rebuild` | tenant | `graph` | `closure.rebuild` |
| `closure.verify` | tenant | `graph` | `unit.edges.manage` |
| `color.upsert` | platform | `color` | `color.manage` |
| `company.appointment.end` | company | `company` | `company.position.manage` |
| `company.appointment.fill` | company | `company` | `company.position.manage` |
| `company.beneficiary.record` | company | `company` | `company.manage` |
| `company.branch.record` | company | `company` | `company.manage` |
| `company.create` | company | `company` | `company.manage` |
| `company.delete` | company | `company` | `company.manage` |
| `company.founding.record` | company | `company` | `company.manage` |
| `company.industry-class.upsert` | company | `company` | `company.catalog.manage` |
| `company.industry.assign` | company | `company` | `company.manage` |
| `company.legal-form.upsert` | company | `company` | `company.catalog.manage` |
| `company.location.add` | company | `company` | `company.manage` |
| `company.position.abolish` | company | `company` | `company.position.manage` |
| `company.position.create` | company | `company` | `company.position.manage` |
| `company.position.update` | company | `company` | `company.position.manage` |
| `company.registration-scheme.upsert` | company | `company` | `company.catalog.manage` |
| `company.registration.add` | company | `company` | `company.manage` |
| `company.registration.update` | company | `company` | `company.manage` |
| `company.shareholding.record` | company | `company` | `company.manage` |
| `company.succession.record` | company | `company` | `company.manage` |
| `company.update` | company | `company` | `company.manage` |
| `document.create` | document | `document` | `document.create` |
| `document.delete` | document | `document` | `document.delete` |
| `document.person.erase` | document | `person` | `person.purge` |
| `document.type.create` | document | `document_type` | `document.type.manage` |
| `document.type.update` | document | `document_type` | `document.type.manage` |
| `document.update` | document | `document` | `document.update` |
| `domain.create` | tenant | `domain` | `domain.manage` |
| `domain.update` | tenant | `domain` | `domain.manage` |
| `education.accreditation-event.create` | education | `education` | `education.catalog.manage` |
| `education.accreditation-event.update` | education | `education` | `education.catalog.manage` |
| `education.appointment.end` | education | `education` | `education.position.manage` |
| `education.appointment.fill` | education | `education` | `education.position.manage` |
| `education.building.create` | education | `education` | `education.manage` |
| `education.building.delete` | education | `education` | `education.manage` |
| `education.building.update` | education | `education` | `education.manage` |
| `education.course-prerequisite.create` | education | `education` | `education.manage` |
| `education.course.create` | education | `education` | `education.manage` |
| `education.course.update` | education | `education` | `education.manage` |
| `education.curriculum-item.create` | education | `education` | `education.manage` |
| `education.curriculum-item.update` | education | `education` | `education.manage` |
| `education.curriculum-version.create` | education | `education` | `education.manage` |
| `education.curriculum-version.update` | education | `education` | `education.manage` |
| `education.dormitory-stay.create` | education | `education` | `education.manage` |
| `education.dormitory-stay.delete` | education | `education` | `education.manage` |
| `education.dormitory-stay.update` | education | `education` | `education.manage` |
| `education.enrollment.create` | education | `education` | `education.enrollment.manage` |
| `education.enrollment.delete` | education | `education` | `education.enrollment.manage` |
| `education.enrollment.update` | education | `education` | `education.enrollment.manage` |
| `education.governance-body.create` | education | `education` | `education.manage` |
| `education.governance-body.update` | education | `education` | `education.manage` |
| `education.governance-membership.create` | education | `education` | `education.manage` |
| `education.governance-membership.update` | education | `education` | `education.manage` |
| `education.grant-holding.create` | education | `education` | `education.manage` |
| `education.grant-holding.update` | education | `education` | `education.manage` |
| `education.grant.create` | education | `education` | `education.manage` |
| `education.grant.update` | education | `education` | `education.manage` |
| `education.group.create` | education | `education` | `education.manage` |
| `education.group.delete` | education | `education` | `education.manage` |
| `education.group.update` | education | `education` | `education.manage` |
| `education.institution-kind.upsert` | education | `education` | `education.catalog.manage` |
| `education.institution.create` | education | `education` | `education.manage` |
| `education.institution.delete` | education | `education` | `education.manage` |
| `education.institution.update` | education | `education` | `education.manage` |
| `education.policy.create` | education | `education` | `education.manage` |
| `education.policy.update` | education | `education` | `education.manage` |
| `education.position.abolish` | education | `education` | `education.position.manage` |
| `education.position.create` | education | `education` | `education.position.manage` |
| `education.position.update` | education | `education` | `education.position.manage` |
| `education.program.create` | education | `education` | `education.manage` |
| `education.program.update` | education | `education` | `education.manage` |
| `education.publication-authorship.create` | education | `education` | `education.manage` |
| `education.publication-authorship.update` | education | `education` | `education.manage` |
| `education.publication.create` | education | `education` | `education.manage` |
| `education.publication.update` | education | `education` | `education.manage` |
| `education.qualification-award.create` | education | `education` | `education.manage` |
| `education.qualification-award.update` | education | `education` | `education.manage` |
| `education.qualification.create` | education | `education` | `education.manage` |
| `education.qualification.update` | education | `education` | `education.manage` |
| `education.research-centre.create` | education | `education` | `education.manage` |
| `education.research-centre.update` | education | `education` | `education.manage` |
| `education.research-group.create` | education | `education` | `education.manage` |
| `education.research-group.update` | education | `education` | `education.manage` |
| `education.research-membership.create` | education | `education` | `education.manage` |
| `education.research-membership.update` | education | `education` | `education.manage` |
| `education.scholarship-award.create` | education | `education` | `education.manage` |
| `education.scholarship-award.update` | education | `education` | `education.manage` |
| `education.scholarship.create` | education | `education` | `education.manage` |
| `education.scholarship.update` | education | `education` | `education.manage` |
| `external-org.create` | external_organization | `external_organization` | `externalorg.manage` |
| `external-org.delete` | external_organization | `external_organization` | `externalorg.manage` |
| `external-org.kind.upsert` | external_organization | `external_organization` | `externalorg.manage` |
| `external-org.merge` | external_organization | `external_organization` | `externalorg.manage` |
| `external-org.update` | external_organization | `external_organization` | `externalorg.manage` |
| `finance.account-type.upsert` | finance | `finance` | `finance.catalog.manage` |
| `finance.account.create` | finance | `finance` | `finance.manage` |
| `finance.account.delete` | finance | `finance` | `finance.manage` |
| `finance.account.update` | finance | `finance` | `finance.manage` |
| `finance.card-network.upsert` | finance | `finance` | `finance.catalog.manage` |
| `finance.card.add` | finance | `finance` | `finance.manage` |
| `finance.card.delete` | finance | `finance` | `finance.manage` |
| `finance.card.update` | finance | `finance` | `finance.manage` |
| `finance.holder.add` | finance | `finance` | `finance.manage` |
| `finance.holder.end` | finance | `finance` | `finance.manage` |
| `finance.holdings.erase` | finance | `finance` | `finance.manage` |
| `graph.create` | tenant | `graph` | `graph.manage` |
| `graph.delete` | tenant | `graph` | `graph.manage` |
| `graph.update` | tenant | `graph` | `graph.manage` |
| `identity.link` | account | `external_identity` | `instance.admin.manage` |
| `identity.unlink` | account | `external_identity` | `instance.admin.manage` |
| `import.colors` | platform | `colors` | `import.manage` |
| `import.ethnicity-scheme` | platform | `ethnicity-scheme` | `import.manage` |
| `import.external-organizations` | platform | `external-organizations` | `import.manage` |
| `import.geo-countries` | platform | `geo-countries` | `import.manage` |
| `import.geo-places` | platform | `geo-places` | `import.manage` |
| `import.language-scheme` | platform | `language-scheme` | `import.manage` |
| `import.language-scripts` | platform | `language-scripts` | `import.manage` |
| `import.person-regulatory-sanctions` | platform | `person-regulatory-sanctions` | `import.manage` |
| `import.religion-scheme` | platform | `religion-scheme` | `import.manage` |
| `import.translations` | platform | `translations` | `import.manage` |
| `instance.admin.grant` | authz | `instance_admin` | `instance.admin.manage` |
| `instance.admin.revoke` | authz | `instance_admin` | `instance.admin.manage` |
| `legal-basis.upsert` | platform | `color` | `legal-basis.manage` |
| `locale.add` | i18n | `locale` | `locale.manage` |
| `locale.update` | i18n | `locale` | `locale.manage` |
| `location.create` | location | `location` | `location.create` |
| `location.delete` | location | `location` | `location.update` |
| `location.update` | location | `location` | `location.update` |
| `membership.create` | membership | `membership` | `membership.create` |
| `membership.end` | membership | `membership` | `membership.update` |
| `membership.fill` | membership | `membership` | `membership.create` |
| `order.create` | order | `order` | `order.create` |
| `order.issue` | order | `order` | `order.issue` |
| `order.revoke` | order | `order` | `order.revoke` |
| `order.type.create` | order | `order_type` | `order.type.manage` |
| `order.type.update` | order | `order_type` | `order.type.manage` |
| `order.update` | order | `order` | `order.create` |
| `organization.create` | tenant | `organization` | `organization.create` |
| `organization.transition` | tenant | `organization` | `organization.lifecycle` |
| `organization.update` | tenant | `organization` | `organization.update` |
| `person.address.delete` | person | `person` | `person.update` |
| `person.address.upsert` | person | `person` | `person.update` |
| `person.association.upsert` | person | `person` | `person.update` |
| `person.call-sign.delete` | person | `person` | `person.update` |
| `person.call-sign.upsert` | person | `person` | `person.update` |
| `person.citizenship.delete` | person | `person` | `person.update` |
| `person.citizenship.upsert` | person | `person` | `person.update` |
| `person.create` | account | `person` | `person.create` |
| `person.crypto_wallet.delete` | person | `person` | `person.update` |
| `person.crypto_wallet.upsert` | person | `person` | `person.update` |
| `person.deactivate` | person | `person` | `person.update` |
| `person.distinguishing_mark.delete` | person | `person` | `person.update` |
| `person.distinguishing_mark.upsert` | person | `person` | `person.update` |
| `person.email.delete` | person | `person` | `person.update` |
| `person.email.upsert` | person | `person` | `person.update` |
| `person.ethnicity.add` | person | `person` | `person.update` |
| `person.ethnicity.delete` | person | `person` | `person.update` |
| `person.ethnicity.erase` | person | `person` | `person.update` |
| `person.ethnicity.update` | person | `person` | `person.update` |
| `person.ethnicity_type.upsert` | person | `person` | `person.update` |
| `person.external_reference.delete` | person | `person` | `person.update` |
| `person.external_reference.upsert` | person | `person` | `person.update` |
| `person.government_position.delete` | person | `person` | `person.update` |
| `person.government_position.upsert` | person | `person` | `person.update` |
| `person.guardianship.upsert` | person | `person` | `person.update` |
| `person.health_record.delete` | person | `person` | `person.update` |
| `person.health_record.upsert` | person | `person` | `person.update` |
| `person.insurance.delete` | person | `person` | `person.update` |
| `person.insurance.upsert` | person | `person` | `person.update` |
| `person.kinship.upsert` | person | `person` | `person.update` |
| `person.language.delete` | person | `person` | `person.update` |
| `person.language.upsert` | person | `person` | `person.update` |
| `person.lobbying.delete` | person | `person` | `person.update` |
| `person.lobbying.upsert` | person | `person` | `person.update` |
| `person.merge` | person | `person` | `person.merge` |
| `person.messenger-link.delete` | person | `person` | `person.update` |
| `person.messenger-link.upsert` | person | `person` | `person.update` |
| `person.name_alias.add` | person | `person` | `person.update` |
| `person.name_alias.delete` | person | `person` | `person.update` |
| `person.name_variant.delete` | person | `person` | `person.update` |
| `person.name_variant.upsert` | person | `person` | `person.update` |
| `person.next-of-kin.upsert` | person | `person` | `person.update` |
| `person.partnership.upsert` | person | `person` | `person.update` |
| `person.party_membership.delete` | person | `person` | `person.update` |
| `person.party_membership.upsert` | person | `person` | `person.update` |
| `person.personality.delete` | person | `person` | `person.update` |
| `person.personality.upsert` | person | `person` | `person.update` |
| `person.phone.delete` | person | `person` | `person.update` |
| `person.phone.upsert` | person | `person` | `person.update` |
| `person.physical_description.delete` | person | `person` | `person.update` |
| `person.physical_description.upsert` | person | `person` | `person.update` |
| `person.political_leaning.delete` | person | `person` | `person.update` |
| `person.political_leaning.set` | person | `person` | `person.update` |
| `person.provisional.create` | person | `person` | `person.update` |
| `person.purge` | person | `person` | `person.purge` |
| `person.rank.assign` | person | `person` | `person.rank.assign` |
| `person.reactivate` | person | `person` | `person.update` |
| `person.regulatory_sanction.delete` | person | `person` | `person.update` |
| `person.regulatory_sanction.upsert` | person | `person` | `person.update` |
| `person.relationship.delete` | person | `person` | `person.update` |
| `person.residence.delete` | person | `person` | `person.update` |
| `person.residence.upsert` | person | `person` | `person.update` |
| `person.social-account.delete` | person | `person` | `person.update` |
| `person.social-account.upsert` | person | `person` | `person.update` |
| `person.sponsorship.upsert` | person | `person` | `person.update` |
| `person.update` | person | `person` | `person.update` |
| `person.watchlist.check` | person | `person` | `person.update` |
| `personal-code-scheme.create` | document | `document` | `personal-code-scheme.manage` |
| `personal-code-scheme.update` | document | `document` | `personal-code-scheme.manage` |
| `personal-code.create` | document | `personal_code` | `personal-code.create` |
| `personal-code.delete` | document | `personal_code` | `personal-code.delete` |
| `personal-code.update` | document | `personal_code` | `personal-code.update` |
| `position.abolish` | membership | `position` | `position.update` |
| `position.create` | membership | `position` | `position.create` |
| `position.update` | membership | `position` | `position.update` |
| `rank.category.create` | rank | `rank_category` | `rank.scheme.manage` |
| `rank.category.delete` | rank | `rank_category` | `rank.scheme.manage` |
| `rank.category.update` | rank | `rank_category` | `rank.scheme.manage` |
| `rank.rank.create` | rank | `rank` | `rank.scheme.manage` |
| `rank.rank.delete` | rank | `rank` | `rank.scheme.manage` |
| `rank.rank.update` | rank | `rank` | `rank.scheme.manage` |
| `rank.scheme.import` | rank | `rank_system` | `rank.scheme.manage` |
| `rank.system.create` | rank | `rank_system` | `rank.scheme.manage` |
| `rank.system.delete` | rank | `rank_system` | `rank.scheme.manage` |
| `rank.system.update` | rank | `rank_system` | `rank.scheme.manage` |
| `rank.type.create` | rank | `rank_type` | `rank.scheme.manage` |
| `rank.type.delete` | rank | `rank_type` | `rank.scheme.manage` |
| `rank.type.update` | rank | `rank_type` | `rank.scheme.manage` |
| `religion.affiliation-type.upsert` | religion | `religion` | `affiliation.manage` |
| `religion.affiliation.add` | religion | `religion` | `affiliation.manage` |
| `religion.affiliation.delete` | religion | `religion` | `affiliation.manage` |
| `religion.affiliation.erase` | religion | `religion` | `affiliation.manage` |
| `religion.affiliation.update` | religion | `religion` | `affiliation.manage` |
| `religion.alias.add` | religion | `religion` | `site.manage` |
| `religion.alias.delete` | religion | `religion` | `site.manage` |
| `religion.classification.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.clergy-credential.add` | religion | `religion` | `clergy.manage` |
| `religion.clergy-credential.update` | religion | `religion` | `clergy.manage` |
| `religion.clergy-grade.upsert` | religion | `religion` | `clergy.manage` |
| `religion.grade-category.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.office-type.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.org-classification.add` | religion | `religion` | `religionorg.manage` |
| `religion.org-classification.remove` | religion | `religion` | `religionorg.manage` |
| `religion.org-kind.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.org-policy.add` | religion | `religion` | `religionorg.manage` |
| `religion.org-policy.remove` | religion | `religion` | `religionorg.manage` |
| `religion.org-profile.set` | religion | `religion` | `religionorg.manage` |
| `religion.policy-kind.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.schedule.add` | religion | `religion` | `schedule.manage` |
| `religion.schedule.delete` | religion | `religion` | `schedule.manage` |
| `religion.service-type.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.site-type.upsert` | religion | `religion` | `site.manage` |
| `religion.site.add` | religion | `religion` | `site.manage` |
| `religion.site.delete` | religion | `religion` | `site.manage` |
| `religion.site.update` | religion | `religion` | `site.manage` |
| `religion.taxon-rank.upsert` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxon.create` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxon.delete` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxon.reparent` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxon.set-classifications` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxon.update` | religion | `religion` | `religion.catalog.manage` |
| `religion.taxonomy.rebuild-closure` | religion | `religion` | `religion.catalog.manage` |
| `religion.unit-type-override.set` | religion | `religion` | `religionorg.manage` |
| `role.create` | authz | `role` | `role.create` |
| `role.delete` | authz | `role` | `role.delete` |
| `role.update` | authz | `role` | `role.update` |
| `service-principal.disable` | account | `service_principal` | `service-principal.manage` |
| `service-principal.enable` | account | `service_principal` | `service-principal.manage` |
| `service-principal.grant` | authz | `principal_grant` | `service-principal.manage` |
| `service-principal.register` | account | `service_principal` | `service-principal.manage` |
| `service-principal.revoke` | authz | `principal_grant` | `service-principal.manage` |
| `service-principal.update` | account | `service_principal` | `service-principal.manage` |
| `translation.upsert` | i18n | `languoid` | `translation.manage` |
| `unit-kind.create` | tenant | `unit` | `unit-kind.manage` |
| `unit-kind.update` | tenant | `unit` | `unit-kind.manage` |
| `unit.create` | tenant | `unit` | `unit.create` |
| `unit.edge.add` | tenant | `unit` | `unit.edges.manage` |
| `unit.edge.remove` | tenant | `unit` | `unit.edges.manage` |
| `unit.language.delete` | tenant | `unit` | `unit.update` |
| `unit.language.upsert` | tenant | `unit` | `unit.update` |
| `unit.recode` | tenant | `unit` | `unit.recode` |
| `unit.transition` | tenant | `unit` | `unit.lifecycle` |
| `unit.update` | tenant | `unit` | `unit.update` |
| `vehicle.brand.upsert` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.create` | vehicle | `vehicle` | `vehicle.manage` |
| `vehicle.delete` | vehicle | `vehicle` | `vehicle.manage` |
| `vehicle.manufacturer.add` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.manufacturer.remove` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.model.upsert` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.number-type.upsert` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.register` | vehicle | `vehicle` | `vehicle.manage` |
| `vehicle.registration.close` | vehicle | `vehicle` | `vehicle.manage` |
| `vehicle.registrations.erase` | vehicle | `vehicle` | `vehicle.manage` |
| `vehicle.type.upsert` | vehicle | `vehicle` | `vehicle.catalog.manage` |
| `vehicle.update` | vehicle | `vehicle` | `vehicle.manage` |

<!-- END action-catalog -->

## 4. Ratified divergences from the ontology ideal

Each is framed: *what the textbook ontology rule wants → what go-oikumenea does → why it is ratified.*
These are **binding, decision-backed exceptions** ([D-Ontology](architecture/decisions.md)), not
defects or open gaps.

**4.1 Temporal Links vs soft-delete (the biggest gap).** Ontology wants `valid_from`/`valid_to` on
**every** Link, with history never overwritten. go-oikumenea instead uses `deleted_at` soft-delete +
`updated_at`, and reconstructs relationship history from [order](modules/order.md) provenance, domain
events, and the [audit](modules/audit.md) log. Notably the *membership* Link **does** carry
`effective_from`/`effective_to` (close to the ontology ideal), but most other Links (edges,
assignments) capture history as grant/revoke timestamps + audit, not as link-native bitemporal
validity. **Verdict:** real divergence, now **bounded and mandated** by
[**D-Temporal**](architecture/decisions.md#d-temporal--a-three-tier-link-history-classification-native-validity-by-default-plus-getobjecthistory-over-the-audit-ledger)
(review-2026-09 R-31), which replaces this open-ended "narrowing" with a **three-tier classification
every Link declares at design time**: (a) **native validity** — the default for relationship/state
Links, carrying `valid_from`/`valid_to` (the canonical pair) or the grandfathered
`effective_from`/`effective_to` · `granted_at`/`revoked_at` · `founded_on` · `awarded_on` equivalents;
(b) **object history** served from the audit ledger by `AuditService.getObjectHistory`; (c) an
explicit, **closed six-member** history-exempt set (reference/structural associations —
`locale_language`, `unit_language`, `written_in`, `curriculum_item`, `course_prerequisite`,
`has_ethnicity`). The thirteen previously-undated relationship Links were brought up to tier (a) by
folding `valid_from`/`valid_to` into each one's original `CREATE TABLE` (edit-in-place + DB rebuild,
the unreleased-build-out convention); a boot/CI drift guard (`temporal_tiers_test.go`) fails the build
if a new kind=Link type is unclassified or a tier-(a) Link's DDL lacks a validity column. As-of
*reconstruction* and full bitemporality (a second, transaction-time axis) remain named seams — not
implied.

**4.2 Rank/position modeled as Links yet barred from authority.** `HOLDS_RANK` and `FILLS` look like
ontology edges, and they are correctly modeled as relationships (not embedded columns). But the design
**forbids** branching authorization on them — authority comes *only* from `HAS_ROLE` assignments
([decisions.md](architecture/decisions.md), Rank ≠ permission). **Verdict:** good ontology hygiene;
the caution is for readers — do not mistake these directory Links for authorization edges.

**4.3 Closure = materialized derived Links.** `ANCESTOR_OF` is a denormalized, maintained projection
of `PARENT_OF`, not a source of truth. Ontology-wise it is a derived link set; integrity is guarded by
on-demand `verify`/`rebuild` + a `closure-drift` health reporter ([tenant](modules/tenant.md)).
**Verdict:** intentional performance denormalization; flag only so it is never edited directly.

**4.4 `source` is not a uniform column.** Ontology wants `source` on every Object and Link.
go-oikumenea tracks provenance richly but **non-uniformly**: `order_item_id` on changed rows,
`created_by`/`granted_by` on some Links, and the `actor`/`subsystem` on every `AuditEntry` — but no
single `source` field across all tables. The RID scheme partly closes this: every Object/Link/Action
RID self-declares its `service` and `kind`/`type` of origin (decoded from the packed UUIDv8), and the
**Action RID keys each audit row** to the write that produced it. **Verdict:** partial gap; provenance is fully recoverable
via the RID + in-transaction audit, so a uniform `source` column would be redundant. The **one
deliberate exception** is the `HOLDS_ACCOUNT` link (D-PersonSocialChannels): a social-account
attribution carries explicit **`source` + `confidence`** columns, because *who claimed this account and
how sure are we* is analytics-grade data an operator filters/weights on at query time — not something to
reconstruct from audit. This is a ratified, scoped column, not a reversal of the non-uniform stance.

**4.5 Status over deletion — partial.** Ontology prefers a terminal status to deletion. go-oikumenea
uses `deleted_at` soft-delete (reversible within a grace window — [glossary](glossary.md),
Reversibility), which is archival-flavored but not a true terminal status; `Unit.state` and
`Person.status` are the cleaner matches, and `Person` purge is a genuine crypto-erase terminal state
with an id tombstone. **Verdict:** aligned in spirit; the lifecycle-state columns are the better
ontology citizens than `deleted_at`.

**4.6 public/shadow visibility has no ontology analog.** Visibility is not a property of the graph
edges; it is a **read-time gate** layered on the Object/Link graph — the app-layer PDP + shadow gate
(deliberately **no Postgres RLS** as the authorization model, only an optional defense-in-depth
backstop — [decisions.md](architecture/decisions.md)). **Verdict:** a legitimate concept the base
ontology lacks; documented here as an access-time filter, not a stored relationship.

**4.7 Permissions are code, not Objects.** Ontology might reflexively model permissions as a type.
go-oikumenea keeps the permission vocabulary **in code** (enforcement-as-code; the surface always
shows in a diff) while roles/assignments are data. **Verdict:** intentional and sound — permission
*strings are codes*, and the closed set is a compile-time concern.

---

## Conflicts

This registry is binding for the **catalog** of Object/Link/Action types
([D-Ontology](architecture/decisions.md)); the [modules/](modules/) docs remain authoritative for
entity **detail**, and [decisions.md](architecture/decisions.md) wins on any genuine conflict. If a
row here is found to contradict a binding decision (not merely diverge in style), record it in
[open-questions.md](open-questions.md) rather than editing this file or `decisions.md` in place.
