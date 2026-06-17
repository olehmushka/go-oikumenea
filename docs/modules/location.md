# Module: location

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.location_*`

## Purpose

Owns one **shared, standalone place entity** that anything with a location references by FK (D-Location).
A location is a **precise point on Earth** plus a structured postal address: a required
`GEOGRAPHY(POINT, 4326)` coordinate, an **app-derived** MGRS string (pure Go), the original input
coordinate preserved in a `source_coordinate` JSONB column, and normalized address parts over the seeded
country registry. The coordinate may be **supplied in several formats** — WGS84 lat/lon, MGRS, UTM, or
СК-42 (Gauss-Krüger, numeric + grid) — and the application converts each to canonical WGS84 (a pluggable
converter registry, `internal/geo/domain/coordinate.go`), derives the MGRS, and records the original
input. It is **purely geographic** — it carries **no owner, no visibility, no purpose**: a referencing
module (e.g. [religion](religion.md) sites, education buildings, company addresses) owns the *meaning* of
a location (which unit, how public, what kind) on its own link, so one shared row can be referenced by
several owners. This re-adopts the geography/PostGIS stack explicitly dropped from `drafts/` (D-Location),
because the analytics scope here needs queryable places. **PostGIS itself is already enabled in the
bootstrap migration as of M16** (D-GeoPlaces pulled it forward for the WOF `geo_places` gazetteer); M19
reuses that extension and adds the `location_locations` point model — **PostGIS is the only spatial
prerequisite** (radius/bbox use `ST_DWithin` on the GiST index; H3 is not used — see the D-Location
amendment).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Location`,
`LocationType` (a small instance-admin catalog of place purposes, optional on a location).
**Actions:** `CreateLocation`/`UpdateLocation`/`DeleteLocation` — each audited, `action__<type>` RID.

- **Location** (aggregate root) — a coordinate + app-derived MGRS + the original input coordinate +
  structured address. No `code`/`name` (a place is identified by its geometry/address, not a
  locale-agnostic code); soft-delete.
- **Location type** — optional catalog label (`code`/translatable `name`) classifying a place
  (e.g. `building`, `address`, `online`); descriptive only, never branched on.

**Geo registry (M16, delivered ahead of the M19 Location model).** The location service (RID service
code **12**) also owns the shared **geo registry** — `geo_countries` (`Country`) and `geo_places`
(`GeoPlace`, the WOF gazetteer; D-GeoPlaces). Both are now **RID-keyed** (F-014): `geo_countries.id`
/ `geo_places.id` are the reference keys, with ISO `code` and `wof_id` retained as `UNIQUE` lookup /
concordance keys. Every country FK across person/document/rank/contact references `geo_countries(id)`,
and the country **RID flows end-to-end** (domain → Conjure → web); the ISO code is lookup-only. A
read-only **`GeoService`** (`GET /geo/countries`, permission `country.read`) returns `{id, code, name,
status}` so clients resolve a code to its RID and populate country pickers. The registry is written by
the hermenea import pipeline (geo-countries / WOF), which streams natural keys and resolves
`wof_id`/`code → id` in SQL on upsert.

## Data model

Conventions (URN RID PKs (D-ResourceIdentifiers), `TIMESTAMPTZ`, `set_updated_at`, soft-delete) per
[conventions.md](../architecture/conventions.md).

**`location_locations`**
- `id` PK — RID, `location` entity-type slot
- `geom GEOGRAPHY(POINT, 4326) NOT NULL` — the authoritative coordinate (**PostGIS**); the required
  spine (address-only records are out of scope — geocode first, D-Location). `pii:none` at rest (a
  coordinate becomes locator data only when an owner links a person to it — that tier lives on the
  owning link).
- `mgrs TEXT` — **app-derived** MGRS string (pure Go, from the resolved WGS84 coordinate; written on
  every coordinate change, nullable for polar UPS points); never client-supplied
- `source_coordinate JSONB NOT NULL DEFAULT '{}'` — the coordinate **as originally supplied** (its
  `format` + raw values), preserved verbatim so a point entered as MGRS/UTM/СК-42 round-trips back in
  the API response alongside the canonical `geom`
- `country_id uuid NOT NULL REFERENCES geo_countries(id) ON DELETE RESTRICT` — the country's RID
  (countries are RID-keyed, F-014; resolve an ISO α2 code via `GET /geo/countries`, D-Geo)
- `admin_area_1 TEXT`, `admin_area_2 TEXT` — state/oblast, county/raion
- `locality TEXT`, `street TEXT`, `house_number TEXT`, `postal_code TEXT`
- `raw_address TEXT` — the unparsed address as supplied
- `type_id TEXT REFERENCES location_location_types(id) ON DELETE RESTRICT` — optional classification
- `created_at`, `updated_at`, `deleted_at`
- Spatial **GIST index** on `geom` (serves the `ST_DWithin` radius + bbox queries); a partial btree on
  `country_id`.

**`location_location_types`** (instance-admin catalog)
- `id` PK, `code TEXT NOT NULL` (stable, unique among active), `name TEXT NOT NULL` (translatable),
  `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','retired'))`,
  `created_at`, `updated_at`, `deleted_at`.

## Conjure API surface

`LocationService`:

| Op | Intent | Perm |
|---|---|---|
| `POST /locations` | Create a location from a coordinate (any supported format) + address; derives MGRS | `location.create` |
| `GET /locations/{id}` | Read one location | `location.read` |
| `PUT /locations/{id}` | Update coordinate/address/type (re-derives MGRS on coord change) | `location.update` |
| `DELETE /locations/{id}` | Soft-delete (blocked if referenced) | `location.update` |
| `GET /locations?near={lat,lng}&radiusM={m}` | Radius query (`ST_DWithin`), token-paginated | `location.read` |
| `GET /locations?bbox={…}` | Viewport / bounding-box query | `location.read` |
| `GET /location/types` | List the type catalog | `location.read` |

The create/update payload carries a `coordinate: CoordinateInput` — a `format` discriminator
(`latlon`|`mgrs`|`utm`|`sk42`|`sk42grid`) plus the fields that format needs. A missing coordinate →
`Location:CoordinateRequired`; one that cannot be parsed/converted or resolves off-Earth →
`Location:CoordinateInvalid`. The response echoes the canonical `latitude`/`longitude` + `mgrs` and the
original `sourceCoordinate`. Translatable type `name` returns as a `locale → text` map.

## Dependencies

- **Calls:** [platform](platform.md) (the PostGIS extension bootstrap + readiness check; DB pool;
  config), [localization](localization.md) (assemble type-name locale-maps).
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
- **MGRS is derived, never client-supplied.** The application resolves the supplied coordinate to WGS84,
  derives `mgrs` from it, and writes both — the client cannot set `mgrs`, so it cannot drift from the
  coordinate. `geom` is built in SQL from the same resolved lat/lon.
- **The source coordinate is preserved.** `source_coordinate` records the input exactly as supplied, so
  the canonical point and its original representation are both retained.
- **A location carries no owner / no visibility.** Meaning is on the referencing link; a shared
  location may be referenced by many owners at different precisions.
- **Referenced locations cannot be hard-deleted** (`ON DELETE RESTRICT` from owner links); soft-delete
  is reversible within the grace window.
- **Extension prerequisite.** The operator DB must carry **PostGIS** (stock postgis image); the
  schema-bootstrap enables it and the readiness gate checks for it ([platform](platform.md), D-Location).

## Open seams / future

- **Address-only / geocoding pipeline** (accept an address, geocode to a coordinate) is out of scope —
  callers geocode first; an additive seam if a geocoder is wired in.
- **More input formats** are additive — register another converter in `coordinate.go` (the СК-42
  grid-square form currently accepts a full numeric reference; map-sheet square nomenclature is a seam).
- A location is **standalone**; it deliberately does not model routes, regions, or polygons — only
  points. Polygon/area geometry is a future additive seam if a real need appears.
