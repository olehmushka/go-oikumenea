-- geo-places import upsert (D-GeoPlaces, M16/hermenea). The Who's-On-First administrative gazetteer
-- (country/region/county/locality). Idempotency is keyed on source_version (GetGeoPlaceVersion): a
-- re-import with the same edition skips, a newer one updates, an absent row inserts — never deletes.
-- Geometry crosses the wire as GeoJSON text and is materialized with ST_GeomFromGeoJSON (so sqlc never
-- sees the geometry type); centroid + bbox are DB-derived (ST_PointOnSurface / ST_Envelope). Absent
-- optional values arrive as 0 / '' and are folded to NULL via NULLIF (parent_id/population can never be
-- a real 0; country_code never a real ''). A placetype=country record additionally enriches the
-- matching geo_countries row in place.

-- name: GetGeoPlaceVersion :one
SELECT source_version FROM oikumenea.geo_places WHERE wof_id = $1;

-- name: InsertGeoPlaceImport :exec
WITH g AS (
  SELECT CASE WHEN sqlc.arg(geometry)::text = '' THEN NULL
              ELSE ST_SetSRID(ST_GeomFromGeoJSON(sqlc.arg(geometry)::text), 4326) END AS geom
)
INSERT INTO oikumenea.geo_places (
  wof_id, placetype, parent_id, country_id, name, population,
  hierarchy, concordances, status, geom, centroid, bbox,
  source, source_version, imported_at)
SELECT sqlc.arg(wof_id)::bigint,
       sqlc.arg(placetype)::text,
       -- Resolve the parent's WOF id to its RID; a non-zero parentId that does not resolve falls back
       -- to a sentinel uuid so the geo_places(id) FK fails loudly (RESTRICT) rather than silently
       -- NULLing a real, but not-yet-loaded, parent reference.
       CASE WHEN NULLIF(sqlc.arg(parent_id)::bigint, 0) IS NULL THEN NULL
            ELSE COALESCE((SELECT p.id FROM oikumenea.geo_places p WHERE p.wof_id = sqlc.arg(parent_id)::bigint),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       CASE WHEN NULLIF(sqlc.arg(country_code)::text, '') IS NULL THEN NULL
            ELSE COALESCE((SELECT c.id FROM oikumenea.geo_countries c WHERE c.code = sqlc.arg(country_code)::text),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       sqlc.arg(name)::text,
       NULLIF(sqlc.arg(population)::bigint, 0),
       sqlc.arg(hierarchy)::jsonb,
       sqlc.arg(concordances)::jsonb,
       sqlc.arg(status)::text,
       g.geom, ST_PointOnSurface(g.geom), ST_Envelope(g.geom),
       sqlc.arg(source)::text,
       sqlc.arg(source_version)::text,
       sqlc.arg(imported_at)::timestamptz
FROM g;

-- name: UpdateGeoPlaceImport :exec
WITH g AS (
  SELECT CASE WHEN sqlc.arg(geometry)::text = '' THEN NULL
              ELSE ST_SetSRID(ST_GeomFromGeoJSON(sqlc.arg(geometry)::text), 4326) END AS geom
)
UPDATE oikumenea.geo_places SET
  placetype      = sqlc.arg(placetype)::text,
  parent_id      = CASE WHEN NULLIF(sqlc.arg(parent_id)::bigint, 0) IS NULL THEN NULL
                        ELSE COALESCE((SELECT p.id FROM oikumenea.geo_places p WHERE p.wof_id = sqlc.arg(parent_id)::bigint),
                                      '00000000-0000-0000-0000-000000000000'::uuid) END,
  country_id     = CASE WHEN NULLIF(sqlc.arg(country_code)::text, '') IS NULL THEN NULL
                        ELSE COALESCE((SELECT c.id FROM oikumenea.geo_countries c WHERE c.code = sqlc.arg(country_code)::text),
                                      '00000000-0000-0000-0000-000000000000'::uuid) END,
  name           = sqlc.arg(name)::text,
  population     = NULLIF(sqlc.arg(population)::bigint, 0),
  hierarchy      = sqlc.arg(hierarchy)::jsonb,
  concordances   = sqlc.arg(concordances)::jsonb,
  status         = sqlc.arg(status)::text,
  geom           = (SELECT geom FROM g),
  centroid       = (SELECT ST_PointOnSurface(geom) FROM g),
  bbox           = (SELECT ST_Envelope(geom) FROM g),
  source         = sqlc.arg(source)::text,
  source_version = sqlc.arg(source_version)::text,
  imported_at    = sqlc.arg(imported_at)::timestamptz
WHERE wof_id = sqlc.arg(wof_id)::bigint;

-- name: EnrichGeoCountryFromWOF :exec
-- Mirror a country place's wof_id + geometry onto its ISO-keyed geo_countries row (D-GeoPlaces). Only
-- the WOF-derived columns are touched; name/status/source provenance stay owned by the geo-countries
-- importer (D-Geo). iso_a3 / numeric_code are set only when supplied.
WITH g AS (
  SELECT CASE WHEN sqlc.arg(geometry)::text = '' THEN NULL
              ELSE ST_SetSRID(ST_GeomFromGeoJSON(sqlc.arg(geometry)::text), 4326) END AS geom
)
UPDATE oikumenea.geo_countries SET
  wof_id       = sqlc.arg(wof_id)::bigint,
  iso_a3       = NULLIF(sqlc.arg(iso_a3)::text, ''),
  numeric_code = NULLIF(sqlc.arg(numeric_code)::text, ''),
  geom         = (SELECT geom FROM g),
  centroid     = (SELECT ST_PointOnSurface(geom) FROM g),
  bbox         = (SELECT ST_Envelope(geom) FROM g)
WHERE code = sqlc.arg(code)::text;
