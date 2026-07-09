//go:build integration

// Integration tests for the Company vertical against a real Postgres (M21 exit criteria, D-Companies /
// D-Audit). They exercise the company module's audited CRUD, registrations with scheme validation,
// industry assignment, locations, the positions/appointments one-holder rule, and the ownership/
// affiliation graph (foundings, shareholdings, beneficiaries, successions, branches), including the
// person-purge erasure of the company person-link rows.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/company/...
package company_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olegamysk/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olegamysk/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olegamysk/go-oikumenea/internal/audit/domain"
	"github.com/olegamysk/go-oikumenea/internal/company/adapters"
	"github.com/olegamysk/go-oikumenea/internal/company/application"
	"github.com/olegamysk/go-oikumenea/internal/company/domain"
	personadapters "github.com/olegamysk/go-oikumenea/internal/person/adapters"
	personevents "github.com/olegamysk/go-oikumenea/internal/person/events"
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	tenantadapters "github.com/olegamysk/go-oikumenea/internal/tenant/adapters"
	tenantapp "github.com/olegamysk/go-oikumenea/internal/tenant/application"
	tenantdomain "github.com/olegamysk/go-oikumenea/internal/tenant/domain"
	"github.com/olegamysk/go-oikumenea/pkg/events"
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

// seedTenantCatalog idempotently seeds the `company` reference domain (pdp_scoped=false) — what
// tenant.Register seeds at boot — so a company (a `company`-domain org) can be created in the test DB
// (M41 / D-UnifiedOrgGraph). Companies have no internal unit tree, so no unit kinds are seeded.
func seedTenantCatalog(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO oikumenea.tenant_domains (code, name, pdp_scoped, sort_order)
		VALUES ('company','Company',false,40)
		ON CONFLICT (code) WHERE deleted_at IS NULL DO NOTHING`); err != nil {
		t.Fatalf("seed company domain: %v", err)
	}
}

func ptr(s string) *string      { return &s }
func fptr(f float64) *float64   { return &f }
func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Owner') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
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

func catalogID(t *testing.T, pool *pgxpool.Pool, table, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea."+table+" WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("resolve %s %s: %v", table, code, err)
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

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// TestCompanyVertical drives the whole M21 exit-criteria slice in one ordered scenario.
func TestCompanyVertical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	llc := catalogID(t, pool, "company_legal_forms", "llc")
	leiScheme := catalogID(t, pool, "company_registration_schemes", "lei")
	edrpouScheme := catalogID(t, pool, "company_registration_schemes", "ua-edrpou")
	infoTech := catalogID(t, pool, "company_industry_classes", "nace-j")

	// --- register a company with a legal form + ownership category ---
	main, err := svc.CreateCompany(ctx, domain.CompanyInput{
		Code: uniq("acme"), LegalName: "Acme LLC", LegalFormID: llc, OwnershipCategory: ptr("private"),
	})
	if err != nil {
		t.Fatalf("create company: %v", err)
	}
	assertOneAction(t, pool, main.ID, "company.create")
	if main.OwnershipCategory != "private" {
		t.Fatalf("ownership category = %q, want private", main.OwnershipCategory)
	}

	// --- LEI (global spine) + national EDRPOU registration, both validated against their patterns ---
	lei, err := svc.AddRegistration(ctx, main.ID, domain.RegistrationInput{SchemeID: leiScheme, Identifier: "529900T8BM49AURSDO55"})
	if err != nil {
		t.Fatalf("add LEI: %v", err)
	}
	if !lei.Validated {
		t.Fatalf("LEI 529900T8BM49AURSDO55 should validate against the ISO-17442 pattern")
	}
	edrpou, err := svc.AddRegistration(ctx, main.ID, domain.RegistrationInput{SchemeID: edrpouScheme, Identifier: "12345678"})
	if err != nil {
		t.Fatalf("add EDRPOU: %v", err)
	}
	if !edrpou.Validated {
		t.Fatalf("EDRPOU 12345678 should validate (8 digits)")
	}
	// a bad EDRPOU is stored but flagged invalid
	badEdrpou, err := svc.AddRegistration(ctx, main.ID, domain.RegistrationInput{SchemeID: edrpouScheme, Identifier: "ABC"})
	if err != nil {
		t.Fatalf("add bad EDRPOU: %v", err)
	}
	if badEdrpou.Validated {
		t.Fatalf("EDRPOU 'ABC' must not validate")
	}

	// --- primary industry + registered address (→ M19 location) ---
	if _, err := svc.AssignIndustry(ctx, main.ID, domain.IndustryInput{IndustryClassID: infoTech, IsPrimary: true}); err != nil {
		t.Fatalf("assign industry: %v", err)
	}
	loc := seedLocation(t, pool)
	if _, err := svc.AddCompanyLocation(ctx, main.ID, domain.CompanyLocationInput{LocationID: loc, Role: ptr("registered")}); err != nil {
		t.Fatalf("add location: %v", err)
	}

	// --- appoint a CEO; one billet, one holder ---
	ceoPos, err := svc.CreatePosition(ctx, main.ID, domain.PositionInput{Code: "ceo", Title: "Chief Executive Officer"})
	if err != nil {
		t.Fatalf("create position: %v", err)
	}
	ceo := seedPerson(t, pool)
	appt, err := svc.FillPosition(ctx, ceoPos.ID, ceo, nil)
	if err != nil {
		t.Fatalf("fill position: %v", err)
	}
	assertOneAction(t, pool, appt.ID, "company.appointment.fill")
	if _, err := svc.FillPosition(ctx, ceoPos.ID, seedPerson(t, pool), nil); !errors.Is(err, domain.ErrPositionAlreadyFilled) {
		t.Fatalf("expected PositionAlreadyFilled on second fill, got %v", err)
	}

	// --- a person founder ---
	founder := seedPerson(t, pool)
	if _, err := svc.RecordFounding(ctx, main.ID, domain.FoundingInput{HolderKind: domain.HolderPerson, HolderID: founder}); err != nil {
		t.Fatalf("record founding: %v", err)
	}

	// --- a 60% corporate shareholder (the ownership DAG) ---
	holdco, err := svc.CreateCompany(ctx, domain.CompanyInput{Code: uniq("holdco"), LegalName: "Holdco JSC", LegalFormID: llc})
	if err != nil {
		t.Fatalf("create holdco: %v", err)
	}
	sh, err := svc.RecordShareholding(ctx, main.ID, domain.ShareholdingInput{HolderKind: domain.HolderCompany, HolderID: holdco.ID, StakePct: fptr(60)})
	if err != nil {
		t.Fatalf("record shareholding: %v", err)
	}
	if sh.StakePct == nil || *sh.StakePct != 60 {
		t.Fatalf("stake pct = %v, want 60", sh.StakePct)
	}

	// --- a beneficial owner (natural person, UBO) ---
	ubo := seedPerson(t, pool)
	if _, err := svc.RecordBeneficiary(ctx, main.ID, domain.BeneficiaryInput{PersonID: ubo, UltimatePct: fptr(60)}); err != nil {
		t.Fatalf("record beneficiary: %v", err)
	}

	// --- link a subsidiary (main holds the sub; the sub is a branch) ---
	sub, err := svc.CreateCompany(ctx, domain.CompanyInput{Code: uniq("sub"), LegalName: "Acme Sub LLC", LegalFormID: llc})
	if err != nil {
		t.Fatalf("create sub: %v", err)
	}
	if _, err := svc.RecordShareholding(ctx, sub.ID, domain.ShareholdingInput{HolderKind: domain.HolderCompany, HolderID: main.ID, StakePct: fptr(100)}); err != nil {
		t.Fatalf("record sub shareholding: %v", err)
	}
	if _, err := svc.RecordBranch(ctx, main.ID, sub.ID); err != nil {
		t.Fatalf("record branch: %v", err)
	}

	// --- link a predecessor (a prior entity succeeded by main) ---
	pred, err := svc.CreateCompany(ctx, domain.CompanyInput{Code: uniq("oldco"), LegalName: "Oldco LLC", LegalFormID: llc, OwnershipCategory: ptr("private")})
	if err != nil {
		t.Fatalf("create predecessor: %v", err)
	}
	if _, err := svc.RecordSuccession(ctx, pred.ID, domain.SuccessionInput{SuccessorID: main.ID, Kind: ptr("merger")}); err != nil {
		t.Fatalf("record succession: %v", err)
	}

	// --- query the ownership graph ---
	g, err := svc.GetOwnershipGraph(ctx, main.ID)
	if err != nil {
		t.Fatalf("ownership graph: %v", err)
	}
	if !hasShareholder(g.Shareholders, domain.HolderCompany, holdco.ID, "Holdco JSC") {
		t.Fatalf("expected Holdco as a 60%% shareholder with label, got %+v", g.Shareholders)
	}
	if !hasHolding(g.Holdings, sub.ID) {
		t.Fatalf("expected the sub among main's holdings, got %+v", g.Holdings)
	}
	if len(g.Beneficiaries) != 1 || g.Beneficiaries[0].PersonID != ubo {
		t.Fatalf("expected one UBO, got %+v", g.Beneficiaries)
	}
	if len(g.Founders) != 1 || g.Founders[0].HolderID != founder {
		t.Fatalf("expected one founder, got %+v", g.Founders)
	}
	if len(g.Branches) != 1 || g.Branches[0].BranchID != sub.ID {
		t.Fatalf("expected one branch, got %+v", g.Branches)
	}
	if len(g.Successions) != 1 || g.Successions[0].PredecessorID != pred.ID || g.Successions[0].SuccessorID != main.ID {
		t.Fatalf("expected the predecessor succession, got %+v", g.Successions)
	}

	// --- person affiliations: the CEO holds an appointment ---
	aff, err := svc.ListPersonAffiliations(ctx, ceo)
	if err != nil {
		t.Fatalf("person affiliations: %v", err)
	}
	if len(aff.Appointments) != 1 || aff.Appointments[0].CompanyID != main.ID || aff.Appointments[0].CompanyName != "Acme LLC" {
		t.Fatalf("expected the CEO appointment enriched with company, got %+v", aff.Appointments)
	}

	// --- person purge erases the person-link company rows (D-PIITiers) ---
	if countRows(t, pool, "SELECT count(*) FROM oikumenea.company_appointments WHERE person_id = $1", ceo) != 1 {
		t.Fatalf("precondition: ceo should have an appointment")
	}
	if _, err := personadapters.NewRepository(pool).Purge(ctx, ceo); err != nil {
		t.Fatalf("purge ceo: %v", err)
	}
	if _, err := personadapters.NewRepository(pool).Purge(ctx, founder); err != nil {
		t.Fatalf("purge founder: %v", err)
	}
	if _, err := personadapters.NewRepository(pool).Purge(ctx, ubo); err != nil {
		t.Fatalf("purge ubo: %v", err)
	}
	// company erases its own person-link rows via PersonPurged (D-PersonModuleSplit)
	firePersonPurge(t, ctx, pool, svc, ceo)
	firePersonPurge(t, ctx, pool, svc, founder)
	firePersonPurge(t, ctx, pool, svc, ubo)
	if n := countRows(t, pool, "SELECT count(*) FROM oikumenea.company_appointments WHERE person_id = $1", ceo); n != 0 {
		t.Fatalf("expected ceo appointment erased on purge, got %d", n)
	}
	if n := countRows(t, pool, "SELECT count(*) FROM oikumenea.company_foundings WHERE holder_kind='person' AND holder_id = $1", founder); n != 0 {
		t.Fatalf("expected founder founding erased on purge, got %d", n)
	}
	if n := countRows(t, pool, "SELECT count(*) FROM oikumenea.company_beneficiaries WHERE person_id = $1", ubo); n != 0 {
		t.Fatalf("expected UBO beneficiary erased on purge, got %d", n)
	}
}

func hasShareholder(rows []domain.Shareholding, kind, id, label string) bool {
	for _, r := range rows {
		if r.HolderKind == kind && r.HolderID == id && r.HolderLabel == label {
			return true
		}
	}
	return false
}

func hasHolding(rows []domain.Shareholding, issuerID string) bool {
	for _, r := range rows {
		if r.CompanyID == issuerID {
			return true
		}
	}
	return false
}
