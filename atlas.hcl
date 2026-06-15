// Atlas configuration for go-oikumenea (D-Migrations / upgrade-safety.md).
// Versioned migrations in a single repo-root migrations/ dir. The migration SQL is fully
// schema-qualified (oikumenea.*) and creates the schema itself, so no search_path is needed.
//
//   set -a; . ./.env; set +a           # export DATABASE_URL (and friends) from .env first
//   atlas migrate hash  --env local   # refresh atlas.sum after editing migrations
//   atlas migrate lint  --env local   # destructive-change gate (uses an ephemeral dev DB)
//   atlas migrate apply --env local   # apply to the target DB

locals {
  // The operator DB DSN comes from $DATABASE_URL (see .env / .env.example); when unset, fall back
  // to the local-dev default so the documented commands still work without sourcing .env.
  db_url = getenv("DATABASE_URL") != "" ? getenv("DATABASE_URL") : "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"
}

env "local" {
  // Target DB for `migrate apply`. Override via $DATABASE_URL (.env) or --url.
  url = local.db_url

  // Ephemeral dev database Atlas uses to analyze/lint migrations (requires Docker). PostGIS-enabled
  // image: the bootstrap migration `CREATE EXTENSION postgis` for the WOF gazetteer (D-GeoPlaces).
  dev = "docker://postgis/postgis/16-3.4/dev"

  migration {
    dir = "file://migrations"
  }
}

// The hermenea companion service owns a SEPARATE database (D-Hermenea) with its own migration set.
//   set -a; . ./.env; set +a
//   atlas migrate hash  --env hermenea
//   atlas migrate apply --env hermenea
env "hermenea" {
  // Target DB for hermenea. Override via $HERMENEA_DATABASE_URL (.env) or --url. Defaults to a
  // local-dev `hermenea` database on the same Postgres.
  url = getenv("HERMENEA_DATABASE_URL") != "" ? getenv("HERMENEA_DATABASE_URL") : "postgres://postgres:dev@localhost:5432/hermenea?sslmode=disable"

  // Ephemeral dev database Atlas uses to analyze/lint migrations (requires Docker).
  dev = "docker://postgres/16/dev"

  migration {
    dir = "file://migrations/hermenea"
  }
}
