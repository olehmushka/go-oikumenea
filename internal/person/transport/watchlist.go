// Watchlists & regulatory exposure transport (D-Watchlists, M34): the live screening check
// (CheckWatchlists — a write, since it persists a match), the single screening result read, and the
// regulatory-sanction overlay CRUD. Reads require person.read; writes require person.update — the same
// PEP gating as the other person sub-resources.
package transport

import (
	"context"
	"time"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- watchlist match (live-lookup)

func (s Service) CheckWatchlists(ctx context.Context, token bearertoken.Token, personID string) (personapi.WatchlistMatch, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.WatchlistMatch{}, err
	}
	m, err := s.app.CheckWatchlists(ctx, personID)
	if err != nil {
		return personapi.WatchlistMatch{}, s.mapError(ctx, err, personID)
	}
	return toAPIWatchlistMatch(m), nil
}

func (s Service) GetWatchlistMatch(ctx context.Context, token bearertoken.Token, personID string) (*personapi.WatchlistMatch, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	m, ok, err := s.app.GetWatchlistMatch(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	if !ok {
		return nil, nil
	}
	out := toAPIWatchlistMatch(m)
	return &out, nil
}

// ---------------------------------------------------------------- regulatory sanctions

func (s Service) ListRegulatorySanctions(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.RegulatorySanction, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	xs, err := s.app.ListRegulatorySanctions(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.RegulatorySanction, 0, len(xs))
	for _, x := range xs {
		out = append(out, toAPIRegulatorySanction(x))
	}
	return out, nil
}

func (s Service) UpsertRegulatorySanction(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertRegulatorySanctionRequest) (personapi.RegulatorySanction, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.RegulatorySanction{}, err
	}
	created, err := s.app.UpsertRegulatorySanction(ctx, domain.RegulatorySanction{
		ID:           derefOr(req.Id, ""),
		PersonID:     personID,
		Regulator:    req.Regulator,
		ActionType:   derefOr(req.ActionType, ""),
		Amount:       req.Amount,
		Currency:     derefOr(req.Currency, ""),
		Status:       derefOr(req.Status, ""),
		SanctionDate: derefOr(req.SanctionDate, ""),
		SourceURL:    derefOr(req.SourceUrl, ""),
		ExternalID:   derefOr(req.ExternalId, ""),
		LegalBasis:   derefOr(req.LegalBasis, ""),
		Source:       derefOr(req.Source, ""),
		Confidence:   derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.RegulatorySanction{}, s.mapError(ctx, err, personID)
	}
	return toAPIRegulatorySanction(created), nil
}

func (s Service) DeleteRegulatorySanction(ctx context.Context, token bearertoken.Token, personID, sanctionID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.app.DeleteRegulatorySanction(ctx, personID, sanctionID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---- mappers ----

func toAPIWatchlistMatch(m domain.WatchlistMatch) personapi.WatchlistMatch {
	lists := m.Lists
	if lists == nil {
		lists = []string{}
	}
	var nextDue *string
	if m.NextCheckDue != nil {
		s := m.NextCheckDue.UTC().Format(time.RFC3339)
		nextDue = &s
	}
	return personapi.WatchlistMatch{
		Id:           m.ID,
		PersonId:     m.PersonID,
		OnList:       m.OnList,
		Lists:        lists,
		Program:      strPtrOrNil(m.Program),
		MatchScore:   m.MatchScore,
		Pep:          m.PEP,
		LastChecked:  m.LastChecked.UTC().Format(time.RFC3339),
		NextCheckDue: nextDue,
		Source:       strPtrOrNil(m.Source),
		Confidence:   strPtrOrNil(m.Confidence),
	}
}

func toAPIRegulatorySanction(x domain.RegulatorySanction) personapi.RegulatorySanction {
	return personapi.RegulatorySanction{
		Id:           x.ID,
		PersonId:     x.PersonID,
		Regulator:    x.Regulator,
		ActionType:   x.ActionType,
		Amount:       x.Amount,
		Currency:     strPtrOrNil(x.Currency),
		Status:       x.Status,
		SanctionDate: strPtrOrNil(x.SanctionDate),
		SourceUrl:    strPtrOrNil(x.SourceURL),
		ExternalId:   strPtrOrNil(x.ExternalID),
		LegalBasis:   strPtrOrNil(x.LegalBasis),
		Source:       strPtrOrNil(x.Source),
		Confidence:   strPtrOrNil(x.Confidence),
	}
}
