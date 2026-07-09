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
-- Keyset pagination over the time-ordered RID (an empty cursor starts at the beginning). The
-- unfiltered directory list; the application routes a text query to SearchPersons instead.
-- Explicit column list (review R-17): the directory list is a hot path and must NOT hydrate the wide
-- generated search_text haystack (nor the always-NULL deleted_at) — it lists exactly the columns the
-- row->domain mapper reads, so sqlc emits a lean row struct. Keep single-row GetPerson as SELECT *.
SELECT id, code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id,
  attributes, status, deactivated_at, purge_after, created_at, updated_at
FROM oikumenea.person_persons
WHERE deleted_at IS NULL AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: SearchPersons :many
-- Trigram directory search (review R-06). @query is a non-empty case-insensitive substring; a person
-- matches on its own search haystack (display name / code / given / surname) OR any name variant
-- (transliteration / alias) — so a native-script or aka form is findable, not just the Latin display
-- name. The two match branches are a UNION of id sets so EACH stays a GIN trigram bitmap scan (an
-- `A OR EXISTS(subquery)` predicate is NOT indexable and would seq-scan the whole directory); the
-- outer keyset then paginates by RID. Sub-3-char queries fall back to a scan (pg_trgm needs a full
-- trigram) — acceptable for the rare short-prefix case. Lean column list (review R-17): a per-keystroke
-- typeahead path must not hydrate search_text/deleted_at — same lean projection as ListPersons.
SELECT id, code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id,
  attributes, status, deactivated_at, purge_after, created_at, updated_at
FROM oikumenea.person_persons
WHERE deleted_at IS NULL AND (@after = '' OR id::text > @after)
  AND id IN (
    SELECT ps.id FROM oikumenea.person_persons ps
      WHERE ps.deleted_at IS NULL AND ps.search_text ILIKE '%' || @query || '%'
    UNION
    SELECT v.person_id FROM oikumenea.person_name_variants v
      WHERE v.search_text ILIKE '%' || @query || '%')
ORDER BY id
LIMIT @lim;

-- name: ListPersonsByIDs :many
-- Load the base person rows for a set of RIDs (the D-PersonReadScope directory-list union resolves
-- visible person ids through memberships, then hydrates the rows here). Ordered by RID so the caller
-- can re-key to its keyset order. Lean column list (review R-17): the scoped-list hydration is a hot
-- path — same lean projection as ListPersons (no search_text/deleted_at).
SELECT id, code, display_name, title, given, given2, surname, surname_prefix, surname2,
  generation, credentials, preferred, birthdate, date_of_death, sex, country_of_birth_id,
  attributes, status, deactivated_at, purge_after, created_at, updated_at
FROM oikumenea.person_persons
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

-- name: GetActivePersonByCode :one
-- Look up an active person by their stable `code` (D-Code). Used by identity-federation for
-- just-in-time link-on-match (D-JIT) and the first-admin bootstrap's find-or-create (D-Bootstrap).
SELECT * FROM oikumenea.person_persons
WHERE code = @code AND status = 'active' AND deleted_at IS NULL;

-- ============================ emails (D-PersonContactChannels) ============================

