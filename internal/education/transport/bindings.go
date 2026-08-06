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

// ListPersonEnrollments returns ONE named person's enrollments. Renamed from ListEnrollments in M58
// ticket 7, when the top-level browse below took that name; the HTTP path is unchanged.
func (s EducationService) ListPersonEnrollments(ctx context.Context, token bearertoken.Token, personID string) (educationapi.EnrollmentList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EnrollmentList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationapi.EnrollmentList{}, s.mapError(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationapi.EnrollmentList{Enrollments: []educationapi.Enrollment{}}, nil
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

// ListEnrollments is the top-level facet-filtered browse (M58 ticket 7 / D-ObjectFacets).
//
// Where ListPersonEnrollments gates on ONE named holder and hides an unreadable one as an empty list,
// this endpoint has no holder to probe: the holder read scope (D-PersonReadScope) is folded into the
// query itself, so an unreadable holder's enrollments are simply not in the result set. Not merely an
// optimization — a Go-side holder check after the keyset page was cut would return a page SHORTER
// than pageSize while still handing back a nextPageToken (R-06).
func (s EducationService) ListEnrollments(ctx context.Context, token bearertoken.Token, institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFromFrom, effectiveFromTo *string, pageSize *int, pageToken *string) (educationapi.EnrollmentPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EnrollmentPage{}, err
	}
	subject, isAdmin, err := s.pep.SubjectAuthority(ctx)
	if err != nil {
		return educationapi.EnrollmentPage{}, s.mapError(ctx, err)
	}
	// A non-admin with no subject is a machine principal (M51): no person identity, no reach, and so
	// no readable holders. It reads nothing rather than everything — the rule stats.Compute writes down
	// for the aggregate half, applied here by hand because the list has no Compute to own it.
	if !isAdmin && subject == "" {
		return educationapi.EnrollmentPage{Enrollments: []educationapi.Enrollment{}}, nil
	}
	arm := subject
	if isAdmin {
		arm = "" // the instance-admin arm carries no scope predicate
	}
	limit := pageSizeOr(pageSize)
	// One filter for the whole vocabulary, built once and passed down BOTH arms — the list and its
	// dashboard must never read the same URL differently.
	filter := enrollmentFilterFrom(institutionID, programID, unitID, groupID, degreeLevelID, status, effectiveFromFrom, effectiveFromTo)
	rows, err := s.app.ListEnrollmentRegister(ctx, arm, filter, decodeToken(pageToken), limit)
	if err != nil {
		return educationapi.EnrollmentPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out := make([]educationapi.Enrollment, 0, len(rows))
	for _, e := range rows {
		out = append(out, enrollmentAPI(e))
	}
	page := educationapi.EnrollmentPage{Enrollments: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
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
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationapi.DormitoryStayList{}, s.mapError(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationapi.DormitoryStayList{DormitoryStays: []educationapi.DormitoryStay{}}, nil
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
