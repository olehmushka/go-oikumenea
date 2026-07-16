package transport

import (
	"context"
	"testing"
)

// ListActionTypes reads the static catalog (no audit.read gate, no DB), so a zero-value Service is
// enough to exercise the parameter-schema wiring end to end: catalog row → action.Params → ActionParam.
func TestListActionTypesAttachesParams(t *testing.T) {
	var s Service
	got, err := s.ListActionTypes(context.Background(), "")
	if err != nil {
		t.Fatalf("ListActionTypes: %v", err)
	}
	byCode := map[string]int{}
	for i, a := range got {
		byCode[a.Code] = i
	}

	// An annotated action carries its request's parameter schema.
	i, ok := byCode["assignment.grant"]
	if !ok {
		t.Fatal("assignment.grant missing from catalog")
	}
	grant := got[i]
	if len(grant.Parameters) == 0 {
		t.Fatalf("assignment.grant has no parameters (RequestType wiring broken)")
	}
	want := map[string]bool{ // subset: required fields must be present and marked required
		"subjectPersonId": true, "roleId": true, "scope": true,
	}
	seenReq := map[string]bool{}
	for _, p := range grant.Parameters {
		if p.Name == "" || p.Type == "" {
			t.Errorf("malformed param: %+v", p)
		}
		if want[p.Name] {
			if !p.Required {
				t.Errorf("param %q should be required", p.Name)
			}
			seenReq[p.Name] = true
		}
	}
	for name := range want {
		if !seenReq[name] {
			t.Errorf("expected required param %q on assignment.grant", name)
		}
	}

	// An action with no request body reports no parameters.
	if i, ok := byCode["graph.delete"]; ok {
		if len(got[i].Parameters) != 0 {
			t.Errorf("graph.delete should have no parameters, got %d", len(got[i].Parameters))
		}
	}
}
