// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters is the religion module's pgx-backed persistence adapter (M22). It uses raw pgx over
// a single command surface (pool for reads, tx for writes) rather than sqlc, because the taxonomy is
// closure- and resolution-heavy (recursive CTEs, nearest-declared-wins walks) — the same raw-SQL style
// the tenant/geo closure code uses. Postgres constraint violations (23505 unique / 23503 FK) map to
// domain sentinels.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/internal/religion/domain"
)

// Repository is the religion persistence adapter bound to one command surface (pool or tx).
type Repository struct{ c db.DBTX }

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository { return &Repository{c: conn} }

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

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
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
		case "23503":
			return domain.ErrInvalid
		}
	}
	return err
}

// ============================ taxon ranks ============================

func (r *Repository) ListTaxonRanks(ctx context.Context) ([]domain.TaxonRank, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, ordinal, status, sort_order
		FROM oikumenea.religion_taxon_ranks WHERE deleted_at IS NULL
		ORDER BY ordinal, sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TaxonRank
	for rows.Next() {
		var t domain.TaxonRank
		var so pgtype.Int4
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.Ordinal, &t.Status, &so); err != nil {
			return nil, err
		}
		t.SortOrder = intPtr(so)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertTaxonRank(ctx context.Context, code, name string, ordinal int, sortOrder *int) (domain.TaxonRank, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_taxon_ranks (code, name, ordinal, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, ordinal=EXCLUDED.ordinal, sort_order=EXCLUDED.sort_order
		RETURNING id, code, name, ordinal, status, sort_order`, code, name, ordinal, sortOrder)
	return scanTaxonRank(row)
}

func scanTaxonRank(row pgx.Row) (domain.TaxonRank, error) {
	var t domain.TaxonRank
	var so pgtype.Int4
	if err := row.Scan(&t.ID, &t.Code, &t.Name, &t.Ordinal, &t.Status, &so); err != nil {
		return domain.TaxonRank{}, mapPGError(err)
	}
	t.SortOrder = intPtr(so)
	return t, nil
}

// ============================ classifications ============================

func (r *Repository) ListClassifications(ctx context.Context) ([]domain.Classification, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), status, sort_order
		FROM oikumenea.religion_classifications WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClassifications(rows)
}

func (r *Repository) GetClassificationsByIDs(ctx context.Context, ids []string) ([]domain.Classification, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), status, sort_order
		FROM oikumenea.religion_classifications WHERE deleted_at IS NULL AND id = ANY($1)
		ORDER BY sort_order NULLS LAST, code`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClassifications(rows)
}

func scanClassifications(rows pgx.Rows) ([]domain.Classification, error) {
	var out []domain.Classification
	for rows.Next() {
		var c domain.Classification
		var so pgtype.Int4
		if err := rows.Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.Status, &so); err != nil {
			return nil, err
		}
		c.SortOrder = intPtr(so)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertClassification(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.Classification, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_classifications (code, name, description, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, sort_order=EXCLUDED.sort_order
		RETURNING id, code, name, COALESCE(description,''), status, sort_order`, code, name, description, sortOrder)
	var c domain.Classification
	var so pgtype.Int4
	if err := row.Scan(&c.ID, &c.Code, &c.Name, &c.Description, &c.Status, &so); err != nil {
		return domain.Classification{}, mapPGError(err)
	}
	c.SortOrder = intPtr(so)
	return c, nil
}

// ============================ org kinds ============================

func (r *Repository) ListOrgKinds(ctx context.Context) ([]domain.OrgKind, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, religion_id, code, name, ordinal, status, sort_order
		FROM oikumenea.religion_org_kinds WHERE deleted_at IS NULL
		ORDER BY ordinal NULLS LAST, sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrgKind
	for rows.Next() {
		k, err := scanOrgKind(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertOrgKind(ctx context.Context, code, name string, religionID *string, ordinal, sortOrder *int) (domain.OrgKind, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_org_kinds (code, name, religion_id, ordinal, sort_order)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, religion_id=EXCLUDED.religion_id, ordinal=EXCLUDED.ordinal, sort_order=EXCLUDED.sort_order
		RETURNING id, religion_id, code, name, ordinal, status, sort_order`, code, name, religionID, ordinal, sortOrder)
	return scanOrgKind(row)
}

func scanOrgKind(row pgx.Row) (domain.OrgKind, error) {
	var k domain.OrgKind
	var rel pgtype.Text
	var ord, so pgtype.Int4
	if err := row.Scan(&k.ID, &rel, &k.Code, &k.Name, &ord, &k.Status, &so); err != nil {
		return domain.OrgKind{}, mapPGError(err)
	}
	k.ReligionID, k.Ordinal, k.SortOrder = textVal(rel), intPtr(ord), intPtr(so)
	return k, nil
}

// ============================ policy kinds ============================

func (r *Repository) ListPolicyKinds(ctx context.Context) ([]domain.PolicyKind, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), status, sort_order
		FROM oikumenea.religion_policy_kinds WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PolicyKind
	for rows.Next() {
		var k domain.PolicyKind
		var so pgtype.Int4
		if err := rows.Scan(&k.ID, &k.Code, &k.Name, &k.Description, &k.Status, &so); err != nil {
			return nil, err
		}
		k.SortOrder = intPtr(so)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertPolicyKind(ctx context.Context, code, name string, description *string, sortOrder *int) (domain.PolicyKind, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_policy_kinds (code, name, description, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, sort_order=EXCLUDED.sort_order
		RETURNING id, code, name, COALESCE(description,''), status, sort_order`, code, name, description, sortOrder)
	var k domain.PolicyKind
	var so pgtype.Int4
	if err := row.Scan(&k.ID, &k.Code, &k.Name, &k.Description, &k.Status, &so); err != nil {
		return domain.PolicyKind{}, mapPGError(err)
	}
	k.SortOrder = intPtr(so)
	return k, nil
}

// ============================ taxa ============================

const taxonCols = `t.id, t.parent_id, t.rank_id, rk.code, t.religion_id, t.code, t.name,
	COALESCE(t.description,''), COALESCE(t.wikidata_id,''), t.sort_order, t.created_at, t.updated_at`

func scanTaxon(row pgx.Row, depth int) (domain.Taxon, error) {
	var t domain.Taxon
	var parent, religion pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&t.ID, &parent, &t.RankID, &t.RankCode, &religion, &t.Code, &t.Name,
		&t.Description, &t.WikidataID, &so, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return domain.Taxon{}, mapPGError(err)
	}
	t.ParentID, t.ReligionID, t.SortOrder, t.Depth = textVal(parent), textVal(religion), intPtr(so), depth
	return t, nil
}

func (r *Repository) GetTaxon(ctx context.Context, id string) (domain.Taxon, error) {
	row := r.c.QueryRow(ctx, `SELECT `+taxonCols+`
		FROM oikumenea.religion_taxa t JOIN oikumenea.religion_taxon_ranks rk ON rk.id=t.rank_id
		WHERE t.id=$1 AND t.deleted_at IS NULL`, id)
	t, err := scanTaxon(row, 0)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Taxon{}, domain.ErrTaxonNotFound
	}
	return t, err
}

func (r *Repository) InsertTaxon(ctx context.Context, in domain.TaxonInput) (domain.Taxon, error) {
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_taxa (code, name, rank_id, parent_id, description, wikidata_id, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.Code, in.Name, in.RankID, nilIfEmpty(in.ParentID), nilIfEmpty(in.Description), nilIfEmpty(in.WikidataID), in.SortOrder).Scan(&id)
	if err != nil {
		return domain.Taxon{}, mapPGError(err)
	}
	return r.GetTaxon(ctx, id)
}

func (r *Repository) UpdateTaxon(ctx context.Context, id string, up domain.TaxonUpdate) (domain.Taxon, error) {
	ct, err := r.c.Exec(ctx, `
		UPDATE oikumenea.religion_taxa SET
			name = COALESCE($2, name),
			rank_id = COALESCE($3, rank_id),
			description = COALESCE($4, description),
			wikidata_id = COALESCE($5, wikidata_id),
			sort_order = COALESCE($6, sort_order)
		WHERE id=$1 AND deleted_at IS NULL`,
		id, up.Name, up.RankID, up.Description, up.WikidataID, up.SortOrder)
	if err != nil {
		return domain.Taxon{}, mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.Taxon{}, domain.ErrTaxonNotFound
	}
	return r.GetTaxon(ctx, id)
}

func (r *Repository) SoftDeleteTaxon(ctx context.Context, id string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_taxa SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrTaxonNotFound
	}
	return nil
}

func (r *Repository) CountTaxonChildren(ctx context.Context, id string) (int, error) {
	var n int
	err := r.c.QueryRow(ctx, `SELECT count(*) FROM oikumenea.religion_taxa WHERE parent_id=$1 AND deleted_at IS NULL`, id).Scan(&n)
	return n, err
}

func (r *Repository) CountUnitsClassifiedBy(ctx context.Context, taxonID string) (int, error) {
	var n int
	err := r.c.QueryRow(ctx, `SELECT count(*) FROM oikumenea.religion_org_classifications WHERE taxon_id=$1 AND deleted_at IS NULL`, taxonID).Scan(&n)
	return n, err
}

// IsDescendant reports whether candidate is descendantID (or below) of ancestorID via the closure.
func (r *Repository) IsDescendant(ctx context.Context, ancestorID, candidateID string) (bool, error) {
	var n int
	err := r.c.QueryRow(ctx, `SELECT count(*) FROM oikumenea.religion_taxa_closure WHERE ancestor_id=$1 AND descendant_id=$2`, ancestorID, candidateID).Scan(&n)
	return n > 0, err
}

func (r *Repository) SetTaxonParent(ctx context.Context, id, parentID string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_taxa SET parent_id=$2 WHERE id=$1 AND deleted_at IS NULL`, id, nilIfEmpty(parentID))
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrTaxonNotFound
	}
	return nil
}

// ListTaxa pages the taxonomy under the shared facet predicate. The filter block comes from
// buildTaxonFilter (stats.go) — the SAME builder TaxonStats uses, so `totalCount` describes exactly
// the set this pages. Only the keyset cursor, the limit and the depth projection are added here: a
// page boundary is not a filter, and the depth column is a projection rather than a predicate.
func (r *Repository) ListTaxa(ctx context.Context, query string, f domain.TaxonFilter, after string, limit int) ([]domain.Taxon, error) {
	a := &argBuf{}
	where := buildTaxonFilter(a, query, f)
	// Depth is reported RELATIVE to the parent the caller asked about, and is 0 without one. It binds
	// the same value the filter already bound, a second time: a projection and a predicate are
	// separate uses, and sharing one placeholder across them would couple the builder to this query.
	depthExpr := "0"
	if f.Parent != nil {
		depthExpr = "(SELECT depth FROM oikumenea.religion_taxa_closure c WHERE c.ancestor_id = " +
			a.add(*f.Parent) + "::uuid AND c.descendant_id = t.id)"
	}
	// KEYSET ON THE SORT COLUMN. This used to order by `rk.ordinal, t.sort_order, t.code, t.id` while
	// paging on `t.id > cursor`, and those are different orders: every row that sorts after the cursor
	// position but carries a smaller id was silently dropped from page 2 onward. On the seeded
	// taxonomy that lost 16 of 100 taxa — invisible, because a short page still looks like a page.
	// Found by M58's differential test (totalCount vs exhaustive paging), which is exactly the class
	// of defect that contract exists to catch.
	//
	// `code` rather than a composite cursor over the old four columns: it is UNIQUE among active taxa
	// (religion_taxa_code_active), indexed, stable, and human-meaningful, so one column both orders
	// and pages correctly. The rank-ladder ordering the old ORDER BY was reaching for survives where
	// it carries meaning — the dashboard's rankId chart orders by the rank's own ordinal, and the
	// workspace tree renders containment from parent ids rather than from row order.
	if after != "" {
		where += " AND t.code > " + a.add(after)
	}
	sql := `SELECT ` + taxonCols + `, ` + depthExpr + ` AS depth
		FROM oikumenea.religion_taxa t JOIN oikumenea.religion_taxon_ranks rk ON rk.id=t.rank_id
		WHERE ` + where + `
		ORDER BY t.code
		LIMIT ` + a.add(limit)
	rows, err := r.c.Query(ctx, sql, a.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Taxon
	for rows.Next() {
		var t domain.Taxon
		var parentID, religionID pgtype.Text
		var so, depth pgtype.Int4
		if err := rows.Scan(&t.ID, &parentID, &t.RankID, &t.RankCode, &religionID, &t.Code, &t.Name,
			&t.Description, &t.WikidataID, &so, &t.CreatedAt, &t.UpdatedAt, &depth); err != nil {
			return nil, err
		}
		t.ParentID, t.ReligionID, t.SortOrder = textVal(parentID), textVal(religionID), intPtr(so)
		if depth.Valid {
			t.Depth = int(depth.Int32)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RebuildClosure recomputes the whole-tree closure + denormalized roots (mirrors migration 0023).
func (r *Repository) RebuildClosure(ctx context.Context) (domain.ClosureReport, error) {
	if _, err := r.c.Exec(ctx, `DELETE FROM oikumenea.religion_taxa_closure`); err != nil {
		return domain.ClosureReport{}, err
	}
	if _, err := r.c.Exec(ctx, `
		INSERT INTO oikumenea.religion_taxa_closure (ancestor_id, descendant_id, depth)
		WITH RECURSIVE anc AS (
		  SELECT id AS ancestor_id, id AS descendant_id, 0 AS depth
		  FROM oikumenea.religion_taxa WHERE deleted_at IS NULL
		  UNION ALL
		  SELECT a.ancestor_id, t.id, a.depth + 1
		  FROM anc a JOIN oikumenea.religion_taxa t ON t.parent_id = a.descendant_id AND t.deleted_at IS NULL
		)
		SELECT ancestor_id, descendant_id, depth FROM anc`); err != nil {
		return domain.ClosureReport{}, err
	}
	if _, err := r.c.Exec(ctx, `
		UPDATE oikumenea.religion_taxa t SET religion_id = root.ancestor_id
		FROM (
		  SELECT c.descendant_id, c.ancestor_id
		  FROM oikumenea.religion_taxa_closure c
		  JOIN oikumenea.religion_taxa a ON a.id = c.ancestor_id AND a.parent_id IS NULL
		) root
		WHERE root.descendant_id = t.id`); err != nil {
		return domain.ClosureReport{}, err
	}
	var n int
	if err := r.c.QueryRow(ctx, `SELECT count(*) FROM oikumenea.religion_taxa_closure`).Scan(&n); err != nil {
		return domain.ClosureReport{}, err
	}
	return domain.ClosureReport{Rows: n, InDrift: false}, nil
}

// EffectiveClassificationsForTaxon resolves the nearest ancestor (incl. self) that declares any theism
// tags, returning that set (nearest-declared-wins).
func (r *Repository) EffectiveClassificationsForTaxon(ctx context.Context, taxonID string) ([]domain.Classification, error) {
	var anchor string
	err := r.c.QueryRow(ctx, `
		SELECT c.ancestor_id
		FROM oikumenea.religion_taxa_closure c
		WHERE c.descendant_id = $1
		  AND EXISTS (SELECT 1 FROM oikumenea.religion_taxon_classifications tc WHERE tc.taxon_id = c.ancestor_id)
		ORDER BY c.depth ASC
		LIMIT 1`, taxonID).Scan(&anchor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.classificationsForTaxon(ctx, anchor)
}

func (r *Repository) classificationsForTaxon(ctx context.Context, taxonID string) ([]domain.Classification, error) {
	rows, err := r.c.Query(ctx, `
		SELECT cl.id, cl.code, cl.name, COALESCE(cl.description,''), cl.status, cl.sort_order
		FROM oikumenea.religion_taxon_classifications tc
		JOIN oikumenea.religion_classifications cl ON cl.id = tc.classification_id
		WHERE tc.taxon_id = $1 AND cl.deleted_at IS NULL
		ORDER BY cl.sort_order NULLS LAST, cl.code`, taxonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClassifications(rows)
}

// SetTaxonClassifications replaces the theism tags declared directly on a taxon.
func (r *Repository) SetTaxonClassifications(ctx context.Context, taxonID string, classificationIDs []string) error {
	if _, err := r.c.Exec(ctx, `DELETE FROM oikumenea.religion_taxon_classifications WHERE taxon_id=$1`, taxonID); err != nil {
		return err
	}
	for _, cid := range classificationIDs {
		if _, err := r.c.Exec(ctx, `INSERT INTO oikumenea.religion_taxon_classifications (taxon_id, classification_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, taxonID, cid); err != nil {
			return mapPGError(err)
		}
	}
	return nil
}

// ============================ org profile + classifications ============================

func (r *Repository) GetOrgProfileRow(ctx context.Context, unitID string) (domain.OrgProfile, error) {
	var p domain.OrgProfile
	var kind, short pgtype.Text
	err := r.c.QueryRow(ctx, `
		SELECT unit_id, org_kind_id, short_code, created_at, updated_at
		FROM oikumenea.religion_org_profiles WHERE unit_id=$1 AND deleted_at IS NULL`, unitID).
		Scan(&p.UnitID, &kind, &short, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrgProfile{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.OrgProfile{}, err
	}
	p.OrgKindID, p.ShortCode = textVal(kind), textVal(short)
	return p, nil
}

func (r *Repository) UpsertOrgProfile(ctx context.Context, unitID string, orgKindID, shortCode *string) (domain.OrgProfile, error) {
	_, err := r.c.Exec(ctx, `
		INSERT INTO oikumenea.religion_org_profiles (unit_id, org_kind_id, short_code)
		VALUES ($1,$2,$3)
		ON CONFLICT (unit_id) DO UPDATE SET org_kind_id=EXCLUDED.org_kind_id, short_code=EXCLUDED.short_code, deleted_at=NULL`,
		unitID, orgKindID, shortCode)
	if err != nil {
		return domain.OrgProfile{}, mapPGError(err)
	}
	return r.GetOrgProfileRow(ctx, unitID)
}

func (r *Repository) ListOrgClassifications(ctx context.Context, unitID string) ([]domain.OrgClassification, error) {
	rows, err := r.c.Query(ctx, `
		SELECT oc.id, oc.unit_id, oc.taxon_id, t.code, t.name, oc.is_primary,
		       COALESCE(oc.source,''), COALESCE(oc.confidence,''), oc.created_at, oc.updated_at
		FROM oikumenea.religion_org_classifications oc
		JOIN oikumenea.religion_taxa t ON t.id = oc.taxon_id
		WHERE oc.unit_id=$1 AND oc.deleted_at IS NULL
		ORDER BY oc.is_primary DESC, t.code`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrgClassification
	for rows.Next() {
		var c domain.OrgClassification
		if err := rows.Scan(&c.ID, &c.UnitID, &c.TaxonID, &c.TaxonCode, &c.TaxonName, &c.IsPrimary,
			&c.Source, &c.Confidence, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) ClearPrimaryClassification(ctx context.Context, unitID string) error {
	_, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_org_classifications SET is_primary=false WHERE unit_id=$1 AND is_primary AND deleted_at IS NULL`, unitID)
	return err
}

func (r *Repository) AddOrgClassification(ctx context.Context, unitID, taxonID string, isPrimary bool, source, confidence *string) (domain.OrgClassification, error) {
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_org_classifications (unit_id, taxon_id, is_primary, source, confidence)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, unitID, taxonID, isPrimary, source, confidence).Scan(&id)
	if err != nil {
		return domain.OrgClassification{}, mapPGError(err)
	}
	rows, err := r.ListOrgClassifications(ctx, unitID)
	if err != nil {
		return domain.OrgClassification{}, err
	}
	for _, c := range rows {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.OrgClassification{}, domain.ErrInvalid
}

func (r *Repository) RemoveOrgClassification(ctx context.Context, unitID, linkID string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_org_classifications SET deleted_at=now() WHERE id=$1 AND unit_id=$2 AND deleted_at IS NULL`, linkID, unitID)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrInvalid
	}
	return nil
}

// ============================ unit theism override + effective type ============================

func (r *Repository) SetUnitClassifications(ctx context.Context, unitID string, classificationIDs []string) error {
	if _, err := r.c.Exec(ctx, `DELETE FROM oikumenea.religion_unit_classifications WHERE unit_id=$1`, unitID); err != nil {
		return err
	}
	for _, cid := range classificationIDs {
		if _, err := r.c.Exec(ctx, `INSERT INTO oikumenea.religion_unit_classifications (unit_id, classification_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, unitID, cid); err != nil {
			return mapPGError(err)
		}
	}
	return nil
}

func (r *Repository) listUnitOverride(ctx context.Context, unitID string) ([]domain.Classification, error) {
	rows, err := r.c.Query(ctx, `
		SELECT cl.id, cl.code, cl.name, COALESCE(cl.description,''), cl.status, cl.sort_order
		FROM oikumenea.religion_unit_classifications uc
		JOIN oikumenea.religion_classifications cl ON cl.id = uc.classification_id
		WHERE uc.unit_id=$1 AND cl.deleted_at IS NULL
		ORDER BY cl.sort_order NULLS LAST, cl.code`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClassifications(rows)
}

// EffectiveTypeForUnit resolves the unit override first; else the primary taxon's effective set.
func (r *Repository) EffectiveTypeForUnit(ctx context.Context, unitID string) (domain.EffectiveType, error) {
	override, err := r.listUnitOverride(ctx, unitID)
	if err != nil {
		return domain.EffectiveType{}, err
	}
	if len(override) > 0 {
		return domain.EffectiveType{UnitID: unitID, Classifications: override, Source: "unit"}, nil
	}
	var primaryTaxon, taxonCode string
	err = r.c.QueryRow(ctx, `
		SELECT oc.taxon_id, t.code
		FROM oikumenea.religion_org_classifications oc
		JOIN oikumenea.religion_taxa t ON t.id = oc.taxon_id
		WHERE oc.unit_id=$1 AND oc.is_primary AND oc.deleted_at IS NULL
		LIMIT 1`, unitID).Scan(&primaryTaxon, &taxonCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EffectiveType{UnitID: unitID, Source: "none"}, nil
	}
	if err != nil {
		return domain.EffectiveType{}, err
	}
	cls, err := r.EffectiveClassificationsForTaxon(ctx, primaryTaxon)
	if err != nil {
		return domain.EffectiveType{}, err
	}
	if len(cls) == 0 {
		return domain.EffectiveType{UnitID: unitID, Source: "none"}, nil
	}
	return domain.EffectiveType{UnitID: unitID, Classifications: cls, Source: "taxon:" + taxonCode}, nil
}

// ============================ org policies ============================

func (r *Repository) ListOrgPolicies(ctx context.Context, unitID string) ([]domain.OrgPolicy, error) {
	rows, err := r.c.Query(ctx, `
		SELECT p.id, p.unit_id, p.policy_kind_id, k.code, COALESCE(p.reason,''),
		       p.decided_by_person_id, p.decided_at, p.created_at, p.updated_at
		FROM oikumenea.religion_org_policies p
		JOIN oikumenea.religion_policy_kinds k ON k.id = p.policy_kind_id
		WHERE p.unit_id=$1 AND p.deleted_at IS NULL
		ORDER BY k.code`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrgPolicy
	for rows.Next() {
		var p domain.OrgPolicy
		var decidedBy pgtype.Text
		var decidedAt pgtype.Timestamptz
		if err := rows.Scan(&p.ID, &p.UnitID, &p.PolicyKindID, &p.PolicyKindCode, &p.Reason,
			&decidedBy, &decidedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.DecidedByPersonID = textVal(decidedBy)
		p.DecidedAt = timePtr(decidedAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) AddOrgPolicy(ctx context.Context, unitID, policyKindID string, reason, decidedByPersonID *string) (domain.OrgPolicy, error) {
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_org_policies (unit_id, policy_kind_id, reason, decided_by_person_id, decided_at)
		VALUES ($1,$2,$3,$4, now()) RETURNING id`, unitID, policyKindID, reason, decidedByPersonID).Scan(&id)
	if err != nil {
		return domain.OrgPolicy{}, mapPGError(err)
	}
	all, err := r.ListOrgPolicies(ctx, unitID)
	if err != nil {
		return domain.OrgPolicy{}, err
	}
	for _, p := range all {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.OrgPolicy{}, domain.ErrInvalid
}

func (r *Repository) RemoveOrgPolicy(ctx context.Context, unitID, policyID string) error {
	ct, err := r.c.Exec(ctx, `UPDATE oikumenea.religion_org_policies SET deleted_at=now() WHERE id=$1 AND unit_id=$2 AND deleted_at IS NULL`, policyID, unitID)
	if err != nil {
		return mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrPolicyNotFound
	}
	return nil
}

func (r *Repository) HasActivePolicy(ctx context.Context, unitID, policyKindCode string) (bool, error) {
	var n int
	err := r.c.QueryRow(ctx, `
		SELECT count(*) FROM oikumenea.religion_org_policies p
		JOIN oikumenea.religion_policy_kinds k ON k.id = p.policy_kind_id
		WHERE p.unit_id=$1 AND k.code=$2 AND p.deleted_at IS NULL`, unitID, policyKindCode).Scan(&n)
	return n > 0, err
}

// itoa is a tiny strconv.Itoa to keep the dynamic-SQL builder dependency-free.
//
//nolint:unused // retained beside the builder helpers; argBuf (stats.go) now numbers placeholders.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
