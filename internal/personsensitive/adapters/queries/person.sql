-- personsensitive module queries (docs/modules/personsensitive.md, D-PersonModuleSplit / review-2026-07 R-09).
-- The person directory's sensitive / envelope-encrypted surface: physical descriptions & distinguishing
-- marks, ethnicity (crypto-erasable), encrypted declared party memberships, watchlist matches &
-- regulatory sanctions, and the M35 overlays (crypto wallets, personality, political leaning). Split out
-- of the person god-module's query surface; the encrypted columns are sealed/unsealed in the module's
-- application layer (the Cipher lives in personsensitive).

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

-- ============================ addresses (D-PersonAddresses, M32) ============================

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

-- ============================ institutional & political ties (D-InstitutionalTies, M33) ============================

-- ---- party memberships (link__party_membership, pii:special, envelope-encrypted party) ----

-- name: InsertPartyMembership :one
-- The party identity is supplied as the envelope (ciphertext/wrapped_dek/key_ref/blind_index) sealed in the
-- application; legal_basis FK validates the lawful basis (Art. 9 political opinion).
INSERT INTO oikumenea.person_party_memberships (
  person_id, party_ciphertext, party_wrapped_dek, party_key_ref, party_blind_index,
  role, valid_from, valid_to, legal_basis, source, confidence
) VALUES (
  @person_id, @party_ciphertext, @party_wrapped_dek, @party_key_ref, @party_blind_index,
  @role, sqlc.narg('valid_from')::date, sqlc.narg('valid_to')::date, @legal_basis, @source, @confidence
)
RETURNING *;

-- name: UpdatePartyMembership :one
-- Re-seal the party and/or flip role/dates/status/legal_basis.
UPDATE oikumenea.person_party_memberships SET
  party_ciphertext = @party_ciphertext, party_wrapped_dek = @party_wrapped_dek,
  party_key_ref = @party_key_ref, party_blind_index = @party_blind_index,
  role = @role, valid_from = sqlc.narg('valid_from')::date, valid_to = sqlc.narg('valid_to')::date,
  legal_basis = @legal_basis, status = @status, source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeletePartyMembership :one
UPDATE oikumenea.person_party_memberships SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListPartyMemberships :many
SELECT * FROM oikumenea.person_party_memberships
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: CryptoErasePartyMemberships :execrows
-- Crypto-erase a person's party memberships on purge (drop the envelope, keep the row tombstone).
UPDATE oikumenea.person_party_memberships
SET party_ciphertext = NULL, party_wrapped_dek = NULL, party_key_ref = NULL, party_blind_index = NULL
WHERE person_id = @person_id AND deleted_at IS NULL AND party_ciphertext IS NOT NULL;

-- ---- government positions (link__government_position, pii:basic, feeds M34 PEP) ----

-- name: UpsertWatchlistMatch :one
-- One active screening result per person: a re-check refreshes the row in place (partial-unique person_id).
INSERT INTO oikumenea.person_watchlist_matches (
  person_id, on_list, lists, program, match_score, pep, last_checked, next_check_due, source, confidence
) VALUES (
  @person_id, @on_list, @lists, sqlc.narg('program'), sqlc.narg('match_score'), @pep,
  @last_checked, sqlc.narg('next_check_due'), @source, @confidence
)
ON CONFLICT (person_id) WHERE deleted_at IS NULL DO UPDATE SET
  on_list = excluded.on_list, lists = excluded.lists, program = excluded.program,
  match_score = excluded.match_score, pep = excluded.pep, last_checked = excluded.last_checked,
  next_check_due = excluded.next_check_due, source = excluded.source, confidence = excluded.confidence
RETURNING *;

-- name: GetWatchlistMatch :one
SELECT * FROM oikumenea.person_watchlist_matches
WHERE person_id = @person_id AND deleted_at IS NULL;

-- name: DeleteAllWatchlistMatches :exec
-- Hard delete on purge (a transient screening result, not a record to tombstone).
DELETE FROM oikumenea.person_watchlist_matches WHERE person_id = @person_id;

-- ---- regulatory sanctions (object regulatory_sanction, pii:sensitive; a hermenea import target) ----

-- name: UpsertRegulatorySanction :one
-- Idempotent by (person, external_id): a re-imported sanction updates in place. Edits by RID use
-- UpdateRegulatorySanction. A NULL external_id never conflicts (always inserts a fresh row).
INSERT INTO oikumenea.person_regulatory_sanctions (
  person_id, regulator, action_type, amount, currency, status, sanction_date,
  source_url, external_id, legal_basis, source, confidence
) VALUES (
  @person_id, @regulator, @action_type, sqlc.narg('amount'), sqlc.narg('currency'), @status,
  sqlc.narg('sanction_date')::date, sqlc.narg('source_url'), sqlc.narg('external_id'),
  sqlc.narg('legal_basis'), @source, @confidence
)
ON CONFLICT (person_id, external_id) WHERE external_id IS NOT NULL AND deleted_at IS NULL DO UPDATE SET
  regulator = excluded.regulator, action_type = excluded.action_type, amount = excluded.amount,
  currency = excluded.currency, status = excluded.status, sanction_date = excluded.sanction_date,
  source_url = excluded.source_url, legal_basis = excluded.legal_basis,
  source = excluded.source, confidence = excluded.confidence
RETURNING *;

-- name: UpdateRegulatorySanction :one
UPDATE oikumenea.person_regulatory_sanctions SET
  regulator = @regulator, action_type = @action_type, amount = sqlc.narg('amount'),
  currency = sqlc.narg('currency'), status = @status, sanction_date = sqlc.narg('sanction_date')::date,
  source_url = sqlc.narg('source_url'), external_id = sqlc.narg('external_id'),
  legal_basis = sqlc.narg('legal_basis'), source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteRegulatorySanction :one
UPDATE oikumenea.person_regulatory_sanctions SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListRegulatorySanctions :many
SELECT * FROM oikumenea.person_regulatory_sanctions
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY sanction_date DESC NULLS LAST, created_at DESC, id;

-- name: DeleteAllRegulatorySanctions :exec
DELETE FROM oikumenea.person_regulatory_sanctions WHERE person_id = @person_id;

-- ============================================================================================
-- Financial / behavioural / psychological overlays (D-PersonOverlays, M35)
-- ============================================================================================

-- ---- crypto wallets (object crypto_wallet, pii:sensitive; M34 sanctioned-wallet synergy) ----

-- name: InsertCryptoWallet :one
INSERT INTO oikumenea.person_crypto_wallets (
  person_id, address, chain, attribution_method, balance_usd_approx,
  first_seen, last_seen, source, confidence
) VALUES (
  @person_id, @address, @chain, @attribution_method, sqlc.narg('balance_usd_approx')::double precision,
  sqlc.narg('first_seen')::date, sqlc.narg('last_seen')::date, @source, @confidence
)
RETURNING *;

-- name: UpdateCryptoWallet :one
UPDATE oikumenea.person_crypto_wallets SET
  address = @address, chain = @chain, attribution_method = @attribution_method,
  balance_usd_approx = sqlc.narg('balance_usd_approx')::double precision,
  first_seen = sqlc.narg('first_seen')::date, last_seen = sqlc.narg('last_seen')::date,
  source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteCryptoWallet :one
UPDATE oikumenea.person_crypto_wallets SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListCryptoWallets :many
SELECT * FROM oikumenea.person_crypto_wallets
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: DeleteAllCryptoWallets :exec
DELETE FROM oikumenea.person_crypto_wallets WHERE person_id = @person_id;

-- ---- personality profiles (object personality, pii:sensitive; declared/assessment only) ----

-- name: InsertPersonality :one
INSERT INTO oikumenea.person_personality (
  person_id, framework, result, instrument, method, assessed_at, source, confidence
) VALUES (
  @person_id, @framework, @result, sqlc.narg('instrument'), @method,
  sqlc.narg('assessed_at')::date, @source, @confidence
)
RETURNING *;

-- name: UpdatePersonality :one
UPDATE oikumenea.person_personality SET
  framework = @framework, result = @result, instrument = sqlc.narg('instrument'),
  method = @method, assessed_at = sqlc.narg('assessed_at')::date, source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeletePersonality :one
UPDATE oikumenea.person_personality SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListPersonalities :many
SELECT * FROM oikumenea.person_personality
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: DeleteAllPersonalities :exec
DELETE FROM oikumenea.person_personality WHERE person_id = @person_id;

-- ---- inferred political leaning (object political_leaning, pii:special, ENCRYPTED) ----

-- name: InsertPoliticalLeaning :one
-- The spectrum is supplied as the envelope (ciphertext/wrapped_dek/key_ref/blind_index) sealed in the
-- application; legal_basis FK validates the lawful basis (Art. 9 political opinion, inferred).
INSERT INTO oikumenea.person_political_leaning (
  person_id, leaning_ciphertext, leaning_wrapped_dek, leaning_key_ref, leaning_blind_index,
  inference_sources, assessed_at, legal_basis, confidence
) VALUES (
  @person_id, @leaning_ciphertext, @leaning_wrapped_dek, @leaning_key_ref, @leaning_blind_index,
  @inference_sources, sqlc.narg('assessed_at')::date, @legal_basis, @confidence
)
RETURNING *;

-- name: UpdatePoliticalLeaning :one
-- Re-seal the spectrum and/or flip sources/date/legal_basis for the single active row.
UPDATE oikumenea.person_political_leaning SET
  leaning_ciphertext = @leaning_ciphertext, leaning_wrapped_dek = @leaning_wrapped_dek,
  leaning_key_ref = @leaning_key_ref, leaning_blind_index = @leaning_blind_index,
  inference_sources = @inference_sources, assessed_at = sqlc.narg('assessed_at')::date,
  legal_basis = @legal_basis, confidence = @confidence
WHERE person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: GetPoliticalLeaning :one
SELECT * FROM oikumenea.person_political_leaning
WHERE person_id = @person_id AND deleted_at IS NULL;

-- name: DeletePoliticalLeaning :one
UPDATE oikumenea.person_political_leaning SET deleted_at = now()
WHERE person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: CryptoErasePoliticalLeaning :execrows
-- Crypto-erase a person's inferred leaning on purge (drop the envelope, keep the row tombstone).
UPDATE oikumenea.person_political_leaning
SET leaning_ciphertext = NULL, leaning_wrapped_dek = NULL, leaning_key_ref = NULL, leaning_blind_index = NULL
WHERE person_id = @person_id AND deleted_at IS NULL AND leaning_ciphertext IS NOT NULL;

-- ============================================================================================
-- Health & vulnerability records (D-HealthVulnerability, M36)
-- ============================================================================================

-- ---- health records (object health_record, pii:special, ENCRYPTED; one active per (person, kind)) ----

-- name: InsertHealthRecord :one
-- The category-level detail is supplied as the envelope (ciphertext/wrapped_dek/key_ref/blind_index)
-- sealed in the application; legal_basis FK validates the lawful basis (Art. 9 health data).
INSERT INTO oikumenea.person_health_records (
  person_id, kind, detail_ciphertext, detail_wrapped_dek, detail_key_ref, detail_blind_index,
  is_public_record, assessed_at, legal_basis, source, confidence
) VALUES (
  @person_id, @kind, @detail_ciphertext, @detail_wrapped_dek, @detail_key_ref, @detail_blind_index,
  @is_public_record, sqlc.narg('assessed_at')::date, @legal_basis, @source, @confidence
)
RETURNING *;

-- name: UpdateHealthRecord :one
-- Re-seal the detail and/or flip the attributes for the single active row of this (person, kind).
UPDATE oikumenea.person_health_records SET
  detail_ciphertext = @detail_ciphertext, detail_wrapped_dek = @detail_wrapped_dek,
  detail_key_ref = @detail_key_ref, detail_blind_index = @detail_blind_index,
  is_public_record = @is_public_record, assessed_at = sqlc.narg('assessed_at')::date,
  legal_basis = @legal_basis, source = @source, confidence = @confidence
WHERE person_id = @person_id AND kind = @kind AND deleted_at IS NULL
RETURNING *;

-- name: DeleteHealthRecord :one
UPDATE oikumenea.person_health_records SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListHealthRecords :many
SELECT * FROM oikumenea.person_health_records
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: CryptoEraseHealthRecords :execrows
-- Crypto-erase a person's health records on purge (drop the envelope, keep the row tombstones).
UPDATE oikumenea.person_health_records
SET detail_ciphertext = NULL, detail_wrapped_dek = NULL, detail_key_ref = NULL, detail_blind_index = NULL
WHERE person_id = @person_id AND deleted_at IS NULL AND detail_ciphertext IS NOT NULL;

-- ---- insurance (object insurance, pii:sensitive) ----

-- name: InsertInsurance :one
INSERT INTO oikumenea.person_insurance (
  person_id, type, provider, policy_reference, employer_sponsored, valid_from, valid_to, source, confidence
) VALUES (
  @person_id, @type, sqlc.narg('provider'), sqlc.narg('policy_reference'), @employer_sponsored,
  sqlc.narg('valid_from')::date, sqlc.narg('valid_to')::date, @source, @confidence
)
RETURNING *;

-- name: UpdateInsurance :one
UPDATE oikumenea.person_insurance SET
  type = @type, provider = sqlc.narg('provider'), policy_reference = sqlc.narg('policy_reference'),
  employer_sponsored = @employer_sponsored, valid_from = sqlc.narg('valid_from')::date,
  valid_to = sqlc.narg('valid_to')::date, source = @source, confidence = @confidence
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: DeleteInsurance :one
UPDATE oikumenea.person_insurance SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING id;

-- name: ListInsurance :many
SELECT * FROM oikumenea.person_insurance
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY created_at DESC, id;

-- name: DeleteAllInsurance :exec
DELETE FROM oikumenea.person_insurance WHERE person_id = @person_id;

-- ============================ person existence / identity (parent guard) ============================

-- name: PersonExists :one
-- Parent-existence guard: personsensitive child writes/reads verify the person exists (and is not
-- soft-deleted) before touching its sensitive rows. A reviewed cross-module read of the person core's
-- aggregate table (personsensitive:person allowlist), mirroring the core GetPerson visibility predicate.
SELECT 1 FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;

-- name: GetPerson :one
-- Person identity read for watchlist screening (CheckWatchlists needs the subject's name / birthdate /
-- nationality). A reviewed cross-module read of the person core aggregate (personsensitive:person).
SELECT * FROM oikumenea.person_persons WHERE id = @id AND deleted_at IS NULL;
