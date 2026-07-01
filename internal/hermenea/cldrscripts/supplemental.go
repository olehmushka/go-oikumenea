// SupplementalMapper is the LIVE language-scripts path (D-Languages, M18): it transforms the raw CLDR
// supplementalData.xml + SIL iso-639-3.tab (fetched fresh from upstream by the http-files streaming
// connector) into the same canonical language-scripts records the bundled-JSON Mapper produces — a Go
// port of deploy/language-presets/gen-presets.py's gen_cldr_scripts. It is a PagedMapper that emits all
// links as a single page.
package cldrscripts

import (
	"context"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// SupplementalMapper reads a staged CLDR directory (supplementalData.xml + iso-639-3.tab) and emits
// canonical language-scripts records.
type SupplementalMapper struct{}

var _ domain.PagedMapper = SupplementalMapper{}

// MapPaged builds the ISO 639-1→3 map from iso-639-3.tab, walks CLDR <language scripts=…> elements
// (primary, plus alt="secondary"), and emits one canonical record per (language, script) link.
func (SupplementalMapper) MapPaged(ctx context.Context, staged domain.StagedSource, emit domain.PageFunc) error {
	part1ToID, ids, err := readISO639(filepath.Join(staged.Path, "iso-639-3.tab"))
	if err != nil {
		return err
	}
	toISO3 := func(subtag string) (string, bool) {
		s := strings.ToLower(strings.TrimSpace(subtag))
		switch {
		case len(s) == 2:
			v, ok := part1ToID[s]
			return v, ok
		case len(s) == 3 && ids[s]:
			return s, true
		default:
			return "", false
		}
	}

	primary, secondary, err := readLanguageData(ctx, filepath.Join(staged.Path, "supplementalData.xml"), toISO3)
	if err != nil {
		return err
	}

	records := buildScriptRecords(primary, secondary)
	return emit(records)
}

// buildScriptRecords flattens the primary (ordered; first = isPrimary) and secondary script maps into
// deduped canonical records, deterministically ordered (matching gen-presets.py).
func buildScriptRecords(primary map[string][]string, secondary map[string]map[string]bool) []map[string]any {
	type link struct {
		iso, ws   string
		isPrimary bool
	}
	var links []link
	seen := map[string]bool{}
	add := func(iso, ws string, isPrimary bool) {
		key := iso + "|" + ws
		if seen[key] {
			return
		}
		seen[key] = true
		links = append(links, link{iso, ws, isPrimary})
	}
	for iso, scripts := range primary {
		for i, ws := range scripts {
			add(iso, ws, i == 0)
		}
	}
	for iso, scripts := range secondary {
		for ws := range scripts {
			add(iso, ws, false)
		}
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].iso != links[j].iso {
			return links[i].iso < links[j].iso
		}
		return links[i].ws < links[j].ws
	})
	out := make([]map[string]any, 0, len(links))
	for _, l := range links {
		out = append(out, map[string]any{"iso639_3": l.iso, "writingSystem": l.ws, "isPrimary": l.isPrimary})
	}
	return out
}

// readISO639 parses the SIL iso-639-3.tab (TSV): Part1 (2-letter) → Id (3-letter), and the set of valid
// 3-letter Ids.
func readISO639(path string) (part1ToID map[string]string, ids map[string]bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("language-scripts: open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("language-scripts: read %s: %w", filepath.Base(path), err)
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("language-scripts: empty %s", filepath.Base(path))
	}
	idIdx, p1Idx := -1, -1
	for i, h := range rows[0] {
		switch strings.TrimSpace(h) {
		case "Id":
			idIdx = i
		case "Part1":
			p1Idx = i
		}
	}
	if idIdx < 0 {
		return nil, nil, fmt.Errorf("language-scripts: %s missing Id column", filepath.Base(path))
	}
	part1ToID = map[string]string{}
	ids = map[string]bool{}
	for _, row := range rows[1:] {
		if idIdx >= len(row) {
			continue
		}
		id := strings.ToLower(strings.TrimSpace(row[idIdx]))
		if id == "" {
			continue
		}
		ids[id] = true
		if p1Idx >= 0 && p1Idx < len(row) {
			if p1 := strings.ToLower(strings.TrimSpace(row[p1Idx])); p1 != "" {
				part1ToID[p1] = id
			}
		}
	}
	return part1ToID, ids, nil
}

// readLanguageData walks every <language scripts=…> element in CLDR supplementalData.xml, resolving the
// type subtag to ISO 639-3 and partitioning scripts into primary (ordered) vs alt="secondary".
func readLanguageData(ctx context.Context, path string, toISO3 func(string) (string, bool)) (primary map[string][]string, secondary map[string]map[string]bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("language-scripts: open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()

	primary = map[string][]string{}
	secondary = map[string]map[string]bool{}
	dec := xml.NewDecoder(f)
	n := 0
	for {
		if n%4096 == 0 && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		n++
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, nil, fmt.Errorf("language-scripts: parse %s: %w", filepath.Base(path), err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "language" {
			continue
		}
		var typ, scripts, alt string
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "type":
				typ = a.Value
			case "scripts":
				scripts = a.Value
			case "alt":
				alt = a.Value
			}
		}
		if strings.TrimSpace(scripts) == "" {
			continue
		}
		iso, ok := toISO3(typ)
		if !ok {
			continue
		}
		scs := strings.Fields(scripts)
		if alt == "secondary" {
			if secondary[iso] == nil {
				secondary[iso] = map[string]bool{}
			}
			for _, sc := range scs {
				secondary[iso][sc] = true
			}
			continue
		}
		for _, sc := range scs {
			if !slices.Contains(primary[iso], sc) {
				primary[iso] = append(primary[iso], sc)
			}
		}
	}
	return primary, secondary, nil
}
