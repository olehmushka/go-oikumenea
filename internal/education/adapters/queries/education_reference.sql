-- Education reference-layer queries (M20 extension / D-Education): curriculum & courses, research,
-- governance/policy, credentials/accreditation, scholarships, and the person↔reference links. RID PKs
-- default at the DB; soft-delete + COALESCE-narg partial updates throughout (a NULL narg leaves the
-- stored value unchanged). FKs validate references (mapped to domain sentinels in the adapter).

-- ============================ programs ============================

-- name: InsertProgram :one
INSERT INTO oikumenea.education_programs
  (institution_id, owning_unit_id, degree_level_id, code, name, mode, duration_years, credit_hours_total)
VALUES (@institution_id, sqlc.narg('owning_unit_id'), sqlc.narg('degree_level_id'), @code, @name,
        COALESCE(sqlc.narg('mode'), 'full_time'), sqlc.narg('duration_years'), sqlc.narg('credit_hours_total'))
RETURNING *;

-- name: GetProgram :one
SELECT * FROM oikumenea.education_programs WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateProgram :one
UPDATE oikumenea.education_programs SET
  name               = COALESCE(sqlc.narg('name'), name),
  owning_unit_id     = COALESCE(sqlc.narg('owning_unit_id'), owning_unit_id),
  degree_level_id    = COALESCE(sqlc.narg('degree_level_id'), degree_level_id),
  mode               = COALESCE(sqlc.narg('mode'), mode),
  duration_years     = COALESCE(sqlc.narg('duration_years'), duration_years),
  credit_hours_total = COALESCE(sqlc.narg('credit_hours_total'), credit_hours_total),
  state              = COALESCE(sqlc.narg('state'), state)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListProgramsByInstitution :many
SELECT * FROM oikumenea.education_programs
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteProgram :execrows
UPDATE oikumenea.education_programs SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ courses ============================

-- name: InsertCourse :one
INSERT INTO oikumenea.education_courses
  (institution_id, owning_unit_id, code, title, credit_hours, level, description, delivery_mode)
VALUES (@institution_id, sqlc.narg('owning_unit_id'), @code, @title, sqlc.narg('credit_hours'),
        sqlc.narg('level'), sqlc.narg('description'), COALESCE(sqlc.narg('delivery_mode'), 'in_person'))
RETURNING *;

-- name: GetCourse :one
SELECT * FROM oikumenea.education_courses WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateCourse :one
UPDATE oikumenea.education_courses SET
  title          = COALESCE(sqlc.narg('title'), title),
  owning_unit_id = COALESCE(sqlc.narg('owning_unit_id'), owning_unit_id),
  credit_hours   = COALESCE(sqlc.narg('credit_hours'), credit_hours),
  level          = COALESCE(sqlc.narg('level'), level),
  description    = COALESCE(sqlc.narg('description'), description),
  delivery_mode  = COALESCE(sqlc.narg('delivery_mode'), delivery_mode),
  status         = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListCoursesByInstitution :many
SELECT * FROM oikumenea.education_courses
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteCourse :execrows
UPDATE oikumenea.education_courses SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ curriculum versions ============================

-- name: InsertCurriculumVersion :one
INSERT INTO oikumenea.education_curriculum_versions (program_id, version_code, effective_from, effective_to, status)
VALUES (@program_id, @version_code, sqlc.narg('effective_from'), sqlc.narg('effective_to'), COALESCE(sqlc.narg('status'), 'draft'))
RETURNING *;

-- name: GetCurriculumVersion :one
SELECT * FROM oikumenea.education_curriculum_versions WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateCurriculumVersion :one
UPDATE oikumenea.education_curriculum_versions SET
  version_code   = COALESCE(sqlc.narg('version_code'), version_code),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to),
  status         = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListCurriculumVersionsByProgram :many
SELECT * FROM oikumenea.education_curriculum_versions
WHERE program_id = @program_id AND deleted_at IS NULL ORDER BY version_code;

-- name: SoftDeleteCurriculumVersion :execrows
UPDATE oikumenea.education_curriculum_versions SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ curriculum items ============================

-- name: InsertCurriculumItem :one
INSERT INTO oikumenea.education_curriculum_items
  (version_id, course_id, is_required, year_of_study, credit_allocation, semester_slot)
VALUES (@version_id, @course_id, COALESCE(sqlc.narg('is_required'), true),
        sqlc.narg('year_of_study'), sqlc.narg('credit_allocation'), sqlc.narg('semester_slot'))
RETURNING *;

-- name: GetCurriculumItem :one
SELECT * FROM oikumenea.education_curriculum_items WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateCurriculumItem :one
UPDATE oikumenea.education_curriculum_items SET
  is_required       = COALESCE(sqlc.narg('is_required'), is_required),
  year_of_study     = COALESCE(sqlc.narg('year_of_study'), year_of_study),
  credit_allocation = COALESCE(sqlc.narg('credit_allocation'), credit_allocation),
  semester_slot     = COALESCE(sqlc.narg('semester_slot'), semester_slot)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListCurriculumItemsByVersion :many
SELECT * FROM oikumenea.education_curriculum_items
WHERE version_id = @version_id AND deleted_at IS NULL ORDER BY year_of_study NULLS LAST, semester_slot NULLS LAST, id;

-- name: SoftDeleteCurriculumItem :execrows
UPDATE oikumenea.education_curriculum_items SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ course prerequisites ============================

-- name: InsertCoursePrerequisite :one
INSERT INTO oikumenea.education_course_prerequisites (course_id, required_course_id, kind, min_grade)
VALUES (@course_id, @required_course_id, COALESCE(sqlc.narg('kind'), 'required'), sqlc.narg('min_grade'))
RETURNING *;

-- name: GetCoursePrerequisite :one
SELECT * FROM oikumenea.education_course_prerequisites WHERE id = @id AND deleted_at IS NULL;

-- name: ListCoursePrerequisitesByCourse :many
SELECT * FROM oikumenea.education_course_prerequisites
WHERE course_id = @course_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeleteCoursePrerequisite :execrows
UPDATE oikumenea.education_course_prerequisites SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- name: ListPrerequisiteEdges :many
-- All active prerequisite edges (course requires required_course). The application walks these in Go to
-- guard against introducing a cycle (sqlc's analyzer can't resolve a recursive-CTE self-join here).
SELECT course_id, required_course_id FROM oikumenea.education_course_prerequisites WHERE deleted_at IS NULL;

-- ============================ research centres ============================

-- name: InsertResearchCentre :one
INSERT INTO oikumenea.education_research_centres (institution_id, code, name, kind, funding_source, founded_on, dissolved_on)
VALUES (@institution_id, @code, @name, COALESCE(sqlc.narg('kind'), 'centre'), sqlc.narg('funding_source'),
        sqlc.narg('founded_on'), sqlc.narg('dissolved_on'))
RETURNING *;

-- name: GetResearchCentre :one
SELECT * FROM oikumenea.education_research_centres WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateResearchCentre :one
UPDATE oikumenea.education_research_centres SET
  name           = COALESCE(sqlc.narg('name'), name),
  kind           = COALESCE(sqlc.narg('kind'), kind),
  funding_source = COALESCE(sqlc.narg('funding_source'), funding_source),
  founded_on     = COALESCE(sqlc.narg('founded_on'), founded_on),
  dissolved_on   = COALESCE(sqlc.narg('dissolved_on'), dissolved_on),
  status         = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListResearchCentresByInstitution :many
SELECT * FROM oikumenea.education_research_centres
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteResearchCentre :execrows
UPDATE oikumenea.education_research_centres SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ research groups ============================

-- name: InsertResearchGroup :one
INSERT INTO oikumenea.education_research_groups (institution_id, centre_id, unit_id, code, name, focus_area)
VALUES (@institution_id, sqlc.narg('centre_id'), sqlc.narg('unit_id'), @code, @name, sqlc.narg('focus_area'))
RETURNING *;

-- name: GetResearchGroup :one
SELECT * FROM oikumenea.education_research_groups WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateResearchGroup :one
UPDATE oikumenea.education_research_groups SET
  name       = COALESCE(sqlc.narg('name'), name),
  centre_id  = COALESCE(sqlc.narg('centre_id'), centre_id),
  unit_id    = COALESCE(sqlc.narg('unit_id'), unit_id),
  focus_area = COALESCE(sqlc.narg('focus_area'), focus_area),
  status     = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListResearchGroupsByInstitution :many
SELECT * FROM oikumenea.education_research_groups
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteResearchGroup :execrows
UPDATE oikumenea.education_research_groups SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ grants ============================

-- name: InsertGrant :one
INSERT INTO oikumenea.education_grants (institution_id, code, title, funder, funder_ref, amount, currency, start_on, end_on, status)
VALUES (@institution_id, @code, @title, sqlc.narg('funder'), sqlc.narg('funder_ref'), sqlc.narg('amount'),
        sqlc.narg('currency'), sqlc.narg('start_on'), sqlc.narg('end_on'), COALESCE(sqlc.narg('status'), 'awarded'))
RETURNING *;

-- name: GetGrant :one
SELECT * FROM oikumenea.education_grants WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateGrant :one
UPDATE oikumenea.education_grants SET
  title      = COALESCE(sqlc.narg('title'), title),
  funder     = COALESCE(sqlc.narg('funder'), funder),
  funder_ref = COALESCE(sqlc.narg('funder_ref'), funder_ref),
  amount     = COALESCE(sqlc.narg('amount'), amount),
  currency   = COALESCE(sqlc.narg('currency'), currency),
  start_on   = COALESCE(sqlc.narg('start_on'), start_on),
  end_on     = COALESCE(sqlc.narg('end_on'), end_on),
  status     = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListGrantsByInstitution :many
SELECT * FROM oikumenea.education_grants
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteGrant :execrows
UPDATE oikumenea.education_grants SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ publications ============================

-- name: InsertPublication :one
INSERT INTO oikumenea.education_publications (institution_id, code, title, kind, doi, venue, published_on, open_access)
VALUES (sqlc.narg('institution_id'), @code, @title, COALESCE(sqlc.narg('kind'), 'journal_article'),
        sqlc.narg('doi'), sqlc.narg('venue'), sqlc.narg('published_on'), COALESCE(sqlc.narg('open_access'), false))
RETURNING *;

-- name: GetPublication :one
SELECT * FROM oikumenea.education_publications WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePublication :one
UPDATE oikumenea.education_publications SET
  title          = COALESCE(sqlc.narg('title'), title),
  institution_id = COALESCE(sqlc.narg('institution_id'), institution_id),
  kind           = COALESCE(sqlc.narg('kind'), kind),
  doi            = COALESCE(sqlc.narg('doi'), doi),
  venue          = COALESCE(sqlc.narg('venue'), venue),
  published_on   = COALESCE(sqlc.narg('published_on'), published_on),
  open_access    = COALESCE(sqlc.narg('open_access'), open_access)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPublications :many
-- Keyset-paginated by RID (review R-21: was an unbounded ORDER BY code full scan). A text query routes
-- to SearchPublications so the trigram GIN is not defeated by a `(@query = '' OR …)` guard.
SELECT * FROM oikumenea.education_publications
WHERE deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: SearchPublications :many
-- The trigram-served twin of ListPublications (review R-21): an unconditional match over the STORED
-- search_text haystack served by the education_publications_search_trgm GIN, keyset-paginated by RID.
SELECT * FROM oikumenea.education_publications
WHERE deleted_at IS NULL
  AND search_text ILIKE '%' || @query || '%'
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: SoftDeletePublication :execrows
UPDATE oikumenea.education_publications SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ governance bodies ============================

-- name: InsertGovernanceBody :one
INSERT INTO oikumenea.education_governance_bodies (institution_id, code, name, kind, mandate)
VALUES (@institution_id, @code, @name, COALESCE(sqlc.narg('kind'), 'committee'), sqlc.narg('mandate'))
RETURNING *;

-- name: GetGovernanceBody :one
SELECT * FROM oikumenea.education_governance_bodies WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateGovernanceBody :one
UPDATE oikumenea.education_governance_bodies SET
  name    = COALESCE(sqlc.narg('name'), name),
  kind    = COALESCE(sqlc.narg('kind'), kind),
  mandate = COALESCE(sqlc.narg('mandate'), mandate),
  status  = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListGovernanceBodiesByInstitution :many
SELECT * FROM oikumenea.education_governance_bodies
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteGovernanceBody :execrows
UPDATE oikumenea.education_governance_bodies SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ policies ============================

-- name: InsertPolicy :one
INSERT INTO oikumenea.education_policies
  (institution_id, governance_body_id, supersedes_id, code, title, kind, effective_on, expiry_on, document_url)
VALUES (@institution_id, sqlc.narg('governance_body_id'), sqlc.narg('supersedes_id'), @code, @title,
        COALESCE(sqlc.narg('kind'), 'academic'), sqlc.narg('effective_on'), sqlc.narg('expiry_on'), sqlc.narg('document_url'))
RETURNING *;

-- name: GetPolicy :one
SELECT * FROM oikumenea.education_policies WHERE id = @id AND deleted_at IS NULL;

-- name: UpdatePolicy :one
UPDATE oikumenea.education_policies SET
  title              = COALESCE(sqlc.narg('title'), title),
  governance_body_id = COALESCE(sqlc.narg('governance_body_id'), governance_body_id),
  supersedes_id      = COALESCE(sqlc.narg('supersedes_id'), supersedes_id),
  kind               = COALESCE(sqlc.narg('kind'), kind),
  effective_on       = COALESCE(sqlc.narg('effective_on'), effective_on),
  expiry_on          = COALESCE(sqlc.narg('expiry_on'), expiry_on),
  document_url       = COALESCE(sqlc.narg('document_url'), document_url),
  status             = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListPoliciesByInstitution :many
SELECT * FROM oikumenea.education_policies
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeletePolicy :execrows
UPDATE oikumenea.education_policies SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ qualifications ============================

-- name: InsertQualification :one
INSERT INTO oikumenea.education_qualifications
  (institution_id, program_id, degree_level_id, code, name, framework_code, framework_level, awarding_body)
VALUES (@institution_id, sqlc.narg('program_id'), sqlc.narg('degree_level_id'), @code, @name,
        sqlc.narg('framework_code'), sqlc.narg('framework_level'), sqlc.narg('awarding_body'))
RETURNING *;

-- name: GetQualification :one
SELECT * FROM oikumenea.education_qualifications WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateQualification :one
UPDATE oikumenea.education_qualifications SET
  name            = COALESCE(sqlc.narg('name'), name),
  program_id      = COALESCE(sqlc.narg('program_id'), program_id),
  degree_level_id = COALESCE(sqlc.narg('degree_level_id'), degree_level_id),
  framework_code  = COALESCE(sqlc.narg('framework_code'), framework_code),
  framework_level = COALESCE(sqlc.narg('framework_level'), framework_level),
  awarding_body   = COALESCE(sqlc.narg('awarding_body'), awarding_body),
  status          = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListQualificationsByInstitution :many
SELECT * FROM oikumenea.education_qualifications
WHERE institution_id = @institution_id AND deleted_at IS NULL ORDER BY code;

-- name: SoftDeleteQualification :execrows
UPDATE oikumenea.education_qualifications SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ scholarships ============================

-- name: InsertScholarship :one
INSERT INTO oikumenea.education_scholarships (institution_id, code, name, kind, amount, currency, frequency, renewable, conditions)
VALUES (sqlc.narg('institution_id'), @code, @name, COALESCE(sqlc.narg('kind'), 'merit'), sqlc.narg('amount'),
        sqlc.narg('currency'), COALESCE(sqlc.narg('frequency'), 'annual'), COALESCE(sqlc.narg('renewable'), false), sqlc.narg('conditions'))
RETURNING *;

-- name: GetScholarship :one
SELECT * FROM oikumenea.education_scholarships WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateScholarship :one
UPDATE oikumenea.education_scholarships SET
  name           = COALESCE(sqlc.narg('name'), name),
  institution_id = COALESCE(sqlc.narg('institution_id'), institution_id),
  kind           = COALESCE(sqlc.narg('kind'), kind),
  amount         = COALESCE(sqlc.narg('amount'), amount),
  currency       = COALESCE(sqlc.narg('currency'), currency),
  frequency      = COALESCE(sqlc.narg('frequency'), frequency),
  renewable      = COALESCE(sqlc.narg('renewable'), renewable),
  conditions     = COALESCE(sqlc.narg('conditions'), conditions),
  status         = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListScholarships :many
-- Keyset-paginated by RID (review R-21: was an unbounded ORDER BY code full scan). A text query routes
-- to SearchScholarships so the trigram GIN is not defeated by a `(@query = '' OR …)` guard.
SELECT * FROM oikumenea.education_scholarships
WHERE deleted_at IS NULL
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: SearchScholarships :many
-- The trigram-served twin of ListScholarships (review R-21): an unconditional match over the STORED
-- search_text haystack served by the education_scholarships_search_trgm GIN, keyset-paginated by RID.
SELECT * FROM oikumenea.education_scholarships
WHERE deleted_at IS NULL
  AND search_text ILIKE '%' || @query || '%'
  AND (@after = '' OR id::text > @after)
ORDER BY id
LIMIT @lim;

-- name: SoftDeleteScholarship :execrows
UPDATE oikumenea.education_scholarships SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ accreditation events ============================

-- name: InsertAccreditationEvent :one
INSERT INTO oikumenea.education_accreditation_events
  (entity_kind, institution_id, program_id, body, body_country_id, outcome, review_on, effective_from, effective_to, notes)
VALUES (@entity_kind, sqlc.narg('institution_id'), sqlc.narg('program_id'), sqlc.narg('body'),
        sqlc.narg('body_country_id'), COALESCE(sqlc.narg('outcome'), 'pending'), sqlc.narg('review_on'),
        sqlc.narg('effective_from'), sqlc.narg('effective_to'), sqlc.narg('notes'))
RETURNING *;

-- name: GetAccreditationEvent :one
SELECT * FROM oikumenea.education_accreditation_events WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateAccreditationEvent :one
UPDATE oikumenea.education_accreditation_events SET
  body            = COALESCE(sqlc.narg('body'), body),
  body_country_id = COALESCE(sqlc.narg('body_country_id'), body_country_id),
  outcome         = COALESCE(sqlc.narg('outcome'), outcome),
  review_on       = COALESCE(sqlc.narg('review_on'), review_on),
  effective_from  = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to    = COALESCE(sqlc.narg('effective_to'), effective_to),
  notes           = COALESCE(sqlc.narg('notes'), notes)
WHERE id = @id AND deleted_at IS NULL
RETURNING *;

-- name: ListAccreditationEvents :many
SELECT * FROM oikumenea.education_accreditation_events
WHERE deleted_at IS NULL
  AND (@institution_id = '' OR institution_id = @institution_id)
  AND (@program_id = '' OR program_id = @program_id)
ORDER BY review_on DESC NULLS LAST, id;

-- name: SoftDeleteAccreditationEvent :execrows
UPDATE oikumenea.education_accreditation_events SET deleted_at = now() WHERE id = @id AND deleted_at IS NULL;

-- ============================ person: publication authorships ============================

-- name: InsertPublicationAuthorship :one
INSERT INTO oikumenea.person_publication_authorships (person_id, publication_id, author_order, corresponding, effective_from, effective_to)
VALUES (@person_id, @publication_id, sqlc.narg('author_order'), COALESCE(sqlc.narg('corresponding'), false),
        sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetPublicationAuthorship :one
SELECT * FROM oikumenea.person_publication_authorships WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdatePublicationAuthorship :one
UPDATE oikumenea.person_publication_authorships SET
  publication_id = COALESCE(sqlc.narg('publication_id'), publication_id),
  author_order   = COALESCE(sqlc.narg('author_order'), author_order),
  corresponding  = COALESCE(sqlc.narg('corresponding'), corresponding),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListPublicationAuthorshipsByPerson :many
SELECT * FROM oikumenea.person_publication_authorships
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeletePublicationAuthorship :execrows
UPDATE oikumenea.person_publication_authorships SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- ============================ person: research memberships ============================

-- name: InsertResearchMembership :one
INSERT INTO oikumenea.person_research_memberships (person_id, group_id, role, status, effective_from, effective_to)
VALUES (@person_id, @group_id, sqlc.narg('role'), COALESCE(sqlc.narg('status'), 'active'), sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetResearchMembership :one
SELECT * FROM oikumenea.person_research_memberships WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateResearchMembership :one
UPDATE oikumenea.person_research_memberships SET
  group_id       = COALESCE(sqlc.narg('group_id'), group_id),
  role           = COALESCE(sqlc.narg('role'), role),
  status         = COALESCE(sqlc.narg('status'), status),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListResearchMembershipsByPerson :many
SELECT * FROM oikumenea.person_research_memberships
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeleteResearchMembership :execrows
UPDATE oikumenea.person_research_memberships SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- ============================ person: grant holdings ============================

-- name: InsertGrantHolding :one
INSERT INTO oikumenea.person_grant_holdings (person_id, grant_id, role, status, effective_from, effective_to)
VALUES (@person_id, @grant_id, COALESCE(sqlc.narg('role'), 'pi'), COALESCE(sqlc.narg('status'), 'active'), sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetGrantHolding :one
SELECT * FROM oikumenea.person_grant_holdings WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateGrantHolding :one
UPDATE oikumenea.person_grant_holdings SET
  grant_id       = COALESCE(sqlc.narg('grant_id'), grant_id),
  role           = COALESCE(sqlc.narg('role'), role),
  status         = COALESCE(sqlc.narg('status'), status),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListGrantHoldingsByPerson :many
SELECT * FROM oikumenea.person_grant_holdings
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeleteGrantHolding :execrows
UPDATE oikumenea.person_grant_holdings SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- ============================ person: governance memberships ============================

-- name: InsertGovernanceMembership :one
INSERT INTO oikumenea.person_governance_memberships (person_id, body_id, role_in_body, status, effective_from, effective_to)
VALUES (@person_id, @body_id, sqlc.narg('role_in_body'), COALESCE(sqlc.narg('status'), 'active'), sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetGovernanceMembership :one
SELECT * FROM oikumenea.person_governance_memberships WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateGovernanceMembership :one
UPDATE oikumenea.person_governance_memberships SET
  body_id        = COALESCE(sqlc.narg('body_id'), body_id),
  role_in_body   = COALESCE(sqlc.narg('role_in_body'), role_in_body),
  status         = COALESCE(sqlc.narg('status'), status),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListGovernanceMembershipsByPerson :many
SELECT * FROM oikumenea.person_governance_memberships
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeleteGovernanceMembership :execrows
UPDATE oikumenea.person_governance_memberships SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- ============================ person: qualification awards ============================

-- name: InsertQualificationAward :one
INSERT INTO oikumenea.person_education_qualifications (person_id, qualification_id, enrollment_id, awarded_on, with_distinction, gpa, status)
VALUES (@person_id, @qualification_id, sqlc.narg('enrollment_id'), sqlc.narg('awarded_on'),
        COALESCE(sqlc.narg('with_distinction'), false), sqlc.narg('gpa'), COALESCE(sqlc.narg('status'), 'awarded'))
RETURNING *;

-- name: GetQualificationAward :one
SELECT * FROM oikumenea.person_education_qualifications WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateQualificationAward :one
UPDATE oikumenea.person_education_qualifications SET
  qualification_id = COALESCE(sqlc.narg('qualification_id'), qualification_id),
  enrollment_id    = COALESCE(sqlc.narg('enrollment_id'), enrollment_id),
  awarded_on       = COALESCE(sqlc.narg('awarded_on'), awarded_on),
  with_distinction = COALESCE(sqlc.narg('with_distinction'), with_distinction),
  gpa              = COALESCE(sqlc.narg('gpa'), gpa),
  status           = COALESCE(sqlc.narg('status'), status)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListQualificationAwardsByPerson :many
SELECT * FROM oikumenea.person_education_qualifications
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY awarded_on DESC NULLS LAST, id;

-- name: SoftDeleteQualificationAward :execrows
UPDATE oikumenea.person_education_qualifications SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- ============================ person: scholarship awards ============================

-- name: InsertScholarshipAward :one
INSERT INTO oikumenea.person_scholarship_awards (person_id, scholarship_id, status, effective_from, effective_to)
VALUES (@person_id, @scholarship_id, COALESCE(sqlc.narg('status'), 'active'), sqlc.narg('effective_from'), sqlc.narg('effective_to'))
RETURNING *;

-- name: GetScholarshipAward :one
SELECT * FROM oikumenea.person_scholarship_awards WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

-- name: UpdateScholarshipAward :one
UPDATE oikumenea.person_scholarship_awards SET
  scholarship_id = COALESCE(sqlc.narg('scholarship_id'), scholarship_id),
  status         = COALESCE(sqlc.narg('status'), status),
  effective_from = COALESCE(sqlc.narg('effective_from'), effective_from),
  effective_to   = COALESCE(sqlc.narg('effective_to'), effective_to)
WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL
RETURNING *;

-- name: ListScholarshipAwardsByPerson :many
SELECT * FROM oikumenea.person_scholarship_awards
WHERE person_id = @person_id AND deleted_at IS NULL ORDER BY id;

-- name: SoftDeleteScholarshipAward :execrows
UPDATE oikumenea.person_scholarship_awards SET deleted_at = now() WHERE id = @id AND person_id = @person_id AND deleted_at IS NULL;

