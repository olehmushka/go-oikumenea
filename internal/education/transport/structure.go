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

// ============================ buildings ============================

func (s EducationService) CreateBuilding(ctx context.Context, token bearertoken.Token, institutionID string, req educationapi.CreateBuildingRequest) (educationapi.Building, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Building{}, err
	}
	b, err := s.app.CreateBuilding(ctx, institutionID, domain.BuildingInput{Code: req.Code, Name: req.Name, Kind: req.Kind, UnitID: req.UnitId, LocationID: req.LocationId})
	if err != nil {
		return educationapi.Building{}, s.mapError(ctx, err)
	}
	return s.toAPIBuilding(ctx, b)
}

func (s EducationService) ListBuildings(ctx context.Context, token bearertoken.Token, institutionID string) (educationapi.BuildingList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.BuildingList{}, err
	}
	bs, err := s.app.ListBuildings(ctx, institutionID)
	if err != nil {
		return educationapi.BuildingList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(bs))
	for _, b := range bs {
		defaults[b.ID] = b.Name
	}
	names, err := s.loc.NamesByID(ctx, entBuilding, defaults)
	if err != nil {
		return educationapi.BuildingList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.Building, 0, len(bs))
	for _, b := range bs {
		out = append(out, buildingAPI(b, names[b.ID]))
	}
	return educationapi.BuildingList{Buildings: out}, nil
}

func (s EducationService) GetBuilding(ctx context.Context, token bearertoken.Token, id string) (educationapi.Building, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.Building{}, err
	}
	b, err := s.app.GetBuilding(ctx, id)
	if err != nil {
		return educationapi.Building{}, s.mapError(ctx, err)
	}
	return s.toAPIBuilding(ctx, b)
}

func (s EducationService) UpdateBuilding(ctx context.Context, token bearertoken.Token, id string, req educationapi.UpdateBuildingRequest) (educationapi.Building, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Building{}, err
	}
	b, err := s.app.UpdateBuilding(ctx, id, domain.BuildingUpdate{Name: req.Name, Kind: req.Kind, UnitID: req.UnitId, LocationID: req.LocationId})
	if err != nil {
		return educationapi.Building{}, s.mapError(ctx, err)
	}
	return s.toAPIBuilding(ctx, b)
}

func (s EducationService) DeleteBuilding(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteBuilding(ctx, id))
}

func (s EducationService) toAPIBuilding(ctx context.Context, b domain.Building) (educationapi.Building, error) {
	name, err := s.nameMap(ctx, entBuilding, b.ID, b.Name)
	if err != nil {
		return educationapi.Building{}, s.mapError(ctx, err)
	}
	return buildingAPI(b, name), nil
}

func buildingAPI(b domain.Building, name map[string]string) educationapi.Building {
	return educationapi.Building{
		Id: b.ID, InstitutionId: b.InstitutionID, UnitId: emptyToNil(b.UnitID), LocationId: emptyToNil(b.LocationID),
		Code: b.Code, Name: name, Kind: b.Kind, CreatedAt: datetime.DateTime(b.CreatedAt), UpdatedAt: datetime.DateTime(b.UpdatedAt),
	}
}

// ============================ groups ============================

func (s EducationService) CreateGroup(ctx context.Context, token bearertoken.Token, unitID string, req educationapi.CreateGroupRequest) (educationapi.Group, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Group{}, err
	}
	g, err := s.app.CreateGroup(ctx, unitID, domain.GroupInput{Code: req.Code, Name: req.Name, AdmissionYear: req.AdmissionYear})
	if err != nil {
		return educationapi.Group{}, s.mapError(ctx, err)
	}
	return s.toAPIGroup(ctx, g)
}

func (s EducationService) ListGroups(ctx context.Context, token bearertoken.Token, unitID string) (educationapi.GroupList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.GroupList{}, err
	}
	gs, err := s.app.ListGroups(ctx, unitID)
	if err != nil {
		return educationapi.GroupList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(gs))
	for _, g := range gs {
		defaults[g.ID] = g.Name
	}
	names, err := s.loc.NamesByID(ctx, entGroup, defaults)
	if err != nil {
		return educationapi.GroupList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.Group, 0, len(gs))
	for _, g := range gs {
		out = append(out, groupAPI(g, names[g.ID]))
	}
	return educationapi.GroupList{Groups: out}, nil
}

func (s EducationService) GetGroup(ctx context.Context, token bearertoken.Token, id string) (educationapi.Group, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.Group{}, err
	}
	g, err := s.app.GetGroup(ctx, id)
	if err != nil {
		return educationapi.Group{}, s.mapError(ctx, err)
	}
	return s.toAPIGroup(ctx, g)
}

func (s EducationService) UpdateGroup(ctx context.Context, token bearertoken.Token, id string, req educationapi.UpdateGroupRequest) (educationapi.Group, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return educationapi.Group{}, err
	}
	g, err := s.app.UpdateGroup(ctx, id, domain.GroupUpdate{Name: req.Name, AdmissionYear: req.AdmissionYear, Status: req.Status})
	if err != nil {
		return educationapi.Group{}, s.mapError(ctx, err)
	}
	return s.toAPIGroup(ctx, g)
}

func (s EducationService) DeleteGroup(ctx context.Context, token bearertoken.Token, id string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteGroup(ctx, id))
}

func (s EducationService) toAPIGroup(ctx context.Context, g domain.Group) (educationapi.Group, error) {
	name, err := s.nameMap(ctx, entGroup, g.ID, g.Name)
	if err != nil {
		return educationapi.Group{}, s.mapError(ctx, err)
	}
	return groupAPI(g, name), nil
}

func groupAPI(g domain.Group, name map[string]string) educationapi.Group {
	return educationapi.Group{
		Id: g.ID, UnitId: g.UnitID, Code: g.Code, Name: name, AdmissionYear: g.AdmissionYear,
		Status: g.Status, CreatedAt: datetime.DateTime(g.CreatedAt), UpdatedAt: datetime.DateTime(g.UpdatedAt),
	}
}

// ============================ positions + appointments ============================

func (s EducationService) CreatePosition(ctx context.Context, token bearertoken.Token, institutionID string, req educationapi.CreatePositionRequest) (educationapi.EducationPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return educationapi.EducationPosition{}, err
	}
	p, err := s.app.CreatePosition(ctx, institutionID, domain.PositionInput{Code: req.Code, Title: req.Title, UnitID: req.UnitId, SortOrder: req.SortOrder})
	if err != nil {
		return educationapi.EducationPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s EducationService) ListPositions(ctx context.Context, token bearertoken.Token, institutionID string, state *string, pageSize *int, pageToken *string) (educationapi.PositionPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.PositionPage{}, err
	}
	limit := pageSizeOr(pageSize)
	rows, err := s.app.ListPositions(ctx, institutionID, strOr(state), decodeToken(pageToken), limit)
	if err != nil {
		return educationapi.PositionPage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	defaults := make(map[string]string, len(rows))
	for _, p := range rows {
		defaults[p.ID] = p.Title
	}
	titles, err := s.loc.NamesByID(ctx, entPosition, defaults)
	if err != nil {
		return educationapi.PositionPage{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.EducationPosition, 0, len(rows))
	for _, p := range rows {
		out = append(out, positionAPI(p, titles[p.ID]))
	}
	page := educationapi.PositionPage{Positions: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s EducationService) GetPosition(ctx context.Context, token bearertoken.Token, id string) (educationapi.EducationPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.EducationPosition{}, err
	}
	p, err := s.app.GetPosition(ctx, id)
	if err != nil {
		return educationapi.EducationPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s EducationService) UpdatePosition(ctx context.Context, token bearertoken.Token, id string, req educationapi.UpdatePositionRequest) (educationapi.EducationPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return educationapi.EducationPosition{}, err
	}
	p, err := s.app.UpdatePosition(ctx, id, domain.PositionUpdate{Title: req.Title, SortOrder: req.SortOrder})
	if err != nil {
		return educationapi.EducationPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s EducationService) AbolishPosition(ctx context.Context, token bearertoken.Token, id string) (educationapi.EducationPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return educationapi.EducationPosition{}, err
	}
	p, err := s.app.AbolishPosition(ctx, id)
	if err != nil {
		return educationapi.EducationPosition{}, s.mapError(ctx, err)
	}
	return s.toAPIPosition(ctx, p)
}

func (s EducationService) FillPosition(ctx context.Context, token bearertoken.Token, id string, req educationapi.FillPositionRequest) (educationapi.Appointment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return educationapi.Appointment{}, err
	}
	a, err := s.app.FillPosition(ctx, id, req.PersonId, fromDateTime(req.EffectiveFrom))
	if err != nil {
		return educationapi.Appointment{}, s.mapError(ctx, err)
	}
	return appointmentAPI(a), nil
}

func (s EducationService) EndAppointment(ctx context.Context, token bearertoken.Token, id string, req educationapi.EndAppointmentRequest) (educationapi.Appointment, error) {
	if err := s.pep.RequireAnywhere(ctx, token, positionPerm); err != nil {
		return educationapi.Appointment{}, err
	}
	a, err := s.app.EndAppointment(ctx, id, fromDateTime(req.EffectiveTo))
	if err != nil {
		return educationapi.Appointment{}, s.mapError(ctx, err)
	}
	return appointmentAPI(a), nil
}

func (s EducationService) toAPIPosition(ctx context.Context, p domain.Position) (educationapi.EducationPosition, error) {
	title, err := s.nameMap(ctx, entPosition, p.ID, p.Title)
	if err != nil {
		return educationapi.EducationPosition{}, s.mapError(ctx, err)
	}
	return positionAPI(p, title), nil
}

func positionAPI(p domain.Position, title map[string]string) educationapi.EducationPosition {
	out := educationapi.EducationPosition{
		Id: p.ID, InstitutionId: p.InstitutionID, UnitId: emptyToNil(p.UnitID), Code: p.Code, Title: title,
		Status: p.Status, SortOrder: p.SortOrder, CreatedAt: datetime.DateTime(p.CreatedAt), UpdatedAt: datetime.DateTime(p.UpdatedAt),
	}
	if p.Holder != nil {
		h := appointmentAPI(*p.Holder)
		out.Holder = &h
	}
	return out
}

func appointmentAPI(a domain.Appointment) educationapi.Appointment {
	out := educationapi.Appointment{
		Id: a.ID, PersonId: a.PersonID, PositionId: a.PositionID, Status: a.Status,
		EffectiveFrom: datetime.DateTime(a.EffectiveFrom), CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
	if a.EffectiveTo != nil {
		t := datetime.DateTime(*a.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

// ListPersonAppointments returns the read-only, institution-enriched view of the positions a person
// holds (managed/filled from the institution view; this surfaces them on the person object).
func (s EducationService) ListPersonAppointments(ctx context.Context, token bearertoken.Token, personID string) (educationapi.PersonAppointmentList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return educationapi.PersonAppointmentList{}, err
	}
	ok, err := s.holderReadable(ctx, personID)
	if err != nil {
		return educationapi.PersonAppointmentList{}, s.mapError(ctx, err)
	}
	if !ok { // holder not readable by this subject (D-PersonReadScope): hide as an empty list
		return educationapi.PersonAppointmentList{Appointments: []educationapi.PersonAppointment{}}, nil
	}
	rows, err := s.app.ListPersonAppointments(ctx, personID)
	if err != nil {
		return educationapi.PersonAppointmentList{}, s.mapError(ctx, err)
	}
	out := make([]educationapi.PersonAppointment, 0, len(rows))
	for _, a := range rows {
		out = append(out, personAppointmentAPI(a))
	}
	return educationapi.PersonAppointmentList{Appointments: out}, nil
}

func personAppointmentAPI(a domain.PersonAppointment) educationapi.PersonAppointment {
	out := educationapi.PersonAppointment{
		Id: a.ID, PersonId: a.PersonID, PositionId: a.PositionID,
		PositionTitle: a.PositionTitle, InstitutionId: a.InstitutionID, InstitutionName: a.InstitutionName,
		Status:        a.Status,
		EffectiveFrom: datetime.DateTime(a.EffectiveFrom), CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
	if a.EffectiveTo != nil {
		t := datetime.DateTime(*a.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

// fromDateTime renders an optional datetime to the RFC3339 string the repository parses for a narg.
func fromDateTime(p *datetime.DateTime) *string {
	if p == nil {
		return nil
	}
	s := p.String()
	return &s
}
