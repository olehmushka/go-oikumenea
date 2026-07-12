-- Company module queries (docs/modules/company.md / D-Companies). A legal-entity registry: catalogs
-- (legal forms / registration schemes / industry classes), companies, registrations, industry
-- assignments, locations, positions/appointments (mirrors membership), and the ownership/affiliation
-- graph (foundings, shareholdings, beneficiaries, successions, branches). RID PKs default at the
-- database. Soft-delete + reversible status flips throughout; a NULL narg leaves the stored value
-- unchanged on update (COALESCE). FKs validate referenced entities (mapped in the adapter). Polymorphic
-- holders are (holder_kind, holder_id) with no FK (F-014).

-- ============================ catalogs ============================

-- name: ListLegalForms :many
SELECT * FROM oikumenea.company_legal_forms WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: UpsertLegalForm :one
INSERT INTO oikumenea.company_legal_forms (code, name, abbreviation, country_id, sort_order)
VALUES (@code, @name, sqlc.narg('abbreviation'), sqlc.narg('country_id'), sqlc.narg('sort_order'))
ON CONFLICT (code) WHERE deleted_at IS NULL
DO UPDATE SET name = EXCLUDED.name, abbreviation = EXCLUDED.abbreviation, country_id = EXCLUDED.country_id,
  sort_order = COALESCE(EXCLUDED.sort_order, oikumenea.company_legal_forms.sort_order), updated_at = now()
RETURNING *;

-- name: ListRegistrationSchemes :many
SELECT * FROM oikumenea.company_registration_schemes WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: GetRegistrationScheme :one
SELECT * FROM oikumenea.company_registration_schemes WHERE id = @id AND deleted_at IS NULL;

-- name: UpsertRegistrationScheme :one
INSERT INTO oikumenea.company_registration_schemes (code, name, validator_pattern, is_global, sort_order)
VALUES (@code, @name, sqlc.narg('validator_pattern'), @is_global, sqlc.narg('sort_order'))
ON CONFLICT (code) WHERE deleted_at IS NULL
DO UPDATE SET name = EXCLUDED.name, validator_pattern = EXCLUDED.validator_pattern, is_global = EXCLUDED.is_global,
  sort_order = COALESCE(EXCLUDED.sort_order, oikumenea.company_registration_schemes.sort_order), updated_at = now()
RETURNING *;

-- name: ListIndustryClasses :many
SELECT * FROM oikumenea.company_industry_classes WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: UpsertIndustryClass :one
INSERT INTO oikumenea.company_industry_classes (code, name, system, sort_order)
VALUES (@code, @name, @system, sqlc.narg('sort_order'))
ON CONFLICT (code) WHERE deleted_at IS NULL
DO UPDATE SET name = EXCLUDED.name, system = EXCLUDED.system,
  sort_order = COALESCE(EXCLUDED.sort_order, oikumenea.company_industry_classes.sort_order), updated_at = now()
RETURNING *;

-- ============================ companies (tenant org + company_org_profiles sidecar — M41) ============================
-- A company is a `company`-domain tenant organization (its stable `code` and registered name = the org's
-- code + name) plus a company_org_profiles sidecar (short_name/legal_form/ownership/country/dates/state).
-- The org itself is created/updated through the tenant service; these queries own the sidecar and the
-- joined read view.

-- name: InsertOrgProfile :one
INSERT INTO oikumenea.company_org_profiles
  (company_id, short_name, legal_form_id, ownership_category, country_id, founded_on)
VALUES (@company_id, sqlc.narg('short_name'), @legal_form_id,
        COALESCE(sqlc.narg('ownership_category'), 'private'), sqlc.narg('country_id'), sqlc.narg('founded_on'))
RETURNING *;

-- name: GetCompany :one
SELECT o.id, o.code, o.name AS legal_name,
  p.short_name, p.legal_form_id, p.ownership_category, p.country_id,
  p.founded_on, p.dissolved_on, p.state, p.created_at, p.updated_at
FROM oikumenea.company_org_profiles p
JOIN oikumenea.tenant_organizations o ON o.id = p.company_id AND o.deleted_at IS NULL
WHERE p.company_id = @id AND p.deleted_at IS NULL;

-- name: UpdateOrgProfile :one
UPDATE oikumenea.company_org_profiles SET
  short_name         = COALESCE(sqlc.narg('short_name'), short_name),
  legal_form_id      = COALESCE(sqlc.narg('legal_form_id'), legal_form_id),
  ownership_category = COALESCE(sqlc.narg('ownership_category'), ownership_category),
  country_id         = COALESCE(sqlc.narg('country_id'), country_id),
  founded_on         = COALESCE(sqlc.narg('founded_on'), founded_on),
  dissolved_on       = COALESCE(sqlc.narg('dissolved_on'), dissolved_on),
  state              = COALESCE(sqlc.narg('state'), state)
WHERE company_id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListCompanies :many
-- Active companies, keyset-paginated by org RID. A text query routes to SearchCompanies (review R-21):
-- a `(@query = '' OR …)` guard would defeat the trigram GIN indexes.
SELECT o.id, o.code, o.name AS legal_name,
  p.short_name, p.legal_form_id, p.ownership_category, p.country_id,
  p.founded_on, p.dissolved_on, p.state, p.created_at, p.updated_at
FROM oikumenea.company_org_profiles p
JOIN oikumenea.tenant_organizations o ON o.id = p.company_id AND o.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND (@after = '' OR o.id::text > @after)
ORDER BY o.id
LIMIT @lim;

-- name: SearchCompanies :many
-- The trigram-served twin of ListCompanies (review R-21). The match spans two tables — org code/name
-- (tenant_organizations) and short_name (the joined company_org_profiles) — so it is a UNION of id-sets
-- rather than an OR across the join: an OR spanning both tables can't BitmapOr, whereas each UNION arm
-- stays a GIN bitmap scan (the person names+variants pattern, D-PersonSearch). Same projection and keyset
-- as ListCompanies so the two rows are convertible in the repository.
SELECT o.id, o.code, o.name AS legal_name,
  p.short_name, p.legal_form_id, p.ownership_category, p.country_id,
  p.founded_on, p.dissolved_on, p.state, p.created_at, p.updated_at
FROM oikumenea.company_org_profiles p
JOIN oikumenea.tenant_organizations o ON o.id = p.company_id AND o.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND o.id IN (
    SELECT org.id FROM oikumenea.tenant_organizations org
      WHERE org.deleted_at IS NULL
        AND org.search_text ILIKE '%' || @query || '%'
    UNION
    SELECT cp.company_id FROM oikumenea.company_org_profiles cp
      WHERE cp.deleted_at IS NULL AND cp.short_name ILIKE '%' || @query || '%')
  AND (@after = '' OR o.id::text > @after)
ORDER BY o.id
LIMIT @lim;

-- name: SoftDeleteCompany :execrows
UPDATE oikumenea.company_org_profiles SET deleted_at = now() WHERE company_id = @id AND deleted_at IS NULL;

-- name: CompanyNamesByIDs :many
SELECT id, name AS legal_name FROM oikumenea.tenant_organizations
WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- ============================ registrations ============================

-- name: InsertRegistration :one
INSERT INTO oikumenea.company_registrations (company_id, scheme_id, identifier, validated)
VALUES (@company_id, @scheme_id, @identifier, @validated)
RETURNING *;

-- name: GetRegistration :one
SELECT * FROM oikumenea.company_registrations WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateRegistration :one
UPDATE oikumenea.company_registrations SET
  scheme_id = @scheme_id, identifier = @identifier, validated = @validated
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListRegistrationsByCompany :many
SELECT * FROM oikumenea.company_registrations
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY created_at;

-- name: SoftDeleteRegistration :execrows
UPDATE oikumenea.company_registrations SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ industry assignments ============================

-- name: InsertIndustryAssignment :one
INSERT INTO oikumenea.company_industry_assignments (company_id, industry_class_id, is_primary)
VALUES (@company_id, @industry_class_id, @is_primary)
RETURNING *;

-- name: ClearPrimaryIndustries :exec
UPDATE oikumenea.company_industry_assignments SET is_primary = false
WHERE company_id = @company_id AND is_primary AND deleted_at IS NULL;

-- name: ListIndustriesByCompany :many
SELECT * FROM oikumenea.company_industry_assignments
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY is_primary DESC, created_at;

-- name: SoftDeleteIndustryAssignment :execrows
UPDATE oikumenea.company_industry_assignments SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ locations ============================

-- name: InsertCompanyLocation :one
INSERT INTO oikumenea.company_locations (company_id, location_id, role)
VALUES (@company_id, @location_id, @role)
RETURNING *;

-- name: ListCompanyLocationsByCompany :many
SELECT * FROM oikumenea.company_locations
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY created_at;

-- name: SoftDeleteCompanyLocation :execrows
UPDATE oikumenea.company_locations SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ positions & appointments ============================

-- name: InsertPosition :one
INSERT INTO oikumenea.company_positions (company_id, code, title, sort_order)
VALUES (@company_id, @code, @title, sqlc.narg('sort_order'))
RETURNING *;

-- name: GetPosition :one
SELECT * FROM oikumenea.company_positions WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePosition :one
UPDATE oikumenea.company_positions SET
  title      = COALESCE(sqlc.narg('title'), title),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: AbolishPosition :one
UPDATE oikumenea.company_positions SET status = 'abolished'
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPositionsByCompany :many
-- Active positions of a company, optional vacant|filled filter via the active-appointment existence.
SELECT p.* FROM oikumenea.company_positions p
WHERE p.company_id = @company_id AND p.deleted_at IS NULL AND p.status = 'active'
  AND (@after = '' OR p.id::text > @after)
  AND (
    @state = '' OR
    (@state = 'filled' AND EXISTS (
      SELECT 1 FROM oikumenea.company_appointments a
      WHERE a.position_id = p.id AND a.status = 'active' AND a.deleted_at IS NULL)) OR
    (@state = 'vacant' AND NOT EXISTS (
      SELECT 1 FROM oikumenea.company_appointments a
      WHERE a.position_id = p.id AND a.status = 'active' AND a.deleted_at IS NULL))
  )
ORDER BY p.id
LIMIT @lim;

-- name: GetActiveAppointmentByPosition :one
SELECT * FROM oikumenea.company_appointments
WHERE position_id = @position_id AND status = 'active' AND deleted_at IS NULL;

-- name: InsertAppointment :one
INSERT INTO oikumenea.company_appointments (person_id, position_id, effective_from)
VALUES (@person_id, @position_id, COALESCE(sqlc.narg('effective_from'), now()))
RETURNING *;

-- name: GetAppointment :one
SELECT * FROM oikumenea.company_appointments WHERE id = @id AND deleted_at IS NULL;

-- name: EndAppointment :one
UPDATE oikumenea.company_appointments
SET status = 'ended', effective_to = COALESCE(sqlc.narg('effective_to'), now())
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: ListAppointmentsByPerson :many
-- A person's company appointments, enriched with the position title + owning company (read-only view).
SELECT a.*, p.title AS position_title, p.company_id AS company_id, c.name AS company_name
FROM oikumenea.company_appointments a
JOIN oikumenea.company_positions p ON p.id = a.position_id
JOIN oikumenea.tenant_organizations c ON c.id = p.company_id
WHERE a.person_id = @person_id AND a.deleted_at IS NULL
ORDER BY a.effective_from DESC;

-- ============================ foundings (link__founded) ============================

-- name: InsertFounding :one
INSERT INTO oikumenea.company_foundings (company_id, holder_kind, holder_id, founded_on)
VALUES (@company_id, @holder_kind, @holder_id, sqlc.narg('founded_on'))
RETURNING *;

-- name: GetFounding :one
SELECT * FROM oikumenea.company_foundings WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteFounding :execrows
UPDATE oikumenea.company_foundings SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListFoundingsByCompany :many
SELECT * FROM oikumenea.company_foundings
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY created_at;

-- name: ListFoundingsByPersonHolder :many
SELECT * FROM oikumenea.company_foundings
WHERE holder_kind = 'person' AND holder_id = @person_id AND deleted_at IS NULL ORDER BY created_at;

-- ============================ shareholdings (link__owns_stake) ============================

-- name: InsertShareholding :one
INSERT INTO oikumenea.company_shareholdings (company_id, holder_kind, holder_id, stake_pct, effective_from, effective_to)
VALUES (@company_id, @holder_kind, @holder_id, sqlc.narg('stake_pct'), sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetShareholding :one
SELECT * FROM oikumenea.company_shareholdings WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteShareholding :execrows
UPDATE oikumenea.company_shareholdings SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListShareholdersByCompany :many
SELECT * FROM oikumenea.company_shareholdings
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY stake_pct DESC NULLS LAST, created_at;

-- name: ListHoldingsByCompanyHolder :many
SELECT * FROM oikumenea.company_shareholdings
WHERE holder_kind = 'company' AND holder_id = @company_id AND deleted_at IS NULL ORDER BY created_at;

-- name: ListShareholdingsByPersonHolder :many
SELECT * FROM oikumenea.company_shareholdings
WHERE holder_kind = 'person' AND holder_id = @person_id AND deleted_at IS NULL ORDER BY created_at;

-- ============================ beneficiaries (link__beneficiary_of) ============================

-- name: InsertBeneficiary :one
INSERT INTO oikumenea.company_beneficiaries (company_id, person_id, ultimate_pct, declared)
VALUES (@company_id, @person_id, sqlc.narg('ultimate_pct'), @declared)
RETURNING *;

-- name: GetBeneficiary :one
SELECT * FROM oikumenea.company_beneficiaries WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteBeneficiary :execrows
UPDATE oikumenea.company_beneficiaries SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListBeneficiariesByCompany :many
SELECT * FROM oikumenea.company_beneficiaries
WHERE company_id = @company_id AND deleted_at IS NULL ORDER BY ultimate_pct DESC NULLS LAST, created_at;

-- name: ListBeneficiariesByPerson :many
SELECT * FROM oikumenea.company_beneficiaries
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY created_at;

-- ============================ successions (link__succeeded_by) ============================

-- name: InsertSuccession :one
INSERT INTO oikumenea.company_successions (predecessor_id, successor_id, kind, effective_on)
VALUES (@predecessor_id, @successor_id, COALESCE(sqlc.narg('kind'), 'reorganization'), sqlc.narg('effective_on'))
RETURNING *;

-- name: GetSuccession :one
SELECT * FROM oikumenea.company_successions WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteSuccession :execrows
UPDATE oikumenea.company_successions SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListSuccessionsByCompany :many
SELECT * FROM oikumenea.company_successions
WHERE (predecessor_id = @company_id OR successor_id = @company_id) AND deleted_at IS NULL ORDER BY created_at;

-- ============================ branches (link__branch_of) ============================

-- name: InsertBranch :one
INSERT INTO oikumenea.company_branches (parent_id, branch_id)
VALUES (@parent_id, @branch_id)
RETURNING *;

-- name: GetBranch :one
SELECT * FROM oikumenea.company_branches WHERE id = @id AND deleted_at IS NULL;

-- name: SoftDeleteBranch :execrows
UPDATE oikumenea.company_branches SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListBranchesByParent :many
SELECT * FROM oikumenea.company_branches
WHERE parent_id = @parent_id AND deleted_at IS NULL ORDER BY created_at;
