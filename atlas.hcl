// Atlas configuration for go-oikumenea (D-Migrations / upgrade-safety.md).
// Versioned migrations in a single repo-root migrations/ dir. The migration SQL is fully
// schema-qualified (oikumenea.*) and creates the schema itself, so no search_path is needed.
//
//   atlas migrate hash  --env local   # refresh atlas.sum after editing migrations
//   atlas migrate lint  --env local   # destructive-change gate (uses an ephemeral dev DB)
//   atlas migrate apply --env local   # apply to the target DB

env "local" {
  // Target DB for `migrate apply`. Override with --url for other environments.
  url = "postgres://postgres:dev@localhost:5432/postgres?sslmode=disable"

  // Ephemeral dev database Atlas uses to analyze/lint migrations (requires Docker).
  dev = "docker://postgres/16/dev"

  migration {
    dir = "file://migrations"
  }
}
