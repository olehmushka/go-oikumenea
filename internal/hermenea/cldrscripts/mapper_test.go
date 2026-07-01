package cldrscripts

import (
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMap covers the CLDR language→script mapping: iso639_3 lower-cased, writingSystem pass-through,
// isPrimary carried.
func TestMap(t *testing.T) {
	raw := domain.RawBatch{Payload: []byte(`[
	  {"iso639_3":"ENG","writingSystem":"Latn","isPrimary":true},
	  {"iso639_3":"jpn","writingSystem":"Jpan","isPrimary":true}
	]`)}
	recs, err := Mapper{}.Map(raw)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["iso639_3"] != "eng" || recs[0]["writingSystem"] != "Latn" || recs[0]["isPrimary"] != true {
		t.Fatalf("record 0 = %+v", recs[0])
	}
}

// TestMapRejectsBadRow: a missing iso/script fails the whole map.
func TestMapRejectsBadRow(t *testing.T) {
	for _, body := range []string{
		`[{"iso639_3":"","writingSystem":"Latn"}]`,
		`[{"iso639_3":"eng","writingSystem":""}]`,
	} {
		if _, err := (Mapper{}).Map(domain.RawBatch{Payload: []byte(body)}); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
}
