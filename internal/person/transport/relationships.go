// Package transport (R-09 file split): person↔person relationship and catalog handlers for the one PersonService Conjure surface.
package transport

import (
	"context"
	"errors"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships)

func (s Service) ListRelationTypes(ctx context.Context, token bearertoken.Token) ([]personapi.RelationType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.profile.ListRelationTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.Code] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, relationTypeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.RelationType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.RelationType{
			Code: t.Code, Name: names[t.Code], Category: t.Category, Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

// directedEndpoints maps the path person + counterpart + role to a directional (from, to) pair, ok=false
// for an unrecognized role.
func directedEndpoints(personID, counterpart, role, fromRole, toRole string) (from, to string, ok bool) {
	switch role {
	case fromRole:
		return personID, counterpart, true
	case toRole:
		return counterpart, personID, true
	}
	return "", "", false
}

func (s Service) ListPartnerships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Partnership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permPartnershipRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListPartnerships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Partnership, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIPartnership(r))
	}
	return out, nil
}

func (s Service) UpsertPartnership(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPartnershipRequest) (personapi.Partnership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Partnership{}, err
	}
	saved, err := s.profile.UpsertPartnership(ctx, personID, domain.Partnership{
		ID:            derefOr(req.Id, ""),
		PersonIDA:     personID,
		PersonIDB:     req.PartnerId,
		Status:        req.Status,
		EffectiveFrom: derefOr(req.EffectiveFrom, ""),
		EffectiveTo:   derefOr(req.EffectiveTo, ""),
	})
	if err != nil {
		return personapi.Partnership{}, s.mapError(ctx, err, personID)
	}
	return toAPIPartnership(saved), nil
}

func (s Service) ListKinships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Kinship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permKinshipRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListKinships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Kinship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIKinship(r))
	}
	return out, nil
}

func (s Service) UpsertKinship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertKinshipRequest) (personapi.Kinship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Kinship{}, err
	}
	parent, child, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "parent", "child")
	if !ok {
		return personapi.Kinship{}, personapi.NewPersonInvalid("role must be parent or child")
	}
	saved, err := s.profile.UpsertKinship(ctx, personID, domain.Kinship{
		ID: derefOr(req.Id, ""), ParentID: parent, ChildID: child, Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.Kinship{}, s.mapError(ctx, err, personID)
	}
	return toAPIKinship(saved), nil
}

func (s Service) ListGuardianships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Guardianship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permGuardianshipRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListGuardianships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Guardianship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIGuardianship(r))
	}
	return out, nil
}

func (s Service) UpsertGuardianship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertGuardianshipRequest) (personapi.Guardianship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Guardianship{}, err
	}
	guardian, ward, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "guardian", "ward")
	if !ok {
		return personapi.Guardianship{}, personapi.NewPersonInvalid("role must be guardian or ward")
	}
	saved, err := s.profile.UpsertGuardianship(ctx, personID, domain.Guardianship{
		ID: derefOr(req.Id, ""), GuardianID: guardian, WardID: ward, RelationCode: derefOr(req.RelationCode, ""),
		Status: derefOr(req.Status, ""), EffectiveFrom: derefOr(req.EffectiveFrom, ""), EffectiveTo: derefOr(req.EffectiveTo, ""),
	})
	if err != nil {
		return personapi.Guardianship{}, s.mapError(ctx, err, personID)
	}
	return toAPIGuardianship(saved), nil
}

func (s Service) ListSponsorships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Sponsorship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permSponsorshipRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListSponsorships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Sponsorship, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPISponsorship(r))
	}
	return out, nil
}

func (s Service) UpsertSponsorship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertSponsorshipRequest) (personapi.Sponsorship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Sponsorship{}, err
	}
	sponsor, sponsored, ok := directedEndpoints(personID, req.CounterpartId, req.Role, "sponsor", "sponsored")
	if !ok {
		return personapi.Sponsorship{}, personapi.NewPersonInvalid("role must be sponsor or sponsored")
	}
	saved, err := s.profile.UpsertSponsorship(ctx, personID, domain.Sponsorship{
		ID: derefOr(req.Id, ""), SponsorID: sponsor, SponsoredID: sponsored, RelationCode: req.RelationCode,
		Status: derefOr(req.Status, ""), EffectiveFrom: derefOr(req.EffectiveFrom, ""), EffectiveTo: derefOr(req.EffectiveTo, ""),
		EnrollmentID: derefOr(req.EnrollmentId, ""), EducationRole: derefOr(req.EducationRole, ""),
	})
	if err != nil {
		return personapi.Sponsorship{}, s.mapError(ctx, err, personID)
	}
	return toAPISponsorship(saved), nil
}

func (s Service) ListNextOfKin(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.NextOfKin, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permNextOfKinRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListNextOfKin(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.NextOfKin, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPINextOfKin(r))
	}
	return out, nil
}

func (s Service) UpsertNextOfKin(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertNextOfKinRequest) (personapi.NextOfKin, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.NextOfKin{}, err
	}
	saved, err := s.profile.UpsertNextOfKin(ctx, personID, domain.NextOfKin{
		ID: derefOr(req.Id, ""), SubjectID: personID, ContactID: req.ContactId,
		RelationCode: derefOr(req.RelationCode, ""), Priority: derefOr(req.Priority, 0), Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.NextOfKin{}, s.mapError(ctx, err, personID)
	}
	return toAPINextOfKin(saved), nil
}

func (s Service) ListAssociations(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Association, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permAssociationRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListAssociations(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Association, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIAssociation(r))
	}
	return out, nil
}

func (s Service) UpsertAssociation(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertAssociationRequest) (personapi.Association, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Association{}, err
	}
	saved, err := s.profile.UpsertAssociation(ctx, personID, domain.Association{
		ID: derefOr(req.Id, ""), PersonIDA: personID, PersonIDB: req.CounterpartId,
		RelationCode: derefOr(req.RelationCode, ""), Kind: req.Kind, Status: derefOr(req.Status, ""),
	})
	if err != nil {
		return personapi.Association{}, s.mapError(ctx, err, personID)
	}
	return toAPIAssociation(saved), nil
}

func (s Service) DeleteRelationship(ctx context.Context, token bearertoken.Token, personID, relationshipID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeleteRelationship(ctx, personID, relationshipID); err != nil {
		if errors.Is(err, domain.ErrRelationshipNotFound) { // idempotent: a missing link is a no-op
			return nil
		}
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- platform catalog

func (s Service) ListPlatforms(ctx context.Context, token bearertoken.Token) ([]personapi.Platform, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	platforms, err := s.profile.ListPlatforms(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	defaults := make(map[string]string, len(platforms))
	for _, p := range platforms {
		defaults[p.Code] = p.Name
	}
	names, err := s.loc.NamesByID(ctx, platformEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.Platform, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, personapi.Platform{
			Code: p.Code, Name: names[p.Code], Category: p.Category, Status: p.Status, SortOrder: sortOrderPtr(p.SortOrder),
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- contact-kind catalogs

func (s Service) ListEmailTypes(ctx context.Context, token bearertoken.Token) ([]personapi.EmailType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.profile.ListEmailTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	names, err := s.contactTypeNames(ctx, emailTypeEntity, types)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.EmailType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.EmailType{
			Code: t.Code, Name: names[t.Code], Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

func (s Service) ListPhoneTypes(ctx context.Context, token bearertoken.Token) ([]personapi.PhoneType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	types, err := s.profile.ListPhoneTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	names, err := s.contactTypeNames(ctx, phoneTypeEntity, types)
	if err != nil {
		return nil, s.mapError(ctx, err, "")
	}
	out := make([]personapi.PhoneType, 0, len(types))
	for _, t := range types {
		out = append(out, personapi.PhoneType{
			Code: t.Code, Name: names[t.Code], Status: t.Status, SortOrder: sortOrderPtr(t.SortOrder),
		})
	}
	return out, nil
}

// contactTypeNames assembles each catalog row's translatable `name` as a locale->text map, keyed by
// the type code, with the default-locale `name` as the fallback (D-i18n).
func (s Service) contactTypeNames(ctx context.Context, entity string, types []domain.ContactType) (map[string]map[string]string, error) {
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.Code] = t.Name
	}
	return s.loc.NamesByID(ctx, entity, defaults)
}
