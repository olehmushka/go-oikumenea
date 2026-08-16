// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	wikidataclient "github.com/olehmushka/go-wikidata-client"
)

// WikidataSPARQL runs a source's SPARQL query against a Wikidata-compatible endpoint via
// go-wikidata-client, then re-encodes the typed result back into the standard SPARQL 1.1 JSON
// Results shape (the same bytes the generic HTTP fetcher already hands a SPARQL-consuming Mapper) —
// so wikidataorgs.Mapper and any future SPARQL Mapper need no change. Locator is a full URL, the same
// shape the `http` connector already uses for Wikidata sources
// ("https://query.wikidata.org/sparql?format=json&query=<encoded query>"): the endpoint and query are
// parsed out of it rather than inventing a new locator format.
type WikidataSPARQL struct{ client *http.Client }

var _ domain.Fetcher = WikidataSPARQL{}

// sparqlJSONEnvelope mirrors the standard SPARQL 1.1 Query Results JSON Format — the same shape
// go-wikidata-client.Result is decoded from, re-marshaled here so a Mapper's Map(RawBatch) contract
// (bytes in) doesn't need to change to consume a typed client.
type sparqlJSONEnvelope struct {
	Head struct {
		Vars []string `json:"vars"`
	} `json:"head"`
	Results struct {
		Bindings []map[string]sparqlJSONValue `json:"bindings"`
	} `json:"results"`
}

type sparqlJSONValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Fetch parses the endpoint + query out of src.Locator, runs the query via go-wikidata-client, and
// re-encodes the result as a RawBatch. SourceVersion/Checksum are a content checksum — a SPARQL query
// result carries no ETag/Last-Modified to key idempotency off, so this mirrors the `file` connector's
// checksum-only fallback.
func (w WikidataSPARQL) Fetch(ctx context.Context, src domain.Source) (domain.RawBatch, error) {
	u, err := url.Parse(src.Locator)
	if err != nil {
		return domain.RawBatch{}, fmt.Errorf("connector wikidata-sparql: parse locator: %w", err)
	}
	query := u.Query().Get("query")
	if query == "" {
		return domain.RawBatch{}, fmt.Errorf("connector wikidata-sparql: locator %q has no query= parameter", src.Locator)
	}
	endpoint := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()

	c := wikidataclient.New(w.client, wikidataclient.WithEndpoint(endpoint), wikidataclient.WithUserAgent(userAgent))
	result, err := c.Query(ctx, query)
	if err != nil {
		return domain.RawBatch{}, fmt.Errorf("connector wikidata-sparql: %w", err)
	}

	var env sparqlJSONEnvelope
	env.Head.Vars = result.Vars
	env.Results.Bindings = make([]map[string]sparqlJSONValue, 0, len(result.Rows))
	for _, row := range result.Rows {
		b := make(map[string]sparqlJSONValue, len(row))
		for k, v := range row {
			b[k] = sparqlJSONValue{Type: v.Type, Value: v.Value}
		}
		env.Results.Bindings = append(env.Results.Bindings, b)
	}

	payload, err := json.Marshal(env)
	if err != nil {
		return domain.RawBatch{}, fmt.Errorf("connector wikidata-sparql: encode result: %w", err)
	}
	sum := checksum(payload)
	return domain.RawBatch{Payload: payload, SourceVersion: sum[:12], Checksum: sum}, nil
}
