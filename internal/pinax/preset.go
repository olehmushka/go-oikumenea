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
	"sort"

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
}

// loadPresets decodes every embedded preset and returns them in a valid dependency order (a preset's
// dependsOn entries come first). A missing dependency or a cycle is a build-the-bundle error, surfaced
// loudly at boot rather than producing a partial seed.
func loadPresets() ([]Preset, error) {
	entries, err := fs.Glob(presetFS, "presets/*.yaml")
	if err != nil {
		return nil, err
	}
	byName := make(map[string]Preset, len(entries))
	for _, path := range entries {
		b, err := presetFS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var p Preset
		if err := yaml.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("pinax preset %s: %w", path, err)
		}
		if p.Name == "" || p.ObjectType == "" {
			return nil, fmt.Errorf("pinax preset %s: missing preset name or objectType", path)
		}
		if _, dup := byName[p.Name]; dup {
			return nil, fmt.Errorf("pinax preset %s: duplicate preset name %q", path, p.Name)
		}
		byName[p.Name] = p
	}
	return topoSort(byName)
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
