// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// CLDFMapper is the LIVE language-scheme path (D-Languages, M18): it transforms the raw Glottolog CLDF
// snapshot (languages.csv + values.csv, fetched fresh from upstream master by the http-files streaming
// connector) into the same canonical language-scheme records the bundled-JSON Mapper produces — a Go
// port of deploy/language-presets/gen-presets.py's gen_glottolog. It is a PagedMapper that emits the
// WHOLE forest as a single page, so the import stays one transaction (the closure + family_code rebuild
// must see every node at once).
package glottolog

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// CLDFMapper reads a staged Glottolog CLDF directory and emits canonical language-scheme records.
type CLDFMapper struct{}

var _ domain.PagedMapper = CLDFMapper{}

// MapPaged parses values.csv (classification → parent, aes → status) then languages.csv (the languoid
// rows), builds the parent-first canonical records via the shared orderAndBuild, and emits them as one
// page (one envelope = one transaction).
func (CLDFMapper) MapPaged(ctx context.Context, staged domain.StagedSource, emit domain.PageFunc) error {
	parent, status, err := readValues(ctx, filepath.Join(staged.Path, "values.csv"))
	if err != nil {
		return err
	}
	langs, err := readLanguages(ctx, filepath.Join(staged.Path, "languages.csv"), parent, status)
	if err != nil {
		return err
	}
	records, err := orderAndBuild(langs)
	if err != nil {
		return err
	}
	return emit(records)
}

// readValues extracts parent (glottocode → immediate parent glottocode, the last classification path
// segment) and status (glottocode → AES endangerment) from the CLDF values.csv.
func readValues(ctx context.Context, path string) (parent, status map[string]string, err error) {
	parent = map[string]string{}
	status = map[string]string{}
	err = eachCSVRow(ctx, path, func(col func(string) string) error {
		switch col("Parameter_ID") {
		case "classification":
			if v := strings.TrimSpace(col("Value")); v != "" {
				segs := strings.Split(v, "/")
				parent[col("Language_ID")] = segs[len(segs)-1]
			}
		case "aes":
			if code := col("Code_ID"); strings.HasPrefix(code, "aes-") {
				status[col("Language_ID")] = strings.TrimPrefix(code, "aes-")
			}
		}
		return nil
	})
	return parent, status, err
}

// readLanguages builds the rawLanguoid set from the CLDF languages.csv, attaching the parent/status
// derived from values.csv (default status not_endangered, mirroring gen-presets.py).
func readLanguages(ctx context.Context, path string, parent, status map[string]string) ([]rawLanguoid, error) {
	var out []rawLanguoid
	err := eachCSVRow(ctx, path, func(col func(string) string) error {
		id := col("ID")
		l := rawLanguoid{Code: id, Level: col("Level"), Name: col("Name")}
		if p, ok := parent[id]; ok {
			l.Parent = p
		}
		if iso := strings.ToLower(strings.TrimSpace(col("ISO639P3code"))); iso != "" {
			l.ISO639_3 = iso
		}
		l.Macroarea = strings.TrimSpace(col("Macroarea"))
		if f, ok := parseFloat(col("Latitude")); ok {
			l.Latitude = &f
		}
		if f, ok := parseFloat(col("Longitude")); ok {
			l.Longitude = &f
		}
		if s, ok := status[id]; ok {
			l.Status = s
		} else {
			l.Status = "not_endangered"
		}
		for c := range strings.SplitSeq(col("Countries"), ";") {
			if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
				l.Countries = append(l.Countries, c)
			}
		}
		out = append(out, l)
		return nil
	})
	return out, err
}

// eachCSVRow streams a header-keyed CSV, calling fn with a column accessor for each data row. It bounds
// memory (no ReadAll) — values.csv is large — and is cancellable.
func eachCSVRow(ctx context.Context, path string, fn func(col func(string) string) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("language-scheme: open %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("language-scheme: read header of %s: %w", filepath.Base(path), err)
	}
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	n := 0
	for {
		if n%4096 == 0 && ctx.Err() != nil {
			return ctx.Err()
		}
		n++
		row, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("language-scheme: read %s: %w", filepath.Base(path), err)
		}
		col := func(name string) string {
			if i, ok := idx[name]; ok && i < len(row) {
				return row[i]
			}
			return ""
		}
		if err := fn(col); err != nil {
			return err
		}
	}
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
