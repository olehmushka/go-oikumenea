# Module: identity-federation

> Reads: [glossary](../glossary.md) · [conventions](../architecture/conventions.md) ·
> [patterns](../architecture/patterns.md) · [decisions](../architecture/decisions.md)
> Table prefix: `oikumenea.account_*`

## Purpose

The seam to the **external identity provider**. go-oikumenea does **not authenticate** — it
**validates inbound identities** issued elsewhere and turns them into a **PDP context**
(L-AuthzOnly). It **stores no credentials and issues no tokens**. This module owns the optional
login **account** (an attachment to a [person](person.md)), the **external identities** linked
to it, and the inbound-token validation middleware (OIDC discovery + JWKS) that maps a verified
token → external identity → account → person → PDP subject. The drafts' OAuth **credential
vault** is deliberately dropped — there are no secrets to keep.

An account is the person's **set of login points**: a person who signs in via more than one
provider (e.g. Google and Keycloak) holds **one** account linking **several** external
identities — one per provider — and resolves to the same person/PDP context regardless of which
IdP issued the inbound token. Whether additional identities may be linked to an existing account
is operator-gated by the **`account.identity_linking.enabled`** install config (default `true`).

## Entities & aggregates

**Ontology kinds** (D-Ontology; [registry](../ontology-mapping.md)) — **Objects:** `Account` (attached
to its person via the `HAS_ACCOUNT` link, ≤1 active) and `External identity` (bearing the append-only
`FEDERATES` link to its account). **Actions:** `CreateAccount`/`DisableAccount`,
`LinkExternalIdentity`/`UnlinkExternalIdentity` — audited, `action__<type>` RID. Inbound-token
resolution to a PDP context is a read, not an Action.

- **Account** (aggregate root) — an optional login attachment to exactly one person. A person
  may have zero accounts (roster-only) or one account. Carries account status; reserves a
  **dormant seam** of columns for a future full-IdP pivot. The install **bootstrap** (D-Bootstrap)
  creates the first account + external identity, binding the seeded instance admin to the
  configured IdP `(issuer, subject)`.
- **External identity** — a verified `(issuer, subject)` pair linked to an account; **one
  link per login point**. An account may carry one or several such links (e.g. a Google
  identity *and* a Keycloak identity for the same person); each `(issuer, subject)` belongs
  to exactly one account. Linking *additional* identities beyond the first is gated by the
  `account.identity_linking.enabled` install config.

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`account_accounts`**
- `id` PK
- `person_id TEXT NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `email CITEXT` — optional, as asserted by the IdP; `UNIQUE WHERE deleted_at IS NULL` when set
  — `pii:contact`
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled'))`
- **Dormant seam (always NULL while auth is delegated):** `password_hash TEXT`,
  `mfa_enrolled_at TIMESTAMPTZ` — reserved so a future "become a full IdP" pivot is additive,
  not a rewrite. Documented as dormant; a `CHECK` keeps them NULL until that capability ships.
  `password_hash` is marked `secret` (a separate axis, not a `pii:` tier — D-PIITiers).
- `created_at`, `updated_at`, `deleted_at`
- `UNIQUE (person_id) WHERE deleted_at IS NULL` — at most one active account per person.

**`account_external_identities`**
- `id` PK
- `account_id TEXT NOT NULL REFERENCES account_accounts(id) ON DELETE CASCADE`
- `issuer TEXT NOT NULL` — the IdP `iss` — `pii:none`
- `subject TEXT NOT NULL` — the IdP `sub` (a pseudonymous identifier for a real person) — `pii:basic`
- `created_at`
- `UNIQUE (issuer, subject)` — a given external identity maps to exactly one account.
- Index `(account_id)`.

The schema permits **many rows per `account_id`** — one per login point. The cap on
*additional* links per account is enforced in the application layer at link time (see
`account.identity_linking.enabled` under [platform](platform.md) install config), not by a
DB constraint, so flipping the config is reversible without a migration.

No token columns: access/refresh tokens are never persisted.

## Inbound token validation (the middleware)

Wired by [platform](platform.md) ahead of every authenticated handler:

1. Read `Authorization: Bearer <jwt>`.
2. Validate against the configured IdP(s): check `iss` is an allowed issuer, fetch/cache the
   issuer's **JWKS** (OIDC discovery), verify **signature**, and validate `exp`/`nbf`/`aud`.
3. Map the verified `(iss, sub)` → `account_external_identities` → `account_accounts` →
   `person`. Because an account may carry several `(iss, sub)` rows (one per login point),
   the same person resolves to the same account regardless of which IdP issued the inbound
   token. Unknown identity → **reject** by default. If just-in-time provisioning is enabled,
   the only behavior is **link-on-match** (D-JIT): match the verified token against an *existing*
   person via an operator-configured mapping (a token claim → `person.code` or a designated
   attribute); on a match, create or extend the person's `account` with this
   `external_identity` and link; on **no match, reject**. JIT **never creates a person**.
4. Construct the **PDP context** `(person, account, request_id)` and attach it to the request
   context. Handlers and the [authorization](authorization.md) PDP read it from there.

Issuer list, JWKS URIs, audience, and clock-skew tolerance are **install config** (ECV +
`pkg/refreshable`); JWKS is cached and refreshed on rotation. Production issuers are **OIDC/JWKS**
(asymmetric). A **symmetric `hs256` issuer** (a shared HMAC key in install config) is a local/dev
convenience so tests can mint tokens; because that key is a credential-equivalent (whoever holds it
can mint valid tokens for any subject), the service **refuses to boot** with an `hs256` issuer
configured unless `environment` is `local` or `dev` — fail-closed (L-AuthzOnly: the service holds
no credentials in staging/prod).

## Conjure API surface

`IdentityFederationService`:

| Op | Intent | Perm |
|---|---|---|
| `GET /accounts/{id}` | Read an account (+ linked identities) | `person.read` (the linked person) |
| `POST /accounts` | Create an account for a person | `person.update` |
| `POST /accounts/{id}/disable` | Disable login (reversible) | `person.update` |
| `POST /accounts/{id}/identities` | Link an external identity (additional login point) | `person.update` |
| `DELETE /accounts/{id}/identities/{idid}` | Unlink an external identity | `person.update` |
| `GET /whoami` | Resolve the caller's own PDP context (person + account) | authenticated |

`POST /accounts/{id}/identities` returns `Conflict` when `account.identity_linking.enabled =
false` and the account already has an active external identity. The first identity on a new
account (bootstrap or first JIT link) is always permitted.

Token validation itself is **middleware**, not an endpoint; there is intentionally no
login/token-issuing endpoint.

## Dependencies

- **Calls:** [person](person.md) (a new/just-in-time account links to a person).
  [platform](platform.md) for config + the JWKS HTTP client + middleware wiring. Emits
  `AccountLinked`, `ExternalIdentityLinked/Unlinked` events.
- **Called by:** **all** transport (the middleware runs before every authenticated handler);
  [authorization](authorization.md) consumes the PDP context it produces; [audit](audit.md).

## Authorization touchpoints

Account/identity management is gated on `person.*` permissions of the linked person (an account
is an attachment to a person, so it inherits that person's authorization surface). `/whoami` is
authenticated-only. The middleware itself is the authentication boundary, not an authorization
check.

## Invariants & safety

- **No credentials, no tokens.** The DB never holds passwords/secrets while auth is delegated;
  the dormant columns are `CHECK`-enforced NULL.
- **`(issuer, subject)` is globally unique** — one external identity → one account.
- **At most one active account per person.**
- **An account may carry one or several external identities** (one per login point); linking
  *additional* identities beyond the first is operator-gated by the
  `account.identity_linking.enabled` install config (default `true`).
- A person can exist with **no** account (roster-only) — accounts are optional.
- Token validation failures are uniform `Unauthorized` (no oracle about which check failed).
- **Symmetric (HS256) issuers are local/dev only.** The service refuses to boot with an `hs256`
  issuer configured unless `environment` is `local`/`dev` (fail-closed; the symmetric key is a
  credential-equivalent — L-AuthzOnly). Staging/prod issuers are OIDC/JWKS (asymmetric).

## Open seams / future

- **Full-IdP pivot:** the dormant `password_hash` / `mfa_enrolled_at` columns + a future
  `account_sessions` table let go-oikumenea optionally *become* an authenticator later, additive
  by design ([patterns.md](../architecture/patterns.md), Dormant seam). Until then it is a pure
  relying party.
- Multiple simultaneous issuers (multi-IdP) are supported by config + the
  `(issuer, subject)` model. The per-account
  `account.identity_linking.enabled` knob is the **per-account counterpart** to that
  per-deployment seam ([open-questions](../open-questions.md) DS-20): the deployment chooses
  which issuers it accepts; the knob chooses whether a single account may attach more than
  one of them.
- Just-in-time provisioning is resolved (D-JIT): default **reject-unknown**; when enabled,
  **link-on-match only** via a configurable claim→person-key mapping — it never auto-creates a
  person. Full auto-enrolment remains out of scope.
