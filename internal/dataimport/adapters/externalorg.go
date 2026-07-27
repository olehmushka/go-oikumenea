// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters — the external-organizations import store (D-ExternalOrgs, M30; set-based per
// chunk since R-05). Raw pgx (no sqlc): one parallel-array merge statement resolves the kind catalog
// + the country registry inline and keys idempotency on the Wikidata id, mirroring the geo-places
// merge but writing the M30 external_organizations table.
package adapters

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// ExternalOrgRepo is the raw-pgx implementation of domain.ExternalOrgStore, bound to a single db.DBTX
// (the pool for reads, or a caller-supplied transaction so the upsert and its audit row commit together).
type ExternalOrgRepo struct{ c db.DBTX }

// NewExternalOrgRepo binds a store to the given command surface.
func NewExternalOrgRepo(conn db.DBTX) *ExternalOrgRepo { return &ExternalOrgRepo{c: conn} }

var _ domain.ExternalOrgStore = (*ExternalOrgRepo)(nil)

// BulkUpsert merges one chunk set-based (R-05): kinds resolve inline (a record whose kind is unknown
// drops out of the join — skipped), the country (ISO alpha-2) resolves to geo_countries (NULL when
// unknown), idempotency keys on the Wikidata id, and an existing row updates only when the name
// changed (the conflict-update WHERE gate) — never deletes. Provenance lands in the attribution
// columns (source=imported + as_of=ImportedAt). RETURNING (xmax = 0) splits creates from updates;
// unmerged rows (unknown kind / unchanged name) are the caller's skips.
func (r *ExternalOrgRepo) BulkUpsert(ctx context.Context, orgs []domain.ExternalOrg, prov domain.Provenance) (created, updated int, err error) {
	if len(orgs) == 0 {
		return 0, 0, nil
	}
	n := len(orgs)
	wikidataIDs := make([]string, 0, n)
	names := make([]string, 0, n)
	kindCodes := make([]string, 0, n)
	countryCodes := make([]string, 0, n)
	for _, o := range orgs {
		wikidataIDs = append(wikidataIDs, o.WikidataID)
		names = append(names, o.Name)
		kindCodes = append(kindCodes, o.KindCode)
		countryCodes = append(countryCodes, o.CountryCode)
	}
	rows, err := r.c.Query(ctx, `
		WITH r AS (
		  SELECT unnest($1::text[]) AS wikidata_id,
		         unnest($2::text[]) AS name,
		         unnest($3::text[]) AS kind_code,
		         unnest($4::text[]) AS country_code
		)
		INSERT INTO oikumenea.external_organizations
			(kind_id, name, country_id, wikidata_id, status, source, confidence, as_of)
		SELECT k.id, r.name,
		       (SELECT c.id FROM oikumenea.geo_countries c WHERE c.code = NULLIF(r.country_code, '')),
		       r.wikidata_id, 'resolved', 'imported', 'probable', $5
		FROM r
		JOIN oikumenea.external_org_kinds k ON k.code = r.kind_code AND k.deleted_at IS NULL
		ON CONFLICT (wikidata_id) WHERE deleted_at IS NULL AND wikidata_id IS NOT NULL
		DO UPDATE SET
			kind_id    = EXCLUDED.kind_id,
			name       = EXCLUDED.name,
			country_id = EXCLUDED.country_id,
			source     = 'imported',
			as_of      = EXCLUDED.as_of,
			updated_at = now()
		WHERE oikumenea.external_organizations.name IS DISTINCT FROM EXCLUDED.name
		RETURNING (xmax = 0) AS inserted`,
		wikidataIDs, names, kindCodes, countryCodes, importedAt(prov))
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var inserted bool
		if err := rows.Scan(&inserted); err != nil {
			return 0, 0, err
		}
		if inserted {
			created++
		} else {
			updated++
		}
	}
	return created, updated, rows.Err()
}

func importedAt(prov domain.Provenance) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: prov.ImportedAt, Valid: !prov.ImportedAt.IsZero()}
}
