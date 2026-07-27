// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Non-encrypted institutional ties orchestration (D-InstitutionalTies, M33): per-type person↔organization
// affiliation edges — government positions / lobbying relationships / external references, all plaintext
// pii:basic. (The envelope-encrypted declared party membership moved to the personsensitive module under
// the R-09 crypto-consolidation rule.) Every write records an audit row in the same transaction (D-Audit)
// with only non-PII identifiers in the payload.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// ---------------------------------------------------------------- government positions (pii:basic)

// ListGovernmentPositions lists a person's government positions (the person must exist).
func (s *Service) ListGovernmentPositions(ctx context.Context, personID string) ([]domain.GovernmentPosition, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
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
		if err := repo.PersonExists(ctx, g.PersonID); err != nil {
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
	if err := repo.PersonExists(ctx, personID); err != nil {
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
		if err := repo.PersonExists(ctx, l.PersonID); err != nil {
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
	if err := repo.PersonExists(ctx, personID); err != nil {
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
		if err := repo.PersonExists(ctx, r.PersonID); err != nil {
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
