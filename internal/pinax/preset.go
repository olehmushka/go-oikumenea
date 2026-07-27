// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package pinax is the reference-plane boot autoseeder (D-Pinax, M45). It embeds the bundled YAML seed
// presets — the instance-global world-model catalogs (languages, writing-systems, countries, religions,
// ethnicities, ranks, colors) — and self-seeds them into oikumenea's own DB on boot via the SAME
// application import service the HTTP POST /import/{objectType} wraps. So a fresh oikumenea is usable
// standalone; the hermenea companion is reserved for the massive/live connectors (geo_places, Wikidata).
//
// Seeding is create-if-absent / never-overwrite (D-Pinax): each preset is applied in dependency order,
// version-gated via oikumenea.pinax_seed_state so a warm DB does an O(#presets) no-op check instead of
// re-parsing + re-upserting on every restart. `oikumenea seed --reconcile` re-runs with update-on-change.
package pinax

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// presetFS embeds every bundled preset. The files are generated (machine presets like the Glottolog
// forest) or hand-authored (curated presets like ranks/religions) into this directory; see
// deploy/pinax-presets/ for the reproducible generator + raw sources.
//
//go:embed presets/*.yaml
var presetFS embed.FS

// Preset is one bundled reference dataset: a manifest header + canonical records for one import
// object-type. `records` are already-canonical (the generator ran any upstream→canonical mapping
// offline) so the seeder decodes them straight into domain.Record for the existing import handlers.
type Preset struct {
	Name          string           `yaml:"preset"`        // unique preset name = pinax_seed_state key
	ObjectType    string           `yaml:"objectType"`    // import routing key (geo-countries, language-scheme, …)
	Source        string           `yaml:"source"`        // provenance source label
	SourceVersion string           `yaml:"sourceVersion"` // provenance/version-gate key
	License       string           `yaml:"license"`       // upstream license (documentation)
	DependsOn     []string         `yaml:"dependsOn"`     // preset names that must seed first (topo order)
	Records       []map[string]any `yaml:"records"`       // canonical records for the object-type handler

	// Pack is the ORIGIN of this preset (D-DataPacks, M54): "" for the embedded bundle, else the
	// operator-mounted pack's directory name. Set by loadPresets, not the YAML; recorded in
	// pinax_seed_state so an operator can see which pack supplied a seeded slice.
	Pack string `yaml:"-"`
}

// loadPresets decodes every embedded preset PLUS any presets under the operator-mounted packs
// directory (D-DataPacks, M54; packsDir "" = embedded-only) and returns them in a valid dependency
// order (a preset's dependsOn entries come first). A missing dependency, a cycle, or a name collision
// (across the embedded bundle and packs, or between two packs) is a build-the-bundle error, surfaced
// loudly at boot rather than producing a partial seed — packs are ADDITIVE, never silent overrides.
func loadPresets(packsDir string) ([]Preset, error) {
	byName := make(map[string]Preset)

	// Embedded bundle (Pack == "").
	entries, err := fs.Glob(presetFS, "presets/*.yaml")
	if err != nil {
		return nil, err
	}
	for _, path := range entries {
		b, err := presetFS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := addPreset(byName, path, "", b); err != nil {
			return nil, err
		}
	}

	// Operator-mounted packs (Pack == the pack's immediate subdir name under packsDir, or the file's
	// base name for a loose top-level .yaml). Scanned recursively so a pack is a directory of presets.
	if packsDir != "" {
		if err := loadPackDir(byName, packsDir); err != nil {
			return nil, err
		}
	}

	return topoSort(byName)
}

// loadPackDir walks the mounted packs directory and adds every `*.yaml` preset, tagging each with the
// pack name it belongs to. A missing directory is a hard error (an operator who configured a packs dir
// expects it to exist), but an EMPTY directory is fine (no packs mounted yet).
func loadPackDir(byName map[string]Preset, packsDir string) error {
	root, err := filepath.Abs(packsDir)
	if err != nil {
		return fmt.Errorf("pinax packs dir %q: %w", packsDir, err)
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("pinax packs scan %q: %w", path, err)
		}
		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return addPreset(byName, path, packName(root, path), b)
	})
}

// packName derives a pack's name from a preset file's path: the first directory segment below the packs
// root (so `<packs>/locale-deu/locales.yaml` → "locale-deu"), or the file's base name for a preset
// dropped loose at the top level.
func packName(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Base(path)
	}
	if dir, _ := filepath.Split(rel); dir != "" {
		return filepath.Clean(strings.SplitN(dir, string(filepath.Separator), 2)[0])
	}
	return rel
}

// addPreset decodes one preset YAML, validates it, and inserts it under its unique name — rejecting a
// collision so the same catalog slice is never seeded from two sources. `pack` is "" for the embedded
// bundle.
func addPreset(byName map[string]Preset, path, pack string, b []byte) error {
	var p Preset
	if err := yaml.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("pinax preset %s: %w", path, err)
	}
	if p.Name == "" || p.ObjectType == "" {
		return fmt.Errorf("pinax preset %s: missing preset name or objectType", path)
	}
	if existing, dup := byName[p.Name]; dup {
		return fmt.Errorf("pinax preset %s: duplicate preset name %q (already provided by %s)",
			path, p.Name, originLabel(existing))
	}
	p.Pack = pack
	byName[p.Name] = p
	return nil
}

// originLabel names where a preset came from, for a readable collision error.
func originLabel(p Preset) string {
	if p.Pack == "" {
		return "the embedded bundle"
	}
	return "pack " + p.Pack
}

// topoSort orders presets so every dependsOn precedes its dependents (Kahn-ish DFS over stable-sorted
// names for determinism). Errors on a missing dependency or a cycle.
func topoSort(byName map[string]Preset) ([]Preset, error) {
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	const (
		white = 0 // unvisited
		gray  = 1 // on the current DFS stack (a back-edge to gray = cycle)
		black = 2 // done
	)
	color := make(map[string]int, len(names))
	out := make([]Preset, 0, len(names))

	var visit func(n string) error
	visit = func(n string) error {
		switch color[n] {
		case gray:
			return fmt.Errorf("pinax presets: dependency cycle at %q", n)
		case black:
			return nil
		}
		color[n] = gray
		p := byName[n]
		for _, dep := range p.DependsOn {
			if _, ok := byName[dep]; !ok {
				return fmt.Errorf("pinax preset %q: unknown dependsOn %q", n, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		color[n] = black
		out = append(out, p)
		return nil
	}
	for _, n := range names {
		if err := visit(n); err != nil {
			return nil, err
		}
	}
	return out, nil
}
