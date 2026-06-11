#!/usr/bin/env bash
# Reset the DEV/operator database to a clean, freshly-migrated state with exactly one admin: drop the
# app data, re-apply every migration, then re-seed the single bootstrap admin (D-Bootstrap) so the API
# and web console are immediately usable again.
#
# This targets the dev DB ($DATABASE_URL — the `postgres` DB), NOT the integration-test DB
# (oikumenea_test, owned by scripts/setup-test-db.sh). Because the `postgres` maintenance DB can't be
# dropped while connected, the reset is a SCHEMA drop (oikumenea + atlas_schema_revisions), not a
# DATABASE drop — the documented "DROP SCHEMA reset" for getting back to a clean dev state (e.g. after
# editing a shipped migration).
#
# Idempotent: safe to re-run. The drops are IF EXISTS, the migrations re-apply from scratch (Atlas
# replays the full history since its revision table is dropped too), and bootstrap-admin seeds on the
# now-fresh schema.
#
# Requires: psql + atlas on PATH, a running Postgres (docker-compose.dev.yml), and either a built
# binary or ./godelw to build one. Reads .env for the operator DSN ($DATABASE_URL); falls back to the
# local-dev default when unset.
set -euo pipefail
cd "$(dirname "$0")/.."

# Source .env if present so we honor a custom host port / credentials.
if [[ -f .env ]]; then set -a; . ./.env; set +a; fi

ADMIN_DSN="${DATABASE_URL:-postgres://postgres:dev@localhost:5432/postgres?sslmode=disable}"

# Derive the target database NAME from the DSN (strip leading path + any ?query) and guard against
# pointing this at the integration-test DB — that one is reset by scripts/setup-test-db.sh --reset.
DEV_DB="${ADMIN_DSN##*/}"; DEV_DB="${DEV_DB%%\?*}"
if [[ "$DEV_DB" == "oikumenea_test" ]]; then
  echo "refusing to reset '$DEV_DB' — that is the integration-test DB; use ./scripts/setup-test-db.sh --reset" >&2
  exit 1
fi

admin() { psql "$ADMIN_DSN" -v ON_ERROR_STOP=1 -tA "$@"; }

# 1. Drop the app data. The cluster-global roles oikumenea_app / oikumenea are intentionally left
#    intact (schema drop doesn't touch roles; migration 0011 re-creates them idempotently).
echo "==> dropping schemas oikumenea + atlas_schema_revisions in $DEV_DB"
admin -c "DROP SCHEMA IF EXISTS oikumenea CASCADE;"
admin -c "DROP SCHEMA IF EXISTS atlas_schema_revisions CASCADE;"

# 2. Re-apply every migration (recreates schema oikumenea, all tables, RLS grants, schema_version).
echo "==> applying migrations to $DEV_DB"
DATABASE_URL="$ADMIN_DSN" atlas migrate apply --env local

# 3. Re-seed the bootstrap admin via the same idempotent seed the first-boot path uses (D-Bootstrap),
#    reading var/conf/install.yml bootstrap-admin. No --force needed on the fresh schema.
#    Always rebuild first: a STALE binary silently seeds against old code/schema (or no-ops), which is
#    worse than a few seconds of incremental build.
BIN="out/build/oikumenea/unspecified/linux-amd64/oikumenea"
echo "==> building oikumenea binary"
./godelw build
echo "==> seeding bootstrap admin"
"$BIN" bootstrap-admin

echo "==> dev database reset: $ADMIN_DSN"
echo "    start the server: $BIN serve"
