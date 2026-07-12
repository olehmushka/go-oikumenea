package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRewrapTablesMatchSchema guards the rewrap registry (review R-22): every table in migrations that
// has a *_wrapped_dek column MUST appear in encTables, and vice versa. Without this, a future
// envelope-encrypted table would silently miss key rotation — exactly the gap R-22 closed. It parses the
// migration SQL directly (no DB), tracking the enclosing CREATE TABLE for each wrapped_dek column.
func TestRewrapTablesMatchSchema(t *testing.T) {
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	createRe := regexp.MustCompile(`CREATE TABLE (?:IF NOT EXISTS )?(oikumenea\.[a-z_]+)`)
	wrappedRe := regexp.MustCompile(`^\s*[a-z_]*wrapped_dek\s+bytea`)

	fromSchema := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var current string
		for _, line := range strings.Split(string(body), "\n") {
			if m := createRe.FindStringSubmatch(line); m != nil {
				current = m[1]
			}
			if wrappedRe.MatchString(line) && current != "" {
				fromSchema[current] = struct{}{}
			}
		}
	}

	fromRegistry := map[string]struct{}{}
	for _, tbl := range encTables {
		fromRegistry[tbl.name] = struct{}{}
	}

	if diff := missing(fromSchema, fromRegistry); len(diff) > 0 {
		t.Fatalf("encrypted tables in migrations but MISSING from the rewrap registry (they would not rotate): %v", diff)
	}
	if diff := missing(fromRegistry, fromSchema); len(diff) > 0 {
		t.Fatalf("tables in the rewrap registry but not found in migrations (stale entry): %v", diff)
	}
}

func missing(want, have map[string]struct{}) []string {
	var out []string
	for k := range want {
		if _, ok := have[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
