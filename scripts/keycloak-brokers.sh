#!/usr/bin/env bash
# Configure BROKERED identity providers on the local-dev Keycloak realm (Example A of
# deploy/oauth/README.md — D-MultiIdPExamples).
#
# Google, GitHub, Entra, GitLab and Okta are federated INTO the `oikumenea` realm, so Keycloak stays
# the only token issuer and go-oikumenea needs no new `idp.issuers[]` entry: the tokens it validates
# are still Keycloak's. This is the ONLY route by which GitHub can reach the service at all — GitHub
# is OAuth2 without OIDC (no ID token, no JWKS), so nothing it issues is directly verifiable.
#
# Applied over the Admin REST API rather than baked into deploy/keycloak/realm-oikumenea.json,
# because each provider needs a CLIENT SECRET and that file is committed. Secrets stay in the
# environment; this script is idempotent, so re-run it after every `docker compose up` (the dev
# Keycloak uses an ephemeral H2 DB and re-imports the realm on each start).
#
# Usage:
#   set -a; . ./.env.oauth; set +a      # your gitignored per-provider credentials
#   scripts/keycloak-brokers.sh
#
# Env (a provider is configured IFF its id+secret are set). For each <provider> below, any of these
# spellings works — checked in order, first hit wins — so credentials already sitting in your .env
# under a different convention are picked up without renaming:
#
#   OAUTH_<PROVIDER>_ID       / OAUTH_<PROVIDER>_SECRET          (canonical)
#   AUTH_<PROVIDER>_ID        / AUTH_<PROVIDER>_SECRET           (Auth.js / console convention)
#   <provider>_client_id      / <provider>_client_secret         (lowercase, as written by hand)
#
#   providers: GOOGLE, GITHUB, GITLAB, ENTRA, OKTA
#   ENTRA and OKTA additionally need an issuer: OAUTH_<P>_ISSUER | AUTH_<P>_ISSUER | <p>_issuer
#   (Entra's issuer MUST end in /v2.0)
# Optional: KC_BASE (http://localhost:8080), KC_REALM (oikumenea),
#           KC_ADMIN / KC_ADMIN_PASS (admin/admin).
#
# Each provider's OAuth app must list this redirect URI:
#   ${KC_BASE}/realms/${KC_REALM}/broker/<alias>/endpoint
set -euo pipefail

KC_BASE="${KC_BASE:-http://localhost:8080}"
KC_REALM="${KC_REALM:-oikumenea}"
KC_ADMIN="${KC_ADMIN:-admin}"
KC_ADMIN_PASS="${KC_ADMIN_PASS:-admin}"

command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }

# cred <PROVIDER> <ID|SECRET|ISSUER> — first non-empty of the accepted spellings, else empty.
# Accepting several spellings is deliberate: these values are hand-copied out of five different
# provider consoles into whatever env file the operator already keeps, and a silent "provider not
# configured" because the variable was named `github_client_id` instead of `OAUTH_GITHUB_ID` is a
# genuinely confusing failure.
cred() {
  local provider="$1" field="$2"
  local upper lower suffix
  upper="$(printf '%s' "$provider" | tr '[:lower:]' '[:upper:]')"
  lower="$(printf '%s' "$provider" | tr '[:upper:]' '[:lower:]')"
  case "$field" in
    ID)     suffix="client_id" ;;
    SECRET) suffix="client_secret" ;;
    ISSUER) suffix="issuer" ;;
  esac
  local name
  for name in "OAUTH_${upper}_${field}" "AUTH_${upper}_${field}" "${lower}_${suffix}"; do
    if [ -n "${!name:-}" ]; then printf '%s' "${!name}"; return 0; fi
  done
  printf ''
}

# Admin token from the MASTER realm (the dev Keycloak's own console login).
TOKEN="$(curl -fsS -X POST "${KC_BASE}/realms/master/protocol/openid-connect/token" \
  -d grant_type=password -d client_id=admin-cli \
  -d "username=${KC_ADMIN}" -d "password=${KC_ADMIN_PASS}" | jq -r .access_token)"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { echo "could not obtain a Keycloak admin token" >&2; exit 1; }

# NOTE: the admin REST path is `identity-provider` (SINGULAR) even though it returns a collection;
# the plural spelling 404s.
IDP_URL="${KC_BASE}/admin/realms/${KC_REALM}/identity-provider/instances"

# upsert <alias> <json-representation>
# POST creates, PUT updates — so a re-run against an already-configured realm is a no-op update
# rather than a 409.
upsert() {
  local alias="$1" body="$2"
  if curl -fsS -o /dev/null -H "Authorization: Bearer ${TOKEN}" "${IDP_URL}/${alias}" 2>/dev/null; then
    curl -fsS -X PUT "${IDP_URL}/${alias}" \
      -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
      -d "$body" >/dev/null
    echo "  updated  ${alias}"
  else
    curl -fsS -X POST "${IDP_URL}" \
      -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
      -d "$body" >/dev/null
    echo "  created  ${alias}"
  fi
  echo "           redirect URI: ${KC_BASE}/realms/${KC_REALM}/broker/${alias}/endpoint"
}

# builtin <alias> <keycloak-provider-id> <client-id> <client-secret> [scopes]
# For the providers Keycloak ships native support for; it already knows their endpoints.
builtin_idp() {
  local alias="$1" pid="$2" cid="$3" secret="$4" scope="${5:-}"
  upsert "$alias" "$(jq -n \
    --arg alias "$alias" --arg pid "$pid" --arg cid "$cid" --arg secret "$secret" --arg scope "$scope" \
    '{alias:$alias, providerId:$pid, enabled:true, storeToken:false, trustEmail:true,
      linkOnly:false, firstBrokerLoginFlowAlias:"first broker login",
      config:({clientId:$cid, clientSecret:$secret}
              + (if $scope == "" then {} else {defaultScope:$scope} end))}')"
}

# generic_oidc <alias> <issuer> <client-id> <client-secret>
# For IdPs with no native Keycloak provider (Entra ID, Okta): resolve the endpoints from the issuer's
# OIDC discovery document so the config is derived rather than hand-copied.
generic_oidc() {
  local alias="$1" issuer="$2" cid="$3" secret="$4"
  local disco
  disco="$(curl -fsS "${issuer%/}/.well-known/openid-configuration")" || {
    echo "  SKIPPED  ${alias}: discovery failed at ${issuer%/}/.well-known/openid-configuration" >&2
    return 0
  }
  upsert "$alias" "$(jq -n \
    --arg alias "$alias" --arg cid "$cid" --arg secret "$secret" \
    --argjson d "$disco" \
    '{alias:$alias, providerId:"oidc", enabled:true, storeToken:false, trustEmail:true,
      linkOnly:false, firstBrokerLoginFlowAlias:"first broker login",
      config:{clientId:$cid, clientSecret:$secret,
              issuer:$d.issuer,
              authorizationUrl:$d.authorization_endpoint,
              tokenUrl:$d.token_endpoint,
              jwksUrl:$d.jwks_uri,
              userInfoUrl:($d.userinfo_endpoint // ""),
              logoutUrl:($d.end_session_endpoint // ""),
              useJwksUrl:"true",
              defaultScope:"openid email profile",
              clientAuthMethod:"client_secret_post"}}')"
}

echo "Configuring brokered identity providers on ${KC_BASE}/realms/${KC_REALM}"
configured=0

GOOGLE_ID="$(cred google ID)";  GOOGLE_SECRET="$(cred google SECRET)"
GITHUB_ID="$(cred github ID)";  GITHUB_SECRET="$(cred github SECRET)"
GITLAB_ID="$(cred gitlab ID)";  GITLAB_SECRET="$(cred gitlab SECRET)"
ENTRA_ID="$(cred entra ID)";    ENTRA_SECRET="$(cred entra SECRET)";  ENTRA_ISSUER="$(cred entra ISSUER)"
OKTA_ID="$(cred okta ID)";      OKTA_SECRET="$(cred okta SECRET)";    OKTA_ISSUER="$(cred okta ISSUER)"

if [ -n "$GOOGLE_ID" ] && [ -n "$GOOGLE_SECRET" ]; then
  builtin_idp google google "$GOOGLE_ID" "$GOOGLE_SECRET" "openid email profile"
  configured=$((configured+1))
fi

# GitHub has no `openid` scope — Keycloak's github provider reads the profile from the GitHub API
# instead. `user:email` is needed because a GitHub user's primary email is often private, and without
# it Keycloak cannot populate the email it matches accounts on.
if [ -n "$GITHUB_ID" ] && [ -n "$GITHUB_SECRET" ]; then
  builtin_idp github github "$GITHUB_ID" "$GITHUB_SECRET" "user:email"
  configured=$((configured+1))
fi

if [ -n "$GITLAB_ID" ] && [ -n "$GITLAB_SECRET" ]; then
  builtin_idp gitlab gitlab "$GITLAB_ID" "$GITLAB_SECRET" "openid email profile"
  configured=$((configured+1))
fi

if [ -n "$ENTRA_ID" ] && [ -n "$ENTRA_SECRET" ] && [ -n "$ENTRA_ISSUER" ]; then
  generic_oidc entra "$ENTRA_ISSUER" "$ENTRA_ID" "$ENTRA_SECRET"
  configured=$((configured+1))
fi

if [ -n "$OKTA_ID" ] && [ -n "$OKTA_SECRET" ] && [ -n "$OKTA_ISSUER" ]; then
  generic_oidc okta "$OKTA_ISSUER" "$OKTA_ID" "$OKTA_SECRET"
  configured=$((configured+1))
fi

if [ "$configured" -eq 0 ]; then
  echo "No provider credentials found in the environment — nothing configured." >&2
  echo "Set at least one OAUTH_<PROVIDER>_ID / _SECRET pair (see the header of this script)." >&2
  exit 1
fi

cat <<EOF

Done — ${configured} provider(s) configured.

The realm login page now offers them. NOTE what a brokered login produces: Keycloak mints the token,
so its \`iss\` stays ${KC_BASE}/realms/${KC_REALM} and its \`sub\` is the KEYCLOAK user id, not the
upstream provider's. go-oikumenea therefore needs no new issuer entry — but a first-time brokered
user is an UNKNOWN (issuer, subject) and is rejected by design (D-JIT reject-unknown). Link it:

  POST /identity/v1/accounts/{accountId}/identities  {"issuer":"...","subject":"<keycloak sub>"}

\`trustEmail\` is on above, so a brokered login whose email matches an EXISTING Keycloak user attaches
to that user and keeps its \`sub\` — meaning an already-linked person keeps working across providers.
That is convenient for a demo and a real decision in production: it makes the upstream IdP's email
verification load-bearing. Turn it off if any configured provider allows unverified emails.
EOF
