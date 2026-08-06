// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters is the external-organizations module's pgx-backed persistence adapter (M30,
// D-ExternalOrgs). It uses raw pgx over a single command surface (the pool for reads, a tx for writes) —
// the vehicle/religion raw-SQL style — because of the cross-module country label lookups and the
// kind-code-filtered listing. Postgres constraint violations (23505 unique / 23503 FK) map to domain
// sentinels.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/externalorg/domain"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// Repository is the external-organizations persistence adapter bound to one command surface (pool or tx).
type Repository struct{ c db.DBTX }

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository { return &Repository{c: conn} }

var _ domain.Repository = (*Repository)(nil)

// ---- small scan/param helpers ----

func textVal(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func intPtr(i pgtype.Int4) *int {
	if i.Valid {
		v := int(i.Int32)
		return &v
	}
	return nil
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		v := t.Time
		return &v
	}
	return nil
}

func mapPGError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return err // callers translate to their NotFound sentinel
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrConflict
		case "23503", "23502":
			return domain.ErrInvalid
		}
	}
	return err
}

// ============================ kind catalog ============================

func (r *Repository) ListKinds(ctx context.Context) ([]domain.Kind, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, status, sort_order
		FROM oikumenea.external_org_kinds WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Kind
	for rows.Next() {
		k, err := scanKindRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *Repository) GetKind(ctx context.Context, id string) (domain.Kind, error) {
	row := r.c.QueryRow(ctx, `
		SELECT id, code, name, status, sort_order
		FROM oikumenea.external_org_kinds WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanKind(row)
}

func (r *Repository) UpsertKind(ctx context.Context, code, name string, sortOrder *int) (domain.Kind, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.external_org_kinds (code, name, sort_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, status, sort_order`, code, name, sortOrder)
	return scanKind(row)
}

func scanKind(row pgx.Row) (domain.Kind, error) {
	var k domain.Kind
	var so pgtype.Int4
	if err := row.Scan(&k.ID, &k.Code, &k.Name, &k.Status, &so); err != nil {
		return domain.Kind{}, mapPGError(err)
	}
	k.SortOrder = intPtr(so)
	return k, nil
}

func scanKindRows(rows pgx.Rows) (domain.Kind, error) {
	var k domain.Kind
	var so pgtype.Int4
	if err := rows.Scan(&k.ID, &k.Code, &k.Name, &k.Status, &so); err != nil {
		return domain.Kind{}, err
	}
	k.SortOrder = intPtr(so)
	return k, nil
}

// ============================ organizations ============================

const orgCols = `id, kind_id, name, code, country_id, wikidata_id, status, source, confidence, as_of, created_at, updated_at`

func (r *Repository) InsertOrg(ctx context.Context, in domain.OrgInput) (domain.Organization, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.external_organizations
			(kind_id, name, code, country_id, wikidata_id, status, source, confidence, as_of)
		VALUES ($1, $2, NULLIF($3,''), NULLIF($4,'')::uuid, NULLIF($5,''),
		        COALESCE(NULLIF($6,''),'resolved'),
		        COALESCE(NULLIF($7,''),'operator_verified'),
		        COALESCE(NULLIF($8,''),'possible'), $9)
		RETURNING `+orgCols,
		in.KindID, strings.TrimSpace(in.Name), in.Code, in.CountryID, in.WikidataID,
		in.Status, in.Source, in.Confidence, in.AsOf)
	return scanOrg(row)
}

func (r *Repository) GetOrg(ctx context.Context, id string) (domain.Organization, error) {
	row := r.c.QueryRow(ctx, `SELECT `+orgCols+`
		FROM oikumenea.external_organizations WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanOrg(row)
}

func (r *Repository) UpdateOrg(ctx context.Context, id string, up domain.OrgUpdate) (domain.Organization, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.external_organizations SET
			kind_id     = COALESCE($2::uuid, kind_id),
			name        = COALESCE($3, name),
			code        = CASE WHEN $4::boolean THEN NULLIF($5,'') ELSE code END,
			country_id  = CASE WHEN $6::boolean THEN NULLIF($7,'')::uuid ELSE country_id END,
			wikidata_id = CASE WHEN $8::boolean THEN NULLIF($9,'') ELSE wikidata_id END,
			status      = COALESCE($10, status),
			source      = COALESCE($11, source),
			confidence  = COALESCE($12, confidence),
			as_of       = CASE WHEN $13::boolean THEN $14 ELSE as_of END,
			updated_at  = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+orgCols,
		id,
		up.KindID,
		up.Name,
		up.Code != nil, derefStr(up.Code),
		up.CountryID != nil, derefStr(up.CountryID),
		up.WikidataID != nil, derefStr(up.WikidataID),
		up.Status, up.Source, up.Confidence,
		up.AsOf != nil, up.AsOf)
	return scanOrg(row)
}

// ListOrgs pages the registry under the shared facet predicate. The filter block comes from
// buildOrgFilter (stats.go) — the SAME builder OrgStats uses, so `totalCount` describes exactly the
// set this pages. Only the keyset cursor and the limit are added here: a page boundary is not a
// filter, which is precisely why it is not in the shared builder.
func (r *Repository) ListOrgs(ctx context.Context, query string, f domain.OrgFilter, after string, lim int) ([]domain.Organization, error) {
	a := &argBuf{}
	where := buildOrgFilter(a, query, f)
	if after != "" {
		where += " AND o.id > " + a.add(after) + "::uuid"
	}
	rows, err := r.c.Query(ctx, `
		SELECT `+qualify(orgCols, "o")+`
		FROM oikumenea.external_organizations o
		JOIN oikumenea.external_org_kinds k ON k.id = o.kind_id
		WHERE `+where+`
		ORDER BY o.id LIMIT `+a.add(lim), a.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Organization
	for rows.Next() {
		o, err := scanOrgRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repository) SoftDeleteOrg(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.external_organizations SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func (r *Repository) TombstoneOrg(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.external_organizations SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func scanOrg(row pgx.Row) (domain.Organization, error) {
	var o domain.Organization
	var code, country, wikidata pgtype.Text
	var asOf pgtype.Timestamptz
	if err := row.Scan(&o.ID, &o.KindID, &o.Name, &code, &country, &wikidata, &o.Status,
		&o.Source, &o.Confidence, &asOf, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return domain.Organization{}, mapPGError(err)
	}
	o.Code, o.CountryID, o.WikidataID, o.AsOf = textVal(code), textVal(country), textVal(wikidata), timePtr(asOf)
	return o, nil
}

func scanOrgRows(rows pgx.Rows) (domain.Organization, error) {
	var o domain.Organization
	var code, country, wikidata pgtype.Text
	var asOf pgtype.Timestamptz
	if err := rows.Scan(&o.ID, &o.KindID, &o.Name, &code, &country, &wikidata, &o.Status,
		&o.Source, &o.Confidence, &asOf, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return domain.Organization{}, err
	}
	o.Code, o.CountryID, o.WikidataID, o.AsOf = textVal(code), textVal(country), textVal(wikidata), timePtr(asOf)
	return o, nil
}

// ============================ label helpers ============================

func (r *Repository) KindNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.external_org_kinds WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) CountryNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.geo_countries WHERE id = ANY($1::uuid[])`, ids)
}

func (r *Repository) namesByIDs(ctx context.Context, sql string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.c.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// ---- param helpers ----

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// qualify prefixes each comma-separated column in cols with the table alias (for the joined list query).
func qualify(cols, alias string) string {
	parts := strings.Split(cols, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
