#!/usr/bin/env bash
# Generate the web console's RID decode tables (web/src/lib/ontology/generated-rid.ts) from pkg/rid —
# the boot-asserted single source of truth for the ontology type registry (R-28, review-2026-09). The
# file is GENERATED, never hand-edited; the tables that used to be hand-maintained in rid.ts had
# drifted (missing services 15/16/17/19, "geo" vs "location", stale pre-M41 education types).
#
# Usage:
#   scripts/gen-ontology-mirror.sh            # regenerate the committed file
#   scripts/gen-ontology-mirror.sh --verify   # regenerate to a temp file and fail if it differs (CI drift gate)
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="web/src/lib/ontology/generated-rid.ts"

if [[ "${1:-}" == "--verify" ]]; then
  WORK="$(mktemp -d)"
  trap 'rm -rf "$WORK"' EXIT
  go run ./tools/gen-ontology-mirror -out "$WORK/generated-rid.ts"
  if ! diff -q "$OUT" "$WORK/generated-rid.ts" >/dev/null 2>&1; then
    echo "gen-ontology-mirror: FAIL — $OUT is out of date vs pkg/rid. Run scripts/gen-ontology-mirror.sh and commit." >&2
    diff "$OUT" "$WORK/generated-rid.ts" | head -40 >&2 || true
    exit 1
  fi
  echo "gen-ontology-mirror: OK — $OUT matches pkg/rid." >&2
else
  go run ./tools/gen-ontology-mirror -out "$OUT"
fi
