// Package adapters — the external-organizations import store (D-ExternalOrgs, M30). Raw pgx (no sqlc):
// the upsert resolves the kind catalog + the country registry inline and keys idempotency on the
// Wikidata id, mirroring the geo-countries store but writing the M30 external_organizations table.
package adapters

import (
	"context"

	"github.com/jackc/pgx/v5"
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

// ResolveKind returns the external_org_kinds id for a code (found=false when the code is unknown).
func (r *ExternalOrgRepo) ResolveKind(ctx context.Context, code string) (string, bool, error) {
	var id string
	err := r.c.QueryRow(ctx, `SELECT id FROM oikumenea.external_org_kinds WHERE code = $1 AND deleted_at IS NULL`, code).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

// GetByWikidata returns the current name of the live org with this Wikidata id (found=false if absent).
func (r *ExternalOrgRepo) GetByWikidata(ctx context.Context, wikidataID string) (string, bool, error) {
	var name string
	err := r.c.QueryRow(ctx, `
		SELECT name FROM oikumenea.external_organizations
		WHERE wikidata_id = $1 AND deleted_at IS NULL`, wikidataID).Scan(&name)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return name, true, nil
}

// Insert creates a resolved, imported external org keyed by Wikidata id; the country (ISO alpha-2)
// resolves to geo_countries in SQL (NULL when unknown). Provenance lands in the attribution columns
// (source=imported + as_of=ImportedAt).
func (r *ExternalOrgRepo) Insert(ctx context.Context, kindID string, o domain.ExternalOrg, prov domain.Provenance) error {
	_, err := r.c.Exec(ctx, `
		INSERT INTO oikumenea.external_organizations
			(kind_id, name, country_id, wikidata_id, status, source, confidence, as_of)
		VALUES ($1, $2,
		        (SELECT id FROM oikumenea.geo_countries WHERE code = NULLIF($3,'')),
		        $4, 'resolved', 'imported', 'probable', $5)`,
		kindID, o.Name, o.CountryCode, o.WikidataID, importedAt(prov))
	return err
}

// UpdateImport refreshes name/kind/country + re-stamps the import attribution on an existing row.
func (r *ExternalOrgRepo) UpdateImport(ctx context.Context, kindID string, o domain.ExternalOrg, prov domain.Provenance) error {
	_, err := r.c.Exec(ctx, `
		UPDATE oikumenea.external_organizations SET
			kind_id    = $1,
			name       = $2,
			country_id = (SELECT id FROM oikumenea.geo_countries WHERE code = NULLIF($3,'')),
			source     = 'imported',
			as_of      = $5,
			updated_at = now()
		WHERE wikidata_id = $4 AND deleted_at IS NULL`,
		kindID, o.Name, o.CountryCode, o.WikidataID, importedAt(prov))
	return err
}

func importedAt(prov domain.Provenance) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: prov.ImportedAt, Valid: !prov.ImportedAt.IsZero()}
}
