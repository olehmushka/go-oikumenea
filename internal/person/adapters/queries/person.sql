-- Person module queries (docs/modules/person.md). The directory aggregate (person_persons), its
-- per-person name variants, and the temporal citizenship/residence links. RID PKs default at the
-- database. The reversible deactivate -> purge lifecycle is the PII-erasure path: purge NULLs every
-- PII column and hard-deletes child rows, keeping the id as a tombstone (audit history references it).
-- A NULL narg leaves the stored value unchanged on update (COALESCE); `code` is immutable.

-- ============================ persons ============================

-- name: InsertPerson :one
-- Create a person. attributes defaults to '{}'; rank_id/country_of_birth/locale are validated by FKs.
INSERT INTO oikumenea.person_persons (
  code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, sex, country_of_birth, attributes, rank_id
) VALUES (
  sqlc.narg('code'), @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), sqlc.narg('birthdate')::date, @sex,
  sqlc.narg('country_of_birth'), COALESCE(sqlc.narg('attributes')::jsonb, '{}'::jsonb),
  sqlc.narg('rank_id')
)
RETURNING *;

-- name: GetPerson :one
SELECT * FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePerson :one
-- Partial update: a NULL narg leaves the value unchanged. country_of_birth/birthdate cannot be
-- cleared to NULL via this path (open seam). `code` is immutable; rank is set via SetRank.
UPDATE oikumenea.person_persons SET
  display_name     = COALESCE(sqlc.narg('display_name'), display_name),
  title            = COALESCE(sqlc.narg('title'), title),
  given            = COALESCE(sqlc.narg('given'), given),
  given2           = COALESCE(sqlc.narg('given2'), given2),
  surname          = COALESCE(sqlc.narg('surname'), surname),
  surname_prefix   = COALESCE(sqlc.narg('surname_prefix'), surname_prefix),
  surname2         = COALESCE(sqlc.narg('surname2'), surname2),
  generation       = COALESCE(sqlc.narg('generation'), generation),
  credentials      = COALESCE(sqlc.narg('credentials'), credentials),
  preferred        = COALESCE(sqlc.narg('preferred'), preferred),
  birthdate        = COALESCE(sqlc.narg('birthdate')::date, birthdate),
  sex              = COALESCE(sqlc.narg('sex'), sex),
  country_of_birth = COALESCE(sqlc.narg('country_of_birth'), country_of_birth),
  attributes       = COALESCE(sqlc.narg('attributes')::jsonb, attributes)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPersons :many
-- Keyset pagination over the time-ordered RID (an empty cursor starts at the beginning).
SELECT * FROM oikumenea.person_persons
WHERE deleted_at IS NULL AND (@after = '' OR id > @after)
ORDER BY id
LIMIT @lim;

-- name: SetRank :one
-- Set or clear the person's one rank; a NULL rank_id clears it. The rank_ranks FK validates existence.
UPDATE oikumenea.person_persons SET rank_id = sqlc.narg('rank_id')
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeactivatePerson :one
UPDATE oikumenea.person_persons
SET status = 'deactivated', deactivated_at = now(), purge_after = @purge_after
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ReactivatePerson :one
UPDATE oikumenea.person_persons
SET status = 'active', deactivated_at = NULL, purge_after = NULL
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: PurgePerson :one
-- Hard-erase PII: NULL every pii:basic/contact column, reset sex/attributes, keep the id tombstone.
-- Child rows are removed separately (DeleteAll*). rank_id is pii:none and left as-is.
UPDATE oikumenea.person_persons SET
  code = NULL, display_name = '', title = NULL, given = NULL, given2 = NULL,
  surname = NULL, surname_prefix = NULL, surname2 = NULL, generation = NULL,
  credentials = NULL, preferred = NULL, birthdate = NULL, sex = 'not_known',
  country_of_birth = NULL, attributes = '{}'::jsonb,
  status = 'purged', deactivated_at = NULL, purge_after = NULL
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAllNameVariants :exec
DELETE FROM oikumenea.person_name_variants WHERE person_id = @person_id;

-- name: DeleteAllCitizenships :exec
DELETE FROM oikumenea.person_citizenships WHERE person_id = @person_id;

-- name: DeleteAllResidences :exec
DELETE FROM oikumenea.person_residences WHERE person_id = @person_id;

-- ============================ name variants ============================

-- name: UpsertNameVariant :one
-- Add or replace the variant for (person, locale). The i18n_locales FK validates the locale.
INSERT INTO oikumenea.person_name_variants (
  person_id, locale, display_name, title, given, given2, surname, surname_prefix,
  surname2, generation, credentials, preferred, is_primary
) VALUES (
  @person_id, @locale, @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), @is_primary
)
ON CONFLICT (person_id, locale) DO UPDATE SET
  display_name = excluded.display_name, title = excluded.title, given = excluded.given,
  given2 = excluded.given2, surname = excluded.surname, surname_prefix = excluded.surname_prefix,
  surname2 = excluded.surname2, generation = excluded.generation, credentials = excluded.credentials,
  preferred = excluded.preferred, is_primary = excluded.is_primary
RETURNING *;

-- name: ClearPrimaryNameVariants :exec
UPDATE oikumenea.person_name_variants SET is_primary = false
WHERE person_id = @person_id AND is_primary;

-- name: DeleteNameVariant :one
DELETE FROM oikumenea.person_name_variants WHERE person_id = @person_id AND locale = @locale
RETURNING id;

-- name: ListNameVariants :many
SELECT * FROM oikumenea.person_name_variants WHERE person_id = @person_id ORDER BY locale;

-- ============================ citizenships ============================

-- name: UpsertCitizenship :one
-- Add or replace the ACTIVE citizenship for (person, country) via the partial unique index. The
-- geo_countries FK validates the country.
INSERT INTO oikumenea.person_citizenships (person_id, country, basis, acquired_on, lost_on, is_primary)
VALUES (@person_id, @country, @basis, sqlc.narg('acquired_on')::date, sqlc.narg('lost_on')::date, @is_primary)
ON CONFLICT (person_id, country) WHERE lost_on IS NULL AND deleted_at IS NULL DO UPDATE SET
  basis = excluded.basis, acquired_on = excluded.acquired_on, lost_on = excluded.lost_on,
  is_primary = excluded.is_primary
RETURNING *;

-- name: ClearPrimaryCitizenships :exec
UPDATE oikumenea.person_citizenships SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeleteCitizenship :one
-- Soft-delete the active citizenship for a country.
UPDATE oikumenea.person_citizenships SET deleted_at = now()
WHERE person_id = @person_id AND country = @country AND deleted_at IS NULL
RETURNING id;

-- name: ListCitizenships :many
SELECT * FROM oikumenea.person_citizenships
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY country;

-- ============================ residences ============================

-- name: InsertResidence :one
INSERT INTO oikumenea.person_residences (person_id, country, region, valid_from, valid_to)
VALUES (@person_id, @country, sqlc.narg('region'), @valid_from::date, sqlc.narg('valid_to')::date)
RETURNING *;

-- name: UpdateResidence :one
UPDATE oikumenea.person_residences SET
  country = @country, region = sqlc.narg('region'),
  valid_from = @valid_from::date, valid_to = sqlc.narg('valid_to')::date
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteResidence :one
UPDATE oikumenea.person_residences SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListResidences :many
SELECT * FROM oikumenea.person_residences
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY valid_from DESC, id;

-- name: GetActivePersonByCode :one
-- Look up an active person by their stable `code` (D-Code). Used by identity-federation for
-- just-in-time link-on-match (D-JIT) and the first-admin bootstrap's find-or-create (D-Bootstrap).
SELECT * FROM oikumenea.person_persons
WHERE code = @code AND status = 'active' AND deleted_at IS NULL;

-- ============================ emails (D-PersonContactChannels) ============================

-- name: InsertEmail :one
-- address is citext (case-insensitive); provider is derived in the application before insert. The
-- person_email_types FK validates type_code; the partial unique index dedupes active (person, address).
INSERT INTO oikumenea.person_emails (person_id, type_code, address, provider, is_primary)
VALUES (@person_id, @type_code, @address, sqlc.narg('provider'), @is_primary)
RETURNING *;

-- name: UpdateEmail :one
UPDATE oikumenea.person_emails SET
  type_code = @type_code, address = @address, provider = sqlc.narg('provider'), is_primary = @is_primary
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ClearPrimaryEmails :exec
UPDATE oikumenea.person_emails SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeleteEmail :one
UPDATE oikumenea.person_emails SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListEmails :many
SELECT * FROM oikumenea.person_emails
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY is_primary DESC, address;

-- ============================ phones (D-PersonContactChannels) ============================

-- name: InsertPhone :one
-- number is E.164-normalized and country derived in the application before insert. The
-- person_phone_types FK validates type_code; geo_countries FK validates the derived country.
INSERT INTO oikumenea.person_phones (person_id, type_code, number, country, is_primary)
VALUES (@person_id, @type_code, @number, sqlc.narg('country'), @is_primary)
RETURNING *;

-- name: UpdatePhone :one
UPDATE oikumenea.person_phones SET
  type_code = @type_code, number = @number, country = sqlc.narg('country'), is_primary = @is_primary
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ClearPrimaryPhones :exec
UPDATE oikumenea.person_phones SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeletePhone :one
UPDATE oikumenea.person_phones SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListPhones :many
SELECT * FROM oikumenea.person_phones
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY is_primary DESC, number;

-- ============================ call signs (D-PersonContactChannels) ============================

-- name: InsertCallSign :one
-- call_sign is required (NOT NULL) and unique per person among active rows.
INSERT INTO oikumenea.person_call_signs (person_id, call_sign, is_primary)
VALUES (@person_id, @call_sign, @is_primary)
RETURNING *;

-- name: UpdateCallSign :one
UPDATE oikumenea.person_call_signs SET
  call_sign = @call_sign, is_primary = @is_primary
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ClearPrimaryCallSigns :exec
UPDATE oikumenea.person_call_signs SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeleteCallSign :one
UPDATE oikumenea.person_call_signs SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListCallSigns :many
SELECT * FROM oikumenea.person_call_signs
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY is_primary DESC, id;

-- ============================ contact-kind catalogs ============================

-- name: ListEmailTypes :many
SELECT * FROM oikumenea.person_email_types WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: ListPhoneTypes :many
SELECT * FROM oikumenea.person_phone_types WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- ============================ purge erasure (extends PurgePerson) ============================

-- name: DeleteAllEmails :exec
DELETE FROM oikumenea.person_emails WHERE person_id = @person_id;

-- name: DeleteAllPhones :exec
DELETE FROM oikumenea.person_phones WHERE person_id = @person_id;

-- name: DeleteAllCallSigns :exec
DELETE FROM oikumenea.person_call_signs WHERE person_id = @person_id;
