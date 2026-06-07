#!/usr/bin/env bash
# Generate the typed API surface for the web console from the committed OpenAPI spec.
# The output (src/lib/api/schema.d.ts) is GENERATED — never hand-edited (D-WebUI). The spec
# itself is kept fresh from Conjure by repo-root scripts/gen-openapi.sh.
set -euo pipefail
cd "$(dirname "$0")/.."

# Spec location: ../docs/api/openapi/openapi.json in the repo; overridable for the Docker build,
# where the spec is copied next to the app (see web/Dockerfile).
SPEC="${OPENAPI_SPEC:-../docs/api/openapi/openapi.json}"

OUT="src/lib/api/schema.d.ts"

if [[ ! -f "$SPEC" ]]; then
  # In the Docker build the spec is not in the build context (./web). Fall back to the
  # committed schema.d.ts if present; only fail when there is nothing to build against.
  if [[ -f "$OUT" ]]; then
    echo "gen-api-types: spec '$SPEC' not found — using existing $OUT." >&2
    exit 0
  fi
  echo "gen-api-types: OpenAPI spec not found at '$SPEC' and no $OUT to fall back to." >&2
  echo "  Set OPENAPI_SPEC, or run repo-root scripts/gen-openapi.sh first." >&2
  exit 1
fi

mkdir -p src/lib/api
npx --yes openapi-typescript "$SPEC" -o "$OUT"
echo "gen-api-types: wrote $OUT from $SPEC"
