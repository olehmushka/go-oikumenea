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
