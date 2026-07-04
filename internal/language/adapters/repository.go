// Package adapters implements the language domain ports against infrastructure: the pgx/sqlc repository
// over oikumenea.language_languoids + oikumenea.writing_systems. It depends on the database, never the
// reverse (overview.md). Generated sqlc code lives in the languagesql subpackage and is never
// hand-edited.
package adapters

import (
	"context"
	"errors"

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

// ListLanguoids returns languoids matching the filter, in code order.
func (r *Repository) ListLanguoids(ctx context.Context, f domain.Filter) ([]domain.Languoid, error) {
	rows, err := r.q.ListLanguoids(ctx, languagesql.ListLanguoidsParams{
		Level:    f.Level,
		Family:   f.Family,
		Parent:   f.Parent,
		TopLevel: f.TopLevel,
		Q:        f.Query,
		After:    f.After,
		Lim:      int32(f.Limit),
	})
	if err != nil {
		return nil, err
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
