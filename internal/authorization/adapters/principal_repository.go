// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/olehmushka/go-oikumenea/internal/authorization/adapters/authzsql"
	"github.com/olehmushka/go-oikumenea/internal/authorization/domain"
)

// The machine-subject authority plane over oikumenea.authz_principal_grants (M51 /
// D-ServiceIdentities). Flat rows: no target unit, no scope, no graph — a service principal has no
// unit reach, so the PDP is never consulted for it.

// compile-time assertion that the adapter satisfies the grant port.
var _ domain.PrincipalRepository = (*Repository)(nil)

func (r *Repository) InsertPrincipalGrant(ctx context.Context, in domain.PrincipalGrantInput) (domain.PrincipalGrant, error) {
	row, err := r.q.InsertPrincipalGrant(ctx, authzsql.InsertPrincipalGrantParams{
		PrincipalID:    in.PrincipalID,
		PermissionCode: string(in.Permission),
		OrgID:          text(in.OrgID),
		GrantedBy:      text(in.GrantedBy),
	})
	if err != nil {
		return domain.PrincipalGrant{}, mapPrincipalGrantWriteErr(err)
	}
	return principalGrantFrom(row), nil
}

func (r *Repository) GetPrincipalGrant(ctx context.Context, id string) (domain.PrincipalGrant, error) {
	row, err := r.q.GetPrincipalGrant(ctx, id)
	if err != nil {
		return domain.PrincipalGrant{}, mapReadErr(err, domain.ErrPrincipalGrantNotFound)
	}
	return principalGrantFrom(row), nil
}

func (r *Repository) RevokePrincipalGrant(ctx context.Context, id, revokedBy string) (domain.PrincipalGrant, error) {
	row, err := r.q.RevokePrincipalGrant(ctx, authzsql.RevokePrincipalGrantParams{ID: id, RevokedBy: text(revokedBy)})
	if err != nil {
		// The query excludes already-revoked rows, so a second revoke is no-rows -> not-found.
		return domain.PrincipalGrant{}, mapReadErr(err, domain.ErrPrincipalGrantNotFound)
	}
	return principalGrantFrom(row), nil
}

func (r *Repository) ListPrincipalGrants(ctx context.Context, principalID string) ([]domain.PrincipalGrant, error) {
	rows, err := r.q.ListPrincipalGrants(ctx, principalID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PrincipalGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, principalGrantFrom(row))
	}
	return out, nil
}

func (r *Repository) ActiveGrantsForPrincipal(ctx context.Context, principalID string) ([]domain.PrincipalGrant, error) {
	rows, err := r.q.ActiveGrantsForPrincipal(ctx, principalID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PrincipalGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PrincipalGrant{
			PrincipalID: principalID,
			Permission:  domain.Permission(row.PermissionCode),
			OrgID:       row.OrgID.String,
		})
	}
	return out, nil
}

func principalGrantFrom(r authzsql.OikumeneaAuthzPrincipalGrant) domain.PrincipalGrant {
	g := domain.PrincipalGrant{
		ID:          r.ID,
		PrincipalID: r.PrincipalID,
		Permission:  domain.Permission(r.PermissionCode),
		OrgID:       r.OrgID.String,
		GrantedBy:   r.GrantedBy.String,
		GrantedAt:   r.GrantedAt.Time,
		RevokedBy:   r.RevokedBy.String,
	}
	if r.RevokedAt.Valid {
		t := r.RevokedAt.Time
		g.RevokedAt = &t
	}
	return g
}

// mapPrincipalGrantWriteErr translates the grant plane's constraint violations. Both partial unique
// indexes (instance-wide and org-scoped) mean the same thing to a caller: this grant already exists.
func mapPrincipalGrantWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	name := pgErr.ConstraintName
	switch pgErr.Code {
	case "23505": // unique_violation
		if strings.Contains(name, "principal_grants") {
			return domain.ErrPrincipalGrantConflict
		}
	case "23503": // foreign_key_violation
		switch {
		case strings.Contains(name, "principal_id"):
			return domain.ErrUnknownPrincipal
		case strings.Contains(name, "org_id"):
			return domain.ErrUnknownOrganization
		}
	}
	return err
}
