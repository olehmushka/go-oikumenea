// Command genactionparams derives the per-action parameter schemas (review-2026-09 R-29 seam) from the
// Conjure IR and writes pkg/action/params_gen.go. It is the single-sourcing step: an action's argument
// shape comes from the Conjure request type that carries its inputs, NOT hand-authored, so it cannot
// drift from the contract. Emits one entry per request-body type used by any endpoint (keyed by the
// IR's package-qualified name, e.g. oikumenea.authorization.GrantAssignmentRequest), so a request type
// referenced by pkg/action's RequestType annotations always resolves.
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
	"sort"
	"strings"
)

// Minimal IR shapes (only the fields we read).
type ir struct {
	Types    []irType    `json:"types"`
	Services []irService `json:"services"`
}
type typeName struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}
type irType struct {
	Type   string `json:"type"`
	Object *struct {
		TypeName typeName  `json:"typeName"`
		Fields   []irField `json:"fields"`
	} `json:"object"`
	Enum *struct {
		TypeName typeName `json:"typeName"`
	} `json:"enum"`
	Alias *struct {
		TypeName typeName `json:"typeName"`
		Alias    typeNode `json:"alias"`
	} `json:"alias"`
}
type irField struct {
	FieldName string   `json:"fieldName"`
	Type      typeNode `json:"type"`
	Docs      string   `json:"docs"`
}
type typeNode struct {
	Type      string `json:"type"` // primitive | optional | list | set | map | reference | external
	Primitive string `json:"primitive"`
	Optional  *struct {
		ItemType typeNode `json:"itemType"`
	} `json:"optional"`
	List *struct {
		ItemType typeNode `json:"itemType"`
	} `json:"list"`
	Set *struct {
		ItemType typeNode `json:"itemType"`
	} `json:"set"`
	Map *struct {
		KeyType   typeNode `json:"keyType"`
		ValueType typeNode `json:"valueType"`
	} `json:"map"`
	Reference *typeName `json:"reference"`
}

type service struct {
	Endpoints []struct {
		Args []struct {
			ParamType struct {
				Type string `json:"type"`
			} `json:"paramType"`
			Type typeNode `json:"type"`
		} `json:"args"`
	} `json:"endpoints"`
}
type irService = service

type param struct {
	Name, Type, Docs string
	Required         bool
	Fields           []param // nested object's flat sub-fields (one level; nil for flat/deep/JSON params)
}

func main() {
	irPath := flag.String("ir", "", "path to Conjure IR JSON (from ir2openapi -dump-ir)")
	out := flag.String("out", "pkg/action/params_gen.go", "output Go file")
	flag.Parse()
	if *irPath == "" {
		log.Fatal("genactionparams: -ir is required")
	}
	raw, err := os.ReadFile(*irPath)
	if err != nil {
		log.Fatalf("genactionparams: read IR: %v", err)
	}
	var doc ir
	if err := json.Unmarshal(raw, &doc); err != nil {
		log.Fatalf("genactionparams: parse IR: %v", err)
	}

	// Index every named type so references resolve (enum→"enum", alias→its aliased token).
	index := map[string]irType{}
	for _, t := range doc.Types {
		if n := typeNameOf(t); n != "" {
			index[n] = t
		}
	}

	// The universe of annotatable request shapes = every body-arg type across all endpoints.
	bodyTypes := map[string]bool{}
	for _, s := range doc.Services {
		for _, e := range s.Endpoints {
			for _, a := range e.Args {
				if a.ParamType.Type == "body" && a.Type.Reference != nil {
					bodyTypes[qual(*a.Type.Reference)] = true
				}
			}
		}
	}

	params := map[string][]param{}
	for _, t := range doc.Types {
		if t.Object == nil {
			continue
		}
		name := qual(t.Object.TypeName)
		if !bodyTypes[name] {
			continue
		}
		var ps []param
		for _, f := range t.Object.Fields {
			p := param{
				Name:     f.FieldName,
				Type:     token(f.Type, index),
				Required: f.Type.Type != "optional",
				Docs:     firstLine(f.Docs),
			}
			// One-level structured nesting (D-ActionInvocation R-33): if the field is a nested object
			// (or a list/set of one) whose OWN fields are all flat, attach them so the runner renders a
			// structured sub-form; a deeper or self-referential nest gets no Fields → JSON editor.
			if obj := elementObject(f.Type, index); obj != "" {
				if sf, ok := subFields(obj, index); ok {
					p.Fields = sf
				}
			}
			ps = append(ps, p)
		}
		params[name] = ps
	}

	if err := os.WriteFile(*out, render(params), 0o644); err != nil {
		log.Fatalf("genactionparams: write %s: %v", *out, err)
	}
	fmt.Printf("genactionparams: wrote %s (%d request types)\n", *out, len(params))
}

func typeNameOf(t irType) string {
	switch {
	case t.Object != nil:
		return qual(t.Object.TypeName)
	case t.Enum != nil:
		return qual(t.Enum.TypeName)
	case t.Alias != nil:
		return qual(t.Alias.TypeName)
	}
	return ""
}

func qual(n typeName) string { return n.Package + "." + n.Name }

// token renders a Conjure field type as a short display string (string, rid, datetime, enum, list<string>…).
func token(n typeNode, index map[string]irType) string {
	switch n.Type {
	case "primitive":
		return strings.ToLower(n.Primitive)
	case "optional":
		return token(n.Optional.ItemType, index)
	case "list":
		return "list<" + token(n.List.ItemType, index) + ">"
	case "set":
		return "set<" + token(n.Set.ItemType, index) + ">"
	case "map":
		return "map<" + token(n.Map.KeyType, index) + "," + token(n.Map.ValueType, index) + ">"
	case "reference":
		ref := index[qual(*n.Reference)]
		switch {
		case ref.Enum != nil:
			return "enum"
		case ref.Alias != nil:
			return token(ref.Alias.Alias, index) // e.g. Rid alias → rid/string
		default:
			return strings.ToLower(n.Reference.Name) // a nested object: show its name
		}
	case "external":
		return "any"
	}
	return "any"
}

// isFlatType reports whether a type renders as a single input (or a list of them) — i.e. NOT a nested
// object or a map. It is the "one-level structurable" test for a nested object's fields.
func isFlatType(n typeNode, index map[string]irType) bool {
	switch n.Type {
	case "primitive", "external":
		return true
	case "optional":
		return isFlatType(n.Optional.ItemType, index)
	case "list":
		return isFlatType(n.List.ItemType, index)
	case "set":
		return isFlatType(n.Set.ItemType, index)
	case "map":
		return false
	case "reference":
		ref := index[qual(*n.Reference)]
		switch {
		case ref.Enum != nil:
			return true
		case ref.Alias != nil:
			return isFlatType(ref.Alias.Alias, index)
		default:
			return false // object reference — a nested object is not flat
		}
	}
	return false
}

// elementObject returns the object type an incoming field ultimately references — unwrapping optional
// and one level of list/set — or "" when the field is not (a collection of) an object.
func elementObject(n typeNode, index map[string]irType) string {
	switch n.Type {
	case "optional":
		return elementObject(n.Optional.ItemType, index)
	case "list":
		return elementObject(n.List.ItemType, index)
	case "set":
		return elementObject(n.Set.ItemType, index)
	case "reference":
		if ref := index[qual(*n.Reference)]; ref.Object != nil {
			return qual(*n.Reference)
		}
	}
	return ""
}

// subFields returns an object's fields as flat params IF every one is flat (so it can render as a
// structured sub-form one level deep); (nil, false) otherwise — the caller then leaves Fields empty and
// the runner falls back to the JSON editor for the whole param.
func subFields(objName string, index map[string]irType) ([]param, bool) {
	t, ok := index[objName]
	if !ok || t.Object == nil {
		return nil, false
	}
	var out []param
	for _, f := range t.Object.Fields {
		if !isFlatType(f.Type, index) {
			return nil, false
		}
		out = append(out, param{
			Name:     f.FieldName,
			Type:     token(f.Type, index),
			Required: f.Type.Type != "optional",
			Docs:     firstLine(f.Docs),
		})
	}
	return out, true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func render(params map[string][]param) []byte {
	names := make([]string, 0, len(params))
	for n := range params {
		names = append(names, n)
	}
	sort.Strings(names)

	var b bytes.Buffer
	b.WriteString("// Code generated by tools/genactionparams from the Conjure IR. DO NOT EDIT.\n")
	b.WriteString("// Regenerate with scripts/gen-action-params.sh (review-2026-09 R-29 parameter-schema seam).\n\n")
	b.WriteString("package action\n\n")
	b.WriteString("// requestParams maps a package-qualified Conjure request type to its fields, projected as\n")
	b.WriteString("// display-oriented Params. Single-sourced from the contract; Params() joins it via ActionType.RequestType.\n")
	b.WriteString("var requestParams = map[string][]Param{\n")
	for _, n := range names {
		b.WriteString(fmt.Sprintf("\t%q: {\n", n))
		for _, p := range params[n] {
			renderParam(&b, p, 2)
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		log.Fatalf("genactionparams: gofmt: %v", err)
	}
	return src
}

// renderParam emits one Param literal, recursing into Fields for a one-level nested object.
func renderParam(b *bytes.Buffer, p param, indent int) {
	tab := strings.Repeat("\t", indent)
	if len(p.Fields) == 0 {
		fmt.Fprintf(b, "%s{Name: %q, Type: %q, Required: %t, Docs: %q},\n", tab, p.Name, p.Type, p.Required, p.Docs)
		return
	}
	fmt.Fprintf(b, "%s{Name: %q, Type: %q, Required: %t, Docs: %q, Fields: []Param{\n", tab, p.Name, p.Type, p.Required, p.Docs)
	for _, f := range p.Fields {
		renderParam(b, f, indent+1)
	}
	fmt.Fprintf(b, "%s}},\n", tab)
}
