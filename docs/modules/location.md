# Module: location

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.location_*`

## Purpose

Owns one **shared, standalone place entity** that anything with a location references by FK (D-Location).
A location is a **precise point on Earth** plus a structured postal address: a required
`GEOGRAPHY(POINT, 4326)` coordinate, **DB-derived** MGRS string and **H3 indexes** (computed by the
PostGIS + `h3-pg` extensions), and normalized address parts over the seeded country registry. It is
**purely geographic** — it carries **no owner, no visibility, no purpose**: a referencing module
(e.g. [religion](religion.md) sites, education buildings, company addresses) owns the *meaning* of a
location (which unit, how public, what kind) on its own link, so one shared row can be referenced by
several owners. This re-adopts the geography/PostGIS/H3 stack explicitly dropped from `drafts/`
(D-Location), because the analytics scope here needs queryable places.

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Location`,
`LocationType` (a small instance-admin catalog of place purposes, optional on a location).
**Actions:** `CreateLocation`/`UpdateLocation`/`DeleteLocation` — each audited, `action__<type>` RID.

- **Location** (aggregate root) — a coordinate + derived spatial indexes + structured address. No
  `code`/`name` (a place is identified by its geometry/address, not a locale-agnostic code); soft-delete.
- **Location type** — optional catalog label (`code`/translatable `name`) classifying a place
  (e.g. `building`, `address`, `online`); descriptive only, never branched on.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete) per
[conventions.md](../architecture/conventions.md).

**`location_locations`**
- `id` PK — RID, `location` entity-type slot
- `geom GEOGRAPHY(POINT, 4326) NOT NULL` — the authoritative coordinate (**PostGIS**); the required
  spine (address-only records are out of scope — geocode first, D-Location). `pii:none` at rest (a
  coordinate becomes locator data only when an owner links a person to it — that tier lives on the
  owning link).
- `mgrs TEXT` — **DB-derived** MGRS string (trigger/function from `geom`); never hand-set
- `h3_res_5 TEXT`, `h3_res_7 TEXT`, `h3_res_9 TEXT`, `h3_res_11 TEXT` — **DB-derived** H3 cells at a
  fixed set of resolutions (≈9 km / 1.2 km / 150 m / 20 m), via `h3-pg`; recomputed on `geom` change.
  Stored **in full regardless of any owner's publish precision** — coarsening is a read-time projection
  on the owning link, not a stored loss (see [religion](religion.md) `public_precision`).
- `country_code TEXT NOT NULL REFERENCES geo_countries(code) ON DELETE RESTRICT` — ISO-3166-1 α2
  (D-Geo)
- `admin_area_1 TEXT`, `admin_area_2 TEXT` — state/oblast, county/raion
- `locality TEXT`, `street TEXT`, `house_number TEXT`, `postal_code TEXT`
- `raw_address TEXT` — the unparsed address as supplied
- `type_id TEXT REFERENCES location_location_types(id) ON DELETE RESTRICT` — optional classification
- `created_at`, `updated_at`, `deleted_at`
- Spatial **GIST index** on `geom`; btree indexes on the H3 columns for cell-lookup search.

**`location_location_types`** (instance-admin catalog)
- `id` PK, `code TEXT NOT NULL` (stable, unique among active), `name TEXT NOT NULL` (translatable),
  `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`,
  `created_at`, `updated_at`, `deleted_at`.

## Conjure API surface

`LocationService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /locations` | Create a location from a coordinate (+ address); derives MGRS/H3 | `location.create` |
| `GET /locations/{id}` | Read one location | `location.read` |
| `PUT /locations/{id}` | Update coordinate/address/type (re-derives MGRS/H3 on coord change) | `location.update` |
| `DELETE /locations/{id}` | Soft-delete (blocked if referenced) | `location.update` |
| `GET /locations?near={lat,lng}&radiusM={m}` | Radius query (`ST_DWithin`), token-paginated | `location.read` |
| `GET /locations?bbox={…}` | Viewport / bounding-box query | `location.read` |
| `GET /location/types` | List the type catalog | `location.read` |

Creating a location with only `country_code` (no coordinate) → `Location:CoordinateRequired`.
Translatable type `name` returns as a `locale → text` map.

## Dependencies

- **Calls:** [platform](platform.md) (the PostGIS + `h3-pg` extension bootstrap + readiness check; DB
  pool; config), [localization](localization.md) (assemble type-name locale-maps).
- **Called by:** [religion](religion.md) (sites), and the planned `education` (buildings/dormitories)
  and `company` (addresses) modules — each references `location_locations(id)` by FK and owns
  visibility/precision/purpose on its own link. [audit](audit.md).

## Authorization touchpoints

Defines/gates `location.create`, `location.read`, `location.update`, and `location.read` for the type
catalog. A location row has **no unit scope of its own** — access scoping is the *owning link's* job
(e.g. a religion site inherits its unit's shadow visibility; the location itself is a neutral place).
Type-catalog writes are instance-scope (`location.types.manage`).

## Invariants & safety

- **Coordinate required.** Every location has a non-null `geom`; address-only records are rejected.
- **Derived columns are authoritative-from-geometry.** `mgrs` and the H3 cells are computed by DB
  functions from `geom` and recomputed on change — never written by the application, so they cannot
  drift from the coordinate.
- **A location carries no owner / no visibility.** Meaning is on the referencing link; a shared
  location may be referenced by many owners at different precisions.
- **Referenced locations cannot be hard-deleted** (`ON DELETE RESTRICT` from owner links); soft-delete
  is reversible within the grace window.
- **Extension prerequisite.** The operator DB must carry **PostGIS + h3-pg**; the schema-bootstrap
  enables them and the readiness gate checks for them ([platform](platform.md), D-Location).

## Open seams / future

- **Address-only / geocoding pipeline** (accept an address, geocode to a coordinate) is out of scope —
  callers geocode first; an additive seam if a geocoder is wired in.
- **Additional H3 resolutions** or a different derived grid are additive (recompute on change).
- A location is **standalone**; it deliberately does not model routes, regions, or polygons — only
  points. Polygon/area geometry is a future additive seam if a real need appears.
