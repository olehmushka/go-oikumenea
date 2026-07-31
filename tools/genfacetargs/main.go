// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Command genfacetargs derives, from the Conjure IR, the query-arg set of every list endpoint the
// facet catalog binds to, writing pkg/facet/args_gen.go (M56 / D-ObjectFacets).
//
// It is the facet counterpart of genactionparams/genactionendpoints: where those single-source an
// action's parameter shape and its endpoint from the contract, this single-sources what the LIST
// endpoints actually ship. pkg/facet/args_test.go then compares the generated mirror against the
// hand-written catalog in BOTH directions — a declared facet with no query arg, or a query arg bound
// to neither a facet nor a classified non-facet role, fails the build. Neither side can be the sole
// authority: the catalog says what the vocabulary is, the IR says what the API is, and drift between
// them is exactly the failure this closes.
//
// Hard errors (non-zero exit, so a contract change breaks the generator and CI rather than silently
// producing a stale map): a ListEndpoint that names no real service+endpoint, an endpoint that ships
// zero query args, a duplicate arg name, or a catalog that parses to zero object types.
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

// Minimal IR shapes (only the fields we read). `optional<T>` nests the real type under itemType.
type typeNode struct {
	Type     string `json:"type"`
	Optional *struct {
		ItemType typeNodeInner `json:"itemType"`
	} `json:"optional"`
	Primitive string `json:"primitive"`
	Reference *struct {
		Name    string `json:"name"`
		Package string `json:"package"`
	} `json:"reference"`
}

type typeNodeInner struct {
	Type      string `json:"type"`
	Primitive string `json:"primitive"`
	Reference *struct {
		Name    string `json:"name"`
		Package string `json:"package"`
	} `json:"reference"`
}

type ir struct {
	Services []struct {
		ServiceName struct {
			Name string `json:"name"`
		} `json:"serviceName"`
		Endpoints []struct {
			EndpointName string `json:"endpointName"`
			Args         []struct {
				ArgName   string   `json:"argName"`
				Type      typeNode `json:"type"`
				ParamType struct {
					Type string `json:"type"`
				} `json:"paramType"`
			} `json:"args"`
		} `json:"endpoints"`
	} `json:"services"`
}

// argSpec is one query arg as the contract actually ships it.
type argSpec struct {
	name     string
	typ      string // display token: string | integer | boolean | double | datetime | <reference>
	optional bool
}

// bindingRe extracts the (object type -> list endpoint) bindings from the catalog source, the way
// genactionendpoints parses pkg/action/registry.go. Deliberately strict: it requires the three fields
// in declaration order, so a reformat that breaks it trips the zero-bindings guard below rather than
// silently dropping a type.
// typeDeclRe counts the object types the catalog DECLARES, so a binding the strict regex misses is a
// hard error rather than a silently thinner mirror.
var typeDeclRe = regexp.MustCompile(`(?m)^\s*Type:\s*"[a-z_][a-z0-9_]*",`)

var bindingRe = regexp.MustCompile(`Type:\s*"([a-z_][a-z0-9_]*)",\s*Module:\s*"([a-z][a-z0-9_-]*)",\s*ListEndpoint:\s*"([A-Za-z]+)\.([A-Za-z][A-Za-z0-9]*)",` +
	`(?:\s*StatsEndpoint:\s*"([A-Za-z]+)\.([A-Za-z][A-Za-z0-9]*)",)?`)

func main() {
	irPath := flag.String("ir", "", "path to Conjure IR JSON (from ir2openapi -dump-ir)")
	catPath := flag.String("catalog", "pkg/facet/catalog.go", "path to the facet catalog source")
	out := flag.String("out", "pkg/facet/args_gen.go", "output Go file")
	flag.Parse()
	if *irPath == "" {
		log.Fatal("genfacetargs: -ir is required")
	}

	raw, err := os.ReadFile(*irPath)
	if err != nil {
		log.Fatalf("genfacetargs: read IR: %v", err)
	}
	var doc ir
	if err := json.Unmarshal(raw, &doc); err != nil {
		log.Fatalf("genfacetargs: parse IR: %v", err)
	}

	catSrc, err := os.ReadFile(*catPath)
	if err != nil {
		log.Fatalf("genfacetargs: read catalog: %v", err)
	}
	matches := bindingRe.FindAllStringSubmatch(string(catSrc), -1)
	if len(matches) == 0 {
		log.Fatalf("genfacetargs: parsed 0 object types from %s — matcher regex out of date?", *catPath)
	}
	// A PARTIAL miss is the dangerous one: zero bindings is loud, but one type silently skipped (a
	// field slipped between Type: and Module:, which the strict regex requires adjacent) writes a
	// mirror that looks healthy and quietly stops checking that type. Count the Type: declarations
	// and demand they all matched.
	if declared := len(typeDeclRe.FindAllString(string(catSrc), -1)); declared != len(matches) {
		log.Fatalf("genfacetargs: %s declares %d object types but only %d matched the binding regex — "+
			"a field between Type:, Module: and ListEndpoint: breaks the match and would drop a type "+
			"from the mirror", *catPath, declared, len(matches))
	}

	// Index the IR by "Service.endpoint".
	type key struct{ svc, ep string }
	queryArgs := map[key][]argSpec{}
	for _, s := range doc.Services {
		for _, e := range s.Endpoints {
			k := key{s.ServiceName.Name, e.EndpointName}
			for _, a := range e.Args {
				if a.ParamType.Type != "query" {
					continue
				}
				queryArgs[k] = append(queryArgs[k], argSpec{
					name:     a.ArgName,
					typ:      typeToken(a.Type),
					optional: a.Type.Type == "optional",
				})
			}
		}
	}

	type binding struct {
		objectType string
		args       []argSpec
		statsArgs  []argSpec // nil when the type has no stats endpoint yet (M57 rolls them out)
	}
	var bindings []binding
	seenType := map[string]bool{}
	for _, m := range matches {
		objectType, svc, ep := m[1], m[3], m[4]
		statsSvc, statsEp := m[5], m[6]
		if seenType[objectType] {
			log.Fatalf("genfacetargs: object type %q is declared twice in %s", objectType, *catPath)
		}
		seenType[objectType] = true

		args, ok := queryArgs[key{svc, ep}]
		if !ok {
			log.Fatalf("genfacetargs: %s binds ListEndpoint %s.%s, which the Conjure IR does not contain "+
				"(renamed or removed?) — fix the catalog or the contract", objectType, svc, ep)
		}
		if len(args) == 0 {
			log.Fatalf("genfacetargs: %s binds %s.%s, which ships NO query args — a facet vocabulary over it "+
				"could never be expressed", objectType, svc, ep)
		}
		seenArg := map[string]bool{}
		for _, a := range args {
			if seenArg[a.name] {
				log.Fatalf("genfacetargs: %s.%s declares query arg %q twice", svc, ep, a.name)
			}
			seenArg[a.name] = true
		}
		sort.Slice(args, func(i, j int) bool { return args[i].name < args[j].name })

		// The M57 stats endpoint, when the type has one. Same hard-error discipline: a StatsEndpoint
		// naming something the IR does not contain is a stale catalog, not a warning.
		var stats []argSpec
		if statsEp != "" {
			stats, ok = queryArgs[key{statsSvc, statsEp}]
			if !ok {
				log.Fatalf("genfacetargs: %s binds StatsEndpoint %s.%s, which the Conjure IR does not "+
					"contain (renamed or removed?) — fix the catalog or the contract", objectType, statsSvc, statsEp)
			}
			if len(stats) == 0 {
				log.Fatalf("genfacetargs: %s binds stats endpoint %s.%s, which ships NO query args — it "+
					"could not take the list's filters", objectType, statsSvc, statsEp)
			}
			sort.Slice(stats, func(i, j int) bool { return stats[i].name < stats[j].name })
		}
		bindings = append(bindings, binding{objectType: objectType, args: args, statsArgs: stats})
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].objectType < bindings[j].objectType })

	var b bytes.Buffer
	b.WriteString("// Copyright 2026 Oleh Mushka\n")
	b.WriteString("// SPDX-License-Identifier: Apache-2.0\n\n")
	b.WriteString("// Code generated by tools/genfacetargs from the Conjure IR. DO NOT EDIT.\n")
	b.WriteString("// Regenerate with scripts/gen-action-params.sh (M56 / D-ObjectFacets facet-arg drift guard).\n\n")
	b.WriteString("package facet\n\n")
	b.WriteString("// ArgSpec is one query arg as the CONTRACT ships it — the mirror the hand-written catalog is\n")
	b.WriteString("// checked against. Type is the Conjure display token (string, integer, boolean, ...).\n")
	b.WriteString("type ArgSpec struct {\n\tName     string\n\tType     string\n\tOptional bool\n}\n\n")
	b.WriteString("// listArgs is every param-type=query arg on each registered object type's list endpoint,\n")
	b.WriteString("// keyed by the object type token and sorted by arg name.\n")
	b.WriteString("var listArgs = map[string][]ArgSpec{\n")
	for _, bd := range bindings {
		b.WriteString(fmt.Sprintf("\t%q: {\n", bd.objectType))
		for _, a := range bd.args {
			b.WriteString(fmt.Sprintf("\t\t{Name: %q, Type: %q, Optional: %t},\n", a.name, a.typ, a.optional))
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("// statsArgs is every param-type=query arg on each registered object type's M57 STATS\n")
	b.WriteString("// endpoint, keyed by the object type token. A type absent here has no stats endpoint yet;\n")
	b.WriteString("// args_test.go holds that against an explicit pending list rather than letting it pass unnoticed.\n")
	b.WriteString("var statsArgs = map[string][]ArgSpec{\n")
	for _, bd := range bindings {
		if len(bd.statsArgs) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\t%q: {\n", bd.objectType))
		for _, a := range bd.statsArgs {
			b.WriteString(fmt.Sprintf("\t\t{Name: %q, Type: %q, Optional: %t},\n", a.name, a.typ, a.optional))
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatalf("genfacetargs: gofmt: %v\n%s", err, b.String())
	}
	if err := os.WriteFile(*out, src, 0o644); err != nil {
		log.Fatalf("genfacetargs: write %s: %v", *out, err)
	}
	fmt.Printf("genfacetargs: wrote %s (%d object types)\n", *out, len(bindings))
}

// typeToken renders a Conjure arg type as the lowercase display token the guard compares against,
// unwrapping optional<T> to T (optionality is carried separately).
func typeToken(t typeNode) string {
	if t.Type == "optional" && t.Optional != nil {
		return innerToken(t.Optional.ItemType)
	}
	return innerToken(typeNodeInner{Type: t.Type, Primitive: t.Primitive, Reference: t.Reference})
}

func innerToken(t typeNodeInner) string {
	switch t.Type {
	case "primitive":
		return strings.ToLower(t.Primitive)
	case "reference":
		if t.Reference != nil {
			return t.Reference.Package + "." + t.Reference.Name
		}
	}
	return t.Type
}
