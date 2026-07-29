#!/usr/bin/env bash
# Regenerate the IR-derived Go mirrors of the Conjure contract — all of them, from ONE IR dump:
#
#   pkg/action/params_gen.go     per-action parameter schemas   (review-2026-09 R-29 seam)
#   pkg/action/endpoints_gen.go  per-action endpoint bindings   (review-2026-09 R-33 seam)
#   pkg/facet/args_gen.go        per-list-endpoint query args   (M56 / D-ObjectFacets facet-arg guard)
#
# Run after changing api/*.conjure.yml, the RequestType annotations in pkg/action/registry.go, or the
# ListEndpoint bindings in pkg/facet/catalog.go. Never hand-edit the generated files.
#
# Usage:
#   scripts/gen-action-params.sh            regenerate in place
#   scripts/gen-action-params.sh --verify   regenerate into a temp dir and diff; non-zero if stale
#
# --verify is what makes the drift guards mean anything. The guard tests compare the CATALOGS against
# the GENERATED mirrors; if a mirror is stale, they happily validate against yesterday's contract and
# pass. This mode proves the committed mirrors match the contract as it stands right now, so CI (and
# `make verify`) catch a contract change whose regeneration was forgotten.
set -euo pipefail
cd "$(dirname "$0")/.."

VERIFY=0
if [[ "${1:-}" == "--verify" ]]; then
  VERIFY=1
elif [[ $# -gt 0 ]]; then
  echo "usage: $0 [--verify]" >&2
  exit 2
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "dumping Conjure IR…"
go run ./tools/ir2openapi -dump-ir "$TMP/ir.json"

# Generate into $dest — the repo itself for a normal run, a scratch tree for --verify.
dest="."
if [[ $VERIFY -eq 1 ]]; then
  dest="$TMP/out"
  mkdir -p "$dest/pkg/action" "$dest/pkg/facet"
fi

echo "generating pkg/action/params_gen.go…"
go run ./tools/genactionparams -ir "$TMP/ir.json" -out "$dest/pkg/action/params_gen.go"
gofmt -w "$dest/pkg/action/params_gen.go"

echo "generating pkg/action/endpoints_gen.go…"
go run ./tools/genactionendpoints -ir "$TMP/ir.json" -out "$dest/pkg/action/endpoints_gen.go"
gofmt -w "$dest/pkg/action/endpoints_gen.go"

echo "generating pkg/facet/args_gen.go…"
go run ./tools/genfacetargs -ir "$TMP/ir.json" -out "$dest/pkg/facet/args_gen.go"
gofmt -w "$dest/pkg/facet/args_gen.go"

if [[ $VERIFY -eq 1 ]]; then
  stale=0
  for f in pkg/action/params_gen.go pkg/action/endpoints_gen.go pkg/facet/args_gen.go; do
    if ! diff -u "$f" "$dest/$f" >/dev/null 2>&1; then
      echo "STALE: $f does not match the current Conjure contract:" >&2
      diff -u "$f" "$dest/$f" >&2 || true
      stale=1
    fi
  done
  if [[ $stale -ne 0 ]]; then
    echo >&2
    echo "run scripts/gen-action-params.sh and commit the regenerated files" >&2
    exit 1
  fi
  echo "verify: all IR-derived mirrors are current."
  exit 0
fi

echo "done."
