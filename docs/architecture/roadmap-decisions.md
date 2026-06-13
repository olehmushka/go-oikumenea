# Roadmap decisions (planned tier — M16–M26)

The landed architectural decisions for the **planned milestones M16–M26** — verticals that are
**decided and designed but not yet built** (no `internal/` or `migrations/` artifacts exist for them
yet). They carry the same ADR rigor as [`decisions.md`](decisions.md) and are **authoritative for
those verticals' design**; they become **binding-against-code** as each milestone enters
implementation (at which point a decision may be promoted into `decisions.md` or stay referenced
here).

They live in a separate file (split out of `decisions.md` per the F-008 review finding) so the
binding **[`decisions.md`](decisions.md)** reflects the **built / in-progress surface (M0–M15)** —
"what the code is actually held to" — without ~500 lines of unbuilt verticals it must keep coherent.
The [`milestones.md`](../milestones.md) stage board sequences these; the
[`ontology-mapping.md`](../ontology-mapping.md) registry classifies their (planned) Object/Link/Action
kinds.

Decisions here, in milestone order: **D-Worker** (M16) · **D-DataIngestion** (M17) · **D-Languages**
(M18) · **D-Location** (M19) · **D-Education** (M20) · **D-Companies** (M21) · **D-Religion** /
**D-ClergyCredential** / **D-ReligiousAffiliation** / **D-SpecialPII** (M22–M25) · **D-GeoSubdivisions**
/ **D-Vehicles** (M26).

> Cross-references into the **core** decisions (D-CryptoProvider, D-WebUI, D-Geo, D-Rank, D-Ontology,
> D-PersonReadScope, …) point at [`decisions.md`](decisions.md); references among the planned-tier
> decisions resolve within this file.

---

### D-Worker — A first-class background-job runtime (promotes DS-25)

**Decision.** The service gains an **in-process background-job runtime** — a cron **scheduler** + a
**job queue** built over the existing `pkg/events` outbox, with witchcraft-managed lifecycle. This
**promotes the long-parked DS-25** (the common scheduler blocker) onto the critical path, because the
M17 connector framework requires scheduled syncs. A `worker_jobs` ledger records status/attempts/
last_error; execution is **at-least-once with idempotency keys**; jobs retry with backoff and surface
failures via a health reporter + audit. It is a **single-process** scheduler — **no external broker**
(DS-26 stays parked) — and drains in-flight jobs on graceful shutdown.

**Why.** "Full connector/ETL" ingestion (D-DataIngestion) needs *scheduled* re-syncs, not just
on-demand imports. DS-25 was already the shared blocker for audit-retention partitioning (DS-28),
future-dated order effects (the residual of D-OrderApply), and expiry sweeps; building it now unblocks
all of them as a side effect. An in-process runtime keeps the self-hosted, single-binary deployment
story intact.

**Why not** (a) *an external broker/queue now* (Kafka, a job server): breaks the single-binary,
operator-owned-Postgres simplicity; deferred to DS-26 when a module is actually extracted. (b) *Keep
everything synchronous*: impossible for scheduled syncs and the parked sweeps. (c) *A cron sidecar
hitting HTTP endpoints*: loses in-transaction idempotency, the outbox, and unified audit.

**Consequence.** New `worker_jobs` table + the scheduler/queue in [platform](../modules/platform.md);
the in-process bus gains a durable, scheduled execution path. Lands as **M16** ([milestones](../milestones.md)),
a prerequisite for M17. DS-25 leaves [open-questions](../open-questions.md) (promoted). Additive /
expand-only.

---

### D-DataIngestion — A generic reference-data ingestion & connector framework (extends D-Ontology)

**Decision.** Bulk reference-data import becomes a **generic, reusable pipeline** in
[platform](../modules/platform.md) (`pkg/dataimport`), not a bespoke importer per domain. It mirrors
Palantir Foundry's ingestion stages — **Data Connection → Pipeline → Ontology mapping** — right-sized
for a self-hosted Go monolith (no Spark). Four parts:

- **Sources & connectors** — an `import_sources` registry (`type ∈ http|file` now; `jdbc-sql`/
  `object-store` parked as **DS-44**), credentials via the D-CryptoProvider seam; a pluggable
  `Connector` interface (`Fetch(ctx, source) → RawBatch`). First connector: **HTTP(S) download**;
  local bundled presets are the degenerate `file` case. Syncs run **on-demand or scheduled** on the
  D-Worker runtime (`import_syncs`, cron).
- **Raw staging** — `import_raw_batches` lands the fetched payload verbatim (checksum,
  `source_version`, `fetched_at`), re-mappable without re-fetch.
- **Transform → ontology** — each module **registers a mapper** for its importable object-types
  (`language-scheme`, `education-institutions`, `company-registry`, …) mapping raw records → a
  **canonical envelope** (`{object_type, source, source_version, license, generated_at, records[]}`) →
  a **code-keyed, idempotent, non-destructive upsert** into the domain catalog (never deletes;
  mismatches reported), in one transaction, emitted as **audited Actions** — preserving the
  bulk-ingest ≠ audited-edit boundary.
- **Lineage & run ledger** — `import_runs` (source, version, counts, checksum, status, errors) +
  `(source, source_version, imported_at)` provenance on every imported row + a sync-failure health
  reporter. One generic `POST /import/{objectType}` endpoint (instance-scope, `import.manage`).

**Why.** The same need recurred three times — rank presets (M15), Glottolog (M18), and the coming
education/company registries — so a per-domain importer is the wrong altitude. Foundry's separation of
*fetch → stage → curate → map* with first-class lineage is the proven pattern; the transferable parts
(explicit schema/mapper, provenance, idempotent re-sync, ingest≠edit) carry over without the heavy
machinery.

**Why not** (a) *Clone the M15 rank importer per domain*: duplication, no lineage, no scheduling.
(b) *Retrofit M15 onto this now*: M15 is shipped and works; left as a **legacy one-off** to avoid churn
(new code uses the framework). (c) *Full Foundry (Spark, branch/merge datasets)*: vastly over-scoped for
a self-hosted monolith; the raw-staging + mapper-registry slice captures the value. (d) *A live API
client per source baked into each module*: outbound deps scattered; the connector seam centralizes them.

**Consequence.** New `import_sources`/`import_syncs`/`import_raw_batches`/`import_runs` tables +
`pkg/dataimport` in [platform](../modules/platform.md); a per-module mapper-registration seam. New
Object/Action kinds in [ontology-mapping](../ontology-mapping.md). Lands as **M17**
([milestones](../milestones.md)), on M16. M15's `/rank-scheme/import` is **not** migrated. Parks
**DS-44** (more connectors). Additive / expand-only.

---

### D-Languages — Languages, language groups & writing systems as a Glottolog-faithful registry (extends D-Ontology, D-i18n)

**Decision.** A new **`language`** module holds the world's languages as a **faithful model of
Glottolog** (the standard genealogical reference), their **writing systems** (ISO 15924), and a
person's **language proficiency**.

- **`language_languoids`** — the recursive **Glottolog forest**, *one* table (not a group/language
  split): PK `code` (glottocode); `level ∈ {family, language, dialect}`; translatable `name`; self-FK
  `parent_id` (Glottolog "father" — a **strict tree**, structural containment FK, *not* a reified
  Link); denormalized `family_code` (root family, derived in SQL via the closure, per the
  denormalized-FK pattern); nullable **UNIQUE** `iso639_3` (the optional ISO 639-3 attribute —
  glottocode is the universal spine because families/dialects have no ISO code); `macroarea`;
  representative `latitude`/`longitude` (plain numeric — the `language` module precedes the PostGIS
  Location, D-Location); AES endangerment `status ∈ {not_endangered…extinct}` (replaces a naïve
  `living` boolean); `glottolog_version` provenance. A maintained **`language_languoid_closure`**
  (mirrors the tenant closure) answers descendant queries; **`language_languoid_countries`** ties
  languoids → `geo_countries` (CLDF `Country_IDs`, D-Geo).
- **Writing systems** — `writing_system_script_types` catalog (seeded `logographic`/`syllabary`/
  `alphabet`/`abjad`/`abugida`/`featural`); `writing_systems` (PK `code` ISO 15924, translatable
  `name`, `script_type`); `language_writing_systems` M:N (`is_primary`).
- **Language links** — `person_languages` (child of `person_persons`: `language_id` constrained to
  `level='language'`, `cefr_level ∈ {A1…C2}` nullable, `is_native`; `pii:basic`, purge-erased);
  `tenant_unit_languages` (a unit's official/working language); `i18n_locale_languages` (a locale's
  canonical language).
- **Population** — the bundled preset is the **full pinned Glottolog 5.3 CLDF snapshot** (~26k
  languoids, `deploy/language-presets/glottolog-5.3.json`, **opt-in asset, never a migration**,
  CC-BY-4.0 attribution carried), loaded through the **D-DataIngestion** `language-scheme` mapper; the
  HTTP connector can pull a newer CLDF release on operator request.

**Why.** Language is a recurring analytics/linking dimension (who speaks what; a unit's working
language; locale provenance). Modeling it on **Glottolog** — the de-facto standard genealogy with
stable glottocodes — gives a complete, authoritative, re-importable dataset instead of a hand-curated
list, and the faithful recursive `languoid` model ("take it fully") preserves families/dialects and the
genealogical tree the simpler split would lose.

**Why not** (a) *ISO 639-3 as the PK*: families/dialects and unlisted languages have no ISO code; the
glottocode is the only universal spine. (b) *A separate `language_groups` + `languages` split*: diverges
from Glottolog's uniform languoid model and complicates the closure. (c) *Seed the full dataset in a
migration*: ~26k rows is a heavy, hard-to-maintain migration; D-DataIngestion's opt-in import is the
right home. (d) *A `living` boolean*: loses Glottolog's graded AES endangerment.

**Consequence.** New `language` module + tables above; person/tenant/localization gain language ties;
new Object/Link kinds in [ontology-mapping](../ontology-mapping.md). First **D-DataIngestion** consumer.
Lands as **M18** ([milestones](../milestones.md)), on M17 (+ M5/M2; M3 for the unit tie). Additive /
expand-only.

---

### D-Location — A shared, standalone Location entity (PostGIS + H3; reverses the `drafts/` geography drop)

**Decision.** A new **`location`** module provides one **standalone** place entity that anything with a
location references by FK. **`location_locations`** carries a **required** `geom GEOGRAPHY(POINT, 4326)`
(**PostGIS**), **DB-derived** MGRS string + **H3 indexes** (a small set of resolutions) via DB
functions/triggers using the **PostGIS + h3-pg** extensions, and a structured postal address:
`country_code` (NOT NULL → `geo_countries`, D-Geo), `admin_area_1`/`admin_area_2`, `locality`, `street`,
`house_number`, `postal_code`, `raw_address`; soft-delete; a spatial GIST index. A `LocationService`
offers CRUD + radius (`ST_DWithin`) queries. The operator DB must carry the **PostGIS + h3-pg**
extensions; the schema-bootstrap enables them and the readiness gate checks for them. This **reverses
the explicit `drafts/` drop of `location`/PostGIS/H3/geography** — re-adopted with rationale, exactly as
D-WebUI re-adopted the UI.

**Why.** Both the education domain (buildings, campuses, dormitories — D-Education) and companies
(registered/operating addresses — D-Companies) need precise, queryable places, and the project's
analytics ambition ("better information for building relations & graphs") wants real spatial indexing.
A single shared entity dedupes addresses and enables "everything near point X" once, instead of
re-inventing address columns per owner. PostGIS + h3-pg are the standard spatial stack; deriving
MGRS/H3 in the DB keeps them consistent with the authoritative geometry.

**Why not** (a) *Embedded address columns per owner*: duplicates schema, blocks cross-entity spatial
queries and dedup. (b) *App-layer geometry, plain columns*: loses native spatial indexing / radius
queries (`ST_DWithin`). (c) *Coordinate optional*: the deployments here want spatial analytics, so the
coordinate is the required spine and a precise point is mandated (address-only records are out of
scope — geocode first). (d) *Stay faithful to the `drafts/` drop*: that drop was for a
church-discovery scope; the army/university analytics scope genuinely needs geography.

**Consequence.** New `location` module + `location_locations`; PostGIS + h3-pg become operator-DB
prerequisites (bootstrap + readiness gate, [platform](../modules/platform.md)); the L-Conventions enum
note is unaffected. New Object kind `Location` in [ontology-mapping](../ontology-mapping.md). The
"Explicitly dropped from `drafts/`" list is updated to mark geography **re-adopted**. Lands as **M19**
([milestones](../milestones.md)); a foundation for M20 + M21. Additive / expand-only.

---

### D-Education — An `education` module: institutions, structure, and person bindings (extends D-Ontology)

**Decision.** A new **`education`** module models the education domain as **external reference
entities** (where people studied/taught), distinct from the deploying org's tenant units and
**independent of companies** (no shared organization foundation). Shape:

- **Reference catalogs** — `education_institution_kinds` (kindergarten…academy), `education_unit_kinds`
  (campus/faculty/department/chair…), `education_degree_levels` (seeded **ISCED 2011** 0–8).
- **Objects** — `education_institutions` (code, name, `kind`, `country`, founded/closed, lifecycle);
  `education_units` (a **dedicated recursive structure tree** per institution, typed, maintained
  closure, `link__education_unit_parent_of` — *not* reused tenant units); `education_buildings` (FK
  `location_id` → D-Location, kind incl. `dormitory`); `education_groups` (cohort under a unit).
- **Person bindings** — `person_education_enrollments` (`link__studied_at`: institution + optional
  unit/group, ISCED `degree_level`, field/specialty, effective-dated, status, qualification; mirrors
  the membership temporal Link; `pii:basic`); **mentorship reuses D-PersonRelationships** — extends
  M14 `person_sponsorships` with an optional **education context** (enrollment ref + role ∈
  professor/tutor/curator/advisor), no new link type; `person_dormitory_stays`
  (`link__resided_in_dormitory` — a **dedicated** stay entity: person ↔ dorm building, room, period;
  `pii:contact`, purge-erased).
- **Positions ("like a military")** — `education_positions` (institution/unit-owned billets,
  vacant-first) + `education_appointments` (`link__holds_education_position`, one-holder,
  effective-dated) — mirrors the membership module.

**Why.** The directory needs to place a person in their education history at analytics grade (who, when,
where, under whom, in which group, living where) for relationship graphs spanning army/church/
university. A dedicated structure tree keeps external institutions cleanly separate from the deploying
org's PDP-bearing tenant units; ISCED gives a standardized degree scale (the project's standards
instinct).

**Why not** (a) *Reuse tenant units for institution structure*: conflates external reference orgs with
the deploying organization, muddying the PDP and visibility semantics. (b) *A shared organization
foundation with companies*: deferred — the user chose fully independent modules (a university-as-legal-
entity tie can come later as a seam). (c) *A new mentorship link type*: M14 sponsorship already covers
advisor/mentor; reuse over reinvention. (d) *Model dorm living as a `person_residence`*: a dedicated
stay carries room/occupancy the generic residence lacks.

**Consequence.** New `education` module + tables above; [person](../modules/person.md)'s
`person_sponsorships` gains an optional education context; new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); institution registries ride D-DataIngestion. Lands as
**M20** ([milestones](../milestones.md)), on M5/M14/M19. Additive / expand-only.

---

### D-Companies — A `company` legal-entity registry with an ownership/affiliation graph (extends D-Ontology)

**Decision.** A new **`company`** module holds organizations (private/public/state-owned/…) at
**registry grade**, scoped to **structural** data — identity, legal form, multi-jurisdiction
registration, locations, positions, and the **ownership/affiliation graph** — **independent of
education**. Volatile registry intelligence is **parked**.

- **Reference catalogs** — `company_legal_forms` (per-country: ТОВ/ПАТ/ФОП, LLC/JSC/GmbH…),
  `company_registration_schemes` (mirrors `document_personal_code_schemes`, D-PersonalCodes:
  `ua-edrpou`/`vat`/`us-ein`/`duns`/**LEI** ISO 17442 global spine, validators per scheme),
  `company_industry_classes` (NACE/ISIC/KVED economic-activity classification).
- **Objects** — `company_companies` (code, legal + short names, `legal_form`, `ownership_category ∈
  private|public|state_owned|municipal|foreign|mixed` — two orthogonal axes, `country`, founded/
  dissolved, lifecycle); `company_registrations` (per-scheme IDs + validation); `company_industry_assignments`
  (M:N, primary+secondary); `company_locations` (→ D-Location, role ∈ registered/operating/branch).
- **Positions** — `company_positions` + `company_appointments` (`link__holds_company_position` —
  CEO/director/employee billets; mirrors membership).
- **Equity / ownership links** — `company_foundings` (`link__founded`, founder a person **or** a
  company); `company_shareholdings` (`link__owns_stake`, **polymorphic holder** person|company, stake
  %, effective-dated — company-holder edges form the **ownership DAG**); `company_beneficiaries`
  (`link__beneficiary_of` — UBO: ultimate %, declared-vs-computed flag).
- **Company↔company** — parent/subsidiary (via shareholdings), `company_successions`
  (`link__succeeded_by` — M&A/reorganization lineage), founder-company (via foundings),
  `company_branches` (`link__branch_of` — non-independent sub-units).

**Why.** Holding companies as first-class entities lets people link into one queryable graph (employer,
founder, owner, ultimate beneficiary) — the "further linking" value the user asked for, YouControl-style.
Registration-as-scheme-registry (reusing the personal-code pattern, LEI as the global spine) makes it
multi-jurisdiction-ready; separate position vs equity links keep employment billets distinct from
ownership stakes; explicit declared UBO records what registries declare (often ≠ computed).

**Why not** (a) *One typed person↔company link*: conflates employment billets with equity stakes.
(b) *Include financials/court/tax/sanctions now*: volatile, feed-dependent, mostly useless without live
sources — parked as **DS-45** (rides D-DataIngestion connectors when a feed exists). (c) *Derive UBO
only*: loses the authoritative declared beneficiary; computed traversal/closure is the parked **DS-47**.
(d) *A shared org foundation with education*: the user chose independent modules.

**Consequence.** New `company` module + tables above; new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); GLEIF/national registries ride D-DataIngestion. Parks
**DS-45** (intelligence feeds), **DS-46** (web/contact), **DS-47** (ownership closure/computed-UBO).
Lands as **M21** ([milestones](../milestones.md)), on M5/M19. Additive / expand-only.

---

### D-Religion — A multi-faith `religion` vertical: catalog-driven taxonomy, organization graphs & discovery (reverses the `drafts/` religion drop, refines L-SingleDomain)

**Decision.** A new **`religion`** module models the religion domain for **any faith** — Christianity,
Islam, Judaism, Hinduism, Buddhism, Sikhism, Bahá'í, Shinto, traditional/indigenous, … — **reusing**
the existing `tenant` unit graph, `person`, `membership`, `order`, `authorization`, `localization`,
and the shared `location` entity (D-Location), and adding only the religion-specific structures. This
**reverses the explicit `drafts/` drop of religion-specific concepts** — re-adopted with rationale,
exactly as [D-Location](#d-location--a-shared-standalone-location-entity-postgis--h3-reverses-the-drafts-geography-drop)
re-adopted geography and [D-WebUI](decisions.md#d-webui--an-optional-standalone-nextjs-admin-ui-reverses-the-api-only-no-ui-drop)
re-adopted the UI. **Binding design rule: no faith's vocabulary is hard-coded.** Every
religion-specific value (organization kind, sub-tradition, clergy grade, office type, affiliation
type, site type, service type) is a **catalog row** (D-Code/D-i18n), keyed per religion/tradition and
seeded with cross-faith examples — **never a fixed `CHECK` enum**, never an `if faith == …` branch.

- **Taxonomy (catalogs).** `religion_religions` (top level: Christianity/Islam/Judaism/…) →
  `religion_tradition_families` (nested under a religion: Catholic/Orthodox/Protestant; Sunni/Shia;
  Orthodox/Conservative/Reform; Theravada/Mahayana/Vajrayana; …) → `religion_sub_traditions`
  (optional, generic — rite / school / madhhab / sampradaya: Latin/Byzantine; Hanafi/Ja'fari; …).
- **Organization nodes reuse `tenant_units`** with a **catalog-driven** `unit_kind` via
  `religion_org_kinds` (`code`/translatable `name`, optional `religion_id`, `ordinal`) — e.g.
  denomination/jurisdiction/congregation, school/community/mosque-community, movement/community,
  school/monastery/sangha. They are placed in **three seeded religion graphs** (D-Graphs):
  **`canonical`** (governance/jurisdictional tree, **authority-bearing** — the PDP cascades subtree
  grants here), **`tradition`** (taxonomic, **directory-only**, D-DirectoryGraphs), **`affiliation`**
  (voluntary association DAG, **directory-only**). `religion_org_profiles` (`unit_id` PK/FK,
  `religion_id`, optional `tradition_family_id`/`sub_tradition_id`, `short_code`) holds a unit's faith
  attributes; `religion_org_policies` is a **generic, data-driven** eligibility/exclusion mechanism
  (replacing any faith-specific doctrinal flag such as Christianity's "Nicene-affirming").
- **Clergy** — see [D-ClergyCredential](#d-clergycredential--clergy-grades--credentials-as-a-per-tradition-ordered-catalog--reified-link-faith-agnostic-parallels-d-rank).
- **Lay affiliation** — see [D-ReligiousAffiliation](#d-religiousaffiliation--lay-religious-affiliation-as-a-reified-piispecial-link-on-d-specialpii).
- **Discovery substrate (data, not CMS).** `religion_sites` (a reified Link: worship-community unit ↔
  `location_locations` (D-Location), `site_type_id` catalog, `visibility ∈ {public,unlisted,private}`,
  `public_precision ∈ {exact,street,neighborhood,city,hidden}`, `is_primary` one-per-unit);
  `religion_service_schedules` (per site: day/RRULE, time, IANA `timezone`, service `language` (ISO
  639-3), `service_type_id` catalog, `mode ∈ {in_person,online,hybrid}`, translatable `description`);
  `religion_aliases` (search-only alt names). **Search** is server-side: religion/tradition filter via
  the `tradition`/`canonical` graph **closure** (reuse `tenant_unit_closure`) + proximity/viewport via
  **PostGIS** on `location_locations` (D-Location) + service-language/-time + fuzzy name/alias. The
  `public_precision` projection (coarsen a published coordinate to an H3 cell) lives on the **site
  link**, not the shared location row — so one location can be published at different precisions by
  different owners (the persecuted-community use case).

**Identity-service use (e.g. a FaithMap-style app).** A discovery/CMS application sits **on top** and
uses go-oikumenea as its **identity / authorization / directory backend**: it delegates authentication
to an external IdP, go-oikumenea validates the inbound token and **decides** authorization (the PDP),
and the app's editorial roles map to **unit-scoped role assignments** (D-BaseRoles) on
denomination/community units. Pages/blocks/themes/slugs/content-i18n stay in that app — **out of
scope** here (this is an authorization+directory service, not a CMS).

**Why.** The original `drafts/` source *was* a religion platform (FaithMap); its religion concepts were
dropped only to keep the core generic. The user now wants the vertical back — and richer: any faith,
many faiths per deployment, with the discovery substrate. Reusing the unit graph (which already
supports multi-parent DAGs, multiple named graphs, public/shadow visibility, and a per-graph closure)
means denominations→dioceses→parishes (or councils→mosques, …) need **no new hierarchy machinery** —
they are units in a `canonical` graph, with a `tradition` taxonomy graph and a voluntary `affiliation`
graph beside it. Keeping all faith vocabulary in catalogs (not enums/branches) is the only way one
schema fits every religion and honors **L-SingleDomain's** "no org-type discriminator in code."

**Why not** (a) *A Christianity-shaped schema (denomination/parish/holy-orders columns)*: excludes
every other faith and bakes one tradition's vocabulary into the model — rejected outright per the
user's "all religions" requirement. (b) *A new bespoke hierarchy table for religious bodies*:
duplicates the tenant DAG + closure + visibility + PDP that already exist; religious governance **is**
a unit graph. (c) *Port the FaithMap CMS (pages/blocks/themes)*: that is a content app's job, not an
identity service's; only the **directory/authz/discovery data** belongs here. (d) *A separate
deployment per denomination*: the user wants many faiths/traditions co-resident (ecumenical
discovery), which the graphs + catalogs support without breaking the single-domain spirit.

**L-SingleDomain is refined, not broken** (exactly as D-RankSystems refined L-OneRankScheme). The
single domain per deployment is **religion**; *within* it, multiple religions/traditions coexist as
**catalog data + units in graphs**. There is still **no org-type discriminator branched on in code** —
`unit_kind` and every faith label remain descriptive catalog rows, never a code switch. The lock's
note below points here.

**Consequence.** New `religion` module + the tables above ([religion](../modules/religion.md)),
reusing `tenant`/`person`/`membership`/`order`/`authorization`/`localization` and `location`
(D-Location); new Object/Link/Action kinds in [ontology-mapping](../ontology-mapping.md); religious
belief data rides [D-SpecialPII](#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29).
Lands as the **M22–M25** cluster ([milestones](../milestones.md)) on M3/M5/M6/M7/M10/M19. **Resolves /
promotes DS-48** (the Religion domain); the "Explicitly dropped from `drafts/`" religion line is
updated to mark religion **re-adopted**. Additive / expand-only.

---

### D-ClergyCredential — Clergy grades & credentials as a per-tradition ordered catalog + reified Link (faith-agnostic, parallels D-Rank)

**Decision.** Clergy/religious-functionary **standing** is modeled in the `religion` module — **not**
by reusing the linear `rank` scheme — because ordination/investiture differs from a military ladder
(sacramental/indelible where applicable, concurrent offices, per-tradition shapes). Two parts plus
offices:

- **`religion_clergy_grades`** — an **ordered, per-tradition catalog** (`code`/translatable `name`,
  `grade_category_id` → a generic `religion_grade_categories` catalog instead of a fixed major/minor
  enum, `ordinal`, optional `tradition_family_id`). Generic across faiths: bishop/presbyter/deacon;
  imam/mufti/sheikh; rabbi/cantor; bhikkhu/lama; pujari/swami.
- **`religion_clergy_credentials`** — a **reified Link** `link__clergy_credential` (`Person` →
  `ClergyGrade` within a tradition/organization unit; `granted_on DATE`, optional conferring-authority
  provenance, `status ∈ {active,suspended,revoked}`, `effective_from`/`effective_to`, `source`/
  `confidence`). **Indelible where sacramental**: a revocation/laicization is a **status flip**, never
  a hard delete. Covers ordination / investiture / recognition uniformly.
- **Offices** reuse `membership` **positions** (unit-owned billets — `religion_office_types` catalog:
  pastor / imam-of-mosque / head-rabbi / abbot / …) + authority via `authorization` role assignments;
  conferral/appointment/transfer/suspension are `order` (decree) types.

**A clergy credential is a directory fact, never an authz input** — exactly the **D-Rank** stance
(rank ≠ permission). The PDP never reads a clergy grade; authority over a community comes **only** from
a role assignment on that unit.

**Why.** Every faith has graded religious functionaries, but the grades are not a single comparable
ladder (no NATO-STANAG analog — DS-43), they are per-tradition and often non-linear, and ordination is
frequently *indelible*. A dedicated per-tradition ordered catalog + an effective-dated credential Link
captures this faithfully, while reusing `membership`/`authorization` for the *office* keeps "who may
act" in the one PDP path. Modeling the credential as a reified Link (identity, attributes, history) is
the binding D-Ontology stance.

**Why not** (a) *Reuse the `rank` module*: ranks are a single ordered scheme barred from authority for
a different reason; clergy grades are per-tradition and sacramental, and overloading `rank` muddies
both. (b) *Branch authority on clergy grade*: would reintroduce rank-as-permission, violating D-Rank.
(c) *A bare `person.clergy_grade` FK*: loses ordination history/provenance/status; the relationship
deserves reification. (d) *A cross-tradition comparator*: there is none (DS-43 stays parked); grades
compare only within a tradition's `ordinal`.

**Consequence.** New `religion_clergy_grades` + `religion_grade_categories` + `religion_office_types`
catalogs and the `religion_clergy_credentials` Link ([religion](../modules/religion.md)); new
`ClergyGrade`/`OfficeType` Objects + `link__clergy_credential` in
[ontology-mapping](../ontology-mapping.md). Part of the **M23** milestone. Additive / expand-only.

---

### D-ReligiousAffiliation — Lay religious affiliation as a reified `pii:special` Link (on D-SpecialPII)

**Decision.** A person's **lay religious affiliation/belief** is recorded as a **reified Link**
`link__affiliated_with` (`religion_affiliations`: `Person` → a `religion`/tradition/community unit;
`affiliation_type_id` → a generic **`religion_affiliation_types`** catalog — adherent/member;
catechumen/baptized/confirmed; shahada; bar/bat-mitzvah; …; `status`, `effective_from`/`effective_to`,
`source`/`confidence`). The affiliation value is **GDPR Art. 9 `pii:special`** (D-PIITiers) and is
therefore **envelope-encrypted at rest** with a blind index for uniqueness, gated on
[D-SpecialPII](#d-specialpii--envelope-encryption-extended-to-the-piispecial-tier-resolves-the-person-field-half-of-ds-29);
it is **crypto-erased on person purge** (the `PersonPurged` subscriber extends to it), reads project
through D-PersonReadScope, writes are audited. Rite-of-passage / life-cycle records (baptism /
bar-mitzvah / …) are a reserved generic catalog-typed seam (**DS-49**).

**Why.** Affiliation is the defining lay-side religion datum a deployment may need — and the
project's own D-PIITiers already names *religious affiliation* as the motivating Art. 9 example. The
clergy **credential** (D-ClergyCredential) is an organizational/public fact; lay **belief** is private
special-category data with a stricter regime, so it gets the envelope and crypto-erase. Reifying it as
a Link (not a column) carries provenance/confidence/history the same way the social-account attribution
does.

**Why not** (a) *A plaintext `person.religion` column*: stores Art. 9 data unprotected — forbidden by
the "no special-category PII without the envelope" rule. (b) *Fold lay affiliation into the clergy
credential*: conflates a private belief with a public office. (c) *A fixed affiliation-type enum*:
excludes other faiths' milestones; the catalog is per-tradition.

**Consequence.** New `religion_affiliations` Link + `religion_affiliation_types` catalog
([religion](../modules/religion.md)); new `link__affiliated_with` + `AffiliationType` in
[ontology-mapping](../ontology-mapping.md); extends the `PersonPurged` erasure path. **Depends on**
D-SpecialPII. Lands as **M24**; parks **DS-49** (rite-of-passage records). Additive / expand-only.

---

### D-SpecialPII — Envelope encryption extended to the `pii:special` tier (resolves the person-field half of DS-29)

**Decision.** The envelope-encryption mechanism that **D-CryptoProvider** ships for `pii:sensitive`
(pluggable `KeyProvider`, ciphertext-in-DB, wrapped DEK, blind index, crypto-erase) is **extended
unchanged to the `pii:special` (GDPR Art. 9) tier** for **person/affiliation fields**. No new
mechanism: a `pii:special` value uses the same `value_ciphertext`/`wrapped_dek`/`key_ref`/
`value_blind_index` shape, the same KMS-on-unwrap path, and the same DEK-destruction crypto-erase on
purge. This **resolves the person-field half of DS-29** (the "extend the envelope to `pii:special`"
escalation). The **audit-payload half** of DS-29 (`before`/`after` JSONB at the `pii:special` ceiling)
remains parked — special-category data still must not enter audit payloads — so DS-29's audit scope is
untouched.

**Why.** The religion vertical (D-ReligiousAffiliation) is the first feature that genuinely needs to
**store** Art. 9 data, so the long-anticipated `pii:special` envelope must ship. Because
D-CryptoProvider already abstracts the backend and the blind-index/erase mechanics, extending the tier
is a scope change, not a new design — the cleanest possible way to unblock it. Confining the resolution
to **person/affiliation fields** (not audit payloads) keeps the blast radius small.

**Why not** (a) *Keep `pii:special` blocked and store affiliation in plaintext*: violates the
"no Art. 9 without the envelope" rule. (b) *Invent a separate special-tier mechanism*: needless; the
sensitive-tier envelope already does exactly what Art. 9 needs. (c) *Resolve the audit-payload half
too*: not required by this work and higher-risk — left parked under DS-29.

**Consequence.** The `pkg/crypto` envelope + `KeyProvider` seam now also protect `pii:special`
person/affiliation columns; D-PIITiers' "`pii:special` is **not stored**" caveat is lifted **for
encrypted person/affiliation fields** (audit-payload ceiling unchanged). **Narrows DS-29** to the
audit-payload extension only; as a side effect it lifts the envelope blocker on **DS-38(b)** (gender
identity), though *storing* gender identity stays a separate parked choice. See
[D-CryptoProvider](decisions.md#d-cryptoprovider--pluggable-envelope-encryption-for-sensitive-pii-reshapes-ds-29),
[platform](../modules/platform.md), [open-questions](../open-questions.md) (DS-29). Lands with **M24**.

---

### D-GeoSubdivisions — Seeded ISO-3166-2 subnational-division registry (extends D-Geo)

**Decision.** Geography gains a **second seeded reference layer below the country**: a new shared table
**`geo_subdivisions`**, owned/seeded by [platform](../modules/platform.md) exactly like
[`geo_countries`](decisions.md#d-geo--seeded-iso-3166-country-registry-citizenship-birth-and-residence-as-first-class-person-data)
(D-Geo) — **not** a standalone domain module. Shape: `code TEXT` PK (**ISO 3166-2**, e.g. `'UA-32'`,
`'UA-46'`); `country_code CHAR(2)` → `geo_countries`; optional `parent_id TEXT` self-FK (a nested
subdivision — raion under oblast); `subdivision_type TEXT` (`TEXT`+`CHECK`: oblast/region/state/
province/raion/district/city/…); translatable `name` (default-locale fallback + the i18n store, new
`entity_type='subdivision'`); `status` (`active`/`retired`), `sort_order`, timestamps. All columns
`pii:none`. Instance-admin-extensible (`subdivision.manage`); read via `GET /subdivisions?country=`.
It is a **code-PK reference table like `geo_countries`/`Country`**, *not* an RID-PK Object.

**Seeding.** The target-country subset (UA first) is **migration-seeded** exactly as `geo_countries`
is; the **full global ISO 3166-2 set rides M17** (D-DataIngestion) as an optional connector, so the
~5k-row global table is never baked into a migration. `person_residences.region` and
`location_locations.admin_area_1`/`admin_area_2` **stay free-text for now**; retrofitting them to a
`geo_subdivisions` FK is the parked **DS-51** seam (additive, expand/contract).

**Why.** The vehicle registry (D-Vehicles) needs a *structured* plate-region (the registration region),
and free text defeats the analytics/graph purpose. Modelling subdivisions as a **seeded registry with
translatable names** (rather than free text or a compiled CHECK list) matches the country/locale/graph
registry pattern, lets the i18n store localize subdivision names, and lets an operator add edge-case
entities without a code change — the same rationale D-Geo gave for countries, one level down.

**Why not** (a) *Free-text region* (the original `person_residences.region` choice): unqueryable, no
referential integrity — rejected for the vehicle plate-region. (b) *A vehicle-module-local region
catalog*: subdivisions are a **shared** geography concept (residences, Location addresses also want
them), so it belongs in platform geo, not one domain module. (c) *Migration-seed the full ISO 3166-2
set*: ~5k rows of volatile reference data belong on the M17 ingestion path, not a migration.

**Consequence.** New shared platform table `geo_subdivisions` (seeded like `geo_countries`);
subdivision names join the [localization](../modules/localization.md) store (`entity_type=
'subdivision'`); new `GET /subdivisions` read + instance-scope `subdivision.manage`; new reference
Object `GeoSubdivision` in [ontology-mapping](../ontology-mapping.md). Module count **unchanged** (geo
is platform-owned reference data). Parks **DS-51** (full ISO 3166-2 set + residence/Location retrofit).
Lands with **M26** ([milestones](../milestones.md)) as the shared foundation under D-Vehicles, exactly
as M19 bundled the PostGIS bootstrap with Location. Additive / expand-only.

---

### D-Vehicles — A `vehicle` registry binding people & companies to vehicles (extends D-Ontology)

**Decision.** A new **`vehicle`** module holds vehicles at **registry grade**, scoped to **structural**
data — a brand/model/type taxonomy, the physical vehicle, and the ownership/plate record — so people
and companies link to vehicles in one queryable graph. Volatile vehicle intelligence is **parked**.

- **Reference catalogs** (instance-scope, `code` + translatable `name`, D-Code/D-i18n):
  - `vehicle_types` — a taxonomy **tree** (car/truck/motorcycle/bus/trailer/special…) via a `parent_id`
    self-FK + denormalized root; a **shallow tree with no maintained closure** (mirrors the `rank_types`
    tree — a structural containment FK, **not** a reified Link).
  - `vehicle_brands` — the marque (Toyota/BMW…); `country_code` → `geo_countries` (origin).
  - `vehicle_models` — `brand_id` FK (containment), `name`, `generation`, `manufacture_start`/`_end DATE`.
  - `vehicle_registration_number_types` — plate-type catalog (regular/temporary/transit/diplomatic/
    military/old…).
- **Object** — `vehicle_vehicles`: RID PK; `type_id`/`model_id` FK; `manufacture_date DATE`; `vin`
  (normalized, **unique among active**, nullable for VIN-less vehicles, `pii:basic`); `color`;
  `attributes JSONB` long-tail grab-bag (DS-6-style); soft-delete; audited writes.
- **Reified Links:**
  - `vehicle_brand_manufacturers` (`link__manufactured_by`): `brand_id` → `company_companies`
    (D-Companies), **temporal** `effective_from`/`effective_to` (a brand's manufacturer changes with
    acquisitions).
  - `vehicle_registrations` (`link__registered_to`): the **ownership + plate record** — `vehicle_id` →
    vehicle; a **polymorphic owner** `owner_person_id` **XOR** `owner_company_id` (person|company,
    mirroring D-Companies' polymorphic `OWNS_STAKE`/`FOUNDED` holder); `country_code` → `geo_countries`;
    `subdivision_id` → `geo_subdivisions` (the plate region, optional); `registration_number` (plate,
    **unique among active per country**); `number_type_id` → catalog; **temporal** `effective_from`/
    `effective_to` + `status` (re-registration = a new row, so registration **is** the ownership
    history). Person-owned rows are `pii:basic`, **holder-scoped** through the person owner
    (D-PersonReadScope) and **purge-erased** by a `PersonPurged` subscriber in the vehicle module
    (mirroring the [document](../modules/document.md) module's purge subscriber).
- **Containment FKs (not Links):** model→brand, vehicle→model/type, type→parent — structural FKs per
  the rank/language precedent, never reified.
- **Authorization:** catalogs are instance-scope (`vehicle.manage`); vehicle/registration reads are
  holder-scoped for person-owned rows; all writes are audited Actions (`CreateVehicle`,
  `RegisterVehicle`/`TransferRegistration`, catalog edits).

**Why.** Holding vehicles as first-class entities lets people and companies link into one queryable
graph (owner, fleet operator, manufacturer) — the "better information for relations & graphs" the user
asked for, AutoRia/registry-style. Modelling registration as a **temporal ownership+plate Link** (not a
separate ownership entity) captures transfers as history with the same discipline membership uses; a
polymorphic owner covers personal vehicles **and** company fleets; the brand→manufacturer link reuses
D-Companies so the legal entity behind a marque is one shared record.

**Why not** (a) *A separate ownership entity beside registration*: a vehicle's owner **is** whoever
holds its current registration — folding them avoids a redundant table. (b) *Person-only owner* (as the
raw todo said): excludes fleets; the polymorphic owner is strictly richer and matches the company
ownership graph. (c) *A maintained closure on `vehicle_types`*: the type taxonomy is shallow; a self-FK
+ denormalized root (the rank-type pattern) suffices. (d) *Include insurance/inspection/accident/
telematics now*: volatile, feed-dependent, useless without live sources — parked as **DS-52** (rides
D-DataIngestion connectors when a feed exists), mirroring DS-45 for companies.

**Consequence.** New `vehicle` module + tables above; brand/model reference data + national vehicle
registries ride **M17** (D-DataIngestion); new Object/Link kinds in
[ontology-mapping](../ontology-mapping.md); the `PersonPurged` erasure path extends to person-owned
registrations. Parks **DS-52** (vehicle intelligence feeds) and **DS-53** (column-ize stabilized vehicle
specs out of `attributes`, the DS-6 pattern for vehicles). **Depends on** D-GeoSubdivisions, D-Companies
(M21), and the person directory (M5). Lands as **M26** ([milestones](../milestones.md)).
Additive / expand-only.
