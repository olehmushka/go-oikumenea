// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Criminal / arrest / court records orchestration (D-LegalRecords, M38). Legal records are
// envelope-encrypted pii:special: the category-level offence detail is sealed on write, decrypted on
// read and crypto-erased on purge (exactly like the M36 health detail), with a required legal_basis
// (Art. 10) and a need-to-know read gate enforced in transport. Two things differ from health:
// `disposition` is mandatory (arrest ≠ guilt), and sealed/expunged records are SUPPRESSED — retained
// but withheld from the normal read gate unless the caller holds the read-suppressed capability
// (computed in transport and passed as includeSuppressed). Never inferred. Every write records an
// audit row in the same transaction (D-Audit) with only non-PII identifiers in the payload.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/pkg/crypto"
)

// ListLegalRecords returns a person's decrypted category-level legal records. Suppressed rows are
// included only when includeSuppressed (the caller holds person.legal-record.read-suppressed).
func (s *Service) ListLegalRecords(ctx context.Context, personID string, includeSuppressed bool) ([]domain.LegalRecord, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	stored, err := repo.ListLegalRecords(ctx, personID, includeSuppressed)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LegalRecord, 0, len(stored))
	for _, st := range stored {
		r, err := s.openLegal(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// UpsertLegalRecord seals the category-level offence detail and stores it (insert when r.ID is empty,
// otherwise replace that row in place). disposition is mandatory (arrest ≠ guilt); legal_basis is
// required (Art. 10). Category-level only — never a full charge sheet.
func (s *Service) UpsertLegalRecord(ctx context.Context, r domain.LegalRecord) (domain.LegalRecord, error) {
	if r.Source == "" {
		r.Source = "imported"
	}
	if r.Confidence == "" {
		r.Confidence = "possible"
	}
	if err := r.Validate(); err != nil {
		return domain.LegalRecord{}, err
	}
	sealed, blind, err := s.seal(ctx, r.Detail)
	if err != nil {
		return domain.LegalRecord{}, err
	}
	rec := domain.StoredLegalRecord{
		ID:               r.ID,
		PersonID:         r.PersonID,
		Kind:             r.Kind,
		Disposition:      r.Disposition,
		DetailCiphertext: sealed.Ciphertext,
		DetailWrappedDEK: sealed.WrappedDEK,
		DetailKeyRef:     sealed.KeyRef,
		DetailBlindIndex: blind,
		OccurredAt:       r.OccurredAt,
		DispositionDate:  r.DispositionDate,
		IsSuppressed:     r.IsSuppressed,
		SuppressedReason: r.SuppressedReason,
		LegalBasis:       r.LegalBasis,
		Source:           r.Source,
		Confidence:       r.Confidence,
	}
	var out domain.LegalRecord
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, r.PersonID); err != nil {
			return err
		}
		// Resolve the jurisdiction ISO code to its geo_countries RID (D-Geo hard FK).
		if r.Jurisdiction != "" {
			id, err := repo.ResolveCountryID(ctx, r.Jurisdiction)
			if err != nil {
				return err
			}
			rec.JurisdictionCountryID = id
		}
		var st domain.StoredLegalRecord
		if rec.ID == "" {
			st, err = repo.InsertLegalRecord(ctx, rec)
		} else {
			st, err = repo.UpdateLegalRecord(ctx, rec)
		}
		if err != nil {
			return err
		}
		out = storedToLegal(st, r.Detail, r.Jurisdiction)
		return s.record(ctx, tx, "person.legal_record.upsert", r.PersonID,
			map[string]any{"id": r.PersonID, "legalRecordId": st.ID, "kind": st.Kind, "disposition": st.Disposition, "legalBasis": r.LegalBasis})
	})
	return out, err
}

// DeleteLegalRecord soft-deletes a person's legal record.
func (s *Service) DeleteLegalRecord(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteLegalRecord(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.legal_record.delete", personID, map[string]any{"id": personID, "legalRecordId": id})
	})
}

// ---- helpers ----

// openLegal decrypts the sealed category-level offence detail (a crypto-erased tombstone yields "").
func (s *Service) openLegal(ctx context.Context, st domain.StoredLegalRecord) (domain.LegalRecord, error) {
	detail := ""
	if len(st.DetailCiphertext) > 0 && len(st.DetailWrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.DetailCiphertext, WrappedDEK: st.DetailWrappedDEK, KeyRef: st.DetailKeyRef})
		if err != nil {
			return domain.LegalRecord{}, err
		}
		detail = string(plain)
	}
	return storedToLegal(st, detail, st.Jurisdiction), nil
}

func storedToLegal(st domain.StoredLegalRecord, detail, jurisdiction string) domain.LegalRecord {
	return domain.LegalRecord{
		ID:               st.ID,
		PersonID:         st.PersonID,
		Kind:             st.Kind,
		Disposition:      st.Disposition,
		Detail:           detail,
		Jurisdiction:     jurisdiction,
		OccurredAt:       st.OccurredAt,
		DispositionDate:  st.DispositionDate,
		IsSuppressed:     st.IsSuppressed,
		SuppressedReason: st.SuppressedReason,
		LegalBasis:       st.LegalBasis,
		Source:           st.Source,
		Confidence:       st.Confidence,
		CreatedAt:        st.CreatedAt,
		UpdatedAt:        st.UpdatedAt,
	}
}
