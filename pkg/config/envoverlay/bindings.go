// Package envoverlay layers process environment variables on top of a YAML config file so
// go-oikumenea can boot 12-factor style: env variables OVERRIDE the YAML, and the YAML file is
// OPTIONAL (an absent file plus env vars is a valid, fully env-only boot). It is framework-free —
// it imports only the stdlib and gopkg.in/yaml.v3, never witchcraft or any internal/* package — so
// both binaries and the CLI path can reuse it. The caller passes reflect.TypeOf(config.Install{})
// (or config.Runtime{}); env-var names are DERIVED from the struct's yaml tags (schema-driven), so
// dashed keys disambiguate cleanly (crypto.local-dev.kek -> OIKUMENEA_CRYPTO_LOCAL_DEV_KEK, never
// crypto/local/dev/kek). See docs/architecture/decisions.md D-EnvConfig.
package envoverlay

import (
	"reflect"
	"strings"
)

// Path is the YAML key path to a node, e.g. {"crypto","local-dev","kek"}.
type Path []string

// kind classifies how a config field is overlaid.
type kind int

const (
	kindScalar      kind = iota // a string/int/bool/float leaf
	kindScalarSlice             // a []scalar (e.g. crypto.local-dev.previous-keks)
	kindStructSlice             // a []struct (e.g. idp.issuers, hermenea sources)
	kindScalarMap               // a map[string]T (e.g. modules)
)

// binding is one compiled override site.
type binding struct {
	// env is the full env var NAME for a scalar (incl. prefix); for a slice/map it is the BASE name
	// the indexed/keyed scan is anchored on. For an ELEMENT field binding it is the leading-underscore
	// relative SUFFIX (e.g. "_HMAC_KEY") appended after "<base>_<index>".
	env     string
	path    Path         // yaml path to the leaf (scalar) or the container (sequence/mapping)
	kind    kind         // scalar / scalarSlice / structSlice / scalarMap
	valKind reflect.Kind // the scalar leaf's Go kind, for type-preserving marshal
	elems   []binding    // element field bindings (relative env + relative path) for slice/map kinds
}

// model is the compiled schema -> env binding set for one config type + prefix.
type model struct {
	scalars map[string]binding // env name -> scalar binding (fast lookup)
	slices  []binding          // slice bindings, scanned against the env snapshot
	maps    []binding          // map bindings, scanned against the env snapshot
	prefix  string
}

// Bindings returns the env-name -> yaml Path map for t under prefix (e.g. "OIKUMENEA"). Scalar leaves
// map to their concrete env name; slice/map element sites use an "_N" / "_<KEY>" placeholder segment.
// It is the source for the generated env-var reference table (docs/reference/env-vars.md).
func Bindings(t reflect.Type, prefix string) map[string]Path {
	m := compile(t, prefix)
	out := make(map[string]Path, len(m.scalars))
	for env, b := range m.scalars {
		out[env] = b.path
	}
	for _, sb := range m.slices {
		if sb.kind == kindScalarSlice {
			out[sb.env+"_N"] = sb.path
			continue
		}
		for _, eb := range sb.elems {
			out[sb.env+"_N"+eb.env] = joinPath(sb.path, eb.path)
		}
	}
	for _, mb := range m.maps {
		for _, eb := range mb.elems {
			out[mb.env+"_<KEY>"+eb.env] = joinPath(mb.path, eb.path)
		}
	}
	return out
}

// compile walks schema t (deref'd through a leading pointer) into a model under prefix.
func compile(t reflect.Type, prefix string) *model {
	m := &model{scalars: map[string]binding{}, prefix: prefix}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		walkStruct(t, prefix, nil, m, map[reflect.Type]bool{})
	}
	return m
}

// walkStruct recurses the yaml-tagged fields of struct type t, accumulating env prefix and yaml path.
// seen is a branch-local guard so a self-referential type cannot recurse forever (it is removed on the
// way back up, so a type legitimately reused in sibling branches is still visited).
func walkStruct(t reflect.Type, prefix string, path Path, m *model, seen map[reflect.Type]bool) {
	if seen[t] {
		return
	}
	seen[t] = true
	defer delete(seen, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// Skip unexported non-embedded fields (reflect cannot set them and yaml ignores them).
		if f.PkgPath != "" && !f.Anonymous {
			continue
		}
		name, opts := parseYAMLTag(f.Tag.Get("yaml"))
		if name == "-" {
			continue
		}
		ft := derefType(f.Type)
		// An embedded struct tagged `,inline` (the wconfig.Install / wconfig.Runtime bases) folds into
		// the SAME namespace: recurse without adding an env segment or a path segment.
		if f.Anonymous && (hasOpt(opts, "inline") || name == "") {
			if ft.Kind() == reflect.Struct {
				walkStruct(ft, prefix, path, m, seen)
			}
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name) // defensive: the config structs always tag
		}
		env := prefix + "_" + upperSnake(name)
		child := appendPath(path, name)
		emit(ft, env, child, m, seen)
	}
}

// emit classifies field type ft (already deref'd) at env/path and records the appropriate binding(s).
func emit(ft reflect.Type, env string, path Path, m *model, seen map[reflect.Type]bool) {
	ft = derefType(ft)
	switch ft.Kind() {
	case reflect.Struct:
		walkStruct(ft, env, path, m, seen)
	case reflect.Slice:
		et := derefType(ft.Elem())
		switch {
		case isScalarKind(et.Kind()):
			m.slices = append(m.slices, binding{env: env, path: path, kind: kindScalarSlice, valKind: et.Kind()})
		case et.Kind() == reflect.Struct:
			m.slices = append(m.slices, binding{env: env, path: path, kind: kindStructSlice, elems: elementBindings(et, seen)})
		}
	case reflect.Map:
		if ft.Key().Kind() != reflect.String {
			return
		}
		vt := derefType(ft.Elem())
		var elems []binding
		switch {
		case vt.Kind() == reflect.Struct:
			elems = elementBindings(vt, seen)
		case isScalarKind(vt.Kind()):
			elems = []binding{{env: "", path: nil, kind: kindScalar, valKind: vt.Kind()}}
		default:
			return
		}
		m.maps = append(m.maps, binding{env: env, path: path, kind: kindScalarMap, elems: elems})
	default:
		if isScalarKind(ft.Kind()) {
			m.scalars[env] = binding{env: env, path: path, kind: kindScalar, valKind: ft.Kind()}
		}
	}
}

// elementBindings compiles the SCALAR field bindings of a slice/map element struct with RELATIVE env
// suffixes (leading-underscore, e.g. "_HMAC_KEY") and RELATIVE paths (e.g. {"hmac-key"}).
func elementBindings(et reflect.Type, seen map[reflect.Type]bool) []binding {
	sub := &model{scalars: map[string]binding{}}
	walkStruct(et, "", nil, sub, seen)
	out := make([]binding, 0, len(sub.scalars))
	for _, eb := range sub.scalars {
		out = append(out, eb)
	}
	return out
}

// --- small helpers ---

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func isScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// parseYAMLTag splits a `yaml:"name,opt1,opt2"` tag into its name and options.
func parseYAMLTag(tag string) (name string, opts []string) {
	parts := strings.Split(tag, ",")
	return parts[0], parts[1:]
}

func hasOpt(opts []string, want string) bool {
	for _, o := range opts {
		if o == want {
			return true
		}
	}
	return false
}

// upperSnake maps a yaml key segment to its env fragment: dashes become underscores, upper-cased.
// This is the disambiguation rule ("local-dev" -> "LOCAL_DEV").
func upperSnake(seg string) string {
	return strings.ToUpper(strings.ReplaceAll(seg, "-", "_"))
}

func appendPath(p Path, seg string) Path {
	out := make(Path, len(p)+1)
	copy(out, p)
	out[len(p)] = seg
	return out
}

func joinPath(a, b Path) Path {
	out := make(Path, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
