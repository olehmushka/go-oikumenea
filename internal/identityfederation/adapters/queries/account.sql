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
