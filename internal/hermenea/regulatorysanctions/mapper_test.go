// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package regulatorysanctions

import (
	"testing"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// TestMapper validates the passthrough: well-formed rows are emitted with their fields, incomplete rows
// (missing personId / regulator / externalId) are skipped, and the amount number survives.
func TestMapper(t *testing.T) {
	payload := []byte(`[
		{"personId":"P1","regulator":"SEC","actionType":"fine","amount":50000,"currency":"USD","status":"active","sanctionDate":"2021-06-01","sourceUrl":"https://sec.gov/x","externalId":"SEC-1"},
		{"personId":"P2","regulator":"FCA","externalId":"FCA-1"},
		{"personId":"P3","regulator":"NBU"},
		{"regulator":"SEC","externalId":"X"}
	]`)
	recs, err := Mapper{}.Map(domain.RawBatch{Payload: payload})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 valid records (incomplete rows skipped), got %d: %+v", len(recs), recs)
	}
	first := recs[0]
	if first["personId"] != "P1" || first["regulator"] != "SEC" || first["externalId"] != "SEC-1" {
		t.Fatalf("first record mismatch: %+v", first)
	}
	if amt, ok := first["amount"].(float64); !ok || amt != 50000 {
		t.Fatalf("amount not preserved: %+v", first["amount"])
	}
	if first["actionType"] != "fine" || first["currency"] != "USD" {
		t.Fatalf("optional fields not forwarded: %+v", first)
	}
}

// TestMapperMalformed fails on a non-array payload.
func TestMapperMalformed(t *testing.T) {
	if _, err := (Mapper{}).Map(domain.RawBatch{Payload: []byte(`{"not":"an array"}`)}); err == nil {
		t.Fatal("expected an error for a non-array payload")
	}
}
