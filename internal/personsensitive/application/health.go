// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Health & vulnerability orchestration (D-HealthVulnerability, M36). Insurance is plaintext pii:sensitive
// (hard-erased on purge). Health records are envelope-encrypted pii:special: the category-level detail is
// sealed on write, decrypted on read and crypto-erased on purge (exactly like the M33 party / M35 leaning),
// with a required legal_basis (Art. 9) and a need-to-know read gate enforced in transport. Health is NEVER
// inferred. Every write records an audit row in the same transaction (D-Audit) with only non-PII
// identifiers in the payload.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

// ---------------------------------------------------------------- health records (pii:special)

// ListHealthRecords returns a person's decrypted category-level health records (the person must exist).
func (s *Service) ListHealthRecords(ctx context.Context, personID string) ([]domain.HealthRecord, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	stored, err := repo.ListHealthRecords(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.HealthRecord, 0, len(stored))
	for _, st := range stored {
		h, err := s.openHealth(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// UpsertHealthRecord seals the category-level detail and stores it as the single active row for the
// (person, kind) pair (insert when none exists, otherwise re-seal in place). legal_basis is required
// (Art. 9). Category-level only — never a diagnosis.
func (s *Service) UpsertHealthRecord(ctx context.Context, h domain.HealthRecord) (domain.HealthRecord, error) {
	if h.Source == "" {
		h.Source = "imported"
	}
	if h.Confidence == "" {
		h.Confidence = "possible"
	}
	if err := h.Validate(); err != nil {
		return domain.HealthRecord{}, err
	}
	sealed, blind, err := s.seal(ctx, h.Detail)
	if err != nil {
		return domain.HealthRecord{}, err
	}
	rec := domain.StoredHealthRecord{
		PersonID:         h.PersonID,
		Kind:             h.Kind,
		DetailCiphertext: sealed.Ciphertext,
		DetailWrappedDEK: sealed.WrappedDEK,
		DetailKeyRef:     sealed.KeyRef,
		DetailBlindIndex: blind,
		IsPublicRecord:   h.IsPublicRecord,
		AssessedAt:       h.AssessedAt,
		LegalBasis:       h.LegalBasis,
		Source:           h.Source,
		Confidence:       h.Confidence,
	}
	var out domain.HealthRecord
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, h.PersonID); err != nil {
			return err
		}
		st, err := repo.UpdateHealthRecord(ctx, rec)
		if err != nil {
			if err == domain.ErrHealthRecordNotFound {
				st, err = repo.InsertHealthRecord(ctx, rec)
			}
			if err != nil {
				return err
			}
		}
		out = storedToHealth(st, h.Detail)
		return s.record(ctx, tx, "person.health_record.upsert", h.PersonID,
			map[string]any{"id": h.PersonID, "healthRecordId": st.ID, "kind": st.Kind, "legalBasis": h.LegalBasis})
	})
	return out, err
}

// DeleteHealthRecord soft-deletes a person's health record.
func (s *Service) DeleteHealthRecord(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteHealthRecord(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.health_record.delete", personID, map[string]any{"id": personID, "healthRecordId": id})
	})
}

// ---------------------------------------------------------------- insurance (pii:sensitive)

// ListInsurance returns a person's insurance coverage rows (the person must exist).
func (s *Service) ListInsurance(ctx context.Context, personID string) ([]domain.Insurance, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListInsurance(ctx, personID)
}

// UpsertInsurance adds an insurance row (or replaces one when i.ID is set).
func (s *Service) UpsertInsurance(ctx context.Context, i domain.Insurance) (domain.Insurance, error) {
	if i.Source == "" {
		i.Source = "imported"
	}
	if i.Confidence == "" {
		i.Confidence = "possible"
	}
	if err := i.Validate(); err != nil {
		return domain.Insurance{}, err
	}
	var out domain.Insurance
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, i.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertInsurance(ctx, i)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.insurance.upsert", i.PersonID,
			map[string]any{"id": i.PersonID, "insuranceId": created.ID, "type": created.Type})
	})
	return out, err
}

// DeleteInsurance soft-deletes an insurance row.
func (s *Service) DeleteInsurance(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteInsurance(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.insurance.delete", personID, map[string]any{"id": personID, "insuranceId": id})
	})
}

// ---- helpers ----

// openHealth decrypts the sealed category-level detail (a crypto-erased tombstone yields "").
func (s *Service) openHealth(ctx context.Context, st domain.StoredHealthRecord) (domain.HealthRecord, error) {
	detail := ""
	if len(st.DetailCiphertext) > 0 && len(st.DetailWrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.DetailCiphertext, WrappedDEK: st.DetailWrappedDEK, KeyRef: st.DetailKeyRef})
		if err != nil {
			return domain.HealthRecord{}, err
		}
		detail = string(plain)
	}
	return storedToHealth(st, detail), nil
}

func storedToHealth(st domain.StoredHealthRecord, detail string) domain.HealthRecord {
	return domain.HealthRecord{
		ID:             st.ID,
		PersonID:       st.PersonID,
		Kind:           st.Kind,
		Detail:         detail,
		IsPublicRecord: st.IsPublicRecord,
		AssessedAt:     st.AssessedAt,
		LegalBasis:     st.LegalBasis,
		Source:         st.Source,
		Confidence:     st.Confidence,
		CreatedAt:      st.CreatedAt,
		UpdatedAt:      st.UpdatedAt,
	}
}
