// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Person↔person relationships orchestration (D-PersonRelationships, R-09 split): the reified typed links
// (partnership/kinship/guardianship/sponsorship/next-of-kin/association) plus the relation-type catalog.
// All writes record an audit row in the same transaction (D-Audit) with only non-PII identifiers.
package application

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
)

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships)

// ListRelationTypes returns the instance-admin relation-label catalog (read; no person scope).
func (s *Service) ListRelationTypes(ctx context.Context) ([]domain.RelationType, error) {
	return s.newRepo(s.pool).ListRelationTypes(ctx)
}

// canonicalPair orders two person ids ascending (the canonical-pair invariant person_id_a < person_id_b).
func canonicalPair(x, y string) (string, string) {
	if x <= y {
		return x, y
	}
	return y, x
}

// requireCounterpart confirms the other endpoint is a real directory person, mapping a missing person to
// ErrUnknownCounterpart (the path person's own existence is checked separately and stays ErrNotFound).
func (s *Service) requireCounterpart(ctx context.Context, repo domain.ProfileRepository, id string) error {
	if err := repo.PersonExists(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrUnknownCounterpart
		}
		return err
	}
	return nil
}

// checkRelationCode validates an optional relation-type code: "" is allowed; otherwise the code must
// exist and (when wantCategory != "") sit in that category.
func checkRelationCode(ctx context.Context, repo domain.ProfileRepository, code, wantCategory string) error {
	if code == "" {
		return nil
	}
	rt, err := repo.GetRelationType(ctx, code)
	if err != nil {
		return err // ErrUnknownRelationType
	}
	if wantCategory != "" && rt.Category != wantCategory {
		return domain.ErrRelationCategory
	}
	return nil
}

// UpsertPartnership records/replaces a partnership between personID and the partner (a symmetric,
// canonically ordered pair), enforcing the single-active-engaged/married-per-person rule for both ends.
func (s *Service) UpsertPartnership(ctx context.Context, personID string, p domain.Partnership) (domain.Partnership, error) {
	if err := p.Validate(); err != nil {
		return domain.Partnership{}, err
	}
	counterpart := otherEndpoint(personID, p.PersonIDA, p.PersonIDB)
	if counterpart == personID {
		return domain.Partnership{}, domain.ErrSelfRelationship
	}
	p.PersonIDA, p.PersonIDB = canonicalPair(personID, counterpart)
	var out domain.Partnership
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if p.IsActivePartnership() {
			for _, who := range []string{personID, counterpart} {
				has, err := repo.HasActivePartnershipExcept(ctx, who, p.ID)
				if err != nil {
					return err
				}
				if has {
					return domain.ErrPartnershipConflict
				}
			}
		}
		saved, err := repo.UpsertPartnership(ctx, p)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.partnership.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertKinship records/replaces a directional parent→child kinship (endpoints set by the transport per role).
func (s *Service) UpsertKinship(ctx context.Context, personID string, k domain.Kinship) (domain.Kinship, error) {
	if k.Status == "" {
		k.Status = "active"
	}
	if err := k.Validate(); err != nil {
		return domain.Kinship{}, err
	}
	if k.ParentID == k.ChildID {
		return domain.Kinship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, k.ParentID, k.ChildID)
	var out domain.Kinship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		saved, err := repo.UpsertKinship(ctx, k)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.kinship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertGuardianship records/replaces a guardian→ward link (relation_code optional, any category).
func (s *Service) UpsertGuardianship(ctx context.Context, personID string, g domain.Guardianship) (domain.Guardianship, error) {
	if g.Status == "" {
		g.Status = "active"
	}
	if err := g.Validate(); err != nil {
		return domain.Guardianship{}, err
	}
	if g.GuardianID == g.WardID {
		return domain.Guardianship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, g.GuardianID, g.WardID)
	var out domain.Guardianship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, g.RelationCode, ""); err != nil {
			return err
		}
		saved, err := repo.UpsertGuardianship(ctx, g)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.guardianship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertSponsorship records/replaces a sponsor→sponsored link (relation_code required, category=sponsorship).
func (s *Service) UpsertSponsorship(ctx context.Context, personID string, sp domain.Sponsorship) (domain.Sponsorship, error) {
	if sp.Status == "" {
		sp.Status = "active"
	}
	if err := sp.Validate(); err != nil {
		return domain.Sponsorship{}, err
	}
	if sp.SponsorID == sp.SponsoredID {
		return domain.Sponsorship{}, domain.ErrSelfRelationship
	}
	counterpart := otherEndpoint(personID, sp.SponsorID, sp.SponsoredID)
	var out domain.Sponsorship
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, sp.RelationCode, domain.RelCategorySponsorship); err != nil {
			return err
		}
		saved, err := repo.UpsertSponsorship(ctx, sp)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.sponsorship.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertNextOfKin nominates/replaces a next-of-kin contact for the subject (personID).
func (s *Service) UpsertNextOfKin(ctx context.Context, personID string, n domain.NextOfKin) (domain.NextOfKin, error) {
	if n.Status == "" {
		n.Status = "active"
	}
	if n.Priority == 0 {
		n.Priority = 1
	}
	if err := n.Validate(); err != nil {
		return domain.NextOfKin{}, err
	}
	n.SubjectID = personID
	if n.SubjectID == n.ContactID {
		return domain.NextOfKin{}, domain.ErrSelfRelationship
	}
	var out domain.NextOfKin
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, n.ContactID); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, n.RelationCode, domain.RelCategoryNextOfKin); err != nil {
			return err
		}
		saved, err := repo.UpsertNextOfKin(ctx, n)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.next-of-kin.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// UpsertAssociation records/replaces a symmetric association (relation_code optional, category=association).
func (s *Service) UpsertAssociation(ctx context.Context, personID string, a domain.Association) (domain.Association, error) {
	if a.Status == "" {
		a.Status = "active"
	}
	if err := a.Validate(); err != nil {
		return domain.Association{}, err
	}
	counterpart := otherEndpoint(personID, a.PersonIDA, a.PersonIDB)
	if counterpart == personID {
		return domain.Association{}, domain.ErrSelfRelationship
	}
	a.PersonIDA, a.PersonIDB = canonicalPair(personID, counterpart)
	var out domain.Association
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, personID); err != nil {
			return err
		}
		if err := s.requireCounterpart(ctx, repo, counterpart); err != nil {
			return err
		}
		if err := checkRelationCode(ctx, repo, a.RelationCode, domain.RelCategoryAssociation); err != nil {
			return err
		}
		saved, err := repo.UpsertAssociation(ctx, a)
		if err != nil {
			return err
		}
		out = saved
		return s.record(ctx, tx, "person.association.upsert", personID, map[string]any{"id": personID, "relationshipId": saved.ID})
	})
	return out, err
}

// DeleteRelationship removes any person↔person link by id; the link table is decoded from the RID and
// the delete is holder-scoped (the person must be an endpoint). Idempotent at the transport layer.
func (s *Service) DeleteRelationship(ctx context.Context, personID, relationshipID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		var err error
		switch domain.RelationLinkType(relationshipID) {
		case domain.LinkPartnership:
			err = repo.DeletePartnership(ctx, personID, relationshipID)
		case domain.LinkKinship:
			err = repo.DeleteKinship(ctx, personID, relationshipID)
		case domain.LinkGuardianship:
			err = repo.DeleteGuardianship(ctx, personID, relationshipID)
		case domain.LinkSponsorship:
			err = repo.DeleteSponsorship(ctx, personID, relationshipID)
		case domain.LinkNextOfKin:
			err = repo.DeleteNextOfKin(ctx, personID, relationshipID)
		case domain.LinkAssociation:
			err = repo.DeleteAssociation(ctx, personID, relationshipID)
		default:
			return domain.ErrUnknownRelationshipKind
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, "person.relationship.delete", personID, map[string]any{"id": personID, "relationshipId": relationshipID})
	})
}

// relationship list reads (holder-scoped: the person must exist; rows touch either endpoint)

func (s *Service) ListPartnerships(ctx context.Context, personID string) ([]domain.Partnership, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPartnerships(ctx, personID)
}

func (s *Service) ListKinships(ctx context.Context, personID string) ([]domain.Kinship, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListKinships(ctx, personID)
}

func (s *Service) ListGuardianships(ctx context.Context, personID string) ([]domain.Guardianship, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListGuardianships(ctx, personID)
}

func (s *Service) ListSponsorships(ctx context.Context, personID string) ([]domain.Sponsorship, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListSponsorships(ctx, personID)
}

func (s *Service) ListNextOfKin(ctx context.Context, personID string) ([]domain.NextOfKin, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListNextOfKin(ctx, personID)
}

func (s *Service) ListAssociations(ctx context.Context, personID string) ([]domain.Association, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListAssociations(ctx, personID)
}

// otherEndpoint returns whichever of a/b is not personID (b when neither matches — a transport invariant
// guarantees one endpoint is the path person).
func otherEndpoint(personID, a, b string) string {
	if a == personID {
		return b
	}
	if b == personID {
		return a
	}
	return b
}
