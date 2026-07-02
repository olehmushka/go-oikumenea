package adapters

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// ReligionRepo is the pgx/sqlc-backed implementation of domain.ReligionStore (D-Religion + D-Pinax,
// M45), bound to a single db.DBTX (the caller's transaction so the upsert + audit row commit together —
// D-Audit). Mirrors EthnicityRepo.
type ReligionRepo struct {
	q *dataimportsql.Queries
}

// NewReligionRepo binds a religion store to the given command surface.
func NewReligionRepo(conn db.DBTX) *ReligionRepo {
	return &ReligionRepo{q: dataimportsql.New(conn)}
}

var _ domain.ReligionStore = (*ReligionRepo)(nil)

// GetVersion returns the taxon's stored source_version (idempotency key) and whether it exists.
func (r *ReligionRepo) GetVersion(ctx context.Context, code string) (string, bool, error) {
	v, err := r.q.GetReligionVersion(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, true, nil
}

// Insert adds a taxon; rank + parent codes are resolved to RIDs in SQL, provenance stamped.
func (r *ReligionRepo) Insert(ctx context.Context, t domain.Religion, prov domain.Provenance) error {
	return r.q.InsertReligionImport(ctx, dataimportsql.InsertReligionImportParams{
		Code:          t.Code,
		Name:          t.Name,
		Description:   t.Description,
		RankCode:      t.RankCode,
		ParentCode:    t.Parent,
		WikidataID:    t.WikidataID,
		Icon:          t.Icon,
		SortOrder:     int4(t.SortOrder),
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
	})
}

// UpdateImport rewrites a taxon (called when the source edition changed).
func (r *ReligionRepo) UpdateImport(ctx context.Context, t domain.Religion, prov domain.Provenance) error {
	return r.q.UpdateReligionImport(ctx, dataimportsql.UpdateReligionImportParams{
		Code:          t.Code,
		Name:          t.Name,
		Description:   t.Description,
		RankCode:      t.RankCode,
		ParentCode:    t.Parent,
		WikidataID:    t.WikidataID,
		Icon:          t.Icon,
		SortOrder:     int4(t.SortOrder),
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
	})
}

// ReplaceClassifications resets a taxon's theism-classification ties (unresolved codes silently dropped).
func (r *ReligionRepo) ReplaceClassifications(ctx context.Context, code string, classificationCodes []string) error {
	if err := r.q.DeleteReligionClassifications(ctx, code); err != nil {
		return err
	}
	for _, cc := range classificationCodes {
		if cc == "" {
			continue
		}
		if err := r.q.InsertReligionClassification(ctx, dataimportsql.InsertReligionClassificationParams{
			Code:               code,
			ClassificationCode: cc,
		}); err != nil {
			return err
		}
	}
	return nil
}

// RebuildClosure recomputes the transitive closure and re-derives each taxon's denormalized root
// religion_id (run once at the end of a religion-scheme import).
func (r *ReligionRepo) RebuildClosure(ctx context.Context) error {
	// Clear then rebuild as two statements (same import tx) — a single DELETE+INSERT CTE would
	// self-conflict on the closure PK when re-importing over an existing closure.
	if err := r.q.ClearReligionClosure(ctx); err != nil {
		return err
	}
	if err := r.q.RebuildReligionClosure(ctx); err != nil {
		return err
	}
	return r.q.DeriveReligionRoots(ctx)
}
