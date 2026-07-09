// Encrypted declared party membership orchestration (D-InstitutionalTies, M33). The party membership is
// envelope-encrypted (political opinion is GDPR Art. 9 special category — sealed on write, decrypted on
// read, crypto-erased on purge, exactly like the M31 ethnicity), so it lives in personsensitive alongside
// the other pii:special stores (R-09 crypto-consolidation rule). The non-encrypted institutional ties
// (government positions / lobbying / external references) stay in personprofile. Every write records an
// audit row in the same transaction (D-Audit) with only non-PII identifiers in the payload.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

// ---------------------------------------------------------------- party memberships (pii:special)

// ListPartyMemberships returns a person's decrypted party memberships (the person must exist).
func (s *Service) ListPartyMemberships(ctx context.Context, personID string) ([]domain.PartyMembership, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	stored, err := repo.ListPartyMemberships(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PartyMembership, 0, len(stored))
	for _, st := range stored {
		p, err := s.openParty(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// UpsertPartyMembership seals the party identity and stores the encrypted link (or replaces it when p.ID
// is set). legal_basis is required (Art. 9).
func (s *Service) UpsertPartyMembership(ctx context.Context, p domain.PartyMembership) (domain.PartyMembership, error) {
	if p.Party == "" {
		return domain.PartyMembership{}, domain.ErrInvalid
	}
	if p.Role == "" {
		p.Role = "member"
	}
	if p.Status == "" {
		p.Status = "active"
	}
	if p.Source == "" {
		p.Source = "operator_verified"
	}
	if p.Confidence == "" {
		p.Confidence = "possible"
	}
	if err := p.Validate(); err != nil {
		return domain.PartyMembership{}, err
	}
	sealed, blind, err := s.seal(ctx, p.Party)
	if err != nil {
		return domain.PartyMembership{}, err
	}
	rec := domain.StoredPartyMembership{
		ID:              p.ID,
		PersonID:        p.PersonID,
		PartyCiphertext: sealed.Ciphertext,
		PartyWrappedDEK: sealed.WrappedDEK,
		PartyKeyRef:     sealed.KeyRef,
		PartyBlindIndex: blind,
		Role:            p.Role,
		ValidFrom:       p.ValidFrom,
		ValidTo:         p.ValidTo,
		LegalBasis:      p.LegalBasis,
		Status:          p.Status,
		Source:          p.Source,
		Confidence:      p.Confidence,
	}
	var out domain.PartyMembership
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, p.PersonID); err != nil {
			return err
		}
		var st domain.StoredPartyMembership
		var err error
		if p.ID == "" {
			st, err = repo.InsertPartyMembership(ctx, rec)
		} else {
			st, err = repo.UpdatePartyMembership(ctx, rec)
		}
		if err != nil {
			return err
		}
		out = storedToParty(st, p.Party)
		return s.record(ctx, tx, "person.party_membership.upsert", p.PersonID,
			map[string]any{"id": p.PersonID, "partyMembershipId": st.ID, "legalBasis": p.LegalBasis})
	})
	return out, err
}

// DeletePartyMembership soft-deletes a party membership.
func (s *Service) DeletePartyMembership(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeletePartyMembership(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.party_membership.delete", personID, map[string]any{"id": personID, "partyMembershipId": id})
	})
}

// ---- helpers ----

func (s *Service) openParty(ctx context.Context, st domain.StoredPartyMembership) (domain.PartyMembership, error) {
	party := ""
	if len(st.PartyCiphertext) > 0 && len(st.PartyWrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.PartyCiphertext, WrappedDEK: st.PartyWrappedDEK, KeyRef: st.PartyKeyRef})
		if err != nil {
			return domain.PartyMembership{}, err
		}
		party = string(plain)
	}
	return storedToParty(st, party), nil
}

func storedToParty(st domain.StoredPartyMembership, party string) domain.PartyMembership {
	return domain.PartyMembership{
		ID:         st.ID,
		PersonID:   st.PersonID,
		Party:      party,
		Role:       st.Role,
		ValidFrom:  st.ValidFrom,
		ValidTo:    st.ValidTo,
		LegalBasis: st.LegalBasis,
		Status:     st.Status,
		Source:     st.Source,
		Confidence: st.Confidence,
		CreatedAt:  st.CreatedAt,
		UpdatedAt:  st.UpdatedAt,
	}
}
