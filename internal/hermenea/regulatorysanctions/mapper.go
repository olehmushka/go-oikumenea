// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package regulatorysanctions implements the person regulatory-sanction Mapper (M34 / D-Watchlists).
// Unlike the reference-catalog connectors, regulatory-sanction data has no single well-known free bulk
// source; an operator registers a source (http/file) whose payload is ALREADY a canonical JSON array of
// sanction records, and this mapper validates + passes them through to oikumenea's
// POST /import/person-regulatory-sanctions endpoint (idempotent by (person, externalId)).
//
// Each record is a JSON object: { personId, regulator, actionType?, amount?, currency?, status?,
// sanctionDate?, sourceUrl?, externalId }. A record missing personId / regulator / externalId is skipped
// (not an error — the batch may carry incomplete rows); a malformed payload fails the whole map.
package regulatorysanctions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectType = "person-regulatory-sanctions"

// Mapper turns a JSON array of sanction records into canonical import records (a validated passthrough).
type Mapper struct{}

var _ domain.Mapper = Mapper{}

// Map decodes the JSON array and emits one canonical record per row with a usable personId + regulator +
// externalId. Optional fields are forwarded when present. A malformed payload fails the whole map (the
// worker surfaces it in import_runs and retries/backs off).
func (Mapper) Map(raw domain.RawBatch) ([]map[string]any, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw.Payload, &rows); err != nil {
		return nil, fmt.Errorf("person-regulatory-sanctions: decode JSON array: %w", err)
	}
	records := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		personID := strings.TrimSpace(str(row["personId"]))
		regulator := strings.TrimSpace(str(row["regulator"]))
		externalID := strings.TrimSpace(str(row["externalId"]))
		if personID == "" || regulator == "" || externalID == "" {
			continue
		}
		rec := map[string]any{"personId": personID, "regulator": regulator, "externalId": externalID}
		for _, k := range []string{"actionType", "currency", "status", "sanctionDate", "sourceUrl"} {
			if v := strings.TrimSpace(str(row[k])); v != "" {
				rec[k] = v
			}
		}
		if amt, ok := row["amount"].(float64); ok {
			rec["amount"] = amt
		}
		records = append(records, rec)
	}
	return records, nil
}

func str(v any) string { s, _ := v.(string); return s }
