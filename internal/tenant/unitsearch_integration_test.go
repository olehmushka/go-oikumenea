// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the unit text search (migration 0022), the predicate that lets the console's
// unit picker stop filtering one page in the browser.
//
// The bug this closes was invisible: a picker that loads `pageSize=200` and filters client-side
// reports "No matches" for units that plainly exist, once an org outgrows a page. So the properties
// worth pinning are not "ILIKE works" but the ones whose failure is silent — matching on `code` as
// well as `name`, matching CODELESS units at all, composing with the other facets rather than
// replacing them, and the list and its dashboard counting the same set.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/tenant/... -run UnitSearch
package tenant_test

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/tenant/application"
	"github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func TestUnitSearch_Integration(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	org := seedOrg(t, svc)

	// A named unit WITH a code, and one deliberately WITHOUT — `code` is nullable (NULL = a
	// non-separate sub-unit), and the generated haystack coalesces it. Without that coalesce
	// `lower(code || ' ' || name)` is NULL for every codeless unit and they vanish from the index
	// entirely; in the dev database every single unit is codeless, so this is the common case, not
	// the edge one.
	coded := mustCreate(t, svc, org, uniqueCode(t, "alpha-recon"))
	codeless, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Bravo Signals Platoon"})
	if err != nil {
		t.Fatalf("create codeless unit: %v", err)
	}
	other := mustCreate(t, svc, org, uniqueCode(t, "zulu-supply"))

	// A shadow unit, to pin that search NARROWS and cannot widen: the visibility gate runs after the
	// page is cut, so this row is returned by the application layer and trimmed by the transport —
	// exactly as it would be with no query at all.
	shadow, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Bravo Hidden Cell"})
	if err != nil {
		t.Fatalf("create shadow unit: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE oikumenea.tenant_units SET visibility='shadow' WHERE id=$1`, shadow.ID); err != nil {
		t.Fatalf("seed shadow visibility: %v", err)
	}

	cases := []struct {
		name    string
		f       domain.UnitFilter
		want    []string
		exclude []string
	}{
		{
			"matches on NAME",
			domain.UnitFilter{OrgID: org.ID, Query: "signals"},
			[]string{codeless.ID}, []string{coded.ID, other.ID},
		},
		{
			// The picker shows code as the hint and users type it; a name-only haystack would make
			// the most precise thing a user can type the one thing that finds nothing.
			"matches on CODE",
			domain.UnitFilter{OrgID: org.ID, Query: "alpha-recon"},
			[]string{coded.ID}, []string{codeless.ID, other.ID},
		},
		{
			"a CODELESS unit is still findable",
			domain.UnitFilter{OrgID: org.ID, Query: "Bravo Signals"},
			[]string{codeless.ID}, []string{coded.ID, other.ID},
		},
		{
			"case-insensitive",
			domain.UnitFilter{OrgID: org.ID, Query: "bRaVo SiGnAlS"},
			[]string{codeless.ID}, []string{other.ID},
		},
		{
			"substring, not prefix",
			domain.UnitFilter{OrgID: org.ID, Query: "ignals Plat"},
			[]string{codeless.ID}, []string{other.ID},
		},
		{
			"no match yields nothing rather than everything",
			domain.UnitFilter{OrgID: org.ID, Query: "no-such-unit-anywhere"},
			nil, []string{coded.ID, codeless.ID, other.ID},
		},
		{
			// The whole point of the separate SearchUnits statement is that it carries ListUnits'
			// filter block verbatim; if the two drifted, a query would silently drop the facets.
			"composes with a facet (ANDed, not replacing it)",
			domain.UnitFilter{OrgID: org.ID, Query: "Bravo", Visibility: strp("public")},
			[]string{codeless.ID}, []string{shadow.ID, other.ID},
		},
		{
			"search NARROWS only — a shadow unit is not surfaced by matching text",
			domain.UnitFilter{OrgID: org.ID, Query: "Bravo"},
			[]string{codeless.ID, shadow.ID}, []string{other.ID},
		},
		{
			// Whitespace is trimmed to empty by the application layer, so this must be "no predicate"
			// and not a search for a space (which would match every multi-word name and look like it
			// worked).
			"a whitespace-only query is no predicate at all",
			domain.UnitFilter{OrgID: org.ID, Query: "   "},
			[]string{coded.ID, codeless.ID, other.ID}, nil,
		},
		{
			"an empty query lists the org",
			domain.UnitFilter{OrgID: org.ID},
			[]string{coded.ID, codeless.ID, other.ID}, nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := allUnitIDs(t, func(tok string) (application.UnitPage, error) {
				return svc.ListUnits(ctx, tc.f, "", nil, false, 0, tok)
			})
			for _, id := range tc.want {
				if !got[id] {
					t.Errorf("search missed unit %s", id)
				}
			}
			for _, id := range tc.exclude {
				if got[id] {
					t.Errorf("search leaked unit %s", id)
				}
			}
		})
	}
}

// TestUnitSearchPagingIsNotShort: the text predicate runs INSIDE the query, before the LIMIT, so
// paging a searched set one row at a time must never hand back an empty page that still carries a
// cursor. A post-filter would pass every assertion above and fail this one.
func TestUnitSearchPagingIsNotShort_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	const want = 4
	for i := 0; i < want; i++ {
		if _, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Needle Company"}); err != nil {
			t.Fatalf("create unit: %v", err)
		}
		if _, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Haystack Company"}); err != nil {
			t.Fatalf("create decoy: %v", err)
		}
	}

	f := domain.UnitFilter{OrgID: org.ID, Query: "Needle"}
	seen, token, pages := 0, "", 0
	for {
		page, err := svc.ListUnits(ctx, f, "", nil, false, 1, token)
		if err != nil {
			t.Fatalf("page: %v", err)
		}
		pages++
		if pages > want+2 {
			t.Fatalf("paging did not terminate after %d pages", pages)
		}
		if len(page.Units) == 0 && page.NextPageToken != "" {
			t.Fatalf("empty page with a cursor — the text predicate is a post-filter, not part of the query")
		}
		seen += len(page.Units)
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	if seen != want {
		t.Fatalf("paged %d matching units, want %d", seen, want)
	}
}

// TestUnitSearchStatsAgreeWithList: D-ObjectFacets' standing rule is that a dashboard aggregates
// EXACTLY the set the list would page under the same filter. `query` is the first unit filter that
// had to be added to two separate SQL statements (the list splits for planning reasons, the stats CTE
// does not), so it is the one most able to drift.
func TestUnitSearchStatsAgreeWithList_Integration(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	org := seedOrg(t, svc)

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Recon Detachment"}); err != nil {
			t.Fatalf("create unit: %v", err)
		}
	}
	if _, err := svc.CreateUnit(ctx, domain.Unit{OrgID: org.ID, DomainID: org.DomainID, Name: "Medical Detachment"}); err != nil {
		t.Fatalf("create decoy: %v", err)
	}

	for _, q := range []string{"", "Recon", "Detachment", "nothing-matches-this"} {
		t.Run("query="+q, func(t *testing.T) {
			f := domain.UnitFilter{OrgID: org.ID, Query: q}
			listed := len(allUnitIDs(t, func(tok string) (application.UnitPage, error) {
				return svc.ListUnits(ctx, f, "", nil, false, 0, tok)
			}))
			// The instance-admin arm: an empty subject with isAdmin true, matching how the transport
			// calls it for an admin caller.
			res, err := svc.UnitStats(ctx, "", true, f, stats.Selection{})
			if err != nil {
				t.Fatalf("unit stats: %v", err)
			}
			if int(res.TotalCount) != listed {
				t.Fatalf("dashboard counted %d, list paged %d — a searched list and its dashboard describe different worlds", res.TotalCount, listed)
			}
		})
	}
}
