// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Name-alias orchestration (D-PhysicalIdentity, M31): alias name forms (aka/former_legal/maiden/
// pseudonym/cover) held in person_name_variants. Aliases are still names, so they stay with the person
// core aggregate (the whole person_name_variants table is core-owned — R-09). The physical descriptions,
// distinguishing marks and the envelope-encrypted declared ethnicity moved to the personsensitive module.
// All writes record an audit row in the same transaction (D-Audit) with only non-PII identifiers.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// ---------------------------------------------------------------- name aliases

// AddNameAlias adds an alias name form (aka/former_legal/maiden/pseudonym/cover) to a person.
func (s *Service) AddNameAlias(ctx context.Context, v domain.NameVariant) (domain.NameVariant, error) {
	if err := v.ValidateAlias(); err != nil {
		return domain.NameVariant{}, err
	}
	var out domain.NameVariant
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, v.PersonID); err != nil {
			return err
		}
		created, err := repo.InsertNameAlias(ctx, v)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.name_alias.add", v.PersonID,
			map[string]any{"id": v.PersonID, "aliasId": created.ID, "variantKind": created.VariantKind})
	})
	return out, err
}

// DeleteNameAlias removes an alias by its RID (holder-scoped to the person).
func (s *Service) DeleteNameAlias(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteNameAlias(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.name_alias.delete", personID, map[string]any{"id": personID, "aliasId": id})
	})
}
