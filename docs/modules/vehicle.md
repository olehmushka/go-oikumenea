# Module: vehicle

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [roadmap-decisions](../architecture/roadmap-decisions.md)
> Table prefix: `oikumenea.vehicle_*`

## Purpose

Holds **vehicles as first-class entities** at **registry grade** (D-Vehicles) — a brand/model/type
taxonomy, the physical vehicle (VIN), the **brand→manufacturer** link (to a M21 [company](company.md)),
and the **ownership+plate registration record** that binds a vehicle to its owner. The point is
*further linking*: people **and** companies join one queryable graph (owner, fleet operator,
manufacturer behind a marque) — AutoRia/registry-style. A vehicle is **external reference data** the
service references but does **not** govern: like [company](company.md)/[education](education.md) it is
**distinct from the deploying org's [tenant](tenant.md) units** (no PDP, no shadow visibility). It
reuses the RID-keyed `geo_countries` registry for the registering country and the WOF **`geo_places`**
gazetteer (placetype=region, D-GeoPlaces) for the plate **region**. Like rank/position, none of this
grants authorization — it is directory data. Scope is **structural only**; volatile vehicle
intelligence (insurance/MTPL, inspection, accidents, theft, odometer, telematics) is **parked**
(DS-52), and column-izing stabilized specs out of `attributes` is parked (DS-53).

## Entities & aggregates

- **Reference catalogs** (`code` + translatable `name`, instance-extensible, D-Code/D-i18n):
  - `vehicle_types` — a taxonomy **tree** (car/truck/motorcycle/bus/trailer/special…): a `parent_id`
    self-FK + denormalized `root_id`, **no maintained closure** (the `rank_types` pattern — a structural
    containment FK, not a reified Link).
  - `vehicle_brands` — the marque (Toyota/BMW…); `country_id` → `geo_countries` (origin, nullable).
  - `vehicle_models` — `brand_id` containment FK, `generation`, manufacture window.
  - `vehicle_registration_number_types` — the plate-type catalog (regular/temporary/transit/diplomatic/
    military/old).
- **Object** — `vehicle_vehicles`: the physical vehicle. `type_id`/`model_id` FK; `vin` (normalized,
  unique among active when present, nullable, `pii:basic`); `color`; `manufacture_date`; `attributes`
  JSONB long-tail grab-bag; soft-delete. The **RID is the external handle** (no separate `code`).
- **Reified Links** (D-Ontology):
  - `vehicle_brand_manufacturers` (`link__manufactured_by`, RID `17,2,1`): brand → a `company`-domain
    `tenant_organizations` row (the manufacturer company — M41 / D-UnifiedOrgGraph),
    **temporal** (`effective_from`/`effective_to`) — a marque's manufacturer changes with acquisitions.
  - `vehicle_registrations` (`link__registered_to`, RID `17,2,2`): the **ownership + plate record** —
    `vehicle_id` → vehicle; a **polymorphic owner** `owner_kind ∈ {person,company}` + `owner_id` (text,
    no FK — F-014); `country_id` → `geo_countries`; `subdivision_id` → `geo_places` (the plate region,
    placetype=region, optional); `registration_number` (unique among **active** per country);
    `number_type_id` → catalog; **temporal** + `status ∈ {active,closed}`. A re-registration is a **new
    row** (the prior closed), so registration **is** the ownership history.

## Data model

One schema `oikumenea`, prefix `vehicle_*`; RID PKs via `new_id(17,kind,type)` with a `*_rid_shape`
CHECK; `set_updated_at()` trigger; soft-delete `deleted_at`; `TEXT`+`CHECK` enums. The plate region is
an FK into the shared `geo_places` gazetteer — **no `geo_subdivisions` table is built** (D-GeoSubdivisions
was superseded by D-GeoPlaces in M16). Country FKs are `country_id uuid → geo_countries(id)` (the geo
re-key amendment), not an ISO `code`. RLS: vehicle entities are instance-global external reference data
(like company/education/location), so **no RLS** is enabled. Migration: `0027_vehicle`.

## Conjure API surface

`VehicleService` (`/vehicle/v1`): catalog reads + instance-scope upserts (types/brands/models/
number-types); `createVehicle`/`listVehicles`/`getVehicle`/`updateVehicle`/`deleteVehicle`;
`listRegistrations`/`registerVehicle` (registers **or** transfers — closes the active row first) /
`closeRegistration`; `listManufacturers`/`addManufacturer`/`removeManufacturer`; and the read-only
`listPersonVehicles` (`GET /persons/{id}/vehicles`). The region picker is served by the geo module
(`GET /geo/v1/places?country=&placetype=region`). Translatable catalog names are returned as a
`locale → text` map (D-i18n); vehicle/registration display labels are best-effort default-locale strings.

## Dependencies

- [company](company.md) (M21) — the manufacturer behind a brand + a company owner of a registration.
- [person](person.md) (M5) — a person owner of a registration.
- The RID-keyed `geo_countries` registry (jurisdiction) and the `geo_places` gazetteer
  ([geo](../architecture/decisions.md) / D-GeoPlaces) — the plate region.
- [audit](audit.md) (M1) — every write records an Action; [localization](localization.md) (M2) — catalog
  name maps.

## Authorization touchpoints

Vehicle entities carry no unit scope of their own (external reference data), so reads gate on
`vehicle.read`, vehicle/registration/manufacturer writes on `vehicle.manage`, catalog management on the
instance-scope `vehicle.catalog.manage` — all satisfied anywhere via the PEP. Owning a vehicle never
grants authority (parallel to rank/position). Writes are audited Actions under a `system` actor.

## Patterns

- **Registration-as-history**: transfers are new rows with the prior closed — no mutation, the same
  discipline membership uses.
- **Polymorphic owner** (person|company): `(owner_kind, owner_id text)`, no FK — mirrors company's
  `OWNS_STAKE`/`FOUNDED` holders (F-014).
- **Shallow type tree**: `parent_id` + denormalized `root_id`, no closure (the rank-type pattern).
- **Plate region over a shared gazetteer**: the WOF `geo_places` region, not a vehicle-local region list.

## Invariants & safety

- VIN unique among active rows when present; plate `registration_number` unique among **active**
  registrations per country.
- A `subdivision_id` must be a `geo_places` **region** (app-validated on write → `Vehicle:RegionInvalid`).
- A registration owner is a person **XOR** a company (`owner_kind` CHECK).
- Person-owned registrations are `pii:basic`, holder-scoped, and **erased on person purge**
  (`ErasePersonRegistrations` — the `PersonPurged` event subscriber is a deferred shared seam, exercised
  directly today, mirroring [document](document.md)/[religion](religion.md)).

## Open seams / future

- **DS-52** — vehicle lifecycle/intelligence feeds (insurance/MTPL, inspection, accidents, theft/wanted,
  odometer, telematics), connector-fed via hermenea (mirrors DS-45 for companies).
- **DS-53** — column-ize stabilized vehicle specs out of `attributes` (the DS-6 pattern).
- **DS-51** — the full WOF locality coverage backfill (the gazetteer rollout, M16) and the residence/
  Location `admin_area_*` retrofit; the vehicle plate region already rides `geo_places`.
- The `PersonPurged` event subscriber wiring (shared with document/religion) once the bus carries it.
