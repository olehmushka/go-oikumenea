-- geo-countries import upsert (M16 / D-Hermenea). Code-keyed, idempotent, non-destructive: the
-- handler reads the existing row, inserts when absent, updates only when the name changed, and leaves
-- the row untouched (skip) otherwise. Provenance (source/source_version/imported_at) is stamped on
-- every insert/update. Never deletes.

-- name: GetGeoCountryName :one
SELECT name FROM oikumenea.geo_countries WHERE code = $1;

-- name: InsertGeoCountryImport :exec
INSERT INTO oikumenea.geo_countries (code, name, source, source_version, imported_at)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdateGeoCountryImport :exec
UPDATE oikumenea.geo_countries
SET name = $2, source = $3, source_version = $4, imported_at = $5
WHERE code = $1;
