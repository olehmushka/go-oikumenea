# Plan — Process remaining `docs/todo.md` items, then delete it

## Context

`docs/todo.md` ("Raw ideas to discuss before inserting to milestones") holds 5 items. Four are
**already designed and promoted** into binding decisions + milestones:

| todo item | Where it landed |
|---|---|
| 1. Language & language group | M18 / D-Languages |
| 2. Education information (+ Location fields) | M20 / D-Education (+ M19 / D-Location) |
| 3. Companies | M21 / D-Companies |
| 4. Religion | M22–M25 / D-Religion… (already marked ✓ in todo.md) |
| **5. Vehicles** | **not analyzed, not in any milestone** |

So the only outstanding design work is **item 5 (Vehicles)**. Once it is designed into the same
binding shape as the others (a `decisions.md` decision + a `milestones.md` milestone +
`ontology-mapping.md` rows + glossary/README/open-questions updates), `docs/todo.md` has served its
purpose and is deleted — the design source of truth is `docs/` proper, never a scratch todo.

This is a **docs-only** repo (design stage, no code). "Designing" = writing the binding docs.

### Resolved design forks (from the user)

1. **Plate "region" blocker** → introduce a **shared `geo_subdivisions` ISO 3166-2 registry now**
   (a new platform-owned reference table mirroring `geo_countries`/D-Geo), reused beyond vehicles.
2. **Vehicle owner** → **polymorphic Person | Company** (fleets), mirroring D-Companies' polymorphic
   `OWNS_STAKE`/`FOUNDED` holder. Adds a dependency on M21.
3. **Brand → manufacturer** → keep the **temporal brand↔Company link** (`link__manufactured_by`).
   Adds a dependency on M21.

## Design — two new decisions, one new milestone (M26)

Vehicles lands as **M26**, after **M21** (Company — brand↔manufacturer link + company-as-owner) and on
**M5** (Person owner) + `geo_countries`. It bundles a small shared-foundation addition
(`geo_subdivisions`), exactly as **M19** bundled the PostGIS/h3 platform bootstrap with the Location
module.

### Decision A — `D-GeoSubdivisions` (extends D-Geo; platform-owned)

A new **shared, platform-seeded** reference table **`geo_subdivisions`**, mirroring `geo_countries`
(D-Geo) precisely — **not** a domain module:

- `code TEXT` PK = **ISO 3166-2** (`'UA-32'`, `'UA-46'`…); `country_code CHAR(2)` → `geo_countries`;
  optional `parent_id` self-FK (nested subdivision, e.g. raion under oblast); `subdivision_type TEXT`
  (`TEXT`+`CHECK`: oblast/region/state/raion/district/city…); translatable `name` via the i18n store
  (new `entity_type='subdivision'`); `status` (active/retired), `sort_order`, timestamps; all
  `pii:none`. Instance-admin-extensible (`subdivision.manage`); `GET /subdivisions?country=`.
- **Seeded** in-migration for the target countries (UA first), mirroring how `geo_countries` is
  seeded; the **full global ISO 3166-2 set rides M17** (D-DataIngestion) as an optional connector.
- `person_residences.region` and `location_locations.admin_area_1/2` stay **free-text for now**; their
  retrofit to a `geo_subdivisions` FK is the parked **DS-51** seam (non-breaking, expand/contract).

*Why / Why not / Consequence* prose written in the D-Geo voice (registry-not-free-text, i18n names,
operator-extensible, effective history N/A). Note: code-PK reference table like `Country`, **not** an
RID-PK Object.

### Decision B — `D-Vehicles` (extends D-Ontology; new `vehicle` module)

A new **`vehicle`** module — an analytics-grade vehicle registry binding people/companies to vehicles
("better information for relations & graphs"), scoped to **structural** registry data (identity,
brand/model taxonomy, ownership/plate); volatile lifecycle intelligence parked.

- **Reference catalogs** (instance-scope, `code`/translatable `name`):
  - `vehicle_types` — taxonomy **tree** (car/truck/motorcycle/bus/trailer/special…), `parent_id`
    self-FK + denormalized root, **shallow tree, no maintained closure** (mirrors the `rank_types`
    tree, a structural containment FK — *not* a reified Link).
  - `vehicle_brands` — marque (Toyota/BMW…); `country_code` → `geo_countries` (origin).
  - `vehicle_models` — `brand_id` FK (containment), `name`, `generation`, `manufacture_start`/`_end`
    DATE.
  - `vehicle_registration_number_types` — plate-type catalog (regular/temporary/transit/diplomatic/
    military/old…).
- **Object** — `vehicle_vehicles`: RID PK; `type_id`/`model_id` FK; `manufacture_date DATE`;
  `vin` (normalized, **unique among active**, nullable for VIN-less vehicles, `pii:basic`); `color`;
  `attributes JSONB` grab-bag (DS-6-style long-tail specs); soft-delete; audited writes.
- **Reified Links**:
  - `vehicle_brand_manufacturers` (`link__manufactured_by`): `brand_id` → `company_companies`
    (M21), **temporal** `effective_from`/`effective_to` (acquisitions over time).
  - `vehicle_registrations` (`link__registered_to`): **the ownership + plate record** —
    `vehicle_id` → vehicle; **polymorphic owner** `owner_person_id` XOR `owner_company_id`;
    `country_code` → `geo_countries`; `subdivision_id` → `geo_subdivisions` (the plate region,
    optional); `registration_number` (plate, **unique active per country**);
    `number_type_id` → catalog; **temporal** `effective_from`/`effective_to` + `status`
    (re-registration = new row, so registration *is* the ownership history). Person-owned rows are
    `pii:basic`, **holder-scoped** through the person owner (D-PersonReadScope) and **purge-erased**
    (a `PersonPurged` subscriber in the vehicle module, mirroring the `document` module's purge
    subscriber).
- **Containment FKs (not Links):** model→brand, vehicle→model/type, type→parent (per the rank/language
  precedent for structural FKs).
- **Authorization:** catalogs instance-scope (`vehicle.manage`); vehicle/registration reads
  holder-scoped for person-owned; all writes audited Actions (`CreateVehicle`, `RegisterVehicle`/
  `TransferRegistration`, catalog edits).
- **Ingestion:** brand/model reference data + national vehicle registries ride **M17** (D-DataIngestion).
- **Parked seams:** **DS-52** (vehicle lifecycle/intelligence feeds — insurance/MTPL, technical
  inspection, accidents, theft/wanted, odometer, telematics; volatile/feed-dependent, mirrors DS-45
  for companies), **DS-53** (column-ize stabilized vehicle specs out of `attributes`, the DS-6 pattern
  for vehicles).

*Why / Why not / Consequence* prose in the D-Companies voice (one queryable person↔vehicle↔company
graph; registration-as-temporal-link vs a separate ownership entity; polymorphic owner; structural-
only scope with intelligence parked).

## Files to change (at execution time)

All edits mirror exactly how M21/D-Companies was woven in (use it as the template diff):

1. **`docs/architecture/decisions.md`** — add `### D-GeoSubdivisions …` and `### D-Vehicles …` under
   *Resolved this session* (full Decision/Why/Why not/Consequence, citing M26 + the DS seams).
2. **`docs/milestones.md`** —
   - add an **M26** row to the *At a glance* table;
   - add a full **`## M26 — Vehicles (+ subnational subdivisions)`** section (Status: planned; Goal;
     Delivers — `geo_subdivisions` foundation + the vehicle catalogs/object/links; Implements
     D-GeoSubdivisions/D-Vehicles; Excluded/parked DS-51/52/53; Depends on M5, M21; Exit criteria);
   - extend the *Deferred to post-v1* narrative to mention M26 + DS-51/52/53.
3. **`docs/ontology-mapping.md`** — add Object rows (`GeoSubdivision`; `Vehicle`, `VehicleBrand`,
   `VehicleModel`, `VehicleType`, `VehicleRegistrationNumberType`, all *(planned, M26)*) and Link rows
   (`MANUFACTURED_BY`, `REGISTERED_TO`); add the M26 Actions to §3 (`CreateVehicle`/`RegisterVehicle`/
   `TransferRegistration`); note model→brand / vehicle→type as containment FKs (not Links).
4. **`docs/open-questions.md`** — add a new `## vehicle` section with **DS-51** (full ISO 3166-2 set +
   Location/residence retrofit), **DS-52** (vehicle intelligence feeds), **DS-53** (column-ize specs),
   each in the Default/Trigger/`parked` format, citing M26.
5. **`docs/README.md`** — add `vehicle *(M26)*` to the **Planned modules** table (no module-doc link,
   matching `language`/`education`/`company`); mention `geo_subdivisions` beside `geo_countries` in the
   platform row; bump the "M16–M25"/"M0…M25" references that must include M26.
6. **`docs/glossary.md`** — add entries: **Vehicle**, **Vehicle brand/model/type**, **Vehicle
   registration**, **Subdivision** (geo_subdivisions / ISO 3166-2).
7. **Delete `docs/todo.md`.**

**No `docs/modules/vehicle.md`** is created — consistent with `language`/`education`/`company`, whose
module docs "follow at implementation time" (README). Only `location.md` + `religion.md` exist early.
**`CLAUDE.md` is untouched** (it documents the 11 core modules; planned modules live in milestones).

## Verification

1. **Link coherence** (the repo's analog of tests — from CLAUDE.md): run the broken-link checker; must
   print `links OK`:
   ```bash
   cd docs && python3 - <<'PY'
   import re,os,glob
   bad=0
   for md in glob.glob('**/*.md',recursive=True):
       base=os.path.dirname(md)
       for m in re.finditer(r'\]\(([^)]+)\)',open(md).read()):
           link=m.group(1).split('#')[0]
           if link and not link.startswith('http') and not os.path.exists(os.path.normpath(os.path.join(base,link))):
               print(f"broken: {md} -> {link}"); bad+=1
   print("links OK" if bad==0 else f"{bad} broken")
   PY
   ```
2. **No dangling todo references:** `grep -rn "todo.md" docs/` returns nothing after deletion (fix any
   stragglers — e.g. milestones.md / open-questions.md currently link `[todo.md]`).
3. **Cross-doc consistency:** D-GeoSubdivisions + D-Vehicles appear in decisions.md; M26 appears in the
   milestones at-a-glance table and as a section; DS-51/52/53 appear in open-questions.md; the new
   Object/Link rows appear in ontology-mapping.md — and the DS numbers are new (max was DS-50, gaps
   never reused).
4. **Spot-read** the M26 section + both decisions to confirm they read self-contained in the house
   style (Status/Goal/Delivers/Implements/Exit for the milestone; Decision/Why/Why not/Consequence for
   the decisions).
