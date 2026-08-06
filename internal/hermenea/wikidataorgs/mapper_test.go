// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package wikidataorgs

import (
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

func TestMapExternalOrgs(t *testing.T) {
	payload := []byte(`{
	  "head": {"vars": ["org","orgLabel","countryCode","kind"]},
	  "results": {"bindings": [
	    {
	      "org": {"type":"uri","value":"http://www.wikidata.org/entity/Q1145276"},
	      "orgLabel": {"type":"literal","value":"Servant of the People"},
	      "countryCode": {"type":"literal","value":"ua"},
	      "kind": {"type":"literal","value":"party"}
	    },
	    {
	      "org": {"type":"uri","value":"http://www.wikidata.org/entity/Q1808487"},
	      "orgLabel": {"type":"literal","value":"Ministry of Defence of Ukraine"},
	      "kind": {"type":"literal","value":"government_body"}
	    },
	    {
	      "org": {"type":"uri","value":"http://www.wikidata.org/entity/Q999"},
	      "orgLabel": {"type":"literal","value":"Weird Thing"},
	      "kind": {"type":"literal","value":"banana"}
	    },
	    {
	      "org": {"type":"uri","value":"http://www.wikidata.org/entity/Q500"},
	      "orgLabel": {"type":"literal","value":""}
	    }
	  ]}
	}`)

	recs, err := Mapper{}.Map(domain.RawBatch{Payload: payload})
	if err != nil {
		t.Fatalf("Map: %v", err)
	}
	// The label-less row is skipped; the other three survive.
	if len(recs) != 3 {
		t.Fatalf("want 3 records, got %d: %+v", len(recs), recs)
	}

	first := recs[0]
	if first["wikidataId"] != "Q1145276" {
		t.Errorf("wikidataId = %v, want Q1145276", first["wikidataId"])
	}
	if first["name"] != "Servant of the People" {
		t.Errorf("name = %v", first["name"])
	}
	if first["kind"] != "party" {
		t.Errorf("kind = %v, want party", first["kind"])
	}
	if first["country"] != "UA" { // upper-cased
		t.Errorf("country = %v, want UA", first["country"])
	}

	// Second has no country binding → no country key.
	if _, ok := recs[1]["country"]; ok {
		t.Errorf("row 2 should have no country, got %v", recs[1]["country"])
	}
	if recs[1]["kind"] != "government_body" {
		t.Errorf("row 2 kind = %v", recs[1]["kind"])
	}

	// Unknown kind folds to "other".
	if recs[2]["kind"] != "other" {
		t.Errorf("row 3 kind = %v, want other (folded)", recs[2]["kind"])
	}
}

func TestMapBadPayload(t *testing.T) {
	if _, err := (Mapper{}).Map(domain.RawBatch{Payload: []byte("not json")}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestWikidataQID(t *testing.T) {
	cases := map[string]string{
		"http://www.wikidata.org/entity/Q42": "Q42",
		"Q7":                                 "Q7",
		"":                                   "",
	}
	for in, want := range cases {
		if got := wikidataQID(in); got != want {
			t.Errorf("wikidataQID(%q) = %q, want %q", in, got, want)
		}
	}
}
