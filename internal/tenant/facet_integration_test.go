// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the unit facet filters (M56 / D-ObjectFacets) against a real Postgres.
//
// Three of the seven unit facets are new in M56 (visibility, state, pdpScoped); the rest
// retro-declare args listUnits already shipped, and are covered here too so the whole vocabulary has
// one round-trip proof. The traversal modes (parent / rootsOnly) must keep IGNORING the flat filters
// — that is the contract they have always had, and quietly starting to honour them would change
// what an expand-on-click tree shows.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/... -run Facet
package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

// allUnitIDs pages a unit listing to exhaustion — the shared test database holds units from every
// other suite, so a single page proves nothing about a filter.
func allUnitIDs(t *testing.T, list func(pageToken string) (application.UnitPage, error)) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	token := ""
	for i := 0; ; i++ {
		if i > 500 {
			t.Fatal("paging did not terminate after 500 pages")
		}
		page, err := list(token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		for _, u := range page.Units {
			out[u.ID] = true
		}
		if page.NextPageToken == "" {
			return out
		}
		token = page.NextPageToken
	}
}

// TestUnitFacetFilters exercises every unit facet against one org seeded with a known mix.
func TestUnitFacetFilters_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	public := mustCreate(t, svc, org, uniqueCode(t, "pub"))
	shadow := mustCreate(t, svc, org, uniqueCode(t, "shd"))
	suspended := mustCreate(t, svc, org, uniqueCode(t, "susp"))
	reference := mustCreate(t, svc, org, uniqueCode(t, "ref"))
	levelled := mustCreate(t, svc, org, uniqueCode(t, "lvl"))

	// Set the facet columns directly: visibility/state/pdp_scoped are otherwise driven by lifecycle
	// operations and the domain derivation, and this test is about the FILTER, not those paths.
	for sql, id := range map[string]string{
		`UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`: shadow.ID,
		`UPDATE oikumenea.tenant_units SET state='suspended' WHERE id=$1`:   suspended.ID,
		`UPDATE oikumenea.tenant_units SET pdp_scoped=false WHERE id=$1`:    reference.ID,
		`UPDATE oikumenea.tenant_units SET level=7 WHERE id=$1`:             levelled.ID,
	} {
		if _, err := pool.Exec(ctx, sql, id); err != nil {
			t.Fatalf("seed facet column: %v", err)
		}
	}

	base := domain.UnitFilter{OrgID: org.ID}
	cases := []struct {
		name    string
		f       domain.UnitFilter
		want    []string
		exclude []string
	}{
		{"visibility=public", domain.UnitFilter{OrgID: org.ID, Visibility: strp("public")},
			[]string{public.ID, suspended.ID}, []string{shadow.ID}},
		{"visibility=shadow", domain.UnitFilter{OrgID: org.ID, Visibility: strp("shadow")},
			[]string{shadow.ID}, []string{public.ID}},
		{"state=active", domain.UnitFilter{OrgID: org.ID, State: strp("active")},
			[]string{public.ID, shadow.ID}, []string{suspended.ID}},
		{"state=suspended", domain.UnitFilter{OrgID: org.ID, State: strp("suspended")},
			[]string{suspended.ID}, []string{public.ID}},
		{"pdpScoped=true", domain.UnitFilter{OrgID: org.ID, PDPScoped: boolp(true)},
			[]string{public.ID}, []string{reference.ID}},
		{"pdpScoped=false", domain.UnitFilter{OrgID: org.ID, PDPScoped: boolp(false)},
			[]string{reference.ID}, []string{public.ID}},
		{"domain", domain.UnitFilter{OrgID: org.ID, DomainID: strp(org.DomainID)},
			[]string{public.ID, shadow.ID}, nil},
		{"level", domain.UnitFilter{OrgID: org.ID, Level: func() *int { n := 7; return &n }()},
			[]string{levelled.ID}, []string{public.ID}},
		{"two facets compose", domain.UnitFilter{OrgID: org.ID, Visibility: strp("public"), State: strp("suspended")},
			[]string{suspended.ID}, []string{public.ID, shadow.ID}},
		{"unfiltered sees them all", base,
			[]string{public.ID, shadow.ID, suspended.ID, reference.ID, levelled.ID}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := allUnitIDs(t, func(tok string) (application.UnitPage, error) {
				return svc.ListUnits(ctx, tc.f, "", nil, false, 0, tok)
			})
			for _, id := range tc.want {
				if !got[id] {
					t.Errorf("filter missed unit %s", id)
				}
			}
			for _, id := range tc.exclude {
				if got[id] {
					t.Errorf("filter leaked unit %s", id)
				}
			}
		})
	}
}

// TestUnitFacetPagingIsNotShort: the predicates run inside the query, so paging a filtered set one
// row at a time must never yield an empty page that still carries a cursor.
func TestUnitFacetPagingIsNotShort_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	var shadows []string
	for i := 0; i < 3; i++ {
		u := mustCreate(t, svc, org, uniqueCode(t, "pshadow"))
		if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`, u.ID); err != nil {
			t.Fatalf("seed shadow: %v", err)
		}
		shadows = append(shadows, u.ID)
	}
	mustCreate(t, svc, org, uniqueCode(t, "ppublic")) // a non-matching row between them

	f := domain.UnitFilter{OrgID: org.ID, Visibility: strp("shadow")}
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 50; i++ {
		page, err := svc.ListUnits(ctx, f, "", nil, false, 1, token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		if len(page.Units) == 0 && page.NextPageToken != "" {
			t.Fatalf("page %d is EMPTY but carries a nextPageToken — a predicate is running after the LIMIT", i)
		}
		for _, u := range page.Units {
			seen[u.ID] = true
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	for _, id := range shadows {
		if !seen[id] {
			t.Errorf("paging one row at a time lost shadow unit %s", id)
		}
	}
}

// TestUnitFacetsIgnoredByTraversalModes: parent/rootsOnly select a different query shape and must
// keep ignoring the flat filters, as the endpoint's contract states.
func TestUnitFacetsIgnoredByTraversalModes_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	parent := mustCreate(t, svc, org, uniqueCode(t, "tparent"))
	child := mustCreate(t, svc, org, uniqueCode(t, "tchild"))
	if _, err := svc.AddEdge(ctx, child.ID, parent.ID, "command"); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`, child.ID); err != nil {
		t.Fatalf("seed shadow child: %v", err)
	}

	// The child is shadow; asking for visibility=public under `parent` must still return it, because
	// the traversal mode ignores the flat filters.
	page, err := svc.ListUnits(ctx,
		domain.UnitFilter{OrgID: org.ID, Visibility: strp("public")}, "command", &parent.ID, false, 0, "")
	if err != nil {
		t.Fatalf("ListUnits(parent): %v", err)
	}
	var found bool
	for _, u := range page.Units {
		if u.ID == child.ID {
			found = true
		}
	}
	if !found {
		t.Error("the parent traversal started honouring the flat visibility filter — that changes what an " +
			"expand-on-click tree shows; the contract says traversal modes ignore the facets")
	}
}

// TestUnitFilterValidation: an ill-formed facet value is a domain error, not a 500 and not a
// silently-ignored filter.
func TestUnitFilterValidation_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	for _, bad := range []domain.UnitFilter{
		{OrgID: org.ID, Visibility: strp("hidden")},
		{OrgID: org.ID, State: strp("deleted")},
		{OrgID: org.ID, DomainID: strp("not-a-rid")},
		{}, // missing the required org scope
	} {
		if _, err := svc.ListUnits(ctx, bad, "", nil, false, 0, ""); !errors.Is(err, domain.ErrInvalidUnit) {
			t.Errorf("accepted %+v (err=%v)", bad, err)
		}
	}
}
