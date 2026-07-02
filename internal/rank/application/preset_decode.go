package application

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// pinax ranks-preset decode types (D-Pinax, M45): the bundled YAML record for one rank system, decoded
// via a YAML round-trip into the Preset shape ImportPreset already consumes. Field names (camelCase)
// match the preset YAML; every ordinal/optional is a pointer so "absent" is distinguishable.
type (
	presetSystemYAML struct {
		Code       string               `yaml:"code"`
		Name       string               `yaml:"name"`
		Country    *string              `yaml:"country"`
		SortOrder  *int                 `yaml:"sortOrder"`
		Categories []presetCategoryYAML `yaml:"categories"`
	}
	presetCategoryYAML struct {
		Code      string           `yaml:"code"`
		Name      string           `yaml:"name"`
		SortOrder *int             `yaml:"sortOrder"`
		Types     []presetTypeYAML `yaml:"types"`
	}
	presetTypeYAML struct {
		Code      string           `yaml:"code"`
		Name      string           `yaml:"name"`
		SortOrder *int             `yaml:"sortOrder"`
		Children  []presetTypeYAML `yaml:"children"`
		Ranks     []presetRankYAML `yaml:"ranks"`
	}
	presetRankYAML struct {
		Code         string  `yaml:"code"`
		Name         string  `yaml:"name"`
		Abbreviation *string `yaml:"abbreviation"`
		GradeCode    *string `yaml:"gradeCode"`
		SortOrder    *int    `yaml:"sortOrder"`
	}
)

// PresetFromMap decodes one pinax ranks-preset record (a nested rank-system map, as parsed from the
// bundled YAML) into a Preset for ImportPreset (D-Pinax, M45). It round-trips through YAML so the nested
// system→category→type→rank shape maps onto the typed Preset without a hand-written walker.
func PresetFromMap(m map[string]any) (Preset, error) {
	raw, err := yaml.Marshal(m)
	if err != nil {
		return Preset{}, err
	}
	var sys presetSystemYAML
	if err := yaml.Unmarshal(raw, &sys); err != nil {
		return Preset{}, fmt.Errorf("decode ranks preset record: %w", err)
	}
	if sys.Code == "" {
		return Preset{}, fmt.Errorf("ranks preset record missing system code")
	}
	return Preset{System: sys.toSystem()}, nil
}

func (s presetSystemYAML) toSystem() PresetSystem {
	cats := make([]PresetCategory, len(s.Categories))
	for i, c := range s.Categories {
		cats[i] = c.toCategory()
	}
	return PresetSystem{Code: s.Code, Name: s.Name, Country: s.Country, SortOrder: s.SortOrder, Categories: cats}
}

func (c presetCategoryYAML) toCategory() PresetCategory {
	types := make([]PresetType, len(c.Types))
	for i, t := range c.Types {
		types[i] = t.toType()
	}
	return PresetCategory{Code: c.Code, Name: c.Name, SortOrder: c.SortOrder, Types: types}
}

func (t presetTypeYAML) toType() PresetType {
	children := make([]PresetType, len(t.Children))
	for i, ch := range t.Children {
		children[i] = ch.toType()
	}
	ranks := make([]PresetRank, len(t.Ranks))
	for i, r := range t.Ranks {
		ranks[i] = r.toRank()
	}
	return PresetType{Code: t.Code, Name: t.Name, SortOrder: t.SortOrder, Children: children, Ranks: ranks}
}

func (r presetRankYAML) toRank() PresetRank {
	return PresetRank{Code: r.Code, Name: r.Name, Abbreviation: r.Abbreviation, GradeCode: r.GradeCode, SortOrder: r.SortOrder}
}
