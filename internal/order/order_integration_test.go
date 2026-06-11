//go:build integration

// Integration tests for the order module against a real Postgres (M10 exit criteria, D-Orders /
// D-OrderApply): the headline ISSUE flow auto-applies an order's structural effects in the SAME
// transaction via the in-process event bus, with the membership/person subscribers wired exactly as
// composition does:
//   - issuing an appointment order fills the billet in the same txn (and audits the fill under the
//     event-subscriber subsystem, correlated to order.issue by request_id);
//   - a failing effect (the billet is already filled) rolls the WHOLE issue back: the order stays
//     draft and the prior holder is untouched (all-or-nothing);
//   - a record-only item stands alone (no membership written);
//   - a rank-change order sets the person's rank in the same txn;
//   - revoke flips issued -> revoked (and does not auto-reverse effects); editing/issuing an issued
//     order is rejected; a duplicate order number in a unit conflicts.
//
// Run against a DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/order/...
package order_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	memadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	memapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	memdomain "github.com/olegamysk/go-oikumenea/internal/membership/domain"
	orderadapters "github.com/olegamysk/go-oikumenea/internal/order/adapters"
	orderapp "github.com/olegamysk/go-oikumenea/internal/order/application"
	orderdomain "github.com/olegamysk/go-oikumenea/internal/order/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
	personapp "github.com/olegamysk/go-oikumenea/internal/person/application"
	persondomain "github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/pkg/events"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

// env bundles the three application services wired to one bus, exactly as composition does.
type env struct {
	order *orderapp.Service
	mem   *memapp.Service
	psn   *personapp.Service
	pool  *pgxpool.Pool
}

func newEnv(t *testing.T) env {
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

	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	bus := events.NewBus()
	mem := memapp.NewService(pool, func(conn pdb.DBTX) memdomain.Repository { return memadapters.NewRepository(conn) }, audit)
	psn := personapp.NewService(pool, func(conn pdb.DBTX) persondomain.Repository { return personadapters.NewRepository(conn) }, audit, func() int { return 720 })
	ord := orderapp.NewService(pool, func(conn pdb.DBTX) orderdomain.Repository { return orderadapters.NewRepository(conn) }, audit, bus)
	mem.SubscribeOrderEvents(bus)
	psn.SubscribeOrderEvents(bus)
	return env{order: ord, mem: mem, psn: psn, pool: pool}
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

func seedUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (code, name) VALUES ($1, 'Unit') RETURNING id`, code(t, "unit")).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Member') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

func seedRank(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var sysID, catID, typeID, rankID string
	mustScan(t, pool.QueryRow(ctx, `INSERT INTO oikumenea.rank_systems (code, name, sort_order) VALUES ($1,'Sys',0) RETURNING id`, code(t, "sys")).Scan(&sysID))
	mustScan(t, pool.QueryRow(ctx, `INSERT INTO oikumenea.rank_categories (system_id, code, name, sort_order) VALUES ($1,$2,'Cat',0) RETURNING id`, sysID, code(t, "cat")).Scan(&catID))
	mustScan(t, pool.QueryRow(ctx, `INSERT INTO oikumenea.rank_types (system_id, category_id, code, name, sort_order) VALUES ($1,$2,$3,'Typ',0) RETURNING id`, sysID, catID, code(t, "typ")).Scan(&typeID))
	mustScan(t, pool.QueryRow(ctx, `INSERT INTO oikumenea.rank_ranks (system_id, type_id, code, name, sort_order) VALUES ($1,$2,$3,'Rnk',0) RETURNING id`, sysID, typeID, code(t, "rnk")).Scan(&rankID))
	return rankID
}

func mustScan(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// orderType creates an order type of the given category/effect and returns its RID.
func (e env) orderType(t *testing.T, category orderdomain.OrderCategory, effect orderdomain.OrderEffect) string {
	t.Helper()
	ot, err := e.order.CreateOrderType(context.Background(), orderdomain.OrderType{
		Code: code(t, "ot"), Name: "Type", Category: category, Effect: effect,
	})
	if err != nil {
		t.Fatalf("create order type (%s/%s): %v", category, effect, err)
	}
	return ot.ID
}

// TestIssueAppointmentFillsBillet covers the headline same-transaction effect: issue fills the billet.
func TestIssueAppointmentFillsBillet(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	person := seedPerson(t, e.pool)
	pos, err := e.mem.CreatePosition(ctx, memdomain.Position{UnitID: unit, Code: code(t, "pos"), Title: "Officer"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	appoint := e.orderType(t, orderdomain.CategoryAppointment, orderdomain.EffectMembershipStart)

	ord, err := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{
		Number: code(t, "no"),
		Items:  []orderdomain.OrderItem{{TypeID: appoint, PersonID: person, PositionID: pos.ID}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if ord.Status != orderdomain.OrderDraft || len(ord.Items) != 1 {
		t.Fatalf("created order = %+v, want draft with 1 item", ord)
	}

	issued, err := e.order.IssueOrder(ctx, ord.ID)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if issued.Status != orderdomain.OrderIssued {
		t.Fatalf("status = %q, want issued", issued.Status)
	}

	// The billet is now filled by the person, citing the order item as provenance — in the same txn.
	got, err := e.mem.GetPosition(ctx, pos.ID)
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if got.Holder == nil {
		t.Fatal("position has no holder after issue — effect did not apply")
	}
	if got.Holder.PersonID != person {
		t.Fatalf("holder person = %q, want %q", got.Holder.PersonID, person)
	}
	if got.Holder.OrderItemID != issued.Items[0].ID {
		t.Fatalf("holder order_item_id = %q, want %q (provenance)", got.Holder.OrderItemID, issued.Items[0].ID)
	}

	// The fill audited under the event-subscriber subsystem, correlated to order.issue by request_id.
	var n int
	if err := e.pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.audit_log WHERE action='membership.fill' AND subsystem='event-subscriber' AND target_id=$1`,
		got.Holder.ID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("event-subscriber fill audit rows = %d, want 1", n)
	}
}

// TestIssueAllOrNothingRollback covers the all-or-nothing guarantee: a failing effect rolls the whole
// issue back, the order stays draft, and the prior holder is untouched.
func TestIssueAllOrNothingRollback(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	personA := seedPerson(t, e.pool)
	personB := seedPerson(t, e.pool)
	pos, err := e.mem.CreatePosition(ctx, memdomain.Position{UnitID: unit, Code: code(t, "pos"), Title: "Officer"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	appoint := e.orderType(t, orderdomain.CategoryAppointment, orderdomain.EffectMembershipStart)

	// First appointment fills the billet with person A.
	first, _ := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: []orderdomain.OrderItem{{TypeID: appoint, PersonID: personA, PositionID: pos.ID}}})
	if _, err := e.order.IssueOrder(ctx, first.ID); err != nil {
		t.Fatalf("issue first: %v", err)
	}

	// Second appointment targets the SAME (now filled) billet with person B → the one-holder index
	// trips, so the whole issue rolls back.
	second, _ := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: []orderdomain.OrderItem{{TypeID: appoint, PersonID: personB, PositionID: pos.ID}}})
	_, err = e.order.IssueOrder(ctx, second.ID)
	if !errors.Is(err, orderdomain.ErrEffectFailed) {
		t.Fatalf("issue second err = %v, want ErrEffectFailed", err)
	}

	// The second order stays draft.
	again, err := e.order.GetOrder(ctx, second.ID)
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if again.Status != orderdomain.OrderDraft {
		t.Fatalf("second order status = %q, want draft (rolled back)", again.Status)
	}
	// The billet is still held by person A.
	got, _ := e.mem.GetPosition(ctx, pos.ID)
	if got.Holder == nil || got.Holder.PersonID != personA {
		t.Fatalf("holder = %+v, want person A still in place", got.Holder)
	}
	// And no membership row was written for person B — the failed fill rolled back cleanly.
	pageB, err := e.mem.ListPersonMemberships(ctx, personB, 50, "")
	if err != nil {
		t.Fatalf("list B memberships: %v", err)
	}
	if len(pageB.Memberships) != 0 {
		t.Fatalf("rolled-back order left %d memberships for person B, want 0", len(pageB.Memberships))
	}
}

// TestRecordOnlyStandsAlone covers a record-only item: no downstream membership is written.
func TestRecordOnlyStandsAlone(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	person := seedPerson(t, e.pool)
	leave := e.orderType(t, orderdomain.CategoryLeaveTravel, orderdomain.EffectRecordOnly)

	ord, _ := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: []orderdomain.OrderItem{{TypeID: leave, PersonID: person, Note: "annual leave"}}})
	if _, err := e.order.IssueOrder(ctx, ord.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	page, err := e.mem.ListPersonMemberships(ctx, person, 50, "")
	if err != nil {
		t.Fatalf("list memberships: %v", err)
	}
	if len(page.Memberships) != 0 {
		t.Fatalf("record-only item produced %d memberships, want 0", len(page.Memberships))
	}
}

// TestRankChangeSetsRank covers the rank-change effect: issue sets the person's rank in the same txn.
func TestRankChangeSetsRank(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	person := seedPerson(t, e.pool)
	rank := seedRank(t, e.pool)
	award := e.orderType(t, orderdomain.CategoryDisciplineIncentive, orderdomain.EffectRankChange)

	ord, _ := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: []orderdomain.OrderItem{{TypeID: award, PersonID: person, RankID: rank}}})
	if _, err := e.order.IssueOrder(ctx, ord.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	got, err := e.psn.GetPerson(ctx, person)
	if err != nil {
		t.Fatalf("get person: %v", err)
	}
	// The rank-change effect upserts the rank in its (derived) system; one rank per system (D-Rank).
	var found bool
	for _, r := range got.Ranks {
		if r.RankID == rank {
			found = true
		}
	}
	if !found {
		t.Fatalf("person ranks = %+v, want one holding %q", got.Ranks, rank)
	}
}

// TestLifecycleGuards covers draft-is-editable / issued-is-locked / revoke and the conflict guards.
func TestLifecycleGuards(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	person := seedPerson(t, e.pool)
	leave := e.orderType(t, orderdomain.CategoryLeaveTravel, orderdomain.EffectRecordOnly)
	number := code(t, "no")

	ord, err := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Number: number, Items: []orderdomain.OrderItem{{TypeID: leave, PersonID: person}}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Duplicate number in the same unit conflicts.
	if _, err := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Number: number, Items: []orderdomain.OrderItem{{TypeID: leave, PersonID: person}}}); !errors.Is(err, orderdomain.ErrOrderConflict) {
		t.Fatalf("dup number err = %v, want ErrOrderConflict", err)
	}

	// Revoking a draft is rejected (not issued).
	if _, err := e.order.RevokeOrder(ctx, ord.ID, nil); !errors.Is(err, orderdomain.ErrNotIssued) {
		t.Fatalf("revoke draft err = %v, want ErrNotIssued", err)
	}

	// Issue, then editing/issuing again is rejected; revoke flips to revoked.
	if _, err := e.order.IssueOrder(ctx, ord.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	newNum := code(t, "no2")
	if _, err := e.order.UpdateOrder(ctx, ord.ID, orderapp.UpdateOrderInput{Number: &newNum}); !errors.Is(err, orderdomain.ErrAlreadyIssued) {
		t.Fatalf("edit issued err = %v, want ErrAlreadyIssued", err)
	}
	if _, err := e.order.IssueOrder(ctx, ord.ID); !errors.Is(err, orderdomain.ErrAlreadyIssued) {
		t.Fatalf("re-issue err = %v, want ErrAlreadyIssued", err)
	}
	revoked, err := e.order.RevokeOrder(ctx, ord.ID, nil)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked.Status != orderdomain.OrderRevoked {
		t.Fatalf("status = %q, want revoked", revoked.Status)
	}
	// Revoke again is rejected.
	if _, err := e.order.RevokeOrder(ctx, ord.ID, nil); !errors.Is(err, orderdomain.ErrNotIssued) {
		t.Fatalf("re-revoke err = %v, want ErrNotIssued", err)
	}
}

// TestEffectTargetValidation covers application-side required-target validation on create.
func TestEffectTargetValidation(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	unit := seedUnit(t, e.pool)
	person := seedPerson(t, e.pool)
	appoint := e.orderType(t, orderdomain.CategoryAppointment, orderdomain.EffectMembershipStart)

	// membership-start needs a unit or position; an item with neither is rejected.
	if _, err := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: []orderdomain.OrderItem{{TypeID: appoint, PersonID: person}}}); !errors.Is(err, orderdomain.ErrOrderInvalid) {
		t.Fatalf("missing-target err = %v, want ErrOrderInvalid", err)
	}
	// An empty order (no items) is rejected.
	if _, err := e.order.CreateOrder(ctx, unit, orderapp.CreateOrderInput{Items: nil}); !errors.Is(err, orderdomain.ErrOrderInvalid) {
		t.Fatalf("empty-order err = %v, want ErrOrderInvalid", err)
	}
}
