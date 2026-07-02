package adapters

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// TranslationRepo is the pgx/sqlc-backed implementation of domain.TranslationStore (D-Pinax + D-i18n,
// M45), bound to a single db.DBTX (the caller's transaction so the writes + audit row commit together).
// It resolves an entity's natural key to the entity_id the read path stores translations under, then
// upserts i18n_translations create-if-absent.
type TranslationRepo struct {
	q *dataimportsql.Queries
}

// NewTranslationRepo binds a translation store to the given command surface.
func NewTranslationRepo(conn db.DBTX) *TranslationRepo {
	return &TranslationRepo{q: dataimportsql.New(conn)}
}

var _ domain.TranslationStore = (*TranslationRepo)(nil)

// Resolve maps an entity's natural key to its i18n entity_id. Code-keyed catalogs (country,
// ethnicity_type) use the key verbatim (that IS their entity_id in the read path); RID-keyed catalogs
// resolve code→RID. rank_* keys are "system/category[/type]/code" paths. An unresolved key returns
// found=false (the record is skipped, resilient to a partly-seeded plane).
func (r *TranslationRepo) Resolve(ctx context.Context, entityType, key string) (string, bool, error) {
	switch entityType {
	case "country", "ethnicity_type":
		return key, key != "", nil
	case "languoid":
		return one(r.q.ResolveLanguoidRID(ctx, key))
	case "writing_system":
		return one(r.q.ResolveWritingSystemByCode(ctx, key))
	case "religion_taxon":
		return one(r.q.ResolveReligionTaxonRID(ctx, key))
	case "color":
		p := split(key, 2)
		if p == nil {
			return "", false, nil
		}
		return one(r.q.ResolveColorRID(ctx, dataimportsql.ResolveColorRIDParams{Domain: p[0], Code: p[1]}))
	case "rank_category":
		p := split(key, 2)
		if p == nil {
			return "", false, nil
		}
		return one(r.q.ResolveRankCategoryRID(ctx, dataimportsql.ResolveRankCategoryRIDParams{SystemCode: p[0], CategoryCode: p[1]}))
	case "rank_type":
		p := split(key, 3)
		if p == nil {
			return "", false, nil
		}
		return one(r.q.ResolveRankTypeRID(ctx, dataimportsql.ResolveRankTypeRIDParams{SystemCode: p[0], CategoryCode: p[1], TypeCode: p[2]}))
	case "rank":
		p := split(key, 3)
		if p == nil {
			return "", false, nil
		}
		return one(r.q.ResolveRankRID(ctx, dataimportsql.ResolveRankRIDParams{SystemCode: p[0], CategoryCode: p[1], RankCode: p[2]}))
	default:
		return "", false, nil
	}
}

// Upsert writes one translation row create-if-absent (ON CONFLICT DO NOTHING in SQL).
func (r *TranslationRepo) Upsert(ctx context.Context, entityType, entityID, field, locale, text string) error {
	return r.q.UpsertTranslationSeed(ctx, dataimportsql.UpsertTranslationSeedParams{
		EntityType: entityType,
		EntityID:   entityID,
		Field:      field,
		Locale:     locale,
		Text:       text,
	})
}

// one maps a sqlc single-row resolver result to (id, found, err): pgx.ErrNoRows → found=false.
func one(id string, err error) (string, bool, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// split parses a "seg0/seg1/…"-joined rank key into exactly n non-empty segments (nil if malformed).
func split(key string, n int) []string {
	parts := strings.Split(key, "/")
	if len(parts) != n {
		return nil
	}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return nil
		}
	}
	return parts
}
