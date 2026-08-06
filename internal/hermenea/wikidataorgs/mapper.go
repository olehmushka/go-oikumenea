// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package wikidataorgs implements the external-organizations Mapper (M30 / D-ExternalOrgs) — hermenea's
// first registry consumer beyond geo/language. It reads a Wikidata SPARQL JSON result set (fetched by
// the `http` connector against https://query.wikidata.org/sparql?format=json&query=…) and emits
// canonical external-organizations records for oikumenea's POST /import/external-organizations endpoint,
// which upserts them Wikidata-id-keyed, idempotently, and non-destructively into
// oikumenea.external_organizations.
//
// The SPARQL query is expected to project an `org` (the entity URI), an `orgLabel` (the English label),
// an optional `countryCode` (ISO-3166 alpha-2 via wdt:P297), and a `kind` literal already mapped to the
// external_org_kinds catalog code (party | government_body | military | ngo | registrant | other). A row
// without an org URI or label is skipped; a missing kind defaults to `other`.
package wikidataorgs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
)

// ObjectType is the oikumenea import object-type this mapper feeds (the source's object_type).
const ObjectType = "external-organizations"

// knownKinds is the external_org_kinds catalog vocabulary (mirrors the migration seed). A kind outside
// this set is normalized to `other` so a query mistake never produces an unresolvable record.
var knownKinds = map[string]bool{
	"party": true, "government_body": true, "military": true,
	"ngo": true, "registrant": true, "other": true,
}

// sparqlResult is the subset of the SPARQL JSON results envelope this mapper reads.
type sparqlResult struct {
	Results struct {
		Bindings []map[string]sparqlValue `json:"bindings"`
	} `json:"results"`
}

type sparqlValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Mapper turns a Wikidata SPARQL JSON result set into canonical external-organizations records.
type Mapper struct{}

var _ domain.Mapper = Mapper{}

// Map decodes the SPARQL payload and emits one record per binding with a usable org URI + label. The
// Wikidata Q-id (the import key) is the last path segment of the org URI. A binding missing the org URI
// or the label is skipped (not an error — the result set may carry incomplete rows); a malformed payload
// fails the whole map (the worker surfaces it in import_runs and retries/backs off).
func (Mapper) Map(raw domain.RawBatch) ([]map[string]any, error) {
	var res sparqlResult
	if err := json.Unmarshal(raw.Payload, &res); err != nil {
		return nil, fmt.Errorf("external-organizations: decode SPARQL JSON: %w", err)
	}
	records := make([]map[string]any, 0, len(res.Results.Bindings))
	for _, b := range res.Results.Bindings {
		qid := wikidataQID(b["org"].Value)
		name := strings.TrimSpace(b["orgLabel"].Value)
		if qid == "" || name == "" {
			continue
		}
		rec := map[string]any{"wikidataId": qid, "name": name}
		if kind := normalizeKind(b["kind"].Value); kind != "" {
			rec["kind"] = kind
		}
		if cc := strings.ToUpper(strings.TrimSpace(b["countryCode"].Value)); len(cc) == 2 {
			rec["country"] = cc
		}
		records = append(records, rec)
	}
	return records, nil
}

// wikidataQID extracts the trailing Q-id from a Wikidata entity URI
// (http://www.wikidata.org/entity/Q1145276 → Q1145276); "" when the value is empty or has no segment.
func wikidataQID(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if i := strings.LastIndex(uri, "/"); i >= 0 {
		uri = uri[i+1:]
	}
	return strings.TrimSpace(uri)
}

// normalizeKind lower-cases the kind literal and folds an unknown value (or the Wikidata "orgLabel"-style
// fallback where kind == the label) to `other`. An empty value returns "" so the record omits kind and
// the importer applies its own default.
func normalizeKind(s string) string {
	k := strings.ToLower(strings.TrimSpace(s))
	if k == "" {
		return ""
	}
	if !knownKinds[k] {
		return "other"
	}
	return k
}
