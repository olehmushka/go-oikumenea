// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the enrollment BROWSE + DASHBOARD (M58 ticket 7 / D-ObjectFacets) against a
// real Postgres.
//
// The differential is the vocabulary's usual one — totalCount equals the rows an exhaustive paging of
// the same list returns under the same filters, and every bucket's count equals what its own filter
// returns. What this type carries alone is the CATALOG-ordered facet (facet.StrategyCatalog), whose
// two defining properties are exactly the two a differential cannot see:
//
//   - the buckets come back in the catalog's ordinal order rather than by count, and
//   - a level with no enrollments still gets a bucket.
//
// Both are asserted directly below, because a chart that is correct row-by-row and wrong in its ORDER
// passes every count-based check ever written for this vocabulary.
//
// The fixture seeds its OWN spread rather than relying on the seeder's. seed-demo left every
// enrollment with a NULL degree level until this ticket, and the test fixture had the same gap — the
// shape ticket 4 hit with languoid macroareas, where the guard was fine and the world was empty.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/education/... -run EnrollmentStats
package education_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/education/application"
	"github.com/olehmushka/go-oikumenea/internal/education/domain"
	"github.com/olehmushka/go-oikumenea/pkg/facet"
	"github.com/olehmushka/go-oikumenea/pkg/stats"
)

func allEnrollmentFacets(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("link__studied_at")
	if !ok {
		t.Fatal("link__studied_at is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

// pageAllEnrollments exhaustively pages the register under one filter and returns the row count.
// Small pages on purpose: a bug that loses rows at a page boundary (listTaxa had silently dropped 16
// of 100 rows since M22) only shows up when the sweep turns over. It also asserts no id repeats,
// which is the other half of a broken keyset.
func pageAllEnrollments(t *testing.T, svc *application.Service, f domain.EnrollmentFilter) int {
	t.Helper()
	ctx := context.Background()
	seen := map[string]bool{}
	token := ""
	for i := 0; i < 500; i++ {
		rows, err := svc.ListEnrollmentRegister(ctx, "", f, token, 3)
		if err != nil {
			t.Fatalf("list enrollments: %v", err)
		}
		page := rows
		next := ""
		if len(rows) > 3 {
			page = rows[:3]
			next = page[len(page)-1].ID
		}
		for _, e := range page {
			if seen[e.ID] {
				t.Fatalf("enrollment %s returned twice — the keyset is broken", e.ID)
			}
			seen[e.ID] = true
		}
		if next == "" {
			return len(seen)
		}
		token = next
	}
	t.Fatal("paging did not terminate in 500 pages")
	return 0
}

// seedEnrollmentSpread creates a population sharing one INSTITUTION, so every assertion below can be
// scoped to this test's rows: the test database is persistent, and an unfiltered assertion would race
// every other test that enrolls somebody.
//
// The degree levels are deliberately PARTIAL — three of the nine ISCED levels are used and six are
// left empty — because the empty ones are what the catalog strategy exists to keep on the chart. A
// fully-populated scale would let a topN regression pass every assertion here.
func seedEnrollmentSpread(t *testing.T, pool *pgxpool.Pool, svc *application.Service) (institution string) {
	t.Helper()
	ctx := context.Background()
	inst, err := svc.CreateInstitution(ctx, domain.InstitutionInput{
		Code: uniq("enr-spread"), Name: "Enrollment Spread University",
		KindID: catalogID(t, pool, "education_institution_kinds", "university"),
	})
	if err != nil {
		t.Fatalf("seed institution: %v", err)
	}
	rows := []struct {
		levelCode string // "" = no recorded level, the (unknown) bucket
		status    string
		from      string // "" = no recorded intake
	}{
		{"isced-6", "enrolled", "2021-09-01"},
		{"isced-6", "enrolled", "2021-09-01"},
		{"isced-6", "graduated", "2019-09-01"},
		{"isced-7", "enrolled", "2022-02-01"},
		{"isced-7", "on_leave", "2022-02-01"},
		{"isced-8", "withdrawn", "2020-09-01"},
		{"", "expelled", ""},
	}
	for _, r := range rows {
		in := domain.EnrollmentInput{InstitutionID: inst.ID, Status: ptr(r.status)}
		if r.levelCode != "" {
			in.DegreeLevelID = ptr(catalogID(t, pool, "education_degree_levels", r.levelCode))
		}
		if r.from != "" {
			in.EffectiveFrom = ptr(r.from)
		}
		if _, err := svc.CreateEnrollment(ctx, seedPerson(t, pool), in); err != nil {
			t.Fatalf("seed enrollment: %v", err)
		}
	}
	return inst.ID
}

// TestEnrollmentStatsTotalEqualsExhaustivePaging is D-ObjectFacets' headline promise on the ADMIN
// arm: the number the dashboard prints is the number of rows the list would hand over.
func TestEnrollmentStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	inst := seedEnrollmentSpread(t, pool, svc)

	f := domain.EnrollmentFilter{InstitutionID: &inst}
	res, err := svc.EnrollmentStats(ctx, "", true, f, allEnrollmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if got, want := int(res.TotalCount), pageAllEnrollments(t, svc, f); got != want {
		t.Errorf("totalCount = %d, exhaustive paging returned %d rows", got, want)
	}
}

// TestEnrollmentStatsEveryBucketEqualsItsOwnFilter is the property the whole vocabulary rests on: a
// chart segment and a list filter must be the same act.
func TestEnrollmentStatsEveryBucketEqualsItsOwnFilter_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	inst := seedEnrollmentSpread(t, pool, svc)

	base := domain.EnrollmentFilter{InstitutionID: &inst}
	res, err := svc.EnrollmentStats(ctx, "", true, base, allEnrollmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	checked, months := 0, 0
	for _, d := range res.Distributions {
		for _, b := range d.Buckets {
			if b.Key == stats.BucketUnknown || b.Key == stats.BucketOther || b.Count == 0 {
				continue
			}
			f := base
			k := b.Key
			switch d.Facet {
			case "institutionId":
				f.InstitutionID = &k
			case "programId":
				f.ProgramID = &k
			case "unitId":
				f.UnitID = &k
			case "groupId":
				f.GroupID = &k
			case "degreeLevelId":
				f.DegreeLevelID = &k
			case "status":
				f.Status = &k
			case "effectiveFrom":
				from, to := b.Key+"-01", monthEnd(t, b.Key)
				f.EffectiveFromFrom, f.EffectiveFromTo = &from, &to
				months++
			default:
				t.Fatalf("unhandled facet %q — a new facet must be given its filter inverse here", d.Facet)
			}
			if got, want := pageAllEnrollments(t, svc, f), int(b.Count); got != want {
				t.Errorf("%s bucket %q counted %d but its own filter returns %d rows", d.Facet, b.Key, want, got)
			}
			checked++
		}
	}
	if months < 3 {
		t.Errorf("only %d month buckets checked — the fixture must span several months or the month-grain "+
			"inverse is not being exercised", months)
	}
	if checked < 8 {
		t.Fatalf("only %d buckets checked — the fixture is too thin to be a differential", checked)
	}
}

// TestEnrollmentStatsDistributionsPartition: every facet's buckets sum to totalCount. Enrollment
// takes no NonPartitioning exemption — every facet is a single column of the listed table, so one row
// lands in exactly one bucket of each.
func TestEnrollmentStatsDistributionsPartition_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	inst := seedEnrollmentSpread(t, pool, svc)

	res, err := svc.EnrollmentStats(ctx, "", true,
		domain.EnrollmentFilter{InstitutionID: &inst}, allEnrollmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if len(res.Distributions) != 7 {
		t.Fatalf("got %d distributions, want 7 (one per declared facet)", len(res.Distributions))
	}
	for _, d := range res.Distributions {
		var sum int64
		for _, b := range d.Buckets {
			sum += b.Count
		}
		if sum != res.TotalCount {
			t.Errorf("%s sums to %d but totalCount is %d — the buckets do not partition the result set",
				d.Facet, sum, res.TotalCount)
		}
	}
}

// TestEnrollmentDegreeLevelIsAScaleNotARanking is the one this ticket exists for, and the one a
// count-based differential CANNOT see: the buckets must arrive in ISCED order with the empty levels
// present, not in descending count with the empty ones dropped.
//
// The fixture makes the two orders disagree on purpose — isced-6 is the most frequent and isced-8 the
// least, so a chart sorted by count would read 6, 7, 8 by accident. It is the interleaving of the
// EMPTY levels (0..5 between and below them) that only the catalog order can produce.
func TestEnrollmentDegreeLevelIsAScaleNotARanking_Integration(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	inst := seedEnrollmentSpread(t, pool, svc)

	res, err := svc.EnrollmentStats(ctx, "", true,
		domain.EnrollmentFilter{InstitutionID: &inst}, allEnrollmentFacets(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	var buckets []stats.Bucket
	for _, d := range res.Distributions {
		if d.Facet == "degreeLevelId" {
			buckets = d.Buckets
		}
	}
	if len(buckets) == 0 {
		t.Fatal("no degreeLevelId distribution")
	}
	// The nine ISCED levels in scale order, plus the (unknown) bucket last.
	var want []string
	rows, err := pool.Query(ctx, `SELECT id::text FROM oikumenea.education_degree_levels
		WHERE deleted_at IS NULL ORDER BY isced_level`)
	if err != nil {
		t.Fatalf("read the ISCED catalog: %v", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		want = append(want, id)
	}
	rows.Close()
	want = append(want, stats.BucketUnknown)
	if len(want) < 9 {
		t.Fatalf("the ISCED catalog has only %d live levels — the fixture cannot show an empty one", len(want)-1)
	}

	got := make([]string, 0, len(buckets))
	empties := 0
	for _, b := range buckets {
		got = append(got, b.Key)
		if b.Count == 0 && b.Key != stats.BucketUnknown {
			empties++
		}
		if b.Key == stats.BucketOther {
			t.Error("the degree-level distribution carries an (other) tail — a closed catalog has no tail to collapse")
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d buckets, want %d (every ISCED level plus (unknown))\n got: %v\nwant: %v",
			len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bucket %d is %s, want %s — the scale is not in ISCED order (a distribution sorted by "+
				"count reads as if a doctorate ranked below a bachelor's)\n got: %v\nwant: %v",
				i, got[i], want[i], got, want)
		}
	}
	if empties < 5 {
		t.Errorf("only %d empty levels present — the fixture populates three of nine, so the rest must "+
			"still be on the chart; dropping them is the regression this test exists for", empties)
	}
}

// monthEnd returns the last day of a YYYY-MM bucket key, which is the upper bound its filter inverse
// takes. Computed rather than assumed, because February is where an off-by-one hides.
func monthEnd(t *testing.T, monthKey string) string {
	t.Helper()
	var out string
	if err := newPool(t).QueryRow(context.Background(),
		`SELECT to_char((date_trunc('month', ($1 || '-01')::date) + INTERVAL '1 month - 1 day')::date, 'YYYY-MM-DD')`,
		monthKey).Scan(&out); err != nil {
		t.Fatalf("month end for %s: %v", monthKey, err)
	}
	return out
}
