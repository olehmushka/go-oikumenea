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

// ---------------------------------------------------------------- systems

func (r *Repository) InsertSystem(ctx context.Context, code, name string, sortOrder *int, country *string) (domain.System, error) {
	row, err := r.q.InsertSystem(ctx, ranksql.InsertSystemParams{
		Code:      code,
		Name:      name,
		SortOrder: int4Ptr(sortOrder),
		Country:   textPtr(country),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.System{}, domain.ErrCodeConflict
		}
		return domain.System{}, err
	}
	return toSystem(row), nil
}

func (r *Repository) GetSystem(ctx context.Context, id string) (domain.System, error) {
	row, err := r.q.GetSystem(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.System{}, domain.ErrSystemNotFound
		}
		return domain.System{}, err
	}
	return toSystem(row), nil
}

func (r *Repository) GetSystemByCode(ctx context.Context, code string) (domain.System, error) {
	row, err := r.q.GetSystemByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.System{}, domain.ErrSystemNotFound
		}
		return domain.System{}, err
	}
	return toSystem(row), nil
}

func (r *Repository) UpdateSystem(ctx context.Context, id string, patch domain.SystemPatch) (domain.System, error) {
	row, err := r.q.UpdateSystem(ctx, ranksql.UpdateSystemParams{
		ID:        id,
		Name:      textPtr(patch.Name),
		SortOrder: int4Ptr(patch.SortOrder),
		Country:   textPtr(patch.Country),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.System{}, domain.ErrSystemNotFound
		}
		return domain.System{}, err
	}
	return toSystem(row), nil
}

func (r *Repository) SoftDeleteSystem(ctx context.Context, id string) error {
	if _, err := r.q.SoftDeleteSystem(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrSystemNotFound
		}
		return err
	}
	return nil
}

func (r *Repository) ListSystems(ctx context.Context) ([]domain.System, error) {
	rows, err := r.q.ListSystems(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.System, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSystem(row))
	}
	return out, nil
}

func (r *Repository) CountActiveCategories(ctx context.Context, systemID string) (int, error) {
	n, err := r.q.CountActiveCategoriesInSystem(ctx, systemID)
	return int(n), err
}

// ---------------------------------------------------------------- grades

func (r *Repository) ListGrades(ctx context.Context) ([]domain.Grade, error) {
	rows, err := r.q.ListGrades(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Grade, 0, len(rows))
	for _, row := range rows {
		out = append(out, toGrade(row))
	}
	return out, nil
}

func (r *Repository) GetGradeByCode(ctx context.Context, code string) (domain.Grade, error) {
	row, err := r.q.GetGrade(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Grade{}, domain.ErrGradeNotFound
		}
		return domain.Grade{}, err
	}
	return toGrade(row), nil
}

// ---------------------------------------------------------------- categories

func (r *Repository) InsertCategory(ctx context.Context, systemID, code, name string, sortOrder *int) (domain.Category, error) {
	row, err := r.q.InsertCategory(ctx, ranksql.InsertCategoryParams{
		SystemID:  systemID,
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

func (r *Repository) GetCategoryByCode(ctx context.Context, systemID, code string) (domain.Category, error) {
	row, err := r.q.GetCategoryByCodeInSystem(ctx, ranksql.GetCategoryByCodeInSystemParams{SystemID: systemID, Code: code})
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

func (r *Repository) InsertType(ctx context.Context, categoryID string, parentTypeID *string, code, name string, sortOrder *int) (domain.Type, error) {
	row, err := r.q.InsertType(ctx, ranksql.InsertTypeParams{
		CategoryID:   categoryID,
		ParentTypeID: textPtr(parentTypeID),
		Code:         code,
		Name:         name,
		SortOrder:    int4Ptr(sortOrder),
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

func (r *Repository) GetTypeByCode(ctx context.Context, categoryID string, parentTypeID *string, code string) (domain.Type, error) {
	row, err := r.q.GetTypeByCodeInParent(ctx, ranksql.GetTypeByCodeInParentParams{
		CategoryID:   categoryID,
		ParentTypeID: textPtr(parentTypeID),
		Code:         code,
	})
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

func (r *Repository) CountActiveChildTypes(ctx context.Context, typeID string) (int, error) {
	n, err := r.q.CountActiveChildTypes(ctx, pgtype.Text{String: typeID, Valid: true})
	return int(n), err
}

// ---------------------------------------------------------------- ranks

func (r *Repository) InsertRank(ctx context.Context, typeID, code, name string, abbreviation, gradeCode *string, sortOrder *int) (domain.Rank, error) {
	row, err := r.q.InsertRank(ctx, ranksql.InsertRankParams{
		TypeID:       typeID,
		Code:         code,
		Name:         name,
		Abbreviation: textPtr(abbreviation),
		GradeCode:    textPtr(gradeCode),
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

func (r *Repository) GetRankByCode(ctx context.Context, typeID, code string) (domain.Rank, error) {
	row, err := r.q.GetRankByCodeInType(ctx, ranksql.GetRankByCodeInTypeParams{TypeID: typeID, Code: code})
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
		GradeCode:    textPtr(patch.GradeCode),
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

func toSystem(row ranksql.OikumeneaRankSystem) domain.System {
	return domain.System{
		ID: row.ID, Code: row.Code, Name: row.Name, SortOrder: int(row.SortOrder),
		Country: row.Country.String, // "" when NULL (supranational)
	}
}

func toGrade(row ranksql.OikumeneaRankGrade) domain.Grade {
	return domain.Grade{Code: row.Code, Tier: domain.Tier(row.Tier), Ordinal: int(row.Ordinal), Name: row.Name}
}

func toCategory(row ranksql.OikumeneaRankCategory) domain.Category {
	return domain.Category{ID: row.ID, Code: row.Code, Name: row.Name, SortOrder: int(row.SortOrder), SystemID: row.SystemID}
}

func toType(row ranksql.OikumeneaRankType) domain.Type {
	return domain.Type{
		ID: row.ID, Code: row.Code, Name: row.Name,
		SortOrder: int(row.SortOrder), SystemID: row.SystemID, CategoryID: row.CategoryID,
		ParentTypeID: row.ParentTypeID.String, // "" when a root type (NULL)
	}
}

func toRank(row ranksql.OikumeneaRankRank) domain.Rank {
	return domain.Rank{
		ID: row.ID, Code: row.Code, Name: row.Name,
		Abbreviation: row.Abbreviation.String, // "" when not valid
		GradeCode:    row.GradeCode.String,    // "" when no standardized grade
		SortOrder:    int(row.SortOrder), SystemID: row.SystemID, TypeID: row.TypeID,
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
