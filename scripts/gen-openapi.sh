#!/usr/bin/env bash
# Generate the OpenAPI v3 document from the Conjure contract (docs/api/README.md). OpenAPI is a derived
# artifact — api/*.conjure.yml is the source of truth — so the spec cannot drift from the server.
#
# Conjure has no official OpenAPI generator, so this uses the repo's own emitter, tools/ir2openapi. That
# tool gets the Conjure IR from godel (no JVM, no network: `godel conjure-publish` builds the IR and the
# tool captures it from a local sink) and converts it to OpenAPI. Output: docs/api/openapi/openapi.json.
#
# Usage:  scripts/gen-openapi.sh
set -euo pipefail

cd "$(dirname "$0")/.."

go run ./tools/ir2openapi -out docs/api/openapi

echo "done. OpenAPI written to docs/api/openapi/openapi.json"
echo "render it with e.g.:  npx @redocly/cli build-docs docs/api/openapi/openapi.json -o openapi.html"
