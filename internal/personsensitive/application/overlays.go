// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Financial / behavioural / psychological overlays orchestration (D-PersonOverlays, M35). Crypto wallets
// and personality profiles are plaintext pii:sensitive; the inferred political leaning is envelope-encrypted
// (political opinion is GDPR Art. 9 — sealed on write, decrypted on read, crypto-erased on purge, exactly
// like the M33 party membership) and is kept in a SEPARATE table from the declared party membership so the
// declared and the inferred are never merged. Every write records an audit row in the same transaction
// (D-Audit) with only non-PII identifiers in the payload.
package application

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	"github.com/olehmushka/go-oikumenea/pkg/crypto"
)

// ---------------------------------------------------------------- crypto wallets (pii:sensitive)

// ListCryptoWallets returns a person's crypto-wallet attributions (the person must exist).
func (s *Service) ListCryptoWallets(ctx context.Context, personID string) ([]domain.CryptoWallet, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListCryptoWallets(ctx, personID)
}

// UpsertCryptoWallet adds a crypto wallet (or replaces one when w.ID is set).
func (s *Service) UpsertCryptoWallet(ctx context.Context, w domain.CryptoWallet) (domain.CryptoWallet, error) {
	if w.Chain == "" {
		w.Chain = "ethereum"
	}
	if w.AttributionMethod == "" {
		w.AttributionMethod = "other"
	}
	if w.Source == "" {
		w.Source = "imported"
	}
	if w.Confidence == "" {
		w.Confidence = "possible"
	}
	if err := w.Validate(); err != nil {
		return domain.CryptoWallet{}, err
	}
	var out domain.CryptoWallet
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, w.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertCryptoWallet(ctx, w)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.crypto_wallet.upsert", w.PersonID,
			map[string]any{"id": w.PersonID, "walletId": created.ID, "chain": created.Chain})
	})
	return out, err
}

// DeleteCryptoWallet soft-deletes a crypto wallet.
func (s *Service) DeleteCryptoWallet(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteCryptoWallet(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.crypto_wallet.delete", personID, map[string]any{"id": personID, "walletId": id})
	})
}

// ---------------------------------------------------------------- personality (pii:sensitive)

// ListPersonalities returns a person's declared/assessed personality profiles (the person must exist).
func (s *Service) ListPersonalities(ctx context.Context, personID string) ([]domain.Personality, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPersonalities(ctx, personID)
}

// UpsertPersonality adds a personality profile (or replaces one when p.ID is set). Declared/assessment
// only — the domain rejects any method outside {self_declared_survey, hr_assessment}.
func (s *Service) UpsertPersonality(ctx context.Context, p domain.Personality) (domain.Personality, error) {
	if p.Framework == "" {
		p.Framework = "mbti"
	}
	if p.Method == "" {
		p.Method = "self_declared_survey"
	}
	if p.Source == "" {
		p.Source = "self_declared"
	}
	if p.Confidence == "" {
		p.Confidence = "possible"
	}
	if err := p.Validate(); err != nil {
		return domain.Personality{}, err
	}
	var out domain.Personality
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, p.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertPersonality(ctx, p)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.personality.upsert", p.PersonID,
			map[string]any{"id": p.PersonID, "personalityId": created.ID, "framework": created.Framework})
	})
	return out, err
}

// DeletePersonality soft-deletes a personality profile.
func (s *Service) DeletePersonality(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeletePersonality(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.personality.delete", personID, map[string]any{"id": personID, "personalityId": id})
	})
}

// ---------------------------------------------------------------- political leaning (pii:special)

// GetPoliticalLeaning returns the person's decrypted inferred leaning, or ErrPoliticalLeaningNotFound.
func (s *Service) GetPoliticalLeaning(ctx context.Context, personID string) (domain.PoliticalLeaning, error) {
	repo := s.newRepo(s.pool)
	if err := repo.PersonExists(ctx, personID); err != nil {
		return domain.PoliticalLeaning{}, err
	}
	st, err := repo.GetPoliticalLeaning(ctx, personID)
	if err != nil {
		return domain.PoliticalLeaning{}, err
	}
	return s.openLeaning(ctx, st)
}

// SetPoliticalLeaning seals the inferred spectrum and stores it as the single active row (insert when
// none exists, otherwise re-seal in place). legal_basis is required (Art. 9). This is inference-only —
// it never touches the declared M33 party membership.
func (s *Service) SetPoliticalLeaning(ctx context.Context, l domain.PoliticalLeaning) (domain.PoliticalLeaning, error) {
	if l.Confidence == "" {
		l.Confidence = "possible"
	}
	if err := l.Validate(); err != nil {
		return domain.PoliticalLeaning{}, err
	}
	// Seal the spectrum as its canonical decimal string.
	sealed, blind, err := s.seal(ctx, strconv.FormatFloat(l.Spectrum, 'f', -1, 64))
	if err != nil {
		return domain.PoliticalLeaning{}, err
	}
	rec := domain.StoredPoliticalLeaning{
		PersonID:          l.PersonID,
		LeaningCiphertext: sealed.Ciphertext,
		LeaningWrappedDEK: sealed.WrappedDEK,
		LeaningKeyRef:     sealed.KeyRef,
		LeaningBlindIndex: blind,
		InferenceSources:  l.InferenceSources,
		AssessedAt:        l.AssessedAt,
		LegalBasis:        l.LegalBasis,
		Confidence:        l.Confidence,
	}
	var out domain.PoliticalLeaning
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if err := repo.PersonExists(ctx, l.PersonID); err != nil {
			return err
		}
		st, err := repo.UpdatePoliticalLeaning(ctx, rec)
		if err != nil {
			if err == domain.ErrPoliticalLeaningNotFound {
				st, err = repo.InsertPoliticalLeaning(ctx, rec)
			}
			if err != nil {
				return err
			}
		}
		out = storedToLeaning(st, l.Spectrum)
		return s.record(ctx, tx, "person.political_leaning.set", l.PersonID,
			map[string]any{"id": l.PersonID, "leaningId": st.ID, "legalBasis": l.LegalBasis})
	})
	return out, err
}

// DeletePoliticalLeaning soft-deletes the person's inferred leaning.
func (s *Service) DeletePoliticalLeaning(ctx context.Context, personID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeletePoliticalLeaning(ctx, personID); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.political_leaning.delete", personID, map[string]any{"id": personID})
	})
}

// ---- helpers ----

// openLeaning decrypts the sealed spectrum (a crypto-erased tombstone yields spectrum 0).
func (s *Service) openLeaning(ctx context.Context, st domain.StoredPoliticalLeaning) (domain.PoliticalLeaning, error) {
	var spectrum float64
	if len(st.LeaningCiphertext) > 0 && len(st.LeaningWrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.LeaningCiphertext, WrappedDEK: st.LeaningWrappedDEK, KeyRef: st.LeaningKeyRef})
		if err != nil {
			return domain.PoliticalLeaning{}, err
		}
		if f, perr := strconv.ParseFloat(string(plain), 64); perr == nil {
			spectrum = f
		}
	}
	return storedToLeaning(st, spectrum), nil
}

func storedToLeaning(st domain.StoredPoliticalLeaning, spectrum float64) domain.PoliticalLeaning {
	sources := st.InferenceSources
	if sources == nil {
		sources = []string{}
	}
	return domain.PoliticalLeaning{
		ID:               st.ID,
		PersonID:         st.PersonID,
		Spectrum:         spectrum,
		InferenceSources: sources,
		AssessedAt:       st.AssessedAt,
		LegalBasis:       st.LegalBasis,
		Confidence:       st.Confidence,
		CreatedAt:        st.CreatedAt,
		UpdatedAt:        st.UpdatedAt,
	}
}
