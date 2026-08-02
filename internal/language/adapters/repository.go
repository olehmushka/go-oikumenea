// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters implements the language domain ports against infrastructure: the pgx/sqlc repository
// over oikumenea.language_languoids + oikumenea.writing_systems. It depends on the database, never the
// reverse (overview.md). Generated sqlc code lives in the languagesql subpackage and is never
// hand-edited.
package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/language/adapters/languagesql"
	"github.com/olegamysk/go-oikumenea/internal/language/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX
// (the pool for reads).
type Repository struct {
	q *languagesql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX satisfies the interface sqlc
// generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: languagesql.New(conn)}
}

var _ domain.Repository = (*Repository)(nil)

// ListLanguoids returns languoids matching the filter, in code order. A non-empty text query routes to
// the dedicated trigram SearchLanguoids (review R-21 / D-PersonSearch generalized) so the name/code
// match stays a GIN bitmap scan; the empty case is the plain keyset list. The two queries share the
// projection, so their rows are convertible and map through the one loop below.
func (r *Repository) ListLanguoids(ctx context.Context, f domain.Filter) ([]domain.Languoid, error) {
	var rows []languagesql.ListLanguoidsRow
	if q := strings.TrimSpace(f.Query); q != "" {
		found, err := r.q.SearchLanguoids(ctx, languagesql.SearchLanguoidsParams{
			Level:     textPtr(f.Level),
			Family:    textPtr(f.Family),
			Macroarea: textPtr(f.Macroarea),
			Status:    textPtr(f.Status),
			Parent:    f.Parent,
			TopLevel:  f.TopLevel,
			Q:         q,
			After:     textPtr(strPtrOrNil(f.After)),
			Lim:       int32(f.Limit),
		})
		if err != nil {
			return nil, err
		}
		rows = make([]languagesql.ListLanguoidsRow, len(found))
		for i, row := range found {
			rows[i] = languagesql.ListLanguoidsRow(row)
		}
	} else {
		var err error
		if rows, err = r.q.ListLanguoids(ctx, languagesql.ListLanguoidsParams{
			Level:     textPtr(f.Level),
			Family:    textPtr(f.Family),
			Macroarea: textPtr(f.Macroarea),
			Status:    textPtr(f.Status),
			Parent:    f.Parent,
			TopLevel:  f.TopLevel,
			After:     textPtr(strPtrOrNil(f.After)),
			Lim:       int32(f.Limit),
		}); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Languoid, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Languoid{
			ID:          row.ID,
			Code:        row.Code,
			Level:       row.Level,
			Name:        row.Name,
			ParentID:    row.ParentID.String,
			HasChildren: row.HasChildren,
			FamilyCode:  row.FamilyCode.String,
			ISO639_3:    row.Iso6393.String,
			Macroarea:   row.Macroarea.String,
			Status:      row.Status,
		})
	}
	return out, nil
}

// GetLanguoid returns one languoid by its RID (found=false when absent).
func (r *Repository) GetLanguoid(ctx context.Context, id string) (domain.Languoid, bool, error) {
	row, err := r.q.GetLanguoid(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Languoid{}, false, nil
	}
	if err != nil {
		return domain.Languoid{}, false, err
	}
	return domain.Languoid{
		ID:          row.ID,
		Code:        row.Code,
		Level:       row.Level,
		Name:        row.Name,
		ParentID:    row.ParentID.String,
		HasChildren: row.HasChildren,
		FamilyCode:  row.FamilyCode.String,
		ISO639_3:    row.Iso6393.String,
		Macroarea:   row.Macroarea.String,
		Status:      row.Status,
	}, true, nil
}

// ListWritingSystems returns the ISO-15924 writing systems in code order.
func (r *Repository) ListWritingSystems(ctx context.Context) ([]domain.WritingSystem, error) {
	rows, err := r.q.ListWritingSystems(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.WritingSystem, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.WritingSystem{
			ID:         row.ID,
			Code:       row.Code,
			Name:       row.Name,
			ScriptType: row.ScriptType.String,
		})
	}
	return out, nil
}
