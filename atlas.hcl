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

  // Ephemeral dev database Atlas uses to analyze/lint migrations (requires Docker).
  dev = "docker://postgres/16/dev"

  migration {
    dir = "file://migrations"
  }
}
