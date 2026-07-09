//go:build integration

// Integration tests for the person read-scope projection (D-PersonReadScope / F-001) against a real
// Postgres. Since review-2026-07 R-02.1 the reach is computed IN SQL from the subject's actual role
// assignments (membership's SubjectCanReadPerson / VisiblePersonIDsForSubject semi-joins), so these
// tests seed a real reader with a real `person.read` grant and assert the projection end-to-end: a
// reader sees a person only when that person's active-membership units fall in the reader's reach.
// (The instance-admin bypass now lives in the transport via pep.SubjectAuthority — not tested here.)
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

// ensureOrgSQL idempotently seeds a test domain + organization (D-TenantOrganizations, M40) so
// seedUnit can place a unit in a real organization.
const ensureOrgSQL = `
INSERT INTO oikumenea.tenant_domains (code, name) VALUES ('test-domain','Test Domain')
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING;
INSERT INTO oikumenea.tenant_organizations (code, name, domain_id)
  SELECT 'test-org','Test Org', d.id FROM oikumenea.tenant_domains d WHERE d.code='test-domain'
  ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`

func seedUnit(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	if _, err := pool.Exec(context.Background(), ensureOrgSQL); err != nil {
		t.Fatalf("ensure org: %v", err)
	}
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.tenant_units (org_id, domain_id, code, name)
		 SELECT o.id, o.domain_id, $1, 'Unit' FROM oikumenea.tenant_organizations o WHERE o.code='test-org'
		 RETURNING id`,
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

// seedReadGrant gives the reader a fresh role carrying person.read with a unit-scope assignment on
// unitID — the real authority rows the R-02.1 semi-join reads.
func seedReadGrant(t *testing.T, pool *pgxpool.Pool, readerID, unitID string) {
	t.Helper()
	ctx := context.Background()
	var roleID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.authz_roles (code, name) VALUES ($1, 'Read-scope test role') RETURNING id`,
		code(t, "readscope-role")).Scan(&roleID); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_permissions (role_id, permission_code) VALUES ($1, 'person.read')`,
		roleID); err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.authz_role_assignments (subject_person_id, role_id, target_unit_id, scope)
		 VALUES ($1, $2, $3, 'unit')`,
		readerID, roleID, unitID); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
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

	reader := seedPerson(t, svc)
	seedReadGrant(t, pool, reader, unitA) // reach = {unitA}

	// ReadablePerson: the unit-A reader sees the unit-A person, not the unit-B nor the
	// membership-less one; a grant-less subject sees nobody.
	for _, tc := range []struct {
		subject string
		person  string
		want    bool
	}{
		{reader, pInA, true},
		{reader, pInB, false},
		{reader, pNone, false},
		{pInB, pInA, false}, // no grant at all
	} {
		got, err := svc.ReadablePerson(ctx, tc.subject, tc.person)
		if err != nil {
			t.Fatalf("ReadablePerson(%s, %s): %v", tc.subject, tc.person, err)
		}
		if got != tc.want {
			t.Fatalf("ReadablePerson(%s, %s) = %v, want %v", tc.subject, tc.person, got, tc.want)
		}
	}

	// ListVisiblePersons: the unit-A reader's directory union contains pInA and excludes pInB / pNone.
	page, err := svc.ListVisiblePersons(ctx, reader, 0, "", "")
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

	// A grant-less subject's directory union is empty.
	emptyPage, err := svc.ListVisiblePersons(ctx, pInB, 0, "", "")
	if err != nil {
		t.Fatalf("ListVisiblePersons(grantless): %v", err)
	}
	if len(emptyPage.Persons) != 0 {
		t.Fatalf("grant-less subject must see an empty directory, got %d persons", len(emptyPage.Persons))
	}
}
