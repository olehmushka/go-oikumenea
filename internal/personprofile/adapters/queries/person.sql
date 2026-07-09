-- personprofile module queries (docs/modules/personprofile.md, D-PersonModuleSplit / review-2026-07 R-09).
-- The person directory's non-encrypted, person-owned data: citizenships, residences, addresses, the
-- contact channels (email/phone/call-sign/messenger/social), SPEAKS languages, person<->person
-- relationships, and the non-encrypted institutional ties. Split out of the person god-module's query
-- surface; person_persons/person_ranks/person_name_variants stay core-owned (queries/person.sql).

-- name: DeleteAllCitizenships :exec
DELETE FROM oikumenea.person_citizenships WHERE person_id = @person_id;

-- name: DeleteAllResidences :exec
DELETE FROM oikumenea.person_residences WHERE person_id = @person_id;

-- ============================ name variants ============================

-- name: UpsertCitizenship :one
-- Add or replace the ACTIVE citizenship for (person, country) via the partial unique index. The
-- geo_countries FK validates the country.
INSERT INTO oikumenea.person_citizenships (person_id, country_id, basis, acquired_on, lost_on, is_primary)
VALUES (@person_id, @country_id, @basis, sqlc.narg('acquired_on')::date, sqlc.narg('lost_on')::date, @is_primary)
ON CONFLICT (person_id, country_id) WHERE lost_on IS NULL AND deleted_at IS NULL DO UPDATE SET
  basis = excluded.basis, acquired_on = excluded.acquired_on, lost_on = excluded.lost_on,
  is_primary = excluded.is_primary
RETURNING *;

-- name: ClearPrimaryCitizenships :exec
UPDATE oikumenea.person_citizenships SET is_primary = false
WHERE person_id = @person_id AND deleted_at IS NULL AND is_primary;

-- name: DeleteCitizenship :one
-- Soft-delete the active citizenship for a country.
UPDATE oikumenea.person_citizenships SET deleted_at = now()
WHERE person_id = @person_id AND country_id = @country_id AND deleted_at IS NULL
RETURNING id;

-- name: ListCitizenships :many
SELECT * FROM oikumenea.person_citizenships
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY country_id;

-- ============================ residences ============================

-- name: InsertResidence :one
INSERT INTO oikumenea.person_residences (person_id, country_id, region, valid_from, valid_to)
VALUES (@person_id, @country_id, sqlc.narg('region'), @valid_from::date, sqlc.narg('valid_to')::date)
RETURNING *;

-- name: UpdateResidence :one
UPDATE oikumenea.person_residences SET
  country_id = @country_id, region = sqlc.narg('region'),
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
-- The phone's country is DERIVED from the number (libphonenumber, application layer) as an ISO-3166-1
-- alpha-2 code, then resolved to the country's RID here (the geo registry is RID-keyed); an absent /
-- unresolvable region folds to NULL.
INSERT INTO oikumenea.person_phones (person_id, type_code, number, country_id, is_primary)
VALUES (@person_id, @type_code, @number,
        (SELECT gc.id FROM oikumenea.geo_countries gc WHERE gc.code = NULLIF(sqlc.narg('country_code')::text, '')),
        @is_primary)
RETURNING *;

-- name: UpdatePhone :one
UPDATE oikumenea.person_phones SET
  type_code = @type_code, number = @number,
  country_id = (SELECT gc.id FROM oikumenea.geo_countries gc WHERE gc.code = NULLIF(sqlc.narg('country_code')::text, '')),
  is_primary = @is_primary
WHERE person_phones.id = @id AND person_id = @person_id AND deleted_at IS NULL
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

-- name: DeleteAllPersonLanguages :exec
-- Erase the person's SPEAKS links (D-Languages, M18). person_languages is pii:basic, so purge must
-- hard-delete it explicitly (the person row is kept as a tombstone).
DELETE FROM oikumenea.person_languages WHERE person_id = @person_id;

-- NOTE (D-PersonModuleSplit, review-2026-07 R-09 / R-08): the education (person_education_*,
-- person_dormitory_stays, and the M20 reference-layer person links) and company (company_* +
-- polymorphic person-holder) rows are NO LONGER erased from here. person publishes PersonPurged and each
-- owning module erases its own rows via personevents.SubscribeErase in the same transaction — person no
-- longer writes other modules' tables. See internal/{education,company}/application/person_merge.go.

-- ============================ person languages (D-Languages, M18) — SPEAKS ============================

-- name: ListPersonLanguages :many
-- The person's spoken languages joined to the languoid for its default-locale display name (the
-- transport assembles the locale->text map). Native first, then by name.
SELECT pl.id, pl.person_id, pl.language_id, pl.cefr_level, pl.is_native, l.name AS language_name
FROM oikumenea.person_languages pl
JOIN oikumenea.language_languoids l ON l.id = pl.language_id
WHERE pl.person_id = @person_id AND pl.deleted_at IS NULL
ORDER BY pl.is_native DESC, l.name, pl.id;

-- name: GetPersonLanguage :one
SELECT pl.id, pl.person_id, pl.language_id, pl.cefr_level, pl.is_native, l.name AS language_name
FROM oikumenea.person_languages pl
JOIN oikumenea.language_languoids l ON l.id = pl.language_id
WHERE pl.person_id = @person_id AND pl.language_id = @language_id AND pl.deleted_at IS NULL;

-- name: InsertPersonLanguage :exec
-- language_level defaults to 'language'; the composite FK (language_id, language_level) ->
-- language_languoids(id, level) rejects a non-language languoid (23503 -> ErrUnknownLanguage).
INSERT INTO oikumenea.person_languages (person_id, language_id, cefr_level, is_native)
VALUES (@person_id, @language_id, sqlc.narg('cefr_level'), @is_native);

-- name: UpdatePersonLanguage :exec
UPDATE oikumenea.person_languages
SET cefr_level = sqlc.narg('cefr_level'), is_native = @is_native
WHERE person_id = @person_id AND language_id = @language_id AND deleted_at IS NULL;

-- name: DeletePersonLanguage :one
UPDATE oikumenea.person_languages SET deleted_at = now()
WHERE person_id = @person_id AND language_id = @language_id AND deleted_at IS NULL
RETURNING id;

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
INSERT INTO oikumenea.person_sponsorships (sponsor_id, sponsored_id, relation_code, status, effective_from, effective_to, enrollment_id, education_role)
VALUES (@sponsor_id, @sponsored_id, @relation_code, @status, sqlc.narg('effective_from'), sqlc.narg('effective_to'), sqlc.narg('enrollment_id'), sqlc.narg('education_role'))
RETURNING *;

-- name: UpdateSponsorship :one
UPDATE oikumenea.person_sponsorships SET
  sponsor_id = @sponsor_id, sponsored_id = @sponsored_id, relation_code = @relation_code,
  status = @status, effective_from = sqlc.narg('effective_from'), effective_to = sqlc.narg('effective_to'),
  enrollment_id = sqlc.narg('enrollment_id'), education_role = sqlc.narg('education_role')
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

-- ============================ physical descriptions (D-PhysicalIdentity, M31) ============================

-- name: InsertAddress :one
INSERT INTO oikumenea.person_addresses (
  person_id, location_id, role, valid_from, valid_to, is_primary, privacy_seeking, source, confidence
) VALUES (
  @person_id, @location_id, @role, @valid_from::date, sqlc.narg('valid_to')::date,
  @is_primary, @privacy_seeking, @source, @confidence
)
RETURNING *;

-- name: UpdateAddress :one
UPDATE oikumenea.person_addresses SET
  location_id = @location_id, role = @role,
  valid_from = @valid_from::date, valid_to = sqlc.narg('valid_to')::date,
  is_primary = @is_primary, privacy_seeking = @privacy_seeking,
  source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteAddress :one
UPDATE oikumenea.person_addresses SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListAddresses :many
SELECT * FROM oikumenea.person_addresses
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY is_primary DESC, valid_from DESC, id;

-- name: DemotePrimaryAddresses :exec
-- Clear is_primary on a person's other active addresses so at most one primary survives (the caller
-- then sets the target row primary). exceptId = "" demotes every active primary.
UPDATE oikumenea.person_addresses SET is_primary = false
WHERE person_id = @person_id AND is_primary AND deleted_at IS NULL
  AND (@except_id::text = '' OR id::text <> @except_id::text);

-- name: DeleteAllAddresses :exec
DELETE FROM oikumenea.person_addresses WHERE person_id = @person_id;

-- ============================ ethnicity-type catalog (D-PhysicalIdentity, M31) ============================

-- name: InsertGovernmentPosition :one
INSERT INTO oikumenea.person_government_positions (
  person_id, title, body, org_id, country_id, level, role_type,
  valid_from, valid_to, pep_trigger, source, confidence
) VALUES (
  @person_id, @title, @body, sqlc.narg('org_id')::uuid, sqlc.narg('country_id')::uuid, @level,
  sqlc.narg('role_type'), sqlc.narg('valid_from')::date, sqlc.narg('valid_to')::date,
  @pep_trigger, @source, @confidence
)
RETURNING *;

-- name: UpdateGovernmentPosition :one
UPDATE oikumenea.person_government_positions SET
  title = @title, body = @body, org_id = sqlc.narg('org_id')::uuid, country_id = sqlc.narg('country_id')::uuid,
  level = @level, role_type = sqlc.narg('role_type'),
  valid_from = sqlc.narg('valid_from')::date, valid_to = sqlc.narg('valid_to')::date,
  pep_trigger = @pep_trigger, source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteGovernmentPosition :one
UPDATE oikumenea.person_government_positions SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListGovernmentPositions :many
SELECT * FROM oikumenea.person_government_positions
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY valid_from DESC NULLS LAST, created_at DESC, id;

-- name: CountActivePEPPositions :one
-- PEP derivation (D-Watchlists, M34): a person is politically exposed if any active pep_trigger position exists.
SELECT count(*) FROM oikumenea.person_government_positions
WHERE person_id = @person_id AND pep_trigger AND deleted_at IS NULL;

-- name: DeleteAllGovernmentPositions :exec
DELETE FROM oikumenea.person_government_positions WHERE person_id = @person_id;

-- ---- lobbying relationships (link__lobbying_rel, pii:basic) ----

-- name: InsertLobbyingRelationship :one
INSERT INTO oikumenea.person_lobbying_relationships (
  person_id, registrant, client, legislative_body, issues, filing_id, source_url,
  valid_from, valid_to, source, confidence
) VALUES (
  @person_id, @registrant, sqlc.narg('client'), sqlc.narg('legislative_body'), @issues,
  sqlc.narg('filing_id'), sqlc.narg('source_url'), sqlc.narg('valid_from')::date, sqlc.narg('valid_to')::date,
  @source, @confidence
)
RETURNING *;

-- name: UpdateLobbyingRelationship :one
UPDATE oikumenea.person_lobbying_relationships SET
  registrant = @registrant, client = sqlc.narg('client'), legislative_body = sqlc.narg('legislative_body'),
  issues = @issues, filing_id = sqlc.narg('filing_id'), source_url = sqlc.narg('source_url'),
  valid_from = sqlc.narg('valid_from')::date, valid_to = sqlc.narg('valid_to')::date,
  source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteLobbyingRelationship :one
UPDATE oikumenea.person_lobbying_relationships SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListLobbyingRelationships :many
SELECT * FROM oikumenea.person_lobbying_relationships
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY valid_from DESC NULLS LAST, created_at DESC, id;

-- name: DeleteAllLobbyingRelationships :exec
DELETE FROM oikumenea.person_lobbying_relationships WHERE person_id = @person_id;

-- ---- external references (object external_reference, pii:basic; a hermenea import target) ----

-- name: UpsertExternalReference :one
-- Idempotent by (person, url): a re-imported reference updates in place rather than duplicating. Edits by
-- RID use UpdateExternalReference.
INSERT INTO oikumenea.person_external_references (
  person_id, kind, url, external_id, categories, last_checked, disputed, source, confidence
) VALUES (
  @person_id, @kind, @url, sqlc.narg('external_id'), @categories, sqlc.narg('last_checked'),
  @disputed, @source, @confidence
)
ON CONFLICT (person_id, url) WHERE deleted_at IS NULL DO UPDATE SET
  kind = excluded.kind, external_id = excluded.external_id, categories = excluded.categories,
  last_checked = excluded.last_checked, disputed = excluded.disputed,
  source = excluded.source, confidence = excluded.confidence
RETURNING *;

-- name: UpdateExternalReference :one
UPDATE oikumenea.person_external_references SET
  kind = @kind, url = @url, external_id = sqlc.narg('external_id'), categories = @categories,
  last_checked = sqlc.narg('last_checked'), disputed = @disputed, source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteExternalReference :one
UPDATE oikumenea.person_external_references SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListExternalReferences :many
SELECT * FROM oikumenea.person_external_references
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: DeleteAllExternalReferences :exec
DELETE FROM oikumenea.person_external_references WHERE person_id = @person_id;

-- ---- watchlist match (object watchlist_match, pii:sensitive; D-Watchlists M34) ----


-- ============================ person existence (parent guard) ============================

-- name: PersonExists :one
-- Parent-existence guard: personprofile child writes/reads verify the person exists (and is not
-- soft-deleted) before touching its directory rows. A reviewed cross-module read of the person core's
-- aggregate table (personprofile:person allowlist), mirroring the core GetPerson visibility predicate.
SELECT 1 FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;
