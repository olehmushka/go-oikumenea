#!/usr/bin/env bash
# Regenerate pkg/action/params_gen.go — the per-action parameter schemas (review-2026-09 R-29 seam),
# single-sourced from the Conjure contracts. Dumps a fresh IR (via the repo's ir2openapi tool), then
# projects each request-body type's fields into Params. Run after changing api/*.conjure.yml or the
# RequestType annotations in pkg/action/registry.go. Never hand-edit params_gen.go.
#
# Usage:  scripts/gen-action-params.sh
set -euo pipefail
cd "$(dirname "$0")/.."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "dumping Conjure IR…"
go run ./tools/ir2openapi -dump-ir "$TMP/ir.json"

echo "generating pkg/action/params_gen.go…"
go run ./tools/genactionparams -ir "$TMP/ir.json" -out pkg/action/params_gen.go
gofmt -w pkg/action/params_gen.go

echo "generating pkg/action/endpoints_gen.go…"
go run ./tools/genactionendpoints -ir "$TMP/ir.json" -out pkg/action/endpoints_gen.go
gofmt -w pkg/action/endpoints_gen.go
echo "done."
