// Physical-identity orchestration (D-PhysicalIdentity, M31): name aliases, physical descriptions,
// distinguishing marks and the self-declared, envelope-encrypted ethnicity. All writes record an audit
// row in the same transaction (D-Audit); the audit payloads carry only non-PII identifiers. The declared
// ethnicity is sealed (Seal) on write, decrypted (Open) on read, and crypto-erased on purge.
package application

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
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

// ---------------------------------------------------------------- physical descriptions

// UpsertPhysicalDescription adds an effective-dated physical description (or replaces one when d.ID set).
func (s *Service) UpsertPhysicalDescription(ctx context.Context, d domain.PhysicalDescription) (domain.PhysicalDescription, error) {
	if err := d.Validate(); err != nil {
		return domain.PhysicalDescription{}, err
	}
	if err := s.checkColor(ctx, d.EyeColorID, "eye"); err != nil {
		return domain.PhysicalDescription{}, err
	}
	if err := s.checkColor(ctx, d.HairColorID, "hair"); err != nil {
		return domain.PhysicalDescription{}, err
	}
	var out domain.PhysicalDescription
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, d.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertPhysicalDescription(ctx, d)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.physical_description.upsert", d.PersonID,
			map[string]any{"id": d.PersonID, "descriptionId": created.ID})
	})
	return out, err
}

// DeletePhysicalDescription soft-deletes a physical description.
func (s *Service) DeletePhysicalDescription(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeletePhysicalDescription(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.physical_description.delete", personID, map[string]any{"id": personID, "descriptionId": id})
	})
}

// ListPhysicalDescriptions lists a person's physical descriptions (the person must exist).
func (s *Service) ListPhysicalDescriptions(ctx context.Context, personID string) ([]domain.PhysicalDescription, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListPhysicalDescriptions(ctx, personID)
}

// ---------------------------------------------------------------- distinguishing marks

// UpsertDistinguishingMark adds a distinguishing mark (or replaces one when m.ID is set).
func (s *Service) UpsertDistinguishingMark(ctx context.Context, m domain.DistinguishingMark) (domain.DistinguishingMark, error) {
	if err := m.Validate(); err != nil {
		return domain.DistinguishingMark{}, err
	}
	var out domain.DistinguishingMark
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, m.PersonID); err != nil {
			return err
		}
		created, err := repo.UpsertDistinguishingMark(ctx, m)
		if err != nil {
			return err
		}
		out = created
		return s.record(ctx, tx, "person.distinguishing_mark.upsert", m.PersonID,
			map[string]any{"id": m.PersonID, "markId": created.ID, "kind": created.Kind})
	})
	return out, err
}

// DeleteDistinguishingMark soft-deletes a distinguishing mark.
func (s *Service) DeleteDistinguishingMark(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteDistinguishingMark(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.distinguishing_mark.delete", personID, map[string]any{"id": personID, "markId": id})
	})
}

// ListDistinguishingMarks lists a person's distinguishing marks (the person must exist).
func (s *Service) ListDistinguishingMarks(ctx context.Context, personID string) ([]domain.DistinguishingMark, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	return repo.ListDistinguishingMarks(ctx, personID)
}

// ---------------------------------------------------------------- ethnicity-type catalog

// ListEthnicityTypes returns the declared-ethnicity vocabulary, filtered (roots / children / search).
func (s *Service) ListEthnicityTypes(ctx context.Context, f domain.EthnicityTypeFilter) ([]domain.EthnicityType, error) {
	return s.newRepo(s.pool).ListEthnicityTypes(ctx, f)
}

// GetEthnicityType returns one ethnicity type (by RID) plus its group-level associated-language and
// homeland-country RIDs (D-PhysicalIdentity amendment, M43).
func (s *Service) GetEthnicityType(ctx context.Context, id string) (domain.EthnicityType, []string, []string, error) {
	repo := s.newRepo(s.pool)
	t, err := repo.GetEthnicityTypeByID(ctx, id)
	if err != nil {
		return domain.EthnicityType{}, nil, nil, err
	}
	langs, err := repo.ListEthnicityTypeLanguages(ctx, id)
	if err != nil {
		return domain.EthnicityType{}, nil, nil, err
	}
	countries, err := repo.ListEthnicityTypeCountries(ctx, id)
	if err != nil {
		return domain.EthnicityType{}, nil, nil, err
	}
	return t, langs, countries, nil
}

// UpsertEthnicityType adds or updates a declared-ethnicity catalog entry (instance-admin managed).
func (s *Service) UpsertEthnicityType(ctx context.Context, t domain.EthnicityType) (domain.EthnicityType, error) {
	if t.Code == "" || t.Name == "" {
		return domain.EthnicityType{}, domain.ErrInvalid
	}
	var out domain.EthnicityType
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		v, err := s.newRepo(tx).UpsertEthnicityType(ctx, t)
		if err != nil {
			return err
		}
		out = v
		return s.record(ctx, tx, "person.ethnicity_type.upsert", v.ID, map[string]any{"id": v.ID, "code": v.Code})
	})
	return out, err
}

// ---------------------------------------------------------------- ethnicities (pii:special)

// ListEthnicities returns a person's declared ethnicities with the value DECRYPTED.
func (s *Service) ListEthnicities(ctx context.Context, personID string) ([]domain.Ethnicity, error) {
	repo := s.newRepo(s.pool)
	if _, err := repo.GetPerson(ctx, personID); err != nil {
		return nil, err
	}
	stored, err := repo.ListEthnicities(ctx, personID)
	if err != nil {
		return nil, err
	}
	names, err := s.ethnicityNamesByCode(ctx, repo)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Ethnicity, 0, len(stored))
	for _, st := range stored {
		e, err := s.toEthnicity(ctx, st, names)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// AddEthnicity validates the declared code against the catalog, seals it, and stores the link.
func (s *Service) AddEthnicity(ctx context.Context, personID, code, legalBasis string, source, confidence string) (domain.Ethnicity, error) {
	if code == "" || legalBasis == "" {
		return domain.Ethnicity{}, domain.ErrInvalid
	}
	sealed, blind, err := s.seal(ctx, code)
	if err != nil {
		return domain.Ethnicity{}, err
	}
	rec := domain.StoredEthnicity{
		PersonID:        personID,
		ValueCiphertext: sealed.Ciphertext,
		WrappedDEK:      sealed.WrappedDEK,
		KeyRef:          sealed.KeyRef,
		ValueBlindIndex: blind,
		LegalBasis:      legalBasis,
		Source:          source,
		Confidence:      confidence,
	}
	var out domain.Ethnicity
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		if _, err := repo.GetPerson(ctx, personID); err != nil {
			return err
		}
		t, err := repo.GetEthnicityTypeByCode(ctx, code) // ErrUnknownEthnicityType when not in the catalog
		if err != nil {
			return err
		}
		st, err := repo.InsertEthnicity(ctx, rec)
		if err != nil {
			return err
		}
		out = storedToEthnicity(st, code, t.Name)
		return s.record(ctx, tx, "person.ethnicity.add", personID,
			map[string]any{"id": personID, "ethnicityId": st.ID, "legalBasis": legalBasis})
	})
	return out, err
}

// UpdateEthnicity re-seals a new declared code and/or flips legal basis / status.
func (s *Service) UpdateEthnicity(ctx context.Context, personID, id, code, legalBasis, status string) (domain.Ethnicity, error) {
	if code == "" || legalBasis == "" {
		return domain.Ethnicity{}, domain.ErrInvalid
	}
	if status == "" {
		status = "active"
	}
	sealed, blind, err := s.seal(ctx, code)
	if err != nil {
		return domain.Ethnicity{}, err
	}
	rec := domain.StoredEthnicity{
		ID:              id,
		PersonID:        personID,
		ValueCiphertext: sealed.Ciphertext,
		WrappedDEK:      sealed.WrappedDEK,
		KeyRef:          sealed.KeyRef,
		ValueBlindIndex: blind,
		LegalBasis:      legalBasis,
		Status:          status,
	}
	var out domain.Ethnicity
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		repo := s.newRepo(tx)
		t, err := repo.GetEthnicityTypeByCode(ctx, code)
		if err != nil {
			return err
		}
		st, err := repo.UpdateEthnicity(ctx, rec)
		if err != nil {
			return err
		}
		out = storedToEthnicity(st, code, t.Name)
		return s.record(ctx, tx, "person.ethnicity.update", personID, map[string]any{"id": personID, "ethnicityId": id})
	})
	return out, err
}

// DeleteEthnicity soft-deletes a declared ethnicity.
func (s *Service) DeleteEthnicity(ctx context.Context, personID, id string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		if err := s.newRepo(tx).DeleteEthnicity(ctx, personID, id); err != nil {
			return err
		}
		return s.record(ctx, tx, "person.ethnicity.delete", personID, map[string]any{"id": personID, "ethnicityId": id})
	})
}

// ErasePersonEthnicities crypto-erases all of a person's ethnicities (drop the envelope, keep the row
// tombstone). The person-purge erasure path for the declared value (also driven by Purge); exposed so it
// can be exercised directly (shared open seam with the religion/document special-PII stores).
func (s *Service) ErasePersonEthnicities(ctx context.Context, personID string) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		c, err := s.newRepo(tx).CryptoEraseEthnicities(ctx, personID)
		if err != nil {
			return err
		}
		n = c
		return s.record(ctx, tx, "person.ethnicity.erase", personID, map[string]any{"id": personID, "erased": c})
	})
	return n, err
}

// ---- helpers ----

func (s *Service) seal(ctx context.Context, value string) (crypto.Sealed, []byte, error) {
	sealed, err := s.cipher.Seal(ctx, []byte(value))
	if err != nil {
		return crypto.Sealed{}, nil, err
	}
	return sealed, s.cipher.BlindIndex([]byte(value)), nil
}

// ethnicityNamesByCode builds a code->name map from the catalog so list reads can resolve display names.
func (s *Service) ethnicityNamesByCode(ctx context.Context, repo domain.Repository) (map[string]string, error) {
	types, err := repo.ListEthnicityTypes(ctx, domain.EthnicityTypeFilter{})
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(types))
	for _, t := range types {
		m[t.Code] = t.Name
	}
	return m, nil
}

// toEthnicity decrypts the declared value (a crypto-erased tombstone yields code "").
func (s *Service) toEthnicity(ctx context.Context, st domain.StoredEthnicity, names map[string]string) (domain.Ethnicity, error) {
	code := ""
	if len(st.ValueCiphertext) > 0 && len(st.WrappedDEK) > 0 {
		plain, err := s.cipher.Open(ctx, crypto.Sealed{Ciphertext: st.ValueCiphertext, WrappedDEK: st.WrappedDEK, KeyRef: st.KeyRef})
		if err != nil {
			return domain.Ethnicity{}, err
		}
		code = string(plain)
	}
	return storedToEthnicity(st, code, names[code]), nil
}

func storedToEthnicity(st domain.StoredEthnicity, code, name string) domain.Ethnicity {
	return domain.Ethnicity{
		ID:         st.ID,
		PersonID:   st.PersonID,
		Code:       code,
		Name:       name,
		LegalBasis: st.LegalBasis,
		Status:     st.Status,
		Source:     st.Source,
		Confidence: st.Confidence,
		CreatedAt:  st.CreatedAt,
		UpdatedAt:  st.UpdatedAt,
	}
}
