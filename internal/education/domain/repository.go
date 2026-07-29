// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"
)

// Repository is the education module's persistence port (implemented by adapters over pgx/sqlc). It is
// bound to a single command surface — the pool for reads, or a caller's transaction for an audited write
// (D-Audit). The application layer owns transaction boundaries; the repository never opens its own.
type Repository interface {
	// catalogs
	ListInstitutionKinds(ctx context.Context) ([]InstitutionKind, error)
	UpsertInstitutionKind(ctx context.Context, code, name string, sortOrder *int) (InstitutionKind, error)
	ListDegreeLevels(ctx context.Context) ([]DegreeLevel, error)

	// institutions — a `university`-domain tenant org + the education_org_profiles sidecar (M41 /
	// D-UnifiedOrgGraph). The org row (code/name/visibility) and the structure tree (tenant units +
	// tenant_unit_closure) are owned by the tenant service; the repository owns the sidecar + read view.
	InsertOrgProfile(ctx context.Context, institutionID, kindID string, countryID, foundedOn, closedOn *string) error
	GetInstitution(ctx context.Context, id string) (Institution, error)
	UpdateOrgProfile(ctx context.Context, id string, up InstitutionUpdate) error
	ListInstitutions(ctx context.Context, query, after string, lim int) ([]Institution, error)
	SoftDeleteInstitution(ctx context.Context, id string) (int64, error)

	// buildings
	InsertBuilding(ctx context.Context, institutionID string, in BuildingInput) (Building, error)
	GetBuilding(ctx context.Context, id string) (Building, error)
	UpdateBuilding(ctx context.Context, id string, up BuildingUpdate) (Building, error)
	ListBuildingsByInstitution(ctx context.Context, institutionID string) ([]Building, error)
	SoftDeleteBuilding(ctx context.Context, id string) (int64, error)

	// groups
	InsertGroup(ctx context.Context, unitID string, in GroupInput) (Group, error)
	GetGroup(ctx context.Context, id string) (Group, error)
	UpdateGroup(ctx context.Context, id string, up GroupUpdate) (Group, error)
	ListGroupsByUnit(ctx context.Context, unitID string) ([]Group, error)
	SoftDeleteGroup(ctx context.Context, id string) (int64, error)

	// positions + appointments
	InsertPosition(ctx context.Context, institutionID string, in PositionInput) (Position, error)
	GetPosition(ctx context.Context, id string) (Position, error)
	UpdatePosition(ctx context.Context, id string, up PositionUpdate) (Position, error)
	AbolishPosition(ctx context.Context, id string) (Position, error)
	ListPositionsByInstitution(ctx context.Context, institutionID, state, after string, lim int) ([]Position, error)
	GetActiveAppointmentByPosition(ctx context.Context, positionID string) (Appointment, error)
	InsertAppointment(ctx context.Context, personID, positionID string, effectiveFrom *string) (Appointment, error)
	GetAppointment(ctx context.Context, id string) (Appointment, error)
	EndAppointment(ctx context.Context, id string, effectiveTo *string) (Appointment, error)
	ListAppointmentsByPerson(ctx context.Context, personID string) ([]PersonAppointment, error)

	// person bindings
	InsertEnrollment(ctx context.Context, personID string, in EnrollmentInput) (Enrollment, error)
	GetEnrollment(ctx context.Context, personID, id string) (Enrollment, error)
	UpdateEnrollment(ctx context.Context, personID, id string, in EnrollmentInput) (Enrollment, error)
	SoftDeleteEnrollment(ctx context.Context, personID, id string) (int64, error)
	ListEnrollmentsByPerson(ctx context.Context, personID string) ([]Enrollment, error)
	InsertDormitoryStay(ctx context.Context, personID string, in DormInput) (DormitoryStay, error)
	GetDormitoryStay(ctx context.Context, personID, id string) (DormitoryStay, error)
	UpdateDormitoryStay(ctx context.Context, personID, id string, in DormInput) (DormitoryStay, error)
	SoftDeleteDormitoryStay(ctx context.Context, personID, id string) (int64, error)
	ListDormitoryStaysByPerson(ctx context.Context, personID string) ([]DormitoryStay, error)

	// ---- reference layer (M20 extension) ----

	// programs
	InsertProgram(ctx context.Context, institutionID string, in ProgramInput) (Program, error)
	GetProgram(ctx context.Context, id string) (Program, error)
	UpdateProgram(ctx context.Context, id string, in ProgramInput) (Program, error)
	ListProgramsByInstitution(ctx context.Context, institutionID string) ([]Program, error)
	SoftDeleteProgram(ctx context.Context, id string) (int64, error)

	// courses
	InsertCourse(ctx context.Context, institutionID string, in CourseInput) (Course, error)
	GetCourse(ctx context.Context, id string) (Course, error)
	UpdateCourse(ctx context.Context, id string, in CourseInput) (Course, error)
	ListCoursesByInstitution(ctx context.Context, institutionID string) ([]Course, error)
	SoftDeleteCourse(ctx context.Context, id string) (int64, error)

	// curriculum versions
	InsertCurriculumVersion(ctx context.Context, programID string, in CurriculumVersionInput) (CurriculumVersion, error)
	GetCurriculumVersion(ctx context.Context, id string) (CurriculumVersion, error)
	UpdateCurriculumVersion(ctx context.Context, id string, in CurriculumVersionInput) (CurriculumVersion, error)
	ListCurriculumVersionsByProgram(ctx context.Context, programID string) ([]CurriculumVersion, error)
	SoftDeleteCurriculumVersion(ctx context.Context, id string) (int64, error)

	// curriculum items
	InsertCurriculumItem(ctx context.Context, versionID string, in CurriculumItemInput) (CurriculumItem, error)
	GetCurriculumItem(ctx context.Context, id string) (CurriculumItem, error)
	UpdateCurriculumItem(ctx context.Context, id string, in CurriculumItemInput) (CurriculumItem, error)
	ListCurriculumItemsByVersion(ctx context.Context, versionID string) ([]CurriculumItem, error)
	SoftDeleteCurriculumItem(ctx context.Context, id string) (int64, error)

	// course prerequisites
	InsertCoursePrerequisite(ctx context.Context, courseID string, in CoursePrerequisiteInput) (CoursePrerequisite, error)
	GetCoursePrerequisite(ctx context.Context, id string) (CoursePrerequisite, error)
	ListCoursePrerequisitesByCourse(ctx context.Context, courseID string) ([]CoursePrerequisite, error)
	SoftDeleteCoursePrerequisite(ctx context.Context, id string) (int64, error)
	ListPrerequisiteEdges(ctx context.Context) ([]PrereqEdge, error)

	// research centres
	InsertResearchCentre(ctx context.Context, institutionID string, in ResearchCentreInput) (ResearchCentre, error)
	GetResearchCentre(ctx context.Context, id string) (ResearchCentre, error)
	UpdateResearchCentre(ctx context.Context, id string, in ResearchCentreInput) (ResearchCentre, error)
	ListResearchCentresByInstitution(ctx context.Context, institutionID string) ([]ResearchCentre, error)
	SoftDeleteResearchCentre(ctx context.Context, id string) (int64, error)

	// research groups
	InsertResearchGroup(ctx context.Context, institutionID string, in ResearchGroupInput) (ResearchGroup, error)
	GetResearchGroup(ctx context.Context, id string) (ResearchGroup, error)
	UpdateResearchGroup(ctx context.Context, id string, in ResearchGroupInput) (ResearchGroup, error)
	ListResearchGroupsByInstitution(ctx context.Context, institutionID string) ([]ResearchGroup, error)
	SoftDeleteResearchGroup(ctx context.Context, id string) (int64, error)

	// grants
	InsertGrant(ctx context.Context, institutionID string, in GrantInput) (Grant, error)
	GetGrant(ctx context.Context, id string) (Grant, error)
	UpdateGrant(ctx context.Context, id string, in GrantInput) (Grant, error)
	ListGrantsByInstitution(ctx context.Context, institutionID string) ([]Grant, error)
	SoftDeleteGrant(ctx context.Context, id string) (int64, error)

	// publications
	InsertPublication(ctx context.Context, in PublicationInput) (Publication, error)
	GetPublication(ctx context.Context, id string) (Publication, error)
	UpdatePublication(ctx context.Context, id string, in PublicationInput) (Publication, error)
	ListPublications(ctx context.Context, query, after string, lim int) ([]Publication, error)
	SoftDeletePublication(ctx context.Context, id string) (int64, error)

	// governance bodies
	InsertGovernanceBody(ctx context.Context, institutionID string, in GovernanceBodyInput) (GovernanceBody, error)
	GetGovernanceBody(ctx context.Context, id string) (GovernanceBody, error)
	UpdateGovernanceBody(ctx context.Context, id string, in GovernanceBodyInput) (GovernanceBody, error)
	ListGovernanceBodiesByInstitution(ctx context.Context, institutionID string) ([]GovernanceBody, error)
	SoftDeleteGovernanceBody(ctx context.Context, id string) (int64, error)

	// policies
	InsertPolicy(ctx context.Context, institutionID string, in PolicyInput) (Policy, error)
	GetPolicy(ctx context.Context, id string) (Policy, error)
	UpdatePolicy(ctx context.Context, id string, in PolicyInput) (Policy, error)
	ListPoliciesByInstitution(ctx context.Context, institutionID string) ([]Policy, error)
	SoftDeletePolicy(ctx context.Context, id string) (int64, error)

	// qualifications
	InsertQualification(ctx context.Context, institutionID string, in QualificationInput) (Qualification, error)
	GetQualification(ctx context.Context, id string) (Qualification, error)
	UpdateQualification(ctx context.Context, id string, in QualificationInput) (Qualification, error)
	ListQualificationsByInstitution(ctx context.Context, institutionID string) ([]Qualification, error)
	SoftDeleteQualification(ctx context.Context, id string) (int64, error)

	// scholarships
	InsertScholarship(ctx context.Context, in ScholarshipInput) (Scholarship, error)
	GetScholarship(ctx context.Context, id string) (Scholarship, error)
	UpdateScholarship(ctx context.Context, id string, in ScholarshipInput) (Scholarship, error)
	ListScholarships(ctx context.Context, query, after string, lim int) ([]Scholarship, error)
	SoftDeleteScholarship(ctx context.Context, id string) (int64, error)

	// accreditation events
	InsertAccreditationEvent(ctx context.Context, in AccreditationEventInput) (AccreditationEvent, error)
	GetAccreditationEvent(ctx context.Context, id string) (AccreditationEvent, error)
	UpdateAccreditationEvent(ctx context.Context, id string, in AccreditationEventInput) (AccreditationEvent, error)
	ListAccreditationEvents(ctx context.Context, institutionID, programID string) ([]AccreditationEvent, error)
	SoftDeleteAccreditationEvent(ctx context.Context, id string) (int64, error)

	// person: publication authorships
	InsertPublicationAuthorship(ctx context.Context, personID string, in PublicationAuthorshipInput) (PublicationAuthorship, error)
	UpdatePublicationAuthorship(ctx context.Context, personID, id string, in PublicationAuthorshipInput) (PublicationAuthorship, error)
	SoftDeletePublicationAuthorship(ctx context.Context, personID, id string) (int64, error)
	ListPublicationAuthorshipsByPerson(ctx context.Context, personID string) ([]PublicationAuthorship, error)

	// person: research memberships
	InsertResearchMembership(ctx context.Context, personID string, in ResearchMembershipInput) (ResearchMembership, error)
	UpdateResearchMembership(ctx context.Context, personID, id string, in ResearchMembershipInput) (ResearchMembership, error)
	SoftDeleteResearchMembership(ctx context.Context, personID, id string) (int64, error)
	ListResearchMembershipsByPerson(ctx context.Context, personID string) ([]ResearchMembership, error)

	// person: grant holdings
	InsertGrantHolding(ctx context.Context, personID string, in GrantHoldingInput) (GrantHolding, error)
	UpdateGrantHolding(ctx context.Context, personID, id string, in GrantHoldingInput) (GrantHolding, error)
	SoftDeleteGrantHolding(ctx context.Context, personID, id string) (int64, error)
	ListGrantHoldingsByPerson(ctx context.Context, personID string) ([]GrantHolding, error)

	// person: governance memberships
	InsertGovernanceMembership(ctx context.Context, personID string, in GovernanceMembershipInput) (GovernanceMembership, error)
	UpdateGovernanceMembership(ctx context.Context, personID, id string, in GovernanceMembershipInput) (GovernanceMembership, error)
	SoftDeleteGovernanceMembership(ctx context.Context, personID, id string) (int64, error)
	ListGovernanceMembershipsByPerson(ctx context.Context, personID string) ([]GovernanceMembership, error)

	// person: qualification awards
	InsertQualificationAward(ctx context.Context, personID string, in QualificationAwardInput) (QualificationAward, error)
	UpdateQualificationAward(ctx context.Context, personID, id string, in QualificationAwardInput) (QualificationAward, error)
	SoftDeleteQualificationAward(ctx context.Context, personID, id string) (int64, error)
	ListQualificationAwardsByPerson(ctx context.Context, personID string) ([]QualificationAward, error)

	// person: scholarship awards
	InsertScholarshipAward(ctx context.Context, personID string, in ScholarshipAwardInput) (ScholarshipAward, error)
	UpdateScholarshipAward(ctx context.Context, personID, id string, in ScholarshipAwardInput) (ScholarshipAward, error)
	SoftDeleteScholarshipAward(ctx context.Context, personID, id string) (int64, error)
	ListScholarshipAwardsByPerson(ctx context.Context, personID string) ([]ScholarshipAward, error)
}

// PrereqEdge is one active prerequisite edge (course requires required), used for the Go-side cycle guard.
type PrereqEdge struct {
	CourseID, RequiredCourseID string
}
