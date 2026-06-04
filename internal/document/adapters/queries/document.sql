-- Document module queries (docs/modules/document.md). Papers (document_documents) typed by
-- document_document_types, and government personal codes (document_personal_codes, value
-- envelope-encrypted) typed by the natural-key document_personal_code_schemes catalog. RID PKs default
-- at the database. A NULL narg leaves the stored value unchanged on update (COALESCE); `code` is
-- immutable. Existence of the referenced person/type/scheme/country is validated by the FKs (mapped in
-- the adapter), so these queries carry no pre-check lookups.

-- ============================ document types ============================

-- name: InsertDocumentType :one
INSERT INTO oikumenea.document_document_types (code, name, sort_order)
VALUES (@code, @name, sqlc.narg('sort_order'))
RETURNING *;

-- name: GetDocumentType :one
SELECT * FROM oikumenea.document_document_types WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateDocumentType :one
UPDATE oikumenea.document_document_types SET
  name       = COALESCE(sqlc.narg('name'), name),
  status     = COALESCE(sqlc.narg('status'), status),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListDocumentTypes :many
SELECT * FROM oikumenea.document_document_types
WHERE deleted_at IS NULL
ORDER BY sort_order NULLS LAST, code;

-- ============================ documents (papers) ============================

-- name: InsertDocument :one
INSERT INTO oikumenea.document_documents (
  person_id, type_id, number, issuer, issuing_country, issued_on, expires_on, attributes
) VALUES (
  @person_id, @type_id, sqlc.narg('number'), sqlc.narg('issuer'), sqlc.narg('issuing_country'),
  sqlc.narg('issued_on')::date, sqlc.narg('expires_on')::date,
  COALESCE(sqlc.narg('attributes')::jsonb, '{}')
)
RETURNING *;

-- name: GetDocument :one
SELECT * FROM oikumenea.document_documents WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateDocument :one
-- Partial update: a NULL narg leaves the value unchanged. Clearing number/issuer/issuing_country to
-- NULL via this path is an open seam (COALESCE cannot set NULL).
UPDATE oikumenea.document_documents SET
  number          = COALESCE(sqlc.narg('number'), number),
  issuer          = COALESCE(sqlc.narg('issuer'), issuer),
  issuing_country = COALESCE(sqlc.narg('issuing_country'), issuing_country),
  issued_on       = COALESCE(sqlc.narg('issued_on')::date, issued_on),
  expires_on      = COALESCE(sqlc.narg('expires_on')::date, expires_on),
  attributes      = COALESCE(sqlc.narg('attributes')::jsonb, attributes),
  status          = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDocument :execrows
UPDATE oikumenea.document_documents SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: ListDocumentsByPerson :many
SELECT * FROM oikumenea.document_documents
WHERE person_id = @person_id AND deleted_at IS NULL
  AND (@after = '' OR id > @after)
ORDER BY id
LIMIT @lim;

-- name: ErasePersonDocuments :execrows
-- Purge: NULL the pii:basic number/issuer and reset the pii:special attributes for a person's
-- documents, keeping the row ids as tombstones (D-PIITiers).
UPDATE oikumenea.document_documents SET number = NULL, issuer = NULL, attributes = '{}'
WHERE person_id = @person_id AND deleted_at IS NULL;

-- ============================ personal-code schemes ============================

-- name: InsertScheme :one
INSERT INTO oikumenea.document_personal_code_schemes (
  code, country_iso, generic_category, name, validation_regex, sort_order
) VALUES (
  @code, sqlc.narg('country_iso'), @generic_category, @name, sqlc.narg('validation_regex'),
  sqlc.narg('sort_order')
)
RETURNING *;

-- name: GetScheme :one
SELECT * FROM oikumenea.document_personal_code_schemes WHERE code = @code AND deleted_at IS NULL;

-- name: UpdateScheme :one
UPDATE oikumenea.document_personal_code_schemes SET
  country_iso      = COALESCE(sqlc.narg('country_iso'), country_iso),
  generic_category = COALESCE(sqlc.narg('generic_category'), generic_category),
  name             = COALESCE(sqlc.narg('name'), name),
  validation_regex = COALESCE(sqlc.narg('validation_regex'), validation_regex),
  status           = COALESCE(sqlc.narg('status'), status),
  sort_order       = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE code = @code AND deleted_at IS NULL
RETURNING *;

-- name: ListSchemes :many
SELECT * FROM oikumenea.document_personal_code_schemes
WHERE deleted_at IS NULL
  AND (@country = '' OR country_iso::text = @country)
  AND (@category = '' OR generic_category = @category)
ORDER BY sort_order NULLS LAST, code;

-- ============================ personal codes (encrypted values) ============================

-- name: InsertPersonalCode :one
-- The person/scheme FKs validate existence; the (scheme_code, value_blind_index) unique index enforces
-- cross-person uniqueness over the blind index (mapped in the adapter).
INSERT INTO oikumenea.document_personal_codes (
  person_id, scheme_code, value_ciphertext, wrapped_dek, key_ref, value_blind_index
) VALUES (
  @person_id, @scheme_code, @value_ciphertext, @wrapped_dek, @key_ref, @value_blind_index
)
RETURNING *;

-- name: GetPersonalCode :one
SELECT * FROM oikumenea.document_personal_codes WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePersonalCode :one
-- Full mutable set: the application supplies the (re-)encrypted value fields + status.
UPDATE oikumenea.document_personal_codes SET
  value_ciphertext  = @value_ciphertext,
  wrapped_dek       = @wrapped_dek,
  key_ref           = @key_ref,
  value_blind_index = @value_blind_index,
  status            = @status
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeletePersonalCode :execrows
UPDATE oikumenea.document_personal_codes SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- name: ListPersonalCodesByPerson :many
SELECT * FROM oikumenea.document_personal_codes
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY id;

-- name: CryptoErasePersonCodes :execrows
-- Purge: destroy the wrapped DEK and ciphertext so the value is unrecoverable (crypto-erase;
-- D-CryptoProvider), keeping the row id + blind index as a tombstone.
UPDATE oikumenea.document_personal_codes SET value_ciphertext = NULL, wrapped_dek = NULL
WHERE person_id = @person_id AND deleted_at IS NULL;
