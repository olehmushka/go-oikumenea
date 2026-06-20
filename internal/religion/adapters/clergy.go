// Clergy persistence (D-ClergyCredential, M23): per-tradition grade catalogs + the credential Link.
// Raw pgx over the shared command surface, mirroring repository.go; constraint violations map to domain
// sentinels via mapPGError.
package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/religion/domain"
)

// ---- grade categories ----

func (r *Repository) ListGradeCategories(ctx context.Context) ([]domain.GradeCategory, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, tradition_taxon_id, code, name, ordinal, status, sort_order
		FROM oikumenea.religion_grade_categories WHERE deleted_at IS NULL
		ORDER BY ordinal NULLS LAST, sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.GradeCategory
	for rows.Next() {
		var g domain.GradeCategory
		var tradition pgtype.Text
		var ord, so pgtype.Int4
		if err := rows.Scan(&g.ID, &tradition, &g.Code, &g.Name, &ord, &g.Status, &so); err != nil {
			return nil, err
		}
		g.TraditionTaxonID, g.Ordinal, g.SortOrder = textVal(tradition), intPtr(ord), intPtr(so)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertGradeCategory(ctx context.Context, traditionTaxonID *string, code, name string, ordinal, sortOrder *int) (domain.GradeCategory, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_grade_categories (tradition_taxon_id, code, name, ordinal, sort_order)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, ordinal=EXCLUDED.ordinal, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, code, name, ordinal, status, sort_order`, traditionTaxonID, code, name, ordinal, sortOrder)
	var g domain.GradeCategory
	var tradition pgtype.Text
	var ord, so pgtype.Int4
	if err := row.Scan(&g.ID, &tradition, &g.Code, &g.Name, &ord, &g.Status, &so); err != nil {
		return domain.GradeCategory{}, mapPGError(err)
	}
	g.TraditionTaxonID, g.Ordinal, g.SortOrder = textVal(tradition), intPtr(ord), intPtr(so)
	return g, nil
}

// ---- clergy grades ----

func scanClergyGrade(row pgx.Row) (domain.ClergyGrade, error) {
	var g domain.ClergyGrade
	var tradition pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&g.ID, &tradition, &g.GradeCategoryID, &g.Code, &g.Name, &g.Ordinal, &g.Status, &so); err != nil {
		return domain.ClergyGrade{}, mapPGError(err)
	}
	g.TraditionTaxonID, g.SortOrder = textVal(tradition), intPtr(so)
	return g, nil
}

func (r *Repository) ListClergyGrades(ctx context.Context, tradition string) ([]domain.ClergyGrade, error) {
	conds := []string{"deleted_at IS NULL"}
	args := []any{}
	if tradition != "" {
		args = append(args, tradition)
		conds = append(conds, "tradition_taxon_id IN (SELECT id FROM oikumenea.religion_taxa WHERE code = $1 OR id::text = $1)")
	}
	sql := `SELECT id, tradition_taxon_id, grade_category_id, code, name, ordinal, status, sort_order
		FROM oikumenea.religion_clergy_grades WHERE ` + strings.Join(conds, " AND ") + `
		ORDER BY tradition_taxon_id NULLS FIRST, ordinal, sort_order NULLS LAST, code`
	rows, err := r.c.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ClergyGrade
	for rows.Next() {
		g, err := scanClergyGrade(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertClergyGrade(ctx context.Context, traditionTaxonID *string, gradeCategoryID, code, name string, ordinal int, sortOrder *int) (domain.ClergyGrade, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_clergy_grades (tradition_taxon_id, grade_category_id, code, name, ordinal, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET grade_category_id=EXCLUDED.grade_category_id, name=EXCLUDED.name, ordinal=EXCLUDED.ordinal, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, grade_category_id, code, name, ordinal, status, sort_order`,
		traditionTaxonID, gradeCategoryID, code, name, ordinal, sortOrder)
	return scanClergyGrade(row)
}

// ---- office types ----

func (r *Repository) ListOfficeTypes(ctx context.Context) ([]domain.OfficeType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, tradition_taxon_id, code, name, status, sort_order
		FROM oikumenea.religion_office_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OfficeType
	for rows.Next() {
		var o domain.OfficeType
		var tradition pgtype.Text
		var so pgtype.Int4
		if err := rows.Scan(&o.ID, &tradition, &o.Code, &o.Name, &o.Status, &so); err != nil {
			return nil, err
		}
		o.TraditionTaxonID, o.SortOrder = textVal(tradition), intPtr(so)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertOfficeType(ctx context.Context, traditionTaxonID *string, code, name string, sortOrder *int) (domain.OfficeType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_office_types (tradition_taxon_id, code, name, sort_order)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (tradition_taxon_id, code) WHERE deleted_at IS NULL
		DO UPDATE SET name=EXCLUDED.name, sort_order=EXCLUDED.sort_order
		RETURNING id, tradition_taxon_id, code, name, status, sort_order`, traditionTaxonID, code, name, sortOrder)
	var o domain.OfficeType
	var tradition pgtype.Text
	var so pgtype.Int4
	if err := row.Scan(&o.ID, &tradition, &o.Code, &o.Name, &o.Status, &so); err != nil {
		return domain.OfficeType{}, mapPGError(err)
	}
	o.TraditionTaxonID, o.SortOrder = textVal(tradition), intPtr(so)
	return o, nil
}

// ---- clergy credentials ----

const credentialCols = `c.id, c.person_id, c.clergy_grade_id, g.code, g.name, c.org_unit_id,
	c.granted_on, c.conferred_by_person_id, c.status, c.effective_from, c.effective_to,
	COALESCE(c.source,''), COALESCE(c.confidence,''), c.created_at, c.updated_at`

func scanCredential(row pgx.Row) (domain.ClergyCredential, error) {
	var c domain.ClergyCredential
	var grantedOn pgtype.Date
	var conferredBy pgtype.Text
	var effectiveTo pgtype.Timestamptz
	if err := row.Scan(&c.ID, &c.PersonID, &c.ClergyGradeID, &c.GradeCode, &c.GradeName, &c.OrgUnitID,
		&grantedOn, &conferredBy, &c.Status, &c.EffectiveFrom, &effectiveTo,
		&c.Source, &c.Confidence, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.ClergyCredential{}, mapPGError(err)
	}
	if grantedOn.Valid {
		t := grantedOn.Time
		c.GrantedOn = &t
	}
	c.ConferredByPersonID = textVal(conferredBy)
	c.EffectiveTo = timePtr(effectiveTo)
	return c, nil
}

func (r *Repository) listCredentials(ctx context.Context, where string, arg string) ([]domain.ClergyCredential, error) {
	rows, err := r.c.Query(ctx, `SELECT `+credentialCols+`
		FROM oikumenea.religion_clergy_credentials c
		JOIN oikumenea.religion_clergy_grades g ON g.id = c.clergy_grade_id
		WHERE `+where+` AND c.deleted_at IS NULL
		ORDER BY c.effective_from DESC, c.id`, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ClergyCredential
	for rows.Next() {
		c, err := scanCredential(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) ListClergyCredentialsByPerson(ctx context.Context, personID string) ([]domain.ClergyCredential, error) {
	return r.listCredentials(ctx, "c.person_id = $1", personID)
}

func (r *Repository) ListClergyCredentialsByUnit(ctx context.Context, unitID string) ([]domain.ClergyCredential, error) {
	return r.listCredentials(ctx, "c.org_unit_id = $1", unitID)
}

func (r *Repository) GetClergyCredential(ctx context.Context, id string) (domain.ClergyCredential, error) {
	row := r.c.QueryRow(ctx, `SELECT `+credentialCols+`
		FROM oikumenea.religion_clergy_credentials c
		JOIN oikumenea.religion_clergy_grades g ON g.id = c.clergy_grade_id
		WHERE c.id = $1 AND c.deleted_at IS NULL`, id)
	c, err := scanCredential(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClergyCredential{}, domain.ErrCredentialNotFound
	}
	return c, err
}

func (r *Repository) InsertClergyCredential(ctx context.Context, in domain.ClergyCredentialInput) (domain.ClergyCredential, error) {
	var id string
	err := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.religion_clergy_credentials
			(person_id, clergy_grade_id, org_unit_id, granted_on, conferred_by_person_id, source, confidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.PersonID, in.ClergyGradeID, in.OrgUnitID, in.GrantedOn, nilIfEmpty(in.ConferredByPersonID),
		nilIfEmpty(in.Source), nilIfEmpty(in.Confidence)).Scan(&id)
	if err != nil {
		return domain.ClergyCredential{}, mapPGError(err)
	}
	return r.GetClergyCredential(ctx, id)
}

func (r *Repository) UpdateClergyCredential(ctx context.Context, id string, up domain.ClergyCredentialUpdate) (domain.ClergyCredential, error) {
	ct, err := r.c.Exec(ctx, `
		UPDATE oikumenea.religion_clergy_credentials SET
			status = COALESCE($2, status),
			effective_to = COALESCE($3, effective_to)
		WHERE id = $1 AND deleted_at IS NULL`, id, up.Status, up.EffectiveTo)
	if err != nil {
		return domain.ClergyCredential{}, mapPGError(err)
	}
	if ct.RowsAffected() == 0 {
		return domain.ClergyCredential{}, domain.ErrCredentialNotFound
	}
	return r.GetClergyCredential(ctx, id)
}
