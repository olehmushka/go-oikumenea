// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"

	educationapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/education"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// ============================ enrollments ============================

func (s EducationService) ListEnrollments(ctx context.Context, token bearertoken.Token, personID string) (educationapi.EnrollmentList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EnrollmentList{}, err
	}
	rows, err := s.app.ListEnrollments(ctx, personID)
	if err != nil {
		return educationapi.EnrollmentList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.Enrollment, 0, len(rows))
	for _, e := range rows {
		out = append(out, enrollmentAPI(e))
	}
	return educationapi.EnrollmentList{Enrollments: out}, nil
}

func (s EducationService) CreateEnrollment(ctx context.Context, token bearertoken.Token, personID string, req educationapi.UpsertEnrollmentRequest) (educationapi.Enrollment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return educationapi.Enrollment{}, err
	}
	e, err := s.app.CreateEnrollment(ctx, personID, enrollmentInput(req))
	if err != nil {
		return educationapi.Enrollment{}, s.mapError(ctx, err)
	}
	return enrollmentAPI(e), nil
}

func (s EducationService) UpdateEnrollment(ctx context.Context, token bearertoken.Token, personID, enrollmentID string, req educationapi.UpsertEnrollmentRequest) (educationapi.Enrollment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return educationapi.Enrollment{}, err
	}
	e, err := s.app.UpdateEnrollment(ctx, personID, enrollmentID, enrollmentInput(req))
	if err != nil {
		return educationapi.Enrollment{}, s.mapError(ctx, err)
	}
	return enrollmentAPI(e), nil
}

func (s EducationService) DeleteEnrollment(ctx context.Context, token bearertoken.Token, personID, enrollmentID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteEnrollment(ctx, personID, enrollmentID))
}

func enrollmentInput(req educationapi.UpsertEnrollmentRequest) domain.EnrollmentInput {
	return domain.EnrollmentInput{
		InstitutionID: req.InstitutionId, UnitID: req.UnitId, GroupID: req.GroupId, ProgramID: req.ProgramId, DegreeLevelID: req.DegreeLevelId,
		FieldOfStudy: req.FieldOfStudy, StudentNumber: req.StudentNumber, Status: req.Status, Qualification: req.Qualification,
		EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo,
	}
}

func enrollmentAPI(e domain.Enrollment) educationapi.Enrollment {
	return educationapi.Enrollment{
		Id: e.ID, PersonId: e.PersonID, InstitutionId: e.InstitutionID, UnitId: emptyToNil(e.UnitID), GroupId: emptyToNil(e.GroupID),
		ProgramId: emptyToNil(e.ProgramID), DegreeLevelId: emptyToNil(e.DegreeLevelID), FieldOfStudy: emptyToNil(e.FieldOfStudy),
		StudentNumber: emptyToNil(e.StudentNumber), Status: e.Status,
		Qualification: emptyToNil(e.Qualification), EffectiveFrom: emptyToNil(e.EffectiveFrom), EffectiveTo: emptyToNil(e.EffectiveTo),
		CreatedAt: datetime.DateTime(e.CreatedAt), UpdatedAt: datetime.DateTime(e.UpdatedAt),
	}
}

// ============================ dormitory stays ============================

func (s EducationService) ListDormitoryStays(ctx context.Context, token bearertoken.Token, personID string) (educationapi.DormitoryStayList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.DormitoryStayList{}, err
	}
	rows, err := s.app.ListDormitoryStays(ctx, personID)
	if err != nil {
		return educationapi.DormitoryStayList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.DormitoryStay, 0, len(rows))
	for _, d := range rows {
		out = append(out, dormAPI(d))
	}
	return educationapi.DormitoryStayList{DormitoryStays: out}, nil
}

func (s EducationService) CreateDormitoryStay(ctx context.Context, token bearertoken.Token, personID string, req educationapi.UpsertDormitoryStayRequest) (educationapi.DormitoryStay, error) {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return educationapi.DormitoryStay{}, err
	}
	d, err := s.app.CreateDormitoryStay(ctx, personID, dormInput(req))
	if err != nil {
		return educationapi.DormitoryStay{}, s.mapError(ctx, err)
	}
	return dormAPI(d), nil
}

func (s EducationService) UpdateDormitoryStay(ctx context.Context, token bearertoken.Token, personID, stayID string, req educationapi.UpsertDormitoryStayRequest) (educationapi.DormitoryStay, error) {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return educationapi.DormitoryStay{}, err
	}
	d, err := s.app.UpdateDormitoryStay(ctx, personID, stayID, dormInput(req))
	if err != nil {
		return educationapi.DormitoryStay{}, s.mapError(ctx, err)
	}
	return dormAPI(d), nil
}

func (s EducationService) DeleteDormitoryStay(ctx context.Context, token bearertoken.Token, personID, stayID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, enrollmentPerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteDormitoryStay(ctx, personID, stayID))
}

func dormInput(req educationapi.UpsertDormitoryStayRequest) domain.DormInput {
	return domain.DormInput{BuildingID: req.BuildingId, Room: req.Room, Status: req.Status, EffectiveFrom: req.EffectiveFrom, EffectiveTo: req.EffectiveTo}
}

func dormAPI(d domain.DormitoryStay) educationapi.DormitoryStay {
	return educationapi.DormitoryStay{
		Id: d.ID, PersonId: d.PersonID, BuildingId: d.BuildingID, Room: emptyToNil(d.Room), Status: d.Status,
		EffectiveFrom: emptyToNil(d.EffectiveFrom), EffectiveTo: emptyToNil(d.EffectiveTo),
		CreatedAt: datetime.DateTime(d.CreatedAt), UpdatedAt: datetime.DateTime(d.UpdatedAt),
	}
}
