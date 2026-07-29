// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package wof implements the Who's-On-First geo-places PagedMapper (D-GeoPlaces, M16) — hermenea's
// first real connector mapper. It reads a staged WOF SQLite distribution (the `wof-sqlite` connector
// stages it to disk), walks the four administrative placetypes parent-first (country → region →
// county → locality), and emits bounded pages of canonical geo-places records for oikumenea's
// POST /import/geo-places endpoint. Geometry travels as GeoJSON text; oikumenea materializes it via
// PostGIS. The SQLite read uses the cgo-free modernc.org/sqlite driver.
package wof

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	_ "modernc.org/sqlite" // registers the cgo-free "sqlite" database/sql driver
)

// ObjectTypeGeoPlaces is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectTypeGeoPlaces = "geo-places"

// pageSize bounds one emitted page. Aligned with the loader's default chunk size (R-05) so one page =
// one canonical envelope (one POST / one oikumenea transaction): small enough that geometry-heavy
// pages stay under any HTTP body limit, large enough to amortize round-trips over a planet-scale
// backfill. Page boundaries are deterministic (fixed placetype walk + ORDER BY s.id), which the
// resume cursor relies on when a retried attempt skips already-acked chunks.
const pageSize = 5000

// placetypes are walked in this order so a child's parent (always a lower placetype) is loaded — and
// committed — before the child (the geo_places.parent_id FK is RESTRICT).
var placetypes = []string{"country", "region", "county", "locality"}

// GeoPlacesMapper reads a staged WOF SQLite DB and emits canonical geo-places pages.
type GeoPlacesMapper struct{}

var _ domain.PagedMapper = GeoPlacesMapper{}

// MapPaged opens the staged SQLite DB and streams each placetype in turn, emitting bounded pages. An
// `emitted` set accumulates every wof_id loaded so far; because parents are always a HIGHER placetype
// (already fully streamed before the current one), a record whose derived parent is NOT in that set is
// emitted as a top-level place (parentId omitted → NULL). This keeps the per-country backfill resilient
// to hierarchy references that point OUTSIDE the distribution — e.g. WOF parents Crimea to a country_id
// that isn't in the Ukraine admin dist, which would otherwise fail the whole region page on the
// geo_places.parent_id RESTRICT FK.
func (m GeoPlacesMapper) MapPaged(ctx context.Context, staged domain.StagedSource, emit domain.PageFunc) error {
	db, err := sql.Open("sqlite", staged.Path)
	if err != nil {
		return fmt.Errorf("open wof sqlite: %w", err)
	}
	defer func() { _ = db.Close() }()

	emitted := map[int64]bool{}
	for _, pt := range placetypes {
		if err := m.streamPlacetype(ctx, db, pt, emitted, emit); err != nil {
			return fmt.Errorf("placetype %s: %w", pt, err)
		}
	}
	return nil
}

// streamPlacetype pages over one placetype's standard-place rows joined to their canonical geometry,
// recording each emitted wof_id in `emitted` and dropping a parentId that references a place not in it.
func (m GeoPlacesMapper) streamPlacetype(ctx context.Context, db *sql.DB, placetype string, emitted map[int64]bool, emit domain.PageFunc) error {
	rows, err := db.QueryContext(ctx, `
		SELECT s.country, s.is_current, g.body
		FROM spr s
		JOIN geojson g ON g.id = s.id
		WHERE s.placetype = ? AND g.is_alt = 0
		ORDER BY s.id`, placetype)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	page := make([]map[string]any, 0, pageSize)
	for rows.Next() {
		var (
			country   sql.NullString
			isCurrent sql.NullInt64
			body      []byte
		)
		if err := rows.Scan(&country, &isCurrent, &body); err != nil {
			return err
		}
		rec, err := mapFeature(placetype, country.String, int(isCurrent.Int64), body)
		if err != nil {
			continue // skip a malformed feature rather than abort the whole backfill
		}
		// Drop a parent the import hasn't seen (out-of-dist / disputed-territory reference) so the row
		// lands top-level instead of failing the page's RESTRICT FK; record this id for its children.
		if pid, ok := rec["parentId"].(int64); ok && !emitted[pid] {
			delete(rec, "parentId")
		}
		if id, ok := rec["wofId"].(int64); ok {
			emitted[id] = true
		}
		page = append(page, rec)
		if len(page) >= pageSize {
			if err := emit(page); err != nil {
				return err
			}
			page = make([]map[string]any, 0, pageSize)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(page) > 0 {
		return emit(page)
	}
	return nil
}

// mapFeature turns one WOF GeoJSON Feature (plus the placetype/country/is_current from the SPR row)
// into a canonical geo-places record. Parent is derived from wof:hierarchy (the nearest imported
// ancestor) rather than wof:parent_id, so a region parented to a macroregion/dependency still anchors
// on its country. is_current=0 maps to a retired status downstream (non-destructive); -1 (unknown) is
// treated as current.
func mapFeature(placetype, country string, isCurrent int, body []byte) (map[string]any, error) {
	var f struct {
		Properties map[string]any  `json:"properties"`
		Geometry   json.RawMessage `json:"geometry"`
	}
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	props := f.Properties
	wofID := jsonInt(props["wof:id"])
	if wofID == 0 {
		return nil, fmt.Errorf("wof feature missing wof:id")
	}
	name := firstNonEmptyStr(asString(props["wof:name"]), firstOfList(props["name:eng_x_preferred"]))
	if name == "" {
		return nil, fmt.Errorf("wof feature %d missing name", wofID)
	}

	rec := map[string]any{
		"wofId":       wofID,
		"placetype":   placetype,
		"name":        name,
		"countryCode": country,
		"isCurrent":   isCurrent != 0, // 1 or -1(unknown) -> current; 0 -> retired
	}
	if pid := parentFromHierarchy(placetype, props["wof:hierarchy"], wofID); pid != 0 {
		rec["parentId"] = pid
	}
	if pop := jsonInt(firstNonNil(props["wof:population"], props["mz:population"])); pop != 0 {
		rec["population"] = pop
	}
	if h := props["wof:hierarchy"]; h != nil {
		rec["hierarchy"] = h
	}
	if c := props["wof:concordances"]; c != nil {
		rec["concordances"] = c
	}
	if len(f.Geometry) > 0 && string(f.Geometry) != "null" {
		var g any
		if err := json.Unmarshal(f.Geometry, &g); err == nil {
			rec["geometry"] = g
		}
	}
	if placetype == "country" {
		if a3 := countryAlpha3(props); a3 != "" {
			rec["isoA3"] = a3
		}
	}
	return rec, nil
}

// parentFromHierarchy picks the nearest ancestor of an imported placetype from the first wof:hierarchy
// entry (a map of <level>_id). Returns 0 when there is no imported parent (e.g. a country).
func parentFromHierarchy(placetype string, hierarchy any, self int64) int64 {
	list, ok := hierarchy.([]any)
	if !ok || len(list) == 0 {
		return 0
	}
	h, ok := list[0].(map[string]any)
	if !ok {
		return 0
	}
	var order []string
	switch placetype {
	case "region":
		order = []string{"country_id"}
	case "county":
		order = []string{"region_id", "country_id"}
	case "locality":
		order = []string{"county_id", "region_id", "country_id"}
	default: // country and anything else have no imported parent
		return 0
	}
	for _, k := range order {
		if id := jsonInt(h[k]); id != 0 && id != self {
			return id
		}
	}
	return 0
}

// countryAlpha3 best-effort extracts an ISO alpha-3 from common WOF/Natural-Earth property keys.
func countryAlpha3(props map[string]any) string {
	for _, k := range []string{"wof:country_alpha3", "ne:adm0_a3", "iso:country_alpha3"} {
		if s := asString(props[k]); s != "" {
			return s
		}
	}
	return ""
}

// ---- small JSON coercion helpers (WOF property values are loosely typed) ----

func jsonInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// firstOfList returns the first element of a JSON array of strings (WOF name variants are arrays).
func firstOfList(v any) string {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return ""
	}
	return asString(list[0])
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
