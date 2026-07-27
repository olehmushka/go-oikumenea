// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package pinax

import (
	"os"
	"path/filepath"
	"testing"
)

// writePack drops a preset YAML at <dir>/<pack>/<file>, creating the pack subdir.
func writePack(t *testing.T, dir, pack, file, body string) {
	t.Helper()
	pd := filepath.Join(dir, pack)
	if err := os.MkdirAll(pd, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pd, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoadPresetsEmbeddedOnly: with no packs dir, only embedded presets load and every one is tagged as
// embedded (Pack == "").
func TestLoadPresetsEmbeddedOnly(t *testing.T) {
	got, err := loadPresets("")
	if err != nil {
		t.Fatalf("loadPresets: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected embedded presets, got none")
	}
	for _, p := range got {
		if p.Pack != "" {
			t.Fatalf("embedded preset %q tagged with pack %q", p.Name, p.Pack)
		}
	}
}

// TestLoadPresetsMountedPackIsAdditive: a mounted pack's preset is loaded ALONGSIDE the embedded set and
// tagged with its pack (directory) name.
func TestLoadPresetsMountedPackIsAdditive(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "locale-xtest", "locales.yaml", `
preset: xtest-locale
objectType: locales
source: test
sourceVersion: "1"
records:
  - code: xts
    name: Testish
`)
	got, err := loadPresets(dir)
	if err != nil {
		t.Fatalf("loadPresets: %v", err)
	}
	embedded, mounted := 0, 0
	var found *Preset
	for i := range got {
		if got[i].Pack == "" {
			embedded++
		} else {
			mounted++
		}
		if got[i].Name == "xtest-locale" {
			found = &got[i]
		}
	}
	if embedded == 0 {
		t.Fatal("embedded presets vanished when a pack was mounted")
	}
	if found == nil {
		t.Fatal("mounted pack preset not loaded")
	}
	if found.Pack != "locale-xtest" {
		t.Fatalf("pack tag = %q, want locale-xtest", found.Pack)
	}
}

// TestLoadPresetsCollisionFails: two packs declaring the same preset name is a loud boot error — packs
// are additive, never silent overrides.
func TestLoadPresetsCollisionFails(t *testing.T) {
	dir := t.TempDir()
	body := `
preset: dup-preset
objectType: locales
source: test
sourceVersion: "1"
records: []
`
	writePack(t, dir, "pack-a", "p.yaml", body)
	writePack(t, dir, "pack-b", "p.yaml", body)
	if _, err := loadPresets(dir); err == nil {
		t.Fatal("expected a duplicate-preset error across two packs, got nil")
	}
}

// TestLoadPresetsCollidesWithEmbedded: a pack preset reusing an EMBEDDED preset name (here `countries`)
// is rejected, so a pack can never silently override a bundled catalog.
func TestLoadPresetsCollidesWithEmbedded(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "rogue", "countries.yaml", `
preset: countries
objectType: geo-countries
source: rogue
sourceVersion: "1"
records: []
`)
	if _, err := loadPresets(dir); err == nil {
		t.Fatal("expected a collision with the embedded `countries` preset, got nil")
	}
}
