//go:build integration

// Integration tests for the membership module against a real Postgres (M6 exit criteria, D-Position /
// one-holder / D-Audit):
//   - a position is created vacant in a unit and reads back with no holder;
//   - vacant/filled listings reflect the derived vacancy predicate;
//   - filling a vacant billet records a membership and shows it as the holder; filling again is
//     rejected (one billet, one holder);
//   - plain belonging is unique per (person, unit); a filling whose position belongs to another
//     unit is rejected;
//   - abolishing a filled billet is refused until the membership is ended; ending vacates the billet;
//   - ending an already-ended membership is rejected;
//   - unknown person/unit references are rejected via the DB FKs;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/membership/...
package membership_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	"github.com/olegamysk/go-oikumenea/internal/membership/application"
	"github.com/olegamysk/go-oikumenea/internal/membership/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

const defaultTestDSN = "postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable"

func newPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

// newService builds the membership application service directly (bypassing Register).
func newService(t *testing.T) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	return application.NewService(pool, repoFor, audit), pool
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

// seedUnit inserts a tenant unit directly and returns its RID.
func seedUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (code, name) VALUES ($1, 'Unit') RETURNING id`,
		code(t, "unit")).Scan(&id); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	return id
}

// seedPerson inserts a person directly and returns its RID.
func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Member') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

// seedRank inserts a category -> type -> rank chain and returns the rank RID.
func seedRank(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var sysID, catID, typeID, rankID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_systems (code, name, sort_order) VALUES ($1, 'Sys', 0) RETURNING id`,
		code(t, "sys")).Scan(&sysID); err != nil {
		t.Fatalf("seed system: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_categories (system_id, code, name, sort_order) VALUES ($1, $2, 'Cat', 0) RETURNING id`,
		sysID, code(t, "cat")).Scan(&catID); err != nil {
		t.Fatalf("seed category: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_types (system_id, category_id, code, name, sort_order) VALUES ($1, $2, $3, 'Typ', 0) RETURNING id`,
		sysID, catID, code(t, "typ")).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_ranks (system_id, type_id, code, name, sort_order) VALUES ($1, $2, $3, 'Rnk', 0) RETURNING id`,
		sysID, typeID, code(t, "rnk")).Scan(&rankID); err != nil {
		t.Fatalf("seed rank: %v", err)
	}
	return rankID
}

// TestPositionVacancyAndFill covers create-vacant -> fill -> holder -> already-filled -> end-vacates.
func TestPositionVacancyAndFill(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	unitID := seedUnit(t, pool)
	rankID := seedRank(t, pool)
	personA := seedPerson(t, pool)
	personB := seedPerson(t, pool)

	pos, err := svc.CreatePosition(ctx, domain.Position{UnitID: unitID, Code: code(t, "pos"), Title: "Operations Officer", RequiredRankID: rankID})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	if pos.Status != domain.PositionActive {
		t.Fatalf("status = %q, want active", pos.Status)
	}

	// vacant: reads back with no holder; vacant list has it, filled list does not.
	got, err := svc.GetPosition(ctx, pos.ID)
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if got.Holder != nil {
		t.Fatalf("new position should be vacant, got holder %+v", got.Holder)
	}
	assertPositionCount(t, svc, unitID, domain.FilterVacant, 1)
	assertPositionCount(t, svc, unitID, domain.FilterFilled, 0)

	// fill it.
	m, err := svc.FillPosition(ctx, pos.ID, personA, "", timeZero())
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	if m.PositionID != pos.ID || m.UnitID != unitID || m.Status != domain.MembershipActive {
		t.Fatalf("filling = %+v, want active membership on the position's unit", m)
	}
	got, _ = svc.GetPosition(ctx, pos.ID)
	if got.Holder == nil || got.Holder.ID != m.ID {
		t.Fatalf("holder = %+v, want the filling %s", got.Holder, m.ID)
	}
	assertPositionCount(t, svc, unitID, domain.FilterVacant, 0)
	assertPositionCount(t, svc, unitID, domain.FilterFilled, 1)

	// one billet, one holder.
	if _, err := svc.FillPosition(ctx, pos.ID, personB, "", timeZero()); !errors.Is(err, domain.ErrPositionAlreadyFilled) {
		t.Fatalf("second fill: want ErrPositionAlreadyFilled, got %v", err)
	}

	// abolish refused while filled.
	if _, err := svc.AbolishPosition(ctx, pos.ID); !errors.Is(err, domain.ErrPositionInUse) {
		t.Fatalf("abolish filled: want ErrPositionInUse, got %v", err)
	}

	// end the membership: vacates the billet.
	ended, err := svc.EndMembership(ctx, m.ID, "", timeZero())
	if err != nil {
		t.Fatalf("end: %v", err)
	}
	if ended.Status != domain.MembershipEnded || ended.EffectiveTo == nil {
		t.Fatalf("ended = %+v, want ended with effective_to", ended)
	}
	if got, _ := svc.GetPosition(ctx, pos.ID); got.Holder != nil {
		t.Fatalf("billet should be vacant after end, got holder %+v", got.Holder)
	}
	// ending again is rejected.
	if _, err := svc.EndMembership(ctx, m.ID, "", timeZero()); !errors.Is(err, domain.ErrMembershipLifecycle) {
		t.Fatalf("re-end: want ErrMembershipLifecycle, got %v", err)
	}
	// now abolish succeeds (idempotent on repeat).
	if _, err := svc.AbolishPosition(ctx, pos.ID); err != nil {
		t.Fatalf("abolish vacant: %v", err)
	}
	if _, err := svc.AbolishPosition(ctx, pos.ID); err != nil {
		t.Fatalf("idempotent abolish: %v", err)
	}
}

// TestPlainBelonging covers belonging uniqueness and the position-unit mismatch guard.
func TestPlainBelonging(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	unitID := seedUnit(t, pool)
	otherUnit := seedUnit(t, pool)
	person := seedPerson(t, pool)

	if _, err := svc.CreateMembership(ctx, domain.Membership{PersonID: person, UnitID: unitID}); err != nil {
		t.Fatalf("plain belonging: %v", err)
	}
	// duplicate active plain belonging for the same (person, unit).
	if _, err := svc.CreateMembership(ctx, domain.Membership{PersonID: person, UnitID: unitID}); !errors.Is(err, domain.ErrMembershipConflict) {
		t.Fatalf("duplicate belonging: want ErrMembershipConflict, got %v", err)
	}

	// a filling whose position belongs to another unit is rejected.
	pos, err := svc.CreatePosition(ctx, domain.Position{UnitID: otherUnit, Code: code(t, "pos"), Title: "Elsewhere"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	if _, err := svc.CreateMembership(ctx, domain.Membership{PersonID: person, UnitID: unitID, PositionID: pos.ID}); !errors.Is(err, domain.ErrPositionUnitMismatch) {
		t.Fatalf("cross-unit fill: want ErrPositionUnitMismatch, got %v", err)
	}

	// person roster reflects the one active membership.
	page, err := svc.ListPersonMemberships(ctx, person, 0, "")
	if err != nil {
		t.Fatalf("list person memberships: %v", err)
	}
	if len(page.Memberships) != 1 {
		t.Fatalf("person memberships = %d, want 1", len(page.Memberships))
	}
}

// TestUnknownReferences rejects unknown person/unit via the DB FKs.
func TestUnknownReferences(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	unitID := seedUnit(t, pool)
	person := seedPerson(t, pool)
	bogus := uuid.NewString()

	if _, err := svc.CreateMembership(ctx, domain.Membership{PersonID: person, UnitID: bogus}); !errors.Is(err, domain.ErrUnknownUnit) {
		t.Fatalf("unknown unit: want ErrUnknownUnit, got %v", err)
	}
	bogusPerson := uuid.NewString()
	if _, err := svc.CreateMembership(ctx, domain.Membership{PersonID: bogusPerson, UnitID: unitID}); !errors.Is(err, domain.ErrUnknownPerson) {
		t.Fatalf("unknown person: want ErrUnknownPerson, got %v", err)
	}
}

// TestCreatePositionAuditsInOneTx confirms a create records exactly one audit row keyed to it.
func TestCreatePositionAuditsInOneTx(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t)
	unitID := seedUnit(t, pool)

	pos, err := svc.CreatePosition(ctx, domain.Position{UnitID: unitID, Code: code(t, "pos"), Title: "Audited"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'position.create' AND actor_type = 'system' AND subsystem = 'membership-admin'",
		pos.ID,
	).Scan(&n); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows for %s = %d, want 1", pos.ID, n)
	}
}

func assertPositionCount(t *testing.T, svc *application.Service, unitID string, filter domain.PositionFilter, want int) {
	t.Helper()
	page, err := svc.ListPositions(context.Background(), unitID, filter, 0, "")
	if err != nil {
		t.Fatalf("list positions (%s): %v", filter, err)
	}
	if len(page.Positions) != want {
		t.Fatalf("positions (%s) = %d, want %d", filter, len(page.Positions), want)
	}
}

// timeZero is the zero instant the application maps to "default to now()".
func timeZero() time.Time { return time.Time{} }
