// Package glottolog implements the Glottolog language-scheme Mapper (D-Languages, M18) — hermenea's
// first NEW import consumer beyond geo. It reads a Glottolog CLDF snapshot (a JSON array of languoids,
// fetched by the `file` or `http` connector) and emits canonical language-scheme records for
// oikumenea's POST /import/language-scheme endpoint, which upserts them glottocode-keyed, idempotently,
// and non-destructively into oikumenea.language_languoids (+ closure + country ties).
//
// It is an in-memory Mapper (not paged): the snapshot is ~26k small rows (a few MiB), and the import's
// closure + family_code rebuild must see the WHOLE forest in one transaction — so the entire scheme is
// loaded in a single canonical envelope. Records are emitted PARENT-FIRST (topologically by tree depth)
// because the languoid parent_id FK is RESTRICT.
package glottolog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectType = "language-scheme"

// rawLanguoid is one entry in the Glottolog CLDF snapshot array. The field set mirrors the canonical
// language-scheme record oikumenea's handler reads, so the mapper is mostly a parent-first re-ordering
// (with light validation) rather than a transformation.
type rawLanguoid struct {
	Code      string   `json:"code"`  // glottocode (8-char)
	Level     string   `json:"level"` // family | language | dialect
	Name      string   `json:"name"`
	Parent    string   `json:"parent"`   // parent glottocode ("" = root)
	ISO639_3  string   `json:"iso639_3"` // optional ISO 639-3
	Macroarea string   `json:"macroarea"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Status    string   `json:"status"`    // AES endangerment; "" => not_endangered downstream
	Countries []string `json:"countries"` // ISO-3166 alpha-2 codes
}

// Mapper turns a raw Glottolog snapshot into parent-first canonical language-scheme records.
type Mapper struct{}

var _ domain.Mapper = Mapper{}

// Map decodes the snapshot, validates each languoid (glottocode + level + name required), orders them
// parent-first by tree depth, and emits canonical records. A malformed row fails the whole map (the
// worker surfaces it in import_runs and retries/backs off).
func (Mapper) Map(raw domain.RawBatch) ([]map[string]any, error) {
	var langs []rawLanguoid
	if err := json.Unmarshal(raw.Payload, &langs); err != nil {
		return nil, fmt.Errorf("language-scheme: decode source: %w", err)
	}
	return orderAndBuild(langs)
}

// orderAndBuild validates the languoids (glottocode + level + name required), orders them parent-first
// by tree depth (so the RESTRICT parent_id FK always resolves on load), and emits canonical
// language-scheme records. Shared by the bundled-JSON Mapper and the live CLDF PagedMapper.
func orderAndBuild(langs []rawLanguoid) ([]map[string]any, error) {
	byCode := make(map[string]rawLanguoid, len(langs))
	for _, l := range langs {
		code := strings.ToLower(strings.TrimSpace(l.Code))
		if len(code) == 0 || strings.TrimSpace(l.Name) == "" || !validLevel(strings.ToLower(l.Level)) {
			return nil, fmt.Errorf("language-scheme: invalid languoid %+v", l)
		}
		l.Code = code
		byCode[code] = l
	}

	// Order parent-first by depth: a node's depth is its distance to a root (a parent not in the set is
	// treated as a root). Sorting by depth (then code, for determinism) guarantees every parent precedes
	// its children, satisfying the RESTRICT parent_id FK on load.
	depth := make(map[string]int, len(byCode))
	var depthOf func(code string, seen map[string]bool) int
	depthOf = func(code string, seen map[string]bool) int {
		if d, ok := depth[code]; ok {
			return d
		}
		l := byCode[code]
		parent := strings.ToLower(strings.TrimSpace(l.Parent))
		if parent == "" || parent == code || seen[code] {
			depth[code] = 0
			return 0
		}
		if _, ok := byCode[parent]; !ok {
			depth[code] = 0 // parent outside the snapshot → treat this node as a root
			return 0
		}
		seen[code] = true
		d := depthOf(parent, seen) + 1
		depth[code] = d
		return d
	}
	codes := make([]string, 0, len(byCode))
	for code := range byCode {
		depthOf(code, map[string]bool{})
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		if depth[codes[i]] != depth[codes[j]] {
			return depth[codes[i]] < depth[codes[j]]
		}
		return codes[i] < codes[j]
	})

	records := make([]map[string]any, 0, len(codes))
	for _, code := range codes {
		l := byCode[code]
		rec := map[string]any{
			"code":  l.Code,
			"level": strings.ToLower(strings.TrimSpace(l.Level)),
			"name":  strings.TrimSpace(l.Name),
		}
		if p := strings.ToLower(strings.TrimSpace(l.Parent)); p != "" && p != l.Code {
			if _, ok := byCode[p]; ok {
				rec["parent"] = p
			}
		}
		if iso := strings.ToLower(strings.TrimSpace(l.ISO639_3)); iso != "" {
			rec["iso639_3"] = iso
		}
		if ma := strings.TrimSpace(l.Macroarea); ma != "" {
			rec["macroarea"] = ma
		}
		if l.Latitude != nil {
			rec["latitude"] = *l.Latitude
		}
		if l.Longitude != nil {
			rec["longitude"] = *l.Longitude
		}
		if st := strings.TrimSpace(l.Status); st != "" {
			rec["status"] = st
		}
		if len(l.Countries) > 0 {
			cs := make([]any, 0, len(l.Countries))
			for _, c := range l.Countries {
				if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
					cs = append(cs, c)
				}
			}
			if len(cs) > 0 {
				rec["countries"] = cs
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

func validLevel(s string) bool {
	switch s {
	case "family", "language", "dialect":
		return true
	}
	return false
}
