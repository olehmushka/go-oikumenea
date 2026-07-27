// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package wof

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMapPagedAgainstSQLite drives the real MapPaged leg over a tiny generated WOF SQLite fixture (the
// spr + geojson tables the mapper joins). It asserts the four placetypes are streamed PARENT-FIRST
// (country -> region -> locality) — the ordering the geo_places.parent_id RESTRICT FK depends on — and
// that the SPR country/is_current columns flow into the emitted records.
func TestMapPagedAgainstSQLite(t *testing.T) {
	path := buildWOFFixture(t)

	var got []map[string]any
	emit := func(page []map[string]any) error {
		got = append(got, page...)
		return nil
	}
	if err := (GeoPlacesMapper{}).MapPaged(context.Background(), domain.StagedSource{Path: path}, emit); err != nil {
		t.Fatalf("MapPaged: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("emitted %d records, want 3", len(got))
	}
	// parent-first: country, then region, then locality.
	wantOrder := []struct {
		wofID     int64
		placetype string
		parent    int64 // 0 = no parent
	}{
		{1, "country", 0},
		{2, "region", 1},
		{3, "locality", 2},
	}
	for i, w := range wantOrder {
		rec := got[i]
		if rec["wofId"].(int64) != w.wofID || rec["placetype"] != w.placetype {
			t.Fatalf("record %d = %+v, want wof %d %s", i, rec, w.wofID, w.placetype)
		}
		if w.parent == 0 {
			if _, ok := rec["parentId"]; ok {
				t.Fatalf("record %d should have no parent: %+v", i, rec)
			}
		} else if rec["parentId"].(int64) != w.parent {
			t.Fatalf("record %d parent = %v, want %d", i, rec["parentId"], w.parent)
		}
		if rec["countryCode"] != "ZZ" {
			t.Fatalf("record %d country = %v, want ZZ", i, rec["countryCode"])
		}
	}
}

// TestMapPagedDropsOutOfDistParent covers the Crimea case: a region whose hierarchy country_id points
// to a country NOT in this distribution must be emitted top-level (no parentId) rather than carrying a
// dangling parent that would fail the whole page on geo_places.parent_id RESTRICT. A sibling region with
// an in-dist parent keeps it.
func TestMapPagedDropsOutOfDistParent(t *testing.T) {
	path := buildFixture(t, []feat{
		{1, "country", map[string]any{"wof:id": 1, "wof:name": "Ukraine", "wof:country": "UA"}},
		{2, "region", map[string]any{"wof:id": 2, "wof:name": "Kyiv", "wof:hierarchy": []any{map[string]any{"country_id": 1, "region_id": 2}}}},
		{3, "region", map[string]any{"wof:id": 3, "wof:name": "Crimea", "wof:hierarchy": []any{map[string]any{"country_id": 999, "region_id": 3}}}},
	})

	got := map[int64]map[string]any{}
	must := (GeoPlacesMapper{}).MapPaged(context.Background(), domain.StagedSource{Path: path}, func(page []map[string]any) error {
		for _, r := range page {
			got[r["wofId"].(int64)] = r
		}
		return nil
	})
	if must != nil {
		t.Fatalf("MapPaged: %v", must)
	}
	if got[2]["parentId"].(int64) != 1 {
		t.Fatalf("in-dist region parent = %v, want 1", got[2]["parentId"])
	}
	if _, ok := got[3]["parentId"]; ok {
		t.Fatalf("out-of-dist region kept a dangling parent: %v", got[3]["parentId"])
	}
}

// feat is one fixture feature.
type feat struct {
	id        int64
	placetype string
	props     map[string]any
}

// buildFixture writes a minimal WOF-shaped SQLite DB (spr + geojson) for the given features.
func buildFixture(t *testing.T, features []feat) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wof.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`
		CREATE TABLE spr (id INTEGER, placetype TEXT, country TEXT, is_current INTEGER);
		CREATE TABLE geojson (id INTEGER, body TEXT, is_alt INTEGER);
	`); err != nil {
		t.Fatalf("create tables: %v", err)
	}
	for _, f := range features {
		if _, err := db.Exec("INSERT INTO spr (id, placetype, country, is_current) VALUES (?,?,?,?)", f.id, f.placetype, "UA", 1); err != nil {
			t.Fatalf("insert spr: %v", err)
		}
		body, _ := json.Marshal(map[string]any{"type": "Feature", "properties": f.props,
			"geometry": map[string]any{"type": "Point", "coordinates": []any{float64(f.id), float64(f.id)}}})
		if _, err := db.Exec("INSERT INTO geojson (id, body, is_alt) VALUES (?,?,0)", f.id, string(body)); err != nil {
			t.Fatalf("insert geojson: %v", err)
		}
	}
	return path
}

// buildWOFFixture writes a minimal WOF-shaped SQLite DB (spr + geojson) with one country, one region,
// one locality (parent-first hierarchy) and returns its path.
func buildWOFFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wof.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`
		CREATE TABLE spr (id INTEGER, placetype TEXT, country TEXT, is_current INTEGER);
		CREATE TABLE geojson (id INTEGER, body TEXT, is_alt INTEGER);
	`); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	type feature struct {
		id        int64
		placetype string
		props     map[string]any
	}
	features := []feature{
		{1, "country", map[string]any{"wof:id": 1, "wof:name": "Testland", "wof:country": "ZZ"}},
		{2, "region", map[string]any{"wof:id": 2, "wof:name": "Test Region", "wof:hierarchy": []any{map[string]any{"country_id": 1, "region_id": 2}}}},
		{3, "locality", map[string]any{"wof:id": 3, "wof:name": "Test City", "wof:population": 1000, "wof:hierarchy": []any{map[string]any{"country_id": 1, "region_id": 2, "locality_id": 3}}}},
	}
	for _, f := range features {
		if _, err := db.Exec("INSERT INTO spr (id, placetype, country, is_current) VALUES (?,?,?,?)", f.id, f.placetype, "ZZ", 1); err != nil {
			t.Fatalf("insert spr: %v", err)
		}
		body, _ := json.Marshal(map[string]any{
			"type":       "Feature",
			"properties": f.props,
			"geometry":   map[string]any{"type": "Point", "coordinates": []any{float64(f.id), float64(f.id)}},
		})
		if _, err := db.Exec("INSERT INTO geojson (id, body, is_alt) VALUES (?,?,0)", f.id, string(body)); err != nil {
			t.Fatalf("insert geojson: %v", err)
		}
	}
	return path
}
