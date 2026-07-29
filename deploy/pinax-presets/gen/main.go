// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Command gen is the reproducible generator for the pinax reference-plane preset bundle (D-Pinax, M45).
// It (re)writes the FULL bundled YAML presets under internal/pinax/presets/ from the committed upstream
// payloads (no network for most; the CIA World Factbook ethnicity set is fetched live, with cache),
// replacing the small bootstrap fixtures. The presets it OWNS:
//
//   - languages.yaml       ← deploy/language-presets/glottolog-5.3.json (topo-sorted parent-first)
//   - writing-systems.yaml ← deploy/language-presets/cldr-scripts.json
//   - countries.yaml       ← deploy/geo-presets/iso-3166.json (iso_a3 + numeric_code enrichment)
//   - religions.yaml       ← deploy/religion-presets/taxa.json (tree + theism classifications)
//   - ethnicities.yaml     ← CIA World Factbook via the tested factbookethnicities mapper (live fetch)
//
// ranks.yaml and colors.yaml are HAND-CURATED (not machine-derivable from a single upstream) and are
// left untouched. Records are emitted as one JSON object per line under `records:` — JSON is a strict
// subset of YAML, so the pinax loader parses them unchanged, and the files stay compact + diffable.
//
// Usage (from the repo root):
//
//	go run ./deploy/pinax-presets/gen                 # regenerate all (Factbook needs network)
//	go run ./deploy/pinax-presets/gen -skip-ethnicities   # offline: everything but Factbook
//	go run ./deploy/pinax-presets/gen -root /path/to/repo
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/factbookethnicities"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/fetcher"
)

func main() {
	root := flag.String("root", ".", "repo root (source payloads + preset output are resolved under it)")
	skipEth := flag.Bool("skip-ethnicities", false, "skip the network Factbook fetch (leave ethnicities.yaml as-is)")
	flag.Parse()

	steps := []struct {
		name string
		fn   func(string) error
	}{
		{"languages", genLanguages},
		{"writing-systems", genWritingSystems},
		{"countries", genCountries},
		{"religions", genReligions},
	}
	for _, s := range steps {
		if err := s.fn(*root); err != nil {
			fatalf("%s: %v", s.name, err)
		}
	}
	if *skipEth {
		fmt.Fprintln(os.Stderr, "ethnicities: skipped (-skip-ethnicities)")
	} else if err := genEthnicities(*root); err != nil {
		fatalf("ethnicities: %v", err)
	}

	// translations depend on the entity presets (ranks.yaml is read for the rank keys) and the network
	// (CLDR); run last. Skipped together with the other network fetches under -skip-ethnicities.
	if *skipEth {
		fmt.Fprintln(os.Stderr, "translations: skipped (-skip-ethnicities)")
	} else if err := genTranslations(*root); err != nil {
		fatalf("translations: %v", err)
	}
}

// ---- preset header manifests (mirror the fixture headers) ----

type manifest struct {
	preset, objectType, source, sourceVersion, license string
	dependsOn                                          []string
	comment                                            string // top-of-file provenance comment (each line prefixed with "# ")
}

// writePreset emits a preset file: the provenance comment, the manifest header, then one JSON object per
// record under `records:` (JSON ⊂ YAML, so the loader parses these directly). Deterministic output.
func writePreset(root string, m manifest, records []map[string]any) error {
	path := filepath.Join(root, "internal", "pinax", "presets", m.preset+".yaml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range splitLines(m.comment) {
		fmt.Fprintf(f, "# %s\n", line)
	}
	fmt.Fprintf(f, "preset: %s\n", m.preset)
	fmt.Fprintf(f, "objectType: %s\n", m.objectType)
	fmt.Fprintf(f, "source: %s\n", m.source)
	fmt.Fprintf(f, "sourceVersion: %q\n", m.sourceVersion)
	fmt.Fprintf(f, "license: %s\n", m.license)
	dep, _ := json.Marshal(m.dependsOn)
	fmt.Fprintf(f, "dependsOn: %s\n", dep)
	if len(records) == 0 {
		fmt.Fprintf(f, "records: []\n")
		fmt.Fprintf(os.Stderr, "%s: 0 records -> %s\n", m.preset, path)
		return nil
	}
	fmt.Fprintf(f, "records:\n")
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, rec := range records {
		if _, err := f.WriteString("  - "); err != nil {
			return err
		}
		if err := enc.Encode(rec); err != nil { // Encode writes a trailing newline
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "%s: %d records -> %s\n", m.preset, len(records), path)
	return nil
}

// ---- languages (Glottolog forest, topo-sorted parent-first) ----

func genLanguages(root string) error {
	var raw []map[string]any
	if err := readJSON(filepath.Join(root, "deploy", "language-presets", "glottolog-5.3.json"), &raw); err != nil {
		return err
	}
	ordered, err := topoParentFirst(raw)
	if err != nil {
		return err
	}
	return writePreset(root, manifest{
		preset:        "languages",
		objectType:    "language-scheme",
		source:        "glottolog",
		sourceVersion: "5.3",
		license:       "CC-BY-4.0",
		dependsOn:     []string{},
		comment: "pinax preset — languages (Glottolog 5.3 languoid forest). D-Pinax (M45), objectType language-scheme.\n" +
			"GENERATED by deploy/pinax-presets/gen from deploy/language-presets/glottolog-5.3.json — do not hand-edit.\n" +
			"Records are topo-sorted parent-first (family → language → dialect) so the languoid parent_id FK\n" +
			"(RESTRICT) resolves; the handler rebuilds the closure + family_code at the end of the batch.",
	}, ordered)
}

// topoParentFirst orders languoid records so every node follows its parent (roots first). The parent is
// carried as the parent glottocode in the "parent" field ("" / absent = a root). A record whose parent
// is not in the set is treated as a root (top-level) — matching the handler's tolerance.
func topoParentFirst(recs []map[string]any) ([]map[string]any, error) {
	present := make(map[string]bool, len(recs))
	for _, r := range recs {
		present[str(r["code"])] = true
	}
	children := make(map[string][]map[string]any, len(recs))
	var roots []map[string]any
	for _, r := range recs {
		p := str(r["parent"])
		if p == "" || !present[p] {
			roots = append(roots, r)
		} else {
			children[p] = append(children[p], r)
		}
	}
	// Stable order: sort roots + each child list by code, then BFS/DFS emit.
	byCode := func(s []map[string]any) {
		sort.Slice(s, func(i, j int) bool { return str(s[i]["code"]) < str(s[j]["code"]) })
	}
	byCode(roots)
	for k := range children {
		byCode(children[k])
	}
	out := make([]map[string]any, 0, len(recs))
	var walk func(r map[string]any)
	walk = func(r map[string]any) {
		out = append(out, r)
		for _, c := range children[str(r["code"])] {
			walk(c)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	if len(out) != len(recs) {
		return nil, fmt.Errorf("topo-sort dropped nodes (cycle?): in=%d out=%d", len(recs), len(out))
	}
	return out, nil
}

// ---- writing systems (CLDR language↔script links) ----

func genWritingSystems(root string) error {
	var raw []map[string]any
	if err := readJSON(filepath.Join(root, "deploy", "language-presets", "cldr-scripts.json"), &raw); err != nil {
		return err
	}
	return writePreset(root, manifest{
		preset:        "writing-systems",
		objectType:    "language-scripts",
		source:        "cldr",
		sourceVersion: "45",
		license:       "Unicode-3.0",
		dependsOn:     []string{"languages"},
		comment: "pinax preset — writing-systems wiring (CLDR language↔script links). D-Pinax (M45), objectType language-scripts.\n" +
			"GENERATED by deploy/pinax-presets/gen from deploy/language-presets/cldr-scripts.json — do not hand-edit.\n" +
			"Ties a language (by ISO 639-3) to an ISO-15924 writing system (migration-seeded); dependsOn languages\n" +
			"so the languoids resolve first. A link whose languoid or script does not resolve is skipped.",
	}, raw)
}

// ---- countries (ISO-3166 enrichment: iso_a3 + numeric_code, fill-if-empty) ----

func genCountries(root string) error {
	records, src, sv := fullCountryRecords(root)
	borders := naturalEarthBorders() // ISO alpha-2 -> GeoJSON geometry (low-res); nil if unavailable
	withGeom := 0
	for _, rec := range records {
		if g, ok := borders[str(rec["code"])]; ok {
			rec["geometry"] = g
			withGeom++
		}
	}
	sort.Slice(records, func(i, j int) bool { return str(records[i]["code"]) < str(records[j]["code"]) })
	fmt.Fprintf(os.Stderr, "countries: %d/%d with border geometry\n", withGeom, len(records))
	geomNote := "border polygons (Natural Earth 110m, low-res) fill `geom` (+ derived centroid/bbox)"
	if withGeom == 0 {
		geomNote = "NO border geometry (Natural Earth fetch unavailable) — geom stays for the WOF connector"
	}
	return writePreset(root, manifest{
		preset:        "countries",
		objectType:    "geo-countries",
		source:        src,
		sourceVersion: sv,
		license:       "ODbL-1.0",
		dependsOn:     []string{},
		comment: "pinax preset — countries (geo_countries enrichment). D-Pinax (M45), objectType geo-countries.\n" +
			"GENERATED by deploy/pinax-presets/gen (mledoze/countries ODbL + Natural Earth 110m public-domain; cached) — do not hand-edit.\n" +
			"The country skeleton (code + name) is migration-seeded; this preset ENRICHES it fill-if-empty with\n" +
			"iso_a3 + numeric_code; " + geomNote + ", never overwriting. The WOF geo-places\n" +
			"connector later upgrades `geom` to the high-res shape; a per-country colorCode overlay can be layered on top.",
	}, records)
}

// naturalEarthBorders fetches the Natural Earth 1:110m admin-0 country polygons (public domain, ~180
// low-res borders) and returns ISO alpha-2 → GeoJSON geometry. Returns nil when the fetch is
// unavailable (the countries preset then carries iso_a3/numeric only; WOF supplies geometry later).
func naturalEarthBorders() map[string]any {
	const url = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_admin_0_countries.geojson"
	b, err := fetchCached(url, "ne_110m_admin_0_countries.geojson")
	if err != nil {
		fmt.Fprintf(os.Stderr, "countries: Natural Earth fetch unavailable (%v)\n", err)
		return nil
	}
	var fc struct {
		Features []struct {
			Properties map[string]any  `json:"properties"`
			Geometry   json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(b, &fc); err != nil {
		fmt.Fprintf(os.Stderr, "countries: Natural Earth parse failed (%v)\n", err)
		return nil
	}
	out := make(map[string]any, len(fc.Features))
	for _, f := range fc.Features {
		cc := isoA2(f.Properties)
		if cc == "" || len(f.Geometry) == 0 {
			continue
		}
		var geom map[string]any
		if json.Unmarshal(f.Geometry, &geom) != nil {
			continue
		}
		out[cc] = geom
	}
	return out
}

// isoA2 reads a Natural Earth feature's ISO alpha-2, preferring ISO_A2 and falling back to the
// de-facto ISO_A2_EH when ISO_A2 is the "-99" sentinel Natural Earth uses for contested/absent codes.
func isoA2(p map[string]any) string {
	a2 := str(p["ISO_A2"])
	if a2 == "" || a2 == "-99" {
		a2 = str(p["ISO_A2_EH"])
	}
	if a2 == "-99" {
		return ""
	}
	return a2
}

// fullCountryRecords returns the enrichment records from the mledoze/countries dataset (full ~250 set
// with alpha-3 / numeric / lat-lng centroid), fetched with cache. It falls back to the small committed
// deploy/geo-presets/iso-3166.json (alpha-3 + numeric only, no centroid) when the fetch is unavailable.
func fullCountryRecords(root string) (records []map[string]any, source, version string) {
	const url = "https://raw.githubusercontent.com/mledoze/countries/master/countries.json"
	if b, err := fetchCached(url, "mledoze-countries.json"); err == nil {
		var raw []struct {
			CCA2    string    `json:"cca2"`
			CCA3    string    `json:"cca3"`
			CCN3    string    `json:"ccn3"`
			LatLng  []float64 `json:"latlng"`
			NameObj struct {
				Common string `json:"common"`
			} `json:"name"`
		}
		if json.Unmarshal(b, &raw) == nil && len(raw) > 0 {
			for _, c := range raw {
				if len(c.CCA2) != 2 {
					continue
				}
				rec := map[string]any{"code": c.CCA2, "name": c.NameObj.Common}
				if c.CCA3 != "" {
					rec["isoA3"] = c.CCA3
				}
				if c.CCN3 != "" {
					rec["numericCode"] = c.CCN3
				}
				if len(c.LatLng) == 2 {
					rec["latitude"] = c.LatLng[0]
					rec["longitude"] = c.LatLng[1]
				}
				records = append(records, rec)
			}
			return records, "mledoze-countries", "master"
		}
	}
	// offline fallback: the committed ISO-3166 subset (no centroids).
	fmt.Fprintln(os.Stderr, "countries: mledoze fetch unavailable, falling back to committed iso-3166.json")
	var iso []struct {
		Alpha2, Name, Alpha3, Numeric string
	}
	_ = readJSON(filepath.Join(root, "deploy", "geo-presets", "iso-3166.json"), &iso)
	for _, c := range iso {
		rec := map[string]any{"code": c.Alpha2, "name": c.Name}
		if c.Alpha3 != "" {
			rec["isoA3"] = c.Alpha3
		}
		if c.Numeric != "" {
			rec["numericCode"] = c.Numeric
		}
		records = append(records, rec)
	}
	return records, "iso-3166", "2026.1"
}

// ---- religions (curated faith taxonomy + theism classifications) ----

func genReligions(root string) error {
	var taxa struct {
		SourceVersion string `json:"source_version"`
		Taxa          []struct {
			Rank     string  `json:"rank"`
			Code     string  `json:"code"`
			Name     string  `json:"name"`
			Parent   *string `json:"parent"`
			Wikidata *string `json:"wikidata"`
		} `json:"taxa"`
		Theism []struct {
			Taxon          string `json:"taxon"`
			Classification string `json:"classification"`
		} `json:"theism"`
	}
	if err := readJSON(filepath.Join(root, "deploy", "religion-presets", "taxa.json"), &taxa); err != nil {
		return err
	}
	classes := make(map[string][]string) // taxon code -> theism codes (preserve file order)
	for _, p := range taxa.Theism {
		if p.Taxon != "" && p.Classification != "" {
			classes[p.Taxon] = append(classes[p.Taxon], p.Classification)
		}
	}
	// taxa.json is already parent-first (roots → branches → …), matching the handler's requirement.
	records := make([]map[string]any, 0, len(taxa.Taxa))
	for i, t := range taxa.Taxa {
		rec := map[string]any{"code": t.Code, "name": t.Name, "rank": t.Rank, "sortOrder": i * 10}
		if t.Parent != nil && *t.Parent != "" {
			rec["parent"] = *t.Parent
		}
		if t.Wikidata != nil && *t.Wikidata != "" {
			rec["wikidataId"] = *t.Wikidata
		}
		if cs := classes[t.Code]; len(cs) > 0 {
			rec["classifications"] = cs
		}
		records = append(records, rec)
	}
	sv := taxa.SourceVersion
	if sv == "" {
		sv = "2026.06"
	}
	return writePreset(root, manifest{
		preset:        "religions",
		objectType:    "religion-scheme",
		source:        "religion-presets",
		sourceVersion: sv,
		license:       "CC-BY-SA-4.0",
		dependsOn:     []string{},
		comment: "pinax preset — religions (religion_taxa faith taxonomy + theism). D-Pinax (M45), objectType religion-scheme.\n" +
			"GENERATED by deploy/pinax-presets/gen from deploy/religion-presets/taxa.json — do not hand-edit.\n" +
			"The migration (0023_religion) seeds the same curated tree (origin='seeded'); this preset re-asserts it\n" +
			"create-if-absent, so on a boot autoseed the existing taxa are skipped. Records are parent-first; `rank`\n" +
			"is the level marker; classifications tie theism tags M:N.",
	}, records)
}

// ---- ethnicities (CIA World Factbook, via the tested factbookethnicities mapper) ----

func genEthnicities(root string) error {
	ctx := context.Background()
	sc, ok := fetcher.Default()[domain.FetcherFactbook].(domain.StreamingFetcher)
	if !ok {
		return fmt.Errorf("factbook connector is not a StreamingFetcher")
	}
	fmt.Fprintln(os.Stderr, "ethnicities: staging CIA World Factbook (network)…")
	staged, err := sc.Stage(ctx, domain.Source{Locator: "factbook/factbook.json@master"})
	if err != nil {
		return err
	}
	defer staged.Cleanup()

	var records []map[string]any
	if err := (factbookethnicities.PagedMapper{}).MapPaged(ctx, staged, func(p []map[string]any) error {
		records = append(records, p...)
		return nil
	}); err != nil {
		return err
	}
	sort.Slice(records, func(i, j int) bool { return str(records[i]["code"]) < str(records[j]["code"]) })

	sv := staged.SourceVersion
	if sv == "" {
		sv = "factbook"
	}
	return writePreset(root, manifest{
		preset:        "ethnicities",
		objectType:    "ethnicity-scheme",
		source:        "cia-world-factbook",
		sourceVersion: sv,
		license:       "Public-Domain",
		dependsOn:     []string{},
		comment: "pinax preset — ethnicities (CIA World Factbook 'Ethnic groups', deduped). D-Pinax (M45), objectType ethnicity-scheme.\n" +
			"GENERATED by deploy/pinax-presets/gen (live Factbook fetch via the factbookethnicities mapper) — do not hand-edit.\n" +
			"A FLAT, country-linked catalog (the Factbook has no ethnicity hierarchy or language linkage). Group-level\n" +
			"reference data only — a person's declared (encrypted) ethnicity is untouched. sourceVersion is the git tree SHA.",
	}, records)
}

// ---- helpers ----

// fetchCached returns url's bytes, preferring a cached copy in $PINAX_CACHE (or the OS temp dir) so
// reruns are offline + reproducible. A network failure with no cache returns the error (caller falls back).
func fetchCached(url, cacheName string) ([]byte, error) {
	dir := os.Getenv("PINAX_CACHE")
	if dir == "" {
		dir = os.TempDir()
	}
	cache := filepath.Join(dir, cacheName)
	if b, err := os.ReadFile(cache); err == nil {
		return b, nil
	}
	fmt.Fprintf(os.Stderr, "downloading %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = os.WriteFile(cache, b, 0o644)
	return b, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func str(v any) string { s, _ := v.(string); return s }

func splitLines(s string) []string {
	var out, cur = []string{}, ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, cur)
	return out
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "pinax-gen: "+format+"\n", a...)
	os.Exit(1)
}
