// Package transport (R-09 file split): physical identity, ethnicity, name-alias + address handlers for the one PersonService Conjure surface.
package transport

import (
	"context"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- physical identity (M31)

func (s Service) AddNameAlias(ctx context.Context, token bearertoken.Token, personID string, req personapi.AddNameAliasRequest) (personapi.NameVariant, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.NameVariant{}, err
	}
	created, err := s.app.AddNameAlias(ctx, domain.NameVariant{
		PersonID: personID,
		Locale:   req.Locale,
		Name: nameFromParts(req.DisplayName, req.Title, req.Given, req.Given2, req.Surname,
			req.SurnamePrefix, req.Surname2, req.Generation, req.Credentials, req.Preferred),
		VariantKind: req.VariantKind,
		Source:      derefOr(req.Source, ""),
		Confidence:  derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.NameVariant{}, s.mapError(ctx, err, personID)
	}
	return toAPIVariant(created), nil
}

func (s Service) DeleteNameAlias(ctx context.Context, token bearertoken.Token, personID, aliasID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteNameAlias(ctx, personID, aliasID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

func (s Service) ListPhysicalDescriptions(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.PhysicalDescription, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ds, err := s.sensitive.ListPhysicalDescriptions(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.PhysicalDescription, 0, len(ds))
	for _, d := range ds {
		out = append(out, toAPIPhysicalDescription(d))
	}
	return out, nil
}

func (s Service) UpsertPhysicalDescription(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPhysicalDescriptionRequest) (personapi.PhysicalDescription, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.PhysicalDescription{}, err
	}
	created, err := s.sensitive.UpsertPhysicalDescription(ctx, domain.PhysicalDescription{
		ID:            derefOr(req.Id, ""),
		PersonID:      personID,
		HeightCm:      req.HeightCm,
		WeightKg:      req.WeightKg,
		EyeColorID:    derefOr(req.EyeColorId, ""),
		HairColorID:   derefOr(req.HairColorId, ""),
		Build:         derefOr(req.Build, ""),
		BloodType:     derefOr(req.BloodType, ""),
		EffectiveFrom: derefOr(req.EffectiveFrom, ""),
		EffectiveTo:   derefOr(req.EffectiveTo, ""),
		Source:        derefOr(req.Source, ""),
		Confidence:    derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.PhysicalDescription{}, s.mapError(ctx, err, personID)
	}
	return toAPIPhysicalDescription(created), nil
}

func (s Service) DeletePhysicalDescription(ctx context.Context, token bearertoken.Token, personID, descriptionID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeletePhysicalDescription(ctx, personID, descriptionID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

func (s Service) ListDistinguishingMarks(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.DistinguishingMark, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ms, err := s.sensitive.ListDistinguishingMarks(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.DistinguishingMark, 0, len(ms))
	for _, m := range ms {
		out = append(out, toAPIDistinguishingMark(m))
	}
	return out, nil
}

func (s Service) UpsertDistinguishingMark(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertDistinguishingMarkRequest) (personapi.DistinguishingMark, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.DistinguishingMark{}, err
	}
	created, err := s.sensitive.UpsertDistinguishingMark(ctx, domain.DistinguishingMark{
		ID:           derefOr(req.Id, ""),
		PersonID:     personID,
		Kind:         req.Kind,
		BodyLocation: derefOr(req.BodyLocation, ""),
		Description:  derefOr(req.Description, ""),
		Source:       derefOr(req.Source, ""),
		Confidence:   derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.DistinguishingMark{}, s.mapError(ctx, err, personID)
	}
	return toAPIDistinguishingMark(created), nil
}

func (s Service) DeleteDistinguishingMark(ctx context.Context, token bearertoken.Token, personID, markID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteDistinguishingMark(ctx, personID, markID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

func (s Service) ListAddresses(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Address, error) {
	// Where a person lives is its own disclosure (D-LinkPermissions) — same code as the lives_at
	// traversal arm, so the person page and the object graph agree.
	if err := s.pep.RequireAnywhere(ctx, token, permAddressRead); err != nil {
		return nil, err
	}
	as, err := s.profile.ListAddresses(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Address, 0, len(as))
	for _, a := range as {
		out = append(out, toAPIAddress(a))
	}
	return out, nil
}

func (s Service) UpsertAddress(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertAddressRequest) (personapi.Address, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Address{}, err
	}
	created, err := s.profile.UpsertAddress(ctx, domain.Address{
		ID:             derefOr(req.Id, ""),
		PersonID:       personID,
		LocationID:     req.LocationId,
		Role:           req.Role,
		ValidFrom:      derefOr(req.ValidFrom, ""),
		ValidTo:        derefOr(req.ValidTo, ""),
		IsPrimary:      derefOr(req.IsPrimary, false),
		PrivacySeeking: derefOr(req.PrivacySeeking, false),
		Source:         derefOr(req.Source, ""),
		Confidence:     derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.Address{}, s.mapError(ctx, err, personID)
	}
	return toAPIAddress(created), nil
}

func (s Service) DeleteAddress(ctx context.Context, token bearertoken.Token, personID, addressID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeleteAddress(ctx, personID, addressID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

func (s Service) ListEthnicityTypes(ctx context.Context, token bearertoken.Token, topLevel *bool, parent *string, query *string, limit *int) ([]personapi.EthnicityType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	f := domain.EthnicityTypeFilter{Parent: derefOr(parent, ""), Query: derefOr(query, "")}
	if topLevel != nil {
		f.TopLevel = *topLevel
	}
	if limit != nil {
		f.Limit = *limit
	}
	types, err := s.sensitive.ListEthnicityTypes(ctx, f)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.Code] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, ethnicityTypeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.EthnicityType, 0, len(types))
	for _, t := range types {
		out = append(out, toAPIEthnicityType(t, names[t.Code], nil, nil))
	}
	return out, nil
}

func (s Service) GetEthnicityType(ctx context.Context, token bearertoken.Token, ethnicityTypeID string) (personapi.EthnicityType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return personapi.EthnicityType{}, err
	}
	t, langs, countries, err := s.sensitive.GetEthnicityType(ctx, ethnicityTypeID)
	if err != nil {
		return personapi.EthnicityType{}, s.mapError(ctx, err, "")
	}
	names, err := s.loc.NamesByID(ctx, ethnicityTypeEntity, map[string]string{t.Code: t.Name})
	if err != nil {
		return personapi.EthnicityType{}, s.mapError(ctx, err, "")
	}
	return toAPIEthnicityType(t, names[t.Code], langs, countries), nil
}

func (s Service) UpsertEthnicityType(ctx context.Context, token bearertoken.Token, req personapi.UpsertEthnicityTypeRequest) (personapi.EthnicityType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.EthnicityType{}, err
	}
	created, err := s.sensitive.UpsertEthnicityType(ctx, domain.EthnicityType{
		Code:       req.Code,
		Name:       req.Name,
		ParentID:   derefOr(req.ParentId, ""),
		WikidataID: derefOr(req.WikidataId, ""),
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		return personapi.EthnicityType{}, s.mapError(ctx, err, "")
	}
	// the just-written default-locale name, projected as a single-entry locale->text map fallback.
	names, err := s.loc.NamesByID(ctx, ethnicityTypeEntity, map[string]string{created.Code: created.Name})
	if err != nil {
		return personapi.EthnicityType{}, s.mapError(ctx, err, "")
	}
	return toAPIEthnicityType(created, names[created.Code], nil, nil), nil
}

// toAPIEthnicityType maps a domain ethnicity type + its resolved i18n name (+ optional group-level
// language/country RIDs) to the wire type. languages/countries are nil for list rows (populated only by
// getEthnicityType) and serialized as empty lists.
func toAPIEthnicityType(t domain.EthnicityType, name map[string]string, langs, countries []string) personapi.EthnicityType {
	if langs == nil {
		langs = []string{}
	}
	if countries == nil {
		countries = []string{}
	}
	out := personapi.EthnicityType{
		Id: t.ID, Code: t.Code, Name: name, HasChildren: t.HasChildren,
		Status: t.Status, SortOrder: t.SortOrder, Languages: langs, Countries: countries,
	}
	if t.ParentID != "" {
		out.ParentId = &t.ParentID
	}
	if t.WikidataID != "" {
		out.WikidataId = &t.WikidataID
	}
	return out
}

func (s Service) ListEthnicities(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Ethnicity, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permEthnicityRead); err != nil {
		return nil, err
	}
	es, err := s.sensitive.ListEthnicities(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Ethnicity, 0, len(es))
	for _, e := range es {
		out = append(out, toAPIEthnicity(e))
	}
	return out, nil
}

func (s Service) AddEthnicity(ctx context.Context, token bearertoken.Token, personID string, req personapi.AddEthnicityRequest) (personapi.Ethnicity, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Ethnicity{}, err
	}
	created, err := s.sensitive.AddEthnicity(ctx, personID, req.Code, req.LegalBasis, derefOr(req.Source, ""), derefOr(req.Confidence, ""))
	if err != nil {
		return personapi.Ethnicity{}, s.mapError(ctx, err, personID)
	}
	return toAPIEthnicity(created), nil
}

func (s Service) UpdateEthnicity(ctx context.Context, token bearertoken.Token, personID, ethnicityID string, req personapi.UpdateEthnicityRequest) (personapi.Ethnicity, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Ethnicity{}, err
	}
	created, err := s.sensitive.UpdateEthnicity(ctx, personID, ethnicityID, req.Code, req.LegalBasis, derefOr(req.Status, ""))
	if err != nil {
		return personapi.Ethnicity{}, s.mapError(ctx, err, personID)
	}
	return toAPIEthnicity(created), nil
}

func (s Service) DeleteEthnicity(ctx context.Context, token bearertoken.Token, personID, ethnicityID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteEthnicity(ctx, personID, ethnicityID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}
