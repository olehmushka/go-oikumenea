// Package adapters is the finance module's pgx-backed persistence adapter (M44, D-Finance). Raw pgx over
// a single command surface (pool for reads, tx for writes) — the vehicle/religion raw-SQL style — because
// of the polymorphic holder, the envelope columns, and the cross-module org/person lookups. Postgres
// constraint violations (23505 unique / 23503 FK) map to domain sentinels.
package adapters

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/finance/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the finance persistence adapter bound to one command surface (pool or tx).
type Repository struct{ c db.DBTX }

// NewRepository binds a repository to the given command surface.
func NewRepository(conn db.DBTX) *Repository { return &Repository{c: conn} }

// compile-time assertion that the adapter satisfies the domain port.
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
		case "23503":
			return domain.ErrInvalid
		}
	}
	return err
}

// ============================ account types ============================

func (r *Repository) ListAccountTypes(ctx context.Context) ([]domain.AccountType, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, status, sort_order
		FROM oikumenea.finance_account_types WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AccountType
	for rows.Next() {
		var t domain.AccountType
		var so pgtype.Int4
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.Status, &so); err != nil {
			return nil, err
		}
		t.SortOrder = intPtr(so)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertAccountType(ctx context.Context, code, name string, sortOrder *int) (domain.AccountType, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.finance_account_types (code, name, sort_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, status, sort_order`, code, name, sortOrder)
	var t domain.AccountType
	var so pgtype.Int4
	if err := row.Scan(&t.ID, &t.Code, &t.Name, &t.Status, &so); err != nil {
		return domain.AccountType{}, mapPGError(err)
	}
	t.SortOrder = intPtr(so)
	return t, nil
}

// ============================ card networks ============================

func (r *Repository) ListCardNetworks(ctx context.Context) ([]domain.CardNetwork, error) {
	rows, err := r.c.Query(ctx, `
		SELECT id, code, name, status, sort_order
		FROM oikumenea.finance_card_networks WHERE deleted_at IS NULL
		ORDER BY sort_order NULLS LAST, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.CardNetwork
	for rows.Next() {
		var n domain.CardNetwork
		var so pgtype.Int4
		if err := rows.Scan(&n.ID, &n.Code, &n.Name, &n.Status, &so); err != nil {
			return nil, err
		}
		n.SortOrder = intPtr(so)
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertCardNetwork(ctx context.Context, code, name string, sortOrder *int) (domain.CardNetwork, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.finance_card_networks (code, name, sort_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, updated_at = now()
		RETURNING id, code, name, status, sort_order`, code, name, sortOrder)
	var n domain.CardNetwork
	var so pgtype.Int4
	if err := row.Scan(&n.ID, &n.Code, &n.Name, &n.Status, &so); err != nil {
		return domain.CardNetwork{}, mapPGError(err)
	}
	n.SortOrder = intPtr(so)
	return n, nil
}

// ============================ accounts ============================

const accountCols = `id, institution_id, iban_ciphertext, iban_wrapped_dek, key_ref, iban_blind_index,
	currency, account_type_id, status, created_at, updated_at`

func (r *Repository) InsertAccount(ctx context.Context, in domain.AccountInput) (domain.StoredAccount, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.finance_accounts
			(institution_id, iban_ciphertext, iban_wrapped_dek, key_ref, iban_blind_index, currency, account_type_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,'')::uuid)
		RETURNING `+accountCols,
		in.InstitutionID, in.IBANCiphertext, in.IBANWrappedDEK, in.KeyRef, in.IBANBlindIndex,
		in.Currency, in.AccountTypeID)
	return scanAccount(row)
}

func (r *Repository) GetAccount(ctx context.Context, id string) (domain.StoredAccount, error) {
	row := r.c.QueryRow(ctx, `SELECT `+accountCols+`
		FROM oikumenea.finance_accounts WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanAccount(row)
}

func (r *Repository) ListAccounts(ctx context.Context, institutionID, after string, lim int) ([]domain.StoredAccount, error) {
	rows, err := r.c.Query(ctx, `SELECT `+accountCols+`
		FROM oikumenea.finance_accounts
		WHERE deleted_at IS NULL
		  AND ($1 = '' OR institution_id = $1::uuid)
		  AND ($2 = '' OR id > $2::uuid)
		ORDER BY id LIMIT $3`, institutionID, after, lim)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StoredAccount
	for rows.Next() {
		a, err := scanAccountRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccount applies partial changes; RekeyIBAN gates the envelope columns (only set when re-keying).
func (r *Repository) UpdateAccount(ctx context.Context, id string, up domain.AccountUpdate) (domain.StoredAccount, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.finance_accounts SET
			iban_ciphertext  = CASE WHEN $2::boolean THEN $3::bytea ELSE iban_ciphertext END,
			iban_wrapped_dek = CASE WHEN $2::boolean THEN $4::bytea ELSE iban_wrapped_dek END,
			key_ref          = CASE WHEN $2::boolean THEN $5 ELSE key_ref END,
			iban_blind_index = CASE WHEN $2::boolean THEN $6::bytea ELSE iban_blind_index END,
			currency         = CASE WHEN $7::boolean THEN NULLIF($8,'') ELSE currency END,
			account_type_id  = CASE WHEN $9::boolean THEN NULLIF($10,'')::uuid ELSE account_type_id END,
			status           = COALESCE($11, status),
			updated_at       = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+accountCols,
		id,
		up.RekeyIBAN, up.IBANCiphertext, up.IBANWrappedDEK, up.KeyRef, up.IBANBlindIndex,
		up.Currency != nil, derefStr(up.Currency),
		up.AccountTypeID != nil, derefStr(up.AccountTypeID),
		up.Status)
	return scanAccount(row)
}

func (r *Repository) SoftDeleteAccount(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.finance_accounts SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func scanAccount(row pgx.Row) (domain.StoredAccount, error) {
	var a domain.StoredAccount
	var currency, acctType pgtype.Text
	if err := row.Scan(&a.ID, &a.InstitutionID, &a.IBANCiphertext, &a.IBANWrappedDEK, &a.KeyRef,
		&a.IBANBlindIndex, &currency, &acctType, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.StoredAccount{}, mapPGError(err)
	}
	a.Currency, a.AccountTypeID = textVal(currency), textVal(acctType)
	return a, nil
}

func scanAccountRows(rows pgx.Rows) (domain.StoredAccount, error) {
	var a domain.StoredAccount
	var currency, acctType pgtype.Text
	if err := rows.Scan(&a.ID, &a.InstitutionID, &a.IBANCiphertext, &a.IBANWrappedDEK, &a.KeyRef,
		&a.IBANBlindIndex, &currency, &acctType, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.StoredAccount{}, err
	}
	a.Currency, a.AccountTypeID = textVal(currency), textVal(acctType)
	return a, nil
}

// ============================ holders ============================

const holderCols = `id, account_id, holder_kind, holder_id, role, effective_from, effective_to, created_at, updated_at`

func (r *Repository) InsertHolder(ctx context.Context, accountID string, in domain.HolderInput) (domain.AccountHolder, error) {
	role := in.Role
	if role == "" {
		role = domain.RolePrimary
	}
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.finance_account_holders (account_id, holder_kind, holder_id, role)
		VALUES ($1, $2, $3, $4)
		RETURNING `+holderCols, accountID, in.HolderKind, strings.TrimSpace(in.HolderID), role)
	return scanHolder(row)
}

func (r *Repository) GetHolder(ctx context.Context, id string) (domain.AccountHolder, error) {
	row := r.c.QueryRow(ctx, `SELECT `+holderCols+`
		FROM oikumenea.finance_account_holders WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanHolder(row)
}

func (r *Repository) ListHoldersByAccount(ctx context.Context, accountID string) ([]domain.AccountHolder, error) {
	rows, err := r.c.Query(ctx, `SELECT `+holderCols+`
		FROM oikumenea.finance_account_holders WHERE account_id = $1 AND deleted_at IS NULL
		ORDER BY effective_from DESC, created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AccountHolder
	for rows.Next() {
		h, err := scanHolderRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *Repository) EndHolder(ctx context.Context, id string) (domain.AccountHolder, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.finance_account_holders
		SET effective_to = COALESCE(effective_to, now()), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+holderCols, id)
	return scanHolder(row)
}

func scanHolder(row pgx.Row) (domain.AccountHolder, error) {
	var h domain.AccountHolder
	var to pgtype.Timestamptz
	if err := row.Scan(&h.ID, &h.AccountID, &h.HolderKind, &h.HolderID, &h.Role,
		&h.EffectiveFrom, &to, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return domain.AccountHolder{}, mapPGError(err)
	}
	h.EffectiveTo = timePtr(to)
	return h, nil
}

func scanHolderRows(rows pgx.Rows) (domain.AccountHolder, error) {
	var h domain.AccountHolder
	var to pgtype.Timestamptz
	if err := rows.Scan(&h.ID, &h.AccountID, &h.HolderKind, &h.HolderID, &h.Role,
		&h.EffectiveFrom, &to, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return domain.AccountHolder{}, err
	}
	h.EffectiveTo = timePtr(to)
	return h, nil
}

// ============================ cards ============================

const cardCols = `id, account_id, pan_ciphertext, pan_wrapped_dek, key_ref, pan_blind_index, bin, last_four,
	network_id, card_type, expiry_month, expiry_year, cardholder_person_id, status, created_at, updated_at`

func (r *Repository) InsertCard(ctx context.Context, accountID string, in domain.CardInput) (domain.StoredCard, error) {
	row := r.c.QueryRow(ctx, `
		INSERT INTO oikumenea.finance_cards
			(account_id, pan_ciphertext, pan_wrapped_dek, key_ref, pan_blind_index, bin, last_four,
			 network_id, card_type, expiry_month, expiry_year, cardholder_person_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,'')::uuid, $9, $10, $11, NULLIF($12,'')::uuid)
		RETURNING `+cardCols,
		accountID, in.PANCiphertext, in.PANWrappedDEK, in.KeyRef, in.PANBlindIndex, in.BIN, in.LastFour,
		in.NetworkID, in.CardType, in.ExpiryMonth, in.ExpiryYear, in.CardholderPersonID)
	return scanCard(row)
}

func (r *Repository) GetCard(ctx context.Context, id string) (domain.StoredCard, error) {
	row := r.c.QueryRow(ctx, `SELECT `+cardCols+`
		FROM oikumenea.finance_cards WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanCard(row)
}

func (r *Repository) ListCardsByAccount(ctx context.Context, accountID string) ([]domain.StoredCard, error) {
	rows, err := r.c.Query(ctx, `SELECT `+cardCols+`
		FROM oikumenea.finance_cards WHERE account_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StoredCard
	for rows.Next() {
		c, err := scanCardRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateCard(ctx context.Context, id string, up domain.CardUpdate) (domain.StoredCard, error) {
	row := r.c.QueryRow(ctx, `
		UPDATE oikumenea.finance_cards SET
			network_id           = CASE WHEN $2::boolean THEN NULLIF($3,'')::uuid ELSE network_id END,
			card_type            = COALESCE($4, card_type),
			expiry_month         = CASE WHEN $5::boolean THEN $6::int ELSE expiry_month END,
			expiry_year          = CASE WHEN $7::boolean THEN $8::int ELSE expiry_year END,
			cardholder_person_id = CASE WHEN $9::boolean THEN NULLIF($10,'')::uuid ELSE cardholder_person_id END,
			status               = COALESCE($11, status),
			updated_at           = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+cardCols,
		id,
		up.NetworkID != nil, derefStr(up.NetworkID),
		up.CardType,
		up.ExpiryMonth != nil, up.ExpiryMonth,
		up.ExpiryYear != nil, up.ExpiryYear,
		up.CardholderPersonID != nil, derefStr(up.CardholderPersonID),
		up.Status)
	return scanCard(row)
}

func (r *Repository) SoftDeleteCard(ctx context.Context, id string) (int64, error) {
	tag, err := r.c.Exec(ctx, `UPDATE oikumenea.finance_cards SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return 0, mapPGError(err)
	}
	return tag.RowsAffected(), nil
}

func scanCard(row pgx.Row) (domain.StoredCard, error) {
	var c domain.StoredCard
	var bin, last, network, cardholder pgtype.Text
	var em, ey pgtype.Int4
	if err := row.Scan(&c.ID, &c.AccountID, &c.PANCiphertext, &c.PANWrappedDEK, &c.KeyRef, &c.PANBlindIndex,
		&bin, &last, &network, &c.CardType, &em, &ey, &cardholder, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.StoredCard{}, mapPGError(err)
	}
	c.BIN, c.LastFour, c.NetworkID, c.CardholderPersonID = strings.TrimSpace(textVal(bin)), strings.TrimSpace(textVal(last)), textVal(network), textVal(cardholder)
	c.ExpiryMonth, c.ExpiryYear = intPtr(em), intPtr(ey)
	return c, nil
}

func scanCardRows(rows pgx.Rows) (domain.StoredCard, error) {
	var c domain.StoredCard
	var bin, last, network, cardholder pgtype.Text
	var em, ey pgtype.Int4
	if err := rows.Scan(&c.ID, &c.AccountID, &c.PANCiphertext, &c.PANWrappedDEK, &c.KeyRef, &c.PANBlindIndex,
		&bin, &last, &network, &c.CardType, &em, &ey, &cardholder, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.StoredCard{}, err
	}
	c.BIN, c.LastFour, c.NetworkID, c.CardholderPersonID = strings.TrimSpace(textVal(bin)), strings.TrimSpace(textVal(last)), textVal(network), textVal(cardholder)
	c.ExpiryMonth, c.ExpiryYear = intPtr(em), intPtr(ey)
	return c, nil
}

// ============================ person view ============================

func (r *Repository) ListAccountsByPersonHolder(ctx context.Context, personID string) ([]domain.PersonAccount, error) {
	rows, err := r.c.Query(ctx, `
		SELECT a.id, a.institution_id, a.currency, a.account_type_id, h.role, a.status, a.created_at
		FROM oikumenea.finance_account_holders h
		JOIN oikumenea.finance_accounts a ON a.id = h.account_id AND a.deleted_at IS NULL
		WHERE h.holder_kind = 'person' AND h.holder_id = $1 AND h.deleted_at IS NULL AND h.effective_to IS NULL
		ORDER BY a.created_at DESC`, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PersonAccount
	for rows.Next() {
		var p domain.PersonAccount
		var currency, acctType pgtype.Text
		if err := rows.Scan(&p.ID, &p.InstitutionID, &currency, &acctType, &p.Role, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.Currency, p.AccountTypeID = textVal(currency), textVal(acctType)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ErasePersonHoldings crypto-erases every account (and its cards) the person SOLELY holds — the envelope
// (ciphertext + wrapped DEK) is destroyed so the value is unrecoverable, keeping the row + blind index as
// a tombstone (mirrors document.CryptoErasePersonCodes) — then soft-deletes the person's holder edges.
// "Solely held" = every active holder edge of the account is this person. Company-held accounts survive.
func (r *Repository) ErasePersonHoldings(ctx context.Context, personID string) (int64, error) {
	// 1. Sole-held account ids: every active holder is (person, personID). Computed BEFORE we drop edges.
	rows, err := r.c.Query(ctx, `
		SELECT h.account_id
		FROM oikumenea.finance_account_holders h
		WHERE h.deleted_at IS NULL AND h.effective_to IS NULL
		GROUP BY h.account_id
		HAVING bool_and(h.holder_kind = 'person' AND h.holder_id = $1)`, personID)
	if err != nil {
		return 0, err
	}
	var sole []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		sole = append(sole, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var erased int64
	if len(sole) > 0 {
		// 2. Crypto-erase the cards of those accounts (destroy the PAN envelope; keep the row).
		if _, err := r.c.Exec(ctx, `
			UPDATE oikumenea.finance_cards SET pan_ciphertext = NULL, pan_wrapped_dek = NULL, updated_at = now()
			WHERE account_id = ANY($1::uuid[]) AND deleted_at IS NULL`, sole); err != nil {
			return 0, err
		}
		// 3. Crypto-erase the accounts themselves (destroy the IBAN envelope; keep the row + blind index).
		tag, err := r.c.Exec(ctx, `
			UPDATE oikumenea.finance_accounts SET iban_ciphertext = NULL, iban_wrapped_dek = NULL, status = 'closed', updated_at = now()
			WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, sole)
		if err != nil {
			return 0, err
		}
		erased = tag.RowsAffected()
	}

	// 4. Soft-delete the person's holder edges across ALL accounts (joint accounts survive with their
	//    remaining holders; only this person's edge goes).
	if _, err := r.c.Exec(ctx, `
		UPDATE oikumenea.finance_account_holders SET deleted_at = now()
		WHERE holder_kind = 'person' AND holder_id = $1 AND deleted_at IS NULL`, personID); err != nil {
		return 0, err
	}
	return erased, nil
}

// ============================ cross-reference helpers ============================

func (r *Repository) OrgExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.c.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM oikumenea.tenant_organizations WHERE id = $1::uuid AND deleted_at IS NULL)`, id).Scan(&ok)
	return ok, err
}

func (r *Repository) PersonExists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.c.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM oikumenea.person_persons WHERE id = $1::uuid AND deleted_at IS NULL)`, id).Scan(&ok)
	return ok, err
}

func (r *Repository) OrgNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.tenant_organizations WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) AccountTypeNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.finance_account_types WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
}

func (r *Repository) NetworkNamesByIDs(ctx context.Context, ids []string) (map[string]string, error) {
	return r.namesByIDs(ctx, `SELECT id, name FROM oikumenea.finance_card_networks WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`, ids)
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
