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
| `__rid` | the **composed URN** primary key (`id TEXT`), `urn:oikumenea:<service>:<env>:<entity_type>:<uuid>`, via `new_rid()` (D-ResourceIdentifiers); immutable, self-describing. See [Identifier scheme](#identifier-scheme-rids) |
| `object_type` / `link_type` | the `<entity_type>` URN slot (Links use `link__<type>`); also the table (per-module prefix `oikumenea.<module>_*`) |
| `created_at` / `updated_at` | `TIMESTAMPTZ` UTC + the `set_updated_at()` trigger |
| `status` | the entity's `status`/`state` enum (`TEXT`+`CHECK`) and/or `deleted_at` soft-delete |
| `source` | **mostly absent as a column** — provenance lives in [order](modules/order.md) refs + the [audit](modules/audit.md) actor; see [§4.4](#4-ratified-divergences-from-the-ontology-ideal) |
| Action `audit_log` | the append-only [audit](modules/audit.md) entry written in the same transaction |
| code vs name | every structural Object has a stable locale-agnostic **`code`** + a translatable **`name`** ([localization](modules/localization.md)) |

---

## Identifier scheme (RIDs)

Every row below is keyed by a composed URN RID (D-ResourceIdentifiers; full grammar in
[conventions.md](architecture/conventions.md#resource-identifiers-rids)):
`urn:oikumenea:<service>:<env>:<entity_type>:<uuid>`, where `<entity_type>` is the Object type for an
Object, `link__<type>` for a Link, and `action__<type>` for an Action. Temporal Links additionally
carry `valid_from`/`valid_to` (the existing `effective_from`/`effective_to`,
`granted_at`/`revoked_at`+`expires_at` columns); the audit row recording an Action is keyed by its
Action RID — the audit log is the action ledger.

## 1. Object Types

Real-world entities with identity over time → Objects.

| Object Type | Module | `code`/`name`? | Lifecycle / removal | Notes |
|---|---|---|---|---|
| `Unit` | [tenant](modules/tenant.md) | yes (`code` unique among active) | `state` (active/suspended/archived) + soft-delete | `visibility` public/shadow; `level` & `unit_kind` are **directory attributes**, never PDP inputs |
| `Graph` | [tenant](modules/tenant.md) | yes | soft-delete (`command` undeletable) | named hierarchy; `is_authority_bearing` gates PDP cascade |
| `Person` | [person](modules/person.md) | optional `code` | `status` (active/deactivated/purged) + soft-delete; **crypto-erase on purge** | instance-global; CLDR structured names (patronymic in `given2`); `birthdate`, ISO-5218 `sex` |
| `PersonEmail` / `PersonPhone` / `PersonCallSign` | [person](modules/person.md) | no | soft-delete; **erased on purge** | effective person child rows; email/phone `pii:contact`, call sign `pii:basic`; each unique per person among active; `is_primary` |
| `PersonEmailType` / `PersonPhoneType` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalogs for the contact-channel `kind` |
| `PersonSocialAccount` | [person](modules/person.md) | no | soft-delete; **erased on purge** | standalone social handle; stable `platform_user_id` vs mutable `handle` (history in `PersonSocialAccountHandle`); `pii:contact`; `platform_verified` vs `verified_by_operator_at` (D-PersonSocialChannels) |
| `PersonSocialAccountHandle` | [person](modules/person.md) | no | temporal (`valid_from`/`valid_to`) + soft-delete | handle-rename history so a rename never breaks links |
| `Platform` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalog of messengers/social networks; `category ∈ messenger\|social` |
| `RelationType` | [person](modules/person.md) | yes (`code`/`name`) | `status` + soft-delete | instance-admin catalog for open-ended person↔person relation labels (`category ∈ sponsorship\|association\|next_of_kin`) |
| `Position` | [membership](modules/membership.md) | yes (unique per unit) | `status` (active/abolished) + soft-delete | unit-owned billet; an Object that **exists while vacant** — not just a link end |
| `Document` / `DocumentType` | [document](modules/document.md) | type has `code`/`name` | `status` + soft-delete | papers, metadata only; type is an instance-admin catalog |
| `PersonalCode` / `PersonalCodeScheme` | [document](modules/document.md) | scheme has `code`/`name` | `status` + soft-delete; crypto-erase | value is `pii:sensitive`, **envelope-encrypted** + blind-indexed |
| `Order` (наказ) / `OrderType` | [order](modules/order.md) | type has `code`/`name` | `Order`: draft→issued→revoked (issued is immutable) | the legal act; `OrderType.effect` declares the downstream consequence |
| `OrderItem` | [order](modules/order.md) | no | parent-scoped (no own `deleted_at`) | one affected person/action; the unit of effect + provenance |
| `RankSystem` / `RankCategory` / `RankType` / `Rank` | [rank](modules/rank.md) | yes | soft-delete (RESTRICT if held) | single system-wide ordered scheme, now rooted at `RankSystem` (a national/organizational ladder — multinational; D-RankSystems); types form a **tree** (`parent_type_id` self-FK — a structural containment FK like `system_id`/`category_id`/`type_id`, **not** a reified Link), ranks on leaf types; a rank carries an optional standardized `grade_code` → `RankGrade` |
| `RankGrade` | [rank](modules/rank.md) | `code` = NATO STANAG 2116 grade (`OF-1`…`OF-10`, `OR-1`…`OR-9`, warrant) | seeded reference registry | the cross-system comparability scale (`tier`/`ordinal`); migration-seeded like `Country` (D-RankSystems / D-Geo carve-out) |
| `Role` | [authorization](modules/authorization.md) | yes | soft-delete | `is_base` roles immutable; permissions are **code**, not rows |
| `Assignment` | [authorization](modules/authorization.md) | no | revoke-flip + optional `expires_at` | a **reified Link** — see [§2](#2-link-types) |
| `InstanceAdmin` | [authorization](modules/authorization.md) | no | revoke-flip | the instance-wide authority plane |
| `Account` / `ExternalIdentity` | [identity-federation](modules/identity-federation.md) | no | account soft-delete; identity append-only | `(issuer, subject)` globally unique; account optional per person |
| `Locale` / `Translation` | [localization](modules/localization.md) | locale `code` is ISO 639-3 | locale soft-delete | the translatable-`name` store |
| `Country` | [platform](modules/platform.md) | `code` = ISO-3166-1 α2 | status | shared reference registry |
| `AuditEntry` | [audit](modules/audit.md) | no | **append-only** (`reject_mutation()`) | not an endpoint; written in-transaction |
| `ImportSource` / `ImportRun` *(planned, M17)* | platform | source has `code` | source soft-delete; run append-only | external source registry + lineage ledger; D-DataIngestion |
| `Languoid` *(planned, M18)* | `language` | `code` = glottocode; nullable unique `iso639_3` | seeded reference (import) | recursive Glottolog forest, `level ∈ family\|language\|dialect`; `parent_id` strict-tree FK (not a Link); AES `status`; D-Languages |
| `WritingSystem` / `WritingSystemScriptType` *(planned, M18)* | `language` | `code` (ISO 15924 / catalog) | seeded reference | scripts + script-type catalog |
| `Location` *(planned, M19)* | `location` | no | soft-delete | required `GEOGRAPHY(POINT,4326)`; DB-derived MGRS/H3; structured address over `geo_countries`; D-Location |
| `EducationInstitution` / `EducationUnit` / `EducationBuilding` / `EducationGroup` *(planned, M20)* | `education` | institution/unit `code` | soft-delete | external reference orgs; `EducationUnit` is a recursive per-institution tree (closure); D-Education |
| `EducationPosition` *(planned, M20)* | `education` | yes (per institution/unit) | `status` + soft-delete | institution-owned billet, vacant-first (mirrors `Position`) |
| `EducationInstitutionKind` / `EducationUnitKind` / `EducationDegreeLevel` *(planned, M20)* | `education` | yes (`code`/`name`) | `status` + soft-delete | catalogs; degree levels seeded ISCED 2011 |
| `Company` *(planned, M21)* | `company` | `code` | soft-delete | legal entity; `legal_form` + `ownership_category` (two axes); D-Companies |
| `CompanyPosition` *(planned, M21)* | `company` | yes (per company) | `status` + soft-delete | company-owned billet (mirrors `Position`) |
| `CompanyLegalForm` / `CompanyRegistrationScheme` / `CompanyIndustryClass` *(planned, M21)* | `company` | yes (`code`/`name`) | `status` + soft-delete | catalogs; registration schemes mirror `PersonalCodeScheme` (LEI spine) |
| `LocationType` *(planned, M19)* | `location` | yes (`code`/`name`) | `status` + soft-delete | optional place-purpose catalog beside `Location`; D-Location |
| `Religion` / `TraditionFamily` / `SubTradition` *(planned, M22)* | `religion` | yes (`code`/`name`) | soft-delete | faith taxonomy catalogs (family nested under religion, sub-tradition under family); D-Religion |
| `OrgKind` / `OrgProfile` / `OrgPolicy` *(planned, M22)* | `religion` | kind has `code`/`name` | soft-delete | org nodes **reuse `Unit`**; `OrgProfile` is per-unit faith attributes, `OrgPolicy` a data-driven eligibility rule (replaces any faith-specific doctrinal flag) |
| `ClergyGrade` / `GradeCategory` / `OfficeType` *(planned, M23)* | `religion` | yes (`code`/`name`) | soft-delete | **per-tradition** ordered clergy catalog (no cross-tradition comparator, DS-43); offices **reuse `Position`**; D-ClergyCredential |
| `AffiliationType` *(planned, M24)* | `religion` | yes (`code`/`name`) | soft-delete | per-tradition lay-affiliation catalog; D-ReligiousAffiliation |
| `SiteType` / `ServiceType` *(planned, M25)* | `religion` | yes (`code`/`name`) | soft-delete | per-tradition discovery catalogs (church/mosque/synagogue/temple…; main/prayer…) |
| `ServiceSchedule` / `Alias` *(planned, M25)* | `religion` | no | soft-delete | per-site recurring service times; search-only alternative names (never displayed) |
| `GeoSubdivision` *(planned, M26)* | [platform](modules/platform.md) | `code` = ISO 3166-2 (`UA-32`…) | status | shared reference registry below `Country`; `parent_id` self-FK (nested), `subdivision_type`; migration-seeded like `Country`; D-GeoSubdivisions |
| `Vehicle` *(planned, M26)* | `vehicle` | optional | soft-delete | physical vehicle; `vin` unique among active (nullable, `pii:basic`); `type_id`/`model_id`; `attributes` JSONB; D-Vehicles |
| `VehicleBrand` / `VehicleModel` / `VehicleType` *(planned, M26)* | `vehicle` | yes (`code`/`name`) | `status` + soft-delete | brand (`country` of origin); model (`brand_id` + generation/manufacture window); type taxonomy **tree** (`parent_id` self-FK + denormalized root, no closure — the `RankType` pattern) |
| `VehicleRegistrationNumberType` *(planned, M26)* | `vehicle` | yes (`code`/`name`) | `status` + soft-delete | plate-type catalog (regular/temporary/transit/diplomatic/military/old…) |

**Non-Objects (correctly):** `Atomic permission` is **code, not data** — a closed vocabulary in Go,
not a table ([authorization](modules/authorization.md)). `Vacancy` is a **derived predicate** (active
position, no active filling), not a stored row ([membership](modules/membership.md)). The `Unit
closure` is a **materialized derived relation**, treated here as a Link set ([§2](#2-link-types)),
not a source-of-truth Object.

---

## 2. Link Types

Relationships with their own identity, attributes, or history → Links (not FK columns). go-oikumenea
already models its load-bearing relationships as join/edge tables, so most map cleanly. Each Link's
RID is `link__<link_type>` in lower_snake (e.g. the `PARENT_OF` row → `link__parent_of`,
`HAS_ROLE` → `link__has_role`); temporal Links additionally carry `valid_from`/`valid_to`
([Identifier scheme](#identifier-scheme-rids)).

| Link Type | From → To | Module | Carries | Temporal? |
|---|---|---|---|---|
| `PARENT_OF` (per graph) | `Unit` → `Unit` | [tenant](modules/tenant.md) | `graph_id`, `created_by` provenance | created-only; multi-parent DAG, no validity interval |
| `ANCESTOR_OF` (derived) | `Unit` → `Unit` | [tenant](modules/tenant.md) | `graph_id`, `depth` | materialized closure; recomputed on edge change |
| `IN_UNIT` | `Position` → `Unit` | [membership](modules/membership.md) | — | structural |
| `MEMBER_OF` / `FILLS` | `Person` → `Unit` (opt. `Position`) | [membership](modules/membership.md) | `position_id` (nullable), `order_item_id` provenance | **yes — `effective_from`/`effective_to`** + `status` |
| `HAS_ROLE` @ scope (the **`Assignment`**) | `Person` → `Role`, scoped to `target_unit` | [authorization](modules/authorization.md) | `scope ∈ {unit,subtree}`, `graph_id`, `granted_by`, `expires_at` | grant/revoke + decision-time expiry |
| `GRANTS` | `Role` → `Permission`(code) | [authorization](modules/authorization.md) | — | code-validated membership |
| `HOLDS_RANK` | `Person` → `Rank` | [person](modules/person.md) | exactly one | **directory attribute — never an authz input** |
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
| `LANGUAGE_SUBGROUP_OF` *(planned, M18)* | `Languoid` → `Languoid` | `language` | structural; `family_code` denormalized | strict tree, a containment FK — *not* a reified Link (closure is `ANCESTOR_OF`-style) |
| `WRITTEN_IN` *(planned, M18)* | `Languoid` → `WritingSystem` | `language` | `is_primary` | — |
| `SPEAKS` *(planned, M18)* | `Person` → `Languoid` (level=language) | `language`/[person](modules/person.md) | `cefr_level`, `is_native`; `pii:basic` | scoped through the holder; purge-erased |
| `OFFICIAL_LANGUAGE` *(planned, M18)* | `Unit` → `Languoid` | `language`/[tenant](modules/tenant.md) | working/official | — |
| `LOCALE_OF` *(planned, M18)* | `Locale` → `Languoid` | `language`/[localization](modules/localization.md) | canonical language of a locale | — |
| `EDUCATION_UNIT_PARENT_OF` *(planned, M20)* | `EducationUnit` → `EducationUnit` | `education` | per institution; closure maintained | recursive structure tree |
| `STUDIED_AT` *(planned, M20)* | `Person` → `EducationInstitution` (opt. unit/group) | `education` | `degree_level`, field, status, qualification; `pii:basic` | **temporal** (effective-dated); mirrors `MEMBER_OF` |
| `RESIDED_IN_DORMITORY` *(planned, M20)* | `Person` → `EducationBuilding` | `education` | room, period; `pii:contact` | **temporal**; dedicated dorm stay; purge-erased |
| `HOLDS_EDUCATION_POSITION` *(planned, M20)* | `Person` → `EducationPosition` | `education` | one-holder | **temporal**; mirrors `FILLS` |
| `SPONSOR_OF` (education context) *(planned, M20)* | `Person` → `Person` | [person](modules/person.md) | optional enrollment ref + role ∈ professor/tutor/curator/advisor | **extends M14 `SPONSOR_OF`** — no new link type (D-Education) |
| `HOLDS_COMPANY_POSITION` *(planned, M21)* | `Person` → `CompanyPosition` | `company` | one-holder | **temporal**; mirrors `FILLS` |
| `FOUNDED` *(planned, M21)* | `Person`\|`Company` → `Company` | `company` | founder (person or company) | — |
| `OWNS_STAKE` *(planned, M21)* | `Person`\|`Company` → `Company` | `company` | stake %; **polymorphic holder** | **temporal**; company-holder edges form the ownership DAG |
| `BENEFICIARY_OF` *(planned, M21)* | `Person` → `Company` | `company` | ultimate %, declared-vs-computed | UBO; computed traversal is DS-47 |
| `SUCCEEDED_BY` *(planned, M21)* | `Company` → `Company` | `company` | M&A/reorganization lineage | — |
| `BRANCH_OF` *(planned, M21)* | `Company` → `Company` | `company` | non-independent sub-unit | distinct from a subsidiary |
| `CLERGY_CREDENTIAL` *(planned, M23)* | `Person` → `ClergyGrade` (in an org `Unit`) | `religion` | `granted_on`, conferrer, `status ∈ active\|suspended\|revoked`, `source`/`confidence` | **temporal**; indelible where sacramental; **never an authz input** (parallels `HOLDS_RANK`) |
| `AFFILIATED_WITH` *(planned, M24)* | `Person` → religion/tradition/community `Unit` | `religion` | `affiliation_type`, **`pii:special`** envelope-encrypted value + blind index, `source`/`confidence` | **temporal**; crypto-erased on purge; never an authz input; D-ReligiousAffiliation / D-SpecialPII |
| `SITE_OF` *(planned, M25)* | `Unit` → `Location` | `religion` | `site_type`, `visibility`, `public_precision`, `is_primary` (one per unit) | — (shared `Location`; precision projected at read time) |
| `MANUFACTURED_BY` *(planned, M26)* | `VehicleBrand` → `Company` | `vehicle` | manufacturer of a marque | **temporal** (`effective_from`/`effective_to`) — changes with acquisitions |
| `REGISTERED_TO` *(planned, M26)* | `Vehicle` → `Person`\|`Company` | `vehicle` | **polymorphic owner** (person XOR company); `country` → `geo_countries`, `subdivision` → `geo_subdivisions` (plate region), `registration_number` (unique active per country), `number_type` | **temporal** + `status`; the ownership+plate record (re-registration = new row); person-owned rows `pii:basic`, holder-scoped, purge-erased; D-Vehicles |

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
by an `action__<action_type>` RID (e.g. `action__issue_order`, `action__grant_assignment`), and the
[audit](modules/audit.md) row that records it is keyed by that RID — so the audit log *is* the action
ledger ([Identifier scheme](#identifier-scheme-rids)).

- **Direct CRUD:** `CreateUnit`, `AddEdge`/`RemoveEdge`, `TransitionUnit`, `CreatePerson`,
  `AssignRank`, `CreatePosition`/`AbolishPosition`, `CreateMembership`/`EndMembership`,
  `AttachDocument`/`AttachPersonalCode`, `UpsertEmail`/`UpsertPhone`/`UpsertCallSign` (+ their
  deletes), `CreateRole`, `GrantAssignment`/`RevokeAssignment`,
  `GrantInstanceAdmin`, `CreateAccount`/`LinkExternalIdentity`, rank/locale/catalog edits.
- **Planned (M16–M26):** `ScheduleJob`/`RunJob` (M16 worker); `RunImport` over a registered mapper
  (M17 — a bulk **code-keyed upsert** emitted as audited Actions, the ingest≠edit boundary);
  `CreateLanguoid`/`ImportLanguageScheme`, `UpsertPersonLanguage` (M18); `CreateLocation` (M19);
  `CreateInstitution`/`CreateEnrollment`/`RecordDormStay`/`AppointEducationPosition` (M20);
  `CreateCompany`/`RecordShareholding`/`RecordBeneficiary`/`AppointCompanyPosition` (M21);
  `ConferCredential`/`SuspendCredential`/`AppointClergy` (M23), `RecordAffiliation` (M24),
  `AttachSite`/`AddServiceSchedule` (M25) — the religion vertical (D-Religion);
  `CreateVehicle`/`RegisterVehicle`/`TransferRegistration` + `geo_subdivisions`/vehicle-catalog edits
  (M26 — D-Vehicles / D-GeoSubdivisions).
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
validity. **Verdict:** real divergence, but **narrowing** — under D-ResourceIdentifiers temporal Links
are now defined to carry `valid_from`/`valid_to` (NULL `valid_to` = active), and the existing
`effective_from`/`effective_to` and `granted_at`/`revoked_at`(+`expires_at`) columns *are* that pair.
History is recoverable; full bitemporality (a second, transaction-time axis) remains an additive seam.

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
RID self-declares its `<service>` and `<environment>` of origin, and the **`action__…` RID keys each
audit row** to the write that produced it. **Verdict:** partial gap; provenance is fully recoverable
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
