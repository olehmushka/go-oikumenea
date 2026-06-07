#!/usr/bin/env bash
# Data-safe upgrade test (upgrade-safety.md §3): apply migrations up to the PRIOR revision, seed
# representative rows, then migrate to HEAD and assert the seeded rows survive and the schema-version
# marker advances. Catches a destructive change that slipped past review/lint on a real upgrade path
# (not just a clean install).
#
# Requires: atlas on PATH, psql on PATH, and $DATABASE_URL pointing at an EMPTY operator DB.
set -euo pipefail

: "${DATABASE_URL:?set DATABASE_URL to an empty Postgres}"
psql() { command psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -tA "$@"; }

# Total number of migration files; apply all but the last to reach the prior revision.
total=$(ls migrations/*.sql | wc -l | tr -d ' ')
prior=$((total - 1))

echo "==> applying first ${prior} of ${total} migrations (prior revision)"
atlas migrate apply --env local "${prior}"

prior_rev=$(psql -c "SELECT revision FROM oikumenea.schema_version")
echo "    prior revision: ${prior_rev}"

echo "==> seeding representative rows at the prior revision"
# i18n_locales: a natural-key table present since an early revision and read by later code. (Tables
# with a new_rid() default id are skipped here — new_rid reads the per-connection app.environment GUC
# that a bare psql session does not set; row-preservation is table-agnostic regardless.)
psql -c "INSERT INTO oikumenea.i18n_locales (code, name, is_default)
         VALUES ('upg', 'Upgrade Test Locale', false)
         ON CONFLICT (code) DO NOTHING"
locales_before=$(psql -c "SELECT count(*) FROM oikumenea.i18n_locales")
geo_before=$(psql -c "SELECT count(*) FROM oikumenea.geo_countries")
echo "    locales=${locales_before} geo_countries=${geo_before}"

echo "==> applying remaining migrations to HEAD"
atlas migrate apply --env local

head_rev=$(psql -c "SELECT revision FROM oikumenea.schema_version")
locales_after=$(psql -c "SELECT count(*) FROM oikumenea.i18n_locales")
geo_after=$(psql -c "SELECT count(*) FROM oikumenea.geo_countries")
echo "    head revision: ${head_rev}  locales=${locales_after} geo_countries=${geo_after}"

fail=0
[ "${head_rev}" != "${prior_rev}" ] || { echo "::error::schema_version did not advance"; fail=1; }
[ "${locales_after}" -ge "${locales_before}" ] || { echo "::error::locale rows lost on upgrade"; fail=1; }
[ "${geo_after}" -ge "${geo_before}" ] || { echo "::error::geo_countries rows lost on upgrade"; fail=1; }
marker=$(psql -c "SELECT count(*) FROM oikumenea.i18n_locales WHERE code = 'upg'")
[ "${marker}" = "1" ] || { echo "::error::seeded marker row missing after upgrade"; fail=1; }

if [ "${fail}" -ne 0 ]; then
  echo "UPGRADE TEST FAILED"
  exit 1
fi
echo "UPGRADE TEST OK: ${prior_rev} -> ${head_rev}, rows preserved"
