// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Lay-affiliation persistence (D-ReligiousAffiliation / D-SpecialPII, M24): the affiliation-type catalog
// + the pii:special affiliation Link. The repository handles only ciphertext (StoredAffiliation); the
// application seals/opens the belief value. Crypto-erase NULLs the envelope, keeping rows as tombstones.
package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
)

// ---- affiliation types ----

func (r *Repository) ListAffiliationTypes(ctx context.Context) ([]domain.AffiliationType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, tradition_taxon_id, code, name, status, sort_order
		FROM oikumenea.religion_affiliation_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AffiliationType
	for rows.Next() {
		var a domain.AffiliationType
		var tradition pgtype.Text
		var so pgtype.Int4
		if err := rows.Scan(&a.ID, &tradition, &a.Code, &a.Name, &a.Status, &so); err != nil {
			return nil, err
		}
		a.TraditionTaxonID, a.SortOrder = textVal(tradition), intPtr(so)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertAffiliationType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.AffiliationType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_affiliation_types (tradition_taxon_id, code, name, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, code, name, status, sort_order`, traditionTaxonID, code, name, sortOrder)
	var a domain.AffiliationType
	var tradition pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&a.ID, &tradition, &a.Code, &a.Name, &a.Status, &so); err != nil {
		return domain.AffiliationType{}, mapPGError(err)
	}
	a.TraditionTaxonID, a.SortOrder = textVal(tradition), intPtr(so)
	return a, nil
}

// ---- affiliations (envelope-encrypted) ----

const affiliationCols = `a.id, a.person_id, COALESCE(a.religion_id::text,''), COALESCE(a.tradition_unit_id::text,''),
	COALESCE(a.community_unit_id::text,''), a.affiliation_type_id, at.code, at.name,
	a.value_ciphertext, a.wrapped_dek, COALESCE(a.key_ref,''), a.value_blind_index,
	a.status, a.effective_from, a.effective_to, COALESCE(a.source,''), COALESCE(a.confidence,''),
	a.created_at, a.updated_at`

func scanAffiliation(row pgx.Row) (domain.StoredAffiliation, error) {
	var a domain.StoredAffiliation
	var effectiveTo pgtype.Timestamptz
	if err := row.Scan(&a.ID, &a.PersonID, &a.ReligionID, &a.TraditionUnitID, &a.CommunityUnitID,
		&a.AffiliationTypeID, &a.AffiliationTypeCode, &a.AffiliationTypeName,
		&a.ValueCiphertext, &a.WrappedDEK, &a.KeyRef, &a.ValueBlindIndex,
		&a.Status, &a.EffectiveFrom, &effectiveTo, &a.Source, &a.Confidence,
		&a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.StoredAffiliation{}, mapPGError(err)
	}
	a.EffectiveTo = timePtr(effectiveTo)
	return a, nil
}

func (r *Repository) ListAffiliationsByPerson(ctx context.Context, personID string) ([]domain.StoredAffiliation, error) {
	rows, err := r.c.Query(ctx, `SELECT `+affiliationCols+`
		FROM oikumenea.religion_affiliations a
		JOIN oikumenea.religion_affiliation_types at ON at.id = a.affiliation_type_id
		WHERE a.person_id = $1 AND a.deleted_at IS NULL
		ORDER BY a.effective_from DESC, a.id`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StoredAffiliation
	for rows.Next() {
		a, err := scanAffiliation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) GetAffiliation(ctx context.Context, id string) (domain.StoredAffiliation, error) {
	row := r.c.QueryRow(ctx, `SELECT `+affiliationCols+`
		FROM oikumenea.religion_affiliations a
		JOIN oikumenea.religion_affiliation_types at ON at.id = a.affiliation_type_id
		WHERE a.id = $1 AND a.deleted_at IS NULL`, id)
	a, err := scanAffiliation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoredAffiliation{}, domain.ErrAffiliationNotFound
	}
	return a, err
}

func (r *Repository) InsertAffiliation(ctx context.Context, in domain.AffiliationInput) (domain.StoredAffiliation, error) {
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_affiliations
			(person_id, religion_id, tradition_unit_id, community_unit_id, affiliation_type_id,
			 value_ciphertext, wrapped_dek, key_ref, value_blind_index, source, confidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		in.PersonID, nilIfEmpty(in.ReligionID), nilIfEmpty(in.TraditionUnitID), nilIfEmpty(in.CommunityUnitID),
		in.AffiliationTypeID, in.ValueCiphertext, in.WrappedDEK, nilIfEmptyBytes(in.KeyRef), in.ValueBlindIndex,
		nilIfEmpty(in.Source), nilIfEmpty(in.Confidence)).Scan(&id)
	if err != nil {
		return domain.StoredAffiliation{}, mapPGError(err)
	}
	return r.GetAffiliation(ctx, id)
}

func (r *Repository) UpdateAffiliation(ctx context.Context, id string, up domain.AffiliationUpdate) (domain.StoredAffiliation, error) {
	var affected int64
	if up.ValueProvided {
		tag, err := r.c.Exec(ctx, `
			UPDATE oikumenea.religion_affiliations SET
				status = COALESCE($2, status),
				value_ciphertext = $3, wrapped_dek = $4, key_ref = $5, value_blind_index = $6
			WHERE id = $1 AND deleted_at IS NULL`,
			id, up.Status, up.ValueCiphertext, up.WrappedDEK, nilIfEmptyBytes(up.KeyRef), up.ValueBlindIndex)
		if err != nil {
			return domain.StoredAffiliation{}, mapPGError(err)
		}
		affected = tag.RowsAffected()
	} else {
		tag, err := r.c.Exec(ctx, `
			UPDATE oikumenea.religion_affiliations SET status = COALESCE($2, status)
			WHERE id = $1 AND deleted_at IS NULL`, id, up.Status)
		if err != nil {
			return domain.StoredAffiliation{}, mapPGError(err)
		}
		affected = tag.RowsAffected()
	}
	if affected == 0 {
		return domain.StoredAffiliation{}, domain.ErrAffiliationNotFound
	}
	return r.GetAffiliation(ctx, id)
}

func (r *Repository) SoftDeleteAffiliation(ctx context.Context, id string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_affiliations SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrAffiliationNotFound
	}
	return nil
}

// CryptoEraseAffiliations drops the envelope (ciphertext + wrapped DEK + blind index) for all of a
// person's affiliations, keeping the rows as tombstones (the person-purge erasure path).
func (r *Repository) CryptoEraseAffiliations(ctx context.Context, personID string) (int64, error) {
	ct, err := r.c.Exec(ctx, `
		UPDATE oikumenea.religion_affiliations SET
			value_ciphertext = NULL, wrapped_dek = NULL, key_ref = NULL, value_blind_index = NULL
		WHERE person_id = $1 AND value_ciphertext IS NOT NULL`, personID)
	if err != nil {
		return 0, mapPGError(err)
	}
	return ct.RowsAffected(), nil
}

// nilIfEmptyBytes returns nil for an empty key_ref so a NULL is stored (a crypto-erased / value-less row).
func nilIfEmptyBytes(s string) any {
	if s == "" {
		return nil
	}
	return s
}
