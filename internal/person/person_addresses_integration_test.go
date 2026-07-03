//go:build integration

// Integration tests for the M32 person-addresses slice (D-PersonAddresses) against a real Postgres.
// Proves the exit criteria:
//
//   - add a home address referencing a shared location_locations row (M19), then list it;
//   - add a second (work) primary address and confirm the prior primary is demoted (one primary/person);
//   - an unknown location_id is rejected (ErrUnknownLocation) via the LocationLookup seam;
//   - privacy_seeking round-trips;
//   - purge HARD-DELETES the address rows (pii:contact).
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
)

// dbLocations is a test LocationLookup that verifies a location RID against the DB — faithful to the
// real geo service's GetLocation-backed LocationExists, without wiring the whole geo module.
type dbLocations struct{ pool *pgxpool.Pool }

func (d dbLocations) LocationExists(ctx context.Context, id string) error {
	var x string
	if err := d.pool.QueryRow(ctx,
		`SELECT id FROM oikumenea.location_locations WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&x); err != nil {
		return domain.ErrUnknownLocation
	}
	return nil
}

// seedLocation inserts a location_locations row (PostGIS point over a seeded country) and returns its RID.
func seedLocation(t *testing.T, pool *pgxpool.Pool, lng, lat float64) string {
	t.Helper()
	uaID := countryRID(t, pool, "UA")
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.location_locations (geom, country_id, locality)
		 VALUES (ST_SetSRID(ST_MakePoint($1,$2),4326)::geography, $3, 'Kyiv') RETURNING id`,
		lng, lat, uaID).Scan(&id); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	return id
}

func TestPersonAddresses(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)
	svc.SetLocationLookup(dbLocations{pool})
	p := newPerson(t, svc, "Mykola Address")

	loc1 := seedLocation(t, pool, 30.5234, 50.4501)
	loc2 := seedLocation(t, pool, 24.0297, 49.8397)

	// Add a primary home address.
	home, err := svc.UpsertAddress(ctx, domain.Address{
		PersonID: p.ID, LocationID: loc1, Role: "home", IsPrimary: true, PrivacySeeking: true,
	})
	if err != nil {
		t.Fatalf("add home: %v", err)
	}
	if !home.IsPrimary || !home.PrivacySeeking || home.ValidFrom == "" {
		t.Fatalf("home round-trip mismatch: %+v", home)
	}

	// A second primary (work) address must demote the prior primary — one active primary per person.
	work, err := svc.UpsertAddress(ctx, domain.Address{
		PersonID: p.ID, LocationID: loc2, Role: "work", IsPrimary: true,
	})
	if err != nil {
		t.Fatalf("add work: %v", err)
	}
	list, err := svc.ListAddresses(ctx, p.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list addresses mismatch: %+v err=%v", list, err)
	}
	var primaries int
	for _, a := range list {
		if a.IsPrimary {
			primaries++
			if a.ID != work.ID {
				t.Fatalf("primary should be the work address, got %s", a.ID)
			}
		}
	}
	if primaries != 1 {
		t.Fatalf("expected exactly one primary after demotion, got %d", primaries)
	}

	// An invalid role is rejected by the domain validator.
	if _, err := svc.UpsertAddress(ctx, domain.Address{PersonID: p.ID, LocationID: loc1, Role: "bogus"}); err == nil {
		t.Fatal("expected invalid-role error")
	}

	// An unknown location is rejected before write via the LocationLookup seam.
	if _, err := svc.UpsertAddress(ctx, domain.Address{
		PersonID: p.ID, LocationID: countryRID(t, pool, "UA"), Role: "home", // a non-location RID
	}); !errors.Is(err, domain.ErrUnknownLocation) {
		t.Fatalf("expected ErrUnknownLocation for an unknown location, got %v", err)
	}

	// Delete the home address by its RID; the work address remains.
	if err := svc.DeleteAddress(ctx, p.ID, home.ID); err != nil {
		t.Fatalf("delete home: %v", err)
	}
	if list, _ = svc.ListAddresses(ctx, p.ID); len(list) != 1 || list[0].ID != work.ID {
		t.Fatalf("after delete expected only the work address, got %+v", list)
	}

	// Purge HARD-DELETES the address rows (pii:contact). A zero-grace service purges immediately.
	svcNow, _ := newService(t, 0)
	if _, err := svcNow.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcNow.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM oikumenea.person_addresses WHERE person_id = $1`, p.ID).Scan(&n); err != nil {
		t.Fatalf("post-purge count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected all address rows hard-deleted on purge, got %d", n)
	}
}
