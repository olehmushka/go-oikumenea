-- Geo module queries (docs/modules/location.md; D-Geo / D-GeoPlaces). A read-only lookup over the
-- RID-keyed country registry (oikumenea.geo_countries) so clients can resolve a country to its RID.
-- The registry is written by the hermenea import pipeline, not here.

-- name: ListCountries :many
-- Active countries in display order (sort_order, then code).
SELECT id, code, name, status FROM oikumenea.geo_countries
WHERE status = 'active'
ORDER BY sort_order, code;
