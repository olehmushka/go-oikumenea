// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the person facet filters (M56 / D-ObjectFacets) against a real Postgres.
//
// What matters here is not that a filter "works" but that it works IDENTICALLY on the two list
// paths — the instance-admin path (person's own ListPersons/SearchPersons) and the read-scope path
// (membership's three visibility queries) — because they are five separate SQL blocks carrying the
// same vocabulary. So each facet is asserted twice, and then the two paths are compared directly:
//
//	scoped(f) == admin(f) ∩ reach     for every filter f
//
// which is the property that makes the duplication safe. Paging is asserted with pageSize=1 across
// a filtered set, because the whole reason the predicates live inside the SQL is that a Go-side
// re-filter would hand back a short page with a nextPageToken (review-2026-07 R-06).
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/person/... -run Facet
package person_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
)

func sp(s string) *string    { return &s }
func bp(b bool) *bool        { return &b }
func dp(s string) *time.Time { t, _ := time.Parse(domain.ISODate, s); return &t }

// facetWorld is a small directory with one readable unit, seeded so that every facet has at least
// one matching and one non-matching person INSIDE the reader's reach — otherwise a filter could
// "pass" by returning nothing on both paths.
type facetWorld struct {
	svc  *application.Service
	pool *pgxpool.Pool

	unit   string // the reader's readable unit
	reader string

	alice  string // female, active,   born 1990-03-04, has account, has rank
	bogdan string // male,   active,   born 1975-11-20, no account,  no rank
	clara  string // female, provisional, birthdate NULL, no account, no rank

	rank    string
	country string

	// tag is a per-run marker carried in every seeded display name, so a test can narrow the
	// INSTANCE-ADMIN directory — which spans every other suite's rows in the shared test database —
	// down to this world without weakening what it asserts.
	tag string
}

func seedFacetWorld(t *testing.T) *facetWorld {
	t.Helper()
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	ctx := context.Background()

	w := &facetWorld{svc: svc, pool: pool, tag: code(t, "facetworld")}
	w.unit = seedUnit(t, pool)
	w.country = seedFacetCountry(t, pool)
	w.rank = seedFacetRank(t, pool)

	w.alice = seedPerson(t, svc)
	w.bogdan = seedPerson(t, svc)
	w.clara = seedPerson(t, svc)
	for _, id := range []string{w.alice, w.bogdan, w.clara} {
		seedMembership(t, pool, id, w.unit)
	}

	exec(t, pool, `UPDATE oikumenea.person_persons
		SET sex='female', status='active', birthdate=DATE '1990-03-04', country_of_birth_id=$2
		WHERE id=$1`, w.alice, w.country)
	exec(t, pool, `UPDATE oikumenea.person_persons
		SET sex='male', status='active', birthdate=DATE '1975-11-20'
		WHERE id=$1`, w.bogdan)
	exec(t, pool, `UPDATE oikumenea.person_persons
		SET sex='female', status='provisional', birthdate=NULL
		WHERE id=$1`, w.clara)

	exec(t, pool, `INSERT INTO oikumenea.person_ranks (person_id, system_id, rank_id)
		SELECT $1, r.system_id, r.id FROM oikumenea.rank_ranks r WHERE r.id=$2`, w.alice, w.rank)
	exec(t, pool, `INSERT INTO oikumenea.account_accounts (person_id) VALUES ($1)`, w.alice)

	// The tag goes into display_name (and so into the generated search haystack), which is what lets
	// the admin-path assertions below scope to this world.
	for _, id := range []string{w.alice, w.bogdan, w.clara} {
		exec(t, pool, `UPDATE oikumenea.person_persons SET display_name = $2 || ' ' || display_name WHERE id=$1`,
			id, w.tag)
	}

	w.reader = seedPerson(t, svc)
	seedReadGrant(t, pool, w.reader, w.unit)
	// The reader is in the unit too, so "everyone the reader can see" is a real four-person set and
	// the intersection assertions below are not trivially the whole directory.
	seedMembership(t, pool, w.reader, w.unit)
	_ = ctx
	return w
}

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %.60s: %v", sql, err)
	}
}

// seedFacetCountry returns a geo country RID. The bootstrap country skeleton is seeded by the
// migrations, so this reuses an existing row rather than minting one (the ISO alpha-2 `code` is
// globally unique, and the facet only needs a real FK target).
func seedFacetCountry(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.geo_countries ORDER BY code LIMIT 1`).Scan(&id); err == nil {
		return id
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.geo_countries (code, name) VALUES ('ZZ', 'Facetland') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed country: %v", err)
	}
	return id
}

// seedFacetRank returns a rank RID, seeding a minimal system→category→type→rank chain if needed.
func seedFacetRank(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM oikumenea.rank_ranks WHERE deleted_at IS NULL LIMIT 1`).Scan(&id); err == nil {
		return id
	}
	var systemID, catID, typeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_systems (code, name, sort_order) VALUES ($1,'Facet system',1) RETURNING id`,
		code(t, "rs")).Scan(&systemID); err != nil {
		t.Fatalf("seed rank system: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_categories (system_id, code, name, sort_order) VALUES ($1,$2,'Cat',1) RETURNING id`,
		systemID, code(t, "rc")).Scan(&catID); err != nil {
		t.Fatalf("seed rank category: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_types (system_id, category_id, code, name, sort_order) VALUES ($1,$2,$3,'Type',1) RETURNING id`,
		systemID, catID, code(t, "rt")).Scan(&typeID); err != nil {
		t.Fatalf("seed rank type: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_ranks (type_id, system_id, code, name, sort_order) VALUES ($1,$2,$3,'Rank',1) RETURNING id`,
		typeID, systemID, code(t, "rr")).Scan(&id); err != nil {
		t.Fatalf("seed rank: %v", err)
	}
	return id
}

// idsOf collects a page's person ids as a set.
func idsOf(page application.Page) map[string]bool {
	out := map[string]bool{}
	for _, p := range page.Persons {
		out[p.ID] = true
	}
	return out
}

// allIDs pages a list to exhaustion. The test database is shared, so the admin directory holds rows
// from every other suite; comparing a single page against a seeded expectation would fail on
// pagination rather than on filtering. Exhaustive paging is also the exact shape M56's exit
// criterion names (a filtered set equals the rows obtained by paging the same list).
func allIDs(t *testing.T, list func(pageToken string) (application.Page, error)) map[string]bool {
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
		for _, p := range page.Persons {
			out[p.ID] = true
		}
		if page.NextPageToken == "" {
			return out
		}
		token = page.NextPageToken
	}
}

// within restricts a result set to the ids this test seeded, so unrelated rows in the shared
// database cannot make an assertion pass or fail for the wrong reason.
func within(got map[string]bool, world []string) map[string]bool {
	out := map[string]bool{}
	for _, id := range world {
		if got[id] {
			out[id] = true
		}
	}
	return out
}

// people is every person seedFacetWorld created — the universe these tests reason about.
func (w *facetWorld) people() []string {
	return []string{w.alice, w.bogdan, w.clara, w.reader}
}

// TestFacetFiltersOnBothPaths asserts every facet selects the SAME people whether the caller is an
// instance admin (person's own queries) or a scoped reader (membership's visibility queries).
func TestFacetFiltersOnBothPaths_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		f       domain.PersonFilter
		want    []string
		exclude []string
	}{
		{"sex=female", domain.PersonFilter{Sex: sp("female")},
			[]string{w.alice, w.clara}, []string{w.bogdan}},
		{"sex=male", domain.PersonFilter{Sex: sp("male")},
			[]string{w.bogdan}, []string{w.alice, w.clara}},
		{"status=provisional", domain.PersonFilter{Status: sp("provisional")},
			[]string{w.clara}, []string{w.alice, w.bogdan}},
		{"birthdate window covering alice only", domain.PersonFilter{
			BirthdateFrom: dp("1985-01-01"), BirthdateTo: dp("1995-01-01")},
			[]string{w.alice}, []string{w.bogdan, w.clara}},
		{"birthdate lower bound excludes the unknown birthdate", domain.PersonFilter{
			BirthdateFrom: dp("1900-01-01")},
			[]string{w.alice, w.bogdan}, []string{w.clara}},
		{"countryOfBirth", domain.PersonFilter{CountryOfBirth: sp(w.country)},
			[]string{w.alice}, []string{w.bogdan, w.clara}},
		{"rankId", domain.PersonFilter{RankID: sp(w.rank)},
			[]string{w.alice}, []string{w.bogdan, w.clara}},
		{"hasAccount=true", domain.PersonFilter{HasAccount: bp(true)},
			[]string{w.alice}, []string{w.bogdan, w.clara}},
		{"hasAccount=false", domain.PersonFilter{HasAccount: bp(false)},
			[]string{w.bogdan, w.clara}, []string{w.alice}},
		{"unitId (direct membership)", domain.PersonFilter{UnitID: sp(w.unit)},
			[]string{w.alice, w.bogdan, w.clara}, nil},
		{"two facets compose", domain.PersonFilter{Sex: sp("female"), Status: sp("active")},
			[]string{w.alice}, []string{w.bogdan, w.clara}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adminIDs := allIDs(t, func(tok string) (application.Page, error) {
				return w.svc.ListPersons(ctx, tc.f, 0, tok)
			})
			scopedIDs := allIDs(t, func(tok string) (application.Page, error) {
				return w.svc.ListVisiblePersons(ctx, w.reader, tc.f, 0, tok)
			})
			for _, id := range tc.want {
				if !adminIDs[id] {
					t.Errorf("admin path missing %s", id)
				}
				if !scopedIDs[id] {
					t.Errorf("scoped path missing %s (it IS in the reader's reach)", id)
				}
			}
			for _, id := range tc.exclude {
				if adminIDs[id] {
					t.Errorf("admin path leaked filtered-out %s", id)
				}
				if scopedIDs[id] {
					t.Errorf("scoped path leaked filtered-out %s", id)
				}
			}
		})
	}
}

// TestFacetAdminScopedParity is the differential contract: for every filter, the scoped result is
// exactly the admin result intersected with the reader's reach. This is what proves the five SQL
// blocks agree — a drifted predicate in one of membership's queries shows up here as an asymmetry.
func TestFacetAdminScopedParity_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()

	reach := map[string]bool{w.alice: true, w.bogdan: true, w.clara: true, w.reader: true}

	filters := []domain.PersonFilter{
		{},
		{Sex: sp("female")},
		{Sex: sp("male")},
		{Status: sp("active")},
		{Status: sp("provisional")},
		{HasAccount: bp(true)},
		{HasAccount: bp(false)},
		{RankID: sp(w.rank)},
		{CountryOfBirth: sp(w.country)},
		{UnitID: sp(w.unit)},
		{BirthdateFrom: dp("1980-01-01")},
		{BirthdateTo: dp("1980-01-01")},
		{Sex: sp("female"), HasAccount: bp(false)},
		{Sex: sp("female"), Status: sp("active"), UnitID: sp(w.unit)},
	}

	for i, f := range filters {
		// Restricted to the seeded world: the shared test database holds unrelated persons that are
		// outside this reader's reach by construction, and they would swamp the comparison.
		adminIDs := within(allIDs(t, func(tok string) (application.Page, error) {
			return w.svc.ListPersons(ctx, f, 0, tok)
		}), w.people())
		scopedIDs := within(allIDs(t, func(tok string) (application.Page, error) {
			return w.svc.ListVisiblePersons(ctx, w.reader, f, 0, tok)
		}), w.people())

		// scoped ⊆ admin ∩ reach
		for id := range scopedIDs {
			if !adminIDs[id] {
				t.Errorf("filter %d: scoped returned %s which the admin filter excludes", i, id)
			}
			if !reach[id] {
				t.Errorf("filter %d: scoped returned %s which is outside the reader's reach", i, id)
			}
		}
		// admin ∩ reach ⊆ scoped
		for id := range adminIDs {
			if reach[id] && !scopedIDs[id] {
				t.Errorf("filter %d: %s matches the filter and is in reach, but the scoped path dropped it", i, id)
			}
		}
	}
}

// TestFacetPagingIsNotShort: the predicates run before the LIMIT, so walking a filtered set one row
// at a time must yield every match with no empty-page-while-hasMore. This is the property a Go-side
// post-filter would break, on BOTH paths.
func TestFacetPagingIsNotShort_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()
	// alice + clara. The admin variant also carries the world tag as a text query: the admin
	// directory spans every other suite's rows, so without it a one-row-at-a-time drain would need
	// thousands of pages to reach the seeded people and would be asserting pagination volume rather
	// than the no-short-page property. The tag rides through SearchPersons, which carries the SAME
	// facet block, so the property under test is unchanged.
	f := domain.PersonFilter{Sex: sp("female")}
	adminF := domain.PersonFilter{Sex: sp("female"), Query: w.tag}

	for _, path := range []struct {
		name string
		list func(pageToken string) (application.Page, error)
	}{
		{"admin", func(tok string) (application.Page, error) { return w.svc.ListPersons(ctx, adminF, 1, tok) }},
		{"scoped", func(tok string) (application.Page, error) {
			return w.svc.ListVisiblePersons(ctx, w.reader, f, 1, tok)
		}},
	} {
		t.Run(path.name, func(t *testing.T) {
			seen := map[string]bool{}
			token := ""
			for i := 0; i < 50; i++ {
				page, err := path.list(token)
				if err != nil {
					t.Fatalf("page %d: %v", i, err)
				}
				if len(page.Persons) == 0 && page.NextPageToken != "" {
					t.Fatalf("page %d is EMPTY but carries a nextPageToken — a predicate is running after the LIMIT", i)
				}
				for _, p := range page.Persons {
					seen[p.ID] = true
				}
				if page.NextPageToken == "" {
					break
				}
				token = page.NextPageToken
			}
			if !seen[w.alice] || !seen[w.clara] {
				t.Errorf("paging one row at a time lost a match: alice=%v clara=%v", seen[w.alice], seen[w.clara])
			}
			if seen[w.bogdan] {
				t.Error("paging returned a person the filter excludes")
			}
		})
	}
}

// TestScopedListSkipsSoftDeletedPersons is the regression for the bug the facet join closed: the
// visibility queries used to return soft-deleted person ids, which the hydration step then silently
// dropped — producing a page SHORTER than pageSize while still handing back a nextPageToken.
func TestScopedListSkipsSoftDeletedPersons_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()

	exec(t, w.pool, `UPDATE oikumenea.person_persons SET deleted_at = now() WHERE id=$1`, w.bogdan)

	page, err := w.svc.ListVisiblePersons(ctx, w.reader, domain.PersonFilter{}, 0, "")
	if err != nil {
		t.Fatalf("ListVisiblePersons: %v", err)
	}
	if idsOf(page)[w.bogdan] {
		t.Fatal("the scoped list returned a soft-deleted person")
	}

	// Walk one row at a time: every page must be full until the last, which is exactly what the
	// silently-dropped id used to break.
	token := ""
	for i := 0; i < 50; i++ {
		p, err := w.svc.ListVisiblePersons(ctx, w.reader, domain.PersonFilter{}, 1, token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		if p.NextPageToken != "" && len(p.Persons) != 1 {
			t.Fatalf("page %d returned %d rows but has more — a soft-deleted id was counted toward the page",
				i, len(p.Persons))
		}
		if p.NextPageToken == "" {
			break
		}
		token = p.NextPageToken
	}
}

// TestFacetValidationRejectsIdentically: an ill-formed facet value must be refused the same way on
// both paths, since validation runs once in the application layer.
func TestFacetValidationRejectsIdentically_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()

	for _, bad := range []domain.PersonFilter{
		{Sex: sp("f")},
		{Status: sp("retired")},
		{CountryOfBirth: sp("UA")},
		{BirthdateFrom: dp("2000-01-01"), BirthdateTo: dp("1990-01-01")},
		{Graph: "command"}, // graph without unitId
	} {
		if _, err := w.svc.ListPersons(ctx, bad, 0, ""); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("admin path accepted %+v (err=%v)", bad, err)
		}
		if _, err := w.svc.ListVisiblePersons(ctx, w.reader, bad, 0, ""); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("scoped path accepted %+v (err=%v)", bad, err)
		}
	}
}

// TestFacetsComposeWithSearch: a text query and a structural facet must AND together, on both the
// admin search query and membership's search-shaped visibility query.
func TestFacetsComposeWithSearch_Integration(t *testing.T) {
	w := seedFacetWorld(t)
	ctx := context.Background()

	exec(t, w.pool, `UPDATE oikumenea.person_persons SET display_name='Facetsearch Alice' WHERE id=$1`, w.alice)
	exec(t, w.pool, `UPDATE oikumenea.person_persons SET display_name='Facetsearch Bogdan' WHERE id=$1`, w.bogdan)

	f := domain.PersonFilter{Query: "Facetsearch", Sex: sp("female")}

	admin := allIDs(t, func(tok string) (application.Page, error) { return w.svc.ListPersons(ctx, f, 0, tok) })
	if !admin[w.alice] || admin[w.bogdan] {
		t.Errorf("admin search+facet: alice=%v bogdan=%v, want alice only", admin[w.alice], admin[w.bogdan])
	}

	scoped := allIDs(t, func(tok string) (application.Page, error) {
		return w.svc.ListVisiblePersons(ctx, w.reader, f, 0, tok)
	})
	if !scoped[w.alice] || scoped[w.bogdan] {
		t.Errorf("scoped search+facet: alice=%v bogdan=%v, want alice only", scoped[w.alice], scoped[w.bogdan])
	}
}
