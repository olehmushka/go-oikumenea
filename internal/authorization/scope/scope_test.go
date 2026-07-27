// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package scope

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// The scope implementations are thin adapters over injected funcs; the SQL parity of those funcs is
// proven differentially elsewhere (membership's TestReachDifferential (b2) for the person probe,
// authorization's FilterVisibleUnits suite for the shadow gate). What is tested HERE is the adapter
// algebra itself: order preservation, the instance-admin short-circuit, the empty fast path, and
// the fail-closed drop of an unmapped unit-scope candidate.

func TestCatalogScopeIsIdentity(t *testing.T) {
	v := NewCatalogScope()
	in := []string{"c", "a", "b"}
	got, err := v.ReadableIDs(context.Background(), "subj", false, in)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("catalog scope must be identity, got %v", got)
	}
}

func TestPersonScopeTrimsAndPreservesOrder(t *testing.T) {
	// Probe returns an UNORDERED subset; the adapter must restore candidate order.
	v := NewPersonScope(func(_ context.Context, subject string, ids []string) ([]string, error) {
		if subject != "subj" {
			t.Fatalf("subject not threaded: %q", subject)
		}
		return []string{"p3", "p1"}, nil
	})
	got, err := v.ReadableIDs(context.Background(), "subj", false, []string{"p1", "p2", "p3"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"p1", "p3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPersonScopeAdminAndEmptyShortCircuit(t *testing.T) {
	v := NewPersonScope(func(context.Context, string, []string) ([]string, error) {
		t.Fatal("probe must not run")
		return nil, nil
	})
	in := []string{"p1", "p2"}
	if got, _ := v.ReadableIDs(context.Background(), "subj", true, in); !reflect.DeepEqual(got, in) {
		t.Fatalf("admin short-circuit: got %v", got)
	}
	if got, err := v.ReadableIDs(context.Background(), "subj", false, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty fast path: got %v, %v", got, err)
	}
}

func TestUnitScopeMapsFiltersAndFailsClosed(t *testing.T) {
	// o1,o2 governed by u1 (shadow, reachable); o3 by u2 (shadow, unreachable); o4 UNMAPPED (must
	// drop, fail closed); o5 by u3 (public, passes the filter).
	mapUnits := func(_ context.Context, ids []string) (map[string]string, map[string]bool, error) {
		return map[string]string{"o1": "u1", "o2": "u1", "o3": "u2", "o5": "u3"},
			map[string]bool{"u1": true, "u2": true, "u3": false}, nil
	}
	filter := func(_ context.Context, subject string, candidates []string, shadow map[string]bool) ([]string, error) {
		// The shadow-gate contract: public passes, shadow only if reachable (here: u1 yes, u2 no).
		var out []string
		for _, u := range candidates {
			if !shadow[u] || u == "u1" {
				out = append(out, u)
			}
		}
		return out, nil
	}
	v := NewUnitScope(mapUnits, filter)
	got, err := v.ReadableIDs(context.Background(), "subj", false, []string{"o1", "o2", "o3", "o4", "o5"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"o1", "o2", "o5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUnitScopePropagatesErrors(t *testing.T) {
	boom := errors.New("boom")
	v := NewUnitScope(
		func(context.Context, []string) (map[string]string, map[string]bool, error) { return nil, nil, boom },
		func(context.Context, string, []string, map[string]bool) ([]string, error) { return nil, nil },
	)
	if _, err := v.ReadableIDs(context.Background(), "subj", false, []string{"x"}); !errors.Is(err, boom) {
		t.Fatalf("mapUnits error not propagated: %v", err)
	}
}
