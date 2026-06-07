# Local Keycloak — manual OIDC end-to-end testing

A real OIDC IdP wired into the local dev stack so you can mint genuine RS256 tokens and exercise the
full `token → validate → resolve → PDP` path by hand. The app validates inbound tokens against
`var/conf/install.yml` `idp.issuers[]`; this brings up a Keycloak that issues tokens those rules accept.

## What's here

- **`docker-compose.dev.yml`** runs `postgres` + `keycloak` (the app runs from the host binary).
- **`realm-oikumenea.json`** is auto-imported on Keycloak start (`start-dev --import-realm`):
  - realm **`oikumenea`**;
  - public client **`oikumenea`** with the direct-access (password) grant + standard flow, an **audience
    mapper** that adds `aud: oikumenea`, and a hardcoded **`person_code: admin`** claim (for the JIT path);
  - user **`admin`** / password **`admin`**, with a **fixed `sub`** UUID
    `11111111-1111-1111-1111-111111111111`.
- `var/conf/install.yml` adds the issuer `http://localhost:8080/realms/oikumenea` (`type: oidc`,
  `audience: oikumenea`) and points **`bootstrap-admin`** at that user's UUID, so the Keycloak `admin`
  user is the instance admin from first boot.

The issuer is `http://localhost:8080/...` for **both** the host (minting tokens) and the host-run app
(OIDC discovery), so there is no hostname/DNS mismatch to manage.

## Run it

```bash
cp .env.example .env                                  # one-time
docker compose -f docker-compose.dev.yml up -d        # postgres + keycloak

# Postgres: apply migrations + create the app login role (one-time, as in the dev compose header)
set -a; . ./.env; set +a
atlas migrate apply --env local
psql "$DATABASE_URL" -c "CREATE ROLE oikumenea LOGIN PASSWORD 'dev' IN ROLE oikumenea_app;"  # if not already created

# Wait for Keycloak to be ready (a few seconds on first boot):
curl -fsS http://localhost:8080/health/ready && echo "keycloak ready"

# Build + run the app (reads var/conf/install.yml; first boot seeds the Keycloak admin)
./godelw build
out/build/oikumenea/*/linux-amd64/oikumenea serve
```

In another shell:

```bash
TOKEN=$(scripts/keycloak-token.sh)                    # admin/admin password grant

curl -k https://localhost:8443/identity/v1/whoami \
  -H "Authorization: Bearer $TOKEN"                   # -> the admin person + account + email

curl -k https://localhost:8443/person/v1/persons \
  -H "Authorization: Bearer $TOKEN"                   # -> 200 (instance-admin path)

curl -k https://localhost:8443/person/v1/persons      # no token -> 401
```

- App API is HTTPS self-signed on **:8443** (`-k`); management/health is on **:8444**
  (`curl -k https://localhost:8444/status/readiness`).
- Keycloak admin console: <http://localhost:8080> (console login `admin`/`admin`).

## How the admin login resolves

First boot runs the idempotent bootstrap (`internal/identityfederation/bootstrap`): it seeds a person
(`code = admin`) + account + the external identity `(http://localhost:8080/realms/oikumenea,
11111111-…-111111111111)` and grants instance-admin. A Keycloak token for the `admin` user carries that
exact `iss`/`sub`, so it resolves straight to the bootstrapped admin — no JIT needed. (JIT-by-`person_code`
also works if you set `idp.jit.enabled: true`, since the client stamps `person_code: admin`.)

## Notes

- **Ephemeral:** Keycloak dev uses an embedded H2 DB and re-imports the realm on every `up`, so any
  console changes are lost on restart — edit `realm-oikumenea.json` for durable changes.
- **Token lifespan** is 30 min (`accessTokenLifespan` in the realm); re-run `scripts/keycloak-token.sh`
  when it expires.
- **Changing the port:** if `OIKU_KEYCLOAK_HOST_PORT` ≠ 8080, also change the issuer URL in
  `var/conf/install.yml` (`idp.issuers` **and** `bootstrap-admin`) — the token `iss` must match exactly.
- This is **insecure local config** (public client, password grant, `admin/admin`). Real deployments use
  a confidential client / proper flows and ECV-encrypted secrets.
