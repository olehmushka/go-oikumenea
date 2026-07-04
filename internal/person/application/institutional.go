// Institutional & political ties orchestration (D-InstitutionalTies, M33): per-type person↔organization
// affiliation edges. The party membership is envelope-encrypted (political opinion is GDPR Art. 9 special
// category — sealed on write, decrypted on read, crypto-erased on purge, exactly like the M31 ethnicity);
// government positions / lobbying relationships / external references are plaintext pii:basic. Every write
// records an audit row in the same transaction (D-Audit) with only non-PII identifiers in the payload.
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
	if _, err := repo.GetPerson(ctx, personID); err != nil {
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
		if _, err := repo.GetPerson(ctx, p.PersonID); err != nil {
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

// ---------------------------------------------------------------- government positions (pii:basic)

// ListGovernmentPositions lists a person's government positions (the person must exist).
func (s *Service) ListGovernmentPositions(ctx context.Context, personID string) ([]domain.GovernmentPosition, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListGovernmentPositions(ctx, personID)
}

// UpsertGovernmentPosition adds a government position (or replaces one when g.ID is set).
func (s *Service) UpsertGovernmentPosition(ctx context.Context, g domain.GovernmentPosition) (domain.GovernmentPosition, error) {
	if g.Level == "" {
		g.Level = "national"
	}
	if g.Source == "" {
		g.Source = "operator_verified"
	}
	if g.Confidence == "" {
		g.Confidence = "possible"
	}
	if g.ID == "" {
		g.PEPTrigger = true // a government position is politically exposing by definition (D-InstitutionalTies)
	}
	if err := g.Validate(); err != nil {
		return domain.GovernmentPosition{}, err
	}
	var out domain.GovernmentPosition
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, g.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertGovernmentPosition(ctx, g)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.government_position.upsert", g.PersonID,
			map[string]any{"id": g.PersonID, "positionId": created.ID, "pepTrigger": created.PEPTrigger})
	})
	return out, err
}

// DeleteGovernmentPosition soft-deletes a government position.
func (s *Service) DeleteGovernmentPosition(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteGovernmentPosition(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.government_position.delete", personID, map[string]any{"id": personID, "positionId": id})
	})
}

// IsPoliticallyExposed reports whether the person has any active PEP-triggering position (the M34 seam).
func (s *Service) IsPoliticallyExposed(ctx context.Context, personID string) (bool, error) {
	return s.newRepo(s.pool).IsPoliticallyExposed(ctx, personID)
}

// ---------------------------------------------------------------- lobbying relationships (pii:basic)

// ListLobbyingRelationships lists a person's lobbying relationships (the person must exist).
func (s *Service) ListLobbyingRelationships(ctx context.Context, personID string) ([]domain.LobbyingRelationship, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListLobbyingRelationships(ctx, personID)
}

// UpsertLobbyingRelationship adds a lobbying relationship (or replaces one when l.ID is set).
func (s *Service) UpsertLobbyingRelationship(ctx context.Context, l domain.LobbyingRelationship) (domain.LobbyingRelationship, error) {
	if l.Source == "" {
		l.Source = "operator_verified"
	}
	if l.Confidence == "" {
		l.Confidence = "possible"
	}
	if err := l.Validate(); err != nil {
		return domain.LobbyingRelationship{}, err
	}
	var out domain.LobbyingRelationship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, l.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertLobbyingRelationship(ctx, l)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.lobbying.upsert", l.PersonID,
			map[string]any{"id": l.PersonID, "lobbyingId": created.ID})
	})
	return out, err
}

// DeleteLobbyingRelationship soft-deletes a lobbying relationship.
func (s *Service) DeleteLobbyingRelationship(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteLobbyingRelationship(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.lobbying.delete", personID, map[string]any{"id": personID, "lobbyingId": id})
	})
}

// ---------------------------------------------------------------- external references (pii:basic)

// ListExternalReferences lists a person's external references (the person must exist).
func (s *Service) ListExternalReferences(ctx context.Context, personID string) ([]domain.ExternalReference, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListExternalReferences(ctx, personID)
}

// UpsertExternalReference adds an external reference (idempotent by URL when r.ID is empty) or replaces the
// named row by RID.
func (s *Service) UpsertExternalReference(ctx context.Context, r domain.ExternalReference) (domain.ExternalReference, error) {
	if r.Kind == "" {
		r.Kind = "other"
	}
	if r.Source == "" {
		r.Source = "imported"
	}
	if r.Confidence == "" {
		r.Confidence = "possible"
	}
	if err := r.Validate(); err != nil {
		return domain.ExternalReference{}, err
	}
	var out domain.ExternalReference
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, r.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertExternalReference(ctx, r)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.external_reference.upsert", r.PersonID,
			map[string]any{"id": r.PersonID, "referenceId": created.ID, "kind": created.Kind})
	})
	return out, err
}

// DeleteExternalReference soft-deletes an external reference.
func (s *Service) DeleteExternalReference(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteExternalReference(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.external_reference.delete", personID, map[string]any{"id": personID, "referenceId": id})
	})
}

// ---- helpers ----

// openParty decrypts the sealed party identity (a crypto-erased tombstone yields party "").
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
