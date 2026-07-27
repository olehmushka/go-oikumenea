// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package action

import "testing"

// TestRequestTypesResolve is the drift guard for the parameter-schema seam (review-2026-09 R-29):
// every action that names a RequestType must resolve to a generated param set, and that set must be
// non-empty. A typo'd/renamed request type, or a contract type that lost its body, fails here rather
// than silently serving empty parameters. requestParams is generated from the Conjure IR by
// tools/genactionparams (scripts/gen-action-params.sh) — regenerate if this fails after a contract change.
func TestRequestTypesResolve(t *testing.T) {
	annotated := 0
	for _, a := range All() {
		if a.RequestType == "" {
			continue
		}
		annotated++
		ps, ok := requestParams[a.RequestType]
		if !ok {
			t.Errorf("%s: RequestType %q not in requestParams — run scripts/gen-action-params.sh (or fix the annotation)", a.Code, a.RequestType)
			continue
		}
		if len(ps) == 0 {
			t.Errorf("%s: RequestType %q resolved to zero params — a body-less type was annotated", a.Code, a.RequestType)
		}
		if got := Params(a.Code); len(got) != len(ps) {
			t.Errorf("%s: Params()=%d want %d", a.Code, len(got), len(ps))
		}
	}
	if annotated == 0 {
		t.Fatal("no actions carry a RequestType — the parameter-schema seam is unwired")
	}
}

// TestUnannotatedHaveNoParams: an action with no RequestType reports nil params (the expand-only
// default), so the wire never carries stale/guessed argument shapes.
func TestUnannotatedHaveNoParams(t *testing.T) {
	for _, a := range All() {
		if a.RequestType == "" && Params(a.Code) != nil {
			t.Errorf("%s: unannotated action returned params", a.Code)
		}
	}
	if Params("does.not.exist") != nil {
		t.Error("unknown code should return nil params")
	}
}

// TestSensitivityOverlay: the compliance-authored paramSensitivity map surfaces on the right Params,
// and every entry keys a real (RequestType, field) pair — so a renamed field drops its masking here
// rather than silently exposing a PAN in the generic runner.
func TestSensitivityOverlay(t *testing.T) {
	// Every paramSensitivity key must resolve to an actual generated param.
	for key, kind := range paramSensitivity {
		i := lastDot(key)
		rt, field := key[:i], key[i+1:]
		ps, ok := requestParams[rt]
		if !ok {
			t.Errorf("paramSensitivity key %q: unknown request type", key)
			continue
		}
		found := false
		for _, p := range ps {
			if p.Name == field {
				found = true
			}
		}
		if !found {
			t.Errorf("paramSensitivity key %q: request type has no field %q", key, field)
		}
		if kind == "" {
			t.Errorf("paramSensitivity[%q] is empty", key)
		}
	}
	// The finance PAN action carries the "pan" sensitivity on its pan field.
	var sawPAN bool
	for _, p := range Params("finance.card.add") {
		if p.Name == "pan" && p.Sensitivity == "pan" {
			sawPAN = true
		}
	}
	if !sawPAN {
		t.Error("finance.card.add.pan should be marked sensitivity=pan")
	}
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// TestNestedParamStructure: one-level structured nesting (D-ActionInvocation R-33). A nested object
// (or list of one) with all-flat fields carries Fields; a deep/self-referential type (the rank import
// tree) carries none (JSON fallback); nested entries are themselves flat.
func TestNestedParamStructure(t *testing.T) {
	coord := findParam(Params("location.create"), "coordinate") // nested object CoordinateInput
	if coord == nil || len(coord.Fields) == 0 {
		t.Fatalf("location.create coordinate should have nested Fields")
	}
	for _, f := range coord.Fields {
		if len(f.Fields) != 0 {
			t.Errorf("nested field %s should be flat (no further Fields)", f.Name)
		}
	}
	if items := findParam(Params("order.create"), "items"); items == nil || len(items.Fields) == 0 {
		t.Error("order.create items (list<OrderItemInput>) should have nested Fields")
	}
	if sys := findParam(Params("rank.scheme.import"), "system"); sys == nil || len(sys.Fields) != 0 {
		t.Errorf("rank.scheme.import system is deep/recursive and must have NO Fields (JSON fallback)")
	}
}

func findParam(ps []Param, name string) *Param {
	for i := range ps {
		if ps[i].Name == name {
			return &ps[i]
		}
	}
	return nil
}

// TestParamsWellFormed: every generated param has a name and a type token.
func TestParamsWellFormed(t *testing.T) {
	for rt, ps := range requestParams {
		for _, p := range ps {
			if p.Name == "" || p.Type == "" {
				t.Errorf("%s: malformed param %+v", rt, p)
			}
		}
	}
}
