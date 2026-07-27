// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package factbookethnicities implements the ethnicity-scheme PagedMapper (D-PhysicalIdentity amendment,
// M43) backed by the CIA World Factbook (US-government PUBLIC DOMAIN). It is driven by the `factbook`
// StreamingFetcher, which stages every `<region>/<cc>.json` country file to a temp directory; this
// mapper walks that directory, parses each country's "Ethnic groups" free-text, derives the country's
// ISO-3166 alpha-2 code from its Internet ccTLD, dedups group names across all countries, and emits
// canonical ethnicity-scheme records — entirely at runtime, in Go, with NO committed preset.
//
// The Factbook has no ethnicity hierarchy and no language linkage, so the emitted catalog is FLAT and
// country-linked only (records carry `code`, `name`, `countries`). Group metadata; a person's declared
// ethnicity (encrypted) is untouched, and the group's country ties are never inferred onto a person.
package factbookethnicities

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds.
const ObjectType = "ethnicity-scheme"

// PagedMapper parses the staged Factbook country files into canonical ethnicity-scheme records.
type PagedMapper struct{}

var _ domain.PagedMapper = PagedMapper{}

var (
	parenRe = regexp.MustCompile(`\([^)]*\)`)      // "(2001 est.)" and other caveats
	noteRe  = regexp.MustCompile(`(?i)\bnote\s*:`) // a trailing "note:" clause
	tokRe   = regexp.MustCompile(`^(.+?)\s+(?:less than\s+)?<?\s*\d[\d.,]*\s*%$`)
	wsRe    = regexp.MustCompile(`\s+`)
	slugRe  = regexp.MustCompile(`[^a-z0-9]+`)
	splitRe = regexp.MustCompile(`[;,]`)
	iso2Re  = regexp.MustCompile(`^[a-z]{2}$`)
)

// drop are catch-all / non-ethnicity tokens.
var drop = map[string]bool{"other": true, "others": true, "unspecified": true, "none": true, "unknown": true, "n/a": true, "na": true, "and": true}

// cctldISO maps the handful of Internet ccTLDs that differ from ISO-3166 alpha-2.
var cctldISO = map[string]string{"uk": "GB"}

// cctldSkip are ccTLDs that are not a single ISO country.
var cctldSkip = map[string]bool{"eu": true, "su": true, "ap": true, "": true}

type group struct {
	name      string
	countries map[string]struct{}
}

// MapPaged walks the staged country files, aggregates a flat deduped catalog, and emits it as one page
// (the set is small — a few hundred groups). A file without an "Ethnic groups" field is skipped.
func (PagedMapper) MapPaged(_ context.Context, staged domain.StagedSource, emit domain.PageFunc) error {
	groups := map[string]*group{}
	entries, err := os.ReadDir(staged.Path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(staged.Path, e.Name()))
		if err != nil {
			return err
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue // a non-country / malformed file — skip, don't fail the whole run
		}
		text := digStr(doc, "People and Society", "Ethnic groups", "text")
		if text == "" {
			continue
		}
		iso := cctldToISO(digStr(doc, "Communications", "Internet country code", "text"))
		for _, name := range parseGroups(text) {
			s := slug(name)
			if s == "" {
				continue
			}
			g := groups[s]
			if g == nil {
				g = &group{name: name, countries: map[string]struct{}{}}
				groups[s] = g
			}
			if iso != "" {
				g.countries[iso] = struct{}{}
			}
		}
	}

	codes := make([]string, 0, len(groups))
	for s := range groups {
		codes = append(codes, s)
	}
	sort.Strings(codes)
	records := make([]map[string]any, 0, len(codes))
	for _, s := range codes {
		g := groups[s]
		rec := map[string]any{"code": s, "name": g.name}
		if len(g.countries) > 0 {
			cc := make([]string, 0, len(g.countries))
			for c := range g.countries {
				cc = append(cc, c)
			}
			sort.Strings(cc)
			anys := make([]any, len(cc))
			for i, c := range cc {
				anys[i] = c
			}
			rec["countries"] = anys
		}
		records = append(records, rec)
	}
	if len(records) == 0 {
		return nil
	}
	return emit(records)
}

// parseGroups extracts group names from a Factbook "Ethnic groups" text (percentages/notes dropped).
func parseGroups(text string) []string {
	text = noteRe.Split(text, 2)[0]
	text = parenRe.ReplaceAllString(text, " ")
	var out []string
	for _, tok := range splitRe.Split(text, -1) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		m := tokRe.FindStringSubmatch(tok)
		if m == nil {
			continue // a fragment without a percentage — skip
		}
		name := wsRe.ReplaceAllString(strings.Trim(m[1], " .:-"), " ")
		if name == "" || drop[strings.ToLower(name)] {
			continue
		}
		out = append(out, name)
	}
	return out
}

func slug(name string) string {
	return strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

func cctldToISO(cctld string) string {
	code := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cctld), ".")))
	if cctldSkip[code] {
		return ""
	}
	if iso, ok := cctldISO[code]; ok {
		return iso
	}
	if iso2Re.MatchString(code) {
		return strings.ToUpper(code)
	}
	return ""
}

// digStr walks nested maps and returns a string leaf, or "".
func digStr(d map[string]any, path ...string) string {
	var cur any = d
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	s, _ := cur.(string)
	return s
}
