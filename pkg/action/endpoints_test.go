package action

import (
	"regexp"
	"strings"
	"testing"
)

// nonInvocable pins the actions that intentionally have NO endpoint binding (D-ActionInvocation,
// review-2026-09 R-33): purge-cascade erasures (emitted internally on PersonPurged) and the bulk
// import.* ingestion plane. This is the test-side mirror of tools/genactionendpoints's `exempt` set —
// a new action that lands without an endpoint fails TestActionEndpointCoverage until it is either wired
// (regenerate) or added here with a reason, so the invocation catalog cannot silently under-report.
var nonInvocable = map[string]bool{
	"document.person.erase":              true, // purge-cascade, no endpoint
	"finance.holdings.erase":             true, // purge-cascade, no endpoint
	"vehicle.registrations.erase":        true, // purge-cascade, no endpoint
	"person.ethnicity.erase":             true, // purge-cascade (crypto-erase), no endpoint
	"religion.affiliation.erase":         true, // purge-cascade, no endpoint
	"import.colors":                      true, // bulk ingestion plane
	"import.ethnicity-scheme":            true, // bulk ingestion plane
	"import.external-organizations":      true, // bulk ingestion plane
	"import.geo-countries":               true, // bulk ingestion plane
	"import.geo-places":                  true, // bulk ingestion plane
	"import.language-scheme":             true, // bulk ingestion plane
	"import.language-scripts":            true, // bulk ingestion plane
	"import.locales":                     true, // bulk ingestion plane (locale packs, M54)
	"import.person-regulatory-sanctions": true, // bulk ingestion plane
	"import.religion-scheme":             true, // bulk ingestion plane
	"import.translations":                true, // bulk ingestion plane
	"connector.register":                 true, // machine self-service (M53), not a console object action
	"connector.sync-run.running":         true, // machine self-service (M53), not a console object action
	"connector.sync-run.succeeded":       true, // machine self-service (M53), not a console object action
	"connector.sync-run.failed":          true, // machine self-service (M53), not a console object action
}

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

// TestActionEndpointCoverage is the drift guard for the invocation seam: every action is either
// invocable (has an EndpointFor binding) or an intentionally non-invocable one (nonInvocable). A new
// action with no binding — because the generator couldn't resolve it or it truly has no endpoint —
// fails here until it is wired or exempted. actionEndpoints is generated from the Conjure IR by
// tools/genactionendpoints (scripts/gen-action-params.sh).
func TestActionEndpointCoverage(t *testing.T) {
	for _, a := range All() {
		_, bound := EndpointFor(a.Code)
		exempt := nonInvocable[a.Code]
		switch {
		case bound && exempt:
			t.Errorf("%s: bound to an endpoint but listed non-invocable — remove it from nonInvocable", a.Code)
		case !bound && !exempt:
			t.Errorf("%s: no endpoint binding and not in nonInvocable — run scripts/gen-action-params.sh, or add it with a reason", a.Code)
		}
	}
	// Guard the exempt list against staleness: every pinned non-invocable code is a real action.
	for code := range nonInvocable {
		if _, ok := Lookup(code); !ok {
			t.Errorf("nonInvocable lists %q which is not a registered action", code)
		}
	}
}

// TestActionEndpointsWellFormed: each binding has a sane method + absolute path, and its PathParams
// exactly match the {…} placeholders in the path (in order) — so the runner's substitution never leaves
// an unfilled placeholder or references a param the path doesn't have.
func TestActionEndpointsWellFormed(t *testing.T) {
	methods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true}
	for _, a := range All() {
		e, ok := EndpointFor(a.Code)
		if !ok {
			continue
		}
		if !methods[e.Method] {
			t.Errorf("%s: bad method %q", a.Code, e.Method)
		}
		if !strings.HasPrefix(e.Path, "/") {
			t.Errorf("%s: path %q is not absolute", a.Code, e.Path)
		}
		var inPath []string
		for _, m := range pathParamRe.FindAllStringSubmatch(e.Path, -1) {
			inPath = append(inPath, m[1])
		}
		if strings.Join(inPath, ",") != strings.Join(e.PathParams, ",") {
			t.Errorf("%s: PathParams %v do not match path placeholders %v", a.Code, e.PathParams, inPath)
		}
	}
}
