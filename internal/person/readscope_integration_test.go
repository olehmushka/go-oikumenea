//go:build integration

// Integration tests for the person read-scope projection (D-PersonReadScope / F-001) against a real
// Postgres: they seed units, people, and active memberships, then assert that ReadablePerson and the
// ListVisiblePersons directory-union honour the reader's effective readable reach — a reader sees a
// person only when that person's active-membership units intersect the reader's reach (or the reader
// is an instance admin). This exercises the new membership union/intersection SQL end-to-end.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	membershipadapters "github.com/olegamysk/go-oikumenea/internal/membership/adapters"
	membershipapp "github.com/olegamysk/go-oikumenea/internal/membership/application"
	membershipdomain "github.com/olegamysk/go-oikumenea/internal/membership/domain"
	"github.com/olegamysk/go-oikumenea/internal/person/application"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
)

// membershipReader builds the membership application service over the same pool and binds it as the
// person service's read-scope query seam (mirroring the composition root's SetMembershipReader).
func bindMembership(t *testing.T, svc *application.Service, pool *pgxpool.Pool) {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	memSvc := membershipapp.NewService(pool, func(conn pdb.DBTX) membershipdomain.Repository {
		return membershipadapters.NewRepository(conn)
	}, audit)
	svc.SetMembershipReader(memSvc)
}

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

func seedPerson(t *testing.T, svc *application.Service) string {
	t.Helper()
	p, err := svc.CreatePerson(context.Background(), domain.Person{Name: domain.Name{DisplayName: "Test"}})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	return p.ID
}

func seedMembership(t *testing.T, pool *pgxpool.Pool, personID, unitID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO oikumenea.membership_memberships (person_id, unit_id) VALUES ($1, $2)`,
		personID, unitID); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func readableReach(units ...string) authzdomain.Reach {
	r := authzdomain.Reach{Readable: map[string]struct{}{}}
	for _, u := range units {
		r.Readable[u] = struct{}{}
	}
	return r
}

func TestReadScopeProjection_Integration(t *testing.T) {
	svc, pool := newService(t, 720)
	bindMembership(t, svc, pool)
	ctx := context.Background()

	unitA := seedUnit(t, pool)
	unitB := seedUnit(t, pool)
	pInA := seedPerson(t, svc)
	pInB := seedPerson(t, svc)
	pNone := seedPerson(t, svc) // membership-less
	seedMembership(t, pool, pInA, unitA)
	seedMembership(t, pool, pInB, unitB)

	reachA := readableReach(unitA)

	// ReadablePerson: a unit-A reader sees the unit-A person, not the unit-B nor the membership-less one.
	for _, tc := range []struct {
		person string
		want   bool
	}{{pInA, true}, {pInB, false}, {pNone, false}} {
		got, err := svc.ReadablePerson(ctx, reachA, tc.person)
		if err != nil {
			t.Fatalf("ReadablePerson(%s): %v", tc.person, err)
		}
		if got != tc.want {
			t.Fatalf("ReadablePerson(%s) = %v, want %v", tc.person, got, tc.want)
		}
	}

	// Instance admin sees the membership-less person.
	if ok, err := svc.ReadablePerson(ctx, authzdomain.Reach{InstanceAdmin: true}, pNone); err != nil || !ok {
		t.Fatalf("instance admin must read a membership-less person (ok=%v err=%v)", ok, err)
	}

	// ListVisiblePersons: the unit-A reader's directory union contains pInA and excludes pInB / pNone.
	page, err := svc.ListVisiblePersons(ctx, reachA, 0, "")
	if err != nil {
		t.Fatalf("ListVisiblePersons: %v", err)
	}
	got := map[string]bool{}
	for _, p := range page.Persons {
		got[p.ID] = true
	}
	if !got[pInA] {
		t.Fatalf("ListVisiblePersons must include the unit-A person %s", pInA)
	}
	if got[pInB] || got[pNone] {
		t.Fatalf("ListVisiblePersons leaked an out-of-reach person: %v", got)
	}
}
