// Package cldrscripts implements the CLDR language-scripts Mapper (D-Languages, M18). It reads a CLDR /
// Unicode language→script dataset (a JSON array of {iso639_3, writingSystem, isPrimary}, fetched by the
// `file` or `http` connector) and emits canonical language-scripts records for oikumenea's
// POST /import/language-scripts endpoint, which resolves each languoid (by ISO 639-3) + writing system
// (by ISO-15924 code) and upserts the language_writing_systems link. Neither Glottolog nor ISO-15924
// carries this mapping, so CLDR's supplemental language data is the source.
//
// In-memory Mapper (the dataset is small): the languoids (language-scheme) and the migration-seeded
// ISO-15924 writing_systems must pre-exist; oikumenea skips a link whose languoid or script does not
// resolve (so import order / partial script seeds are tolerated).
package cldrscripts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectType = "language-scripts"

// rawLink is one entry in the CLDR language-scripts dataset. `iso639_3` keys the language (resolved to
// a languoid downstream); `writingSystem` is the ISO-15924 script code; `isPrimary` marks the
// language's main script.
type rawLink struct {
	ISO639_3      string `json:"iso639_3"`
	WritingSystem string `json:"writingSystem"`
	IsPrimary     bool   `json:"isPrimary"`
}

// Mapper turns a raw CLDR language-scripts array into canonical language-scripts records.
type Mapper struct{}

var _ domain.Mapper = Mapper{}

// Map decodes the dataset and emits one canonical record per link. iso639_3 + writingSystem are
// required per row; a malformed row fails the whole map (surfaced in import_runs).
func (Mapper) Map(raw domain.RawBatch) ([]map[string]any, error) {
	var links []rawLink
	if err := json.Unmarshal(raw.Payload, &links); err != nil {
		return nil, fmt.Errorf("language-scripts: decode source: %w", err)
	}
	records := make([]map[string]any, 0, len(links))
	for _, l := range links {
		iso := strings.ToLower(strings.TrimSpace(l.ISO639_3))
		ws := strings.TrimSpace(l.WritingSystem)
		if iso == "" || ws == "" {
			return nil, fmt.Errorf("language-scripts: invalid link %+v", l)
		}
		records = append(records, map[string]any{
			"iso639_3":      iso,
			"writingSystem": ws,
			"isPrimary":     l.IsPrimary,
		})
	}
	return records, nil
}
