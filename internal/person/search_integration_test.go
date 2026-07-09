//go:build integration

// Integration tests for the person directory SEARCH (review-2026-07 R-06) against a real Postgres:
// the pg_trgm-indexed admin search (SearchPersons) and the scoped search folded into the read-scope
// semi-join (VisiblePersonIDsForSubjectSearch). These assert the two behaviours the fix is about:
//
//  1. a person is findable by a native-script name variant / alias, not just the Latin display name;
//
//  2. the scoped search filters IN SQL (no Go post-filter), so it never returns an empty page while
//     a next-page token is set, and never leaks an out-of-reach person even when it matches the query.
//
//     OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//     go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

func seedPersonNamed(t *testing.T, svc *application.Service, display string) string {
	t.Helper()
	p, err := svc.CreatePerson(context.Background(), domain.Person{Name: domain.Name{DisplayName: display}})
	if err != nil {
		t.Fatalf("create person %q: %v", display, err)
	}
	return p.ID
}

// seedNameVariant inserts a per-person name form (transliteration / alias) directly, mirroring the
// raw-SQL seeding the other read-scope tests use. The search_text generated column + GIN index make
// it searchable; variant_kind distinguishes a transliteration from an aka.
func seedNameVariant(t *testing.T, pool *pgxpool.Pool, personID, locale, display, kind string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO oikumenea.person_name_variants (person_id, locale, display_name, variant_kind)
		 VALUES ($1, $2, $3, $4)`,
		personID, locale, display, kind); err != nil {
		t.Fatalf("seed name variant %q: %v", display, err)
	}
}

func idSet(page application.Page) map[string]bool {
	got := map[string]bool{}
	for _, p := range page.Persons {
		got[p.ID] = true
	}
	return got
}

// TestAdminSearch_VariantAndAlias covers the correctness gap: the admin directory search finds a
// person by a Cyrillic transliteration and by an alias, not only by the Latin display name.
func TestAdminSearch_VariantAndAlias_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	ctx := context.Background()

	special := seedPersonNamed(t, svc, "Oleh Zelenko")
	seedNameVariant(t, pool, special, "ukr", "Олег Зеленко", "transliteration")
	seedNameVariant(t, pool, special, "eng", "Shadow Wolf", "aka")
	_ = seedPersonNamed(t, svc, "Jane Roe") // a non-matching decoy

	for _, tc := range []struct{ name, query string }{
		{"latin display", "Zelenko"},
		{"cyrillic variant", "Зеленко"},
		{"alias", "Shadow"},
	} {
		page, err := svc.ListPersons(ctx, 0, "", tc.query)
		if err != nil {
			t.Fatalf("%s: ListPersons(%q): %v", tc.name, tc.query, err)
		}
		if !idSet(page)[special] {
			t.Fatalf("%s: search %q must find the person via its %s form", tc.name, tc.query, tc.name)
		}
	}

	// A query that matches nothing returns an empty page.
	none, err := svc.ListPersons(ctx, 0, "", "zzz-no-such-name")
	if err != nil {
		t.Fatalf("ListPersons(no-match): %v", err)
	}
	if len(none.Persons) != 0 {
		t.Fatalf("no-match query must return an empty page, got %d", len(none.Persons))
	}
}

// TestScopedSearch covers the scoped path: search is filtered in SQL, so it (a) respects read-scope
// (never leaks an out-of-reach match), (b) finds an in-reach person by a variant/alias, and (c)
// never returns an empty page while a next-page token is set.
func TestScopedSearch_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	ctx := context.Background()

	unitA := seedUnit(t, pool)
	unitB := seedUnit(t, pool)
	reader := seedPerson(t, svc)
	seedReadGrant(t, pool, reader, unitA) // reach = {unitA}

	// Five in-reach persons all matching "Falcon", one findable only by a Cyrillic variant, plus an
	// out-of-reach person that also matches the query (must never surface).
	inReach := map[string]bool{}
	for i := 0; i < 4; i++ {
		p := seedPersonNamed(t, svc, "Falcon Agent")
		seedMembership(t, pool, p, unitA)
		inReach[p] = true
	}
	variantOnly := seedPersonNamed(t, svc, "Xavier Reed") // Latin name does NOT contain "Falcon"
	seedNameVariant(t, pool, variantOnly, "ukr", "Фалькон Рід Falcon", "transliteration")
	seedMembership(t, pool, variantOnly, unitA)
	inReach[variantOnly] = true

	outOfReach := seedPersonNamed(t, svc, "Falcon Impostor")
	seedMembership(t, pool, outOfReach, unitB)

	// Drain the scoped search in size-2 pages: collect every match, and assert no page is empty while
	// a next-page token is set (the bug the SQL fold fixes) and the out-of-reach match never appears.
	got := map[string]bool{}
	token := ""
	for pages := 0; ; pages++ {
		if pages > 100 {
			t.Fatal("scoped search paging did not terminate")
		}
		page, err := svc.ListVisiblePersons(ctx, reader, 2, token, "Falcon")
		if err != nil {
			t.Fatalf("ListVisiblePersons(search): %v", err)
		}
		if page.NextPageToken != "" && len(page.Persons) == 0 {
			t.Fatal("scoped search returned an empty page while hasMore — the R-06 fold regressed")
		}
		for _, p := range page.Persons {
			if p.ID == outOfReach {
				t.Fatalf("scoped search leaked the out-of-reach match %s", outOfReach)
			}
			got[p.ID] = true
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}

	for id := range inReach {
		if !got[id] {
			t.Fatalf("scoped search missed in-reach match %s (incl. the variant-only person)", id)
		}
	}
	if len(got) != len(inReach) {
		t.Fatalf("scoped search returned %d persons, want %d in-reach matches", len(got), len(inReach))
	}
}
