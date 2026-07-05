// Package adapters — the person regulatory-sanction import store (D-Watchlists, M34). Raw pgx (no sqlc):
// a person-scoped import target (unusual — most targets are instance-global catalogs). The upsert keys
// idempotency on (person_id, external_id) and writes the M34 person_regulatory_sanctions table; a record
// whose person RID does not resolve is skipped by the handler (PersonExists guards it).
package adapters

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// RegulatorySanctionRepo is the raw-pgx implementation of domain.RegulatorySanctionStore, bound to a
// single db.DBTX (the pool for reads, or a caller-supplied transaction so the upsert and its audit row
// commit together).
type RegulatorySanctionRepo struct{ c db.DBTX }

// NewRegulatorySanctionRepo binds a store to the given command surface.
func NewRegulatorySanctionRepo(conn db.DBTX) *RegulatorySanctionRepo { return &RegulatorySanctionRepo{c: conn} }

var _ domain.RegulatorySanctionStore = (*RegulatorySanctionRepo)(nil)

// PersonExists reports whether an active (non-deleted) person carries this RID.
func (r *RegulatorySanctionRepo) PersonExists(ctx context.Context, personID string) (bool, error) {
	var one int
	err := r.c.QueryRow(ctx, `SELECT 1 FROM oikumenea.person_persons WHERE id = $1 AND deleted_at IS NULL`, personID).Scan(&one)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Get returns the existing (person, external_id) sanction's comparable fields (found=false when absent).
func (r *RegulatorySanctionRepo) Get(ctx context.Context, personID, externalID string) (domain.RegulatorySanction, bool, error) {
	var (
		s        = domain.RegulatorySanction{PersonID: personID, ExternalID: externalID}
		amount   pgtype.Numeric
		currency pgtype.Text
		date     pgtype.Date
		srcURL   pgtype.Text
	)
	err := r.c.QueryRow(ctx, `
		SELECT regulator, action_type, amount, currency, status, sanction_date, source_url
		FROM oikumenea.person_regulatory_sanctions
		WHERE person_id = $1 AND external_id = $2 AND deleted_at IS NULL`, personID, externalID).
		Scan(&s.Regulator, &s.ActionType, &amount, &currency, &s.Status, &date, &srcURL)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.RegulatorySanction{}, false, nil
		}
		return domain.RegulatorySanction{}, false, err
	}
	s.Amount = numPtr(amount)
	s.Currency = currency.String
	if date.Valid {
		s.SanctionDate = date.Time.Format("2006-01-02")
	}
	s.SourceURL = srcURL.String
	return s, true, nil
}

// numPtr maps a stored numeric back into an optional float64 (via its string Value()).
func numPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil || v == nil {
		return nil
	}
	str, ok := v.(string)
	if !ok {
		return nil
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return nil
	}
	return &f
}

// Insert creates an imported regulatory sanction for the person.
func (r *RegulatorySanctionRepo) Insert(ctx context.Context, s domain.RegulatorySanction, prov domain.Provenance) error {
	_, err := r.c.Exec(ctx, `
		INSERT INTO oikumenea.person_regulatory_sanctions
			(person_id, regulator, action_type, amount, currency, status, sanction_date,
			 source_url, external_id, source, confidence)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''), NULLIF($9,''), 'imported', 'probable')`,
		s.PersonID, s.Regulator, actionOrDefault(s.ActionType), numArg(s.Amount), s.Currency,
		statusOrDefault(s.Status), dateArg(s.SanctionDate), s.SourceURL, s.ExternalID)
	return err
}

// UpdateImport refreshes the sanction fields on the existing (person, external_id) row.
func (r *RegulatorySanctionRepo) UpdateImport(ctx context.Context, s domain.RegulatorySanction, prov domain.Provenance) error {
	_, err := r.c.Exec(ctx, `
		UPDATE oikumenea.person_regulatory_sanctions SET
			regulator     = $3,
			action_type   = $4,
			amount        = $5,
			currency      = NULLIF($6,''),
			status        = $7,
			sanction_date = $8,
			source_url    = NULLIF($9,''),
			source        = 'imported',
			updated_at    = now()
		WHERE person_id = $1 AND external_id = $2 AND deleted_at IS NULL`,
		s.PersonID, s.ExternalID, s.Regulator, actionOrDefault(s.ActionType), numArg(s.Amount),
		s.Currency, statusOrDefault(s.Status), dateArg(s.SanctionDate), s.SourceURL)
	return err
}

func actionOrDefault(a string) string {
	if a == "" {
		return "other"
	}
	return a
}

func statusOrDefault(s string) string {
	if s == "" {
		return "active"
	}
	return s
}

// numArg maps an optional float to a nullable numeric column (nil => NULL).
func numArg(p *float64) pgtype.Numeric {
	var n pgtype.Numeric
	if p == nil {
		return n
	}
	if err := n.Scan(strconv.FormatFloat(*p, 'f', -1, 64)); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

// dateArg maps an ISO date string to a nullable date column ("" => NULL).
func dateArg(s string) pgtype.Date {
	var d pgtype.Date
	if s == "" {
		return d
	}
	if err := d.Scan(s); err != nil {
		return pgtype.Date{}
	}
	return d
}
