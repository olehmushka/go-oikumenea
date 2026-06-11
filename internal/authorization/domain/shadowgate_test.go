package domain

import (
	"reflect"
	"sort"
	"testing"
)

// TestShadowGate covers the shadow-visibility gate (F-002): a `public` unit always passes, a `shadow`
// unit passes only when the subject's reach covers it, and an instance admin sees everything. The set
// returned is the allowed unit ids.
func TestShadowGate(t *testing.T) {
	reach := Reach{Readable: map[string]struct{}{"u-reached-shadow": {}, "u-reached-public": {}}}
	candidates := []string{"u-public", "u-reached-public", "u-shadow", "u-reached-shadow"}
	shadow := map[string]bool{
		"u-public":         false,
		"u-reached-public": false,
		"u-shadow":         true,
		"u-reached-shadow": true,
	}

	got := ShadowGate(reach, candidates, shadow)
	want := []string{"u-public", "u-reached-public", "u-reached-shadow"} // u-shadow dropped (shadow, not reached)
	assertSet(t, got, want)

	// An instance admin reaches everything, so no candidate is dropped.
	adminGot := ShadowGate(Reach{InstanceAdmin: true}, candidates, shadow)
	assertSet(t, adminGot, candidates)

	// A subject with empty reach keeps only the public units.
	emptyGot := ShadowGate(Reach{}, candidates, shadow)
	assertSet(t, emptyGot, []string{"u-public", "u-reached-public"})
}

func assertSet(t *testing.T, got map[string]struct{}, want []string) {
	t.Helper()
	gotKeys := make([]string, 0, len(got))
	for k := range got {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	sort.Strings(want)
	if !reflect.DeepEqual(gotKeys, want) {
		t.Fatalf("ShadowGate set = %v, want %v", gotKeys, want)
	}
}
