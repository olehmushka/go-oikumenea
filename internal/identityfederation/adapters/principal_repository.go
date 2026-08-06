// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/olehmushka/go-oikumenea/internal/identityfederation/adapters/accountsql"
	"github.com/olehmushka/go-oikumenea/internal/identityfederation/domain"
)

// The service-principal registry side of the adapter (M51 / D-ServiceIdentities): the machine
// counterpart of the account/external-identity tables, over oikumenea.account_service_principals.
// Same Repository value, same db.DBTX binding — so a registration and its audit row commit together.

// compile-time assertion that the adapter satisfies the registry port.
var _ domain.PrincipalRepository = (*Repository)(nil)

func (r *Repository) InsertPrincipal(ctx context.Context, p domain.ServicePrincipal) (domain.ServicePrincipal, error) {
	row, err := r.q.InsertPrincipal(ctx, accountsql.InsertPrincipalParams{
		Code:        p.Code,
		Name:        p.Name,
		Description: text(p.Description),
		Issuer:      p.Issuer,
		Subject:     p.Subject,
		ClientID:    text(p.ClientID),
	})
	if err != nil {
		return domain.ServicePrincipal{}, mapPrincipalWriteErr(err)
	}
	return toPrincipal(row), nil
}

func (r *Repository) GetPrincipal(ctx context.Context, id string) (domain.ServicePrincipal, error) {
	row, err := r.q.GetPrincipal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ServicePrincipal{}, domain.ErrPrincipalNotFound
		}
		return domain.ServicePrincipal{}, err
	}
	return toPrincipal(row), nil
}

func (r *Repository) GetPrincipalByCode(ctx context.Context, code string) (domain.ServicePrincipal, error) {
	row, err := r.q.GetPrincipalByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ServicePrincipal{}, domain.ErrPrincipalNotFound
		}
		return domain.ServicePrincipal{}, err
	}
	return toPrincipal(row), nil
}

func (r *Repository) UpdatePrincipal(ctx context.Context, p domain.ServicePrincipal) (domain.ServicePrincipal, error) {
	row, err := r.q.UpdatePrincipal(ctx, accountsql.UpdatePrincipalParams{
		ID:          p.ID,
		Name:        p.Name,
		Description: text(p.Description),
		ClientID:    text(p.ClientID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ServicePrincipal{}, domain.ErrPrincipalNotFound
		}
		return domain.ServicePrincipal{}, mapPrincipalWriteErr(err)
	}
	return toPrincipal(row), nil
}

func (r *Repository) SetPrincipalStatus(ctx context.Context, id string, status domain.PrincipalStatus) (domain.ServicePrincipal, error) {
	row, err := r.q.SetPrincipalStatus(ctx, accountsql.SetPrincipalStatusParams{ID: id, Status: string(status)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ServicePrincipal{}, domain.ErrPrincipalNotFound
		}
		return domain.ServicePrincipal{}, err
	}
	return toPrincipal(row), nil
}

func (r *Repository) ListPrincipals(ctx context.Context, afterID string, limit int) ([]domain.ServicePrincipal, error) {
	rows, err := r.q.ListPrincipals(ctx, accountsql.ListPrincipalsParams{
		After:    afterID,
		RowLimit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.ServicePrincipal, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPrincipal(row))
	}
	return out, nil
}

func (r *Repository) ResolvePrincipalBySubject(ctx context.Context, issuer, subject string) (domain.PrincipalResolution, error) {
	row, err := r.q.ResolvePrincipalBySubject(ctx, accountsql.ResolvePrincipalBySubjectParams{Issuer: issuer, Subject: subject})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PrincipalResolution{}, domain.ErrPrincipalNotFound
		}
		return domain.PrincipalResolution{}, err
	}
	return domain.PrincipalResolution{PrincipalID: row.ID, Code: row.Code, ClientID: row.ClientID.String}, nil
}

func (r *Repository) PrincipalIsActive(ctx context.Context, id string) (bool, error) {
	return r.q.PrincipalIsActive(ctx, id)
}

// mapPrincipalWriteErr translates the registry's constraint violations into domain sentinels. Both
// the active-unique indexes AND the symmetric collision triggers (which RAISE unique_violation with
// an explicit CONSTRAINT name) fold into ErrPrincipalConflict: from the caller's side "this code is
// taken", "this (issuer, subject) is another principal", and "this (issuer, subject) is a person
// identity" are the same 409.
func mapPrincipalWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	if pgErr.Code == "23505" { // unique_violation
		name := pgErr.ConstraintName
		switch {
		case strings.Contains(name, "service_principals_code_active"),
			strings.Contains(name, "service_principals_identity_active"),
			strings.Contains(name, "identity_collision"):
			return domain.ErrPrincipalConflict
		}
	}
	return err
}

func toPrincipal(r accountsql.OikumeneaAccountServicePrincipal) domain.ServicePrincipal {
	return domain.ServicePrincipal{
		ID:          r.ID,
		Code:        r.Code,
		Name:        r.Name,
		Description: r.Description.String,
		Issuer:      r.Issuer,
		Subject:     r.Subject,
		ClientID:    r.ClientID.String,
		Status:      domain.PrincipalStatus(r.Status),
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}
