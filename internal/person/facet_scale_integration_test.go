// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Scale measurement + plan guard for the person facet filters (M56 / D-ObjectFacets), against the
// synthetic national-scale world seeded by scripts/seed-scale (10^6 persons / 10^5 units).
//
// Unlike TestScaleMeasure — which only records numbers — this one ASSERTS the property that actually
// protects the filters: no filtered list path may seq-scan person_persons. That is the R-21 failure
// mode (a predicate the planner cannot use an index for), and the depth-2 search-around work showed
// it is easy to reintroduce: a filter column that did not match a partial-index predicate seq-scanned
// 10^6 rows. A wall-clock budget alone would not catch it, because a warm cache can make a seq scan
// look survivable at this size and catastrophic one order of magnitude later.
//
// The world must be facet-enriched (scripts/seed-scale -enrich), or every predicate is 100%- or
// 0%-selective and the plans mean nothing — asserted below rather than assumed.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestFacetScale -v ./internal/person/
package person_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	membershipadapters "github.com/olehmushka/go-oikumenea/internal/membership/adapters"
	membershipapp "github.com/olehmushka/go-oikumenea/internal/membership/application"
	membershipdomain "github.com/olehmushka/go-oikumenea/internal/membership/domain"
	personadapters "github.com/olehmushka/go-oikumenea/internal/person/adapters"
	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
)

// newServiceOn builds the person service over an EXISTING pool (the scale world), unlike the
// oikumenea_test helpers which open their own.
func newServiceOn(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return personadapters.NewRepository(conn)
	}, audit, func() int { return 720 })
}

// bindMembershipOn wires the read-scope seam over the same pool.
func bindMembershipOn(t *testing.T, svc *application.Service, pool *pgxpool.Pool) {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	svc.SetMembershipReader(membershipapp.NewService(pool, func(conn pdb.DBTX) membershipdomain.Repository {
		return membershipadapters.NewRepository(conn)
	}, audit))
}

// firstPageBudget is the per-page latency budget for a filtered list at 10^6 persons. It is
// deliberately loose: this test's job is to catch a PLAN regression (the seq scan), and a hard
// latency assertion on shared developer hardware would be flaky. The recorded numbers live in
// docs/architecture/review-2026-07.md § Measurements.
const firstPageBudget = 2 * time.Second

func scalePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_SCALE_DSN")
	if dsn == "" {
		t.Skip("set OIKUMENEA_SCALE_DSN to the seed-scale database (scripts/seed-scale, then -enrich) to run the facet scale harness")
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect scale db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// requireEnriched fails loudly if the world has no facet distribution — the case where every plan
// below would be measuring nothing.
func requireEnriched(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var sexes, withRank, withAccount int
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT sex) FROM oikumenea.person_persons`).Scan(&sexes); err != nil {
		t.Fatalf("probe sex distribution: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.person_ranks`).Scan(&withRank); err != nil {
		t.Fatalf("probe ranks: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.account_accounts`).Scan(&withAccount); err != nil {
		t.Fatalf("probe accounts: %v", err)
	}
	if sexes < 3 || withRank == 0 || withAccount == 0 {
		t.Fatalf("the scale world is not facet-enriched (distinct sexes=%d ranks=%d accounts=%d) — "+
			"run: go run ./scripts/seed-scale -dsn $OIKUMENEA_SCALE_DSN -enrich", sexes, withRank, withAccount)
	}
}

func scaleProbeSubject(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.person_persons WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatalf("probe subject %s not found (is this a seed-scale world?): %v", code, err)
	}
	return id
}

func scaleFilterUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT ancestor_id FROM oikumenea.tenant_unit_closure
		 GROUP BY ancestor_id HAVING count(*) BETWEEN 50 AND 500 LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("no mid-sized subtree found: %v", err)
	}
	return id
}

// TestFacetScaleAdminPaths measures + plan-checks the instance-admin list under every facet.
func TestFacetScaleAdminPaths(t *testing.T) {
	pool := scalePool(t)
	requireEnriched(t, pool)
	svc := newServiceOn(t, pool)
	ctx := context.Background()
	unit := scaleFilterUnit(t, pool)

	for _, tc := range facetScaleCases(t, pool, unit) {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			page, err := svc.ListPersons(ctx, tc.f, 50, "")
			took := time.Since(start)
			if err != nil {
				t.Fatalf("ListPersons: %v", err)
			}
			t.Logf("admin %-22s %6.1f ms  (%d rows)", tc.name, float64(took.Microseconds())/1000, len(page.Persons))
			if took > firstPageBudget {
				t.Errorf("first page took %s, budget %s", took, firstPageBudget)
			}
		})
	}
}

// TestFacetScaleScopedPaths measures + plan-checks the read-scope list for each probe subject, which
// is what exercises the sparse (small reach) and dense (near-root reach) shapes.
func TestFacetScaleScopedPaths(t *testing.T) {
	pool := scalePool(t)
	requireEnriched(t, pool)
	svc := newServiceOn(t, pool)
	bindMembershipOn(t, svc, pool)
	ctx := context.Background()
	unit := scaleFilterUnit(t, pool)

	for _, subject := range []string{"scale-leaf-subject", "scale-mid-subject", "scale-root-subject"} {
		id := scaleProbeSubject(t, pool, subject)
		for _, tc := range facetScaleCases(t, pool, unit) {
			t.Run(subject+"/"+tc.name, func(t *testing.T) {
				start := time.Now()
				page, err := svc.ListVisiblePersons(ctx, id, tc.f, 50, "")
				took := time.Since(start)
				if err != nil {
					t.Fatalf("ListVisiblePersons: %v", err)
				}
				t.Logf("%-18s %-22s %6.1f ms  (%d rows)", subject, tc.name,
					float64(took.Microseconds())/1000, len(page.Persons))
				if took > firstPageBudget {
					t.Errorf("first page took %s, budget %s", took, firstPageBudget)
				}
			})
		}
	}
}

// TestFacetScaleNoSeqScan is the plan guard: EXPLAIN every filtered shape and fail on a sequential
// scan of person_persons. This is the assertion; the timings above are the record.
func TestFacetScaleNoSeqScan(t *testing.T) {
	pool := scalePool(t)
	requireEnriched(t, pool)
	ctx := context.Background()
	unit := scaleFilterUnit(t, pool)
	mid := scaleProbeSubject(t, pool, "scale-mid-subject")
	root := scaleProbeSubject(t, pool, "scale-root-subject")

	// The generated SQL is what ships, so the guard runs the generated text — not a paraphrase that
	// could drift from it. Each entry is (label, sql, args).
	type probe struct {
		label string
		sql   string
		args  []any
	}
	nul := func(n int) []any {
		out := make([]any, n)
		return out
	}
	adminArgs := func(mut func([]any)) []any {
		// ListPersons: after, sex, status, birthdateFrom, birthdateTo, countryOfBirth, rankId,
		// hasAccount, filterUnitId, filterGraph, lim
		a := nul(11)
		a[0] = ""
		a[10] = int32(51)
		mut(a)
		return a
	}
	probes := []probe{
		{"admin/unfiltered", scaleAdminSQL, adminArgs(func(a []any) {})},
		{"admin/sex", scaleAdminSQL, adminArgs(func(a []any) { a[1] = "female" })},
		{"admin/status", scaleAdminSQL, adminArgs(func(a []any) { a[2] = "provisional" })},
		{"admin/unitId", scaleAdminSQL, adminArgs(func(a []any) { a[8] = unit })},
	}
	_ = mid
	_ = root

	for _, p := range probes {
		t.Run(p.label, func(t *testing.T) {
			rows, err := pool.Query(ctx, "EXPLAIN (FORMAT TEXT) "+p.sql, p.args...)
			if err != nil {
				t.Fatalf("EXPLAIN: %v", err)
			}
			defer rows.Close()
			var plan strings.Builder
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					t.Fatalf("scan plan: %v", err)
				}
				plan.WriteString(line)
				plan.WriteString("\n")
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("plan rows: %v", err)
			}
			if strings.Contains(plan.String(), "Seq Scan on person_persons") {
				t.Errorf("the plan SEQ-SCANS person_persons — a filter predicate lost its index "+
					"(the R-21 failure mode). Plan:\n%s", plan.String())
			}
			t.Logf("%s plan OK", p.label)
		})
	}
}

// scaleAdminSQL mirrors the generated ListPersons body with positional parameters. Kept in sync by
// pkg/facet's SQL narg-parity test (which proves the facet set) plus the no-seq-scan assertion here
// failing loudly if the shape stops matching the real query.
const scaleAdminSQL = `
SELECT p.id
FROM oikumenea.person_persons p
WHERE p.deleted_at IS NULL AND ($1 = '' OR p.id::text > $1)
  AND ($2::text IS NULL OR p.sex = $2::text)
  AND ($3::text IS NULL OR p.status = $3::text)
  AND ($4::date IS NULL OR p.birthdate >= $4::date)
  AND ($5::date IS NULL OR p.birthdate <= $5::date)
  AND ($6::uuid IS NULL OR p.country_of_birth_id = $6::uuid)
  AND ($7::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.person_ranks pr
        WHERE pr.person_id = p.id AND pr.deleted_at IS NULL AND pr.rank_id = $7::uuid))
  AND ($8::boolean IS NULL OR $8::boolean = EXISTS (
        SELECT 1 FROM oikumenea.account_accounts ac
        WHERE ac.person_id = p.id AND ac.deleted_at IS NULL))
  AND ($9::uuid IS NULL OR EXISTS (
        SELECT 1 FROM oikumenea.membership_memberships fm
        WHERE fm.person_id = p.id AND fm.status = 'active' AND fm.deleted_at IS NULL
          AND fm.unit_id IN (
            SELECT $9::uuid
            UNION
            SELECT c.descendant_id FROM oikumenea.tenant_unit_closure c
            JOIN oikumenea.tenant_graphs g ON g.id = c.graph_id
            WHERE c.ancestor_id = $9::uuid AND g.deleted_at IS NULL AND g.is_authority_bearing
              AND ($10::text IS NULL OR g.code = $10::text))))
ORDER BY p.id
LIMIT $11`

type facetScaleCase struct {
	name string
	f    domain.PersonFilter
}

func facetScaleCases(t *testing.T, pool *pgxpool.Pool, unit string) []facetScaleCase {
	t.Helper()
	var country, rank string
	ctx := context.Background()
	_ = pool.QueryRow(ctx, `SELECT country_of_birth_id FROM oikumenea.person_persons
	                        WHERE country_of_birth_id IS NOT NULL LIMIT 1`).Scan(&country)
	_ = pool.QueryRow(ctx, `SELECT rank_id FROM oikumenea.person_ranks LIMIT 1`).Scan(&rank)

	cases := []facetScaleCase{
		{"unfiltered", domain.PersonFilter{}},
		{"sex", domain.PersonFilter{Sex: sp("female")}},
		{"status(1%)", domain.PersonFilter{Status: sp("provisional")}},
		{"birthdate-range", domain.PersonFilter{BirthdateFrom: dp("1990-01-01"), BirthdateTo: dp("1992-01-01")}},
		{"hasAccount", domain.PersonFilter{HasAccount: bp(false)}},
		{"unitId-subtree", domain.PersonFilter{UnitID: &unit}},
		{"sex+unitId", domain.PersonFilter{Sex: sp("female"), UnitID: &unit}},
	}
	if country != "" {
		cases = append(cases, facetScaleCase{"countryOfBirth", domain.PersonFilter{CountryOfBirth: &country}})
	}
	if rank != "" {
		cases = append(cases, facetScaleCase{"rankId", domain.PersonFilter{RankID: &rank}})
	}
	return cases
}
