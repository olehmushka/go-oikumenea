#!/usr/bin/env bash
# Generate the TypeScript SDK (clients/typescript) from the Conjure contract — the SAME source of
# truth as the Go SDK (client/) and the server (internal/conjure). The per-service clients under
# clients/typescript/src/generated are GENERATED, never hand-edited (D-ClientSDK / D-Conjure); only
# src/index.ts (the unified façade) and the package scaffolding are hand-written.
#
# Pipeline (no JVM):
#   api/*.conjure.yml --(godel, via tools/ir2openapi -dump-ir)--> Conjure IR JSON
#     --(rewrite-ir-packages.mjs: 2-seg -> 3-seg packages)--> IR conjure-typescript accepts
#       --(conjure-typescript generate --rawSource)--> clients/typescript/src/generated
#
# Usage:
#   scripts/gen-ts-client.sh            # regenerate src/generated
#   scripts/gen-ts-client.sh --verify   # regenerate to a temp dir and fail if it differs (CI drift gate)
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

CT_VERSION="${CONJURE_TS_VERSION:-5.18.0}"
PKG_DIR="clients/typescript"
GEN_DIR="$PKG_DIR/src/generated"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "gen-ts-client: extracting Conjure IR…" >&2
go run ./tools/ir2openapi -dump-ir "$WORK/conjure-ir.json"

echo "gen-ts-client: rewriting IR package names for conjure-typescript…" >&2
node "$PKG_DIR/scripts/rewrite-ir-packages.mjs" "$WORK/conjure-ir.json" "$WORK/conjure-ir.3seg.json"

target="$GEN_DIR"
if [[ "${1:-}" == "--verify" ]]; then
  target="$WORK/generated"
fi
rm -rf "$target"
mkdir -p "$target"

echo "gen-ts-client: running conjure-typescript@$CT_VERSION…" >&2
npx --yes "conjure-typescript@$CT_VERSION" generate --rawSource "$WORK/conjure-ir.3seg.json" "$target"

if [[ "${1:-}" == "--verify" ]]; then
  if ! diff -r -q "$GEN_DIR" "$target" >/dev/null 2>&1; then
    echo "gen-ts-client: FAIL — $GEN_DIR is out of date vs the contract. Run scripts/gen-ts-client.sh and commit." >&2
    diff -r "$GEN_DIR" "$target" | head -40 >&2 || true
    exit 1
  fi
  echo "gen-ts-client: OK — generated SDK matches the contract." >&2
else
  echo "gen-ts-client: wrote $GEN_DIR" >&2
fi
