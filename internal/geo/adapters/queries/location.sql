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
-- Locations within radiusM metres of (lat,lng), nearest first (ST_DWithin on geography). Keyset-paginated
-- (review R-21: replaces the last offset-paginated query in the codebase, which re-scanned and shifted
-- rows under concurrent inserts). The sort key is the (distance, id) pair, so the cursor is that pair: the
-- distance_m column is returned for the application to carry into the next page token, and the row-value
-- predicate resumes strictly after the last row seen. Empty after_id starts at the nearest. Distance is
-- the EXACT ST_Distance (not the `<->` KNN operator) in every clause — mixing the index-approximated KNN
-- order with an exact keyset recompute would skip rows at the page boundary; the ST_DWithin radius bounds
-- the set, so ordering without the KNN index is fine.
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at,
  ST_Distance(geom, ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography)::double precision AS distance_m
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND ST_DWithin(
        geom,
        ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography,
        sqlc.arg(radius_m)::double precision)
  AND (sqlc.arg(after_id)::text = ''
       OR (ST_Distance(geom, ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography)::double precision,
            id::text) > (sqlc.arg(after_dist)::double precision, sqlc.arg(after_id)::text))
ORDER BY ST_Distance(geom, ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography), id
LIMIT sqlc.arg(lim)::int;

-- name: ListLocationsInBbox :many
-- Locations whose coordinate falls inside the bounding box, keyset-paginated by id (review R-21:
-- replaces offset pagination). Empty after starts at the beginning.
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
  AND (sqlc.arg(after)::text = '' OR id::text > sqlc.arg(after)::text)
ORDER BY id
LIMIT sqlc.arg(lim)::int;

-- name: SearchLocationsByText :many
-- Case-insensitive text search over the address fields (no spatial window required), keyset-paginated by
-- id (review R-21: replaces offset pagination). Backs the typeahead picker — a location has no `code`, so the match runs
-- over locality, the admin areas, street, mgrs, and the raw address, folded into the STORED search_text
-- haystack that the location_locations_search_trgm GIN index serves as a bitmap scan.
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND search_text ILIKE '%' || sqlc.arg(query)::text || '%'
  AND (sqlc.arg(after)::text = '' OR id::text > sqlc.arg(after)::text)
ORDER BY id
LIMIT sqlc.arg(lim)::int;

-- name: ListLocationTypes :many
SELECT id, code, name, status FROM oikumenea.location_location_types
WHERE deleted_at IS NULL AND status = 'active'
ORDER BY code;
