// Command ir2openapi converts the go-oikumenea Conjure IR into an OpenAPI 3.0 document.
//
// Conjure has no official OpenAPI generator, so this is the repo's own emitter. It needs the Conjure
// IR (the compiled api/*.conjure.yml). The IR is produced offline by godel — `godel conjure-publish`
// builds it and uploads it; this tool captures it by standing up a tiny local sink and pointing the
// publish at it (no JVM, no network, no external tools). Pass -ir to convert an existing IR file
// instead.
//
// Usage:
//
//	go run ./tools/ir2openapi -out docs/api/openapi              # extract IR via godel, then convert
//	go run ./tools/ir2openapi -ir out/conjure-ir.json -out docs/api/openapi
//
// The output (docs/api/openapi/openapi.json) is a single OpenAPI 3.0.3 document covering every service.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	irPath := flag.String("ir", "", "path to an existing Conjure IR JSON (skips godel extraction)")
	outDir := flag.String("out", "docs/api/openapi", "output directory for openapi.json")
	godelw := flag.String("godelw", "./godelw", "path to the godelw wrapper (for IR extraction)")
	apiVersion := flag.String("version", "0.0.0-dev", "value for info.version")
	server := flag.String("server", "https://localhost:8443", "default servers[].url")
	flag.Parse()

	var raw []byte
	var err error
	if *irPath != "" {
		raw, err = os.ReadFile(*irPath)
	} else {
		raw, err = extractIR(*godelw)
	}
	if err != nil {
		fatal("obtain IR: %v", err)
	}

	var doc ir
	if err := json.Unmarshal(raw, &doc); err != nil {
		fatal("parse IR: %v", err)
	}

	spec := convert(doc, *apiVersion, *server)
	out, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		fatal("marshal openapi: %v", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal("mkdir %s: %v", *outDir, err)
	}
	dest := filepath.Join(*outDir, "openapi.json")
	if err := os.WriteFile(dest, append(out, '\n'), 0o644); err != nil {
		fatal("write %s: %v", dest, err)
	}
	fmt.Printf("wrote %s (%d services, %d schemas)\n", dest, len(doc.Services), len(doc.Types)+1)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ir2openapi: "+format+"\n", a...)
	os.Exit(1)
}

// ---------------------------------------------------------------- IR extraction (offline, no JVM)

// extractIR runs `godel conjure-publish` against an in-process HTTP sink and returns the captured
// Conjure IR JSON. godel compiles api/ to IR and PUTs it; we keep the platform project's IR.
func extractIR(godelw string) ([]byte, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	captured := map[string][]byte{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body)
			if strings.HasSuffix(r.URL.Path, ".conjure.json") {
				captured[r.URL.Path] = b
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK) // checksum POSTs etc.
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	cmd := exec.Command(godelw, "conjure-publish",
		"--group-id", "local.openapi.gen", "--no-pom",
		"--repository", "local", "--url", fmt.Sprintf("http://127.0.0.1:%d", port))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// conjure-publish exits non-zero on the checksum step; ignore the error and rely on the capture.
	_ = cmd.Run()

	if len(captured) == 0 {
		return nil, fmt.Errorf("godel published no IR (stderr: %s)", strings.TrimSpace(stderr.String()))
	}
	// Prefer the platform project's IR; otherwise take any (they describe the same API).
	for path, body := range captured {
		if strings.Contains(path, "/platform/") {
			return body, nil
		}
	}
	for _, body := range captured {
		return body, nil
	}
	return nil, fmt.Errorf("unreachable")
}

// ---------------------------------------------------------------- Conjure IR types (subset used here)

type typeName struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}

type irType struct {
	Type      string    `json:"type"`
	Primitive string    `json:"primitive"`
	Reference *typeName `json:"reference"`
	Optional  *struct {
		ItemType irType `json:"itemType"`
	} `json:"optional"`
	List *struct {
		ItemType irType `json:"itemType"`
	} `json:"list"`
	Set *struct {
		ItemType irType `json:"itemType"`
	} `json:"set"`
	Map *struct {
		KeyType   irType `json:"keyType"`
		ValueType irType `json:"valueType"`
	} `json:"map"`
}

type field struct {
	FieldName string `json:"fieldName"`
	Type      irType `json:"type"`
	Docs      string `json:"docs"`
}

type typeDef struct {
	Type   string `json:"type"`
	Object *struct {
		TypeName typeName `json:"typeName"`
		Fields   []field  `json:"fields"`
		Docs     string   `json:"docs"`
	} `json:"object"`
	Enum *struct {
		TypeName typeName `json:"typeName"`
		Values   []struct {
			Value string `json:"value"`
			Docs  string `json:"docs"`
		} `json:"values"`
		Docs string `json:"docs"`
	} `json:"enum"`
	Alias *struct {
		TypeName typeName `json:"typeName"`
		Alias    irType   `json:"alias"`
		Docs     string   `json:"docs"`
	} `json:"alias"`
}

type argDef struct {
	ArgName   string `json:"argName"`
	Type      irType `json:"type"`
	Docs      string `json:"docs"`
	ParamType struct {
		Type  string `json:"type"`
		Query *struct {
			ParamID string `json:"paramId"`
		} `json:"query"`
		Header *struct {
			ParamID string `json:"paramId"`
		} `json:"header"`
	} `json:"paramType"`
}

type endpoint struct {
	EndpointName string `json:"endpointName"`
	HTTPMethod   string `json:"httpMethod"`
	HTTPPath     string `json:"httpPath"`
	Auth         *struct {
		Type string `json:"type"`
	} `json:"auth"`
	Args    []argDef `json:"args"`
	Returns *irType  `json:"returns"`
	Docs    string   `json:"docs"`
}

type service struct {
	ServiceName typeName   `json:"serviceName"`
	Endpoints   []endpoint `json:"endpoints"`
	Docs        string     `json:"docs"`
}

type ir struct {
	Version  int        `json:"version"`
	Types    []typeDef  `json:"types"`
	Services []service  `json:"services"`
	Errors   []struct{} `json:"errors"`
}

// ---------------------------------------------------------------- conversion

type obj = map[string]any

func convert(doc ir, version, server string) obj {
	schemas := obj{}
	for _, t := range doc.Types {
		switch t.Type {
		case "object":
			schemas[t.Object.TypeName.Name] = objectSchema(t)
		case "enum":
			vals := make([]string, 0, len(t.Enum.Values))
			for _, v := range t.Enum.Values {
				vals = append(vals, v.Value)
			}
			s := obj{"type": "string", "enum": vals}
			withDocs(s, t.Enum.Docs)
			schemas[t.Enum.TypeName.Name] = s
		case "alias":
			s := schemaFor(t.Alias.Alias)
			schemas[t.Alias.TypeName.Name] = withDocsRefSafe(s, t.Alias.Docs)
		}
	}
	schemas["SerializableError"] = serializableError()

	paths := obj{}
	for _, svc := range doc.Services {
		for _, ep := range svc.Endpoints {
			p, _ := paths[ep.HTTPPath].(obj)
			if p == nil {
				p = obj{}
				paths[ep.HTTPPath] = p
			}
			p[strings.ToLower(ep.HTTPMethod)] = operation(svc, ep)
		}
	}

	return obj{
		"openapi": "3.0.3",
		"info": obj{
			"title":       "go-oikumenea API",
			"version":     version,
			"description": "Generated from the Conjure contract (api/*.conjure.yml) by tools/ir2openapi — do not hand-edit. Every endpoint takes a bearer token; authorization is decided by the PDP.",
		},
		"servers":  []any{obj{"url": server, "description": "local dev server (self-signed TLS)"}},
		"security": []any{obj{"bearerAuth": []any{}}},
		"paths":    paths,
		"components": obj{
			"securitySchemes": obj{
				"bearerAuth": obj{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
			"schemas": schemas,
		},
	}
}

func objectSchema(t typeDef) obj {
	props := obj{}
	var required []string
	for _, f := range t.Object.Fields {
		props[f.FieldName] = withDocsRefSafe(schemaFor(f.Type), f.Docs)
		if f.Type.Type != "optional" {
			required = append(required, f.FieldName)
		}
	}
	s := obj{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		s["required"] = required
	}
	withDocs(s, t.Object.Docs)
	return s
}

// schemaFor maps a Conjure IR type to an OpenAPI schema. optional<T> maps to the schema of T (the
// optionality is expressed by leaving the field out of `required`).
func schemaFor(t irType) obj {
	switch t.Type {
	case "primitive":
		return primitiveSchema(t.Primitive)
	case "optional":
		return schemaFor(t.Optional.ItemType)
	case "list":
		return obj{"type": "array", "items": schemaFor(t.List.ItemType)}
	case "set":
		return obj{"type": "array", "uniqueItems": true, "items": schemaFor(t.Set.ItemType)}
	case "map":
		return obj{"type": "object", "additionalProperties": schemaFor(t.Map.ValueType)}
	case "reference":
		return obj{"$ref": "#/components/schemas/" + t.Reference.Name}
	default:
		return obj{}
	}
}

func primitiveSchema(p string) obj {
	switch p {
	case "STRING", "RID", "BEARERTOKEN", "UUID":
		return obj{"type": "string"}
	case "DATETIME":
		return obj{"type": "string", "format": "date-time"}
	case "BINARY":
		return obj{"type": "string", "format": "binary"}
	case "INTEGER":
		return obj{"type": "integer", "format": "int32"}
	case "SAFELONG":
		return obj{"type": "integer", "format": "int64"}
	case "DOUBLE":
		return obj{"type": "number", "format": "double"}
	case "BOOLEAN":
		return obj{"type": "boolean"}
	case "ANY":
		return obj{} // empty schema = any JSON value
	default:
		return obj{}
	}
}

func operation(svc service, ep endpoint) obj {
	op := obj{
		"operationId": svc.ServiceName.Name + "_" + ep.EndpointName,
		"tags":        []any{svc.ServiceName.Name},
	}
	if ep.Docs != "" {
		op["summary"] = firstLine(ep.Docs)
		op["description"] = ep.Docs
	}

	var params []any
	for _, a := range ep.Args {
		switch a.ParamType.Type {
		case "body":
			op["requestBody"] = obj{
				"required": a.Type.Type != "optional",
				"content":  obj{"application/json": obj{"schema": schemaFor(a.Type)}},
			}
		case "path", "query", "header":
			params = append(params, parameter(a))
		}
	}
	if len(params) > 0 {
		op["parameters"] = params
	}

	responses := obj{}
	if ep.Returns != nil {
		responses["200"] = obj{
			"description": "Success",
			"content":     obj{"application/json": obj{"schema": schemaFor(*ep.Returns)}},
		}
	} else {
		responses["204"] = obj{"description": "No Content"}
	}
	responses["default"] = obj{
		"description": "Conjure SerializableError envelope (errorCode/errorName/parameters).",
		"content":     obj{"application/json": obj{"schema": obj{"$ref": "#/components/schemas/SerializableError"}}},
	}
	op["responses"] = responses

	if ep.Auth == nil {
		op["security"] = []any{} // override the global bearer requirement for an unauthenticated endpoint
	}
	return op
}

func parameter(a argDef) obj {
	in := a.ParamType.Type
	name := a.ArgName
	switch in {
	case "query":
		if a.ParamType.Query != nil && a.ParamType.Query.ParamID != "" {
			name = a.ParamType.Query.ParamID
		}
	case "header":
		if a.ParamType.Header != nil && a.ParamType.Header.ParamID != "" {
			name = a.ParamType.Header.ParamID
		}
	}
	p := obj{
		"name":     name,
		"in":       in,
		"required": in == "path" || a.Type.Type != "optional",
		"schema":   schemaFor(a.Type),
	}
	if a.Docs != "" {
		p["description"] = a.Docs
	}
	return p
}

func serializableError() obj {
	return obj{
		"type":        "object",
		"description": "The Conjure error envelope returned on any non-2xx response.",
		"properties": obj{
			"errorCode":       obj{"type": "string", "description": "Coarse category, e.g. NOT_FOUND, INVALID_ARGUMENT, PERMISSION_DENIED, CONFLICT, FAILED_PRECONDITION."},
			"errorName":       obj{"type": "string", "description": "The specific error, e.g. Person:PersonNotFound."},
			"errorInstanceId": obj{"type": "string", "format": "uuid"},
			"parameters":      obj{"type": "object", "additionalProperties": obj{}},
		},
		"required": []string{"errorCode", "errorName", "errorInstanceId"},
	}
}

// withDocs sets description on a plain schema. withDocsRefSafe handles the OpenAPI 3.0 rule that a
// $ref must not have sibling keys: it wraps the ref in allOf so the description survives.
func withDocs(s obj, docs string) {
	if docs != "" {
		s["description"] = docs
	}
}

func withDocsRefSafe(s obj, docs string) obj {
	if docs == "" {
		return s
	}
	if _, isRef := s["$ref"]; isRef {
		return obj{"allOf": []any{s}, "description": docs}
	}
	s["description"] = docs
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
