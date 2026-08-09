// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Clergy transport (D-ClergyCredential, M23): catalog reads on religion.read, catalog writes on
// religion.catalog.manage (instance), credential writes on clergy.manage against the conferring org unit
// over the canonical graph. Translatable names assembled as locale->text maps.
package transport

import (
	"context"
	"time"

	religionapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/religion"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// ---- grade categories ----

func (s ReligionService) ListGradeCategories(ctx context.Context, token bearertoken.Token) (religionapi.GradeCategoryList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.GradeCategoryList{}, err
	}
	cats, err := s.app.ListGradeCategories(ctx)
	if err != nil {
		return religionapi.GradeCategoryList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(cats))
	for _, c := range cats {
		defaults[c.ID] = c.Name
	}
	names, err := s.names(ctx, entGradeCategory, defaults)
	if err != nil {
		return religionapi.GradeCategoryList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.GradeCategory, 0, len(cats))
	for _, c := range cats {
		out = append(out, religionapi.GradeCategory{Id: c.ID, TraditionTaxonId: emptyToNil(c.TraditionTaxonID), Code: c.Code, Name: names[c.ID], Ordinal: c.Ordinal, Status: c.Status, SortOrder: c.SortOrder})
	}
	return religionapi.GradeCategoryList{GradeCategories: out}, nil
}

func (s ReligionService) UpsertGradeCategory(ctx context.Context, token bearertoken.Token, req religionapi.UpsertGradeCategoryRequest) (religionapi.GradeCategory, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.GradeCategory{}, err
	}
	c, err := s.app.UpsertGradeCategory(ctx, req.TraditionTaxonId, req.Code, req.Name, req.Ordinal, req.SortOrder)
	if err != nil {
		return religionapi.GradeCategory{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entGradeCategory, c.ID, c.Name)
	if err != nil {
		return religionapi.GradeCategory{}, s.mapError(ctx, err)
	}
	return religionapi.GradeCategory{Id: c.ID, TraditionTaxonId: emptyToNil(c.TraditionTaxonID), Code: c.Code, Name: name, Ordinal: c.Ordinal, Status: c.Status, SortOrder: c.SortOrder}, nil
}

// ---- clergy grades ----

func (s ReligionService) ListClergyGrades(ctx context.Context, token bearertoken.Token, tradition *string) (religionapi.ClergyGradeList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.ClergyGradeList{}, err
	}
	grades, err := s.app.ListClergyGrades(ctx, strOr(tradition))
	if err != nil {
		return religionapi.ClergyGradeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(grades))
	for _, g := range grades {
		defaults[g.ID] = g.Name
	}
	names, err := s.names(ctx, entClergyGrade, defaults)
	if err != nil {
		return religionapi.ClergyGradeList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.ClergyGrade, 0, len(grades))
	for _, g := range grades {
		out = append(out, clergyGradeAPI(g, names[g.ID]))
	}
	return religionapi.ClergyGradeList{ClergyGrades: out}, nil
}

func (s ReligionService) UpsertClergyGrade(ctx context.Context, token bearertoken.Token, req religionapi.UpsertClergyGradeRequest) (religionapi.ClergyGrade, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.ClergyGrade{}, err
	}
	g, err := s.app.UpsertClergyGrade(ctx, req.TraditionTaxonId, req.GradeCategoryId, req.Code, req.Name, req.Ordinal, req.SortOrder)
	if err != nil {
		return religionapi.ClergyGrade{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entClergyGrade, g.ID, g.Name)
	if err != nil {
		return religionapi.ClergyGrade{}, s.mapError(ctx, err)
	}
	return clergyGradeAPI(g, name), nil
}

func clergyGradeAPI(g domain.ClergyGrade, name map[string]string) religionapi.ClergyGrade {
	return religionapi.ClergyGrade{
		Id: g.ID, TraditionTaxonId: emptyToNil(g.TraditionTaxonID), GradeCategoryId: g.GradeCategoryID,
		Code: g.Code, Name: name, Ordinal: g.Ordinal, Status: g.Status, SortOrder: g.SortOrder,
	}
}

// ---- office types ----

func (s ReligionService) ListOfficeTypes(ctx context.Context, token bearertoken.Token) (religionapi.OfficeTypeList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.OfficeTypeList{}, err
	}
	types, err := s.app.ListOfficeTypes(ctx)
	if err != nil {
		return religionapi.OfficeTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.names(ctx, entOfficeType, defaults)
	if err != nil {
		return religionapi.OfficeTypeList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.OfficeType, 0, len(types))
	for _, t := range types {
		out = append(out, religionapi.OfficeType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: names[t.ID], Status: t.Status, SortOrder: t.SortOrder})
	}
	return religionapi.OfficeTypeList{OfficeTypes: out}, nil
}

func (s ReligionService) UpsertOfficeType(ctx context.Context, token bearertoken.Token, req religionapi.UpsertOfficeTypeRequest) (religionapi.OfficeType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.OfficeType{}, err
	}
	t, err := s.app.UpsertOfficeType(ctx, req.TraditionTaxonId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return religionapi.OfficeType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entOfficeType, t.ID, t.Name)
	if err != nil {
		return religionapi.OfficeType{}, s.mapError(ctx, err)
	}
	return religionapi.OfficeType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: name, Status: t.Status, SortOrder: t.SortOrder}, nil
}

// ---- clergy credentials ----

func (s ReligionService) ListPersonClergyCredentials(ctx context.Context, token bearertoken.Token, personID string) (religionapi.ClergyCredentialList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.ClergyCredentialList{}, err
	}
	rows, err := s.app.ListPersonClergyCredentials(ctx, personID)
	if err != nil {
		return religionapi.ClergyCredentialList{}, s.mapError(ctx, err)
	}
	return s.credentialList(ctx, rows)
}

func (s ReligionService) ListUnitClergyCredentials(ctx context.Context, token bearertoken.Token, unitID string) (religionapi.ClergyCredentialList, error) {
	if err := s.pep.Require(ctx, token, readPerm, unitID); err != nil {
		return religionapi.ClergyCredentialList{}, err
	}
	rows, err := s.app.ListUnitClergyCredentials(ctx, unitID)
	if err != nil {
		return religionapi.ClergyCredentialList{}, s.mapError(ctx, err)
	}
	return s.credentialList(ctx, rows)
}

func (s ReligionService) AddClergyCredential(ctx context.Context, token bearertoken.Token, personID string, req religionapi.AddClergyCredentialRequest) (religionapi.ClergyCredential, error) {
	if err := s.pep.Require(ctx, token, clergyPerm, req.OrgUnitId); err != nil {
		return religionapi.ClergyCredential{}, err
	}
	granted, err := parseDate(req.GrantedOn)
	if err != nil {
		return religionapi.ClergyCredential{}, s.mapError(ctx, err)
	}
	c, err := s.app.AddClergyCredential(ctx, domain.ClergyCredentialInput{
		PersonID: personID, ClergyGradeID: req.ClergyGradeId, OrgUnitID: req.OrgUnitId,
		GrantedOn: granted, ConferredByPersonID: strOr(req.ConferredByPersonId),
		Source: strOr(req.Source), Confidence: strOr(req.Confidence),
	})
	if err != nil {
		return religionapi.ClergyCredential{}, s.mapError(ctx, err)
	}
	return s.credentialAPI(ctx, c)
}

func (s ReligionService) UpdateClergyCredential(ctx context.Context, token bearertoken.Token, credentialID string, req religionapi.UpdateClergyCredentialRequest) (religionapi.ClergyCredential, error) {
	// Gate against the credential's conferring org unit (fetch first; reads need only religion.read).
	cur, err := s.app.GetClergyCredential(ctx, credentialID)
	if err != nil {
		return religionapi.ClergyCredential{}, s.mapError(ctx, err)
	}
	if err := s.pep.Require(ctx, token, clergyPerm, cur.OrgUnitID); err != nil {
		return religionapi.ClergyCredential{}, err
	}
	var effectiveTo *time.Time
	if req.EffectiveTo != nil {
		t := time.Time(*req.EffectiveTo)
		effectiveTo = &t
	}
	c, err := s.app.UpdateClergyCredential(ctx, credentialID, domain.ClergyCredentialUpdate{Status: req.Status, EffectiveTo: effectiveTo})
	if err != nil {
		return religionapi.ClergyCredential{}, s.mapError(ctx, err)
	}
	return s.credentialAPI(ctx, c)
}

func (s ReligionService) credentialList(ctx context.Context, rows []domain.ClergyCredential) (religionapi.ClergyCredentialList, error) {
	defaults := make(map[string]string, len(rows))
	for _, c := range rows {
		defaults[c.ClergyGradeID] = c.GradeName
	}
	names, err := s.names(ctx, entClergyGrade, defaults)
	if err != nil {
		return religionapi.ClergyCredentialList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.ClergyCredential, 0, len(rows))
	for _, c := range rows {
		out = append(out, credentialToAPI(c, names[c.ClergyGradeID]))
	}
	return religionapi.ClergyCredentialList{Credentials: out}, nil
}

func (s ReligionService) credentialAPI(ctx context.Context, c domain.ClergyCredential) (religionapi.ClergyCredential, error) {
	name, err := s.nameMap(ctx, entClergyGrade, c.ClergyGradeID, c.GradeName)
	if err != nil {
		return religionapi.ClergyCredential{}, s.mapError(ctx, err)
	}
	return credentialToAPI(c, name), nil
}

func credentialToAPI(c domain.ClergyCredential, gradeName map[string]string) religionapi.ClergyCredential {
	var effectiveTo *datetime.DateTime
	if c.EffectiveTo != nil {
		d := datetime.DateTime(*c.EffectiveTo)
		effectiveTo = &d
	}
	return religionapi.ClergyCredential{
		Id: c.ID, PersonId: c.PersonID, ClergyGradeId: c.ClergyGradeID, GradeCode: c.GradeCode, GradeName: gradeName,
		OrgUnitId: c.OrgUnitID, GrantedOn: dateStr(c.GrantedOn), ConferredByPersonId: emptyToNil(c.ConferredByPersonID),
		Status: c.Status, EffectiveFrom: datetime.DateTime(c.EffectiveFrom), EffectiveTo: effectiveTo,
		Source: emptyToNil(c.Source), Confidence: emptyToNil(c.Confidence),
		CreatedAt: datetime.DateTime(c.CreatedAt), UpdatedAt: datetime.DateTime(c.UpdatedAt),
	}
}

// ---- date helpers (YYYY-MM-DD day strings) ----

func dateStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func parseDate(p *string) (*time.Time, error) {
	if p == nil || *p == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *p)
	if err != nil {
		return nil, domain.ErrInvalid
	}
	return &t, nil
}
