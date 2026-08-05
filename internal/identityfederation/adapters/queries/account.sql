-- identity-federation queries (M8). Optional login accounts + the verified (issuer, subject) external
-- identities that federate to them (docs/modules/identity-federation.md). RID PKs are minted by the
-- column DEFAULT new_rid('account', ...) on the GUC-bearing pool. No token/credential columns are ever
-- written (L-AuthzOnly). Reads exclude soft-deleted accounts; identities are immutable once created.

-- name: InsertAccount :one
-- Create an optional login account for a person (email optional). The per-person active-account index
-- backstops the <=1-active invariant; the person FK backstops existence.
INSERT INTO oikumenea.account_accounts (person_id, email)
VALUES (@person_id, sqlc.narg('email'))
RETURNING *;

-- name: GetAccount :one
SELECT * FROM oikumenea.account_accounts
WHERE id = @id AND deleted_at IS NULL;

-- name: GetActiveAccountByPerson :one
-- The person's single active (not soft-deleted) account, of any status. Used to reuse an existing
-- account when linking another identity and by the first-admin bootstrap (D-Bootstrap).
SELECT * FROM oikumenea.account_accounts
WHERE person_id = @person_id AND deleted_at IS NULL;

-- name: GetActiveAccountByEmail :one
-- The single active account holding this IdP-asserted email, of any status. Backs the D-JIT
-- link-on-match ATTRIBUTE arm: an operator who knows only a person's email creates a login-less shell
-- account carrying it, and the person's first sign-in matches here and attaches its (issuer, subject).
--
-- `email` is citext, so the comparison is case-insensitive, and the partial unique index
-- (account_accounts_email_active_idx) makes "the single account" true by construction rather than by
-- convention — there is no ambiguous-match case to resolve in Go.
SELECT * FROM oikumenea.account_accounts
WHERE email = @email AND deleted_at IS NULL;

-- name: DisableAccount :one
-- Reversible login block: flip status to 'disabled'. Idempotent at the app layer.
UPDATE oikumenea.account_accounts
SET status = 'disabled'
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: InsertIdentity :one
-- Link a verified (issuer, subject) login point to an account. The global (issuer, subject) unique
-- index backstops one-identity-one-account; the account FK backstops existence.
INSERT INTO oikumenea.account_external_identities (account_id, issuer, subject)
VALUES (@account_id, @issuer, @subject)
RETURNING *;

-- name: GetIdentity :one
SELECT * FROM oikumenea.account_external_identities
WHERE id = @id;

-- name: DeleteIdentity :execrows
-- Unlink (hard remove) an identity from a specific account. Scoping by account_id keeps an unlink
-- from another account a no-op (zero rows affected -> ErrIdentityNotFound).
DELETE FROM oikumenea.account_external_identities
WHERE id = @id AND account_id = @account_id;

-- name: ListIdentitiesByAccount :many
SELECT * FROM oikumenea.account_external_identities
WHERE account_id = @account_id
ORDER BY created_at, id;

-- name: CountActiveIdentities :one
-- The number of login points federated to an account (identities have no soft-delete: existence is
-- "active"). Feeds the account.identity_linking.enabled cap on ADDITIONAL identities.
SELECT count(*) FROM oikumenea.account_external_identities
WHERE account_id = @account_id;

-- name: ResolveBySubject :one
-- The inbound-token directory lookup: map a verified (issuer, subject) to its active account + person
-- (the PDP subject). Restricting to active, not-soft-deleted accounts means a disabled/removed account
-- resolves to nothing -> the middleware rejects (uniform Unauthorized).
SELECT a.person_id, a.id AS account_id, a.email
FROM oikumenea.account_external_identities e
JOIN oikumenea.account_accounts a ON a.id = e.account_id
WHERE e.issuer = @issuer AND e.subject = @subject
  AND a.deleted_at IS NULL AND a.status = 'active';

-- ---------------------------------------------------------------------------------------------------
-- Service principals (M51 / D-ServiceIdentities): the (issuer, subject) -> MACHINE-subject registry.
-- Same key shape as account_external_identities, resolved by the same middleware. A principal holds no
-- role assignment and no unit reach; its authority is authz_principal_grants (authorization module).

-- name: InsertPrincipal :one
-- Register a machine subject. The active (issuer, subject) and code unique indexes backstop the
-- conflict shapes; the symmetric collision trigger backstops "this pair is already a person identity".
INSERT INTO oikumenea.account_service_principals (code, name, description, issuer, subject, client_id)
VALUES (@code, @name, sqlc.narg('description'), @issuer, @subject, sqlc.narg('client_id'))
RETURNING *;

-- name: GetPrincipal :one
SELECT * FROM oikumenea.account_service_principals
WHERE id = @id AND deleted_at IS NULL;

-- name: GetPrincipalByCode :one
SELECT * FROM oikumenea.account_service_principals
WHERE code = @code AND deleted_at IS NULL;

-- name: UpdatePrincipal :one
-- Mutable fields only. (issuer, subject) is the identity key the middleware resolves on and is
-- immutable by design — re-pointing it would silently transfer a machine's authority to another IdP
-- client, so the application rejects it rather than offering it here.
UPDATE oikumenea.account_service_principals
SET name = @name, description = sqlc.narg('description'), client_id = sqlc.narg('client_id')
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SetPrincipalStatus :one
-- Reversible kill switch: a disabled principal fails resolution, so its tokens stop working at once
-- while the audit rows referencing it stay intact.
UPDATE oikumenea.account_service_principals
SET status = @status
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPrincipals :many
-- Keyset page over the registry (RID order), matching the repo-wide token-pagination convention.
SELECT * FROM oikumenea.account_service_principals
WHERE deleted_at IS NULL
  AND (sqlc.arg('after')::text = '' OR id::text > sqlc.arg('after')::text)
ORDER BY id
LIMIT @row_limit;

-- name: ResolvePrincipalBySubject :one
-- The inbound machine-token lookup. Restricting to active, not-soft-deleted principals means a
-- disabled or removed principal resolves to nothing -> the middleware rejects (uniform Unauthorized).
SELECT id, code, client_id
FROM oikumenea.account_service_principals
WHERE issuer = @issuer AND subject = @subject
  AND deleted_at IS NULL AND status = 'active';

-- name: PrincipalIsActive :one
-- Backs the authorization module's PrincipalDirectory port so a grant write validates its principal
-- without reading this module's table directly (CLAUDE.md: cross-module queries are interface calls).
SELECT EXISTS (
  SELECT 1 FROM oikumenea.account_service_principals
  WHERE id = @id AND deleted_at IS NULL AND status = 'active'
);
