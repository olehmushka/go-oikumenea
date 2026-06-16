# M19 — Location (the next milestone)

## Context

M0–M18 are verified; the stage board (`docs/milestones.md`) shows **M19 — Location** at the
`designed` gate (Decided ✅, Designed ✅, Backend ⬜). It is the next milestone and a **foundation**
that M20 (education buildings/dorms) and M21 (company addresses) will reference by FK.

M19 introduces one **shared, standalone place entity** — a precise coordinate plus a structured
postal address — with DB-derived MGRS + H3 spatial indexes, so anything with a location points at one
queryable record. Binding decision: **D-Location** (`docs/architecture/roadmap-decisions.md`); module
doc already written: `docs/modules/location.md`.

**Two decisions taken with the user up front:**
1. **MGRS/H3 are DB-derived (faithful to D-Location).** The operator image `postgis/postgis:16-3.4`
   ships PostGIS but **not** h3-pg, and PostGIS has no native MGRS. So M19 provisions a **custom
   Postgres image** (adds h3-pg) and an **MGRS plpgsql function**, derives both via a trigger, and
   the readiness gate checks the extensions are present.
2. **Scope includes the web console UI** (location browser + create-from-coordinate + radius search +
   ontology-registry entry), matching how M18 shipped UI.

The work lives in the **existing `internal/geo` module** — RID **service 12 is already named
`location`** (`platform_rid_services`), and currently holds only the read-only `country`/`geo_place`
registry. M19 adds the writeable `location_locations` entity beside it; `GeoService` stays read-only,
a new audited `LocationService` is added.

## Key facts grounded in the codebase

- **Geometry never reaches sqlc.** Established pattern (`internal/dataimport/adapters/queries/geo_places.sql`):
  geometry crosses the wire as **GeoJSON text**, materialized with `ST_SetSRID(ST_GeomFromGeoJSON(...),4326)`;
  reads project scalars (`ST_X`/`ST_Y`/`ST_AsGeoJSON`). MGRS/H3 are plain `TEXT` columns sqlc reads directly.
- **PostGIS already enabled** in `migrations/20260601000000_schema_bootstrap.sql` (M16). M19 adds only
  `h3` + `h3_postgis` and the MGRS function in the new migration.
- **RID gap:** service 12 has **no `kind=3` action type** (geo was read-only — see bootstrap line 165–166).
  Migration 0019 must `INSERT` `(12,1,3,'location')`, `(12,1,4,'location_type')`, **and `(12,3,0,'action')`**
  into `platform_rid_types`. (`new_id()` reads no GUC, so migrations may seed RID rows directly.)
- **Write-module wiring template:** `internal/rank/module.go` — `Register(info, pool, audit, loc, enforcer)`,
  `RepositoryFactory` injection, audited-in-txn writes, localization for `locale→text` name maps.
- **Read-only call site to update:** `cmd/oikumenea/main.go:202` (`geo.Register(info, pool, enforcer)`).

## Implementation

### 1. Custom Postgres image (h3-pg)
- New `Dockerfile.postgres` extending `postgis/postgis:16-3.4`: install build deps + compile/install
  **h3-pg** (`zachasme/h3-pg`, provides `h3` + `h3_postgis` extensions) against PG16.
- Swap the DB `image:` for `build:` in `docker-compose.yml` (prod), `docker-compose.dev.yml`, and the
  atlas/test DB services that must share the image. Document the prerequisite in `docs/modules/platform.md`.

### 2. Migration `migrations/20260601000019_location.sql`
- `CREATE EXTENSION IF NOT EXISTS h3; CREATE EXTENSION IF NOT EXISTS h3_postgis;`
- **MGRS function** `oikumenea.location_mgrs(geography) RETURNS text` (plpgsql): use PostGIS
  `ST_Transform` into the point's UTM EPSG for easting/northing, then compute zone/latitude-band/
  100 km-square letters + truncated easting/northing digits. (Projection math in PostGIS, lettering in plpgsql.)
- **`location_location_types`** catalog (`id` RID, `code`, translatable `name`, `status`,
  timestamps, soft-delete); seed a few (`building`, `address`, `online`).
- **`location_locations`**: `id` RID PK `DEFAULT new_id(12,1,3)` + `rid_shape` CHECK (service 12 / kind 1 /
  type 3); `geom GEOGRAPHY(POINT,4326) NOT NULL`; derived `mgrs TEXT`, `h3_res_5/7/9/11 TEXT`;
  `country_id uuid NOT NULL REFERENCES geo_countries(id) ON DELETE RESTRICT`; address columns
  (`admin_area_1/2`, `locality`, `street`, `house_number`, `postal_code`, `raw_address`);
  `type_id` → `location_location_types`; timestamps + soft-delete; `set_updated_at` trigger.
- **Derive-trigger** `BEFORE INSERT OR UPDATE OF geom`: set `mgrs = location_mgrs(geom)` and
  `h3_res_N = h3_lat_lng_to_cell(geom::geometry, N)::text` for N∈{5,7,9,11} — so derived columns are
  authoritative-from-geometry and cannot drift (D-Location invariant).
- Spatial **GIST** index on `geom`; btree on each h3 column.
- RID type-row `INSERT`s (the three rows from "RID gap" above). PII comments per `pkg`/conventions.
- Bump `schema_version` expected revision (`internal/platform/db`).

### 3. Conjure `api/location.conjure.yml` (new) → `LocationService`
- Objects: `Location` (id, lat, lng, mgrs, h3 cells, country RID, address parts, type, `locale→text`
  type name), `LocationType`, list wrappers. RID/geo fields typed `string` (the `Rid`-alias trap).
- Endpoints per `docs/modules/location.md`: `POST /locations`, `GET /locations/{id}`,
  `PUT /locations/{id}`, `DELETE /locations/{id}`, `GET /locations?near={lat,lng}&radiusM=`,
  `GET /locations?bbox=`, `GET /location/types`. Perms `location.create|read|update`,
  `location.types.manage`. Generated Go lands in `internal/conjure/oikumenea/location`.
- Register the new `*.conjure.yml` in the gödel conjure plugin config; run codegen.

### 4. `internal/geo` module — add the location vertical (mirror `rank`)
- **domain**: add `Location` + `LocationType` structs and extend the `Repository` port with the new
  read/write methods (create/get/update/soft-delete/near/bbox/list-types).
- **adapters/queries/location.sql** (sqlc): `InsertLocation` (geom from lat/lng via
  `ST_SetSRID(ST_MakePoint(@lng,@lat),4326)::geography`; mgrs/h3 left to the trigger), `GetLocation`
  (project `ST_Y/ST_X(geom::geometry)` + mgrs/h3/address), `UpdateLocation`, `SoftDeleteLocation`
  (RESTRICT-guarded), `ListLocationsNear` (`ST_DWithin(geom, …, @radius_m)`), `ListLocationsInBbox`,
  `ListLocationTypes`. Extend `adapters/repository.go` + regenerate `geosql`.
- **application/service.go**: add audited-in-txn `CreateLocation/UpdateLocation/DeleteLocation`
  (each `Record`s an Action in the same txn — D-Audit), pool reads for `GetLocation/ListNear/ListBbox/
  ListTypes`. Inject `audit` like `rank`.
- **transport/service.go**: implement the generated `LocationService` interface; PEP-gate each op via
  the `enforcer`; assemble type-name `locale→text` maps via `localization` (`NamesByID`); map
  coordinate-missing → `Location:CoordinateRequired` (`pkg/errors`).
- **module.go**: extend `Register(info, pool, audit, loc, enforcer)`; register both `GeoService`
  (unchanged) and `LocationService` routes. Update the call site `cmd/oikumenea/main.go:202`.

### 5. Readiness gate — extension check
- Extend `internal/platform/health/readiness.go` `Status()` to also verify `postgis`, `h3`,
  `h3_postgis` are present (`SELECT … FROM pg_extension`), refusing readiness with a clear reason if
  not — surfacing the new operator-DB prerequisite (D-Location).

### 6. Web console UI (`web/`)
- Ontology-registry entry for **`Location`** in `web/src/lib/ontology/registry.ts` (→ explorer + ⌘K +
  `/o/[rid]` object view), reusing `components/ontology/*` (PropertyList, DataTable, ObjectHeader).
- Location browser/list page + **create-from-coordinate** form (lat/lng + address + type; shows derived
  MGRS/H3 read-back) + a **radius search** surface, following the M18 person/unit language UI patterns
  (`web/src/app/(dashboard)/…`). Country picker reuses the existing `GET /geo/countries` source.

### 7. Docs / bookkeeping (same pass)
- Flip the `docs/milestones.md` **stage board** M19 row Backend/Migrated/UI/Verified ✅ with real
  artifact references; update the M19 prose status to *verified*.
- `docs/ontology-mapping.md`: register the `Location`/`LocationType` Objects + `Create/Update/DeleteLocation`
  Actions. Note h3-pg as an operator prerequisite in `docs/modules/platform.md`.
- Run the docs link-coherence check from `CLAUDE.md`.

## Verification (end-to-end)

1. **Build the image + migrate.** `docker compose build` the custom Postgres image; `atlas migrate`
   applies 0019 cleanly and **re-applies idempotently**; `atlas migrate hash`/`lint` clean.
2. **Integration tests** (`internal/geo`, dedicated test DB via `scripts/setup-test-db.sh`):
   - create a location from a coordinate → MGRS + all four H3 cells are derived (non-null) on write;
   - update the coordinate → derived columns recompute;
   - create with only `country_code`/no coordinate → `Location:CoordinateRequired`;
   - `ListLocationsNear` returns rows within radius and excludes those outside (`ST_DWithin`);
   - soft-delete of a referenced location is RESTRICT-blocked;
   - each write emits exactly one audited Action (`system`/actor) in the same txn.
3. **Readiness gate:** with h3-pg present `/status/readiness` is green; simulate a missing extension →
   503 with the extension reason.
4. **Live demo:** `docker compose up`, create a Kyiv location via `POST /location/v1/locations`, read
   it back (MGRS + H3 populated), run a radius query around it, and confirm it appears in the web
   console location browser + `/o/[rid]` object view.
5. **Docs coherence:** the `CLAUDE.md` link-check prints `links OK`; stage board row is ✅ across
   Backend/Migrated/UI/Verified.

## Risks / watch-items
- **h3-pg build** is the main ops risk (compiling against the PostGIS image); pin the h3-pg release.
- **MGRS plpgsql** correctness — validate the function against a few known lat/lng→MGRS fixtures
  (incl. a UTM-zone boundary and a southern-hemisphere point) in the integration suite.
- Editing only the **new** migration (0019) — do **not** touch the shipped bootstrap; the action-type
  and object-type RID rows are added by 0019's `INSERT`s, not by editing line 137–166.
