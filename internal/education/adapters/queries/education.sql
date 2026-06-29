-- Education module queries (docs/modules/education.md / D-Education). External reference institutions,
-- their recursive structure tree (+ a maintained transitive closure), buildings, groups, the
-- positions/appointments (mirrors membership), and the person bindings (enrollments, dorm stays). RID
-- PKs default at the database. Soft-delete + reversible status flips throughout; a NULL narg leaves the
-- stored value unchanged on update (COALESCE). FKs validate referenced entities (mapped in the adapter).

-- ============================ catalogs ============================

-- name: ListInstitutionKinds :many
SELECT * FROM oikumenea.education_institution_kinds
WHERE deleted_at IS NULL ORDER BY sort_order, code;

-- name: UpsertInstitutionKind :one
INSERT INTO oikumenea.education_institution_kinds (code, name, sort_order)
VALUES (@code, @name, sqlc.narg('sort_order'))
ON CONFLICT (code) WHERE deleted_at IS NULL
DO UPDATE SET name = EXCLUDED.name, sort_order = COALESCE(EXCLUDED.sort_order, oikumenea.education_institution_kinds.sort_order), updated_at = now()
RETURNING *;

-- name: ListDegreeLevels :many
SELECT * FROM oikumenea.education_degree_levels
WHERE deleted_at IS NULL ORDER BY isced_level;

-- ============================ institutions (tenant org + education_org_profiles sidecar — M41) ============================
-- An institution is a `university`-domain tenant organization (code/name/visibility) plus an
-- education_org_profiles sidecar (kind/country/dates/state). The org itself is created/updated through
-- the tenant service; these queries own the sidecar and the joined read view.

-- name: InsertOrgProfile :one
INSERT INTO oikumenea.education_org_profiles (institution_id, kind_id, country_id, founded_on, closed_on)
VALUES (@institution_id, @kind_id, sqlc.narg('country_id'), sqlc.narg('founded_on'), sqlc.narg('closed_on'))
RETURNING *;

-- name: GetInstitution :one
SELECT o.id, o.code, o.name,
  p.kind_id, p.country_id, p.founded_on, p.closed_on, p.state, p.created_at, p.updated_at
FROM oikumenea.education_org_profiles p
JOIN oikumenea.tenant_organizations o ON o.id = p.institution_id AND o.deleted_at IS NULL
WHERE p.institution_id = @id AND p.deleted_at IS NULL;

-- name: UpdateOrgProfile :one
UPDATE oikumenea.education_org_profiles SET
  kind_id    = COALESCE(sqlc.narg('kind_id'), kind_id),
  country_id = COALESCE(sqlc.narg('country_id'), country_id),
  founded_on = COALESCE(sqlc.narg('founded_on'), founded_on),
  closed_on  = COALESCE(sqlc.narg('closed_on'), closed_on),
  state      = COALESCE(sqlc.narg('state'), state)
WHERE institution_id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListInstitutions :many
-- Active institutions (orgs with an education profile), optional case-insensitive code/name filter,
-- keyset-paginated by org RID.
SELECT o.id, o.code, o.name,
  p.kind_id, p.country_id, p.founded_on, p.closed_on, p.state, p.created_at, p.updated_at
FROM oikumenea.education_org_profiles p
JOIN oikumenea.tenant_organizations o ON o.id = p.institution_id AND o.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND (@query = '' OR o.code ILIKE '%' || @query || '%' OR o.name ILIKE '%' || @query || '%')
  AND (@after = '' OR o.id::text > @after)
ORDER BY o.id
LIMIT @lim;

-- name: SoftDeleteInstitution :execrows
UPDATE oikumenea.education_org_profiles SET deleted_at = now()
WHERE institution_id = @id AND deleted_at IS NULL;

-- ============================ buildings ============================

-- name: InsertBuilding :one
INSERT INTO oikumenea.education_buildings (institution_id, unit_id, location_id, code, name, kind)
VALUES (@institution_id, sqlc.narg('unit_id'), sqlc.narg('location_id'), @code, @name, @kind)
RETURNING *;

-- name: GetBuilding :one
SELECT * FROM oikumenea.education_buildings WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateBuilding :one
UPDATE oikumenea.education_buildings SET
  name        = COALESCE(sqlc.narg('name'), name),
  kind        = COALESCE(sqlc.narg('kind'), kind),
  unit_id     = COALESCE(sqlc.narg('unit_id'), unit_id),
  location_id = COALESCE(sqlc.narg('location_id'), location_id)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListBuildingsByInstitution :many
SELECT * FROM oikumenea.education_buildings
WHERE institution_id = @institution_id AND deleted_at IS NULL
ORDER BY code;

-- name: SoftDeleteBuilding :execrows
UPDATE oikumenea.education_buildings SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- ============================ groups ============================

-- name: InsertGroup :one
INSERT INTO oikumenea.education_groups (unit_id, code, name, admission_year)
VALUES (@unit_id, @code, @name, sqlc.narg('admission_year'))
RETURNING *;

-- name: GetGroup :one
SELECT * FROM oikumenea.education_groups WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateGroup :one
UPDATE oikumenea.education_groups SET
  name           = COALESCE(sqlc.narg('name'), name),
  admission_year = COALESCE(sqlc.narg('admission_year'), admission_year),
  status         = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListGroupsByUnit :many
SELECT * FROM oikumenea.education_groups
WHERE unit_id = @unit_id AND deleted_at IS NULL
ORDER BY code;

-- name: SoftDeleteGroup :execrows
UPDATE oikumenea.education_groups SET deleted_at = now()
WHERE id = @id AND deleted_at IS NULL;

-- ============================ positions ============================

-- name: InsertPosition :one
INSERT INTO oikumenea.education_positions (institution_id, unit_id, code, title, sort_order)
VALUES (
  @institution_id, sqlc.narg('unit_id'), @code, @title,
  COALESCE(sqlc.narg('sort_order'), (
    SELECT COALESCE(MAX(sort_order), 0) + 1 FROM oikumenea.education_positions
    WHERE institution_id = @institution_id AND status = 'active' AND deleted_at IS NULL
  ))
)
RETURNING *;

-- name: GetPosition :one
SELECT * FROM oikumenea.education_positions WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePosition :one
UPDATE oikumenea.education_positions SET
  title      = COALESCE(sqlc.narg('title'), title),
  sort_order = COALESCE(sqlc.narg('sort_order'), sort_order)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: AbolishPosition :one
UPDATE oikumenea.education_positions SET status = 'abolished'
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: ListPositionsByInstitution :many
SELECT * FROM oikumenea.education_positions
WHERE institution_id = @institution_id AND status = 'active' AND deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: ListVacantPositionsByInstitution :many
SELECT p.* FROM oikumenea.education_positions p
WHERE p.institution_id = @institution_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM oikumenea.education_appointments a
    WHERE a.position_id = p.id AND a.status = 'active' AND a.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- name: ListFilledPositionsByInstitution :many
SELECT p.* FROM oikumenea.education_positions p
WHERE p.institution_id = @institution_id AND p.status = 'active' AND p.deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM oikumenea.education_appointments a
    WHERE a.position_id = p.id AND a.status = 'active' AND a.deleted_at IS NULL
  )
  AND (@after = '' OR p.id::text > @after)
ORDER BY p.id
LIMIT @lim;

-- ============================ appointments ============================

-- name: InsertAppointment :one
INSERT INTO oikumenea.education_appointments (person_id, position_id, effective_from)
VALUES (@person_id, @position_id, COALESCE(sqlc.narg('effective_from')::timestamptz, now()))
RETURNING *;

-- name: GetAppointment :one
SELECT * FROM oikumenea.education_appointments WHERE id = @id AND deleted_at IS NULL;

-- name: GetActiveAppointmentByPosition :one
SELECT * FROM oikumenea.education_appointments
WHERE position_id = @position_id AND status = 'active' AND deleted_at IS NULL;

-- name: EndAppointment :one
UPDATE oikumenea.education_appointments
SET status = 'ended', effective_to = COALESCE(sqlc.narg('effective_to')::timestamptz, now())
WHERE id = @id AND status = 'active' AND deleted_at IS NULL
RETURNING *;

-- name: ListAppointmentsByPerson :many
SELECT a.*, p.title AS position_title, p.institution_id AS institution_id, i.name AS institution_name
FROM oikumenea.education_appointments a
JOIN oikumenea.education_positions p ON p.id = a.position_id
JOIN oikumenea.tenant_organizations i ON i.id = p.institution_id
WHERE a.person_id = @person_id AND a.deleted_at IS NULL
ORDER BY a.status, a.effective_from DESC NULLS LAST, a.id;

-- ============================ person bindings ============================

-- name: InsertEnrollment :one
INSERT INTO oikumenea.person_education_enrollments
  (person_id, institution_id, unit_id, group_id, program_id, degree_level_id, field_of_study, student_number, status, qualification, effective_from, effective_to)
VALUES (
  @person_id, @institution_id, sqlc.narg('unit_id'), sqlc.narg('group_id'), sqlc.narg('program_id'), sqlc.narg('degree_level_id'),
  sqlc.narg('field_of_study'), sqlc.narg('student_number'), COALESCE(sqlc.narg('status'), 'enrolled'), sqlc.narg('qualification'),
  sqlc.narg('effective_from'), sqlc.narg('effective_to')
)
RETURNING *;

-- name: GetEnrollment :one
SELECT * FROM oikumenea.person_education_enrollments
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateEnrollment :one
UPDATE oikumenea.person_education_enrollments SET
  institution_id  = COALESCE(sqlc.narg('institution_id'), institution_id),
  unit_id         = COALESCE(sqlc.narg('unit_id'), unit_id),
  group_id        = COALESCE(sqlc.narg('group_id'), group_id),
  program_id      = COALESCE(sqlc.narg('program_id'), program_id),
  degree_level_id = COALESCE(sqlc.narg('degree_level_id'), degree_level_id),
  field_of_study  = COALESCE(sqlc.narg('field_of_study'), field_of_study),
  student_number  = COALESCE(sqlc.narg('student_number'), student_number),
  status          = COALESCE(sqlc.narg('status'), status),
  qualification   = COALESCE(sqlc.narg('qualification'), qualification),
  effective_from  = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to    = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteEnrollment :execrows
UPDATE oikumenea.person_education_enrollments SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: ListEnrollmentsByPerson :many
SELECT * FROM oikumenea.person_education_enrollments
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY effective_from DESC NULLS LAST, id;

-- name: InsertDormitoryStay :one
INSERT INTO oikumenea.person_dormitory_stays (person_id, building_id, room, status, effective_from, effective_to)
VALUES (@person_id, @building_id, sqlc.narg('room'), COALESCE(sqlc.narg('status'), 'active'),
        sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetDormitoryStay :one
SELECT * FROM oikumenea.person_dormitory_stays
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateDormitoryStay :one
UPDATE oikumenea.person_dormitory_stays SET
  building_id    = COALESCE(sqlc.narg('building_id'), building_id),
  room           = COALESCE(sqlc.narg('room'), room),
  status         = COALESCE(sqlc.narg('status'), status),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteDormitoryStay :execrows
UPDATE oikumenea.person_dormitory_stays SET deleted_at = now()
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: ListDormitoryStaysByPerson :many
SELECT * FROM oikumenea.person_dormitory_stays
WHERE person_id = @person_id AND deleted_at IS NULL
ORDER BY effective_from DESC NULLS LAST, id;
