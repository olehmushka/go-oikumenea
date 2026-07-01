package geocountries

import (
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
)

// TestMap covers the raw ISO-3166 array -> canonical record mapping: alpha2 -> code (upper-cased),
// name pass-through, and the optional alpha3/numeric extras.
func TestMap(t *testing.T) {
	raw := domain.RawBatch{Payload: []byte(`[
	  {"alpha2":"ua","name":"Ukraine","alpha3":"ukr","numeric":"804"},
	  {"alpha2":"PL","name":"Poland"}
	]`)}

	recs, err := Mapper{}.Map(raw)
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["code"] != "UA" || recs[0]["name"] != "Ukraine" {
		t.Fatalf("record 0 = %+v", recs[0])
	}
	if recs[0]["alpha3"] != "UKR" || recs[0]["numeric"] != "804" {
		t.Fatalf("record 0 extras = %+v", recs[0])
	}
	if recs[1]["code"] != "PL" || recs[1]["name"] != "Poland" {
		t.Fatalf("record 1 = %+v", recs[1])
	}
	if _, ok := recs[1]["alpha3"]; ok {
		t.Fatalf("record 1 should omit absent alpha3: %+v", recs[1])
	}
}

// TestMapRejectsBadRow: a non-2-letter code or empty name fails the whole map (loud, not silent).
func TestMapRejectsBadRow(t *testing.T) {
	for _, body := range []string{
		`[{"alpha2":"USA","name":"United States"}]`,
		`[{"alpha2":"US","name":""}]`,
		`{"not":"an array"}`,
	} {
		if _, err := (Mapper{}).Map(domain.RawBatch{Payload: []byte(body)}); err == nil {
			t.Fatalf("expected error for %q", body)
		}
	}
}
