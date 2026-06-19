-- Location module queries (docs/modules/location.md; D-Location, M19). Full audited CRUD + spatial
-- queries over oikumenea.location_locations, plus the place-type catalog read. The geometry never
-- crosses to sqlc: inputs build the point with ST_SetSRID(ST_MakePoint(lng,lat),4326)::geography, and
-- outputs project the coordinate back as latitude/longitude via ST_Y/ST_X with an explicit
-- ::double precision cast (so sqlc types them as float64). mgrs is a plain text column the application
-- derives and writes; source_coordinate is jsonb (the original input as supplied).

-- name: InsertLocation :one
INSERT INTO oikumenea.location_locations (
  geom, mgrs, source_coordinate, country_id, admin_area_1, admin_area_2, locality, street,
  house_number, postal_code, raw_address, type_id
) VALUES (
  ST_SetSRID(ST_MakePoint(sqlc.arg(longitude)::double precision, sqlc.arg(latitude)::double precision), 4326)::geography,
  sqlc.narg(mgrs), sqlc.arg(source_coordinate), sqlc.arg(country_id), sqlc.narg(admin_area_1),
  sqlc.narg(admin_area_2), sqlc.narg(locality), sqlc.narg(street), sqlc.narg(house_number),
  sqlc.narg(postal_code), sqlc.narg(raw_address), sqlc.narg(type_id)
)
RETURNING id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at;

-- name: GetLocation :one
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UpdateLocation :one
-- Full replace: re-sets geom from the supplied coordinate, the app-derived mgrs, the original source
-- coordinate, the country, the address parts, and the type.
UPDATE oikumenea.location_locations
SET geom = ST_SetSRID(ST_MakePoint(sqlc.arg(longitude)::double precision, sqlc.arg(latitude)::double precision), 4326)::geography,
    mgrs = sqlc.narg(mgrs),
    source_coordinate = sqlc.arg(source_coordinate),
    country_id = sqlc.arg(country_id),
    admin_area_1 = sqlc.narg(admin_area_1),
    admin_area_2 = sqlc.narg(admin_area_2),
    locality = sqlc.narg(locality),
    street = sqlc.narg(street),
    house_number = sqlc.narg(house_number),
    postal_code = sqlc.narg(postal_code),
    raw_address = sqlc.narg(raw_address),
    type_id = sqlc.narg(type_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at;

-- name: SoftDeleteLocation :execrows
UPDATE oikumenea.location_locations
SET deleted_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListLocationsNear :many
-- Locations within radiusM metres of (lat,lng), nearest first (ST_DWithin on geography). Offset/limit
-- paginated by the application's opaque page token.
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND ST_DWithin(
        geom,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography,
        sqlc.arg(radius_m)::double precision)
ORDER BY geom <-> ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography, id
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: ListLocationsInBbox :many
-- Locations whose coordinate falls inside the bounding box, ordered by id for stable pagination.
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND ST_Intersects(
        geom,
        ST_MakeEnvelope(sqlc.arg(min_lng)::double precision, sqlc.arg(min_lat)::double precision,
                        sqlc.arg(max_lng)::double precision, sqlc.arg(max_lat)::double precision, 4326)::geography)
ORDER BY id
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: SearchLocationsByText :many
-- Case-insensitive text search over the address fields (no spatial window required), ordered by id for
-- stable pagination. Backs the typeahead picker — a location has no `code`, so the match runs over
-- locality, the admin areas, street, mgrs, and the raw address.
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND (locality ILIKE '%' || sqlc.arg(query)::text || '%'
       OR admin_area_1 ILIKE '%' || sqlc.arg(query)::text || '%'
       OR admin_area_2 ILIKE '%' || sqlc.arg(query)::text || '%'
       OR street ILIKE '%' || sqlc.arg(query)::text || '%'
       OR mgrs ILIKE '%' || sqlc.arg(query)::text || '%'
       OR raw_address ILIKE '%' || sqlc.arg(query)::text || '%')
ORDER BY id
LIMIT sqlc.arg(lim)::int OFFSET sqlc.arg(off)::int;

-- name: ListLocationTypes :many
SELECT id, code, name, status FROM oikumenea.location_location_types
WHERE deleted_at IS NULL AND status = 'active'
ORDER BY code;
