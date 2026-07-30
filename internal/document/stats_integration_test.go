// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the document-register DASHBOARD aggregate (M57 ticket 2 / D-ObjectFacets).
//
// The exit criterion is the same differential on both arms. What is specific to `document` is where the
// visibility predicate lives: a document carries no unit, so reach goes THROUGH THE HOLDER's active
// memberships. A document held by someone the caller cannot read must not be counted — and because the
// join is on the holder rather than on the row, getting it wrong would not look like a leak in the
// list's shape, only in a number. Hence the explicit test below.
//
// The `expiresOn` `(unknown)` bucket is the NO-EXPIRY (permanent document) population: a real set, not
// missing data, which is why it must be present even when empty.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/document/... -run Stats
package document_test

import (
	"context"
	"testing"

	"github.com/olegamysk/go-oikumenea/internal/document/domain"
	"github.com/olegamysk/go-oikumenea/pkg/facet"
	"github.com/olegamysk/go-oikumenea/pkg/stats"
)

func docSel(t *testing.T) stats.Selection {
	t.Helper()
	o, ok := facet.Default.Get("document")
	if !ok {
		t.Fatal("document is not registered in the facet catalog")
	}
	sel, err := stats.Select(o, "", nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return sel
}

func dBuckets(t *testing.T, res stats.Result, key string) []stats.Bucket {
	t.Helper()
	for _, d := range res.Distributions {
		if d.Facet == key {
			return d.Buckets
		}
	}
	t.Fatalf("no %q distribution in the response", key)
	return nil
}

func dCount(bs []stats.Bucket, key string) int64 {
	for _, b := range bs {
		if b.Key == key {
			return b.Count
		}
	}
	return -1
}

func dSum(bs []stats.Bucket) int64 {
	var n int64
	for _, b := range bs {
		n += b.Count
	}
	return n
}

func TestDocumentStatsTotalEqualsExhaustivePaging_Integration(t *testing.T) {
	w := seedDFacetWorld(t)
	ctx := context.Background()
	sel := docSel(t)

	filters := []domain.DocumentFilter{
		{},
		{Status: dsp("active")},
		{Status: dsp("superseded")},
		{TypeID: dsp(w.typeA)},
		{TypeID: dsp(w.typeB)},
		{IssuingCountryID: dsp(w.country)},
		{IssuedOnFrom: ddp("2023-01-01")},
		{IssuedOnTo: ddp("2021-01-01")},
		{ExpiresOnFrom: ddp("2029-01-01")},
		{ExpiresOnTo: ddp("2031-01-01")},
		{TypeID: dsp(w.typeA), Status: dsp("active")},
	}
	for i, f := range filters {
		scoped, err := w.svc.DocumentStats(ctx, w.reader, false, f, sel)
		if err != nil {
			t.Fatalf("filter %d: scoped stats: %v", i, err)
		}
		if got, want := scoped.TotalCount, int64(len(w.scoped(t, f))); got != want {
			t.Errorf("filter %d (%+v): scoped totalCount = %d, but exhaustively paging the scoped list "+
				"returned %d rows", i, f, got, want)
		}
		// Bounded to one of this world's document types: the test database is shared, so an unbounded
		// admin comparison would race another suite's inserts rather than measure drift between the
		// aggregate and the list. The differential holds over any predicate.
		af := f
		if af.TypeID == nil {
			af.TypeID = dsp(w.typeA)
		}
		admin, err := w.svc.DocumentStats(ctx, "", true, af, sel)
		if err != nil {
			t.Fatalf("filter %d: admin stats: %v", i, err)
		}
		if got, want := admin.TotalCount, int64(len(w.admin(t, af))); got != want {
			t.Errorf("filter %d (%+v): admin totalCount = %d, but exhaustively paging the admin list "+
				"returned %d rows", i, f, got, want)
		}
	}
}

// TestDocumentStatsBucketsSumToTotal: every document facet is a column of the row itself, so all five
// partition the candidate set — including the two nullable ones, whose (unknown) buckets are the
// no-country and no-expiry populations.
func TestDocumentStatsBucketsSumToTotal_Integration(t *testing.T) {
	w := seedDFacetWorld(t)
	ctx := context.Background()
	sel := docSel(t)

	res, err := w.svc.DocumentStats(ctx, w.reader, false, domain.DocumentFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	for _, key := range []string{"typeId", "status", "issuingCountryId", "issuedOn", "expiresOn"} {
		if got := dSum(dBuckets(t, res, key)); got != res.TotalCount {
			t.Errorf("facet %q sums to %d, want totalCount %d", key, got, res.TotalCount)
		}
	}
}

func TestDocumentStatsDistributionsMatchTheSeededWorld_Integration(t *testing.T) {
	w := seedDFacetWorld(t)
	ctx := context.Background()
	sel := docSel(t)

	// The reader reaches unitIn, so holderIn's two documents: dActive (typeA, active, issued 2024-05,
	// expires 2030-01, country UA) and dSuperseded (typeB, superseded, issued 2020-02, NO expiry, no
	// country).
	res, err := w.svc.DocumentStats(ctx, w.reader, false, domain.DocumentFilter{}, sel)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 2 {
		t.Fatalf("totalCount = %d, want the two documents of the in-reach holder", res.TotalCount)
	}
	status := dBuckets(t, res, "status")
	if got := dCount(status, "active"); got != 1 {
		t.Errorf("status[active] = %d, want 1", got)
	}
	if got := dCount(status, "revoked"); got != 0 {
		t.Errorf("status[revoked] = %d, want a PRESENT zero bucket (chart shape is stable)", got)
	}
	types := dBuckets(t, res, "typeId")
	if dCount(types, w.typeA) != 1 || dCount(types, w.typeB) != 1 {
		t.Errorf("typeId = %+v, want one of each seeded type", types)
	}
	// The no-country document is the (unknown) bucket, not a dropped row.
	country := dBuckets(t, res, "issuingCountryId")
	if got := dCount(country, w.country); got != 1 {
		t.Errorf("issuingCountryId[%s] = %d, want 1", w.country, got)
	}
	if got := dCount(country, stats.BucketUnknown); got != 1 {
		t.Errorf("issuingCountryId[(unknown)] = %d, want the one document with no issuing country", got)
	}
	// expiresOn: one expiry in 2030-01, and the permanent document in (unknown).
	expires := dBuckets(t, res, "expiresOn")
	if got := dCount(expires, "2030-01"); got != 1 {
		t.Errorf("expiresOn[2030-01] = %d, want 1", got)
	}
	if got := dCount(expires, stats.BucketUnknown); got != 1 {
		t.Errorf("expiresOn[(unknown)] = %d, want the permanent document — a real set, not missing data", got)
	}
}

// TestDocumentStatsCountsInsideTheHolderPredicate is the property this module's separate scoped query
// exists for: reach runs through the HOLDER, so a document whose holder the caller cannot read must be
// absent from the total and from every bucket.
func TestDocumentStatsCountsInsideTheHolderPredicate_Integration(t *testing.T) {
	w := seedDFacetWorld(t)
	ctx := context.Background()
	sel := docSel(t)

	scoped, err := w.svc.DocumentStats(ctx, w.reader, false, domain.DocumentFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats: %v", err)
	}
	if scoped.TotalCount != 2 {
		t.Errorf("scoped totalCount = %d, want 2 — the out-of-reach holder's document must not be counted",
			scoped.TotalCount)
	}

	// Add a second document to the UNREADABLE holder: the scoped total must not move, while the admin
	// total does. That asymmetry is the proof the predicate is inside the count.
	before := scoped.TotalCount
	seedFacetDocument(t, w.pool, w.holderOut, w.typeA, "active", "2024-05-06", "2030-01-01", w.country)

	after, err := w.svc.DocumentStats(ctx, w.reader, false, domain.DocumentFilter{}, sel)
	if err != nil {
		t.Fatalf("scoped stats (after): %v", err)
	}
	if after.TotalCount != before {
		t.Errorf("the scoped total moved from %d to %d when a document was added to a holder the reader "+
			"CANNOT read — the count is being taken outside the holder predicate", before, after.TotalCount)
	}
	admin, err := w.svc.DocumentStats(ctx, "", true, domain.DocumentFilter{TypeID: dsp(w.typeA)}, sel)
	if err != nil {
		t.Fatalf("admin stats: %v", err)
	}
	if admin.TotalCount < 3 {
		t.Errorf("the admin arm counts %d typeA documents, want at least the three seeded — otherwise the "+
			"assertion above proves nothing", admin.TotalCount)
	}
}

func TestDocumentStatsNonAdminWithNoSubjectReadsNothing_Integration(t *testing.T) {
	w := seedDFacetWorld(t)
	res, err := w.svc.DocumentStats(context.Background(), "", false, domain.DocumentFilter{}, docSel(t))
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if res.TotalCount != 0 || len(res.Distributions) != 0 {
		t.Errorf("a non-admin with no subject got totalCount %d — it must read nothing, never the admin arm",
			res.TotalCount)
	}
}
