// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// EthnicityRepo is the pgx/sqlc-backed implementation of domain.EthnicityStore (D-PhysicalIdentity
// amendment, M43), bound to a single db.DBTX (the caller's transaction so the upsert + audit row commit
// together — D-Audit). Mirrors LanguoidRepo.
type EthnicityRepo struct {
	q *dataimportsql.Queries
}

// NewEthnicityRepo binds an ethnicity store to the given command surface.
func NewEthnicityRepo(conn db.DBTX) *EthnicityRepo {
	return &EthnicityRepo{q: dataimportsql.New(conn)}
}

var _ domain.EthnicityStore = (*EthnicityRepo)(nil)

// GetVersion returns the ethnicity type's stored source_version (idempotency key) and whether it exists.
func (r *EthnicityRepo) GetVersion(ctx context.Context, code string) (string, bool, error) {
	v, err := r.q.GetEthnicityVersion(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, true, nil
}

// Insert adds an ethnicity type; parent code is resolved to its RID in SQL, provenance stamped.
func (r *EthnicityRepo) Insert(ctx context.Context, e domain.Ethnicity, prov domain.Provenance) error {
	return r.q.InsertEthnicityImport(ctx, dataimportsql.InsertEthnicityImportParams{
		Code:          e.Code,
		Name:          e.Name,
		ParentCode:    e.Parent,
		WikidataID:    e.WikidataID,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport rewrites an ethnicity type (called when the source edition changed).
func (r *EthnicityRepo) UpdateImport(ctx context.Context, e domain.Ethnicity, prov domain.Provenance) error {
	return r.q.UpdateEthnicityImport(ctx, dataimportsql.UpdateEthnicityImportParams{
		Code:          e.Code,
		Name:          e.Name,
		ParentCode:    e.Parent,
		WikidataID:    e.WikidataID,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// ReplaceLanguages resets a group's associated-language ties (each key is a glottocode or ISO-639-3;
// unresolved keys are silently dropped by the insert).
func (r *EthnicityRepo) ReplaceLanguages(ctx context.Context, code string, languageKeys []string) error {
	if err := r.q.DeleteEthnicityLanguages(ctx, code); err != nil {
		return err
	}
	for _, k := range languageKeys {
		if k == "" {
			continue
		}
		if err := r.q.InsertEthnicityLanguage(ctx, dataimportsql.InsertEthnicityLanguageParams{Code: code, LanguageKey: k}); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceCountries resets a group's homeland ties to the given ISO alpha-2 codes (unresolved dropped).
func (r *EthnicityRepo) ReplaceCountries(ctx context.Context, code string, countryCodes []string) error {
	if err := r.q.DeleteEthnicityCountries(ctx, code); err != nil {
		return err
	}
	for _, cc := range countryCodes {
		if cc == "" {
			continue
		}
		if err := r.q.InsertEthnicityCountry(ctx, dataimportsql.InsertEthnicityCountryParams{Code: code, CountryCode: cc}); err != nil {
			return err
		}
	}
	return nil
}

// RebuildClosure recomputes the transitive closure (run once at the end of an ethnicity-scheme import).
func (r *EthnicityRepo) RebuildClosure(ctx context.Context) error {
	if err := r.q.ClearEthnicityClosure(ctx); err != nil {
		return err
	}
	return r.q.RebuildEthnicityClosure(ctx)
}
