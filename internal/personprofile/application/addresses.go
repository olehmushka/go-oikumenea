// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Address orchestration (D-PersonAddresses, M32): a person's precise, effective-dated addresses over the
// shared M19 Location entity. Writes verify the location exists (the late-bound LocationLookup seam),
// enforce a single active primary per person (demote-then-set in one transaction), and record an audit
// row in the same transaction (D-Audit); the audit payloads carry only non-PII identifiers.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// UpsertAddress adds an address (or replaces one when a.ID is set). Defaults the attribution and the
// effective-from date, verifies the person and the referenced location exist, and — when the address is
// marked primary — demotes any other active primary before persisting so the one-primary invariant holds.
func (s *Service) UpsertAddress(ctx context.Context, a domain.Address) (domain.Address, error) {
	if a.Source == "" {
		a.Source = "operator_verified"
	}
	if a.Confidence == "" {
		a.Confidence = "possible"
	}
	if a.ValidFrom == "" {
		a.ValidFrom = s.now().Format("2006-01-02")
	}
	if err := a.Validate(); err != nil {
		return domain.Address{}, err
	}
	var out domain.Address
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, a.PersonID); err != nil {
			return err
		}
		if s.locations != nil {
			if err := s.locations.LocationExists(ctx, a.LocationID); err != nil {
				return domain.ErrUnknownLocation // normalize (unknown id or lookup failure), like ColorLookup
			}
		}
		if a.IsPrimary {
			if err := repo.DemotePrimaryAddresses(ctx, a.PersonID, a.ID); err != nil {
				return err
			}
		}
		created, err := repo.UpsertAddress(ctx, a)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.address.upsert", a.PersonID,
			map[string]any{"id": a.PersonID, "addressId": created.ID, "role": created.Role})
	})
	return out, err
}

// DeleteAddress soft-deletes an address.
func (s *Service) DeleteAddress(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteAddress(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.address.delete", personID, map[string]any{"id": personID, "addressId": id})
	})
}

// ListAddresses lists a person's addresses (the person must exist).
func (s *Service) ListAddresses(ctx context.Context, personID string) ([]domain.Address, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListAddresses(ctx, personID)
}
