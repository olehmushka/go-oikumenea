// Lay-affiliation orchestration (D-ReligiousAffiliation / D-SpecialPII, M24): audited writes with the
// belief value envelope-encrypted (Seal) on write and decrypted (Open) on read; crypto-erased on purge.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
)

// ---- affiliation types ----

func (s *Service) ListAffiliationTypes(ctx context.Context) ([]domain.AffiliationType, error) {
	return s.newRepo(s.querier(ctx)).ListAffiliationTypes(ctx)
}

func (s *Service) UpsertAffiliationType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.AffiliationType, error) {
	var out domain.AffiliationType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertAffiliationType(ctx, traditionTaxonID, code, name, sortOrder)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "religion.affiliation-type.upsert", v.ID, "", v)
	})
	return out, err
}

// ---- affiliations (pii:special) ----

// ListPersonAffiliations returns a person's affiliations with the belief value DECRYPTED.
func (s *Service) ListPersonAffiliations(ctx context.Context, personID string) ([]domain.Affiliation, error) {
	stored, err := s.newRepo(s.querier(ctx)).ListAffiliationsByPerson(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Affiliation, 0, len(stored))
	for _, st := range stored {
		a, err := s.toAffiliation(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// AddAffiliation seals the optional belief value and stores the affiliation, recording the action.
func (s *Service) AddAffiliation(ctx context.Context, in domain.AffiliationInput, value string) (domain.Affiliation, error) {
	if err := in.Validate(); err != nil {
		return domain.Affiliation{}, err
	}
	if value != "" {
		sealed, blind, err := s.seal(ctx, value)
		if err != nil {
			return domain.Affiliation{}, err
		}
		in.ValueCiphertext, in.WrappedDEK, in.KeyRef, in.ValueBlindIndex = sealed.Ciphertext, sealed.WrappedDEK, sealed.KeyRef, blind
	}
	var out domain.Affiliation
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		st, err := s.newRepo(tx).InsertAffiliation(ctx, in)
		if err != nil {
			return err
		}
		// the just-written plaintext is returned without a re-decrypt round-trip
		out = storedToAffiliation(st, value)
		return s.record(ctx, tx, "religion.affiliation.add", st.ID, "",
			map[string]any{"id": st.ID, "personId": st.PersonID, "affiliationTypeId": st.AffiliationTypeID})
	})
	return out, err
}

// UpdateAffiliation flips status and/or re-seals a new belief value.
func (s *Service) UpdateAffiliation(ctx context.Context, id string, status *string, value *string) (domain.Affiliation, error) {
	if status != nil && !domain.ValidAffiliationStatus(*status) {
		return domain.Affiliation{}, domain.ErrInvalid
	}
	up := domain.AffiliationUpdate{Status: status}
	plain := ""
	if value != nil {
		up.ValueProvided = true
		if *value != "" {
			sealed, blind, err := s.seal(ctx, *value)
			if err != nil {
				return domain.Affiliation{}, err
			}
			up.ValueCiphertext, up.WrappedDEK, up.KeyRef, up.ValueBlindIndex = sealed.Ciphertext, sealed.WrappedDEK, sealed.KeyRef, blind
		}
		plain = *value
	}
	var out domain.Affiliation
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		st, err := s.newRepo(tx).UpdateAffiliation(ctx, id, up)
		if err != nil {
			return err
		}
		if value != nil {
			out = storedToAffiliation(st, plain)
		} else {
			a, derr := s.toAffiliation(ctx, st)
			if derr != nil {
				return derr
			}
			out = a
		}
		return s.record(ctx, tx, "religion.affiliation.update", id, "",
			map[string]any{"id": id, "valueChanged": value != nil})
	})
	return out, err
}

func (s *Service) DeleteAffiliation(ctx context.Context, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).SoftDeleteAffiliation(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "religion.affiliation.delete", id, "", map[string]any{"id": id, "deleted": true})
	})
}

// ErasePersonAffiliations crypto-erases all of a person's affiliations (drop the wrapped DEK +
// ciphertext), keeping rows as tombstones. This is the person-purge erasure path (D-ReligiousAffiliation);
// triggered by the PersonPurged event (SubscribePersonPurge) and also exercised directly.
func (s *Service) ErasePersonAffiliations(ctx context.Context, personID string) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := s.erasePersonAffiliationsTx(ctx, tx, personID)
		n = c
		return err
	})
	return n, err
}

// erasePersonAffiliationsTx is the body of the person-purge erasure, run in a caller-supplied transaction
// so it executes either standalone (ErasePersonAffiliations) or inside the person-purge tx as the
// PersonPurged subscriber (SubscribePersonPurge). The audit row is written only when something was erased.
func (s *Service) erasePersonAffiliationsTx(ctx context.Context, tx pgx.Tx, personID string) (int64, error) {
	c, err := s.newRepo(tx).CryptoEraseAffiliations(ctx, personID)
	if err != nil {
		return 0, err
	}
	if c > 0 {
		if err := s.record(ctx, tx, "religion.affiliation.erase", personID, "", map[string]any{"personId": personID, "erased": c}); err != nil {
			return 0, err
		}
	}
	return c, nil
}

// ---- crypto helpers ----

func (s *Service) seal(ctx context.Context, value string) (crypto.Sealed, []byte, error) {
	sealed, err := s.cipher.Seal(ctx, []byte(value))
	if err != nil {
		return crypto.Sealed{}, nil, err
	}
	return sealed, s.cipher.BlindIndex([]byte(value)), nil
}

// toAffiliation maps a stored row to the application shape, decrypting the value (a crypto-erased row
// yields "" — the tombstone).
func (s *Service) toAffiliation(ctx context.Context, st domain.StoredAffiliation) (domain.Affiliation, error) {
	value := ""
	if len(st.ValueCiphertext) > 0 && len(st.WrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.ValueCiphertext, WrappedDEK: st.WrappedDEK, KeyRef: st.KeyRef})
		if err != nil {
			return domain.Affiliation{}, err
		}
		value = string(plain)
	}
	return storedToAffiliation(st, value), nil
}

func storedToAffiliation(st domain.StoredAffiliation, value string) domain.Affiliation {
	return domain.Affiliation{
		ID:                  st.ID,
		PersonID:            st.PersonID,
		ReligionID:          st.ReligionID,
		TraditionUnitID:     st.TraditionUnitID,
		CommunityUnitID:     st.CommunityUnitID,
		AffiliationTypeID:   st.AffiliationTypeID,
		AffiliationTypeCode: st.AffiliationTypeCode,
		AffiliationTypeName: st.AffiliationTypeName,
		Value:               value,
		Status:              st.Status,
		EffectiveFrom:       st.EffectiveFrom,
		EffectiveTo:         st.EffectiveTo,
		Source:              st.Source,
		Confidence:          st.Confidence,
		CreatedAt:           st.CreatedAt,
		UpdatedAt:           st.UpdatedAt,
	}
}
