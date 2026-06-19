# M20 — Education

## Context

The next unbuilt milestone is **M20 — Education** (stage board: `decided`, `Designed` 🚧). M0–M19 are
all **verified**; the binding decision **D-Education** already exists in
`docs/architecture/roadmap-decisions.md`, so this is a full vertical-slice build through the pipeline:
**designed → backend → migrated → ui → verified**.

M20 adds a new **`education`** module: external reference institutions (where a person studied/taught),
their internal structure + buildings, and the person bindings (enrollments, dorm stays, education
positions). It is **independent of companies** and **distinct from the deploying org's tenant units**
(its own structure tree, never the PDP-bearing `tenant_units`). It is additive / expand-only and reuses
two existing templates almost verbatim: **membership** (positions + one-holder effective-dated
appointments) and **tenant** (recursive tree + maintained transitive closure). It references the M19
`location_locations` for buildings/dorms.

New **RID service = 14** (12=location, 13=language; 14 is the next free — confirmed in `pkg/rid`).
New migration = `migrations/20260601000020_education.sql`.

## Design decisions (the few non-obvious calls)

1. **`education_units` is a single-parent tree** (not a multi-graph DAG like tenant). Its closure
   therefore mirrors **`language_languoid_closure`** (`ancestor_id, descendant_id, depth`, PK on the
   pair) — *not* the per-graph tenant closure. `parent_id` self-FK `ON DELETE RESTRICT`; closure
   rebuilt per-institution in SQL inside the write transaction (recursive CTE over `parent_id`,
   exactly the `RebuildClosureForGraph` shape minus the `graph_id`).
2. **Person-binding links live on `EducationService`** (service 14), as person sub-resources
   `GET|PUT|DELETE /education/v1/persons/{personId}/enrollments` and `.../dormitory-stays`. This
   matches `ontology-mapping.md` (which already lists `STUDIED_AT` / `RESIDED_IN_DORMITORY` under the
   `education` module). The **purge erasure**, however, follows the M18 `person_languages` precedent:
   the **person** module gets two new cross-table `DeleteAll*` queries so the erasure stays in the
   purge transaction (see `internal/person/adapters/repository.go:232`).
3. **Sponsorship education-context** is two additive **nullable** columns on the existing
   `person_sponsorships` (no new link type — D-Education): `enrollment_id uuid REFERENCES
   person_education_enrollments(id) ON DELETE SET NULL` + `education_role text CHECK (… IN
   ('professor','tutor','curator','advisor'))`. The `ALTER TABLE` lands in the **0020** migration
   (after the enrollments table exists). The person module's sponsorship domain/sqlc/Conjure/transport
   surface the two new optional fields.
4. **Catalogs are RID-keyed objects** mirroring `location_location_types` (`code`/translatable `name`/
   `status`/soft-delete): `education_institution_kinds`, `education_unit_kinds`. `education_degree_levels`
   is **migration-seeded ISCED 2011 0–8** (RID PK via `new_id()` — reads no GUC, so seeding RID rows in
   the migration is fine; carries `isced_level INT` + `sort_order`).
5. **Positions mirror membership exactly**: `education_positions` (institution/unit-owned billet,
   vacant-first, `UNIQUE (owner, code) WHERE deleted_at IS NULL`) + `education_appointments`
   (one-holder partial-unique on `(position_id) WHERE status='active' AND deleted_at IS NULL`,
   effective-dated, reversible via status flip).

## Stage 1 — Designed gate (docs)

- **`docs/modules/education.md`** — new module doc on the fixed template (purpose → entities → data
  model → Conjure sketch → dependencies → authorization touchpoints → patterns → invariants → open
  seams). Use `docs/modules/membership.md` + `docs/modules/location.md` as the shape reference.
- **`docs/ontology-mapping.md`** — the M20 rows already exist as placeholders (objects L76–78, links
  L146–150). Just remove the `*(planned, M20)*` markers and fix RID-service references to **14**.
- **`docs/glossary.md`** — add: education institution, education unit, degree level (ISCED), education
  group, education position, enrollment, dormitory stay.
- **`docs/milestones.md`** — flip the M20 stage-board row (L100) `Designed 🚧 → ✅` and the later gates
  as each lands; the M20 prose section (L710–743) already exists.
- Run the CLAUDE.md link-checker after doc edits.

## Stage 2 — Backend (`api/education.conjure.yml` + `internal/education/`)

Generate-first (D-Conjure): author the contract, run the gödel conjure generation, then implement.

**`api/education.conjure.yml`** — new `EducationService`:
- Catalog reads/writes: `GET /institution-kinds`, `GET /unit-kinds`, `GET /degree-levels` (+ admin
  create/update for the two managed catalogs).
- Institutions: `POST /institutions`, `GET /institutions/{id}`, `PUT /institutions/{id}`,
  `GET /institutions` (paginated), soft-delete.
- Units (per-institution tree): `POST /institutions/{id}/units`, `GET /units/{id}`,
  `PUT /units/{id}`, reparent endpoint, `GET /institutions/{id}/units` (tree/closure-backed),
  closure verify/rebuild (mirror `TenantService`).
- Buildings: CRUD, FK `locationId` → M19 (`GET /location/v1/locations/{id}` resolves the place).
- Groups: CRUD under a unit.
- Positions + appointments: mirror `MembershipService` endpoints
  (`POST /units/{id}/positions`, `.../positions/{id}/fill`, `.../appointments/{id}/end`, vacant filter).
- Person bindings: `GET|PUT|DELETE /persons/{personId}/enrollments`,
  `GET|PUT|DELETE /persons/{personId}/dormitory-stays`.
- Conjure error types (`Education:InstitutionNotFound`, `Education:UnitCycleDetected`,
  `Education:PositionAlreadyFilled`, etc.) — copy the membership/tenant error patterns.

**`internal/education/`** — hexagonal layers mirroring `internal/membership` + `internal/tenant`:
- `domain/education.go` — Institution, Unit, Building, Group, Position, Appointment, Enrollment,
  DormitoryStay + `Repository` interface + `Validate()` methods.
- `application/service.go` — `inTx` + `record(ctx, tx, …)` audit-on-write (mint Action RID via
  `SELECT oikumenea.new_id(14,3,0)`); closure recompute in the unit-write txn; one-holder/cycle guards.
- `adapters/repository.go` + `adapters/queries/education.sql` (sqlc) + generated `educationsql/`.
  SQLSTATE→domain error mapping (23505/23503) like `membership/adapters/repository.go:259`.
- `transport/service.go` — implements the generated `EducationService`, PEP-gated reads, i18n
  locale-map assembly for catalog/institution `name` via `LocalizationService`.
- `module.go` — `Register(info, pool, audit, loc, enforcer)`; seed the two managed catalogs +
  degree-levels idempotently (tenant `module.go` seeding pattern).

**Wiring:**
- `cmd/oikumenea/main.go` — add `education.Register(info, pool, auditSvc, locSvc, enforcer)` after the
  geo/location + language registrations (~L215).
- `pkg/rid` — add `SvcEducation = 14` + the service-14 type registry rows (object/link/action) and the
  `platform_rid_types` mirror; `AssertMatches` must stay green.
- `sqlc.yaml` — add the `internal/education` package block (copy the language block).
- **Authorization** — add the `education.*` permission strings (`education.read`,
  `education.institution.manage`, `education.position.create`, `education.appointment.create`,
  `education.enrollment.manage`, catalog `education.catalog.manage`) to the code-defined permission
  catalog; institution/unit-scoped where applicable (catalogs instance-scope).

**Person module touches (sponsorship extension):**
- `api/person.conjure.yml` — add `enrollmentId` + `educationRole` optional fields to `Sponsorship` +
  `UpsertSponsorshipRequest` (L425/L699).
- `internal/person/{domain,application,adapters,transport}` — surface the two new optional fields on
  the sponsorship path; SQL upsert/select read/write the new columns.
- `internal/person/adapters/queries/person.sql` + `repository.go` — add `DeleteAllPersonEducationEnrollments`
  and `DeleteAllPersonDormitoryStays`; call both in `Purge(...)` after L232 (the `person_languages`
  block).

## Stage 3 — Migration (`migrations/20260601000020_education.sql`)

Expand-only, conventions per `migrations/20260601000019_location.sql` header. Contents:
- `platform_rid_services` row `(14,'education')` + `platform_rid_types` rows for every object/link kind
  (and `(14,3,0,'action')`).
- Catalogs: `education_institution_kinds`, `education_unit_kinds` (RID PK `(14,1,6/7)`, code/name/
  status/soft-delete); `education_degree_levels` (RID PK `(14,1,8)`, ISCED `isced_level`, **seeded 0–8**).
- Objects: `education_institutions` (RID `(14,1,1)`, `code`, `name`, `kind_id`, `country_id` →
  `geo_countries(id)`, founded/closed dates, `state` lifecycle); `education_units` (RID `(14,1,2)`,
  `institution_id`, `kind_id`, `parent_id` self-FK RESTRICT, `code`, `name`); `education_unit_closure`
  (mirror `language_languoid_closure`); `education_buildings` (RID `(14,1,3)`, `institution_id`,
  `location_id` → `location_locations(id)`, `kind` incl. `dormitory`); `education_groups` (RID
  `(14,1,4)`, `unit_id`, admission year).
- Links: `person_education_enrollments` (RID `(14,2,2)`, `person_id` CASCADE, `institution_id`, optional
  `unit_id`/`group_id`, `degree_level_id`, field/specialty, effective-dated, status, qualification;
  `pii:basic`); `person_dormitory_stays` (RID `(14,2,3)`, `person_id` CASCADE, `building_id`, room,
  period; `pii:contact`); `education_positions` (RID `(14,1,5)` object — billet) +
  `education_appointments` (RID `(14,2,4)` link, one-holder + effective-dated).
- `ALTER TABLE oikumenea.person_sponsorships ADD COLUMN enrollment_id … ADD COLUMN education_role …`.
- `set_updated_at` triggers, soft-delete partial indexes, `rid_shape` CHECKs, `COMMENT ON COLUMN … pii:*`
  on every PII column, and the `schema_version` revision bump to `0020_education`.
- **No RLS** on the person-binding link tables (holder-scoped, like `person_languages` /
  `person_relationships`); institution/unit/position tables are not unit-scoped against `tenant_units`
  — document the deliberate RLS exemption in the migration header.
- Run `atlas migrate hash`; rebuild dev DB; confirm idempotent re-apply.

## Stage 4 — Web UI (`web/`)

- `web/src/lib/ontology/registry.ts` — add object-type entries (`institution`, `education_unit`,
  `building`, `group`, `education_position`) following the `location` entry (L667); add an "Education"
  links block to the `person` entry pointing at `/education/v1/persons/{id}/enrollments`.
- `web/src/app/(dashboard)/education/page.tsx` — institution browser + create/structure view, modelled
  on `locations/page.tsx` (`bffGet`/`mutate`, `@/components/ui`, `CountrySelect`).
- A person-page editor for enrollments + dorm stays + the sponsorship education-context (extend the
  existing person sponsorship form), mirroring `UnitLanguageForms.tsx` / `LanguagePicker.tsx`.
- `pnpm typecheck` + `pnpm build` must pass.

## Stage 5 — Verify (M20 exit criteria)

Mirror the M18/M19 integration-test approach against a real DB (`scripts/setup-test-db.sh` provisions
`oikumenea_test`). Prove the milestone's exit criteria end-to-end:
- Create an institution → faculty unit → study group; closure answers "all units under the institution"
  in one lookup; a reparent recomputes the closure; a cycle attempt is rejected.
- Record a person's **enrollment** at the faculty + group with an ISCED degree level + graduation
  qualification; attach the professor as an **education-context sponsorship** (enrollment ref + role);
  record a **dorm stay** in a `dormitory` building (FK to a real M19 location).
- Fill a **dean `education_position`**; filling an already-filled billet → `PositionAlreadyFilled`;
  end the appointment → vacates.
- Each write emits exactly one `system`-actor audited Action.
- **Purge** the person → `person_education_enrollments`, `person_dormitory_stays` erased and the
  sponsorship education-context cleared.
- Re-apply the migration idempotently; `go build ./...` + `go test` green; web typechecks/builds.

## Critical files

| Area | Path |
|---|---|
| Module doc | `docs/modules/education.md` (new) |
| Ontology / glossary / board | `docs/ontology-mapping.md`, `docs/glossary.md`, `docs/milestones.md` |
| Contract | `api/education.conjure.yml` (new); `api/person.conjure.yml` (sponsorship fields) |
| Go module | `internal/education/{domain,application,adapters,transport,module.go}` (new) |
| RID / sqlc / root | `pkg/rid/{rid.go,registry.go}`, `sqlc.yaml`, `cmd/oikumenea/main.go` |
| Person touches | `internal/person/{domain,application,adapters,transport}` (sponsorship + 2 purge queries) |
| Migration | `migrations/20260601000020_education.sql` (new) |
| Web | `web/src/lib/ontology/registry.ts`, `web/src/app/(dashboard)/education/page.tsx`, person forms |

## Notes
- Work on a new branch `m20-education` (repo is on clean `main`). Per repo norms (memory), milestones
  land **uncommitted** on a branch unless the user asks to commit.
- Templates to copy nearly verbatim: `internal/membership` (positions/appointments), `internal/tenant`
  (closure rebuild/verify), `internal/language` + M18 `person_languages` (cross-module person sub-resource
  + purge), `internal/geo` location (building/dorm FK + web page).
