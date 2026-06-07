#!/usr/bin/env bash
# Mint an access token from the local-dev Keycloak (docker-compose.dev.yml + deploy/keycloak/) via the
# OAuth2 password grant, and print the raw access token to stdout. Use it to call the app by hand:
#
#   TOKEN=$(scripts/keycloak-token.sh)
#   curl -k https://localhost:8443/identity/v1/whoami -H "Authorization: Bearer $TOKEN"
#
# Args (optional): <username> <password>   (default: admin admin — the imported realm user).
# Env (optional):  KC_BASE (default http://localhost:8080), KC_REALM (oikumenea), KC_CLIENT (oikumenea).
set -euo pipefail

USER_NAME="${1:-admin}"
PASS="${2:-admin}"
KC_BASE="${KC_BASE:-http://localhost:8080}"
KC_REALM="${KC_REALM:-oikumenea}"
KC_CLIENT="${KC_CLIENT:-oikumenea}"

RESP="$(curl -fsS -X POST \
  "${KC_BASE}/realms/${KC_REALM}/protocol/openid-connect/token" \
  -d "grant_type=password" \
  -d "client_id=${KC_CLIENT}" \
  -d "username=${USER_NAME}" \
  -d "password=${PASS}")"

# Prefer jq; fall back to a portable grep/sed extraction so the script works without jq installed.
if command -v jq >/dev/null 2>&1; then
  printf '%s\n' "$RESP" | jq -r '.access_token'
else
  printf '%s' "$RESP" | grep -o '"access_token":"[^"]*"' | sed 's/"access_token":"\([^"]*\)"/\1/'
fi
