-- geo-countries import upsert (M16 / D-Hermenea). Code-keyed, idempotent, non-destructive: the
-- handler reads the existing row, inserts when absent, updates only when the name changed, and leaves
-- the row untouched (skip) otherwise. Provenance (source/source_version/imported_at) is stamped on
-- every insert/update. Never deletes.

-- name: GetGeoCountryName :one
SELECT name FROM oikumenea.geo_countries WHERE code = $1;

-- name: InsertGeoCountryImport :exec
INSERT INTO oikumenea.geo_countries (code, name, source, source_version, imported_at, origin)
VALUES ($1, $2, $3, $4, $5, 'seeded');  -- import-path rows are pinax seeded-owned (D-Pinax, M45)

-- name: UpdateGeoCountryImport :exec
UPDATE oikumenea.geo_countries
SET name = $2, source = $3, source_version = $4, imported_at = $5
WHERE code = $1;

-- name: EnrichGeoCountryFillEmpty :exec
-- Pinax country enrichment (D-Pinax, M45): fill-if-empty only — every column is set via COALESCE(col,
-- new) so a value already present (from the migration skeleton or the WOF geo-places connector) is
-- NEVER overwritten, only a currently-NULL column is filled. Safe to run on boot autoseed and on
-- --reconcile alike. `geom` is a low-res country border (GeoJSON Polygon/MultiPolygon) materialized via
-- ST_GeomFromGeoJSON; centroid (ST_PointOnSurface) + bbox (ST_Envelope) are derived from it. The WOF
-- geo-places connector later replaces `geom` with the high-res shape (its EnrichCountry is an
-- unconditional UPDATE). color_id resolves the domain='country' palette code (unresolved = untouched).
UPDATE oikumenea.geo_countries SET
  iso_a3       = COALESCE(iso_a3, NULLIF(sqlc.arg(iso_a3)::text, '')),
  numeric_code = COALESCE(numeric_code, NULLIF(sqlc.arg(numeric_code)::text, '')),
  geom         = COALESCE(geom, gj.g),
  -- centroid: prefer a point derived from the border polygon; fall back to the representative lat/lng
  -- point (so a country with no bundled polygon — a small nation — is still locatable).
  centroid     = COALESCE(centroid,
                   CASE WHEN gj.g IS NOT NULL THEN ST_PointOnSurface(gj.g) END,
                   CASE WHEN sqlc.narg(latitude)::float8 IS NOT NULL AND sqlc.narg(longitude)::float8 IS NOT NULL
                        THEN ST_SetSRID(ST_MakePoint(sqlc.narg(longitude)::float8, sqlc.narg(latitude)::float8), 4326)
                        END),
  bbox         = COALESCE(bbox, CASE WHEN gj.g IS NOT NULL THEN ST_Envelope(gj.g) END),
  color_id     = COALESCE(color_id,
                   (SELECT id FROM oikumenea.platform_colors
                    WHERE domain = 'country' AND code = NULLIF(sqlc.arg(color_code)::text, '')
                      AND deleted_at IS NULL))
FROM (SELECT CASE WHEN NULLIF(sqlc.arg(geometry)::text, '') IS NULL THEN NULL
                  ELSE ST_SetSRID(ST_GeomFromGeoJSON(sqlc.arg(geometry)::text), 4326) END AS g) AS gj
WHERE code = sqlc.arg(code)::text;
