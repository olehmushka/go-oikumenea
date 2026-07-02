// Package adapters implements the data-import domain ports against infrastructure: the pgx/sqlc
// upsert over oikumenea.geo_countries (M16 first catalog). It depends on the database, never the
// reverse. Generated sqlc code lives in the dataimportsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/adapters/dataimportsql"
	"github.com/olegamysk/go-oikumenea/internal/dataimport/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// GeoCountryRepo is the pgx/sqlc-backed implementation of domain.GeoCountryStore, bound to a single
// db.DBTX — the pool for reads, or a caller-supplied transaction so the upsert and its audit row
// commit together (D-Audit).
type GeoCountryRepo struct {
	q *dataimportsql.Queries
}

// NewGeoCountryRepo binds a store to the given command surface. A db.DBTX satisfies the interface sqlc
// generates, so the pool and a pgx.Tx are both accepted.
func NewGeoCountryRepo(conn db.DBTX) *GeoCountryRepo {
	return &GeoCountryRepo{q: dataimportsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.GeoCountryStore = (*GeoCountryRepo)(nil)

// GetName returns the country's current name and whether the row exists.
func (r *GeoCountryRepo) GetName(ctx context.Context, code string) (string, bool, error) {
	name, err := r.q.GetGeoCountryName(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return name, true, nil
}

// Insert adds a country row with import provenance.
func (r *GeoCountryRepo) Insert(ctx context.Context, code, name string, prov domain.Provenance) error {
	return r.q.InsertGeoCountryImport(ctx, dataimportsql.InsertGeoCountryImportParams{
		Code:          code,
		Name:          name,
		Source:        text(prov.Source),
		SourceVersion: text(prov.SourceVersion),
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport updates a country's name + provenance (called only when the name changed).
func (r *GeoCountryRepo) UpdateImport(ctx context.Context, code, name string, prov domain.Provenance) error {
	return r.q.UpdateGeoCountryImport(ctx, dataimportsql.UpdateGeoCountryImportParams{
		Code:          code,
		Name:          name,
		Source:        text(prov.Source),
		SourceVersion: text(prov.SourceVersion),
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// Enrich fills the pinax country reference columns fill-if-empty (D-Pinax, M45): iso_a3, numeric_code,
// centroid (from lat/lng), and color_id (resolved from the domain='country' palette code). Every column
// is COALESCE(col, new) in SQL, so a value already present is never overwritten.
func (r *GeoCountryRepo) Enrich(ctx context.Context, code string, e domain.GeoCountryEnrichment) error {
	return r.q.EnrichGeoCountryFillEmpty(ctx, dataimportsql.EnrichGeoCountryFillEmptyParams{
		Code:        code,
		IsoA3:       e.ISOA3,
		NumericCode: e.NumericCode,
		Geometry:    e.GeometryJSON,
		Latitude:    f8(e.Latitude),
		Longitude:   f8(e.Longitude),
		ColorCode:   e.ColorCode,
	})
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// int4 maps an optional int to a pgtype.Int4 (nil → NULL) for the seeded catalogs' sort_order.
func int4(p *int) pgtype.Int4 {
	if p == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*p), Valid: true}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()} }

// GeoPlaceRepo is the pgx/sqlc-backed implementation of domain.GeoPlaceStore (D-GeoPlaces), bound to a
// single db.DBTX (the caller's transaction so the upsert + audit row commit together — D-Audit).
type GeoPlaceRepo struct {
	q *dataimportsql.Queries
}

// NewGeoPlaceRepo binds a geo-places store to the given command surface.
func NewGeoPlaceRepo(conn db.DBTX) *GeoPlaceRepo {
	return &GeoPlaceRepo{q: dataimportsql.New(conn)}
}

// compile-time assertion that the adapter satisfies the domain port.
var _ domain.GeoPlaceStore = (*GeoPlaceRepo)(nil)

// GetVersion returns the place's stored source_version (the idempotency key) and whether the row
// exists. A NULL stored version reads back as "" (always treated as stale, so it re-imports).
func (r *GeoPlaceRepo) GetVersion(ctx context.Context, wofID int64) (string, bool, error) {
	v, err := r.q.GetGeoPlaceVersion(ctx, wofID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, true, nil
}

// Insert adds a gazetteer row; geometry is materialized from GeoJSON, provenance stamped.
func (r *GeoPlaceRepo) Insert(ctx context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	return r.q.InsertGeoPlaceImport(ctx, dataimportsql.InsertGeoPlaceImportParams{
		WofID:         p.WofID,
		Placetype:     p.Placetype,
		ParentID:      deref(p.ParentID),
		CountryCode:   p.CountryCode,
		Name:          p.Name,
		Population:    deref(p.Population),
		Hierarchy:     p.Hierarchy,
		Concordances:  p.Concordances,
		Status:        p.Status,
		Geometry:      p.GeometryJSON,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport rewrites a gazetteer row (called when the source edition changed).
func (r *GeoPlaceRepo) UpdateImport(ctx context.Context, p domain.GeoPlace, prov domain.Provenance) error {
	return r.q.UpdateGeoPlaceImport(ctx, dataimportsql.UpdateGeoPlaceImportParams{
		WofID:         p.WofID,
		Placetype:     p.Placetype,
		ParentID:      deref(p.ParentID),
		CountryCode:   p.CountryCode,
		Name:          p.Name,
		Population:    deref(p.Population),
		Hierarchy:     p.Hierarchy,
		Concordances:  p.Concordances,
		Status:        p.Status,
		Geometry:      p.GeometryJSON,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// EnrichCountry mirrors a country place's wof_id + geometry onto its geo_countries row (D-GeoPlaces).
func (r *GeoPlaceRepo) EnrichCountry(ctx context.Context, p domain.GeoPlace, _ domain.Provenance) error {
	return r.q.EnrichGeoCountryFromWOF(ctx, dataimportsql.EnrichGeoCountryFromWOFParams{
		Code:        p.CountryCode,
		WofID:       p.WofID,
		Geometry:    p.GeometryJSON,
		IsoA3:       p.ISOA3,
		NumericCode: p.NumericCode,
	})
}

// deref returns the pointed-to int64 or 0 (the absent sentinel the queries fold to NULL via NULLIF).
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// f8 maps an optional float to a pgtype.Float8 (nil → NULL).
func f8(p *float64) pgtype.Float8 {
	if p == nil {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: *p, Valid: true}
}

// LanguoidRepo is the pgx/sqlc-backed implementation of domain.LanguoidStore (D-Languages, M18), bound
// to a single db.DBTX (the caller's transaction so the upsert + audit row commit together — D-Audit).
type LanguoidRepo struct {
	q *dataimportsql.Queries
}

// NewLanguoidRepo binds a languoid store to the given command surface.
func NewLanguoidRepo(conn db.DBTX) *LanguoidRepo {
	return &LanguoidRepo{q: dataimportsql.New(conn)}
}

var _ domain.LanguoidStore = (*LanguoidRepo)(nil)

// GetVersion returns the languoid's stored source_version (the idempotency key) and whether it exists.
func (r *LanguoidRepo) GetVersion(ctx context.Context, code string) (string, bool, error) {
	v, err := r.q.GetLanguoidVersion(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v.String, true, nil
}

// Insert adds a languoid; parent glottocode is resolved to its RID in SQL, provenance stamped.
func (r *LanguoidRepo) Insert(ctx context.Context, l domain.Languoid, prov domain.Provenance) error {
	return r.q.InsertLanguoidImport(ctx, dataimportsql.InsertLanguoidImportParams{
		Code:          l.Code,
		Level:         l.Level,
		Name:          l.Name,
		ParentCode:    l.Parent,
		Iso6393:       l.ISO639_3,
		Macroarea:     l.Macroarea,
		Latitude:      f8(l.Latitude),
		Longitude:     f8(l.Longitude),
		Status:        l.Status,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// UpdateImport rewrites a languoid (called when the source edition changed).
func (r *LanguoidRepo) UpdateImport(ctx context.Context, l domain.Languoid, prov domain.Provenance) error {
	return r.q.UpdateLanguoidImport(ctx, dataimportsql.UpdateLanguoidImportParams{
		Code:          l.Code,
		Level:         l.Level,
		Name:          l.Name,
		ParentCode:    l.Parent,
		Iso6393:       l.ISO639_3,
		Macroarea:     l.Macroarea,
		Latitude:      f8(l.Latitude),
		Longitude:     f8(l.Longitude),
		Status:        l.Status,
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
	})
}

// ReplaceCountries resets a languoid's country ties to the given ISO alpha-2 codes (unresolved codes
// are silently dropped by the insert).
func (r *LanguoidRepo) ReplaceCountries(ctx context.Context, code string, countryCodes []string) error {
	if err := r.q.DeleteLanguoidCountries(ctx, code); err != nil {
		return err
	}
	for _, cc := range countryCodes {
		if cc == "" {
			continue
		}
		if err := r.q.InsertLanguoidCountry(ctx, dataimportsql.InsertLanguoidCountryParams{Code: code, CountryCode: cc}); err != nil {
			return err
		}
	}
	return nil
}

// RebuildClosure recomputes the transitive closure and the denormalized family_code (run once at the
// end of a language-scheme import).
func (r *LanguoidRepo) RebuildClosure(ctx context.Context) error {
	// Clear then rebuild as two statements (same import tx) — a single DELETE+INSERT CTE would
	// self-conflict on the closure PK when re-importing over an existing closure.
	if err := r.q.ClearLanguoidClosure(ctx); err != nil {
		return err
	}
	if err := r.q.RebuildLanguoidClosure(ctx); err != nil {
		return err
	}
	return r.q.RebuildLanguoidFamilyCodes(ctx)
}

// ReconcileLocaleLanguages populates the i18n_locale_languages link by matching each supported UI
// locale's ISO-639-3 code to the Glottolog languoid carrying that iso639_3 (D-i18n; run once at the
// end of a language-scheme import, after languoids exist). Idempotent and self-healing.
func (r *LanguoidRepo) ReconcileLocaleLanguages(ctx context.Context) error {
	return r.q.ReconcileLocaleLanguages(ctx)
}

// LanguageScriptRepo is the pgx/sqlc-backed implementation of domain.LanguageScriptStore (D-Languages,
// M18), bound to a single db.DBTX (the caller's transaction).
type LanguageScriptRepo struct {
	q *dataimportsql.Queries
}

// NewLanguageScriptRepo binds a language-scripts store to the given command surface.
func NewLanguageScriptRepo(conn db.DBTX) *LanguageScriptRepo {
	return &LanguageScriptRepo{q: dataimportsql.New(conn)}
}

var _ domain.LanguageScriptStore = (*LanguageScriptRepo)(nil)

// ResolveLanguoid maps an ISO 639-3 code to a languoid RID (found=false when no language carries it).
func (r *LanguageScriptRepo) ResolveLanguoid(ctx context.Context, iso639_3 string) (string, bool, error) {
	id, err := r.q.ResolveLanguoidByISO(ctx, pgtype.Text{String: iso639_3, Valid: iso639_3 != ""})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// ResolveWritingSystem maps an ISO-15924 code to a writing-system RID (found=false when not seeded).
func (r *LanguageScriptRepo) ResolveWritingSystem(ctx context.Context, code string) (string, bool, error) {
	id, err := r.q.ResolveWritingSystemByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// GetLinkPrimary returns the link's is_primary flag and whether the link exists.
func (r *LanguageScriptRepo) GetLinkPrimary(ctx context.Context, languoidID, writingSystemID string) (bool, bool, error) {
	p, err := r.q.GetLanguageWritingSystemPrimary(ctx, dataimportsql.GetLanguageWritingSystemPrimaryParams{
		LanguoidID:      languoidID,
		WritingSystemID: writingSystemID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return p, true, nil
}

// InsertLink adds a languoid↔writing-system link with provenance.
func (r *LanguageScriptRepo) InsertLink(ctx context.Context, languoidID, writingSystemID string, isPrimary bool, prov domain.Provenance) error {
	return r.q.InsertLanguageWritingSystem(ctx, dataimportsql.InsertLanguageWritingSystemParams{
		LanguoidID:      languoidID,
		WritingSystemID: writingSystemID,
		IsPrimary:       isPrimary,
		Source:          text(prov.Source),
		SourceVersion:   text(prov.SourceVersion),
		ImportedAt:      ts(prov.ImportedAt),
	})
}

// UpdateLink updates a link's is_primary + provenance (called when is_primary changed).
func (r *LanguageScriptRepo) UpdateLink(ctx context.Context, languoidID, writingSystemID string, isPrimary bool, prov domain.Provenance) error {
	return r.q.UpdateLanguageWritingSystem(ctx, dataimportsql.UpdateLanguageWritingSystemParams{
		LanguoidID:      languoidID,
		WritingSystemID: writingSystemID,
		IsPrimary:       isPrimary,
		Source:          text(prov.Source),
		SourceVersion:   text(prov.SourceVersion),
		ImportedAt:      ts(prov.ImportedAt),
	})
}
