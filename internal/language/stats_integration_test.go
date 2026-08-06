// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the languoid DASHBOARD aggregate (M58 ticket 4 / D-ObjectFacets) against a
// real Postgres holding the imported Glottolog catalog.
//
// Two properties here are not shared with any earlier type:
//
//  1. The COMPOSITE code facet. `macroarea` is set-valued, stored semicolon-joined, and grouped by
//     the literal string. That is only defensible if the bucket key round-trips as a filter value —
//     so `Africa;Eurasia` is asserted explicitly, not just covered by the generic sweep.
//
//  2. The R-21 SEARCH TWIN. LanguoidStats and LanguoidStatsSearch carry one filter block across four
//     queries; if they ever drift, a searched dashboard describes a different set from the searched
//     list with nothing to show that anything is wrong. The parity guard checks the TEXT is
//     identical; this checks the two actually agree against data.
//
// `family_code` is char(8) and space-padded, so its bucket keys are rtrim()ed in SQL. An untrimmed
// key would count rows its own click-through could not return — also asserted below.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/language/... -run Stats
package language_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/language/adapters"
	"github.com/olehmushka/go-oikumenea/internal/language/application"
	"github.com/olehmushka/go-oikumenea/internal/language/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newLanguageService(t *testing.T) *application.Service {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_TEST_DSN")
	if dsn == "" {
		dsn = defaultTestDSN
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	seedMacroareas(t, pool)
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	})
}

// seedMacroareas makes the `macroarea` facet TESTABLE. The language fixture in the test database is a
// small slice of Glottolog with `macroarea` NULL on every row, which would leave that distribution
// with a single `(unknown)` bucket — and `(unknown)` is deliberately not a filter value, so the
// bucket-equals-its-own-filter sweep would pass over the facet entirely without checking anything.
// That is exactly the defect ticket 3 found in seed-demo, in its other form: a chart that renders and
// proves nothing.
//
// It writes a spread INCLUDING a composite (`Africa;Eurasia`), because the composite is the whole
// argument for grouping this column by its literal string rather than unnesting it. Idempotent and
// deterministic by code, so repeated runs against the persistent test database do not drift; the
// production catalog is import-written and never touched by this.
func seedMacroareas(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var withArea int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.language_languoids WHERE macroarea IS NOT NULL`).Scan(&withArea); err != nil {
		t.Fatalf("probe macroareas: %v", err)
	}
	if withArea > 0 {
		return
	}
	// Deterministic buckets by code hash: a real spread, with the composite big enough to survive the
	// top-N cut rather than falling into (other).
	if _, err := pool.Exec(ctx, `
		UPDATE oikumenea.language_languoids SET macroarea = CASE (abs(hashtext(code)) % 5)
			WHEN 0 THEN 'Eurasia'
			WHEN 1 THEN 'Africa'
			WHEN 2 THEN 'Papunesia'
			WHEN 3 THEN 'Africa;Eurasia'
			ELSE NULL
		END`); err != nil {
		t.Fatalf("seed macroareas: %v", err)
	}
}

func allLanguoidFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("languoid")
	if !ok {
		t.Fatal("languoid is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func languoidBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

// pageAllLanguoids exhaustively pages listLanguages under one filter and returns the row count. The
// page size is small relative to the catalog on purpose: a keyset that loses rows at a page boundary
// — the bug ticket 2 found in listTaxa, which had silently dropped 16 of 100 rows since M22 — only
// shows up when the sweep turns over many times. It also asserts no code repeats.
func pageAllLanguoids(t *testing.T, svc *application.Service, f domain.Filter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	f.Limit = 250
	f.After = ""
	for i := 0; i < 500; i++ {
		rows, next, err := svc.ListLanguoidsPage(ctx, f)
		if err != nil {
			t.Fatalf("list languoids: %v", err)
		}
		for _, l := range rows {
			if seen[l.Code] {
				t.Fatalf("languoid %s returned twice — the keyset is broken", l.Code)
			}
			seen[l.Code] = true
		}
		if next == "" {
			return len(seen)
		}
		f.After = next
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

func sptr(s string) *string { return &s }

// TestLanguoidStatsTotalEqualsExhaustivePaging is D-ObjectFacets' headline promise: the number the
// dashboard prints is the number of rows the list would hand over, unfiltered and under each facet.
func TestLanguoidStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)
	sel := allLanguoidFacets(t)

	for name, f := range map[string]domain.Filter{
		"unfiltered":      {},
		"level":           {Level: sptr("language")},
		"status":          {Status: sptr("extinct")},
		"macroarea":       {Macroarea: sptr("Eurasia")},
		"level+macroarea": {Level: sptr("dialect"), Macroarea: sptr("Africa")},
	} {
		res, err := svc.LanguoidStats(ctx, f, sel)
		if err != nil {
			t.Fatalf("%s: stats: %v", name, err)
		}
		if got, want := int(res.TotalCount), pageAllLanguoids(t, svc, f); got != want {
			t.Errorf("%s: totalCount = %d, exhaustive paging returned %d rows", name, got, want)
		}
	}
}

// TestLanguoidStatsEveryBucketEqualsItsOwnFilter is the property the whole vocabulary rests on, and
// the one a chart cannot check for itself: clicking a segment must land on exactly the rows that
// segment counted. It sweeps every non-synthetic bucket of every distribution.
func TestLanguoidStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)

	res, err := svc.LanguoidStats(ctx, domain.Filter{}, allLanguoidFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
				continue // synthetic keys are deliberately not filter values, and not clickable
			}
			var f domain.Filter
			switch d.Facet {
			case "level":
				f.Level = sptr(b.Key)
			case "macroarea":
				f.Macroarea = sptr(b.Key)
			case "status":
				f.Status = sptr(b.Key)
			case "family":
				f.Family = sptr(b.Key)
			default:
				t.Fatalf("undeclared distribution %q in the response", d.Facet)
			}
			if got, want := int(b.Count), pageAllLanguoids(t, svc, f); got != want {
				t.Errorf("%s[%s] counted %d, but its own filter returns %d rows", d.Facet, b.Key, got, want)
			}
		}
	}
}

// TestLanguoidCompositeMacroareaRoundTrips is the macroarea decision's whole justification, isolated.
// The column is set-valued and semicolon-joined; grouping by the literal string is honest ONLY if
// `Africa;Eurasia` is also a working filter value. If it ever stops being one, the right fix is to
// unnest and take a NonPartitioning exemption — which this facet cannot legally have, since its table
// IS the listed table — so this failing means the facet has to be redesigned, not patched.
func TestLanguoidCompositeMacroareaRoundTrips_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)

	res, err := svc.LanguoidStats(ctx, domain.Filter{}, allLanguoidFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	var composites int
	for _, b := range languoidBuckets(t, res, "macroarea") {
		if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
			continue
		}
		if !containsSemicolon(b.Key) {
			continue
		}
		composites++
		if got, want := int(b.Count), pageAllLanguoids(t, svc, domain.Filter{Macroarea: sptr(b.Key)}); got != want {
			t.Errorf("composite macroarea %q counted %d, but its own filter returns %d rows", b.Key, got, want)
		}
	}
	if composites == 0 {
		t.Skip("no composite macroarea inside the top-N — the catalog's multi-macroarea languoids are all in (other)")
	}
}

// TestLanguoidFamilyKeysAreTrimmed: family_code is char(8), which pads on read. A padded bucket key
// would look right in a chart and might not round-trip as a filter value.
//
// This is a REGRESSION guard, not evidence of a bug that was found: measured against the shipped
// catalog every glottocode is exactly 8 characters, and the branch's `::text` cast strips trailing
// blanks on its own — so the explicit rtrim is belt-and-braces. What this catches is a future change
// that drops the cast, or a catalog that starts carrying short codes.
func TestLanguoidFamilyKeysAreTrimmed_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)

	res, err := svc.LanguoidStats(ctx, domain.Filter{}, allLanguoidFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	var checked int
	for _, b := range languoidBuckets(t, res, "family") {
		if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther {
			continue
		}
		if b.Key != trimRight(b.Key) {
			t.Errorf("family bucket %q carries trailing padding — it will not round-trip as a filter value", b.Key)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no family buckets at all — the catalog is empty or the branch is dead, so this proves nothing")
	}
}

// TestLanguoidStatsDistributionsSumToTotal: languoid declares no NonPartitioning facet — including
// macroarea, which is the point of grouping it by the literal string — so every distribution must
// account for every counted row exactly once.
func TestLanguoidStatsDistributionsSumToTotal_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)

	res, err := svc.LanguoidStats(ctx, domain.Filter{}, allLanguoidFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(res.Distributions) != 4 {
		t.Fatalf("got %d distributions, want 4 (level, macroarea, status, family)", len(res.Distributions))
	}
	for _, d := range res.Distributions {
		var n int64
		for _, b := range d.Buckets {
			n += b.Count
		}
		if n != res.TotalCount {
			t.Errorf("%s sums to %d, totalCount is %d — a partitioning facet must account for every row once",
				d.Facet, n, res.TotalCount)
		}
	}
}

// TestLanguoidStatsSearchArmMatchesTheSearchList is what the R-21 twin buys, and what would break
// silently if the two filter blocks drifted: with `query` set, the aggregate must describe exactly
// the set the searched LIST pages. The parity guard proves the SQL text matches; this proves the
// behaviour does.
func TestLanguoidStatsSearchArmMatchesTheSearchList_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)
	sel := allLanguoidFacets(t)

	for _, q := range []string{"stan", "ukr", "zzz-no-such-languoid"} {
		f := domain.Filter{Query: q}
		res, err := svc.LanguoidStats(ctx, f, sel)
		if err != nil {
			t.Fatalf("query %q: stats: %v", q, err)
		}
		if got, want := int(res.TotalCount), pageAllLanguoids(t, svc, f); got != want {
			t.Errorf("query %q: totalCount = %d, exhaustive paging of the SEARCH list returned %d rows", q, got, want)
		}
	}
	// And the search arm genuinely narrows, so the assertion above is not passing on a no-op.
	all, err := svc.LanguoidStats(ctx, domain.Filter{}, sel)
	if err != nil {
		t.Fatalf("unfiltered stats: %v", err)
	}
	narrowed, err := svc.LanguoidStats(ctx, domain.Filter{Query: "stan"}, sel)
	if err != nil {
		t.Fatalf("searched stats: %v", err)
	}
	if narrowed.TotalCount == 0 || narrowed.TotalCount >= all.TotalCount {
		t.Errorf("the search arm counted %d of %d — it must narrow to a non-empty subset, or this test is vacuous",
			narrowed.TotalCount, all.TotalCount)
	}
}

// TestLanguoidStatusBucketsAreInSeverityOrder: `status` is an identity-bucket enum whose declared
// Values are the DDL's CHECK-set order, which is severity order. An endangerment profile re-sorted by
// frequency destroys the only ordering that means anything, so the kernel must be emitting the
// catalog's order rather than SQL's.
func TestLanguoidStatusBucketsAreInSeverityOrder_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)

	res, err := svc.LanguoidStats(ctx, domain.Filter{}, allLanguoidFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	want := []string{"not_endangered", "threatened", "shifting", "moribund", "nearly_extinct", "extinct"}
	got := languoidBuckets(t, res, "status")
	if len(got) != len(want) {
		t.Fatalf("got %d status buckets, want %d — an identity facet zero-fills its whole CHECK set", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Key != w {
			t.Errorf("status bucket %d = %q, want %q — the severity order comes from the catalog, not from counts", i, got[i].Key, w)
		}
	}
}

// TestLanguoidFilterRejectsMalformedEnums: only the two CLOSED value sets are validated. `macroarea`
// and `family` are open code sets (no CHECK, and a glottocode is whatever the import says), so an
// unknown value there must be an empty result rather than a 400 — asserting the difference keeps a
// later "tighten the validation" change from breaking legitimate filters.
func TestLanguoidFilterRejectsMalformedEnums_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newLanguageService(t)
	sel := allLanguoidFacets(t)

	for name, f := range map[string]domain.Filter{
		"level":  {Level: sptr("phylum")},
		"status": {Status: sptr("doing_fine")},
	} {
		if _, err := svc.LanguoidStats(ctx, f, sel); err == nil {
			t.Errorf("%s: a value outside the CHECK set was accepted", name)
		}
	}
	for name, f := range map[string]domain.Filter{
		"macroarea": {Macroarea: sptr("Atlantis")},
		"family":    {Family: sptr("nosu1234")},
	} {
		res, err := svc.LanguoidStats(ctx, f, sel)
		if err != nil {
			t.Errorf("%s: an unknown value in an OPEN code set must be an empty result, not an error: %v", name, err)
			continue
		}
		if res.TotalCount != 0 {
			t.Errorf("%s: expected an empty result, got totalCount %d", name, res.TotalCount)
		}
	}
}

func containsSemicolon(s string) bool {
	for i := range s {
		if s[i] == ';' {
			return true
		}
	}
	return false
}

func trimRight(s string) string {
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
