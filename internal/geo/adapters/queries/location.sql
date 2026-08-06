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
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id')::uuid)
  AND (sqlc.narg('type_id')::uuid IS NULL OR type_id = sqlc.narg('type_id')::uuid)
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
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id')::uuid)
  AND (sqlc.narg('type_id')::uuid IS NULL OR type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.arg(after)::text = '' OR id::text > sqlc.arg(after)::text)
ORDER BY id
LIMIT sqlc.arg(lim)::int;

-- name: ListLocations :many
-- The BROWSE mode (M58 ticket 6): the whole registry in RID order, keyset-paginated on id. Until this
-- query existed, `listLocations` had no unwindowed branch and returned Location:QueryWindowRequired
-- when given no radius, box or text — there was no way to ask for "the locations", which is why this
-- type had no list filters and no dashboard while every other M58 type did.
--
-- Kept as its own statement rather than folded into ListLocationsInBbox behind a nullable envelope:
-- `(narg IS NULL OR ST_Intersects(...))` is not GiST-indexable, so one query for both modes would
-- have paid for the browse mode by regressing the spatial one (the R-21 lesson about nullable
-- trigram predicates, in geography).
SELECT id,
  ST_Y(geom::geometry)::double precision AS latitude,
  ST_X(geom::geometry)::double precision AS longitude,
  mgrs, source_coordinate, country_id,
  admin_area_1, admin_area_2, locality, street, house_number, postal_code, raw_address,
  type_id, created_at, updated_at
FROM oikumenea.location_locations
WHERE deleted_at IS NULL
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id')::uuid)
  AND (sqlc.narg('type_id')::uuid IS NULL OR type_id = sqlc.narg('type_id')::uuid)
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
  AND (sqlc.narg('country_id')::uuid IS NULL OR country_id = sqlc.narg('country_id')::uuid)
  AND (sqlc.narg('type_id')::uuid IS NULL OR type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.arg(after)::text = '' OR id::text > sqlc.arg(after)::text)
ORDER BY id
LIMIT sqlc.arg(lim)::int;

-- ============================ location dashboards (M58 ticket 6 / D-ObjectFacets) ============================
-- FOUR arms — and the axis is the MODE, not visibility. A location carries no owner, no unit and no
-- public/shadow bit (D-Location: a referencing module owns the *meaning* of a place on its own link),
-- so `location.read` held anywhere is the whole gate and there is no second visibility arm for a
-- decision to be made in. That is languoid's shape, the ABSENCE of a decision, and NOT the audit
-- ledger's, which is a decision made entirely by which connection the query runs on; the two are not
-- interchangeable and this comment is the claim pkg/facet/statsparity_test.go reads.
--
-- What IS four-way here is the window, because `listLocations` has four modes and each is a different
-- PLAN: a trigram bitmap scan (search), a GiST radius (near), a GiST envelope (bbox), and a plain
-- keyset scan (browse). Each list query therefore has exactly one aggregate twin carrying the same
-- filter block, which is what makes "the chart describes the list" structural rather than asserted —
-- a nullable spatial predicate would have collapsed the four into one query and lost the index in
-- three of them.
--
-- Mode PRECEDENCE (query beats radius beats bbox) is not written here and must not be: it is resolved
-- ONCE, in the transport's shared filter builder, and both surfaces are handed the answer. Writing it
-- twice is exactly how a chart and a list come to read one URL differently.
--
-- The aggregate half below is byte-identical across all four arms (statsparity_test.go), or an
-- unwindowed dashboard and a windowed one would bucket the same world differently.

-- name: LocationStats :many
-- The BROWSE arm: no spatial predicate at all, which is why it is a separate query rather than one of
-- the others with a disabled window.
WITH cand AS MATERIALIZED (
  SELECT l.country_id, l.type_id
  FROM oikumenea.location_locations l
  WHERE l.deleted_at IS NULL
    AND (sqlc.narg('country_id')::uuid IS NULL OR l.country_id = sqlc.narg('country_id')::uuid)
    AND (sqlc.narg('type_id')::uuid IS NULL OR l.type_id = sqlc.narg('type_id')::uuid)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'countryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: LocationStatsNear :many
-- The RADIUS arm. Carries ListLocationsNear's ST_DWithin verbatim; the distance is not projected,
-- because a dashboard counts a set and does not order it.
WITH cand AS MATERIALIZED (
  SELECT l.country_id, l.type_id
  FROM oikumenea.location_locations l
  WHERE l.deleted_at IS NULL
    AND ST_DWithin(
          l.geom,
          ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::double precision, sqlc.arg(lat)::double precision), 4326)::geography,
          sqlc.arg(radius_m)::double precision)
    AND (sqlc.narg('country_id')::uuid IS NULL OR l.country_id = sqlc.narg('country_id')::uuid)
    AND (sqlc.narg('type_id')::uuid IS NULL OR l.type_id = sqlc.narg('type_id')::uuid)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'countryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: LocationStatsInBbox :many
-- The BOUNDING-BOX arm. Carries ListLocationsInBbox's ST_Intersects envelope verbatim.
WITH cand AS MATERIALIZED (
  SELECT l.country_id, l.type_id
  FROM oikumenea.location_locations l
  WHERE l.deleted_at IS NULL
    AND ST_Intersects(
          l.geom,
          ST_MakeEnvelope(sqlc.arg(min_lng)::double precision, sqlc.arg(min_lat)::double precision,
                          sqlc.arg(max_lng)::double precision, sqlc.arg(max_lat)::double precision, 4326)::geography)
    AND (sqlc.narg('country_id')::uuid IS NULL OR l.country_id = sqlc.narg('country_id')::uuid)
    AND (sqlc.narg('type_id')::uuid IS NULL OR l.type_id = sqlc.narg('type_id')::uuid)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'countryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: LocationStatsSearch :many
-- The TEXT arm. Carries SearchLocationsByText's ILIKE over the STORED search_text haystack, so a
-- searched list and its dashboard read one index.
WITH cand AS MATERIALIZED (
  SELECT l.country_id, l.type_id
  FROM oikumenea.location_locations l
  WHERE l.deleted_at IS NULL
    AND l.search_text ILIKE '%' || sqlc.arg(query)::text || '%'
    AND (sqlc.narg('country_id')::uuid IS NULL OR l.country_id = sqlc.narg('country_id')::uuid)
    AND (sqlc.narg('type_id')::uuid IS NULL OR l.type_id = sqlc.narg('type_id')::uuid)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n
FROM cand
UNION ALL
SELECT 'countryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.country_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2;

-- name: ListLocationTypes :many
SELECT id, code, name, status FROM oikumenea.location_location_types
WHERE deleted_at IS NULL AND status = 'active'
ORDER BY code;
