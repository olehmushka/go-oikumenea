// Package adapters implements the identity-federation domain ports against infrastructure: the
// pgx/sqlc repository over the oikumenea.account_* tables. It depends on the database, never the
// reverse (overview.md). Generated sqlc code lives in the accountsql subpackage and is never
// hand-edited.
package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/adapters/accountsql"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *accountsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: accountsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- accounts

func (r *Repository) InsertAccount(ctx context.Context, a domain.Account) (domain.Account, error) {
	row, err := r.q.InsertAccount(ctx, accountsql.InsertAccountParams{
		PersonID: a.PersonID,
		Email:    text(a.Email),
	})
	if err != nil {
		return domain.Account{}, mapWriteErr(err)
	}
	return toAccount(row), nil
}

func (r *Repository) GetAccount(ctx context.Context, id string) (domain.Account, error) {
	row, err := r.q.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, err
	}
	return toAccount(row), nil
}

func (r *Repository) GetActiveAccountByPerson(ctx context.Context, personID string) (domain.Account, error) {
	row, err := r.q.GetActiveAccountByPerson(ctx, personID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, err
	}
	return toAccount(row), nil
}

func (r *Repository) DisableAccount(ctx context.Context, id string) (domain.Account, error) {
	row, err := r.q.DisableAccount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrAccountNotFound
		}
		return domain.Account{}, err
	}
	return toAccount(row), nil
}

// ---------------------------------------------------------------- external identities

func (r *Repository) InsertIdentity(ctx context.Context, e domain.ExternalIdentity) (domain.ExternalIdentity, error) {
	row, err := r.q.InsertIdentity(ctx, accountsql.InsertIdentityParams{
		AccountID: e.AccountID,
		Issuer:    e.Issuer,
		Subject:   e.Subject,
	})
	if err != nil {
		return domain.ExternalIdentity{}, mapWriteErr(err)
	}
	return toIdentity(row), nil
}

func (r *Repository) GetIdentity(ctx context.Context, id string) (domain.ExternalIdentity, error) {
	row, err := r.q.GetIdentity(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ExternalIdentity{}, domain.ErrIdentityNotFound
		}
		return domain.ExternalIdentity{}, err
	}
	return toIdentity(row), nil
}

func (r *Repository) DeleteIdentity(ctx context.Context, accountID, id string) error {
	rows, err := r.q.DeleteIdentity(ctx, accountsql.DeleteIdentityParams{ID: id, AccountID: accountID})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrIdentityNotFound
	}
	return nil
}

func (r *Repository) ListIdentitiesByAccount(ctx context.Context, accountID string) ([]domain.ExternalIdentity, error) {
	rows, err := r.q.ListIdentitiesByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ExternalIdentity, 0, len(rows))
	for _, row := range rows {
		out = append(out, toIdentity(row))
	}
	return out, nil
}

func (r *Repository) CountActiveIdentities(ctx context.Context, accountID string) (int, error) {
	n, err := r.q.CountActiveIdentities(ctx, accountID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *Repository) ResolveBySubject(ctx context.Context, issuer, subject string) (domain.Resolution, error) {
	row, err := r.q.ResolveBySubject(ctx, accountsql.ResolveBySubjectParams{Issuer: issuer, Subject: subject})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Resolution{}, domain.ErrIdentityNotFound
		}
		return domain.Resolution{}, err
	}
	return domain.Resolution{PersonID: row.PersonID, AccountID: row.AccountID, Email: row.Email.String}, nil
}

// ---------------------------------------------------------------- mapping helpers

func toAccount(r accountsql.OikumeneaAccountAccount) domain.Account {
	return domain.Account{
		ID:        r.ID,
		PersonID:  r.PersonID,
		Email:     r.Email.String,
		Status:    domain.AccountStatus(r.Status),
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}

func toIdentity(r accountsql.OikumeneaAccountExternalIdentity) domain.ExternalIdentity {
	return domain.ExternalIdentity{
		ID:        r.ID,
		AccountID: r.AccountID,
		Issuer:    r.Issuer,
		Subject:   r.Subject,
		CreatedAt: r.CreatedAt.Time,
	}
}

// mapWriteErr translates Postgres constraint violations into the module's domain sentinels: the
// global (issuer, subject) unique index -> already-linked; the per-person active-account index ->
// account conflict; the person FK -> unknown person.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		switch {
		case strings.Contains(name, "issuer_subject"):
			return domain.ErrIdentityConflict
		case strings.Contains(name, "person_active"):
			return domain.ErrAccountConflict
		case strings.Contains(name, "email_active"):
			return domain.ErrAccountConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "person"):
			return domain.ErrUnknownPerson
		case strings.Contains(name, "account"):
			return domain.ErrIdentityInvalid
		}
	}
	return err
}

func text(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
