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
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id, attributes
) VALUES (
  sqlc.narg('code'), @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), sqlc.narg('birthdate')::date,
  sqlc.narg('date_of_death')::date, @sex,
  sqlc.narg('country_of_birth_id'), COALESCE(sqlc.narg('attributes')::jsonb, '{}'::jsonb)
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
  country_of_birth_id = COALESCE(sqlc.narg('country_of_birth_id'), country_of_birth_id),
  attributes       = COALESCE(sqlc.narg('attributes')::jsonb, attributes)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPersons :many
-- Keyset pagination over the time-ordered RID (an empty cursor starts at the beginning). An optional
-- case-insensitive @query narrows by display name / code / given / surname (the filter sits in the
-- same WHERE so the keyset cursor stays correct).
SELECT * FROM oikumenea.person_persons
WHERE deleted_at IS NULL AND (@after = '' OR id::text > @after)
  AND (@query = '' OR display_name ILIKE '%' || @query || '%' OR code ILIKE '%' || @query || '%'
       OR given ILIKE '%' || @query || '%' OR surname ILIKE '%' || @query || '%')
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
  country_of_birth_id = NULL, attributes = '{}'::jsonb,
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
-- Add or replace the canonical TRANSLITERATION variant for (person, locale) — the original one-per-locale
-- name form (D-PersonNamesCLDR). The i18n_locales FK validates the locale. variant_kind is forced to
-- 'transliteration' so the conflict targets the partial unique index; aliases use InsertNameAlias.
INSERT INTO oikumenea.person_name_variants (
  person_id, locale, display_name, title, given, given2, surname, surname_prefix,
  surname2, generation, credentials, preferred, is_primary, variant_kind
) VALUES (
  @person_id, @locale, @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), @is_primary, 'transliteration'
)
ON CONFLICT (person_id, locale) WHERE variant_kind = 'transliteration' DO UPDATE SET
  display_name = excluded.display_name, title = excluded.title, given = excluded.given,
  given2 = excluded.given2, surname = excluded.surname, surname_prefix = excluded.surname_prefix,
  surname2 = excluded.surname2, generation = excluded.generation, credentials = excluded.credentials,
  preferred = excluded.preferred, is_primary = excluded.is_primary
RETURNING *;

-- name: ClearPrimaryNameVariants :exec
-- Demote the person's primary transliteration variant(s) (is_primary marks at most one transliteration).
UPDATE oikumenea.person_name_variants SET is_primary = false
WHERE person_id = @person_id AND is_primary AND variant_kind = 'transliteration';

-- name: DeleteNameVariant :one
-- Delete the TRANSLITERATION variant for a locale (aliases are deleted by id via DeleteNameAlias).
DELETE FROM oikumenea.person_name_variants
WHERE person_id = @person_id AND locale = @locale AND variant_kind = 'transliteration'
RETURNING id;

-- name: ListNameVariants :many
SELECT * FROM oikumenea.person_name_variants WHERE person_id = @person_id
ORDER BY variant_kind, locale, id;

-- name: InsertNameAlias :one
-- Add an ALIAS name form (variant_kind in aka|former_legal|maiden|pseudonym|cover; D-PhysicalIdentity).
-- Aliases are unconstrained per (person, locale) and addressed by their RID; they may carry attribution.
INSERT INTO oikumenea.person_name_variants (
  person_id, locale, display_name, title, given, given2, surname, surname_prefix,
  surname2, generation, credentials, preferred, is_primary, variant_kind, source, confidence
) VALUES (
  @person_id, @locale, @display_name, sqlc.narg('title'), sqlc.narg('given'), sqlc.narg('given2'),
  sqlc.narg('surname'), sqlc.narg('surname_prefix'), sqlc.narg('surname2'), sqlc.narg('generation'),
  sqlc.narg('credentials'), sqlc.narg('preferred'), false, @variant_kind,
  sqlc.narg('source'), sqlc.narg('confidence')
)
RETURNING *;

-- name: DeleteNameAlias :one
-- Delete one alias by its RID (holder-scoped). Refuses the transliteration kind (use DeleteNameVariant).
DELETE FROM oikumenea.person_name_variants
WHERE id = @id AND person_id = @person_id AND variant_kind <> 'transliteration'
RETURNING id;

-- ============================ citizenships ============================

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

-- name: DeleteAllPersonEducationEnrollments :exec
-- Erase the person's STUDIED_AT links (D-Education, M20). Education-owned table; person purge hard-deletes
-- it (pii:basic) — the enrollment_id FK on person_sponsorships is ON DELETE SET NULL.
DELETE FROM oikumenea.person_education_enrollments WHERE person_id = @person_id;

-- name: DeleteAllPersonDormitoryStays :exec
-- Erase the person's RESIDED_IN_DORMITORY links (D-Education, M20; pii:contact).
DELETE FROM oikumenea.person_dormitory_stays WHERE person_id = @person_id;

-- The education reference-layer person↔reference links (D-Education, M20 extension; pii:basic). Each is
-- an education-owned table erased on person purge.
-- name: DeleteAllPersonPublicationAuthorships :exec
DELETE FROM oikumenea.person_publication_authorships WHERE person_id = @person_id;

-- name: DeleteAllPersonResearchMemberships :exec
DELETE FROM oikumenea.person_research_memberships WHERE person_id = @person_id;

-- name: DeleteAllPersonGrantHoldings :exec
DELETE FROM oikumenea.person_grant_holdings WHERE person_id = @person_id;

-- name: DeleteAllPersonGovernanceMemberships :exec
DELETE FROM oikumenea.person_governance_memberships WHERE person_id = @person_id;

-- name: DeleteAllPersonEducationQualifications :exec
DELETE FROM oikumenea.person_education_qualifications WHERE person_id = @person_id;

-- name: DeleteAllPersonScholarshipAwards :exec
DELETE FROM oikumenea.person_scholarship_awards WHERE person_id = @person_id;

-- The company person-link rows (D-Companies, M21; pii:basic). Each is a company-owned table that names
-- this person — erased on person purge: appointments (employment) + beneficiary (UBO) by person_id FK,
-- and the polymorphic person-holder founding/shareholding rows by (holder_kind='person', holder_id).
-- name: DeleteAllCompanyAppointments :exec
DELETE FROM oikumenea.company_appointments WHERE person_id = @person_id;

-- name: DeleteAllCompanyBeneficiariesForPerson :exec
DELETE FROM oikumenea.company_beneficiaries WHERE person_id = @person_id;

-- name: DeleteAllCompanyFoundingsForPerson :exec
DELETE FROM oikumenea.company_foundings WHERE holder_kind = 'person' AND holder_id = @person_id;

-- name: DeleteAllCompanyShareholdingsForPerson :exec
DELETE FROM oikumenea.company_shareholdings WHERE holder_kind = 'person' AND holder_id = @person_id;

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

-- name: InsertPhysicalDescription :one
INSERT INTO oikumenea.person_physical_descriptions (
  person_id, height_cm, weight_kg, eye_color_id, hair_color_id, build, blood_type,
  effective_from, effective_to, source, confidence
) VALUES (
  @person_id, sqlc.narg('height_cm'), sqlc.narg('weight_kg'), sqlc.narg('eye_color_id')::uuid,
  sqlc.narg('hair_color_id')::uuid, sqlc.narg('build'), sqlc.narg('blood_type'),
  COALESCE(sqlc.narg('effective_from')::date, (now() AT TIME ZONE 'UTC')::date), sqlc.narg('effective_to')::date,
  sqlc.narg('source'), sqlc.narg('confidence')
)
RETURNING *;

-- name: UpdatePhysicalDescription :one
UPDATE oikumenea.person_physical_descriptions SET
  height_cm = sqlc.narg('height_cm'), weight_kg = sqlc.narg('weight_kg'),
  eye_color_id = sqlc.narg('eye_color_id')::uuid, hair_color_id = sqlc.narg('hair_color_id')::uuid,
  build = sqlc.narg('build'), blood_type = sqlc.narg('blood_type'),
  effective_from = COALESCE(sqlc.narg('effective_from')::date, effective_from),
  effective_to = sqlc.narg('effective_to')::date,
  source = sqlc.narg('source'), confidence = sqlc.narg('confidence')
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeletePhysicalDescription :one
UPDATE oikumenea.person_physical_descriptions SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListPhysicalDescriptions :many
SELECT * FROM oikumenea.person_physical_descriptions
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY effective_from DESC, id;

-- name: DeleteAllPhysicalDescriptions :exec
DELETE FROM oikumenea.person_physical_descriptions WHERE person_id = @person_id;

-- ============================ distinguishing marks (D-PhysicalIdentity, M31) ============================

-- name: InsertDistinguishingMark :one
INSERT INTO oikumenea.person_distinguishing_marks (
  person_id, kind, body_location, description, source, confidence
) VALUES (
  @person_id, @kind, sqlc.narg('body_location'), sqlc.narg('description'),
  sqlc.narg('source'), sqlc.narg('confidence')
)
RETURNING *;

-- name: UpdateDistinguishingMark :one
UPDATE oikumenea.person_distinguishing_marks SET
  kind = @kind, body_location = sqlc.narg('body_location'), description = sqlc.narg('description'),
  source = sqlc.narg('source'), confidence = sqlc.narg('confidence')
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteDistinguishingMark :one
UPDATE oikumenea.person_distinguishing_marks SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListDistinguishingMarks :many
SELECT * FROM oikumenea.person_distinguishing_marks
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY kind, id;

-- name: DeleteAllDistinguishingMarks :exec
DELETE FROM oikumenea.person_distinguishing_marks WHERE person_id = @person_id;

-- ============================ ethnicity-type catalog (D-PhysicalIdentity, M31) ============================

-- name: ListEthnicityTypes :many
-- Hierarchical catalog listing (D-PhysicalIdentity amendment, M43), mirroring listLanguages: filter to
-- the forest roots (top_level), the immediate children of a parent RID, or a name/code substring. The
-- has_children flag lets a tree browser show the expand affordance only where children exist.
SELECT e.id, e.code, e.name, e.parent_id, e.wikidata_id, e.status, e.sort_order,
  EXISTS (SELECT 1 FROM oikumenea.person_ethnicity_types c WHERE c.parent_id = e.id AND c.deleted_at IS NULL) AS has_children
FROM oikumenea.person_ethnicity_types e
WHERE e.deleted_at IS NULL
  AND (NOT sqlc.arg(top_level)::boolean OR e.parent_id IS NULL)
  AND (sqlc.narg(parent)::uuid IS NULL OR e.parent_id = sqlc.narg(parent)::uuid)
  AND (NULLIF(sqlc.arg(query)::text, '') IS NULL
       OR e.name ILIKE '%' || sqlc.arg(query)::text || '%'
       OR e.code ILIKE '%' || sqlc.arg(query)::text || '%')
ORDER BY (e.parent_id IS NULL) DESC, e.sort_order NULLS LAST, e.code
LIMIT sqlc.arg(lim)::int;

-- name: GetEthnicityTypeByCode :one
SELECT * FROM oikumenea.person_ethnicity_types WHERE code = @code AND deleted_at IS NULL;

-- name: GetEthnicityTypeByID :one
SELECT e.id, e.code, e.name, e.parent_id, e.wikidata_id, e.status, e.sort_order,
  EXISTS (SELECT 1 FROM oikumenea.person_ethnicity_types c WHERE c.parent_id = e.id AND c.deleted_at IS NULL) AS has_children
FROM oikumenea.person_ethnicity_types e
WHERE e.id = @id AND e.deleted_at IS NULL;

-- name: ListEthnicityTypeLanguages :many
-- Associated-language RIDs for a group (ethnolinguistic metadata; group-level, never a person's datum).
SELECT l.id
FROM oikumenea.person_ethnicity_type_languages pel
JOIN oikumenea.language_languoids l ON l.id = pel.language_id
WHERE pel.ethnicity_type_id = @ethnicity_type_id
ORDER BY l.code;

-- name: ListEthnicityTypeCountries :many
-- Homeland-country RIDs for a group.
SELECT c.id
FROM oikumenea.person_ethnicity_type_countries pec
JOIN oikumenea.geo_countries c ON c.id = pec.country_id
WHERE pec.ethnicity_type_id = @ethnicity_type_id
ORDER BY c.code;

-- name: UpsertEthnicityType :one
INSERT INTO oikumenea.person_ethnicity_types (code, name, parent_id, wikidata_id, sort_order)
VALUES (@code, @name, sqlc.narg('parent_id')::uuid, sqlc.narg('wikidata_id'), sqlc.narg('sort_order'))
ON CONFLICT (code) WHERE deleted_at IS NULL DO UPDATE SET
  name = excluded.name, parent_id = excluded.parent_id, wikidata_id = excluded.wikidata_id,
  sort_order = excluded.sort_order, status = 'active'
RETURNING *;

-- ============================ ethnicities — link__has_ethnicity (pii:special, encrypted) ============================

-- name: InsertEthnicity :one
-- The declared ethnicity is supplied as the envelope (ciphertext/wrapped_dek/key_ref/blind_index) sealed
-- in the application; legal_basis FK validates the lawful basis.
INSERT INTO oikumenea.person_ethnicities (
  person_id, value_ciphertext, wrapped_dek, key_ref, value_blind_index, legal_basis, source, confidence
) VALUES (
  @person_id, @value_ciphertext, @wrapped_dek, @key_ref, @value_blind_index, @legal_basis,
  sqlc.narg('source'), sqlc.narg('confidence')
)
RETURNING *;

-- name: UpdateEthnicity :one
-- Re-seal the declared value and/or flip status/legal_basis.
UPDATE oikumenea.person_ethnicities SET
  value_ciphertext = @value_ciphertext, wrapped_dek = @wrapped_dek, key_ref = @key_ref,
  value_blind_index = @value_blind_index, legal_basis = @legal_basis,
  status = @status, source = sqlc.narg('source'), confidence = sqlc.narg('confidence')
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteEthnicity :one
UPDATE oikumenea.person_ethnicities SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListEthnicities :many
SELECT * FROM oikumenea.person_ethnicities
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: CryptoEraseEthnicities :execrows
-- Crypto-erase all of a person's ethnicities (drop the envelope, keep the row tombstone). The person-purge
-- erasure path for the pii:special declared value (D-PhysicalIdentity / D-SpecialPII).
UPDATE oikumenea.person_ethnicities
SET value_ciphertext = NULL, wrapped_dek = NULL, key_ref = NULL, value_blind_index = NULL
WHERE person_id = @person_id AND deleted_at IS NULL AND value_ciphertext IS NOT NULL;
