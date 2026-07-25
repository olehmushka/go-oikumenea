package envoverlay

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// OSEnviron snapshots the process environment into a map for the slice/map/DB scans (built once).
func OSEnviron() map[string]string {
	env := os.Environ()
	out := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			out[kv[:i]] = kv[i+1:]
		}
	}
	return out
}

// Apply overlays env onto baseYAML for schema type schema under prefix (e.g. "OIKUMENEA").
// baseYAML may be nil/empty for a fully env-only boot. Precedence within a field: full DSN scalar >
// discrete DB_* parts > yaml; more generally env > yaml. It returns the re-marshaled YAML bytes for
// witchcraft to ECV-decrypt + unmarshal (env values are plaintext and never match `enc:`).
func Apply(baseYAML []byte, schema reflect.Type, prefix string, env map[string]string) ([]byte, error) {
	return ApplyWithAliases(baseYAML, schema, prefix, env, nil)
}

// ApplyWithAliases is Apply plus a set of legacy env-name -> yaml Path aliases (string-valued),
// applied BEFORE the schema-derived names so a canonical name wins when both are set. This preserves
// the R-16 documented names (OIKUMENEA_HERMENEA_TOKEN, HERMENEA_OIKUMENEA_TOKEN) without a second
// read path.
func ApplyWithAliases(baseYAML []byte, schema reflect.Type, prefix string, env map[string]string, aliases map[string]Path) ([]byte, error) {
	m := compile(schema, prefix)

	var doc yaml.Node
	if len(bytes.TrimSpace(baseYAML)) > 0 {
		if err := yaml.Unmarshal(baseYAML, &doc); err != nil {
			return nil, fmt.Errorf("envoverlay: parse base yaml: %w", err)
		}
	}
	root := ensureRootMapping(&doc)
	if root == nil {
		return nil, fmt.Errorf("envoverlay: top-level config must be a YAML mapping")
	}

	// 1. Aliases (string), applied first so canonical names below can override.
	for name, path := range aliases {
		if v, ok := env[name]; ok {
			if err := setLeaf(root, path, v, reflect.String); err != nil {
				return nil, err
			}
		}
	}
	// 2. Schema-derived scalars.
	for name, b := range m.scalars {
		if v, ok := env[name]; ok {
			if err := setLeaf(root, b.path, v, b.valKind); err != nil {
				return nil, fmt.Errorf("envoverlay: %s: %w", name, err)
			}
		}
	}
	// 3. Slices (indexed, per-index merge).
	for _, sb := range m.slices {
		if err := applySlice(root, sb, env); err != nil {
			return nil, err
		}
	}
	// 4. Maps (e.g. modules.<key>.enabled).
	for _, mb := range m.maps {
		if err := applyMap(root, mb, env); err != nil {
			return nil, err
		}
	}
	// 5. DB discrete parts -> postgres.dsn (only when the full DSN is unset).
	if err := applyDBParts(root, prefix, env); err != nil {
		return nil, err
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, fmt.Errorf("envoverlay: marshal: %w", err)
	}
	return out, nil
}

// applySlice overlays indexed env vars onto a sequence node with per-index MERGE semantics: an env
// override for element i replaces only the matched field, leaving yaml-supplied siblings intact.
// Referencing a sparse index materializes empty preceding elements — keep indices contiguous.
func applySlice(root *yaml.Node, sb binding, env map[string]string) error {
	base := sb.env + "_"
	type hit struct {
		idx    int
		suffix string // leading-underscore relative suffix ("" for scalar slices)
		val    string
	}
	var hits []hit
	for k, v := range env {
		if !strings.HasPrefix(k, base) {
			continue
		}
		rest := k[len(base):]
		j := 0
		for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
			j++
		}
		if j == 0 {
			continue
		}
		idx, err := strconv.Atoi(rest[:j])
		if err != nil {
			continue
		}
		if sb.kind == kindScalarSlice {
			if j != len(rest) { // must be exactly "<base>_<idx>"
				continue
			}
			hits = append(hits, hit{idx, "", v})
			continue
		}
		if j == len(rest) || rest[j] != '_' { // must be "<base>_<idx>_<SUFFIX>"
			continue
		}
		hits = append(hits, hit{idx, rest[j:], v})
	}
	if len(hits) == 0 {
		return nil
	}
	seq := childSeq(root, sb.path)
	for _, h := range hits {
		growSeq(seq, h.idx+1, sb.kind)
		if sb.kind == kindScalarSlice {
			node, err := scalarNode(h.val, sb.valKind)
			if err != nil {
				return fmt.Errorf("envoverlay: %s%d: %w", base, h.idx, err)
			}
			seq.Content[h.idx] = node
			continue
		}
		eb, ok := findElem(sb.elems, h.suffix)
		if !ok {
			continue // unknown element field: ignore
		}
		if err := setLeaf(seq.Content[h.idx], eb.path, h.val, eb.valKind); err != nil {
			return fmt.Errorf("envoverlay: %s%d%s: %w", base, h.idx, h.suffix, err)
		}
	}
	return nil
}

// applyMap overlays keyed env vars onto a mapping node (e.g. OIKUMENEA_MODULES_FINANCE_ENABLED ->
// modules.finance.enabled). The map key is the captured segment lower-cased.
func applyMap(root *yaml.Node, mb binding, env map[string]string) error {
	base := mb.env + "_"
	for k, v := range env {
		if !strings.HasPrefix(k, base) {
			continue
		}
		rest := k[len(base):]
		for _, eb := range mb.elems {
			if eb.env == "" || !strings.HasSuffix(rest, eb.env) {
				continue
			}
			key := rest[:len(rest)-len(eb.env)]
			if key == "" {
				continue
			}
			mapNode := mappingAt(root, mb.path)
			elemMap := childMap(mapNode, strings.ToLower(key))
			if err := setLeaf(elemMap, eb.path, v, eb.valKind); err != nil {
				return fmt.Errorf("envoverlay: %s: %w", k, err)
			}
			break
		}
	}
	return nil
}

func findElem(elems []binding, suffix string) (binding, bool) {
	for _, eb := range elems {
		if eb.env == suffix {
			return eb, true
		}
	}
	return binding{}, false
}

// --- yaml.Node manipulation ---

// ensureRootMapping returns the top-level mapping node of doc, synthesizing an empty document +
// mapping when doc is empty (the file-less boot). It returns nil if the document's top level is a
// sequence or scalar rather than a mapping.
func ensureRootMapping(doc *yaml.Node) *yaml.Node {
	if doc.Kind == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
		return doc.Content[0]
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	if root := doc.Content[0]; root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

// mappingAt walks/creates nested mapping nodes for path and returns the mapping at its end.
func mappingAt(root *yaml.Node, path Path) *yaml.Node {
	cur := root
	for _, seg := range path {
		cur = childMap(cur, seg)
	}
	return cur
}

// setLeaf sets a scalar (typed by vk) at path, creating intermediate mappings as needed.
func setLeaf(root *yaml.Node, path Path, value string, vk reflect.Kind) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}
	parent := mappingAt(root, path[:len(path)-1])
	node, err := scalarNode(value, vk)
	if err != nil {
		return err
	}
	setChild(parent, path[len(path)-1], node)
	return nil
}

// childMap returns the child mapping node under key, creating it (or replacing a non-mapping value —
// env override wins) as needed.
func childMap(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			if m.Content[i+1].Kind != yaml.MappingNode {
				m.Content[i+1] = &yaml.Node{Kind: yaml.MappingNode}
			}
			return m.Content[i+1]
		}
	}
	vn := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content, keyNode(key), vn)
	return vn
}

// childSeq returns the sequence node at path, creating mappings along the way and the sequence at the
// leaf (replacing a non-sequence value).
func childSeq(root *yaml.Node, path Path) *yaml.Node {
	parent := mappingAt(root, path[:len(path)-1])
	key := path[len(path)-1]
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			if parent.Content[i+1].Kind != yaml.SequenceNode {
				parent.Content[i+1] = &yaml.Node{Kind: yaml.SequenceNode}
			}
			return parent.Content[i+1]
		}
	}
	sn := &yaml.Node{Kind: yaml.SequenceNode}
	parent.Content = append(parent.Content, keyNode(key), sn)
	return sn
}

// growSeq pads seq up to length n with empty elements of the right shape.
func growSeq(seq *yaml.Node, n int, k kind) {
	for len(seq.Content) < n {
		if k == kindScalarSlice {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: ""})
		} else {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.MappingNode})
		}
	}
}

func setChild(m *yaml.Node, key string, node *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = node
			return
		}
	}
	m.Content = append(m.Content, keyNode(key), node)
}

func keyNode(key string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
}

// scalarNode builds a type-preserving scalar node: strings are double-quoted (so a numeric-looking
// value like a person-code stays a string), numbers/bools are unquoted with the right tag. Bad
// numeric/bool input fails fast (naming propagated by the caller).
func scalarNode(value string, vk reflect.Kind) (*yaml.Node, error) {
	n := &yaml.Node{Kind: yaml.ScalarNode}
	switch vk {
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("%q is not a bool", value)
		}
		n.Tag, n.Value = "!!bool", strconv.FormatBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", value)
		}
		n.Tag, n.Value = "!!int", strconv.FormatInt(i, 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an unsigned integer", value)
		}
		n.Tag, n.Value = "!!int", strconv.FormatUint(u, 10)
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return nil, fmt.Errorf("%q is not a number", value)
		}
		n.Tag, n.Value = "!!float", value
	default: // string and any named string kind (e.g. wlog.LogLevel)
		n.Tag, n.Style, n.Value = "!!str", yaml.DoubleQuotedStyle, value
	}
	return n, nil
}
