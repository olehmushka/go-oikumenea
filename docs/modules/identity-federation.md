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

## Entities & aggregates

- **Account** (aggregate root) — an optional login attachment to exactly one person. A person
  may have zero accounts (roster-only) or one account. Carries account status; reserves a
  **dormant seam** of columns for a future full-IdP pivot.
- **External identity** — a verified `(issuer, subject)` pair linked to an account. One account
  may link several (the same person logging in via two IdPs).

## Data model

Conventions per [conventions.md](../architecture/conventions.md).

**`account_accounts`**
- `id` PK
- `person_id UUID NOT NULL REFERENCES person_persons(id) ON DELETE RESTRICT`
- `email CITEXT` — optional, as asserted by the IdP; `UNIQUE WHERE deleted_at IS NULL` when set
- `status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled'))`
- **Dormant seam (always NULL while auth is delegated):** `password_hash TEXT`,
  `mfa_enrolled_at TIMESTAMPTZ` — reserved so a future "become a full IdP" pivot is additive,
  not a rewrite. Documented as dormant; a `CHECK` keeps them NULL until that capability ships.
- `created_at`, `updated_at`, `deleted_at`
- `UNIQUE (person_id) WHERE deleted_at IS NULL` — at most one active account per person.

**`account_external_identities`**
- `id` PK
- `account_id UUID NOT NULL REFERENCES account_accounts(id) ON DELETE CASCADE`
- `issuer TEXT NOT NULL` — the IdP `iss`
- `subject TEXT NOT NULL` — the IdP `sub`
- `created_at`
- `UNIQUE (issuer, subject)` — a given external identity maps to exactly one account.
- Index `(account_id)`.

No token columns: access/refresh tokens are never persisted.

## Inbound token validation (the middleware)

Wired by [platform](platform.md) ahead of every authenticated handler:

1. Read `Authorization: Bearer <jwt>`.
2. Validate against the configured IdP(s): check `iss` is an allowed issuer, fetch/cache the
   issuer's **JWKS** (OIDC discovery), verify **signature**, and validate `exp`/`nbf`/`aud`.
3. Map the verified `(iss, sub)` → `account_external_identities` → `account_accounts` →
   `person`. Unknown identity → configurable: reject, or (if just-in-time provisioning is
   enabled) create a pending account linked to a person per operator policy.
4. Construct the **PDP context** `(person, account, request_id)` and attach it to the request
   context. Handlers and the [authorization](authorization.md) PDP read it from there.

Issuer list, JWKS URIs, audience, and clock-skew tolerance are **install config** (ECV +
`pkg/refreshable`); JWKS is cached and refreshed on rotation.

## Conjure API surface

`IdentityFederationService`:

| Op | Intent | Perm |
|---|---|---|
| `GET /accounts/{id}` | Read an account (+ linked identities) | `person.read` (the linked person) |
| `POST /accounts` | Create an account for a person | `person.update` |
| `POST /accounts/{id}/disable` | Disable login (reversible) | `person.update` |
| `POST /accounts/{id}/identities` | Link an external identity | `person.update` |
| `DELETE /accounts/{id}/identities/{idid}` | Unlink an external identity | `person.update` |
| `GET /whoami` | Resolve the caller's own PDP context (person + account) | authenticated |

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
- A person can exist with **no** account (roster-only) — accounts are optional.
- Token validation failures are uniform `Unauthorized` (no oracle about which check failed).

## Open seams / future

- **Full-IdP pivot:** the dormant `password_hash` / `mfa_enrolled_at` columns + a future
  `account_sessions` table let go-oikumenea optionally *become* an authenticator later, additive
  by design ([patterns.md](../architecture/patterns.md), Dormant seam). Until then it is a pure
  relying party.
- Multiple simultaneous issuers (multi-IdP) are supported by config + the
  `(issuer, subject)` model.
- Just-in-time account provisioning policy is operator-configurable; the default is
  reject-unknown.
