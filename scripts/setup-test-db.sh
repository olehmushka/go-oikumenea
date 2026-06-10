#!/usr/bin/env bash
# Create and migrate the dedicated integration-test database, kept SEPARATE from the dev/operator DB
# so `go test -tags=integration` never pollutes the data the running server (and the web console)
# reads. The dev DB stays on `postgres` ($DATABASE_URL); the tests run against `oikumenea_test`
# ($OIKUMENEA_TEST_DSN) on the same Postgres instance.
#
# Why a separate DB matters: the first-admin bootstrap seed is idempotent — it skips once ANY active
# instance admin exists (D-Bootstrap). When the tests seeded their admins into the dev DB, the seed
# skipped at boot and the real Keycloak admin identity was never created, 401-ing the whole console.
#
# Idempotent: safe to re-run. Pass --reset to drop and recreate the test DB from scratch.
#
# Requires: psql + atlas on PATH, and a running Postgres (docker-compose.dev.yml). Reads .env for the
# admin/superuser DSN ($DATABASE_URL) and the test DSN ($OIKUMENEA_TEST_DSN); falls back to the
# local-dev defaults when unset.
set -euo pipefail
cd "$(dirname "$0")/.."

# Source .env if present so we honor a custom host port / credentials.
if [[ -f .env ]]; then set -a; . ./.env; set +a; fi

ADMIN_DSN="${DATABASE_URL:-postgres://postgres:dev@localhost:5432/postgres?sslmode=disable}"
TEST_DSN="${OIKUMENEA_TEST_DSN:-postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable}"

# Derive the test database NAME from the test DSN's path (strip leading / and any ?query).
TEST_DB="${TEST_DSN##*/}"; TEST_DB="${TEST_DB%%\?*}"
if [[ -z "$TEST_DB" || "$TEST_DB" == "postgres" ]]; then
  echo "refusing to use '$TEST_DB' as the test database — set OIKUMENEA_TEST_DSN to a dedicated DB (e.g. .../oikumenea_test)" >&2
  exit 1
fi

# admin() runs psql against the maintenance (postgres) DB as the owner/superuser, for CREATE DATABASE
# and role bootstrap (neither can run inside the target DB / a transaction block).
admin() { psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -tA "$@"; }

if [[ "${1:-}" == "--reset" ]]; then
  echo "==> dropping database $TEST_DB"
  admin -c "DROP DATABASE IF EXISTS \"$TEST_DB\" WITH (FORCE);"
fi

# CREATE DATABASE is not IF-NOT-EXISTS in Postgres; gate on the catalog.
if [[ "$(admin -c "SELECT 1 FROM pg_database WHERE datname = '$TEST_DB';")" != "1" ]]; then
  echo "==> creating database $TEST_DB"
  admin -c "CREATE DATABASE \"$TEST_DB\";"
else
  echo "==> database $TEST_DB already exists"
fi

# The non-superuser app login role the RLS-backstop test logs in as (rls_integration_test.go derives
# it from the superuser DSN as oikumenea/dev). Roles are CLUSTER-global, so this also covers the dev
# DB; create it once if missing. oikumenea_app (NOLOGIN parent) is created idempotently by migration
# 0011, so it may not exist yet on a brand-new cluster — make it first.
echo "==> ensuring roles oikumenea_app + oikumenea"
admin -c "DO \$\$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'oikumenea_app') THEN
    CREATE ROLE oikumenea_app NOLOGIN NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'oikumenea') THEN
    CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;
  END IF;
END \$\$;"

# Apply all migrations to the test DB (creates the oikumenea schema + RLS grants/policies in THIS DB;
# the GRANTs to oikumenea_app are per-database).
echo "==> applying migrations to $TEST_DB"
DATABASE_URL="$TEST_DSN" atlas migrate apply --env local

echo "==> test database ready: $TEST_DSN"
echo "    run: set -a; . ./.env; set +a; go test -tags=integration ./internal/..."
