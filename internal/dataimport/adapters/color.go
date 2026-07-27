// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// ColorRepo is the pgx/sqlc-backed implementation of domain.ColorStore (D-Color + D-Pinax, M45), bound
// to a single db.DBTX (the caller's transaction so the upsert + audit row commit together — D-Audit).
type ColorRepo struct {
	q *dataimportsql.Queries
}

// NewColorRepo binds a color store to the given command surface.
func NewColorRepo(conn db.DBTX) *ColorRepo {
	return &ColorRepo{q: dataimportsql.New(conn)}
}

var _ domain.ColorStore = (*ColorRepo)(nil)

// Get returns the color's current name + hex (idempotency comparands) and whether the row exists.
func (r *ColorRepo) Get(ctx context.Context, domainKey, code string) (string, string, bool, error) {
	row, err := r.q.GetColor(ctx, dataimportsql.GetColorParams{Domain: domainKey, Code: code})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return row.Name, row.Hex.String, true, nil
}

// Insert adds a palette color with origin='seeded'.
func (r *ColorRepo) Insert(ctx context.Context, c domain.Color) error {
	return r.q.InsertColorImport(ctx, dataimportsql.InsertColorImportParams{
		Domain:    c.Domain,
		Code:      c.Code,
		Name:      c.Name,
		Hex:       c.Hex,
		SortOrder: int4(c.SortOrder),
	})
}

// UpdateImport updates a color's name/hex/sort_order (called only when name or hex changed).
func (r *ColorRepo) UpdateImport(ctx context.Context, c domain.Color) error {
	return r.q.UpdateColorImport(ctx, dataimportsql.UpdateColorImportParams{
		Domain:    c.Domain,
		Code:      c.Code,
		Name:      c.Name,
		Hex:       c.Hex,
		SortOrder: int4(c.SortOrder),
	})
}
