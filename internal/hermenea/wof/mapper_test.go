// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package wof

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMapFeature covers the pure WOF Feature -> canonical record mapping: id/name/parent derivation,
// geometry pass-through, population, and the is_current -> status flag.
func TestMapFeature(t *testing.T) {
	body := []byte(`{
	  "type": "Feature",
	  "properties": {
	    "wof:id": 85632563,
	    "wof:name": "Kyiv City",
	    "wof:placetype": "region",
	    "wof:country": "UA",
	    "wof:population": 2900000,
	    "wof:hierarchy": [{"continent_id": 102191581, "country_id": 85633267, "region_id": 85632563}],
	    "wof:concordances": {"wd:id": "Q1899"},
	    "name:eng_x_preferred": ["Kyiv"]
	  },
	  "geometry": {"type": "Point", "coordinates": [30.5, 50.45]}
	}`)

	rec, err := mapFeature("region", "UA", 1, body)
	if err != nil {
		t.Fatalf("mapFeature: %v", err)
	}
	if rec["wofId"].(int64) != 85632563 {
		t.Fatalf("wofId = %v", rec["wofId"])
	}
	if rec["name"] != "Kyiv City" {
		t.Fatalf("name = %v", rec["name"])
	}
	if rec["countryCode"] != "UA" {
		t.Fatalf("countryCode = %v", rec["countryCode"])
	}
	if rec["parentId"].(int64) != 85633267 { // region -> country_id from hierarchy
		t.Fatalf("parentId = %v, want country 85633267", rec["parentId"])
	}
	if rec["population"].(int64) != 2900000 {
		t.Fatalf("population = %v", rec["population"])
	}
	if rec["isCurrent"] != true {
		t.Fatalf("isCurrent = %v", rec["isCurrent"])
	}
	if rec["geometry"] == nil {
		t.Fatal("geometry not passed through")
	}

	// is_current = 0 retires (non-destructive); name falls back to the eng_x_preferred list.
	noName := []byte(`{"properties":{"wof:id":1,"name:eng_x_preferred":["Fallback"]}}`)
	rec2, err := mapFeature("locality", "UA", 0, noName)
	if err != nil {
		t.Fatalf("mapFeature fallback: %v", err)
	}
	if rec2["name"] != "Fallback" {
		t.Fatalf("name fallback = %v", rec2["name"])
	}
	if rec2["isCurrent"] != false {
		t.Fatalf("isCurrent = %v, want false (retired)", rec2["isCurrent"])
	}

	// A missing wof:id is rejected.
	if _, err := mapFeature("region", "UA", 1, []byte(`{"properties":{"wof:name":"x"}}`)); err == nil {
		t.Fatal("expected error for missing wof:id")
	}
}

func TestParentFromHierarchy(t *testing.T) {
	h := []any{map[string]any{
		"country_id": float64(85633267),
		"region_id":  float64(85632563),
		"county_id":  float64(101748479),
	}}
	if got := parentFromHierarchy("country", h, 85633267); got != 0 {
		t.Fatalf("country parent = %d, want 0", got)
	}
	if got := parentFromHierarchy("region", h, 85632563); got != 85633267 {
		t.Fatalf("region parent = %d, want country", got)
	}
	if got := parentFromHierarchy("county", h, 101748479); got != 85632563 {
		t.Fatalf("county parent = %d, want region", got)
	}
	if got := parentFromHierarchy("locality", h, 999); got != 101748479 {
		t.Fatalf("locality parent = %d, want county", got)
	}
}

// TestMapPaged builds a tiny WOF-shaped SQLite DB and asserts the mapper streams parent-first
// (country -> region -> locality) and skips alt geometries.
func TestMapPaged(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "wof.db")
	buildFixtureDB(t, dbPath)

	var order []int64
	byID := map[int64]map[string]any{}
	err := GeoPlacesMapper{}.MapPaged(context.Background(), domain.StagedSource{Path: dbPath},
		func(records []map[string]any) error {
			for _, r := range records {
				id := r["wofId"].(int64)
				order = append(order, id)
				byID[id] = r
			}
			return nil
		})
	if err != nil {
		t.Fatalf("MapPaged: %v", err)
	}

	// 3 admin records (the alt-geometry row is excluded by is_alt=0).
	if len(order) != 3 {
		t.Fatalf("got %d records, want 3: %v", len(order), order)
	}
	// Parent-first: country (85633267) before region (85632563) before locality (101751957).
	if order[0] != 85633267 || order[1] != 85632563 || order[2] != 101751957 {
		t.Fatalf("order = %v, want country,region,locality", order)
	}
	if byID[101751957]["parentId"].(int64) != 85632563 {
		t.Fatalf("locality parent = %v, want region", byID[101751957]["parentId"])
	}
	if byID[85633267]["placetype"] != "country" {
		t.Fatalf("country placetype = %v", byID[85633267]["placetype"])
	}
}

// buildFixtureDB creates the minimal spr + geojson tables the mapper reads.
func buildFixtureDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, stmt := range []string{
		`CREATE TABLE spr (id INTEGER, placetype TEXT, country TEXT, is_current INTEGER)`,
		`CREATE TABLE geojson (id INTEGER, body TEXT, is_alt INTEGER)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	rows := []struct {
		id        int64
		placetype string
		body      string
	}{
		{85633267, "country", `{"properties":{"wof:id":85633267,"wof:name":"Ukraine","wof:hierarchy":[{"country_id":85633267}]},"geometry":{"type":"Point","coordinates":[31,49]}}`},
		{85632563, "region", `{"properties":{"wof:id":85632563,"wof:name":"Kyiv City","wof:hierarchy":[{"country_id":85633267,"region_id":85632563}]},"geometry":{"type":"Point","coordinates":[30.5,50.45]}}`},
		{101751957, "locality", `{"properties":{"wof:id":101751957,"wof:name":"Kyiv","wof:population":2900000,"wof:hierarchy":[{"country_id":85633267,"region_id":85632563,"locality_id":101751957}]},"geometry":{"type":"Point","coordinates":[30.52,50.45]}}`},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO spr VALUES (?,?,?,?)`, r.id, r.placetype, "UA", 1); err != nil {
			t.Fatalf("insert spr: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO geojson VALUES (?,?,?)`, r.id, r.body, 0); err != nil {
			t.Fatalf("insert geojson: %v", err)
		}
	}
	// An alt geometry for the locality — must be excluded (is_alt=1).
	if _, err := db.Exec(`INSERT INTO geojson VALUES (?,?,?)`, int64(101751957), `{"properties":{"wof:id":101751957,"wof:name":"Kyiv (alt)"}}`, 1); err != nil {
		t.Fatalf("insert alt: %v", err)
	}
}
