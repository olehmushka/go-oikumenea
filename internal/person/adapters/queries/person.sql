-- Person module queries (docs/modules/person.md). The directory aggregate (person_persons), its
-- per-person name variants, and the temporal citizenship/residence links. RID PKs default at the
-- database. The reversible deactivate -> purge lifecycle is the PII-erasure path: purge NULLs every
-- PII column and hard-deletes child rows, keeping the id as a tombstone (audit history references it).
-- A NULL narg leaves the stored value unchanged on update (COALESCE); `code` is immutable.

-- ============================ persons ============================

-- name: InsertPerson :one
-- Create a person. attributes defaults to '{}'; country_of_birth/locale are validated by FKs. Rank is
-- NOT set here — a person holds one rank per rank system via person_ranks (UpsertPersonRank, D-Rank).
INSERT INTO oikumenea.person_persons (
  code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth, attributes
) VALUES (
  sqlc.narg('code'), @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), sqlc.narg('birthdate')::date,
  sqlc.narg('date_of_death')::date, @sex,
  sqlc.narg('country_of_birth'), COALESCE(sqlc.narg('attributes')::jsonb, '{}'::jsonb)
)
RETURNING *;

-- name: GetPerson :one
SELECT * FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePerson :one
-- Partial update: a NULL narg leaves the value unchanged. country_of_birth/birthdate/date_of_death
-- cannot be cleared to NULL via this path (open seam). `code` is immutable; rank is set via the
-- person_ranks path (UpsertPersonRank / ClearPersonRank).
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
  date_of_death    = COALESCE(sqlc.narg('date_of_death')::date, date_of_death),
  sex              = COALESCE(sqlc.narg('sex'), sex),
  country_of_birth = COALESCE(sqlc.narg('country_of_birth'), country_of_birth),
  attributes       = COALESCE(sqlc.narg('attributes')::jsonb, attributes)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPersons :many
-- Keyset pagination over the time-ordered RID (an empty cursor starts at the beginning).
SELECT * FROM oikumenea.person_persons
WHERE deleted_at IS NULL AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListPersonsByIDs :many
-- Load the base person rows for a set of RIDs (the D-PersonReadScope directory-list union resolves
-- visible person ids through memberships, then hydrates the rows here). Ordered by RID so the caller
-- can re-key to its keyset order.
SELECT * FROM oikumenea.person_persons
WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL
ORDER BY id;

-- name: UpsertPersonRank :one
-- Set the person's rank in ONE rank system (the HOLDS_RANK link; one rank per system — D-Rank). The
-- system is DERIVED in SQL from the rank (rank_ranks.system_id), so the caller passes only person + rank.
-- Selecting FROM rank_ranks means an unknown/soft-deleted rank yields NO row → the repo maps the empty
-- result to ErrUnknownRank (no FK/not-null ambiguity). On an existing active (person, system) row the
-- rank is replaced; a previously cleared (soft-deleted) row is left and a fresh active row inserted.
INSERT INTO oikumenea.person_ranks (person_id, system_id, rank_id)
SELECT @person_id, r.system_id, r.id
FROM oikumenea.rank_ranks r
WHERE r.id = @rank_id AND r.deleted_at IS NULL
ON CONFLICT (person_id, system_id) WHERE deleted_at IS NULL
  DO UPDATE SET rank_id = excluded.rank_id
RETURNING *;

-- name: ClearPersonRank :exec
-- Clear (soft-delete) the person's active rank in one rank system. No-op when none is held there.
UPDATE oikumenea.person_ranks SET deleted_at = now()
WHERE person_id = @person_id AND system_id = @system_id AND deleted_at IS NULL;

-- name: ListPersonRanks :many
-- The person's active ranks, one per system, ordered by the rank system's sort_order (D-RankSystems).
SELECT pr.* FROM oikumenea.person_ranks pr
JOIN oikumenea.rank_systems s ON s.id = pr.system_id
WHERE pr.person_id = @person_id AND pr.deleted_at IS NULL
ORDER BY s.sort_order, pr.system_id;

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
-- Child rows (incl. person_ranks) are removed separately (DeleteAll*).
UPDATE oikumenea.person_persons SET
  code = NULL, display_name = '', title = NULL, given = NULL, given2 = NULL,
  surname = NULL, surname_prefix = NULL, surname2 = NULL, generation = NULL,
  credentials = NULL, preferred = NULL, birthdate = NULL, date_of_death = NULL, sex = 'not_known',
  country_of_birth = NULL, attributes = '{}'::jsonb,
  status = 'purged', deactivated_at = NULL, purge_after = NULL
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAllPersonRanks :exec
DELETE FROM oikumenea.person_ranks WHERE person_id = @person_id;

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

-- ============================ platform catalog (D-PersonSocialChannels) ============================

-- name: ListPlatforms :many
SELECT * FROM oikumenea.person_platforms WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: GetPlatform :one
-- Resolve one platform by code (used to enforce the category='messenger' rule on a messenger link).
SELECT * FROM oikumenea.person_platforms WHERE code = @code AND deleted_at IS NULL;

-- ============================ messenger links (D-PersonSocialChannels, layer a) ============================

-- name: PhonePersonID :one
-- The owning person of a contact phone (holder-scope check for a messenger link). ErrNoRows when the
-- phone is missing or soft-deleted.
SELECT person_id FROM oikumenea.person_phones WHERE id = @id AND deleted_at IS NULL;

-- name: EmailPersonID :one
SELECT person_id FROM oikumenea.person_emails WHERE id = @id AND deleted_at IS NULL;

-- name: InsertMessengerLink :one
-- Exactly one of phone_id/email_id is set (XOR CHECK). platform_code's category='messenger' is enforced
-- in the application; the FK only checks existence. The partial-unique index dedupes (channel, platform).
INSERT INTO oikumenea.person_messenger_links (phone_id, email_id, platform_code, is_primary, verified_at)
VALUES (sqlc.narg('phone_id'), sqlc.narg('email_id'), @platform_code, @is_primary, sqlc.narg('verified_at'))
RETURNING *;

-- name: UpdateMessengerLink :one
UPDATE oikumenea.person_messenger_links SET
  phone_id = sqlc.narg('phone_id'), email_id = sqlc.narg('email_id'),
  platform_code = @platform_code, is_primary = @is_primary, verified_at = sqlc.narg('verified_at')
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ClearPrimaryMessengerLinks :exec
-- Demote every active primary messenger link the person reaches through any of their phones/emails.
UPDATE oikumenea.person_messenger_links SET is_primary = false
WHERE deleted_at IS NULL AND is_primary
  AND (phone_id IN (SELECT ph.id FROM oikumenea.person_phones ph WHERE ph.person_id = @person_id)
    OR email_id IN (SELECT em.id FROM oikumenea.person_emails em WHERE em.person_id = @person_id));

-- name: DeleteMessengerLink :one
-- Soft-delete a messenger link, holder-scoped: it must reach the person through its phone/email.
UPDATE oikumenea.person_messenger_links ml SET deleted_at = now()
WHERE ml.id = @id AND ml.deleted_at IS NULL
  AND (ml.phone_id IN (SELECT ph.id FROM oikumenea.person_phones ph WHERE ph.person_id = @person_id)
    OR ml.email_id IN (SELECT em.id FROM oikumenea.person_emails em WHERE em.person_id = @person_id))
RETURNING ml.id;

-- name: ListMessengerLinks :many
-- A person's messenger links, resolved through the owning phone/email.
SELECT ml.* FROM oikumenea.person_messenger_links ml
LEFT JOIN oikumenea.person_phones ph ON ml.phone_id = ph.id
LEFT JOIN oikumenea.person_emails em ON ml.email_id = em.id
WHERE ml.deleted_at IS NULL AND COALESCE(ph.person_id, em.person_id) = @person_id
ORDER BY ml.is_primary DESC, ml.id;

-- ============================ social accounts (D-PersonSocialChannels, layer b) ============================

-- name: InsertSocialAccount :one
INSERT INTO oikumenea.person_social_accounts (
  person_id, platform_code, platform_user_id, handle, display_name, profile_url, language,
  platform_verified, verified_by_operator_at, source, confidence, is_primary
) VALUES (
  @person_id, @platform_code, sqlc.narg('platform_user_id'), @handle, sqlc.narg('display_name'),
  sqlc.narg('profile_url'), sqlc.narg('language'), @platform_verified,
  sqlc.narg('verified_by_operator_at'), @source, @confidence, @is_primary
)
RETURNING *;

-- name: UpdateSocialAccount :one
UPDATE oikumenea.person_social_accounts SET
  platform_code = @platform_code, platform_user_id = sqlc.narg('platform_user_id'), handle = @handle,
  display_name = sqlc.narg('display_name'), profile_url = sqlc.narg('profile_url'),
  language = sqlc.narg('language'), platform_verified = @platform_verified,
  verified_by_operator_at = sqlc.narg('verified_by_operator_at'), source = @source,
  confidence = @confidence, is_primary = @is_primary
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: GetSocialAccount :one
SELECT * FROM oikumenea.person_social_accounts
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: ClearPrimarySocialAccounts :exec
UPDATE oikumenea.person_social_accounts SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeleteSocialAccount :one
UPDATE oikumenea.person_social_accounts SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListSocialAccounts :many
SELECT * FROM oikumenea.person_social_accounts
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY is_primary DESC, platform_code, id;

-- ============================ social account handle history ============================

-- name: InsertSocialAccountHandle :one
INSERT INTO oikumenea.person_social_account_handles (account_id, handle, valid_from, valid_to)
VALUES (@account_id, @handle, @valid_from, sqlc.narg('valid_to'))
RETURNING *;

-- name: CloseCurrentSocialAccountHandle :exec
-- Close the open (valid_to IS NULL) handle period for an account at now() on rename.
UPDATE oikumenea.person_social_account_handles SET valid_to = now()
WHERE account_id = @account_id AND valid_to IS NULL AND deleted_at IS NULL;

-- name: ListSocialAccountHandles :many
SELECT * FROM oikumenea.person_social_account_handles
WHERE account_id = @account_id AND deleted_at IS NULL ORDER BY valid_from DESC, id;

-- ============================ purge erasure (extends PurgePerson) ============================

-- name: DeleteAllEmails :exec
DELETE FROM oikumenea.person_emails WHERE person_id = @person_id;

-- name: DeleteAllPhones :exec
DELETE FROM oikumenea.person_phones WHERE person_id = @person_id;

-- name: DeleteAllCallSigns :exec
DELETE FROM oikumenea.person_call_signs WHERE person_id = @person_id;

-- name: DeleteAllMessengerLinks :exec
-- Erase the person's messenger links (D-PersonSocialChannels). They also CASCADE when their phone/email
-- is hard-deleted, but this makes the purge erasure order-independent and explicit.
DELETE FROM oikumenea.person_messenger_links
WHERE phone_id IN (SELECT ph.id FROM oikumenea.person_phones ph WHERE ph.person_id = @person_id)
   OR email_id IN (SELECT em.id FROM oikumenea.person_emails em WHERE em.person_id = @person_id);

-- name: DeleteAllSocialAccountHandles :exec
-- Erase the rename history of all the person's social accounts (handles also CASCADE from the account).
DELETE FROM oikumenea.person_social_account_handles
WHERE account_id IN (SELECT id FROM oikumenea.person_social_accounts WHERE person_id = @person_id);

-- name: DeleteAllSocialAccounts :exec
-- Erase the person's social accounts (CASCADE-deletes their handle history). The person row itself is
-- kept as a tombstone, so these are not removed by the person delete — purge must erase them explicitly.
DELETE FROM oikumenea.person_social_accounts WHERE person_id = @person_id;

-- ============================ person↔person relationships (D-PersonRelationships, M14) ============================

-- relation-type catalog ------------------------------------------------------

-- name: ListRelationTypes :many
SELECT * FROM oikumenea.person_relation_types WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: GetRelationType :one
-- Resolve one relation type by code (used to validate the relation_code's category). ErrNoRows when missing.
SELECT * FROM oikumenea.person_relation_types WHERE code = @code AND deleted_at IS NULL;

-- partnerships ----------------------------------------------------------------

-- name: HasActivePartnershipExcept :one
-- Whether the person has any active engaged/married partnership other than except_id (the single-active
-- rule a partial-unique index cannot span both endpoint columns).
SELECT EXISTS (
  SELECT 1 FROM oikumenea.person_partnerships
  -- except_id is "" when inserting a new partnership (no row to exclude); compare as text so the
  -- empty sentinel excludes nothing rather than failing the uuid cast.
  WHERE deleted_at IS NULL AND status IN ('engaged','married') AND id::text <> @except_id
    AND (person_id_a = @person_id OR person_id_b = @person_id)
) AS exists;

-- name: InsertPartnership :one
INSERT INTO oikumenea.person_partnerships (person_id_a, person_id_b, status, effective_from, effective_to)
VALUES (@person_id_a, @person_id_b, @status, sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: UpdatePartnership :one
UPDATE oikumenea.person_partnerships SET
  person_id_a = @person_id_a, person_id_b = @person_id_b, status = @status,
  effective_from = sqlc.narg('effective_from'), effective_to = sqlc.narg('effective_to')
WHERE id = @id AND deleted_at IS NULL AND (person_id_a = @person_id_a OR person_id_b = @person_id_b)
RETURNING *;

-- name: ListPartnerships :many
SELECT * FROM oikumenea.person_partnerships
WHERE deleted_at IS NULL AND (person_id_a = @person_id OR person_id_b = @person_id)
ORDER BY created_at DESC, id;

-- name: DeletePartnership :one
UPDATE oikumenea.person_partnerships SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (person_id_a = @person_id OR person_id_b = @person_id)
RETURNING id;

-- name: DeleteAllPartnerships :exec
DELETE FROM oikumenea.person_partnerships WHERE person_id_a = @person_id OR person_id_b = @person_id;

-- kinships --------------------------------------------------------------------

-- name: InsertKinship :one
INSERT INTO oikumenea.person_kinships (parent_id, child_id, status)
VALUES (@parent_id, @child_id, @status)
RETURNING *;

-- name: UpdateKinship :one
UPDATE oikumenea.person_kinships SET parent_id = @parent_id, child_id = @child_id, status = @status
WHERE id = @id AND deleted_at IS NULL AND (parent_id = @parent_id OR child_id = @child_id)
RETURNING *;

-- name: ListKinships :many
SELECT * FROM oikumenea.person_kinships
WHERE deleted_at IS NULL AND (parent_id = @person_id OR child_id = @person_id)
ORDER BY created_at DESC, id;

-- name: DeleteKinship :one
UPDATE oikumenea.person_kinships SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (parent_id = @person_id OR child_id = @person_id)
RETURNING id;

-- name: DeleteAllKinships :exec
DELETE FROM oikumenea.person_kinships WHERE parent_id = @person_id OR child_id = @person_id;

-- guardianships ---------------------------------------------------------------

-- name: InsertGuardianship :one
INSERT INTO oikumenea.person_guardianships (guardian_id, ward_id, relation_code, status, effective_from, effective_to)
VALUES (@guardian_id, @ward_id, sqlc.narg('relation_code'), @status, sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: UpdateGuardianship :one
UPDATE oikumenea.person_guardianships SET
  guardian_id = @guardian_id, ward_id = @ward_id, relation_code = sqlc.narg('relation_code'),
  status = @status, effective_from = sqlc.narg('effective_from'), effective_to = sqlc.narg('effective_to')
WHERE id = @id AND deleted_at IS NULL AND (guardian_id = @guardian_id OR ward_id = @ward_id)
RETURNING *;

-- name: ListGuardianships :many
SELECT * FROM oikumenea.person_guardianships
WHERE deleted_at IS NULL AND (guardian_id = @person_id OR ward_id = @person_id)
ORDER BY created_at DESC, id;

-- name: DeleteGuardianship :one
UPDATE oikumenea.person_guardianships SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (guardian_id = @person_id OR ward_id = @person_id)
RETURNING id;

-- name: DeleteAllGuardianships :exec
DELETE FROM oikumenea.person_guardianships WHERE guardian_id = @person_id OR ward_id = @person_id;

-- sponsorships ----------------------------------------------------------------

-- name: InsertSponsorship :one
INSERT INTO oikumenea.person_sponsorships (sponsor_id, sponsored_id, relation_code, status, effective_from, effective_to)
VALUES (@sponsor_id, @sponsored_id, @relation_code, @status, sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: UpdateSponsorship :one
UPDATE oikumenea.person_sponsorships SET
  sponsor_id = @sponsor_id, sponsored_id = @sponsored_id, relation_code = @relation_code,
  status = @status, effective_from = sqlc.narg('effective_from'), effective_to = sqlc.narg('effective_to')
WHERE id = @id AND deleted_at IS NULL AND (sponsor_id = @sponsor_id OR sponsored_id = @sponsored_id)
RETURNING *;

-- name: ListSponsorships :many
SELECT * FROM oikumenea.person_sponsorships
WHERE deleted_at IS NULL AND (sponsor_id = @person_id OR sponsored_id = @person_id)
ORDER BY created_at DESC, id;

-- name: DeleteSponsorship :one
UPDATE oikumenea.person_sponsorships SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (sponsor_id = @person_id OR sponsored_id = @person_id)
RETURNING id;

-- name: DeleteAllSponsorships :exec
DELETE FROM oikumenea.person_sponsorships WHERE sponsor_id = @person_id OR sponsored_id = @person_id;

-- next of kin -----------------------------------------------------------------

-- name: InsertNextOfKin :one
INSERT INTO oikumenea.person_next_of_kin (subject_id, contact_id, relation_code, priority, status)
VALUES (@subject_id, @contact_id, sqlc.narg('relation_code'), @priority, @status)
RETURNING *;

-- name: UpdateNextOfKin :one
UPDATE oikumenea.person_next_of_kin SET
  subject_id = @subject_id, contact_id = @contact_id, relation_code = sqlc.narg('relation_code'),
  priority = @priority, status = @status
WHERE id = @id AND deleted_at IS NULL AND (subject_id = @subject_id OR contact_id = @contact_id)
RETURNING *;

-- name: ListNextOfKin :many
SELECT * FROM oikumenea.person_next_of_kin
WHERE deleted_at IS NULL AND (subject_id = @person_id OR contact_id = @person_id)
ORDER BY priority, created_at DESC, id;

-- name: DeleteNextOfKin :one
UPDATE oikumenea.person_next_of_kin SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (subject_id = @person_id OR contact_id = @person_id)
RETURNING id;

-- name: DeleteAllNextOfKin :exec
DELETE FROM oikumenea.person_next_of_kin WHERE subject_id = @person_id OR contact_id = @person_id;

-- associations ----------------------------------------------------------------

-- name: InsertAssociation :one
INSERT INTO oikumenea.person_associations (person_id_a, person_id_b, relation_code, kind, status)
VALUES (@person_id_a, @person_id_b, sqlc.narg('relation_code'), @kind, @status)
RETURNING *;

-- name: UpdateAssociation :one
UPDATE oikumenea.person_associations SET
  person_id_a = @person_id_a, person_id_b = @person_id_b, relation_code = sqlc.narg('relation_code'),
  kind = @kind, status = @status
WHERE id = @id AND deleted_at IS NULL AND (person_id_a = @person_id_a OR person_id_b = @person_id_b)
RETURNING *;

-- name: ListAssociations :many
SELECT * FROM oikumenea.person_associations
WHERE deleted_at IS NULL AND (person_id_a = @person_id OR person_id_b = @person_id)
ORDER BY created_at DESC, id;

-- name: DeleteAssociation :one
UPDATE oikumenea.person_associations SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL AND (person_id_a = @person_id OR person_id_b = @person_id)
RETURNING id;

-- name: DeleteAllAssociations :exec
DELETE FROM oikumenea.person_associations WHERE person_id_a = @person_id OR person_id_b = @person_id;
