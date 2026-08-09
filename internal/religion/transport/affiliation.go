// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Lay-affiliation transport (D-ReligiousAffiliation / D-SpecialPII, M24): catalog reads on religion.read,
// catalog writes on religion.catalog.manage; affiliation reads + writes on affiliation.manage (the value
// is GDPR Art. 9 pii:special — decrypted only for authorized readers).
package transport

import (
	"context"

	religionapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/religion"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

// ---- affiliation types ----

func (s ReligionService) ListAffiliationTypes(ctx context.Context, token bearertoken.Token) (religionapi.AffiliationTypeList, error) {
	if err := s.pep.RequireServiceOrPerson(ctx, token, readPerm, ""); err != nil {
		return religionapi.AffiliationTypeList{}, err
	}
	types, err := s.app.ListAffiliationTypes(ctx)
	if err != nil {
		return religionapi.AffiliationTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.names(ctx, entAffiliationType, defaults)
	if err != nil {
		return religionapi.AffiliationTypeList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.AffiliationType, 0, len(types))
	for _, t := range types {
		out = append(out, religionapi.AffiliationType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: names[t.ID], Status: t.Status, SortOrder: t.SortOrder})
	}
	return religionapi.AffiliationTypeList{AffiliationTypes: out}, nil
}

func (s ReligionService) UpsertAffiliationType(ctx context.Context, token bearertoken.Token, req religionapi.UpsertAffiliationTypeRequest) (religionapi.AffiliationType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return religionapi.AffiliationType{}, err
	}
	t, err := s.app.UpsertAffiliationType(ctx, req.TraditionTaxonId, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return religionapi.AffiliationType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entAffiliationType, t.ID, t.Name)
	if err != nil {
		return religionapi.AffiliationType{}, s.mapError(ctx, err)
	}
	return religionapi.AffiliationType{Id: t.ID, TraditionTaxonId: emptyToNil(t.TraditionTaxonID), Code: t.Code, Name: name, Status: t.Status, SortOrder: t.SortOrder}, nil
}

// ---- affiliations (pii:special) ----

func (s ReligionService) ListPersonAffiliations(ctx context.Context, token bearertoken.Token, personID string) (religionapi.AffiliationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, affiliationPerm); err != nil {
		return religionapi.AffiliationList{}, err
	}
	rows, err := s.app.ListPersonAffiliations(ctx, personID)
	if err != nil {
		return religionapi.AffiliationList{}, s.mapError(ctx, err)
	}
	return s.affiliationList(ctx, rows)
}

func (s ReligionService) AddAffiliation(ctx context.Context, token bearertoken.Token, personID string, req religionapi.AddAffiliationRequest) (religionapi.Affiliation, error) {
	if err := s.pep.RequireAnywhere(ctx, token, affiliationPerm); err != nil {
		return religionapi.Affiliation{}, err
	}
	a, err := s.app.AddAffiliation(ctx, domain.AffiliationInput{
		PersonID: personID, AffiliationTypeID: req.AffiliationTypeId,
		ReligionID: strOr(req.ReligionId), TraditionUnitID: strOr(req.TraditionUnitId), CommunityUnitID: strOr(req.CommunityUnitId),
		Source: strOr(req.Source), Confidence: strOr(req.Confidence),
	}, strOr(req.Value))
	if err != nil {
		return religionapi.Affiliation{}, s.mapError(ctx, err)
	}
	return s.affiliationAPI(ctx, a)
}

func (s ReligionService) UpdateAffiliation(ctx context.Context, token bearertoken.Token, affiliationID string, req religionapi.UpdateAffiliationRequest) (religionapi.Affiliation, error) {
	if err := s.pep.RequireAnywhere(ctx, token, affiliationPerm); err != nil {
		return religionapi.Affiliation{}, err
	}
	a, err := s.app.UpdateAffiliation(ctx, affiliationID, req.Status, req.Value)
	if err != nil {
		return religionapi.Affiliation{}, s.mapError(ctx, err)
	}
	return s.affiliationAPI(ctx, a)
}

func (s ReligionService) DeleteAffiliation(ctx context.Context, token bearertoken.Token, affiliationID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, affiliationPerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteAffiliation(ctx, affiliationID))
}

func (s ReligionService) affiliationList(ctx context.Context, rows []domain.Affiliation) (religionapi.AffiliationList, error) {
	defaults := make(map[string]string, len(rows))
	for _, a := range rows {
		defaults[a.AffiliationTypeID] = a.AffiliationTypeName
	}
	names, err := s.names(ctx, entAffiliationType, defaults)
	if err != nil {
		return religionapi.AffiliationList{}, s.mapError(ctx, err)
	}
	out := make([]religionapi.Affiliation, 0, len(rows))
	for _, a := range rows {
		out = append(out, affiliationToAPI(a, names[a.AffiliationTypeID]))
	}
	return religionapi.AffiliationList{Affiliations: out}, nil
}

func (s ReligionService) affiliationAPI(ctx context.Context, a domain.Affiliation) (religionapi.Affiliation, error) {
	name, err := s.nameMap(ctx, entAffiliationType, a.AffiliationTypeID, a.AffiliationTypeName)
	if err != nil {
		return religionapi.Affiliation{}, s.mapError(ctx, err)
	}
	return affiliationToAPI(a, name), nil
}

func affiliationToAPI(a domain.Affiliation, typeName map[string]string) religionapi.Affiliation {
	var effectiveTo *datetime.DateTime
	if a.EffectiveTo != nil {
		d := datetime.DateTime(*a.EffectiveTo)
		effectiveTo = &d
	}
	return religionapi.Affiliation{
		Id: a.ID, PersonId: a.PersonID, ReligionId: emptyToNil(a.ReligionID),
		TraditionUnitId: emptyToNil(a.TraditionUnitID), CommunityUnitId: emptyToNil(a.CommunityUnitID),
		AffiliationTypeId: a.AffiliationTypeID, AffiliationTypeCode: a.AffiliationTypeCode, AffiliationTypeName: typeName,
		Value: emptyToNil(a.Value), Status: a.Status,
		EffectiveFrom: datetime.DateTime(a.EffectiveFrom), EffectiveTo: effectiveTo,
		Source: emptyToNil(a.Source), Confidence: emptyToNil(a.Confidence),
		CreatedAt: datetime.DateTime(a.CreatedAt), UpdatedAt: datetime.DateTime(a.UpdatedAt),
	}
}
