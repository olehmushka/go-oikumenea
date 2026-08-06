// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Reference-layer transport (M20 extension): implements the generated educationrefapi
// .EducationReferenceService. PEP-gates each op (instance-global reference data — satisfied anywhere),
// maps requests↔domain, and translates domain sentinels to the educationref Conjure errors. Unlike the
// M20 base service, reference names/titles are plain strings (no i18n assembly).
package transport

import (
	"context"
	"errors"

	authzdomain "github.com/olehmushka/go-oikumenea/internal/authorization/domain"
	"github.com/olehmushka/go-oikumenea/internal/authorization/pep"
	educationrefapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/educationref"
	"github.com/olehmushka/go-oikumenea/internal/education/application"
	"github.com/olehmushka/go-oikumenea/internal/education/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// ReferenceService adapts *application.Service to educationrefapi.EducationReferenceService.
type ReferenceService struct {
	app    *application.Service
	pep    *pep.Enforcer
	person PersonReader
}

// NewReferenceService builds the reference transport adapter over the education application service.
// The person reader answers the holder read scope for the six person-binding reads below (M58 ticket
// 7) — see EducationService.holderReadable for why every one of them needs it.
func NewReferenceService(app *application.Service, enforcer *pep.Enforcer, person PersonReader) ReferenceService {
	return ReferenceService{app: app, pep: enforcer, person: person}
}

// holderReadable is the ReferenceService's copy of the EducationService probe: instance admins pass,
// everyone else is answered by the person reader (D-PersonReadScope). Duplicated as a two-line method
// rather than shared through a base struct, because the two transports are separate generated
// interfaces and a shared embedded helper would make the gate easy to inherit without choosing it —
// the same reasoning that made gateOrgs a sibling of gateUnits rather than a flag on it (ticket 4).
func (s ReferenceService) holderReadable(ctx context.Context, personID string) (bool, error) {
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	return s.person.ReadablePerson(ctx, subject, personID)
}

var _ educationrefapi.EducationReferenceService = ReferenceService{}

const refManagePerm = string(authzdomain.PermEducationManage)
const refReadPerm = string(authzdomain.PermEducationRead)
const refLinkPerm = string(authzdomain.PermEducationEnrollmentManage)

// ============================ programs ============================

func (s ReferenceService) CreateProgram(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertProgramRequest) (educationrefapi.Program, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Program{}, err
	}
	v, err := s.app.CreateProgram(ctx, institutionID, domain.ProgramInput{
		Code: req.Code, Name: req.Name, OwningUnitID: req.OwningUnitId, DegreeLevelID: req.DegreeLevelId,
		Mode: req.Mode, DurationYears: req.DurationYears, CreditHoursTotal: req.CreditHoursTotal, State: req.State,
	})
	if err != nil {
		return educationrefapi.Program{}, s.err(ctx, err)
	}
	return programAPI(v), nil
}

func (s ReferenceService) ListPrograms(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.ProgramList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ProgramList{}, err
	}
	rows, err := s.app.ListPrograms(ctx, institutionID)
	if err != nil {
		return educationrefapi.ProgramList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.Program, 0, len(rows))
	for _, v := range rows {
		out = append(out, programAPI(v))
	}
	return educationrefapi.ProgramList{Programs: out}, nil
}

func (s ReferenceService) GetProgram(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Program, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Program{}, err
	}
	v, err := s.app.GetProgram(ctx, id)
	if err != nil {
		return educationrefapi.Program{}, s.err(ctx, err)
	}
	return programAPI(v), nil
}

func (s ReferenceService) UpdateProgram(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertProgramRequest) (educationrefapi.Program, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Program{}, err
	}
	v, err := s.app.UpdateProgram(ctx, id, domain.ProgramInput{
		Code: req.Code, Name: req.Name, OwningUnitID: req.OwningUnitId, DegreeLevelID: req.DegreeLevelId,
		Mode: req.Mode, DurationYears: req.DurationYears, CreditHoursTotal: req.CreditHoursTotal, State: req.State,
	})
	if err != nil {
		return educationrefapi.Program{}, s.err(ctx, err)
	}
	return programAPI(v), nil
}

func (s ReferenceService) DeleteProgram(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteProgram(ctx, id))
}

// ============================ courses ============================

func (s ReferenceService) CreateCourse(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertCourseRequest) (educationrefapi.Course, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Course{}, err
	}
	v, err := s.app.CreateCourse(ctx, institutionID, courseInput(req))
	if err != nil {
		return educationrefapi.Course{}, s.err(ctx, err)
	}
	return courseAPI(v), nil
}

func (s ReferenceService) ListCourses(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.CourseList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.CourseList{}, err
	}
	rows, err := s.app.ListCourses(ctx, institutionID)
	if err != nil {
		return educationrefapi.CourseList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.Course, 0, len(rows))
	for _, v := range rows {
		out = append(out, courseAPI(v))
	}
	return educationrefapi.CourseList{Courses: out}, nil
}

func (s ReferenceService) GetCourse(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Course, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Course{}, err
	}
	v, err := s.app.GetCourse(ctx, id)
	if err != nil {
		return educationrefapi.Course{}, s.err(ctx, err)
	}
	return courseAPI(v), nil
}

func (s ReferenceService) UpdateCourse(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertCourseRequest) (educationrefapi.Course, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Course{}, err
	}
	v, err := s.app.UpdateCourse(ctx, id, courseInput(req))
	if err != nil {
		return educationrefapi.Course{}, s.err(ctx, err)
	}
	return courseAPI(v), nil
}

func (s ReferenceService) DeleteCourse(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteCourse(ctx, id))
}

// ============================ curriculum versions ============================

func (s ReferenceService) CreateCurriculumVersion(ctx context.Context, t bearertoken.Token, programID string, req educationrefapi.UpsertCurriculumVersionRequest) (educationrefapi.CurriculumVersion, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.CurriculumVersion{}, err
	}
	v, err := s.app.CreateCurriculumVersion(ctx, programID, domain.CurriculumVersionInput{VersionCode: req.VersionCode, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo, Status: req.Status})
	if err != nil {
		return educationrefapi.CurriculumVersion{}, s.err(ctx, err)
	}
	return curriculumVersionAPI(v), nil
}

func (s ReferenceService) ListCurriculumVersions(ctx context.Context, t bearertoken.Token, programID string) (educationrefapi.CurriculumVersionList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.CurriculumVersionList{}, err
	}
	rows, err := s.app.ListCurriculumVersions(ctx, programID)
	if err != nil {
		return educationrefapi.CurriculumVersionList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.CurriculumVersion, 0, len(rows))
	for _, v := range rows {
		out = append(out, curriculumVersionAPI(v))
	}
	return educationrefapi.CurriculumVersionList{Versions: out}, nil
}

func (s ReferenceService) GetCurriculumVersion(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.CurriculumVersion, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.CurriculumVersion{}, err
	}
	v, err := s.app.GetCurriculumVersion(ctx, id)
	if err != nil {
		return educationrefapi.CurriculumVersion{}, s.err(ctx, err)
	}
	return curriculumVersionAPI(v), nil
}

func (s ReferenceService) UpdateCurriculumVersion(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertCurriculumVersionRequest) (educationrefapi.CurriculumVersion, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.CurriculumVersion{}, err
	}
	v, err := s.app.UpdateCurriculumVersion(ctx, id, domain.CurriculumVersionInput{VersionCode: req.VersionCode, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo, Status: req.Status})
	if err != nil {
		return educationrefapi.CurriculumVersion{}, s.err(ctx, err)
	}
	return curriculumVersionAPI(v), nil
}

func (s ReferenceService) DeleteCurriculumVersion(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteCurriculumVersion(ctx, id))
}

// ============================ curriculum items ============================

func (s ReferenceService) AddCurriculumItem(ctx context.Context, t bearertoken.Token, versionID string, req educationrefapi.UpsertCurriculumItemRequest) (educationrefapi.CurriculumItem, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.CurriculumItem{}, err
	}
	v, err := s.app.AddCurriculumItem(ctx, versionID, curriculumItemInput(req))
	if err != nil {
		return educationrefapi.CurriculumItem{}, s.err(ctx, err)
	}
	return curriculumItemAPI(v), nil
}

func (s ReferenceService) ListCurriculumItems(ctx context.Context, t bearertoken.Token, versionID string) (educationrefapi.CurriculumItemList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.CurriculumItemList{}, err
	}
	rows, err := s.app.ListCurriculumItems(ctx, versionID)
	if err != nil {
		return educationrefapi.CurriculumItemList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.CurriculumItem, 0, len(rows))
	for _, v := range rows {
		out = append(out, curriculumItemAPI(v))
	}
	return educationrefapi.CurriculumItemList{Items: out}, nil
}

func (s ReferenceService) UpdateCurriculumItem(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertCurriculumItemRequest) (educationrefapi.CurriculumItem, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.CurriculumItem{}, err
	}
	v, err := s.app.UpdateCurriculumItem(ctx, id, curriculumItemInput(req))
	if err != nil {
		return educationrefapi.CurriculumItem{}, s.err(ctx, err)
	}
	return curriculumItemAPI(v), nil
}

func (s ReferenceService) DeleteCurriculumItem(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteCurriculumItem(ctx, id))
}

// ============================ course prerequisites ============================

func (s ReferenceService) AddCoursePrerequisite(ctx context.Context, t bearertoken.Token, courseID string, req educationrefapi.CreateCoursePrerequisiteRequest) (educationrefapi.CoursePrerequisite, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.CoursePrerequisite{}, err
	}
	v, err := s.app.AddCoursePrerequisite(ctx, courseID, domain.CoursePrerequisiteInput{RequiredCourseID: req.RequiredCourseId, Kind: req.Kind, MinGrade: req.MinGrade})
	if err != nil {
		return educationrefapi.CoursePrerequisite{}, s.err(ctx, err)
	}
	return coursePrereqAPI(v), nil
}

func (s ReferenceService) ListCoursePrerequisites(ctx context.Context, t bearertoken.Token, courseID string) (educationrefapi.CoursePrerequisiteList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.CoursePrerequisiteList{}, err
	}
	rows, err := s.app.ListCoursePrerequisites(ctx, courseID)
	if err != nil {
		return educationrefapi.CoursePrerequisiteList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.CoursePrerequisite, 0, len(rows))
	for _, v := range rows {
		out = append(out, coursePrereqAPI(v))
	}
	return educationrefapi.CoursePrerequisiteList{Prerequisites: out}, nil
}

func (s ReferenceService) DeleteCoursePrerequisite(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteCoursePrerequisite(ctx, id))
}

// ============================ research centres ============================

func (s ReferenceService) CreateResearchCentre(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertResearchCentreRequest) (educationrefapi.ResearchCentre, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.ResearchCentre{}, err
	}
	v, err := s.app.CreateResearchCentre(ctx, institutionID, researchCentreInput(req))
	if err != nil {
		return educationrefapi.ResearchCentre{}, s.err(ctx, err)
	}
	return researchCentreAPI(v), nil
}

func (s ReferenceService) ListResearchCentres(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.ResearchCentreList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ResearchCentreList{}, err
	}
	rows, err := s.app.ListResearchCentres(ctx, institutionID)
	if err != nil {
		return educationrefapi.ResearchCentreList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.ResearchCentre, 0, len(rows))
	for _, v := range rows {
		out = append(out, researchCentreAPI(v))
	}
	return educationrefapi.ResearchCentreList{ResearchCentres: out}, nil
}

func (s ReferenceService) GetResearchCentre(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.ResearchCentre, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ResearchCentre{}, err
	}
	v, err := s.app.GetResearchCentre(ctx, id)
	if err != nil {
		return educationrefapi.ResearchCentre{}, s.err(ctx, err)
	}
	return researchCentreAPI(v), nil
}

func (s ReferenceService) UpdateResearchCentre(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertResearchCentreRequest) (educationrefapi.ResearchCentre, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.ResearchCentre{}, err
	}
	v, err := s.app.UpdateResearchCentre(ctx, id, researchCentreInput(req))
	if err != nil {
		return educationrefapi.ResearchCentre{}, s.err(ctx, err)
	}
	return researchCentreAPI(v), nil
}

func (s ReferenceService) DeleteResearchCentre(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteResearchCentre(ctx, id))
}

// ============================ research groups ============================

func (s ReferenceService) CreateResearchGroup(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertResearchGroupRequest) (educationrefapi.ResearchGroup, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.ResearchGroup{}, err
	}
	v, err := s.app.CreateResearchGroup(ctx, institutionID, researchGroupInput(req))
	if err != nil {
		return educationrefapi.ResearchGroup{}, s.err(ctx, err)
	}
	return researchGroupAPI(v), nil
}

func (s ReferenceService) ListResearchGroups(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.ResearchGroupList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ResearchGroupList{}, err
	}
	rows, err := s.app.ListResearchGroups(ctx, institutionID)
	if err != nil {
		return educationrefapi.ResearchGroupList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.ResearchGroup, 0, len(rows))
	for _, v := range rows {
		out = append(out, researchGroupAPI(v))
	}
	return educationrefapi.ResearchGroupList{ResearchGroups: out}, nil
}

func (s ReferenceService) GetResearchGroup(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.ResearchGroup, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ResearchGroup{}, err
	}
	v, err := s.app.GetResearchGroup(ctx, id)
	if err != nil {
		return educationrefapi.ResearchGroup{}, s.err(ctx, err)
	}
	return researchGroupAPI(v), nil
}

func (s ReferenceService) UpdateResearchGroup(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertResearchGroupRequest) (educationrefapi.ResearchGroup, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.ResearchGroup{}, err
	}
	v, err := s.app.UpdateResearchGroup(ctx, id, researchGroupInput(req))
	if err != nil {
		return educationrefapi.ResearchGroup{}, s.err(ctx, err)
	}
	return researchGroupAPI(v), nil
}

func (s ReferenceService) DeleteResearchGroup(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteResearchGroup(ctx, id))
}

// ============================ grants ============================

func (s ReferenceService) CreateGrant(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertGrantRequest) (educationrefapi.Grant, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Grant{}, err
	}
	v, err := s.app.CreateGrant(ctx, institutionID, grantInput(req))
	if err != nil {
		return educationrefapi.Grant{}, s.err(ctx, err)
	}
	return grantAPI(v), nil
}

func (s ReferenceService) ListGrants(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.GrantList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.GrantList{}, err
	}
	rows, err := s.app.ListGrants(ctx, institutionID)
	if err != nil {
		return educationrefapi.GrantList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.Grant, 0, len(rows))
	for _, v := range rows {
		out = append(out, grantAPI(v))
	}
	return educationrefapi.GrantList{Grants: out}, nil
}

func (s ReferenceService) GetGrant(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Grant, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Grant{}, err
	}
	v, err := s.app.GetGrant(ctx, id)
	if err != nil {
		return educationrefapi.Grant{}, s.err(ctx, err)
	}
	return grantAPI(v), nil
}

func (s ReferenceService) UpdateGrant(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertGrantRequest) (educationrefapi.Grant, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Grant{}, err
	}
	v, err := s.app.UpdateGrant(ctx, id, grantInput(req))
	if err != nil {
		return educationrefapi.Grant{}, s.err(ctx, err)
	}
	return grantAPI(v), nil
}

func (s ReferenceService) DeleteGrant(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteGrant(ctx, id))
}

// ============================ publications ============================

func (s ReferenceService) CreatePublication(ctx context.Context, t bearertoken.Token, req educationrefapi.UpsertPublicationRequest) (educationrefapi.Publication, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Publication{}, err
	}
	v, err := s.app.CreatePublication(ctx, publicationInput(req))
	if err != nil {
		return educationrefapi.Publication{}, s.err(ctx, err)
	}
	return publicationAPI(v), nil
}

func (s ReferenceService) ListPublications(ctx context.Context, t bearertoken.Token, query *string, pageSize *int, pageToken *string) (educationrefapi.PublicationPage, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.PublicationPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListPublications(ctx, strOr(query), decodeToken(pageToken), limit)
	if err != nil {
		return educationrefapi.PublicationPage{}, s.err(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out := make([]educationrefapi.Publication, 0, len(rows))
	for _, v := range rows {
		out = append(out, publicationAPI(v))
	}
	page := educationrefapi.PublicationPage{Publications: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s ReferenceService) GetPublication(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Publication, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Publication{}, err
	}
	v, err := s.app.GetPublication(ctx, id)
	if err != nil {
		return educationrefapi.Publication{}, s.err(ctx, err)
	}
	return publicationAPI(v), nil
}

func (s ReferenceService) UpdatePublication(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertPublicationRequest) (educationrefapi.Publication, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Publication{}, err
	}
	v, err := s.app.UpdatePublication(ctx, id, publicationInput(req))
	if err != nil {
		return educationrefapi.Publication{}, s.err(ctx, err)
	}
	return publicationAPI(v), nil
}

func (s ReferenceService) DeletePublication(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeletePublication(ctx, id))
}

// ============================ governance bodies ============================

func (s ReferenceService) CreateGovernanceBody(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertGovernanceBodyRequest) (educationrefapi.GovernanceBody, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.GovernanceBody{}, err
	}
	v, err := s.app.CreateGovernanceBody(ctx, institutionID, governanceBodyInput(req))
	if err != nil {
		return educationrefapi.GovernanceBody{}, s.err(ctx, err)
	}
	return governanceBodyAPI(v), nil
}

func (s ReferenceService) ListGovernanceBodies(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.GovernanceBodyList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.GovernanceBodyList{}, err
	}
	rows, err := s.app.ListGovernanceBodies(ctx, institutionID)
	if err != nil {
		return educationrefapi.GovernanceBodyList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.GovernanceBody, 0, len(rows))
	for _, v := range rows {
		out = append(out, governanceBodyAPI(v))
	}
	return educationrefapi.GovernanceBodyList{GovernanceBodies: out}, nil
}

func (s ReferenceService) GetGovernanceBody(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.GovernanceBody, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.GovernanceBody{}, err
	}
	v, err := s.app.GetGovernanceBody(ctx, id)
	if err != nil {
		return educationrefapi.GovernanceBody{}, s.err(ctx, err)
	}
	return governanceBodyAPI(v), nil
}

func (s ReferenceService) UpdateGovernanceBody(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertGovernanceBodyRequest) (educationrefapi.GovernanceBody, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.GovernanceBody{}, err
	}
	v, err := s.app.UpdateGovernanceBody(ctx, id, governanceBodyInput(req))
	if err != nil {
		return educationrefapi.GovernanceBody{}, s.err(ctx, err)
	}
	return governanceBodyAPI(v), nil
}

func (s ReferenceService) DeleteGovernanceBody(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteGovernanceBody(ctx, id))
}

// ============================ policies ============================

func (s ReferenceService) CreatePolicy(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertPolicyRequest) (educationrefapi.Policy, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Policy{}, err
	}
	v, err := s.app.CreatePolicy(ctx, institutionID, policyInput(req))
	if err != nil {
		return educationrefapi.Policy{}, s.err(ctx, err)
	}
	return policyAPI(v), nil
}

func (s ReferenceService) ListPolicies(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.PolicyList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.PolicyList{}, err
	}
	rows, err := s.app.ListPolicies(ctx, institutionID)
	if err != nil {
		return educationrefapi.PolicyList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.Policy, 0, len(rows))
	for _, v := range rows {
		out = append(out, policyAPI(v))
	}
	return educationrefapi.PolicyList{Policies: out}, nil
}

func (s ReferenceService) GetPolicy(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Policy, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Policy{}, err
	}
	v, err := s.app.GetPolicy(ctx, id)
	if err != nil {
		return educationrefapi.Policy{}, s.err(ctx, err)
	}
	return policyAPI(v), nil
}

func (s ReferenceService) UpdatePolicy(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertPolicyRequest) (educationrefapi.Policy, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Policy{}, err
	}
	v, err := s.app.UpdatePolicy(ctx, id, policyInput(req))
	if err != nil {
		return educationrefapi.Policy{}, s.err(ctx, err)
	}
	return policyAPI(v), nil
}

func (s ReferenceService) DeletePolicy(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeletePolicy(ctx, id))
}

// ============================ qualifications ============================

func (s ReferenceService) CreateQualification(ctx context.Context, t bearertoken.Token, institutionID string, req educationrefapi.UpsertQualificationRequest) (educationrefapi.Qualification, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Qualification{}, err
	}
	v, err := s.app.CreateQualification(ctx, institutionID, qualificationInput(req))
	if err != nil {
		return educationrefapi.Qualification{}, s.err(ctx, err)
	}
	return qualificationAPI(v), nil
}

func (s ReferenceService) ListQualifications(ctx context.Context, t bearertoken.Token, institutionID string) (educationrefapi.QualificationList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.QualificationList{}, err
	}
	rows, err := s.app.ListQualifications(ctx, institutionID)
	if err != nil {
		return educationrefapi.QualificationList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.Qualification, 0, len(rows))
	for _, v := range rows {
		out = append(out, qualificationAPI(v))
	}
	return educationrefapi.QualificationList{Qualifications: out}, nil
}

func (s ReferenceService) GetQualification(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Qualification, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Qualification{}, err
	}
	v, err := s.app.GetQualification(ctx, id)
	if err != nil {
		return educationrefapi.Qualification{}, s.err(ctx, err)
	}
	return qualificationAPI(v), nil
}

func (s ReferenceService) UpdateQualification(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertQualificationRequest) (educationrefapi.Qualification, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Qualification{}, err
	}
	v, err := s.app.UpdateQualification(ctx, id, qualificationInput(req))
	if err != nil {
		return educationrefapi.Qualification{}, s.err(ctx, err)
	}
	return qualificationAPI(v), nil
}

func (s ReferenceService) DeleteQualification(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteQualification(ctx, id))
}

// ============================ scholarships ============================

func (s ReferenceService) CreateScholarship(ctx context.Context, t bearertoken.Token, req educationrefapi.UpsertScholarshipRequest) (educationrefapi.Scholarship, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Scholarship{}, err
	}
	v, err := s.app.CreateScholarship(ctx, scholarshipInput(req))
	if err != nil {
		return educationrefapi.Scholarship{}, s.err(ctx, err)
	}
	return scholarshipAPI(v), nil
}

func (s ReferenceService) ListScholarships(ctx context.Context, t bearertoken.Token, query *string, pageSize *int, pageToken *string) (educationrefapi.ScholarshipPage, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ScholarshipPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListScholarships(ctx, strOr(query), decodeToken(pageToken), limit)
	if err != nil {
		return educationrefapi.ScholarshipPage{}, s.err(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out := make([]educationrefapi.Scholarship, 0, len(rows))
	for _, v := range rows {
		out = append(out, scholarshipAPI(v))
	}
	page := educationrefapi.ScholarshipPage{Scholarships: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s ReferenceService) GetScholarship(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.Scholarship, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.Scholarship{}, err
	}
	v, err := s.app.GetScholarship(ctx, id)
	if err != nil {
		return educationrefapi.Scholarship{}, s.err(ctx, err)
	}
	return scholarshipAPI(v), nil
}

func (s ReferenceService) UpdateScholarship(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertScholarshipRequest) (educationrefapi.Scholarship, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.Scholarship{}, err
	}
	v, err := s.app.UpdateScholarship(ctx, id, scholarshipInput(req))
	if err != nil {
		return educationrefapi.Scholarship{}, s.err(ctx, err)
	}
	return scholarshipAPI(v), nil
}

func (s ReferenceService) DeleteScholarship(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteScholarship(ctx, id))
}

// ============================ accreditation events ============================

func (s ReferenceService) CreateAccreditationEvent(ctx context.Context, t bearertoken.Token, req educationrefapi.UpsertAccreditationEventRequest) (educationrefapi.AccreditationEvent, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.AccreditationEvent{}, err
	}
	v, err := s.app.CreateAccreditationEvent(ctx, accreditationInput(req))
	if err != nil {
		return educationrefapi.AccreditationEvent{}, s.err(ctx, err)
	}
	return accreditationAPI(v), nil
}

func (s ReferenceService) ListAccreditationEvents(ctx context.Context, t bearertoken.Token, institutionID *string, programID *string) (educationrefapi.AccreditationEventList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.AccreditationEventList{}, err
	}
	rows, err := s.app.ListAccreditationEvents(ctx, strOr(institutionID), strOr(programID))
	if err != nil {
		return educationrefapi.AccreditationEventList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.AccreditationEvent, 0, len(rows))
	for _, v := range rows {
		out = append(out, accreditationAPI(v))
	}
	return educationrefapi.AccreditationEventList{Events: out}, nil
}

func (s ReferenceService) GetAccreditationEvent(ctx context.Context, t bearertoken.Token, id string) (educationrefapi.AccreditationEvent, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.AccreditationEvent{}, err
	}
	v, err := s.app.GetAccreditationEvent(ctx, id)
	if err != nil {
		return educationrefapi.AccreditationEvent{}, s.err(ctx, err)
	}
	return accreditationAPI(v), nil
}

func (s ReferenceService) UpdateAccreditationEvent(ctx context.Context, t bearertoken.Token, id string, req educationrefapi.UpsertAccreditationEventRequest) (educationrefapi.AccreditationEvent, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return educationrefapi.AccreditationEvent{}, err
	}
	v, err := s.app.UpdateAccreditationEvent(ctx, id, accreditationInput(req))
	if err != nil {
		return educationrefapi.AccreditationEvent{}, s.err(ctx, err)
	}
	return accreditationAPI(v), nil
}

func (s ReferenceService) DeleteAccreditationEvent(ctx context.Context, t bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refManagePerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteAccreditationEvent(ctx, id))
}

// ============================ person: publication authorships ============================

func (s ReferenceService) ListPublicationAuthorships(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.PublicationAuthorshipList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.PublicationAuthorshipList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.PublicationAuthorshipList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.PublicationAuthorshipList{Authorships: []educationrefapi.PublicationAuthorship{}}, nil
	}
	rows, err := s.app.ListPublicationAuthorships(ctx, personID)
	if err != nil {
		return educationrefapi.PublicationAuthorshipList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.PublicationAuthorship, 0, len(rows))
	for _, v := range rows {
		out = append(out, authorshipAPI(v))
	}
	return educationrefapi.PublicationAuthorshipList{Authorships: out}, nil
}

func (s ReferenceService) CreatePublicationAuthorship(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertPublicationAuthorshipRequest) (educationrefapi.PublicationAuthorship, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.PublicationAuthorship{}, err
	}
	v, err := s.app.CreatePublicationAuthorship(ctx, personID, domain.PublicationAuthorshipInput{PublicationID: req.PublicationId, AuthorOrder: req.AuthorOrder, Corresponding: req.Corresponding, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.PublicationAuthorship{}, s.err(ctx, err)
	}
	return authorshipAPI(v), nil
}

func (s ReferenceService) UpdatePublicationAuthorship(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertPublicationAuthorshipRequest) (educationrefapi.PublicationAuthorship, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.PublicationAuthorship{}, err
	}
	v, err := s.app.UpdatePublicationAuthorship(ctx, personID, linkID, domain.PublicationAuthorshipInput{PublicationID: req.PublicationId, AuthorOrder: req.AuthorOrder, Corresponding: req.Corresponding, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.PublicationAuthorship{}, s.err(ctx, err)
	}
	return authorshipAPI(v), nil
}

func (s ReferenceService) DeletePublicationAuthorship(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeletePublicationAuthorship(ctx, personID, linkID))
}

// ============================ person: research memberships ============================

func (s ReferenceService) ListResearchMemberships(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.ResearchMembershipList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ResearchMembershipList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.ResearchMembershipList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.ResearchMembershipList{Memberships: []educationrefapi.ResearchMembership{}}, nil
	}
	rows, err := s.app.ListResearchMemberships(ctx, personID)
	if err != nil {
		return educationrefapi.ResearchMembershipList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.ResearchMembership, 0, len(rows))
	for _, v := range rows {
		out = append(out, researchMembershipAPI(v))
	}
	return educationrefapi.ResearchMembershipList{Memberships: out}, nil
}

func (s ReferenceService) CreateResearchMembership(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertResearchMembershipRequest) (educationrefapi.ResearchMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.ResearchMembership{}, err
	}
	v, err := s.app.CreateResearchMembership(ctx, personID, domain.ResearchMembershipInput{GroupID: req.GroupId, Role: req.Role, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.ResearchMembership{}, s.err(ctx, err)
	}
	return researchMembershipAPI(v), nil
}

func (s ReferenceService) UpdateResearchMembership(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertResearchMembershipRequest) (educationrefapi.ResearchMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.ResearchMembership{}, err
	}
	v, err := s.app.UpdateResearchMembership(ctx, personID, linkID, domain.ResearchMembershipInput{GroupID: req.GroupId, Role: req.Role, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.ResearchMembership{}, s.err(ctx, err)
	}
	return researchMembershipAPI(v), nil
}

func (s ReferenceService) DeleteResearchMembership(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteResearchMembership(ctx, personID, linkID))
}

// ============================ person: grant holdings ============================

func (s ReferenceService) ListGrantHoldings(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.GrantHoldingList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.GrantHoldingList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.GrantHoldingList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.GrantHoldingList{Holdings: []educationrefapi.GrantHolding{}}, nil
	}
	rows, err := s.app.ListGrantHoldings(ctx, personID)
	if err != nil {
		return educationrefapi.GrantHoldingList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.GrantHolding, 0, len(rows))
	for _, v := range rows {
		out = append(out, grantHoldingAPI(v))
	}
	return educationrefapi.GrantHoldingList{Holdings: out}, nil
}

func (s ReferenceService) CreateGrantHolding(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertGrantHoldingRequest) (educationrefapi.GrantHolding, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.GrantHolding{}, err
	}
	v, err := s.app.CreateGrantHolding(ctx, personID, domain.GrantHoldingInput{GrantID: req.GrantId, Role: req.Role, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.GrantHolding{}, s.err(ctx, err)
	}
	return grantHoldingAPI(v), nil
}

func (s ReferenceService) UpdateGrantHolding(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertGrantHoldingRequest) (educationrefapi.GrantHolding, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.GrantHolding{}, err
	}
	v, err := s.app.UpdateGrantHolding(ctx, personID, linkID, domain.GrantHoldingInput{GrantID: req.GrantId, Role: req.Role, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.GrantHolding{}, s.err(ctx, err)
	}
	return grantHoldingAPI(v), nil
}

func (s ReferenceService) DeleteGrantHolding(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteGrantHolding(ctx, personID, linkID))
}

// ============================ person: governance memberships ============================

func (s ReferenceService) ListGovernanceMemberships(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.GovernanceMembershipList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.GovernanceMembershipList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.GovernanceMembershipList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.GovernanceMembershipList{Memberships: []educationrefapi.GovernanceMembership{}}, nil
	}
	rows, err := s.app.ListGovernanceMemberships(ctx, personID)
	if err != nil {
		return educationrefapi.GovernanceMembershipList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.GovernanceMembership, 0, len(rows))
	for _, v := range rows {
		out = append(out, governanceMembershipAPI(v))
	}
	return educationrefapi.GovernanceMembershipList{Memberships: out}, nil
}

func (s ReferenceService) CreateGovernanceMembership(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertGovernanceMembershipRequest) (educationrefapi.GovernanceMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.GovernanceMembership{}, err
	}
	v, err := s.app.CreateGovernanceMembership(ctx, personID, domain.GovernanceMembershipInput{BodyID: req.BodyId, RoleInBody: req.RoleInBody, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.GovernanceMembership{}, s.err(ctx, err)
	}
	return governanceMembershipAPI(v), nil
}

func (s ReferenceService) UpdateGovernanceMembership(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertGovernanceMembershipRequest) (educationrefapi.GovernanceMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.GovernanceMembership{}, err
	}
	v, err := s.app.UpdateGovernanceMembership(ctx, personID, linkID, domain.GovernanceMembershipInput{BodyID: req.BodyId, RoleInBody: req.RoleInBody, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.GovernanceMembership{}, s.err(ctx, err)
	}
	return governanceMembershipAPI(v), nil
}

func (s ReferenceService) DeleteGovernanceMembership(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteGovernanceMembership(ctx, personID, linkID))
}

// ============================ person: qualification awards ============================

func (s ReferenceService) ListQualificationAwards(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.QualificationAwardList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.QualificationAwardList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.QualificationAwardList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.QualificationAwardList{Awards: []educationrefapi.QualificationAward{}}, nil
	}
	rows, err := s.app.ListQualificationAwards(ctx, personID)
	if err != nil {
		return educationrefapi.QualificationAwardList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.QualificationAward, 0, len(rows))
	for _, v := range rows {
		out = append(out, qualificationAwardAPI(v))
	}
	return educationrefapi.QualificationAwardList{Awards: out}, nil
}

func (s ReferenceService) CreateQualificationAward(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertQualificationAwardRequest) (educationrefapi.QualificationAward, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.QualificationAward{}, err
	}
	v, err := s.app.CreateQualificationAward(ctx, personID, domain.QualificationAwardInput{QualificationID: req.QualificationId, EnrollmentID: req.EnrollmentId, AwardedOn: req.AwardedOn, WithDistinction: req.WithDistinction, Gpa: req.Gpa, Status: req.Status})
	if err != nil {
		return educationrefapi.QualificationAward{}, s.err(ctx, err)
	}
	return qualificationAwardAPI(v), nil
}

func (s ReferenceService) UpdateQualificationAward(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertQualificationAwardRequest) (educationrefapi.QualificationAward, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.QualificationAward{}, err
	}
	v, err := s.app.UpdateQualificationAward(ctx, personID, linkID, domain.QualificationAwardInput{QualificationID: req.QualificationId, EnrollmentID: req.EnrollmentId, AwardedOn: req.AwardedOn, WithDistinction: req.WithDistinction, Gpa: req.Gpa, Status: req.Status})
	if err != nil {
		return educationrefapi.QualificationAward{}, s.err(ctx, err)
	}
	return qualificationAwardAPI(v), nil
}

func (s ReferenceService) DeleteQualificationAward(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteQualificationAward(ctx, personID, linkID))
}

// ============================ person: scholarship awards ============================

func (s ReferenceService) ListScholarshipAwards(ctx context.Context, t bearertoken.Token, personID string) (educationrefapi.ScholarshipAwardList, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refReadPerm); err != nil {
		return educationrefapi.ScholarshipAwardList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationrefapi.ScholarshipAwardList{}, s.err(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationrefapi.ScholarshipAwardList{Awards: []educationrefapi.ScholarshipAward{}}, nil
	}
	rows, err := s.app.ListScholarshipAwards(ctx, personID)
	if err != nil {
		return educationrefapi.ScholarshipAwardList{}, s.err(ctx, err)
	}
	out := make([]educationrefapi.ScholarshipAward, 0, len(rows))
	for _, v := range rows {
		out = append(out, scholarshipAwardAPI(v))
	}
	return educationrefapi.ScholarshipAwardList{Awards: out}, nil
}

func (s ReferenceService) CreateScholarshipAward(ctx context.Context, t bearertoken.Token, personID string, req educationrefapi.UpsertScholarshipAwardRequest) (educationrefapi.ScholarshipAward, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.ScholarshipAward{}, err
	}
	v, err := s.app.CreateScholarshipAward(ctx, personID, domain.ScholarshipAwardInput{ScholarshipID: req.ScholarshipId, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.ScholarshipAward{}, s.err(ctx, err)
	}
	return scholarshipAwardAPI(v), nil
}

func (s ReferenceService) UpdateScholarshipAward(ctx context.Context, t bearertoken.Token, personID, linkID string, req educationrefapi.UpsertScholarshipAwardRequest) (educationrefapi.ScholarshipAward, error) {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return educationrefapi.ScholarshipAward{}, err
	}
	v, err := s.app.UpdateScholarshipAward(ctx, personID, linkID, domain.ScholarshipAwardInput{ScholarshipID: req.ScholarshipId, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo})
	if err != nil {
		return educationrefapi.ScholarshipAward{}, s.err(ctx, err)
	}
	return scholarshipAwardAPI(v), nil
}

func (s ReferenceService) DeleteScholarshipAward(ctx context.Context, t bearertoken.Token, personID, linkID string) error {
	if err := s.pep.RequireAnywhere(ctx, t, refLinkPerm); err != nil {
		return err
	}
	return s.err(ctx, s.app.DeleteScholarshipAward(ctx, personID, linkID))
}

// ============================ request → input mappers ============================

func courseInput(req educationrefapi.UpsertCourseRequest) domain.CourseInput {
	return domain.CourseInput{Code: req.Code, Title: req.Title, OwningUnitID: req.OwningUnitId, CreditHours: req.CreditHours, Level: req.Level, Description: req.Description, DeliveryMode: req.DeliveryMode, Status: req.Status}
}
func curriculumItemInput(req educationrefapi.UpsertCurriculumItemRequest) domain.CurriculumItemInput {
	return domain.CurriculumItemInput{CourseID: req.CourseId, IsRequired: req.IsRequired, YearOfStudy: req.YearOfStudy, CreditAllocation: req.CreditAllocation, SemesterSlot: req.SemesterSlot}
}
func researchCentreInput(req educationrefapi.UpsertResearchCentreRequest) domain.ResearchCentreInput {
	return domain.ResearchCentreInput{Code: req.Code, Name: req.Name, Kind: req.Kind, FundingSource: req.FundingSource, FoundedOn: req.FoundedOn, DissolvedOn: req.DissolvedOn, Status: req.Status}
}
func researchGroupInput(req educationrefapi.UpsertResearchGroupRequest) domain.ResearchGroupInput {
	return domain.ResearchGroupInput{Code: req.Code, Name: req.Name, CentreID: req.CentreId, UnitID: req.UnitId, FocusArea: req.FocusArea, Status: req.Status}
}
func grantInput(req educationrefapi.UpsertGrantRequest) domain.GrantInput {
	return domain.GrantInput{Code: req.Code, Title: req.Title, Funder: req.Funder, FunderRef: req.FunderRef, Amount: req.Amount, Currency: req.Currency, StartOn: req.StartOn, EndOn: req.EndOn, Status: req.Status}
}
func publicationInput(req educationrefapi.UpsertPublicationRequest) domain.PublicationInput {
	return domain.PublicationInput{Code: req.Code, Title: req.Title, InstitutionID: req.InstitutionId, Kind: req.Kind, Doi: req.Doi, Venue: req.Venue, PublishedOn: req.PublishedOn, OpenAccess: req.OpenAccess}
}
func governanceBodyInput(req educationrefapi.UpsertGovernanceBodyRequest) domain.GovernanceBodyInput {
	return domain.GovernanceBodyInput{Code: req.Code, Name: req.Name, Kind: req.Kind, Mandate: req.Mandate, Status: req.Status}
}
func policyInput(req educationrefapi.UpsertPolicyRequest) domain.PolicyInput {
	return domain.PolicyInput{Code: req.Code, Title: req.Title, GovernanceBodyID: req.GovernanceBodyId, SupersedesID: req.SupersedesId, Kind: req.Kind, EffectiveOn: req.EffectiveOn, ExpiryOn: req.ExpiryOn, DocumentURL: req.DocumentUrl, Status: req.Status}
}
func qualificationInput(req educationrefapi.UpsertQualificationRequest) domain.QualificationInput {
	return domain.QualificationInput{Code: req.Code, Name: req.Name, ProgramID: req.ProgramId, DegreeLevelID: req.DegreeLevelId, FrameworkCode: req.FrameworkCode, FrameworkLevel: req.FrameworkLevel, AwardingBody: req.AwardingBody, Status: req.Status}
}
func scholarshipInput(req educationrefapi.UpsertScholarshipRequest) domain.ScholarshipInput {
	return domain.ScholarshipInput{Code: req.Code, Name: req.Name, InstitutionID: req.InstitutionId, Kind: req.Kind, Amount: req.Amount, Currency: req.Currency, Frequency: req.Frequency, Renewable: req.Renewable, Conditions: req.Conditions, Status: req.Status}
}
func accreditationInput(req educationrefapi.UpsertAccreditationEventRequest) domain.AccreditationEventInput {
	return domain.AccreditationEventInput{EntityKind: req.EntityKind, InstitutionID: req.InstitutionId, ProgramID: req.ProgramId, Body: req.Body, BodyCountryID: req.BodyCountryId, Outcome: req.Outcome, ReviewOn: req.ReviewOn, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo, Notes: req.Notes}
}

// ============================ domain → API mappers ============================

func programAPI(v domain.Program) educationrefapi.Program {
	return educationrefapi.Program{
		Id: v.ID, InstitutionId: v.InstitutionID, OwningUnitId: emptyToNil(v.OwningUnitID), DegreeLevelId: emptyToNil(v.DegreeLevelID),
		Code: v.Code, Name: v.Name, Mode: v.Mode, DurationYears: emptyToNil(v.DurationYears), CreditHoursTotal: v.CreditHoursTotal,
		State: v.State, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func courseAPI(v domain.Course) educationrefapi.Course {
	return educationrefapi.Course{
		Id: v.ID, InstitutionId: v.InstitutionID, OwningUnitId: emptyToNil(v.OwningUnitID), Code: v.Code, Title: v.Title,
		CreditHours: v.CreditHours, Level: v.Level, Description: emptyToNil(v.Description), DeliveryMode: v.DeliveryMode, Status: v.Status,
		CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func curriculumVersionAPI(v domain.CurriculumVersion) educationrefapi.CurriculumVersion {
	return educationrefapi.CurriculumVersion{
		Id: v.ID, ProgramId: v.ProgramID, VersionCode: v.VersionCode, EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo),
		Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func curriculumItemAPI(v domain.CurriculumItem) educationrefapi.CurriculumItem {
	return educationrefapi.CurriculumItem{
		Id: v.ID, VersionId: v.VersionID, CourseId: v.CourseID, IsRequired: v.IsRequired, YearOfStudy: v.YearOfStudy,
		CreditAllocation: v.CreditAllocation, SemesterSlot: v.SemesterSlot, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func coursePrereqAPI(v domain.CoursePrerequisite) educationrefapi.CoursePrerequisite {
	return educationrefapi.CoursePrerequisite{
		Id: v.ID, CourseId: v.CourseID, RequiredCourseId: v.RequiredCourseID, Kind: v.Kind, MinGrade: emptyToNil(v.MinGrade),
		CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func researchCentreAPI(v domain.ResearchCentre) educationrefapi.ResearchCentre {
	return educationrefapi.ResearchCentre{
		Id: v.ID, InstitutionId: v.InstitutionID, Code: v.Code, Name: v.Name, Kind: v.Kind, FundingSource: emptyToNil(v.FundingSource),
		FoundedOn: emptyToNil(v.FoundedOn), DissolvedOn: emptyToNil(v.DissolvedOn), Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func researchGroupAPI(v domain.ResearchGroup) educationrefapi.ResearchGroup {
	return educationrefapi.ResearchGroup{
		Id: v.ID, InstitutionId: v.InstitutionID, CentreId: emptyToNil(v.CentreID), UnitId: emptyToNil(v.UnitID), Code: v.Code, Name: v.Name,
		FocusArea: emptyToNil(v.FocusArea), Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func grantAPI(v domain.Grant) educationrefapi.Grant {
	return educationrefapi.Grant{
		Id: v.ID, InstitutionId: v.InstitutionID, Code: v.Code, Title: v.Title, Funder: emptyToNil(v.Funder), FunderRef: emptyToNil(v.FunderRef),
		Amount: emptyToNil(v.Amount), Currency: emptyToNil(v.Currency), StartOn: emptyToNil(v.StartOn), EndOn: emptyToNil(v.EndOn), Status: v.Status,
		CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func publicationAPI(v domain.Publication) educationrefapi.Publication {
	return educationrefapi.Publication{
		Id: v.ID, InstitutionId: emptyToNil(v.InstitutionID), Code: v.Code, Title: v.Title, Kind: v.Kind, Doi: emptyToNil(v.Doi), Venue: emptyToNil(v.Venue),
		PublishedOn: emptyToNil(v.PublishedOn), OpenAccess: v.OpenAccess, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func governanceBodyAPI(v domain.GovernanceBody) educationrefapi.GovernanceBody {
	return educationrefapi.GovernanceBody{
		Id: v.ID, InstitutionId: v.InstitutionID, Code: v.Code, Name: v.Name, Kind: v.Kind, Mandate: emptyToNil(v.Mandate),
		Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func policyAPI(v domain.Policy) educationrefapi.Policy {
	return educationrefapi.Policy{
		Id: v.ID, InstitutionId: v.InstitutionID, GovernanceBodyId: emptyToNil(v.GovernanceBodyID), SupersedesId: emptyToNil(v.SupersedesID),
		Code: v.Code, Title: v.Title, Kind: v.Kind, EffectiveOn: emptyToNil(v.EffectiveOn), ExpiryOn: emptyToNil(v.ExpiryOn),
		DocumentUrl: emptyToNil(v.DocumentURL), Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func qualificationAPI(v domain.Qualification) educationrefapi.Qualification {
	return educationrefapi.Qualification{
		Id: v.ID, InstitutionId: v.InstitutionID, ProgramId: emptyToNil(v.ProgramID), DegreeLevelId: emptyToNil(v.DegreeLevelID), Code: v.Code, Name: v.Name,
		FrameworkCode: emptyToNil(v.FrameworkCode), FrameworkLevel: emptyToNil(v.FrameworkLevel), AwardingBody: emptyToNil(v.AwardingBody), Status: v.Status,
		CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func scholarshipAPI(v domain.Scholarship) educationrefapi.Scholarship {
	return educationrefapi.Scholarship{
		Id: v.ID, InstitutionId: emptyToNil(v.InstitutionID), Code: v.Code, Name: v.Name, Kind: v.Kind, Amount: emptyToNil(v.Amount), Currency: emptyToNil(v.Currency),
		Frequency: v.Frequency, Renewable: v.Renewable, Conditions: emptyToNil(v.Conditions), Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func accreditationAPI(v domain.AccreditationEvent) educationrefapi.AccreditationEvent {
	return educationrefapi.AccreditationEvent{
		Id: v.ID, EntityKind: v.EntityKind, InstitutionId: emptyToNil(v.InstitutionID), ProgramId: emptyToNil(v.ProgramID), Body: emptyToNil(v.Body),
		BodyCountryId: emptyToNil(v.BodyCountryID), Outcome: v.Outcome, ReviewOn: emptyToNil(v.ReviewOn), EffectiveFrom: emptyToNil(v.EffectiveFrom),
		EffectiveTo: emptyToNil(v.EffectiveTo), Notes: emptyToNil(v.Notes), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func authorshipAPI(v domain.PublicationAuthorship) educationrefapi.PublicationAuthorship {
	return educationrefapi.PublicationAuthorship{
		Id: v.ID, PersonId: v.PersonID, PublicationId: v.PublicationID, AuthorOrder: v.AuthorOrder, Corresponding: v.Corresponding,
		EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func researchMembershipAPI(v domain.ResearchMembership) educationrefapi.ResearchMembership {
	return educationrefapi.ResearchMembership{
		Id: v.ID, PersonId: v.PersonID, GroupId: v.GroupID, Role: emptyToNil(v.Role), Status: v.Status,
		EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func grantHoldingAPI(v domain.GrantHolding) educationrefapi.GrantHolding {
	return educationrefapi.GrantHolding{
		Id: v.ID, PersonId: v.PersonID, GrantId: v.GrantID, Role: v.Role, Status: v.Status,
		EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func governanceMembershipAPI(v domain.GovernanceMembership) educationrefapi.GovernanceMembership {
	return educationrefapi.GovernanceMembership{
		Id: v.ID, PersonId: v.PersonID, BodyId: v.BodyID, RoleInBody: emptyToNil(v.RoleInBody), Status: v.Status,
		EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func qualificationAwardAPI(v domain.QualificationAward) educationrefapi.QualificationAward {
	return educationrefapi.QualificationAward{
		Id: v.ID, PersonId: v.PersonID, QualificationId: v.QualificationID, EnrollmentId: emptyToNil(v.EnrollmentID), AwardedOn: emptyToNil(v.AwardedOn),
		WithDistinction: v.WithDistinction, Gpa: emptyToNil(v.Gpa), Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}
func scholarshipAwardAPI(v domain.ScholarshipAward) educationrefapi.ScholarshipAward {
	return educationrefapi.ScholarshipAward{
		Id: v.ID, PersonId: v.PersonID, ScholarshipId: v.ScholarshipID, Status: v.Status,
		EffectiveFrom: emptyToNil(v.EffectiveFrom), EffectiveTo: emptyToNil(v.EffectiveTo), CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}

// err maps reference-layer domain sentinels to the educationref Conjure errors.
func (s ReferenceService) err(ctx context.Context, e error) error {
	if e == nil {
		return nil
	}
	switch {
	case errors.Is(e, domain.ErrRefNotFound):
		return educationrefapi.NewReferenceNotFound("education", "")
	case errors.Is(e, domain.ErrPrereqCycle):
		return educationrefapi.NewPrerequisiteCycle("")
	case errors.Is(e, domain.ErrConflict):
		return educationrefapi.NewRefConflict("code already exists in scope")
	case errors.Is(e, domain.ErrInUse):
		return educationrefapi.NewRefInUse("entity still referenced")
	case errors.Is(e, domain.ErrInvalid):
		return educationrefapi.NewRefInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, e, "education reference operation failed")
}
