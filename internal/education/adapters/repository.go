// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters is the education module's pgx/sqlc-backed persistence adapter (implements
// domain.Repository). It maps domain values to the generated educationsql params/rows and translates
// Postgres constraint violations (23505 unique / 23503 FK) into domain sentinels.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/education/adapters/educationsql"
	"github.com/olegamysk/go-oikumenea/internal/education/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository implements domain.Repository over a single command surface (pool or tx).
type Repository struct {
	q *educationsql.Queries
}

// NewRepository binds a repository to the given command surface (a db.DBTX value).
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: educationsql.New(conn)}
}

var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- catalogs

func (r *Repository) ListInstitutionKinds(ctx context.Context) ([]domain.InstitutionKind, error) {
	rows, err := r.q.ListInstitutionKinds(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.InstitutionKind, 0, len(rows))
	for _, k := range rows {
		out = append(out, domain.InstitutionKind{ID: k.ID, Code: k.Code, Name: k.Name, Status: k.Status, SortOrder: int4ptr(k.SortOrder)})
	}
	return out, nil
}

func (r *Repository) UpsertInstitutionKind(ctx context.Context, code, name string, sortOrder *int) (domain.InstitutionKind, error) {
	k, err := r.q.UpsertInstitutionKind(ctx, educationsql.UpsertInstitutionKindParams{Code: code, Name: name, SortOrder: int4(sortOrder)})
	if err != nil {
		return domain.InstitutionKind{}, mapErr(err)
	}
	return domain.InstitutionKind{ID: k.ID, Code: k.Code, Name: k.Name, Status: k.Status, SortOrder: int4ptr(k.SortOrder)}, nil
}

func (r *Repository) ListDegreeLevels(ctx context.Context) ([]domain.DegreeLevel, error) {
	rows, err := r.q.ListDegreeLevels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DegreeLevel, 0, len(rows))
	for _, d := range rows {
		out = append(out, domain.DegreeLevel{ID: d.ID, Code: d.Code, Name: d.Name, IscedLevel: int(d.IscedLevel), Status: d.Status, SortOrder: int4ptr(d.SortOrder)})
	}
	return out, nil
}

// ---------------------------------------------------------------- institutions (tenant org + sidecar — M41)

// InsertOrgProfile writes the education_org_profiles sidecar for an already-created `university`-domain
// tenant organization (its code/name live on the org; the org is created via the tenant service).
func (r *Repository) InsertOrgProfile(ctx context.Context, institutionID, kindID string, countryID, foundedOn, closedOn *string) error {
	_, err := r.q.InsertOrgProfile(ctx, educationsql.InsertOrgProfileParams{
		InstitutionID: institutionID, KindID: kindID,
		CountryID: text(countryID), FoundedOn: datePtr(foundedOn), ClosedOn: datePtr(closedOn),
	})
	return mapErr(err)
}

func (r *Repository) GetInstitution(ctx context.Context, id string) (domain.Institution, error) {
	row, err := r.q.GetInstitution(ctx, id)
	if err != nil {
		return domain.Institution{}, notFound(err, domain.ErrInstitutionNotFound)
	}
	return domain.Institution{
		ID: row.ID, Code: row.Code, Name: row.Name, KindID: row.KindID, CountryID: strp(row.CountryID),
		FoundedOn: dateStr(row.FoundedOn), ClosedOn: dateStr(row.ClosedOn), State: row.State,
		CreatedAt: ts(row.CreatedAt), UpdatedAt: ts(row.UpdatedAt),
	}, nil
}

// UpdateOrgProfile applies the education-specific fields (kind/country/dates/state) on the sidecar; the
// org name is changed separately through the tenant service.
func (r *Repository) UpdateOrgProfile(ctx context.Context, id string, up domain.InstitutionUpdate) error {
	_, err := r.q.UpdateOrgProfile(ctx, educationsql.UpdateOrgProfileParams{
		KindID: text(up.KindID), CountryID: text(up.CountryID),
		FoundedOn: datePtr(up.FoundedOn), ClosedOn: datePtr(up.ClosedOn), State: text(up.State), ID: id,
	})
	return notFound(mapErr(err), domain.ErrInstitutionNotFound)
}

// ListInstitutions returns a keyset page of institutions. A non-empty query routes to the dedicated
// trigram SearchInstitutions (review R-21) so the code/name match stays a GIN bitmap scan; the empty
// case is the plain keyset list. The two queries share the projection, so their rows are convertible.
func (r *Repository) ListInstitutions(ctx context.Context, query, after string, lim int) ([]domain.Institution, error) {
	var rows []educationsql.ListInstitutionsRow
	if q := strings.TrimSpace(query); q != "" {
		found, err := r.q.SearchInstitutions(ctx, educationsql.SearchInstitutionsParams{Query: pgtype.Text{String: q, Valid: true}, After: after, Lim: int32(lim)})
		if err != nil {
			return nil, err
		}
		rows = make([]educationsql.ListInstitutionsRow, len(found))
		for i, row := range found {
			rows[i] = educationsql.ListInstitutionsRow(row)
		}
	} else {
		var err error
		if rows, err = r.q.ListInstitutions(ctx, educationsql.ListInstitutionsParams{After: after, Lim: int32(lim)}); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Institution, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Institution{
			ID: row.ID, Code: row.Code, Name: row.Name, KindID: row.KindID, CountryID: strp(row.CountryID),
			FoundedOn: dateStr(row.FoundedOn), ClosedOn: dateStr(row.ClosedOn), State: row.State,
			CreatedAt: ts(row.CreatedAt), UpdatedAt: ts(row.UpdatedAt),
		})
	}
	return out, nil
}

func (r *Repository) SoftDeleteInstitution(ctx context.Context, id string) (int64, error) {
	n, err := r.q.SoftDeleteInstitution(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

// ---------------------------------------------------------------- buildings

func (r *Repository) InsertBuilding(ctx context.Context, institutionID string, in domain.BuildingInput) (domain.Building, error) {
	row, err := r.q.InsertBuilding(ctx, educationsql.InsertBuildingParams{
		InstitutionID: institutionID, UnitID: text(in.UnitID), LocationID: text(in.LocationID),
		Code: in.Code, Name: in.Name, Kind: in.Kind,
	})
	if err != nil {
		return domain.Building{}, mapErr(err)
	}
	return toBuilding(row), nil
}

func (r *Repository) GetBuilding(ctx context.Context, id string) (domain.Building, error) {
	row, err := r.q.GetBuilding(ctx, id)
	if err != nil {
		return domain.Building{}, notFound(err, domain.ErrBuildingNotFound)
	}
	return toBuilding(row), nil
}

func (r *Repository) UpdateBuilding(ctx context.Context, id string, up domain.BuildingUpdate) (domain.Building, error) {
	row, err := r.q.UpdateBuilding(ctx, educationsql.UpdateBuildingParams{
		Name: text(up.Name), Kind: text(up.Kind), UnitID: text(up.UnitID), LocationID: text(up.LocationID), ID: id,
	})
	if err != nil {
		return domain.Building{}, notFound(mapErr(err), domain.ErrBuildingNotFound)
	}
	return toBuilding(row), nil
}

func (r *Repository) ListBuildingsByInstitution(ctx context.Context, institutionID string) ([]domain.Building, error) {
	rows, err := r.q.ListBuildingsByInstitution(ctx, institutionID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Building, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBuilding(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteBuilding(ctx context.Context, id string) (int64, error) {
	n, err := r.q.SoftDeleteBuilding(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

// ---------------------------------------------------------------- groups

func (r *Repository) InsertGroup(ctx context.Context, unitID string, in domain.GroupInput) (domain.Group, error) {
	row, err := r.q.InsertGroup(ctx, educationsql.InsertGroupParams{
		UnitID: unitID, Code: in.Code, Name: in.Name, AdmissionYear: int4(in.AdmissionYear),
	})
	if err != nil {
		return domain.Group{}, mapErr(err)
	}
	return toGroup(row), nil
}

func (r *Repository) GetGroup(ctx context.Context, id string) (domain.Group, error) {
	row, err := r.q.GetGroup(ctx, id)
	if err != nil {
		return domain.Group{}, notFound(err, domain.ErrGroupNotFound)
	}
	return toGroup(row), nil
}

func (r *Repository) UpdateGroup(ctx context.Context, id string, up domain.GroupUpdate) (domain.Group, error) {
	row, err := r.q.UpdateGroup(ctx, educationsql.UpdateGroupParams{
		Name: text(up.Name), AdmissionYear: int4(up.AdmissionYear), Status: text(up.Status), ID: id,
	})
	if err != nil {
		return domain.Group{}, notFound(mapErr(err), domain.ErrGroupNotFound)
	}
	return toGroup(row), nil
}

func (r *Repository) ListGroupsByUnit(ctx context.Context, unitID string) ([]domain.Group, error) {
	rows, err := r.q.ListGroupsByUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGroup(row))
	}
	return out, nil
}

func (r *Repository) SoftDeleteGroup(ctx context.Context, id string) (int64, error) {
	n, err := r.q.SoftDeleteGroup(ctx, id)
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

// ---------------------------------------------------------------- positions + appointments

func (r *Repository) InsertPosition(ctx context.Context, institutionID string, in domain.PositionInput) (domain.Position, error) {
	row, err := r.q.InsertPosition(ctx, educationsql.InsertPositionParams{
		InstitutionID: institutionID, UnitID: text(in.UnitID), Code: in.Code, Title: in.Title, SortOrder: ifaceInt(in.SortOrder),
	})
	if err != nil {
		return domain.Position{}, mapErr(err)
	}
	return toPosition(row), nil
}

func (r *Repository) GetPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.GetPosition(ctx, id)
	if err != nil {
		return domain.Position{}, notFound(err, domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) UpdatePosition(ctx context.Context, id string, up domain.PositionUpdate) (domain.Position, error) {
	row, err := r.q.UpdatePosition(ctx, educationsql.UpdatePositionParams{Title: text(up.Title), SortOrder: int4(up.SortOrder), ID: id})
	if err != nil {
		return domain.Position{}, notFound(mapErr(err), domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) AbolishPosition(ctx context.Context, id string) (domain.Position, error) {
	row, err := r.q.AbolishPosition(ctx, id)
	if err != nil {
		return domain.Position{}, notFound(err, domain.ErrPositionNotFound)
	}
	return toPosition(row), nil
}

func (r *Repository) ListPositionsByInstitution(ctx context.Context, institutionID, state, after string, lim int) ([]domain.Position, error) {
	var rows []educationsql.OikumeneaEducationPosition
	var err error
	switch state {
	case "vacant":
		rows, err = r.q.ListVacantPositionsByInstitution(ctx, educationsql.ListVacantPositionsByInstitutionParams{InstitutionID: institutionID, After: after, Lim: int32(lim)})
	case "filled":
		rows, err = r.q.ListFilledPositionsByInstitution(ctx, educationsql.ListFilledPositionsByInstitutionParams{InstitutionID: institutionID, After: after, Lim: int32(lim)})
	default:
		rows, err = r.q.ListPositionsByInstitution(ctx, educationsql.ListPositionsByInstitutionParams{InstitutionID: institutionID, After: after, Lim: int32(lim)})
	}
	if err != nil {
		return nil, err
	}
	out := make([]domain.Position, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPosition(row))
	}
	return out, nil
}

func (r *Repository) GetActiveAppointmentByPosition(ctx context.Context, positionID string) (domain.Appointment, error) {
	row, err := r.q.GetActiveAppointmentByPosition(ctx, positionID)
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

func (r *Repository) InsertAppointment(ctx context.Context, personID, positionID string, effectiveFrom *string) (domain.Appointment, error) {
	row, err := r.q.InsertAppointment(ctx, educationsql.InsertAppointmentParams{PersonID: personID, PositionID: positionID, EffectiveFrom: tsArg(effectiveFrom)})
	if err != nil {
		return domain.Appointment{}, mapErr(err)
	}
	return toAppointment(row), nil
}

func (r *Repository) GetAppointment(ctx context.Context, id string) (domain.Appointment, error) {
	row, err := r.q.GetAppointment(ctx, id)
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

func (r *Repository) EndAppointment(ctx context.Context, id string, effectiveTo *string) (domain.Appointment, error) {
	row, err := r.q.EndAppointment(ctx, educationsql.EndAppointmentParams{EffectiveTo: tsArg(effectiveTo), ID: id})
	if err != nil {
		return domain.Appointment{}, notFound(err, domain.ErrAppointmentNotFound)
	}
	return toAppointment(row), nil
}

// ---------------------------------------------------------------- person bindings

func (r *Repository) ListAppointmentsByPerson(ctx context.Context, personID string) ([]domain.PersonAppointment, error) {
	rows, err := r.q.ListAppointmentsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PersonAppointment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPersonAppointment(row))
	}
	return out, nil
}

func (r *Repository) InsertEnrollment(ctx context.Context, personID string, in domain.EnrollmentInput) (domain.Enrollment, error) {
	row, err := r.q.InsertEnrollment(ctx, educationsql.InsertEnrollmentParams{
		PersonID: personID, InstitutionID: in.InstitutionID, UnitID: text(in.UnitID), GroupID: text(in.GroupID),
		ProgramID: text(in.ProgramID), DegreeLevelID: text(in.DegreeLevelID), FieldOfStudy: text(in.FieldOfStudy),
		StudentNumber: text(in.StudentNumber), Status: iface(in.Status),
		Qualification: text(in.Qualification), EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.Enrollment{}, mapErr(err)
	}
	return toEnrollment(row), nil
}

func (r *Repository) GetEnrollment(ctx context.Context, personID, id string) (domain.Enrollment, error) {
	row, err := r.q.GetEnrollment(ctx, educationsql.GetEnrollmentParams{ID: id, PersonID: personID})
	if err != nil {
		return domain.Enrollment{}, notFound(err, domain.ErrEnrollmentNotFound)
	}
	return toEnrollment(row), nil
}

func (r *Repository) UpdateEnrollment(ctx context.Context, personID, id string, in domain.EnrollmentInput) (domain.Enrollment, error) {
	var instID *string
	if in.InstitutionID != "" {
		instID = &in.InstitutionID
	}
	row, err := r.q.UpdateEnrollment(ctx, educationsql.UpdateEnrollmentParams{
		InstitutionID: text(instID), UnitID: text(in.UnitID), GroupID: text(in.GroupID), ProgramID: text(in.ProgramID),
		DegreeLevelID: text(in.DegreeLevelID), FieldOfStudy: text(in.FieldOfStudy), StudentNumber: text(in.StudentNumber),
		Status: text(in.Status), Qualification: text(in.Qualification),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.Enrollment{}, notFound(mapErr(err), domain.ErrEnrollmentNotFound)
	}
	return toEnrollment(row), nil
}

func (r *Repository) SoftDeleteEnrollment(ctx context.Context, personID, id string) (int64, error) {
	n, err := r.q.SoftDeleteEnrollment(ctx, educationsql.SoftDeleteEnrollmentParams{ID: id, PersonID: personID})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func (r *Repository) ListEnrollmentsByPerson(ctx context.Context, personID string) ([]domain.Enrollment, error) {
	rows, err := r.q.ListEnrollmentsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Enrollment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toEnrollment(row))
	}
	return out, nil
}

func (r *Repository) InsertDormitoryStay(ctx context.Context, personID string, in domain.DormInput) (domain.DormitoryStay, error) {
	row, err := r.q.InsertDormitoryStay(ctx, educationsql.InsertDormitoryStayParams{
		PersonID: personID, BuildingID: in.BuildingID, Room: text(in.Room), Status: iface(in.Status),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo),
	})
	if err != nil {
		return domain.DormitoryStay{}, mapErr(err)
	}
	return toDorm(row), nil
}

func (r *Repository) GetDormitoryStay(ctx context.Context, personID, id string) (domain.DormitoryStay, error) {
	row, err := r.q.GetDormitoryStay(ctx, educationsql.GetDormitoryStayParams{ID: id, PersonID: personID})
	if err != nil {
		return domain.DormitoryStay{}, notFound(err, domain.ErrDormNotFound)
	}
	return toDorm(row), nil
}

func (r *Repository) UpdateDormitoryStay(ctx context.Context, personID, id string, in domain.DormInput) (domain.DormitoryStay, error) {
	var buildingID *string
	if in.BuildingID != "" {
		buildingID = &in.BuildingID
	}
	row, err := r.q.UpdateDormitoryStay(ctx, educationsql.UpdateDormitoryStayParams{
		BuildingID: text(buildingID), Room: text(in.Room), Status: text(in.Status),
		EffectiveFrom: datePtr(in.EffectiveFrom), EffectiveTo: datePtr(in.EffectiveTo), ID: id, PersonID: personID,
	})
	if err != nil {
		return domain.DormitoryStay{}, notFound(mapErr(err), domain.ErrDormNotFound)
	}
	return toDorm(row), nil
}

func (r *Repository) SoftDeleteDormitoryStay(ctx context.Context, personID, id string) (int64, error) {
	n, err := r.q.SoftDeleteDormitoryStay(ctx, educationsql.SoftDeleteDormitoryStayParams{ID: id, PersonID: personID})
	if err != nil {
		return 0, mapErr(err)
	}
	return n, nil
}

func (r *Repository) ListDormitoryStaysByPerson(ctx context.Context, personID string) ([]domain.DormitoryStay, error) {
	rows, err := r.q.ListDormitoryStaysByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DormitoryStay, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDorm(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- mappers

func toBuilding(r educationsql.OikumeneaEducationBuilding) domain.Building {
	return domain.Building{
		ID: r.ID, InstitutionID: r.InstitutionID, UnitID: strp(r.UnitID), LocationID: strp(r.LocationID),
		Code: r.Code, Name: r.Name, Kind: r.Kind, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toGroup(r educationsql.OikumeneaEducationGroup) domain.Group {
	return domain.Group{
		ID: r.ID, UnitID: r.UnitID, Code: r.Code, Name: r.Name, AdmissionYear: int4ptr(r.AdmissionYear),
		Status: r.Status, CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toPosition(r educationsql.OikumeneaEducationPosition) domain.Position {
	return domain.Position{
		ID: r.ID, InstitutionID: r.InstitutionID, UnitID: strp(r.UnitID), Code: r.Code, Title: r.Title,
		Status: r.Status, SortOrder: int4ptr(r.SortOrder), CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toAppointment(r educationsql.OikumeneaEducationAppointment) domain.Appointment {
	return domain.Appointment{
		ID: r.ID, PersonID: r.PersonID, PositionID: r.PositionID, Status: r.Status,
		EffectiveFrom: ts(r.EffectiveFrom), EffectiveTo: tsPtr(r.EffectiveTo),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toPersonAppointment(r educationsql.ListAppointmentsByPersonRow) domain.PersonAppointment {
	return domain.PersonAppointment{
		ID: r.ID, PersonID: r.PersonID, PositionID: r.PositionID, Status: r.Status,
		PositionTitle: r.PositionTitle, InstitutionID: r.InstitutionID, InstitutionName: r.InstitutionName,
		EffectiveFrom: ts(r.EffectiveFrom), EffectiveTo: tsPtr(r.EffectiveTo),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toEnrollment(r educationsql.OikumeneaPersonEducationEnrollment) domain.Enrollment {
	return domain.Enrollment{
		ID: r.ID, PersonID: r.PersonID, InstitutionID: r.InstitutionID, UnitID: strp(r.UnitID), GroupID: strp(r.GroupID),
		ProgramID: strp(r.ProgramID), DegreeLevelID: strp(r.DegreeLevelID), FieldOfStudy: strp(r.FieldOfStudy),
		StudentNumber: strp(r.StudentNumber), Status: r.Status, Qualification: strp(r.Qualification),
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

func toDorm(r educationsql.OikumeneaPersonDormitoryStay) domain.DormitoryStay {
	return domain.DormitoryStay{
		ID: r.ID, PersonID: r.PersonID, BuildingID: r.BuildingID, Room: strp(r.Room), Status: r.Status,
		EffectiveFrom: dateStr(r.EffectiveFrom), EffectiveTo: dateStr(r.EffectiveTo),
		CreatedAt: ts(r.CreatedAt), UpdatedAt: ts(r.UpdatedAt),
	}
}

// ---------------------------------------------------------------- helpers

func text(p *string) pgtype.Text {
	if p == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *p, Valid: true}
}

func strp(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func int4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func int4ptr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	out := int(v.Int32)
	return &out
}

// iface passes an optional string to a COALESCE narg typed as interface{} (nil => SQL default).
func iface(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func ifaceInt(p *int) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func dateText(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse(domain.ISODate, s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func datePtr(p *string) pgtype.Date {
	if p == nil {
		return pgtype.Date{}
	}
	return dateText(*p)
}

func dateStr(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(domain.ISODate)
}

func ts(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	out := t.Time
	return &out
}

// tsArg maps an optional ISO date/datetime string to a nullable timestamptz narg (nil => SQL default now()).
func tsArg(p *string) pgtype.Timestamptz {
	if p == nil || *p == "" {
		return pgtype.Timestamptz{}
	}
	if t, err := time.Parse(time.RFC3339, *p); err == nil {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	if t, err := time.Parse(domain.ISODate, *p); err == nil {
		return pgtype.Timestamptz{Time: t, Valid: true}
	}
	return pgtype.Timestamptz{}
}

// notFound maps pgx.ErrNoRows to a domain not-found sentinel; other errors pass through.
func notFound(err error, sentinel error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return err
}

// mapErr translates Postgres constraint violations into domain sentinels (23505 unique => Conflict;
// 23503 FK => Invalid for a bad reference, or InUse when an owner row references a soft-deleted target).
func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23505":
		return domain.ErrConflict
	case "23503":
		// A FK violation on insert/update of an education row is a bad reference; a violation raised by
		// a delete (RESTRICT from a child) is an in-use conflict.
		if strings.Contains(strings.ToLower(pgErr.Message), "still referenced") || pgErr.ConstraintName == "" {
			return domain.ErrInUse
		}
		return domain.ErrInvalid
	}
	return err
}
