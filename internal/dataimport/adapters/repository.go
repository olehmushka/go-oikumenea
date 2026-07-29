// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package adapters implements the data-import domain ports against infrastructure: the pgx/sqlc
// upsert over oikumenea.geo_countries (M16 first catalog). It depends on the database, never the
// reverse. Generated sqlc code lives in the dataimportsql subpackage and is never hand-edited.
package adapters

import (
	"context"
	"errors"
	"strconv"
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

// BulkUpsert merges one chunk set-based (R-05): the parallel-array merge statement, then the
// second-pass parent resolution over the touched rows only (a skipped row keeps its stored parent).
func (r *GeoPlaceRepo) BulkUpsert(ctx context.Context, places []domain.GeoPlace, prov domain.Provenance) (created, updated []int64, err error) {
	if len(places) == 0 {
		return nil, nil, nil
	}
	p := dataimportsql.BulkUpsertGeoPlacesParams{
		Source:        prov.Source,
		SourceVersion: prov.SourceVersion,
		ImportedAt:    ts(prov.ImportedAt),
		WofIds:        make([]int64, 0, len(places)),
		Placetypes:    make([]string, 0, len(places)),
		CountryCodes:  make([]string, 0, len(places)),
		Names:         make([]string, 0, len(places)),
		Populations:   make([]int64, 0, len(places)),
		Hierarchies:   make([]string, 0, len(places)),
		Concordances:  make([]string, 0, len(places)),
		Statuses:      make([]string, 0, len(places)),
		Geometries:    make([]string, 0, len(places)),
	}
	for _, pl := range places {
		p.WofIds = append(p.WofIds, pl.WofID)
		p.Placetypes = append(p.Placetypes, pl.Placetype)
		p.CountryCodes = append(p.CountryCodes, pl.CountryCode)
		p.Names = append(p.Names, pl.Name)
		p.Populations = append(p.Populations, deref(pl.Population))
		p.Hierarchies = append(p.Hierarchies, string(pl.Hierarchy))
		p.Concordances = append(p.Concordances, string(pl.Concordances))
		p.Statuses = append(p.Statuses, pl.Status)
		p.Geometries = append(p.Geometries, pl.GeometryJSON)
	}
	rows, err := r.q.BulkUpsertGeoPlaces(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	touched := make(map[int64]bool, len(rows))
	for _, row := range rows {
		touched[row.WofID] = true
		if row.Inserted {
			created = append(created, row.WofID)
		} else {
			updated = append(updated, row.WofID)
		}
	}
	if len(touched) == 0 {
		return created, updated, nil
	}
	sp := dataimportsql.BulkSetGeoPlaceParentsParams{
		WofIds:       make([]int64, 0, len(touched)),
		ParentWofIds: make([]int64, 0, len(touched)),
	}
	for _, pl := range places {
		if !touched[pl.WofID] {
			continue
		}
		sp.WofIds = append(sp.WofIds, pl.WofID)
		sp.ParentWofIds = append(sp.ParentWofIds, deref(pl.ParentID))
	}
	if err := r.q.BulkSetGeoPlaceParents(ctx, sp); err != nil {
		return nil, nil, err
	}
	return created, updated, nil
}

// BulkEnrichCountries mirrors the touched country places' wof_id/geometry/ISO concordances onto their
// geo_countries rows (upgrade-or-keep; D-GeoPlaces).
func (r *GeoPlaceRepo) BulkEnrichCountries(ctx context.Context, places []domain.GeoPlace) error {
	if len(places) == 0 {
		return nil
	}
	p := dataimportsql.BulkEnrichGeoCountriesFromWOFParams{
		WofIds:       make([]int64, 0, len(places)),
		Codes:        make([]string, 0, len(places)),
		IsoA3s:       make([]string, 0, len(places)),
		NumericCodes: make([]string, 0, len(places)),
		Geometries:   make([]string, 0, len(places)),
	}
	for _, pl := range places {
		p.WofIds = append(p.WofIds, pl.WofID)
		p.Codes = append(p.Codes, pl.CountryCode)
		p.IsoA3s = append(p.IsoA3s, pl.ISOA3)
		p.NumericCodes = append(p.NumericCodes, pl.NumericCode)
		p.Geometries = append(p.Geometries, pl.GeometryJSON)
	}
	return r.q.BulkEnrichGeoCountriesFromWOF(ctx, p)
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

// floatText renders an optional float for a bulk text[] parameter (nil → "" → NULL via NULLIF; the
// parallel-array merges carry optionals as text so one array can hold absent values).
func floatText(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
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

// BulkUpsert merges one chunk set-based (R-05): the parallel-array merge (or, under
// prov.CreateOnly, the insert-absent-only variant that never touches an existing row), then the
// second-pass parent resolution over the touched rows only. Latitude/longitude cross as text
// (” = NULL) so one array can carry absent values.
func (r *LanguoidRepo) BulkUpsert(ctx context.Context, ls []domain.Languoid, prov domain.Provenance) (created, updated []string, err error) {
	if len(ls) == 0 {
		return nil, nil, nil
	}
	n := len(ls)
	codes := make([]string, 0, n)
	levels := make([]string, 0, n)
	names := make([]string, 0, n)
	isos := make([]string, 0, n)
	macros := make([]string, 0, n)
	lats := make([]string, 0, n)
	lngs := make([]string, 0, n)
	statuses := make([]string, 0, n)
	for _, l := range ls {
		codes = append(codes, l.Code)
		levels = append(levels, l.Level)
		names = append(names, l.Name)
		isos = append(isos, l.ISO639_3)
		macros = append(macros, l.Macroarea)
		lats = append(lats, floatText(l.Latitude))
		lngs = append(lngs, floatText(l.Longitude))
		statuses = append(statuses, l.Status)
	}
	touched := make(map[string]bool, n)
	if prov.CreateOnly {
		created, err = r.q.BulkInsertLanguoidsAbsent(ctx, dataimportsql.BulkInsertLanguoidsAbsentParams{
			SourceVersion: prov.SourceVersion,
			Source:        prov.Source,
			ImportedAt:    ts(prov.ImportedAt),
			Codes:         codes,
			Levels:        levels,
			Names:         names,
			Iso6393s:      isos,
			Macroareas:    macros,
			Latitudes:     lats,
			Longitudes:    lngs,
			Statuses:      statuses,
		})
		if err != nil {
			return nil, nil, err
		}
		for _, c := range created {
			touched[c] = true
		}
	} else {
		rows, err := r.q.BulkUpsertLanguoids(ctx, dataimportsql.BulkUpsertLanguoidsParams{
			SourceVersion: prov.SourceVersion,
			Source:        prov.Source,
			ImportedAt:    ts(prov.ImportedAt),
			Codes:         codes,
			Levels:        levels,
			Names:         names,
			Iso6393s:      isos,
			Macroareas:    macros,
			Latitudes:     lats,
			Longitudes:    lngs,
			Statuses:      statuses,
		})
		if err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			touched[row.Code] = true
			if row.Inserted {
				created = append(created, row.Code)
			} else {
				updated = append(updated, row.Code)
			}
		}
	}
	if len(touched) == 0 {
		return created, updated, nil
	}
	sp := dataimportsql.BulkSetLanguoidParentsParams{
		Codes:       make([]string, 0, len(touched)),
		ParentCodes: make([]string, 0, len(touched)),
	}
	for _, l := range ls {
		if !touched[l.Code] {
			continue
		}
		sp.Codes = append(sp.Codes, l.Code)
		sp.ParentCodes = append(sp.ParentCodes, l.Parent)
	}
	if err := r.q.BulkSetLanguoidParents(ctx, sp); err != nil {
		return nil, nil, err
	}
	return created, updated, nil
}

// BulkReplaceCountries resets the touched languoids' country ties set-based: clear all ties for
// codes, then insert the flattened (pairCodes[i], pairCountries[i]) ties (an unresolved country code
// is silently dropped by the join).
func (r *LanguoidRepo) BulkReplaceCountries(ctx context.Context, codes []string, pairCodes, pairCountries []string) error {
	if len(codes) == 0 {
		return nil
	}
	if err := r.q.BulkDeleteLanguoidCountries(ctx, codes); err != nil {
		return err
	}
	if len(pairCodes) == 0 {
		return nil
	}
	return r.q.BulkInsertLanguoidCountries(ctx, dataimportsql.BulkInsertLanguoidCountriesParams{
		Codes:        pairCodes,
		CountryCodes: pairCountries,
	})
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
