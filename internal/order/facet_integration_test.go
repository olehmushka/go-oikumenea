// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the top-level order listing and its facet filters (M56 ticket 3 /
// D-ObjectFacets) against a real Postgres.
//
// The same shape as the membership suite: each facet is asserted on BOTH list paths — the
// instance-admin ListOrders and the reach-scoped ListOrdersForSubject — and then the paths are
// compared directly, scoped(f) == admin(f) ∩ reach, which is the property that makes two separate
// SQL blocks carrying one vocabulary safe.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/order/... -run Facet
package order_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/order/application"
	"github.com/olehmushka/go-oikumenea/internal/order/domain"
)

func osp(s string) *string    { return &s }
func odp(s string) *time.Time { t, _ := time.Parse(domain.ISODate, s); return &t }

// oFacetWorld is a two-unit world: the reader may read unitIn, not unitOut. Every facet has a
// matching and a non-matching order inside the reader's reach, plus a matching one outside it, so
// the intersection assertion is not trivially the whole set.
type oFacetWorld struct {
	svc  *application.Service
	pool *pgxpool.Pool

	unitIn, unitOut string
	reader          string

	oDraft   string // draft,   issued_on NULL
	oIssued  string // issued,  issued_on 2024-05-06, has an item of typeA
	oRevoked string // revoked, issued_on 2020-02-03
	oOut     string // issued,  in unitOut

	typeA string
}

func seedOFacetWorld(t *testing.T) *oFacetWorld {
	t.Helper()
	e := newEnv(t)
	w := &oFacetWorld{svc: e.order, pool: e.pool}

	w.unitIn = seedUnit(t, e.pool)
	w.unitOut = seedUnit(t, e.pool)
	w.typeA = e.orderType(t, domain.CategoryPersonnelList, domain.EffectRecordOnly)

	w.oDraft = seedFacetOrder(t, e.pool, w.unitIn, "draft", "")
	w.oIssued = seedFacetOrder(t, e.pool, w.unitIn, "issued", "2024-05-06")
	w.oRevoked = seedFacetOrder(t, e.pool, w.unitIn, "revoked", "2020-02-03")
	w.oOut = seedFacetOrder(t, e.pool, w.unitOut, "issued", "2024-05-06")

	// One item, so the orderTypeId facet (an EXISTS over the items) has a target.
	person := seedPerson(t, e.pool)
	if _, err := e.pool.Exec(context.Background(),
		`INSERT INTO oikumenea.order_order_items (order_id, type_id, person_id) VALUES ($1, $2, $3)`,
		w.oIssued, w.typeA, person); err != nil {
		t.Fatalf("seed order item: %v", err)
	}

	w.reader = seedPerson(t, e.pool)
	seedOrderReadGrant(t, e.pool, w.reader, w.unitIn)
	return w
}

// seedFacetOrder inserts one order header with an explicit status and issued_on. Written as SQL
// because the service only mints drafts and the list must show every status.
func seedFacetOrder(t *testing.T, pool *pgxpool.Pool, unitID, status, issuedOn string) string {
	t.Helper()
	var on any
	if issuedOn != "" {
		on = issuedOn
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.order_orders (number, issuing_unit_id, status, issued_on)
		 VALUES ($1, $2, $3, $4::date) RETURNING id`,
		code(t, "facet-ord"), unitID, status, on).Scan(&id); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	return id
}

func seedOrderReadGrant(t *testing.T, pool *pgxpool.Pool, readerID, unitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Order facet test role') RETURNING id`,
		code(t, "ofacet-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'order.read')`,
		roleID); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1, $2, $3, 'unit')`, readerID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
}

func oAllIDs(t *testing.T, list func(pageToken string) (application.OrderPage, error)) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	token := ""
	for i := 0; ; i++ {
		if i > 2000 {
			t.Fatal("paging did not terminate after 2000 pages")
		}
		page, err := list(token)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		for _, o := range page.Orders {
			out[o.ID] = true
		}
		if page.NextPageToken == "" {
			return out
		}
		token = page.NextPageToken
	}
}

func (w *oFacetWorld) admin(t *testing.T, f domain.OrderFilter) map[string]bool {
	t.Helper()
	return oAllIDs(t, func(tok string) (application.OrderPage, error) {
		return w.svc.ListOrders(context.Background(), f, 50, tok)
	})
}

func (w *oFacetWorld) scoped(t *testing.T, f domain.OrderFilter) map[string]bool {
	t.Helper()
	return oAllIDs(t, func(tok string) (application.OrderPage, error) {
		return w.svc.ListVisibleOrders(context.Background(), w.reader, f, 50, tok)
	})
}

func TestOrderFacetsRoundTrip(t *testing.T) {
	w := seedOFacetWorld(t)

	cases := []struct {
		name              string
		filter            domain.OrderFilter
		wantIn, wantOutOf []string
	}{
		{
			"issuingUnitId", domain.OrderFilter{IssuingUnitID: osp(w.unitIn)},
			[]string{w.oDraft, w.oIssued, w.oRevoked}, []string{w.oOut},
		},
		{
			"status=draft", domain.OrderFilter{Status: osp("draft")},
			[]string{w.oDraft}, []string{w.oIssued, w.oRevoked},
		},
		{
			"status=revoked", domain.OrderFilter{Status: osp("revoked")},
			[]string{w.oRevoked}, []string{w.oDraft, w.oIssued},
		},
		{
			// An order's effect lives on its items, so this is an EXISTS semi-join. The order must
			// appear exactly ONCE — a join would multiply it across items and corrupt the keyset.
			"orderTypeId", domain.OrderFilter{OrderTypeID: osp(w.typeA)},
			[]string{w.oIssued}, []string{w.oDraft, w.oRevoked},
		},
		{
			// A draft has no issue date, so any bound excludes it (three-valued logic) — the
			// behaviour the (unknown) bucket exists for on the M57 stats endpoint.
			"issuedOn range excludes drafts", domain.OrderFilter{
				IssuedOnFrom: odp("2024-01-01"), IssuedOnTo: odp("2024-12-31"),
			},
			[]string{w.oIssued}, []string{w.oDraft, w.oRevoked},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := w.admin(t, c.filter)
			for _, id := range c.wantIn {
				if !got[id] {
					t.Errorf("admin: %s missing from the filtered set", id)
				}
			}
			for _, id := range c.wantOutOf {
				if got[id] {
					t.Errorf("admin: %s should not match this filter", id)
				}
			}
			assertOrderScopedIsAdminIntersectReach(t, w, c.filter)
		})
	}
}

func TestOrderScopedEqualsAdminIntersectReach(t *testing.T) {
	w := seedOFacetWorld(t)
	for _, f := range []domain.OrderFilter{
		{},
		{Status: osp("issued")},
		{Status: osp("draft")},
		{IssuingUnitID: osp(w.unitIn)},
		{IssuingUnitID: osp(w.unitOut)},
		{OrderTypeID: osp(w.typeA)},
		{IssuedOnFrom: odp("2024-01-01")},
	} {
		assertOrderScopedIsAdminIntersectReach(t, w, f)
	}
}

func assertOrderScopedIsAdminIntersectReach(t *testing.T, w *oFacetWorld, f domain.OrderFilter) {
	t.Helper()
	admin := w.admin(t, f)
	scoped := w.scoped(t, f)

	inUnit := map[string]bool{}
	rows, err := w.pool.Query(context.Background(),
		`SELECT id FROM oikumenea.order_orders WHERE issuing_unit_id = $1 AND deleted_at IS NULL`, w.unitIn)
	if err != nil {
		t.Fatalf("read unit orders: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		inUnit[id] = true
	}

	for id := range scoped {
		if !admin[id] {
			t.Errorf("scoped returned %s, which admin with the same filter does not — the scoped path "+
				"must never be WIDER than admin", id)
		}
		if !inUnit[id] {
			t.Errorf("scoped returned %s, whose issuing unit is outside the reader's reach", id)
		}
	}
	for id := range admin {
		if inUnit[id] && !scoped[id] {
			t.Errorf("scoped omitted %s, which is both admin-visible and in reach — the two filter "+
				"blocks have drifted", id)
		}
	}
}

// TestOrderFilteredPagingDrainsExactly is the R-06 assertion: a filtered listing paged one row at a
// time yields the same set as one large page. Doubly important for orderTypeId, where a join instead
// of an EXISTS would emit an order once per matching item and silently duplicate keyset rows.
func TestOrderFilteredPagingDrainsExactly(t *testing.T) {
	w := seedOFacetWorld(t)
	for _, f := range []domain.OrderFilter{
		{IssuingUnitID: osp(w.unitIn)},
		{OrderTypeID: osp(w.typeA)},
	} {
		bulk := w.admin(t, f)
		drain := oAllIDs(t, func(tok string) (application.OrderPage, error) {
			return w.svc.ListOrders(context.Background(), f, 1, tok)
		})
		if len(bulk) != len(drain) {
			t.Fatalf("pageSize=1 drain yielded %d rows, bulk yielded %d", len(drain), len(bulk))
		}
		for id := range bulk {
			if !drain[id] {
				t.Errorf("%s appears in the bulk page but not in the one-at-a-time drain", id)
			}
		}
	}
}

func TestOrderFilterValidation(t *testing.T) {
	w := seedOFacetWorld(t)
	ctx := context.Background()

	for _, c := range []struct {
		name string
		f    domain.OrderFilter
	}{
		{"unknown status", domain.OrderFilter{Status: osp("cancelled")}},
		{"non-RID issuingUnitId", domain.OrderFilter{IssuingUnitID: osp("not-a-rid")}},
		{"inverted range", domain.OrderFilter{
			IssuedOnFrom: odp("2025-01-01"), IssuedOnTo: odp("2024-01-01"),
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, err := w.svc.ListOrders(ctx, c.f, 50, ""); err == nil {
				t.Error("admin path accepted an invalid filter")
			}
			if _, err := w.svc.ListVisibleOrders(ctx, w.reader, c.f, 50, ""); err == nil {
				t.Error("scoped path accepted an invalid filter")
			}
		})
	}
}
