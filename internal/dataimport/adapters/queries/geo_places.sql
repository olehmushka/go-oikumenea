-- geo-places import upsert (D-GeoPlaces, M16/hermenea; set-based per chunk since R-05). The
-- Who's-On-First administrative gazetteer (country/region/county/locality). Idempotency is keyed on
-- source_version: a re-import with the same edition skips, a newer one updates, an absent row
-- inserts — never deletes.
-- Geometry crosses the wire as GeoJSON text and is materialized with ST_GeomFromGeoJSON (so sqlc never
-- sees the geometry type); centroid + bbox are DB-derived (ST_PointOnSurface / ST_Envelope). Absent
-- optional values arrive as 0 / '' and are folded to NULL via NULLIF (parent_id/population can never be
-- a real 0; country_code never a real ''). A placetype=country record additionally enriches the
-- matching geo_countries row in place.

-- name: BulkUpsertGeoPlaces :many
-- Set-based chunk merge (R-05): one INSERT … SELECT over the chunk's parallel arrays replaces the
-- per-record loop. Insert absent wof_ids, update rows whose stored source_version differs from the
-- incoming edition, and leave the rest untouched (the conflict-update WHERE gate) — never deletes.
-- parent_id is deliberately NOT written here: subqueries in one INSERT cannot see sibling rows of the
-- same statement, so parent resolution is a second pass (BulkSetGeoPlaceParents) over the rows this
-- merge reports as created/updated — which also lets a parent arrive in the same chunk. RETURNING
-- (xmax = 0) distinguishes fresh inserts from conflict-updates; skipped rows return nothing.
WITH r AS (
  SELECT unnest(@wof_ids::bigint[])      AS wof_id,
         unnest(@placetypes::text[])     AS placetype,
         unnest(@country_codes::text[])  AS country_code,
         unnest(@names::text[])          AS name,
         unnest(@populations::bigint[])  AS population,
         unnest(@hierarchies::text[])    AS hierarchy,
         unnest(@concordances::text[])   AS concordance,
         unnest(@statuses::text[])       AS status,
         unnest(@geometries::text[])     AS geometry
)
INSERT INTO oikumenea.geo_places (
  wof_id, placetype, country_id, name, population,
  hierarchy, concordances, status, geom, centroid, bbox,
  source, source_version, imported_at)
SELECT r.wof_id,
       r.placetype,
       CASE WHEN NULLIF(r.country_code, '') IS NULL THEN NULL
            ELSE COALESCE((SELECT c.id FROM oikumenea.geo_countries c WHERE c.code = r.country_code),
                          '00000000-0000-0000-0000-000000000000'::uuid) END,
       r.name,
       NULLIF(r.population, 0),
       NULLIF(r.hierarchy, '')::jsonb,
       NULLIF(r.concordance, '')::jsonb,
       r.status,
       g.geom, ST_PointOnSurface(g.geom), ST_Envelope(g.geom),
       @source::text,
       @source_version::text,
       @imported_at::timestamptz
FROM r
CROSS JOIN LATERAL (
  SELECT CASE WHEN r.geometry = '' THEN NULL
              ELSE ST_SetSRID(ST_GeomFromGeoJSON(r.geometry), 4326) END AS geom) g
ON CONFLICT (wof_id) DO UPDATE SET
  placetype      = EXCLUDED.placetype,
  country_id     = EXCLUDED.country_id,
  name           = EXCLUDED.name,
  population     = EXCLUDED.population,
  hierarchy      = EXCLUDED.hierarchy,
  concordances   = EXCLUDED.concordances,
  status         = EXCLUDED.status,
  geom           = EXCLUDED.geom,
  centroid       = EXCLUDED.centroid,
  bbox           = EXCLUDED.bbox,
  source         = EXCLUDED.source,
  source_version = EXCLUDED.source_version,
  imported_at    = EXCLUDED.imported_at
WHERE oikumenea.geo_places.source_version IS DISTINCT FROM EXCLUDED.source_version
RETURNING wof_id, (xmax = 0) AS inserted;

-- name: BulkSetGeoPlaceParents :exec
-- Second pass of the chunk merge: resolve each touched row's parent WOF id to its RID. Runs after
-- BulkUpsertGeoPlaces in the same transaction, so a parent inserted by this very chunk resolves. A
-- non-zero parent that does not resolve falls back to the sentinel uuid so the geo_places(id) FK
-- fails loudly (RESTRICT) rather than silently NULLing a real, but not-yet-loaded, parent reference.
UPDATE oikumenea.geo_places g SET
  parent_id = CASE WHEN r.parent_wof_id = 0 THEN NULL
                   ELSE COALESCE((SELECT p.id FROM oikumenea.geo_places p WHERE p.wof_id = r.parent_wof_id),
                                 '00000000-0000-0000-0000-000000000000'::uuid) END
FROM (SELECT unnest(@wof_ids::bigint[])        AS wof_id,
             unnest(@parent_wof_ids::bigint[]) AS parent_wof_id) r
WHERE g.wof_id = r.wof_id;

-- name: BulkEnrichGeoCountriesFromWOF :exec
-- Mirror each created/updated country place's wof_id + geometry onto its ISO-keyed geo_countries row
-- (D-GeoPlaces). Only the WOF-derived columns are touched; name/status/source provenance stay owned
-- by the geo-countries importer (D-Geo). WOF UPGRADES the border: when it carries geometry, its
-- high-res shape OVERWRITES whatever was there (e.g. the pinax low-res bootstrap border, D-Pinax M45)
-- and re-derives centroid/bbox; when a WOF country record lacks geometry the existing value is KEPT
-- (COALESCE), so WOF never downgrades the pinax baseline to NULL. iso_a3 / numeric_code likewise
-- upgrade-or-keep.
UPDATE oikumenea.geo_countries gc SET
  wof_id       = r.wof_id,
  iso_a3       = COALESCE(NULLIF(r.iso_a3, ''), gc.iso_a3),
  numeric_code = COALESCE(NULLIF(r.numeric_code, ''), gc.numeric_code),
  geom         = COALESCE(g.geom, gc.geom),
  centroid     = COALESCE(ST_PointOnSurface(g.geom), gc.centroid),
  bbox         = COALESCE(ST_Envelope(g.geom), gc.bbox)
FROM (SELECT unnest(@wof_ids::bigint[])       AS wof_id,
             unnest(@codes::text[])           AS code,
             unnest(@iso_a3s::text[])         AS iso_a3,
             unnest(@numeric_codes::text[])   AS numeric_code,
             unnest(@geometries::text[])      AS geometry) r
CROSS JOIN LATERAL (
  SELECT CASE WHEN r.geometry = '' THEN NULL
              ELSE ST_SetSRID(ST_GeomFromGeoJSON(r.geometry), 4326) END AS geom) g
WHERE gc.code = r.code;




