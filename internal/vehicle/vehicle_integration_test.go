//go:build integration

// Integration tests for the Vehicle vertical against a real Postgres (M26 exit criteria, D-Vehicles /
// D-Audit). They exercise the vehicle module's audited catalog/vehicle CRUD, the duplicate-VIN guard,
// registration to a person in a plate region (the WOF geo_places gazetteer), the bad-region guard, the
// per-country active-plate uniqueness guard, transfer-as-history (close prior + open new), the brand→
// manufacturer link, and the person-purge erasure of person-owned registrations.
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/vehicle/...
package vehicle_test

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
	pdb "github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/adapters"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/application"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
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

func newService(t *testing.T, pool *pgxpool.Pool) *application.Service {
	t.Helper()
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	return application.NewService(pool, func(conn pdb.DBTX) domain.Repository {
		return adapters.NewRepository(conn)
	}, audit)
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func uaCountryID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM oikumenea.geo_countries WHERE code = 'UA'").Scan(&id); err != nil {
		t.Fatalf("resolve UA: %v", err)
	}
	return id
}

func catalogID(t *testing.T, pool *pgxpool.Pool, table, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), "SELECT id FROM oikumenea."+table+" WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("resolve %s %s: %v", table, code, err)
	}
	return id
}

// seedPlace inserts a WOF gazetteer place (region/county/…) and returns its RID.
func seedPlace(t *testing.T, pool *pgxpool.Pool, placetype, countryID, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.geo_places (wof_id, placetype, country_id, name)
		 VALUES ($1, $2, $3, $4) RETURNING id`, time.Now().UnixNano(), placetype, countryID, name).Scan(&id); err != nil {
		t.Fatalf("seed place: %v", err)
	}
	return id
}

func seedPerson(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.person_persons (display_name) VALUES ('Owner') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return id
}

func seedCompany(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.company_companies (code, legal_name, legal_form_id)
		 SELECT $1, 'Acme Motors', id FROM oikumenea.company_legal_forms WHERE code = 'llc' LIMIT 1
		 RETURNING id`, uniq("co")).Scan(&id); err != nil {
		t.Fatalf("seed company: %v", err)
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

func ptr(s string) *string { return &s }

// TestVehicleVertical drives the whole M26 exit-criteria slice in one ordered scenario.
func TestVehicleVertical(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	ctx := context.Background()

	ua := uaCountryID(t, pool)
	region := seedPlace(t, pool, "region", ua, "Kyiv Oblast")
	county := seedPlace(t, pool, "county", ua, "Kyiv-Sviatoshyn Raion") // a non-region place
	carType := catalogID(t, pool, "vehicle_types", "car")
	regular := catalogID(t, pool, "vehicle_registration_number_types", "regular")
	person := seedPerson(t, pool)
	company := seedCompany(t, pool)

	// --- 1. brand + model catalog (created through the service, audited) ---
	brand, err := svc.UpsertBrand(ctx, uniq("toyota"), "Toyota", ptr(ua), nil)
	if err != nil {
		t.Fatalf("upsert brand: %v", err)
	}
	assertOneAction(t, pool, brand.ID, "vehicle.brand.upsert")
	model, err := svc.UpsertModel(ctx, brand.ID, uniq("corolla"), "Corolla", ptr("E210"), ptr("2018-01-01"), nil, nil)
	if err != nil {
		t.Fatalf("upsert model: %v", err)
	}
	if model.BrandID != brand.ID {
		t.Fatalf("model brand mismatch")
	}

	// --- 2. create a vehicle with a VIN under a brand/model/type ---
	vin := uniq("VIN1")
	plate := uniq("AA")
	v, err := svc.CreateVehicle(ctx, domain.VehicleInput{TypeID: carType, ModelID: model.ID, VIN: vin, Color: "blue"})
	if err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	assertOneAction(t, pool, v.ID, "vehicle.create")
	if v.BrandID != brand.ID {
		t.Fatalf("expected derived brand %s, got %q", brand.ID, v.BrandID)
	}

	// --- 3. duplicate VIN → conflict ---
	if _, err := svc.CreateVehicle(ctx, domain.VehicleInput{TypeID: carType, VIN: vin}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate VIN, got %v", err)
	}

	// --- 4. register to a PERSON in a plate region ---
	reg, err := svc.RegisterVehicle(ctx, v.ID, domain.RegistrationInput{
		OwnerKind: domain.OwnerPerson, OwnerID: person, CountryID: ua,
		SubdivisionID: region, RegistrationNumber: plate, NumberTypeID: regular,
	})
	if err != nil {
		t.Fatalf("register to person: %v", err)
	}
	assertOneAction(t, pool, reg.ID, "vehicle.register")
	if reg.Status != "active" {
		t.Fatalf("expected active registration, got %q", reg.Status)
	}

	// --- 5. a non-region plate region → RegionInvalid ---
	if _, err := svc.RegisterVehicle(ctx, v.ID, domain.RegistrationInput{
		OwnerKind: domain.OwnerPerson, OwnerID: person, CountryID: ua,
		SubdivisionID: county, RegistrationNumber: "BAD0000X",
	}); !errors.Is(err, domain.ErrRegionInvalid) {
		t.Fatalf("expected ErrRegionInvalid for a county region, got %v", err)
	}

	// --- 6. duplicate active plate per country → conflict (a second vehicle, same plate) ---
	v2, err := svc.CreateVehicle(ctx, domain.VehicleInput{TypeID: carType, VIN: uniq("VIN2")})
	if err != nil {
		t.Fatalf("create vehicle 2: %v", err)
	}
	if _, err := svc.RegisterVehicle(ctx, v2.ID, domain.RegistrationInput{
		OwnerKind: domain.OwnerCompany, OwnerID: company, CountryID: ua, RegistrationNumber: plate,
	}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate active plate, got %v", err)
	}

	// --- 7. transfer the first vehicle to the COMPANY (new active row, prior closed) ---
	reg2, err := svc.RegisterVehicle(ctx, v.ID, domain.RegistrationInput{
		OwnerKind: domain.OwnerCompany, OwnerID: company, CountryID: ua,
		SubdivisionID: region, RegistrationNumber: plate, NumberTypeID: regular,
	})
	if err != nil {
		t.Fatalf("transfer to company: %v", err)
	}
	regs, err := svc.ListRegistrationsByVehicle(ctx, v.ID)
	if err != nil {
		t.Fatalf("list registrations: %v", err)
	}
	if len(regs) != 2 {
		t.Fatalf("expected 2 registration rows (history), got %d", len(regs))
	}
	active := 0
	for _, r := range regs {
		if r.Status == "active" {
			active++
			if r.ID != reg2.ID || r.OwnerKind != domain.OwnerCompany {
				t.Fatalf("expected the company registration to be the active one")
			}
		}
	}
	if active != 1 {
		t.Fatalf("expected exactly 1 active registration after transfer, got %d", active)
	}

	// --- 8. brand → manufacturer link (who makes the marque) ---
	man, err := svc.AddManufacturer(ctx, brand.ID, domain.ManufacturerInput{CompanyID: company, EffectiveFrom: "1990-01-01"})
	if err != nil {
		t.Fatalf("add manufacturer: %v", err)
	}
	mans, err := svc.ListManufacturersByBrand(ctx, brand.ID)
	if err != nil {
		t.Fatalf("list manufacturers: %v", err)
	}
	if len(mans) != 1 || mans[0].ID != man.ID || mans[0].CompanyID != company {
		t.Fatalf("expected the company as the brand manufacturer")
	}

	// --- 9. person view: the person still has its (now-closed) historical registration ---
	pv, err := svc.ListPersonVehicles(ctx, person)
	if err != nil {
		t.Fatalf("list person vehicles: %v", err)
	}
	if len(pv) != 1 {
		t.Fatalf("expected 1 person-owned registration (historical), got %d", len(pv))
	}

	// --- 10. purge erasure: a person purge erases person-owned registrations ---
	if countRows(t, pool, "SELECT count(*) FROM oikumenea.vehicle_registrations WHERE owner_kind='person' AND owner_id=$1 AND deleted_at IS NULL", person) != 1 {
		t.Fatalf("precondition: person should have 1 live owned registration")
	}
	n, err := svc.ErasePersonRegistrations(ctx, person)
	if err != nil {
		t.Fatalf("erase person registrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 erased registration, got %d", n)
	}
	if got := countRows(t, pool, "SELECT count(*) FROM oikumenea.vehicle_registrations WHERE owner_kind='person' AND owner_id=$1 AND deleted_at IS NULL", person); got != 0 {
		t.Fatalf("expected person-owned registrations erased on purge, got %d live", got)
	}
	pv2, err := svc.ListPersonVehicles(ctx, person)
	if err != nil {
		t.Fatalf("list person vehicles after purge: %v", err)
	}
	if len(pv2) != 0 {
		t.Fatalf("expected 0 person vehicles after erasure, got %d", len(pv2))
	}
}
