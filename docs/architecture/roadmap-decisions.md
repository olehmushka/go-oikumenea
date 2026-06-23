# Roadmap decisions (planned tier — M16–M27)

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

Decisions here, in milestone order: **D-Worker** (M16, *superseded*) · **D-Hermenea** (M16, supersedes
D-Worker, folds D-DataIngestion) · **D-GeoPlaces** (M16, WOF gazetteer + first connector, supersedes
D-GeoSubdivisions, pulls PostGIS forward) · **D-DataIngestion** (M17→folded into M16) · **D-Languages**
(M18) · **D-Location** (M19) · **D-Education** (M20) · **D-Companies** (M21) · **D-Religion** /
**D-ClergyCredential** / **D-ReligiousAffiliation** / **D-SpecialPII** (M22–M25) · **D-GeoSubdivisions**
(*superseded by D-GeoPlaces*) / **D-Vehicles** (M26) · **D-ClientSDK** (M27, unified Go + TypeScript
SDKs from the Conjure contract; *in implementation*).

> Cross-references into the **core** decisions (D-CryptoProvider, D-WebUI, D-Geo, D-Rank, D-Ontology,
> D-PersonReadScope, …) point at [`decisions.md`](decisions.md); references among the planned-tier
> decisions resolve within this file.

---

### D-Worker — A first-class background-job runtime (promotes DS-25)

> **Superseded by [D-Hermenea](#d-hermenea--an-out-of-process-ingestion--scheduler-companion-supersedes-d-worker-folds-d-dataingestion) (M16).** The
> *need* (scheduled syncs, a job queue, at-least-once with idempotency, retry/backoff, drain, a job
> ledger, health+audit) is unchanged and carried forward — but the *placement* is reversed: the
> runtime is **not** in-process inside oikumenea. It lives in a **separate companion binary,
> `hermenea`**, with its **own Postgres**, coupled to oikumenea **only over HTTP**. The single-binary
> premise below ("keeps the self-hosted, single-binary deployment story intact") is the one part
> intentionally dropped (rationale in D-Hermenea). The original text is retained for provenance.

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

### D-Hermenea — An out-of-process ingestion & scheduler companion (supersedes D-Worker, folds D-DataIngestion)

**Decision.** The background-job runtime and the reference-data ingestion pipeline are realized as a
**second deployable, `hermenea`** (`cmd/hermenea`) — a companion ETL + scheduler service beside
`oikumenea`, with its **own PostgreSQL** and its own Atlas migrations, coupled to oikumenea **only over
HTTP** (it never touches oikumenea's database). This **supersedes [D-Worker](#d-worker--a-first-class-background-job-runtime-promotes-ds-25)**
(reverses *in-process*) and **collapses [D-DataIngestion](#d-dataingestion--a-generic-reference-data-ingestion--connector-framework-extends-d-ontology) (M17) into M16** (the connector
framework moves into hermenea). The name pairs *hermenea* (interpretation — it interprets/imports the
world) with *oikumenea*.

- **What lives in hermenea** (its own DB, own ontology/RIDs): the `Connector` interface
  (`Fetch(ctx, source) → RawBatch`; **HTTP(S)** + the degenerate **`file`** connector), `import_sources`
  registry, `import_raw_batches` raw staging, a per-object-type **mapper registry** (raw records → the
  canonical envelope), an in-process **cron scheduler + `worker_jobs` queue** (`SELECT … FOR UPDATE SKIP
  LOCKED`, at-least-once with idempotency keys, **exponential backoff with per-job-type config**,
  dead-letter after max attempts, witchcraft-managed **graceful drain**, a **job-health reporter**), and
  the `import_runs` lineage ledger.
- **What lives in oikumenea**: a **generic `POST /import/{objectType}`** endpoint over an upsert
  registry — each importable object-type registers a **code-keyed, idempotent, non-destructive** upsert
  handler run in one transaction and emitted as **audited Actions** (preserving bulk-ingest ≠
  audited-edit); **per-row provenance** (`source`, `source_version`, `imported_at`) on imported rows; the
  `import.manage` permission.
- **Two trust directions, two runtime secrets** (env/ECV refreshable, never install config, never
  stored): `HERMENEA_OIKUMENEA_TOKEN` authorizes hermenea→oikumenea **import** calls; `OIKUMENEA_HERMENEA_TOKEN`
  authorizes oikumenea→hermenea **push triggers** (`POST /sync/{source}`). Beyond cron, an oikumenea
  admin/console action pushes a trigger to hermenea.
- **Service principal** — the import token maps to a `hermenea-importer` **service principal** holding
  exactly `import.manage` (instance scope), audited as a **`system`** actor (the audit actor-shape CHECK
  already permits `person | system`); see the **L-AuthzOnly** amendment in [decisions.md](decisions.md).

**Why.** The user requires hard service separation: ingestion is failure-prone, bursty, and pulls in
outbound network deps and parser surface that should **not** share a process or a database with the PDP.
An out-of-process companion keeps oikumenea's core synchronous and dependency-light, makes the importer
independently deployable/scalable/restartable, and forces a clean **HTTP-only** contract (oikumenea's
public import API is the only coupling — the extraction-ready boundary, realized). Per-row provenance +
idempotent re-sync + ingest≠edit (the transferable Foundry parts of D-DataIngestion) are all preserved.

**Why not** (a) *In-process worker (D-Worker as written)*: couples ingestion blast radius to the PDP
process and DB; rejected for separation. (b) *Hermenea writes oikumenea's DB directly*: breaks the
module/service boundary and the audit/PDP invariants; the HTTP import API is the only sanctioned write
path. (c) *Pull triggers (hermenea polls oikumenea)*: added latency + a polling endpoint for no benefit
over a direct push; push chosen. (d) *One shared secret both directions*: distinct per-direction secrets
limit blast radius. (e) *OIDC client-credentials for the service*: heavier IdP setup than the operator's
runtime shared secret; the env-secret path is simpler and audited as `system`.

**Consequence.** A new `cmd/hermenea` binary + `internal/hermenea/**` + `api/hermenea.conjure.yml` +
`migrations/hermenea/` (own `atlas.sum`); oikumenea gains the import endpoint + service-principal auth +
`import.manage` + provenance columns + a push-trigger client. **D-Worker** and **D-DataIngestion** are
superseded/folded; **M16 absorbs M17** ([milestones](../milestones.md)). DS-25 stays promoted; **DS-44**
(more connector types) stays parked. Additive / expand-only on oikumenea; hermenea is greenfield.

---

### D-DataIngestion — A generic reference-data ingestion & connector framework (extends D-Ontology)

> **Folded into [D-Hermenea](#d-hermenea--an-out-of-process-ingestion--scheduler-companion-supersedes-d-worker-folds-d-dataingestion) (M16 absorbs M17).** The pipeline
> shape below (sources/connectors → raw staging → mapper → canonical envelope → idempotent upsert →
> lineage) is **adopted unchanged**, but **relocated**: connectors, raw staging, the mapper registry,
> `import_sources`/`import_raw_batches`/`import_runs` and the scheduler live in the **hermenea**
> service's own DB — *not* `pkg/dataimport` inside oikumenea. oikumenea keeps only the generic
> `POST /import/{objectType}` upsert endpoint + per-row provenance + `import.manage`. The original text
> is retained for the pipeline-design rationale it still carries.

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
- **Population** — the migration **bootstraps the ~50 most-spoken languages** (`level='language'`,
  required columns only; real glottocodes so the first import updates them in place) so the catalog is
  usable on a fresh DB before any import — mirroring the `geo_countries` seed. Beyond that, by default
  the sources fetch **live from upstream master each run** via hermenea's
  `http-files` streaming connector + a Go transform: Glottolog CLDF (`languages.csv` + `values.csv`,
  ~27k languoids) and CLDR (`supplementalData.xml` + `iso-639-3.tab`) are staged to disk and mapped by
  the `CLDFMapper` / `SupplementalMapper` (the Go port of `deploy/language-presets/gen-presets.py`),
  emitting the whole forest as one page (single transaction). The **bundled preset** (the pinned
  Glottolog 5.3 CLDF snapshot, `deploy/language-presets/*.json`, opt-in asset / never a migration,
  CC-BY-4.0 attribution carried) remains as the offline/air-gap `file`-connector fallback. Tracking
  master trades reproducibility for freshness; a failed run is logged + retried + dead-lettered and
  never corrupts the catalog (imports are transactional).

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
Lands as **M18** ([milestones](../milestones.md)), on M16 (the M17 pipeline, folded into M16) + M5/M2;
M3 for the unit tie. Additive / expand-only.

**Built (M18) — reconciliations.** Three details where the as-built code refines this decision (the
code is authoritative once built; recorded here so the two don't drift):
1. **RID PK, not `code` PK.** `language_languoids` (and `writing_systems`/`writing_system_script_types`)
   are **RID-keyed** (`id` PK, service 13), with `code`/`iso639_3` retained as UNIQUE lookup keys —
   consistent with **F-014/D-ResourceIdentifiers** making every structural entity RID-keyed (as
   `geo_countries`/`geo_places` already are). The glottocode is still the universal external spine.
2. **Writing systems are migration-seeded; the language↔script M:N is imported from CLDR.** Glottolog
   has no script data and ISO-15924 is only a code registry, so `writing_systems` + `script_types` are
   seeded in the migration (small/stable) and `language_writing_systems` is loaded by a second import
   object-type, **`language-scripts`**, sourced from CLDR `languageData` (`is_primary`). The Glottolog
   forest loads via the **`language-scheme`** object-type.
3. **Import shape.** The ~27k-languoid snapshot loads in one in-memory, parent-first envelope (not paged)
   so the closure + `family_code` rebuild sees the whole forest in one transaction; the bundled presets
   (`deploy/language-presets/{glottolog-5.3.json,cldr-scripts.json}`) are reproducible via
   `gen-presets.py`. UI is **deferred** (the `ui` gate).

---

### D-Location — A shared, standalone Location entity (PostGIS; app-derived MGRS; multi-format input)

**Decision.** A new **`location`** module provides one **standalone** place entity that anything with a
location references by FK. **`location_locations`** carries a **required** `geom GEOGRAPHY(POINT, 4326)`
(**PostGIS**), an **app-derived** MGRS string (pure Go, written on every coordinate change; nullable for
polar UPS points), the original input coordinate in a **`source_coordinate` JSONB** column, and a
structured postal address: `country_id` (NOT NULL → `geo_countries`, D-Geo), `admin_area_1`/
`admin_area_2`, `locality`, `street`, `house_number`, `postal_code`, `raw_address`; soft-delete; a
spatial GIST index. A `LocationService` offers CRUD + radius/bbox (`ST_DWithin`) queries. The coordinate
is accepted in **several formats** — WGS84 lat/lon, MGRS, UTM, СК-42 (Gauss-Krüger, numeric + grid) —
via a **pluggable converter registry** (`internal/geo/domain/coordinate.go`); the application converts
each to canonical WGS84, derives the MGRS, and persists the original input. **PostGIS is the only
operator-DB prerequisite** (the stock postgis image); the schema-bootstrap enables it and the readiness
gate checks for it. This **reverses the explicit `drafts/` drop of `location`/PostGIS/geography** —
re-adopted with rationale, exactly as D-WebUI re-adopted the UI.

**Amendment (2026-06-17).** The original decision derived **MGRS + H3** in the DB via the **h3-pg**
extension and a plpgsql `location_mgrs()` function/trigger, and accepted only WGS84 lat/lon input. This
was amended: **H3 is dropped entirely** (its sole intended use was efficient radius search, which PostGIS
already serves exactly via `ST_DWithin` on the GiST index — H3 is redundant given a real spatial index),
**MGRS moves to the application** (pure Go, no cgo, no DB extension), the coordinate gains **multi-format
input + `source_coordinate`**, and the operator image reverts from the custom `Dockerfile.postgres`
(postgis + h3-pg) to the **stock postgis image**. The change was applied by editing the M19 migration
`0019_location` in place (no new migration; the location tables had no production data).

**Why.** Both the education domain (buildings, campuses, dormitories — D-Education) and companies
(registered/operating addresses — D-Companies) need precise, queryable places, and the project's
analytics ambition ("better information for building relations & graphs") wants real spatial indexing.
A single shared entity dedupes addresses and enables "everything near point X" once, instead of
re-inventing address columns per owner. PostGIS `ST_DWithin` + GiST is the standard, efficient radius/kNN
stack; deriving MGRS in the app keeps the spatial dependency to PostGIS alone (no cgo, no custom image),
and supporting multiple input formats lets operators enter coordinates as they have them (e.g. MGRS or
СК-42 off a military map) without pre-converting.

**Why not** (a) *Embedded address columns per owner*: duplicates schema, blocks cross-entity spatial
queries and dedup. (b) *App-layer geometry, plain columns*: loses native spatial indexing / radius
queries (`ST_DWithin`) — so the authoritative `geom` stays PostGIS. (c) *Coordinate optional*: the
deployments here want spatial analytics, so the coordinate is the required spine and a precise point is
mandated (address-only records are out of scope — geocode first). (d) *Keep H3 / derive in DB*: H3 adds
an operator-DB extension (custom image) and a cgo binding for no benefit over the GiST `ST_DWithin` path;
deriving MGRS in plpgsql couples the operator image to h3-pg unnecessarily. (e) *Stay faithful to the
`drafts/` drop*: that drop was for a church-discovery scope; the army/university analytics scope
genuinely needs geography.

**Consequence.** New `location` module + `location_locations`; **PostGIS** becomes the operator-DB
prerequisite (bootstrap + readiness gate, [platform](../modules/platform.md)); the L-Conventions enum
note is unaffected. New Object kind `Location` in [ontology-mapping](../ontology-mapping.md). The
"Explicitly dropped from `drafts/`" list is updated to mark geography **re-adopted** (H3 stays dropped).
The planned religion `public_precision` projection (D-Religion, M22) was sketched as a coarsening to an
H3 cell at read time; with H3 no longer in the stack it must adopt an app-side coarsening (e.g. rounding
the coordinate / a geohash, or computing a single cell in Go) when M22 lands — a planned-tier seam, not a
built dependency. Lands as **M19** ([milestones](../milestones.md)); a foundation for M20 + M21. Additive
/ expand-only.

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

**Extension (M20 reference layer — `university_ontology.md` adoption).** The M20 base is enriched with
the **reference-grade** slice of a full university ontology (`docs/university_ontology.md`), still as
**external reference data + person↔reference directory links** — explicitly **not** an operational SIS.

- **Adopted (additive, migration `0021_education_reference`):** *Curriculum/courses* — `education_programs`,
  `education_courses`, `education_curriculum_versions`, `education_curriculum_items` (reified
  `link__curriculum_item`), `education_course_prerequisites` (reified `link__course_prerequisite`,
  Go-side cycle guard); a person's enrollment gains optional `program_id` + `student_number`.
  *Research* — `education_research_centres`, `education_research_groups`, `education_grants`,
  `education_publications`. *Governance/credentials* — `education_governance_bodies`,
  `education_policies`, `education_qualifications`, `education_scholarships`,
  `education_accreditation_events`. *Person links (CASCADE, `pii:basic`, purge-erased):*
  `person_publication_authorships`, `person_research_memberships`, `person_grant_holdings`,
  `person_governance_memberships`, `person_education_qualifications` (the diploma award),
  `person_scholarship_awards`. A `diploma` row is added to the `document_types` catalog (the paper lives
  in the [document](../modules/document.md) module; the academic fact is the qualification award).
- **Deliberately excluded (kept reference, not operational):** academic **terms / calendars**,
  **course sections**, section-level **enrollment with grades**, **assessments**, GPA / grading; the
  source doc's **Person→Student/StaffMember subtype** split (the repo keeps a single person + directory
  attributes); **bi-temporal validity everywhere** (effective-dated links + soft-delete instead); and
  `country_code CHAR(2)` (the richer `geo_countries` FK is kept).
- **Surface.** A **second Conjure service** `EducationReferenceService` (own package, same `/education/v1`
  base-path) carries the reference CRUD + person links — isolated from the M20 base service. Reference
  names/titles are plain `string` (external reference data, **not** the i18n translation store) — a
  deliberate simplification vs the i18n'd M20 base entities. Reuses `education.read` / `education.manage`
  / `education.enrollment.manage`; no new permission strings. RID service 14 extended with object kinds
  9–20 and link kinds 5–12. No PDP/RLS (instance-global reference data).

**Why this scope.** The directory benefits from the curriculum/research/governance/credentials *facts*
about an institution and a person's relationship to them (analytics + relationship graphs), but the
operational SIS (scheduling, registration, grades) is a different product and conflicts with the
external-reference + single-person + soft-delete model. Adopt the reference half; leave the rest.

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

**Refined (M22 design, 2026-06-19).** The taxonomy half is reshaped so it can hold the **real-world
depth of any faith — especially Christianity, which nests far deeper than three levels** (Christianity →
Catholic/Eastern-Orthodox/Oriental-Orthodox/Church-of-the-East/Protestant → Lutheran/Reformed/Anglican/
Baptist/Methodist/Pentecostal/… → rite/movement → a named denomination). The three fixed catalog tables
(`religion_religions`/`religion_tradition_families`/`religion_sub_traditions`) are **replaced by a single
recursive `religion_taxa` tree + maintained closure**, reusing the proven `language_languoids`-forest /
`education_unit_closure` pattern. Six refinements:

1. **Recursive `religion_taxa` + `religion_taxa_closure`.** One self-referencing tree (`parent_id`
   RESTRICT, NULL = root religion) with a denormalized root `religion_id` (derived via closure, like
   `language_languoids.family_code`). Arbitrary depth; no fixed level columns.
2. **Catalog-driven level marker.** Each taxon carries a `rank_id` → an **ordered `religion_taxon_ranks`
   catalog** (seeded `religion`→`branch`→`tradition`→`sub_tradition`→`denomination`, extensible) so the
   main branches stay findable and Christianity-the-religion is distinguishable from
   Eastern-Orthodoxy-the-tradition. The rank set is structural, not faith vocabulary, so it stays
   catalog-driven (no `CHECK` enum of faith terms — the lock holds).
3. **Religion-type ("theism") classification.** A multi-tag `religion_classifications` catalog
   (monotheistic/polytheistic/henotheistic/monistic/nontheistic/pantheistic/animistic/dualistic/…),
   attached M:N to taxa (`religion_taxon_classifications`) and seeded at the `religion` level. Resolution
   is **nearest-declared-wins**: a descendant taxon **or** an org unit
   (`religion_unit_classifications`) may declare its own set, which **fully overrides** the inherited
   one (walk up the closure to the nearest declaring node). Read-time projection, not stored.
4. **Taxonomy/organization boundary = hybrid.** The seeded tree goes to sub-tradition/movement for most
   faiths **plus** the globally-significant classificatory bodies (Eastern Orthodox autocephalous
   churches, the 23 Eastern Catholic sui-iuris churches, Oriental Orthodox churches, major Protestant
   denominations) as `denomination`-level taxa. Concrete **governed instances** (a specific
   diocese/parish) remain `tenant_units` linking to the nearest taxon.
5. **Org → taxon as M:N tagged links, one primary.** A unit's faith classification is
   `religion_org_classifications` (`is_primary` partial-unique per unit), replacing the single
   `tradition_family_id`/`sub_tradition_id` columns on `religion_org_profiles` — a body often fits
   several classifications at once (Reformed Baptist; Eastern Catholic = Catholic + Byzantine-rite).
6. **Wikidata anchor + curated seed.** Each taxon carries an optional `wikidata_id`; the **rich curated
   seed ships in the migration** (deep Christianity + broad world religions), generated by a
   reproducible `deploy/religion-presets/gen-presets.py` anchored to Wikidata QIDs. A hermenea import
   connector is left as an open seam.

**Why the refinement.** The user's binding requirement is "cover all existing religions, especially
Christianity, reflect the real world as much as possible." A fixed 3-level shape cannot hold
Christianity's branches→families→denominations (uneven depth across faiths); a recursive tree + closure
can, and the repo already proves that pattern twice. The faith-agnostic lock is **unchanged** — every
faith term remains a catalog row, never an enum or `if faith == …` branch. **M22 builds only the
taxonomy + organization slice** (catalogs, `religion_taxa` + closure, org profiles/classifications/
policies, the three graphs); clergy (M23), affiliation (M24), and discovery (M25) are unchanged below
and deferred. The `religion_clergy_grades`/`religion_affiliation_types`/etc. catalogs in M23–M25 keep
their per-tradition shape but now FK a `religion_taxa` node instead of a `tradition_family` row.

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
**Status: landed (M23).** Built in migration `0024_religion_clergy` (`religion_grade_categories` `16,1,7`
/ `religion_clergy_grades` `16,1,8` / `religion_office_types` `16,1,9` / `religion_clergy_credentials`
`16,2,2`); the per-tradition catalogs FK `religion_taxa` via `tradition_taxon_id`. `clergy.manage`
gates credential writes over the canonical graph. *Offices as Positions remain future work.*

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
**Status: landed (M24).** Built in migration `0025_religion_affiliation` (`religion_affiliation_types`
`16,1,10` / `religion_affiliations` `16,2,3`). The optional belief value is envelope-encrypted (reuses
`pkg/crypto` `Cipher`); `affiliation.manage` gates read+write; `ErasePersonAffiliations` crypto-erases.
The `PersonPurged` auto-trigger stays a shared open seam with document `ErasePersonRecords` (exercised
directly today).

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
**Status: landed (M24).** The existing `pkg/crypto` envelope (`Seal`/`Open`/`BlindIndex`) now also
protects the `pii:special` `religion_affiliations` value columns — no new mechanism, exactly as decided.

---

### D-GeoPlaces — Who's-On-First administrative gazetteer (`geo_places`), the first hermenea connector (extends D-Geo, supersedes D-GeoSubdivisions, pulls PostGIS forward)

**Decision.** Geography gains a **full administrative gazetteer** sourced from **Who's-On-First (WOF)**,
loaded by hermenea's **first real connector** (M16, D-Hermenea). A new shared platform table
**`geo_places`** holds the four WOF admin placetypes — **country / region / county / locality**
(city·town·village; WOF has no town/village split — that is a `population` property, not a placetype) —
as a single tree: an `id uuid` **RID PK** under the **`location` service (code 12)** — minted by
`new_id(12,1,2)` with a `rid_*` shape `CHECK` like every other Object (F-014; *amended from the
original `wof_id BIGINT` PK*) — with `wof_id BIGINT NOT NULL UNIQUE` retained as the stable WOF
import/concordance key; `placetype TEXT`+`CHECK`; `parent_id uuid` self-FK → `geo_places(id)`
(a structural containment edge derived from `wof:hierarchy` to the nearest imported ancestor, the
`rank_types` pattern — **not** a reified Link); denormalized `country_id uuid` → `geo_countries(id)`;
default-locale `name` (other locales via the i18n store, `entity_type='geo_place'`); `population`;
`hierarchy`/`concordances` JSONB; `status` (`active`/`retired`, the latter mirroring WOF
`mz:is_current=0`/supersession — **non-destructive**); the `(source, source_version, imported_at)`
provenance trio; and **PostGIS** geometry — `geom GEOMETRY(Geometry,4326)` (full shape) + DB-derived
`centroid`/`bbox`, served as GeoJSON via `ST_AsGeoJSON`, GIST-indexed. The existing **`geo_countries`
is enriched in place** (additive `wof_id` + geometry + `iso_a3`/`numeric_code`), and a
`placetype=country` WOF record mirrors its geometry onto the country row in the same import
transaction. Coverage is **global** (all four placetypes worldwide), rolled out **per country** (one
`wof-geo-<iso>` source each, cron-staggered, Ukraine first).

**Amendment (M16 — geo becomes RID-keyed, full RID end-to-end).** `geo_countries` left the
natural-key carve-out: it gained an `id uuid` **RID PK** (`new_id(12,1,1)`, location service 12) with
`code CHAR(2)` demoted to `NOT NULL UNIQUE` (the canonical external **lookup** key). **All 8 country
FK consumers** — `person.country_of_birth`, `person_citizenships`/`person_residences`/`person_phones`,
`document_documents.issuing_country`, `document_personal_code_schemes`, `rank_systems`, and
`geo_places` — repoint to `geo_countries(id)` (the columns become `*_id uuid`). The **country RID
flows end-to-end** through domain → Conjure → web (ISO `code` is lookup-only). Because country entry
was free-text ISO with no countries endpoint, a read-only **`GeoService` (`GET /geo/countries`,
`country.read`)** returns `{id, code, name, status}` so clients resolve a code to its RID and populate
pickers. Ingestion is unchanged on the wire — the WOF/ISO importer still streams natural keys and
resolves `wof_id`/`code → id` in SQL on upsert (an unresolvable non-zero parent/country trips the FK
loudly, preserving the parent-first RESTRICT guarantee); the phone country is derived as an ISO code
and likewise resolved in SQL; the rank **preset import** resolves its ISO country code to the RID
before insert.

**Pipeline / ingestion.** WOF ships as per-country "combined" SQLite admin DBs (`.db.bz2`), not a JSON
API, and a single country's geometry far exceeds the 16 MiB in-memory batch cap. So M16 adds: a new
**`wof-sqlite` StreamingConnector** (fetch `.db.bz2` → bzip2-decompress → stage to a temp file, never
in-memory/BYTEA) and a **paged-mapper seam** (`PagedMapper`) that walks the SQLite **parent-first**
(country→region→county→locality) emitting bounded pages, each loaded as its own canonical envelope via
`POST /import/geo-places`; `import_runs` aggregates the counts. Idempotency is keyed on `source_version`
(re-import of the same WOF edition skips; a newer one updates; never deletes). Hermenea stays
PostGIS-agnostic — it ships GeoJSON **text**; only oikumenea materializes geometry.

**Why.** A real gazetteer with shapes + parentage is the "better information for relations & graphs"
the registry verticals need; WOF is an open, ID-stable, concordance-rich global admin source. Keeping
`geo_countries` as the ISO-keyed FK anchor (enriched, not re-keyed) preserves every existing consumer
while giving countries WOF geometry. A WOF tree subsumes ISO-3166-2 subdivisions **and** reaches down to
localities, which ISO-3166-2 cannot. Pulling **PostGIS forward** from D-Location (M19) is required to
store/query the shapes now; M19 reuses the same stack.

**Why not** (a) *Keep the ISO-3166-2 `geo_subdivisions` plan (D-GeoSubdivisions)*: ISO-3166-2 has no
codes for cities/villages and no geometry — WOF is strictly richer, so D-GeoSubdivisions is
**superseded**. (b) *Full WOF replacement (re-key `geo_countries` onto WOF ids)*: breaks binding D-Geo
and 8 FKs — rejected. (c) *JSONB GeoJSON instead of PostGIS*: dead-weight, unqueryable — rejected.
(d) *Single planet SQLite in one batch*: multi-GB, no failure isolation — per-country streaming +
paging chosen instead.

**Consequence.** New `geo_places` table + enriched `geo_countries` (folded into the bootstrap migration,
PostGIS extension added); new import object-type `geo-places` + handler in `dataimport`; hermenea gains
the `wof-sqlite` connector, the `PagedMapper` seam, a file-staged raw batch (`staged_path`), and the
`geo-places` mapper; new reference Object `GeoPlace` in [ontology-mapping](../ontology-mapping.md).
**Supersedes D-GeoSubdivisions**; **D-Vehicles**' plate-region FK `subdivision_id` → `geo_places`
(placetype=region). Global locality coverage is millions of rows + GBs of geometry — a long, staggered
backfill, not a single sync. Lands in **M16** ([milestones](../milestones.md)). Additive / expand-only.

---

### D-GeoSubdivisions — Seeded ISO-3166-2 subnational-division registry (extends D-Geo) — **SUPERSEDED by D-GeoPlaces**

> **Superseded (M16).** Replaced by **D-GeoPlaces** (the WOF `geo_places` gazetteer), which covers
> region/county/locality with codes, geometry, and parentage that ISO-3166-2 cannot. The original
> design is retained below for provenance; `geo_subdivisions` is **not built**, and D-Vehicles'
> `subdivision_id` now targets `geo_places`.

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

---

### D-ClientSDK — Unified Go + TypeScript SDKs generated from the Conjure contract (extends D-Conjure, D-WebUI)

**Decision.** Ship **two symmetric, published client SDKs** for the go-oikumenea API — one **Go**, one
**TypeScript** — both **generated from the single source of truth**, the `api/*.conjure.yml` Conjure
contract, and each exposing a **unified façade** that binds one base URL + one bearer token to every
service. Consumers `go get` / `npm install` two independent packages; this repo consumes them from the
tree. See the [clients module doc](../modules/clients.md).

- **One contract, three generated surfaces.** The server interfaces (`internal/conjure`), the Go SDK
  (`clients/go/oikumenea/<module>`) and the TS SDK (`clients/typescript/src/generated`) are all generated
  from the same IR, so none can drift (D-Conjure). Generated code is **never hand-edited**.
- **Go SDK** (`clients/go/`, the existing nested module): per-service generated clients **plus a
  hand-written façade** `client.New(baseURL, token, opts…)` (and `NewWithTokenProvider`) in
  `clients/go/client.go` — one `Dial`, one bound token, every service as a field.
- **TS SDK** (`clients/typescript/`, new publishable npm package): per-service clients generated by
  **conjure-typescript** (a Node CLI — no JVM) from the same Conjure IR, **plus a hand-written façade**
  `createOikumeneaClient({ baseUrl, token? })` in `src/index.ts` over one conjure-client
  `DefaultHttpApiBridge`. The IR is obtained offline by reusing `tools/ir2openapi -dump-ir`; its package
  names are rewritten `oikumenea.<m>` → `oikumenea.api.<m>` (a derived-artifact transform — the wire
  contract is untouched) so they meet conjure-typescript's ≥3-segment rule. Driven by
  `scripts/gen-ts-client.sh` (with a `--verify` drift gate, the analog of `godel conjure --verify`).
- **Hermenea is a member of both façades.** oikumenea reverse-proxies the hermenea control/read API at
  `/hermenea/v1/*` (D-Hermenea), so `client.Hermenea` / `client.hermenea` reach the companion through
  oikumenea — a single client + base URL covers both oikumenea-native and hermenea-proxied endpoints.
  No DB coupling is introduced; this is purely the existing HTTP proxy surfaced in the SDKs.
- **Web consumes the TS SDK** (supersedes the in-`web` `openapi-typescript`/`openapi-fetch` layer,
  D-WebUI): `web/` imports `oikumenea-client` as a `file:` dependency. The browser client points the
  façade at the BFF (`/api/oikumenea`, no token in the browser); Server Components bind the session
  bearer via `oikumenea()` (`web/src/lib/api/server.ts`). The BFF proxy and token-injection trust model
  are unchanged.

**Why.** The Go SDK and an OpenAPI-derived TS type file already existed but diverged in kind (a real
typed SDK vs. types-only inside the app) and neither offered a unified one-call client. Generating both
from the one Conjure IR makes the published packages true peers that cannot drift from the server or
each other; the hand-written façades give the "single client for everything" ergonomics the per-service
generated clients lack. Using conjure-typescript (not the OpenAPI hop) keeps the TS SDK's shape a
faithful mirror of the Go SDK.

**Why not** (a) *OpenAPI → openapi-typescript for the published TS SDK*: it adds a lossy generator hop
(IR → OpenAPI → TS) so the TS surface wouldn't mirror the Go SDK; conjure-typescript consumes the same
IR the Go SDK does. (The OpenAPI artifact stays for docs/reference, and `openapi-typescript` is dropped
from `web`.) (b) *Keep the TS client inside `web/` only*: not distributable; external consumers need a
package. (c) *Hand-write the per-service clients*: violates D-Conjure (drift). (d) *A separate
gateway/proxy service for hermenea*: unnecessary — oikumenea already proxies `/hermenea/v1/*`
(D-Hermenea); the SDKs just expose it.

**Consequence.** New `clients/typescript/` npm package + `scripts/gen-ts-client.sh` +
`clients/typescript/scripts/rewrite-ir-packages.mjs`; a `-dump-ir` flag on `tools/ir2openapi`; the new
façade files `clients/go/client.go` and `clients/typescript/src/index.ts`. `web/` swaps its API layer onto
the SDK (drops `schema.d.ts` + `gen-api-types.sh` + `openapi-fetch`/`openapi-typescript`); its Docker
build context moves to the repo root so the `file:` SDK dep is in scope, and `outputFileTracingRoot` is
set so the standalone bundle includes it. No schema, contract, or server-behavior change — **additive /
tooling-only**. Publishing tags (npm / pkg.go.dev) are a follow-up; in-repo consumption verifies the
SDKs. Lands as **M27** ([milestones](../milestones.md)).
