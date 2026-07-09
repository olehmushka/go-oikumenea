// Financial / behavioural / psychological overlays transport (D-PersonOverlays, M35): crypto wallets and
// personality profiles (pii:sensitive) and the inferred political leaning (pii:special). Reads require
// person.read; writes require person.update — the same PEP gating as the other person sub-resources.
package transport

import (
	"context"

	"github.com/olegamysk/go-oikumenea/internal/person/domain"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- crypto wallets

func (s Service) ListCryptoWallets(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.CryptoWallet, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ws, err := s.sensitive.ListCryptoWallets(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.CryptoWallet, 0, len(ws))
	for _, w := range ws {
		out = append(out, toAPICryptoWallet(w))
	}
	return out, nil
}

func (s Service) UpsertCryptoWallet(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertCryptoWalletRequest) (personapi.CryptoWallet, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.CryptoWallet{}, err
	}
	created, err := s.sensitive.UpsertCryptoWallet(ctx, domain.CryptoWallet{
		ID:                derefOr(req.Id, ""),
		PersonID:          personID,
		Address:           req.Address,
		Chain:             derefOr(req.Chain, ""),
		AttributionMethod: derefOr(req.AttributionMethod, ""),
		BalanceUSDApprox:  req.BalanceUsdApprox,
		FirstSeen:         derefOr(req.FirstSeen, ""),
		LastSeen:          derefOr(req.LastSeen, ""),
		Source:            derefOr(req.Source, ""),
		Confidence:        derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.CryptoWallet{}, s.mapError(ctx, err, personID)
	}
	return toAPICryptoWallet(created), nil
}

func (s Service) DeleteCryptoWallet(ctx context.Context, token bearertoken.Token, personID, walletID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteCryptoWallet(ctx, personID, walletID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- personality

func (s Service) ListPersonalities(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Personality, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	ps, err := s.sensitive.ListPersonalities(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Personality, 0, len(ps))
	for _, p := range ps {
		out = append(out, toAPIPersonality(p))
	}
	return out, nil
}

func (s Service) UpsertPersonality(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPersonalityRequest) (personapi.Personality, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Personality{}, err
	}
	created, err := s.sensitive.UpsertPersonality(ctx, domain.Personality{
		ID:         derefOr(req.Id, ""),
		PersonID:   personID,
		Framework:  derefOr(req.Framework, ""),
		Result:     req.Result,
		Instrument: derefOr(req.Instrument, ""),
		Method:     derefOr(req.Method, ""),
		AssessedAt: derefOr(req.AssessedAt, ""),
		Source:     derefOr(req.Source, ""),
		Confidence: derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.Personality{}, s.mapError(ctx, err, personID)
	}
	return toAPIPersonality(created), nil
}

func (s Service) DeletePersonality(ctx context.Context, token bearertoken.Token, personID, personalityID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeletePersonality(ctx, personID, personalityID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- political leaning

func (s Service) GetPoliticalLeaning(ctx context.Context, token bearertoken.Token, personID string) (*personapi.PoliticalLeaning, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	l, err := s.sensitive.GetPoliticalLeaning(ctx, personID)
	if err != nil {
		if err == domain.ErrPoliticalLeaningNotFound {
			return nil, nil
		}
		return nil, s.mapError(ctx, err, personID)
	}
	out := toAPIPoliticalLeaning(l)
	return &out, nil
}

func (s Service) SetPoliticalLeaning(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertPoliticalLeaningRequest) (personapi.PoliticalLeaning, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.PoliticalLeaning{}, err
	}
	var sources []string
	if req.InferenceSources != nil {
		sources = *req.InferenceSources
	}
	created, err := s.sensitive.SetPoliticalLeaning(ctx, domain.PoliticalLeaning{
		PersonID:         personID,
		Spectrum:         req.Spectrum,
		InferenceSources: sources,
		AssessedAt:       derefOr(req.AssessedAt, ""),
		LegalBasis:       req.LegalBasis,
		Confidence:       derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.PoliticalLeaning{}, s.mapError(ctx, err, personID)
	}
	return toAPIPoliticalLeaning(created), nil
}

func (s Service) DeletePoliticalLeaning(ctx context.Context, token bearertoken.Token, personID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeletePoliticalLeaning(ctx, personID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---- mappers ----

func toAPICryptoWallet(w domain.CryptoWallet) personapi.CryptoWallet {
	return personapi.CryptoWallet{
		Id:                w.ID,
		PersonId:          w.PersonID,
		Address:           w.Address,
		Chain:             w.Chain,
		AttributionMethod: w.AttributionMethod,
		BalanceUsdApprox:  w.BalanceUSDApprox,
		FirstSeen:         strPtrOrNil(w.FirstSeen),
		LastSeen:          strPtrOrNil(w.LastSeen),
		Source:            strPtrOrNil(w.Source),
		Confidence:        strPtrOrNil(w.Confidence),
	}
}

func toAPIPersonality(p domain.Personality) personapi.Personality {
	return personapi.Personality{
		Id:         p.ID,
		PersonId:   p.PersonID,
		Framework:  p.Framework,
		Result:     p.Result,
		Instrument: strPtrOrNil(p.Instrument),
		Method:     p.Method,
		AssessedAt: strPtrOrNil(p.AssessedAt),
		Source:     strPtrOrNil(p.Source),
		Confidence: strPtrOrNil(p.Confidence),
	}
}

func toAPIPoliticalLeaning(l domain.PoliticalLeaning) personapi.PoliticalLeaning {
	sources := l.InferenceSources
	if sources == nil {
		sources = []string{}
	}
	return personapi.PoliticalLeaning{
		Id:               l.ID,
		PersonId:         l.PersonID,
		Spectrum:         l.Spectrum,
		InferenceSources: sources,
		AssessedAt:       strPtrOrNil(l.AssessedAt),
		LegalBasis:       l.LegalBasis,
		Confidence:       strPtrOrNil(l.Confidence),
	}
}
