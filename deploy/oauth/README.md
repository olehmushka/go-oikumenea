# OAuth2 / OIDC provider examples

How to sign into go-oikumenea with **Google**, **GitHub**, **Microsoft Entra ID**, **GitLab**,
**Okta** — or **any other provider you want** — alongside (or instead of) the local Keycloak.

Binding decision: [**D-MultiIdPExamples**](../../docs/architecture/decisions.md) ·
Module: [identity-federation](../../docs/modules/identity-federation.md) ·
Local Keycloak stack: [../keycloak/README.md](../keycloak/README.md)

**Contents**

1. [The one thing to understand first](#the-one-thing-to-understand-first)
2. [Add any provider — the general recipe](#add-any-provider--the-general-recipe)
3. [Registering the app in each provider's own UI](#registering-the-app-in-each-providers-own-ui)
4. [Example A — broker through Keycloak](#example-a--broker-through-keycloak)
5. [Example B — direct issuers, no broker](#example-b--direct-issuers-no-broker)
6. [Linking a person to a provider in the console UI](#linking-a-person-to-a-provider-in-the-console-ui)
7. [Troubleshooting](#troubleshooting)

---

## The one thing to understand first

go-oikumenea **never authenticates** ([L-AuthzOnly](../../docs/architecture/decisions.md)). It
validates a **JWT** that someone else issued: it routes on the token's `iss` to a configured
`idp.issuers[]` entry, verifies the signature against that issuer's **JWKS**, checks `aud`/`exp`, and
maps the verified `(issuer, subject)` to an account.

Everything below follows from that one sentence:

| | Google | GitHub | Entra ID | GitLab | Okta |
|---|---|---|---|---|---|
| Is it OIDC (ID token + JWKS)? | ✅ | ❌ **no** | ✅ | ✅ | ✅ |
| Can be a **direct** `idp.issuers[]` entry | ✅ | ❌ | ✅ | ✅ | ✅ |
| Works **brokered** through Keycloak | ✅ | ✅ | ✅ | ✅ | ✅ |

**GitHub is the exception that shapes the design.** GitHub is OAuth2 *without* OIDC: it issues no ID
token, publishes no JWKS, and its access tokens are opaque `gho_*` strings. There is nothing for this
service to verify, so GitHub can never be a direct issuer — it reaches oikumenea only via **Example
A**. Anything that claimed otherwise would have to make the service trust an unverifiable bearer.

So there are two topologies, and you can run both at once:

```
 EXAMPLE A — brokered                     EXAMPLE B — direct
 (one issuer; the only route for GitHub)  (one issuer per provider)

  Google ─┐                                Google ──────────┐
  GitHub ─┼─→ Keycloak ─→ oikumenea        Entra ───────────┼─→ oikumenea
  Entra  ─┘   (the issuer)                 Keycloak ────────┘   (validates each
                                                                 iss separately)
```

| | Example A (broker) | Example B (direct) |
|---|---|---|
| `idp.issuers[]` entries | 1 (Keycloak) | 1 per provider |
| Supports GitHub | ✅ | ❌ |
| Keycloak required | ✅ | ❌ |
| Where provider choice happens | Keycloak's login page | the console's login page |
| Token `sub` | the **Keycloak** user id | the **provider's** subject |
| Add a provider | no oikumenea restart | edit `install.yml`, restart |

That `sub` row is the one that bites. Under A, every provider maps to one Keycloak user and therefore
**one** stable `(issuer, subject)`; under B, the same human signing in via Google and via Keycloak is
**two** external identities — which is exactly what an account is designed to hold (several login
points, one person), but they must each be linked.

---

## Add any provider — the general recipe

This works for providers not named in this document (Authentik, Zitadel, Auth0, Ping, Discord, your
corporate IdP…). Five steps, and **step 0 decides which topology you are in**.

### Step 0 — Ask the provider whether it speaks OIDC

```bash
curl -fsS https://THE-PROVIDER/.well-known/openid-configuration | jq '{issuer, jwks_uri}'
```

- **It returns JSON with `issuer` and `jwks_uri`** → OIDC. Either topology works; **direct** (B) is
  simpler. Note the `issuer` value it prints — that exact string, byte for byte, is what you will
  configure. (Some providers' discovery lives under a path, e.g.
  `https://id.example.org/application/o/app/.well-known/openid-configuration`.)
- **404, HTML, or connection error** → OAuth2 only, like GitHub. It **cannot** be a direct issuer.
  Go to Example A and broker it; Keycloak's generic `oauth2` / per-provider support does the
  translation and issues a real JWT.

### Step 1 — Register the OAuth app with the provider

In the provider's own console (walkthroughs [below](#registering-the-app-in-each-providers-own-ui)).
The **redirect / callback URI** is the part people get wrong, and it depends on the topology:

| Topology | Redirect URI to register |
|---|---|
| A — brokered | `http://localhost:8080/realms/oikumenea/broker/<alias>/endpoint` |
| B — direct | `http://localhost:8445/api/auth/callback/<provider-id>` |

`<alias>` / `<provider-id>` is a short name you choose (`google`, `github`, `authentik`…). Register
**both** URIs if you want the provider available in both topologies. Keep the **client ID** and
**client secret** it gives you.

### Step 2 — Tell the console about it

Add to `web/.env.local` (gitignored). For a provider with a named entry in
[`web/src/lib/auth/providers.ts`](../../web/src/lib/auth/providers.ts):

```bash
AUTH_GOOGLE_ID="…"      AUTH_GOOGLE_SECRET="…"
```

For **anything else — no code change needed** — use a generic OIDC slot. `<NAME>` is yours to pick;
it becomes the provider id (lowercased) and therefore the callback path:

```bash
AUTH_OIDC_AUTHENTIK_ISSUER="https://id.example.org/application/o/oikumenea/"
AUTH_OIDC_AUTHENTIK_ID="…"
AUTH_OIDC_AUTHENTIK_SECRET="…"
AUTH_OIDC_AUTHENTIK_LABEL="Authentik"        # optional button text
```

That registers a button for it, using discovery for the endpoints and forwarding the ID token. Its
callback URI is `http://localhost:8445/api/auth/callback/authentik`.

*(Brokered providers need nothing here — to the console they are one Keycloak login.)*

### Step 3 — Tell the service about it

**Topology A: skip this entirely.** Keycloak is still the issuer; the existing entry covers it.

**Topology B:** add the issuer to `var/conf/install.yml` and restart:

```yaml
idp:
  issuers:
    - issuer: "https://id.example.org/application/o/oikumenea/"   # EXACTLY as discovery printed it
      type: oidc
      label: "Authentik"                                          # display name in the console
      audience: "<the same client id as AUTH_OIDC_AUTHENTIK_ID>"  # MANDATORY
```

Three rules, all enforced:

- **`issuer` must match the token's `iss` byte for byte** — it is the routing key. A trailing slash,
  `http` vs `https`, or a missing `/v2.0` means the token routes nowhere and 401s.
- **`audience` is mandatory** and must be the OAuth **client id**, not `oikumenea`. The service
  **refuses to boot** without it — see [Troubleshooting](#troubleshooting) for why.
- If several clients of *this* deployment share one issuer (a console and a CLI registered
  separately), list them all under `audiences: [a, b]` instead of adding a second issuer entry.

### Step 4 — Sign in once, expect 401, then link

Unknown identities are rejected by design. See
[Linking a person to a provider](#linking-a-person-to-a-provider-in-the-console-ui).

### Step 5 — Confirm

```bash
curl -sk https://localhost:8443/identity/v1/issuers -H "Authorization: Bearer $TOKEN" | jq
```

Your provider should be listed with its label and audiences. If it is not, the service did not load
your config.

---

## Registering the app in each provider's own UI

Click-by-click, since every console words this differently. Throughout: **A-URI** =
`http://localhost:8080/realms/oikumenea/broker/<alias>/endpoint` (brokered), **B-URI** =
`http://localhost:8445/api/auth/callback/<id>` (direct).

### Google — Google Cloud Console

1. <https://console.cloud.google.com> → pick or create a **project**.
2. **APIs & Services → OAuth consent screen**. Choose **External** (or *Internal* for a Workspace-only
   deployment), fill app name + support email, save. While the app is in **Testing**, only accounts
   you add under **Test users** can sign in — this is the usual cause of "Access blocked".
3. **APIs & Services → Credentials → Create credentials → OAuth client ID**.
4. Application type **Web application**.
5. **Authorised redirect URIs** → add B-URI (`…/callback/google`) and/or A-URI (`…/broker/google/endpoint`).
6. Create → copy **Client ID** and **Client secret**. (The *Download JSON* button gives you the same
   values; that file contains a live secret — keep it out of git.)

- Direct issuer: `https://accounts.google.com` · audience: the client ID.

### GitHub — Developer settings

**Brokered only.** GitHub has no OIDC discovery, so there is no direct option.

1. <https://github.com/settings/developers> → **OAuth Apps → New OAuth App**.
   (For an org-owned app: *Organisation settings → Developer settings → OAuth Apps*.)
2. **Application name**: anything. **Homepage URL**: `http://localhost:8445`.
3. **Authorization callback URL**: `http://localhost:8080/realms/oikumenea/broker/github/endpoint`
   — the Keycloak broker endpoint, **not** the console. GitHub allows one callback URL per app, so a
   second app is needed if you also want a non-brokered use.
4. Register → copy the **Client ID**, then **Generate a new client secret** and copy it (shown once).

- Scope `user:email` is requested for you — a GitHub primary email is often private, and Keycloak
  needs it to match accounts.
- Note: *OAuth Apps*, not *GitHub Apps*. GitHub Apps are a different product with a different flow.

### Microsoft Entra ID — Azure portal

1. <https://portal.azure.com> → **Microsoft Entra ID → App registrations → New registration**.
2. **Supported account types**: single tenant unless you deliberately want multi-tenant (multi-tenant
   changes the issuer and means you must validate the tenant yourself).
3. **Redirect URI**: platform **Web**, value A-URI and/or B-URI (`…/callback/entra`).
4. Register → copy **Application (client) ID** and **Directory (tenant) ID** from *Overview*.
5. **Certificates & secrets → New client secret** → copy the **Value** (not the Secret ID).
6. **Token configuration** (optional): add `email` as an optional claim if your tenant does not emit it.

- Issuer: `https://login.microsoftonline.com/<tenant-id>/v2.0` — **the `/v2.0` suffix is mandatory**.
  The v1 issuer `https://sts.windows.net/<tenant>/` mints a different token shape.
- Audience: the Application (client) ID.

### GitLab — User/Group settings → Applications

1. <https://gitlab.com/-/user_settings/applications> (or *Admin → Applications* on self-managed).
2. **Name**: anything. **Redirect URI**: A-URI and/or B-URI (`…/callback/gitlab`) — one per line.
3. **Scopes**: tick **`openid`**, **`email`**, **`profile`**. Without `openid` GitLab returns no ID
   token and the direct topology silently cannot work.
4. Uncheck *Confidential* only if you know why. Save → copy **Application ID** and **Secret**.

- Issuer: `https://gitlab.com`, or your instance's base URL (no trailing path).
- Audience: the Application ID.

### Okta — Admin console

1. Okta admin → **Applications → Create App Integration**.
2. **OIDC — OpenID Connect** + **Web Application**.
3. **Sign-in redirect URIs**: A-URI and/or B-URI (`…/callback/okta`).
4. Assign the app to the users/groups who should be able to sign in — Okta denies unassigned users.
5. Save → copy **Client ID** and **Client secret**.

- Issuer: `https://<tenant>.okta.com`, **or** the custom authorization server's issuer
  (`https://<tenant>.okta.com/oauth2/<auth-server-id>`) if you use one. Check with
  `curl .../.well-known/openid-configuration` and use exactly what it prints.
- Audience: the Client ID.

---

## Example A — broker through Keycloak

Keycloak stays the only issuer, so **`var/conf/install.yml` needs no change** and GitHub works.

**1. Register the OAuth app** with each provider using the **A-URI** above.

**2. Put the credentials in a gitignored env file.** The script accepts several spellings per
provider — `OAUTH_GITHUB_ID`, `AUTH_GITHUB_ID`, or `github_client_id` — so credentials already in
your `.env` are picked up without renaming:

```bash
OAUTH_GOOGLE_ID=...          OAUTH_GOOGLE_SECRET=...
github_client_id=...         github_client_secret=...
OAUTH_ENTRA_ID=...           OAUTH_ENTRA_SECRET=...    OAUTH_ENTRA_ISSUER=https://login.microsoftonline.com/<tenant>/v2.0
```

**3. Apply them:**

```bash
docker compose -f docker-compose.dev.yml up -d       # postgres + keycloak
set -a; . ./.env; set +a                             # or ./.env.oauth
scripts/keycloak-brokers.sh                          # idempotent
```

The dev Keycloak re-imports its realm on every start (ephemeral H2), so **re-run the script after
each `up`**. It is idempotent, so that is safe.

**4. Sign in** at the console (`http://localhost:8445`) → *Sign in with Keycloak* → the Keycloak login
page now shows a button per configured provider. Then follow
[Linking](#linking-a-person-to-a-provider-in-the-console-ui).

### Per-provider notes

- **GitHub** — no `openid` scope exists; Keycloak reads the profile from GitHub's API and the script
  requests `user:email` because primary addresses are frequently private.
- **Entra ID / Okta** — no native Keycloak provider, so the script configures the generic `oidc` one
  and fills the endpoints from the issuer's discovery document rather than hand-copied URLs.
- **Account linking** — the script sets `trustEmail: true`, so a brokered login whose email matches an
  existing Keycloak user attaches to it and keeps that user's `sub` (one oikumenea identity across
  providers). This makes the upstream IdP's email verification load-bearing; turn it off if any
  configured provider permits unverified addresses.
- **Doing it in the Keycloak admin UI instead:** *Identity providers → Add provider →* pick the
  provider, paste client id/secret, copy the *Redirect URI* Keycloak shows into the provider's app.
  The script exists so this is reproducible after each realm re-import, not because the UI is wrong.

---

## Example B — direct issuers, no broker

Each provider becomes its own `idp.issuers[]` entry. GitHub cannot participate. Follow
[the general recipe](#add-any-provider--the-general-recipe) — steps 1–3 are exactly this topology.

### The two ways this goes wrong

**Which token gets forwarded.** The service validates a JWT, so the console must forward one. A public
IdP's *access* token is usually an opaque string; its **ID token** is the JWT. The console's provider
registry ([`web/src/lib/auth/providers.ts`](../../web/src/lib/auth/providers.ts)) records this per
provider — Keycloak forwards its `access_token` (its realm audience mapper stamps `aud: oikumenea`),
every public IdP forwards its `id_token`. Get it wrong and you get a console that logs in happily and
then 401s on every API call.

**Which audience gets pinned.** Because the ID token's `aud` is the console's OAuth **client id**, the
`audience` in `install.yml` must be that same client id — not `oikumenea`.

### Per-provider summary

| Provider | `issuer` | `audience` | Watch out for |
|---|---|---|---|
| Google | `https://accounts.google.com` | OAuth client id | Refresh tokens need `access_type=offline` + `prompt=consent` (the console sets both). While the consent screen is in *Testing*, only listed test users can sign in. |
| Entra ID | `https://login.microsoftonline.com/<tenant>/v2.0` | application (client) id | **Must** end in `/v2.0`. |
| GitLab | `https://gitlab.com` (or your instance) | application id | Returns an `id_token` only when `openid` is among the app's scopes. |
| Okta | `https://<tenant>.okta.com` | client id | Custom authorization servers have a different issuer; users must be assigned to the app. |
| *anything else* | whatever discovery prints | its client id | Use a generic `AUTH_OIDC_<NAME>_*` slot — no code change. |
| GitHub | — | — | Not possible. Use Example A. |

---

## Linking a person to a provider in the console UI

A first login through a new provider is an **unknown `(issuer, subject)` and is rejected with 401**.
That is the designed enrolment path, not a misconfiguration
([D-JIT](../../docs/architecture/decisions.md): reject-unknown; JIT never creates a person). Linking
is what turns that provider into a working login point for an existing person.

**1. Attempt the login once and let it fail.** The service logs the pair it rejected:

```
unknown inbound identity  issuer=https://accounts.google.com  subject=1179…
```

Copy the `subject`. What it *is* depends on the topology — and this is the most common mistake:

| Topology | `subject` is… | Looks like | It is **not**… |
|---|---|---|---|
| A — brokered | the **Keycloak** user's `sub` | `11111111-1111-1111-1111-111111111111` | the Google/GitHub id, the username, or the email |
| B — direct, Google | Google's `sub` claim | `117701234567890123456` (~21 digits) | the email address |
| B — direct, Entra | the object id (`oid`/`sub`) | a UUID | the UPN |
| B — direct, GitLab/Okta | that provider's `sub` | numeric (GitLab) / opaque (Okta) | the username |

Two things people reasonably expect and which are **not** true:

- **A brokered Google login does not carry Google's id.** Keycloak minted the token, so `iss` is the
  realm and `sub` is the Keycloak user's UUID — the upstream provider's identifier never reaches
  oikumenea. Find it under *Users → the new user → ID* in the Keycloak admin console.
- **Google's `sub` is not scoped to your OAuth client.** It identifies the Google *account*, is stable
  forever and is never reused — which is exactly why the `audience` pin does the work of binding a
  token to this deployment (see the boot guard above).

Never use the email address as `subject`: it is mutable, may be unverified, and is not the `sub`.

**2. Open the person in the console:** `http://localhost:8445` → **Persons** → the person → the
**Account** panel.

**3. If the person has no account yet**, create one (a person is account-optional — many are
roster-only). The form takes the person's email and optionally the first identity at the same time.

**4. Add the login point.** In *Login identities* → the **issuer** dropdown lists every issuer this
instance accepts, by its configured `label` ("Google", "Keycloak") — that list comes from
`GET /identity/v1/issuers`, so **a provider that does not appear there is not configured on the
service**, and linking it would create an identity that can never authenticate. Pick the issuer,
paste the `subject`, save.

**5. Sign in again.** Same person, now reachable through the new provider. The panel shows all their
login points; an account may hold several — one per provider — and every one resolves to the same
person and the same PDP context.

To revoke a single login point, unlink it there; to disable *all* login for the person, use **Disable
login** (reversible, and it keeps the directory record intact).

> **Same thing over the API**, if you prefer:
> ```bash
> curl -k -X POST https://localhost:8443/identity/v1/accounts/$ACCOUNT_ID/identities \
>   -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
>   -d '{"issuer":"https://accounts.google.com","subject":"1179…"}'
> ```
> `GET /identity/v1/persons/{personId}/account` gives you `$ACCOUNT_ID`. Linking *additional*
> identities is gated by `account.identity-linking-enabled` (default `true`).

### If you only know people's email addresses

The flow above needs the `subject`, which you can only get from a token. When you are enrolling people
you know **by email**, invert it: prepare the account first, and let the first sign-in attach itself.

**1. Enable the account-email match arm** in `var/conf/install.yml` and restart:

```yaml
idp:
  jit:
    enabled: true
    claim: email             # read the token's `email` claim ...
    match: account-email     # ... and match it against the account's email
```

**2. For each person, create a login-less shell account carrying their address** — the *Account* panel
does exactly this when you fill in the email and leave issuer/subject blank (`POST /accounts` with
`personId` + `email`, no `identity`).

**3. They sign in.** The token's verified email matches the account, the `(issuer, subject)` is
attached to it automatically, and nobody ever types a 21-digit number.

This is still **link-on-match**, not auto-enrolment: an address with no prepared account is rejected,
and [D-JIT](../../docs/architecture/decisions.md)'s "JIT never creates a person" is unchanged.

**What you are trusting when you enable it.** The email stops being a label and becomes an
authentication key, so:

- **`email_verified` must be present and true** — the service refuses the match otherwise. An IdP that
  does not emit the claim cannot use this arm at all (fail-closed, deliberately).
- **Any configured issuer that asserts that verified address matches.** If you accept both Keycloak and
  Google, someone signing in through *either* with that address reaches the account. That is the
  intended "one person, several login points" semantic — but it means every issuer you configure is
  trusted to verify email honestly. Configure only issuers you trust to do so, and prefer a tenant
  whose domain you control.
- **`account.identity-linking-enabled: false` caps each account at one login point**, including on this
  path. Set it when you want exactly one provider per person and no silent second identity.
- Prefer per-person addresses over shared mailboxes: whoever controls the mailbox controls the login.

*(The other arm, `match: code` — the default — matches the claim against `person.code` instead, and is
the right one when your IdP already stamps a personnel number into the token.)*

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| Boot fails: *"pins no audience"* | An `oidc` issuer has no `audience`/`audiences`. Deliberate and fail-closed: a public IdP's `iss` is shared by every app registered with it, so without an `aud` check an ID token minted for an unrelated application would carry an issuer/subject this instance accepts. |
| Console logs in, every API call 401s | The forwarded token is not a JWT the service accepts: wrong `forward` for the provider, or `audience` pinned to `oikumenea` instead of the client id. |
| 401 on a brand-new provider login | Expected — unknown `(issuer, subject)`. [Link it.](#linking-a-person-to-a-provider-in-the-console-ui) |
| 401 after a previously working login | `iss` mismatch: the configured issuer must equal the token's `iss` **exactly** (trailing slash, `/v2.0`, `localhost` vs `127.0.0.1`). |
| Provider button missing from `/login` | Its `AUTH_<PROVIDER>_ID`/`_SECRET` are not set in the console's environment. |
| Provider missing from the issuer dropdown | It is not in the service's `idp.issuers[]` — expected and correct for a **brokered** provider (Keycloak is the issuer). |
| `redirect_uri_mismatch` at the provider | The registered URI differs from the one sent — check topology (broker vs console), port, and scheme. |
| Brokered login works, no email | GitHub without `user:email`, or a provider not releasing the claim. |
| Google: *"Access blocked: app not verified"* | Consent screen still in *Testing* — add the account under **Test users**. |
| `scripts/keycloak-brokers.sh` says nothing configured | No recognised credential variables in the environment; `set -a; . ./.env; set +a` first. |
