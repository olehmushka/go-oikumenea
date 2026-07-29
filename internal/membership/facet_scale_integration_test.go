// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Scale measurement + plan guard for the M56 ticket-3 top-level list endpoints (listMemberships /
// listOrders / listDocuments), against the synthetic national-scale world seeded by
// scripts/seed-scale (10^6 persons / 10^5 units / 10^6 memberships / 2x10^5 orders / 6x10^5 docs).
//
// The measurement lives in ONE place for all three modules because the property under test is one
// property — every filtered path is index-backed — and the three endpoints share a filter shape and
// a reach fold. Splitting it three ways would triplicate the harness for no extra coverage.
//
// The ASSERTION is the plan, not the clock: no filtered path may sequential-scan its driving table.
// That is the R-21 failure mode, and the depth-2 search-around work showed how easily it returns — a
// filter column that did not match a partial-index predicate seq-scanned 10^6 rows (cost 23660 -> 48
// once it did). Ticket 3 walks straight into that trap by design: every pre-existing
// membership_memberships index is PARTIAL on status='active', and the top-level list deliberately
// ships no implicit status filter. Migration 0017's keyset indexes are what close it, and this is
// what proves they do.
//
// A wall-clock budget alone would not catch it: a warm cache makes a seq scan look survivable at
// this size and catastrophic one order of magnitude later.
//
// The world must be facet-enriched (scripts/seed-scale -enrich), or every predicate is 100%- or
// 0%-selective and the plans mean nothing — asserted below rather than assumed.
//
//	OIKUMENEA_SCALE_DSN="postgres://postgres:dev@localhost:5432/oikumenea_scale?sslmode=disable" \
//	  go test -tags integration -run TestTicket3Scale -v ./internal/membership/
package membership_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	documentadapters "github.com/olegamysk/go-oikumenea/internal/document/adapters"
	documentapp "github.com/olegamysk/go-oikumenea/internal/document/application"
	documentdomain "github.com/olegamysk/go-oikumenea/internal/document/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	orderadapters "github.com/olegamysk/go-oikumenea/internal/order/adapters"
	orderapp "github.com/olegamysk/go-oikumenea/internal/order/application"
	orderdomain "github.com/olegamysk/go-oikumenea/internal/order/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/crypto"
	"github.com/olegamysk/go-oikumenea/pkg/events"
	"github.com/olegamysk/go-oikumenea/pkg/personalcode"
)

// ticket3Budget is the per-page latency budget for a filtered first page at this scale. Deliberately
// loose: this suite's job is to catch a PLAN regression, and a hard latency assertion on shared
// developer hardware would be flaky. The recorded numbers live in
// docs/architecture/review-2026-07.md § Measurements.
const ticket3Budget = 2 * time.Second

func ticket3Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("OIKUMENEA_SCALE_DSN")
	if dsn == "" {
		t.Skip("set OIKUMENEA_SCALE_DSN to the seed-scale database (scripts/seed-scale, then -enrich) to run the ticket-3 scale harness")
	}
	pool, err := pdb.NewPool(context.Background(), dsn, "local")
	if err != nil {
		t.Fatalf("connect scale db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// requireTicket3Enriched fails loudly if the world has no ticket-3 distribution — the case where
// every plan below would be measuring nothing. Specifically it demands ENDED memberships and DRAFT
// orders: those are the populations that do not match the pre-existing partial indexes, so a world
// without them would let a missing index pass unnoticed.
func requireTicket3Enriched(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var ended, orders, drafts, docs, noExpiry int
	probes := []struct {
		sql string
		out *int
	}{
		{`SELECT count(*) FROM oikumenea.membership_memberships WHERE status = 'ended'`, &ended},
		{`SELECT count(*) FROM oikumenea.order_orders`, &orders},
		{`SELECT count(*) FROM oikumenea.order_orders WHERE status = 'draft'`, &drafts},
		{`SELECT count(*) FROM oikumenea.document_documents`, &docs},
		{`SELECT count(*) FROM oikumenea.document_documents WHERE expires_on IS NULL`, &noExpiry},
	}
	for _, p := range probes {
		if err := pool.QueryRow(ctx, p.sql).Scan(p.out); err != nil {
			t.Fatalf("probe %q: %v", p.sql, err)
		}
	}
	if ended == 0 || orders == 0 || drafts == 0 || docs == 0 || noExpiry == 0 {
		t.Fatalf("the scale world lacks the ticket-3 distribution (ended memberships=%d orders=%d "+
			"drafts=%d documents=%d no-expiry=%d) — run: "+
			"go run ./scripts/seed-scale -dsn $OIKUMENEA_SCALE_DSN -enrich",
			ended, orders, drafts, docs, noExpiry)
	}
	t.Logf("world: %d ended memberships, %d orders (%d draft), %d documents (%d no-expiry)",
		ended, orders, drafts, docs, noExpiry)
}

func ticket3Subject(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM oikumenea.person_persons WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatalf("probe subject %s not found (is this a seed-scale world?): %v", code, err)
	}
	return id
}

func ticket3Unit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT unit_id FROM oikumenea.membership_memberships
		 GROUP BY unit_id HAVING count(*) BETWEEN 5 AND 200 LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("no mid-sized unit roster found: %v", err)
	}
	return id
}

func ticket3Audit(pool *pgxpool.Pool) *auditapp.Service {
	return auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
}

func ticket3Membership(pool *pgxpool.Pool) *application.Service {
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return membershipadapters.NewRepository(conn)
	}, ticket3Audit(pool))
}

// shippingSQL returns the text of one generated sqlc query constant, read from the generated source.
//
// Extracted rather than copied: the plan guard is only meaningful if it EXPLAINs the SQL that
// actually executes. A pasted copy silently stops guarding the moment the query changes, which is
// exactly when the guard is needed most.
func shippingSQL(t *testing.T, genFile, constName string) string {
	t.Helper()
	raw, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatalf("read %s: %v", genFile, err)
	}
	marker := "\nconst " + constName + " = `"
	i := strings.Index(string(raw), marker)
	if i < 0 {
		t.Fatalf("query constant %q not found in %s — renamed or removed?", constName, genFile)
	}
	rest := string(raw)[i+len(marker):]
	j := strings.Index(rest, "`")
	if j < 0 {
		t.Fatalf("unterminated query constant %q in %s", constName, genFile)
	}
	body := rest[:j]
	if !strings.Contains(body, "SELECT") {
		t.Fatalf("extracted %q from %s but it contains no SELECT — the extractor is broken", constName, genFile)
	}
	return body
}

func ticket3Order(pool *pgxpool.Pool) *orderapp.Service {
	return orderapp.NewService(pool, func(conn pdb.DBTX) orderdomain.Repository {
		return orderadapters.NewRepository(conn)
	}, ticket3Audit(pool), events.NewBus())
}

func ticket3Document(t *testing.T, pool *pgxpool.Pool) *documentapp.Service {
	t.Helper()
	kek := make([]byte, 32)
	for i := range kek {
		kek[i] = byte(i + 7)
	}
	provider, err := crypto.NewLocalDevProvider(kek)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	cipher, err := crypto.NewCipher(provider, []byte("scale-blind-index-key"), 0)
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return documentapp.NewService(pool, func(conn pdb.DBTX) documentdomain.Repository {
		return documentadapters.NewRepository(conn)
	}, ticket3Audit(pool), cipher, personalcode.New())
}

// TestTicket3ScaleMemberships measures the top-level membership list on both paths.
func TestTicket3ScaleMemberships(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	svc := ticket3Membership(pool)
	ctx := context.Background()
	unit := ticket3Unit(t, pool)

	cases := []struct {
		name string
		f    domain.MembershipFilter
	}{
		{"unfiltered", domain.MembershipFilter{}},
		{"status=active", domain.MembershipFilter{Status: msp("active")}},
		{"status=ended", domain.MembershipFilter{Status: msp("ended")}},
		{"unitId", domain.MembershipFilter{UnitID: msp(unit)}},
		{"effectiveFrom range", domain.MembershipFilter{
			EffectiveFromAfter: mdp("2020-01-01"), EffectiveFromBefore: mdp("2020-12-31"),
		}},
		{"unitId+status", domain.MembershipFilter{UnitID: msp(unit), Status: msp("ended")}},
	}

	for _, tc := range cases {
		t.Run("admin/"+tc.name, func(t *testing.T) {
			start := time.Now()
			page, err := svc.ListMemberships(ctx, tc.f, 50, "")
			took := time.Since(start)
			if err != nil {
				t.Fatalf("ListMemberships: %v", err)
			}
			t.Logf("admin  %-22s %7.1f ms  (%d rows)", tc.name, float64(took.Microseconds())/1000, len(page.Memberships))
			if took > ticket3Budget {
				t.Errorf("first page took %s, budget %s", took, ticket3Budget)
			}
		})
	}

	for _, subject := range []string{"scale-leaf-subject", "scale-mid-subject", "scale-root-subject"} {
		id := ticket3Subject(t, pool, subject)
		for _, tc := range cases {
			t.Run(subject+"/"+tc.name, func(t *testing.T) {
				start := time.Now()
				page, err := svc.ListVisibleMemberships(ctx, id, tc.f, 50, "")
				took := time.Since(start)
				if err != nil {
					t.Fatalf("ListVisibleMemberships: %v", err)
				}
				t.Logf("%-18s %-22s %7.1f ms  (%d rows)", subject, tc.name,
					float64(took.Microseconds())/1000, len(page.Memberships))
				if took > ticket3Budget {
					t.Errorf("first page took %s, budget %s", took, ticket3Budget)
				}
			})
		}
	}
}

// TestTicket3ScaleOrders measures the top-level order list on both paths.
func TestTicket3ScaleOrders(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	svc := ticket3Order(pool)
	ctx := context.Background()

	var typeID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM oikumenea.order_order_types WHERE code LIKE 'scale-otype-%' ORDER BY sort_order LIMIT 1`).
		Scan(&typeID); err != nil {
		t.Fatalf("scale order type: %v", err)
	}

	cases := []struct {
		name string
		f    orderdomain.OrderFilter
	}{
		{"unfiltered", orderdomain.OrderFilter{}},
		{"status=draft", orderdomain.OrderFilter{Status: strPtr("draft")}},
		{"status=revoked", orderdomain.OrderFilter{Status: strPtr("revoked")}},
		{"orderTypeId", orderdomain.OrderFilter{OrderTypeID: strPtr(typeID)}},
		{"issuedOn range", orderdomain.OrderFilter{
			IssuedOnFrom: datePtr("2020-01-01"), IssuedOnTo: datePtr("2020-12-31"),
		}},
	}

	for _, tc := range cases {
		t.Run("admin/"+tc.name, func(t *testing.T) {
			start := time.Now()
			page, err := svc.ListOrders(ctx, tc.f, 50, "")
			took := time.Since(start)
			if err != nil {
				t.Fatalf("ListOrders: %v", err)
			}
			t.Logf("admin  %-22s %7.1f ms  (%d rows)", tc.name, float64(took.Microseconds())/1000, len(page.Orders))
			if took > ticket3Budget {
				t.Errorf("first page took %s, budget %s", took, ticket3Budget)
			}
		})
	}

	for _, subject := range []string{"scale-mid-subject", "scale-root-subject"} {
		id := ticket3Subject(t, pool, subject)
		for _, tc := range cases {
			t.Run(subject+"/"+tc.name, func(t *testing.T) {
				start := time.Now()
				page, err := svc.ListVisibleOrders(ctx, id, tc.f, 50, "")
				took := time.Since(start)
				if err != nil {
					t.Fatalf("ListVisibleOrders: %v", err)
				}
				t.Logf("%-18s %-22s %7.1f ms  (%d rows)", subject, tc.name,
					float64(took.Microseconds())/1000, len(page.Orders))
				if took > ticket3Budget {
					t.Errorf("first page took %s, budget %s", took, ticket3Budget)
				}
			})
		}
	}
}

// TestTicket3ScaleDocuments measures the top-level document list. The scoped arm is the interesting
// one: its holder semi-join reaches through membership_memberships into the reach set, the deepest
// visibility fold of the three.
func TestTicket3ScaleDocuments(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	svc := ticket3Document(t, pool)
	ctx := context.Background()

	var typeID string
	if err := pool.QueryRow(ctx,
		`SELECT id FROM oikumenea.document_document_types WHERE code LIKE 'scale-dtype-%' ORDER BY sort_order LIMIT 1`).
		Scan(&typeID); err != nil {
		t.Fatalf("scale document type: %v", err)
	}

	cases := []struct {
		name string
		f    documentdomain.DocumentFilter
	}{
		{"unfiltered", documentdomain.DocumentFilter{}},
		{"typeId", documentdomain.DocumentFilter{TypeID: strPtr(typeID)}},
		{"status=revoked", documentdomain.DocumentFilter{Status: strPtr("revoked")}},
		{"expiresOn range", documentdomain.DocumentFilter{
			ExpiresOnFrom: datePtr("2026-01-01"), ExpiresOnTo: datePtr("2026-12-31"),
		}},
	}

	for _, tc := range cases {
		t.Run("admin/"+tc.name, func(t *testing.T) {
			start := time.Now()
			page, err := svc.ListDocuments(ctx, tc.f, 50, "")
			took := time.Since(start)
			if err != nil {
				t.Fatalf("ListDocuments: %v", err)
			}
			t.Logf("admin  %-22s %7.1f ms  (%d rows)", tc.name, float64(took.Microseconds())/1000, len(page.Documents))
			if took > ticket3Budget {
				t.Errorf("first page took %s, budget %s", took, ticket3Budget)
			}
		})
	}

	for _, subject := range []string{"scale-mid-subject", "scale-root-subject"} {
		id := ticket3Subject(t, pool, subject)
		for _, tc := range cases {
			t.Run(subject+"/"+tc.name, func(t *testing.T) {
				start := time.Now()
				page, err := svc.ListVisibleDocuments(ctx, id, tc.f, 50, "")
				took := time.Since(start)
				if err != nil {
					t.Fatalf("ListVisibleDocuments: %v", err)
				}
				t.Logf("%-18s %-22s %7.1f ms  (%d rows)", subject, tc.name,
					float64(took.Microseconds())/1000, len(page.Documents))
				if took > ticket3Budget {
					t.Errorf("first page took %s, budget %s", took, ticket3Budget)
				}
			})
		}
	}
}

// TestTicket3ScaleNoSeqScan is the assertion the timings above only record: EXPLAIN each filtered
// shape and fail on a sequential scan of the driving table.
//
// The probes run the SHIPPING SQL text (copied from the generated queries below), not a paraphrase —
// a paraphrase would drift from what actually executes and quietly stop guarding it.
func TestTicket3ScaleNoSeqScan(t *testing.T) {
	pool := ticket3Pool(t)
	requireTicket3Enriched(t, pool)
	ctx := context.Background()
	unit := ticket3Unit(t, pool)
	mid := ticket3Subject(t, pool, "scale-mid-subject")

	type probe struct {
		label  string
		sql    string
		args   []any
		driver string // the table that must not be sequentially scanned
	}

	// ListMemberships: after, unit_id, person_id, position_id, status, eff_after, eff_before, lim
	mAdmin := func(mut func([]any)) []any {
		a := make([]any, 8)
		a[0] = ""
		a[7] = int32(51)
		mut(a)
		return a
	}
	// ListMembershipsForSubject: ..., subject_person_id, lim
	mScoped := func(mut func([]any)) []any {
		a := make([]any, 9)
		a[0] = ""
		a[7] = mid
		a[8] = int32(51)
		mut(a)
		return a
	}
	// ListOrders: after, issuing_unit_id, status, issued_from, issued_to, order_type_id, lim
	oAdmin := func(mut func([]any)) []any {
		a := make([]any, 7)
		a[0] = ""
		a[6] = int32(51)
		mut(a)
		return a
	}
	// ListDocuments: after, type, status, country, issued_from, issued_to, expires_from, expires_to, lim
	dAdmin := func(mut func([]any)) []any {
		a := make([]any, 9)
		a[0] = ""
		a[8] = int32(51)
		mut(a)
		return a
	}

	var (
		mSQL   = shippingSQL(t, "adapters/membershipsql/membership.sql.go", "listMemberships")
		msSQL  = shippingSQL(t, "adapters/membershipsql/membership.sql.go", "listMembershipsForSubject")
		mdSQL  = shippingSQL(t, "adapters/membershipsql/membership.sql.go", "listMembershipsForSubjectDense")
		oSQL   = shippingSQL(t, "../order/adapters/ordersql/order.sql.go", "listOrders")
		osSQL  = shippingSQL(t, "../order/adapters/ordersql/order.sql.go", "listOrdersForSubject")
		odSQL  = shippingSQL(t, "../order/adapters/ordersql/order.sql.go", "listOrdersForSubjectDense")
		dSQL   = shippingSQL(t, "../document/adapters/documentsql/document.sql.go", "listDocuments")
		dsSQL  = shippingSQL(t, "../document/adapters/documentsql/document.sql.go", "listDocumentsForSubject")
		ddSQL  = shippingSQL(t, "../document/adapters/documentsql/document.sql.go", "listDocumentsForSubjectDense")
		oScope = func(mut func([]any)) []any {
			a := make([]any, 8)
			a[0] = ""
			a[6] = mid
			a[7] = int32(51)
			mut(a)
			return a
		}
		dScope = func(mut func([]any)) []any {
			a := make([]any, 10)
			a[0] = ""
			a[8] = mid
			a[9] = int32(51)
			mut(a)
			return a
		}
	)

	// BOTH plan shapes are probed for every scoped path. The dense shape is the one that matters
	// most here: it drops the reach set for a per-row point probe, which is exactly the change that
	// could turn a keyset index scan into a full scan if the driving table lost its index.
	probes := []probe{
		{"memberships/unfiltered", mSQL, mAdmin(func(a []any) {}), "membership_memberships"},
		{"memberships/status=ended", mSQL, mAdmin(func(a []any) { a[4] = "ended" }), "membership_memberships"},
		{"memberships/unitId", mSQL, mAdmin(func(a []any) { a[1] = unit }), "membership_memberships"},
		{"memberships/scoped-sparse", msSQL, mScoped(func(a []any) {}), "membership_memberships"},
		{"memberships/scoped-sparse+ended", msSQL, mScoped(func(a []any) { a[4] = "ended" }), "membership_memberships"},
		{"memberships/scoped-dense", mdSQL, mScoped(func(a []any) {}), "membership_memberships"},
		{"memberships/scoped-dense+ended", mdSQL, mScoped(func(a []any) { a[4] = "ended" }), "membership_memberships"},
		{"orders/unfiltered", oSQL, oAdmin(func(a []any) {}), "order_orders"},
		{"orders/status=draft", oSQL, oAdmin(func(a []any) { a[2] = "draft" }), "order_orders"},
		{"orders/scoped-sparse", osSQL, oScope(func(a []any) {}), "order_orders"},
		{"orders/scoped-dense", odSQL, oScope(func(a []any) {}), "order_orders"},
		{"documents/unfiltered", dSQL, dAdmin(func(a []any) {}), "document_documents"},
		{"documents/status=revoked", dSQL, dAdmin(func(a []any) { a[2] = "revoked" }), "document_documents"},
		{"documents/scoped-sparse", dsSQL, dScope(func(a []any) {}), "document_documents"},
		{"documents/scoped-dense", ddSQL, dScope(func(a []any) {}), "document_documents"},
	}

	for _, p := range probes {
		t.Run(p.label, func(t *testing.T) {
			rows, err := pool.Query(ctx, "EXPLAIN (FORMAT TEXT) "+p.sql, p.args...)
			if err != nil {
				t.Fatalf("EXPLAIN: %v", err)
			}
			var plan strings.Builder
			for rows.Next() {
				var line string
				if err := rows.Scan(&line); err != nil {
					rows.Close()
					t.Fatalf("scan plan: %v", err)
				}
				plan.WriteString(line + "\n")
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				t.Fatalf("plan rows: %v", err)
			}
			text := plan.String()
			if strings.Contains(text, "Seq Scan on "+p.driver) {
				t.Errorf("%s sequentially scans %s — the filter is not index-backed:\n%s", p.label, p.driver, text)
			}
			t.Logf("%s:\n%s", p.label, text)
		})
	}
}

func strPtr(s string) *string { return &s }
func datePtr(s string) *time.Time {
	t, _ := time.Parse(domain.ISODate, s)
	return &t
}
