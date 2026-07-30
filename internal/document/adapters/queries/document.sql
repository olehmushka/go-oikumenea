-- Document module queries (docs/modules/document.md). Papers (document_documents) typed by
-- document_document_types, and government personal codes (document_personal_codes, value
-- envelope-encrypted) typed by the natural-key document_personal_code_schemes catalog. RID PKs default
-- at the database. A NULL narg leaves the stored value unchanged on update (COALESCE); `code` is
-- immutable. Existence of the referenced person/type/scheme/country is validated by the FKs (mapped in
-- the adapter), so these queries carry no pre-check lookups.

-- ============================ document types ============================

-- name: InsertDocumentType :one
INSERT INTO oikumenea.document_document_types (code, name, attr_schema, sort_order)
VALUES (@code, @name, sqlc.narg('attr_schema')::jsonb, sqlc.narg('sort_order'))
RETURNING *;

-- name: GetDocumentType :one
SELECT * FROM oikumenea.document_document_types WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateDocumentType :one
-- attr_schema is replaced when provided (NULL narg leaves it unchanged via COALESCE; clearing it back
-- to NULL is an open seam, consistent with the other COALESCE'd fields).
UPDATE oikumenea.document_document_types SET
  name        = COALESCE(sqlc.narg('name'), name),
  attr_schema = COALESCE(sqlc.narg('attr_schema')::jsonb, attr_schema),
  status      = COALESCE(sqlc.narg('status'), status),
  sort_order  = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListDocumentTypes :many
SELECT * FROM oikumenea.document_document_types
WHERE deleted_at IS NULL
ORDER BY sort_order NULLS LAST, code;

-- ============================ documents (papers) ============================

-- name: InsertDocument :one
INSERT INTO oikumenea.document_documents (
  person_id, type_id, number, issuer, issuing_country_id, issued_on, expires_on, attributes
) VALUES (
  @person_id, @type_id, sqlc.narg('number'), sqlc.narg('issuer'), sqlc.narg('issuing_country_id'),
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
  issuing_country_id = COALESCE(sqlc.narg('issuing_country_id'), issuing_country_id),
  issued_on       = COALESCE(sqlc.narg('issued_on')::date, issued_on),
  expires_on      = COALESCE(sqlc.narg('expires_on')::date, expires_on),
  attributes      = COALESCE(sqlc.narg('attributes')::jsonb, attributes),
  status          = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDocument :execrows
UPDATE oikumenea.document_documents SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- ============================ top-level facet-filtered list (M56 ticket 3 / D-ObjectFacets) ============================
-- GET /documents. Two shapes, ONE filter block, byte-identical between them: the admin path and the
-- holder-scoped path must select the same rows for the same filters, differing ONLY by the
-- visibility predicate. sqlparity_test.go proves the block is present in both with no database.
--
-- document_documents carries NO unit column and NO RLS policy (0005): documents are scoped THROUGH
-- THE HOLDER (D-PersonReadScope). The scoped arm therefore folds the holder semi-join — the person
-- has an active membership in a unit of the subject's reach — rather than a unit predicate. This is
-- the SQL form of the holderReadable() point probe the per-person listing already uses; folded into
-- the query because a Go-side holder check after the page is cut would return a short page WITH a
-- nextPageToken (R-06).
--
-- Metadata only. The pii:basic number/issuer and the pii:special `attributes` bag are returned by the
-- same projection the per-holder listing returns, and neither is filterable (D-ObjectFacets rule 1).

-- name: ListDocuments :many
-- Instance-admin path: every document, keyset-paginated by RID.
SELECT * FROM oikumenea.document_documents d
WHERE d.deleted_at IS NULL
  AND (@after = '' OR d.id::text > @after)
  AND (sqlc.narg('type_id')::uuid IS NULL OR d.type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issuing_country_id')::uuid IS NULL OR d.issuing_country_id = sqlc.narg('issuing_country_id')::uuid)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR d.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR d.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('expires_on_from')::date IS NULL OR d.expires_on >= sqlc.narg('expires_on_from')::date)
  AND (sqlc.narg('expires_on_to')::date IS NULL OR d.expires_on <= sqlc.narg('expires_on_to')::date)
ORDER BY d.id
LIMIT @lim;

-- name: ListDocumentsForSubject :many
-- Read-scope path: the same set restricted to documents whose HOLDER the subject may read. The reach
-- set is UNCORRELATED (it reads only @subject_person_id), so the planner evaluates it once and probes
-- a hash instead of re-deriving the closure per candidate document.
SELECT * FROM oikumenea.document_documents d
WHERE d.deleted_at IS NULL
  AND (@after = '' OR d.id::text > @after)
  AND (sqlc.narg('type_id')::uuid IS NULL OR d.type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issuing_country_id')::uuid IS NULL OR d.issuing_country_id = sqlc.narg('issuing_country_id')::uuid)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR d.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR d.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('expires_on_from')::date IS NULL OR d.expires_on >= sqlc.narg('expires_on_from')::date)
  AND (sqlc.narg('expires_on_to')::date IS NULL OR d.expires_on <= sqlc.narg('expires_on_to')::date)
  AND EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.person_id = d.person_id AND m.status = 'active' AND m.deleted_at IS NULL
      AND m.unit_id IN (SELECT oikumenea.authz_readable_units(@subject_person_id)))
ORDER BY d.id
LIMIT @lim;

-- name: CountReadableUnitsForDispatch :one
-- The capped reach-cardinality probe the sparse/dense list dispatch reads (migration 0017). Capped,
-- because the question is never "how big is the reach" but "is it past the threshold".
SELECT oikumenea.authz_readable_unit_count(@subject_person_id, @cap::integer) AS n;

-- name: ListDocumentsForSubjectDense :many
-- DENSE-reach plan shape of the query above, byte-identical in its filter block and differing ONLY in
-- how the holder's reach is applied: a per-row point probe instead of a materialized reach set. See
-- migration 0017 for the measured reason both shapes exist — at root reach the set form measured
-- 6 419 ms against 4.7 ms here, because materializing the reach makes the planner drive from it,
-- build a 9x10^5-row person hash and top-N sort, so the LIMIT never terminates early. The adapter
-- dispatches on CountReadableUnitsCapped.
SELECT * FROM oikumenea.document_documents d
WHERE d.deleted_at IS NULL
  AND (@after = '' OR d.id::text > @after)
  AND (sqlc.narg('type_id')::uuid IS NULL OR d.type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issuing_country_id')::uuid IS NULL OR d.issuing_country_id = sqlc.narg('issuing_country_id')::uuid)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR d.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR d.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('expires_on_from')::date IS NULL OR d.expires_on >= sqlc.narg('expires_on_from')::date)
  AND (sqlc.narg('expires_on_to')::date IS NULL OR d.expires_on <= sqlc.narg('expires_on_to')::date)
  AND EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.person_id = d.person_id AND m.status = 'active' AND m.deleted_at IS NULL
      AND oikumenea.authz_unit_readable_by(m.unit_id, @subject_person_id))
ORDER BY d.id
LIMIT @lim;

-- ============================ dashboard aggregates (M57) ============================

-- name: DocumentStats :many
-- The INSTANCE-ADMIN dashboard aggregate for the document register (M57 / D-ObjectFacets): the
-- candidate CTE carries ListDocuments' filter block VERBATIM, then one branch per facet, each skipped
-- by the planner when its want_* flag is false. No LIMIT, and so no sparse/dense dispatch.
--
-- expiresOn's (unknown) bucket is the NO-EXPIRY (permanent document) population — a real set, not
-- missing data, which is why the catalog makes the bucket mandatory here.
WITH cand AS MATERIALIZED (
  SELECT d.id, d.type_id, d.status, d.issuing_country_id, d.issued_on, d.expires_on
  FROM oikumenea.document_documents d
  WHERE d.deleted_at IS NULL
  AND (sqlc.narg('type_id')::uuid IS NULL OR d.type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issuing_country_id')::uuid IS NULL OR d.issuing_country_id = sqlc.narg('issuing_country_id')::uuid)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR d.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR d.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('expires_on_from')::date IS NULL OR d.expires_on >= sqlc.narg('expires_on_from')::date)
  AND (sqlc.narg('expires_on_to')::date IS NULL OR d.expires_on <= sqlc.narg('expires_on_to')::date)
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY c.status
UNION ALL
SELECT 'issuingCountryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.issuing_country_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_issuing_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'issuedOn'::text, to_char(date_trunc('month', c.issued_on), 'YYYY-MM'), count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_issued_on')::boolean GROUP BY 2
UNION ALL
SELECT 'expiresOn'::text, to_char(date_trunc('month', c.expires_on), 'YYYY-MM'), count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_expires_on')::boolean GROUP BY 2;

-- name: DocumentStatsForSubject :many
-- The READ-SCOPE arm. Documents carry no unit, so reach goes THROUGH THE HOLDER: the same active-
-- membership semi-join ListDocumentsForSubject uses, folded into the candidate set. An unreadable
-- holder's documents are therefore absent from the count rather than counted and trimmed.
--
-- This is the table whose LIST could not use the materialized reach set at root reach (the LIMIT never
-- terminated early — 6 419 ms). The AGGREGATE has no LIMIT, and re-measuring the holder semi-join both
-- ways confirmed the set form wins here too: 25.7 / 218 / 4 322 ms at leaf / mid / root reach against
-- the point probe's 12 447 / 15 771 / 23 651 ms. So one scoped query, like the other four types.
WITH cand AS MATERIALIZED (
  SELECT d.id, d.type_id, d.status, d.issuing_country_id, d.issued_on, d.expires_on
  FROM oikumenea.document_documents d
  WHERE d.deleted_at IS NULL
  AND (sqlc.narg('type_id')::uuid IS NULL OR d.type_id = sqlc.narg('type_id')::uuid)
  AND (sqlc.narg('status')::text IS NULL OR d.status = sqlc.narg('status')::text)
  AND (sqlc.narg('issuing_country_id')::uuid IS NULL OR d.issuing_country_id = sqlc.narg('issuing_country_id')::uuid)
  AND (sqlc.narg('issued_on_from')::date IS NULL OR d.issued_on >= sqlc.narg('issued_on_from')::date)
  AND (sqlc.narg('issued_on_to')::date IS NULL OR d.issued_on <= sqlc.narg('issued_on_to')::date)
  AND (sqlc.narg('expires_on_from')::date IS NULL OR d.expires_on >= sqlc.narg('expires_on_from')::date)
  AND (sqlc.narg('expires_on_to')::date IS NULL OR d.expires_on <= sqlc.narg('expires_on_to')::date)
  AND EXISTS (
    SELECT 1 FROM oikumenea.membership_memberships m
    WHERE m.person_id = d.person_id AND m.status = 'active' AND m.deleted_at IS NULL
      AND m.unit_id IN (SELECT oikumenea.authz_readable_units(@subject_person_id)))
)
SELECT '(total)'::text AS facet, NULL::text AS bucket, count(*)::bigint AS n, NULL::bigint AS ord
FROM cand
UNION ALL
SELECT 'typeId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.type_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_type_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'status'::text, c.status::text, count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_status')::boolean GROUP BY c.status
UNION ALL
SELECT 'issuingCountryId'::text,
       CASE WHEN t.k IS NULL THEN '(unknown)'
            WHEN t.rk <= sqlc.arg('top_n')::integer THEN t.k
            ELSE '(other)' END,
       sum(t.n)::bigint, NULL::bigint
FROM (SELECT g.k, g.n, row_number() OVER (ORDER BY (g.k IS NULL), g.n DESC, g.k) AS rk
      FROM (SELECT c.issuing_country_id::text AS k, count(*) AS n
            FROM cand c
            WHERE sqlc.arg('want_issuing_country_id')::boolean
            GROUP BY 1) g) t
GROUP BY 2
UNION ALL
SELECT 'issuedOn'::text, to_char(date_trunc('month', c.issued_on), 'YYYY-MM'), count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_issued_on')::boolean GROUP BY 2
UNION ALL
SELECT 'expiresOn'::text, to_char(date_trunc('month', c.expires_on), 'YYYY-MM'), count(*)::bigint, NULL::bigint
FROM cand c WHERE sqlc.arg('want_expires_on')::boolean GROUP BY 2;

-- name: ListDocumentsByPerson :many
SELECT * FROM oikumenea.document_documents
WHERE person_id = @person_id AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
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
  code, country_id, generic_category, name, validation_regex, sort_order
) VALUES (
  @code, sqlc.narg('country_id'), @generic_category, @name, sqlc.narg('validation_regex'),
  sqlc.narg('sort_order')
)
RETURNING *;

-- name: GetScheme :one
SELECT * FROM oikumenea.document_personal_code_schemes WHERE code = @code AND deleted_at IS NULL;

-- name: UpdateScheme :one
UPDATE oikumenea.document_personal_code_schemes SET
  country_id       = COALESCE(sqlc.narg('country_id'), country_id),
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
  AND (@country = '' OR country_id::text = @country)
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
