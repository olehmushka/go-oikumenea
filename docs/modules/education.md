# Module: education

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> Table prefix: `oikumenea.education_*` (+ the person-binding tables `oikumenea.person_education_*` /
> `oikumenea.person_dormitory_stays`)

## Purpose

> **M41 / D-UnifiedOrgGraph (unification):** an **institution is a [tenant](tenant.md) organization**
> (domain=`university`, `pdp_scoped=false` → instance-global reference: public reads, app-permission
> writes, no reach-RLS, no auto graph seed) carrying an **`education_org_profiles` sidecar** (keyed by
> the org RID — kind/country/dates/state; mirrors `religion_org_profiles`). A **unit is a tenant unit**
> in the org's `structure` graph, and the tree's transitive closure is **`tenant_unit_closure`** — the
> dedicated `education_institutions` / `education_units` / `education_unit_closure` tables and the
> `EducationInstitution` / `EducationUnit` / `EducationUnitKind` objects are **gone**; unit kinds are
> `tenant_unit_kinds` under the `university` domain. `EducationService` is a façade over the tenant
> service for structure, owning the sidecar, the reference layer, and the person links. The
> institution/unit `code`s and the education API surface are preserved; only closure verify/rebuild and
> unit-kind upsert endpoints were dropped (the tenant service owns them). The prose below predates the
> merge where it still says "recursive tree / maintained closure / external reference org".

Records the **education domain** as **external reference entities** — *where a person studied or
taught* — at analytics grade (D-Education). An **institution** is a school/university/academy the
service references but does **not** govern; it is a `university`-domain **reference organization**
(M41), kept **distinct from operational PDP-bearing units** by the domain's `pdp_scoped=false` flag.
Its **structure tree** (campus → faculty → department → chair) is **tenant units + the tenant
closure** (M41), **buildings** located via the shared M19 [location](location.md),
**groups** (cohorts), and **positions** (rector/dean/chair billets that fill like
[membership](membership.md) positions). People connect to it through **enrollments** (studied at),
**dormitory stays** (resided in), and **appointments** (held a position); mentorship reuses M14
`person_sponsorships` with an optional education context (no new link type). Like rank/position,
none of these grant authorization — they are directory data.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) —
**Objects:** `EducationInstitution`, `EducationUnit` (a node in the per-institution tree),
`EducationBuilding`, `EducationGroup`, `EducationPosition` (a billet that **exists while vacant**),
and the catalogs `EducationInstitutionKind` / `EducationUnitKind` / `EducationDegreeLevel`.
**Links:** the temporal `link__studied_at` (enrollment), `link__resided_in_dormitory` (dorm stay),
`link__holds_education_position` (appointment); the structural `link__education_unit_parent_of` is a
**containment FK + maintained closure**, not a reified Link. **Actions:** create/update/delete,
fill/abolish/end, closure verify/rebuild — each audited, `action__<type>` RID (education service 14).

- **EducationInstitution** (aggregate root) — a `code` (unique among active), translatable `name`,
  `kind`, optional `country` → `geo_countries`, founded/closed dates, `state` lifecycle. Soft-delete.
- **EducationUnit** — a typed node in **one institution's** strict tree (`parent_id` self-FK,
  RESTRICT). The closure answers "all units under a faculty" in one lookup.
- **EducationBuilding** — a building of an institution (optionally a unit), located via a shared
  `location_id` (M19); `kind` distinguishes a `dormitory` from academic/administrative/etc.
- **EducationGroup** — a cohort under a unit, with an admission year.
- **EducationPosition** — an institution/unit-owned billet (vacant-first); an **Appointment** is its
  one-holder, effective-dated filling (reversible).

The person bindings (`person_education_enrollments`, `person_dormitory_stays`) are **holder-scoped**
person child rows, erased on person **purge** (D-PIITiers).

**Reference layer (M20 extension — `university_ontology.md` adoption).** The module also carries the
**reference-grade** slice of a full university ontology, still as external reference data + person links
(see D-Education *Extension* in [roadmap-decisions](../architecture/roadmap-decisions.md)) — **not** an
operational SIS (no terms/sections/grades/assessments). Additional **Objects:** `EducationProgram`,
`EducationCourse`, `CurriculumVersion`, `ResearchCentre`, `ResearchGroup`, `Grant`, `Publication`,
`GovernanceBody`, `Policy`, `Qualification`, `Scholarship`, `AccreditationEvent`. Additional **Links:**
reified `link__curriculum_item` (version↔course) and `link__course_prerequisite` (course↔course,
cycle-guarded); the person↔reference links `link__authored_publication`,
`link__member_of_research_group`, `link__holds_grant`, `link__member_of_governance_body`,
`link__awarded_qualification` (the diploma award), `link__awarded_scholarship` — all CASCADE, `pii:basic`,
purge-erased. A person's enrollment gains optional `program_id` + `student_number`. The **diploma paper**
itself is a `diploma` row in the [document](document.md) `document_types` catalog. These reference
entities use **plain-string** names (not the i18n store) and are served by a second Conjure service,
`EducationReferenceService` (same `/education/v1` base-path). Full DDL in
`migrations/20260601000021_education_reference.sql`.

## Data model

Conventions per [conventions.md](../architecture/conventions.md) (RID PKs via `new_id(14,…)`,
`TIMESTAMPTZ`, `set_updated_at`, soft-delete, `TEXT`+`CHECK` enums, `code`/translatable `name`). Full
DDL in `migrations/20260601000020_education.sql`.

**Reference catalogs** (`education_institution_kinds`, `education_unit_kinds`,
`education_degree_levels`) — RID PK, `code` (unique active) + translatable `name`, `status`,
`sort_order`. `education_degree_levels` adds `isced_level INT` and is **migration-seeded with ISCED
2011 levels 0–8** (the institution/unit kinds are seeded with a starter set and instance-extensible).

**`education_institutions`** — `id`, `code`, `name`, `kind_id` → kinds (RESTRICT),
`country_id` → `geo_countries` (RESTRICT, nullable), `founded_on`/`closed_on` DATE,
`state ∈ active|closed|merged`, soft-delete. `UNIQUE (code) WHERE deleted_at IS NULL`.

**`education_units`** — `id`, `institution_id` (RESTRICT), `parent_id` self-FK (RESTRICT, NULL =
top-level; the parent must belong to the **same institution**, enforced in the application),
`kind_id`, `code`, `name`, `status ∈ active|archived`, `sort_order`.
`UNIQUE (institution_id, code) WHERE deleted_at IS NULL`. A unit carries **no degree level** — degree
level is a property of a *programme* / *enrollment* / *qualification* (a unit hosts many levels at once),
so it lives on those entities, not on the org-structure node.

**`education_unit_closure`** — the maintained transitive closure (mirrors
`language_languoid_closure`): `(ancestor_id, descendant_id, depth)` PK, no RID; includes the reflexive
`(u,u,0)` row. Recomputed in SQL (per institution) inside each unit insert/reparent transaction.

**`education_buildings`** — `id`, `institution_id` (RESTRICT), `unit_id` (RESTRICT, nullable),
`location_id` → `location_locations` (RESTRICT, nullable), `code`, `name`,
`kind ∈ academic|dormitory|administrative|library|sports|other`.

**`education_groups`** — `id`, `unit_id` (RESTRICT), `code`, `name`, `admission_year`,
`status ∈ active|graduated|disbanded`.

**`education_positions`** — `id`, `institution_id` (RESTRICT), `unit_id` (RESTRICT, nullable =
institution-level), `code`, translatable `title`, `status ∈ active|abolished`, `sort_order`.
`UNIQUE (institution_id, code) WHERE deleted_at IS NULL`.

**`education_appointments`** *(Link `link__holds_education_position`)* — `id`, `person_id` (CASCADE),
`position_id` (RESTRICT), `status ∈ active|ended`, `effective_from`/`effective_to`. **One holder per
billet:** `UNIQUE (position_id) WHERE status='active' AND deleted_at IS NULL`.

**`person_education_enrollments`** *(Link `link__studied_at`, `pii:basic`)* — `id`, `person_id`
(CASCADE), `institution_id` (RESTRICT), optional `unit_id`/`group_id`/`degree_level_id` (RESTRICT),
`field_of_study`, `status ∈ enrolled|graduated|withdrawn|expelled|on_leave`, `qualification`,
`effective_from`/`effective_to` DATE. Erased on person purge.

**`person_dormitory_stays`** *(Link `link__resided_in_dormitory`, `pii:contact`)* — `id`, `person_id`
(CASCADE), `building_id` (RESTRICT), `room`, `status ∈ active|ended`, `effective_from`/`effective_to`.
Erased on person purge.

**Sponsorship education context** — `person_sponsorships` (M14) gains two **nullable** columns
(D-Education, no new link type): `enrollment_id` → `person_education_enrollments` (`ON DELETE SET
NULL`) and `education_role ∈ professor|tutor|curator|advisor`.

## Conjure API surface

`EducationService` (`/education/v1`, full sketch in `api/education.conjure.yml`):

| Group | Ops | Perm |
|---|---|---|
| Catalogs | `GET /institution-kinds`, `/unit-kinds`, `/degree-levels`; `PUT /institution-kinds`, `/unit-kinds` | read / `education.catalog.manage` |
| Institutions | `POST/GET/PUT/DELETE /institutions[/{id}]` (list paginated + `query`) | `education.read` / `education.manage` |
| Units | `POST /institutions/{id}/units`, `GET /institutions/{id}/units` (closure depth), `GET/PUT /units/{id}`, `POST /units/{id}/reparent`, `…/verify-closure`, `…/rebuild-closure` | `education.read` / `education.manage` |
| Buildings / Groups | CRUD under an institution / unit | `education.read` / `education.manage` |
| Positions | `POST/GET/PUT /…positions`, `/abolish`, `/fill`, `POST /appointments/{id}/end` (filter `state=vacant\|filled`) | `education.read` / `education.position.manage` |
| Person bindings | `GET/POST /persons/{id}/enrollments`, `PUT/DELETE …/{enrollmentId}`; same for `/dormitory-stays` | `education.read` / `education.enrollment.manage` |
| **Reference layer** *(`EducationReferenceService`, `api/education_reference.conjure.yml`)* — programs, courses, curriculum versions/items, prerequisites, research centres/groups, grants, publications, governance bodies, policies, qualifications, scholarships, accreditation events (CRUD); person links `…/persons/{id}/{publication-authorships,research-memberships,grant-holdings,governance-memberships,qualification-awards,scholarship-awards}` | `education.read` / `education.manage` / `education.enrollment.manage` |

Translatable labels return as a `locale → text` map (assembled via [localization](localization.md)
`NamesByID`). Filling a filled billet → `Education:PositionAlreadyFilled`; a reparent cycle →
`Education:UnitCycleDetected`.

## Dependencies

- **Calls:** [person](person.md) (person exists; CASCADE), [location](location.md) (building FK),
  [geo](location.md) (institution country), [localization](localization.md) (name maps for the M20 base
  entities; the reference-layer entities use plain-string names), [document](document.md) (the `diploma`
  document-type for the awarded-qualification paper), [audit](audit.md) (every write records a `system`
  Action in-transaction).
- **Called by:** [person](person.md) **purge** erases `person_education_enrollments` +
  `person_dormitory_stays`; the person sponsorship surface carries the optional education context.
- National institution registries (e.g. UA EDBO) can ride the [hermenea](hermenea.md) import pipeline
  via the generic `POST /import/{objectType}` — a future connector, not built in M20.

## Authorization touchpoints

Defines/gates `education.read`, `education.manage`, `education.position.manage`,
`education.enrollment.manage`, and the instance-scope `education.catalog.manage`. Education entities
are **instance-global external reference data** — they carry **no unit scope of their own**, so the PEP
satisfies these **anywhere** (like [location](location.md)), and there is **no RLS** on the education
tables. Nothing here is ever an authorization input (D-Rank parallel).

**The INSTITUTION ROW itself is shadow-gated** (M58 ticket 5). An institution IS a
`university`-domain tenant organization plus a sidecar (M41 / D-UnifiedOrgGraph), so it carries that
organization's public/shadow bit and is trimmed by the same rule `listOrganizations` applies —
organization reach DERIVED from unit reach (D-VisibilityScope). This is a correction, not an addition:
the gate should always have been here and was not, so `listInstitutions`, `getInstitution` and unified
**search** handed shadow organizations to any caller holding `education.read` from M20 until M58
ticket 5. All three now route through one helper (`gateInstitutions` in the transport,
`scope.NewOrgScope` for search), and a gated-out institution is `InstitutionNotFound` rather than a
permission error, because `shadow` hides existence. The institution's CHILDREN (units, buildings,
groups, positions) are not separately gated: they are reached through the institution's RID, which
this gate is what a caller obtains, and they carry no visibility bit of their own.

## Patterns

- **Closure** mirrors [tenant](tenant.md): a strict single-parent tree (not a DAG) → the
  `language_languoid_closure` shape, recomputed per institution inside the write transaction.
- **Positions/appointments** mirror [membership](membership.md): vacant-first billets, one active
  holder enforced by a partial-unique index, reversible end (status flip + `effective_to`).
- **Person bindings + purge** mirror M18 `person_languages`: education-owned link tables, erased in the
  person purge transaction (cross-table `DeleteAll*` queries in the person repository).
- **Audit-on-write** (D-Audit): each mutation records one `system`-actor Action (`new_id(14,3,0)`).

## Invariants & safety

- A unit's `parent_id` stays within the **same institution**; reparenting is **cycle-guarded** via the
  closure; the closure has no drift after any structural write (verify/rebuild diagnostics exist).
- **One active appointment per position**; abolishing a filled billet is refused (`Education:InUse`).
- A `code` is unique among active rows in its scope (institution, or unit for groups).
- Enrollments/dorm stays are **holder-scoped** and **purge-erased**; the sponsorship `enrollment_id`
  FK is `ON DELETE SET NULL` so erasing an enrollment never orphans a sponsorship.
- Soft-delete everywhere; institutions/units/buildings/groups are never hard-deleted from app code.

## Open seams / future

- **Facets & dashboards (M58).** [D-ObjectFacets](../architecture/decisions.md#d-objectfacets--one-per-object-type-facet-vocabulary-driving-both-list-filters-and-per-module-stats-endpoints-extends-d-visibilityscope-d-personreadscope-constrained-by-d-datascope) lands filters + a stats endpoint + a console dashboard
  for this module's listable types: `GET /institutions/stats` and enrollment stats over `kindId`/`countryId`/`foundedOn`/`state` and `institutionId`/`programId`/`degreeLevelId`/`status`/`startedOn`; the degree-level bar orders by **ISCED level, not count**. Plus the module's first ontology-registry entry.
  Facets and proposed charts are catalogued in [facets.md](../architecture/facets.md).

- **Operational SIS** (academic terms/calendars, course sections, section-level enrollment with grades,
  assessments, GPA) is **deliberately out of scope** — the module stays external-reference (D-Education
  *Extension*). It could become a separate operational module later if ever needed.

- **Multi-incumbent positions** (shared chairs) — relax the one-holder index (the membership seam).
- **National registry connectors** (UA EDBO, ELSST) over [hermenea](hermenea.md) — institutions/units
  carry the `(source, source_version, imported_at)` provenance slots conceptually; the import
  object-type registration is a follow-up.
- **University-as-legal-entity** tie to companies — deliberately deferred (independent
  modules; a later cross-link seam).
- **Establishment control** for education positions (central vs per-institution) — the same DS-11 seam
  as membership; default is per-institution.
