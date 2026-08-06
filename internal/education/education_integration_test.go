// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the Education vertical against a real Postgres (M20 exit criteria, D-Education
// / D-Audit). They exercise the education module's audited CRUD, the unit structure tree + maintained
// closure (cycle reject + reparent), the positions/appointments one-holder rule, and the person
// bindings (enrollments, dorm stays) including the person-purge erasure + the sponsorship education
// context.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/education/...
package education_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/education/adapters"
	"github.com/olehmushka/go-oikumenea/internal/education/application"
	"github.com/olehmushka/go-oikumenea/internal/education/domain"
	personadapters "github.com/olehmushka/go-oikumenea/internal/person/adapters"
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olehmushka/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olehmushka/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olehmushka/go-oikumenea/internal/tenant/domain"
	"github.com/olehmushka/go-oikumenea/pkg/events"
)

// firePersonPurge drives this module's PersonPurged erase subscription — the event path that replaced
// person's inline cross-module purge deletes (D-PersonModuleSplit, review-2026-07 R-09) — in one
// committed tx, so the purge-erasure tests exercise the real flow rather than the removed inline deletes.
func firePersonPurge(t *testing.T, ctx context.Context, pool *pgxpool.Pool, svc *application.Service, personID string) {
	t.Helper()
	bus := events.NewBus()
	svc.SubscribePersonPurge(bus)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin purge tx: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := bus.Publish(ctx, tx, personevents.PersonPurged{ID: personID}); err != nil {
		t.Fatalf("publish PersonPurged: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit purge tx: %v", err)
	}
}

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

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	seedTenantCatalog(t, pool)
	tenantSvc := tenantapp.NewService(pool, func(conn pdb.DBTX) tenantdomain.Repository {
		return tenantadapters.NewRepository(conn)
	}, audit)
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit, tenantSvc)
}

// seedTenantCatalog idempotently seeds the `university` reference domain (pdp_scoped=false) + its
// unit-kind catalog — what tenant.Register seeds at boot — so an institution (a university org) and its
// units can be created in the test DB (M41 / D-UnifiedOrgGraph).
func seedTenantCatalog(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO oikumenea.tenant_domains (code, name, pdp_scoped, sort_order)
		VALUES ('university','University',false,30)
		ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed university domain: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO oikumenea.tenant_unit_kinds (domain_id, code, name, sort_order)
		SELECT d.id, k.code, k.name, k.so
		FROM (VALUES ('campus','Campus',0),('institute','Institute',5),('faculty','Faculty',10),('department','Department',20),('chair','Chair',30)) AS k(code,name,so)
		JOIN oikumenea.tenant_domains d ON d.code='university' AND d.deleted_at IS NULL
		ON CONFLICT (domain_id, code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed university unit kinds: %v", err)
	}
}

// tenantUnitKindID resolves a `university`-domain unit-kind RID by code (tenant_unit_kinds — M41).
func tenantUnitKindID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`SELECT k.id FROM oikumenea.tenant_unit_kinds k
		 JOIN oikumenea.tenant_domains d ON d.id = k.domain_id
		 WHERE d.code='university' AND k.code=$1 AND k.deleted_at IS NULL`, code).Scan(&id); err != nil {
		t.Fatalf("resolve university unit kind %s: %v", code, err)
	}
	return id
}

func ptr(s string) *string { return &s }

// uniq makes the test idempotent against a persistent test DB (codes are unique among active rows).
func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Student') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

func catalogID(t *testing.T, pool *pgxpool.Pool, table, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea."+table+" WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("resolve %s %s: %v", table, code, err)
	}
	return id
}

func seedLocation(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var country, id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.geo_countries WHERE code = 'UA'").Scan(&country); err != nil {
		t.Fatalf("resolve UA: %v", err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.location_locations (geom, country_id)
		 VALUES (ST_SetSRID(ST_MakePoint(30.5234, 50.4501), 4326)::geography, $1) RETURNING id`, country).Scan(&id); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	return id
}

func assertOneAction(t *testing.T, pool *pgxpool.Pool, targetID, action string) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = $2 AND actor_type = 'system'",
		targetID, action).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 %s system action for %s, got %d", action, targetID, n)
	}
}

// TestEducationVertical drives the whole M20 exit-criteria slice in one ordered scenario.
func TestEducationVertical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	uniKind := catalogID(t, pool, "education_institution_kinds", "university")
	facultyKind := tenantUnitKindID(t, pool, "faculty")
	deptKind := tenantUnitKindID(t, pool, "department")
	masterLevel := catalogID(t, pool, "education_degree_levels", "isced-7")

	// --- institution -> faculty -> department + a study group ---
	inst, err := svc.CreateInstitution(ctx, domain.InstitutionInput{Code: uniq("kpi"), Name: "KPI", KindID: uniKind, CountryID: nil})
	if err != nil {
		t.Fatalf("create institution: %v", err)
	}
	assertOneAction(t, pool, inst.ID, "education.institution.create")

	faculty, err := svc.CreateUnit(ctx, inst.ID, domain.UnitInput{Code: uniq("fiot"), Name: "FIOT", KindID: facultyKind})
	if err != nil {
		t.Fatalf("create faculty: %v", err)
	}
	dept, err := svc.CreateUnit(ctx, inst.ID, domain.UnitInput{Code: uniq("cs"), Name: "Computer Science", KindID: deptKind, ParentID: ptr(faculty.ID)})
	if err != nil {
		t.Fatalf("create dept: %v", err)
	}
	group, err := svc.CreateGroup(ctx, dept.ID, domain.GroupInput{Code: uniq("cs-21"), Name: "CS-21"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// closure: "all units under the institution" + depth from root.
	units, err := svc.ListUnits(ctx, inst.ID)
	if err != nil {
		t.Fatalf("list units: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}
	depthByID := map[string]int{}
	for _, u := range units {
		depthByID[u.ID] = u.Depth
	}
	if depthByID[faculty.ID] != 0 || depthByID[dept.ID] != 1 {
		t.Fatalf("expected faculty depth 0 + dept depth 1, got %v", depthByID)
	}

	// cycle reject: reparent the faculty under its own descendant (the dept).
	if _, err := svc.ReparentUnit(ctx, faculty.ID, ptr(dept.ID)); !errors.Is(err, domain.ErrUnitCycle) {
		t.Fatalf("expected ErrUnitCycle, got %v", err)
	}

	// reparent dept to top-level then verify the tenant closure recomputed (dept depth -> 0).
	if _, err := svc.ReparentUnit(ctx, dept.ID, nil); err != nil {
		t.Fatalf("reparent dept: %v", err)
	}
	units, _ = svc.ListUnits(ctx, inst.ID)
	for _, u := range units {
		if u.ID == dept.ID && u.Depth != 0 {
			t.Fatalf("expected dept depth 0 after reparent to top, got %d", u.Depth)
		}
	}
	// put it back under the faculty for the enrollment.
	if _, err := svc.ReparentUnit(ctx, dept.ID, ptr(faculty.ID)); err != nil {
		t.Fatalf("reparent dept back: %v", err)
	}

	// --- person: enrollment + education-context sponsorship + dorm stay ---
	student := seedPerson(t, pool)
	enr, err := svc.CreateEnrollment(ctx, student, domain.EnrollmentInput{
		InstitutionID: inst.ID, UnitID: ptr(dept.ID), GroupID: ptr(group.ID), DegreeLevelID: ptr(masterLevel),
		FieldOfStudy: ptr("Software Engineering"), Qualification: ptr("MSc Computer Science"), EffectiveFrom: ptr("2021-09-01"),
	})
	if err != nil {
		t.Fatalf("create enrollment: %v", err)
	}
	assertOneAction(t, pool, enr.ID, "education.enrollment.create")

	// a dorm building (located via the shared M19 location) + a dorm stay.
	loc := seedLocation(t, pool)
	dorm, err := svc.CreateBuilding(ctx, inst.ID, domain.BuildingInput{Code: uniq("dorm-7"), Name: "Dormitory 7", Kind: "dormitory", LocationID: ptr(loc)})
	if err != nil {
		t.Fatalf("create dorm building: %v", err)
	}
	stay, err := svc.CreateDormitoryStay(ctx, student, domain.DormInput{BuildingID: dorm.ID, Room: ptr("512"), EffectiveFrom: ptr("2021-09-01")})
	if err != nil {
		t.Fatalf("create dorm stay: %v", err)
	}

	// --- positions: fill a dean billet, reject double-fill, end (vacate) ---
	professor := seedPerson(t, pool)
	dean, err := svc.CreatePosition(ctx, inst.ID, domain.PositionInput{Code: uniq("dean-fiot"), Title: "Dean of FIOT", UnitID: ptr(faculty.ID)})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	appt, err := svc.FillPosition(ctx, dean.ID, professor, nil)
	if err != nil {
		t.Fatalf("fill position: %v", err)
	}
	assertOneAction(t, pool, appt.ID, "education.appointment.fill")

	if _, err := svc.FillPosition(ctx, dean.ID, student, nil); !errors.Is(err, domain.ErrPositionAlreadyFilled) {
		t.Fatalf("expected ErrPositionAlreadyFilled, got %v", err)
	}
	// the position read attaches the current holder.
	got, err := svc.GetPosition(ctx, dean.ID)
	if err != nil || got.Holder == nil || got.Holder.PersonID != professor {
		t.Fatalf("expected dean holder=%s, got %+v (err %v)", professor, got.Holder, err)
	}
	if _, err := svc.EndAppointment(ctx, appt.ID, nil); err != nil {
		t.Fatalf("end appointment: %v", err)
	}
	got, _ = svc.GetPosition(ctx, dean.ID)
	if got.Holder != nil {
		t.Fatalf("expected dean vacant after end, got holder %+v", got.Holder)
	}

	// --- person purge erases the enrollment + dorm stay (D-Education / D-PIITiers) ---
	if _, err := personadapters.NewRepository(pool).Purge(ctx, student); err != nil {
		t.Fatalf("purge student: %v", err)
	}
	firePersonPurge(t, ctx, pool, svc, student) // education erases its own rows via PersonPurged (D-PersonModuleSplit)
	if rows, _ := svc.ListEnrollments(ctx, student); len(rows) != 0 {
		t.Fatalf("expected enrollments erased on purge, got %d", len(rows))
	}
	if rows, _ := svc.ListDormitoryStays(ctx, student); len(rows) != 0 {
		t.Fatalf("expected dorm stays erased on purge, got %d", len(rows))
	}
	_ = stay
}
