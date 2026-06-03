// Package adapters implements the rank domain ports against infrastructure: the pgx/sqlc repository
// over the oikumenea.rank_* tables. It depends on the database, never the reverse (overview.md).
// Generated sqlc code lives in the ranksql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/rank/adapters/ranksql"
	"github.com/olegamysk/go-oikumenea/internal/rank/domain"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository, bound to a single db.DBTX —
// the pool for reads, or a caller-supplied transaction so a write and its audit row commit together
// (D-Audit).
type Repository struct {
	q *ranksql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: ranksql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

// ---------------------------------------------------------------- categories

func (r *Repository) InsertCategory(ctx context.Context, code, name string, sortOrder *int) (domain.Category, error) {
	row, err := r.q.InsertCategory(ctx, ranksql.InsertCategoryParams{
		Code:      code,
		Name:      name,
		SortOrder: int4Ptr(sortOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Category{}, domain.ErrCodeConflict
		}
		return domain.Category{}, err
	}
	return toCategory(row), nil
}

func (r *Repository) GetCategory(ctx context.Context, id string) (domain.Category, error) {
	row, err := r.q.GetCategory(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrCategoryNotFound
		}
		return domain.Category{}, err
	}
	return toCategory(row), nil
}

func (r *Repository) UpdateCategory(ctx context.Context, id string, patch domain.CategoryPatch) (domain.Category, error) {
	row, err := r.q.UpdateCategory(ctx, ranksql.UpdateCategoryParams{
		ID:        id,
		Name:      textPtr(patch.Name),
		SortOrder: int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrCategoryNotFound
		}
		return domain.Category{}, err
	}
	return toCategory(row), nil
}

func (r *Repository) SoftDeleteCategory(ctx context.Context, id string) error {
	if _, err := r.q.SoftDeleteCategory(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCategoryNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.q.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCategory(row))
	}
	return out, nil
}

func (r *Repository) CountActiveTypes(ctx context.Context, categoryID string) (int, error) {
	n, err := r.q.CountActiveTypesInCategory(ctx, categoryID)
	return int(n), err
}

// ---------------------------------------------------------------- types

func (r *Repository) InsertType(ctx context.Context, categoryID, code, name string, sortOrder *int) (domain.Type, error) {
	row, err := r.q.InsertType(ctx, ranksql.InsertTypeParams{
		CategoryID: categoryID,
		Code:       code,
		Name:       name,
		SortOrder:  int4Ptr(sortOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Type{}, domain.ErrCodeConflict
		}
		return domain.Type{}, err
	}
	return toType(row), nil
}

func (r *Repository) GetType(ctx context.Context, id string) (domain.Type, error) {
	row, err := r.q.GetType(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Type{}, domain.ErrTypeNotFound
		}
		return domain.Type{}, err
	}
	return toType(row), nil
}

func (r *Repository) UpdateType(ctx context.Context, id string, patch domain.TypePatch) (domain.Type, error) {
	row, err := r.q.UpdateType(ctx, ranksql.UpdateTypeParams{
		ID:        id,
		Name:      textPtr(patch.Name),
		SortOrder: int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Type{}, domain.ErrTypeNotFound
		}
		return domain.Type{}, err
	}
	return toType(row), nil
}

func (r *Repository) SoftDeleteType(ctx context.Context, id string) error {
	if _, err := r.q.SoftDeleteType(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTypeNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListTypes(ctx context.Context) ([]domain.Type, error) {
	rows, err := r.q.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Type, 0, len(rows))
	for _, row := range rows {
		out = append(out, toType(row))
	}
	return out, nil
}

func (r *Repository) CountActiveRanks(ctx context.Context, typeID string) (int, error) {
	n, err := r.q.CountActiveRanksInType(ctx, typeID)
	return int(n), err
}

// ---------------------------------------------------------------- ranks

func (r *Repository) InsertRank(ctx context.Context, typeID, code, name string, abbreviation *string, sortOrder *int) (domain.Rank, error) {
	row, err := r.q.InsertRank(ctx, ranksql.InsertRankParams{
		TypeID:       typeID,
		Code:         code,
		Name:         name,
		Abbreviation: textPtr(abbreviation),
		SortOrder:    int4Ptr(sortOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Rank{}, domain.ErrCodeConflict
		}
		return domain.Rank{}, err
	}
	return toRank(row), nil
}

func (r *Repository) GetRank(ctx context.Context, id string) (domain.Rank, error) {
	row, err := r.q.GetRank(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Rank{}, domain.ErrRankNotFound
		}
		return domain.Rank{}, err
	}
	return toRank(row), nil
}

func (r *Repository) UpdateRank(ctx context.Context, id string, patch domain.RankPatch) (domain.Rank, error) {
	row, err := r.q.UpdateRank(ctx, ranksql.UpdateRankParams{
		ID:           id,
		Name:         textPtr(patch.Name),
		Abbreviation: textPtr(patch.Abbreviation),
		SortOrder:    int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Rank{}, domain.ErrRankNotFound
		}
		return domain.Rank{}, err
	}
	return toRank(row), nil
}

func (r *Repository) SoftDeleteRank(ctx context.Context, id string) error {
	if _, err := r.q.SoftDeleteRank(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRankNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListRanks(ctx context.Context) ([]domain.Rank, error) {
	rows, err := r.q.ListRanks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Rank, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRank(row))
	}
	return out, nil
}

// ---------------------------------------------------------------- mapping helpers

func toCategory(row ranksql.OikumeneaRankCategory) domain.Category {
	return domain.Category{ID: row.ID, Code: row.Code, Name: row.Name, SortOrder: int(row.SortOrder)}
}

func toType(row ranksql.OikumeneaRankType) domain.Type {
	return domain.Type{
		ID: row.ID, Code: row.Code, Name: row.Name,
		SortOrder: int(row.SortOrder), CategoryID: row.CategoryID,
	}
}

func toRank(row ranksql.OikumeneaRankRank) domain.Rank {
	return domain.Rank{
		ID: row.ID, Code: row.Code, Name: row.Name,
		Abbreviation: row.Abbreviation.String, // "" when not valid
		SortOrder:    int(row.SortOrder), TypeID: row.TypeID,
	}
}

// isUniqueViolation reports whether err is a Postgres unique-constraint violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func textPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func int4Ptr(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}
