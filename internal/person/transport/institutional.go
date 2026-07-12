// Institutional & political ties transport (D-InstitutionalTies, M33): party memberships (pii:special),
// government positions / lobbying relationships (pii:basic) and external references (pii:basic). Reads
// require person.read; writes require person.update — the same PEP gating as the other person sub-resources.
package transport

import (
	"context"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	werror "github.com/palantir/witchcraft-go-error"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- party memberships

func (s Service) ListPartyMemberships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.PartyMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permPartyMembershipRead); err != nil {
		return nil, err
	}
	ps, err := s.sensitive.ListPartyMemberships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.PartyMembership, 0, len(ps))
	for _, p := range ps {
		out = append(out, toAPIPartyMembership(p))
	}
	return out, nil
}

func (s Service) UpsertPartyMembership(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPartyMembershipRequest) (personapi.PartyMembership, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.PartyMembership{}, err
	}
	created, err := s.sensitive.UpsertPartyMembership(ctx, domain.PartyMembership{
		ID:         derefOr(req.Id, ""),
		PersonID:   personID,
		Party:      req.Party,
		Role:       derefOr(req.Role, ""),
		ValidFrom:  derefOr(req.ValidFrom, ""),
		ValidTo:    derefOr(req.ValidTo, ""),
		LegalBasis: req.LegalBasis,
		Status:     derefOr(req.Status, ""),
		Source:     derefOr(req.Source, ""),
		Confidence: derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.PartyMembership{}, s.mapError(ctx, err, personID)
	}
	return toAPIPartyMembership(created), nil
}

func (s Service) DeletePartyMembership(ctx context.Context, token bearertoken.Token, personID, membershipID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeletePartyMembership(ctx, personID, membershipID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- government positions

func (s Service) ListGovernmentPositions(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.GovernmentPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	gs, err := s.profile.ListGovernmentPositions(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.GovernmentPosition, 0, len(gs))
	for _, g := range gs {
		out = append(out, toAPIGovernmentPosition(g))
	}
	return out, nil
}

func (s Service) UpsertGovernmentPosition(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertGovernmentPositionRequest) (personapi.GovernmentPosition, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.GovernmentPosition{}, err
	}
	created, err := s.profile.UpsertGovernmentPosition(ctx, domain.GovernmentPosition{
		ID:         derefOr(req.Id, ""),
		PersonID:   personID,
		Title:      req.Title,
		Body:       req.Body,
		OrgID:      derefOr(req.OrgId, ""),
		CountryID:  derefOr(req.CountryId, ""),
		Level:      derefOr(req.Level, ""),
		RoleType:   derefOr(req.RoleType, ""),
		ValidFrom:  derefOr(req.ValidFrom, ""),
		ValidTo:    derefOr(req.ValidTo, ""),
		PEPTrigger: derefOr(req.PepTrigger, true),
		Source:     derefOr(req.Source, ""),
		Confidence: derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.GovernmentPosition{}, s.mapError(ctx, err, personID)
	}
	return toAPIGovernmentPosition(created), nil
}

func (s Service) DeleteGovernmentPosition(ctx context.Context, token bearertoken.Token, personID, positionID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeleteGovernmentPosition(ctx, personID, positionID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- lobbying relationships

func (s Service) ListLobbyingRelationships(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.LobbyingRelationship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ls, err := s.profile.ListLobbyingRelationships(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.LobbyingRelationship, 0, len(ls))
	for _, l := range ls {
		out = append(out, toAPILobbying(l))
	}
	return out, nil
}

func (s Service) UpsertLobbyingRelationship(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertLobbyingRelationshipRequest) (personapi.LobbyingRelationship, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.LobbyingRelationship{}, err
	}
	var issues []string
	if req.Issues != nil {
		issues = *req.Issues
	}
	created, err := s.profile.UpsertLobbyingRelationship(ctx, domain.LobbyingRelationship{
		ID:              derefOr(req.Id, ""),
		PersonID:        personID,
		Registrant:      req.Registrant,
		Client:          derefOr(req.Client, ""),
		LegislativeBody: derefOr(req.LegislativeBody, ""),
		Issues:          issues,
		FilingID:        derefOr(req.FilingId, ""),
		SourceURL:       derefOr(req.SourceUrl, ""),
		ValidFrom:       derefOr(req.ValidFrom, ""),
		ValidTo:         derefOr(req.ValidTo, ""),
		Source:          derefOr(req.Source, ""),
		Confidence:      derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.LobbyingRelationship{}, s.mapError(ctx, err, personID)
	}
	return toAPILobbying(created), nil
}

func (s Service) DeleteLobbyingRelationship(ctx context.Context, token bearertoken.Token, personID, relationshipID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeleteLobbyingRelationship(ctx, personID, relationshipID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- external references

func (s Service) ListExternalReferences(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.ExternalReference, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	rs, err := s.profile.ListExternalReferences(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.ExternalReference, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPIExternalReference(r))
	}
	return out, nil
}

func (s Service) UpsertExternalReference(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertExternalReferenceRequest) (personapi.ExternalReference, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.ExternalReference{}, err
	}
	var categories []string
	if req.Categories != nil {
		categories = *req.Categories
	}
	lastChecked, err := parseRFC3339Ptr(req.LastChecked)
	if err != nil {
		return personapi.ExternalReference{}, s.mapError(ctx, domain.ErrInvalid, personID)
	}
	created, err := s.profile.UpsertExternalReference(ctx, domain.ExternalReference{
		ID:          derefOr(req.Id, ""),
		PersonID:    personID,
		Kind:        derefOr(req.Kind, ""),
		URL:         req.Url,
		ExternalID:  derefOr(req.ExternalId, ""),
		Categories:  categories,
		LastChecked: lastChecked,
		Disputed:    derefOr(req.Disputed, false),
		Source:      derefOr(req.Source, ""),
		Confidence:  derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.ExternalReference{}, s.mapError(ctx, err, personID)
	}
	return toAPIExternalReference(created), nil
}

func (s Service) DeleteExternalReference(ctx context.Context, token bearertoken.Token, personID, referenceID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.profile.DeleteExternalReference(ctx, personID, referenceID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---- mappers ----

func toAPIPartyMembership(p domain.PartyMembership) personapi.PartyMembership {
	return personapi.PartyMembership{
		Id:         p.ID,
		PersonId:   p.PersonID,
		Party:      p.Party,
		Role:       p.Role,
		ValidFrom:  strPtrOrNil(p.ValidFrom),
		ValidTo:    strPtrOrNil(p.ValidTo),
		LegalBasis: p.LegalBasis,
		Status:     p.Status,
		Source:     strPtrOrNil(p.Source),
		Confidence: strPtrOrNil(p.Confidence),
	}
}

func toAPIGovernmentPosition(g domain.GovernmentPosition) personapi.GovernmentPosition {
	return personapi.GovernmentPosition{
		Id:         g.ID,
		PersonId:   g.PersonID,
		Title:      g.Title,
		Body:       g.Body,
		OrgId:      strPtrOrNil(g.OrgID),
		CountryId:  strPtrOrNil(g.CountryID),
		Level:      g.Level,
		RoleType:   strPtrOrNil(g.RoleType),
		ValidFrom:  strPtrOrNil(g.ValidFrom),
		ValidTo:    strPtrOrNil(g.ValidTo),
		PepTrigger: g.PEPTrigger,
		Source:     strPtrOrNil(g.Source),
		Confidence: strPtrOrNil(g.Confidence),
	}
}

func toAPILobbying(l domain.LobbyingRelationship) personapi.LobbyingRelationship {
	issues := l.Issues
	if issues == nil {
		issues = []string{}
	}
	return personapi.LobbyingRelationship{
		Id:              l.ID,
		PersonId:        l.PersonID,
		Registrant:      l.Registrant,
		Client:          strPtrOrNil(l.Client),
		LegislativeBody: strPtrOrNil(l.LegislativeBody),
		Issues:          issues,
		FilingId:        strPtrOrNil(l.FilingID),
		SourceUrl:       strPtrOrNil(l.SourceURL),
		ValidFrom:       strPtrOrNil(l.ValidFrom),
		ValidTo:         strPtrOrNil(l.ValidTo),
		Source:          strPtrOrNil(l.Source),
		Confidence:      strPtrOrNil(l.Confidence),
	}
}

func toAPIExternalReference(r domain.ExternalReference) personapi.ExternalReference {
	categories := r.Categories
	if categories == nil {
		categories = []string{}
	}
	var lastChecked *string
	if r.LastChecked != nil {
		s := r.LastChecked.UTC().Format(time.RFC3339)
		lastChecked = &s
	}
	return personapi.ExternalReference{
		Id:          r.ID,
		PersonId:    r.PersonID,
		Kind:        r.Kind,
		Url:         r.URL,
		ExternalId:  strPtrOrNil(r.ExternalID),
		Categories:  categories,
		LastChecked: lastChecked,
		Disputed:    r.Disputed,
		Source:      strPtrOrNil(r.Source),
		Confidence:  strPtrOrNil(r.Confidence),
	}
}

// parseRFC3339Ptr parses an optional RFC-3339 timestamp string into an *time.Time (nil string => nil).
func parseRFC3339Ptr(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, werror.Wrap(err, "invalid RFC-3339 timestamp")
	}
	tu := t.UTC()
	return &tu, nil
}
