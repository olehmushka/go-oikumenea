package glottolog

import (
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMapParentFirst verifies the mapper emits every parent before its children (the RESTRICT
// parent_id FK requires this), regardless of the source order, and passes through the fields.
func TestMapParentFirst(t *testing.T) {
	// Deliberately child-first in the source.
	raw := domain.RawBatch{Payload: []byte(`[
	  {"code":"some1234","level":"dialect","name":"Some dialect","parent":"stan1293"},
	  {"code":"stan1293","level":"language","name":"English","parent":"germ1287","iso639_3":"eng","countries":["GB","US"]},
	  {"code":"germ1287","level":"family","name":"Germanic","parent":"indo1319"},
	  {"code":"indo1319","level":"family","name":"Indo-European"}
	]`)}

	recs, err := Mapper{}.Map(raw)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if len(recs) != 4 {
		t.Fatalf("got %d records, want 4", len(recs))
	}
	pos := map[string]int{}
	for i, r := range recs {
		pos[r["code"].(string)] = i
	}
	// Each child must appear after its parent.
	for child, parent := range map[string]string{"some1234": "stan1293", "stan1293": "germ1287", "germ1287": "indo1319"} {
		if pos[parent] >= pos[child] {
			t.Fatalf("%s (pos %d) must precede %s (pos %d)", parent, pos[parent], child, pos[child])
		}
	}
	// Root carries no parent; English carries iso + countries.
	if _, ok := recs[pos["indo1319"]]["parent"]; ok {
		t.Fatalf("root indo1319 must have no parent: %+v", recs[pos["indo1319"]])
	}
	eng := recs[pos["stan1293"]]
	if eng["iso639_3"] != "eng" {
		t.Fatalf("english iso = %v, want eng", eng["iso639_3"])
	}
	if cs, ok := eng["countries"].([]any); !ok || len(cs) != 2 || cs[0] != "GB" {
		t.Fatalf("english countries = %v, want [GB US]", eng["countries"])
	}
}

// TestMapDropsOutOfSnapshotParent: a parent not present in the snapshot is dropped so the node lands
// top-level (parent omitted) rather than dangling a forward reference.
func TestMapDropsOutOfSnapshotParent(t *testing.T) {
	raw := domain.RawBatch{Payload: []byte(`[
	  {"code":"orph1234","level":"language","name":"Orphan","parent":"miss0000"}
	]`)}
	recs, err := Mapper{}.Map(raw)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if _, ok := recs[0]["parent"]; ok {
		t.Fatalf("out-of-snapshot parent must be dropped: %+v", recs[0])
	}
}

// TestMapRejectsBadRow: an empty code/name or bad level fails the whole map (loud, not silent).
func TestMapRejectsBadRow(t *testing.T) {
	for _, body := range []string{
		`[{"code":"","level":"language","name":"x"}]`,
		`[{"code":"stan1293","level":"language","name":""}]`,
		`[{"code":"stan1293","level":"kingdom","name":"x"}]`,
	} {
		if _, err := (Mapper{}).Map(domain.RawBatch{Payload: []byte(body)}); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
}
