// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Dashboard bucket-label wiring (M57 / D-ObjectFacets). A `ref` facet's bucket key is a RID — a
// country, a rank, a unit — and a chart axis of RID tails is unreadable, so each ref target type gets
// a resolver returning a locale→text display name (D-i18n: all locales in every response).
//
// It reuses the D-LinkTraversal labelers verbatim (link_descriptors.go's overlayLabeler), which is
// the point: a unit must be named identically in a graph row and in a chart segment, and a second
// name-resolution path would be a second thing to keep in step with the translation store.
//
// Coverage is asserted at boot rather than discovered in production: every registered facet's
// RefType must appear in the table below, so a new ref facet fails startup instead of silently
// shipping a dashboard labelled with RID tails.
package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
	linksapp "github.com/olegamysk/go-oikumenea/internal/links/application"
	localizationapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

// refLabelSources is one row per ref-facet target type: where its display text lives and which i18n
// entity type overlays it. `person` is absent because its names live in the per-person variant table
// rather than the translation store (personLabeler), and is added separately below.
func refLabelSources() []struct{ typ, table, col, entity string } {
	return []struct{ typ, table, col, entity string }{
		{"unit", "tenant_units", "name", "unit"},
		{"organization", "tenant_organizations", "name", "organization"},
		{"domain", "tenant_domains", "name", "domain"},
		{"unit_kind", "tenant_unit_kinds", "name", "unit_kind"},
		{"rank", "rank_ranks", "name", "rank"},
		{"country", "geo_countries", "name", "country"},
		{"position", "membership_positions", "title", "position"},
		{"order_type", "order_order_types", "name", "order_type"},
		{"document_type", "document_document_types", "name", "document_type"},
		// M58 ticket 2 — the external-organization and taxonomy dashboards. `taxon` resolves twice
		// over: as the `religionId` facet's root and as the `subtree` facet's ancestor, which is one
		// table and therefore one resolver.
		{"external_org_kind", "external_org_kinds", "name", "external_org_kind"},
		{"taxon", "religion_taxa", "name", "taxon"},
		{"taxon_rank", "religion_taxon_ranks", "name", "taxon_rank"},
		{"classification", "religion_classifications", "name", "classification"},
	}
}

// bucketLabeler builds the per-object-type resolver the stats kernel calls, one batch per ref facet
// target. An unregistered type resolves to nothing rather than an error: the bucket keeps its RID and
// the client falls back to the RID tail, exactly as an unlabelled link row does — a dashboard is
// never withheld because a name is missing.
func bucketLabeler(pool *pgxpool.Pool, loc *localizationapp.Service) stats.Labeler {
	byType := map[string]linksapp.LabelFunc{"person": personLabeler(pool, loc)}
	for _, l := range refLabelSources() {
		byType[l.typ] = overlayLabeler(pool, loc, l.table, l.col, l.entity)
	}
	return func(ctx context.Context, objectType string, ids []string) (map[string]map[string]string, error) {
		fn, ok := byType[objectType]
		if !ok || len(ids) == 0 {
			return nil, nil
		}
		return fn(ctx, ids)
	}
}

// assertBucketLabelersBound is the boot seam check (the D-ObjectFacets counterpart of the links
// engine's MustBeBound): every ref facet's target type must have a resolver. Pure — it compares two
// in-memory sets and touches no database, so it is a startup assertion rather than a probe.
func assertBucketLabelersBound() error {
	covered := map[string]bool{"person": true}
	for _, l := range refLabelSources() {
		covered[l.typ] = true
	}
	missing := map[string]bool{}
	for _, o := range facet.Default.All() {
		for _, f := range o.Facets {
			if f.Kind == facet.KindRef && !covered[f.RefType] {
				missing[f.RefType] = true
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	out := make([]string, 0, len(missing))
	for t := range missing {
		out = append(out, t)
	}
	sort.Strings(out)
	return fmt.Errorf("stats bucket labelers: no resolver for ref facet target type(s) %v "+
		"(cmd/oikumenea/stats_labelers.go) — their chart segments would show RID tails", out)
}
