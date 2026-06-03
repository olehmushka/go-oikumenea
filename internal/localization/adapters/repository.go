// Package adapters implements the localization domain ports against infrastructure: the pgx/sqlc
// repository over oikumenea.i18n_locales and oikumenea.i18n_translations. It depends on the
// database, never the reverse (overview.md). Generated sqlc code lives in the localizationsql
// subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/localization/adapters/localizationsql"
	"github.com/olegamysk/go-oikumenea/internal/localization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// Repository is the pgx/sqlc-backed implementation of domain.Repository. It is bound to a single
// db.DBTX — the pool for reads, or a caller-supplied transaction so a write commits together with
// its audit row (D-Audit).
type Repository struct {
	q *localizationsql.Queries
}

// NewRepository binds a repository to the given command surface. A db.DBTX value satisfies the
// interface sqlc generates, so the pool and a pgx.Tx are both accepted.
func NewRepository(conn db.DBTX) *Repository {
	return &Repository{q: localizationsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.Repository = (*Repository)(nil)

func (r *Repository) ListLocales(ctx context.Context) ([]domain.Locale, error) {
	rows, err := r.q.ListLocales(ctx)
	if err != nil {
		return nil, err
	}
	locales := make([]domain.Locale, 0, len(rows))
	for _, row := range rows {
		locales = append(locales, toLocale(row))
	}
	return locales, nil
}

func (r *Repository) GetLocaleByCode(ctx context.Context, code string) (domain.Locale, error) {
	row, err := r.q.GetLocaleByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Locale{}, domain.ErrLocaleNotFound
		}
		return domain.Locale{}, err
	}
	return toLocale(row), nil
}

func (r *Repository) InsertLocale(ctx context.Context, l domain.Locale) (domain.Locale, error) {
	row, err := r.q.InsertLocale(ctx, localizationsql.InsertLocaleParams{
		Code:      l.Code,
		Name:      l.Name,
		Enabled:   l.Enabled,
		IsDefault: l.IsDefault,
		SortOrder: int32(l.SortOrder),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Locale{}, domain.ErrLocaleConflict
		}
		return domain.Locale{}, err
	}
	return toLocale(row), nil
}

func (r *Repository) UpdateLocale(ctx context.Context, code string, patch domain.LocalePatch) (domain.Locale, error) {
	row, err := r.q.UpdateLocale(ctx, localizationsql.UpdateLocaleParams{
		Code:      code,
		Name:      textPtr(patch.Name),
		Enabled:   boolPtr(patch.Enabled),
		IsDefault: boolPtr(patch.IsDefault),
		SortOrder: int4Ptr(patch.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Locale{}, domain.ErrLocaleNotFound
		}
		return domain.Locale{}, err
	}
	return toLocale(row), nil
}

func (r *Repository) ClearDefault(ctx context.Context) error {
	return r.q.ClearDefaultLocales(ctx)
}

func (r *Repository) ExistingLocaleCodes(ctx context.Context, codes []string) ([]string, error) {
	return r.q.ExistingLocaleCodes(ctx, codes)
}

func (r *Repository) GetTranslations(ctx context.Context, entityType, entityID string) ([]domain.Translation, error) {
	rows, err := r.q.GetTranslationsForEntity(ctx, localizationsql.GetTranslationsForEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, err
	}
	return toTranslations(rows), nil
}

func (r *Repository) UpsertTranslations(ctx context.Context, ts []domain.Translation) error {
	for _, t := range ts {
		if err := r.q.UpsertTranslation(ctx, localizationsql.UpsertTranslationParams{
			EntityType: t.EntityType,
			EntityID:   t.EntityID,
			Field:      t.Field,
			Locale:     t.Locale,
			Text:       t.Text,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) TranslationsForBatch(ctx context.Context, entityType string, entityIDs, fields []string) ([]domain.Translation, error) {
	rows, err := r.q.TranslationsForBatch(ctx, localizationsql.TranslationsForBatchParams{
		EntityType: entityType,
		EntityIds:  entityIDs,
		Fields:     fields,
	})
	if err != nil {
		return nil, err
	}
	return toTranslations(rows), nil
}

func toLocale(row localizationsql.OikumeneaI18nLocale) domain.Locale {
	return domain.Locale{
		Code:      row.Code,
		Name:      row.Name,
		Enabled:   row.Enabled,
		IsDefault: row.IsDefault,
		SortOrder: int(row.SortOrder),
	}
}

func toTranslations(rows []localizationsql.OikumeneaI18nTranslation) []domain.Translation {
	ts := make([]domain.Translation, 0, len(rows))
	for _, row := range rows {
		ts = append(ts, domain.Translation{
			EntityType: row.EntityType,
			EntityID:   row.EntityID,
			Field:      row.Field,
			Locale:     row.Locale,
			Text:       row.Text,
		})
	}
	return ts
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

func boolPtr(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func int4Ptr(n *int) pgtype.Int4 {
	if n == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*n), Valid: true}
}
