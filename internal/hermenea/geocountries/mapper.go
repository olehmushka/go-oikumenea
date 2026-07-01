// Package geocountries implements the ISO-3166 geo-countries Mapper (M16 / D-Hermenea) — hermenea's
// simplest, network-free proving consumer (the milestone's named "first consumer"). It reads a raw
// ISO-3166 source payload (a JSON array of countries, fetched by the `file` or `http` connector) and
// emits canonical geo-countries records for oikumenea's POST /import/geo-countries endpoint, which
// upserts them code-keyed, idempotently, and non-destructively into oikumenea.geo_countries.
//
// Unlike the wof geo-places mapper this is an in-memory Mapper (not paged): the country list is tiny.
package geocountries

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectType = "geo-countries"

// rawCountry is one entry in the ISO-3166 source array. `alpha2` is the 2-letter code oikumenea keys
// on; alpha3/numeric are carried through as harmless extras (the geo-countries handler reads only
// code+name, but provenance-rich records keep the envelope faithful to the source).
type rawCountry struct {
	Alpha2  string `json:"alpha2"`
	Name    string `json:"name"`
	Alpha3  string `json:"alpha3"`
	Numeric string `json:"numeric"`
}

// Mapper turns a raw ISO-3166 country array into canonical geo-countries records.
type Mapper struct{}

var _ domain.Mapper = Mapper{}

// Map decodes the raw payload (a JSON array of ISO-3166 countries) into canonical records keyed by the
// alpha-2 `code` + translatable `name`. A 2-letter code and a name are required per row; a malformed
// row fails the whole map (the worker then surfaces it in import_runs and retries/backs off).
func (Mapper) Map(raw domain.RawBatch) ([]map[string]any, error) {
	var countries []rawCountry
	if err := json.Unmarshal(raw.Payload, &countries); err != nil {
		return nil, fmt.Errorf("geo-countries: decode source: %w", err)
	}
	records := make([]map[string]any, 0, len(countries))
	for _, c := range countries {
		code := strings.ToUpper(strings.TrimSpace(c.Alpha2))
		name := strings.TrimSpace(c.Name)
		if len(code) != 2 || name == "" {
			return nil, fmt.Errorf("geo-countries: invalid row %+v", c)
		}
		rec := map[string]any{"code": code, "name": name}
		if a3 := strings.ToUpper(strings.TrimSpace(c.Alpha3)); a3 != "" {
			rec["alpha3"] = a3
		}
		if num := strings.TrimSpace(c.Numeric); num != "" {
			rec["numeric"] = num
		}
		records = append(records, rec)
	}
	return records, nil
}
