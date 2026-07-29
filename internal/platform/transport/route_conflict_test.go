// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The route-conflict guard.
//
// witchcraft serves on julienschmidt/httprouter (wrouter/whttprouter), whose radix tree does NOT
// support a literal path segment beside a wildcard at the same position: registering
// `GET /persons/stats` next to `GET /persons/{personId}` PANICS at registration —
//
//	'stats' in new path '/persons/stats' conflicts with existing wildcard ':personId'
//
// which means server startup, not a request. A contract change can therefore make the binary
// unbootable while `go build`, `make generate` and every unit test stay green, and the failure would
// first appear when the process refuses to start.
//
// M57 hit exactly this: D-ObjectFacets specifies the stats endpoint as
// `/<module>/v1/<collection>/stats`, which is unroutable beside `/<collection>/{id}` — hence
// `/stats/<collection>`, recorded in the decision's as-built note. This test is what keeps the next
// such endpoint from being written the unroutable way.
//
// It parses api/*.conjure.yml directly (the plaintext_test technique, applied to YAML) rather than
// the Conjure IR, so it needs no generator run: it must be able to fail in the same commit that
// introduces the conflict.

var (
	basePathRe = regexp.MustCompile(`(?m)^\s*base-path:\s*(\S+)\s*$`)
	httpRe     = regexp.MustCompile(`(?m)^\s*http:\s+([A-Z]+)\s+(\S+)\s*$`)
)

// TestNoRouteWildcardConflicts asserts that no two registered paths put a literal and a wildcard at
// the same tree position under the same prefix — the exact condition httprouter refuses.
func TestNoRouteWildcardConflicts(t *testing.T) {
	routes := allRoutes(t)
	if len(routes) < 100 {
		t.Fatalf("only %d routes parsed from api/*.conjure.yml — the parser is broken, and every "+
			"assertion below would pass vacuously", len(routes))
	}

	// (method, prefix) -> the next segment seen there, and one example route that put it there.
	// Keyed by METHOD because httprouter keeps ONE TREE PER METHOD: `DELETE /rank-scheme/{level}/…`
	// coexists with `POST /rank-scheme/systems` and always has. Only same-method siblings collide.
	type seg struct{ literal, wildcard string }
	at := map[string]*seg{}
	for _, r := range routes {
		p := r.method + " " + r.path
		segs := strings.Split(strings.Trim(r.path, "/"), "/")
		for i := 1; i <= len(segs); i++ {
			prefix := r.method + " /" + strings.Join(segs[:i-1], "/")
			s, ok := at[prefix]
			if !ok {
				s = &seg{}
				at[prefix] = s
			}
			if strings.HasPrefix(segs[i-1], "{") {
				if s.wildcard == "" {
					s.wildcard = p
				}
			} else if s.literal == "" {
				s.literal = p
			}
		}
	}
	var bad []string
	for prefix, s := range at {
		if s.literal != "" && s.wildcard != "" {
			bad = append(bad, prefix+": literal in "+s.literal+" vs wildcard in "+s.wildcard)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Errorf("routes that httprouter will refuse to register (the server would PANIC at boot):\n  %s\n"+
			"A literal segment cannot sit beside a {param} at the same position. Move the literal to its "+
			"own prefix — /stats/persons, not /persons/stats.", strings.Join(bad, "\n  "))
	}
}

// TestGuardCatchesTheKnownConflictShape is the live negative: the detector must go red on the exact
// pair M57 discovered, or its silence above means nothing.
func TestGuardCatchesTheKnownConflictShape(t *testing.T) {
	if !hasConflict([]route{{"GET", "/person/v1/persons/{personId}"}, {"GET", "/person/v1/persons/stats"}}) {
		t.Error("the detector does not flag /persons/stats beside GET /persons/{personId} — it is vacuous")
	}
	if hasConflict([]route{{"GET", "/person/v1/persons/{personId}"}, {"GET", "/person/v1/stats/persons"}}) {
		t.Error("the detector flags the SHIPPED shape (/stats/persons), which httprouter accepts")
	}
	// One tree per method: the pre-existing rank pair is legal and must NOT be reported, or the guard
	// would be a standing red that teaches everyone to ignore it.
	if hasConflict([]route{{"POST", "/rank/v1/rank-scheme/systems"}, {"DELETE", "/rank/v1/rank-scheme/{level}/{nodeId}"}}) {
		t.Error("the detector flags a cross-METHOD pair; httprouter keeps one tree per method")
	}
}

func hasConflict(routes []route) bool {
	type seg struct{ literal, wildcard bool }
	at := map[string]*seg{}
	for _, r := range routes {
		segs := strings.Split(strings.Trim(r.path, "/"), "/")
		for i := 1; i <= len(segs); i++ {
			prefix := r.method + " /" + strings.Join(segs[:i-1], "/")
			s, ok := at[prefix]
			if !ok {
				s = &seg{}
				at[prefix] = s
			}
			if strings.HasPrefix(segs[i-1], "{") {
				s.wildcard = true
			} else {
				s.literal = true
			}
		}
	}
	for _, s := range at {
		if s.literal && s.wildcard {
			return true
		}
	}
	return false
}

// route is one declared endpoint: its method and its FULL path (base-path + endpoint path). Both
// matter — the base path because the tree spans the whole server, the method because there is one
// tree per method.
type route struct{ method, path string }

func allRoutes(t *testing.T) []route {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "api")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []route
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conjure.yml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		// Walked line by line rather than matched file-wide: a file may declare SEVERAL services
		// (platform.conjure.yml does), and each endpoint belongs to the base-path most recently
		// declared above it. Prefixing every endpoint with the file's first base-path would compare
		// routes that never share a tree.
		base := ""
		for _, line := range strings.Split(string(raw), "\n") {
			if m := basePathRe.FindStringSubmatch(line); m != nil {
				base = strings.TrimRight(m[1], "/")
				continue
			}
			if m := httpRe.FindStringSubmatch(line); m != nil {
				out = append(out, route{method: m[1], path: base + m[2]})
			}
		}
	}
	return out
}
