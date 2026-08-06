// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Clergy orchestration (D-ClergyCredential, M23): audited catalog + credential writes. A clergy
// credential is a public directory fact (no encryption); revocation is a status flip, never a delete.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
)

// ---- catalogs ----

func (s *Service) ListGradeCategories(ctx context.Context) ([]domain.GradeCategory, error) {
	return s.newRepo(s.querier(ctx)).ListGradeCategories(ctx)
}

func (s *Service) UpsertGradeCategory(ctx context.Context, traditionTaxonID *string, code, name string, ordinal, sortOrder *int) (domain.GradeCategory, error) {
	var out domain.GradeCategory
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertGradeCategory(ctx, traditionTaxonID, code, name, ordinal, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.grade-category.upsert", v.ID, "", v)
	})
	return out, err
}

func (s *Service) ListClergyGrades(ctx context.Context, tradition string) ([]domain.ClergyGrade, error) {
	return s.newRepo(s.querier(ctx)).ListClergyGrades(ctx, tradition)
}

func (s *Service) UpsertClergyGrade(ctx context.Context, traditionTaxonID *string, gradeCategoryID, code, name string, ordinal int, sortOrder *int) (domain.ClergyGrade, error) {
	var out domain.ClergyGrade
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertClergyGrade(ctx, traditionTaxonID, gradeCategoryID, code, name, ordinal, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.clergy-grade.upsert", v.ID, "", v)
	})
	return out, err
}

func (s *Service) ListOfficeTypes(ctx context.Context) ([]domain.OfficeType, error) {
	return s.newRepo(s.querier(ctx)).ListOfficeTypes(ctx)
}

func (s *Service) UpsertOfficeType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.OfficeType, error) {
	var out domain.OfficeType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertOfficeType(ctx, traditionTaxonID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.office-type.upsert", v.ID, "", v)
	})
	return out, err
}

// ---- clergy credentials ----

func (s *Service) ListPersonClergyCredentials(ctx context.Context, personID string) ([]domain.ClergyCredential, error) {
	return s.newRepo(s.querier(ctx)).ListClergyCredentialsByPerson(ctx, personID)
}

func (s *Service) ListUnitClergyCredentials(ctx context.Context, unitID string) ([]domain.ClergyCredential, error) {
	return s.newRepo(s.querier(ctx)).ListClergyCredentialsByUnit(ctx, unitID)
}

func (s *Service) GetClergyCredential(ctx context.Context, id string) (domain.ClergyCredential, error) {
	return s.newRepo(s.querier(ctx)).GetClergyCredential(ctx, id)
}

func (s *Service) AddClergyCredential(ctx context.Context, in domain.ClergyCredentialInput) (domain.ClergyCredential, error) {
	if err := in.Validate(); err != nil {
		return domain.ClergyCredential{}, err
	}
	var out domain.ClergyCredential
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := s.newRepo(tx).InsertClergyCredential(ctx, in)
		if err != nil {
			return err
		}
		out = c
		return s.record(ctx, tx, "religion.clergy-credential.add", c.ID, c.OrgUnitID, c)
	})
	return out, err
}

func (s *Service) UpdateClergyCredential(ctx context.Context, id string, up domain.ClergyCredentialUpdate) (domain.ClergyCredential, error) {
	if up.Status != nil && !domain.ValidCredentialStatus(*up.Status) {
		return domain.ClergyCredential{}, domain.ErrInvalid
	}
	var out domain.ClergyCredential
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := s.newRepo(tx).UpdateClergyCredential(ctx, id, up)
		if err != nil {
			return err
		}
		out = c
		return s.record(ctx, tx, "religion.clergy-credential.update", c.ID, c.OrgUnitID, c)
	})
	return out, err
}
