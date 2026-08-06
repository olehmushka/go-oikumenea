// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Integration tests for the person module against a real Postgres (M5 exit criteria, D-PersonGlobal /
// D-PersonNamesCLDR / D-Geo / D-PersonReadScope / D-Audit):
//   - a person is created with no account and no unit, and reads back with its children;
//   - the optional external code is unique among active persons;
//   - a person may hold several citizenships (one active per country) with a single primary;
//   - rank is set/cleared; an unknown rank/country/locale is rejected via the DB FKs;
//   - name variants are unique per (person, locale);
//   - deactivate -> reactivate is reversible; purge before the grace window is refused, and after it
//     NULLs the PII while keeping the id tombstone;
//   - a create write + its audit row share one transaction.
//
// Run against a throwaway DB that has the migrations applied:
//
//	OIKUMENEA_TEST_DSN="postgres://postgres:dev@localhost:5432/oikumenea_test?sslmode=disable" \
//	  go test -tags integration ./internal/person/...
package person_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	auditadapters "github.com/olehmushka/go-oikumenea/internal/audit/adapters"
	auditapp "github.com/olehmushka/go-oikumenea/internal/audit/application"
	auditdomain "github.com/olehmushka/go-oikumenea/internal/audit/domain"
	"github.com/olehmushka/go-oikumenea/internal/person/adapters"
	"github.com/olehmushka/go-oikumenea/internal/person/application"
	"github.com/olehmushka/go-oikumenea/internal/person/domain"
	personevents "github.com/olehmushka/go-oikumenea/internal/person/events"
	profileadapters "github.com/olehmushka/go-oikumenea/internal/personprofile/adapters"
	profileapp "github.com/olehmushka/go-oikumenea/internal/personprofile/application"
	sensitiveadapters "github.com/olehmushka/go-oikumenea/internal/personsensitive/adapters"
	sensitiveapp "github.com/olehmushka/go-oikumenea/internal/personsensitive/application"
	platformcatalog "github.com/olehmushka/go-oikumenea/internal/platform/catalog"
	pdb "github.com/olehmushka/go-oikumenea/internal/platform/db"
	"github.com/olehmushka/go-oikumenea/pkg/crypto"
	"github.com/olehmushka/go-oikumenea/pkg/events"
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

// newService builds the person CORE application service directly (bypassing Register) with a fixed purge
// grace window in hours. (The envelope-encrypted / crypto surface moved to personsensitive under R-09 —
// use newSensitive for tests that exercise physical identity, ethnicity, overlays, watchlist or party.)
func newService(t *testing.T, graceHours int) (*application.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, func() int { return graceHours })
	return svc, pool
}

// newServices builds the person core service, the personprofile service, AND the personsensitive service
// over one shared pool/audit, for R-09 split integration tests: create a person via the core svc, then
// exercise person-owned directory data via the profile svc and physical identity / ethnicity / overlays /
// watchlist / party membership via the sensitive svc. Unused services are blanked at the call site.
func newServices(t *testing.T, graceHours int) (*application.Service, *profileapp.Service, *sensitiveapp.Service, *pgxpool.Pool) {
	t.Helper()
	pool := newPool(t)
	audit := auditapp.NewService(pool, func(conn pdb.DBTX) auditdomain.Repository {
		return auditadapters.NewRepository(conn)
	}, func() int { return 50 })
	repoFor := func(conn pdb.DBTX) domain.Repository { return adapters.NewRepository(conn) }
	profRepoFor := func(conn pdb.DBTX) domain.ProfileRepository { return profileadapters.NewRepository(conn) }
	sensRepoFor := func(conn pdb.DBTX) domain.SensitiveRepository { return sensitiveadapters.NewRepository(conn) }
	svc := application.NewService(pool, repoFor, audit, func() int { return graceHours })
	prof := profileapp.NewService(pool, profRepoFor, audit)
	sens := sensitiveapp.NewService(pool, sensRepoFor, audit, testCipher(t))
	// D-Color: wire the platform color catalog so eye/hair hard-FK palette checks run in tests.
	sens.SetColorLookup(platformcatalog.NewColorService(pool, audit))
	// PEP snapshot seam (R-09 split): watchlist screening reads the flag from the profile service.
	sens.SetPEPStatusReader(prof)
	// R-09 split: PurgePerson publishes PersonPurged and the profile/sensitive modules erase their own
	// rows in the purge transaction. Auto-wire that bus here so purge tests exercise real erasure without
	// per-test boilerplate (a test that needs its own bus just calls SubscribeOrderEvents again).
	purgeBus := events.NewBus()
	svc.SubscribeOrderEvents(purgeBus)
	prof.SubscribePersonPurge(purgeBus)
	sens.SubscribePersonPurge(purgeBus)
	return svc, prof, sens, pool
}

// testCipher builds a local-dev envelope cipher for the pii:special declared ethnicity (D-PhysicalIdentity).
func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	provider, err := crypto.NewLocalDevProvider(make([]byte, 32))
	if err != nil {
		t.Fatalf("local-dev key provider: %v", err)
	}
	cipher, err := crypto.NewCipher(provider, []byte("integration-blind-index-key"), 0)
	if err != nil {
		t.Fatalf("build cipher: %v", err)
	}
	return cipher
}

func code(t *testing.T, prefix string) string {
	t.Helper()
	return prefix + "-" + uuid.NewString()[:8]
}

// seedRank inserts a fresh system -> category -> type -> rank chain and returns the rank RID.
func seedRank(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	_, rankID := seedRankSystem(t, pool)
	return rankID
}

// seedRankSystem inserts a fresh rank system with one category -> type -> rank chain and returns
// (systemID, rankID) — used by the one-rank-per-system tests (D-Rank).
func seedRankSystem(t *testing.T, pool *pgxpool.Pool) (string, string) {
	t.Helper()
	ctx := context.Background()
	var sysID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.rank_systems (code, name, sort_order) VALUES ($1, 'Sys', 0) RETURNING id`,
		code(t, "sys")).Scan(&sysID); err != nil {
		t.Fatalf("seed system: %v", err)
	}
	return sysID, seedRankInSystem(t, pool, sysID)
}

// seedRankInSystem adds another category -> type -> rank chain under an existing rank system and
// returns the new rank RID — used to test the same-system replace / concurrent-system cases.
func seedRankInSystem(t *testing.T, pool *pgxpool.Pool, sysID string) string {
	t.Helper()
	ctx := context.Background()
	var catID, typeID, rankID string
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

func newPerson(t *testing.T, svc *application.Service, display string) domain.Person {
	t.Helper()
	p, err := svc.CreatePerson(context.Background(), domain.Person{Name: domain.Name{DisplayName: display}})
	if err != nil {
		t.Fatalf("create person: %v", err)
	}
	return p
}

// countryRID resolves a seeded ISO-3166-1 alpha-2 code to its geo_countries RID. Countries are
// RID-keyed (F-014); person references them by RID, so tests resolve the code at setup.
func countryRID(t *testing.T, pool *pgxpool.Pool, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		"SELECT id FROM oikumenea.geo_countries WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatalf("country %s: %v", code, err)
	}
	return id
}

// unknownCountryRID mints a syntactically valid but nonexistent country RID, so a write trips the
// geo FK (ErrUnknownCountry) rather than failing shape validation.
func unknownCountryRID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), "SELECT oikumenea.new_id(12,1,1)").Scan(&id); err != nil {
		t.Fatalf("mint unknown country rid: %v", err)
	}
	return id
}

// TestCreateAndReadAccountless creates a person with no account/unit and reads it back.
func TestCreateAndReadAccountless(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)

	created, err := svc.CreatePerson(ctx, domain.Person{
		Name:        domain.Name{DisplayName: "Тарас Григорович Шевченко", Given: "Тарас", Given2: "Григорович", Surname: "Шевченко"},
		Birthdate:   "1990-05-02",
		DateOfDeath: "2024-01-15",
		Sex:         "male",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != domain.StatusActive {
		t.Fatalf("status = %q, want active", created.Status)
	}
	got, err := svc.GetPerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Given2 != "Григорович" {
		t.Fatalf("given2 = %q, want the по-батькові", got.Given2)
	}
	if got.Sex != "male" || got.Birthdate != "1990-05-02" || got.DateOfDeath != "2024-01-15" {
		t.Fatalf("bio not round-tripped: sex=%q birthdate=%q date_of_death=%q", got.Sex, got.Birthdate, got.DateOfDeath)
	}
}

// TestCodeUniqueAmongActive rejects a duplicate external code.
func TestCodeUniqueAmongActive(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)

	c := code(t, "svc")
	if _, err := svc.CreatePerson(ctx, domain.Person{Code: c, Name: domain.Name{DisplayName: "First"}}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := svc.CreatePerson(ctx, domain.Person{Code: c, Name: domain.Name{DisplayName: "Second"}})
	if !errors.Is(err, domain.ErrCodeConflict) {
		t.Fatalf("want ErrCodeConflict, got %v", err)
	}
}

// rankInSystem returns the rank a person holds in systemID, or "" if none (test helper).
func rankInSystem(ranks []domain.PersonRank, systemID string) string {
	for _, r := range ranks {
		if r.SystemID == systemID {
			return r.RankID
		}
	}
	return ""
}

// TestRankAssignment exercises one rank per rank system (D-Rank): set in one system, a concurrent
// rank in a second system, a same-system replace, a per-system clear, and an unknown rank.
func TestRankAssignment(t *testing.T) {
	ctx := context.Background()
	svc, pool := newService(t, 720)
	p := newPerson(t, svc, "Ranked Person")
	sys1, rank1 := seedRankSystem(t, pool)
	sys2, rank2 := seedRankSystem(t, pool)

	// Set a rank in system 1 (the system is derived from the rank).
	if _, err := svc.SetPersonRank(ctx, p.ID, "", &rank1); err != nil {
		t.Fatalf("set rank1: %v", err)
	}
	got, _ := svc.GetPerson(ctx, p.ID)
	if len(got.Ranks) != 1 || rankInSystem(got.Ranks, sys1) != rank1 {
		t.Fatalf("after set rank1: ranks = %+v, want one {%s:%s}", got.Ranks, sys1, rank1)
	}

	// A concurrent rank in system 2 — both persist (the multi-track case).
	if _, err := svc.SetPersonRank(ctx, p.ID, "", &rank2); err != nil {
		t.Fatalf("set rank2: %v", err)
	}
	got, _ = svc.GetPerson(ctx, p.ID)
	if len(got.Ranks) != 2 || rankInSystem(got.Ranks, sys1) != rank1 || rankInSystem(got.Ranks, sys2) != rank2 {
		t.Fatalf("after set rank2: ranks = %+v, want two systems", got.Ranks)
	}

	// A second rank in system 1 REPLACES (one rank per system) — still two ranks total.
	rank1b := seedRankInSystem(t, pool, sys1)
	if _, err := svc.SetPersonRank(ctx, p.ID, "", &rank1b); err != nil {
		t.Fatalf("replace rank1: %v", err)
	}
	got, _ = svc.GetPerson(ctx, p.ID)
	if len(got.Ranks) != 2 || rankInSystem(got.Ranks, sys1) != rank1b {
		t.Fatalf("after replace: ranks = %+v, want sys1 -> %s", got.Ranks, rank1b)
	}

	// Clear system 1 — system 2's rank remains.
	if _, err := svc.SetPersonRank(ctx, p.ID, sys1, nil); err != nil {
		t.Fatalf("clear sys1: %v", err)
	}
	got, _ = svc.GetPerson(ctx, p.ID)
	if len(got.Ranks) != 1 || rankInSystem(got.Ranks, sys2) != rank2 {
		t.Fatalf("after clear sys1: ranks = %+v, want only sys2", got.Ranks)
	}

	// Unknown rank → ErrUnknownRank.
	bogus := uuid.NewString()
	if _, err := svc.SetPersonRank(ctx, p.ID, "", &bogus); !errors.Is(err, domain.ErrUnknownRank) {
		t.Fatalf("unknown rank: want ErrUnknownRank, got %v", err)
	}
}

// TestCitizenships holds several citizenships with one active per country and a single primary.
func TestCitizenships(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 720)
	ua, pl := countryRID(t, pool, "UA"), countryRID(t, pool, "PL")
	p := newPerson(t, svc, "Multi National")

	if _, err := prof.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: ua, Basis: "birth", IsPrimary: true}); err != nil {
		t.Fatalf("add UA: %v", err)
	}
	if _, err := prof.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: pl, Basis: "naturalization", IsPrimary: true}); err != nil {
		t.Fatalf("add PL: %v", err)
	}
	cs, err := prof.ListCitizenships(ctx, p.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("citizenships = %d, want 2", len(cs))
	}
	primaries := 0
	for _, c := range cs {
		if c.IsPrimary {
			primaries++
		}
	}
	if primaries != 1 {
		t.Fatalf("primary citizenships = %d, want exactly 1", primaries)
	}
	// re-upsert UA: still one active UA row (no duplicate).
	if _, err := prof.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: ua, Basis: "birth"}); err != nil {
		t.Fatalf("re-upsert UA: %v", err)
	}
	if cs, _ := prof.ListCitizenships(ctx, p.ID); len(cs) != 2 {
		t.Fatalf("after re-upsert, citizenships = %d, want 2", len(cs))
	}
	// unknown country.
	if _, err := prof.UpsertCitizenship(ctx, domain.Citizenship{PersonID: p.ID, Country: unknownCountryRID(t, pool), Basis: "other"}); !errors.Is(err, domain.ErrUnknownCountry) {
		t.Fatalf("unknown country: want ErrUnknownCountry, got %v", err)
	}
	// remove PL.
	if err := prof.DeleteCitizenship(ctx, p.ID, pl); err != nil {
		t.Fatalf("delete PL: %v", err)
	}
	if cs, _ := prof.ListCitizenships(ctx, p.ID); len(cs) != 1 {
		t.Fatalf("after delete, citizenships = %d, want 1", len(cs))
	}
}

// TestNameVariants are unique per (person, locale); an unknown locale is rejected.
func TestNameVariants(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Тарас Шевченко")

	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "John Doe"}, IsPrimary: true}); err != nil {
		t.Fatalf("add eng: %v", err)
	}
	// re-upsert eng updates in place (no duplicate, no conflict).
	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "eng", Name: domain.Name{DisplayName: "John V. Doe"}}); err != nil {
		t.Fatalf("re-upsert eng: %v", err)
	}
	vs, err := svc.ListNameVariants(ctx, p.ID)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(vs) != 1 || vs[0].DisplayName != "John V. Doe" {
		t.Fatalf("variants = %+v, want one updated eng variant", vs)
	}
	// unknown locale.
	if _, err := svc.UpsertNameVariant(ctx, domain.NameVariant{PersonID: p.ID, Locale: "zzz", Name: domain.Name{DisplayName: "x"}}); !errors.Is(err, domain.ErrUnknownLocale) {
		t.Fatalf("unknown locale: want ErrUnknownLocale, got %v", err)
	}
}

// TestResidences adds and replaces a residence row, rejecting an unknown country.
func TestResidences(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 720)
	pl := countryRID(t, pool, "PL")
	p := newPerson(t, svc, "Resident")

	created, err := prof.UpsertResidence(ctx, domain.Residence{PersonID: p.ID, Country: pl, Region: "Mazowieckie", ValidFrom: "2021-09-01"})
	if err != nil {
		t.Fatalf("add residence: %v", err)
	}
	// replace by id.
	if _, err := prof.UpsertResidence(ctx, domain.Residence{ID: created.ID, PersonID: p.ID, Country: pl, Region: "Krakow", ValidFrom: "2021-09-01", ValidTo: "2023-01-01"}); err != nil {
		t.Fatalf("replace residence: %v", err)
	}
	rs, _ := prof.ListResidences(ctx, p.ID)
	if len(rs) != 1 || rs[0].Region != "Krakow" || rs[0].ValidTo != "2023-01-01" {
		t.Fatalf("residences = %+v, want one replaced row", rs)
	}
	if _, err := prof.UpsertResidence(ctx, domain.Residence{PersonID: p.ID, Country: unknownCountryRID(t, pool), ValidFrom: "2020-01-01"}); !errors.Is(err, domain.ErrUnknownCountry) {
		t.Fatalf("unknown country: want ErrUnknownCountry, got %v", err)
	}
}

// TestLifecycleReversible deactivates then reactivates within the grace window.
func TestLifecycleReversible(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t, 720)
	p := newPerson(t, svc, "Reversible")

	d, err := svc.DeactivatePerson(ctx, p.ID, "leave")
	if err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if d.Status != domain.StatusDeactivated || d.PurgeAfter == nil {
		t.Fatalf("deactivate state = %+v, want deactivated with purge_after", d)
	}
	r, err := svc.ReactivatePerson(ctx, p.ID)
	if err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if r.Status != domain.StatusActive || r.PurgeAfter != nil {
		t.Fatalf("reactivate state = %+v, want active with no purge_after", r)
	}
	// reactivating an active person is rejected.
	if _, err := svc.ReactivatePerson(ctx, p.ID); !errors.Is(err, domain.ErrLifecycle) {
		t.Fatalf("reactivate active: want ErrLifecycle, got %v", err)
	}
}

// TestPurgeGate refuses purge before the grace window; after it, PII is NULLed and the id remains.
func TestPurgeGate(t *testing.T) {
	ctx := context.Background()

	// Long grace: purge is refused immediately after deactivation.
	svcLong, _ := newService(t, 720)
	refused := newPerson(t, svcLong, "To Be Refused").ID
	if _, err := svcLong.DeactivatePerson(ctx, refused, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svcLong.PurgePerson(ctx, refused); !errors.Is(err, domain.ErrLifecycle) {
		t.Fatalf("purge before grace: want ErrLifecycle, got %v", err)
	}

	// Zero grace: purge is allowed and erases PII.
	svcNow, profNow, _, poolNow := newServices(t, 0)
	created, err := svcNow.CreatePerson(ctx, domain.Person{
		Code:        code(t, "purge"),
		Name:        domain.Name{DisplayName: "Erase Me", Given: "Erase", Surname: "Me"},
		Sex:         "female",
		Birthdate:   "1980-03-04",
		DateOfDeath: "2024-02-02",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := profNow.UpsertCitizenship(ctx, domain.Citizenship{PersonID: created.ID, Country: countryRID(t, poolNow, "UA"), Basis: "birth"}); err != nil {
		t.Fatalf("add citizenship: %v", err)
	}
	if _, err := svcNow.DeactivatePerson(ctx, created.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	purged, err := svcNow.PurgePerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged.Status != domain.StatusPurged || purged.DisplayName != "" || purged.Given != "" || purged.Code != "" {
		t.Fatalf("purge did not erase PII: %+v", purged)
	}
	if purged.Birthdate != "" || purged.DateOfDeath != "" {
		t.Fatalf("purge did not NULL bio dates: birthdate=%q date_of_death=%q", purged.Birthdate, purged.DateOfDeath)
	}
	// the id tombstone remains queryable, citizenship rows are gone.
	got, err := svcNow.GetPerson(ctx, created.ID)
	if err != nil {
		t.Fatalf("tombstone get: %v", err)
	}
	if got.Status != domain.StatusPurged {
		t.Fatalf("tombstone = %+v, want purged", got)
	}
	// purge is idempotent.
	if _, err := svcNow.PurgePerson(ctx, created.ID); err != nil {
		t.Fatalf("idempotent purge: %v", err)
	}
	var n int
	if err := poolNow.QueryRow(ctx, "SELECT count(*) FROM oikumenea.person_citizenships WHERE person_id = $1", created.ID).Scan(&n); err != nil {
		t.Fatalf("count citizenships: %v", err)
	}
	if n != 0 {
		t.Fatalf("citizenship rows after purge = %d, want 0", n)
	}
}

// TestContactChannels exercises emails/phones/call signs (D-PersonContactChannels): provider/country
// are derived on write, validation rejects bad input, and a purge erases every channel row.
func TestContactChannels(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 0)

	p := newPerson(t, svc, "Contactable Person")

	// Email: provider derived from the domain; primary flag honored.
	email, err := prof.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "personal", Address: "Person@Gmail.com", IsPrimary: true})
	if err != nil {
		t.Fatalf("upsert email: %v", err)
	}
	if email.Address != "person@gmail.com" || email.Provider != "google" || !email.IsPrimary {
		t.Fatalf("email not normalized/derived: %+v", email)
	}
	// Duplicate active address is a conflict.
	if _, err := prof.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "work", Address: "person@gmail.com"}); !errors.Is(err, domain.ErrEmailConflict) {
		t.Fatalf("duplicate email: want ErrEmailConflict, got %v", err)
	}
	// Unknown type code is rejected (FK).
	if _, err := prof.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "nope", Address: "x@y.com"}); !errors.Is(err, domain.ErrUnknownContactType) {
		t.Fatalf("unknown email type: want ErrUnknownContactType, got %v", err)
	}
	// Malformed address is rejected before the DB.
	if _, err := prof.UpsertEmail(ctx, domain.Email{PersonID: p.ID, TypeCode: "personal", Address: "not-an-email"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad email: want ErrInvalid, got %v", err)
	}

	// Phone: E.164-normalized + country derived.
	phone, err := prof.UpsertPhone(ctx, domain.Phone{PersonID: p.ID, TypeCode: "mobile", Number: "+380 (44) 123-45-67"})
	if err != nil {
		t.Fatalf("upsert phone: %v", err)
	}
	if phone.Number != "+380441234567" || phone.Country != countryRID(t, pool, "UA") {
		t.Fatalf("phone not normalized/derived: %+v", phone)
	}
	if _, err := prof.UpsertPhone(ctx, domain.Phone{PersonID: p.ID, TypeCode: "mobile", Number: "garbage"}); !errors.Is(err, domain.ErrUnparseablePhone) {
		t.Fatalf("bad phone: want ErrUnparseablePhone, got %v", err)
	}

	// Call sign: required value, unique per person among active.
	if _, err := prof.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Сокіл", IsPrimary: true}); err != nil {
		t.Fatalf("upsert call sign: %v", err)
	}
	if _, err := prof.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Беркут"}); err != nil {
		t.Fatalf("second distinct call sign: %v", err)
	}
	// Duplicate value for the same person is a conflict.
	if _, err := prof.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: "Сокіл"}); !errors.Is(err, domain.ErrCallSignConflict) {
		t.Fatalf("duplicate call sign: want ErrCallSignConflict, got %v", err)
	}
	// An empty call sign is rejected (NOT NULL).
	if _, err := prof.UpsertCallSign(ctx, domain.CallSign{PersonID: p.ID, CallSign: ""}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty call sign: want ErrInvalid, got %v", err)
	}

	// The contact channels are personprofile-owned now (R-09); read them via the profile service (the
	// transport composes them onto GetPerson — exercised by the transport tests).
	emails, err := prof.ListEmails(ctx, p.ID)
	if err != nil {
		t.Fatalf("list emails: %v", err)
	}
	phones, err := prof.ListPhones(ctx, p.ID)
	if err != nil {
		t.Fatalf("list phones: %v", err)
	}
	callSigns, err := prof.ListCallSigns(ctx, p.ID)
	if err != nil {
		t.Fatalf("list call signs: %v", err)
	}
	if len(emails) != 1 || len(phones) != 1 || len(callSigns) != 2 {
		t.Fatalf("channels: emails=%d phones=%d callSigns=%d", len(emails), len(phones), len(callSigns))
	}

	// Purge erases every channel row, keeping the id tombstone (personprofile erases via PersonPurged,
	// auto-wired by newServices).
	if _, err := svc.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	for _, table := range []string{"person_emails", "person_phones", "person_call_signs"} {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM oikumenea."+table+" WHERE person_id = $1", p.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Fatalf("%s rows after purge = %d, want 0", table, n)
		}
	}
}

// TestContactTypeCatalogs reads the seeded email/phone-type catalogs.
func TestContactTypeCatalogs(t *testing.T) {
	ctx := context.Background()
	_, prof, _, _ := newServices(t, 720)
	ets, err := prof.ListEmailTypes(ctx)
	if err != nil || len(ets) == 0 {
		t.Fatalf("list email types: %d err %v", len(ets), err)
	}
	pts, err := prof.ListPhoneTypes(ctx)
	if err != nil || len(pts) == 0 {
		t.Fatalf("list phone types: %d err %v", len(pts), err)
	}
}

// TestSocialChannels exercises the M13 exit criteria (D-PersonSocialChannels): the platform catalog, a
// messenger link over an existing phone (messenger-only + holder-scope rules), a standalone social
// account with sourced/weighted attribution and a stable id, a handle rename recorded in history without
// breaking the link, and purge erasing all four tables.
func TestSocialChannels(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 0)

	p := newPerson(t, svc, "Reachable Person")
	other := newPerson(t, svc, "Other Person")

	// The platform catalog is seeded with both categories, including the M13 + 0026 additions.
	platforms, err := prof.ListPlatforms(ctx)
	if err != nil || len(platforms) == 0 {
		t.Fatalf("list platforms: %d err %v", len(platforms), err)
	}
	platformCodes := map[string]bool{}
	for _, pl := range platforms {
		platformCodes[pl.Code] = true
	}
	for _, want := range []string{"telegram", "threema", "milchat", "vkontakte", "odnoklassniki", "bluesky", "mastodon"} {
		if !platformCodes[want] {
			t.Fatalf("platform catalog missing %q; got %v", want, platformCodes)
		}
	}

	// A phone to be reachable on.
	phone, err := prof.UpsertPhone(ctx, domain.Phone{PersonID: p.ID, TypeCode: "mobile", Number: "+380441234567"})
	if err != nil {
		t.Fatalf("seed phone: %v", err)
	}
	otherPhone, err := prof.UpsertPhone(ctx, domain.Phone{PersonID: other.ID, TypeCode: "mobile", Number: "+380441111111"})
	if err != nil {
		t.Fatalf("seed other phone: %v", err)
	}

	// Messenger link over the phone on a messenger platform.
	link, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PhoneID: phone.ID, PlatformCode: "telegram", IsPrimary: true})
	if err != nil {
		t.Fatalf("upsert messenger link: %v", err)
	}
	if link.PhoneID != phone.ID || !link.IsPrimary {
		t.Fatalf("messenger link not stored: %+v", link)
	}
	// A non-messenger (social) platform is rejected.
	if _, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PhoneID: phone.ID, PlatformCode: "instagram"}); !errors.Is(err, domain.ErrPlatformNotMessenger) {
		t.Fatalf("social platform on messenger link: want ErrPlatformNotMessenger, got %v", err)
	}
	// An unknown platform is rejected.
	if _, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PhoneID: phone.ID, PlatformCode: "nope"}); !errors.Is(err, domain.ErrUnknownPlatform) {
		t.Fatalf("unknown platform: want ErrUnknownPlatform, got %v", err)
	}
	// A channel held by another person is rejected (holder scope).
	if _, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PhoneID: otherPhone.ID, PlatformCode: "signal"}); !errors.Is(err, domain.ErrChannelNotOwned) {
		t.Fatalf("not-owned channel: want ErrChannelNotOwned, got %v", err)
	}
	// Both / neither channel is invalid (XOR).
	if _, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PhoneID: phone.ID, EmailID: "x", PlatformCode: "telegram"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("both channels: want ErrInvalid, got %v", err)
	}
	if _, err := prof.UpsertMessengerLink(ctx, p.ID, domain.MessengerLink{PlatformCode: "telegram"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("no channel: want ErrInvalid, got %v", err)
	}

	// Social account with a stable id + sourced/weighted attribution; profile_url derived, confidence defaulted.
	acct, err := prof.UpsertSocialAccount(ctx, domain.SocialAccount{
		PersonID: p.ID, PlatformCode: "instagram", PlatformUserID: "17841400000000000",
		Handle: "@reachable", Source: "self_declared", IsPrimary: true,
	})
	if err != nil {
		t.Fatalf("upsert social account: %v", err)
	}
	if acct.Handle != "reachable" || acct.ProfileURL != "https://instagram.com/reachable" || acct.Confidence != "possible" {
		t.Fatalf("social account not normalized/derived/defaulted: %+v", acct)
	}
	// A bad source is rejected before the DB.
	if _, err := prof.UpsertSocialAccount(ctx, domain.SocialAccount{PersonID: p.ID, PlatformCode: "x", Handle: "h", Source: "bogus"}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad source: want ErrInvalid, got %v", err)
	}

	// Exactly one handle-history period after creation, and it is current.
	if got := countHandles(t, pool, ctx, acct.ID, true); got != 1 {
		t.Fatalf("handle history after create: current=%d, want 1", got)
	}

	// Rename the handle: the old period closes and a new current one opens — the link (id) is unchanged.
	renamed, err := prof.UpsertSocialAccount(ctx, domain.SocialAccount{
		ID: acct.ID, PersonID: p.ID, PlatformCode: "instagram", PlatformUserID: "17841400000000000",
		Handle: "renamed", Source: "self_declared",
	})
	if err != nil {
		t.Fatalf("rename social account: %v", err)
	}
	if renamed.ID != acct.ID || renamed.Handle != "renamed" {
		t.Fatalf("rename should keep id, change handle: %+v", renamed)
	}
	if total := countHandles(t, pool, ctx, acct.ID, false); total != 2 {
		t.Fatalf("handle history after rename: total=%d, want 2", total)
	}
	if cur := countHandles(t, pool, ctx, acct.ID, true); cur != 1 {
		t.Fatalf("handle history after rename: current=%d, want 1", cur)
	}
	handles, err := prof.ListSocialAccountHandles(ctx, p.ID, acct.ID)
	if err != nil || len(handles) != 2 {
		t.Fatalf("list handle history: %d err %v", len(handles), err)
	}

	// The social channels are personprofile-owned now (R-09); read them via the profile service.
	msgLinks, err := prof.ListMessengerLinks(ctx, p.ID)
	if err != nil {
		t.Fatalf("list messenger links: %v", err)
	}
	socials, err := prof.ListSocialAccounts(ctx, p.ID)
	if err != nil {
		t.Fatalf("list social accounts: %v", err)
	}
	if len(msgLinks) != 1 || len(socials) != 1 {
		t.Fatalf("social channels: links=%d accounts=%d", len(msgLinks), len(socials))
	}

	// Purge erases all four tables (the phone cascade also removes the link; social account cascades its
	// handles); personprofile erases via PersonPurged (R-09, auto-wired by newServices).
	if _, err := svc.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var links, accounts, histories int
	if err := pool.QueryRow(ctx, `SELECT
	  (SELECT count(*) FROM oikumenea.person_messenger_links ml LEFT JOIN oikumenea.person_phones ph ON ml.phone_id = ph.id LEFT JOIN oikumenea.person_emails em ON ml.email_id = em.id WHERE COALESCE(ph.person_id, em.person_id) = $1),
	  (SELECT count(*) FROM oikumenea.person_social_accounts WHERE person_id = $1),
	  (SELECT count(*) FROM oikumenea.person_social_account_handles WHERE account_id = $2)`,
		p.ID, acct.ID).Scan(&links, &accounts, &histories); err != nil {
		t.Fatalf("count after purge: %v", err)
	}
	if links != 0 || accounts != 0 || histories != 0 {
		t.Fatalf("rows after purge: links=%d accounts=%d histories=%d, want 0/0/0", links, accounts, histories)
	}
}

// countHandles counts a social account's handle-history rows; currentOnly restricts to the open period.
func countHandles(t *testing.T, pool *pgxpool.Pool, ctx context.Context, accountID string, currentOnly bool) int {
	t.Helper()
	q := "SELECT count(*) FROM oikumenea.person_social_account_handles WHERE account_id = $1 AND deleted_at IS NULL"
	if currentOnly {
		q += " AND valid_to IS NULL"
	}
	var n int
	if err := pool.QueryRow(ctx, q, accountID).Scan(&n); err != nil {
		t.Fatalf("count handles: %v", err)
	}
	return n
}

// TestCreateAuditsInOneTx confirms a create records exactly one audit row keyed to it.
func TestCreateAuditsInOneTx(t *testing.T) {
	ctx := context.Background()
	svc, p := newService(t, 720)
	created := newPerson(t, svc, "Audited")

	var n int
	if err := p.QueryRow(ctx,
		"SELECT count(*) FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'person.create' AND actor_type = 'system' AND subsystem = 'person-admin'",
		created.ID,
	).Scan(&n); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows for %s = %d, want 1", created.ID, n)
	}
	// audit payload carries no PII (no display name).
	var payload string
	if err := p.QueryRow(ctx,
		"SELECT coalesce(after::text, '') FROM oikumenea.audit_log WHERE target_id = $1 AND action = 'person.create'",
		created.ID,
	).Scan(&payload); err != nil {
		t.Fatalf("query payload: %v", err)
	}
	if want := "Audited"; len(payload) > 0 && contains(payload, want) {
		t.Fatalf("audit payload %q must not contain the display name", payload)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestPersonRelationships exercises the M14 per-type reified self-links (D-PersonRelationships):
// canonical-pair partnerships with the single-active rule, directional kinship/sponsorship/guardianship
// via role, next-of-kin nomination, COI association, the polymorphic
// delete-by-id, and purge erasure on either endpoint.
func TestPersonRelationships(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, _ := newServices(t, 0)

	a := newPerson(t, svc, "Alice")
	b := newPerson(t, svc, "Bob")
	c := newPerson(t, svc, "Carol")

	if rts, err := prof.ListRelationTypes(ctx); err != nil || len(rts) == 0 {
		t.Fatalf("list relation types: %d err %v", len(rts), err)
	}

	// Partnership: canonical pair (a<b), single active per person, self/unknown rejects.
	part, err := prof.UpsertPartnership(ctx, a.ID, domain.Partnership{PersonIDA: a.ID, PersonIDB: b.ID, Status: "married"})
	if err != nil {
		t.Fatalf("partnership: %v", err)
	}
	lo, hi := a.ID, b.ID
	if lo > hi {
		lo, hi = hi, lo
	}
	if part.PersonIDA != lo || part.PersonIDB != hi {
		t.Fatalf("partnership not canonical: %+v", part)
	}
	if _, err := prof.UpsertPartnership(ctx, a.ID, domain.Partnership{PersonIDA: a.ID, PersonIDB: c.ID, Status: "engaged"}); !errors.Is(err, domain.ErrPartnershipConflict) {
		t.Fatalf("single active partnership: want ErrPartnershipConflict, got %v", err)
	}
	if _, err := prof.UpsertPartnership(ctx, a.ID, domain.Partnership{PersonIDA: a.ID, PersonIDB: a.ID, Status: "married"}); !errors.Is(err, domain.ErrSelfRelationship) {
		t.Fatalf("self partnership: want ErrSelfRelationship, got %v", err)
	}
	if _, err := prof.UpsertPartnership(ctx, a.ID, domain.Partnership{PersonIDA: a.ID, PersonIDB: "00000000-0000-8101-8601-000000000000", Status: "married"}); !errors.Is(err, domain.ErrUnknownCounterpart) {
		t.Fatalf("unknown counterpart: want ErrUnknownCounterpart, got %v", err)
	}

	// Kinship parent_of (a is parent of c).
	kin, err := prof.UpsertKinship(ctx, a.ID, domain.Kinship{ParentID: a.ID, ChildID: c.ID, Status: "active"})
	if err != nil {
		t.Fatalf("kinship: %v", err)
	}
	if kin.ParentID != a.ID || kin.ChildID != c.ID {
		t.Fatalf("kinship endpoints: %+v", kin)
	}

	// Sponsorship: a category-mismatched code is rejected; a sponsorship code succeeds.
	if _, err := prof.UpsertSponsorship(ctx, a.ID, domain.Sponsorship{SponsorID: a.ID, SponsoredID: c.ID, RelationCode: "spouse"}); !errors.Is(err, domain.ErrRelationCategory) {
		t.Fatalf("sponsorship wrong category: want ErrRelationCategory, got %v", err)
	}
	if _, err := prof.UpsertSponsorship(ctx, a.ID, domain.Sponsorship{SponsorID: a.ID, SponsoredID: c.ID, RelationCode: "godparent"}); err != nil {
		t.Fatalf("sponsorship: %v", err)
	}

	// Guardianship (a guardian of c).
	if _, err := prof.UpsertGuardianship(ctx, a.ID, domain.Guardianship{GuardianID: a.ID, WardID: c.ID}); err != nil {
		t.Fatalf("guardianship: %v", err)
	}

	// Next-of-kin nomination (a → b), default priority 1.
	nk, err := prof.UpsertNextOfKin(ctx, a.ID, domain.NextOfKin{SubjectID: a.ID, ContactID: b.ID, RelationCode: "spouse"})
	if err != nil {
		t.Fatalf("next of kin: %v", err)
	}
	if nk.Priority != 1 || nk.SubjectID != a.ID {
		t.Fatalf("next of kin: %+v", nk)
	}

	// Association (COI), symmetric.
	if _, err := prof.UpsertAssociation(ctx, a.ID, domain.Association{PersonIDA: a.ID, PersonIDB: c.ID, Kind: "coi"}); err != nil {
		t.Fatalf("association: %v", err)
	}

	// Lists touch either endpoint.
	if ps, err := prof.ListPartnerships(ctx, b.ID); err != nil || len(ps) != 1 {
		t.Fatalf("list partnerships for b: %d err %v", len(ps), err)
	}
	if ks, err := prof.ListKinships(ctx, c.ID); err != nil || len(ks) != 1 {
		t.Fatalf("list kinships for c: %d err %v", len(ks), err)
	}

	// Polymorphic delete-by-id (holder-scoped); idempotent re-delete; bad RID rejected.
	if err := prof.DeleteRelationship(ctx, a.ID, kin.ID); err != nil {
		t.Fatalf("delete kinship: %v", err)
	}
	if ks, err := prof.ListKinships(ctx, a.ID); err != nil || len(ks) != 0 {
		t.Fatalf("kinship after delete: %d err %v", len(ks), err)
	}
	if err := prof.DeleteRelationship(ctx, a.ID, kin.ID); !errors.Is(err, domain.ErrRelationshipNotFound) {
		t.Fatalf("re-delete: want ErrRelationshipNotFound, got %v", err)
	}
	if err := prof.DeleteRelationship(ctx, a.ID, "00000000-0000-8101-8401-000000000000"); !errors.Is(err, domain.ErrUnknownRelationshipKind) {
		t.Fatalf("bad rid: want ErrUnknownRelationshipKind, got %v", err)
	}

	// Purge b erases every relationship touching b; a-c links remain.
	if _, err := svc.DeactivatePerson(ctx, b.ID, "x"); err != nil {
		t.Fatalf("deactivate b: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, b.ID); err != nil {
		t.Fatalf("purge b: %v", err)
	}
	if ps, err := prof.ListPartnerships(ctx, a.ID); err != nil || len(ps) != 0 {
		t.Fatalf("partnerships for a after purging b: %d err %v", len(ps), err)
	}
	if nks, err := prof.ListNextOfKin(ctx, a.ID); err != nil || len(nks) != 0 {
		t.Fatalf("next-of-kin for a after purging b: %d err %v", len(nks), err)
	}
	if ss, err := prof.ListSponsorships(ctx, a.ID); err != nil || len(ss) != 1 {
		t.Fatalf("sponsorships for a after purging b: %d err %v", len(ss), err)
	}
}

// seedLanguoid inserts a Glottolog languoid (D-Languages, M18) at the given level and returns its RID.
// The composite FK person_languages(language_id, language_level)->language_languoids(id, level) only
// accepts level='language', so the tests seed both a language and a family to prove the constraint.
func seedLanguoid(t *testing.T, pool *pgxpool.Pool, code, level, name string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO oikumenea.language_languoids (code, level, name) VALUES ($1,$2,$3) RETURNING id`,
		code, level, name).Scan(&id); err != nil {
		t.Fatalf("seed languoid %s: %v", code, err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		// A concurrent language-scheme import (other package) rebuilds the global closure, which adds a
		// reflexive row for this bare languoid; clear dependents before the RESTRICT delete.
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.language_languoid_closure WHERE ancestor_id = $1 OR descendant_id = $1", id)
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.person_languages WHERE language_id = $1", id)
		_, _ = pool.Exec(ctx, "DELETE FROM oikumenea.language_languoids WHERE id = $1", id)
	})
	return id
}

// TestPersonLanguages covers the SPEAKS link (D-Languages, M18): add with CEFR + native, the
// (person, language) upsert key, the composite FK that rejects a non-language languoid, delete, and
// purge erasure.
func TestPersonLanguages(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 0)

	lang := seedLanguoid(t, pool, "test1234", "language", "Testish")
	family := seedLanguoid(t, pool, "testfam1", "family", "Testic")
	p := newPerson(t, svc, "Polyglot")

	// add a spoken language with proficiency + native flag
	saved, err := prof.UpsertPersonLanguage(ctx, domain.PersonLanguage{PersonID: p.ID, LanguageID: lang, CEFRLevel: "B2", IsNative: true})
	if err != nil {
		t.Fatalf("upsert language: %v", err)
	}
	if saved.LanguageName != "Testish" || saved.CEFRLevel != "B2" || !saved.IsNative {
		t.Fatalf("saved = %+v, want name=Testish cefr=B2 native", saved)
	}

	// upsert is keyed on (person, language): a second call updates rather than duplicating
	if _, err := prof.UpsertPersonLanguage(ctx, domain.PersonLanguage{PersonID: p.ID, LanguageID: lang, CEFRLevel: "C1"}); err != nil {
		t.Fatalf("update language: %v", err)
	}
	ls, err := prof.ListPersonLanguages(ctx, p.ID)
	if err != nil || len(ls) != 1 {
		t.Fatalf("list after update: len=%d err=%v", len(ls), err)
	}
	if ls[0].CEFRLevel != "C1" || ls[0].IsNative {
		t.Fatalf("updated row = %+v, want cefr=C1 native=false", ls[0])
	}

	// the composite FK rejects a family-level languoid (must be level='language')
	if _, err := prof.UpsertPersonLanguage(ctx, domain.PersonLanguage{PersonID: p.ID, LanguageID: family}); !errors.Is(err, domain.ErrUnknownLanguage) {
		t.Fatalf("family languoid should be rejected with ErrUnknownLanguage, got %v", err)
	}

	// delete, then purge erasure leaves no rows
	if err := prof.DeletePersonLanguage(ctx, p.ID, lang); err != nil {
		t.Fatalf("delete language: %v", err)
	}
	if ls, err := prof.ListPersonLanguages(ctx, p.ID); err != nil || len(ls) != 0 {
		t.Fatalf("list after delete: len=%d err=%v", len(ls), err)
	}

	// re-add, then purge the person — person_languages is pii:basic and must be erased
	if _, err := prof.UpsertPersonLanguage(ctx, domain.PersonLanguage{PersonID: p.ID, LanguageID: lang}); err != nil {
		t.Fatalf("re-add language: %v", err)
	}
	if _, err := svc.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM oikumenea.person_languages WHERE person_id = $1", p.ID).Scan(&remaining); err != nil {
		t.Fatalf("count after purge: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("person_languages after purge = %d, want 0", remaining)
	}
}

// TestPurgePublishesPersonPurged proves the D-PersonModuleSplit (review-2026-07 R-09) purge fan-out end
// to end: a real PurgePerson publishes PersonPurged on the wired bus, so a subscribing module erases its
// own person-referencing rows IN THE PURGE TRANSACTION — the mechanism that replaced person's inline
// cross-module purge deletes. A stand-in account_accounts subscriber (the same SubscribeErase helper the
// real education/company modules use) stands in for a cross-module owner. It also guards the negative:
// without the publish, the row would survive.
func TestPurgePublishesPersonPurged(t *testing.T) {
	ctx := context.Background()
	svc, _, _, pool := newServices(t, 0)

	bus := events.NewBus()
	svc.SubscribeOrderEvents(bus) // sets the service's bus so PurgePerson publishes PersonPurged
	personevents.SubscribeErase(bus,
		`DELETE FROM oikumenea.account_accounts WHERE person_id = $1`)

	p := newPerson(t, svc, "Purge Publisher")
	if _, err := pool.Exec(ctx,
		`INSERT INTO oikumenea.account_accounts (person_id) VALUES ($1)`, p.ID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	if _, err := svc.DeactivatePerson(ctx, p.ID, "x"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := svc.PurgePerson(ctx, p.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.account_accounts WHERE person_id = $1`, p.ID).Scan(&n); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if n != 0 {
		t.Fatalf("PurgePerson did not publish PersonPurged (subscriber left %d account rows, want 0)", n)
	}
}

// TestMergeProvisionalPerson proves the D-OverlayFoundation (M29) merge: a provisional stub is created,
// carries a person-owned edge (kinship) and a cross-module reference (an identity account), then is
// merged into a canonical person — re-homing both in one transaction and tombstoning the stub. It also
// asserts the source must be provisional.
func TestMergeProvisionalPerson(t *testing.T) {
	ctx := context.Background()
	svc, prof, _, pool := newServices(t, 50)

	// Wire the event bus so MergePerson publishes PersonMerged, and register a stand-in for the
	// identity-federation subscriber (the same SubscribeRepoint helper the real module uses) so the
	// cross-module re-home runs in the merge transaction.
	bus := events.NewBus()
	svc.SubscribeOrderEvents(bus) // sets the service's bus (also registers the rank handler — harmless)
	personevents.SubscribeRepoint(bus,
		`UPDATE oikumenea.account_accounts SET person_id = $2 WHERE person_id = $1`)

	// A provisional stub, a canonical target, and a third person the stub is related to.
	stub, err := svc.CreateProvisionalPerson(ctx, domain.Person{Name: domain.Name{DisplayName: "Unresolved Source"}})
	if err != nil {
		t.Fatalf("create provisional: %v", err)
	}
	if stub.Status != domain.StatusProvisional {
		t.Fatalf("stub status = %q, want provisional", stub.Status)
	}
	canonical := newPerson(t, svc, "Canonical Person")
	other := newPerson(t, svc, "Related Person")

	// Person-owned edge: a kinship stub(parent) -> other(child).
	if _, err := prof.UpsertKinship(ctx, stub.ID, domain.Kinship{ParentID: stub.ID, ChildID: other.ID}); err != nil {
		t.Fatalf("add kinship: %v", err)
	}
	// Cross-module reference: an identity account on the stub.
	var acctID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO oikumenea.account_accounts (person_id) VALUES ($1) RETURNING id`, stub.ID).Scan(&acctID); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	// Merge the stub into the canonical person.
	merged, err := svc.MergePerson(ctx, stub.ID, canonical.ID, "confirmed")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.ID != canonical.ID {
		t.Fatalf("merge returned %s, want canonical %s", merged.ID, canonical.ID)
	}

	// The stub is tombstoned (status=purged).
	gone, err := svc.GetPerson(ctx, stub.ID)
	if err != nil {
		t.Fatalf("get stub after merge: %v", err)
	}
	if gone.Status != domain.StatusPurged {
		t.Fatalf("stub status after merge = %q, want purged", gone.Status)
	}

	// The person-owned kinship now belongs to the canonical person.
	ks, err := prof.ListKinships(ctx, canonical.ID)
	if err != nil {
		t.Fatalf("list kinships: %v", err)
	}
	if len(ks) != 1 || ks[0].ParentID != canonical.ID || ks[0].ChildID != other.ID {
		t.Fatalf("kinship not re-homed onto canonical: %+v", ks)
	}

	// The cross-module identity account now points at the canonical person.
	var owner string
	if err := pool.QueryRow(ctx, `SELECT person_id FROM oikumenea.account_accounts WHERE id = $1`, acctID).Scan(&owner); err != nil {
		t.Fatalf("read account owner: %v", err)
	}
	if owner != canonical.ID {
		t.Fatalf("account owner = %s, want canonical %s", owner, canonical.ID)
	}

	// A merge audit row was written.
	var audits int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM oikumenea.audit_log WHERE action = 'person.merge' AND target_id = $1`, canonical.ID).Scan(&audits); err != nil {
		t.Fatalf("count merge audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("merge audit rows = %d, want 1", audits)
	}

	// Merging a non-provisional source is rejected.
	if _, err := svc.MergePerson(ctx, canonical.ID, other.ID, "possible"); !errors.Is(err, domain.ErrMergeNotProvisional) {
		t.Fatalf("merge non-provisional source err = %v, want ErrMergeNotProvisional", err)
	}
}
