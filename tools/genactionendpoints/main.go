// Command genactionendpoints derives the per-action HTTP endpoint binding (review-2026-09 R-33 seam,
// the action-invocation backbone) from the Conjure IR and pkg/action's catalog, writing
// pkg/action/endpoints_gen.go. It is the invocation counterpart of genactionparams: where that single-
// sources an action's *parameter shape* from its request type, this single-sources the *endpoint*
// (method + path + path params) an invocation must POST/PUT/DELETE to — so the console's generic action
// runner never hand-authors a URL and cannot drift from the contract.
//
// The join: an action's RequestType (the body type) pins its endpoint by body reference; the action
// code's verb (create→POST, update→PUT, delete→DELETE) and its tokens (matched against the endpoint
// name + path segments) disambiguate the create/update pairs that share a body; body-less actions
// (deletes, lifecycle POSTs) are matched within their module's path prefix. An action that resolves to
// zero or more-than-one endpoint is a HARD ERROR (non-zero exit) unless pinned in the override map —
// so a contract change that breaks a binding fails the generator (and CI), the drift guard R-33 buys.
//
// Run via scripts/gen-action-params.sh (which dumps a fresh IR first). Never hand-edit the output.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Minimal IR shapes (only the fields we read).
type ir struct {
	Services []struct {
		ServiceName struct {
			Name    string `json:"name"`
			Package string `json:"package"`
		} `json:"serviceName"`
		Endpoints []struct {
			EndpointName string `json:"endpointName"`
			HTTPMethod   string `json:"httpMethod"`
			HTTPPath     string `json:"httpPath"`
			Args         []struct {
				ArgName   string `json:"argName"`
				ParamType struct {
					Type string `json:"type"`
				} `json:"paramType"`
				Type struct {
					Type      string `json:"type"`
					Reference *struct {
						Name    string `json:"name"`
						Package string `json:"package"`
					} `json:"reference"`
				} `json:"type"`
			} `json:"args"`
		} `json:"endpoints"`
	} `json:"services"`
}

type endpoint struct {
	name, method, path, body string
	pathParams               []string
}

type action struct {
	code, rt, svc string
}

// override pins actions the automatic matcher can't uniquely resolve. Keyed by action code →
// "METHOD PATH". The matcher ties when the object noun equals the module name (every /company/v1/*
// path contains "company", so it can't tell deleteCompany from deleteRegistration) — these are pinned
// by hand and still IR-validated: the generator asserts the pinned METHOD+PATH names a real endpoint.
var override = map[string]string{
	"company.delete":             "DELETE /company/v1/companies/{companyId}",
	"document.delete":            "DELETE /document/v1/documents/{documentId}",
	"education.group.delete":     "DELETE /education/v1/groups/{groupId}",
	"person.relationship.delete": "DELETE /person/v1/persons/{personId}/relationships/{relationshipId}",
	"translation.upsert":         "PUT /localization/v1/translations/{entityType}/{entityId}",
}

// exempt lists audited actions with NO invocable HTTP endpoint — so the generator neither binds them
// nor errors on them, and the runner shows them as non-invocable (the analog of links.md's exempt link
// types). Two classes: purge-cascade erasures (emitted internally on PersonPurged, no endpoint) and the
// bulk-ingestion import.* actions (the hermenea import plane — POST /import/v1/import/{objectType} takes
// a bulk envelope, not a per-object console action).
var exempt = map[string]string{
	"document.person.erase":              "purge-cascade: erases the person's documents on PersonPurged, no endpoint",
	"finance.holdings.erase":             "purge-cascade: erases the person's finance holdings on PersonPurged, no endpoint",
	"vehicle.registrations.erase":        "purge-cascade: erases the person's vehicle registrations on PersonPurged, no endpoint",
	"person.ethnicity.erase":             "purge-cascade: crypto-erases declared ethnicity on PersonPurged, no endpoint",
	"religion.affiliation.erase":         "purge-cascade: erases religious affiliation on PersonPurged, no endpoint",
	"import.colors":                      "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.ethnicity-scheme":            "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.external-organizations":      "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.geo-countries":               "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.geo-places":                  "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.language-scheme":             "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.language-scripts":            "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.person-regulatory-sanctions": "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.religion-scheme":             "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.translations":                "bulk ingestion (hermenea import plane), not a per-object console action",
	"import.locales":                     "bulk ingestion (locale packs, M54), not a per-object console action",
	"connector.register":                 "machine self-service (M53 connector plane), not a console object action",
	"connector.sync-run.running":         "machine self-service (M53 connector plane), not a console object action",
	"connector.sync-run.succeeded":       "machine self-service (M53 connector plane), not a console object action",
	"connector.sync-run.failed":          "machine self-service (M53 connector plane), not a console object action",
}

// sharedEndpoints are (method path) templates legitimately bound by more than one action — a single
// generic endpoint discriminated by a path param value. The rank scheme delete is one endpoint keyed by
// {level} (category/type/rank/system). Anything NOT listed here that binds twice is a mis-match.
var sharedEndpoints = map[string]bool{
	"DELETE /rank/v1/rank-scheme/{level}/{nodeId}": true, // rank.{category,type,rank,system}.delete
}

var actionRe = regexp.MustCompile(`\{Code:\s*"([^"]+)",\s*Service:\s*rid\.(\w+),\s*TargetType:\s*"[^"]+",\s*Permission:\s*"[^"]+"(?:,\s*RequestType:\s*"([^"]+)")?\}`)

func main() {
	irPath := flag.String("ir", "", "path to Conjure IR JSON (from ir2openapi -dump-ir)")
	regPath := flag.String("registry", "pkg/action/registry.go", "path to the action catalog source")
	out := flag.String("out", "pkg/action/endpoints_gen.go", "output Go file")
	flag.Parse()
	if *irPath == "" {
		log.Fatal("genactionendpoints: -ir is required")
	}

	raw, err := os.ReadFile(*irPath)
	if err != nil {
		log.Fatalf("genactionendpoints: read IR: %v", err)
	}
	var doc ir
	if err := json.Unmarshal(raw, &doc); err != nil {
		log.Fatalf("genactionendpoints: parse IR: %v", err)
	}

	// Flatten the IR into an endpoint list + an index of every real "METHOD PATH" (for override checks).
	var eps []endpoint
	realKey := map[string]bool{}
	for _, s := range doc.Services {
		for _, e := range s.Endpoints {
			ep := endpoint{name: e.EndpointName, method: e.HTTPMethod, path: e.HTTPPath}
			for _, a := range e.Args {
				switch a.ParamType.Type {
				case "path":
					ep.pathParams = append(ep.pathParams, a.ArgName)
				case "body":
					if a.Type.Type == "reference" && a.Type.Reference != nil {
						ep.body = a.Type.Reference.Package + "." + a.Type.Reference.Name
					}
				}
			}
			eps = append(eps, ep)
			realKey[e.HTTPMethod+" "+e.HTTPPath] = true
		}
	}

	regSrc, err := os.ReadFile(*regPath)
	if err != nil {
		log.Fatalf("genactionendpoints: read registry: %v", err)
	}
	var acts []action
	for _, m := range actionRe.FindAllStringSubmatch(string(regSrc), -1) {
		acts = append(acts, action{code: m[1], svc: m[2], rt: m[3]})
	}
	if len(acts) == 0 {
		log.Fatal("genactionendpoints: parsed 0 actions from registry — matcher regex out of date?")
	}

	byBody := map[string][]endpoint{}
	for _, e := range eps {
		if e.body != "" {
			byBody[e.body] = append(byBody[e.body], e)
		}
	}

	// Module path prefix (e.g. "/person/v1") per RID-service const, derived from the bodied actions that
	// already resolve — so body-less actions are matched only within their own module, no hardcoding.
	resolved := map[string]endpoint{}
	svcPrefixes := map[string]map[string]bool{}

	// Pass 1: bodied actions (join by body type, disambiguate by verb+tokens).
	var unresolved []string
	for _, a := range acts {
		if a.rt == "" {
			continue
		}
		if _, ok := exempt[a.code]; ok {
			continue
		}
		if pin, ok := override[a.code]; ok {
			resolved[a.code] = mustPin(a.code, pin, realKey, eps)
			addPrefix(svcPrefixes, a.svc, resolved[a.code].path)
			continue
		}
		cands := byBody[a.rt]
		e, ok := pick(a.code, cands)
		if !ok {
			unresolved = append(unresolved, fmt.Sprintf("%s (body %s → %d candidates)", a.code, a.rt, len(cands)))
			continue
		}
		resolved[a.code] = e
		addPrefix(svcPrefixes, a.svc, e.path)
	}

	// Pass 2: body-less actions (deletes, lifecycle) — candidates are the body-less endpoints inside the
	// action's module prefix, disambiguated by verb+tokens.
	for _, a := range acts {
		if a.rt != "" {
			continue
		}
		if _, ok := exempt[a.code]; ok {
			continue
		}
		if pin, ok := override[a.code]; ok {
			resolved[a.code] = mustPin(a.code, pin, realKey, eps)
			continue
		}
		var cands []endpoint
		for _, e := range eps {
			if e.body != "" {
				continue
			}
			if inPrefix(svcPrefixes[a.svc], e.path) {
				cands = append(cands, e)
			}
		}
		e, ok := pick(a.code, cands)
		if !ok {
			unresolved = append(unresolved, fmt.Sprintf("%s (body-less, svc %s → %d candidates in prefix)", a.code, a.svc, len(cands)))
			continue
		}
		resolved[a.code] = e
	}

	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		log.Fatalf("genactionendpoints: %d action(s) did not resolve to exactly one endpoint — pin them in override (or add to exempt):\n  %s",
			len(unresolved), strings.Join(unresolved, "\n  "))
	}

	// Duplicate-binding guard: two actions binding the same endpoint means a mis-match (e.g. a cascade
	// *.erase auto-binding to the *.delete endpoint it shares no verb with). Catch it — the offender is
	// either a real endpoint that needs a distinct binding, or a no-endpoint action that belongs in exempt.
	seen := map[string][]string{}
	for code, e := range resolved {
		k := e.method + " " + e.path
		seen[k] = append(seen[k], code)
	}
	var dups []string
	for k, codes := range seen {
		if len(codes) > 1 && !sharedEndpoints[k] {
			sort.Strings(codes)
			dups = append(dups, fmt.Sprintf("%s ← %s", k, strings.Join(codes, ", ")))
		}
	}
	if len(dups) > 0 {
		sort.Strings(dups)
		log.Fatalf("genactionendpoints: %d endpoint(s) bound by more than one action (likely mis-match — pin or exempt):\n  %s",
			len(dups), strings.Join(dups, "\n  "))
	}

	if err := os.WriteFile(*out, render(resolved), 0o644); err != nil {
		log.Fatalf("genactionendpoints: write %s: %v", *out, err)
	}
	fmt.Printf("genactionendpoints: wrote %s (%d actions bound)\n", *out, len(resolved))
}

// pick chooses the single best endpoint for a code, or (·,false) if the choice is not unique.
func pick(code string, cands []endpoint) (endpoint, bool) {
	if len(cands) == 1 {
		return cands[0], true
	}
	if len(cands) == 0 {
		return endpoint{}, false
	}
	best, bestScore, tie := endpoint{}, -1, false
	for _, e := range cands {
		s := score(code, e)
		switch {
		case s > bestScore:
			best, bestScore, tie = e, s, false
		case s == bestScore:
			tie = true
		}
	}
	if tie {
		return endpoint{}, false
	}
	return best, true
}

var verbMethod = map[string]string{
	"create": "POST", "add": "POST", "record": "POST", "register": "POST", "grant": "POST",
	"link": "POST", "fill": "POST", "assign": "POST", "issue": "POST", "merge": "POST",
	"provisional": "POST", "import": "POST", "check": "POST", "reparent": "POST",
	"update": "PUT", "set": "PUT", "recode": "PUT",
	"delete": "DELETE", "remove": "DELETE", "revoke": "DELETE", "unlink": "DELETE",
	"disable": "DELETE", "erase": "DELETE",
}

// score ranks an endpoint for an action code: token overlap (code segments vs endpoint name + path
// segments) plus a bonus when the code's leaf verb matches the endpoint's HTTP method.
func score(code string, e endpoint) int {
	seg := splitTokens(code)
	hay := splitTokens(e.name + " " + e.path)
	set := map[string]bool{}
	for _, t := range hay {
		set[t] = true
	}
	s := 0
	for _, t := range seg {
		if set[t] {
			s++
		}
	}
	s *= 10
	if len(seg) > 0 {
		if m, ok := verbMethod[seg[len(seg)-1]]; ok && m == e.method {
			s += 5
		}
	}
	return s
}

// splitTokens lowercases and splits on separators and camelCase humps.
func splitTokens(s string) []string {
	// Insert a space at camelCase humps, then split on the separator class.
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := s[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteByte(' ')
			}
		}
		b.WriteRune(r)
	}
	fields := regexp.MustCompile(`[._/\-{} ]+`).Split(strings.ToLower(b.String()), -1)
	var out []string
	for _, f := range fields {
		if f != "" && f != "v1" && f != "v2" {
			out = append(out, f)
		}
	}
	return out
}

func addPrefix(m map[string]map[string]bool, svc, path string) {
	if m[svc] == nil {
		m[svc] = map[string]bool{}
	}
	m[svc][modulePrefix(path)] = true
}

func inPrefix(set map[string]bool, path string) bool {
	return set != nil && set[modulePrefix(path)]
}

// modulePrefix is the "/<module>/v1" head of a Conjure path.
func modulePrefix(path string) string {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) >= 2 {
		return "/" + parts[0] + "/" + parts[1]
	}
	return path
}

func mustPin(code, pin string, real map[string]bool, eps []endpoint) endpoint {
	if !real[pin] {
		log.Fatalf("genactionendpoints: override %q → %q names no real endpoint", code, pin)
	}
	sp := strings.SplitN(pin, " ", 2)
	for _, e := range eps {
		if e.method == sp[0] && e.path == sp[1] {
			return e
		}
	}
	log.Fatalf("genactionendpoints: override %q unreachable", code) // unreachable given real[pin]
	return endpoint{}
}

func render(resolved map[string]endpoint) []byte {
	codes := make([]string, 0, len(resolved))
	for c := range resolved {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	var b bytes.Buffer
	b.WriteString("// Code generated by tools/genactionendpoints from the Conjure IR. DO NOT EDIT.\n")
	b.WriteString("// Regenerate with scripts/gen-action-params.sh (review-2026-09 R-33 action-invocation seam).\n\n")
	b.WriteString("package action\n\n")
	b.WriteString("// actionEndpoints binds each action code to the HTTP endpoint an invocation targets, single-\n")
	b.WriteString("// sourced from the Conjure IR. PathParams lists the endpoint's path params in order; the first is\n")
	b.WriteString("// the target object's own RID, any remainder are sub-resource ids the caller must supply.\n")
	b.WriteString("var actionEndpoints = map[string]Endpoint{\n")
	for _, c := range codes {
		e := resolved[c]
		pp := "nil"
		if len(e.pathParams) > 0 {
			qs := make([]string, len(e.pathParams))
			for i, p := range e.pathParams {
				qs[i] = fmt.Sprintf("%q", p)
			}
			pp = "[]string{" + strings.Join(qs, ", ") + "}"
		}
		b.WriteString(fmt.Sprintf("\t%q: {Method: %q, Path: %q, PathParams: %s},\n", c, e.method, e.path, pp))
	}
	b.WriteString("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatalf("genactionendpoints: gofmt: %v\n%s", err, b.String())
	}
	return src
}
